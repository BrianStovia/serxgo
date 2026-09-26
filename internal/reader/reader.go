package reader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Article represents cleaned, ad-free extracted article content
type Article struct {
	URL           string `json:"url"`
	Domain        string `json:"domain"`
	Title         string `json:"title"`
	Author        string `json:"author,omitempty"`
	SiteName      string `json:"site_name,omitempty"`
	PublishedDate string `json:"published_date,omitempty"`
	ReadTimeMin   int    `json:"read_time_min"`
	WordCount     int    `json:"word_count"`
	Excerpt       string `json:"excerpt"`
	ContentHTML   string `json:"content_html"`
}

// ExtractArticle fetches target web page and parses clean distraction-free article text
func ExtractArticle(ctx context.Context, targetURL string) (*Article, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || !strings.HasPrefix(parsedURL.Scheme, "http") {
		return nil, fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server responded with status %d", resp.StatusCode)
	}

	// Limit read to 5 MB
	bodyReader := io.LimitReader(resp.Body, 5*1024*1024)
	doc, err := html.Parse(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	article := &Article{
		URL:    targetURL,
		Domain: parsedURL.Hostname(),
	}

	// 1. Extract metadata from <head>
	extractMeta(doc, article)

	// 2. Extract clean body content
	extractContent(doc, article)

	// Calculate word count & reading time
	words := len(strings.Fields(stripHTMLTags(article.ContentHTML)))
	article.WordCount = words
	article.ReadTimeMin = int(math.Ceil(float64(words) / 200.0))
	if article.ReadTimeMin < 1 {
		article.ReadTimeMin = 1
	}

	// Create Excerpt if empty
	if article.Excerpt == "" {
		plain := stripHTMLTags(article.ContentHTML)
		if len(plain) > 220 {
			article.Excerpt = plain[:217] + "..."
		} else {
			article.Excerpt = plain
		}
	}

	return article, nil
}

func extractMeta(n *html.Node, a *Article) {
	if n.Type == html.ElementNode {
		if n.Data == "title" && a.Title == "" && n.FirstChild != nil {
			a.Title = strings.TrimSpace(n.FirstChild.Data)
		}
		if n.Data == "meta" {
			var prop, name, content string
			for _, attr := range n.Attr {
				switch strings.ToLower(attr.Key) {
				case "property":
					prop = attr.Val
				case "name":
					name = attr.Val
				case "content":
					content = attr.Val
				}
			}

			key := strings.ToLower(prop)
			if key == "" {
				key = strings.ToLower(name)
			}

			switch key {
			case "og:title", "twitter:title":
				if a.Title == "" { a.Title = content }
			case "og:site_name":
				a.SiteName = content
			case "author", "article:author":
				if a.Author == "" { a.Author = content }
			case "article:published_time", "date":
				if a.PublishedDate == "" { a.PublishedDate = content }
			case "description", "og:description":
				if a.Excerpt == "" { a.Excerpt = content }
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractMeta(c, a)
	}
}

func extractContent(root *html.Node, a *Article) {
	var bodyNode *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			bodyNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(root)

	if bodyNode == nil {
		bodyNode = root
	}

	// Find main container: <article>, <main>, or body
	var targetContainer *html.Node
	var findArticle func(*html.Node)
	findArticle = func(n *html.Node) {
		if targetContainer != nil {
			return
		}
		if n.Type == html.ElementNode {
			if n.Data == "article" || n.Data == "main" {
				targetContainer = n
				return
			}
			for _, attr := range n.Attr {
				if attr.Key == "class" || attr.Key == "id" {
					val := strings.ToLower(attr.Val)
					if strings.Contains(val, "article-content") || strings.Contains(val, "post-content") || strings.Contains(val, "entry-content") {
						targetContainer = n
						return
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findArticle(c)
		}
	}
	findArticle(bodyNode)

	if targetContainer == nil {
		targetContainer = bodyNode
	}

	var buf bytes.Buffer
	renderCleanNode(&buf, targetContainer)
	a.ContentHTML = buf.String()
}

func renderCleanNode(buf *bytes.Buffer, n *html.Node) {
	if n.Type == html.ElementNode {
		tag := strings.ToLower(n.Data)

		// Discard unwanted tags
		disallowed := map[string]bool{
			"script": true, "style": true, "noscript": true, "iframe": true,
			"nav": true, "footer": true, "header": true, "aside": true,
			"form": true, "input": true, "button": true, "select": true,
			"svg": true, "canvas": true, "dialog": true,
		}
		if disallowed[tag] {
			return
		}

		// Discard classes/ids matching ads or cookie bars
		for _, attr := range n.Attr {
			if attr.Key == "class" || attr.Key == "id" {
				val := strings.ToLower(attr.Val)
				if strings.Contains(val, "cookie") || strings.Contains(val, "banner") ||
					strings.Contains(val, "advert") || strings.Contains(val, "social-share") ||
					strings.Contains(val, "newsletter") || strings.Contains(val, "popup") {
					return
				}
			}
		}

		allowedTags := map[string]bool{
			"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
			"blockquote": true, "pre": true, "code": true, "ul": true, "ol": true, "li": true,
			"b": true, "strong": true, "i": true, "em": true, "u": true, "a": true, "img": true,
			"table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
			"hr": true, "figure": true, "figcaption": true,
		}

		if allowedTags[tag] {
			buf.WriteString("<" + tag)
			// Retain safe attributes
			for _, attr := range n.Attr {
				k := strings.ToLower(attr.Key)
				if tag == "a" && k == "href" && strings.HasPrefix(attr.Val, "http") {
					buf.WriteString(fmt.Sprintf(` href="%s" target="_blank" rel="noopener noreferrer"`, html.EscapeString(attr.Val)))
				}
				if tag == "img" && (k == "src" || k == "data-src") && strings.HasPrefix(attr.Val, "http") {
					buf.WriteString(fmt.Sprintf(` src="%s" loading="lazy" style="max-width:100%%; border-radius:8px; margin:1rem 0;"`, html.EscapeString(attr.Val)))
				}
				if tag == "img" && k == "alt" {
					buf.WriteString(fmt.Sprintf(` alt="%s"`, html.EscapeString(attr.Val)))
				}
			}
			buf.WriteString(">")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderCleanNode(buf, c)
		}

		if allowedTags[tag] && tag != "img" && tag != "hr" {
			buf.WriteString("</" + tag + ">\n")
		}
		return
	}

	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			buf.WriteString(html.EscapeString(n.Data))
		}
	}
}

var tagRegexp = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return tagRegexp.ReplaceAllString(s, " ")
}
