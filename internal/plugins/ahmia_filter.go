package plugins

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"strings"

	"searxgo/internal/models"
)

// ahmiaBlacklistHashes contains MD5 hashes of known banned/illegal onion addresses
// according to Ahmia's public safety guidelines (https://ahmia.fi/blacklist).
var ahmiaBlacklistHashes = map[string]bool{
	"d41d8cd98f00b204e9800998ecf8427e": true, // placeholder hash
}

// ahmiaBannedKeywords contains safety keyword filters for darknet/onion queries
var ahmiaBannedKeywords = []string{
	"child abuse",
	"pedophilia",
	"cp onion",
}

// ApplyAhmiaFilter removes results that violate safety guidelines or appear on Ahmia's blacklist
func ApplyAhmiaFilter(results []models.SearchResult) []models.SearchResult {
	var filtered []models.SearchResult

	for _, item := range results {
		if strings.Contains(item.URL, ".onion") {
			u, err := url.Parse(item.URL)
			if err == nil {
				host := strings.ToLower(u.Host)
				h := md5.Sum([]byte(host))
				hashStr := hex.EncodeToString(h[:])
				if ahmiaBlacklistHashes[hashStr] {
					continue
				}
			}
		}

		// Content safety keyword check
		lowerTitle := strings.ToLower(item.Title)
		lowerContent := strings.ToLower(item.Content)
		isBanned := false
		for _, kw := range ahmiaBannedKeywords {
			if strings.Contains(lowerTitle, kw) || strings.Contains(lowerContent, kw) {
				isBanned = true
				break
			}
		}
		if isBanned {
			continue
		}

		filtered = append(filtered, item)
	}

	return filtered
}
