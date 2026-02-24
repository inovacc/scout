package recipe

import (
	"context"
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// SampleExtract runs the recipe on the first page only and returns sample items.
// It navigates to the recipe URL, waits for the page to load, then uses the
// recipe's container and field selectors to extract items without pagination.
func SampleExtract(browser *scout.Browser, r *Recipe) ([]map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("recipe: sample: nil recipe")
	}

	if r.URL == "" {
		return nil, fmt.Errorf("recipe: sample: no URL in recipe")
	}

	if r.Items == nil {
		return nil, fmt.Errorf("recipe: sample: no items spec in recipe")
	}

	page, err := browser.NewPage(r.URL)
	if err != nil {
		return nil, fmt.Errorf("recipe: sample: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("recipe: sample: wait load: %w", err)
	}

	if r.WaitFor != "" {
		if _, err := page.Element(r.WaitFor); err != nil {
			return nil, fmt.Errorf("recipe: sample: wait for %q: %w", r.WaitFor, err)
		}
	}

	// Extract from the first page only (no pagination).
	items, err := extractPage(page, r.Items)
	if err != nil {
		return nil, fmt.Errorf("recipe: sample: extract: %w", err)
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

// FixRecipe re-analyzes the page and updates broken selectors in the recipe.
// It returns the fixed recipe and a list of human-readable changes made.
func FixRecipe(browser *scout.Browser, r *Recipe) (*Recipe, []string, error) {
	if r == nil {
		return nil, nil, fmt.Errorf("recipe: fix: nil recipe")
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
		return nil, nil, fmt.Errorf("recipe: fix: no URL to navigate to")
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return nil, nil, fmt.Errorf("recipe: fix: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, nil, fmt.Errorf("recipe: fix: wait load: %w", err)
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
		// All selectors are healthy; return the recipe unchanged.
		return r, nil, nil
	}

	// Re-analyze the page to find replacement selectors.
	analysis, err := AnalyzeSite(context.Background(), browser, url)
	if err != nil {
		return nil, nil, fmt.Errorf("recipe: fix: re-analyze: %w", err)
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

	// Deep-copy the recipe so we don't mutate the original.
	fixed := copyRecipe(r)
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

		case strings.HasPrefix(name, "step["):
			// Step selectors: try matching by action type and field candidates.
			// This is best-effort; steps are harder to auto-fix.
		}
	}

	return fixed, changes, nil
}

// guessPurpose maps a recipe field name to a FieldCandidate name from analysis.
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

// copyRecipe creates a shallow-ish copy of a recipe sufficient for fix mutations.
func copyRecipe(r *Recipe) *Recipe {
	cp := *r

	if r.Items != nil {
		items := *r.Items
		items.Fields = make(map[string]string, len(r.Items.Fields))
		for k, v := range r.Items.Fields {
			items.Fields[k] = v
		}
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
		for k, v := range r.Selectors {
			cp.Selectors[k] = v
		}
	}

	return &cp
}
