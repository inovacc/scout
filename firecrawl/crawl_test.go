package firecrawl

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestCrawl(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/crawl", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp := CrawlJob{
			Success: true,
			ID:      "crawl-123",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	job, err := client.Crawl(context.Background(), "https://example.com",
		WithCrawlLimit(10),
		WithMaxDepth(2),
	)
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}

	if job.ID != "crawl-123" {
		t.Errorf("id = %q, want %q", job.ID, "crawl-123")
	}
}

func TestGetCrawlStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/crawl/crawl-123", func(w http.ResponseWriter, r *http.Request) {
		resp := CrawlJob{
			Success:   true,
			ID:        "crawl-123",
			Status:    "completed",
			Total:     5,
			Completed: 5,
			Data: []Document{
				{Markdown: "# Page 1"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	job, err := client.GetCrawlStatus(context.Background(), "crawl-123")
	if err != nil {
		t.Fatalf("get crawl status: %v", err)
	}

	if job.Status != "completed" {
		t.Errorf("status = %q, want %q", job.Status, "completed")
	}

	if len(job.Data) != 1 {
		t.Errorf("data len = %d, want 1", len(job.Data))
	}
}

func TestWaitForCrawl(t *testing.T) {
	var calls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/crawl/crawl-456", func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)

		status := "scraping"
		if n >= 3 {
			status = "completed"
		}

		resp := CrawlJob{
			Success: true,
			ID:      "crawl-456",
			Status:  status,
			Data: []Document{
				{Markdown: "# Done"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	job, err := client.WaitForCrawl(context.Background(), "crawl-456", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("wait for crawl: %v", err)
	}

	if job.Status != "completed" {
		t.Errorf("status = %q, want %q", job.Status, "completed")
	}
}

func TestCancelCrawl(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/crawl/crawl-789", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	if err := client.CancelCrawl(context.Background(), "crawl-789"); err != nil {
		t.Fatalf("cancel crawl: %v", err)
	}
}
