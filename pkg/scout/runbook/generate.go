package runbook

import (
	"fmt"
	"net/url"
	"strings"
)

// GenerateOption configures GenerateRunbook behavior.
type GenerateOption func(*generateOptions)

type generateOptions struct {
	forceType string
	fields    []string
	maxPages  int
}

// WithGenerateType forces the runbook type ("extract" or "automate").
func WithGenerateType(t string) GenerateOption {
	return func(o *generateOptions) { o.forceType = t }
}

// WithGenerateFields specifies which fields to include in an extract runbook.
func WithGenerateFields(fields ...string) GenerateOption {
	return func(o *generateOptions) { o.fields = fields }
}

// WithGenerateMaxPages sets the max pages for pagination.
func WithGenerateMaxPages(n int) GenerateOption {
	return func(o *generateOptions) { o.maxPages = n }
}

// GenerateRunbook creates a Runbook from a SiteAnalysis.
func GenerateRunbook(analysis *SiteAnalysis, opts ...GenerateOption) (*Runbook, error) {
	if analysis == nil {
		return nil, fmt.Errorf("runbook: generate: nil analysis")
	}

	o := &generateOptions{maxPages: 1}
	for _, fn := range opts {
		fn(o)
	}

	runbookType := detectRunbookType(analysis)
	if o.forceType != "" {
		runbookType = o.forceType
	}

	name := inferName(analysis)

	var r *Runbook
	var err error

	switch runbookType {
	case "extract":
		r, err = generateExtract(analysis, o, name)
	case "automate":
		r, err = generateAutomate(analysis, o, name)
	default:
		return nil, fmt.Errorf("runbook: generate: cannot determine runbook type from analysis")
	}
	if err != nil {
		return nil, err
	}

	// Score all selectors and warn about fragile ones.
	scores := ScoreRunbookSelectors(r)
	for name, s := range scores {
		if s.Tier == "fragile" {
			r.Warnings = append(r.Warnings, fmt.Sprintf(
				"fragile selector for %s: %s (score: %.2f, consider using data-* attributes)",
				name, s.Selector, s.Score,
			))
		}
	}

	return r, nil
}

func detectRunbookType(a *SiteAnalysis) string {
	if len(a.Containers) > 0 && a.Containers[0].Count >= 3 {
		return "extract"
	}
	if len(a.Forms) > 0 {
		return "automate"
	}
	return ""
}

func inferName(a *SiteAnalysis) string {
	if title, ok := a.Metadata["title"]; ok && title != "" {
		// Sanitize: lowercase, replace spaces with hyphens, truncate
		name := strings.ToLower(title)
		name = strings.ReplaceAll(name, " ", "-")
		if len(name) > 40 {
			name = name[:40]
		}
		return name
	}

	u, err := url.Parse(a.URL)
	if err == nil && u.Host != "" {
		return u.Host
	}

	return "generated-runbook"
}

func generateExtract(a *SiteAnalysis, o *generateOptions, name string) (*Runbook, error) {
	if len(a.Containers) == 0 {
		return nil, fmt.Errorf("runbook: generate: no containers found for extract runbook")
	}

	top := a.Containers[0]
	fields := make(map[string]string)

	if len(o.fields) > 0 {
		// Only include requested fields
		fieldMap := make(map[string]FieldCandidate)
		for _, f := range top.Fields {
			fieldMap[f.Name] = f
		}
		for _, name := range o.fields {
			if fc, ok := fieldMap[name]; ok {
				sel := fc.Selector
				if fc.Attr != "" {
					sel += "@" + fc.Attr
				}
				fields[name] = sel
			}
		}
	} else {
		// Include all discovered fields
		for _, f := range top.Fields {
			sel := f.Selector
			if f.Attr != "" {
				sel += "@" + f.Attr
			}
			fields[f.Name] = sel
		}
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("runbook: generate: no fields discovered in top container")
	}

	r := &Runbook{
		Version: "1",
		Name:    name,
		Type:    "extract",
		URL:     a.URL,
		WaitFor: top.Selector,
		Items: &ItemSpec{
			Container: top.Selector,
			Fields:    fields,
		},
		Output: Output{Format: "json"},
	}

	if a.Pagination != nil && a.Pagination.Strategy == "click" {
		maxPages := o.maxPages
		if maxPages < 2 {
			maxPages = 5
		}
		r.Pagination = &Pagination{
			Strategy:     "click",
			NextSelector: a.Pagination.NextSelector,
			MaxPages:     maxPages,
		}
	}

	return r, nil
}

func generateAutomate(a *SiteAnalysis, _ *generateOptions, name string) (*Runbook, error) {
	if len(a.Forms) == 0 {
		return nil, fmt.Errorf("runbook: generate: no forms found for automate runbook")
	}

	form := a.Forms[0]

	steps := []Step{
		{Action: "navigate", URL: a.URL},
	}

	for _, field := range form.Fields {
		if field.Type == "hidden" || field.Type == "submit" {
			continue
		}
		if field.Selector == "" {
			continue
		}
		steps = append(steps, Step{
			Action:   "type",
			Selector: field.Selector,
			Text:     fmt.Sprintf("{{%s}}", field.Name),
		})
	}

	// Add submit click
	steps = append(steps, Step{
		Action:   "click",
		Selector: form.Selector + " [type=\"submit\"], " + form.Selector + " button",
	})

	return &Runbook{
		Version: "1",
		Name:    name,
		Type:    "automate",
		Steps:   steps,
		Output:  Output{Format: "json"},
	}, nil
}
