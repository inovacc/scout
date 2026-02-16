package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inovacc/scout/firecrawl"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(firecrawlCmd)
	firecrawlCmd.PersistentFlags().String("api-key", "", "Firecrawl API key (or FIRECRAWL_API_KEY env)")
	firecrawlCmd.PersistentFlags().String("api-url", "", "custom Firecrawl API URL (self-hosted)")

	firecrawlCmd.AddCommand(fcScrapeCmd)
	fcScrapeCmd.Flags().StringSlice("formats", []string{"markdown"}, "output formats (markdown, html, rawHtml, links, screenshot)")
	fcScrapeCmd.Flags().Bool("only-main", false, "extract only main content")
	fcScrapeCmd.Flags().Int("wait", 0, "wait for dynamic content (ms)")
	fcScrapeCmd.Flags().Int("timeout", 0, "scrape timeout (ms)")

	firecrawlCmd.AddCommand(fcCrawlCmd)
	fcCrawlCmd.Flags().Int("limit", 0, "max pages to crawl")
	fcCrawlCmd.Flags().Int("depth", 0, "max crawl depth")
	fcCrawlCmd.Flags().Bool("wait", false, "wait for crawl to complete")
	fcCrawlCmd.Flags().Duration("poll-interval", 2*time.Second, "poll interval when waiting")

	firecrawlCmd.AddCommand(fcSearchCmd)
	fcSearchCmd.Flags().Int("limit", 5, "max results")
	fcSearchCmd.Flags().String("lang", "", "language (e.g. en)")
	fcSearchCmd.Flags().String("country", "", "country (e.g. US)")

	firecrawlCmd.AddCommand(fcMapCmd)
	fcMapCmd.Flags().String("search", "", "filter URLs by search term")
	fcMapCmd.Flags().Int("limit", 0, "max URLs")
	fcMapCmd.Flags().Bool("include-subdomains", false, "include subdomains")

	firecrawlCmd.AddCommand(fcBatchCmd)
	fcBatchCmd.Flags().StringSlice("urls", nil, "URLs to scrape")
	fcBatchCmd.Flags().String("urls-file", "", "file with URLs (one per line)")
	fcBatchCmd.Flags().Bool("wait", false, "wait for batch to complete")
	fcBatchCmd.Flags().Duration("poll-interval", 2*time.Second, "poll interval when waiting")

	firecrawlCmd.AddCommand(fcExtractCmd)
	fcExtractCmd.Flags().String("prompt", "", "extraction prompt")
	fcExtractCmd.Flags().String("schema", "", "JSON schema file for structured extraction")
	fcExtractCmd.Flags().Bool("web-search", false, "enable web search for extraction")
}

var firecrawlCmd = &cobra.Command{
	Use:   "firecrawl",
	Short: "Firecrawl API for LLM-ready web scraping",
}

func newFirecrawlClient(cmd *cobra.Command) (*firecrawl.Client, error) {
	apiKey, _ := cmd.Flags().GetString("api-key")
	if apiKey == "" {
		apiKey = os.Getenv("FIRECRAWL_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("firecrawl: API key required (--api-key or FIRECRAWL_API_KEY)")
	}

	var opts []firecrawl.Option
	if apiURL, _ := cmd.Flags().GetString("api-url"); apiURL != "" {
		opts = append(opts, firecrawl.WithAPIURL(apiURL))
	}

	return firecrawl.New(apiKey, opts...)
}

func fcOutputJSON(cmd *cobra.Command, data any) error {
	format, _ := cmd.Flags().GetString("format")

	if format == "json" {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}

		_, err = writeOutput(cmd, b, "")

		return err
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")

	return enc.Encode(data)
}

var fcScrapeCmd = &cobra.Command{
	Use:   "scrape <url>",
	Short: "Scrape a URL into LLM-ready content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFirecrawlClient(cmd)
		if err != nil {
			return err
		}

		var opts []firecrawl.ScrapeOption
		if fmts, _ := cmd.Flags().GetStringSlice("formats"); len(fmts) > 0 {
			var formats []firecrawl.Format
			for _, f := range fmts {
				formats = append(formats, firecrawl.Format(f))
			}
			opts = append(opts, firecrawl.WithFormats(formats...))
		}
		if onlyMain, _ := cmd.Flags().GetBool("only-main"); onlyMain {
			opts = append(opts, firecrawl.WithOnlyMainContent())
		}
		if wait, _ := cmd.Flags().GetInt("wait"); wait > 0 {
			opts = append(opts, firecrawl.WithWaitFor(wait))
		}
		if timeout, _ := cmd.Flags().GetInt("timeout"); timeout > 0 {
			opts = append(opts, firecrawl.WithScrapeTimeout(timeout))
		}

		doc, err := client.Scrape(context.Background(), args[0], opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			return fcOutputJSON(cmd, doc)
		}

		if doc.Markdown != "" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), doc.Markdown)
		} else if doc.HTML != "" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), doc.HTML)
		}

		return nil
	},
}

var fcCrawlCmd = &cobra.Command{
	Use:   "crawl <url>",
	Short: "Crawl a website and extract content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFirecrawlClient(cmd)
		if err != nil {
			return err
		}

		var opts []firecrawl.CrawlOption
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			opts = append(opts, firecrawl.WithCrawlLimit(limit))
		}
		if depth, _ := cmd.Flags().GetInt("depth"); depth > 0 {
			opts = append(opts, firecrawl.WithMaxDepth(depth))
		}

		job, err := client.Crawl(context.Background(), args[0], opts...)
		if err != nil {
			return err
		}

		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			interval, _ := cmd.Flags().GetDuration("poll-interval")
			job, err = client.WaitForCrawl(context.Background(), job.ID, interval)
			if err != nil {
				return err
			}
		}

		return fcOutputJSON(cmd, job)
	},
}

var fcSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the web via Firecrawl",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFirecrawlClient(cmd)
		if err != nil {
			return err
		}

		var opts []firecrawl.SearchOption
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			opts = append(opts, firecrawl.WithSearchLimit(limit))
		}
		if lang, _ := cmd.Flags().GetString("lang"); lang != "" {
			opts = append(opts, firecrawl.WithSearchLang(lang))
		}
		if country, _ := cmd.Flags().GetString("country"); country != "" {
			opts = append(opts, firecrawl.WithSearchCountry(country))
		}

		result, err := client.Search(context.Background(), args[0], opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			return fcOutputJSON(cmd, result)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Search: %s (%d results)\n\n", args[0], len(result.Data))
		for i, doc := range result.Data {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n   %s\n   %s\n\n",
				i+1, doc.Metadata.Title, doc.Metadata.URL, truncate(doc.Markdown, 200))
		}

		return nil
	},
}

var fcMapCmd = &cobra.Command{
	Use:   "map <url>",
	Short: "Discover URLs on a website",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFirecrawlClient(cmd)
		if err != nil {
			return err
		}

		var opts []firecrawl.MapOption
		if search, _ := cmd.Flags().GetString("search"); search != "" {
			opts = append(opts, firecrawl.WithMapSearch(search))
		}
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			opts = append(opts, firecrawl.WithMapLimit(limit))
		}
		if sub, _ := cmd.Flags().GetBool("include-subdomains"); sub {
			opts = append(opts, firecrawl.WithIncludeSubdomains())
		}

		result, err := client.Map(context.Background(), args[0], opts...)
		if err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			return fcOutputJSON(cmd, result)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d URLs:\n", len(result.Links))
		for _, link := range result.Links {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), link)
		}

		return nil
	},
}

var fcBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Batch scrape multiple URLs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFirecrawlClient(cmd)
		if err != nil {
			return err
		}

		urls, _ := cmd.Flags().GetStringSlice("urls")

		if urlsFile, _ := cmd.Flags().GetString("urls-file"); urlsFile != "" {
			data, err := os.ReadFile(urlsFile)
			if err != nil {
				return fmt.Errorf("firecrawl: read urls file: %w", err)
			}
			for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					urls = append(urls, trimmed)
				}
			}
		}

		if len(urls) == 0 {
			return fmt.Errorf("firecrawl: no URLs provided (use --urls or --urls-file)")
		}

		job, err := client.BatchScrape(context.Background(), urls)
		if err != nil {
			return err
		}

		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			interval, _ := cmd.Flags().GetDuration("poll-interval")
			job, err = client.WaitForBatch(context.Background(), job.ID, interval)
			if err != nil {
				return err
			}
		}

		return fcOutputJSON(cmd, job)
	},
}

var fcExtractCmd = &cobra.Command{
	Use:   "extract <url>",
	Short: "AI-powered data extraction from a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFirecrawlClient(cmd)
		if err != nil {
			return err
		}

		var opts []firecrawl.ExtractOption
		if prompt, _ := cmd.Flags().GetString("prompt"); prompt != "" {
			opts = append(opts, firecrawl.WithExtractPrompt(prompt))
		}
		if schemaFile, _ := cmd.Flags().GetString("schema"); schemaFile != "" {
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				return fmt.Errorf("firecrawl: read schema: %w", err)
			}
			var schema any
			if err := json.Unmarshal(data, &schema); err != nil {
				return fmt.Errorf("firecrawl: parse schema: %w", err)
			}
			opts = append(opts, firecrawl.WithExtractSchema(schema))
		}
		if ws, _ := cmd.Flags().GetBool("web-search"); ws {
			opts = append(opts, firecrawl.WithWebSearch())
		}

		result, err := client.Extract(context.Background(), []string{args[0]}, opts...)
		if err != nil {
			return err
		}

		return fcOutputJSON(cmd, result)
	},
}
