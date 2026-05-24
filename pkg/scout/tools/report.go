package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// Report tools don't take a *scout.Browser — they're pure filesystem
// reads/writes over ~/.scout/reports/. Signature is (ctx, Input) → Output.

// ReportListInput is empty (placeholder for future filters: type, since, until).
type ReportListInput struct{}

// ReportListOutput is the saved-report summary list.
type ReportListOutput struct {
	Reports []scout.Report `json:"reports"`
	Total   int            `json:"total"`
}

// ReportList returns every saved report under ~/.scout/reports/.
func ReportList(_ context.Context, _ ReportListInput) (*ReportListOutput, error) {
	reports, err := scout.ListReports()
	if err != nil {
		return nil, fmt.Errorf("tools: report-list: %w", err)
	}
	return &ReportListOutput{Reports: reports, Total: len(reports)}, nil
}

// ReportShowInput targets a single report by ID.
type ReportShowInput struct {
	ID  string `json:"id"            jsonschema:"the report UUID returned by report_list"`
	Raw bool   `json:"raw,omitempty" jsonschema:"return the rendered markdown body instead of the structured Report"`
}

// ReportShowOutput is one of (Report, Raw). At most one is populated.
type ReportShowOutput struct {
	Report *scout.Report `json:"report,omitempty"`
	Raw    string        `json:"raw,omitempty"`
}

// ReportShow loads one saved report. If Raw is set, returns the rendered
// markdown body (AI-consumable); otherwise returns the structured Report.
func ReportShow(_ context.Context, in ReportShowInput) (*ReportShowOutput, error) {
	if in.ID == "" {
		return nil, fmt.Errorf("tools: report-show: id required")
	}
	if in.Raw {
		body, err := scout.ReadReportRaw(in.ID)
		if err != nil {
			return nil, fmt.Errorf("tools: report-show: %w", err)
		}
		return &ReportShowOutput{Raw: body}, nil
	}
	r, err := scout.ReadReport(in.ID)
	if err != nil {
		return nil, fmt.Errorf("tools: report-show: %w", err)
	}
	return &ReportShowOutput{Report: r}, nil
}

// ReportDeleteInput targets a single report by ID.
type ReportDeleteInput struct {
	ID string `json:"id" jsonschema:"the report UUID to delete"`
}

// ReportDeleteOutput confirms deletion.
type ReportDeleteOutput struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// ReportDelete removes one saved report.
func ReportDelete(_ context.Context, in ReportDeleteInput) (*ReportDeleteOutput, error) {
	if in.ID == "" {
		return nil, fmt.Errorf("tools: report-delete: id required")
	}
	if err := scout.DeleteReport(in.ID); err != nil {
		return nil, fmt.Errorf("tools: report-delete: %w", err)
	}
	return &ReportDeleteOutput{ID: in.ID, Deleted: true}, nil
}
