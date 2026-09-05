package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"searxgo/internal/models"
)

type YouTubeEngine struct {
	client *http.Client
}

func NewYouTubeEngine() *YouTubeEngine {
	return &YouTubeEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *YouTubeEngine) Name() string {
	return "youtube"
}

func (e *YouTubeEngine) DisplayName() string {
	return "YouTube & Videos"
}

func (e *YouTubeEngine) Categories() []models.Category {
	return []models.Category{models.CategoryVideos, models.CategoryGeneral}
}

func (e *YouTubeEngine) DefaultOn() bool {
	return true
}

func (e *YouTubeEngine) Weight() float64 {
	return 1.4
}

func (e *YouTubeEngine) About() string {
	return "Global video sharing aggregator featuring YouTube, Vimeo, and web video streaming."
}

func (e *YouTubeEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	// 1. First attempt: DuckDuckGo Fast Video API (Aggregates YouTube & Web Videos)
	results, err := e.searchDDGVideos(ctx, req)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// 2. Fallback: Invidious API with fast 2.5s timeout
	return e.searchInvidiousFallback(ctx, req)
}

func (e *YouTubeEngine) searchDDGVideos(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	// Step 1: Extract VQD token
	vqdURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&iax=videos&ia=videos", url.QueryEscape(req.Query))
	tokenReq, err := http.NewRequestWithContext(ctx, "GET", vqdURL, nil)
	if err != nil {
		return nil, err
	}
	tokenReq.Header.Set("User-Agent", GetRandomUserAgent())

	tokenResp, err := e.client.Do(tokenReq)
	if err != nil {
		return nil, err
	}
	defer tokenResp.Body.Close()

	bodyBytes, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return nil, err
	}
	bodyStr := string(bodyBytes)

	vqdRegex := regexp.MustCompile(`vqd=([0-9-]+)`)
	matches := vqdRegex.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		altRegex := regexp.MustCompile(`vqd["']?[:=]["']?([0-9-]+)`)
		matches = altRegex.FindStringSubmatch(bodyStr)
		if len(matches) < 2 {
			return nil, fmt.Errorf("unable to extract video token")
		}
	}
	vqd := matches[1]

	// Step 2: Fetch videos JSON
	safeParam := "1"
	if req.SafeSearch == models.SafeSearchOff {
		safeParam = "-1"
	} else if req.SafeSearch == models.SafeSearchStrict {
		safeParam = "1"
	}

	vidAPI := fmt.Sprintf("https://duckduckgo.com/v.js?l=us-en&o=json&q=%s&vqd=%s&f=,,,,,&p=%s",
		url.QueryEscape(req.Query), vqd, safeParam)

	vidReq, err := http.NewRequestWithContext(ctx, "GET", vidAPI, nil)
	if err != nil {
		return nil, err
	}
	vidReq.Header.Set("User-Agent", GetRandomUserAgent())
	vidReq.Header.Set("Referer", "https://duckduckgo.com/")

	vidResp, err := e.client.Do(vidReq)
	if err != nil {
		return nil, err
	}
	defer vidResp.Body.Close()

	if vidResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo video API returned %d", vidResp.StatusCode)
	}

	var data struct {
		Results []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Content     string `json:"content"` // Video URL
			Duration    string `json:"duration"`
			Publisher   string `json:"publisher"`
			Published   string `json:"published"`
			Images      struct {
				Large  string `json:"large"`
				Medium string `json:"medium"`
				Small  string `json:"small"`
			} `json:"images"`
		} `json:"results"`
	}

	if err := json.NewDecoder(vidResp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Results {
		if item.Content == "" {
			continue
		}

		thumb := item.Images.Large
		if thumb == "" {
			thumb = item.Images.Medium
		}
		if thumb == "" {
			thumb = item.Images.Small
		}

		// Check if YouTube URL to construct embed player
		embedURL := ""
		if strings.Contains(item.Content, "youtube.com/watch?v=") {
			u, err := url.Parse(item.Content)
			if err == nil {
				vidID := u.Query().Get("v")
				if vidID != "" {
					embedURL = fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s?autoplay=1", vidID)
					if thumb == "" {
						thumb = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vidID)
					}
				}
			}
		} else if strings.Contains(item.Content, "youtu.be/") {
			parts := strings.Split(item.Content, "youtu.be/")
			if len(parts) > 1 {
				vidID := strings.Split(parts[1], "?")[0]
				embedURL = fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s?autoplay=1", vidID)
			}
		}

		parsed, err := url.Parse(item.Content)
		pretty := item.Content
		if err == nil {
			pretty = parsed.Host + parsed.Path
		}

		extra := make(map[string]string)
		if item.Duration != "" {
			extra["Duration"] = item.Duration
		}
		if item.Publisher != "" {
			extra["Publisher"] = item.Publisher
		}

		results = append(results, models.SearchResult{
			Title:     CleanHTMLText(item.Title),
			URL:       item.Content,
			PrettyURL: pretty,
			Content:   CleanHTMLText(item.Description),
			Engine:    e.Name(),
			Category:  models.CategoryVideos,
			Thumbnail: thumb,
			VideoURL:  embedURL,
			Duration:  item.Duration,
			Author:    item.Publisher,
			Extra:     extra,
		})
	}

	return results, nil
}

func (e *YouTubeEngine) searchInvidiousFallback(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	instances := []string{
		"https://yt.artemislena.eu",
		"https://invidious.nerdvpn.de",
	}

	for _, inst := range instances {
		apiURL := fmt.Sprintf("%s/api/v1/search?q=%s&type=video", inst, url.QueryEscape(req.Query))
		httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			continue
		}
		httpReq.Header.Set("User-Agent", "SearXGo/1.0")

		resp, err := e.client.Do(httpReq)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var data []struct {
				Title         string `json:"title"`
				VideoID       string `json:"videoId"`
				Author        string `json:"author"`
				LengthSeconds int    `json:"lengthSeconds"`
				Description   string `json:"description"`
				PublishedText string `json:"publishedText"`
			}
			err = json.NewDecoder(resp.Body).Decode(&data)
			resp.Body.Close()

			if err == nil && len(data) > 0 {
				var results []models.SearchResult
				for _, item := range data {
					if item.VideoID == "" {
						continue
					}
					durStr := ""
					if item.LengthSeconds > 0 {
						durStr = fmt.Sprintf("%d:%02d", item.LengthSeconds/60, item.LengthSeconds%60)
					}
					results = append(results, models.SearchResult{
						Title:     item.Title,
						URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.VideoID),
						PrettyURL: fmt.Sprintf("youtube.com/watch?v=%s", item.VideoID),
						Content:   item.Description,
						Engine:    e.Name(),
						Category:  models.CategoryVideos,
						Thumbnail: fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", item.VideoID),
						VideoURL:  fmt.Sprintf("https://www.youtube-nocookie.com/embed/%s?autoplay=1", item.VideoID),
						Duration:  durStr,
						Author:    item.Author,
					})
				}
				return results, nil
			}
		} else {
			resp.Body.Close()
		}
	}

	return nil, fmt.Errorf("all video search sources failed")
}
