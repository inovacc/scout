package firecrawl

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestExtract(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/extract", func(w http.ResponseWriter, r *http.Request) {
		resp := ExtractResult{
			Success: true,
			Data: map[string]any{
				"pricing": []any{
					map[string]any{"plan": "free", "price": "$0"},
					map[string]any{"plan": "pro", "price": "$29"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	result, err := client.Extract(context.Background(),
		[]string{"https://example.com/pricing"},
		WithExtractPrompt("get pricing plans"),
		WithWebSearch(),
	)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if result.Data == nil {
		t.Error("expected data")
	}
}
