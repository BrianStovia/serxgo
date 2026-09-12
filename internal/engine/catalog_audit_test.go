package engine

import (
	"strings"
	"testing"
)

func TestEngineCatalogCompleteness(t *testing.T) {
	if len(FullEngineCatalog) < 320 {
		t.Fatalf("Expected at least 320 engines for full SearXNG parity, got %d", len(FullEngineCatalog))
	}

	seenIDs := make(map[string]bool)
	for _, e := range FullEngineCatalog {
		if strings.TrimSpace(e.ID) == "" {
			t.Errorf("Engine has empty ID: %+v", e)
		}
		if seenIDs[e.ID] {
			t.Errorf("Duplicate engine ID in catalog: %s", e.ID)
		}
		seenIDs[e.ID] = true

		if e.Category == "" {
			t.Errorf("Engine %s missing category", e.ID)
		}
		if e.Shortcut == "" {
			t.Errorf("Engine %s missing shortcut bang", e.ID)
		}
	}

	categorized := GetCategorizedCatalog()
	if len(categorized) < 8 {
		t.Errorf("Expected categorized catalog to have categories, got %d", len(categorized))
	}
}
