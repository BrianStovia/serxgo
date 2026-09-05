package engine

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"searxgo/internal/models"
)

type ArxivEngine struct {
	client *http.Client
}

func NewArxivEngine() *ArxivEngine {
	return &ArxivEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *ArxivEngine) Name() string {
	return "arxiv"
}

func (e *ArxivEngine) DisplayName() string {
	return "arXiv"
}

func (e *ArxivEngine) Categories() []models.Category {
	return []models.Category{models.CategoryScience, models.CategoryGeneral}
}

func (e *ArxivEngine) DefaultOn() bool {
	return true
}

func (e *ArxivEngine) Weight() float64 {
	return 1.2
}

func (e *ArxivEngine) About() string {
	return "Open-access archive for scholarly articles in physics, mathematics, CS, and quantitative biology."
}

type arxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
}

func (e *ArxivEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	start := 0
	if req.Page > 1 {
		start = (req.Page - 1) * 10
	}
	apiURL := fmt.Sprintf("https://export.arxiv.org/api/query?search_query=all:%s&start=%d&max_results=10&sortBy=relevance",
		url.QueryEscape(req.Query), start)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv api returned status %d", resp.StatusCode)
	}

	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, entry := range feed.Entries {
		title := CleanHTMLText(entry.Title)
		summary := CleanHTMLText(entry.Summary)
		if len(summary) > 250 {
			summary = summary[:250] + "..."
		}

		var authors []string
		for _, a := range entry.Authors {
			authors = append(authors, a.Name)
		}
		authorStr := strings.Join(authors, ", ")

		extra := make(map[string]string)
		if authorStr != "" {
			extra["Authors"] = authorStr
		}

		var pubDate *time.Time
		if t, err := time.Parse(time.RFC3339, entry.Published); err == nil {
			pubDate = &t
		}

		cleanID := strings.TrimSpace(entry.ID)
		pretty := strings.Replace(cleanID, "http://", "", 1)
		pretty = strings.Replace(pretty, "https://", "", 1)

		results = append(results, models.SearchResult{
			Title:         title,
			URL:           cleanID,
			PrettyURL:     pretty,
			Content:       summary,
			Engine:        e.Name(),
			Category:      models.CategoryScience,
			PublishedDate: pubDate,
			Author:        authorStr,
			Extra:         extra,
		})
	}

	return results, nil
}
