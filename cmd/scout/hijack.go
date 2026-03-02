package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(hijackCmd)
	hijackCmd.AddCommand(hijackWatchCmd)

	hijackWatchCmd.Flags().Bool("body", false, "capture response bodies")
	hijackWatchCmd.Flags().StringSlice("filter", nil, "URL pattern filter (repeatable)")
	hijackWatchCmd.Flags().Bool("ws-only", false, "WebSocket frames only")
	hijackWatchCmd.Flags().String("output", "", "write to file instead of stdout")
}

var hijackCmd = &cobra.Command{
	Use:   "hijack",
	Short: "Intercept network traffic in real time",
}

var hijackWatchCmd = &cobra.Command{
	Use:   "watch <url>",
	Short: "Stream all network traffic as NDJSON",
	Long:  "Navigate to a URL and stream intercepted HTTP requests, responses, and WebSocket frames as newline-delimited JSON. Press Ctrl+C to stop.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		captureBody, _ := cmd.Flags().GetBool("body")
		filters, _ := cmd.Flags().GetStringSlice("filter")
		wsOnly, _ := cmd.Flags().GetBool("ws-only")
		outFile, _ := cmd.Flags().GetString("output")

		opts := baseOpts(cmd)

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: hijack: %w", err)
		}

		defer func() { _ = b.Close() }()

		page, err := b.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: hijack: %w", err)
		}

		_ = page.WaitLoad()

		var hijackOpts []scout.HijackOption
		if captureBody {
			hijackOpts = append(hijackOpts, scout.WithHijackBodyCapture())
		}

		if len(filters) > 0 {
			hijackOpts = append(hijackOpts, scout.WithHijackURLFilter(filters...))
		}

		hijacker, err := page.NewSessionHijacker(hijackOpts...)
		if err != nil {
			return fmt.Errorf("scout: hijack: %w", err)
		}
		defer hijacker.Stop()

		// Set up output writer.
		var out *os.File

		if outFile != "" && outFile != "-" {
			f, fErr := os.Create(outFile)
			if fErr != nil {
				return fmt.Errorf("scout: hijack: create output: %w", fErr)
			}

			defer func() { _ = f.Close() }()

			out = f
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		if out != nil {
			enc = json.NewEncoder(out)
		}

		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "hijacking... press Ctrl+C to stop")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		for {
			select {
			case ev, ok := <-hijacker.Events():
				if !ok {
					return nil
				}

				if wsOnly {
					switch ev.Type { //nolint:exhaustive
					case scout.HijackWSSent, scout.HijackWSReceived, scout.HijackWSOpened, scout.HijackWSClosed:
						// pass
					default:
						continue
					}
				}

				_ = enc.Encode(ev)

			case <-sigCh:
				signal.Stop(sigCh)
				return nil
			}
		}
	},
}
