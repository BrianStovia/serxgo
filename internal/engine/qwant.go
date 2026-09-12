package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

type QwantEngine struct {
	client *http.Client
}

func NewQwantEngine() *QwantEngine {
	return &QwantEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *QwantEngine) Name() string {
	return "qwant"
}

func (e *QwantEngine) DisplayName() string {
	return "Qwant"
}

func (e *QwantEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryImages, models.CategoryNews}
}

func (e *QwantEngine) DefaultOn() bool {
	return true
}

func (e *QwantEngine) Weight() float64 {
	return 1.15
}

func (e *QwantEngine) About() string {
	return "Privacy-first European search engine with independent index and zero tracking."
}

func (e *QwantEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	count := req.PageSize
	if count <= 0 {
		count = 20
	}
	offset := (req.Page - 1) * count
	if offset < 0 {
		offset = 0
	}

	locale := "en_US"
	if req.Language != "" {
		locale = req.Language
	}

	searchType := "web"
	switch req.Category {
	case models.CategoryImages:
		searchType = "images"
	case models.CategoryNews:
		searchType = "news"
	}

	safeLevel := 0
	if req.SafeSearch == models.SafeSearchStrict {
		safeLevel = 2
	} else if req.SafeSearch == models.SafeSearchModerate {
		safeLevel = 1
	}

	apiURL := fmt.Sprintf("https://api.qwant.com/v3/search/%s?q=%s&count=%d&offset=%d&locale=%s&safesearch=%d",
		searchType, url.QueryEscape(req.Query), count, offset, locale, safeLevel)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var qwantResp struct {
			Status string `json:"status"`
			Data   struct {
				Result struct {
					Items []struct {
						Title       string `json:"title"`
						URL         string `json:"url"`
						Desc        string `json:"desc"`
						Media       string `json:"media"`
						Thumbnail   string `json:"thumbnail"`
						MediaURL    string `json:"media_url"`
						Date        int64  `json:"date"`
						Source      string `json:"source"`
					} `json:"items"`
				} `json:"result"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&qwantResp); err == nil && len(qwantResp.Data.Result.Items) > 0 {
			var results []models.SearchResult
			for _, item := range qwantResp.Data.Result.Items {
				if item.URL == "" || item.Title == "" {
					continue
				}

				thumb := item.Thumbnail
				if thumb == "" {
					thumb = item.Media
				}
				if thumb == "" {
					thumb = item.MediaURL
				}

				var pubDate *time.Time
				if item.Date > 0 {
					t := time.Unix(item.Date, 0)
					pubDate = &t
				}

				results = append(results, models.SearchResult{
					Title:         CleanHTMLText(item.Title),
					URL:           item.URL,
					PrettyURL:     cleanDisplayURL(item.URL),
					Content:       CleanHTMLText(item.Desc),
					Engine:        e.Name(),
					Category:      req.Category,
					Thumbnail:     thumb,
					ImageURL:      thumb,
					PublishedDate: pubDate,
					Author:        item.Source,
				})
			}
			return results, nil
		}
	}

	// Fallback to Lite Web HTML Scraping
	htmlURL := fmt.Sprintf("https://lite.qwant.com/?q=%s&s=%d", url.QueryEscape(req.Query), safeLevel)
	fallbackReq, err := http.NewRequestWithContext(ctx, "GET", htmlURL, nil)
	if err != nil {
		return nil, err
	}
	fallbackReq.Header.Set("User-Agent", GetRandomUserAgent())
	fallbackReq.Header.Set("Accept", "text/html,application/xhtml+xml")

	fallbackResp, err := e.client.Do(fallbackReq)
	if err != nil {
		return nil, err
	}
	defer fallbackResp.Body.Close()

	if fallbackResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qwant returned status %d", fallbackResp.StatusCode)
	}

	doc, err := xhtml.Parse(fallbackResp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "div", "result")
	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "")
		if linkNode == nil {
			continue
		}
		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "qwant.com") {
			continue
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "p", "desc")
		if descNode == nil {
			descNode = FindFirstNode(node, "p", "")
		}
		desc := ""
		if descNode != nil {
			desc = CleanHTMLText(ExtractText(descNode))
		}

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       href,
			PrettyURL: cleanDisplayURL(href),
			Content:   desc,
			Engine:    e.Name(),
			Category:  req.Category,
		})
	}

	return results, nil
}
