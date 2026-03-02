package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	sitemapCmd.AddCommand(sitemapExtractCmd)
	rootCmd.AddCommand(sitemapCmd)

	sitemapExtractCmd.Flags().Int("depth", 3, "maximum crawl depth")
	sitemapExtractCmd.Flags().Int("max-pages", 100, "maximum pages to extract")
	sitemapExtractCmd.Flags().Duration("delay", 500*time.Millisecond, "delay between page visits")
	sitemapExtractCmd.Flags().StringSlice("domains", nil, "restrict crawling to these domains")
	sitemapExtractCmd.Flags().Int("dom-depth", 50, "maximum DOM tree depth for JSON extraction")
	sitemapExtractCmd.Flags().String("selector", "", "CSS selector to scope DOM extraction")
	sitemapExtractCmd.Flags().Bool("main-only", false, "extract main content area only (markdown)")
	sitemapExtractCmd.Flags().Bool("skip-json", false, "skip DOM JSON extraction")
	sitemapExtractCmd.Flags().Bool("skip-markdown", false, "skip markdown extraction")
	sitemapExtractCmd.Flags().String("output", "", "output directory for per-page files")
}

var sitemapCmd = &cobra.Command{
	Use:   "sitemap",
	Short: "Sitemap crawl and DOM extraction",
}

var sitemapExtractCmd = &cobra.Command{
	Use:   "extract <url>",
	Short: "Crawl a site and extract DOM JSON + Markdown for every page",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		depth, _ := cmd.Flags().GetInt("depth")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		delay, _ := cmd.Flags().GetDuration("delay")
		domains, _ := cmd.Flags().GetStringSlice("domains")
		domDepth, _ := cmd.Flags().GetInt("dom-depth")
		selector, _ := cmd.Flags().GetString("selector")
		mainOnly, _ := cmd.Flags().GetBool("main-only")
		skipJSON, _ := cmd.Flags().GetBool("skip-json")
		skipMD, _ := cmd.Flags().GetBool("skip-markdown")
		outputDir, _ := cmd.Flags().GetString("output")

		browserOpts := append(baseOpts(cmd), scout.WithBridge(), scout.WithTimeout(30*time.Second))

		browser, err := scout.New(browserOpts...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		var opts []scout.SitemapOption

		opts = append(opts, scout.WithSitemapMaxDepth(depth))
		opts = append(opts, scout.WithSitemapMaxPages(maxPages))
		opts = append(opts, scout.WithSitemapDelay(delay))

		if len(domains) > 0 {
			opts = append(opts, scout.WithSitemapAllowedDomains(domains...))
		}

		if domDepth != 50 {
			opts = append(opts, scout.WithSitemapDOMDepth(domDepth))
		}

		if selector != "" {
			opts = append(opts, scout.WithSitemapSelector(selector))
		}

		if mainOnly {
			opts = append(opts, scout.WithSitemapMainOnly())
		}

		if skipJSON {
			opts = append(opts, scout.WithSitemapSkipJSON())
		}

		if skipMD {
			opts = append(opts, scout.WithSitemapSkipMarkdown())
		}

		if outputDir != "" {
			opts = append(opts, scout.WithSitemapOutputDir(outputDir))
		}

		format, _ := cmd.Flags().GetString("format")

		result, err := browser.SitemapExtract(args[0], opts...)
		if err != nil {
			return fmt.Errorf("scout: sitemap extract: %w", err)
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(result)
		}

		for _, p := range result.Pages {
			status := "OK"
			if p.Error != "" {
				status = p.Error
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[depth=%d] %s  %s  links=%d dom=%v md=%d\n",
				p.Depth, status, p.URL, len(p.Links), p.DOM != nil, len(p.Markdown))
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nExtracted %d pages\n", result.Total)

		if outputDir != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Output: %s\n", outputDir)
		}

		return nil
	},
}
