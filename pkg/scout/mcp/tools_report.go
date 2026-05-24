package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerReportTools exposes report_list / report_show / report_delete.
// No browser needed — these are pure filesystem ops over ~/.scout/reports/.
func registerReportTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "report_list",
		Description: "List every saved report under ~/.scout/reports/. Returns summary metadata; use report_show to fetch the body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()
		out, err := tools.ReportList(ctx, tools.ReportListInput{})
		if err != nil {
			return errResult(err.Error())
		}
		return jsonResult(out)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "report_show",
		Description: "Load one saved report by ID. Pass raw=true for the AI-consumable markdown body; otherwise returns the structured Report.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["id"],
			"properties":{
				"id": {"type":"string","description":"report UUID returned by report_list"},
				"raw":{"type":"boolean","description":"return rendered markdown body instead of the structured Report"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()
		var in tools.ReportShowInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("report_show: parse args: " + err.Error())
		}
		out, err := tools.ReportShow(ctx, in)
		if err != nil {
			return errResult(err.Error())
		}
		// Raw → return as text content for easier AI consumption
		if in.Raw {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: out.Raw}},
			}, nil
		}
		return jsonResult(out)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "report_delete",
		Description: "Delete one saved report by ID. Irreversible.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["id"],
			"properties":{"id":{"type":"string","description":"report UUID to delete"}}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()
		var in tools.ReportDeleteInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("report_delete: parse args: " + err.Error())
		}
		out, err := tools.ReportDelete(ctx, in)
		if err != nil {
			return errResult(err.Error())
		}
		return jsonResult(out)
	})
}

// jsonResult is defined in server.go.
