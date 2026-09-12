package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

var (
	tmeChannelMsgRegex = regexp.MustCompile(`t\.me\/(?:s\/)?([a-zA-Z0-9_]{3,32})\/([0-9]+)`)
	tmeChannelOnlyRegex = regexp.MustCompile(`t\.me\/(?:s\/)?([a-zA-Z0-9_]{3,32})`)
	tgstatChannelRegex  = regexp.MustCompile(`tgstat\.(?:com|ru)\/channel\/@?([a-zA-Z0-9_]{3,32})`)
	telemetrRegex       = regexp.MustCompile(`telemetr\.(?:io|me)\/(?:en\/)?channels\/([a-zA-Z0-9_]{3,32})`)
)

type TelegramLeaksEngine struct {
	client *http.Client
}

func NewTelegramLeaksEngine() *TelegramLeaksEngine {
	return &TelegramLeaksEngine{
		client: NewHTTPClient(4 * time.Second),
	}
}

func (e *TelegramLeaksEngine) Name() string {
	return "telegram_leaks"
}

func (e *TelegramLeaksEngine) DisplayName() string {
	return "Telegram OSINT & Public Dumps"
}

func (e *TelegramLeaksEngine) Categories() []models.Category {
	return []models.Category{models.CategorySocial, models.CategoryGeneral, models.CategoryIT, models.CategoryFiles}
}

func (e *TelegramLeaksEngine) DefaultOn() bool {
	return true
}

func (e *TelegramLeaksEngine) Weight() float64 {
	return 1.35
}

func (e *TelegramLeaksEngine) About() string {
	return "Deep Telegram OSINT & Leaks crawler: searches public t.me channels, message dumps, TGStat, and Telemetr intelligence indexes."
}

func (e *TelegramLeaksEngine) Search(ctx context.Context, req models.SearchRequest) ([]models.SearchResult, error) {
	cleanQ := strings.TrimSpace(req.Query)
	// Remove redundant bangs or prefixes
	cleanQ = strings.TrimPrefix(cleanQ, "!tg ")
	cleanQ = strings.TrimPrefix(cleanQ, "!telegram ")
	cleanQ = strings.TrimPrefix(cleanQ, "!tgdump ")
	cleanQ = strings.TrimPrefix(cleanQ, "tg: ")
	cleanQ = strings.TrimPrefix(cleanQ, "telegram: ")

	queryWithSites := fmt.Sprintf("(site:t.me/s/ OR site:tgstat.com OR site:telemetr.io OR site:lyzem.com OR site:t.me) %s", cleanQ)

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(queryWithSites))

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
		return nil, fmt.Errorf("telegram osint returned status %d", resp.StatusCode)
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

		// Intelligent classification & tagging of Telegram artifacts
		displayTitle := title
		author := ""
		cluster := "social"

		if matches := tmeChannelMsgRegex.FindStringSubmatch(actualURL); len(matches) > 2 {
			channel := matches[1]
			msgID := matches[2]
			author = "@" + channel
			displayTitle = fmt.Sprintf("💬 [@%s #%s] %s", channel, msgID, title)
			cluster = "discussions"
		} else if matches := tmeChannelOnlyRegex.FindStringSubmatch(actualURL); len(matches) > 1 {
			channel := matches[1]
			author = "@" + channel
			displayTitle = fmt.Sprintf("📱 [@%s] %s", channel, title)
			cluster = "social"
		} else if matches := tgstatChannelRegex.FindStringSubmatch(actualURL); len(matches) > 1 {
			channel := matches[1]
			author = "@" + channel
			displayTitle = fmt.Sprintf("📊 [TGStat Analytics @%s] %s", channel, title)
			cluster = "tools"
		} else if matches := telemetrRegex.FindStringSubmatch(actualURL); len(matches) > 1 {
			channel := matches[1]
			author = "@" + channel
			displayTitle = fmt.Sprintf("📈 [Telemetr Intel @%s] %s", channel, title)
			cluster = "tools"
		} else {
			displayTitle = fmt.Sprintf("🔍 [Telegram OSINT] %s", title)
		}

		results = append(results, models.SearchResult{
			Title:     displayTitle,
			URL:       actualURL,
			PrettyURL: cleanDisplayURL(actualURL),
			Content:   desc,
			Author:    author,
			Engine:    e.Name(),
			Category:  models.CategorySocial,
			Clusters:  cluster,
		})
	}

	return results, nil
}

