package cli

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(tableCmd, metaCmd)

	tableCmd.Flags().String("url", "", "URL to navigate to before extracting")
	tableCmd.Flags().String("selector", "table", "CSS selector for the table")

	metaCmd.Flags().String("url", "", "URL to navigate to before extracting")
}

var tableCmd = &cobra.Command{
	Use:   "table",
	Short: "Extract table data from a page",
	RunE: func(cmd *cobra.Command, _ []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")
		selector, _ := cmd.Flags().GetString("selector")

		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required for standalone table extraction")
		}

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		table, err := page.ExtractTable(selector)
		if err != nil {
			return fmt.Errorf("scout: extract table: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(table) //nolint:musttag
		}

		// Text output
		if len(table.Headers) > 0 {
			for i, h := range table.Headers {
				if i > 0 {
					_, _ = fmt.Fprint(cmd.OutOrStdout(), "\t")
				}
				_, _ = fmt.Fprint(cmd.OutOrStdout(), h)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		for _, row := range table.Rows {
			for i, cell := range row {
				if i > 0 {
					_, _ = fmt.Fprint(cmd.OutOrStdout(), "\t")
				}
				_, _ = fmt.Fprint(cmd.OutOrStdout(), cell)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}

		return nil
	},
}

var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Extract page metadata (title, description, OG tags, etc.)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		urlFlag, _ := cmd.Flags().GetString("url")

		if urlFlag == "" {
			return fmt.Errorf("scout: --url is required for standalone meta extraction")
		}

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		meta, err := page.ExtractMeta()
		if err != nil {
			return fmt.Errorf("scout: extract meta: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(meta) //nolint:musttag
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Title:       %s\n", meta.Title)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", meta.Description)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Canonical:   %s\n", meta.Canonical)

		for k, v := range meta.OG {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "OG %-9s %s\n", k+":", v)
		}
		for k, v := range meta.Twitter {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Twitter %-5s %s\n", k+":", v)
		}

		return nil
	},
}
