package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout/guide"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerGuideTools adds step-by-step guide recording tools.
func registerGuideTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "guide_start",
		Description: "Start recording a step-by-step guide. Navigates to the URL and captures an initial screenshot as step 0.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to navigate to"},"title":{"type":"string","description":"guide title (defaults to page title)"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		}

		_ = json.Unmarshal(req.Params.Arguments, &args)

		if args.URL == "" {
			return errResult("scout-mcp: guide_start: url is required")
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		// Navigate to the URL.
		if err := page.Navigate(args.URL); err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_start: navigate: %s", err))
		}

		if err := page.WaitLoad(); err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_start: wait load: %s", err))
		}

		title := args.Title
		if title == "" {
			t, err := page.Title()
			if err == nil && t != "" {
				title = t
			} else {
				title = "Untitled Guide"
			}
		}

		if err := state.guideRecorder.Start(title, args.URL); err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_start: %s", err))
		}

		// Take initial screenshot as step 0.
		screenshot, err := page.Screenshot()
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_start: screenshot: %s", err))
		}

		pageURL, _ := page.URL()
		pageTitle, _ := page.Title()

		if err := state.guideRecorder.AddStep(pageURL, pageTitle, "Initial page", screenshot); err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_start: add step: %s", err))
		}

		return textResult(fmt.Sprintf("Guide '%s' started. Use guide_step to add steps, guide_finish to complete.", title))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "guide_step",
		Description: "Record a step in the current guide. Captures the current page URL, title, and screenshot.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"annotation":{"type":"string","description":"description of what this step does"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Annotation string `json:"annotation"`
		}

		_ = json.Unmarshal(req.Params.Arguments, &args)

		if !state.guideRecorder.IsRecording() {
			return errResult("scout-mcp: guide_step: no guide recording in progress (call guide_start first)")
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		screenshot, err := page.Screenshot()
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_step: screenshot: %s", err))
		}

		pageURL, _ := page.URL()
		pageTitle, _ := page.Title()

		if err := state.guideRecorder.AddStep(pageURL, pageTitle, args.Annotation, screenshot); err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_step: %s", err))
		}

		return textResult(fmt.Sprintf("Step recorded: %s (%s)", pageTitle, pageURL))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "guide_finish",
		Description: "Finish recording the guide and return the rendered markdown with embedded screenshots.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		g, err := state.guideRecorder.Finish()
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_finish: %s", err))
		}

		md, err := guide.RenderMarkdown(g)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: guide_finish: render: %s", err))
		}

		return textResult(string(md))
	})
}
