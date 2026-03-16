package swarm

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/inovacc/scout/internal/engine"
)

// Worker represents a crawl worker that pulls batches from a coordinator,
// processes URLs, and submits results.
type Worker struct {
	ID     string
	Status WorkerStatus
	Proxy  string

	coordinator *Coordinator
	batchSize   int
	logger      *slog.Logger
	cancel      context.CancelFunc
	done        chan struct{}

	browser     *engine.Browser
	browserOpts []engine.Option
}

// WorkerOption is a functional option for configuring a Worker.
type WorkerOption func(*Worker)

// WithWorkerBrowser sets the browser options used when the worker creates its
// browser instance for crawling.
func WithWorkerBrowser(opts ...engine.Option) WorkerOption {
	return func(w *Worker) {
		w.browserOpts = append(w.browserOpts, opts...)
	}
}

// NewWorker creates a new worker with the given ID and optional proxy.
func NewWorker(id string, proxy string, batchSize int, logger *slog.Logger, opts ...WorkerOption) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	w := &Worker{
		ID:        id,
		Status:    WorkerIdle,
		Proxy:     proxy,
		batchSize: batchSize,
		logger:    logger,
		done:      make(chan struct{}),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Connect registers this worker with the given coordinator.
// In the initial scaffold this is an in-process call; future versions will
// use gRPC to connect to a remote coordinator.
func (w *Worker) Connect(c *Coordinator) error {
	if err := c.RegisterWorker(w.ID, w.Proxy); err != nil {
		return fmt.Errorf("scout: swarm: connect: %w", err)
	}
	w.coordinator = c
	w.Status = WorkerIdle
	w.logger.Info("scout: swarm: worker connected", "worker", w.ID)
	return nil
}

// Disconnect unregisters this worker from the coordinator and closes the
// browser if one was created.
func (w *Worker) Disconnect() error {
	if w.browser != nil {
		if err := w.browser.Close(); err != nil {
			w.logger.Warn("scout: swarm: browser close error", "worker", w.ID, "error", err)
		}
		w.browser = nil
	}

	if w.coordinator == nil {
		return nil
	}
	if err := w.coordinator.UnregisterWorker(w.ID); err != nil {
		return fmt.Errorf("scout: swarm: disconnect: %w", err)
	}
	w.coordinator = nil
	w.Status = WorkerDisconnected
	w.logger.Info("scout: swarm: worker disconnected", "worker", w.ID)
	return nil
}

// Run starts the worker loop: pull batch, process, submit, repeat.
// Blocks until the context is cancelled or Stop is called.
func (w *Worker) Run(ctx context.Context) error {
	if w.coordinator == nil {
		return fmt.Errorf("scout: swarm: run: worker %q not connected to coordinator", w.ID)
	}

	ctx, w.cancel = context.WithCancel(ctx)
	defer close(w.done)

	w.logger.Info("scout: swarm: worker started", "worker", w.ID)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("scout: swarm: worker stopping", "worker", w.ID)
			return nil
		default:
		}

		// Send heartbeat.
		_ = w.coordinator.Heartbeat(w.ID)

		// Pull a batch.
		batch, err := w.coordinator.Dequeue(w.ID, w.batchSize)
		if err != nil {
			w.logger.Error("scout: swarm: dequeue failed", "worker", w.ID, "error", err)
			return fmt.Errorf("scout: swarm: run: %w", err)
		}

		if len(batch) == 0 {
			// No work available; wait briefly before retrying.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}

		// Process batch.
		w.Status = WorkerBusy
		results := w.processBatch(ctx, batch)
		w.Status = WorkerIdle

		// Submit results.
		if err := w.coordinator.SubmitResults(w.ID, results); err != nil {
			w.logger.Error("scout: swarm: submit results failed", "worker", w.ID, "error", err)
			return fmt.Errorf("scout: swarm: run: %w", err)
		}
	}
}

// Stop cancels the worker run loop and waits for it to finish.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	<-w.done
}

// ensureBrowser lazily creates a browser instance with the configured options.
func (w *Worker) ensureBrowser() error {
	if w.browser != nil {
		return nil
	}

	opts := []engine.Option{
		engine.WithHeadless(true),
		engine.WithNoSandbox(),
	}
	opts = append(opts, w.browserOpts...)

	if w.Proxy != "" {
		opts = append(opts, engine.WithProxy(w.Proxy))
	}

	b, err := engine.New(opts...)
	if err != nil {
		return fmt.Errorf("scout: swarm: create browser: %w", err)
	}
	w.browser = b
	return nil
}

// processBatch crawls each URL in the batch using a real browser.
// If the browser cannot be created the results contain error strings.
func (w *Worker) processBatch(ctx context.Context, batch []CrawlRequest) []CrawlResult {
	results := make([]CrawlResult, 0, len(batch))

	if err := w.ensureBrowser(); err != nil {
		w.logger.Error("scout: swarm: browser init failed", "worker", w.ID, "error", err)
		for _, req := range batch {
			results = append(results, CrawlResult{
				URL:   req.URL,
				Error: err.Error(),
			})
		}
		return results
	}

	for _, req := range batch {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		result := w.crawlURL(req)
		results = append(results, result)
	}
	return results
}

// crawlURL navigates to a single URL and extracts title + links.
func (w *Worker) crawlURL(req CrawlRequest) CrawlResult {
	start := time.Now()

	page, err := w.browser.NewPage(req.URL)
	if err != nil {
		return CrawlResult{
			URL:      req.URL,
			Error:    fmt.Sprintf("scout: swarm: new page: %v", err),
			Duration: time.Since(start),
		}
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return CrawlResult{
			URL:      req.URL,
			Error:    fmt.Sprintf("scout: swarm: wait load: %v", err),
			Duration: time.Since(start),
		}
	}

	// Extract page title.
	data := make(map[string]any)
	title, err := page.Eval(`() => document.title`)
	if err == nil && title != nil {
		data["title"] = title.String()
	}

	// Extract links from the page.
	links, err := page.Eval(`() => {
		const anchors = document.querySelectorAll('a[href]');
		const urls = [];
		for (const a of anchors) {
			const href = a.href;
			if (href && href.startsWith('http')) {
				urls.push(href);
			}
		}
		return [...new Set(urls)];
	}`)

	var discovered []string
	if err == nil && links != nil {
		if arr, ok := links.Value.([]any); ok {
			for _, v := range arr {
				if u, ok := v.(string); ok && u != "" && isSameDomain(req.URL, u) {
					discovered = append(discovered, u)
				}
			}
		}
	}

	w.logger.Debug("scout: swarm: crawled url",
		"worker", w.ID,
		"url", req.URL,
		"title", data["title"],
		"links", len(discovered),
	)

	return CrawlResult{
		URL:            req.URL,
		StatusCode:     200,
		DiscoveredURLs: discovered,
		Data:           data,
		Duration:       time.Since(start),
	}
}

// isSameDomain returns true if two URLs share the same root domain.
func isSameDomain(base, candidate string) bool {
	bu, err := url.Parse(base)
	if err != nil {
		return false
	}
	cu, err := url.Parse(candidate)
	if err != nil {
		return false
	}

	bHost := strings.TrimPrefix(bu.Hostname(), "www.")
	cHost := strings.TrimPrefix(cu.Hostname(), "www.")
	return strings.EqualFold(bHost, cHost)
}
