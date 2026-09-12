package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

type Web3Engine struct {
	client *http.Client
}

func NewWeb3Engine() *Web3Engine {
	return &Web3Engine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *Web3Engine) Name() string {
	return "web3"
}

func (e *Web3Engine) DisplayName() string {
	return "Web3 & IPFS / Arweave"
}

func (e *Web3Engine) Categories() []models.Category {
	return []models.Category{models.CategoryFiles, models.CategoryGeneral, models.CategoryIT}
}

func (e *Web3Engine) DefaultOn() bool {
	return true
}

func (e *Web3Engine) Weight() float64 {
	return 1.2
}

func (e *Web3Engine) About() string {
	return "Aggregates decentralized, immutable content from IPFS, Arweave, and Nostr protocols."
}

func (e *Web3Engine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	cleanQ := strings.TrimSpace(req.Query)
	web3Query := fmt.Sprintf("%s (site:ipfs.io OR site:dweb.link OR site:gateway.pinata.cloud OR site:arweave.net OR site:viewblock.io OR site:nostr.band)", cleanQ)

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(web3Query))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", GetRandomUserAgent())
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("web3 engine returned status %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	resultsNodes := FindNodes(doc, "div", "result")
	if len(resultsNodes) == 0 {
		resultsNodes = FindNodes(doc, "div", "results_links")
	}

	for _, node := range resultsNodes {
		linkNode := FindFirstNode(node, "a", "result__url")
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "result__snippet")
		}
		if linkNode == nil {
			linkNode = FindFirstNode(node, "a", "")
		}
		if linkNode == nil {
			continue
		}

		href := GetAttr(linkNode, "href")
		if href == "" || strings.HasPrefix(href, "/") || strings.Contains(href, "duckduckgo.com") {
			continue
		}

		actualURL := extractDuckDuckGoURL(href)
		if actualURL == "" {
			actualURL = href
		}

		titleNode := FindFirstNode(node, "h2", "")
		if titleNode == nil {
			titleNode = linkNode
		}
		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		descNode := FindFirstNode(node, "a", "result__snippet")
		if descNode == nil {
			descNode = FindFirstNode(node, "div", "result__snippet")
		}
		desc := ""
		if descNode != nil {
			desc = CleanHTMLText(ExtractText(descNode))
		}

		results = append(results, models.SearchResult{
			Title:     fmt.Sprintf("[Web3/P2P] %s", title),
			URL:       actualURL,
			PrettyURL: cleanDisplayURL(actualURL),
			Content:   desc,
			Engine:    e.Name(),
			Category:  models.CategoryFiles,
		})
	}

	return results, nil
}
