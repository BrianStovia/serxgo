package plugins

import (
	"searxgo/internal/models"
)

// ProcessSearchResultPlugins runs all active SearXNG post-search plugins on aggregated results
func ProcessSearchResultPlugins(results []models.SearchResult, enableRedirects bool, doiResolver string, removeTrackers bool) []models.SearchResult {
	// 1. Tracker URL Remover (SearXNG tracker_url_remover plugin)
	if removeTrackers {
		results = StripURLTrackers(results)
	}

	// 2. Ahmia Onion Content Safety Filter
	results = ApplyAhmiaFilter(results)

	// 3. Privacy Frontend Rewriters (Invidious, Nitter, Libreddit, Scribe, Rimgo)
	if enableRedirects {
		results = ApplyFrontendRedirects(results)
	} else {
		for i := range results {
			if results[i].CachedURL == "" {
				results[i].CachedURL = "https://web.archive.org/web/2/" + results[i].URL
			}
		}
	}

	// 4. Open Access DOI Resolver
	results = ApplyDOIResolver(results, doiResolver)

	return results
}
