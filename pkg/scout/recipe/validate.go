package recipe

import (
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// ValidationResult holds the outcome of a recipe dry-run validation.
type ValidationResult struct {
	Valid       bool              `json:"valid"`
	URL         string            `json:"url"`
	Errors      []ValidationError `json:"errors,omitempty"`
	SampleItems int               `json:"sample_items"`
}

// ValidationError describes a selector that failed to match any elements.
type ValidationError struct {
	Field    string `json:"field"`
	Selector string `json:"selector"`
	Error    string `json:"error"`
}

// ValidateRecipe navigates to the recipe URL and checks that all selectors
// match at least one element on the page. It returns a ValidationResult
// summarising the health of each selector without extracting data.
func ValidateRecipe(browser *scout.Browser, r *Recipe) (*ValidationResult, error) {
	if r == nil {
		return nil, fmt.Errorf("recipe: validate: nil recipe")
	}

	url := r.URL
	// For automate recipes, find the first navigate step URL.
	if url == "" && r.Type == "automate" {
		for _, s := range r.Steps {
			if s.Action == "navigate" && s.URL != "" {
				url = s.URL
				break
			}
		}
	}

	if url == "" {
		return nil, fmt.Errorf("recipe: validate: no URL to navigate to")
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return nil, fmt.Errorf("recipe: validate: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("recipe: validate: wait load: %w", err)
	}

	result := &ValidationResult{
		Valid: true,
		URL:   url,
	}

	// Collect all selectors to check.
	selectors := collectSelectors(r)
	counts := SelectorHealthCheck(page, selectors)

	for name, sel := range selectors {
		count := counts[name]
		if count == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:    name,
				Selector: sel,
				Error:    "no matching elements found",
			})
		}
	}

	// Count container matches as sample_items.
	if r.Items != nil && r.Items.Container != "" {
		result.SampleItems = counts["container"]
	}

	return result, nil
}

// SelectorHealthCheck takes a map of name->selector and returns name->matchCount.
func SelectorHealthCheck(page *scout.Page, selectors map[string]string) map[string]int {
	counts := make(map[string]int, len(selectors))
	for name, sel := range selectors {
		// Strip attribute suffix (e.g. "a@href" -> "a") and sibling prefix.
		css := selectorToCSS(sel)
		if css == "" {
			counts[name] = 0
			continue
		}
		elems, err := page.Elements(css)
		if err != nil {
			counts[name] = 0
			continue
		}
		counts[name] = len(elems)
	}
	return counts
}

// collectSelectors gathers all CSS selectors referenced by a recipe into a
// name->selector map suitable for SelectorHealthCheck.
func collectSelectors(r *Recipe) map[string]string {
	sels := make(map[string]string)

	if r.Items != nil {
		if r.Items.Container != "" {
			sels["container"] = r.Items.Container
		}
		for name, sel := range r.Items.Fields {
			sels["field:"+name] = sel
		}
	}

	if r.WaitFor != "" {
		sels["wait_for"] = r.WaitFor
	}

	if r.Pagination != nil && r.Pagination.NextSelector != "" {
		sels["pagination:next"] = r.Pagination.NextSelector
	}

	for i, step := range r.Steps {
		if step.Selector != "" {
			sels[fmt.Sprintf("step[%d]:%s", i, step.Action)] = step.Selector
		}
	}

	return sels
}

// selectorToCSS strips the sibling prefix (+) and attribute suffix (@attr)
// from a recipe selector string, returning a pure CSS selector.
func selectorToCSS(sel string) string {
	s := sel
	// Strip sibling prefix.
	s = strings.TrimPrefix(s, "+")
	// Strip attribute suffix.
	if idx := strings.Index(s, "@"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}
