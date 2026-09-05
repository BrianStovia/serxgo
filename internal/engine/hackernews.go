package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"searxgo/internal/models"
)

type HackerNewsEngine struct {
	client *http.Client
}

func NewHackerNewsEngine() *HackerNewsEngine {
	return &HackerNewsEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *HackerNewsEngine) Name() string {
	return "hackernews"
}

func (e *HackerNewsEngine) DisplayName() string {
	return "Hacker News"
}

func (e *HackerNewsEngine) Categories() []models.Category {
	return []models.Category{models.CategoryIT, models.CategoryGeneral, models.CategoryNews}
}

func (e *HackerNewsEngine) DefaultOn() bool {
	return true
}

func (e *HackerNewsEngine) Weight() float64 {
	return 1.1
}

func (e *HackerNewsEngine) About() string {
	return "Social news website focusing on computer science, technology, and entrepreneurship."
}

func (e *HackerNewsEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s&tags=story&hitsPerPage=10",
		url.QueryEscape(req.Query))
	if req.Page > 1 {
		apiURL += fmt.Sprintf("&page=%d", req.Page-1)
	}

	if req.TimeRange != "" {
		now := time.Now().Unix()
		var since int64
		switch req.TimeRange {
		case "day":
			since = now - 86400
		case "week":
			since = now - 604800
		case "month":
			since = now - 2592000
		case "year":
			since = now - 31536000
		}
		if since > 0 {
			apiURL += fmt.Sprintf("&numericFilters=created_at_i>%d", since)
		}
	}

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
		return nil, fmt.Errorf("hackernews api returned status %d", resp.StatusCode)
	}

	var data struct {
		Hits []struct {
			ObjectID    string    `json:"objectID"`
			Title       string    `json:"title"`
			URL         string    `json:"url"`
			Author      string    `json:"author"`
			Points      int       `json:"points"`
			NumComments int       `json:"num_comments"`
			CreatedAt   time.Time `json:"created_at"`
			StoryText   string    `json:"story_text"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, hit := range data.Hits {
		if hit.Title == "" {
			continue
		}

		targetURL := hit.URL
		if targetURL == "" {
			targetURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		}

		content := hit.StoryText
		if content == "" {
			content = fmt.Sprintf("Posted by %s with %d points and %d comments.", hit.Author, hit.Points, hit.NumComments)
		} else {
			content = CleanHTMLText(content)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
		}

		extra := map[string]string{
			"Points":   strconv.Itoa(hit.Points),
			"Comments": strconv.Itoa(hit.NumComments),
			"HN_Item":  fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID),
		}

		parsedURL, err := url.Parse(targetURL)
		pretty := targetURL
		if err == nil {
			pretty = parsedURL.Host + parsedURL.Path
		}

		results = append(results, models.SearchResult{
			Title:         hit.Title,
			URL:           targetURL,
			PrettyURL:     pretty,
			Content:       content,
			Engine:        e.Name(),
			Category:      models.CategoryIT,
			PublishedDate: &hit.CreatedAt,
			Author:        hit.Author,
			Extra:         extra,
		})
	}

	return results, nil
}
