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

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

type DuckDuckGoEngine struct {
	client *http.Client
}

func NewDuckDuckGoEngine() *DuckDuckGoEngine {
	return &DuckDuckGoEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *DuckDuckGoEngine) Name() string {
	return "duckduckgo"
}

func (e *DuckDuckGoEngine) DisplayName() string {
	return "DuckDuckGo"
}

func (e *DuckDuckGoEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryImages, models.CategoryNews}
}

func (e *DuckDuckGoEngine) DefaultOn() bool {
	return true
}

func (e *DuckDuckGoEngine) Weight() float64 {
	return 1.2
}

func (e *DuckDuckGoEngine) About() string {
	return "Privacy-focused general search engine aggregating various web sources."
}

func (e *DuckDuckGoEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	if req.Category == models.CategoryImages {
		return e.searchImages(ctx, req)
	}
	return e.searchWeb(ctx, req)
}

func (e *DuckDuckGoEngine) searchWeb(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	formData := url.Values{}
	formData.Set("q", req.Query)
	formData.Set("b", "")
	formData.Set("kl", "wt-wt")

	switch req.TimeRange {
	case "day":
		formData.Set("df", "d")
	case "week":
		formData.Set("df", "w")
	case "month":
		formData.Set("df", "m")
	case "year":
		formData.Set("df", "y")
	}

	if req.SafeSearch == models.SafeSearchOff {
		formData.Set("kp", "-2")
	} else if req.SafeSearch == models.SafeSearchStrict {
		formData.Set("kp", "1")
	} else {
		formData.Set("kp", "-1")
	}

	if req.Page > 1 {
		formData.Set("s", fmt.Sprintf("%d", (req.Page-1)*30))
		formData.Set("dc", fmt.Sprintf("%d", (req.Page-1)*30+1))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://html.duckduckgo.com/html/", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	httpReq.Header.Set("Referer", "https://html.duckduckgo.com/")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	resultNodes := FindNodes(doc, "div", "result")

	for _, node := range resultNodes {
		// Ignore ad results or empty result blocks
		if HasClass(node, "result--ad") || HasClass(node, "result--no-result") {
			continue
		}

		titleNode := FindFirstNode(node, "a", "result__a")
		if titleNode == nil {
			titleNode = FindFirstNode(node, "a", "result__url")
		}
		if titleNode == nil {
			continue
		}

		rawURL := GetAttr(titleNode, "href")
		cleanURL := extractDuckDuckGoURL(rawURL)
		if cleanURL == "" || strings.HasPrefix(cleanURL, "/") {
			continue
		}

		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		snippetNode := FindFirstNode(node, "a", "result__snippet")
		snippet := ""
		if snippetNode != nil {
			snippet = CleanHTMLText(ExtractText(snippetNode))
		}

		parsedURL, err := url.Parse(cleanURL)
		prettyURL := cleanURL
		if err == nil {
			prettyURL = parsedURL.Host + parsedURL.Path
		}

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       cleanURL,
			PrettyURL: prettyURL,
			Content:   snippet,
			Engine:    e.Name(),
			Category:  models.CategoryGeneral,
		})
	}

	return results, nil
}

func (e *DuckDuckGoEngine) searchImages(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	// 1. Get VQD token first
	vqdURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&iax=images&ia=images", url.QueryEscape(req.Query))
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

	// Extract vqd token
	vqdRegex := regexp.MustCompile(`vqd=([0-9-]+)`)
	matches := vqdRegex.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		// Fallback regex pattern: vqd: "..." or vqd='...'
		altRegex := regexp.MustCompile(`vqd["']?[:=]["']?([0-9-]+)`)
		matches = altRegex.FindStringSubmatch(bodyStr)
		if len(matches) < 2 {
			return nil, fmt.Errorf("unable to extract duckduckgo image token")
		}
	}
	vqd := matches[1]

	// 2. Fetch images JSON
	safeParam := "1"
	if req.SafeSearch == models.SafeSearchOff {
		safeParam = "-1"
	} else if req.SafeSearch == models.SafeSearchStrict {
		safeParam = "1"
	}

	imgAPI := fmt.Sprintf("https://duckduckgo.com/i.js?l=us-en&o=json&q=%s&vqd=%s&f=,,,,,&p=%s",
		url.QueryEscape(req.Query), vqd, safeParam)

	imgReq, err := http.NewRequestWithContext(ctx, "GET", imgAPI, nil)
	if err != nil {
		return nil, err
	}
	imgReq.Header.Set("User-Agent", GetRandomUserAgent())
	imgReq.Header.Set("Referer", "https://duckduckgo.com/")

	imgResp, err := e.client.Do(imgReq)
	if err != nil {
		return nil, err
	}
	defer imgResp.Body.Close()

	var data struct {
		Results []struct {
			Title     string `json:"title"`
			Image     string `json:"image"`
			Thumbnail string `json:"thumbnail"`
			URL       string `json:"url"`
			Source    string `json:"source"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"results"`
	}

	if err := json.NewDecoder(imgResp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Results {
		if item.Image == "" {
			continue
		}
		targetURL := item.URL
		if targetURL == "" {
			targetURL = item.Image
		}
		results = append(results, models.SearchResult{
			Title:     item.Title,
			URL:       targetURL,
			ImageURL:  item.Image,
			Thumbnail: item.Thumbnail,
			Engine:    e.Name(),
			Category:  models.CategoryImages,
			PrettyURL: item.Source,
		})
	}

	return results, nil
}

func extractDuckDuckGoURL(raw string) string {
	if strings.Contains(raw, "uddg=") {
		u, err := url.Parse(raw)
		if err == nil {
			val := u.Query().Get("uddg")
			if val != "" {
				return val
			}
		}
	}
	if strings.HasPrefix(raw, "//duckduckgo.com/l/?uddg=") {
		raw = "https:" + raw
		u, err := url.Parse(raw)
		if err == nil {
			return u.Query().Get("uddg")
		}
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return ""
}
