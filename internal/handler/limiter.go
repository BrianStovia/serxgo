package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientBucket struct {
	tokens    float64
	lastCheck time.Time
}

// RateLimiter provides thread-safe in-memory token bucket rate limiting matching SearXNG limiter.py
type RateLimiter struct {
	mu        sync.Mutex
	rate      float64 // tokens added per second
	burst     int     // maximum token capacity
	enabled   bool
	clients   map[string]*clientBucket
	stopClean chan struct{}
}

func NewRateLimiter(rate float64, burst int, enabled bool) *RateLimiter {
	if rate <= 0 {
		rate = 20.0
	}
	if burst <= 0 {
		burst = 40
	}

	limiter := &RateLimiter{
		rate:      rate,
		burst:     burst,
		enabled:   enabled,
		clients:   make(map[string]*clientBucket),
		stopClean: make(chan struct{}),
	}

	// Periodic garbage collection for stale IP buckets
	go limiter.cleanupLoop(10 * time.Minute)

	return limiter
}

func (l *RateLimiter) Stop() {
	close(l.stopClean)
}

func (l *RateLimiter) isWhitelisted(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	// Bypass loopback (127.0.0.1, ::1) and local private networks
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	return false
}

// Allow evaluates if a client request is within rate limits.
// Returns (allowed, retryAfterDuration).
func (l *RateLimiter) Allow(ip string) (bool, time.Duration) {
	if !l.enabled {
		return true, 0
	}

	if l.isWhitelisted(ip) {
		return true, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	bucket, exists := l.clients[ip]
	if !exists {
		l.clients[ip] = &clientBucket{
			tokens:    float64(l.burst) - 1.0,
			lastCheck: now,
		}
		return true, 0
	}

	// Replenish tokens based on elapsed duration
	elapsed := now.Sub(bucket.lastCheck).Seconds()
	bucket.tokens = math.Min(float64(l.burst), bucket.tokens+(elapsed*l.rate))
	bucket.lastCheck = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true, 0
	}

	// Calculate wait time until 1 token is available
	missing := 1.0 - bucket.tokens
	waitTime := time.Duration((missing / l.rate) * float64(time.Second))
	if waitTime < time.Second {
		waitTime = time.Second
	}
	return false, waitTime
}

func (l *RateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			threshold := time.Now().Add(-15 * time.Minute)
			for ip, b := range l.clients {
				if b.lastCheck.Before(threshold) {
					delete(l.clients, ip)
				}
			}
			l.mu.Unlock()
		case <-l.stopClean:
			return
		}
	}
}

// ResolveClientIP extracts real client IP considering reverse proxy headers
func ResolveClientIP(r *http.Request) string {
	// Cloudflare
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return strings.TrimSpace(cf)
	}
	// Standard proxy
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// Middleware wraps an http.Handler with rate limiting and returns HTTP 429 when throttled
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Static assets and health checks are always exempted
		path := r.URL.Path
		if strings.HasPrefix(path, "/static/") || path == "/healthz" || path == "/health" || path == "/favicon.ico" {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := ResolveClientIP(r)
		allowed, retryAfter := l.Allow(clientIP)
		if !allowed {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", l.rate))

			if strings.HasPrefix(path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "Too Many Requests",
					"message":     "Rate limit exceeded. Please wait before making more requests.",
					"retry_after": seconds,
				})
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>429 - Too Many Requests | SearXGo</title>
  <style>
    body { font-family: system-ui, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
    .card { background: #1e293b; border: 1px solid #334155; padding: 2.5rem; border-radius: 1rem; max-width: 460px; text-align: center; box-shadow: 0 10px 25px rgba(0,0,0,0.5); }
    h1 { color: #f43f5e; margin: 0 0 1rem; font-size: 2rem; }
    p { color: #94a3b8; line-height: 1.6; margin-bottom: 1.5rem; }
    a { display: inline-block; background: #6366f1; color: #fff; padding: 0.75rem 1.5rem; border-radius: 0.5rem; text-decoration: none; font-weight: 500; }
    a:hover { background: #4f46e5; }
  </style>
</head>
<body>
  <div class="card">
    <h1>429 - Rate Limited</h1>
    <p>You have made too many search requests in a short period. Please wait <strong>%d second(s)</strong> before trying again.</p>
    <a href="/">Return Home</a>
  </div>
</body>
</html>`, seconds)
			return
		}

		next.ServeHTTP(w, r)
	})
}
