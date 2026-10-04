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
	Transport: &http.Transport{
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
	},
	Timeout: 30 * time.Second,
}

// ServeProxy streams an m3u8 playlist or TS chunk with CORS headers & rewritten URLs
func ServeProxy(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		// Fallback: extract url directly from RawQuery if unencoded '&' split parameters
		if idx := strings.Index(r.URL.RawQuery, "url="); idx != -1 {
			rawURL = r.URL.RawQuery[idx+4:]
		}
	}
	if rawURL == "" {
		http.Error(w, "url parameter is required", http.StatusBadRequest)
		return
	}

	// Only unescape if it was doubly percent-encoded (e.g. starts with http%3A)
	if strings.HasPrefix(rawURL, "http%3A") || strings.HasPrefix(rawURL, "https%3A") {
		if decodedURL, err := url.QueryUnescape(rawURL); err == nil && strings.HasPrefix(decodedURL, "http") {
			rawURL = decodedURL
		}
	}

	// Unwrap internal proxy hostnames (e.g. *.realcrius.com) to genuine TikTok CDN domains
	rawURL = CleanStreamURL(rawURL)

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

	// Forward client Range header if requested (for seeking or chunked range loading)
	if rangeHdr := r.Header.Get("Range"); rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}

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

	// If upstream returned error (e.g. 403, 404, 500), forward status and body directly without parsing as M3U8
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	isM3U8 := strings.Contains(contentType, "mpegurl") || strings.Contains(contentType, "x-mpegURL") || strings.HasSuffix(parsedTarget.Path, ".m3u8") || strings.Contains(rawURL, ".m3u8")

	if isM3U8 {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(resp.StatusCode)

		base, _ := url.Parse(rawURL)

		// Rewrite lines in M3U8 so segment links route back through the proxy
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// If it's a URI line (not comment)
			if len(trimmed) > 0 && !strings.HasPrefix(trimmed, "#") {
				var segmentURL string
				if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
					// Absolute URL: attach auth params if missing from segment but present in playlist
					if base != nil && base.RawQuery != "" {
						if u, err := url.Parse(trimmed); err == nil {
							if u.RawQuery == "" {
								u.RawQuery = base.RawQuery
								segmentURL = u.String()
							} else if !strings.Contains(u.RawQuery, "expire=") && strings.Contains(base.RawQuery, "expire=") {
								u.RawQuery = u.RawQuery + "&" + base.RawQuery
								segmentURL = u.String()
							} else {
								segmentURL = trimmed
							}
						} else {
							segmentURL = trimmed
						}
					} else {
						segmentURL = trimmed
					}
				} else {
					// Relative URL
					if base != nil {
						rel, err := url.Parse(trimmed)
						if err == nil {
							resolved := base.ResolveReference(rel)
							// Preserve signature query parameters from playlist URL if not in segment
							if resolved.RawQuery == "" && base.RawQuery != "" {
								resolved.RawQuery = base.RawQuery
							} else if base.RawQuery != "" && !strings.Contains(resolved.RawQuery, "expire=") && strings.Contains(base.RawQuery, "expire=") {
								resolved.RawQuery = resolved.RawQuery + "&" + base.RawQuery
							}
							segmentURL = resolved.String()
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

	// Forward range and length headers to client
	for _, h := range []string{"Content-Length", "Content-Range", "Accept-Ranges"} {
		if val := resp.Header.Get(h); val != "" {
			w.Header().Set(h, val)
		}
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
				resolved := base.ResolveReference(rel)
				if resolved.RawQuery == "" && base.RawQuery != "" {
					resolved.RawQuery = base.RawQuery
				} else if base.RawQuery != "" && !strings.Contains(resolved.RawQuery, "expire=") && strings.Contains(base.RawQuery, "expire=") {
					resolved.RawQuery = resolved.RawQuery + "&" + base.RawQuery
				}
				fullURI = resolved.String()
			}
		}
		if fullURI == "" {
			fullURI = uri
		}
	}

	proxiedURI := fmt.Sprintf("/api/tiktoklive/proxy?url=%s", url.QueryEscape(fullURI))
	return line[:start] + proxiedURI + line[start+end:]
}
