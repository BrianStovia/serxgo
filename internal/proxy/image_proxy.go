package proxy

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ImageProxy struct {
	client *http.Client
}

func NewImageProxy() *ImageProxy {
	safeDialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}

			ip := net.ParseIP(host)
			if ip == nil {
				ips, err := net.LookupIP(host)
				if err != nil || len(ips) == 0 {
					return nil, fmt.Errorf("cannot resolve host: %v", err)
				}
				ip = ips[0]
			}

			if isPrivateIP(ip) {
				return nil, fmt.Errorf("access to private network address is restricted")
			}

			return safeDialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	return &ImageProxy{
		client: &http.Client{
			Transport: transport,
			Timeout:   8 * time.Second,
		},
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, block := range privateBlocks {
		_, subnet, err := net.ParseCIDR(block)
		if err == nil && subnet.Contains(ip) {
			return true
		}
	}
	return false
}

// ServeHTTP handles /proxy/image requests safely
func (p *ImageProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawQuery := r.URL.RawQuery
	var rawTarget string

	// Extract full target URL even if query parameters were unescaped
	if strings.HasPrefix(rawQuery, "url=") {
		rawTarget = strings.TrimPrefix(rawQuery, "url=")
		if unescaped, err := url.QueryUnescape(rawTarget); err == nil && (strings.HasPrefix(unescaped, "http://") || strings.HasPrefix(unescaped, "https://")) {
			rawTarget = unescaped
		}
	} else {
		rawTarget = r.URL.Query().Get("url")
	}

	if rawTarget == "" {
		http.Error(w, "missing image url parameter", http.StatusBadRequest)
		return
	}

	// Support base64 encoded URLs if prefixed with "b64:"
	if strings.HasPrefix(rawTarget, "b64:") {
		decoded, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(rawTarget, "b64:"))
		if err == nil {
			rawTarget = string(decoded)
		}
	}

	parsed, err := url.Parse(rawTarget)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		http.Error(w, "invalid image url", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", rawTarget, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Referer", parsed.Scheme+"://"+parsed.Host+"/")

	resp, err := p.client.Do(req)
	if err != nil {
		http.Error(w, "failed to fetch upstream image", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("upstream returned status %d", resp.StatusCode), resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && !strings.Contains(contentType, "octet-stream") {
		contentType = "image/jpeg"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	_, _ = io.Copy(w, resp.Body)
}
