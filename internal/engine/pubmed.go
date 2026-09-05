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

type PubMedEngine struct {
	client *http.Client
}

func NewPubMedEngine() *PubMedEngine {
	return &PubMedEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *PubMedEngine) Name() string {
	return "pubmed"
}

func (e *PubMedEngine) DisplayName() string {
	return "PubMed"
}

func (e *PubMedEngine) Categories() []models.Category {
	return []models.Category{models.CategoryScience, models.CategoryGeneral}
}

func (e *PubMedEngine) DefaultOn() bool {
	return true
}

func (e *PubMedEngine) Weight() float64 {
	return 1.2
}

func (e *PubMedEngine) About() string {
	return "Free search engine accessing primarily the MEDLINE database of life sciences and biomedical topics."
}

func (e *PubMedEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	// 1. Search for PMIDs
	retstart := 0
	if req.Page > 1 {
		retstart = (req.Page - 1) * 8
	}
	esearchURL := fmt.Sprintf("https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi?db=pubmed&term=%s&retmode=json&retmax=8&retstart=%d",
		url.QueryEscape(req.Query), retstart)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", esearchURL, nil)
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
		return nil, fmt.Errorf("pubmed esearch returned status %d", resp.StatusCode)
	}

	var searchData struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return nil, err
	}

	if len(searchData.ESearchResult.IDList) == 0 {
		return nil, nil
	}

	// 2. Fetch summaries for PMIDs
	idParam := strings.Join(searchData.ESearchResult.IDList, ",")
	summaryURL := fmt.Sprintf("https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi?db=pubmed&id=%s&retmode=json", idParam)

	sumReq, err := http.NewRequestWithContext(ctx, "GET", summaryURL, nil)
	if err != nil {
		return nil, err
	}
	sumReq.Header.Set("User-Agent", "SearXGo/1.0")

	sumResp, err := e.client.Do(sumReq)
	if err != nil {
		return nil, err
	}
	defer sumResp.Body.Close()

	var sumData struct {
		Result map[string]interface{} `json:"result"`
	}

	if err := json.NewDecoder(sumResp.Body).Decode(&sumData); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, id := range searchData.ESearchResult.IDList {
		rawItem, exists := sumData.Result[id]
		if !exists {
			continue
		}

		itemMap, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}

		title, _ := itemMap["title"].(string)
		source, _ := itemMap["source"].(string)
		pubDate, _ := itemMap["pubdate"].(string)

		if title == "" {
			continue
		}

		cleanTitle := CleanHTMLText(title)
		cleanTitle = strings.TrimSuffix(cleanTitle, ".")

		articleURL := fmt.Sprintf("https://pubmed.ncbi.nlm.nih.gov/%s/", id)
		content := fmt.Sprintf("Journal: %s | Published: %s", source, pubDate)

		extra := map[string]string{
			"PMID":    id,
			"Journal": source,
		}

		results = append(results, models.SearchResult{
			Title:     cleanTitle,
			URL:       articleURL,
			PrettyURL: fmt.Sprintf("pubmed.ncbi.nlm.nih.gov/%s", id),
			Content:   content,
			Engine:    e.Name(),
			Category:  models.CategoryScience,
			Extra:     extra,
		})
	}

	return results, nil
}
