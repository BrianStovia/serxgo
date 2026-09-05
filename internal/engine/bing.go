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

type BingEngine struct {
	client *http.Client
}

func NewBingEngine() *BingEngine {
	return &BingEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *BingEngine) Name() string {
	return "bing"
}

func (e *BingEngine) DisplayName() string {
	return "Bing"
}

func (e *BingEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryNews}
}

func (e *BingEngine) DefaultOn() bool {
	return true
}

func (e *BingEngine) Weight() float64 {
	return 1.1
}

func (e *BingEngine) About() string {
	return "Microsoft's global web search engine."
}

func (e *BingEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s&count=15", url.QueryEscape(req.Query))
	if req.Page > 1 {
		searchURL += fmt.Sprintf("&first=%d", (req.Page-1)*15+1)
	}

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
		return nil, fmt.Errorf("bing returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "li", "b_algo")

	for _, item := range items {
		titleNode := FindFirstNode(item, "h2", "")
		if titleNode == nil {
			continue
		}
		linkNode := FindFirstNode(titleNode, "a", "")
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "bing.com") {
			continue
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		captionNode := FindFirstNode(item, "div", "b_caption")
		content := ""
		if captionNode != nil {
			pNode := FindFirstNode(captionNode, "p", "")
			if pNode != nil {
				content = CleanHTMLText(ExtractText(pNode))
			} else {
				content = CleanHTMLText(ExtractText(captionNode))
			}
		}

		parsed, err := url.Parse(href)
		pretty := href
		if err == nil {
			pretty = parsed.Host + parsed.Path
		}

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       href,
			PrettyURL: pretty,
			Content:   content,
			Engine:    e.Name(),
			Category:  models.CategoryGeneral,
		})
	}

	return results, nil
}
