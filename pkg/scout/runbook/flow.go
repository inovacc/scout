package runbook

import (
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// FlowStep describes a single page in a multi-page flow.
type FlowStep struct {
	URL      string     `json:"url"`
	PageType string     `json:"page_type"` // "login", "search", "listing", "detail", "form", "unknown"
	Forms    []FormInfo `json:"forms,omitempty"`
	Links    []string   `json:"links,omitempty"`
	IsLogin  bool       `json:"is_login"`
	IsSearch bool       `json:"is_search"`
}

// FormInfo describes a detected form on a page.
type FormInfo struct {
	Selector    string   `json:"selector"`
	Action      string   `json:"action,omitempty"`
	Method      string   `json:"method,omitempty"`
	HasPassword bool     `json:"has_password"`
	HasSearch   bool     `json:"has_search"`
	Fields      []string `json:"fields,omitempty"`
}

// DetectFlow visits each URL in sequence and detects page types and transitions.
func DetectFlow(browser *scout.Browser, urls []string) ([]FlowStep, error) {
	if browser == nil {
		return nil, fmt.Errorf("runbook: flow: nil browser")
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("runbook: flow: no URLs provided")
	}

	steps := make([]FlowStep, 0, len(urls))

	for _, u := range urls {
		step, err := detectFlowStep(browser, u)
		if err != nil {
			return nil, fmt.Errorf("runbook: flow: %s: %w", u, err)
		}
		steps = append(steps, *step)
	}

	return steps, nil
}

func detectFlowStep(browser *scout.Browser, url string) (*FlowStep, error) {
	page, err := browser.NewPage(url)
	if err != nil {
		return nil, fmt.Errorf("new page: %w", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("wait load: %w", err)
	}

	step := &FlowStep{URL: url}

	// Detect forms via JS.
	formsResult, err := page.Eval(`() => {
		const forms = Array.from(document.querySelectorAll('form'));
		return forms.map(f => {
			const inputs = Array.from(f.querySelectorAll('input, select, textarea'));
			return {
				action: f.action || '',
				method: (f.method || 'get').toUpperCase(),
				hasPassword: inputs.some(i => i.type === 'password'),
				hasSearch: inputs.some(i => i.type === 'search' || i.name === 'q' || i.name === 'query' || i.name === 'search' || (i.placeholder && i.placeholder.toLowerCase().includes('search'))),
				fields: inputs.map(i => i.name || i.id || i.type).filter(Boolean),
				selector: f.id ? '#' + f.id : (f.className ? 'form.' + f.className.split(' ')[0] : 'form')
			};
		});
	}`)
	if err == nil {
		if arr, ok := formsResult.Value.([]any); ok {
			for _, item := range arr {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				fi := FormInfo{
					Selector:    stringVal(m, "selector"),
					Action:      stringVal(m, "action"),
					Method:      stringVal(m, "method"),
					HasPassword: boolVal(m, "hasPassword"),
					HasSearch:   boolVal(m, "hasSearch"),
				}
				if fields, ok := m["fields"].([]any); ok {
					for _, f := range fields {
						if s, ok := f.(string); ok {
							fi.Fields = append(fi.Fields, s)
						}
					}
				}
				step.Forms = append(step.Forms, fi)
				if fi.HasPassword {
					step.IsLogin = true
				}
				if fi.HasSearch {
					step.IsSearch = true
				}
			}
		}
	}

	// Detect links (limited to first 20).
	linksResult, err := page.Eval(`() => {
		const links = Array.from(document.querySelectorAll('a[href]'));
		return links.slice(0, 20).map(a => a.href).filter(h => h.startsWith('http'));
	}`)
	if err == nil {
		if arr, ok := linksResult.Value.([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					step.Links = append(step.Links, s)
				}
			}
		}
	}

	// Detect page type via repeating elements.
	countResult, err := page.Eval(`() => {
		const candidates = ['article', '.item', '.card', '.product', '.result', 'li', 'tr'];
		for (const sel of candidates) {
			const count = document.querySelectorAll(sel).length;
			if (count >= 3) return count;
		}
		return 0;
	}`)

	hasRepeating := false
	if err == nil {
		if n, ok := countResult.Value.(float64); ok && n >= 3 {
			hasRepeating = true
		}
	}

	// Classify page type.
	switch {
	case step.IsLogin:
		step.PageType = "login"
	case step.IsSearch:
		step.PageType = "search"
	case hasRepeating:
		step.PageType = "listing"
	case len(step.Forms) > 0:
		step.PageType = "form"
	default:
		step.PageType = "unknown"
	}

	return step, nil
}

func stringVal(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func boolVal(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GenerateFlowRunbook creates a multi-step automate runbook from detected flow steps.
func GenerateFlowRunbook(steps []FlowStep, name string) (*Runbook, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("runbook: flow: no steps provided")
	}

	if name == "" {
		name = "flow-runbook"
	}

	// Single listing page: generate extract runbook instead.
	if len(steps) == 1 && steps[0].PageType == "listing" {
		return &Runbook{
			Version: "1",
			Name:    name,
			Type:    "extract",
			URL:     steps[0].URL,
			Items: &ItemSpec{
				Container: "article, .item, .card, .product, .result",
				Fields:    map[string]string{"title": "h2, h3, .title"},
			},
			Output: Output{Format: "json"},
		}, nil
	}

	var runbookSteps []Step

	for _, step := range steps {
		runbookSteps = append(runbookSteps, Step{
			Action: "navigate",
			URL:    step.URL,
		})

		switch step.PageType {
		case "login":
			for _, form := range step.Forms {
				if !form.HasPassword {
					continue
				}
				for _, field := range form.Fields {
					lower := strings.ToLower(field)
					switch {
					case lower == "password" || lower == "passwd":
						runbookSteps = append(runbookSteps, Step{
							Action:   "type",
							Selector: fmt.Sprintf("input[name=%q], input[type=password]", field),
							Text:     "{{password}}",
						})
					case lower == "email" || lower == "username" || lower == "user" || lower == "login":
						runbookSteps = append(runbookSteps, Step{
							Action:   "type",
							Selector: fmt.Sprintf("input[name=%q]", field),
							Text:     "{{username}}",
						})
					}
				}
				runbookSteps = append(runbookSteps, Step{
					Action:   "click",
					Selector: form.Selector + " [type=submit], " + form.Selector + " button",
				})
				break
			}

		case "search":
			for _, form := range step.Forms {
				if !form.HasSearch {
					continue
				}
				runbookSteps = append(runbookSteps, Step{
					Action:   "type",
					Selector: "input[type=search], input[name=q], input[name=query], input[name=search]",
					Text:     "{{query}}",
				})
				runbookSteps = append(runbookSteps, Step{
					Action:   "click",
					Selector: form.Selector + " [type=submit], " + form.Selector + " button",
				})
				break
			}

		case "listing":
			runbookSteps = append(runbookSteps, Step{
				Action: "extract",
				As:     "items",
			})
		}
	}

	if len(runbookSteps) == 0 {
		return nil, fmt.Errorf("runbook: flow: could not generate steps from flow")
	}

	return &Runbook{
		Version: "1",
		Name:    name,
		Type:    "automate",
		Steps:   runbookSteps,
		Output:  Output{Format: "json"},
	}, nil
}
