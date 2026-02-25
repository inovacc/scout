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
	bridgeCmd.AddCommand(bridgeStatusCmd, bridgeSendCmd, bridgeListenCmd, bridgeObserveCmd,
		bridgeEventsCmd, bridgeWSSendCmd, bridgeQueryCmd, bridgeClickCmd, bridgeTypeCmd,
		bridgeDOMCmd, bridgeTabsCmd, bridgeClipboardCmd, bridgeRecordCmd,
		bridgeCallExposedCmd, bridgeEmitCmd, bridgeFramesCmd)

	bridgeListenCmd.Flags().StringSlice("events", nil, "event types to filter (e.g. mutation)")
	bridgeListenCmd.Flags().Duration("timeout", 0, "stop after duration (0 = indefinite)")

	bridgeRecordCmd.Flags().String("output", "", "output file for recipe JSON (default: stdout)")
	bridgeRecordCmd.Flags().String("name", "Recorded Recipe", "recipe name")
	bridgeRecordCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeSendCmd.Flags().String("url", "", "URL to navigate to before sending")
	bridgeObserveCmd.Flags().String("url", "", "URL to navigate to before observing")

	bridgeEventsCmd.Flags().String("type", "", "filter by event type (e.g. dom.mutation)")
	bridgeEventsCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeWSSendCmd.Flags().String("params", "{}", "JSON params to send")
	bridgeWSSendCmd.Flags().String("page", "", "target page ID (empty = first client)")
	bridgeWSSendCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeQueryCmd.Flags().Bool("all", false, "query all matching elements")
	bridgeQueryCmd.Flags().String("page", "", "target page ID")
	bridgeQueryCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeClickCmd.Flags().String("page", "", "target page ID")
	bridgeClickCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeTypeCmd.Flags().String("page", "", "target page ID")
	bridgeTypeCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeDOMCmd.AddCommand(bridgeDOMInsertCmd, bridgeDOMRemoveCmd)
	bridgeDOMInsertCmd.Flags().String("position", "afterend", "insert position (beforebegin/afterbegin/beforeend/afterend)")
	bridgeDOMInsertCmd.Flags().String("page", "", "target page ID")
	bridgeDOMInsertCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")
	bridgeDOMRemoveCmd.Flags().String("page", "", "target page ID")
	bridgeDOMRemoveCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeTabsCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeClipboardCmd.Flags().Bool("read", false, "read clipboard text")
	bridgeClipboardCmd.Flags().String("write", "", "write text to clipboard")
	bridgeClipboardCmd.Flags().String("page", "", "target page ID")
	bridgeClipboardCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeCallExposedCmd.Flags().String("args", "[]", "JSON array of arguments")
	bridgeCallExposedCmd.Flags().String("page", "", "target page ID")
	bridgeCallExposedCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeEmitCmd.Flags().String("data", "{}", "JSON data to emit")
	bridgeEmitCmd.Flags().String("page", "", "target page ID")
	bridgeEmitCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")

	bridgeFramesCmd.Flags().String("page", "", "target page ID")
	bridgeFramesCmd.Flags().Int("port", 0, "bridge WebSocket port (0 = auto)")
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

var bridgeEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream bridge WebSocket events to stdout",
	RunE: func(cmd *cobra.Command, _ []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		eventType, _ := cmd.Flags().GetString("type")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge events: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge events: WebSocket server not started (use --port)")
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "bridge WebSocket server at %s\n", bs.Addr())
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "streaming events... (Ctrl+C to stop)")

		if eventType != "" {
			bs.Subscribe(eventType, func(e scout.BridgeEvent) {
				data, _ := json.Marshal(e)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
			})
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		if eventType == "" {
			// Stream all events from channel.
			for {
				select {
				case evt := <-bs.Events():
					data, _ := json.Marshal(evt)
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
				case <-sigCh:
					return nil
				}
			}
		}

		<-sigCh
		return nil
	},
}

func waitForBridgeClient(bs *scout.BridgeServer, pageID string) (string, error) {
	deadline := time.After(30 * time.Second)
	for {
		clients := bs.Clients()
		if len(clients) > 0 {
			if pageID == "" {
				return clients[0], nil
			}
			for _, c := range clients {
				if c == pageID {
					return c, nil
				}
			}
		}
		select {
		case <-deadline:
			return "", fmt.Errorf("no clients connected after 30s")
		case <-time.After(500 * time.Millisecond):
		}
	}
}

var bridgeQueryCmd = &cobra.Command{
	Use:   "query <selector>",
	Short: "Query DOM elements via the bridge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		allFlag, _ := cmd.Flags().GetBool("all")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge query: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge query: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge query: %w", err)
		}

		results, err := bs.QueryDOM(pid, args[0], allFlag)
		if err != nil {
			return err
		}

		data, _ := json.MarshalIndent(results, "", "  ")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
		return nil
	},
}

var bridgeClickCmd = &cobra.Command{
	Use:   "click <selector>",
	Short: "Click an element via the bridge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge click: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge click: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge click: %w", err)
		}

		if err := bs.ClickElement(pid, args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "clicked: %s\n", args[0])
		return nil
	},
}

var bridgeTypeCmd = &cobra.Command{
	Use:   "type <selector> <text>",
	Short: "Type text into an element via the bridge",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge type: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge type: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge type: %w", err)
		}

		if err := bs.TypeText(pid, args[0], args[1]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "typed into: %s\n", args[0])
		return nil
	},
}

var bridgeDOMCmd = &cobra.Command{
	Use:   "dom",
	Short: "DOM manipulation commands via the bridge",
}

var bridgeDOMInsertCmd = &cobra.Command{
	Use:   "insert <selector> <html>",
	Short: "Insert HTML adjacent to an element",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		position, _ := cmd.Flags().GetString("position")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge dom insert: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge dom insert: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge dom insert: %w", err)
		}

		if err := bs.InsertHTML(pid, args[0], position, args[1]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "inserted HTML at %s of %s\n", position, args[0])
		return nil
	},
}

var bridgeDOMRemoveCmd = &cobra.Command{
	Use:   "remove <selector>",
	Short: "Remove an element from the DOM",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge dom remove: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge dom remove: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge dom remove: %w", err)
		}

		if err := bs.RemoveElement(pid, args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", args[0])
		return nil
	},
}

var bridgeTabsCmd = &cobra.Command{
	Use:   "tabs",
	Short: "List open browser tabs via the bridge",
	RunE: func(cmd *cobra.Command, _ []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge tabs: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge tabs: WebSocket server not started")
		}

		if _, err := waitForBridgeClient(bs, ""); err != nil {
			return fmt.Errorf("scout: bridge tabs: %w", err)
		}

		tabs, err := bs.ListTabs()
		if err != nil {
			return err
		}

		data, _ := json.MarshalIndent(tabs, "", "  ")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
		return nil
	},
}

var bridgeClipboardCmd = &cobra.Command{
	Use:   "clipboard",
	Short: "Read or write clipboard via the bridge",
	RunE: func(cmd *cobra.Command, _ []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		readFlag, _ := cmd.Flags().GetBool("read")
		writeText, _ := cmd.Flags().GetString("write")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		if !readFlag && writeText == "" {
			return fmt.Errorf("scout: bridge clipboard: specify --read or --write=<text>")
		}

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge clipboard: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge clipboard: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge clipboard: %w", err)
		}

		if writeText != "" {
			if err := bs.SetClipboard(pid, writeText); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "clipboard: written")
			return nil
		}

		text, err := bs.GetClipboard(pid)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", text)
		return nil
	},
}

var bridgeWSSendCmd = &cobra.Command{
	Use:   "ws-send <method>",
	Short: "Send a command via bridge WebSocket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		paramsStr, _ := cmd.Flags().GetString("params")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge ws-send: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge ws-send: WebSocket server not started (use --port)")
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "bridge WebSocket server at %s\n", bs.Addr())

		// Wait for a client to connect.
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "waiting for client connection...")
		deadline := time.After(30 * time.Second)
		for {
			clients := bs.Clients()
			if len(clients) > 0 {
				if pageID == "" {
					pageID = clients[0]
				}
				break
			}
			select {
			case <-deadline:
				return fmt.Errorf("scout: bridge ws-send: no clients connected after 30s")
			case <-time.After(500 * time.Millisecond):
			}
		}

		var params any
		if paramsStr != "{}" && paramsStr != "" {
			var parsed json.RawMessage
			if err := json.Unmarshal([]byte(paramsStr), &parsed); err != nil {
				params = paramsStr
			} else {
				params = parsed
			}
		}

		method := args[0]
		resp, err := bs.Send(pageID, method, params)
		if err != nil {
			return err
		}

		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))

		return nil
	},
}

var bridgeCallExposedCmd = &cobra.Command{
	Use:   "call-exposed <funcName>",
	Short: "Call a function exposed via window.__scout.expose()",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		argsStr, _ := cmd.Flags().GetString("args")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge call-exposed: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge call-exposed: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge call-exposed: %w", err)
		}

		var fnArgs []any
		if argsStr != "[]" && argsStr != "" {
			if err := json.Unmarshal([]byte(argsStr), &fnArgs); err != nil {
				return fmt.Errorf("scout: bridge call-exposed: invalid --args JSON: %w", err)
			}
		}

		result, err := bs.CallExposed(pid, args[0], fnArgs...)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(result))
		return nil
	},
}

var bridgeEmitCmd = &cobra.Command{
	Use:   "emit <event>",
	Short: "Emit an event to page's window.__scout.on() listeners",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		dataStr, _ := cmd.Flags().GetString("data")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge emit: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge emit: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge emit: %w", err)
		}

		var data any
		if dataStr != "{}" && dataStr != "" {
			if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
				data = dataStr
			}
		}

		if err := bs.EmitEvent(pid, args[0], data); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "emitted: %s\n", args[0])
		return nil
	},
}

var bridgeFramesCmd = &cobra.Command{
	Use:   "frames",
	Short: "List all frames in the page",
	RunE: func(cmd *cobra.Command, _ []string) error {
		headless, _ := cmd.Flags().GetBool("headless")
		pageID, _ := cmd.Flags().GetString("page")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(headless),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge frames: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge frames: WebSocket server not started")
		}

		pid, err := waitForBridgeClient(bs, pageID)
		if err != nil {
			return fmt.Errorf("scout: bridge frames: %w", err)
		}

		frames, err := bs.ListFrames(pid)
		if err != nil {
			return err
		}

		data, _ := json.MarshalIndent(frames, "", "  ")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
		return nil
	},
}

var bridgeRecordCmd = &cobra.Command{
	Use:   "record <url>",
	Short: "Record browser interactions as a recipe",
	Long:  `Opens a non-headless browser with bridge enabled, records user interactions, and outputs a recipe JSON on Ctrl+C.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		output, _ := cmd.Flags().GetString("output")
		port, _ := cmd.Flags().GetInt("port")

		browser, err := scout.New(
			scout.WithHeadless(false),
			scout.WithNoSandbox(),
			scout.WithBridge(),
			scout.WithBridgePort(port),
		)
		if err != nil {
			return fmt.Errorf("scout: bridge record: %w", err)
		}
		defer func() { _ = browser.Close() }()

		bs := browser.BridgeServer()
		if bs == nil {
			return fmt.Errorf("scout: bridge record: WebSocket server not started")
		}

		url := args[0]
		_, err = browser.NewPage(url)
		if err != nil {
			return fmt.Errorf("scout: bridge record: %w", err)
		}

		rec := scout.NewBridgeRecorder(bs)
		rec.Start()

		_, _ = fmt.Fprintf(os.Stderr, "recording interactions on %s... (Ctrl+C to stop)\n", url)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh

		steps := rec.Stop()
		recipeData := rec.ToRecipe(name, url)

		_, _ = fmt.Fprintf(os.Stderr, "recorded %d steps\n", len(steps))

		data, err := json.MarshalIndent(recipeData, "", "  ")
		if err != nil {
			return fmt.Errorf("scout: bridge record: marshal: %w", err)
		}

		if output != "" {
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("scout: bridge record: write: %w", err)
			}
			_, _ = fmt.Fprintf(os.Stderr, "recipe written to %s\n", output)
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", string(data))
		}

		return nil
	},
}
