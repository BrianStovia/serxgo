package engine

import (
	"crypto/tls"
	"html"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"searxgo/internal/models"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
}

// GetRandomUserAgent returns a modern desktop browser user-agent
func GetRandomUserAgent() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return userAgents[r.Intn(len(userAgents))]
}

// NewHTTPClient returns a privacy-preserving http.Client with connection pooling and timeouts
func NewHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		DisableKeepAlives: false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// CleanHTMLText unescapes HTML entities, removes consecutive whitespaces and trims
func CleanHTMLText(s string) string {
	s = html.UnescapeString(s)
	// Replace line breaks and tabs with space
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// ExtractText recursively retrieves all text inside an HTML node
func ExtractText(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == xhtml.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		// Skip script and style tags
		if c.Type == xhtml.ElementNode && (c.Data == "script" || c.Data == "style" || c.Data == "noscript") {
			continue
		}
		sb.WriteString(ExtractText(c))
	}
	return sb.String()
}

// GetAttr returns attribute value for a key in node
func GetAttr(n *xhtml.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

// HasClass checks if an HTML node contains a specific CSS class
func HasClass(n *xhtml.Node, className string) bool {
	if n == nil {
		return false
	}
	classVal := GetAttr(n, "class")
	classes := strings.Fields(classVal)
	for _, c := range classes {
		if c == className {
			return true
		}
	}
	return false
}

// FindNodes searches subtree for nodes matching tag and optional class
func FindNodes(n *xhtml.Node, tag string, className string) []*xhtml.Node {
	var results []*xhtml.Node
	if n == nil {
		return results
	}

	var walk func(*xhtml.Node)
	walk = func(curr *xhtml.Node) {
		if curr.Type == xhtml.ElementNode {
			tagMatch := (tag == "" || strings.EqualFold(curr.Data, tag))
			classMatch := (className == "" || HasClass(curr, className))
			if tagMatch && classMatch {
				results = append(results, curr)
			}
		}
		for c := curr.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(n)
	return results
}

// FindFirstNode returns the first matching node
func FindFirstNode(n *xhtml.Node, tag string, className string) *xhtml.Node {
	if n == nil {
		return nil
	}
	if n.Type == xhtml.ElementNode {
		tagMatch := (tag == "" || strings.EqualFold(currTag(n), tag))
		classMatch := (className == "" || HasClass(n, className))
		if tagMatch && classMatch {
			return n
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := FindFirstNode(c, tag, className); found != nil {
			return found
		}
	}
	return nil
}

func currTag(n *xhtml.Node) string {
	if n != nil && n.Type == xhtml.ElementNode {
		return n.Data
	}
	return ""
}

// cleanDisplayURL produces a human-friendly pretty display URL
func cleanDisplayURL(rawURL string) string {
	u, err := netURLParse(rawURL)
	if err != nil {
		return rawURL
	}
	res := u.Host + u.Path
	if strings.HasSuffix(res, "/") {
		res = strings.TrimSuffix(res, "/")
	}
	return res
}

func netURLParse(raw string) (*netURL, error) {
	// Parse helper
	idx := strings.Index(raw, "://")
	if idx != -1 {
		raw = raw[idx+3:]
	}
	slashIdx := strings.Index(raw, "/")
	if slashIdx == -1 {
		return &netURL{Host: raw, Path: ""}, nil
	}
	return &netURL{Host: raw[:slashIdx], Path: raw[slashIdx:]}, nil
}

type netURL struct {
	Host string
	Path string
}

// parseDuckDuckGoHTML extracts search results from DuckDuckGo HTML output
func parseDuckDuckGoHTML(htmlContent string) []models.SearchResult {
	doc, err := xhtml.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}

	var results []models.SearchResult
	resultNodes := FindNodes(doc, "div", "result")

	for _, node := range resultNodes {
		if HasClass(node, "result--ad") || HasClass(node, "result--no-result") {
			continue
		}

		titleNode := FindFirstNode(node, "a", "result__a")
		if titleNode == nil {
			titleNode = FindFirstNode(node, "a", "result__url")
		}
		if titleNode == nil {
			continue
		}

		rawURL := GetAttr(titleNode, "href")
		cleanURL := extractDuckDuckGoURL(rawURL)
		if cleanURL == "" || strings.HasPrefix(cleanURL, "/") {
			continue
		}

		title := CleanHTMLText(ExtractText(titleNode))
		if title == "" {
			continue
		}

		snippetNode := FindFirstNode(node, "a", "result__snippet")
		snippet := ""
		if snippetNode != nil {
			snippet = CleanHTMLText(ExtractText(snippetNode))
		}

		results = append(results, models.SearchResult{
			Title:     title,
			URL:       cleanURL,
			PrettyURL: cleanDisplayURL(cleanURL),
			Content:   snippet,
		})
	}

	return results
}

