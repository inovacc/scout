package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var mcpOpenCmd = &cobra.Command{
	Use:   "open <url>",
	Short: "Open a URL in a visible browser for manual inspection",
	Long: `Launch a headed browser and navigate to the given URL for visual analysis.
The browser stays open until you press Ctrl+C. Useful for debugging selectors,
inspecting page state, or comparing headless vs headed rendering.

By default opens in headed mode (--headless=false is implied).
Use --headless to open in headless mode (useful with --devtools).

Examples:
  scout mcp open https://example.com
  scout mcp open https://example.com --stealth
  scout mcp open https://example.com --devtools
  scout mcp open https://example.com --browser brave`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		opts := baseOpts(cmd)

		// Default to headed mode unless --headless was explicitly set.
		if !cmd.Flags().Changed("headless") {
			opts = append(opts, scout.WithHeadless(false))
		}

		maximized, _ := cmd.Flags().GetBool("maximized")
		if maximized {
			opts = append(opts, scout.WithMaximized())
		}

		devtools, _ := cmd.Flags().GetBool("devtools")
		if devtools {
			opts = append(opts, scout.WithDevTools())
		}

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}

		page, err := b.NewPage(url)
		if err != nil {
			_ = b.Close()
			return fmt.Errorf("scout: new page: %w", err)
		}

		_ = page.WaitLoad()

		title, _ := page.Title()
		finalURL, _ := page.URL()
		_, _ = fmt.Fprintf(os.Stderr, "Opened %s (%s)\n", finalURL, title)
		_, _ = fmt.Fprintln(os.Stderr, "Press Ctrl+C or close the browser window to exit...")

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		// Wait for either Ctrl+C or browser window/tab close.
		select {
		case <-sig:
		case <-page.WaitClose():
			_, _ = fmt.Fprintln(os.Stderr, "\nBrowser window closed.")
		}

		_, _ = fmt.Fprintln(os.Stderr, "Closing browser...")
		_ = b.Close()

		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpOpenCmd)
}
