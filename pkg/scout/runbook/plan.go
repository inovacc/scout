package runbook

import (
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// PlanStatus describes the outcome of checking a single selector or step.
type PlanStatus string

const (
	PlanOK      PlanStatus = "ok"
	PlanMissing PlanStatus = "missing"
	PlanSkipped PlanStatus = "skipped"
)

// SelectorCheck holds the result of validating one named selector on a live page.
type SelectorCheck struct {
	Name     string        `json:"name"`
	Selector string        `json:"selector"`
	Status   PlanStatus    `json:"status"`
	Count    int           `json:"count"`
	Score    SelectorScore `json:"score"`
	Error    string        `json:"error,omitempty"`
}

// StepCheck holds the result of validating one automation step's selector.
type StepCheck struct {
	Index    int        `json:"index"`
	Action   string     `json:"action"`
	Selector string     `json:"selector,omitempty"`
	Status   PlanStatus `json:"status"`
	Error    string     `json:"error,omitempty"`
}

// ExecutionPlan describes what Apply would do, without executing it.
type ExecutionPlan struct {
	Runbook     string          `json:"runbook"`
	Type        string          `json:"type"`
	URL         string          `json:"url"`
	Valid       bool            `json:"valid"`
	Selectors   []SelectorCheck `json:"selectors,omitempty"`
	Steps       []StepCheck     `json:"steps,omitempty"`
	SampleCount int             `json:"sample_count"`
	Errors      []string        `json:"errors,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
}

// Plan navigates to the runbook URL and checks all selectors on the live page
// without extracting data or executing automation steps. It returns a structured
// ExecutionPlan showing what Apply would do.
func Plan(browser *scout.Browser, r *Runbook) (*ExecutionPlan, error) {
	if r == nil {
		return nil, fmt.Errorf("runbook: plan: nil runbook")
	}

	plan := &ExecutionPlan{
		Runbook: r.Name,
		Type:    r.Type,
		URL:     r.URL,
		Valid:   true,
	}

	switch r.Type {
	case "extract":
		return planExtract(browser, r, plan)
	case "automate":
		return planAutomate(browser, r, plan)
	default:
		return nil, fmt.Errorf("runbook: plan: unknown type %q", r.Type)
	}
}

// planExtract checks extract runbook selectors on the live page.
func planExtract(browser *scout.Browser, r *Runbook, plan *ExecutionPlan) (*ExecutionPlan, error) {
	if r.URL == "" {
		return nil, fmt.Errorf("runbook: plan: no URL to navigate to")
	}

	page, err := browser.NewPage(r.URL)
	if err != nil {
		return nil, fmt.Errorf("runbook: plan: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("runbook: plan: wait load: %w", err)
	}

	// Check wait_for selector.
	if r.WaitFor != "" {
		check := checkSelector(page, "wait_for", r.WaitFor)

		plan.Selectors = append(plan.Selectors, check)
		if check.Status == PlanMissing {
			plan.Valid = false
			plan.Errors = append(plan.Errors, fmt.Sprintf("wait_for selector %q not found", r.WaitFor))
		}
	}

	// Check container selector.
	if r.Items != nil && r.Items.Container != "" {
		check := checkSelector(page, "container", r.Items.Container)
		plan.Selectors = append(plan.Selectors, check)

		plan.SampleCount = check.Count
		if check.Status == PlanMissing {
			plan.Valid = false
			plan.Errors = append(plan.Errors, fmt.Sprintf("container selector %q not found", r.Items.Container))
		}

		// Check field selectors.
		for name, sel := range r.Items.Fields {
			check := checkSelector(page, "field:"+name, sel)

			plan.Selectors = append(plan.Selectors, check)
			if check.Status == PlanMissing {
				plan.Valid = false
				plan.Errors = append(plan.Errors, fmt.Sprintf("field %q selector %q not found", name, sel))
			}
		}
	}

	// Check pagination selector.
	if r.Pagination != nil && r.Pagination.NextSelector != "" {
		check := checkSelector(page, "pagination:next", r.Pagination.NextSelector)

		plan.Selectors = append(plan.Selectors, check)
		if check.Status == PlanMissing {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("pagination next selector %q not found (may appear on later pages)", r.Pagination.NextSelector))
		}
	}

	// Add score warnings for fragile selectors.
	for i := range plan.Selectors {
		if plan.Selectors[i].Score.Tier == "fragile" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("fragile selector for %s: %s (score: %.2f)",
				plan.Selectors[i].Name, plan.Selectors[i].Selector, plan.Selectors[i].Score.Score))
		}
	}

	return plan, nil
}

// planAutomate validates each step's selector on the initial page.
func planAutomate(browser *scout.Browser, r *Runbook, plan *ExecutionPlan) (*ExecutionPlan, error) {
	// Find URL from first navigate step.
	url := r.URL
	if url == "" {
		for _, s := range r.Steps {
			if s.Action == "navigate" && s.URL != "" {
				url = s.URL
				break
			}
		}
	}

	plan.URL = url

	if url == "" {
		return nil, fmt.Errorf("runbook: plan: no URL to navigate to")
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return nil, fmt.Errorf("runbook: plan: new page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("runbook: plan: wait load: %w", err)
	}

	for i, step := range r.Steps {
		sc := StepCheck{
			Index:    i,
			Action:   step.Action,
			Selector: step.Selector,
			Status:   PlanOK,
		}

		if step.Selector != "" {
			css := selectorToCSS(step.Selector)
			if css != "" {
				has, _ := page.Has(css)
				if !has {
					sc.Status = PlanMissing
					sc.Error = "selector not found on initial page"
					plan.Valid = false
					plan.Errors = append(plan.Errors, fmt.Sprintf("step %d (%s): selector %q not found", i, step.Action, step.Selector))
				}
			}

			// Score the selector.
			selCheck := checkSelector(page, fmt.Sprintf("step[%d]:%s", i, step.Action), step.Selector)
			plan.Selectors = append(plan.Selectors, selCheck)
		} else if step.Action == "navigate" {
			sc.Status = PlanOK // navigate steps don't need selectors
		} else if step.Action == "eval" || step.Action == "screenshot" || step.Action == "key" {
			sc.Status = PlanSkipped // these don't require selectors
		}

		plan.Steps = append(plan.Steps, sc)
	}

	return plan, nil
}

// checkSelector tests a single selector on the page and scores it.
func checkSelector(page *scout.Page, name, sel string) SelectorCheck {
	css := selectorToCSS(sel)
	check := SelectorCheck{
		Name:     name,
		Selector: sel,
		Score:    ScoreSelector(sel),
	}

	if css == "" {
		check.Status = PlanSkipped
		check.Error = "empty selector"

		return check
	}

	elems, err := page.Elements(css)
	if err != nil {
		check.Status = PlanMissing
		check.Count = 0
		check.Error = err.Error()

		return check
	}

	check.Count = len(elems)
	if check.Count == 0 {
		check.Status = PlanMissing
	} else {
		check.Status = PlanOK
	}

	return check
}

// String returns a terraform-style text representation of the execution plan.
func (p *ExecutionPlan) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Runbook: %s (%s)\n", p.Runbook, p.Type)
	fmt.Fprintf(&b, "URL: %s\n", p.URL)
	b.WriteString("\n")

	if len(p.Selectors) > 0 {
		b.WriteString("Selectors:\n")

		for _, s := range p.Selectors {
			icon := "+"

			switch s.Status {
			case PlanOK:
				icon = "+"
			case PlanMissing:
				icon = "-"
			case PlanSkipped:
				icon = "~"
			}

			fmt.Fprintf(&b, "  %s %-18s %-30s %3d found  (%s, %.2f)\n",
				icon, s.Name, fmt.Sprintf("%q", s.Selector), s.Count, s.Score.Tier, s.Score.Score)
		}

		b.WriteString("\n")
	}

	if len(p.Steps) > 0 {
		b.WriteString("Steps:\n")

		for _, s := range p.Steps {
			icon := "+"
			switch s.Status {
			case PlanMissing:
				icon = "-"
			case PlanSkipped:
				icon = "~"
			}

			detail := s.Selector
			if detail == "" {
				detail = s.Action
			}

			fmt.Fprintf(&b, "  %s [%d] %-12s %s\n", icon, s.Index, s.Action, detail)
		}

		b.WriteString("\n")
	}

	errCount := len(p.Errors)
	warnCount := len(p.Warnings)

	if p.Type == "extract" {
		fmt.Fprintf(&b, "Plan: %d items to extract", p.SampleCount)
	} else {
		fmt.Fprintf(&b, "Plan: %d steps to execute", len(p.Steps))
	}

	if errCount > 0 {
		fmt.Fprintf(&b, ", %d error(s)", errCount)
	}

	if warnCount > 0 {
		fmt.Fprintf(&b, ", %d warning(s)", warnCount)
	}

	b.WriteString(".\n")

	for _, e := range p.Errors {
		fmt.Fprintf(&b, "  Error: %s\n", e)
	}

	for _, w := range p.Warnings {
		fmt.Fprintf(&b, "  Warning: %s\n", w)
	}

	return b.String()
}
