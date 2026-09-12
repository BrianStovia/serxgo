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

type EcosiaEngine struct {
	client *http.Client
}

func NewEcosiaEngine() *EcosiaEngine {
	return &EcosiaEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *EcosiaEngine) Name() string {
	return "ecosia"
}

func (e *EcosiaEngine) DisplayName() string {
	return "Ecosia"
}

func (e *EcosiaEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryImages}
}

func (e *EcosiaEngine) DefaultOn() bool {
	return true
}

func (e *EcosiaEngine) Weight() float64 {
	return 1.05
}

func (e *EcosiaEngine) About() string {
	return "Eco-friendly search engine that uses 100% renewable energy and funds reforestation."
}

func (e *EcosiaEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	searchPath := "search"
	if req.Category == models.CategoryImages {
		searchPath = "images"
	}

	searchURL := fmt.Sprintf("https://www.ecosia.org/%s?q=%s&p=%d", searchPath, url.QueryEscape(req.Query), page-1)

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
		return nil, fmt.Errorf("ecosia returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "div", "result")
	if len(items) == 0 {
		items = FindNodes(doc, "article", "")
	}

	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "result-title")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "ecosia.org") {
			continue
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "p", "result-snippet")
		if descNode == nil {
			descNode = FindFirstNode(node, "div", "snippet")
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
