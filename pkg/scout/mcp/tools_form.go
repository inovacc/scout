package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerFormTools exposes form_detect. form_fill + form_submit are
// follow-up work: they need typed-data passing (map[string]string) and
// post-submit assertion semantics that warrant their own design pass.
func registerFormTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "form_detect",
		Description: "Navigate to a URL and return every detected form (or one matching a selector). " +
			"Returns form action, method, and per-field metadata (name, type, value, required). " +
			"Use before form_fill to introspect what data to send.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["url"],
			"properties":{
				"url":      {"type":"string","description":"URL of the page to inspect"},
				"selector":{"type":"string","description":"CSS selector to narrow to a specific form (default: all forms)"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.FormDetectInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("form_detect: parse args: " + err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.FormDetect(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		return jsonResult(out)
	})
}
