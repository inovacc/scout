package firecrawl

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var params searchParams

		_ = json.NewDecoder(r.Body).Decode(&params)

		resp := SearchResult{
			Success: true,
			Data: []Document{
				{
					Markdown: "# Result 1",
					Metadata: DocumentMetadata{Title: "Result 1", URL: "https://example.com/1"},
				},
				{
					Markdown: "# Result 2",
					Metadata: DocumentMetadata{Title: "Result 2", URL: "https://example.com/2"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	result, err := client.Search(context.Background(), "test query",
		WithSearchLimit(10),
		WithSearchLang("en"),
		WithSearchCountry("US"),
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Data) != 2 {
		t.Errorf("data len = %d, want 2", len(result.Data))
	}
}
