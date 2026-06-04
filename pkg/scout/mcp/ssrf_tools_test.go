package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCrawlSitemapToolsSSRFGate proves the crawl and sitemap MCP tools run the
// SSRF url-policy gate (state.checkURL) before touching the browser — matching
// every sibling URL-ingress tool. It builds a server with block-by-default
// policy (no SCOUT_ALLOW_LOCAL_TARGETS), so a cloud-metadata IP is rejected and
// the tool returns an error result without ever launching Chromium (the gate
// sits ahead of ensureBrowser).
//
// Note: this test deliberately does NOT use connectTestClient — that helper
// forces SCOUT_ALLOW_LOCAL_TARGETS=true, which short-circuits Policy.Check and
// would mask the gate.
func TestCrawlSitemapToolsSSRFGate(t *testing.T) {
	t.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "") // force block-by-default, ignore ambient env

	server := NewServer(ServerConfig{Headless: true, Logger: slog.Default()})
	client := mcp.NewClient(&mcp.Implementation{Name: "ssrf-test", Version: "1.0.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()

	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() { _ = cs.Close() })

	for _, tool := range []string{"crawl", "sitemap"} {
		result, err := callTool(ctx, cs, tool, map[string]any{"url": "http://169.254.169.254/latest/meta-data/"})
		if err != nil {
			t.Fatalf("%s: transport error: %v", tool, err)
		}
		if !result.IsError {
			t.Fatalf("%s(metadata IP) returned ok, want SSRF-blocked error result", tool)
		}

		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "blocked") {
			t.Errorf("%s(metadata IP) error = %q, want a 'blocked' SSRF-gate error", tool, text)
		}
	}
}
