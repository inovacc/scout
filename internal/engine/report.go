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
	ID        string       `json:"id"`
	Type      ReportType   `json:"type"`
	URL       string       `json:"url"`
	CreatedAt time.Time    `json:"created_at"`
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

// SaveReport persists a report to ~/.scout/reports/{uuidv7}.txt as JSON.
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

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("scout: report: marshal: %w", err)
	}

	path := filepath.Join(dir, r.ID+".txt")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("scout: report: write: %w", err)
	}

	return r.ID, nil
}

// ReadReport reads a report by ID from ~/.scout/reports/.
func ReadReport(id string) (*Report, error) {
	path := filepath.Join(ReportsDir(), id+".txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: report: read %s: %w", id, err)
	}

	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("scout: report: parse %s: %w", id, err)
	}

	return &r, nil
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
