package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/swarm"
	"github.com/spf13/cobra"
)

func init() {
	swarmCmd.AddCommand(swarmStartCmd)
	swarmCmd.AddCommand(swarmStatusCmd)

	swarmStartCmd.Flags().IntP("workers", "w", 3, "number of local workers")
	swarmStartCmd.Flags().IntP("depth", "d", 2, "maximum crawl depth")
	swarmStartCmd.Flags().Int("max-pages", 100, "maximum total pages to crawl")
	swarmStartCmd.Flags().String("browser", "chrome", "browser type: chrome, brave, edge")
	swarmStartCmd.Flags().Bool("report", false, "save crawl report to ~/.scout/reports/")

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

var swarmStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show swarm coordinator status (placeholder)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Swarm status: no active coordinator (placeholder)")
		return nil
	},
}
