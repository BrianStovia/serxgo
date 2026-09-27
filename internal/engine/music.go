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
// 1. Deezer Global Music Engine (Ultra-accurate global catalog, MP3 previews & 500x500 HD art)
// ==============================================================================

type DeezerEngine struct {
	client *http.Client
}

func NewDeezerEngine() *DeezerEngine {
	return &DeezerEngine{
		client: NewHTTPClient(6 * time.Second),
	}
}

func (e *DeezerEngine) Name() string {
	return "deezer"
}

func (e *DeezerEngine) DisplayName() string {
	return "Deezer Music"
}

func (e *DeezerEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic}
}

func (e *DeezerEngine) DefaultOn() bool {
	return true
}

func (e *DeezerEngine) Weight() float64 {
	return 2.5 // Highest priority for exact song and artist matches
}

func (e *DeezerEngine) About() string {
	return "Global music streaming catalog with over 90 million songs, HD album art, and direct MP3 audio previews."
}

func (e *DeezerEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://api.deezer.com/search?q=%s&limit=30", url.QueryEscape(req.Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo/1.0")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer status %d", resp.StatusCode)
	}

	var data struct {
		Data []struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			Link     string `json:"link"`
			Duration int    `json:"duration"`
			Preview  string `json:"preview"`
			Artist   struct {
				Name string `json:"name"`
			} `json:"artist"`
			Album struct {
				Title    string `json:"title"`
				CoverBig string `json:"cover_big"`
				CoverXL  string `json:"cover_xl"`
			} `json:"album"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Data {
		if item.Title == "" {
			continue
		}

		thumb := item.Album.CoverBig
		if thumb == "" {
			thumb = item.Album.CoverXL
		}

		durationStr := ""
		if item.Duration > 0 {
			durationStr = fmt.Sprintf("%d:%02d", item.Duration/60, item.Duration%60)
		}

		contentParts := []string{fmt.Sprintf("Artist: %s", item.Artist.Name)}
		if item.Album.Title != "" {
			contentParts = append(contentParts, fmt.Sprintf("Album: %s", item.Album.Title))
		}
		if durationStr != "" {
			contentParts = append(contentParts, fmt.Sprintf("Duration: %s", durationStr))
		}

		extra := map[string]string{
			"Artist": item.Artist.Name,
			"Album":  item.Album.Title,
		}
		if durationStr != "" {
			extra["Duration"] = durationStr
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("%s - %s", item.Artist.Name, item.Title),
			URL:       item.Link,
			PrettyURL: fmt.Sprintf("deezer.com/track/%d", item.ID),
			Content:   strings.Join(contentParts, " • "),
			Engine:    e.Name(),
			Category:  models.CategoryMusic,
			Thumbnail: thumb,
			Author:    item.Artist.Name,
			AudioURL:  item.Preview, // MP3 preview stream
			Duration:  durationStr,
			Extra:     extra,
			Engines:   []string{e.Name()},
			Score:     10.0,
		})
	}

	return results, nil
}

// ==============================================================================
// 2. Apple iTunes Music & Audio Previews Engine (Official Global Database)
// ==============================================================================

type ITunesEngine struct {
	client *http.Client
}

func NewITunesEngine() *ITunesEngine {
	return &ITunesEngine{
		client: NewHTTPClient(6 * time.Second),
	}
}

func (e *ITunesEngine) Name() string {
	return "itunes"
}

func (e *ITunesEngine) DisplayName() string {
	return "Apple Music"
}

func (e *ITunesEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic}
}

func (e *ITunesEngine) DefaultOn() bool {
	return true
}

func (e *ITunesEngine) Weight() float64 {
	return 2.0
}

func (e *ITunesEngine) About() string {
	return "Apple iTunes official music database with HD artwork, audio previews, and metadata."
}

func (e *ITunesEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=music&entity=song&limit=25", url.QueryEscape(req.Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo/1.0")

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
			TrackTimeMillis  int    `json:"trackTimeMillis"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, item := range data.Results {
		if item.TrackName == "" {
			continue
		}

		thumb := strings.Replace(item.ArtworkURL100, "100x100bb.jpg", "400x400bb.jpg", 1)
		if thumb == "" {
			thumb = item.ArtworkURL100
		}

		genre := item.PrimaryGenreName
		if genre == "" {
			genre = "Music"
		}

		durationStr := ""
		if item.TrackTimeMillis > 0 {
			totalSec := item.TrackTimeMillis / 1000
			durationStr = fmt.Sprintf("%d:%02d", totalSec/60, totalSec%60)
		}

		contentParts := []string{}
		if item.CollectionName != "" {
			contentParts = append(contentParts, fmt.Sprintf("Album: %s", item.CollectionName))
		}
		if genre != "" {
			contentParts = append(contentParts, fmt.Sprintf("Genre: %s", genre))
		}

		targetWebURL := item.TrackViewURL
		if targetWebURL == "" {
			targetWebURL = fmt.Sprintf("https://music.apple.com/search?term=%s", url.QueryEscape(item.ArtistName+" "+item.TrackName))
		}

		extra := map[string]string{
			"Artist": item.ArtistName,
			"Genre":  genre,
		}
		if durationStr != "" {
			extra["Duration"] = durationStr
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("%s - %s", item.ArtistName, item.TrackName),
			URL:       targetWebURL,
			PrettyURL: fmt.Sprintf("music.apple.com • %s", item.TrackName),
			Content:   strings.Join(contentParts, " • "),
			Engine:    e.Name(),
			Category:  models.CategoryMusic,
			Thumbnail: thumb,
			Author:    item.ArtistName,
			AudioURL:  item.PreviewURL, // AAC stream
			Duration:  durationStr,
			Extra:     extra,
			Engines:   []string{e.Name()},
			Score:     9.0,
		})
	}

	return results, nil
}

// ==============================================================================
// 3. Genius Lyrics & Song Annotations Engine
// ==============================================================================

type GeniusEngine struct {
	client *http.Client
}

func NewGeniusEngine() *GeniusEngine {
	return &GeniusEngine{
		client: NewHTTPClient(6 * time.Second),
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
	return 1.3
}

func (e *GeniusEngine) About() string {
	return "World's biggest collection of song lyrics, producer notes, and crowdsourced annotations."
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
				Title:     fmt.Sprintf("📜 %s - %s (Lyrics)", res.ArtistNames, res.Title),
				URL:       res.URL,
				PrettyURL: fmt.Sprintf("genius.com%s", res.URL),
				Content:   fmt.Sprintf("Verified song lyrics & annotations for '%s' by %s on Genius.", res.Title, res.ArtistNames),
				Engine:    e.Name(),
				Category:  models.CategoryMusic,
				Thumbnail: thumb,
				Author:    res.ArtistNames,
				Engines:   []string{e.Name()},
				Score:     6.0,
			})
		}
	}

	return results, nil
}

// ==============================================================================
// 4. Radio Browser Global Streaming Radio Engine
// ==============================================================================

type RadioBrowserEngine struct {
	client *http.Client
}

func NewRadioBrowserEngine() *RadioBrowserEngine {
	return &RadioBrowserEngine{
		client: NewHTTPClient(6 * time.Second),
	}
}

func (e *RadioBrowserEngine) Name() string {
	return "radio"
}

func (e *RadioBrowserEngine) DisplayName() string {
	return "Radio Global"
}

func (e *RadioBrowserEngine) Categories() []models.Category {
	return []models.Category{models.CategoryMusic}
}

func (e *RadioBrowserEngine) DefaultOn() bool {
	return true
}

func (e *RadioBrowserEngine) Weight() float64 {
	return 0.8
}

func (e *RadioBrowserEngine) About() string {
	return "Community-driven database of 40,000+ live radio stations worldwide."
}

func (e *RadioBrowserEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	lowerQ := strings.ToLower(req.Query)
	isRadioQuery := strings.Contains(lowerQ, "radio") || strings.Contains(lowerQ, "fm") || strings.Contains(lowerQ, "stream") || strings.Contains(lowerQ, "broadcast")

	// Only query Radio Browser if query mentions radio, or limit to top 3 so it never drowns out actual songs
	limit := 3
	if isRadioQuery {
		limit = 20
	}

	apiURL := fmt.Sprintf("https://de1.api.radio-browser.info/json/stations/byname/%s?limit=%d", url.QueryEscape(req.Query), limit)

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

		infoParts := []string{"Live Radio Broadcast"}
		if st.Country != "" {
			infoParts = append(infoParts, st.Country)
		}
		if st.Codec != "" {
			infoParts = append(infoParts, fmt.Sprintf("%s %dkbps", strings.ToUpper(st.Codec), st.Bitrate))
		}

		webURL := st.Homepage
		if webURL == "" {
			webURL = st.URLResolved
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("📻 %s (Live Radio)", strings.TrimSpace(st.Name)),
			URL:       webURL,
			PrettyURL: fmt.Sprintf("radio-browser.info • %s", st.Country),
			Content:   strings.Join(infoParts, " • "),
			Engine:    e.Name(),
			Category:  models.CategoryMusic,
			Thumbnail: st.Favicon,
			Author:    st.Country,
			AudioURL:  st.URLResolved,
			Engines:   []string{e.Name()},
			Score:     5.0,
		})
	}

	return results, nil
}
