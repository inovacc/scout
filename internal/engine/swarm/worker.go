package swarm

import (
	"context"
	"fmt"
	"log/slog"
	"time"
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
}

// NewWorker creates a new worker with the given ID and optional proxy.
func NewWorker(id string, proxy string, batchSize int, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	return &Worker{
		ID:        id,
		Status:    WorkerIdle,
		Proxy:     proxy,
		batchSize: batchSize,
		logger:    logger,
		done:      make(chan struct{}),
	}
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

// Disconnect unregisters this worker from the coordinator.
func (w *Worker) Disconnect() error {
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

// processBatch is a stub that simulates crawling each URL.
// This will be wired to scout.Browser in a future iteration.
func (w *Worker) processBatch(_ context.Context, batch []CrawlRequest) []CrawlResult {
	results := make([]CrawlResult, 0, len(batch))
	for _, req := range batch {
		start := time.Now()
		results = append(results, CrawlResult{
			URL:            req.URL,
			StatusCode:     200,
			DiscoveredURLs: nil, // stub: no link extraction yet
			Data:           nil,
			Duration:       time.Since(start),
		})
	}
	return results
}
