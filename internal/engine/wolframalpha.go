package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"searxgo/internal/models"
)

type WolframAlphaEngine struct {
	client *http.Client
}

func NewWolframAlphaEngine() *WolframAlphaEngine {
	return &WolframAlphaEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *WolframAlphaEngine) Name() string {
	return "wolframalpha"
}

func (e *WolframAlphaEngine) DisplayName() string {
	return "Wolfram|Alpha"
}

func (e *WolframAlphaEngine) Categories() []models.Category {
	return []models.Category{models.CategoryScience, models.CategoryGeneral}
}

func (e *WolframAlphaEngine) DefaultOn() bool {
	return true
}

func (e *WolframAlphaEngine) Weight() float64 {
	return 1.4
}

func (e *WolframAlphaEngine) About() string {
	return "Computational intelligence engine providing expert-level knowledge and instant mathematical calculation."
}

func (e *WolframAlphaEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	escapedQuery := url.QueryEscape(req.Query)
	apiURL := fmt.Sprintf("https://api.wolframalpha.com/v1/result?i=%s&appid=DEMO", escapedQuery)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Accept", "text/plain")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []models.SearchResult
	directURL := fmt.Sprintf("https://www.wolframalpha.com/input?i=%s", escapedQuery)

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			answer := strings.TrimSpace(string(bodyBytes))
			if answer != "" && !strings.Contains(strings.ToLower(answer), "no short answer") {
				results = append(results, models.SearchResult{
					Title:     fmt.Sprintf("Wolfram|Alpha: %s", req.Query),
					URL:       directURL,
					PrettyURL: fmt.Sprintf("wolframalpha.com/input?i=%s", req.Query),
					Content:   answer,
					Engine:    e.Name(),
					Category:  models.CategoryScience,
					Score:     1.5,
				})
				return results, nil
			}
		}
	}

	// Secondary check: If calculation or scientific query, provide computational entry link
	results = append(results, models.SearchResult{
		Title:     fmt.Sprintf("Wolfram|Alpha Computation for '%s'", req.Query),
		URL:       directURL,
		PrettyURL: fmt.Sprintf("wolframalpha.com/input?i=%s", req.Query),
		Content:   fmt.Sprintf("Explore expert calculation, algorithmic steps, data analysis, and visualizations for '%s' on Wolfram|Alpha.", req.Query),
		Engine:    e.Name(),
		Category:  models.CategoryScience,
		Score:     1.2,
	})

	return results, nil
}
