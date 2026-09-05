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

type GoogleEngine struct {
	client *http.Client
}

func NewGoogleEngine() *GoogleEngine {
	return &GoogleEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *GoogleEngine) Name() string {
	return "google"
}

func (e *GoogleEngine) DisplayName() string {
	return "Google"
}

func (e *GoogleEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryNews}
}

func (e *GoogleEngine) DefaultOn() bool {
	return true
}

func (e *GoogleEngine) Weight() float64 {
	return 1.3
}

func (e *GoogleEngine) About() string {
	return "World's most widely used web search engine."
}

func (e *GoogleEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	hl := "en"
	if req.Language != "" {
		hl = req.Language
	}

	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&hl=%s&num=15&gbv=1",
		url.QueryEscape(req.Query), hl)
	if req.Page > 1 {
		searchURL += fmt.Sprintf("&start=%d", (req.Page-1)*10)
	}

	if req.TimeRange != "" {
		switch req.TimeRange {
		case "day":
			searchURL += "&tbs=qdr:d"
		case "week":
			searchURL += "&tbs=qdr:w"
		case "month":
			searchURL += "&tbs=qdr:m"
		case "year":
			searchURL += "&tbs=qdr:y"
		}
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
		return nil, fmt.Errorf("google returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult

	// Google gbv=1 simple HTML structure has divs with class 'g' or 'Gx5Zad'
	blocks := FindNodes(doc, "div", "Gx5Zad")
	if len(blocks) == 0 {
		blocks = FindNodes(doc, "div", "g")
	}

	for _, block := range blocks {
		linkNode := FindFirstNode(block, "a", "")
		if linkNode == nil {
			continue
		}

		rawHref := GetAttr(linkNode, "href")
		cleanHref := extractGoogleURL(rawHref)
		if cleanHref == "" || strings.HasPrefix(cleanHref, "/") || strings.Contains(cleanHref, "google.com") {
			continue
		}

		titleNode := FindFirstNode(block, "h3", "")
		if titleNode == nil {
			titleNode = FindFirstNode(block, "div", "BNeawe")
		}
		if titleNode == nil {
			titleNode = linkNode
		}

		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		// Content snippet
		snippetNode := FindFirstNode(block, "div", "BNeawe s3v9rd AP7Wnd")
		if snippetNode == nil {
			snippetNode = FindFirstNode(block, "div", "VwiC3b")
		}
		content := ""
		if snippetNode != nil {
			content = CleanHTMLText(ExtractText(snippetNode))
		}

		parsed, err := url.Parse(cleanHref)
		pretty := cleanHref
		if err == nil {
			pretty = parsed.Host + parsed.Path
		}

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       cleanHref,
			PrettyURL: pretty,
			Content:   content,
			Engine:    e.Name(),
			Category:  models.CategoryGeneral,
		})
	}

	return results, nil
}

func extractGoogleURL(raw string) string {
	if strings.HasPrefix(raw, "/url?q=") {
		u, err := url.Parse(raw)
		if err == nil {
			return u.Query().Get("q")
		}
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return ""
}
