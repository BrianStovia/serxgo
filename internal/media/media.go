package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type MediaResult struct {
	Platform    string `json:"platform"`
	Title       string `json:"title"`
	Author      string `json:"author,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	DownloadURL string `json:"download_url"`
	AudioURL    string `json:"audio_url,omitempty"`
	SourceURL   string `json:"source_url"`
	DurationSec int    `json:"duration_sec,omitempty"`
	MediaType   string `json:"media_type"` // "video", "audio", "photo"
	FileName    string `json:"file_name"`
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

func DetectPlatform(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "Unknown"
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be"):
		return "YouTube"
	case strings.Contains(host, "tiktok.com"):
		return "TikTok"
	case strings.Contains(host, "twitter.com") || strings.Contains(host, "x.com"):
		return "Twitter / X"
	case strings.Contains(host, "instagram.com"):
		return "Instagram"
	case strings.Contains(host, "reddit.com") || strings.Contains(host, "redd.it"):
		return "Reddit"
	case strings.Contains(host, "facebook.com") || strings.Contains(host, "fb.watch"):
		return "Facebook"
	case strings.Contains(host, "soundcloud.com"):
		return "SoundCloud"
	case strings.Contains(host, "vimeo.com"):
		return "Vimeo"
	case strings.Contains(host, "pinterest.com") || strings.Contains(host, "pin.it"):
		return "Pinterest"
	default:
		return "Web Media"
	}
}

// Cobalt instances pool
var cobaltInstances = []string{
	"https://api.cobalt.tools/api/json",
	"https://cobalt.kwiatekm.pl/api/json",
	"https://co.eugen.pt/api/json",
	"https://cobalt.synapticlabs.net/api/json",
}

func (s *Service) Extract(ctx context.Context, targetURL string) (*MediaResult, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	platform := DetectPlatform(targetURL)

	// Specialized Twitter/X Fast Resolver via fxtwitter API
	if platform == "Twitter / X" {
		if res, err := s.extractTwitter(ctx, targetURL); err == nil && res != nil {
			return res, nil
		}
	}

	// Specialized Reddit Fast Resolver
	if platform == "Reddit" {
		if res, err := s.extractReddit(ctx, targetURL); err == nil && res != nil {
			return res, nil
		}
	}

	// Multi-Instance Cobalt Universal Extractor
	for _, endpoint := range cobaltInstances {
		res, err := s.extractCobalt(ctx, endpoint, targetURL, platform)
		if err == nil && res != nil && res.DownloadURL != "" {
			return res, nil
		}
	}

	return nil, fmt.Errorf("unable to extract media streams from %s. Platform might be protected or URL is private.", platform)
}

func (s *Service) extractCobalt(ctx context.Context, apiURL, targetURL, platform string) (*MediaResult, error) {
	reqBody := map[string]interface{}{
		"url":           targetURL,
		"vQuality":      "720",
		"filenameStyle": "classic",
		"downloadMode":  "auto",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 SearXGo-Media-Extractor/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		return nil, fmt.Errorf("cobalt status %d", resp.StatusCode)
	}

	var res struct {
		Status   string `json:"status"` // "stream", "redirect", "picker", "tunnel"
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Picker   []struct {
			Type  string `json:"type"`
			URL   string `json:"url"`
			Thumb string `json:"thumb"`
		} `json:"picker"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	downloadURL := res.URL
	var thumb string
	if len(res.Picker) > 0 {
		if downloadURL == "" {
			downloadURL = res.Picker[0].URL
		}
		thumb = res.Picker[0].Thumb
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("empty stream URL")
	}

	fileName := res.Filename
	if fileName == "" {
		fileName = fmt.Sprintf("media-%s-%d.mp4", strings.ToLower(platform), time.Now().Unix())
	}

	return &MediaResult{
		Platform:    platform,
		Title:       fmt.Sprintf("%s Media Stream", platform),
		SourceURL:   targetURL,
		DownloadURL: downloadURL,
		Thumbnail:   thumb,
		MediaType:   "video",
		FileName:    fileName,
	}, nil
}

func (s *Service) extractTwitter(ctx context.Context, targetURL string) (*MediaResult, error) {
	// Pattern to find status ID
	re := regexp.MustCompile(`status/(\d+)`)
	m := re.FindStringSubmatch(targetURL)
	if len(m) < 2 {
		return nil, fmt.Errorf("no tweet ID found")
	}
	tweetID := m[1]

	apiURL := fmt.Sprintf("https://api.fxtwitter.com/i/status/%s", tweetID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo-Twitter-Resolver/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Tweet struct {
			Text   string `json:"text"`
			Author struct {
				Name       string `json:"name"`
				ScreenName string `json:"screen_name"`
			} `json:"author"`
			Media struct {
				Videos []struct {
					URL          string `json:"url"`
					ThumbnailURL string `json:"thumbnail_url"`
					Duration     float64 `json:"duration"`
				} `json:"videos"`
				Photos []struct {
					URL string `json:"url"`
				} `json:"photos"`
			} `json:"media"`
		} `json:"tweet"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Tweet.Media.Videos) > 0 {
		vid := data.Tweet.Media.Videos[0]
		return &MediaResult{
			Platform:    "Twitter / X",
			Title:       data.Tweet.Text,
			Author:      fmt.Sprintf("%s (@%s)", data.Tweet.Author.Name, data.Tweet.Author.ScreenName),
			Thumbnail:   vid.ThumbnailURL,
			DownloadURL: vid.URL,
			SourceURL:   targetURL,
			DurationSec: int(vid.Duration),
			MediaType:   "video",
			FileName:    fmt.Sprintf("twitter-%s.mp4", tweetID),
		}, nil
	}

	if len(data.Tweet.Media.Photos) > 0 {
		photo := data.Tweet.Media.Photos[0]
		return &MediaResult{
			Platform:    "Twitter / X",
			Title:       data.Tweet.Text,
			Author:      fmt.Sprintf("%s (@%s)", data.Tweet.Author.Name, data.Tweet.Author.ScreenName),
			Thumbnail:   photo.URL,
			DownloadURL: photo.URL,
			SourceURL:   targetURL,
			MediaType:   "photo",
			FileName:    fmt.Sprintf("twitter-%s.jpg", tweetID),
		}, nil
	}

	return nil, fmt.Errorf("no media found in tweet")
}

func (s *Service) extractReddit(ctx context.Context, targetURL string) (*MediaResult, error) {
	cleanURL := strings.Split(targetURL, "?")[0]
	if !strings.HasSuffix(cleanURL, ".json") {
		cleanURL = strings.TrimSuffix(cleanURL, "/") + ".json"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", cleanURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo-Reddit-Resolver/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var posts []struct {
		Data struct {
			Children []struct {
				Data struct {
					Title  string `json:"title"`
					Author string `json:"author"`
					SecureMedia struct {
						RedditVideo struct {
							FallbackURL string `json:"fallback_url"`
							Duration    int    `json:"duration"`
						} `json:"reddit_video"`
					} `json:"secure_media"`
					Thumbnail string `json:"thumbnail"`
					URL       string `json:"url"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil || len(posts) == 0 || len(posts[0].Data.Children) == 0 {
		return nil, fmt.Errorf("reddit post parse error")
	}

	item := posts[0].Data.Children[0].Data
	videoURL := item.SecureMedia.RedditVideo.FallbackURL
	if videoURL == "" && (strings.HasSuffix(item.URL, ".mp4") || strings.HasSuffix(item.URL, ".gif") || strings.HasSuffix(item.URL, ".jpg")) {
		videoURL = item.URL
	}

	if videoURL == "" {
		return nil, fmt.Errorf("no direct video found on reddit post")
	}

	return &MediaResult{
		Platform:    "Reddit",
		Title:       item.Title,
		Author:      "u/" + item.Author,
		Thumbnail:   item.Thumbnail,
		DownloadURL: videoURL,
		SourceURL:   targetURL,
		DurationSec: item.SecureMedia.RedditVideo.Duration,
		MediaType:   "video",
		FileName:    fmt.Sprintf("reddit-media-%d.mp4", time.Now().Unix()),
	}, nil
}
