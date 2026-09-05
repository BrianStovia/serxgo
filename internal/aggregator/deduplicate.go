package aggregator

import (
	"net/url"
	"strings"

	"searxgo/internal/models"
)

var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"fbclid":       true,
	"gclid":        true,
	"msclkid":      true,
	"mc_eid":       true,
	"ref":          true,
	"ref_src":      true,
	"source":       true,
	"_ga":          true,
	"_gl":          true,
	"igshid":       true,
	"si":           true,
	"feature":      true,
	"ved":          true,
	"usg":          true,
	"ncid":         true,
}

// CleanAndNormalizeURL removes tracking parameters and standardizes the URL structure
func CleanAndNormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	// Normalize scheme and host to lowercase
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove default ports
	if u.Scheme == "http" && strings.HasSuffix(u.Host, ":80") {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	if u.Scheme == "https" && strings.HasSuffix(u.Host, ":443") {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}

	// Strip www. prefix for consistent comparison
	u.Host = strings.TrimPrefix(u.Host, "www.")

	// Filter query parameters
	if u.RawQuery != "" {
		q := u.Query()
		for key := range q {
			if trackingParams[strings.ToLower(key)] {
				q.Del(key)
			}
		}
		u.RawQuery = q.Encode()
	}

	// Remove trailing slash for non-root paths
	if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	// Remove fragment/hash
	u.Fragment = ""

	return u.String()
}

// Deduplicate merges results with identical normalized URLs
func Deduplicate(results []models.SearchResult) []models.SearchResult {
	seen := make(map[string]int) // normalized URL -> index in unique
	var unique []models.SearchResult

	for _, item := range results {
		normURL := CleanAndNormalizeURL(item.URL)
		if normURL == "" {
			continue
		}

		if idx, exists := seen[normURL]; exists {
			// Result already exists: Merge engines and positions (SearXNG ResultContainer spec)
			existing := &unique[idx]

			// Add engine if not yet listed
			foundEngine := false
			for _, e := range existing.Engines {
				if strings.EqualFold(e, item.Engine) {
					foundEngine = true
					break
				}
			}
			if !foundEngine {
				existing.Engines = append(existing.Engines, item.Engine)
			}

			// Append positions
			if len(item.Positions) > 0 {
				existing.Positions = append(existing.Positions, item.Positions...)
			}

			// Keep richer content snippet
			if len(item.Content) > len(existing.Content) {
				existing.Content = item.Content
			}

			// Keep thumbnail if existing is missing
			if existing.Thumbnail == "" && item.Thumbnail != "" {
				existing.Thumbnail = item.Thumbnail
			}

			// Keep ImageURL if existing is missing
			if existing.ImageURL == "" && item.ImageURL != "" {
				existing.ImageURL = item.ImageURL
			}

			// Keep VideoURL if existing is missing
			if existing.VideoURL == "" && item.VideoURL != "" {
				existing.VideoURL = item.VideoURL
			}

			// Keep MagnetURL if existing is missing
			if existing.MagnetURL == "" && item.MagnetURL != "" {
				existing.MagnetURL = item.MagnetURL
			}
		} else {
			if len(item.Engines) == 0 {
				item.Engines = []string{item.Engine}
			}
			if len(item.Positions) == 0 {
				item.Positions = []int{1}
			}
			seen[normURL] = len(unique)
			unique = append(unique, item)
		}
	}

	return unique
}
