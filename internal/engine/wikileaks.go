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

type WikiLeaksEngine struct {
	client *http.Client
}

func NewWikiLeaksEngine() *WikiLeaksEngine {
	return &WikiLeaksEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *WikiLeaksEngine) Name() string {
	return "wikileaks"
}

func (e *WikiLeaksEngine) DisplayName() string {
	return "WikiLeaks & Transparency Archives"
}

func (e *WikiLeaksEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryNews, models.CategoryOther}
}

func (e *WikiLeaksEngine) DefaultOn() bool {
	return true
}

func (e *WikiLeaksEngine) Weight() float64 {
	return 1.3
}

func (e *WikiLeaksEngine) About() string {
	return "Searches public transparency archives, government cables, and investigative leak releases across WikiLeaks, DDoSecrets, and Cryptome."
}

func (e *WikiLeaksEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	cleanQ := strings.TrimSpace(req.Query)
	queryWithSites := fmt.Sprintf("%s (site:wikileaks.org OR site:ddosecrets.com OR site:cryptome.org)", cleanQ)

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(queryWithSites))

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
		return nil, fmt.Errorf("wikileaks aggregator returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	resultsNodes := FindNodes(doc, "div", "result")
	if len(resultsNodes) == 0 {
		resultsNodes = FindNodes(doc, "div", "results_links")
	}

	for _, node := range resultsNodes {
		linkNode := FindFirstNode(node, "a", "result__url")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "result__snippet")
		}
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "duckduckgo.com") {
			continue
		}

		actualURL := extractDuckDuckGoURL(href)
		if actualURL == "" {
			actualURL = href
		}

		titleNode := FindFirstNode(node, "h2", "")
		if titleNode == nil {
			titleNode = linkNode
		}
		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "a", "result__snippet")
		if descNode == nil {
			descNode = FindFirstNode(node, "div", "result__snippet")
		}
		desc := ""
		if descNode != nil {
			desc = CleanHTMLText(ExtractText(descNode))
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("[WikiLeaks/FOIA] %s", title),
			URL:       actualURL,
			PrettyURL: cleanDisplayURL(actualURL),
			Content:   desc,
			Engine:    e.Name(),
			Category:  models.CategoryNews,
		})
	}

	return results, nil
}
