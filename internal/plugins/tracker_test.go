package plugins

import (
	"testing"

	"searxgo/internal/models"
)

func TestCleanTrackerParameters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Google Analytics UTM parameters",
			input:    "https://example.com/article?utm_source=twitter&utm_medium=social&utm_campaign=launch&id=42",
			expected: "https://example.com/article?id=42",
		},
		{
			name:     "Facebook click ID",
			input:    "https://example.com/shop?fbclid=IwAR0b_12345&prod=99",
			expected: "https://example.com/shop?prod=99",
		},
		{
			name:     "Only trackers in query",
			input:    "https://example.com/?utm_source=newsletter",
			expected: "https://example.com/",
		},
		{
			name:     "Clean URL untouched",
			input:    "https://example.com/docs/api?version=2&lang=go",
			expected: "https://example.com/docs/api?version=2&lang=go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := CleanTrackerParameters(tc.input)
			if actual != tc.expected {
				t.Errorf("CleanTrackerParameters(%q) = %q, expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestStripURLTrackers(t *testing.T) {
	results := []models.SearchResult{
		{
			Title: "Test Article",
			URL:   "https://example.org/test?utm_source=rss&utm_medium=feed&view=full",
		},
	}

	cleaned := StripURLTrackers(results)
	if cleaned[0].URL != "https://example.org/test?view=full" {
		t.Errorf("expected cleaned URL without utm parameters, got %s", cleaned[0].URL)
	}
}
