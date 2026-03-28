package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/swarm"
	"github.com/inovacc/scout/internal/metrics"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerSwarmTools adds swarm crawl tools.
func registerSwarmTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "swarm_crawl",
		Description: "Crawl a website using multiple browser workers in parallel. Discovers pages via BFS, respects depth/maxPages limits, and saves a crawl report.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url":      {"type": "string",  "description": "Seed URL to start crawling"},
				"workers":  {"type": "integer", "description": "Number of parallel browser workers (default 2, max 8)"},
				"depth":    {"type": "integer", "description": "Maximum BFS crawl depth (default 2)"},
				"maxPages": {"type": "integer", "description": "Maximum number of pages to crawl (default 50)"}
			},
			"required": ["url"]
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()

		var args struct {
			URL      string `json:"url"`
			Workers  int    `json:"workers"`
			Depth    int    `json:"depth"`
			MaxPages int    `json:"maxPages"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.URL == "" {
			return errResult("scout-mcp: swarm_crawl: url is required")
		}
		if args.Workers <= 0 {
			args.Workers = 2
		}
		if args.Workers > 8 {
			args.Workers = 8
		}
		if args.Depth <= 0 {
			args.Depth = 2
		}
		if args.MaxPages <= 0 {
			args.MaxPages = 50
		}

		logger := state.config.Logger
		if logger == nil {
			logger = slog.Default()
		}

		// Create coordinator.
		cfg := swarm.DefaultConfig()
		cfg.MaxWorkers = args.Workers
		cfg.BatchSize = 5

		coord := swarm.NewCoordinator(cfg, logger)
		coord.Start(ctx)

		// Enqueue seed URL.
		if _, err := coord.Enqueue([]swarm.CrawlRequest{{URL: args.URL, Depth: 0}}); err != nil {
			coord.Stop()
			return errResult(fmt.Sprintf("scout-mcp: swarm_crawl: enqueue seed: %s", err))
		}

		// Build browser options matching the MCP state config.
		var browserOpts []engine.Option
		if state.config.BrowserBin != "" {
			browserOpts = append(browserOpts, engine.WithExecPath(state.config.BrowserBin))
		}
		if state.config.Stealth {
			browserOpts = append(browserOpts, engine.WithStealth())
		}

		// Create and connect workers.
		workers := make([]*swarm.Worker, args.Workers)
		for i := range args.Workers {
			id := fmt.Sprintf("mcp-worker-%d", i)
			w := swarm.NewWorker(id, "", cfg.BatchSize, logger, swarm.WithWorkerBrowser(browserOpts...))
			if err := w.Connect(coord); err != nil {
				coord.Stop()
				return errResult(fmt.Sprintf("scout-mcp: swarm_crawl: connect worker %s: %s", id, err))
			}
			workers[i] = w
		}

		// Run workers in goroutines.
		crawlCtx, crawlCancel := context.WithCancel(ctx)
		defer crawlCancel()

		var wg sync.WaitGroup
		for _, w := range workers {
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
				if len(results) >= args.MaxPages {
					break monitorLoop
				}

				// Check if all workers are idle and queue is empty.
				qLen := coord.QueueLen()
				allIdle := true
				for _, w := range workers {
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

		for _, w := range workers {
			_ = w.Disconnect()
		}
		coord.Stop()

		// Collect results.
		allResults := coord.Results()
		duration := time.Since(start)

		metrics.Get().NavigationsTotal.Add(int64(len(allResults)))

		// Trim to maxPages.
		if len(allResults) > args.MaxPages {
			allResults = allResults[:args.MaxPages]
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
			URL:  args.URL,
			Crawl: &engine.CrawlReport{
				URL:      args.URL,
				Pages:    len(allResults),
				Duration: duration.Round(time.Millisecond).String(),
				Links:    links,
				Errors:   crawlErrors,
			},
		}

		reportID, err := engine.SaveReport(report)
		if err != nil {
			logger.Warn("scout-mcp: swarm_crawl: save report failed", "error", err)
		}

		// Return summary.
		summary := map[string]any{
			"url":          args.URL,
			"pagesCrawled": len(allResults),
			"errors":       len(crawlErrors),
			"linksFound":   len(links),
			"duration":     duration.Round(time.Millisecond).String(),
			"reportID":     reportID,
			"workers":      args.Workers,
			"depth":        args.Depth,
		}

		if len(links) > 20 {
			summary["topLinks"] = links[:20]
		} else {
			summary["topLinks"] = links
		}

		if len(crawlErrors) > 10 {
			summary["topErrors"] = crawlErrors[:10]
		} else if len(crawlErrors) > 0 {
			summary["topErrors"] = crawlErrors
		}

		return jsonResult(summary)
	})
}
