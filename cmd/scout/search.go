package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().String("engine", "google", "search engine (google, bing, ddg)")
	searchCmd.Flags().Int("max-pages", 1, "maximum result pages to fetch")
	searchCmd.Flags().String("language", "", "language code (e.g. en, pt-BR)")
	searchCmd.Flags().String("region", "", "region code (e.g. us, br)")

	// --fetch flags migrated from websearch
	searchCmd.Flags().String("fetch", "", "fetch mode for result pages: markdown, text, full (empty = no fetch)")
	searchCmd.Flags().Int("max-fetch", 5, "max results to fetch when --fetch is set")
	searchCmd.Flags().Bool("main-only", false, "extract only main content from fetched pages")
	searchCmd.Flags().Int("concurrency", 3, "fetch concurrency when --fetch is set")

	// wikipedia subcommand (unique engine, kept per D-05)
	searchCmd.AddCommand(searchWikipediaCmd)
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the web using Google, Bing, or DuckDuckGo",
	Long: `Search the web using a configurable engine.

Engines: google (default), bing, ddg (DuckDuckGo).

Use --fetch to also retrieve the content of each result page.
Use 'search wikipedia <query>' for Wikipedia article search.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, _ := cmd.Flags().GetString("engine")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		language, _ := cmd.Flags().GetString("language")
		region, _ := cmd.Flags().GetString("region")
		fetchMode, _ := cmd.Flags().GetString("fetch")
		maxFetch, _ := cmd.Flags().GetInt("max-fetch")
		mainOnly, _ := cmd.Flags().GetBool("main-only")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		format, _ := cmd.Flags().GetString("format")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		// If --fetch is set, use WebSearch API (supports content fetching)
		if fetchMode != "" {
			var opts []scout.WebSearchOption

			switch strings.ToLower(engine) {
			case "bing":
				opts = append(opts, scout.WithWebSearchEngine(scout.Bing))
			case "ddg", "duckduckgo":
				opts = append(opts, scout.WithWebSearchEngine(scout.DuckDuckGo))
			default:
				opts = append(opts, scout.WithWebSearchEngine(scout.Google))
			}

			opts = append(opts, scout.WithWebSearchMaxPages(maxPages))
			opts = append(opts, scout.WithWebSearchMaxFetch(maxFetch))
			opts = append(opts, scout.WithWebSearchConcurrency(concurrency))
			opts = append(opts, scout.WithWebSearchFetch(fetchMode))

			if mainOnly {
				opts = append(opts, scout.WithWebSearchMainContent())
			}

			if language != "" {
				opts = append(opts, scout.WithWebSearchLanguage(language))
			}

			if region != "" {
				opts = append(opts, scout.WithWebSearchRegion(region))
			}

			result, err := browser.WebSearch(args[0], opts...)
			if err != nil {
				return fmt.Errorf("scout: search: %w", err)
			}

			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")

				return enc.Encode(result)
			}

			for _, item := range result.Results {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n   %s\n   %s\n", item.Position, item.Title, item.URL, item.Snippet)
				if item.Content != nil && item.Content.Markdown != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   --- content ---\n%s\n", item.Content.Markdown)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}

			outFile, _ := cmd.Flags().GetString("output")
			if outFile != "" {
				data, _ := json.MarshalIndent(result, "", "  ") //nolint:errchkjson

				dest, writeErr := writeOutput(cmd, data, "search.json")
				if writeErr != nil {
					return writeErr
				}

				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Written to %s\n", dest)
			}

			return nil
		}

		// Standard search (no fetch)
		var searchEngine scout.SearchEngine

		switch strings.ToLower(engine) {
		case "google":
			searchEngine = scout.Google
		case "bing":
			searchEngine = scout.Bing
		case "ddg", "duckduckgo":
			searchEngine = scout.DuckDuckGo
		default:
			return fmt.Errorf("scout: unknown engine %q (use google, bing, or ddg)", engine)
		}

		var opts []scout.SearchOption

		opts = append(opts, scout.WithSearchEngine(searchEngine))
		opts = append(opts, scout.WithSearchMaxPages(maxPages))

		if language != "" {
			opts = append(opts, scout.WithSearchLanguage(language))
		}

		if region != "" {
			opts = append(opts, scout.WithSearchRegion(region))
		}

		results, err := browser.Search(args[0], opts...)
		if err != nil {
			return fmt.Errorf("scout: search: %w", err)
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(results) //nolint:musttag
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Query: %s (%d results)\n\n", results.Query, len(results.Results))
		for _, r := range results.Results {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n   %s\n   %s\n\n", r.Position, r.Title, r.URL, r.Snippet)
		}

		return nil
	},
}

var searchWikipediaCmd = &cobra.Command{
	Use:   "wikipedia <query>",
	Short: "Search Wikipedia for articles",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		maxPages, _ := cmd.InheritedFlags().GetInt("max-pages")
		if maxPages == 0 {
			maxPages = 1
		}

		language, _ := cmd.InheritedFlags().GetString("language")
		region, _ := cmd.InheritedFlags().GetString("region")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		var opts []scout.SearchOption

		opts = append(opts, scout.WithSearchEngine(scout.Wikipedia))
		opts = append(opts, scout.WithSearchMaxPages(maxPages))

		if language != "" {
			opts = append(opts, scout.WithSearchLanguage(language))
		}

		if region != "" {
			opts = append(opts, scout.WithSearchRegion(region))
		}

		results, err := browser.Search(args[0], opts...)
		if err != nil {
			return fmt.Errorf("scout: search: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(results) //nolint:musttag
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Query: %s (%d results)\n\n", results.Query, len(results.Results))
		for _, r := range results.Results {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n   %s\n   %s\n\n", r.Position, r.Title, r.URL, r.Snippet)
		}

		return nil
	},
}
