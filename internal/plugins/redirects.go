package plugins

import (
	"fmt"
	"net/url"
	"strings"

	"searxgo/internal/models"
)

// ApplyFrontendRedirects rewrites URLs to privacy-friendly frontends (Invidious, Nitter, Libreddit, Scribe, Rimgo)
func ApplyFrontendRedirects(results []models.SearchResult) []models.SearchResult {
	for i := range results {
		rawURL := results[i].URL
		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}

		host := strings.ToLower(parsed.Host)
		host = strings.TrimPrefix(host, "www.")

		// 1. YouTube -> Invidious
		if host == "youtube.com" || host == "m.youtube.com" || host == "youtu.be" {
			parsed.Host = "yewtu.be"
			results[i].URL = parsed.String()
		}

		// 2. Reddit -> Teddit / Libreddit
		if host == "reddit.com" || host == "old.reddit.com" {
			parsed.Host = "libreddit.kavin.rocks"
			results[i].URL = parsed.String()
		}

		// 3. Twitter / X -> Nitter
		if host == "twitter.com" || host == "x.com" {
			parsed.Host = "nitter.net"
			results[i].URL = parsed.String()
		}

		// 4. Medium -> Scribe
		if strings.HasSuffix(host, "medium.com") {
			parsed.Host = "scribe.rip"
			results[i].URL = parsed.String()
		}

		// 5. Imgur -> Rimgo
		if host == "imgur.com" || host == "i.imgur.com" {
			parsed.Host = "rimgo.totaldarkness.net"
			results[i].URL = parsed.String()
		}

		// 6. Generate Wayback Cached Link
		results[i].CachedURL = fmt.Sprintf("https://web.archive.org/web/2/%s", rawURL)
	}

	return results
}
