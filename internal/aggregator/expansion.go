package aggregator

import (
	"strings"
)

var idToEnMap = map[string][]string{
	"cara":        {"how to", "guide"},
	"membuat":     {"build", "create", "make"},
	"belajar":     {"learn", "tutorial", "guide"},
	"pemrograman": {"programming", "coding", "software"},
	"arsitektur":  {"architecture", "design"},
	"keamanan":    {"security", "cybersecurity"},
	"jaringan":    {"network", "networking"},
	"kinerja":     {"performance", "benchmark"},
	"algoritma":   {"algorithm", "data structure"},
	"basis data":  {"database", "storage"},
	"gratis":      {"free", "open source"},
	"terbaik":     {"best", "top", "comparison"},
	"berita":      {"news", "update"},
	"jurnal":      {"paper", "research", "journal"},
	"penelitian":  {"research", "study"},
	"kecerdasan":  {"artificial intelligence", "AI", "machine learning"},
	"masalah":     {"issue", "error", "bug", "fix"},
	"solusi":      {"solution", "fix", "troubleshoot"},
	"unduh":       {"download", "install"},
}

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

// GenerateQueryVariants produces bilingual and synonym-expanded query variations
func GenerateQueryVariants(rawQuery string) []string {
	q := strings.TrimSpace(rawQuery)
	if q == "" {
		return nil
	}

	lower := strings.ToLower(q)
	words := strings.Fields(lower)
	if len(words) == 0 {
		return nil
	}

	var variants []string
	seen := make(map[string]bool)
	seen[lower] = true

	// 1. Indonesian to English translation variant
	translatedWords := make([]string, len(words))
	hasTranslation := false

	for i, word := range words {
		if synonyms, ok := idToEnMap[word]; ok && len(synonyms) > 0 {
			translatedWords[i] = synonyms[0]
			hasTranslation = true
		} else {
			translatedWords[i] = word
		}
	}

	if hasTranslation {
		tQuery := strings.Join(translatedWords, " ")
		if !seen[tQuery] {
			seen[tQuery] = true
			variants = append(variants, tQuery)
		}
	}

	// 2. Phrase matching (e.g. "cara membuat" -> "how to build")
	for idPhrase, enPhrases := range idToEnMap {
		if strings.Contains(lower, idPhrase) {
			for _, en := range enPhrases {
				replaced := strings.ReplaceAll(lower, idPhrase, en)
				if !seen[replaced] {
					seen[replaced] = true
					variants = append(variants, replaced)
				}
			}
		}
	}

	return variants
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
