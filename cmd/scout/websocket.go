package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(wsCmd)
	wsCmd.AddCommand(wsListenCmd)

	wsListenCmd.Flags().String("filter", "", "filter WebSocket URLs by pattern")
	wsListenCmd.Flags().Duration("timeout", 5*time.Minute, "max listen duration")
	wsListenCmd.Flags().Bool("json", false, "output as JSON")
}

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "WebSocket automation and monitoring",
	Long:  "Monitor, intercept, and interact with WebSocket connections on a page.",
}

var wsListenCmd = &cobra.Command{
	Use:   "listen <url>",
	Short: "Monitor WebSocket traffic on a page",
	Long:  "Navigate to a URL and capture all WebSocket messages in real-time.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]
		filter, _ := cmd.Flags().GetString("filter")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		opts := baseOpts(cmd)
		opts = append(opts, scout.WithTimeout(0))

		browser, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: ws: launch browser: %w", err)
		}

		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(targetURL)
		if err != nil {
			return fmt.Errorf("scout: ws: navigate: %w", err)
		}

		_ = page.WaitLoad()

		var wsOpts []scout.WebSocketOption
		if filter != "" {
			wsOpts = append(wsOpts, scout.WithWSURLFilter(filter))
		}

		messages, stop, err := page.MonitorWebSockets(wsOpts...)
		if err != nil {
			return fmt.Errorf("scout: ws: monitor: %w", err)
		}

		defer stop()

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "monitoring WebSocket traffic on %s (Ctrl+C to stop)\n", targetURL)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		timer := time.NewTimer(timeout)
		defer timer.Stop()

		enc := json.NewEncoder(cmd.OutOrStdout())
		count := 0

		for {
			select {
			case <-sigCh:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\ncaptured %d messages\n", count)
				return nil
			case <-timer.C:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "timeout — captured %d messages\n", count)
				return nil
			case msg, ok := <-messages:
				if !ok {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "connection closed — captured %d messages\n", count)
					return nil
				}

				if jsonOutput {
					_ = enc.Encode(msg)
				} else {
					dir := "←"
					if msg.Direction == "sent" {
						dir = "→"
					}

					data := msg.Data
					if len(data) > 200 {
						data = data[:200] + "..."
					}

					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", msg.Timestamp.Format("15:04:05.000"), dir, data)
				}

				count++
			}
		}
	},
}
