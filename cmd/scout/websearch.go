package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(websearchCmd)

	websearchCmd.Flags().String("engine", "google", "search engine: google, bing, duckduckgo")
	websearchCmd.Flags().String("fetch", "", "fetch mode for result pages: markdown, text, full (empty = no fetch)")
	websearchCmd.Flags().Int("max-fetch", 5, "max results to fetch")
	websearchCmd.Flags().Int("max-pages", 1, "max search result pages")
	websearchCmd.Flags().String("language", "", "search language (e.g. en, pt-BR)")
	websearchCmd.Flags().String("region", "", "search region (e.g. us, br)")
	websearchCmd.Flags().Bool("main-only", false, "extract only main content from fetched pages")
	websearchCmd.Flags().Int("concurrency", 3, "fetch concurrency")
}

var websearchCmd = &cobra.Command{
	Use:   "websearch <query>",
	Short: "Search the web and optionally fetch result pages",
	Long:  "Perform a web search and optionally fetch each result page for markdown/content extraction.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		engine, _ := cmd.Flags().GetString("engine")
		fetchMode, _ := cmd.Flags().GetString("fetch")
		maxFetch, _ := cmd.Flags().GetInt("max-fetch")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		language, _ := cmd.Flags().GetString("language")
		region, _ := cmd.Flags().GetString("region")
		mainOnly, _ := cmd.Flags().GetBool("main-only")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		format, _ := cmd.Flags().GetString("format")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		var opts []scout.WebSearchOption

		switch engine {
		case "bing":
			opts = append(opts, scout.WithWebSearchEngine(scout.Bing))
		case "duckduckgo", "ddg":
			opts = append(opts, scout.WithWebSearchEngine(scout.DuckDuckGo))
		default:
			opts = append(opts, scout.WithWebSearchEngine(scout.Google))
		}

		opts = append(opts, scout.WithWebSearchMaxPages(maxPages))
		opts = append(opts, scout.WithWebSearchMaxFetch(maxFetch))
		opts = append(opts, scout.WithWebSearchConcurrency(concurrency))

		if fetchMode != "" {
			opts = append(opts, scout.WithWebSearchFetch(fetchMode))
		}

		if mainOnly {
			opts = append(opts, scout.WithWebSearchMainContent())
		}

		if language != "" {
			opts = append(opts, scout.WithWebSearchLanguage(language))
		}

		if region != "" {
			opts = append(opts, scout.WithWebSearchRegion(region))
		}

		result, err := browser.WebSearch(query, opts...)
		if err != nil {
			return err
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(result) //nolint:musttag
		}

		// Text output
		for _, item := range result.Results {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n   %s\n   %s\n", item.Position, item.Title, item.URL, item.Snippet)
			if item.Content != nil && item.Content.Markdown != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   --- content ---\n%s\n", item.Content.Markdown)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile != "" {
			data, _ := json.MarshalIndent(result, "", "  ") //nolint:errchkjson,musttag

			dest, writeErr := writeOutput(cmd, data, "websearch.json")
			if writeErr != nil {
				return writeErr
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Written to %s\n", dest)
		}

		return nil
	},
}
