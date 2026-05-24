package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerRunbookTools exposes runbook_plan + runbook_apply. Both load
// the runbook file server-side, so the AI agent only passes the absolute
// path.
func registerRunbookTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "runbook_plan",
		Description: "Dry-run a runbook against the live page: resolves every selector, " +
			"classifies each step as ok/warn/fail, reports without executing side-effects. " +
			"Use this BEFORE runbook_apply to validate the runbook will work on the current page.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["file"],
			"properties":{"file":{"type":"string","description":"absolute path to the runbook JSON/YAML file"}}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.RunbookPlanInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("runbook_plan: parse args: " + err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.RunbookPlan(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		return jsonResult(out)
	})

	addTracedTool(server, &mcp.Tool{
		Name: "runbook_apply",
		Description: "Execute a runbook against the live browser. Performs side-effects (clicks, navigation, form fills, etc.). " +
			"Recommend calling runbook_plan first to verify the runbook resolves cleanly on the target page.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["file"],
			"properties":{"file":{"type":"string","description":"absolute path to the runbook JSON/YAML file"}}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.RunbookApplyInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("runbook_apply: parse args: " + err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.RunbookApply(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		return jsonResult(out)
	})
}
