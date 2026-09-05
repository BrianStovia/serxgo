package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"searxgo/internal/models"
)

type WikipediaEngine struct {
	client *http.Client
}

func NewWikipediaEngine() *WikipediaEngine {
	return &WikipediaEngine{
		client: NewHTTPClient(3 * time.Second),
	}
}

func (e *WikipediaEngine) Name() string {
	return "wikipedia"
}

func (e *WikipediaEngine) DisplayName() string {
	return "Wikipedia"
}

func (e *WikipediaEngine) Categories() []models.Category {
	return []models.Category{models.CategoryGeneral, models.CategoryScience}
}

func (e *WikipediaEngine) DefaultOn() bool {
	return true
}

func (e *WikipediaEngine) Weight() float64 {
	return 1.4
}

func (e *WikipediaEngine) About() string {
	return "Free online encyclopedia providing factual information and knowledge cards."
}

func (e *WikipediaEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	lang := "en"
	if req.Language != "" {
		lang = req.Language
	}

	apiURL := fmt.Sprintf("https://%s.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&utf8=1&format=json&srlimit=10",
		lang, url.QueryEscape(req.Query))
	if req.Page > 1 {
		apiURL += fmt.Sprintf("&sroffset=%d", (req.Page-1)*10)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "SearXGo/1.0 (Privacy Metasearch Engine)")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Query struct {
			Search []struct {
				Title     string `json:"title"`
				Snippet   string `json:"snippet"`
				PageID    int    `json:"pageid"`
				Timestamp string `json:"timestamp"`
			} `json:"search"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Query.Search {
		pageURL := fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, url.PathEscape(item.Title))
		snippet := CleanHTMLText(item.Snippet)

		results = append(results, models.SearchResult{
			Title:     item.Title + " - Wikipedia",
			URL:       pageURL,
			PrettyURL: fmt.Sprintf("%s.wikipedia.org/wiki/%s", lang, item.Title),
			Content:   snippet,
			Engine:    e.Name(),
			Category:  models.CategoryGeneral,
		})
	}

	return results, nil
}
