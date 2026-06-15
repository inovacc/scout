package engine

import (
	"fmt"
	"strings"
	"time"
)

const defaultExpectTimeout = 5 * time.Second

func normSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// ExpectOption configures a web-first assertion.
type ExpectOption func(*expectCfg)

type expectCfg struct{ timeout time.Duration }

// WithExpectTimeout overrides the assertion polling timeout (default 5s).
func WithExpectTimeout(d time.Duration) ExpectOption { return func(c *expectCfg) { c.timeout = d } }

// LocatorAssertions are Playwright-style web-first assertions: each polls until
// the condition holds or the timeout elapses, returning a descriptive error.
type LocatorAssertions struct {
	loc     *Locator
	timeout time.Duration
	negate  bool
}

// Expect begins a retrying assertion chain on a locator.
func Expect(loc *Locator, opts ...ExpectOption) *LocatorAssertions {
	c := expectCfg{timeout: defaultExpectTimeout}
	for _, o := range opts {
		o(&c)
	}
	return &LocatorAssertions{loc: loc, timeout: c.timeout}
}

// Not inverts the next assertion.
func (a *LocatorAssertions) Not() *LocatorAssertions { c := *a; c.negate = !c.negate; return &c }

// poll retries check until it reports the expected outcome or the timeout hits.
func (a *LocatorAssertions) poll(what string, check func() (bool, string, error)) error {
	deadline := time.Now().Add(a.timeout)
	var detail string
	for {
		ok, d, err := check()
		if d != "" {
			detail = d
		}
		if err == nil && ok != a.negate {
			return nil
		}
		if time.Now().After(deadline) {
			neg := ""
			if a.negate {
				neg = "Not."
			}
			if err != nil {
				return fmt.Errorf("scout: expect %s: %sToBe%s: %w (after %s)", a.loc.describe(), neg, what, err, a.timeout)
			}
			return fmt.Errorf("scout: expect %s: %s%s failed after %s: %s", a.loc.describe(), neg, what, a.timeout, detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ToBeVisible asserts the element is visible.
func (a *LocatorAssertions) ToBeVisible() error {
	return a.poll("Visible", func() (bool, string, error) {
		v, err := a.loc.IsVisible()
		return v, fmt.Sprintf("visible=%v", v), err
	})
}

// ToBeHidden asserts the element is not visible (or absent).
func (a *LocatorAssertions) ToBeHidden() error {
	return a.poll("Hidden", func() (bool, string, error) {
		v, err := a.loc.IsVisible()
		return !v, fmt.Sprintf("visible=%v", v), err
	})
}

// ToBeEnabled asserts the element is enabled.
func (a *LocatorAssertions) ToBeEnabled() error {
	return a.poll("Enabled", func() (bool, string, error) {
		v, err := a.loc.IsEnabled()
		return v, fmt.Sprintf("enabled=%v", v), err
	})
}

// ToBeDisabled asserts the element is disabled.
func (a *LocatorAssertions) ToBeDisabled() error {
	return a.poll("Disabled", func() (bool, string, error) {
		v, err := a.loc.IsEnabled()
		return !v, fmt.Sprintf("enabled=%v", v), err
	})
}

// ToBeChecked asserts a checkbox/radio is checked.
func (a *LocatorAssertions) ToBeChecked() error {
	return a.poll("Checked", func() (bool, string, error) {
		v, err := a.loc.IsChecked()
		return v, fmt.Sprintf("checked=%v", v), err
	})
}

// ToHaveText asserts the element's text equals (whitespace-normalized) text.
func (a *LocatorAssertions) ToHaveText(text string) error {
	return a.poll("HaveText", func() (bool, string, error) {
		r, err := a.loc.jsValue("el.textContent == null ? '' : el.textContent")
		if err != nil {
			return false, "", err
		}
		got := r.String()
		return normSpace(got) == normSpace(text), fmt.Sprintf("text=%q want=%q", got, text), nil
	})
}

// ToContainText asserts the element's text contains (whitespace-normalized) text.
func (a *LocatorAssertions) ToContainText(text string) error {
	return a.poll("ContainText", func() (bool, string, error) {
		r, err := a.loc.jsValue("el.textContent == null ? '' : el.textContent")
		if err != nil {
			return false, "", err
		}
		got := r.String()
		return strings.Contains(normSpace(got), normSpace(text)), fmt.Sprintf("text=%q contains=%q", got, text), nil
	})
}

// ToHaveValue asserts the input's value equals value.
func (a *LocatorAssertions) ToHaveValue(value string) error {
	return a.poll("HaveValue", func() (bool, string, error) {
		got, err := a.loc.InputValue()
		return got == value, fmt.Sprintf("value=%q want=%q", got, value), err
	})
}

// ToHaveCount asserts the locator matches exactly n elements.
func (a *LocatorAssertions) ToHaveCount(n int) error {
	return a.poll("HaveCount", func() (bool, string, error) {
		got, err := a.loc.Count()
		return got == n, fmt.Sprintf("count=%d want=%d", got, n), err
	})
}

// ToHaveAttribute asserts the element's attribute equals value.
func (a *LocatorAssertions) ToHaveAttribute(name, value string) error {
	return a.poll("HaveAttribute", func() (bool, string, error) {
		got, err := a.loc.GetAttribute(name)
		return got == value, fmt.Sprintf("%s=%q want=%q", name, got, value), err
	})
}

// ---- page-level assertions ----

// PageAssertions are web-first assertions on a Page.
type PageAssertions struct {
	page    *Page
	timeout time.Duration
	negate  bool
}

// ExpectPage begins a retrying assertion chain on a page.
func ExpectPage(p *Page, opts ...ExpectOption) *PageAssertions {
	c := expectCfg{timeout: defaultExpectTimeout}
	for _, o := range opts {
		o(&c)
	}
	return &PageAssertions{page: p, timeout: c.timeout}
}

// Not inverts the next assertion.
func (a *PageAssertions) Not() *PageAssertions { c := *a; c.negate = !c.negate; return &c }

func (a *PageAssertions) poll(what string, check func() (bool, string, error)) error {
	deadline := time.Now().Add(a.timeout)
	var detail string
	for {
		ok, d, err := check()
		if d != "" {
			detail = d
		}
		if err == nil && ok != a.negate {
			return nil
		}
		if time.Now().After(deadline) {
			neg := ""
			if a.negate {
				neg = "Not."
			}
			return fmt.Errorf("scout: expect page: %s%s failed after %s: %s", neg, what, a.timeout, detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ToHaveTitle asserts the page title equals (normalized) title.
func (a *PageAssertions) ToHaveTitle(title string) error {
	return a.poll("HaveTitle", func() (bool, string, error) {
		got, err := a.page.Title()
		return normSpace(got) == normSpace(title), fmt.Sprintf("title=%q want=%q", got, title), err
	})
}

// ToHaveURL asserts the page URL contains url.
func (a *PageAssertions) ToHaveURL(url string) error {
	return a.poll("HaveURL", func() (bool, string, error) {
		got, err := a.page.URL()
		return strings.Contains(got, url), fmt.Sprintf("url=%q contains=%q", got, url), err
	})
}
