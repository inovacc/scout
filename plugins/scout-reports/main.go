// scout-reports is a Scout plugin providing report management MCP tools.
//
// Install: scout plugin install ./plugins/scout-reports
// Or build: go build -o scout-reports ./plugins/scout-reports
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("report_list", sdk.ToolHandlerFunc(handleReportList))
	srv.RegisterTool("report_show", sdk.ToolHandlerFunc(handleReportShow))
	srv.RegisterTool("report_delete", sdk.ToolHandlerFunc(handleReportDelete))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func handleReportList(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
	reports, err := scout.ListReports()
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("list reports: %s", err)), nil
	}

	if len(reports) == 0 {
		return sdk.TextResult("No reports found."), nil
	}

	type summary struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		URL       string `json:"url"`
		Issues    int    `json:"issues"`
		CreatedAt string `json:"created_at"`
	}

	summaries := make([]summary, 0, len(reports))
	for _, r := range reports {
		issues := countIssues(&r)
		summaries = append(summaries, summary{
			ID:        r.ID,
			Type:      string(r.Type),
			URL:       r.URL,
			Issues:    issues,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}

	b, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}

func handleReportShow(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return sdk.ErrorResult("id is required"), nil
	}

	content, err := scout.ReadReportRaw(id)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("read report: %s", err)), nil
	}

	return sdk.TextResult(content), nil
}

func handleReportDelete(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return sdk.ErrorResult("id is required"), nil
	}

	if err := scout.DeleteReport(id); err != nil {
		return sdk.ErrorResult(fmt.Sprintf("delete report: %s", err)), nil
	}

	return sdk.TextResult(fmt.Sprintf("Report %s deleted.", id)), nil
}

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
