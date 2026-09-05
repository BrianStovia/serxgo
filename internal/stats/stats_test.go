package stats

import (
	"strings"
	"testing"
)

func TestStatsAndOpenMetrics(t *testing.T) {
	tracker := NewTracker()
	tracker.RecordQuery()
	tracker.RecordQuery()

	tracker.RecordEngineResult("google", "Google", 120, true)
	tracker.RecordEngineResult("google", "Google", 180, true)
	tracker.RecordEngineResult("duckduckgo", "DuckDuckGo", 250, false)

	sys := tracker.GetSystemStats()
	if sys.TotalQueries != 2 {
		t.Errorf("expected TotalQueries=2, got %d", sys.TotalQueries)
	}

	if len(sys.EngineStats) != 2 {
		t.Errorf("expected 2 engine stats, got %d", len(sys.EngineStats))
	}

	metrics := tracker.GetOpenMetrics()
	if !strings.Contains(metrics, "searxng_searches_total 2") {
		t.Errorf("missing or incorrect searxng_searches_total in metrics")
	}
	if !strings.Contains(metrics, "searxng_engine_requests_total{engine=\"google\"} 2") {
		t.Errorf("missing google engine requests in metrics")
	}
	if !strings.Contains(metrics, "searxng_engine_errors_total{engine=\"duckduckgo\"} 1") {
		t.Errorf("missing duckduckgo errors in metrics")
	}
	if !strings.Contains(metrics, "go_goroutines") {
		t.Errorf("missing go_goroutines in metrics")
	}
}
