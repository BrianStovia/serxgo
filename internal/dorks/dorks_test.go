package dorks

import (
	"strings"
	"testing"
)

func TestDorksCategories(t *testing.T) {
	categories := GetAllCategories()
	if len(categories) == 0 {
		t.Fatalf("Expected non-empty dork categories")
	}

	foundTotalDorks := 0
	for _, cat := range categories {
		if cat.ID == "" || cat.Name == "" {
			t.Errorf("Category missing ID or Name: %+v", cat)
		}
		if len(cat.Dorks) == 0 {
			t.Errorf("Category %s has no dorks", cat.ID)
		}
		foundTotalDorks += len(cat.Dorks)

		for _, dork := range cat.Dorks {
			if dork.Query == "" {
				t.Errorf("Dork %s has empty query", dork.ID)
			}

			// Test scoping
			scoped := dork.BuildQuery("example.com")
			if !strings.Contains(scoped, "site:example.com") && !strings.Contains(dork.Query, "site:") {
				t.Errorf("Expected scoped query to contain site:example.com, got: %s", scoped)
			}

			googleURL := dork.GoogleURL("target.org")
			if !strings.HasPrefix(googleURL, "https://www.google.com/search?q=") {
				t.Errorf("Invalid Google URL: %s", googleURL)
			}

			searxURL := dork.SearxgoURL("target.org")
			if !strings.HasPrefix(searxURL, "/search?q=") {
				t.Errorf("Invalid SearXGo URL: %s", searxURL)
			}
		}
	}

	if foundTotalDorks < 15 {
		t.Errorf("Expected at least 15 dorks, got %d", foundTotalDorks)
	}
}
