package aggregator

import (
	"testing"

	"searxgo/internal/models"
)

func TestCleanAndNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://example.com/test?utm_source=google&utm_medium=cpc&id=123",
			expected: "https://example.com/test?id=123",
		},
		{
			input:    "https://www.example.com/path/?fbclid=IwAR123",
			expected: "https://example.com/path",
		},
		{
			input:    "http://example.com:80/about/",
			expected: "http://example.com/about",
		},
		{
			input:    "https://EXAMPLE.COM/Page#section",
			expected: "https://example.com/Page",
		},
	}

	for _, tt := range tests {
		got := CleanAndNormalizeURL(tt.input)
		if got != tt.expected {
			t.Errorf("CleanAndNormalizeURL(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDeduplicate(t *testing.T) {
	results := []models.SearchResult{
		{
			Title:   "Example 1",
			URL:     "https://example.com/page?utm_source=twitter",
			Content: "Short content",
			Engine:  "duckduckgo",
			Score:   10.0,
		},
		{
			Title:   "Example 1 Full",
			URL:     "https://www.example.com/page",
			Content: "Longer content snippet from second engine",
			Engine:  "brave",
			Score:   8.0,
		},
		{
			Title:   "Different Page",
			URL:     "https://other.com/article",
			Content: "Another content",
			Engine:  "wikipedia",
			Score:   5.0,
		},
	}

	deduped := Deduplicate(results)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 unique results, got %d", len(deduped))
	}

	first := deduped[0]
	if len(first.Engines) != 2 {
		t.Errorf("expected 2 merged engines for first result, got %d (%v)", len(first.Engines), first.Engines)
	}
	if first.Content != "Longer content snippet from second engine" {
		t.Errorf("expected longer snippet to be preserved, got: %s", first.Content)
	}
}
