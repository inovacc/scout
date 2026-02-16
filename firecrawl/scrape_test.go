package firecrawl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestScrape(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/scrape", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})

			return
		}

		var params scrapeParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := map[string]any{
			"success": true,
			"data": Document{
				Markdown: "# Example",
				Metadata: DocumentMetadata{
					Title:     "Example Domain",
					URL:       params.URL,
					SourceURL: params.URL,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	t.Run("basic scrape", func(t *testing.T) {
		doc, err := client.Scrape(context.Background(), "https://example.com")
		if err != nil {
			t.Fatalf("scrape: %v", err)
		}

		if doc.Markdown != "# Example" {
			t.Errorf("markdown = %q, want %q", doc.Markdown, "# Example")
		}

		if doc.Metadata.Title != "Example Domain" {
			t.Errorf("title = %q, want %q", doc.Metadata.Title, "Example Domain")
		}
	})

	t.Run("with options", func(t *testing.T) {
		doc, err := client.Scrape(context.Background(), "https://example.com",
			WithFormats(FormatMarkdown, FormatHTML),
			WithOnlyMainContent(),
			WithWaitFor(1000),
		)
		if err != nil {
			t.Fatalf("scrape: %v", err)
		}

		if doc.Markdown == "" {
			t.Error("expected markdown content")
		}
	})
}

func TestScrapeAuthError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/scrape", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid api key"})
	})

	ts := newTestServer(t, mux)
	c, _ := New("bad-key", WithAPIURL(ts.URL))

	_, err := c.Scrape(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestScrapeRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/scrape", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limited"})
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	_, err := client.Scrape(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error")
	}

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Errorf("expected RateLimitError, got %T: %v", err, err)
	}

	if rlErr.RetryAfter != 30 {
		t.Errorf("retryAfter = %d, want 30", rlErr.RetryAfter)
	}
}
