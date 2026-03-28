package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetReturnsSameInstance(t *testing.T) {
	a := Get()
	b := Get()
	if a != b {
		t.Error("Get() should return the same instance")
	}
}

func TestReset(t *testing.T) {
	Get().PagesCreated.Add(5)
	Reset()
	if Get().PagesCreated.Load() != 0 {
		t.Error("Reset should zero all counters")
	}
}

func TestCounterIncrements(t *testing.T) {
	Reset()
	m := Get()

	m.PagesCreated.Add(1)
	m.PagesActive.Add(3)
	m.NavigationsTotal.Add(10)
	m.ScreenshotsTotal.Add(2)
	m.ExtractionsTotal.Add(5)
	m.ErrorsTotal.Add(1)
	m.ToolCallsTotal.Add(18)

	if m.PagesCreated.Load() != 1 {
		t.Errorf("PagesCreated = %d", m.PagesCreated.Load())
	}
	if m.PagesActive.Load() != 3 {
		t.Errorf("PagesActive = %d", m.PagesActive.Load())
	}
	if m.NavigationsTotal.Load() != 10 {
		t.Errorf("NavigationsTotal = %d", m.NavigationsTotal.Load())
	}
	if m.ScreenshotsTotal.Load() != 2 {
		t.Errorf("ScreenshotsTotal = %d", m.ScreenshotsTotal.Load())
	}
	if m.ExtractionsTotal.Load() != 5 {
		t.Errorf("ExtractionsTotal = %d", m.ExtractionsTotal.Load())
	}
	if m.ErrorsTotal.Load() != 1 {
		t.Errorf("ErrorsTotal = %d", m.ErrorsTotal.Load())
	}
	if m.ToolCallsTotal.Load() != 18 {
		t.Errorf("ToolCallsTotal = %d", m.ToolCallsTotal.Load())
	}
}

func TestJSONHandler(t *testing.T) {
	Reset()
	Get().NavigationsTotal.Add(42)

	req := httptest.NewRequest("GET", "/metrics/json", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}

	if v, ok := data["navigations_total"].(float64); !ok || v != 42 {
		t.Errorf("navigations_total = %v, want 42", data["navigations_total"])
	}

	if _, ok := data["uptime_seconds"]; !ok {
		t.Error("missing uptime_seconds")
	}
}

func TestPrometheusHandler(t *testing.T) {
	Reset()
	Get().ScreenshotsTotal.Add(7)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	PrometheusHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q, want text/plain", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "scout_screenshots_total 7") {
		t.Errorf("body missing screenshots counter:\n%s", body)
	}
	if !strings.Contains(body, "# TYPE scout_screenshots_total counter") {
		t.Error("body missing TYPE annotation")
	}
	if !strings.Contains(body, "scout_uptime_seconds") {
		t.Error("body missing uptime gauge")
	}
}

func TestPagesActiveDecrement(t *testing.T) {
	Reset()
	m := Get()
	m.PagesActive.Add(5)
	m.PagesActive.Add(-2)
	if m.PagesActive.Load() != 3 {
		t.Errorf("PagesActive = %d, want 3", m.PagesActive.Load())
	}
}
