package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/swarm"
	"github.com/inovacc/scout/pkg/scout"
)

// SwarmCrawlInput drives a distributed multi-worker BFS crawl. Distinct from
// `crawl` — uses a coordinator + parallel browser workers with a
// domain-partitioned queue.
type SwarmCrawlInput struct {
	URL      string `json:"url"                jsonschema:"seed URL to start crawling"`
	Workers  int    `json:"workers,omitempty"  jsonschema:"number of parallel browser workers (default 2, max 8)"`
	Depth    int    `json:"depth,omitempty"    jsonschema:"maximum BFS crawl depth (default 2)"`
	MaxPages int    `json:"maxPages,omitempty" jsonschema:"maximum number of pages to crawl (default 50)"`
}

// SwarmCrawlOutput is the swarm crawl summary returned to callers.
type SwarmCrawlOutput struct {
	URL          string   `json:"url"`
	PagesCrawled int      `json:"pagesCrawled"`
	Errors       int      `json:"errors"`
	LinksFound   int      `json:"linksFound"`
	Duration     string   `json:"duration"`
	ReportID     string   `json:"reportID"`
	Workers      int      `json:"workers"`
	Depth        int      `json:"depth"`
	TopLinks     []string `json:"topLinks,omitempty"`
	TopErrors    []string `json:"topErrors,omitempty"`
}

// SwarmCrawl runs a distributed crawl using a swarm coordinator + N browser
// workers, saves a crawl report, and returns a summary. The caller owns `b`;
// the workers spawn their own browsers (derived from b's exec path / stealth).
//
//nolint:gocyclo // wraps the orchestration loop verbatim from the MCP handler.
func SwarmCrawl(ctx context.Context, b *scout.Browser, in SwarmCrawlInput) (*SwarmCrawlOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: swarm_crawl: nil browser")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: swarm_crawl: url required")
	}

	workers := in.Workers
	if workers <= 0 {
		workers = 2
	}
	if workers > 8 {
		workers = 8
	}
	depth := in.Depth
	if depth <= 0 {
		depth = 2
	}
	maxPages := in.MaxPages
	if maxPages <= 0 {
		maxPages = 50
	}

	logger := slog.Default()

	// Create coordinator.
	cfg := swarm.DefaultConfig()
	cfg.MaxWorkers = workers
	cfg.BatchSize = 5

	coord := swarm.NewCoordinator(cfg, logger)
	coord.Start(ctx)

	// Enqueue seed URL.
	if _, err := coord.Enqueue([]swarm.CrawlRequest{{URL: in.URL, Depth: 0}}); err != nil {
		coord.Stop()
		return nil, fmt.Errorf("tools: swarm_crawl: enqueue seed: %w", err)
	}

	// Build browser options for the workers, inheriting exec path + stealth.
	browserOpts := b.WorkerOptions()

	// Create and connect workers.
	wkrs := make([]*swarm.Worker, workers)
	for i := range workers {
		id := fmt.Sprintf("tools-worker-%d", i)
		w := swarm.NewWorker(id, "", cfg.BatchSize, logger, swarm.WithWorkerBrowser(browserOpts...))
		if err := w.Connect(coord); err != nil {
			coord.Stop()
			return nil, fmt.Errorf("tools: swarm_crawl: connect worker %s: %w", id, err)
		}
		wkrs[i] = w
	}

	// Run workers in goroutines.
	crawlCtx, crawlCancel := context.WithCancel(ctx)
	defer crawlCancel()

	var wg sync.WaitGroup
	for _, w := range wkrs {
		wg.Add(1)
		go func(w *swarm.Worker) {
			defer wg.Done()
			_ = w.Run(crawlCtx)
		}(w)
	}

	// Monitor: stop when queue is empty + no in-flight work, or maxPages reached.
	start := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

monitorLoop:
	for {
		select {
		case <-ctx.Done():
			break monitorLoop
		case <-ticker.C:
			results := coord.Results()
			if len(results) >= maxPages {
				break monitorLoop
			}

			// Check if all workers are idle and queue is empty.
			qLen := coord.QueueLen()
			allIdle := true
			for _, w := range wkrs {
				if coord.InFlightCount(w.ID) > 0 {
					allIdle = false
					break
				}
			}
			if qLen == 0 && allIdle && len(results) > 0 {
				break monitorLoop
			}

			// Safety timeout: 5 minutes max.
			if time.Since(start) > 5*time.Minute {
				break monitorLoop
			}
		}
	}

	// Shutdown workers.
	crawlCancel()
	wg.Wait()

	for _, w := range wkrs {
		_ = w.Disconnect()
	}
	coord.Stop()

	// Collect results.
	allResults := coord.Results()
	duration := time.Since(start)

	// Trim to maxPages.
	if len(allResults) > maxPages {
		allResults = allResults[:maxPages]
	}

	// Build summary.
	var crawlErrors []string
	linkSet := make(map[string]struct{})
	for _, r := range allResults {
		if r.Error != "" {
			crawlErrors = append(crawlErrors, fmt.Sprintf("%s: %s", r.URL, r.Error))
		}
		for _, u := range r.DiscoveredURLs {
			linkSet[u] = struct{}{}
		}
	}

	links := make([]string, 0, len(linkSet))
	for u := range linkSet {
		links = append(links, u)
	}

	// Save report.
	report := &engine.Report{
		Type: engine.ReportCrawl,
		URL:  in.URL,
		Crawl: &engine.CrawlReport{
			URL:      in.URL,
			Pages:    len(allResults),
			Duration: duration.Round(time.Millisecond).String(),
			Links:    links,
			Errors:   crawlErrors,
		},
	}

	reportID, err := engine.SaveReport(report)
	if err != nil {
		logger.Warn("tools: swarm_crawl: save report failed", "error", err)
	}

	out := &SwarmCrawlOutput{
		URL:          in.URL,
		PagesCrawled: len(allResults),
		Errors:       len(crawlErrors),
		LinksFound:   len(links),
		Duration:     duration.Round(time.Millisecond).String(),
		ReportID:     reportID,
		Workers:      workers,
		Depth:        depth,
	}

	if len(links) > 20 {
		out.TopLinks = links[:20]
	} else {
		out.TopLinks = links
	}

	if len(crawlErrors) > 10 {
		out.TopErrors = crawlErrors[:10]
	} else if len(crawlErrors) > 0 {
		out.TopErrors = crawlErrors
	}

	return out, nil
}
