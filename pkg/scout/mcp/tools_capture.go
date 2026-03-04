package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerCaptureTools adds screenshot, snapshot, and PDF tools.
func registerCaptureTools(server *mcp.Server, state *mcpState) {
	server.AddTool(&mcp.Tool{
		Name:        "screenshot",
		Description: "Take a screenshot of the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"fullPage":{"type":"boolean","description":"capture full page"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			FullPage bool `json:"fullPage"`
		}

		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var data []byte
		if args.FullPage {
			data, err = page.FullScreenshot()
		} else {
			data, err = page.Screenshot()
		}

		if err != nil {
			return errResult(err.Error())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				MIMEType: "image/png",
				Data:     data,
			}},
		}, nil
	})

	server.AddTool(&mcp.Tool{
		Name:        "snapshot",
		Description: "Get the accessibility tree of the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"interactableOnly":{"type":"boolean","description":"only include interactable elements"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			InteractableOnly bool `json:"interactableOnly"`
		}

		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.SnapshotOption
		if args.InteractableOnly {
			opts = append(opts, scout.WithSnapshotInteractableOnly())
		}

		snap, err := page.SnapshotWithOptions(opts...)
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(snap)
	})

	server.AddTool(&mcp.Tool{
		Name:        "pdf",
		Description: "Generate a PDF of the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"landscape":{"type":"boolean","description":"landscape orientation"},"printBackground":{"type":"boolean","description":"print background graphics"},"scale":{"type":"number","description":"scale factor (0.1 to 2.0)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Landscape       bool    `json:"landscape"`
			PrintBackground bool    `json:"printBackground"`
			Scale           float64 `json:"scale"`
		}

		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var data []byte
		if args.Landscape || args.PrintBackground || args.Scale > 0 {
			data, err = page.PDFWithOptions(scout.PDFOptions{
				Landscape:       args.Landscape,
				PrintBackground: args.PrintBackground,
				Scale:           args.Scale,
			})
		} else {
			data, err = page.PDF()
		}

		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: pdf: %s", err))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				MIMEType: "application/pdf",
				Data:     data,
			}},
		}, nil
	})
}
