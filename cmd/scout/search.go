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
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the web using Google, Bing, or DuckDuckGo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, _ := cmd.Flags().GetString("engine")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		language, _ := cmd.Flags().GetString("language")
		region, _ := cmd.Flags().GetString("region")

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

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

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
