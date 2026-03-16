package swarm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// --- Queue tests ---

func TestDomainQueue_EnqueueDequeue(t *testing.T) {
	q := NewDomainQueue(0) // no rate limit for tests

	reqs := []*CrawlRequest{
		{URL: "https://example.com/a", Depth: 0},
		{URL: "https://example.com/b", Depth: 1},
		{URL: "https://other.com/c", Depth: 0},
	}
	if err := q.Enqueue(reqs); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if got := q.Len(); got != 3 {
		t.Fatalf("expected len 3, got %d", got)
	}

	// Dequeue all.
	out := q.Dequeue(10)
	if len(out) != 3 {
		t.Fatalf("expected 3 dequeued, got %d", len(out))
	}

	if q.Len() != 0 {
		t.Fatalf("expected empty queue, got %d", q.Len())
	}
}

func TestDomainQueue_DomainPartitioning(t *testing.T) {
	q := NewDomainQueue(0)

	var reqs []*CrawlRequest
	for i := range 5 {
		reqs = append(reqs, &CrawlRequest{
			URL:   fmt.Sprintf("https://a.com/page%d", i),
			Depth: i,
		})
	}
	for i := range 3 {
		reqs = append(reqs, &CrawlRequest{
			URL:   fmt.Sprintf("https://b.com/page%d", i),
			Depth: i,
		})
	}

	if err := q.Enqueue(reqs); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	domains := q.Domains()
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d: %v", len(domains), domains)
	}
}

func TestDomainQueue_DepthFirstWithinDomain(t *testing.T) {
	q := NewDomainQueue(0)

	reqs := []*CrawlRequest{
		{URL: "https://example.com/shallow", Depth: 0},
		{URL: "https://example.com/deep", Depth: 5},
		{URL: "https://example.com/mid", Depth: 2},
	}
	if err := q.Enqueue(reqs); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	out := q.Dequeue(3)
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}

	// Should be depth-first: deepest first.
	if out[0].Depth != 5 {
		t.Errorf("expected first item depth 5, got %d", out[0].Depth)
	}
	if out[1].Depth != 2 {
		t.Errorf("expected second item depth 2, got %d", out[1].Depth)
	}
	if out[2].Depth != 0 {
		t.Errorf("expected third item depth 0, got %d", out[2].Depth)
	}
}

func TestDomainQueue_InvalidURL(t *testing.T) {
	q := NewDomainQueue(0)

	err := q.Enqueue([]*CrawlRequest{
		{URL: "://bad-url", Depth: 0},
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// --- Coordinator tests ---

func TestCoordinator_Deduplication(t *testing.T) {
	logger := testLogger()
	cfg := DefaultConfig()
	c := NewCoordinator(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() {
		cancel()
		c.Stop()
	}()

	urls := []CrawlRequest{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
		{URL: "https://example.com/1"}, // duplicate
	}

	n, err := c.Enqueue(urls)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 new urls, got %d", n)
	}

	// Enqueue same URLs again.
	n, err = c.Enqueue(urls)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new urls (all dupes), got %d", n)
	}
}

func TestCoordinator_WorkerRegistration(t *testing.T) {
	logger := testLogger()
	cfg := DefaultConfig()
	cfg.MaxWorkers = 2
	c := NewCoordinator(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() {
		cancel()
		c.Stop()
	}()

	if err := c.RegisterWorker("w1", ""); err != nil {
		t.Fatalf("register w1: %v", err)
	}
	if err := c.RegisterWorker("w2", ""); err != nil {
		t.Fatalf("register w2: %v", err)
	}

	// Should fail: max workers reached.
	if err := c.RegisterWorker("w3", ""); err == nil {
		t.Fatal("expected error for exceeding max workers")
	}

	// Duplicate registration should fail.
	if err := c.RegisterWorker("w1", ""); err == nil {
		t.Fatal("expected error for duplicate worker")
	}

	workers := c.Workers()
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}
}

func TestCoordinator_BatchDispatch(t *testing.T) {
	logger := testLogger()
	cfg := DefaultConfig()
	cfg.DefaultRateLimit = 0
	c := NewCoordinator(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() {
		cancel()
		c.Stop()
	}()

	_ = c.RegisterWorker("w1", "")

	urls := []CrawlRequest{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
		{URL: "https://example.com/3"},
	}
	_, _ = c.Enqueue(urls)

	batch, err := c.Dequeue("w1", 2)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("expected batch of 2, got %d", len(batch))
	}

	// Worker should be busy.
	workers := c.Workers()
	for _, w := range workers {
		if w.ID == "w1" && w.Status != WorkerBusy {
			t.Fatalf("expected worker busy, got %s", w.Status)
		}
	}
}

func TestCoordinator_SubmitResults(t *testing.T) {
	logger := testLogger()
	cfg := DefaultConfig()
	cfg.DefaultRateLimit = 0
	c := NewCoordinator(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() {
		cancel()
		c.Stop()
	}()

	_ = c.RegisterWorker("w1", "")
	_, _ = c.Enqueue([]CrawlRequest{{URL: "https://example.com/1"}})

	batch, _ := c.Dequeue("w1", 10)

	results := []CrawlResult{
		{
			URL:            batch[0].URL,
			StatusCode:     200,
			DiscoveredURLs: []string{"https://example.com/new1", "https://example.com/new2"},
		},
	}

	if err := c.SubmitResults("w1", results); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Discovered URLs should be enqueued.
	if c.QueueLen() != 2 {
		t.Fatalf("expected 2 queued from discovered urls, got %d", c.QueueLen())
	}

	allResults := c.Results()
	if len(allResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(allResults))
	}
}

func TestCoordinator_UnknownWorker(t *testing.T) {
	logger := testLogger()
	c := NewCoordinator(DefaultConfig(), logger)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() {
		cancel()
		c.Stop()
	}()

	_, err := c.Dequeue("nonexistent", 5)
	if err == nil {
		t.Fatal("expected error for unknown worker")
	}
}

// --- Worker tests ---

func TestWorker_ConnectDisconnect(t *testing.T) {
	logger := testLogger()
	cfg := DefaultConfig()
	c := NewCoordinator(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() {
		cancel()
		c.Stop()
	}()

	w := NewWorker("w1", "socks5://proxy:1080", 5, logger)

	if err := w.Connect(c); err != nil {
		t.Fatalf("connect: %v", err)
	}

	workers := c.Workers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}

	if err := w.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	workers = c.Workers()
	if len(workers) != 0 {
		t.Fatalf("expected 0 workers after disconnect, got %d", len(workers))
	}
}

func TestWorker_RunLifecycle(t *testing.T) {
	logger := testLogger()
	cfg := DefaultConfig()
	cfg.DefaultRateLimit = 0
	c := NewCoordinator(cfg, logger)

	cctx, ccancel := context.WithCancel(context.Background())
	c.Start(cctx)
	defer func() {
		ccancel()
		c.Stop()
	}()

	// Seed some URLs.
	_, _ = c.Enqueue([]CrawlRequest{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
		{URL: "https://example.com/3"},
	})

	w := NewWorker("w1", "", 10, logger)
	if err := w.Connect(c); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Run worker with a timeout context.
	wctx, wcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer wcancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(wctx)
	}()

	// Wait for worker to finish (should process all URLs quickly then idle until timeout).
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("worker run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop in time")
	}

	// Check results were submitted.
	results := c.Results()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if err := w.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
}

func TestWorker_RunNotConnected(t *testing.T) {
	w := NewWorker("w1", "", 5, testLogger())
	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("expected error when running without connection")
	}
}
