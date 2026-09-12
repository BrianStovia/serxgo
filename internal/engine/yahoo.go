package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

type YahooEngine struct {
	client *http.Client
}

func NewYahooEngine() *YahooEngine {
	return &YahooEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *YahooEngine) Name() string {
	return "yahoo"
}

func (e *YahooEngine) DisplayName() string {
	return "Yahoo Search"
}

func (e *YahooEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryNews}
}

func (e *YahooEngine) DefaultOn() bool {
	return true
}

func (e *YahooEngine) Weight() float64 {
	return 1.05
}

func (e *YahooEngine) About() string {
	return "Global web search engine aggregating extensive international results."
}

func (e *YahooEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	start := (page - 1) * 10 + 1
	searchURL := fmt.Sprintf("https://search.yahoo.com/search?p=%s&b=%d&nojs=1", url.QueryEscape(req.Query), start)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "div", "algo")
	if len(items) == 0 {
		items = FindNodes(doc, "li", "")
	}

	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "ac-algo")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "yahoo.com/search") {
			continue
		}

		// Yahoo wraps URLs with redirection link (r.search.yahoo.com)
		actualURL := extractYahooURL(href)
		if actualURL == "" {
			actualURL = href
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "div", "compText")
		if descNode == nil {
			descNode = FindFirstNode(node, "p", "")
		}
		desc := ""
		if descNode != nil {
			desc = CleanHTMLText(ExtractText(descNode))
		}

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       actualURL,
			PrettyURL: cleanDisplayURL(actualURL),
			Content:   desc,
			Engine:    e.Name(),
			Category:  models.CategoryGeneral,
		})
	}

	return results, nil
}

func extractYahooURL(raw string) string {
	if strings.Contains(raw, "RU=") {
		parts := strings.Split(raw, "RU=")
		if len(parts) > 1 {
			target := strings.Split(parts[1], "/RK=")[0]
			if unescaped, err := url.QueryUnescape(target); err == nil {
				return unescaped
			}
		}
	}
	return raw
}
