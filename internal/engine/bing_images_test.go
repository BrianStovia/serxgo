package engine

import (
	"context"
	"testing"
	"time"

	"searxgo/internal/models"
)

func TestBingImagesEngine(t *testing.T) {
	eng := NewBingImagesEngine()
	if eng.Name() != "bing_images" {
		t.Fatalf("expected name bing_images, got %s", eng.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := eng.Search(ctx, models.SearchRequest{
		Query:    "mountain landscape",
		Category: models.CategoryImages,
	})
	if err != nil {
		t.Logf("Bing Images returned error (may be network restricted): %v", err)
		return
	}

	if len(results) == 0 {
		t.Logf("Bing Images returned 0 results for query")
		return
	}

	for i, r := range results {
		if r.Thumbnail == "" && r.ImageURL == "" {
			t.Errorf("result %d missing both Thumbnail and ImageURL", i)
		}
		if r.Category != models.CategoryImages {
			t.Errorf("result %d expected CategoryImages, got %s", i, r.Category)
		}
	}
	t.Logf("Bing Images successfully retrieved %d valid image items", len(results))
}
