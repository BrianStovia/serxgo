package plugins

import (
	"testing"

	"searxgo/internal/models"
)

func TestApplyAhmiaFilter(t *testing.T) {
	results := []models.SearchResult{
		{
			Title:   "Valid Linux Onion Site",
			URL:     "http://exampleonionaddress.onion/linux",
			Content: "Linux documentation and tutorials on the darknet",
		},
		{
			Title:   "Unsafe banned item",
			URL:     "http://maliciousonion.onion/evil",
			Content: "pedophilia and illegal content",
		},
	}

	filtered := ApplyAhmiaFilter(results)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 result after ahmia filter, got %d", len(filtered))
	}
	if filtered[0].Title != "Valid Linux Onion Site" {
		t.Errorf("unexpected remaining item: %s", filtered[0].Title)
	}
}
