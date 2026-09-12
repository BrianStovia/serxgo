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

type StartpageEngine struct {
	client *http.Client
}

func NewStartpageEngine() *StartpageEngine {
	return &StartpageEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *StartpageEngine) Name() string {
	return "startpage"
}

func (e *StartpageEngine) DisplayName() string {
	return "Startpage"
}

func (e *StartpageEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryNews}
}

func (e *StartpageEngine) DefaultOn() bool {
	return true
}

func (e *StartpageEngine) Weight() float64 {
	return 1.15
}

func (e *StartpageEngine) About() string {
	return "Delivers Google-grade web results enhanced with European privacy protection."
}

func (e *StartpageEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	searchURL := fmt.Sprintf("https://html.startpage.com/do/search?query=%s&page=%d", url.QueryEscape(req.Query), page)
	if req.Language != "" {
		searchURL += fmt.Sprintf("&language=%s", url.QueryEscape(req.Language))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", searchURL, strings.NewReader("query="+url.QueryEscape(req.Query)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback to GET
		getReq, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://www.startpage.com/sp/search?query=%s&page=%d", url.QueryEscape(req.Query), page), nil)
		if err == nil {
			getReq.Header.Set("User-Agent", GetRandomUserAgent())
			if getResp, err := e.client.Do(getReq); err == nil && getResp.StatusCode == http.StatusOK {
				defer getResp.Body.Close()
				return e.parseHTML(getResp.Body, req.Category)
			}
		}
		return nil, fmt.Errorf("startpage returned status %d", resp.StatusCode)
	}

	return e.parseHTML(resp.Body, req.Category)
}

func (e *StartpageEngine) parseHTML(body interface{ Read([]byte) (int, error) }, cat models.Category) ([]models.SearchResult, error) {
	doc, err := xhtml.Parse(body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	resultsNodes := FindNodes(doc, "div", "w-gl__result")
	if len(resultsNodes) == 0 {
		resultsNodes = FindNodes(doc, "div", "result")
	}

	for _, node := range resultsNodes {
		linkNode := FindFirstNode(node, "a", "w-gl__result-title")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "result-link")
		}
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "startpage.com") {
			continue
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "p", "w-gl__description")
		if descNode == nil {
			descNode = FindFirstNode(node, "p", "result-snippet")
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
			Category:  cat,
		})
	}

	return results, nil
}
