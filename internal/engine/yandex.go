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

type YandexEngine struct {
	client *http.Client
}

func NewYandexEngine() *YandexEngine {
	return &YandexEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *YandexEngine) Name() string {
	return "yandex"
}

func (e *YandexEngine) DisplayName() string {
	return "Yandex"
}

func (e *YandexEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryImages}
}

func (e *YandexEngine) DefaultOn() bool {
	return true
}

func (e *YandexEngine) Weight() float64 {
	return 1.1
}

func (e *YandexEngine) About() string {
	return "Independent global search engine with strong non-Latin language processing."
}

func (e *YandexEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	searchURL := fmt.Sprintf("https://yandex.com/search/?text=%s&p=%d", url.QueryEscape(req.Query), page-1)

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
		return nil, fmt.Errorf("yandex returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	items := FindNodes(doc, "li", "serp-item")
	if len(items) == 0 {
		items = FindNodes(doc, "div", "serp-item")
	}

	for _, node := range items {
		linkNode := FindFirstNode(node, "a", "organic__url")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "yandex.com") || strings.Contains(href, "yandex.ru") {
			continue
		}

		titleNode := FindFirstNode(node, "h2", "")
		if titleNode == nil {
			titleNode = linkNode
		}

		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "div", "organic__text")
		if descNode == nil {
			descNode = FindFirstNode(node, "span", "organic__text")
		}
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
