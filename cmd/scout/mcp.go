package main

import (
	"context"
	"log/slog"
	"os"

	scoutmcp "github.com/inovacc/scout/pkg/scout/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for LLM browser control via stdio",
	Long: `Start a Model Context Protocol server that exposes Scout browser
automation capabilities as MCP tools. Communicates via stdio (JSON-RPC).

Tools: navigate, click, type, screenshot, snapshot, extract, eval, back, forward, wait
Resources: scout://page/markdown, scout://page/url, scout://page/title`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		headless, _ := cmd.Flags().GetBool("headless")
		stealth, _ := cmd.Flags().GetBool("stealth")
		return scoutmcp.Serve(context.Background(), logger, headless, stealth)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
