package aggregator

import (
	"net/url"
	"sort"
	"strings"
	"unicode"

	"searxgo/internal/engine"
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

// CalculateSearXNGScore calculates high-precision relevance score incorporating:
// 1. Multi-engine consensus corroboration
// 2. Exact phrase and word-boundary title matching
// 3. Navigational domain matching
// 4. Term proximity and density in snippet
// 5. Regional language & TLD (.id) localization
// 6. Low-quality noise penalties
func CalculateSearXNGScore(result *models.SearchResult, query string) float64 {
	if len(result.Positions) == 0 {
		return 0
	}

	// 1. Engine Multiplicity & Consensus Multiplier
	engineCount := float64(len(result.Engines))
	if engineCount < 1.0 {
		engineCount = 1.0
	}
	// Multi-engine corroboration bonus (exponential signal for consensus)
	consensusMultiplier := 1.0 + (engineCount-1.0)*0.45
	weight := 1.0 * float64(len(result.Positions)) * consensusMultiplier

	// 2. Sum position reciprocal scores: sum(weight / position)
	score := 0.0
	for _, pos := range result.Positions {
		if pos <= 0 {
			pos = 1
		}
		score += (weight * 20.0) / float64(pos)
	}

	// 3. Relevance bonus based on query terms matching Title, URL, and Content
	queryClean := strings.TrimSpace(query)
	queryLower := strings.ToLower(queryClean)
	titleLower := strings.ToLower(result.Title)
	urlLower := strings.ToLower(result.URL)
	contentLower := strings.ToLower(result.Content)

	if queryLower != "" {
		terms := strings.Fields(queryLower)

		// 3.1 Exact Full Query Match in Title
		if strings.Contains(titleLower, queryLower) {
			score += 22.0
			// Extra boost if title begins with query
			if strings.HasPrefix(strings.TrimSpace(titleLower), queryLower) {
				score += 8.0
			}
		}

		// 3.2 Exact Full Query Match in Host / Domain (Navigational boost)
		if u, err := url.Parse(result.URL); err == nil {
			hostLower := strings.ToLower(u.Host)
			hostLower = strings.TrimPrefix(hostLower, "www.")
			// Query matches domain name (e.g. "golang" -> "golang.org" or "go.dev")
			if strings.Contains(hostLower, queryLower) || strings.Contains(queryLower, hostLower) {
				score += 25.0
			}
			// Educational / Government / Official domain boost
			if strings.HasSuffix(hostLower, ".edu") || strings.HasSuffix(hostLower, ".gov") || strings.HasSuffix(hostLower, ".go.id") || strings.HasSuffix(hostLower, ".ac.id") {
				score += 15.0
			}
		}

		// 3.3 Exact Word Boundary Matching (Title, URL, Content)
		allTermsInTitle := true
		termMatchesInTitle := 0
		termMatchesInContent := 0

		for _, term := range terms {
			if len(term) == 0 {
				continue
			}

			if containsWordBoundary(titleLower, term) {
				score += 7.0
				termMatchesInTitle++
			} else if strings.Contains(titleLower, term) {
				score += 3.5
				termMatchesInTitle++
			} else {
				allTermsInTitle = false
			}

			if containsWordBoundary(urlLower, term) || strings.Contains(urlLower, term) {
				score += 3.0
			}

			// Rich snippet matches (strong relevance signal)
			if containsWordBoundary(contentLower, term) {
				score += 5.5
				termMatchesInContent++
			} else if strings.Contains(contentLower, term) {
				score += 2.5
				termMatchesInContent++
			}
		}

		// 3.4 All Terms Present Bonus
		if len(terms) > 1 && allTermsInTitle {
			score += 15.0
		}
		if len(terms) > 1 && termMatchesInContent >= len(terms) {
			score += 18.0 // High quality snippet answering the query
		}

		// 3.5 Regional Indonesian Language & Domain Match Boost
		if engine.IsIndonesianText(queryLower) {
			if u, err := url.Parse(result.URL); err == nil {
				h := strings.ToLower(u.Host)
				if strings.HasSuffix(h, ".id") {
					score += 35.0 // Top priority for authentic Indonesian sites (.id, .co.id, etc.)
				}
			}
			if engine.IsIndonesianText(result.Title) || engine.IsIndonesianText(result.Content) {
				score += 25.0 // Authentic Indonesian language content
			} else if len(result.Content) > 25 && !engine.IsIndonesianText(result.Content) {
				score -= 15.0 // Discourage completely foreign articles when query is Indonesian
			}
		}

		// 3.6 Penalty for low-quality / empty results
		if strings.TrimSpace(result.Title) == "" || strings.EqualFold(result.Title, "Untitled") {
			score -= 50.0
		}
		if len(result.Content) < 15 {
			score -= 10.0
		}
	}

	return score
}

// containsWordBoundary checks if a term exists as an isolated token or surrounded by non-alphanumeric chars
func containsWordBoundary(text, term string) bool {
	idx := strings.Index(text, term)
	for idx != -1 {
		startOk := (idx == 0) || !isAlphaNum(rune(text[idx-1]))
		endIdx := idx + len(term)
		endOk := (endIdx == len(text)) || !isAlphaNum(rune(text[endIdx]))

		if startOk && endOk {
			return true
		}

		next := strings.Index(text[idx+1:], term)
		if next == -1 {
			break
		}
		idx = idx + 1 + next
	}
	return false
}

func isAlphaNum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
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
