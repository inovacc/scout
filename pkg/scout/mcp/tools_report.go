package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deprecated: registerReportTools adds report management tools.
// These tools are now available as the scout-reports plugin. Built-in versions will be
// removed after 2026-04-16. Install: scout plugin install ./plugins/scout-reports
func registerReportTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "report_list",
		Description: "List all saved reports (health checks, gathers, crawls) with ID, type, URL, issues count, and creation date",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()

		reports, err := scout.ListReports()
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: list reports: %s", err))
		}

		if len(reports) == 0 {
			return textResult("No reports found.")
		}

		type reportSummary struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			URL       string `json:"url"`
			Issues    int    `json:"issues"`
			CreatedAt string `json:"created_at"`
		}

		summaries := make([]reportSummary, 0, len(reports))
		for _, r := range reports {
			issues := countIssues(&r)
			summaries = append(summaries, reportSummary{
				ID:        r.ID,
				Type:      string(r.Type),
				URL:       r.URL,
				Issues:    issues,
				CreatedAt: r.CreatedAt.Format(time.RFC3339),
			})
		}

		return jsonResult(summaries)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "report_show",
		Description: "Show the full AI-consumable text of a saved report by ID",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Report ID (UUID)"}},"required":["id"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()

		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.ID == "" {
			return errResult("id is required")
		}

		content, err := scout.ReadReportRaw(args.ID)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: read report: %s", err))
		}

		return textResult(content)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "report_delete",
		Description: "Delete a saved report by ID",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Report ID (UUID)"}},"required":["id"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()

		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.ID == "" {
			return errResult("id is required")
		}

		if err := scout.DeleteReport(args.ID); err != nil {
			return errResult(fmt.Sprintf("scout-mcp: delete report: %s", err))
		}

		return textResult(fmt.Sprintf("Report %s deleted.", args.ID))
	})
}

// countIssues returns the number of issues in a report based on its type.
func countIssues(r *scout.Report) int {
	switch {
	case r.Health != nil:
		return len(r.Health.Issues)
	case r.Crawl != nil:
		return len(r.Crawl.Errors)
	default:
		return 0
	}
}
