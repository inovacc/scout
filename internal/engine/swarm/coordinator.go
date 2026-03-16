package swarm

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Coordinator manages the BFS crawl queue, URL deduplication, and worker pool.
type Coordinator struct {
	mu      sync.Mutex
	config  SwarmConfig
	queue   *DomainQueue
	seen    map[string]struct{} // URL deduplication
	workers map[string]*WorkerInfo
	results []CrawlResult
	logger  *slog.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

// NewCoordinator creates a new swarm coordinator with the given config.
func NewCoordinator(cfg SwarmConfig, logger *slog.Logger) *Coordinator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Coordinator{
		config:  cfg,
		queue:   NewDomainQueue(cfg.DefaultRateLimit),
		seen:    make(map[string]struct{}),
		workers: make(map[string]*WorkerInfo),
		logger:  logger,
		done:    make(chan struct{}),
	}
}

// Start begins the coordinator's background goroutines (heartbeat monitoring).
func (c *Coordinator) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	go c.monitorWorkers(ctx)
	c.logger.Info("scout: swarm: coordinator started",
		"batch_size", c.config.BatchSize,
		"max_workers", c.config.MaxWorkers,
	)
}

// Stop shuts down the coordinator and waits for background goroutines.
func (c *Coordinator) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	<-c.done
	c.logger.Info("scout: swarm: coordinator stopped")
}

// Enqueue adds URLs to the crawl queue, skipping already-seen URLs.
// Returns the number of new URLs enqueued.
func (c *Coordinator) Enqueue(urls []CrawlRequest) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var newReqs []*CrawlRequest
	for i := range urls {
		u := urls[i]
		if _, ok := c.seen[u.URL]; ok {
			continue
		}
		c.seen[u.URL] = struct{}{}
		newReqs = append(newReqs, &u)
	}

	if len(newReqs) == 0 {
		return 0, nil
	}

	if err := c.queue.Enqueue(newReqs); err != nil {
		return 0, fmt.Errorf("scout: swarm: enqueue: %w", err)
	}

	c.logger.Debug("scout: swarm: enqueued urls",
		"new", len(newReqs),
		"total_seen", len(c.seen),
	)
	return len(newReqs), nil
}

// Dequeue pulls a batch of URLs for the given worker.
// The worker is marked busy and the URLs are tracked as in-flight.
func (c *Coordinator) Dequeue(workerID string, batchSize int) ([]CrawlRequest, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.workers[workerID]
	if !ok {
		return nil, fmt.Errorf("scout: swarm: dequeue: unknown worker %q", workerID)
	}

	if batchSize <= 0 {
		batchSize = c.config.BatchSize
	}

	reqs := c.queue.Dequeue(batchSize)
	if len(reqs) == 0 {
		return nil, nil
	}

	w.Status = WorkerBusy
	w.LastSeen = time.Now()
	inFlight := make([]string, 0, len(reqs))
	result := make([]CrawlRequest, 0, len(reqs))
	for _, r := range reqs {
		inFlight = append(inFlight, r.URL)
		result = append(result, *r)
	}
	w.InFlight = inFlight

	c.logger.Debug("scout: swarm: dispatched batch",
		"worker", workerID,
		"count", len(result),
	)
	return result, nil
}

// SubmitResults receives crawl results from a worker.
// Discovered URLs are enqueued (with deduplication).
func (c *Coordinator) SubmitResults(workerID string, results []CrawlResult) error {
	c.mu.Lock()
	w, ok := c.workers[workerID]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("scout: swarm: submit results: unknown worker %q", workerID)
	}

	w.Status = WorkerIdle
	w.LastSeen = time.Now()
	w.InFlight = nil
	w.Processed += int64(len(results))
	c.results = append(c.results, results...)
	c.mu.Unlock()

	// Enqueue discovered URLs at depth+1.
	var discovered []CrawlRequest
	for _, r := range results {
		for _, u := range r.DiscoveredURLs {
			discovered = append(discovered, CrawlRequest{URL: u, Depth: 1})
		}
	}
	if len(discovered) > 0 {
		if _, err := c.Enqueue(discovered); err != nil {
			return fmt.Errorf("scout: swarm: submit results: %w", err)
		}
	}

	c.logger.Debug("scout: swarm: results submitted",
		"worker", workerID,
		"count", len(results),
	)
	return nil
}

// RegisterWorker adds a worker to the coordinator.
func (c *Coordinator) RegisterWorker(id, proxy string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.workers) >= c.config.MaxWorkers {
		return fmt.Errorf("scout: swarm: register worker: max workers (%d) reached", c.config.MaxWorkers)
	}
	if _, ok := c.workers[id]; ok {
		return fmt.Errorf("scout: swarm: register worker: worker %q already registered", id)
	}

	c.workers[id] = &WorkerInfo{
		ID:       id,
		Status:   WorkerIdle,
		Proxy:    proxy,
		LastSeen: time.Now(),
	}
	c.logger.Info("scout: swarm: worker registered", "worker", id)
	return nil
}

// UnregisterWorker removes a worker. In-flight URLs are re-queued.
func (c *Coordinator) UnregisterWorker(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.workers[id]
	if !ok {
		return fmt.Errorf("scout: swarm: unregister worker: unknown worker %q", id)
	}

	// Re-queue in-flight URLs.
	if len(w.InFlight) > 0 {
		var reqs []*CrawlRequest
		for _, u := range w.InFlight {
			// Remove from seen so they can be re-enqueued.
			delete(c.seen, u)
			reqs = append(reqs, &CrawlRequest{URL: u})
		}
		_ = c.queue.Enqueue(reqs)
	}

	delete(c.workers, id)
	c.logger.Info("scout: swarm: worker unregistered", "worker", id)
	return nil
}

// Heartbeat updates the worker's last-seen time.
func (c *Coordinator) Heartbeat(workerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.workers[workerID]
	if !ok {
		return fmt.Errorf("scout: swarm: heartbeat: unknown worker %q", workerID)
	}
	w.LastSeen = time.Now()
	return nil
}

// QueueLen returns the number of pending URLs.
func (c *Coordinator) QueueLen() int {
	return c.queue.Len()
}

// Config returns the coordinator's swarm configuration.
func (c *Coordinator) Config() SwarmConfig {
	return c.config
}

// InFlightCount returns the number of in-flight URLs for the given worker.
// Returns 0 if the worker is unknown.
func (c *Coordinator) InFlightCount(workerID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.workers[workerID]
	if !ok {
		return 0
	}
	return len(w.InFlight)
}

// Workers returns a snapshot of all registered workers.
func (c *Coordinator) Workers() []WorkerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]WorkerInfo, 0, len(c.workers))
	for _, w := range c.workers {
		out = append(out, *w)
	}
	return out
}

// Results returns a snapshot of all collected results.
func (c *Coordinator) Results() []CrawlResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]CrawlResult, len(c.results))
	copy(out, c.results)
	return out
}

// monitorWorkers periodically checks for workers that missed heartbeats.
func (c *Coordinator) monitorWorkers(ctx context.Context) {
	defer close(c.done)

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for id, w := range c.workers {
				if w.Status != WorkerDisconnected && now.Sub(w.LastSeen) > c.config.HeartbeatTimeout {
					c.logger.Warn("scout: swarm: worker timed out", "worker", id)
					w.Status = WorkerDisconnected

					// Re-queue in-flight URLs.
					if len(w.InFlight) > 0 {
						var reqs []*CrawlRequest
						for _, u := range w.InFlight {
							delete(c.seen, u)
							reqs = append(reqs, &CrawlRequest{URL: u})
						}
						_ = c.queue.Enqueue(reqs)
						w.InFlight = nil
					}
				}
			}
			c.mu.Unlock()
		}
	}
}
