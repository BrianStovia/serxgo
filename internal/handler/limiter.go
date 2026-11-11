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
	trustedProxies []*net.IPNet
}

func NewRateLimiter(rate float64, burst int, enabled bool, trustedProxies ...[]string) *RateLimiter {
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
	if len(trustedProxies) > 0 {
		for _, raw := range trustedProxies[0] {
			_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
			if err == nil {
				limiter.trustedProxies = append(limiter.trustedProxies, network)
			}
		}
	}

	// Periodic garbage collection for stale IP buckets
	go limiter.cleanupLoop(10 * time.Minute)

	return limiter
}

func (l *RateLimiter) Stop() {
	close(l.stopClean)
}

// Allow evaluates if a client request is within rate limits.
// Returns (allowed, retryAfterDuration).
func (l *RateLimiter) Allow(ip string) (bool, time.Duration) {
	if !l.enabled {
		return true, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	bucket, exists := l.clients[ip]
	if !exists {
		if len(l.clients) >= 100000 {
			return false, time.Minute
		}
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

// ResolveClientIP uses only the TCP peer address. Forwarding headers are
// untrusted unless the server explicitly configures a trusted proxy.
func ResolveClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (l *RateLimiter) clientIP(r *http.Request) string {
	remote := net.ParseIP(ResolveClientIP(r))
	if remote == nil || !l.isTrustedProxy(remote) {
		return ResolveClientIP(r)
	}
	if cf := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); cf != nil {
		return cf.String()
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(parts[i]))
		if candidate != nil && !l.isTrustedProxy(candidate) {
			return candidate.String()
		}
	}
	if realIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); realIP != nil {
		return realIP.String()
	}
	return remote.String()
}

func (l *RateLimiter) isTrustedProxy(ip net.IP) bool {
	for _, network := range l.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
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

		clientIP := l.clientIP(r)
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
