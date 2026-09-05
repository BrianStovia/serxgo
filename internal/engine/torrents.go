package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"searxgo/internal/models"
)

type PirateBayEngine struct {
	client *http.Client
}

func NewPirateBayEngine() *PirateBayEngine {
	return &PirateBayEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *PirateBayEngine) Name() string {
	return "thepiratebay"
}

func (e *PirateBayEngine) DisplayName() string {
	return "The Pirate Bay"
}

func (e *PirateBayEngine) Categories() []models.Category {
	return []models.Category{models.CategoryFiles}
}

func (e *PirateBayEngine) DefaultOn() bool {
	return true
}

func (e *PirateBayEngine) Weight() float64 {
	return 1.3
}

func (e *PirateBayEngine) About() string {
	return "Index of digital content of entertainment media and software torrents."
}

func (e *PirateBayEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://apibay.org/q.php?q=%s", url.QueryEscape(req.Query))

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
		return nil, fmt.Errorf("apibay returned status %d", resp.StatusCode)
	}

	var items []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		InfoHash string `json:"info_hash"`
		Leechers string `json:"leechers"`
		Seeders  string `json:"seeders"`
		Size     string `json:"size"`
		Username string `json:"username"`
		Added    string `json:"added"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, it := range items {
		if it.Name == "" || it.Name == "No results returned" || it.InfoHash == "" {
			continue
		}

		seeders, _ := strconv.Atoi(it.Seeders)
		leechers, _ := strconv.Atoi(it.Leechers)
		sizeBytes, _ := strconv.ParseInt(it.Size, 10, 64)

		sizeStr := formatBytes(sizeBytes)
		magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", it.InfoHash, url.QueryEscape(it.Name))
		targetURL := fmt.Sprintf("https://thepiratebay.org/description.php?id=%s", it.ID)

		content := fmt.Sprintf("Size: %s | Seeders: %d | Leechers: %d | Uploaded by: %s",
			sizeStr, seeders, leechers, it.Username)

		extra := map[string]string{
			"Seeders":  it.Seeders,
			"Leechers": it.Leechers,
			"Size":     sizeStr,
			"Magnet":   magnetURL,
		}

		results = append(results, models.SearchResult{
			Title:     it.Name,
			URL:       targetURL,
			PrettyURL: fmt.Sprintf("thepiratebay.org/id/%s", it.ID),
			Content:   content,
			Engine:    e.Name(),
			Category:  models.CategoryFiles,
			MagnetURL: magnetURL,
			Seeders:   seeders,
			Leechers:  leechers,
			FileSize:  sizeStr,
			Author:    it.Username,
			Extra:     extra,
		})
	}

	return results, nil
}

// InternetArchiveEngine searches archive.org digital files
type InternetArchiveEngine struct {
	client *http.Client
}

func NewInternetArchiveEngine() *InternetArchiveEngine {
	return &InternetArchiveEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *InternetArchiveEngine) Name() string {
	return "archiveorg"
}

func (e *InternetArchiveEngine) DisplayName() string {
	return "Internet Archive"
}

func (e *InternetArchiveEngine) Categories() []models.Category {
	return []models.Category{models.CategoryFiles}
}

func (e *InternetArchiveEngine) DefaultOn() bool {
	return true
}

func (e *InternetArchiveEngine) Weight() float64 {
	return 1.1
}

func (e *InternetArchiveEngine) About() string {
	return "Non-profit digital library of free books, movies, software, music, and websites."
}

func (e *InternetArchiveEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://archive.org/advancedsearch.php?q=%s&fl[]=identifier,title,description,mediatype,publicdate&rows=10&output=json",
		url.QueryEscape(req.Query))

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

	var data struct {
		Response struct {
			Docs []struct {
				Identifier  string `json:"identifier"`
				Title       string `json:"title"`
				Description string `json:"description"`
				MediaType   string `json:"mediatype"`
				PublicDate  string `json:"publicdate"`
			} `json:"docs"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, doc := range data.Response.Docs {
		if doc.Identifier == "" {
			continue
		}

		title := doc.Title
		if title == "" {
			title = doc.Identifier
		}

		targetURL := fmt.Sprintf("https://archive.org/details/%s", doc.Identifier)
		thumb := fmt.Sprintf("https://archive.org/services/img/%s", doc.Identifier)

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       targetURL,
			PrettyURL: fmt.Sprintf("archive.org/details/%s", doc.Identifier),
			Content:   doc.Description,
			Engine:    e.Name(),
			Category:  models.CategoryFiles,
			Thumbnail: thumb,
		})
	}

	return results, nil
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "Unknown"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
