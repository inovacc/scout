package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// FormDetectInput finds forms on a page. If Selector is empty, returns
// every form on the page; otherwise narrows to the matching form.
type FormDetectInput struct {
	URL      string `json:"url"                jsonschema:"URL of the page to inspect"`
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector to narrow to a specific form (default: all forms)"`
}

// FormDetectOutput holds detected form(s). At most one of Form/Forms is set.
type FormDetectOutput struct {
	Form  *scout.Form  `json:"form,omitempty"`
	Forms []scout.Form `json:"forms,omitempty"`
	Total int          `json:"total"`
}

// FormDetect navigates to the URL and returns detected forms.
func FormDetect(_ context.Context, b *scout.Browser, in FormDetectInput) (*FormDetectOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: form-detect: nil browser")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: form-detect: url required")
	}
	page, err := b.NewPage(in.URL)
	if err != nil {
		return nil, fmt.Errorf("tools: form-detect: navigate: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("tools: form-detect: wait load: %w", err)
	}
	if in.Selector != "" {
		f, err := page.DetectForm(in.Selector)
		if err != nil {
			return nil, fmt.Errorf("tools: form-detect: %w", err)
		}
		return &FormDetectOutput{Form: f, Total: 1}, nil
	}
	forms, err := page.DetectForms()
	if err != nil {
		return nil, fmt.Errorf("tools: form-detect: %w", err)
	}
	out := make([]scout.Form, len(forms))
	for i, f := range forms {
		out[i] = *f
	}
	return &FormDetectOutput{Forms: out, Total: len(out)}, nil
}
