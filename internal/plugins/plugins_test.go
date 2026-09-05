package plugins

import (
	"testing"

	"searxgo/internal/models"
)

func TestDOIPlugin(t *testing.T) {
	results := []models.SearchResult{
		{
			Title:   "Quantum Computing Breakthrough",
			URL:     "https://nature.com/articles/10.1038/s41586-019-1666-5",
			Content: "Experimental demonstration of quantum supremacy.",
		},
	}

	processed := ApplyDOIResolver(results, "sci-hub.se")
	if len(processed) != 1 {
		t.Fatalf("expected 1 result, got %d", len(processed))
	}

	if processed[0].Extra["DOI"] != "10.1038/s41586-019-1666-5" {
		t.Errorf("expected DOI extracted, got %s", processed[0].Extra["DOI"])
	}

	expectedOA := "https://sci-hub.se/10.1038/s41586-019-1666-5"
	if processed[0].Extra["Open Access"] != expectedOA {
		t.Errorf("expected Open Access link %s, got %s", expectedOA, processed[0].Extra["Open Access"])
	}
}

func TestDOIInstantAnswer(t *testing.T) {
	ans := CheckDOIQuery("10.1038/nature12373", "oadoi.org")
	if ans == nil {
		t.Fatal("expected instant answer for DOI query")
	}

	if ans.Type != "infobox" {
		t.Errorf("expected infobox type, got %s", ans.Type)
	}
}

func TestFrontendRedirects(t *testing.T) {
	results := []models.SearchResult{
		{
			Title: "Funny Video",
			URL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			Title: "Golang Post",
			URL:   "https://www.reddit.com/r/golang/comments/12345/",
		},
	}

	redirected := ApplyFrontendRedirects(results)
	if redirected[0].URL != "https://yewtu.be/watch?v=dQw4w9WgXcQ" {
		t.Errorf("expected yewtu.be redirect, got %s", redirected[0].URL)
	}
	if redirected[1].URL != "https://libreddit.kavin.rocks/r/golang/comments/12345/" {
		t.Errorf("expected libreddit redirect, got %s", redirected[1].URL)
	}
}
