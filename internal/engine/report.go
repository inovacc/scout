package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReportType identifies the kind of report.
type ReportType string

const (
	ReportHealthCheck ReportType = "health_check"
)

// Report wraps a health check (or future report types) with metadata.
type Report struct {
	ID        string        `json:"id"`
	Type      ReportType    `json:"type"`
	URL       string        `json:"url"`
	CreatedAt time.Time     `json:"created_at"`
	Health    *HealthReport `json:"health,omitempty"`
}

// ReportsDir returns the base directory for reports: ~/.scout/reports.
func ReportsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "scout", "reports")
	}

	return filepath.Join(home, ".scout", "reports")
}

// SaveReport persists a report to ~/.scout/reports/{uuidv7}.txt as a
// structured, AI-consumable document with context, findings, and instructions.
// Returns the report ID.
func SaveReport(r *Report) (string, error) {
	if r.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("scout: report: generate id: %w", err)
		}

		r.ID = id.String()
	}

	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}

	dir := ReportsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scout: report: create dir: %w", err)
	}

	content := renderReport(r)

	path := filepath.Join(dir, r.ID+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("scout: report: write: %w", err)
	}

	return r.ID, nil
}

// renderReport produces a structured text document designed to be used as
// an AI prompt. It includes context, raw data, analysis instructions, and
// suggested actions so another AI can process and act on the findings.
func renderReport(r *Report) string {
	var b strings.Builder

	b.WriteString("# Scout Report\n\n")
	b.WriteString("## Metadata\n\n")
	fmt.Fprintf(&b, "- **Report ID:** %s\n", r.ID)
	fmt.Fprintf(&b, "- **Type:** %s\n", r.Type)
	fmt.Fprintf(&b, "- **Target URL:** %s\n", r.URL)
	fmt.Fprintf(&b, "- **Generated:** %s\n", r.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Tool:** Scout (browser automation health checker)\n")
	b.WriteString("\n---\n\n")

	if r.Health != nil {
		renderHealthReport(&b, r)
	}

	// Append raw JSON for machine parsing.
	b.WriteString("## Raw Data (JSON)\n\n")
	b.WriteString("```json\n")

	raw, _ := json.MarshalIndent(r, "", "  ")

	b.Write(raw)
	b.WriteString("\n```\n")

	return b.String()
}

func renderHealthReport(b *strings.Builder, r *Report) {
	h := r.Health

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(b, "- **Pages crawled:** %d\n", h.Pages)
	fmt.Fprintf(b, "- **Duration:** %s\n", h.Duration)
	fmt.Fprintf(b, "- **Errors:** %d\n", h.Summary["error"])
	fmt.Fprintf(b, "- **Warnings:** %d\n", h.Summary["warning"])
	fmt.Fprintf(b, "- **Info:** %d\n", h.Summary["info"])

	total := h.Summary["error"] + h.Summary["warning"] + h.Summary["info"]
	if total == 0 {
		b.WriteString("\nNo issues found. The site appears healthy.\n")
	}

	b.WriteString("\n---\n\n")

	if len(h.Issues) > 0 {
		b.WriteString("## Issues Found\n\n")

		// Group by severity.
		grouped := map[string][]HealthIssue{}
		for _, issue := range h.Issues {
			grouped[issue.Severity] = append(grouped[issue.Severity], issue)
		}

		for _, severity := range []string{"error", "warning", "info"} {
			issues := grouped[severity]
			if len(issues) == 0 {
				continue
			}

			fmt.Fprintf(b, "### %s (%d)\n\n", strings.ToUpper(severity), len(issues))

			for i, issue := range issues {
				fmt.Fprintf(b, "%d. **[%s]** `%s`\n", i+1, issue.Source, issue.URL)
				fmt.Fprintf(b, "   - Message: %s\n", issue.Message)

				if issue.StatusCode > 0 {
					fmt.Fprintf(b, "   - HTTP Status: %d\n", issue.StatusCode)
				}

				if issue.Location != "" {
					fmt.Fprintf(b, "   - Location: %s\n", issue.Location)
				}

				b.WriteString("\n")
			}
		}

		b.WriteString("---\n\n")
	}

	// Instructions for AI processing.
	b.WriteString("## Instructions\n\n")
	b.WriteString("You are reviewing a health check report for a website. Analyze the issues above and provide:\n\n")
	b.WriteString("1. **Root Cause Analysis** — For each error/warning, explain the likely cause.\n")
	b.WriteString("2. **Priority Ranking** — Rank issues by impact (critical, high, medium, low).\n")
	b.WriteString("3. **Recommended Fixes** — Specific, actionable steps to resolve each issue.\n")
	b.WriteString("4. **Quick Wins** — Issues that can be fixed immediately with minimal effort.\n")
	b.WriteString("5. **Monitoring Suggestions** — What to watch for to prevent recurrence.\n\n")

	fmt.Fprintf(b, "Target site: %s\n", r.URL)
	fmt.Fprintf(b, "Pages crawled: %d\n", h.Pages)
	fmt.Fprintf(b, "Total issues: %d (errors: %d, warnings: %d, info: %d)\n\n",
		total, h.Summary["error"], h.Summary["warning"], h.Summary["info"])

	if total == 0 {
		b.WriteString("The site has no issues. Confirm the site is healthy and suggest proactive improvements (performance, SEO, accessibility).\n\n")
	}
}

// ReadReport reads a report by ID from ~/.scout/reports/.
// Supports both the new structured text format (extracts JSON from code block)
// and legacy raw JSON format.
func ReadReport(id string) (*Report, error) {
	path := filepath.Join(ReportsDir(), id+".txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: report: read %s: %w", id, err)
	}

	content := string(data)

	// New format: extract JSON from ```json ... ``` code block.
	if start := strings.Index(content, "```json\n"); start >= 0 {
		start += len("```json\n")
		if end := strings.Index(content[start:], "\n```"); end >= 0 {
			data = []byte(content[start : start+end])
		}
	}

	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("scout: report: parse %s: %w", id, err)
	}

	return &r, nil
}

// ReadReportRaw returns the full text content of a report file.
func ReadReportRaw(id string) (string, error) {
	path := filepath.Join(ReportsDir(), id+".txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("scout: report: read %s: %w", id, err)
	}

	return string(data), nil
}

// ListReports returns all reports sorted by creation time (newest first).
func ListReports() ([]Report, error) {
	dir := ReportsDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("scout: report: list: %w", err)
	}

	var reports []Report

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".txt")

		r, err := ReadReport(id)
		if err != nil {
			continue
		}

		reports = append(reports, *r)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})

	return reports, nil
}

// DeleteReport removes a report file by ID.
func DeleteReport(id string) error {
	path := filepath.Join(ReportsDir(), id+".txt")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("scout: report: delete %s: %w", id, err)
	}

	return nil
}
