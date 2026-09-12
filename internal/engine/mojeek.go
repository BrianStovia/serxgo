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

type MojeekEngine struct {
	client *http.Client
}

func NewMojeekEngine() *MojeekEngine {
	return &MojeekEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *MojeekEngine) Name() string {
	return "mojeek"
}

func (e *MojeekEngine) DisplayName() string {
	return "Mojeek"
}

func (e *MojeekEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral}
}

func (e *MojeekEngine) DefaultOn() bool {
	return true
}

func (e *MojeekEngine) Weight() float64 {
	return 1.1
}

func (e *MojeekEngine) About() string {
	return "Completely independent, crawler-based search engine with its own index of billions of pages."
}

func (e *MojeekEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	start := (page - 1) * 10
	searchURL := fmt.Sprintf("https://www.mojeek.com/search?q=%s&s=%d", url.QueryEscape(req.Query), start)

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
		return nil, fmt.Errorf("mojeek returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "li", "")
	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "ob")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "title")
		}
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "mojeek.com") {
			continue
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "p", "s")
		if descNode == nil {
			descNode = FindFirstNode(node, "p", "snippet")
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
