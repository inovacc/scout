package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerFormTools adds form detection, filling, and submission tools.
func registerFormTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "form_detect",
		Description: "Detect forms on the current page. Optionally target a specific form by CSS selector.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector for a specific form element"}}}`),
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

		if args.Selector != "" {
			form, err := page.DetectForm(args.Selector)
			if err != nil {
				return errResult(err.Error())
			}

			return jsonResult(form)
		}

		forms, err := page.DetectForms()
		if err != nil {
			return errResult(err.Error())
		}

		return jsonResult(forms)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "form_fill",
		Description: "Fill a form with the provided field name-value pairs",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector for the form (default: form)"},"data":{"type":"object","description":"field name to value mapping","additionalProperties":{"type":"string"}}},"required":["data"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string            `json:"selector"`
			Data     map[string]string `json:"data"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Selector == "" {
			args.Selector = "form"
		}

		if len(args.Data) == 0 {
			return errResult("data is required and must contain at least one field")
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		form, err := page.DetectForm(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		if err := form.Fill(args.Data); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("filled %d field(s) in %s", len(args.Data), args.Selector))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "form_submit",
		Description: "Submit a form on the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector for the form (default: form)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Selector == "" {
			args.Selector = "form"
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		form, err := page.DetectForm(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		if err := form.Submit(); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("submitted form %s", args.Selector))
	})
}
