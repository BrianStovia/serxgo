package tiktoklive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StreamQuality represents an available stream resolution
type StreamQuality struct {
	Name string `json:"name"` // "Origin", "HD", "SD", "LD"
	URL  string `json:"url"`
	Type string `json:"type"` // "m3u8", "flv"
}

// LiveRoomInfo represents extracted metadata for a TikTok Live room
type LiveRoomInfo struct {
	IsLive      bool            `json:"is_live"`
	RoomID      string          `json:"room_id"`
	Username    string          `json:"username"`
	Nickname    string          `json:"nickname"`
	Avatar      string          `json:"avatar"`
	Title       string          `json:"title"`
	ViewerCount int64           `json:"viewer_count"`
	LikeCount   int64           `json:"like_count"`
	StartedAt   int64           `json:"started_at"`
	StreamURL   string          `json:"stream_url"` // Default best m3u8
	FLVURL      string          `json:"flv_url,omitempty"`
	Qualities   []StreamQuality `json:"qualities"`
	ShareURL    string          `json:"share_url"`
}

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

var (
	csRegex        = regexp.MustCompile(`id="cs"\s+class="([^"]+)"`)
	rehydrationReg = regexp.MustCompile(`<script id="__UNIVERSAL_DATA_FOR_REHYDRATION__"[^>]*>([\s\S]*?)</script>`)
	sigiRegex      = regexp.MustCompile(`<script id="SIGI_STATE"[^>]*>([\s\S]*?)</script>`)
	roomIDRegex    = regexp.MustCompile(`"roomId"\s*:\s*"?(\d{15,25})"?`)
)

// CleanUsername extracts clean username from URL or handle
func CleanUsername(input string) string {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err == nil {
			path := strings.Trim(u.Path, "/")
			parts := strings.Split(path, "/")
			for _, p := range parts {
				if strings.HasPrefix(p, "@") {
					return strings.TrimPrefix(p, "@")
				}
			}
			if len(parts) > 0 {
				return strings.TrimPrefix(parts[0], "@")
			}
		}
	}
	input = strings.TrimPrefix(input, "@")
	if idx := strings.Index(input, "/"); idx != -1 {
		input = input[:idx]
	}
	return input
}

// SolveSlardarWAF solves the SHA256 PoW challenge provided by TikTok WAF
func SolveSlardarWAF(csB64 string) (string, error) {
	pad := len(csB64) % 4
	if pad > 0 {
		csB64 += strings.Repeat("=", 4-pad)
	}
	raw, err := base64.StdEncoding.DecodeString(csB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode cs base64: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("failed to unmarshal cs json: %w", err)
	}

	v, ok := data["v"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid cs format: missing v")
	}

	aStr, _ := v["a"].(string)
	cStr, _ := v["c"].(string)

	aBytes, err := base64.StdEncoding.DecodeString(aStr)
	if err != nil {
		return "", fmt.Errorf("failed to decode v.a: %w", err)
	}
	cBytes, err := base64.StdEncoding.DecodeString(cStr)
	if err != nil {
		return "", fmt.Errorf("failed to decode v.c: %w", err)
	}

	// Solve PoW: find non-negative integer i where sha256(aBytes + str(i)) == cBytes
	var solution int = -1
	for i := 0; i < 500000; i++ {
		h := sha256.Sum256(append(aBytes, []byte(strconv.Itoa(i))...))
		if bytes.Equal(h[:], cBytes) {
			solution = i
			break
		}
	}

	if solution == -1 {
		return "", fmt.Errorf("could not find WAF PoW solution")
	}

	v["d"] = solution
	data["v"] = v

	solvedJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(solvedJSON), nil
}

// Extract retrieves live stream metadata and m3u8 URLs for a given TikTok username, URL, or room ID
func (s *Service) Extract(ctx context.Context, input string) (*LiveRoomInfo, error) {
	username := CleanUsername(input)
	if username == "" {
		return nil, fmt.Errorf("username or TikTok live URL is required")
	}

	// If numeric input, it might be a direct room_id
	isNumeric := regexp.MustCompile(`^\d{15,25}$`).MatchString(username)
	if isNumeric {
		return s.FetchRoomDetail(ctx, username, username)
	}

	var lastOfflineInfo *LiveRoomInfo

	// 1. Try primary official web API endpoint (Fast, structured JSON, bypasses SlardarWAF)
	if apiInfo, err := s.fetchLiveFromAPI(ctx, username); err == nil && apiInfo != nil {
		if apiInfo.IsLive {
			return apiInfo, nil
		}
		lastOfflineInfo = apiInfo
	}

	// 2. Try HTML scraping with WAF solver as secondary fallback
	targetURL := fmt.Sprintf("https://www.tiktok.com/@%s/live", username)
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Sec-Ch-Ua", `"Chromium";v="126", "Google Chrome";v="126", "Not-A.Brand";v="99"`)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")

		resp, err := s.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyStr := string(bodyBytes)

			// Check if challenged by SlardarWAF
			if csMatch := csRegex.FindStringSubmatch(bodyStr); len(csMatch) > 1 {
				if sol, solveErr := SolveSlardarWAF(csMatch[1]); solveErr == nil && sol != "" {
					retryReq, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
					retryReq.Header = req.Header.Clone()
					retryReq.Header.Set("Cookie", fmt.Sprintf("_wafchallengeid=%s", sol))
					if retryResp, err := s.httpClient.Do(retryReq); err == nil {
						defer retryResp.Body.Close()
						if retryBytes, err := io.ReadAll(retryResp.Body); err == nil && len(retryBytes) > len(bodyBytes) {
							bodyStr = string(retryBytes)
						}
					}
				}
			}

			// Try SIGI_STATE
			if m := sigiRegex.FindStringSubmatch(bodyStr); len(m) > 1 {
				var root map[string]interface{}
				if err := json.Unmarshal([]byte(m[1]), &root); err == nil {
					if info := s.parseSigiState(root, username); info != nil {
						if info.IsLive {
							return info, nil
						}
						if lastOfflineInfo == nil || (lastOfflineInfo.Avatar == "" && info.Avatar != "") {
							lastOfflineInfo = info
						}
					}
				}
			}

			// Try __UNIVERSAL_DATA_FOR_REHYDRATION__
			if m := rehydrationReg.FindStringSubmatch(bodyStr); len(m) > 1 {
				var root map[string]interface{}
				if err := json.Unmarshal([]byte(m[1]), &root); err == nil {
					if info := s.parseRehydrationData(root, username); info != nil {
						if info.IsLive {
							return info, nil
						}
						if lastOfflineInfo == nil || (lastOfflineInfo.Avatar == "" && info.Avatar != "") {
							lastOfflineInfo = info
						}
					}
				}
			}

			// Try Room ID fallback check
			if rMatch := roomIDRegex.FindStringSubmatch(bodyStr); len(rMatch) > 1 {
				if detailInfo, err := s.FetchRoomDetail(ctx, rMatch[1], username); err == nil && detailInfo != nil && detailInfo.IsLive {
					return detailInfo, nil
				}
			}
		}
	}

	// 3. Return offline info if profile found
	if lastOfflineInfo != nil {
		return lastOfflineInfo, nil
	}

	// 4. Default fallback
	return &LiveRoomInfo{
		IsLive:    false,
		Username:  username,
		Nickname:  username,
		ShareURL:  fmt.Sprintf("https://www.tiktok.com/@%s/live", username),
		Qualities: []StreamQuality{},
	}, nil
}

func (s *Service) fetchLiveFromAPI(ctx context.Context, username string) (*LiveRoomInfo, error) {
	apiURL := fmt.Sprintf("https://www.tiktok.com/api-live/user/room/?aid=1988&app_name=tiktok_web&device_platform=web_pc&sourceType=54&uniqueId=%s", url.QueryEscape(username))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.tiktok.com/")
	req.Header.Set("Cookie", "tt-target-idc=useast1a")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="126", "Google Chrome";v="126", "Not-A.Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			User     map[string]interface{} `json:"user"`
			LiveRoom map[string]interface{} `json:"liveRoom"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Data.User == nil {
		return nil, fmt.Errorf("user not found in api response")
	}

	return s.parseUserInfo(result.Data.User, result.Data.LiveRoom, username), nil
}

func (s *Service) parseUserInfo(user, liveRoom map[string]interface{}, username string) *LiveRoomInfo {
	if user == nil {
		return nil
	}

	nickname := username
	if n, ok := user["nickname"].(string); ok && n != "" {
		nickname = n
	}

	avatar := ""
	if av, ok := user["avatarLarger"].(string); ok && av != "" {
		avatar = av
	} else if av, ok := user["avatarMedium"].(string); ok && av != "" {
		avatar = av
	} else if av, ok := user["avatarThumb"].(string); ok && av != "" {
		avatar = av
	}

	roomID := ""
	if r, ok := user["roomId"].(string); ok {
		roomID = r
	} else if r, ok := user["roomId"].(float64); ok {
		roomID = fmt.Sprintf("%.0f", r)
	}

	statusNum := 0
	if st, ok := user["status"].(float64); ok {
		statusNum = int(st)
	}

	title := ""
	var viewerCount int64
	var startedAt int64

	if liveRoom != nil {
		if t, ok := liveRoom["title"].(string); ok {
			title = t
		}
		if st, ok := liveRoom["startTime"].(float64); ok {
			startedAt = int64(st)
		}
		if lrs, ok := liveRoom["liveRoomStats"].(map[string]interface{}); ok {
			if uc, ok := lrs["userCount"].(float64); ok {
				viewerCount = int64(uc)
			}
		}
		if statusNum == 0 {
			if lrs, ok := liveRoom["status"].(float64); ok {
				statusNum = int(lrs)
			}
		}
	}

	isLive := (statusNum == 2)

	info := &LiveRoomInfo{
		IsLive:      isLive,
		RoomID:      roomID,
		Username:    username,
		Nickname:    nickname,
		Avatar:      avatar,
		Title:       title,
		ViewerCount: viewerCount,
		StartedAt:   startedAt,
		ShareURL:    fmt.Sprintf("https://www.tiktok.com/@%s/live", username),
		Qualities:   []StreamQuality{},
	}

	if liveRoom != nil {
		if sd, ok := liveRoom["streamData"].(map[string]interface{}); ok {
			s.populateFromStreamData(info, sd)
		} else if su, ok := liveRoom["streamUrl"].(map[string]interface{}); ok {
			s.populateStreams(info, su)
		}
	}

	return info
}

func (s *Service) parseSigiState(root map[string]interface{}, username string) *LiveRoomInfo {
	liveRoomMap, ok := root["LiveRoom"].(map[string]interface{})
	if !ok {
		return nil
	}

	liveRoomUserInfo, _ := liveRoomMap["liveRoomUserInfo"].(map[string]interface{})
	if liveRoomUserInfo == nil {
		return nil
	}

	user, _ := liveRoomUserInfo["user"].(map[string]interface{})
	liveRoom, _ := liveRoomUserInfo["liveRoom"].(map[string]interface{})

	return s.parseUserInfo(user, liveRoom, username)
}

func (s *Service) parseRehydrationData(root map[string]interface{}, username string) *LiveRoomInfo {
	scope, ok := root["__DEFAULT_SCOPE__"].(map[string]interface{})
	if !ok {
		return nil
	}

	liveDetail, ok := scope["webapp.live-detail"].(map[string]interface{})
	if !ok {
		return nil
	}

	liveRoomUserInfo, _ := liveDetail["liveRoomUserInfo"].(map[string]interface{})
	if liveRoomUserInfo != nil {
		user, _ := liveRoomUserInfo["user"].(map[string]interface{})
		lr, _ := liveRoomUserInfo["liveRoom"].(map[string]interface{})
		if info := s.parseUserInfo(user, lr, username); info != nil {
			return info
		}
	}

	liveRoom, ok := liveDetail["liveRoom"].(map[string]interface{})
	if !ok {
		return nil
	}

	status, _ := liveRoom["status"].(float64)
	isLive := int(status) == 2

	roomID := fmt.Sprintf("%v", liveRoom["roomId"])
	title, _ := liveRoom["title"].(string)

	nickname := username
	avatar := ""
	if owner, ok := liveRoom["owner"].(map[string]interface{}); ok {
		if n, ok := owner["nickname"].(string); ok && n != "" {
			nickname = n
		}
		if av, ok := owner["avatar_large"].(map[string]interface{}); ok {
			if urlList, ok := av["url_list"].([]interface{}); ok && len(urlList) > 0 {
				avatar, _ = urlList[0].(string)
			}
		}
	}

	var viewerCount int64
	if stats, ok := liveRoom["liveRoomStats"].(map[string]interface{}); ok {
		if uc, ok := stats["userCount"].(float64); ok {
			viewerCount = int64(uc)
		}
	}

	info := &LiveRoomInfo{
		IsLive:      isLive,
		RoomID:      roomID,
		Username:    username,
		Nickname:    nickname,
		Avatar:      avatar,
		Title:       title,
		ViewerCount: viewerCount,
		ShareURL:    fmt.Sprintf("https://www.tiktok.com/@%s/live", username),
		Qualities:   []StreamQuality{},
	}

	if sd, ok := liveRoom["streamData"].(map[string]interface{}); ok {
		s.populateFromStreamData(info, sd)
	} else if streamURLData, ok := liveRoom["streamUrl"].(map[string]interface{}); ok {
		s.populateStreams(info, streamURLData)
	}

	return info
}

func (s *Service) populateFromStreamData(info *LiveRoomInfo, sd map[string]interface{}) {
	pullData, _ := sd["pull_data"].(map[string]interface{})
	if pullData == nil {
		return
	}

	streamDataStr, _ := pullData["stream_data"].(string)
	if streamDataStr == "" {
		return
	}

	var root map[string]interface{}
	if err := json.Unmarshal([]byte(streamDataStr), &root); err != nil {
		return
	}

	dataMap, _ := root["data"].(map[string]interface{})
	if dataMap == nil {
		return
	}

	order := []string{"origin", "uhd", "hd", "sd", "ld"}
	for _, key := range order {
		qObj, ok := dataMap[key].(map[string]interface{})
		if !ok {
			continue
		}
		mainMap, _ := qObj["main"].(map[string]interface{})
		if mainMap == nil {
			continue
		}

		hlsURL, _ := mainMap["hls"].(string)
		flvURL, _ := mainMap["flv"].(string)

		qLabel := strings.ToUpper(key)
		switch key {
		case "origin":
			qLabel = "Origin (1080p)"
		case "uhd":
			qLabel = "UHD (720p 60fps)"
		case "hd":
			qLabel = "HD (720p)"
		case "sd":
			qLabel = "SD (540p)"
		case "ld":
			qLabel = "Low (360p)"
		}

		if hlsURL != "" {
			info.Qualities = append(info.Qualities, StreamQuality{
				Name: qLabel + " [HLS]",
				URL:  hlsURL,
				Type: "m3u8",
			})
			if info.StreamURL == "" {
				info.StreamURL = hlsURL
			}
		}
		if flvURL != "" {
			info.Qualities = append(info.Qualities, StreamQuality{
				Name: qLabel + " [FLV]",
				URL:  flvURL,
				Type: "flv",
			})
			if info.FLVURL == "" {
				info.FLVURL = flvURL
			}
		}
	}
}

// FetchRoomDetail queries TikTok's live room detail API using room_id
func (s *Service) FetchRoomDetail(ctx context.Context, roomID, username string) (*LiveRoomInfo, error) {
	apiURL := fmt.Sprintf("https://www.tiktok.com/api/live/detail/?roomID=%s", roomID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://www.tiktok.com/@%s/live", username))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	liveRoomInfo, ok := data["LiveRoomInfo"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no LiveRoomInfo in response")
	}

	status, _ := liveRoomInfo["status"].(float64)
	isLive := int(status) == 2

	title, _ := liveRoomInfo["title"].(string)
	ownerInfo, _ := liveRoomInfo["ownerInfo"].(map[string]interface{})
	nickname := username
	avatar := ""
	if ownerInfo != nil {
		if n, ok := ownerInfo["nickname"].(string); ok {
			nickname = n
		}
		if av, ok := ownerInfo["avatarLarger"].(string); ok {
			avatar = av
		}
	}

	info := &LiveRoomInfo{
		IsLive:    isLive,
		RoomID:    roomID,
		Username:  username,
		Nickname:  nickname,
		Avatar:    avatar,
		Title:     title,
		ShareURL:  fmt.Sprintf("https://www.tiktok.com/@%s/live", username),
		Qualities: []StreamQuality{},
	}

	if sd, ok := liveRoomInfo["streamData"].(map[string]interface{}); ok {
		s.populateFromStreamData(info, sd)
	} else if streamURL, ok := liveRoomInfo["streamUrl"].(map[string]interface{}); ok {
		s.populateStreams(info, streamURL)
	}

	return info, nil
}

func (s *Service) populateStreams(info *LiveRoomInfo, streamURL map[string]interface{}) {
	// 1. Direct pull_url
	if pullURL, ok := streamURL["pull_url"].(map[string]interface{}); ok {
		for qName, qVal := range pullURL {
			if uStr, ok := qVal.(string); ok && uStr != "" {
				info.Qualities = append(info.Qualities, StreamQuality{
					Name: strings.ToUpper(qName),
					URL:  uStr,
					Type: "flv",
				})
				if info.FLVURL == "" {
					info.FLVURL = uStr
				}
			}
		}
	}

	// 2. live_core_sdk_data -> pull_data -> stream_data
	if liveCore, ok := streamURL["live_core_sdk_data"].(map[string]interface{}); ok {
		if pullData, ok := liveCore["pull_data"].(map[string]interface{}); ok {
			if streamDataStr, ok := pullData["stream_data"].(string); ok && streamDataStr != "" {
				var streamData map[string]interface{}
				if err := json.Unmarshal([]byte(streamDataStr), &streamData); err == nil {
					if d, ok := streamData["data"].(map[string]interface{}); ok {
						for qualityKey, qObj := range d {
							qMap, ok := qObj.(map[string]interface{})
							if !ok {
								continue
							}
							mainMap, _ := qMap["main"].(map[string]interface{})
							if mainMap == nil {
								continue
							}
							hlsURL, _ := mainMap["hls"].(string)
							flvURL, _ := mainMap["flv"].(string)

							qLabel := strings.ToUpper(qualityKey)
							switch qualityKey {
							case "origin":
								qLabel = "Origin (1080p)"
							case "uhd":
								qLabel = "UHD (720p 60fps)"
							case "hd":
								qLabel = "HD (720p)"
							case "sd":
								qLabel = "SD (540p)"
							case "ld":
								qLabel = "Low (480p)"
							}

							if hlsURL != "" {
								info.Qualities = append(info.Qualities, StreamQuality{
									Name: qLabel + " [HLS]",
									URL:  hlsURL,
									Type: "m3u8",
								})
								if info.StreamURL == "" || qualityKey == "origin" || qualityKey == "uhd" {
									info.StreamURL = hlsURL
								}
							}
							if flvURL != "" {
								info.Qualities = append(info.Qualities, StreamQuality{
									Name: qLabel + " [FLV]",
									URL:  flvURL,
									Type: "flv",
								})
								if info.FLVURL == "" {
									info.FLVURL = flvURL
								}
							}
						}
					}
				}
			}
		}
	}

	// 3. Fallback to hls_pull_url
	if hlsPull, ok := streamURL["hls_pull_url"].(string); ok && hlsPull != "" && info.StreamURL == "" {
		info.StreamURL = hlsPull
		info.Qualities = append(info.Qualities, StreamQuality{
			Name: "Default [HLS]",
			URL:  hlsPull,
			Type: "m3u8",
		})
	}
}
