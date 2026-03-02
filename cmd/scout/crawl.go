package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(crawlCmd)

	crawlCmd.Flags().Int("max-depth", 3, "maximum crawl depth")
	crawlCmd.Flags().Int("max-pages", 100, "maximum pages to crawl")
	crawlCmd.Flags().Duration("delay", 500*time.Millisecond, "delay between page visits")
	crawlCmd.Flags().StringSlice("domains", nil, "restrict crawling to these domains")
	crawlCmd.Flags().Bool("async", false, "run in background, print job ID and return immediately")
}

var crawlCmd = &cobra.Command{
	Use:   "crawl <url>",
	Short: "Crawl a website starting from a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		maxDepth, _ := cmd.Flags().GetInt("max-depth")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		delay, _ := cmd.Flags().GetDuration("delay")
		domains, _ := cmd.Flags().GetStringSlice("domains")
		async, _ := cmd.Flags().GetBool("async")

		var opts []scout.CrawlOption

		opts = append(opts, scout.WithCrawlMaxDepth(maxDepth))
		opts = append(opts, scout.WithCrawlMaxPages(maxPages))

		opts = append(opts, scout.WithCrawlDelay(delay))
		if len(domains) > 0 {
			opts = append(opts, scout.WithCrawlAllowedDomains(domains...))
		}

		format, _ := cmd.Flags().GetString("format")

		if async {
			jm, err := scout.NewAsyncJobManager(defaultJobsDir())
			if err != nil {
				return fmt.Errorf("scout: crawl: init job manager: %w", err)
			}

			jobID, err := jm.Create("crawl", map[string]any{
				"start_url": args[0],
				"max_depth": maxDepth,
				"max_pages": maxPages,
			})
			if err != nil {
				return fmt.Errorf("scout: crawl: create job: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Job %s submitted. Check status with: scout jobs status %s\n", jobID, jobID)

			go func() {
				browser, bErr := scout.New(baseOpts(cmd)...)
				if bErr != nil {
					_ = jm.Fail(jobID, bErr.Error())
					return
				}

				defer func() { _ = browser.Close() }()

				_ = jm.Start(jobID)

				results, cErr := browser.Crawl(args[0], nil, opts...)

				errCount := 0

				for _, r := range results {
					if r.Error != nil {
						errCount++
					}
				}

				_ = jm.UpdateProgress(jobID, len(results), errCount)

				if cErr != nil {
					_ = jm.Fail(jobID, cErr.Error())
					return
				}

				_ = jm.Complete(jobID, results)
			}()

			return nil
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		handler := func(_ *scout.Page, result *scout.CrawlResult) error {
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				return enc.Encode(result) //nolint:musttag
			}

			status := "OK"
			if result.Error != nil {
				status = result.Error.Error()
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[depth=%d] %s  %s  links=%d\n",
				result.Depth, status, result.URL, len(result.Links))

			return nil
		}

		results, err := browser.Crawl(args[0], handler, opts...)
		if err != nil {
			return fmt.Errorf("scout: crawl: %w", err)
		}

		if format != "json" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nCrawled %d pages\n", len(results))
		}

		return nil
	},
}
