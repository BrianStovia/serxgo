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

type RedditEngine struct {
	client *http.Client
}

func NewRedditEngine() *RedditEngine {
	return &RedditEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *RedditEngine) Name() string {
	return "reddit"
}

func (e *RedditEngine) DisplayName() string {
	return "Reddit"
}

func (e *RedditEngine) Categories() []models.Category {
	return []models.Category{models.CategorySocial, models.CategoryNews}
}

func (e *RedditEngine) DefaultOn() bool {
	return true
}

func (e *RedditEngine) Weight() float64 {
	return 1.0
}

func (e *RedditEngine) About() string {
	return "Online community discussions, subreddits, and user shared content."
}

func (e *RedditEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	timeSort := "all"
	switch req.TimeRange {
	case "day":
		timeSort = "day"
	case "week":
		timeSort = "week"
	case "month":
		timeSort = "month"
	case "year":
		timeSort = "year"
	}

	apiURL := fmt.Sprintf("https://www.reddit.com/search.json?q=%s&limit=10&t=%s&sort=relevance",
		url.QueryEscape(req.Query), timeSort)
	if req.Page > 1 {
		apiURL += fmt.Sprintf("&count=%d", (req.Page-1)*10)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo/1.0")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit api returned status %d", resp.StatusCode)
	}

	var data struct {
		Data struct {
			Children []struct {
				Data struct {
					Title        string  `json:"title"`
					Subreddit    string  `json:"subreddit_name_prefixed"`
					Permalink    string  `json:"permalink"`
					Selftext     string  `json:"selftext"`
					Score        int     `json:"score"`
					NumComments  int     `json:"num_comments"`
					Thumbnail    string  `json:"thumbnail"`
					CreatedUTC   float64 `json:"created_utc"`
					Author       string  `json:"author"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Data.Children {
		d := item.Data
		if d.Title == "" {
			continue
		}

		fullURL := fmt.Sprintf("https://www.reddit.com%s", d.Permalink)
		snippet := d.Selftext
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		if snippet == "" {
			snippet = fmt.Sprintf("Discussion in %s by u/%s with %d comments.", d.Subreddit, d.Author, d.NumComments)
		}

		extra := map[string]string{
			"Subreddit": d.Subreddit,
			"Score":     strconv.Itoa(d.Score),
			"Comments":  strconv.Itoa(d.NumComments),
		}

		var pubDate *time.Time
		if d.CreatedUTC > 0 {
			t := time.Unix(int64(d.CreatedUTC), 0)
			pubDate = &t
		}

		thumb := d.Thumbnail
		if thumb == "default" || thumb == "self" || thumb == "nsfw" || thumb == "" {
			thumb = ""
		}

		results = append(results, models.SearchResult{
			Title:         fmt.Sprintf("[%s] %s", d.Subreddit, d.Title),
			URL:           fullURL,
			PrettyURL:     fmt.Sprintf("reddit.com%s", d.Permalink),
			Content:       snippet,
			Engine:        e.Name(),
			Category:      models.CategorySocial,
			PublishedDate: pubDate,
			Thumbnail:     thumb,
			Author:        d.Author,
			Extra:         extra,
		})
	}

	return results, nil
}
