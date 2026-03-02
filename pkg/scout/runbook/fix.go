package runbook

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// SampleExtract runs the runbook on the first page only and returns sample items.
// It navigates to the runbook URL, waits for the page to load, then uses the
// runbook's container and field selectors to extract items without pagination.
func SampleExtract(browser *scout.Browser, r *Runbook) ([]map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("runbook: sample: nil runbook")
	}

	if r.URL == "" {
		return nil, fmt.Errorf("runbook: sample: no URL in runbook")
	}

	if r.Items == nil {
		return nil, fmt.Errorf("runbook: sample: no items spec in runbook")
	}

	page, err := browser.NewPage(r.URL)
	if err != nil {
		return nil, fmt.Errorf("runbook: sample: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("runbook: sample: wait load: %w", err)
	}

	if r.WaitFor != "" {
		if _, err := page.Element(r.WaitFor); err != nil {
			return nil, fmt.Errorf("runbook: sample: wait for %q: %w", r.WaitFor, err)
		}
	}

	// Extract from the first page only (no pagination).
	items, err := extractPage(page, r.Items)
	if err != nil {
		return nil, fmt.Errorf("runbook: sample: extract: %w", err)
	}

	// Convert []map[string]string to []map[string]any for a more flexible return type.
	result := make([]map[string]any, len(items))
	for i, item := range items {
		m := make(map[string]any, len(item))
		for k, v := range item {
			m[k] = v
		}

		result[i] = m
	}

	return result, nil
}

// FixRunbook re-analyzes the page and updates broken selectors in the runbook.
// It returns the fixed runbook and a list of human-readable changes made.
func FixRunbook(browser *scout.Browser, r *Runbook) (*Runbook, []string, error) {
	if r == nil {
		return nil, nil, fmt.Errorf("runbook: fix: nil runbook")
	}

	url := r.URL
	if url == "" && r.Type == "automate" {
		for _, s := range r.Steps {
			if s.Action == "navigate" && s.URL != "" {
				url = s.URL
				break
			}
		}
	}

	if url == "" {
		return nil, nil, fmt.Errorf("runbook: fix: no URL to navigate to")
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return nil, nil, fmt.Errorf("runbook: fix: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, nil, fmt.Errorf("runbook: fix: wait load: %w", err)
	}

	// Collect selectors and check which ones are broken.
	selectors := collectSelectors(r)
	counts := SelectorHealthCheck(page, selectors)

	var broken []string

	for name, count := range counts {
		if count == 0 {
			broken = append(broken, name)
		}
	}

	if len(broken) == 0 {
		// All selectors are healthy; return the runbook unchanged.
		return r, nil, nil
	}

	// Re-analyze the page to find replacement selectors.
	analysis, err := AnalyzeSite(context.Background(), browser, url)
	if err != nil {
		return nil, nil, fmt.Errorf("runbook: fix: re-analyze: %w", err)
	}

	// Build a lookup from field purpose to candidate selectors from analysis.
	fieldCandidates := make(map[string]FieldCandidate)

	for _, c := range analysis.Containers {
		for _, f := range c.Fields {
			fieldCandidates[f.Name] = f
		}
	}

	// Also collect container selectors from analysis.
	var containerCandidates []string
	for _, c := range analysis.Containers {
		containerCandidates = append(containerCandidates, c.Selector)
	}

	// Deep-copy the runbook so we don't mutate the original.
	fixed := copyRunbook(r)

	var changes []string

	for _, name := range broken {
		oldSel := selectors[name]

		switch {
		case name == "container":
			if len(containerCandidates) > 0 {
				newSel := containerCandidates[0]
				if newSel != oldSel {
					fixed.Items.Container = newSel
					if fixed.WaitFor == oldSel {
						fixed.WaitFor = newSel
					}

					changes = append(changes, fmt.Sprintf("container: changed selector from %q to %q", oldSel, newSel))
				}
			}

		case strings.HasPrefix(name, "field:"):
			fieldName := strings.TrimPrefix(name, "field:")

			purpose := guessPurpose(fieldName)
			if cand, ok := fieldCandidates[purpose]; ok {
				newSel := cand.Selector
				if cand.Attr != "" {
					newSel += "@" + cand.Attr
				}

				if newSel != oldSel {
					fixed.Items.Fields[fieldName] = newSel
					changes = append(changes, fmt.Sprintf("field %q: changed selector from %q to %q", fieldName, oldSel, newSel))
				}
			}

		case name == "wait_for":
			// Try to use the new container selector.
			if fixed.Items != nil && fixed.Items.Container != "" {
				newSel := fixed.Items.Container
				if newSel != oldSel {
					fixed.WaitFor = newSel
					changes = append(changes, fmt.Sprintf("wait_for: changed selector from %q to %q", oldSel, newSel))
				}
			}

		case name == "pagination:next":
			if analysis.Pagination != nil && analysis.Pagination.NextSelector != "" {
				newSel := analysis.Pagination.NextSelector
				if newSel != oldSel && fixed.Pagination != nil {
					fixed.Pagination.NextSelector = newSel
					changes = append(changes, fmt.Sprintf("pagination next: changed selector from %q to %q", oldSel, newSel))
				}
			}

		default:
			// Step selectors and others: try matching by action type and field candidates.
			// This is best-effort; steps are harder to auto-fix.
		}
	}

	return fixed, changes, nil
}

// guessPurpose maps a runbook field name to a FieldCandidate name from analysis.
// For example, "product_title" or "heading" maps to "title".
func guessPurpose(fieldName string) string {
	lower := strings.ToLower(fieldName)
	// Ordered by specificity — check longer/more-specific patterns first.
	checks := []struct {
		key, purpose string
	}{
		{"heading", "title"},
		{"title", "title"},
		{"image", "image"},
		{"photo", "image"},
		{"img", "image"},
		{"price", "price"},
		{"cost", "price"},
		{"date", "date"},
		{"time", "date"},
		{"link", "link"},
		{"href", "link"},
		{"url", "link"},
		{"name", "title"},
	}

	for _, c := range checks {
		if strings.Contains(lower, c.key) {
			return c.purpose
		}
	}

	return fieldName
}

// copyRunbook creates a shallow-ish copy of a runbook sufficient for fix mutations.
func copyRunbook(r *Runbook) *Runbook {
	cp := *r

	if r.Items != nil {
		items := *r.Items

		items.Fields = make(map[string]string, len(r.Items.Fields))
		maps.Copy(items.Fields, r.Items.Fields)

		cp.Items = &items
	}

	if r.Pagination != nil {
		pag := *r.Pagination
		cp.Pagination = &pag
	}

	if len(r.Steps) > 0 {
		cp.Steps = make([]Step, len(r.Steps))
		copy(cp.Steps, r.Steps)
	}

	if len(r.Selectors) > 0 {
		cp.Selectors = make(map[string]string, len(r.Selectors))
		maps.Copy(cp.Selectors, r.Selectors)
	}

	return &cp
}
