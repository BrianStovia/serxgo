package aggregator

import (
	"context"
	"testing"
	"time"

	"searxgo/internal/engine"
	"searxgo/internal/models"
)

type mockEngine struct {
	name        string
	category    models.Category
	weight      float64
	defaultOn   bool
	results     []models.SearchResult
	shouldError bool
}

func (m *mockEngine) Name() string { return m.name }
func (m *mockEngine) DisplayName() string { return m.name }
func (m *mockEngine) Categories() []models.Category { return []models.Category{m.category} }
func (m *mockEngine) DefaultOn() bool { return m.defaultOn }
func (m *mockEngine) Weight() float64 { return m.weight }
func (m *mockEngine) About() string { return "Mock engine for unit testing" }
func (m *mockEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	if m.shouldError {
		return nil, context.DeadlineExceeded
	}
	return m.results, nil
}

func TestAggregatorSearchAndFallback(t *testing.T) {
	reg := engine.NewRegistry()

	t1 := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	// Primary engine with few results (triggers fallback)
	reg.Register(&mockEngine{
		name:      "mock_primary",
		category:  models.CategoryGeneral,
		weight:    1.0,
		defaultOn: true,
		results: []models.SearchResult{
			{Title: "Golang Concurrency Patterns", URL: "https://golang.org/doc", Content: "Learn concurrency with goroutines and channels.", PublishedDate: &t1},
		},
	})

	// Secondary fallback engine
	reg.Register(&mockEngine{
		name:      "brave",
		category:  models.CategoryGeneral,
		weight:    1.1,
		defaultOn: false,
		results: []models.SearchResult{
			{Title: "Advanced Go Programming", URL: "https://example.com/go", Content: "Deep dive into Go concurrency and memory safety.", PublishedDate: &t1},
			{Title: "Rust Concurrency Guide", URL: "https://rust-lang.org", Content: "Fearless concurrency in Rust language.", PublishedDate: &t2},
		},
	})

	agg := NewAggregator(reg, 3*time.Second)

	// 1. Test Fallback Execution & Custom PageSize
	req := models.SearchRequest{
		Query:    "golang concurrency",
		Category: models.CategoryGeneral,
		Page:     1,
		PageSize: 25,
	}

	resp, err := agg.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(resp.Results) < 2 {
		t.Errorf("Expected at least 2 results after fallback, got %d", len(resp.Results))
	}
	if !resp.IsFallback {
		t.Errorf("Expected resp.IsFallback = true")
	}

	// 2. Test Boolean NOT operator
	boolReq := models.SearchRequest{
		Query:    "concurrency NOT rust",
		Category: models.CategoryGeneral,
	}
	boolResp, err := agg.Search(context.Background(), boolReq)
	if err != nil {
		t.Fatalf("Boolean search failed: %v", err)
	}
	for _, r := range boolResp.Results {
		if r.URL == "https://rust-lang.org" {
			t.Errorf("Item with 'rust' should have been filtered out by NOT operator")
		}
	}

	// 3. Test Date filter
	dateReq := models.SearchRequest{
		Query:    "concurrency after:2024-01-01",
		Category: models.CategoryGeneral,
	}
	dateResp, err := agg.Search(context.Background(), dateReq)
	if err != nil {
		t.Fatalf("Date search failed: %v", err)
	}
	for _, r := range dateResp.Results {
		if r.PublishedDate != nil && r.PublishedDate.Before(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("Found item older than 2024: %v", r.PublishedDate)
		}
	}
}

func TestQueryExpansionAndClassification(t *testing.T) {
	// 1. Test bilingual expansion
	variants := GenerateQueryVariants("belajar pemrograman")
	if len(variants) == 0 {
		t.Errorf("Expected query variants for 'belajar pemrograman'")
	}
	foundTutorial := false
	for _, v := range variants {
		if v == "learn programming" || v == "tutorial programming" || v == "guide programming" {
			foundTutorial = true
			break
		}
	}
	if !foundTutorial {
		t.Errorf("Expected translated English variant, got %v", variants)
	}

	// 2. Test query domain classification
	if !IsTechnicalQuery("golang goroutine memory leak") {
		t.Errorf("Expected IsTechnicalQuery to be true for golang query")
	}
	if !IsScientificQuery("quantum entanglement physics theorem") {
		t.Errorf("Expected IsScientificQuery to be true for physics query")
	}
	if !IsDiscussionQuery("best linux distro reddit discussion") {
		t.Errorf("Expected IsDiscussionQuery to be true for reddit query")
	}
	if !IsTorDeepWebQuery("onion hidden service darknet") {
		t.Errorf("Expected IsTorDeepWebQuery to be true for onion query")
	}
}
