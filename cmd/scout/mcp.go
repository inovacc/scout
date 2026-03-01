package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

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
Use --install --global to register via "claude mcp add" (global Claude Code config).
Use --sse to start with HTTP+SSE transport instead of stdio (default addr: localhost:8080).
Use --addr to customize the SSE listen address.

Tools (33):
  Browser:     navigate, click, type, back, forward, wait, screenshot, snapshot, extract, eval, open
  Content:     markdown, table, meta, pdf, search, fetch
  Network:     cookie, header, block, ping, curl
  Forms:       form_detect, form_fill, form_submit
  Analysis:    crawl, detect
  Inspection:  storage, hijack, har, swagger
  Session:     session_list, session_reset
Resources: scout://page/markdown, scout://page/url, scout://page/title

Subcommands:
  scout mcp screenshot <url>  Take a screenshot and save to file
  scout mcp open <url>        Open URL in headed browser for inspection`,
	RunE: func(cmd *cobra.Command, args []string) error {
		install, _ := cmd.Flags().GetBool("install")
		if install {
			global, _ := cmd.Flags().GetBool("global")

			cfg := mcpConfig{
				MCPServers: map[string]mcpServerConfig{
					"scout": {
						Command: "scout",
						Args:    []string{"mcp"},
					},
				},
			}

			if global {
				// Use `claude mcp add` to register globally.
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

		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		headless, _ := cmd.Flags().GetBool("headless")
		stealth, _ := cmd.Flags().GetBool("stealth")
		useSSE, _ := cmd.Flags().GetBool("sse")
		addr, _ := cmd.Flags().GetString("addr")

		if useSSE {
			return scoutmcp.ServeSSE(context.Background(), logger, addr, headless, stealth)
		}
		return scoutmcp.Serve(context.Background(), logger, headless, stealth)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().BoolP("install", "i", false, "Generate and write .mcp.json to current directory")
	mcpCmd.Flags().BoolP("global", "g", false, "Write to ~/.claude/mcp.json (use with --install)")
	mcpCmd.Flags().Bool("sse", false, "Use HTTP+SSE transport instead of stdio")
	mcpCmd.Flags().String("addr", "localhost:8080", "Listen address for SSE transport")
}
