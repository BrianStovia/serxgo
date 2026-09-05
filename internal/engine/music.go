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

type GeniusEngine struct {
	client *http.Client
}

func NewGeniusEngine() *GeniusEngine {
	return &GeniusEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *GeniusEngine) Name() string {
	return "genius"
}

func (e *GeniusEngine) DisplayName() string {
	return "Genius Lyrics"
}

func (e *GeniusEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic, models.CategoryGeneral}
}

func (e *GeniusEngine) DefaultOn() bool {
	return true
}

func (e *GeniusEngine) Weight() float64 {
	return 1.2
}

func (e *GeniusEngine) About() string {
	return "World's biggest collection of song lyrics and crowdsourced musical knowledge."
}

func (e *GeniusEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://genius.com/api/search/multi?q=%s", url.QueryEscape(req.Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "SearXGo/1.0")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("genius returned status %d", resp.StatusCode)
	}

	var data struct {
		Response struct {
			Sections []struct {
				Type string `json:"type"`
				Hits []struct {
					Result struct {
						Title           string `json:"title"`
						FullTitle       string `json:"full_title"`
						URL             string `json:"url"`
						ArtistNames     string `json:"artist_names"`
						HeaderImageURL  string `json:"header_image_url"`
						SongArtImageURL string `json:"song_art_image_url"`
					} `json:"result"`
				} `json:"hits"`
			} `json:"sections"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, sec := range data.Response.Sections {
		if sec.Type != "song" && sec.Type != "top_hit" {
			continue
		}
		for _, hit := range sec.Hits {
			res := hit.Result
			if res.Title == "" || res.URL == "" {
				continue
			}

			thumb := res.SongArtImageURL
			if thumb == "" {
				thumb = res.HeaderImageURL
			}

			extra := map[string]string{
				"Artist": res.ArtistNames,
			}

			results = append(results, models.SearchResult{
				Title:     fmt.Sprintf("%s - %s (Lyrics)", res.ArtistNames, res.Title),
				URL:       res.URL,
				PrettyURL: fmt.Sprintf("genius.com%s", res.URL),
				Content:   fmt.Sprintf("Song by %s. View full lyrics and annotations on Genius.", res.ArtistNames),
				Engine:    e.Name(),
				Category:  models.CategoryMusic,
				Thumbnail: thumb,
				Author:    res.ArtistNames,
				Extra:     extra,
			})
		}
	}

	return results, nil
}
