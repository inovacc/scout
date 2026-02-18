package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(bridgeCmd)
	bridgeCmd.AddCommand(bridgeStatusCmd, bridgeSendCmd, bridgeListenCmd, bridgeObserveCmd)

	bridgeListenCmd.Flags().StringSlice("events", nil, "event types to filter (e.g. mutation)")
	bridgeListenCmd.Flags().Duration("timeout", 0, "stop after duration (0 = indefinite)")

	bridgeSendCmd.Flags().String("url", "", "URL to navigate to before sending")
	bridgeObserveCmd.Flags().String("url", "", "URL to navigate to before observing")
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Scout Bridge extension commands",
	Long:  `Commands for the built-in Scout Bridge extension that enables Go↔browser communication.`,
}

var bridgeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the bridge extension is active",
	RunE: func(cmd *cobra.Command, _ []string) error {
		headless, _ := cmd.Flags().GetBool("headless")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
		)
		if err != nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "bridge: disconnected (browser unavailable)")
			return nil
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage("about:blank")
		if err != nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "bridge: disconnected (page error)")
			return nil
		}
		defer func() { _ = page.Close() }()

		// Give the content script a moment to load.
		time.Sleep(500 * time.Millisecond)

		bridge, err := page.Bridge()
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "bridge: error (%v)\n", err)
			return nil
		}

		if bridge.Available() {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "bridge: connected")
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "bridge: loaded (content script not yet ready)")
		}

		return nil
	},
}

var bridgeSendCmd = &cobra.Command{
	Use:   "send <type> [json-data]",
	Short: "Send a command to the browser via the bridge",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		urlFlag, _ := cmd.Flags().GetString("url")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge send: %w", err)
		}
		defer func() { _ = browser.Close() }()

		target := "about:blank"
		if urlFlag != "" {
			target = urlFlag
		}

		page, err := browser.NewPage(target)
		if err != nil {
			return fmt.Errorf("scout: bridge send: %w", err)
		}
		defer func() { _ = page.Close() }()

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: bridge send: %w", err)
		}

		bridge, err := page.Bridge()
		if err != nil {
			return fmt.Errorf("scout: bridge send: %w", err)
		}

		eventType := args[0]
		var data any
		if len(args) > 1 {
			var parsed json.RawMessage
			if err := json.Unmarshal([]byte(args[1]), &parsed); err != nil {
				data = args[1] // send as string
			} else {
				data = parsed
			}
		}

		if err := bridge.Send(eventType, data); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sent: %s\n", eventType)

		return nil
	},
}

var bridgeListenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Stream bridge events to stdout",
	RunE: func(cmd *cobra.Command, _ []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		events, _ := cmd.Flags().GetStringSlice("events")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge listen: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage("about:blank")
		if err != nil {
			return fmt.Errorf("scout: bridge listen: %w", err)
		}
		defer func() { _ = page.Close() }()

		bridge, err := page.Bridge()
		if err != nil {
			return fmt.Errorf("scout: bridge listen: %w", err)
		}

		filterSet := make(map[string]bool, len(events))
		for _, e := range events {
			filterSet[e] = true
		}

		handler := func(data json.RawMessage) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
		}

		if len(filterSet) > 0 {
			for evtType := range filterSet {
				bridge.On(evtType, handler)
			}
		} else {
			// Listen for all events by registering a catch-all via the internal event.
			bridge.On("*", handler)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "listening for bridge events... (Ctrl+C to stop)")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		if timeout > 0 {
			select {
			case <-sigCh:
			case <-time.After(timeout):
			}
		} else {
			<-sigCh
		}

		return nil
	},
}

var bridgeObserveCmd = &cobra.Command{
	Use:   "observe <selector>",
	Short: "Start DOM mutation observer via the bridge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		urlFlag, _ := cmd.Flags().GetString("url")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge observe: %w", err)
		}
		defer func() { _ = browser.Close() }()

		target := "about:blank"
		if urlFlag != "" {
			target = urlFlag
		}

		page, err := browser.NewPage(target)
		if err != nil {
			return fmt.Errorf("scout: bridge observe: %w", err)
		}
		defer func() { _ = page.Close() }()

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: bridge observe: %w", err)
		}

		bridge, err := page.Bridge()
		if err != nil {
			return fmt.Errorf("scout: bridge observe: %w", err)
		}

		selector := strings.TrimSpace(args[0])

		bridge.OnMutation(func(mutations []scout.MutationEvent) {
			data, _ := json.Marshal(mutations)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
		})

		// Tell the content script to start observing.
		js := fmt.Sprintf(`function() { if (window.__scout) window.__scout.observeMutations(%q) }`, selector)
		if _, err := page.Eval(js); err != nil {
			return fmt.Errorf("scout: bridge observe: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "observing mutations on %q... (Ctrl+C to stop)\n", selector)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh

		return nil
	},
}
