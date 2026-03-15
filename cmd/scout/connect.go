package main

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	connectCmd.Flags().String("cdp", "", "Chrome DevTools Protocol WebSocket endpoint (e.g., ws://127.0.0.1:9222)")
	rootCmd.AddCommand(connectCmd)
}

var connectCmd = &cobra.Command{
	Use:   "connect [url]",
	Short: "Connect to a running browser via Chrome DevTools Protocol",
	Long: `Connect to an already-running browser instance using its CDP WebSocket endpoint.
This lets you automate your real browser with logged-in sessions, existing cookies,
and your real fingerprint — no credential management needed.

To get the CDP endpoint, launch Chrome with:
  chrome --remote-debugging-port=9222

Then connect:
  scout connect --cdp ws://127.0.0.1:9222 https://example.com`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cdp, _ := cmd.Flags().GetString("cdp")
		if cdp == "" {
			return fmt.Errorf("scout: connect: --cdp endpoint is required (e.g., ws://127.0.0.1:9222)")
		}

		opts := make([]scout.Option, 0, 4) //nolint:mnd
		opts = append(opts,
			scout.WithRemoteCDP(cdp),
			scout.WithNoSandbox(),
		)
		opts = append(opts, stealthOpts(cmd)...)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: connect: %w", err)
		}

		defer func() { _ = b.Close() }()

		url := ""
		if len(args) > 0 {
			url = args[0]
		}

		page, err := b.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: connect: new page: %w", err)
		}

		if url != "" {
			if err := page.Navigate(url); err != nil {
				return fmt.Errorf("scout: connect: navigate: %w", err)
			}

			_ = page.WaitLoad()
		}

		title, _ := page.Title()
		u, _ := page.URL()

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connected to %s\n", cdp)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Page: %s (%s)\n", u, title)

		return nil
	},
}
