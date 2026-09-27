package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"searxgo/internal/models"
)

// ==============================================================================
// 1. Genius Lyrics & Annotations Engine
// ==============================================================================

type GeniusEngine struct {
	client *http.Client
}

func NewGeniusEngine() *GeniusEngine {
	return &GeniusEngine{
		client: NewHTTPClient(5 * time.Second),
	}
}

func (e *GeniusEngine) Name() string {
	return "genius"
}

func (e *GeniusEngine) DisplayName() string {
	return "Genius Lyrics"
}

func (e *GeniusEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic}
}

func (e *GeniusEngine) DefaultOn() bool {
	return true
}

func (e *GeniusEngine) Weight() float64 {
	return 1.2
}

func (e *GeniusEngine) About() string {
	return "World's biggest collection of song lyrics and musical knowledge."
}

func (e *GeniusEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://genius.com/api/search/multi?q=%s", url.QueryEscape(req.Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 SearXGo/1.0")

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

			results = append(results, models.SearchResult{
				Title:     fmt.Sprintf("%s - %s (Lyrics)", res.ArtistNames, res.Title),
				URL:       res.URL,
				PrettyURL: fmt.Sprintf("genius.com%s", res.URL),
				Content:   fmt.Sprintf("Song lyrics & annotations by %s on Genius.", res.ArtistNames),
				Engine:    e.Name(),
				Category:  models.CategoryMusic,
				Thumbnail: thumb,
				Author:    res.ArtistNames,
				Engines:   []string{e.Name()},
			})
		}
	}

	return results, nil
}

// ==============================================================================
// 2. iTunes Music & Audio Previews Engine (Free Public API)
// ==============================================================================

type ITunesEngine struct {
	client *http.Client
}

func NewITunesEngine() *ITunesEngine {
	return &ITunesEngine{
		client: NewHTTPClient(5 * time.Second),
	}
}

func (e *ITunesEngine) Name() string {
	return "itunes"
}

func (e *ITunesEngine) DisplayName() string {
	return "Apple iTunes Music"
}

func (e *ITunesEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic}
}

func (e *ITunesEngine) DefaultOn() bool {
	return true
}

func (e *ITunesEngine) Weight() float64 {
	return 1.4
}

func (e *ITunesEngine) About() string {
	return "Apple iTunes official music database with high quality audio previews and cover art."
}

func (e *ITunesEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=music&entity=song&limit=25", url.QueryEscape(req.Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 SearXGo/1.0")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("itunes status %d", resp.StatusCode)
	}

	var data struct {
		ResultCount int `json:"resultCount"`
		Results     []struct {
			TrackName        string `json:"trackName"`
			ArtistName       string `json:"artistName"`
			CollectionName   string `json:"collectionName"`
			TrackViewURL     string `json:"trackViewUrl"`
			PreviewURL       string `json:"previewUrl"`
			ArtworkURL100    string `json:"artworkUrl100"`
			PrimaryGenreName string `json:"primaryGenreName"`
			ReleaseDate      string `json:"releaseDate"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Results {
		if item.TrackName == "" || item.TrackViewURL == "" {
			continue
		}

		targetURL := item.TrackViewURL
		if item.PreviewURL != "" {
			// If audio stream preview is available, use it so users can stream directly in SearXGo!
			targetURL = item.PreviewURL
		}

		genre := item.PrimaryGenreName
		if genre == "" {
			genre = "Music"
		}

		albumInfo := ""
		if item.CollectionName != "" {
			albumInfo = fmt.Sprintf("Album: %s | ", item.CollectionName)
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("🎵 %s - %s", item.ArtistName, item.TrackName),
			URL:       targetURL,
			PrettyURL: fmt.Sprintf("music.apple.com/%s", item.TrackName),
			Content:   fmt.Sprintf("%sGenre: %s. Includes instant audio stream preview.", albumInfo, genre),
			Engine:    e.Name(),
			Category:  models.CategoryMusic,
			Thumbnail: item.ArtworkURL100,
			Author:    item.ArtistName,
			Engines:   []string{e.Name()},
		})
	}

	return results, nil
}

// ==============================================================================
// 3. Radio Browser Global Streaming Radio Engine (Free Open Database)
// ==============================================================================

type RadioBrowserEngine struct {
	client *http.Client
}

func NewRadioBrowserEngine() *RadioBrowserEngine {
	return &RadioBrowserEngine{
		client: NewHTTPClient(5 * time.Second),
	}
}

func (e *RadioBrowserEngine) Name() string {
	return "radio"
}

func (e *RadioBrowserEngine) DisplayName() string {
	return "Radio Browser"
}

func (e *RadioBrowserEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic}
}

func (e *RadioBrowserEngine) DefaultOn() bool {
	return true
}

func (e *RadioBrowserEngine) Weight() float64 {
	return 1.1
}

func (e *RadioBrowserEngine) About() string {
	return "Community-driven database of over 40,000 live streaming radio stations worldwide."
}

func (e *RadioBrowserEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://de1.api.radio-browser.info/json/stations/byname/%s?limit=20", url.QueryEscape(req.Query))

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
		return nil, fmt.Errorf("radio-browser status %d", resp.StatusCode)
	}

	var stations []struct {
		Name        string `json:"name"`
		URLResolved string `json:"url_resolved"`
		Homepage    string `json:"homepage"`
		Favicon     string `json:"favicon"`
		Country     string `json:"country"`
		Tags        string `json:"tags"`
		Bitrate     int    `json:"bitrate"`
		Codec       string `json:"codec"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&stations); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, st := range stations {
		if st.Name == "" || st.URLResolved == "" {
			continue
		}

		infoParts := []string{}
		if st.Country != "" {
			infoParts = append(infoParts, st.Country)
		}
		if st.Codec != "" {
			infoParts = append(infoParts, fmt.Sprintf("%s %dkbps", strings.ToUpper(st.Codec), st.Bitrate))
		}
		if st.Tags != "" {
			infoParts = append(infoParts, fmt.Sprintf("Tags: %s", st.Tags))
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("📻 %s (Live Radio)", st.Name),
			URL:       st.URLResolved,
			PrettyURL: st.Homepage,
			Content:   fmt.Sprintf("Live online radio broadcast. %s", strings.Join(infoParts, " • ")),
			Engine:    e.Name(),
			Category:  models.CategoryMusic,
			Thumbnail: st.Favicon,
			Engines:   []string{e.Name()},
		})
	}

	return results, nil
}
