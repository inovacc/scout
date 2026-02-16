package firecrawl

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchScrape(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/batch/scrape", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp := BatchJob{
			Success: true,
			ID:      "batch-123",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	job, err := client.BatchScrape(context.Background(),
		[]string{"https://example.com/1", "https://example.com/2"},
		FormatMarkdown,
	)
	if err != nil {
		t.Fatalf("batch scrape: %v", err)
	}

	if job.ID != "batch-123" {
		t.Errorf("id = %q, want %q", job.ID, "batch-123")
	}
}

func TestWaitForBatch(t *testing.T) {
	var calls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/batch/scrape/batch-456", func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)

		status := "scraping"
		if n >= 2 {
			status = "completed"
		}

		resp := BatchJob{
			Success:   true,
			ID:        "batch-456",
			Status:    status,
			Total:     2,
			Completed: int(n),
			Data: []Document{
				{Markdown: "# Page 1"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := newTestServer(t, mux)
	client := newTestClient(t, ts)

	job, err := client.WaitForBatch(context.Background(), "batch-456", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("wait for batch: %v", err)
	}

	if job.Status != "completed" {
		t.Errorf("status = %q, want %q", job.Status, "completed")
	}
}
