package aggregator

import (
	"strings"
	"testing"

	"searxgo/internal/models"
)

func TestPrecisionTopicClustering(t *testing.T) {
	results := []models.SearchResult{
		{
			Title:   "Go programming language documentation",
			URL:     "https://pkg.go.dev/net/http",
			Content: "Package http provides HTTP client and server implementations. See official API reference.",
		},
		{
			Title:   "golang/go: The Go programming language",
			URL:     "https://github.com/golang/go",
			Content: "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.",
		},
		{
			Title:   "How to fix goroutine memory leak in Go?",
			URL:     "https://stackoverflow.com/questions/12345/goroutine-leak",
			Content: "I have a question regarding goroutine memory leak. Upvotes and community answers below.",
		},
		{
			Title:   "Deep Learning for Quantum Computing Paper",
			URL:     "https://arxiv.org/abs/2301.00001",
			Content: "Abstract: We present a novel neural architecture for quantum circuit synthesis. DOI: 10.1234/arxiv.2301.",
		},
		{
			Title:   "Breaking News: New Tech Announcement",
			URL:     "https://reuters.com/technology/article-tech-update",
			Content: "Reuters reports breaking news regarding recent semiconductor advancements.",
		},
	}

	processed, clusters := GenerateTopicClusters(results, "golang")

	if len(processed) != len(results) {
		t.Fatalf("expected %d results, got %d", len(results), len(processed))
	}

	// Verify GitHub repo is ONLY tagged as 'code', not falsely tagged as 'news' or 'research'
	githubItem := processed[1]
	if !strings.Contains(githubItem.Clusters, "code") {
		t.Errorf("expected GitHub item to have 'code' cluster, got: %s", githubItem.Clusters)
	}
	if strings.Contains(githubItem.Clusters, "news") {
		t.Errorf("GitHub item was falsely tagged as 'news': %s", githubItem.Clusters)
	}

	// Verify arXiv paper is tagged as 'research', not 'tools'
	arxivItem := processed[3]
	if !strings.Contains(arxivItem.Clusters, "research") {
		t.Errorf("expected arXiv item to have 'research' cluster, got: %s", arxivItem.Clusters)
	}

	// Verify All Results is first cluster
	if len(clusters) == 0 || clusters[0].Key != "all" {
		t.Errorf("expected first cluster to be 'all', got: %+v", clusters)
	}
}

func TestContainsWord(t *testing.T) {
	tests := []struct {
		text     string
		target   string
		expected bool
	}{
		{"Go documentation and api reference", "api", true},
		{"Capitalism is an economic system", "api", false}, // substring inside word should not match!
		{"Start your engines", "art", false},               // substring inside word should not match!
		{"State-of-the-art technology", "art", true},       // hyphenated boundary matches
		{"Breaking news reported today", "breaking news", true},
	}

	for _, tt := range tests {
		res := containsWord(tt.text, tt.target)
		if res != tt.expected {
			t.Errorf("containsWord(%q, %q) = %v, expected %v", tt.text, tt.target, res, tt.expected)
		}
	}
}
