package newshub

import (
	"context"
	"testing"
	"time"
)

func TestNewshubFeedSources(t *testing.T) {
	if len(FeedSources) == 0 {
		t.Fatal("expected FeedSources to not be empty")
	}

	for _, src := range FeedSources {
		if src.Name == "" || src.URL == "" || src.Category == "" {
			t.Errorf("invalid feed source configuration: %+v", src)
		}
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	cases := []struct {
		diff     time.Duration
		expected string
	}{
		{30 * time.Second, "Just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{48 * time.Hour, "2d ago"},
	}

	for _, c := range cases {
		past := now.Add(-c.diff)
		res := formatRelativeTime(now, past)
		if res != c.expected {
			t.Errorf("for diff %v: expected %s, got %s", c.diff, c.expected, res)
		}
	}
}

func TestNewshubServiceCache(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	// Seed cache directly
	svc.cacheMu.Lock()
	svc.cache["tech:world"] = &cachedFeed{
		articles: []NewsArticle{
			{Title: "Golang 1.23 Released", Source: "TechCrunch", Category: "tech", Region: "world"},
			{Title: "AI Breakthrough Announced", Source: "The Verge", Category: "tech", Region: "world"},
		},
		fetchedAt: time.Now(),
	}
	svc.cacheMu.Unlock()

	// 1. Get cached
	res, err := svc.GetNews(ctx, "tech", "world", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 cached articles, got %d", len(res))
	}

	// 2. Keyword query filter
	filtered, err := svc.GetNews(ctx, "tech", "world", "golang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Title != "Golang 1.23 Released" {
		t.Fatalf("expected 1 filtered article matching golang, got %+v", filtered)
	}
}
