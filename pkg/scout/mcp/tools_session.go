package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerSessionTools adds session management and open tools.
func registerSessionTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "session_list",
		Description: "List current session info (URL, title of current page)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()
		state.mu.Lock()
		hasPage := state.page != nil
		state.mu.Unlock()

		if !hasPage {
			return textResult(`{"status":"no active session"}`)
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		u, _ := page.URL()
		title, _ := page.Title()

		info := map[string]string{
			"status": "active",
			"url":    u,
			"title":  title,
		}
		data, _ := json.Marshal(info)

		return textResult(string(data))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "session_reset",
		Description: "Close the current browser and page, forcing re-initialization on next use",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()
		state.reset()

		return textResult("Session reset")
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "open",
		Description: "Open a URL in a visible (headed) browser for manual inspection. The browser remains open for interactive analysis.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to open"},"devtools":{"type":"boolean","description":"open Chrome DevTools automatically"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL      string `json:"url"`
			DevTools bool   `json:"devtools"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		// Launch a separate headed browser for inspection.
		opts := []scout.Option{
			scout.WithHeadless(false),
			scout.WithNoSandbox(),
			scout.WithTargetURL(args.URL),
		}
		if state.config.Stealth {
			opts = append(opts, scout.WithStealth())
		}

		if args.DevTools {
			opts = append(opts, scout.WithDevTools())
		}

		b, err := scout.New(opts...) //nolint:contextcheck
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: open: %s", err))
		}

		page, err := b.NewPage(args.URL)
		if err != nil {
			_ = b.Close()
			return errResult(fmt.Sprintf("scout-mcp: open: %s", err))
		}

		_ = page.WaitLoad()

		title, _ := page.Title()
		u, _ := page.URL()

		return textResult(fmt.Sprintf("Opened %s (%s) in headed browser. Close the browser window when done.", u, title))
	})
}
