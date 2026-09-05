package engine

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/models"
)

type StackOverflowEngine struct {
	client *http.Client
}

func NewStackOverflowEngine() *StackOverflowEngine {
	return &StackOverflowEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *StackOverflowEngine) Name() string {
	return "stackoverflow"
}

func (e *StackOverflowEngine) DisplayName() string {
	return "StackOverflow"
}

func (e *StackOverflowEngine) Categories() []models.Category {
	return []models.Category{models.CategoryIT, models.CategoryGeneral}
}

func (e *StackOverflowEngine) DefaultOn() bool {
	return true
}

func (e *StackOverflowEngine) Weight() float64 {
	return 1.3
}

func (e *StackOverflowEngine) About() string {
	return "Community of developers learning and sharing programming knowledge."
}

func (e *StackOverflowEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := 1
	if req.Page > 1 {
		page = req.Page
	}
	apiURL := fmt.Sprintf("https://api.stackexchange.com/2.3/search/advanced?order=desc&sort=relevance&q=%s&site=stackoverflow&pagesize=10&page=%d",
		url.QueryEscape(req.Query), page)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "SearXGo/1.0")
	httpReq.Header.Set("Accept-Encoding", "gzip")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stackexchange api returned status %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}

	var data struct {
		Items []struct {
			Title        string   `json:"title"`
			Link         string   `json:"link"`
			Score        int      `json:"score"`
			AnswerCount  int      `json:"answer_count"`
			IsAnswered   bool     `json:"is_answered"`
			CreationDate int64    `json:"creation_date"`
			Tags         []string `json:"tags"`
			Owner        struct {
				DisplayName string `json:"display_name"`
			} `json:"owner"`
		} `json:"items"`
	}

	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Items {
		title := CleanHTMLText(item.Title)
		tagsStr := strings.Join(item.Tags, ", ")

		answeredStatus := "Unanswered"
		if item.IsAnswered {
			answeredStatus = "Accepted Answer"
		}

		content := fmt.Sprintf("Score: %d | Answers: %d (%s) | Tags: [%s]",
			item.Score, item.AnswerCount, answeredStatus, tagsStr)

		extra := map[string]string{
			"Score":   strconv.Itoa(item.Score),
			"Answers": strconv.Itoa(item.AnswerCount),
			"Tags":    tagsStr,
		}

		var pubDate *time.Time
		if item.CreationDate > 0 {
			t := time.Unix(item.CreationDate, 0)
			pubDate = &t
		}

		parsed, err := url.Parse(item.Link)
		pretty := item.Link
		if err == nil {
			pretty = parsed.Host + parsed.Path
		}

		results = append(results, models.SearchResult{
			Title:         title,
			URL:           item.Link,
			PrettyURL:     pretty,
			Content:       content,
			Engine:        e.Name(),
			Category:      models.CategoryIT,
			PublishedDate: pubDate,
			Author:        item.Owner.DisplayName,
			Extra:         extra,
		})
	}

	return results, nil
}
