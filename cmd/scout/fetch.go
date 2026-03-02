package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(fetchCmd)

	fetchCmd.Flags().String("url", "", "URL to fetch (required)")
	fetchCmd.Flags().String("mode", "full", "extraction mode: full, markdown, html, text, links, meta")
	fetchCmd.Flags().Bool("main-only", false, "extract only the main content")
	fetchCmd.Flags().Bool("include-html", false, "include raw HTML in result (full mode)")
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch a URL and extract structured content",
	Long:  "Navigate to a URL and return clean content (markdown, metadata, links) in a single call.",
	RunE: func(cmd *cobra.Command, args []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" && len(args) > 0 {
			urlFlag = args[0]
		}

		if urlFlag == "" {
			return fmt.Errorf("scout: --url or positional URL is required")
		}

		mode, _ := cmd.Flags().GetString("mode")
		mainOnly, _ := cmd.Flags().GetBool("main-only")
		includeHTML, _ := cmd.Flags().GetBool("include-html")
		format, _ := cmd.Flags().GetString("format")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		var opts []scout.WebFetchOption
		if mode != "" {
			opts = append(opts, scout.WithFetchMode(mode))
		}

		if mainOnly {
			opts = append(opts, scout.WithFetchMainContent())
		}

		if includeHTML {
			opts = append(opts, scout.WithFetchHTML())
		}

		result, err := browser.WebFetch(urlFlag, opts...)
		if err != nil {
			return err
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(result)
		}

		// Text output
		if result.Markdown != "" {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Markdown)
		} else if result.HTML != "" {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), result.HTML)
		} else if result.Meta != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\nDescription: %s\nCanonical: %s\n",
				result.Meta.Title, result.Meta.Description, result.Meta.Canonical)
		} else if len(result.Links) > 0 {
			for _, link := range result.Links {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), link)
			}
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile != "" {
			data, _ := json.MarshalIndent(result, "", "  ")

			dest, writeErr := writeOutput(cmd, data, "fetch.json")
			if writeErr != nil {
				return writeErr
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Written to %s\n", dest)
		}

		return nil
	},
}
