package plugins

import (
	"net/url"
	"strings"

	"searxgo/internal/models"
)

// trackerParamMap defines well-known marketing and telemetry tracking parameters
// identical to SearXNG tracker_url_remover and ClearURLs rules.
var trackerParamMap = map[string]bool{
	// Google Analytics & AdWords
	"utm_source":        true,
	"utm_medium":        true,
	"utm_campaign":      true,
	"utm_term":          true,
	"utm_content":       true,
	"utm_id":            true,
	"utm_name":          true,
	"utm_reader":        true,
	"utm_viz_id":        true,
	"utm_pubreferrer":   true,
	"utm_swu":           true,
	"gclid":             true,
	"gclsrc":            true,
	"dclid":             true,
	"wbraid":            true,
	"gbraid":            true,
	// Facebook & Instagram
	"fbclid":            true,
	"igshid":            true,
	// Microsoft / Bing
	"msclkid":           true,
	// Twitter / X & TikTok
	"twclid":            true,
	"ttclid":            true,
	// Yandex
	"yclid":             true,
	"ym_debug":          true,
	// MailChimp
	"mc_cid":            true,
	"mc_eid":            true,
	// HubSpot
	"_hsenc":            true,
	"_hsmi":             true,
	// General Referrers & Trackers
	"ref":               true,
	"ref_src":           true,
	"ref_url":           true,
	"referrer":          true,
	// Alibaba & AliExpress
	"spm":               true,
	"scm":               true,
	// Piwik / Matomo
	"pk_campaign":       true,
	"pk_kwd":            true,
	"piwik_campaign":    true,
	"piwik_kwd":         true,
	"matomo_campaign":   true,
	"matomo_kwd":        true,
	// Vero & Marketo
	"vero_id":           true,
	"vero_conv":         true,
	"mkt_tok":           true,
	// Affiliates & Ads
	"zanpid":            true,
	"aff_platform":      true,
	"aff_trace_key":     true,
	// Spotify & YouTube
	"si":                true,
	"feature":           true,
	// OpenStat
	"_openstat":         true,
	// Omniture / Adobe
	"s_kwcid":           true,
}

// CleanTrackerParameters removes tracker parameters from a raw URL string
func CleanTrackerParameters(rawURL string) string {
	if rawURL == "" || !strings.Contains(rawURL, "?") {
		return rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	queryParams := u.Query()
	if len(queryParams) == 0 {
		return rawURL
	}

	modified := false
	for param := range queryParams {
		lowerParam := strings.ToLower(param)
		if trackerParamMap[lowerParam] || strings.HasPrefix(lowerParam, "utm_") {
			queryParams.Del(param)
			modified = true
		}
	}

	if !modified {
		return rawURL
	}

	encodedQuery := queryParams.Encode()
	u.RawQuery = encodedQuery
	return u.String()
}

// StripURLTrackers cleans tracking parameters from all search results
func StripURLTrackers(results []models.SearchResult) []models.SearchResult {
	for i := range results {
		cleaned := CleanTrackerParameters(results[i].URL)
		if cleaned != results[i].URL {
			results[i].URL = cleaned
			// Update pretty url if needed
			if u, err := url.Parse(cleaned); err == nil {
				path := u.Path
				if len(path) > 1 && path[len(path)-1] == '/' {
					path = path[:len(path)-1]
				}
				results[i].PrettyURL = u.Host + path
			}
		}
	}
	return results
}
