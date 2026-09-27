package bypass

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type MirrorLink struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type BypassArticle struct {
	TargetURL    string       `json:"target_url"`
	Domain       string       `json:"domain"`
	Title        string       `json:"title"`
	Author       string       `json:"author,omitempty"`
	Date         string       `json:"date,omitempty"`
	CleanHTML    string       `json:"clean_html"`
	ReadTimeMin  int          `json:"read_time_min"`
	Mirrors      []MirrorLink `json:"mirrors"`
	BypassMethod string       `json:"bypass_method"`
	WordCount    int          `json:"word_count"`
}

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func GenerateMirrors(targetURL string) []MirrorLink {
	encodedURL := url.QueryEscape(targetURL)
	rawURL := targetURL

	u, err := url.Parse(targetURL)
	isMedium := false
	if err == nil && (strings.Contains(u.Host, "medium.com") || strings.Contains(u.Host, "towardsdatascience.com")) {
		isMedium = true
	}

	mirrors := []MirrorLink{
		{
			Name:        "Archive.today",
			URL:         "https://archive.is/newest/" + rawURL,
			Description: "Best for WSJ, NYT, Bloomberg & hard paywalls",
			Icon:        "📜",
		},
		{
			Name:        "Wayback Machine",
			URL:         "https://web.archive.org/web/" + rawURL,
			Description: "Internet Archive historical snapshot",
			Icon:        "🕰️",
		},
		{
			Name:        "12ft Ladder",
			URL:         "https://12ft.io/proxy?q=" + encodedURL,
			Description: "Removes javascript paywall popups & banners",
			Icon:        "🪜",
		},
		{
			Name:        "Google Webcache",
			URL:         "https://webcache.googleusercontent.com/search?q=cache:" + encodedURL,
			Description: "Google's pre-rendered snapshot",
			Icon:        "🔍",
		},
		{
			Name:        "Txtify.it",
			URL:         "https://txtify.it/" + rawURL,
			Description: "Pure stripped text & distraction-free",
			Icon:        "📄",
		},
	}

	if isMedium {
		mirrors = append([]MirrorLink{
			{
				Name:        "Freedium",
				URL:         "https://freedium.cfd/" + rawURL,
				Description: "Unlocks Medium member-only paywalled stories",
				Icon:        "📖",
			},
		}, mirrors...)
	}

	return mirrors
}

var (
	reScript     = regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	reStyle      = regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
	rePaywallDiv = regexp.MustCompile(`(?is)<(?:div|aside|section)[^>]*(?:paywall|subscription|metered|gate|modal|overlay|ad-banner)[^>]*>.*?</(?:div|aside|section)>`)
	reTitle      = regexp.MustCompile(`(?i)<title>(.*?)</title>`)
	reH1         = regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`)
	reMetaAuthor = regexp.MustCompile(`(?i)<meta\s+name=["']author["']\s+content=["'](.*?)["']`)
	reArticle    = regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	reMain       = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)
	reParagraphs = regexp.MustCompile(`(?is)<p[^>]*>.*?</p>`)
)

func (s *Service) ExtractClean(ctx context.Context, targetURL string) (*BypassArticle, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	// Use Googlebot user-agent to bypass metered paywalls
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.google.com/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return nil, err
	}
	rawHTML := string(bodyBytes)

	// Clean out scripts & styles
	cleaned := reScript.ReplaceAllString(rawHTML, "")
	cleaned = reStyle.ReplaceAllString(cleaned, "")
	cleaned = rePaywallDiv.ReplaceAllString(cleaned, "")

	// Extract Title
	title := u.Hostname()
	if matches := reTitle.FindStringSubmatch(rawHTML); len(matches) > 1 {
		title = strings.TrimSpace(matches[1])
	}
	if matches := reH1.FindStringSubmatch(rawHTML); len(matches) > 1 {
		titleCandidate := strings.TrimSpace(stripTags(matches[1]))
		if len(titleCandidate) > 5 {
			title = titleCandidate
		}
	}

	// Extract Author
	author := ""
	if matches := reMetaAuthor.FindStringSubmatch(rawHTML); len(matches) > 1 {
		author = strings.TrimSpace(matches[1])
	}

	// Extract Content (try <article>, then <main>, then all <p>)
	var contentHTML string
	if m := reArticle.FindStringSubmatch(cleaned); len(m) > 1 {
		contentHTML = m[1]
	} else if m := reMain.FindStringSubmatch(cleaned); len(m) > 1 {
		contentHTML = m[1]
	} else {
		// Collect all paragraphs
		paras := reParagraphs.FindAllString(cleaned, -1)
		contentHTML = strings.Join(paras, "\n")
	}

	// Word count & read time
	plainText := stripTags(contentHTML)
	words := len(strings.Fields(plainText))
	readTime := words / 200
	if readTime < 1 {
		readTime = 1
	}

	article := &BypassArticle{
		TargetURL:    targetURL,
		Domain:       u.Hostname(),
		Title:        title,
		Author:       author,
		Date:         time.Now().Format("2006-01-02"),
		CleanHTML:    contentHTML,
		ReadTimeMin:  readTime,
		WordCount:    words,
		Mirrors:      GenerateMirrors(targetURL),
		BypassMethod: "Googlebot Crawl & Clean Parser",
	}

	return article, nil
}

func stripTags(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(html, "")
}
