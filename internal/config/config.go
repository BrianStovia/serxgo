package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents the application runtime settings
type Config struct {
	Host              string
	Port              int
	Timeout           time.Duration
	SafeSearchDefault int
	ImageProxySecret  string
	MaxResults        int
	Debug             bool
	LimiterEnabled    bool
	LimiterRate       float64
	LimiterBurst      int
}

// LoadConfig loads configuration from environment variables and settings with safe defaults
func LoadConfig() *Config {
	port := 8184
	for _, key := range []string{"SEARXNG_PORT", "PORT", "SERVER_PORT"} {
		if p := os.Getenv(key); p != "" {
			if val, err := strconv.Atoi(p); err == nil {
				port = val
				break
			}
		}
	}

	host := "0.0.0.0"
	for _, key := range []string{"SEARXNG_BIND_ADDRESS", "BIND_ADDRESS", "HOST"} {
		if h := os.Getenv(key); h != "" {
			host = h
			break
		}
	}

	timeoutMs := 3500
	for _, key := range []string{"SEARXNG_TIMEOUT_MS", "SEARCH_TIMEOUT_MS", "TIMEOUT_MS"} {
		if t := os.Getenv(key); t != "" {
			if val, err := strconv.Atoi(t); err == nil {
				timeoutMs = val
				break
			}
		}
	}

	secret := "searxgo-secret-key-change-in-prod"
	for _, key := range []string{"SEARXNG_SECRET_KEY", "SECRET_KEY"} {
		if s := os.Getenv(key); s != "" {
			secret = s
			break
		}
	}

	debug := false
	for _, key := range []string{"SEARXNG_DEBUG", "DEBUG"} {
		if d := os.Getenv(key); d == "true" || d == "1" {
			debug = true
			break
		}
	}

	// Default false matching SearXNG settings.yml: server.limiter: false (Unlimited Search Mode)
	limiterEnabled := false
	for _, key := range []string{"SEARXNG_LIMITER", "LIMITER_ENABLED"} {
		if l := os.Getenv(key); l == "true" || l == "1" {
			limiterEnabled = true
			break
		}
	}

	return &Config{
		Host:              host,
		Port:              port,
		Timeout:           time.Duration(timeoutMs) * time.Millisecond,
		SafeSearchDefault: 1, // Moderate
		ImageProxySecret:  secret,
		MaxResults:        50,
		Debug:             debug,
		LimiterEnabled:    limiterEnabled,
		LimiterRate:       200.0, // High-throughput 200 requests/sec if enabled
		LimiterBurst:      1000,  // High-capacity burst buffer
	}
}
