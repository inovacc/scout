package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// overrideReportsDir sets ReportsDir to return dir and returns a cleanup func.
func overrideReportsDir(t *testing.T, dir string) {
	t.Helper()

	orig := ReportsDir
	ReportsDir = func() string { return dir }

	t.Cleanup(func() { ReportsDir = orig })
}

func TestSaveReadReport(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	r := &Report{
		Type: ReportHealthCheck,
		URL:  "https://example.com",
		Health: &HealthReport{
			URL:      "https://example.com",
			Pages:    3,
			Duration: "1.5s",
			Issues: []HealthIssue{
				{URL: "https://example.com/a", Source: "link", Severity: "error", Message: "broken"},
			},
			Summary: map[string]int{"error": 1, "warning": 0, "info": 0},
		},
	}

	id, err := SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	if id == "" {
		t.Fatal("SaveReport returned empty ID")
	}

	got, err := ReadReport(id)
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID = %q, want %q", got.ID, id)
	}

	if got.Type != ReportHealthCheck {
		t.Errorf("Type = %q, want %q", got.Type, ReportHealthCheck)
	}

	if got.URL != "https://example.com" {
		t.Errorf("URL = %q, want https://example.com", got.URL)
	}

	if got.Health == nil {
		t.Fatal("Health is nil")
	}

	if got.Health.Pages != 3 {
		t.Errorf("Health.Pages = %d, want 3", got.Health.Pages)
	}

	if len(got.Health.Issues) != 1 {
		t.Errorf("Health.Issues count = %d, want 1", len(got.Health.Issues))
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestSaveReadReportRaw(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	r := &Report{
		Type: ReportHealthCheck,
		URL:  "https://example.com",
		Health: &HealthReport{
			URL:      "https://example.com",
			Pages:    1,
			Duration: "500ms",
			Issues:   nil,
			Summary:  map[string]int{"error": 0, "warning": 0, "info": 0},
		},
	}

	id, err := SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	raw, err := ReadReportRaw(id)
	if err != nil {
		t.Fatalf("ReadReportRaw: %v", err)
	}

	if !strings.Contains(raw, "# Scout Report") {
		t.Error("raw does not contain markdown header")
	}

	if !strings.Contains(raw, "## Metadata") {
		t.Error("raw does not contain Metadata section")
	}

	if !strings.Contains(raw, "```json") {
		t.Error("raw does not contain JSON code block")
	}

	if !strings.Contains(raw, id) {
		t.Error("raw does not contain report ID")
	}
}

func TestListReports(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	var ids []string

	for i := 0; i < 3; i++ {
		r := &Report{
			Type:      ReportHealthCheck,
			URL:       "https://example.com",
			CreatedAt: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Health: &HealthReport{
				URL:      "https://example.com",
				Pages:    1,
				Duration: "1s",
				Summary:  map[string]int{"error": 0, "warning": 0, "info": 0},
			},
		}

		id, err := SaveReport(r)
		if err != nil {
			t.Fatalf("SaveReport[%d]: %v", i, err)
		}

		ids = append(ids, id)
	}

	reports, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}

	if len(reports) != 3 {
		t.Fatalf("ListReports count = %d, want 3", len(reports))
	}

	// Newest first.
	for i := 1; i < len(reports); i++ {
		if reports[i].CreatedAt.After(reports[i-1].CreatedAt) {
			t.Errorf("reports[%d].CreatedAt (%v) is after reports[%d].CreatedAt (%v)",
				i, reports[i].CreatedAt, i-1, reports[i-1].CreatedAt)
		}
	}
}

func TestDeleteReport(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	r := &Report{
		Type: ReportGather,
		URL:  "https://example.com",
		Gather: &GatherResult{
			URL:      "https://example.com",
			Title:    "Example",
			Duration: "1s",
		},
	}

	id, err := SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	if err := DeleteReport(id); err != nil {
		t.Fatalf("DeleteReport: %v", err)
	}

	_, err = ReadReport(id)
	if err == nil {
		t.Fatal("ReadReport should fail after delete")
	}
}

func TestDeleteReportNotFound(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	err := DeleteReport("nonexistent-id")
	if err == nil {
		t.Fatal("DeleteReport(nonexistent) should return error")
	}
}

func TestReadReportNotFound(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	_, err := ReadReport("nonexistent-id")
	if err == nil {
		t.Fatal("ReadReport(nonexistent) should return error")
	}
}

func TestRenderHealthReport(t *testing.T) {
	r := &Report{
		ID:        "test-health-id",
		Type:      ReportHealthCheck,
		URL:       "https://example.com",
		CreatedAt: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		Health: &HealthReport{
			URL:      "https://example.com",
			Pages:    5,
			Duration: "2.3s",
			Issues: []HealthIssue{
				{
					URL:        "https://example.com/broken",
					Source:     "link",
					Severity:   "error",
					Message:    "page load failed: 404",
					StatusCode: 404,
				},
				{
					URL:      "https://example.com",
					Source:   "console",
					Severity: "warning",
					Message:  "deprecated API usage",
				},
				{
					URL:      "https://example.com",
					Source:   "js_exception",
					Severity: "error",
					Message:  "Uncaught TypeError",
					Location: "line 42, col 10",
				},
			},
			Summary: map[string]int{"error": 2, "warning": 1, "info": 0},
		},
	}

	text := renderReport(r)

	checks := []struct {
		label string
		sub   string
	}{
		{"report ID", "test-health-id"},
		{"type", "health_check"},
		{"URL", "https://example.com"},
		{"metadata section", "## Metadata"},
		{"summary section", "## Summary"},
		{"issues section", "## Issues Found"},
		{"instructions section", "## Instructions"},
		{"error severity", "ERROR (2)"},
		{"warning severity", "WARNING (1)"},
		{"broken link", "page load failed: 404"},
		{"console warning", "deprecated API usage"},
		{"js exception", "Uncaught TypeError"},
		{"location", "line 42, col 10"},
		{"HTTP status", "404"},
		{"JSON block", "```json"},
		{"pages crawled", "Pages crawled:** 5"},
		{"root cause instruction", "Root Cause Analysis"},
	}

	for _, c := range checks {
		if !strings.Contains(text, c.sub) {
			t.Errorf("rendered text missing %s (%q)", c.label, c.sub)
		}
	}
}

func TestRenderHealthReportNoIssues(t *testing.T) {
	r := &Report{
		ID:        "test-no-issues",
		Type:      ReportHealthCheck,
		URL:       "https://healthy.example.com",
		CreatedAt: time.Now(),
		Health: &HealthReport{
			URL:      "https://healthy.example.com",
			Pages:    2,
			Duration: "1s",
			Issues:   nil,
			Summary:  map[string]int{"error": 0, "warning": 0, "info": 0},
		},
	}

	text := renderReport(r)

	if !strings.Contains(text, "No issues found") {
		t.Error("rendered text should contain 'No issues found' for healthy site")
	}

	if !strings.Contains(text, "proactive improvements") {
		t.Error("rendered text should contain proactive improvements suggestion")
	}
}

func TestRenderGatherReport(t *testing.T) {
	r := &Report{
		ID:        "test-gather-id",
		Type:      ReportGather,
		URL:       "https://example.com",
		CreatedAt: time.Now(),
		Gather: &GatherResult{
			URL:      "https://example.com",
			Title:    "Example Domain",
			Duration: "3s",
			Links:    []string{"https://example.com/about", "https://example.com/contact"},
			Frameworks: []FrameworkInfo{
				{Name: "React", Version: "18.2.0", SPA: true},
				{Name: "Next.js", Version: "14.0.0"},
			},
			ConsoleLog: []string{"[log] app initialized"},
		},
	}

	text := renderReport(r)

	checks := []struct {
		label string
		sub   string
	}{
		{"page title", "Example Domain"},
		{"link count", "Links found:** 2"},
		{"framework count", "Frameworks detected:** 2"},
		{"React", "React"},
		{"React version", "18.2.0"},
		{"Next.js", "Next.js"},
		{"link URL", "https://example.com/about"},
		{"console message", "app initialized"},
		{"instructions", "## Instructions"},
		{"SEO suggestion", "SEO Suggestions"},
		{"JSON block", "```json"},
	}

	for _, c := range checks {
		if !strings.Contains(text, c.sub) {
			t.Errorf("rendered text missing %s (%q)", c.label, c.sub)
		}
	}
}

func TestRenderCrawlReport(t *testing.T) {
	r := &Report{
		ID:        "test-crawl-id",
		Type:      ReportCrawl,
		URL:       "https://example.com",
		CreatedAt: time.Now(),
		Crawl: &CrawlReport{
			URL:      "https://example.com",
			Pages:    10,
			Duration: "5s",
			Links:    []string{"https://example.com/a", "https://example.com/b"},
			Errors:   []string{"timeout on /slow", "404 on /missing"},
		},
	}

	text := renderReport(r)

	checks := []struct {
		label string
		sub   string
	}{
		{"start URL", "https://example.com"},
		{"pages crawled", "Pages crawled:** 10"},
		{"links discovered", "Links discovered:** 2"},
		{"errors encountered", "Errors encountered:** 2"},
		{"error detail", "timeout on /slow"},
		{"error detail 2", "404 on /missing"},
		{"link", "https://example.com/a"},
		{"errors section", "### Errors"},
		{"links section", "### Discovered Links"},
		{"instructions", "## Instructions"},
		{"broken link instruction", "Broken Link Identification"},
		{"JSON block", "```json"},
	}

	for _, c := range checks {
		if !strings.Contains(text, c.sub) {
			t.Errorf("rendered text missing %s (%q)", c.label, c.sub)
		}
	}
}

func TestRenderCrawlReportNoErrors(t *testing.T) {
	r := &Report{
		ID:        "test-crawl-clean",
		Type:      ReportCrawl,
		URL:       "https://example.com",
		CreatedAt: time.Now(),
		Crawl: &CrawlReport{
			URL:      "https://example.com",
			Pages:    5,
			Duration: "2s",
			Links:    []string{"https://example.com/page1"},
			Errors:   nil,
		},
	}

	text := renderReport(r)

	if !strings.Contains(text, "No errors found during crawl") {
		t.Error("rendered text should note no errors for clean crawl")
	}
}

func TestListReportsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	reports, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}

	if len(reports) != 0 {
		t.Errorf("ListReports on empty dir = %d, want 0", len(reports))
	}
}

func TestListReportsNonexistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	overrideReportsDir(t, dir)

	reports, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports on nonexistent dir should return nil, got: %v", err)
	}

	if reports != nil {
		t.Errorf("ListReports on nonexistent dir = %v, want nil", reports)
	}
}

func TestReadReportLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	overrideReportsDir(t, dir)

	// Write raw JSON directly (legacy format, no markdown wrapper).
	r := &Report{
		ID:        "legacy-test-id",
		Type:      ReportHealthCheck,
		URL:       "https://legacy.example.com",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Health: &HealthReport{
			URL:      "https://legacy.example.com",
			Pages:    1,
			Duration: "1s",
			Summary:  map[string]int{"error": 0, "warning": 0, "info": 0},
		},
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	path := filepath.Join(dir, "legacy-test-id.txt")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadReport("legacy-test-id")
	if err != nil {
		t.Fatalf("ReadReport(legacy): %v", err)
	}

	if got.ID != "legacy-test-id" {
		t.Errorf("ID = %q, want legacy-test-id", got.ID)
	}

	if got.URL != "https://legacy.example.com" {
		t.Errorf("URL = %q, want https://legacy.example.com", got.URL)
	}

	if got.Health == nil {
		t.Fatal("Health is nil for legacy report")
	}
}

func TestReportsDir(t *testing.T) {
	dir := ReportsDir()
	if dir == "" {
		t.Fatal("ReportsDir() returned empty string")
	}

	if !strings.Contains(dir, "reports") {
		t.Errorf("ReportsDir() = %q, expected to contain 'reports'", dir)
	}
}

func TestReportsDirOverride(t *testing.T) {
	custom := "/custom/reports/path"
	overrideReportsDir(t, custom)

	if got := ReportsDir(); got != custom {
		t.Errorf("ReportsDir() = %q after override, want %q", got, custom)
	}
}
