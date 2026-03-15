package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerBrowserTools adds navigation and interaction tools.
func registerBrowserTools(server *mcp.Server, state *mcpState) { //nolint:maintidx // tool registration function is necessarily long
	addTracedTool(server, &mcp.Tool{
		Name:        "navigate",
		Description: "Navigate the browser to a URL",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to navigate to"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.Navigate(args.URL); err != nil {
			return errResult(fmt.Sprintf("scout: navigate to %s: %s", args.URL, err))
		}

		// Best-effort WaitLoad with 15s timeout — SPAs may never fire the load event.
		done := make(chan struct{})

		go func() {
			_ = page.WaitLoad()

			close(done)
		}()

		select {
		case <-done:
		case <-time.After(15 * time.Second):
		}

		title, _ := page.Title()
		url, _ := page.URL()

		return textResult(fmt.Sprintf("Navigated to %s (%s)", url, title))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "click",
		Description: "Click an element by CSS selector",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"}},"required":["selector"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		el, err := page.Element(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		if err := el.Click(); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("Clicked %s", args.Selector))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "type",
		Description: "Type text into an element by CSS selector",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"},"text":{"type":"string","description":"text to type"}},"required":["selector","text"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
			Text     string `json:"text"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		el, err := page.Element(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		if err := el.Input(args.Text); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("Typed into %s", args.Selector))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "extract",
		Description: "Extract text from an element by CSS selector",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"}},"required":["selector"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		el, err := page.Element(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		text, err := el.Text()
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(text)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "eval",
		Description: "Evaluate JavaScript in the page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string","description":"JavaScript expression"}},"required":["expression"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		result, err := page.Eval(args.Expression)
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(result.String())
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "back",
		Description: "Navigate back in browser history",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.NavigateBack(); err != nil {
			return errResult(err.Error())
		}

		return textResult("Navigated back")
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "forward",
		Description: "Navigate forward in browser history",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.NavigateForward(); err != nil {
			return errResult(err.Error())
		}

		return textResult("Navigated forward")
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "wait",
		Description: "Wait for a page condition (load, selector)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector to wait for"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
		}

		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if args.Selector != "" {
			if _, err := page.WaitSelector(args.Selector); err != nil {
				return errResult(err.Error())
			}

			return textResult(fmt.Sprintf("Found %s", args.Selector))
		}

		if err := page.WaitLoad(); err != nil {
			return errResult(err.Error())
		}

		return textResult("Page loaded")
	})
}
