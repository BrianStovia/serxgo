package aggregator

import (
	"fmt"
	"net/url"
	"strings"

	"searxgo/internal/models"
)

type clusterRule struct {
	Key      string
	Name     string
	Icon     string
	Domains  []string
	Keywords []string
}

var defaultClusterRules = []clusterRule{
	{
		Key:      "docs",
		Name:     "Docs & Specs",
		Icon:     "📚",
		Domains:  []string{"docs.", "developer.", "pkg.go.dev", "readthedocs", "w3schools", "mdn", "devdocs", "wiki"},
		Keywords: []string{"documentation", "reference", "specification", "tutorial", "manual", "guide", "cheatsheet", "handbook", "api"},
	},
	{
		Key:      "code",
		Name:     "Code & Repos",
		Icon:     "💻",
		Domains:  []string{"github.com", "gitlab.com", "codeberg.org", "bitbucket.org", "crates.io", "npmjs.com", "pypi.org", "packagist"},
		Keywords: []string{"repository", "source code", "git", "package", "library", "sdk", "module", "implementation", "release"},
	},
	{
		Key:      "community",
		Name:     "Discussions",
		Icon:     "💬",
		Domains:  []string{"stackoverflow.com", "reddit.com", "news.ycombinator.com", "superuser.com", "serverfault.com", "quora.com", "discourse.", "forum."},
		Keywords: []string{"discussion", "question", "problem", "solve", "community", "reddit", "thread", "opinion", "review"},
	},
	{
		Key:      "media",
		Name:     "Media & Video",
		Icon:     "🎬",
		Domains:  []string{"youtube.com", "youtu.be", "vimeo.com", "bilibili.com", "dailymotion.com", "soundcloud.com", "bandcamp.com"},
		Keywords: []string{"video", "watch", "course", "stream", "listen", "audio", "track", "podcast"},
	},
	{
		Key:      "research",
		Name:     "Research Papers",
		Icon:     "🔬",
		Domains:  []string{"arxiv.org", "pubmed.ncbi", "sciencedirect.com", "nature.com", "springer.com", "ieee.org", "biorxiv.org", "semanticscholar.org"},
		Keywords: []string{"abstract", "journal", "citation", "paper", "scientific", "study", "peer-reviewed", "experiment", "pdf"},
	},
	{
		Key:      "news",
		Name:     "News & Articles",
		Icon:     "📰",
		Domains:  []string{"news.", "reuters.com", "bbc.com", "theguardian.com", "bloomberg.com", "techcrunch.com", "theverge.com", "medium.com", "dev.to"},
		Keywords: []string{"news", "announced", "breaking", "update", "article", "report", "weekly", "release notes", "interview"},
	},
	{
		Key:      "tools",
		Name:     "Tools & Downloads",
		Icon:     "📦",
		Domains:  []string{"download.", "releases.", "hub.docker.com", "portableapps.com", "flathub.org", "snapcraft.io"},
		Keywords: []string{"download", "installer", "cli tool", "binary", "desktop app", "docker container", "setup", "portable"},
	},
}

// GenerateTopicClusters classifies search results into dynamic smart semantic clusters
// and assigns matching cluster keys to each item's Extra["clusters"] for zero-latency client filtering
func GenerateTopicClusters(results []models.SearchResult, query string) ([]models.SearchResult, []models.TopicCluster) {
	if len(results) == 0 {
		return results, nil
	}

	clusterCounts := make(map[string]int)
	processedResults := make([]models.SearchResult, len(results))

	for i, item := range results {
		itemCopy := item
		if itemCopy.Extra == nil {
			itemCopy.Extra = make(map[string]string)
		}

		u, _ := url.Parse(item.URL)
		host := ""
		if u != nil {
			host = strings.ToLower(u.Host)
		}

		titleLower := strings.ToLower(item.Title)
		contentLower := strings.ToLower(item.Content)
		urlLower := strings.ToLower(item.URL)

		var matchedKeys []string

		for _, rule := range defaultClusterRules {
			matched := false

			// 1. Check domains
			for _, d := range rule.Domains {
				if strings.Contains(host, d) || strings.Contains(urlLower, d) {
					matched = true
					break
				}
			}

			// 2. Check keywords if not matched by domain
			if !matched {
				for _, kw := range rule.Keywords {
					if containsWord(titleLower, kw) || containsWord(contentLower, kw) {
						matched = true
						break
					}
				}
			}

			if matched {
				matchedKeys = append(matchedKeys, rule.Key)
				clusterCounts[rule.Key]++
			}
		}

		// Store matched cluster keys as comma-separated in Clusters and Extra
		if len(matchedKeys) > 0 {
			itemCopy.Clusters = strings.Join(matchedKeys, ",")
			itemCopy.Extra["clusters"] = strings.Join(matchedKeys, ",")
		} else {
			itemCopy.Clusters = "general"
			itemCopy.Extra["clusters"] = "general"
		}

		processedResults[i] = itemCopy
	}

	// Build the cluster list
	var activeClusters []models.TopicCluster

	// Always prepend All cluster
	activeClusters = append(activeClusters, models.TopicCluster{
		Name:  "All Results",
		Icon:  "🌐",
		Count: len(results),
		Key:   "all",
	})

	for _, rule := range defaultClusterRules {
		count := clusterCounts[rule.Key]
		if count > 0 {
			activeClusters = append(activeClusters, models.TopicCluster{
				Name:  rule.Name,
				Icon:  rule.Icon,
				Count: count,
				Key:   rule.Key,
			})
		}
	}

	// If fewer than 2 specific clusters, generate query-specific dynamic keyword clusters
	if len(activeClusters) <= 2 && len(results) >= 4 {
		dynamicClusters := generateDynamicKeywordClusters(results, query)
		activeClusters = append(activeClusters, dynamicClusters...)
	}

	return processedResults, activeClusters
}

// generateDynamicKeywordClusters extracts recurring sub-topics from result titles
func generateDynamicKeywordClusters(results []models.SearchResult, query string) []models.TopicCluster {
	wordFreq := make(map[string]int)
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
		"from": true, "how": true, "what": true, "when": true, "where": true, "which": true,
		"into": true, "over": true, "after": true, "your": true, "about": true, "more": true,
		"com": true, "org": true, "net": true, "http": true, "https": true, "www": true,
	}

	for _, qToken := range strings.Fields(strings.ToLower(query)) {
		stopWords[qToken] = true
	}

	for _, item := range results {
		words := strings.Fields(strings.ToLower(item.Title))
		seenInItem := make(map[string]bool)
		for _, w := range words {
			w = strings.Trim(w, ",.-:;\"'()[]{}!/?")
			if len(w) >= 4 && !stopWords[w] && !seenInItem[w] {
				seenInItem[w] = true
				wordFreq[w]++
			}
		}
	}

	var dyn []models.TopicCluster
	for word, count := range wordFreq {
		if count >= 2 && len(dyn) < 3 {
			dyn = append(dyn, models.TopicCluster{
				Name:  fmt.Sprintf("Topic: %s", strings.Title(word)),
				Icon:  "🏷️",
				Count: count,
				Key:   "dyn-" + word,
			})
		}
	}

	return dyn
}

func containsWord(text, target string) bool {
	textLower := strings.ToLower(text)
	targetLower := strings.ToLower(target)

	if strings.Contains(targetLower, " ") {
		return strings.Contains(textLower, targetLower)
	}

	idx := 0
	targetLen := len(targetLower)
	for {
		pos := strings.Index(textLower[idx:], targetLower)
		if pos == -1 {
			return false
		}
		actualPos := idx + pos

		startOk := actualPos == 0 || isBoundaryRune(rune(textLower[actualPos-1]))
		endPos := actualPos + targetLen
		endOk := endPos == len(textLower) || isBoundaryRune(rune(textLower[endPos]))

		if startOk && endOk {
			return true
		}

		idx = actualPos + 1
		if idx >= len(textLower) {
			break
		}
	}
	return false
}

func isBoundaryRune(r rune) bool {
	return (r < 'a' || r > 'z') && (r < '0' || r > '9')
}
