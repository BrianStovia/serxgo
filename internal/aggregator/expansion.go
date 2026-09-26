package aggregator

import (
	"strings"
)


var techKeywords = []string{
	"golang", "python", "javascript", "typescript", "rust", "c++", "docker", "kubernetes",
	"linux", "api", "github", "git", "sql", "nosql", "redis", "nginx", "react", "vue",
	"error", "bug", "compile", "exception", "struct", "class", "function", "method",
	"goroutine", "async", "await", "kernel", "package", "npm", "pypi", "framework",
}

var scienceKeywords = []string{
	"quantum", "physics", "biology", "chemistry", "arxiv", "pubmed", "doi", "paper",
	"theorem", "equation", "dna", "crispr", "genetics", "astronomy", "black hole",
	"relativity", "particle", "clinical", "vaccine", "neural network", "transformer model",
}

var discussionKeywords = []string{
	"reddit", "hackernews", "forum", "opinion", "review", "vs", "versus", "pros and cons",
	"experience", "recommendation", "discussion", "community",
}

var torDeepKeywords = []string{
	"onion", "tor", "darknet", "deep web", "privacy", "anonymity", "hidden service",
}

var leakBreachKeywords = []string{
	"kebocoran data", "data bocor", "peretasan data", "data breach", "databreach",
	"database dump", "dump database", "leaked database", "leaked credentials",
	"haveibeenpwned", "dehashed", "pastebin dump", "ransomware", "wikileaks",
	"telegram leak", "tg dump", "cve exploit", "zero-day exploit", "0day",
	"exploit-db", "victim leak", "exfiltration", "pwned",
}

var mediaVideoKeywords = []string{
	"video", "lagu", "musik", "nonton", "film", "streaming", "clip", "song",
	"trailer", "stream", "youtube", "podcast", "audio", "mv",
}

// GenerateQueryVariants produces clean query variations without artificial language translation
func GenerateQueryVariants(rawQuery string) []string {
	q := strings.TrimSpace(rawQuery)
	if q == "" {
		return nil
	}

	// If query is enclosed in quotes, provide unquoted variation
	if (strings.HasPrefix(q, "\"") && strings.HasSuffix(q, "\"")) || (strings.HasPrefix(q, "'") && strings.HasSuffix(q, "'")) {
		trimmed := strings.Trim(q, "\"'")
		if trimmed != "" && trimmed != q {
			return []string{trimmed}
		}
	}

	return nil
}

// IsTechnicalQuery returns true if query matches programming or developer topics
func IsTechnicalQuery(q string) bool {
	lower := strings.ToLower(q)
	for _, kw := range techKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsScientificQuery returns true if query contains academic or scientific terms
func IsScientificQuery(q string) bool {
	lower := strings.ToLower(q)
	for _, kw := range scienceKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsDiscussionQuery returns true if query asks for opinions, comparisons, or community threads
func IsDiscussionQuery(q string) bool {
	lower := strings.ToLower(q)
	for _, kw := range discussionKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsTorDeepWebQuery returns true if query targets privacy deep web or onion networks
func IsTorDeepWebQuery(q string) bool {
	lower := strings.ToLower(q)
	for _, kw := range torDeepKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsLeakOrBreachQuery returns true if natural language query targets data leaks, breaches, or dumps
func IsLeakOrBreachQuery(q string) bool {
	lower := strings.ToLower(q)
	for _, kw := range leakBreachKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsMediaOrVideoQuery returns true if query targets videos, songs, or streaming media
func IsMediaOrVideoQuery(q string) bool {
	lower := strings.ToLower(q)
	for _, kw := range mediaVideoKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
