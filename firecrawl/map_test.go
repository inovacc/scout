package firecrawl

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestMap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/map", func(w http.ResponseWriter, r *http.Request) {
		resp := MapResult{
			Success: true,
			Links: []string{
				"https://example.com/page1",
				"https://example.com/page2",
				"https://example.com/page3",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	result, err := client.Map(context.Background(), "https://example.com",
		WithMapSearch("pricing"),
		WithMapLimit(50),
		WithIncludeSubdomains(),
	)
	if err != nil {
		t.Fatalf("map: %v", err)
	}

	if len(result.Links) != 3 {
		t.Errorf("links len = %d, want 3", len(result.Links))
	}
}
