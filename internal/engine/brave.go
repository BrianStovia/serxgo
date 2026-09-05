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

type BraveEngine struct {
	client *http.Client
}

func NewBraveEngine() *BraveEngine {
	return &BraveEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *BraveEngine) Name() string {
	return "brave"
}

func (e *BraveEngine) DisplayName() string {
	return "Brave Search"
}

func (e *BraveEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryNews}
}

func (e *BraveEngine) DefaultOn() bool {
	return true
}

func (e *BraveEngine) Weight() float64 {
	return 1.1
}

func (e *BraveEngine) About() string {
	return "Independent privacy search engine powered by Brave's web index."
}

func (e *BraveEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://search.brave.com/search?q=%s&source=web", url.QueryEscape(req.Query))
	if req.Page > 1 {
		searchURL += fmt.Sprintf("&offset=%d", (req.Page-1)*20)
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
		return nil, fmt.Errorf("brave returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult

	// Brave result containers often have class "snippet" or data-type="search"
	snippets := FindNodes(doc, "div", "snippet")
	if len(snippets) == 0 {
		snippets = FindNodes(doc, "div", "result")
	}

	for _, node := range snippets {
		// Title link
		linkNode := FindFirstNode(node, "a", "")
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "brave.com") {
			continue
		}

		titleNode := FindFirstNode(node, "span", "snippet-title")
		if titleNode == nil {
			titleNode = FindFirstNode(node, "div", "title")
		}
		if titleNode == nil {
			titleNode = linkNode
		}

		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		// Snippet content
		descNode := FindFirstNode(node, "p", "snippet-description")
		if descNode == nil {
			descNode = FindFirstNode(node, "div", "snippet-content")
		}
		if descNode == nil {
			descNode = FindFirstNode(node, "div", "content")
		}

		content := ""
		if descNode != nil {
			content = CleanHTMLText(ExtractText(descNode))
		}

		parsedURL, err := url.Parse(href)
		pretty := href
		if err == nil {
			pretty = parsedURL.Host + parsedURL.Path
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
