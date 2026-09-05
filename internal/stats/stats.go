package stats

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"searxgo/internal/models"
)

type Tracker struct {
	mu           sync.RWMutex
	startTime    time.Time
	totalQueries int64
	engines      map[string]*engineMetric
}

type engineMetric struct {
	DisplayName  string
	TotalCount   int64
	SuccessCount int64
	ErrorCount   int64
	TotalTimeMs  int64
	LastPingMs   int64
	LastActive   time.Time
}

var GlobalTracker = NewTracker()

func NewTracker() *Tracker {
	return &Tracker{
		startTime: time.Now(),
		engines:   make(map[string]*engineMetric),
	}
}

// RecordQuery increments global query counter
func (t *Tracker) RecordQuery() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalQueries++
}

// RecordEngineResult records latency and success/failure for an engine
func (t *Tracker) RecordEngineResult(engineName, displayName string, pingMs int64, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	em, exists := t.engines[engineName]
	if !exists {
		em = &engineMetric{
			DisplayName: displayName,
		}
		t.engines[engineName] = em
	}

	em.TotalCount++
	em.LastPingMs = pingMs
	em.TotalTimeMs += pingMs
	em.LastActive = time.Now()

	if success {
		em.SuccessCount++
	} else {
		em.ErrorCount++
	}
}

// GetSystemStats returns a snapshot of server and engine performance
func (t *Tracker) GetSystemStats() models.SystemStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	uptimeDur := time.Since(t.startTime)
	uptimeStr := fmt.Sprintf("%dh %dm %ds", int(uptimeDur.Hours()), int(uptimeDur.Minutes())%60, int(uptimeDur.Seconds())%60)

	var items []models.EngineStatItem
	for name, m := range t.engines {
		avg := int64(0)
		rate := 0.0
		if m.TotalCount > 0 {
			avg = m.TotalTimeMs / m.TotalCount
			rate = (float64(m.SuccessCount) / float64(m.TotalCount)) * 100.0
		}

		lastActiveStr := "Never"
		if !m.LastActive.IsZero() {
			lastActiveStr = m.LastActive.Format("15:04:05")
		}

		items = append(items, models.EngineStatItem{
			Name:         name,
			DisplayName:  m.DisplayName,
			TotalCount:   m.TotalCount,
			SuccessCount: m.SuccessCount,
			ErrorCount:   m.ErrorCount,
			AvgPingMs:    avg,
			LastPingMs:   m.LastPingMs,
			SuccessRate:  rate,
			LastActive:   lastActiveStr,
		})
	}

	return models.SystemStats{
		Uptime:       uptimeStr,
		TotalQueries: t.totalQueries,
		EngineStats:  items,
	}
}

// GetEngineAvgPing returns average ping ms for an engine if available
func (t *Tracker) GetEngineAvgPing(engineName string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if em, ok := t.engines[engineName]; ok && em.TotalCount > 0 {
		return em.TotalTimeMs / em.TotalCount
	}
	return 0
}

// GetOpenMetrics exports Prometheus / OpenMetrics text exposition format (SearXNG /metrics parity)
func (t *Tracker) GetOpenMetrics() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var sb strings.Builder
	uptimeSeconds := time.Since(t.startTime).Seconds()

	sb.WriteString("# HELP searxng_searches_total Total number of search queries handled\n")
	sb.WriteString("# TYPE searxng_searches_total counter\n")
	sb.WriteString(fmt.Sprintf("searxng_searches_total %d\n\n", t.totalQueries))

	sb.WriteString("# HELP searxng_uptime_seconds Process uptime in seconds\n")
	sb.WriteString("# TYPE searxng_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("searxng_uptime_seconds %.2f\n\n", uptimeSeconds))

	sb.WriteString("# HELP searxng_engine_requests_total Total requests sent to each search engine\n")
	sb.WriteString("# TYPE searxng_engine_requests_total counter\n")
	for name, m := range t.engines {
		sb.WriteString(fmt.Sprintf("searxng_engine_requests_total{engine=\"%s\"} %d\n", name, m.TotalCount))
	}
	sb.WriteString("\n")

	sb.WriteString("# HELP searxng_engine_success_total Total successful responses from each search engine\n")
	sb.WriteString("# TYPE searxng_engine_success_total counter\n")
	for name, m := range t.engines {
		sb.WriteString(fmt.Sprintf("searxng_engine_success_total{engine=\"%s\"} %d\n", name, m.SuccessCount))
	}
	sb.WriteString("\n")

	sb.WriteString("# HELP searxng_engine_errors_total Total failed responses from each search engine\n")
	sb.WriteString("# TYPE searxng_engine_errors_total counter\n")
	for name, m := range t.engines {
		sb.WriteString(fmt.Sprintf("searxng_engine_errors_total{engine=\"%s\"} %d\n", name, m.ErrorCount))
	}
	sb.WriteString("\n")

	sb.WriteString("# HELP searxng_engine_response_time_seconds Average response time per engine\n")
	sb.WriteString("# TYPE searxng_engine_response_time_seconds gauge\n")
	for name, m := range t.engines {
		avgSec := 0.0
		if m.TotalCount > 0 {
			avgSec = float64(m.TotalTimeMs) / float64(m.TotalCount) / 1000.0
		}
		sb.WriteString(fmt.Sprintf("searxng_engine_response_time_seconds{engine=\"%s\"} %.4f\n", name, avgSec))
	}
	sb.WriteString("\n")

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	sb.WriteString("# HELP go_goroutines Number of goroutines currently existing\n")
	sb.WriteString("# TYPE go_goroutines gauge\n")
	sb.WriteString(fmt.Sprintf("go_goroutines %d\n\n", runtime.NumGoroutine()))

	sb.WriteString("# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use\n")
	sb.WriteString("# TYPE go_memstats_alloc_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("go_memstats_alloc_bytes %d\n\n", mem.Alloc))

	sb.WriteString("# HELP go_memstats_sys_bytes Number of bytes obtained from system\n")
	sb.WriteString("# TYPE go_memstats_sys_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("go_memstats_sys_bytes %d\n", mem.Sys))

	return sb.String()
}
