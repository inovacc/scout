package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/inovacc/scout/internal/engine/browser"
	scoutmcp "github.com/inovacc/scout/pkg/scout/mcp"
	"github.com/spf13/cobra"
)

type mcpServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type mcpConfig struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers"`
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for LLM browser control via stdio",
	Long: `Start a Model Context Protocol server that exposes Scout browser
automation capabilities as MCP tools. Communicates via stdio (JSON-RPC).

Use --install to generate .mcp.json in the current directory.
Use --install --claude to register globally via "claude mcp add".

Tools (19 built-in browser automation tools):
  Browser:     navigate, click, type, back, forward, wait, screenshot, snapshot, extract, eval, open
  Capture:     pdf, session_list, session_reset, open, swarm_crawl, hijack_watch
  WebSocket:   ws_listen, ws_send, ws_connections
Resources: scout://page/markdown, scout://page/url, scout://page/title

Subcommands:
  scout mcp screenshot <url>  Take a screenshot and save to file
  scout mcp open <url>        Open URL in headed browser for inspection`,
	RunE: func(cmd *cobra.Command, args []string) error {
		install, _ := cmd.Flags().GetBool("install")
		if install {
			claude, _ := cmd.Flags().GetBool("claude")

			if !claude {
				cfg := mcpConfig{
					MCPServers: map[string]mcpServerConfig{
						"scout": {
							Command: "scout",
							Args:    []string{"mcp"},
						},
					},
				}

				data, err := json.MarshalIndent(cfg, "", "  ")
				if err != nil {
					return fmt.Errorf("scout: marshal mcp config: %w", err)
				}

				if err := os.WriteFile(".mcp.json", append(data, '\n'), 0644); err != nil {
					return fmt.Errorf("scout: write .mcp.json: %w", err)
				}

				_, _ = fmt.Fprintln(os.Stderr, "Wrote .mcp.json")

				return nil
			}

			// Default: register globally via claude mcp add
			bin, err := exec.LookPath("claude")
			if err != nil {
				return fmt.Errorf("scout: claude CLI not found: %w", err)
			}

			add := exec.Command(bin, "mcp", "add", "-s", "user", "scout", "--", "scout", "mcp")
			add.Stdout = os.Stdout
			add.Stderr = os.Stderr

			if err := add.Run(); err != nil {
				return fmt.Errorf("scout: claude mcp add: %w", err)
			}

			_, _ = fmt.Fprintln(os.Stderr, "Registered scout MCP server globally via claude mcp add")

			return nil
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		headless, _ := cmd.Flags().GetBool("headless")
		stealth, _ := cmd.Flags().GetBool("stealth")
		bin, _ := cmd.Flags().GetString("bin")
		browserType, _ := cmd.Flags().GetString("browser")
		idleTimeout, _ := cmd.Flags().GetDuration("idle-timeout")

		if v, _ := cmd.Flags().GetBool("allow-local-targets"); v {
			_ = os.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "1")
		}
		if v, _ := cmd.Flags().GetStringSlice("allow-target"); len(v) > 0 {
			_ = os.Setenv("SCOUT_ALLOW_TARGETS", strings.Join(v, ","))
		}

		// Resolve --browser type name to a binary path if --bin is not set.
		if bin == "" && browserType != "" {
			if resolved, err := browser.ResolveCached(context.Background(), browser.BrowserType(browserType)); err == nil {
				bin = resolved
			}
		}

		return scoutmcp.Serve(context.Background(), logger, headless, stealth, bin, idleTimeout)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().BoolP("install", "i", false, "Write .mcp.json to current directory")
	mcpCmd.Flags().BoolP("claude", "c", false, "Register globally via claude mcp add (use with --install)")
	mcpCmd.Flags().String("bin", "", "Path to browser executable")
	mcpCmd.Flags().String("browser", "", "Browser type: chrome, brave, edge (resolves to cached binary)")
	mcpCmd.Flags().Bool("allow-local-targets", false, "allow MCP tools to navigate to local/internal addresses (off by default)")
	mcpCmd.Flags().StringSlice("allow-target", nil, "allow a specific host or CIDR as an MCP navigation target (repeatable)")
}
