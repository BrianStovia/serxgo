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

type SwisscowsEngine struct {
	client *http.Client
}

func NewSwisscowsEngine() *SwisscowsEngine {
	return &SwisscowsEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *SwisscowsEngine) Name() string {
	return "swisscows"
}

func (e *SwisscowsEngine) DisplayName() string {
	return "Swisscows"
}

func (e *SwisscowsEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryImages}
}

func (e *SwisscowsEngine) DefaultOn() bool {
	return true
}

func (e *SwisscowsEngine) Weight() float64 {
	return 1.1
}

func (e *SwisscowsEngine) About() string {
	return "Swiss privacy-respecting semantic search engine with zero data tracking."
}

func (e *SwisscowsEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	apiURL := fmt.Sprintf("https://swisscows.com/api/web/search?query=%s&page=%d&itemsCount=15&region=iv", url.QueryEscape(req.Query), page)
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
		var swData struct {
			Items []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&swData); err == nil && len(swData.Items) > 0 {
			var results []models.SearchResult
			for _, item := range swData.Items {
				if item.URL == "" || item.Title == "" {
					continue
				}
				results = append(results, models.SearchResult{
					Title:     CleanHTMLText(item.Title),
					URL:       item.URL,
					PrettyURL: cleanDisplayURL(item.URL),
					Content:   CleanHTMLText(item.Description),
					Engine:    e.Name(),
					Category:  models.CategoryGeneral,
				})
			}
			return results, nil
		}
	}

	// Fallback to Web Scraping
	webURL := fmt.Sprintf("https://swisscows.com/en/web?query=%s&page=%d", url.QueryEscape(req.Query), page)
	webReq, err := http.NewRequestWithContext(ctx, "GET", webURL, nil)
	if err != nil {
		return nil, err
	}
	webReq.Header.Set("User-Agent", GetRandomUserAgent())
	webReq.Header.Set("Accept", "text/html,application/xhtml+xml")

	webResp, err := e.client.Do(webReq)
	if err != nil {
		return nil, err
	}
	defer webResp.Body.Close()

	if webResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("swisscows returned status %d", webResp.StatusCode)
	}

	doc, err := xhtml.Parse(webResp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "article", "item")
	if len(items) == 0 {
		items = FindNodes(doc, "div", "item")
	}

	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "title")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "swisscows.com") {
			continue
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "p", "description")
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
			Category:  models.CategoryGeneral,
		})
	}

	return results, nil
}
