package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	gourl "net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/swarm"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	swarmCmd.AddCommand(swarmStartCmd)
	swarmCmd.AddCommand(swarmStatusCmd)
	swarmCmd.AddCommand(swarmJoinCmd)

	swarmStartCmd.Flags().IntP("workers", "w", 3, "number of local workers")
	swarmStartCmd.Flags().IntP("depth", "d", 2, "maximum crawl depth")
	swarmStartCmd.Flags().Int("max-pages", 100, "maximum total pages to crawl")
	swarmStartCmd.Flags().String("browser", "chrome", "browser type: chrome, brave, edge")
	swarmStartCmd.Flags().Bool("report", false, "save crawl report to ~/.scout/reports/")

	swarmJoinCmd.Flags().String("browser", "", "browser type: chrome, brave, edge (default: coordinator's choice)")
	swarmJoinCmd.Flags().Bool("headless", true, "run browser in headless mode")

	rootCmd.AddCommand(swarmCmd)
}

var swarmCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Distributed crawl coordination with multiple browser workers",
}

var swarmStartCmd = &cobra.Command{
	Use:   "start <url>",
	Short: "Start coordinator and local workers to crawl a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		seedURL := args[0]
		numWorkers, _ := cmd.Flags().GetInt("workers")
		maxDepth, _ := cmd.Flags().GetInt("depth")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		browserType, _ := cmd.Flags().GetString("browser")
		saveReport, _ := cmd.Flags().GetBool("report")
		format, _ := cmd.Flags().GetString("format")

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

		cfg := swarm.DefaultConfig()
		cfg.MaxWorkers = numWorkers
		cfg.DefaultRateLimit = 0 // handled by browser latency

		coord := swarm.NewCoordinator(cfg, logger)
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		coord.Start(ctx)

		// Enqueue seed URL.
		if _, err := coord.Enqueue([]swarm.CrawlRequest{
			{URL: seedURL, Depth: 0},
		}); err != nil {
			coord.Stop()
			return fmt.Errorf("scout: swarm: enqueue seed: %w", err)
		}

		start := time.Now()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Starting swarm: %d workers, depth=%d, max-pages=%d\n", numWorkers, maxDepth, maxPages)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Seed: %s\n\n", seedURL)

		// Build browser options for workers.
		browserOpts := []engine.Option{
			engine.WithHeadless(isHeadless(cmd)),
			engine.WithNoSandbox(),
			engine.WithBrowser(engine.BrowserType(browserType)),
		}

		// Spawn workers.
		workers := make([]*swarm.Worker, numWorkers)
		var wg sync.WaitGroup

		for i := range numWorkers {
			id := fmt.Sprintf("worker-%d", i)
			w := swarm.NewWorker(id, "", cfg.BatchSize, logger,
				swarm.WithWorkerBrowser(browserOpts...),
			)
			if err := w.Connect(coord); err != nil {
				cancel()
				coord.Stop()
				return fmt.Errorf("scout: swarm: connect worker %s: %w", id, err)
			}
			workers[i] = w
		}

		// Run workers in goroutines.
		errCh := make(chan error, numWorkers)
		for _, w := range workers {
			wg.Add(1)
			go func(w *swarm.Worker) {
				defer wg.Done()
				if err := w.Run(ctx); err != nil {
					errCh <- err
				}
			}(w)
		}

		// Monitor: wait until queue is empty and all workers idle, or max-pages reached.
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case err := <-errCh:
				cancel()
				wg.Wait()
				coord.Stop()
				return fmt.Errorf("scout: swarm: worker error: %w", err)
			case <-ticker.C:
				results := coord.Results()
				qLen := coord.QueueLen()

				if len(results) >= maxPages {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Max pages (%d) reached.\n", maxPages)
					goto done
				}

				// Check depth limit: stop enqueuing URLs beyond maxDepth.
				// The coordinator handles depth via CrawlRequest.Depth.
				// For now, we stop when the queue is empty and all workers are idle.
				allIdle := true
				for _, wi := range coord.Workers() {
					if wi.Status == swarm.WorkerBusy {
						allIdle = false
						break
					}
				}

				if qLen == 0 && allIdle && len(results) > 0 {
					goto done
				}
			}
		}

	done:
		cancel()
		wg.Wait()

		// Disconnect workers.
		for _, w := range workers {
			if err := w.Disconnect(); err != nil {
				logger.Warn("scout: swarm: disconnect error", "error", err)
			}
		}
		coord.Stop()

		// Print results summary.
		results := coord.Results()
		var errCount int
		for _, r := range results {
			if r.Error != "" {
				errCount++
			}
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nSwarm crawl complete: %d pages crawled, %d errors\n", len(results), errCount)

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return fmt.Errorf("scout: swarm: encode results: %w", err)
			}
		} else {
			for _, r := range results {
				status := "OK"
				if r.Error != "" {
					status = r.Error
				}
				title, _ := r.Data["title"].(string)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  links=%d  %s  (%s)\n",
					status, r.URL, len(r.DiscoveredURLs), title, r.Duration.Round(time.Millisecond))
			}
		}

		if saveReport {
			var allLinks []string
			var allErrors []string

			for _, r := range results {
				allLinks = append(allLinks, r.DiscoveredURLs...)
				if r.Error != "" {
					allErrors = append(allErrors, fmt.Sprintf("%s: %s", r.URL, r.Error))
				}
			}

			report := &engine.Report{
				Type: engine.ReportCrawl,
				URL:  seedURL,
				Crawl: &engine.CrawlReport{
					URL:      seedURL,
					Pages:    len(results),
					Duration: time.Since(start).Round(time.Millisecond).String(),
					Links:    allLinks,
					Errors:   allErrors,
				},
			}

			id, err := engine.SaveReport(report)
			if err != nil {
				return fmt.Errorf("scout: swarm: save report: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Report saved: %s\n", id)
		}

		return nil
	},
}

var swarmJoinCmd = &cobra.Command{
	Use:   "join <coordinator-address>",
	Short: "Join a remote swarm coordinator as a worker",
	Long:  "Connect to a remote swarm coordinator via gRPC, fetch URL batches, crawl them locally with a browser, and submit results back.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := args[0]
		browserType, _ := cmd.Flags().GetString("browser")
		headless, _ := cmd.Flags().GetBool("headless")

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

		// Generate a unique worker ID.
		workerID := fmt.Sprintf("worker-%d", time.Now().UnixNano())

		// Connect to the coordinator via gRPC.
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("scout: swarm join: dial coordinator: %w", err)
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewScoutServiceClient(conn)

		// Set up context with signal handling for clean shutdown.
		ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		// Register with the coordinator.
		joinResp, err := client.JoinSwarm(ctx, &pb.JoinSwarmRequest{
			WorkerId: workerID,
		})
		if err != nil {
			return fmt.Errorf("scout: swarm join: register: %w", err)
		}
		if !joinResp.GetAccepted() {
			return fmt.Errorf("scout: swarm join: rejected: %s", joinResp.GetMessage())
		}

		batchSize := int32(10)
		if joinResp.GetBatchSize() > 0 {
			batchSize = joinResp.GetBatchSize()
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Joined swarm at %s as %s (batch_size=%d)\n", addr, workerID, batchSize)

		// Build browser options.
		browserOpts := []engine.Option{
			engine.WithHeadless(headless),
			engine.WithNoSandbox(),
		}
		if browserType != "" {
			browserOpts = append(browserOpts, engine.WithBrowser(engine.BrowserType(browserType)))
		}

		// Create a browser for local crawling.
		browser, err := engine.New(browserOpts...)
		if err != nil {
			return fmt.Errorf("scout: swarm join: create browser: %w", err)
		}
		defer func() {
			if err := browser.Close(); err != nil {
				logger.Warn("scout: swarm join: browser close error", "error", err)
			}
		}()

		var totalProcessed int

		// Main work loop: fetch batch, process locally, submit results.
		for {
			select {
			case <-ctx.Done():
				goto shutdown
			default:
			}

			// Fetch a batch of URLs from the coordinator.
			fetchResp, err := client.FetchBatch(ctx, &pb.FetchBatchRequest{
				WorkerId: workerID,
				MaxUrls:  batchSize,
			})
			if err != nil {
				// Context cancelled is a clean shutdown.
				if ctx.Err() != nil {
					goto shutdown
				}
				logger.Error("scout: swarm join: fetch batch failed", "error", err)
				goto shutdown
			}

			// If drain is set and no URLs, the crawl is complete.
			if fetchResp.GetDrain() && len(fetchResp.GetUrls()) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Coordinator signaled drain — no more work.\n")
				goto shutdown
			}

			if len(fetchResp.GetUrls()) == 0 {
				// No work available; wait briefly before retrying.
				select {
				case <-ctx.Done():
					goto shutdown
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}

			// Convert protobuf URLs to swarm.CrawlRequest for local processing.
			batch := make([]swarm.CrawlRequest, 0, len(fetchResp.GetUrls()))
			for _, u := range fetchResp.GetUrls() {
				batch = append(batch, swarm.CrawlRequest{
					URL:    u.GetUrl(),
					Depth:  int(u.GetDepth()),
					Domain: u.GetDomain(),
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Processing batch: %d URLs\n", len(batch))

			// Process each URL locally using the browser (mirrors worker.crawlURL pattern).
			results := make([]swarm.CrawlResult, 0, len(batch))
			for _, req := range batch {
				select {
				case <-ctx.Done():
					break
				default:
				}
				r := swarmJoinCrawlURL(browser, req, logger)
				results = append(results, r)
			}

			// Convert results to protobuf and submit.
			entries := make([]*pb.CrawlResultEntry, 0, len(results))
			for _, r := range results {
				dataJSON := "{}"
				if r.Data != nil {
					if b, err := json.Marshal(r.Data); err == nil {
						dataJSON = string(b)
					}
				}
				entries = append(entries, &pb.CrawlResultEntry{
					Url:            r.URL,
					StatusCode:     int32(r.StatusCode),
					Error:          r.Error,
					DiscoveredUrls: r.DiscoveredURLs,
					DataJson:       dataJSON,
					DurationMs:     float64(r.Duration.Milliseconds()),
				})
			}

			submitResp, err := client.SubmitResults(ctx, &pb.SubmitResultsRequest{
				WorkerId: workerID,
				Results:  entries,
			})
			if err != nil {
				if ctx.Err() != nil {
					goto shutdown
				}
				logger.Error("scout: swarm join: submit results failed", "error", err)
				goto shutdown
			}

			totalProcessed += int(submitResp.GetAccepted())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Submitted %d results (%d new URLs queued, %d total processed)\n",
				submitResp.GetAccepted(), submitResp.GetNewUrlsQueued(), totalProcessed)
		}

	shutdown:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nLeaving swarm...\n")

		// Use a fresh context for the leave call since the main one may be cancelled.
		leaveCtx, leaveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer leaveCancel()

		leaveResp, err := client.LeaveSwarm(leaveCtx, &pb.LeaveSwarmRequest{
			WorkerId: workerID,
		})
		if err != nil {
			logger.Warn("scout: swarm join: leave failed", "error", err)
		} else if leaveResp.GetAcknowledged() {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Left swarm cleanly (%d URLs requeued)\n", leaveResp.GetUrlsRequeued())
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Total pages processed: %d\n", totalProcessed)
		return nil
	},
}

// swarmJoinCrawlURL navigates to a single URL and extracts title + links.
// Mirrors the worker.crawlURL pattern from internal/engine/swarm/worker.go.
func swarmJoinCrawlURL(browser *engine.Browser, req swarm.CrawlRequest, logger *slog.Logger) swarm.CrawlResult {
	start := time.Now()

	page, err := browser.NewPage(req.URL)
	if err != nil {
		return swarm.CrawlResult{
			URL:      req.URL,
			Error:    fmt.Sprintf("scout: swarm join: new page: %v", err),
			Duration: time.Since(start),
		}
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return swarm.CrawlResult{
			URL:      req.URL,
			Error:    fmt.Sprintf("scout: swarm join: wait load: %v", err),
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
				if u, ok := v.(string); ok && u != "" && swarmJoinIsSameDomain(req.URL, u) {
					discovered = append(discovered, u)
				}
			}
		}
	}

	logger.Debug("scout: swarm join: crawled url",
		"url", req.URL,
		"title", data["title"],
		"links", len(discovered),
	)

	return swarm.CrawlResult{
		URL:            req.URL,
		StatusCode:     200,
		DiscoveredURLs: discovered,
		Data:           data,
		Duration:       time.Since(start),
	}
}

// swarmJoinIsSameDomain returns true if two URLs share the same root domain.
func swarmJoinIsSameDomain(base, candidate string) bool {
	bu, err := gourl.Parse(base)
	if err != nil {
		return false
	}
	cu, err := gourl.Parse(candidate)
	if err != nil {
		return false
	}
	bHost := strings.TrimPrefix(bu.Hostname(), "www.")
	cHost := strings.TrimPrefix(cu.Hostname(), "www.")
	return strings.EqualFold(bHost, cHost)
}

var swarmStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show swarm coordinator status (placeholder)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Swarm status: no active coordinator (placeholder)")
		return nil
	},
}
