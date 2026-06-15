package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Locator is a lazy, Playwright-style handle to an element (or a set of
// elements) on a page. It does not resolve until an action or query runs, so it
// auto-waits for the element to appear and become actionable. Build one with
// Page.GetByRole/GetByText/GetByLabel/GetByPlaceholder/GetByTestID/GetByAltText/
// GetByTitle/Locate, then narrow with First/Nth/Last.
type Locator struct {
	page *Page
	spec locSpec
	nth  int // 0-based; negative counts from the end (-1 = last)
}

type locSpec struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Name       string `json:"name,omitempty"`
	Exact      bool   `json:"exact"`
	TestIDAttr string `json:"testIdAttr,omitempty"`
}

// LocatorOption tunes how a locator matches.
type LocatorOption func(*locSpec)

// WithExact requires an exact (whitespace-normalized, case-sensitive) match
// rather than the default case-insensitive substring match.
func WithExact(exact bool) LocatorOption { return func(s *locSpec) { s.Exact = exact } }

// WithName filters GetByRole matches by accessible name.
func WithName(name string) LocatorOption { return func(s *locSpec) { s.Name = name } }

func (p *Page) newLocator(kind, value string, opts ...LocatorOption) *Locator {
	s := locSpec{Kind: kind, Value: value}
	for _, o := range opts {
		o(&s)
	}
	return &Locator{page: p, spec: s, nth: 0}
}

// GetByRole locates elements by ARIA role (button, link, textbox, checkbox,
// heading, …), optionally filtered by accessible name via WithName.
func (p *Page) GetByRole(role string, opts ...LocatorOption) *Locator {
	return p.newLocator("role", role, opts...)
}

// GetByText locates elements by their text content.
func (p *Page) GetByText(text string, opts ...LocatorOption) *Locator {
	return p.newLocator("text", text, opts...)
}

// GetByLabel locates a form control by its associated <label> text.
func (p *Page) GetByLabel(text string, opts ...LocatorOption) *Locator {
	return p.newLocator("label", text, opts...)
}

// GetByPlaceholder locates an input by its placeholder text.
func (p *Page) GetByPlaceholder(text string, opts ...LocatorOption) *Locator {
	return p.newLocator("placeholder", text, opts...)
}

// GetByTestID locates an element by its data-testid attribute.
func (p *Page) GetByTestID(id string) *Locator {
	l := p.newLocator("testid", id)
	l.spec.TestIDAttr = "data-testid"
	return l
}

// GetByAltText locates an element (typically an image) by its alt text.
func (p *Page) GetByAltText(text string, opts ...LocatorOption) *Locator {
	return p.newLocator("alttext", text, opts...)
}

// GetByTitle locates an element by its title attribute.
func (p *Page) GetByTitle(text string, opts ...LocatorOption) *Locator {
	return p.newLocator("title", text, opts...)
}

// Locate builds a Locator from a CSS selector.
func (p *Page) Locate(cssSelector string) *Locator {
	return p.newLocator("css", cssSelector)
}

// First narrows the locator to its first match.
func (l *Locator) First() *Locator { c := *l; c.nth = 0; return &c }

// Last narrows the locator to its last match.
func (l *Locator) Last() *Locator { c := *l; c.nth = -1; return &c }

// Nth narrows the locator to the n-th match (0-based; negative counts from end).
func (l *Locator) Nth(n int) *Locator { c := *l; c.nth = n; return &c }

// describe returns a human-readable form for error messages.
func (l *Locator) describe() string {
	d := l.spec.Kind + "=" + strconv.Quote(l.spec.Value)
	if l.spec.Name != "" {
		d += " name=" + strconv.Quote(l.spec.Name)
	}
	return d
}

func (l *Locator) actionTimeout() time.Duration {
	if l.page != nil && l.page.browser != nil && l.page.browser.opts != nil && l.page.browser.opts.timeout > 0 {
		return l.page.browser.opts.timeout
	}
	return 30 * time.Second
}

// queryPrefix returns the JS that (idempotently) installs the resolver and
// computes the array of matches + the selected index.
func (l *Locator) queryPrefix() string {
	spec, _ := json.Marshal(l.spec)
	return "() => { if (!window.__scoutLocate) {" + locateScript + "} " +
		"var __m = window.__scoutLocate(" + string(spec) + "); " +
		"var __i = " + strconv.Itoa(l.nth) + "; if (__i < 0) __i = __m.length + __i; "
}

// resolve makes a single (no-wait) attempt to resolve the selected element.
func (l *Locator) resolve() (*Element, error) {
	js := l.queryPrefix() + "var el = __m[__i]; return el || null; }"
	return l.page.ElementByJS(js)
}

// jsValue evaluates a JS expression with `el` bound to the resolved element
// (null-safe; returns null if no match).
func (l *Locator) jsValue(expr string) (*EvalResult, error) {
	js := l.queryPrefix() + "var el = __m[__i]; if (!el) return null; return (" + expr + "); }"
	return l.page.Eval(js)
}

// waitFor resolves the selected element, retrying until it exists and is visible
// (or the action timeout elapses).
func (l *Locator) waitFor() (*Element, error) {
	deadline := time.Now().Add(l.actionTimeout())
	var lastErr error
	for {
		el, err := l.resolve()
		if err == nil && el != nil {
			if vis, verr := el.Visible(); verr == nil && vis {
				return el, nil
			} else if verr != nil {
				lastErr = verr
			}
		} else if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("not found or not visible")
			}
			return nil, fmt.Errorf("scout: locator %s: %w", l.describe(), lastErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ---- actions (each auto-waits for actionability) ----

// Click waits for the element to be actionable, then clicks it.
func (l *Locator) Click() error {
	el, err := l.waitFor()
	if err != nil {
		return err
	}
	return el.Click()
}

// Fill focuses the element and replaces its value with text.
func (l *Locator) Fill(text string) error {
	el, err := l.waitFor()
	if err != nil {
		return err
	}
	if err := el.Focus(); err != nil {
		return err
	}
	return el.Input(text)
}

// Hover moves the mouse over the element.
func (l *Locator) Hover() error {
	el, err := l.waitFor()
	if err != nil {
		return err
	}
	return el.Hover()
}

// Check ensures a checkbox/radio is checked (clicks only if needed).
func (l *Locator) Check() error { return l.setChecked(true) }

// Uncheck ensures a checkbox is unchecked (clicks only if needed).
func (l *Locator) Uncheck() error { return l.setChecked(false) }

func (l *Locator) setChecked(want bool) error {
	cur, err := l.IsChecked()
	if err != nil {
		return err
	}
	if cur == want {
		return nil
	}
	return l.Click()
}

// ---- queries ----

// TextContent returns the element's text content.
func (l *Locator) TextContent() (string, error) {
	el, err := l.waitFor()
	if err != nil {
		return "", err
	}
	return el.Text()
}

// InputValue returns the value of an input/textarea/select.
func (l *Locator) InputValue() (string, error) {
	r, err := l.jsValue("el.value == null ? '' : String(el.value)")
	if err != nil {
		return "", fmt.Errorf("scout: locator %s: input value: %w", l.describe(), err)
	}
	return r.String(), nil
}

// GetAttribute returns an attribute value (empty if absent).
func (l *Locator) GetAttribute(name string) (string, error) {
	r, err := l.jsValue("el.getAttribute(" + strconv.Quote(name) + ") || ''")
	if err != nil {
		return "", fmt.Errorf("scout: locator %s: attribute: %w", l.describe(), err)
	}
	return r.String(), nil
}

// IsVisible reports whether the (first) match is currently visible. It does not
// wait — it returns false if there is no match.
func (l *Locator) IsVisible() (bool, error) {
	r, err := l.jsValue("(function(){var s=el.ownerDocument.defaultView.getComputedStyle(el);return s.display!=='none'&&s.visibility!=='hidden'&&el.getClientRects().length>0;})()")
	if err != nil {
		return false, fmt.Errorf("scout: locator %s: is-visible: %w", l.describe(), err)
	}
	return r.Bool(), nil
}

// IsEnabled reports whether the match is enabled (not disabled).
func (l *Locator) IsEnabled() (bool, error) {
	r, err := l.jsValue("!el.disabled")
	if err != nil {
		return false, fmt.Errorf("scout: locator %s: is-enabled: %w", l.describe(), err)
	}
	return r.Bool(), nil
}

// IsChecked reports whether a checkbox/radio is checked.
func (l *Locator) IsChecked() (bool, error) {
	r, err := l.jsValue("!!el.checked")
	if err != nil {
		return false, fmt.Errorf("scout: locator %s: is-checked: %w", l.describe(), err)
	}
	return r.Bool(), nil
}

// Count returns how many elements the locator currently matches.
func (l *Locator) Count() (int, error) {
	spec, _ := json.Marshal(l.spec)
	js := "() => { if (!window.__scoutLocate) {" + locateScript + "} return window.__scoutLocate(" + string(spec) + ").length; }"
	r, err := l.page.Eval(js)
	if err != nil {
		return 0, fmt.Errorf("scout: locator %s: count: %w", l.describe(), err)
	}
	return r.Int(), nil
}

// Element resolves the selected element now (auto-waiting until actionable).
func (l *Locator) Element() (*Element, error) { return l.waitFor() }
