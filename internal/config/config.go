package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application runtime settings
type Config struct {
	Host              string
	Port              int
	Timeout           time.Duration
	SafeSearchDefault int
	ImageProxySecret  string
	MaxResults        int
	DefaultPageSize   int
	MaxPageSize       int
	Debug             bool
	LimiterEnabled    bool
	LimiterRate       float64
	LimiterBurst      int
}

// settingsYML mirrors the structure of settings.yml
type settingsYML struct {
	General struct {
		Debug bool `yaml:"debug"`
	} `yaml:"general"`
	Server struct {
		Port      int    `yaml:"port"`
		BindAddr  string `yaml:"bind_address"`
		SecretKey string `yaml:"secret_key"`
	} `yaml:"server"`
	Search struct {
		SafeSearch int `yaml:"safe_search"`
	} `yaml:"search"`
	Outgoing struct {
		RequestTimeout float64 `yaml:"request_timeout"`
	} `yaml:"outgoing"`
}

// loadSettingsYML tries to read settings.yml from well-known locations.
// Returns zero-value struct if the file is missing or unparseable.
func loadSettingsYML() settingsYML {
	candidates := []string{
		"settings.yml",
		"settings.yaml",
		"/etc/searxgo/settings.yml",
		"/etc/searxng/settings.yml",
	}
	// Also honour SEARXNG_SETTINGS_PATH env override
	if ep := os.Getenv("SEARXNG_SETTINGS_PATH"); ep != "" {
		candidates = append([]string{ep}, candidates...)
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s settingsYML
		if err := yaml.Unmarshal(data, &s); err == nil {
			return s
		}
	}
	return settingsYML{}
}

// envInt reads an integer from a list of env var names, returning fallback if none set.
func envInt(fallback int, keys ...string) int {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	return fallback
}

// envStr reads a string from a list of env var names, returning fallback if none set.
func envStr(fallback string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return fallback
}

// envBool reads a boolean from a list of env var names, returning fallback if none set.
func envBool(fallback bool, keys ...string) bool {
	for _, k := range keys {
		if v := os.Getenv(k); v == "true" || v == "1" {
			return true
		} else if v == "false" || v == "0" {
			return false
		}
	}
	return fallback
}

// LoadConfig loads configuration with the following priority (highest → lowest):
//  1. CLI flags (handled in main.go via flag.Parse, overrides after this call)
//  2. Environment variables
//  3. settings.yml (read from disk next to the binary, or /etc/searxgo/)
//  4. Hardcoded defaults
func LoadConfig() *Config {
	yml := loadSettingsYML()

	// --- Port ---
	ymlPort := yml.Server.Port
	if ymlPort == 0 {
		ymlPort = 8184 // hardcoded default
	}
	port := envInt(ymlPort, "SEARXNG_PORT", "PORT", "SERVER_PORT")

	// --- Bind address ---
	ymlHost := yml.Server.BindAddr
	if ymlHost == "" {
		ymlHost = "0.0.0.0"
	}
	host := envStr(ymlHost, "SEARXNG_BIND_ADDRESS", "BIND_ADDRESS", "HOST")

	// --- Search timeout ---
	ymlTimeoutMs := int(yml.Outgoing.RequestTimeout * 1000)
	if ymlTimeoutMs <= 0 {
		ymlTimeoutMs = 3500
	}
	timeoutMs := envInt(ymlTimeoutMs, "SEARXNG_TIMEOUT_MS", "SEARCH_TIMEOUT_MS", "TIMEOUT_MS")

	// --- Secret key ---
	ymlSecret := yml.Server.SecretKey
	if ymlSecret == "" {
		ymlSecret = "searxgo-secret-key-change-in-prod"
	}
	secret := envStr(ymlSecret, "SEARXNG_SECRET_KEY", "SECRET_KEY")

	// --- Debug ---
	debug := envBool(yml.General.Debug, "SEARXNG_DEBUG", "DEBUG")

	// --- Limiter ---
	limiterEnabled := envBool(false, "SEARXNG_LIMITER", "LIMITER_ENABLED")

	// --- Safe search ---
	safeSearch := envInt(yml.Search.SafeSearch, "SEARXNG_SAFE_SEARCH", "SAFE_SEARCH")

	// --- Page size ---
	pageSize := envInt(20, "SEARXNG_PAGE_SIZE", "PAGE_SIZE", "RESULTS_PER_PAGE")

	return &Config{
		Host:              host,
		Port:              port,
		Timeout:           time.Duration(timeoutMs) * time.Millisecond,
		SafeSearchDefault: safeSearch,
		ImageProxySecret:  secret,
		MaxResults:        50,
		DefaultPageSize:   pageSize,
		MaxPageSize:       50,
		Debug:             debug,
		LimiterEnabled:    limiterEnabled,
		LimiterRate:       200.0,
		LimiterBurst:      1000,
	}
}

