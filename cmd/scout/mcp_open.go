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

		// Default to headed mode unless --headless was explicitly set.
		headless := isHeadless(cmd)
		if !cmd.Flags().Changed("headless") {
			headless = false
		}

		opts := []scout.Option{
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		}
		opts = append(opts, stealthOpts(cmd)...)

		maximized, _ := cmd.Flags().GetBool("maximized")
		if maximized {
			opts = append(opts, scout.WithMaximized())
		}

		devtools, _ := cmd.Flags().GetBool("devtools")
		if devtools {
			opts = append(opts, scout.WithDevTools())
		}

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

		title, _ := page.Title()
		finalURL, _ := page.URL()
		_, _ = fmt.Fprintf(os.Stderr, "Opened %s (%s)\n", finalURL, title)
		_, _ = fmt.Fprintln(os.Stderr, "Press Ctrl+C to close the browser...")

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		_, _ = fmt.Fprintln(os.Stderr, "\nClosing browser...")
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpOpenCmd)
}
