package metrics

import (
	"net/http/httptest"
	"testing"
)

func BenchmarkCounterIncrement(b *testing.B) {
	Reset()
	m := Get()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ToolCallsTotal.Add(1)
	}
}

func BenchmarkJSONHandler(b *testing.B) {
	Reset()
	Get().ToolCallsTotal.Add(42)
	handler := Handler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/metrics/json", nil)
		handler.ServeHTTP(w, r)
	}
}

func BenchmarkPrometheusHandler(b *testing.B) {
	Reset()
	Get().ToolCallsTotal.Add(42)
	handler := PrometheusHandler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/metrics", nil)
		handler.ServeHTTP(w, r)
	}
}
