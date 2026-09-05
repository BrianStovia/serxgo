package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter(t *testing.T) {
	// 2 requests per second, burst 2
	limiter := NewRateLimiter(2.0, 2, true)
	defer limiter.Stop()

	// 1. Whitelisted loopback IP should always be allowed
	if allowed, _ := limiter.Allow("127.0.0.1"); !allowed {
		t.Errorf("expected 127.0.0.1 to be allowed unconditionally")
	}

	// 2. Public IP
	publicIP := "203.0.113.195"

	// First 2 requests should be allowed (burst = 2)
	if allowed, _ := limiter.Allow(publicIP); !allowed {
		t.Errorf("expected 1st request from %s to be allowed", publicIP)
	}
	if allowed, _ := limiter.Allow(publicIP); !allowed {
		t.Errorf("expected 2nd request from %s to be allowed", publicIP)
	}

	// 3rd request immediately should be throttled
	allowed, waitTime := limiter.Allow(publicIP)
	if allowed {
		t.Errorf("expected 3rd immediate request from %s to be throttled", publicIP)
	}
	if waitTime <= 0 {
		t.Errorf("expected positive wait time, got %v", waitTime)
	}

	// Middleware test
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := limiter.Middleware(nextHandler)

	// Test request with throttled public IP header
	req := httptest.NewRequest("GET", "/search?q=test", nil)
	req.Header.Set("X-Real-IP", publicIP)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429 Too Many Requests, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Errorf("expected Retry-After header to be set")
	}

	// Test disabled limiter
	disabledLimiter := NewRateLimiter(1.0, 1, false)
	defer disabledLimiter.Stop()
	if allowed, _ := disabledLimiter.Allow(publicIP); !allowed {
		t.Errorf("expected disabled limiter to allow all requests")
	}
}
