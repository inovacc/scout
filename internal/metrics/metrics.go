package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds runtime metrics for Scout.
type Metrics struct {
	PagesCreated     atomic.Int64 `json:"pages_created"`
	PagesActive      atomic.Int64 `json:"pages_active"`
	NavigationsTotal atomic.Int64 `json:"navigations_total"`
	ScreenshotsTotal atomic.Int64 `json:"screenshots_total"`
	ExtractionsTotal atomic.Int64 `json:"extractions_total"`
	ErrorsTotal      atomic.Int64 `json:"errors_total"`
	ToolCallsTotal   atomic.Int64 `json:"tool_calls_total"`
	StartTime        time.Time    `json:"start_time"`
}

// Global instance.
var global = &Metrics{StartTime: time.Now()}

// Get returns the global metrics instance.
func Get() *Metrics { return global }

// Reset clears all counters (useful for testing).
func Reset() {
	global = &Metrics{StartTime: time.Now()}
}

// Handler returns an HTTP handler that serves metrics as JSON.
// Compatible with monitoring systems that scrape JSON endpoints.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		m := Get()
		data := map[string]any{
			"pages_created":    m.PagesCreated.Load(),
			"pages_active":     m.PagesActive.Load(),
			"navigations_total": m.NavigationsTotal.Load(),
			"screenshots_total": m.ScreenshotsTotal.Load(),
			"extractions_total": m.ExtractionsTotal.Load(),
			"errors_total":     m.ErrorsTotal.Load(),
			"tool_calls_total": m.ToolCallsTotal.Load(),
			"uptime_seconds":   time.Since(m.StartTime).Seconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	}
}

// PrometheusHandler returns an HTTP handler that serves metrics in Prometheus text format.
func PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		m := Get()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		_, _ = fmt.Fprintf(w, "# HELP scout_pages_created_total Total pages created.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_pages_created_total counter\n")
		_, _ = fmt.Fprintf(w, "scout_pages_created_total %d\n\n", m.PagesCreated.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_pages_active Current active pages.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_pages_active gauge\n")
		_, _ = fmt.Fprintf(w, "scout_pages_active %d\n\n", m.PagesActive.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_navigations_total Total page navigations.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_navigations_total counter\n")
		_, _ = fmt.Fprintf(w, "scout_navigations_total %d\n\n", m.NavigationsTotal.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_screenshots_total Total screenshots taken.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_screenshots_total counter\n")
		_, _ = fmt.Fprintf(w, "scout_screenshots_total %d\n\n", m.ScreenshotsTotal.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_extractions_total Total data extractions.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_extractions_total counter\n")
		_, _ = fmt.Fprintf(w, "scout_extractions_total %d\n\n", m.ExtractionsTotal.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_errors_total Total errors.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "scout_errors_total %d\n\n", m.ErrorsTotal.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_tool_calls_total Total tool calls.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_tool_calls_total counter\n")
		_, _ = fmt.Fprintf(w, "scout_tool_calls_total %d\n\n", m.ToolCallsTotal.Load())

		_, _ = fmt.Fprintf(w, "# HELP scout_uptime_seconds Seconds since process start.\n")
		_, _ = fmt.Fprintf(w, "# TYPE scout_uptime_seconds gauge\n")
		_, _ = fmt.Fprintf(w, "scout_uptime_seconds %.1f\n", time.Since(m.StartTime).Seconds())
	}
}
