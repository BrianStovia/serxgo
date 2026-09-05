package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"searxgo/internal/models"
)

type OpenStreetMapEngine struct {
	client *http.Client
}

func NewOpenStreetMapEngine() *OpenStreetMapEngine {
	return &OpenStreetMapEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *OpenStreetMapEngine) Name() string {
	return "openstreetmap"
}

func (e *OpenStreetMapEngine) DisplayName() string {
	return "OpenStreetMap"
}

func (e *OpenStreetMapEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMaps, models.CategoryGeneral}
}

func (e *OpenStreetMapEngine) DefaultOn() bool {
	return true
}

func (e *OpenStreetMapEngine) Weight() float64 {
	return 1.3
}

func (e *OpenStreetMapEngine) About() string {
	return "Free, editable map of the whole world being built by volunteers."
}

func (e *OpenStreetMapEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&addressdetails=1&limit=5",
		url.QueryEscape(req.Query))
	if req.Page > 1 {
		apiURL += fmt.Sprintf("&offset=%d", (req.Page-1)*5)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "SearXGo/1.0 (Privacy Metasearch)")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	var places []struct {
		PlaceID     int64    `json:"place_id"`
		Lat         string   `json:"lat"`
		Lon         string   `json:"lon"`
		DisplayName string   `json:"display_name"`
		Class       string   `json:"class"`
		Type        string   `json:"type"`
		Importance  float64  `json:"importance"`
		Icon        string   `json:"icon"`
		BoundingBox []string `json:"boundingbox"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&places); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, p := range places {
		osmURL := fmt.Sprintf("https://www.openstreetmap.org/?mlat=%s&mlon=%s#map=15/%s/%s",
			p.Lat, p.Lon, p.Lat, p.Lon)

		extra := map[string]string{
			"Coordinates": fmt.Sprintf("%s, %s", p.Lat, p.Lon),
			"Type":        p.Type,
		}

		results = append(results, models.SearchResult{
			Title:     p.DisplayName,
			URL:       osmURL,
			PrettyURL: fmt.Sprintf("openstreetmap.org/?lat=%s&lon=%s", p.Lat, p.Lon),
			Content:   fmt.Sprintf("Category: %s (%s) | Location: Latitude %s, Longitude %s", p.Class, p.Type, p.Lat, p.Lon),
			Engine:    e.Name(),
			Category:  models.CategoryMaps,
			Thumbnail: p.Icon,
			Extra:     extra,
		})
	}

	return results, nil
}
