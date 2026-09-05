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
		score += (weight * 10.0) / float64(pos)
	}

	// 3. Exact query in title bonus (SearXNG ranking enhancement)
	queryLower := strings.ToLower(strings.TrimSpace(query))
	titleLower := strings.ToLower(result.Title)

	if queryLower != "" {
		if strings.Contains(titleLower, queryLower) {
			score += 15.0
		}
		for _, term := range strings.Fields(queryLower) {
			if strings.Contains(titleLower, term) {
				score += 2.5
			}
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

