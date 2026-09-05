package plugins

import (
	"net/url"
	"strings"

	"searxgo/internal/models"
)

// FilterHostnames removes blocked domains and boosts prioritized domains (SearXNG Hostnames plugin)
func FilterHostnames(results []models.SearchResult, blockedDomains []string, priorityDomains map[string]float64) []models.SearchResult {
	if len(blockedDomains) == 0 && len(priorityDomains) == 0 {
		return results
	}

	blockedMap := make(map[string]bool)
	for _, d := range blockedDomains {
		blockedMap[strings.ToLower(strings.TrimSpace(d))] = true
	}

	var filtered []models.SearchResult
	for i := range results {
		u, err := url.Parse(results[i].URL)
		if err != nil {
			filtered = append(filtered, results[i])
			continue
		}

		host := strings.ToLower(u.Host)
		host = strings.TrimPrefix(host, "www.")

		// Skip blocked domain
		if blockedMap[host] {
			continue
		}

		// Apply priority weight boost
		if boost, ok := priorityDomains[host]; ok {
			results[i].Score *= boost
		}

		filtered = append(filtered, results[i])
	}

	return filtered
}
