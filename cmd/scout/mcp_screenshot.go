package main

import (
	"fmt"
	"os"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var mcpScreenshotCmd = &cobra.Command{
	Use:   "screenshot <url>",
	Short: "Take a screenshot of a URL and save to file",
	Long: `Navigate to a URL and take a screenshot using the Scout browser.
Supports both headless (default) and headed mode (--headless=false).

Examples:
  scout mcp screenshot https://example.com
  scout mcp screenshot https://example.com --output page.png
  scout mcp screenshot https://example.com --full-page
  scout mcp screenshot https://example.com --headless=false`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		fullPage, _ := cmd.Flags().GetBool("full-page")
		output, _ := cmd.Flags().GetString("output")

		opts := baseOpts(cmd)

		browser, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: new page: %w", err)
		}

		_ = page.WaitLoad()

		var data []byte
		if fullPage {
			data, err = page.FullScreenshot()
		} else {
			data, err = page.Screenshot()
		}

		if err != nil {
			return fmt.Errorf("scout: screenshot: %w", err)
		}

		dest, err := writeOutput(cmd, data, output)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(os.Stderr, "Screenshot saved to %s (%d bytes)\n", dest, len(data))

		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpScreenshotCmd)
	mcpScreenshotCmd.Flags().Bool("full-page", false, "capture full page screenshot")
	mcpScreenshotCmd.Flags().StringP("output", "o", "screenshot.png", "output file path (use - for stdout)")
}
