package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func registerAriaTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "browser_snapshot",
		Description: "Capture and store an ARIA snapshot of the current page. " +
			"Returns YAML with [ref=eN] tags identifying interactive elements. " +
			"Use refs from this snapshot in action tools (Phase B).",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(fmt.Sprintf("scout: browser_snapshot: browser unavailable: %v", err))
		}

		snap, err := aria.Capture(ctx, page)
		if err != nil {
			return errResult(fmt.Sprintf("scout: browser_snapshot: capture failed: %v", err))
		}

		state.ariaStore.Put(snap.PageID, snap)
		installInvalidationHooks(page, state)

		var buf bytes.Buffer
		if err := snap.RenderYAML(&buf); err != nil {
			return errResult(fmt.Sprintf("scout: browser_snapshot: render failed: %v", err))
		}

		text := fmt.Sprintf("snapshot_uri=%s snapshot_version=%d\n\n%s",
			snap.URI, snap.Version, buf.String())
		return textResult(text)
	})
}
