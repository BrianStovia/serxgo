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

// NPMEngine searches the NPM JavaScript package ecosystem
type NPMEngine struct {
	client *http.Client
}

func NewNPMEngine() *NPMEngine {
	return &NPMEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *NPMEngine) Name() string {
	return "npm"
}

func (e *NPMEngine) DisplayName() string {
	return "NPM"
}

func (e *NPMEngine) Categories() []models.Category {
	return []models.Category{models.CategoryIT}
}

func (e *NPMEngine) DefaultOn() bool {
	return true
}

func (e *NPMEngine) Weight() float64 {
	return 1.1
}

func (e *NPMEngine) About() string {
	return "Package manager for JavaScript and the Node.js runtime environment."
}

func (e *NPMEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	apiURL := fmt.Sprintf("https://registry.npmjs.org/-/v1/search?text=%s&size=10", url.QueryEscape(req.Query))
	if req.Page > 1 {
		apiURL += fmt.Sprintf("&from=%d", (req.Page-1)*10)
	}

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
		return nil, fmt.Errorf("npm api returned status %d", resp.StatusCode)
	}

	var data struct {
		Objects []struct {
			Package struct {
				Name        string `json:"name"`
				Version     string `json:"version"`
				Description string `json:"description"`
				Links       struct {
					NPM        string `json:"npm"`
					Repository string `json:"repository"`
				} `json:"links"`
				Publisher struct {
					Username string `json:"username"`
				} `json:"publisher"`
				Date time.Time `json:"date"`
			} `json:"package"`
		} `json:"objects"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, obj := range data.Objects {
		pkg := obj.Package
		if pkg.Name == "" {
			continue
		}

		targetURL := pkg.Links.NPM
		if targetURL == "" {
			targetURL = fmt.Sprintf("https://www.npmjs.com/package/%s", pkg.Name)
		}

		extra := map[string]string{
			"Version":   pkg.Version,
			"Publisher": pkg.Publisher.Username,
		}

		results = append(results, models.SearchResult{
			Title:         fmt.Sprintf("%s (v%s)", pkg.Name, pkg.Version),
			URL:           targetURL,
			PrettyURL:     fmt.Sprintf("npmjs.com/package/%s", pkg.Name),
			Content:       pkg.Description,
			Engine:        e.Name(),
			Category:      models.CategoryIT,
			PublishedDate: &pkg.Date,
			Author:        pkg.Publisher.Username,
			Extra:         extra,
		})
	}

	return results, nil
}

// PyPIEngine searches Python packages
type PyPIEngine struct {
	client *http.Client
}

func NewPyPIEngine() *PyPIEngine {
	return &PyPIEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *PyPIEngine) Name() string {
	return "pypi"
}

func (e *PyPIEngine) DisplayName() string {
	return "PyPI"
}

func (e *PyPIEngine) Categories() []models.Category {
	return []models.Category{models.CategoryIT}
}

func (e *PyPIEngine) DefaultOn() bool {
	return true
}

func (e *PyPIEngine) Weight() float64 {
	return 1.1
}

func (e *PyPIEngine) About() string {
	return "Official package repository for the Python programming language."
}

func (e *PyPIEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	// Directly check if query looks like a specific package
	cleanPkg := url.PathEscape(req.Query)
	apiURL := fmt.Sprintf("https://pypi.org/pypi/%s/json", cleanPkg)

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
		return nil, nil // Not found or error
	}

	var data struct {
		Info struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Summary     string `json:"summary"`
			PackageURL  string `json:"package_url"`
			ProjectURL  string `json:"project_url"`
			Author      string `json:"author"`
			License     string `json:"license"`
		} `json:"info"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	extra := map[string]string{
		"Version": data.Info.Version,
		"License": data.Info.License,
	}

	return []models.SearchResult{
		{
			Title:     fmt.Sprintf("%s (v%s)", data.Info.Name, data.Info.Version),
			URL:       data.Info.PackageURL,
			PrettyURL: fmt.Sprintf("pypi.org/project/%s", data.Info.Name),
			Content:   data.Info.Summary,
			Engine:    e.Name(),
			Category:  models.CategoryIT,
			Author:    data.Info.Author,
			Extra:     extra,
		},
	}, nil
}
