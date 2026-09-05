package instant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"searxgo/internal/models"
	"searxgo/internal/plugins"
)

type InstantService struct {
	client *http.Client
}

func NewInstantService() *InstantService {
	return &InstantService{
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

// FindInstantAnswers looks for instant tools, calculator, and Wikipedia knowledge graph cards
func (s *InstantService) FindInstantAnswers(ctx context.Context, query string, clientIP, userAgent string) []models.InstantAnswer {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	var answers []models.InstantAnswer

	// 1. Check Instant Tools (Weather, Hashes, QR, UUID, IP, Dice)
	if toolAns := CheckInstantTools(ctx, q, clientIP, userAgent); toolAns != nil {
		answers = append(answers, *toolAns)
		return answers
	}

	// 2. Check Calculator & Conversions
	if calcAns := CheckCalculator(q); calcAns != nil {
		answers = append(answers, *calcAns)
		return answers
	}

	// 3. Check Open Access DOI Query
	if doiAns := plugins.CheckDOIQuery(q, "oadoi.org"); doiAns != nil {
		answers = append(answers, *doiAns)
		return answers
	}

	// 3. Check Wikipedia Summary Infobox for definitions / entity lookups
	if infoAns := s.checkWikipediaSummary(ctx, q); infoAns != nil {
		answers = append(answers, *infoAns)
	}

	return answers
}

func (s *InstantService) checkWikipediaSummary(ctx context.Context, query string) *models.InstantAnswer {
	cleanQ := query
	prefixes := []string{
		"what is a ", "what is an ", "what is ", "what are ",
		"who is ", "who was ", "define ", "meaning of ",
	}
	qLower := strings.ToLower(cleanQ)
	for _, p := range prefixes {
		if strings.HasPrefix(qLower, p) {
			cleanQ = strings.TrimSpace(cleanQ[len(p):])
			break
		}
	}

	if len(cleanQ) < 2 {
		return nil
	}

	apiURL := fmt.Sprintf("https://en.wikipedia.org/api/rest_v1/page/summary/%s", url.PathEscape(cleanQ))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := s.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var summary struct {
		Title       string `json:"title"`
		Extract     string `json:"extract"`
		Description string `json:"description"`
		Thumbnail   struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
		ContentUrls struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
		Type string `json:"type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil
	}

	if summary.Type != "standard" || summary.Extract == "" {
		return nil
	}

	return &models.InstantAnswer{
		Type:        "infobox",
		Title:       summary.Title,
		Value:       summary.Description,
		Description: summary.Extract,
		URL:         summary.ContentUrls.Desktop.Page,
		Thumbnail:   summary.Thumbnail.Source,
	}
}
