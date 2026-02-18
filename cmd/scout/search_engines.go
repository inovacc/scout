package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	searchCmd.AddCommand(searchGoogleCmd)
	searchCmd.AddCommand(searchBingCmd)
	searchCmd.AddCommand(searchDDGCmd)
	searchCmd.AddCommand(searchWikipediaCmd)

	// DDG --type flag
	searchDDGCmd.Flags().String("type", "web", "search type (web, news, images)")
}

var searchGoogleCmd = &cobra.Command{
	Use:   "google <query>",
	Short: "Search with Google",
	Args:  cobra.ExactArgs(1),
	RunE:  makeEngineSearchFunc(scout.Google),
}

var searchBingCmd = &cobra.Command{
	Use:   "bing <query>",
	Short: "Search with Bing",
	Args:  cobra.ExactArgs(1),
	RunE:  makeEngineSearchFunc(scout.Bing),
}

var searchDDGCmd = &cobra.Command{
	Use:   "duckduckgo <query>",
	Short: "Search with DuckDuckGo",
	Args:  cobra.ExactArgs(1),
	RunE:  makeDDGSearchFunc(),
}

var searchWikipediaCmd = &cobra.Command{
	Use:   "wikipedia <query>",
	Short: "Search Wikipedia for articles",
	Args:  cobra.ExactArgs(1),
	RunE:  makeEngineSearchFunc(scout.Wikipedia),
}

func makeEngineSearchFunc(engine scout.SearchEngine) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return runSearch(cmd, args[0], engine, nil)
	}
}

func makeDDGSearchFunc() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		searchType, _ := cmd.Flags().GetString("type")
		var extraOpts []scout.SearchOption
		if searchType == "news" {
			extraOpts = append(extraOpts, scout.WithDDGSearchType("news"))
		} else if searchType == "images" {
			extraOpts = append(extraOpts, scout.WithDDGSearchType("images"))
		}
		return runSearch(cmd, args[0], scout.DuckDuckGo, extraOpts)
	}
}

func runSearch(cmd *cobra.Command, query string, engine scout.SearchEngine, extraOpts []scout.SearchOption) error {
	// Inherit flags from parent search command
	maxPages, _ := cmd.InheritedFlags().GetInt("max-pages")
	if maxPages == 0 {
		maxPages = 1
	}
	language, _ := cmd.InheritedFlags().GetString("language")
	region, _ := cmd.InheritedFlags().GetString("region")

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
	opts = append(opts, scout.WithSearchEngine(engine))
	opts = append(opts, scout.WithSearchMaxPages(maxPages))
	if language != "" {
		opts = append(opts, scout.WithSearchLanguage(language))
	}
	if region != "" {
		opts = append(opts, scout.WithSearchRegion(region))
	}
	opts = append(opts, extraOpts...)

	results, err := browser.Search(query, opts...)
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
}
