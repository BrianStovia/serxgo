package newshub

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type NewsArticle struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	SourceIcon  string    `json:"source_icon"`
	ImageURL    string    `json:"image_url,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	TimeAgo     string    `json:"time_ago"`
	Category    string    `json:"category"`
	Region      string    `json:"region"`
}

type FeedSource struct {
	Name     string
	URL      string
	Icon     string
	Category string
	Region   string
}

var FeedSources = []FeedSource{
	// 🌍 World / International
	{Name: "BBC News", URL: "http://feeds.bbci.co.uk/news/world/rss.xml", Icon: "🇬🇧", Category: "world", Region: "world"},
	{Name: "Al Jazeera", URL: "https://www.aljazeera.com/xml/rss/all.xml", Icon: "🌍", Category: "world", Region: "asia"},
	{Name: "The Guardian", URL: "https://www.theguardian.com/world/rss", Icon: "🌐", Category: "world", Region: "europe"},
	{Name: "DW World", URL: "https://rss.dw.com/rdf/rss-en-all", Icon: "🇩🇪", Category: "world", Region: "europe"},
	{Name: "France 24", URL: "https://www.france24.com/en/rss", Icon: "🇫🇷", Category: "world", Region: "europe"},

	// 🇮🇩 Indonesia Terkini
	{Name: "Antara News", URL: "https://www.antaranews.com/rss/terkini.xml", Icon: "🇮🇩", Category: "indonesia", Region: "indonesia"},
	{Name: "CNN Indonesia", URL: "https://www.cnnindonesia.com/nasional/rss", Icon: "🇮🇩", Category: "indonesia", Region: "indonesia"},
	{Name: "Tempo", URL: "https://rss.tempo.co/nasional", Icon: "🇮🇩", Category: "indonesia", Region: "indonesia"},

	// 🇺🇸 US / Americas
	{Name: "NPR News", URL: "https://feeds.npr.org/1001/rss.xml", Icon: "🇺🇸", Category: "world", Region: "us"},
	{Name: "Politico", URL: "https://rss.politico.com/politics-news.xml", Icon: "🏛️", Category: "world", Region: "us"},

	// 🇯🇵 Asia-Pacific
	{Name: "NHK World", URL: "https://www3.nhk.or.jp/rss/news/cat0.xml", Icon: "🇯🇵", Category: "world", Region: "asia"},
	{Name: "SCMP", URL: "https://www.scmp.com/rss/91/feed", Icon: "🇭🇰", Category: "world", Region: "asia"},

	// 🇪🇺 Europe
	{Name: "Euronews", URL: "https://www.euronews.com/rss?format=mrss&level=theme&name=news", Icon: "🇪🇺", Category: "world", Region: "europe"},

	// 💻 Tech & AI
	{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml", Icon: "⚡", Category: "tech", Region: "world"},
	{Name: "TechCrunch", URL: "https://techcrunch.com/feed/", Icon: "🚀", Category: "tech", Region: "world"},
	{Name: "Ars Technica", URL: "https://feeds.arstechnica.com/arstechnica/index", Icon: "💻", Category: "tech", Region: "world"},
	{Name: "Wired", URL: "https://www.wired.com/feed/rss", Icon: "🔌", Category: "tech", Region: "world"},

	// 🛡️ Cybersecurity & Threat Intel
	{Name: "The Hacker News", URL: "https://feeds.feedburner.com/TheHackersNews", Icon: "🛡️", Category: "cyber", Region: "world"},
	{Name: "BleepingComputer", URL: "https://www.bleepingcomputer.com/feed/", Icon: "🔒", Category: "cyber", Region: "world"},

	// 📈 Business & Finance
	{Name: "CNBC Markets", URL: "https://search.cnbc.com/rs/search/view.html?partnerId=2000&keywords=markets&category=news&output=rss", Icon: "📈", Category: "business", Region: "world"},
	{Name: "Yahoo Finance", URL: "https://finance.yahoo.com/news/rssindex", Icon: "💼", Category: "business", Region: "world"},

	// 🪙 Crypto & Web3
	{Name: "CoinDesk", URL: "https://www.coindesk.com/arc/outboundfeeds/rss/", Icon: "🪙", Category: "crypto", Region: "world"},
	{Name: "Cointelegraph", URL: "https://cointelegraph.com/rss", Icon: "⛓️", Category: "crypto", Region: "world"},

	// 🔬 Science & Space
	{Name: "NASA News", URL: "https://www.nasa.gov/news-release/feed/", Icon: "🚀", Category: "science", Region: "world"},
	{Name: "Nature", URL: "https://www.nature.com/nature.rss", Icon: "🔬", Category: "science", Region: "world"},

	// ⚽ Sports
	{Name: "BBC Sport", URL: "http://feeds.bbci.co.uk/sport/rss.xml", Icon: "⚽", Category: "sports", Region: "world"},
	{Name: "ESPN", URL: "https://www.espn.com/espn/rss/news", Icon: "🏆", Category: "sports", Region: "world"},
}

type cachedFeed struct {
	articles  []NewsArticle
	fetchedAt time.Time
}

type Service struct {
	httpClient *http.Client
	cacheMu    sync.RWMutex
	cache      map[string]*cachedFeed
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
		cache: make(map[string]*cachedFeed),
	}
}

// GetNews returns aggregated articles, optionally filtered by category, region, and search keyword
func (s *Service) GetNews(ctx context.Context, category, region, query string) ([]NewsArticle, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		category = "all"
	}
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		region = "all"
	}

	// 1. Check in-memory cache (TTL: 4 minutes)
	cacheKey := category + ":" + region
	s.cacheMu.RLock()
	cached, found := s.cache[cacheKey]
	s.cacheMu.RUnlock()

	var articles []NewsArticle
	if found && time.Since(cached.fetchedAt) < 4*time.Minute {
		articles = cached.articles
	} else {
		// 2. Fetch feeds concurrently
		var targets []FeedSource
		for _, f := range FeedSources {
			matchCat := category == "all" || f.Category == category
			matchReg := region == "all" || f.Region == region
			if matchCat && matchReg {
				targets = append(targets, f)
			}
		}

		// Fallback: if no target matched specific region + category, match by category
		if len(targets) == 0 && region != "all" {
			for _, f := range FeedSources {
				if f.Region == region {
					targets = append(targets, f)
				}
			}
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, feed := range targets {
			wg.Add(1)
			go func(src FeedSource) {
				defer wg.Done()
				items := s.fetchFeed(ctx, src)
				mu.Lock()
				articles = append(articles, items...)
				mu.Unlock()
			}(feed)
		}
		wg.Wait()

		// 3. Sort by PublishedAt descending
		sort.Slice(articles, func(i, j int) bool {
			return articles[i].PublishedAt.After(articles[j].PublishedAt)
		})

		// Format relative time
		now := time.Now()
		for i := range articles {
			articles[i].TimeAgo = formatRelativeTime(now, articles[i].PublishedAt)
		}

		// Save to cache
		s.cacheMu.Lock()
		s.cache[cacheKey] = &cachedFeed{
			articles:  articles,
			fetchedAt: time.Now(),
		}
		s.cacheMu.Unlock()
	}

	// 4. Keyword search filter if requested
	if q := strings.TrimSpace(strings.ToLower(query)); q != "" {
		var filtered []NewsArticle
		for _, a := range articles {
			if strings.Contains(strings.ToLower(a.Title), q) || strings.Contains(strings.ToLower(a.Description), q) || strings.Contains(strings.ToLower(a.Source), q) {
				filtered = append(filtered, a)
			}
		}
		return filtered, nil
	}

	return articles, nil
}

// XML Structs for RSS 2.0 and Atom
type rssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
			Enclosure   struct {
				URL string `xml:"url,attr"`
			} `xml:"enclosure"`
			MediaContent struct {
				URL string `xml:"url,attr"`
			} `xml:"content"`
			MediaThumbnail struct {
				URL string `xml:"url,attr"`
			} `xml:"thumbnail"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Entries []struct {
		Title   string `xml:"title"`
		Link    struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Summary string `xml:"summary"`
		Content string `xml:"content"`
		Updated string `xml:"updated"`
	} `xml:"entry"`
}

var (
	reImgSrc = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	reTags   = regexp.MustCompile(`<[^>]*>`)
)

func (s *Service) fetchFeed(ctx context.Context, src FeedSource) []NewsArticle {
	req, err := http.NewRequestWithContext(ctx, "GET", src.URL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo-NewsHub/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var rss rssFeed
	decoder := xml.NewDecoder(resp.Body)
	decoder.Strict = false
	if err := decoder.Decode(&rss); err == nil && len(rss.Channel.Items) > 0 {
		var results []NewsArticle
		for _, it := range rss.Channel.Items {
			title := cleanText(it.Title)
			link := strings.TrimSpace(it.Link)
			if title == "" || link == "" {
				continue
			}

			img := it.MediaContent.URL
			if img == "" {
				img = it.MediaThumbnail.URL
			}
			if img == "" {
				img = it.Enclosure.URL
			}
			if img == "" {
				if m := reImgSrc.FindStringSubmatch(it.Description); len(m) > 1 {
					img = m[1]
				}
			}

			desc := cleanText(reTags.ReplaceAllString(it.Description, ""))
			if len(desc) > 220 {
				desc = desc[:217] + "..."
			}

			pubTime := parsePubDate(it.PubDate)

			results = append(results, NewsArticle{
				Title:       title,
				Link:        link,
				Description: desc,
				Source:      src.Name,
				SourceIcon:  src.Icon,
				ImageURL:    img,
				PublishedAt: pubTime,
				Category:    src.Category,
				Region:      src.Region,
			})
		}
		return results
	}

	return nil
}

func parsePubDate(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 GMT",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t
		}
	}
	return time.Now()
}

func formatRelativeTime(now, t time.Time) string {
	diff := now.Sub(t)
	if diff < time.Minute {
		return "Just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(diff.Hours() / 24)
	if days == 1 {
		return "Yesterday"
	}
	return fmt.Sprintf("%dd ago", days)
}

func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.TrimSpace(s)
	return s
}
