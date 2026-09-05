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

type GitHubEngine struct {
	client *http.Client
}

func NewGitHubEngine() *GitHubEngine {
	return &GitHubEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *GitHubEngine) Name() string {
	return "github"
}

func (e *GitHubEngine) DisplayName() string {
	return "GitHub"
}

func (e *GitHubEngine) Categories() []models.Category {
	return []models.Category{models.CategoryIT}
}

func (e *GitHubEngine) DefaultOn() bool {
	return true
}

func (e *GitHubEngine) Weight() float64 {
	return 1.3
}

func (e *GitHubEngine) About() string {
	return "World's leading platform for open-source code and software repositories."
}

func (e *GitHubEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	page := 1
	if req.Page > 1 {
		page = req.Page
	}
	apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=10&page=%d",
		url.QueryEscape(req.Query), page)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "SearXGo-Metasearch")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var data struct {
		Items []struct {
			FullName        string `json:"full_name"`
			HTMLURL         string `json:"html_url"`
			Description     string `json:"description"`
			StargazersCount int    `json:"stargazers_count"`
			ForksCount      int    `json:"forks_count"`
			Language        string `json:"language"`
			Owner           struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Items {
		desc := item.Description
		if desc == "" {
			desc = "No description provided."
		}

		extra := make(map[string]string)
		if item.Language != "" {
			extra["Language"] = item.Language
		}
		extra["Stars"] = strconv.Itoa(item.StargazersCount)
		extra["Forks"] = strconv.Itoa(item.ForksCount)

		results = append(results, models.SearchResult{
			Title:     item.FullName,
			URL:       item.HTMLURL,
			PrettyURL: fmt.Sprintf("github.com/%s", item.FullName),
			Content:   desc,
			Thumbnail: item.Owner.AvatarURL,
			Engine:    e.Name(),
			Category:  models.CategoryIT,
			Extra:     extra,
		})
	}

	return results, nil
}
