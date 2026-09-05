package aggregator

import (
	"sort"
	"strings"

	"searxgo/internal/models"
)

// CalculateInitialScores assigns rank-based positions and reciprocal scores
func CalculateInitialScores(results []models.SearchResult, engineWeight float64) []models.SearchResult {
	n := len(results)
	if n == 0 {
		return results
	}

	for i := range results {
		pos := i + 1
		results[i].Positions = []int{pos}
		// Base reciprocal rank score: (weight * 100) / pos
		results[i].Score = (engineWeight * 100.0) / float64(pos)
	}

	return results
}

// CalculateSearXNGScore replicates SearXNG's calculate_score(result, priority)
func CalculateSearXNGScore(result *models.SearchResult, query string) float64 {
	if len(result.Positions) == 0 {
		return 0
	}

	// 1. Engine Multiplicity: Multi-engine intersection multiplier (SearXNG spec)
	engineMultiplier := float64(len(result.Engines))
	weight := 1.0 * float64(len(result.Positions)) * engineMultiplier

	// 2. Sum position reciprocal scores: sum(weight / position)
	score := 0.0
	for _, pos := range result.Positions {
		if pos <= 0 {
			pos = 1
		}
		score += (weight * 15.0) / float64(pos)
	}

	// 3. Relevance bonus based on query terms matching Title, URL, and Content
	queryLower := strings.ToLower(strings.TrimSpace(query))
	titleLower := strings.ToLower(result.Title)
	urlLower := strings.ToLower(result.URL)
	contentLower := strings.ToLower(result.Content)

	if queryLower != "" {
		// Full query exact match in title: massive boost
		if strings.Contains(titleLower, queryLower) {
			score += 25.0
		}
		// Full query exact match in URL / Domain
		if strings.Contains(urlLower, queryLower) {
			score += 15.0
		}

		terms := strings.Fields(queryLower)
		allTermsInTitle := true
		for _, term := range terms {
			if strings.Contains(titleLower, term) {
				score += 5.0
			} else {
				allTermsInTitle = false
			}
			if strings.Contains(urlLower, term) {
				score += 3.0
			}
			if strings.Contains(contentLower, term) {
				score += 1.5
			}
		}
		// Bonus if all words of query are present in title
		if len(terms) > 1 && allTermsInTitle {
			score += 10.0
		}
	}

	return score
}

// RankAndSort sorts results descending by SearXNG score
func RankAndSort(results []models.SearchResult, query string) []models.SearchResult {
	for i := range results {
		results[i].Score = CalculateSearXNGScore(&results[i], query)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

