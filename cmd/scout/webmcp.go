package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(webmcpCmd)
	webmcpCmd.AddCommand(webmcpDiscoverCmd)
	webmcpCmd.AddCommand(webmcpCallCmd)

	webmcpCallCmd.Flags().String("params", "{}", "JSON parameters for the tool call")
}

var webmcpCmd = &cobra.Command{
	Use:   "webmcp",
	Short: "Discover and invoke MCP tools exposed by web pages",
}

var webmcpDiscoverCmd = &cobra.Command{
	Use:   "discover <url>",
	Short: "List MCP tools exposed by a web page",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: webmcp: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(args[0])
		if err != nil {
			return fmt.Errorf("scout: webmcp: navigate: %w", err)
		}
		defer func() { _ = page.Close() }()

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: webmcp: wait load: %w", err)
		}

		tools, err := page.DiscoverWebMCPTools()
		if err != nil {
			return err
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(tools)
		}

		if len(tools) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No MCP tools found.")
			return nil
		}

		for _, t := range tools {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-10s  %s\n", t.Name, t.Source, t.Description)
		}

		return nil
	},
}

var webmcpCallCmd = &cobra.Command{
	Use:   "call <url> <tool>",
	Short: "Invoke an MCP tool exposed by a web page",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		paramsStr, _ := cmd.Flags().GetString("params")
		format, _ := cmd.Flags().GetString("format")

		var params map[string]any
		if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
			return fmt.Errorf("scout: webmcp: invalid params JSON: %w", err)
		}

		browser, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return fmt.Errorf("scout: webmcp: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(args[0])
		if err != nil {
			return fmt.Errorf("scout: webmcp: navigate: %w", err)
		}
		defer func() { _ = page.Close() }()

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: webmcp: wait load: %w", err)
		}

		result, err := page.CallWebMCPTool(args[1], params)
		if err != nil {
			return err
		}

		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		if result.IsError {
			return fmt.Errorf("scout: webmcp: tool error: %s", result.Content)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Content)
		return nil
	},
}
