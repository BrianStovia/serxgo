package plugins

import (
	"fmt"
	"regexp"
	"strings"

	"searxgo/internal/models"
)

var doiRegex = regexp.MustCompile(`(?i)\b(10\.\d{4,9}/[-._;()/:A-Za-z0-9]+)\b`)

// ApplyDOIResolver inspects search results and injects Open Access DOI resolver links
func ApplyDOIResolver(results []models.SearchResult, preferredResolver string) []models.SearchResult {
	if preferredResolver == "" {
		preferredResolver = "oadoi.org"
	}

	for i := range results {
		doi := extractDOI(results[i].URL)
		if doi == "" {
			doi = extractDOI(results[i].Content)
		}

		if doi != "" {
			if results[i].Extra == nil {
				results[i].Extra = make(map[string]string)
			}
			results[i].Extra["DOI"] = doi

			resolverURL := buildResolverURL(doi, preferredResolver)
			results[i].Extra["Open Access"] = resolverURL
		}
	}
	return results
}

// CheckDOIQuery returns an instant answer card if the search query itself is a DOI
func CheckDOIQuery(query string, preferredResolver string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	doi := extractDOI(q)
	if doi == "" {
		return nil
	}

	if preferredResolver == "" {
		preferredResolver = "oadoi.org"
	}

	resolverURL := buildResolverURL(doi, preferredResolver)

	return &models.InstantAnswer{
		Type:        "infobox",
		Title:       fmt.Sprintf("Digital Object Identifier (DOI): %s", doi),
		Value:       "Scientific Publication / Research Article",
		Description: fmt.Sprintf("Access this research article directly via Open Access resolver (%s).", preferredResolver),
		URL:         resolverURL,
		Attributes: map[string]string{
			"DOI":         doi,
			"Resolver":    preferredResolver,
			"Open Access": resolverURL,
		},
	}
}

func extractDOI(text string) string {
	match := doiRegex.FindStringSubmatch(text)
	if len(match) > 1 {
		return strings.TrimRight(match[1], ".,;)")
	}
	return ""
}

func buildResolverURL(doi, resolver string) string {
	cleanDOI := strings.TrimSpace(doi)
	switch strings.ToLower(resolver) {
	case "sci-hub.se", "sci-hub", "scihub":
		return "https://sci-hub.se/" + cleanDOI
	case "sci-hub.st":
		return "https://sci-hub.st/" + cleanDOI
	case "sci-hub.ru":
		return "https://sci-hub.ru/" + cleanDOI
	case "unpaywall.org", "unpaywall":
		return "https://unpaywall.org/" + cleanDOI
	case "libgen.is", "libgen":
		return "https://libgen.is/scimag/?q=" + cleanDOI
	default:
		return "https://doi.org/" + cleanDOI
	}
}

