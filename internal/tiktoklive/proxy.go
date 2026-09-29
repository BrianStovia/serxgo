package tiktoklive

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProxyClient handles fetching CDN chunks with proper Referer & User-Agent
var proxyHttpClient = &http.Client{
	Timeout: 15 * time.Second,
}

// ServeProxy streams an m3u8 playlist or TS chunk with CORS headers & rewritten URLs
func ServeProxy(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, "url parameter is required", http.StatusBadRequest)
		return
	}

	decodedURL, err := url.QueryUnescape(rawURL)
	if err == nil && strings.HasPrefix(decodedURL, "http") {
		rawURL = decodedURL
	}

	// Validate target URL
	parsedTarget, err := url.Parse(rawURL)
	if err != nil || (!strings.HasPrefix(parsedTarget.Scheme, "http")) {
		http.Error(w, "invalid stream url", http.StatusBadRequest)
		return
	}

	// Outgoing request to TikTok CDN
	req, err := http.NewRequestWithContext(r.Context(), "GET", rawURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.tiktok.com/")
	req.Header.Set("Origin", "https://www.tiktok.com")
	req.Header.Set("Accept", "*/*")

	resp, err := proxyHttpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("stream fetch failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// CORS Headers for browser Hls.js
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	contentType := resp.Header.Get("Content-Type")
	isM3U8 := strings.Contains(contentType, "mpegurl") || strings.Contains(contentType, "x-mpegURL") || strings.HasSuffix(parsedTarget.Path, ".m3u8") || strings.Contains(rawURL, ".m3u8")

	if isM3U8 {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(resp.StatusCode)

		// Rewrite lines in M3U8 so segment links route back through the proxy
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// If it's a URI line (not comment)
			if len(trimmed) > 0 && !strings.HasPrefix(trimmed, "#") {
				var segmentURL string
				if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
					segmentURL = trimmed
				} else {
					// Relative URL
					base, err := url.Parse(rawURL)
					if err == nil {
						rel, err := url.Parse(trimmed)
						if err == nil {
							segmentURL = base.ResolveReference(rel).String()
						}
					}
					if segmentURL == "" {
						segmentURL = trimmed
					}
				}
				// Route through proxy
				proxiedSegment := fmt.Sprintf("/api/tiktoklive/proxy?url=%s", url.QueryEscape(segmentURL))
				fmt.Fprintln(w, proxiedSegment)
			} else if strings.HasPrefix(trimmed, "#EXT-X-KEY:") || strings.HasPrefix(trimmed, "#EXT-X-MAP:") {
				// Rewrite URI attribute if present
				rewritten := rewriteURIAttribute(trimmed, rawURL)
				fmt.Fprintln(w, rewritten)
			} else {
				fmt.Fprintln(w, line)
			}
		}
		return
	}

	// For .ts video segments or other binary stream data
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func rewriteURIAttribute(line, baseURL string) string {
	idx := strings.Index(line, `URI="`)
	if idx == -1 {
		return line
	}
	start := idx + 5
	end := strings.Index(line[start:], `"`)
	if end == -1 {
		return line
	}
	uri := line[start : start+end]

	var fullURI string
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		fullURI = uri
	} else {
		base, err := url.Parse(baseURL)
		if err == nil {
			rel, err := url.Parse(uri)
			if err == nil {
				fullURI = base.ResolveReference(rel).String()
			}
		}
		if fullURI == "" {
			fullURI = uri
		}
	}

	proxiedURI := fmt.Sprintf("/api/tiktoklive/proxy?url=%s", url.QueryEscape(fullURI))
	return line[:start] + proxiedURI + line[start+end:]
}
