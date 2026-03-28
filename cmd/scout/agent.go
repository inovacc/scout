package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/scout/internal/engine/browser"
	"github.com/inovacc/scout/pkg/scout/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "AI agent integration tools",
	Long:  `Tools for integrating Scout with AI agent frameworks (OpenAI, Anthropic, LangChain, etc.).`,
}

var agentServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server for AI agent tool calls",
	Long: `Start an HTTP server that exposes Scout browser tools as a REST API.

Endpoints:
  GET  /health           Server status and tool count
  GET  /tools            List tools (OpenAI function calling format)
  GET  /tools/openai     List tools (OpenAI format)
  GET  /tools/anthropic  List tools (Anthropic tool_use format)
  GET  /tools/schema     Full JSON schema for all tools
  POST /call             Execute a tool (request-response)
  POST /stream           Execute a tool with SSE streaming (progress events)
  GET  /docs             Swagger UI (interactive API documentation)
  GET  /openapi.yaml     OpenAPI 3.1 specification

The /stream endpoint returns Server-Sent Events with event types:
  start   Tool execution started (includes tool name and timestamp)
  result  Successful result content
  error   Error occurred during execution
  done    Stream ended (status: "complete" or "error")

Examples:
  # Request-response:
  curl -X POST http://localhost:9000/call \
    -H 'Content-Type: application/json' \
    -d '{"name": "navigate", "arguments": {"url": "https://example.com"}}'

  # SSE streaming:
  curl -N -X POST http://localhost:9000/stream \
    -H 'Content-Type: application/json' \
    -d '{"name": "navigate", "arguments": {"url": "https://example.com"}}'`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		headless, _ := cmd.Flags().GetBool("headless")
		stealth, _ := cmd.Flags().GetBool("stealth")
		bin, _ := cmd.Flags().GetString("bin")
		browserType, _ := cmd.Flags().GetString("browser")
		idleTimeout, _ := cmd.Flags().GetDuration("idle-timeout")
		rateLimit, _ := cmd.Flags().GetFloat64("rate-limit")
		apiKey, _ := cmd.Flags().GetString("api-key")
		if apiKey == "" {
			apiKey = os.Getenv("SCOUT_AGENT_API_KEY")
		}

		// Resolve --browser type name to a binary path if --bin is not set.
		if bin == "" && browserType != "" {
			if resolved, err := browser.ResolveCached(context.Background(), browser.BrowserType(browserType)); err == nil {
				bin = resolved
			}
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

		cfg := agent.ServerConfig{
			Addr:        addr,
			Headless:    headless,
			Stealth:     stealth,
			BrowserBin:  bin,
			Logger:      logger,
			IdleTimeout: idleTimeout,
			RateLimit:   rateLimit,
			APIKey:      apiKey,
		}

		srv, err := agent.NewServer(cfg)
		if err != nil {
			return err
		}
		defer srv.Close()

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		return srv.ListenAndServe(ctx, cancel)
	},
}

var agentToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List available agent tools and their schemas",
	RunE: func(cmd *cobra.Command, _ []string) error {
		format, _ := cmd.Flags().GetString("format")

		tools := []string{
			"navigate", "screenshot", "extract_text", "click",
			"type_text", "markdown", "eval", "page_url", "page_title",
		}

		switch format {
		case "anthropic":
			_, _ = fmt.Fprintln(os.Stdout, "Available tools (Anthropic format):")
		default:
			_, _ = fmt.Fprintln(os.Stdout, "Available tools (OpenAI format):")
		}

		for _, t := range tools {
			_, _ = fmt.Fprintf(os.Stdout, "  %s\n", t)
		}

		_, _ = fmt.Fprintln(os.Stdout, "\nStart the server with: scout agent serve")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentServeCmd, agentToolsCmd)

	agentServeCmd.Flags().StringP("addr", "a", "localhost:9000", "HTTP listen address")
	agentServeCmd.Flags().String("bin", "", "Path to browser executable")
	agentServeCmd.Flags().String("browser", "", "Browser type: chrome, brave, edge (resolves to cached binary)")
	agentServeCmd.Flags().Float64("rate-limit", 100, "Max requests per second (0 = unlimited)")
	agentServeCmd.Flags().String("api-key", "", "API key for authentication (empty = no auth, env: SCOUT_AGENT_API_KEY)")

	agentToolsCmd.Flags().String("format", "openai", "Output format: openai, anthropic")
}
