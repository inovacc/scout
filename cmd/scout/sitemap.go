package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/tools"
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
	Long: `Crawl a site and extract DOM JSON + Markdown for every page.

Also available as MCP tool ` + "`mcp__scout__sitemap`" + ` — both surfaces delegate
to pkg/scout/tools.Sitemap.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		browserOpts := append(baseOpts(cmd), scout.WithBridge(), scout.WithTimeout(30*time.Second))
		browser, err := scout.New(browserOpts...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

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
		format, _ := cmd.Flags().GetString("format")

		in := tools.SitemapInput{
			URL:            args[0],
			MaxDepth:       depth,
			MaxPages:       maxPages,
			AllowedDomains: domains,
			DOMDepth:       domDepth,
			Selector:       selector,
			MainOnly:       mainOnly,
			SkipJSON:       skipJSON,
			SkipMarkdown:   skipMD,
			OutputDir:      outputDir,
		}
		if delay > 0 {
			in.Delay = delay.String()
		}

		result, err := tools.Sitemap(context.Background(), browser, in)
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
