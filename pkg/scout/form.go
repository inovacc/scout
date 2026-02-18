package scout

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/rod"
	"github.com/inovacc/scout/pkg/rod/lib/proto"
)

// FormField describes a single field within an HTML form.
type FormField struct {
	Name        string
	Type        string
	ID          string
	Value       string
	Placeholder string
	Required    bool
	Options     []string // populated for <select> elements
}

// Form represents a detected HTML form on the page.
type Form struct {
	Action string
	Method string
	Fields []FormField
	page   *rod.Page
	el     *rod.Element
}

// DetectForms finds all <form> elements on the page and returns parsed Form objects.
func (p *Page) DetectForms() ([]*Form, error) {
	els, err := p.page.Elements("form")
	if err != nil {
		return nil, fmt.Errorf("scout: detect forms: %w", err)
	}

	forms := make([]*Form, 0, len(els))
	for _, el := range els {
		f, err := parseForm(p.page, el)
		if err != nil {
			return nil, err
		}

		forms = append(forms, f)
	}

	return forms, nil
}

// DetectForm finds a specific form by CSS selector.
func (p *Page) DetectForm(selector string) (*Form, error) {
	el, err := p.page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: detect form %q: %w", selector, err)
	}

	return parseForm(p.page, el)
}

// Fill fills form fields using a map of field name (or ID) to value.
func (f *Form) Fill(data map[string]string) error {
	for key, value := range data {
		if err := f.fillField(key, value); err != nil {
			return fmt.Errorf("scout: fill field %q: %w", key, err)
		}
	}

	return nil
}

// FillStruct fills form fields using struct tags: `form:"field_name"`.
func (f *Form) FillStruct(data any) error {
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("scout: fill struct: data must be a struct or pointer to struct")
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		tag := field.Tag.Get("form")
		if tag == "" || tag == "-" {
			continue
		}

		val := fmt.Sprintf("%v", rv.Field(i).Interface())
		if err := f.fillField(tag, val); err != nil {
			return fmt.Errorf("scout: fill struct field %q: %w", field.Name, err)
		}
	}

	return nil
}

// Submit submits the form by clicking the submit button or triggering form submit via JS.
func (f *Form) Submit() error {
	// Try to find and click a submit button
	submitBtn, err := f.page.ElementByJS(rod.Eval(`() => {
		const btn = this.querySelector('button[type="submit"], input[type="submit"]');
		return btn;
	}`).This(f.el.Object))
	if err == nil && submitBtn != nil {
		if err := submitBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return fmt.Errorf("scout: submit click: %w", err)
		}

		return nil
	}

	// Fallback: submit via JS
	_, err = f.el.Eval(`() => this.submit()`)
	if err != nil {
		return fmt.Errorf("scout: submit: %w", err)
	}

	return nil
}

// CSRFToken attempts to find a CSRF token field in the form.
// It looks for hidden inputs with common CSRF field names.
func (f *Form) CSRFToken() (string, error) {
	csrfNames := []string{
		"_token", "csrf_token", "csrfmiddlewaretoken", "_csrf",
		"csrf", "authenticity_token", "XSRF-TOKEN", "__RequestVerificationToken",
	}

	for _, name := range csrfNames {
		for _, field := range f.Fields {
			if field.Type == "hidden" && (field.Name == name || field.ID == name) && field.Value != "" {
				return field.Value, nil
			}
		}
	}

	// Also check meta tag
	result, err := f.page.Eval(`() => {
		const meta = document.querySelector('meta[name="csrf-token"]') || document.querySelector('meta[name="_csrf"]');
		return meta ? meta.getAttribute("content") : "";
	}`)
	if err == nil && result.Value.Str() != "" {
		return result.Value.Str(), nil
	}

	return "", fmt.Errorf("scout: csrf token not found")
}

// WizardStep describes a step in a multi-step form wizard.
type WizardStep struct {
	FormSelector string
	Data         map[string]string
	NextSelector string // CSS selector for "next" button; empty on last step
	WaitFor      string // CSS selector to wait for before filling
}

// FormWizard manages multi-step form workflows.
type FormWizard struct {
	page  *Page
	steps []WizardStep
}

// NewFormWizard creates a wizard for multi-step form interaction.
func (p *Page) NewFormWizard(steps ...WizardStep) *FormWizard {
	return &FormWizard{page: p, steps: steps}
}

// Run executes all steps of the form wizard sequentially.
func (w *FormWizard) Run() error {
	for i, step := range w.steps {
		// Wait for step to be ready
		if step.WaitFor != "" {
			if _, err := w.page.page.Element(step.WaitFor); err != nil {
				return fmt.Errorf("scout: wizard step %d wait for %q: %w", i+1, step.WaitFor, err)
			}
		}

		// Detect and fill form
		form, err := w.page.DetectForm(step.FormSelector)
		if err != nil {
			return fmt.Errorf("scout: wizard step %d detect form: %w", i+1, err)
		}

		if err := form.Fill(step.Data); err != nil {
			return fmt.Errorf("scout: wizard step %d fill: %w", i+1, err)
		}

		// Click next or submit
		if step.NextSelector != "" {
			nextBtn, err := w.page.page.Element(step.NextSelector)
			if err != nil {
				return fmt.Errorf("scout: wizard step %d next button %q: %w", i+1, step.NextSelector, err)
			}

			if err := nextBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
				return fmt.Errorf("scout: wizard step %d click next: %w", i+1, err)
			}
			// Small delay for page transition
			time.Sleep(100 * time.Millisecond)
		} else {
			if err := form.Submit(); err != nil {
				return fmt.Errorf("scout: wizard step %d submit: %w", i+1, err)
			}
		}
	}

	return nil
}

// --- internal helpers ---

func parseForm(page *rod.Page, el *rod.Element) (*Form, error) {
	f := &Form{page: page, el: el}

	// Action
	action, err := el.Attribute("action")
	if err == nil && action != nil {
		f.Action = *action
	}

	// Method
	method, err := el.Attribute("method")
	if err == nil && method != nil {
		f.Method = strings.ToUpper(*method)
	} else {
		f.Method = "GET"
	}

	// Parse fields: input, select, textarea
	inputs, _ := page.ElementsByJS(rod.Eval(`() => this.querySelectorAll("input, select, textarea")`).This(el.Object))
	for _, input := range inputs {
		field, err := parseFormField(input)
		if err != nil {
			continue
		}

		f.Fields = append(f.Fields, field)
	}

	return f, nil
}

func parseFormField(el *rod.Element) (FormField, error) {
	ff := FormField{}

	tagName, err := el.Eval(`() => this.tagName.toLowerCase()`)
	if err != nil {
		return ff, err
	}

	tag := tagName.Value.Str()

	name, _ := el.Attribute("name")
	if name != nil {
		ff.Name = *name
	}

	id, _ := el.Attribute("id")
	if id != nil {
		ff.ID = *id
	}

	placeholder, _ := el.Attribute("placeholder")
	if placeholder != nil {
		ff.Placeholder = *placeholder
	}

	switch tag {
	case "input":
		typ, _ := el.Attribute("type")
		if typ != nil {
			ff.Type = *typ
		} else {
			ff.Type = "text"
		}

		val, _ := el.Attribute("value")
		if val != nil {
			ff.Value = *val
		}
	case "select":
		ff.Type = "select"
		val, _ := el.Property("value")
		ff.Value = val.Str()
		// Collect options
		opts, _ := el.Eval(`() => Array.from(this.options).map(o => o.text)`)
		if opts != nil && !opts.Value.Nil() {
			for _, o := range opts.Value.Arr() {
				ff.Options = append(ff.Options, o.Str())
			}
		}
	case "textarea":
		ff.Type = "textarea"
		val, _ := el.Text()
		ff.Value = val
	}

	reqAttr, _ := el.Attribute("required")
	ff.Required = reqAttr != nil

	return ff, nil
}

func (f *Form) fillField(key, value string) error {
	// Try to find by name, then by id
	el, err := f.page.ElementByJS(rod.Eval(`(key) => {
		return this.querySelector('[name="' + key + '"]') ||
			this.querySelector('#' + key) ||
			this.querySelector('[id="' + key + '"]');
	}`, key).This(f.el.Object))
	if err != nil {
		return fmt.Errorf("field %q not found", key)
	}

	tagResult, err := el.Eval(`() => this.tagName.toLowerCase()`)
	if err != nil {
		return err
	}

	tag := tagResult.Value.Str()

	if tag == "select" {
		// For select, try setting value via JS
		_, err := el.Eval(`(val) => {
			this.value = val;
			this.dispatchEvent(new Event('change', {bubbles: true}));
		}`, value)

		return err
	}

	// For input/textarea: clear and type
	if err := el.SelectAllText(); err != nil {
		// Field may be empty, ignore
		_ = err
	}

	if err := el.Input(value); err != nil {
		return err
	}

	return nil
}
