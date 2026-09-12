package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"searxgo/internal/bangs"
)

type SuggestService struct {
	client *http.Client
}

func NewSuggestService() *SuggestService {
	return &SuggestService{
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

// GetSuggestions queries default autocomplete providers ("all")
func (s *SuggestService) GetSuggestions(ctx context.Context, query string) []string {
	return s.GetSuggestionsWithBackend(ctx, query, "all")
}

// GetSuggestionsWithBackend queries a specific provider or combines multiple providers
func (s *SuggestService) GetSuggestionsWithBackend(ctx context.Context, query string, backend string) []string {
	q := strings.TrimSpace(query)
	if len(q) < 1 {
		return []string{}
	}

	// 1. Bang Autocompletion: !gh, !yt, !images, !mar, etc.
	if strings.HasPrefix(q, "!") || strings.HasPrefix(q, "-!") || strings.HasPrefix(q, "!~") {
		return bangs.SuggestBangs(q, 8)
	}

	// 2. Language Modifier Autocompletion: :en, :id, :de, :all
	if strings.HasPrefix(q, ":") {
		langs := []string{":en (English)", ":id (Indonesian)", ":de (German)", ":fr (French)", ":es (Spanish)", ":ja (Japanese)", ":zh (Chinese)", ":all (All Languages)"}
		var matched []string
		for _, l := range langs {
			if strings.HasPrefix(l, q) {
				matched = append(matched, l)
			}
		}
		return matched
	}

	if len(q) < 2 {
		return []string{}
	}

	b := strings.ToLower(strings.TrimSpace(backend))
	if b == "off" || b == "none" {
		return []string{}
	}

	switch b {
	case "duckduckgo", "ddg":
		return s.fetchDuckDuckGo(ctx, q)
	case "google":
		return s.fetchGoogle(ctx, q)
	case "brave":
		return s.fetchBrave(ctx, q)
	case "bing":
		return s.fetchBing(ctx, q)
	case "wikipedia":
		return s.fetchWikipedia(ctx, q)
	case "startpage":
		return s.fetchStartpage(ctx, q)
	case "qwant":
		return s.fetchQwant(ctx, q)
	case "all", "":
		// Multi-provider aggregate: query top engines concurrently
		return s.fetchMultiAggregated(ctx, q)
	default:
		return s.fetchDuckDuckGo(ctx, q)
	}
}

func (s *SuggestService) fetchOpenSearchURL(ctx context.Context, apiURL string, userAgent string) []string {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	}

	resp, err := s.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	// Standard OpenSearch / JSON format: [query, [sug1, sug2, ...]]
	var raw []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil && len(raw) >= 2 {
		if list, ok := raw[1].([]interface{}); ok {
			var items []string
			for _, it := range list {
				if str, ok := it.(string); ok {
					items = append(items, str)
				}
			}
			return items
		}
	}
	return nil
}

func (s *SuggestService) fetchDuckDuckGo(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://duckduckgo.com/ac/?q=%s&type=list", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "")
}

func (s *SuggestService) fetchGoogle(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://suggestqueries.google.com/complete/search?client=firefox&q=%s", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "")
}

func (s *SuggestService) fetchBrave(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://search.brave.com/api/suggest?q=%s", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "")
}

func (s *SuggestService) fetchBing(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://api.bing.com/osjson.aspx?query=%s", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "")
}

func (s *SuggestService) fetchWikipedia(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=opensearch&search=%s&limit=6&namespace=0&format=json", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "SearXGo/1.0")
}

func (s *SuggestService) fetchStartpage(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://www.startpage.com/osuggestions?q=%s", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "")
}

func (s *SuggestService) fetchQwant(ctx context.Context, q string) []string {
	apiURL := fmt.Sprintf("https://api.qwant.com/v3/suggest?q=%s&client=opensearch", url.QueryEscape(q))
	return s.fetchOpenSearchURL(ctx, apiURL, "")
}

func (s *SuggestService) fetchMultiAggregated(ctx context.Context, q string) []string {
	providers := []func(context.Context, string) []string{
		s.fetchGoogle,
		s.fetchBrave,
		s.fetchDuckDuckGo,
		s.fetchWikipedia,
	}

	resultsChan := make(chan []string, len(providers))
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(fn func(context.Context, string) []string) {
			defer wg.Done()
			resultsChan <- fn(ctx, q)
		}(p)
	}

	wg.Wait()
	close(resultsChan)

	type suggestionStat struct {
		original string
		count    int
		score    float64
	}

	statsMap := make(map[string]*suggestionStat)
	qLower := strings.ToLower(strings.TrimSpace(q))

	for items := range resultsChan {
		for rank, it := range items {
			trimmed := strings.TrimSpace(it)
			lower := strings.ToLower(trimmed)
			if lower == "" {
				continue
			}

			if _, exists := statsMap[lower]; !exists {
				statsMap[lower] = &suggestionStat{original: trimmed, count: 0, score: 0}
			}

			stat := statsMap[lower]
			stat.count++
			// Higher rank in provider gives higher base score
			stat.score += float64(10 - rank)

			// Exact prefix match bonus
			if strings.HasPrefix(lower, qLower) {
				stat.score += 15.0
			}
		}
	}

	var candidates []*suggestionStat
	for _, st := range statsMap {
		candidates = append(candidates, st)
	}

	// Sort descending by relevance score & multi-engine consensus
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	var finalSuggestions []string
	for _, c := range candidates {
		finalSuggestions = append(finalSuggestions, c.original)
		if len(finalSuggestions) >= 8 {
			break
		}
	}

	return finalSuggestions
}
