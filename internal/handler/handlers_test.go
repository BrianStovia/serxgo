package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"searxgo/internal/aggregator"
	"searxgo/internal/config"
	"searxgo/internal/engine"
)

func TestHTTPHandlers(t *testing.T) {
	cfg := &config.Config{
		Timeout: 3 * time.Second,
	}
	reg := engine.NewRegistry()
	reg.Register(engine.NewWikipediaEngine())
	agg := aggregator.NewAggregator(reg, cfg.Timeout)

	h, err := NewHandler(cfg, agg)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 1. Test Index /
	recIndex := httptest.NewRecorder()
	reqIndex := httptest.NewRequest("GET", "/", nil)
	mux.ServeHTTP(recIndex, reqIndex)

	if recIndex.Code != http.StatusOK {
		t.Errorf("GET / returned status %d; want %d", recIndex.Code, http.StatusOK)
	}

	// 2. Test OpenSearch /opensearch.xml
	recOS := httptest.NewRecorder()
	reqOS := httptest.NewRequest("GET", "/opensearch.xml", nil)
	mux.ServeHTTP(recOS, reqOS)

	if recOS.Code != http.StatusOK {
		t.Errorf("GET /opensearch.xml returned status %d; want %d", recOS.Code, http.StatusOK)
	}

	// 3. Test API Search /api/search without query
	recAPI := httptest.NewRecorder()
	reqAPI := httptest.NewRequest("GET", "/api/search", nil)
	mux.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusBadRequest {
		t.Errorf("GET /api/search without q returned status %d; want %d", recAPI.Code, http.StatusBadRequest)
	}

	// 4. Test Metrics /metrics (Prometheus OpenMetrics)
	recMetrics := httptest.NewRecorder()
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	mux.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Errorf("GET /metrics returned status %d; want %d", recMetrics.Code, http.StatusOK)
	}
	if !strings.Contains(recMetrics.Body.String(), "searxng_searches_total") {
		t.Errorf("GET /metrics body missing 'searxng_searches_total'")
	}

	// 5. Test Config /config
	recConfig := httptest.NewRecorder()
	reqConfig := httptest.NewRequest("GET", "/config", nil)
	mux.ServeHTTP(recConfig, reqConfig)

	if recConfig.Code != http.StatusOK {
		t.Errorf("GET /config returned status %d; want %d", recConfig.Code, http.StatusOK)
	}

	// 6. Test Healthz /healthz
	recHealth := httptest.NewRecorder()
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	mux.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Errorf("GET /healthz returned status %d; want %d", recHealth.Code, http.StatusOK)
	}
}
