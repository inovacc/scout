package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(batchCmd)

	batchCmd.Flags().StringSlice("urls", nil, "comma-separated list of URLs")
	batchCmd.Flags().String("urls-file", "", "file with one URL per line")
	batchCmd.Flags().Int("concurrency", 3, "number of parallel pages")
	batchCmd.Flags().Bool("async", false, "run in background, print job ID and return immediately")
}

type batchOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Batch scrape multiple URLs concurrently",
	RunE: func(cmd *cobra.Command, args []string) error {
		urls, _ := cmd.Flags().GetStringSlice("urls")
		urlsFile, _ := cmd.Flags().GetString("urls-file")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		async, _ := cmd.Flags().GetBool("async")

		if urlsFile != "" {
			fileURLs, err := readURLsFile(urlsFile)
			if err != nil {
				return fmt.Errorf("scout: batch: %w", err)
			}

			urls = append(urls, fileURLs...)
		}

		if len(urls) == 0 {
			return fmt.Errorf("scout: batch: no URLs provided; use --urls or --urls-file")
		}

		var jm *scout.AsyncJobManager
		if async {
			var err error
			jm, err = scout.NewAsyncJobManager(defaultJobsDir())
			if err != nil {
				return fmt.Errorf("scout: batch: init job manager: %w", err)
			}
		}

		handler := func(page *scout.Page, url string) (any, error) {
			title, _ := page.Title()
			text, _ := page.ExtractText("body")

			return &batchOutput{
				URL:   url,
				Title: title,
				Text:  text,
			}, nil
		}

		if async {
			// Pre-create the job so we can print its ID immediately.
			jobID, err := jm.Create("batch", map[string]any{
				"urls":        urls,
				"concurrency": concurrency,
			})
			if err != nil {
				return fmt.Errorf("scout: batch: create job: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Job %s submitted. Check status with: scout jobs status %s\n", jobID, jobID)

			// Run the batch in a background goroutine.
			go func() {
				browser, bErr := scout.New(baseOpts(cmd)...)
				if bErr != nil {
					_ = jm.Fail(jobID, bErr.Error())
					return
				}
				defer func() { _ = browser.Close() }()

				_ = jm.Start(jobID)

				results := browser.BatchScrape(urls, handler,
					scout.WithBatchConcurrency(concurrency),
				)

				completed, failed := 0, 0
				for _, r := range results {
					completed++
					if r.Error != nil {
						failed++
					}
				}

				_ = jm.UpdateProgress(jobID, completed, failed)
				_ = jm.Complete(jobID, results)
			}()

			return nil
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: batch: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		results := browser.BatchScrape(urls, handler,
			scout.WithBatchConcurrency(concurrency),
		)

		var output []batchOutput
		for _, r := range results {
			var entry batchOutput
			if r.Data != nil {
				if bo, ok := r.Data.(*batchOutput); ok {
					entry = *bo
				}
			}

			entry.URL = r.URL
			if r.Error != nil {
				entry.Error = r.Error.Error()
			}

			output = append(output, entry)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(output)
		}

		for _, e := range output {
			status := "OK"
			if e.Error != "" {
				status = e.Error
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  title=%q\n",
				status, e.URL, truncate(e.Title, 60))
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nProcessed %d URLs\n", len(output))

		return nil
	},
}

func readURLsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var urls []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	return urls, scanner.Err()
}
