package cli

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

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: batch: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		handler := func(page *scout.Page, url string) (any, error) {
			title, _ := page.Title()
			text, _ := page.ExtractText("body")

			return &batchOutput{
				URL:   url,
				Title: title,
				Text:  text,
			}, nil
		}

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
