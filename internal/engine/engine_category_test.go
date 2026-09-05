package engine

import (
	"testing"

	"searxgo/internal/models"
)

func TestEngineCategorization(t *testing.T) {
	// 1. YouTube should only be in Videos
	yt := NewYouTubeEngine()
	for _, c := range yt.Categories() {
		if c == models.CategoryGeneral {
			t.Errorf("YouTube must not be in CategoryGeneral")
		}
	}

	// 2. GitHub should only be in IT
	gh := NewGitHubEngine()
	for _, c := range gh.Categories() {
		if c == models.CategoryGeneral {
			t.Errorf("GitHub must not be in CategoryGeneral")
		}
	}

	// 3. OpenStreetMap should only be in Maps
	osm := NewOpenStreetMapEngine()
	for _, c := range osm.Categories() {
		if c == models.CategoryGeneral {
			t.Errorf("OpenStreetMap must not be in CategoryGeneral")
		}
	}

	// 4. Arxiv and PubMed should only be in Science
	arx := NewArxivEngine()
	for _, c := range arx.Categories() {
		if c == models.CategoryGeneral {
			t.Errorf("Arxiv must not be in CategoryGeneral")
		}
	}
	pm := NewPubMedEngine()
	for _, c := range pm.Categories() {
		if c == models.CategoryGeneral {
			t.Errorf("PubMed must not be in CategoryGeneral")
		}
	}
}

func TestExtractBingURL(t *testing.T) {
	testRaw := "https://www.bing.com/ck/a?!&&p=45225e281643ad28e07d17ccdcec342b6e427d3de7cb0517024c1de0cd830714JmltdHM9MTc4ODU2NjQwMA&ptn=3&ver=2&hsh=4&fclid=3b924597-f3d3-6c7e-2cbc-525cf2276d08&u=a1aHR0cHM6Ly9nb2xhbmcub3JnLw&ntb=1"
	extracted := extractBingURL(testRaw)
	if extracted != "https://golang.org/" {
		t.Fatalf("Expected https://golang.org/, got %s", extracted)
	}
}
