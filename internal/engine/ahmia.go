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

type AhmiaEngine struct {
	client *http.Client
}

func NewAhmiaEngine() *AhmiaEngine {
	return &AhmiaEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *AhmiaEngine) Name() string {
	return "ahmia"
}

func (e *AhmiaEngine) DisplayName() string {
	return "Ahmia (Tor Onion)"
}

func (e *AhmiaEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryFiles, models.CategoryOther}
}

func (e *AhmiaEngine) DefaultOn() bool {
	return true
}

func (e *AhmiaEngine) Weight() float64 {
	return 1.2
}

func (e *AhmiaEngine) About() string {
	return "Verified Tor hidden services (.onion) and privacy Deep Web index."
}

func (e *AhmiaEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://ahmia.fi/search/?q=%s", url.QueryEscape(req.Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ahmia returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "li", "result")
	if len(items) == 0 {
		items = FindNodes(doc, "div", "result")
	}

	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "")
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "ahmia.fi/search") {
			continue
		}

		// Ahmia redirect link parsing: e.g. /search/redirect?url=http://xxx.onion
		if strings.Contains(href, "redirect?url=") {
			parts := strings.Split(href, "redirect?url=")
			if len(parts) > 1 {
				if unesc, err := url.QueryUnescape(parts[1]); err == nil {
					href = unesc
				}
			}
		}

		title := CleanHTMLText(ExtractText(linkNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "p", "")
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
