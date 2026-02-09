package scout

import (
	"testing"
)

func TestElementClick(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	btn, err := page.Element("#btn")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}
	if err := btn.Click(); err != nil {
		t.Fatalf("Click() error: %v", err)
	}

	info, err := page.Element("#info")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}
	text, err := info.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}
	if text != "Clicked" {
		t.Errorf("Text() = %q, want %q", text, "Clicked")
	}
}

func TestElementInput(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	input, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := input.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}
	if err := input.Input("John Doe"); err != nil {
		t.Fatalf("Input() error: %v", err)
	}

	val, err := input.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}
	if val != "John Doe" {
		t.Errorf("Property('value') = %q, want %q", val, "John Doe")
	}
}

func TestElementVisible(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	visible, err := page.Element("#info")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}
	v, err := visible.Visible()
	if err != nil {
		t.Fatalf("Visible() error: %v", err)
	}
	if !v {
		t.Error("Visible() should be true for #info")
	}

	hidden, err := page.Element("#hidden")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}
	v, err = hidden.Visible()
	if err != nil {
		t.Fatalf("Visible() error: %v", err)
	}
	if v {
		t.Error("Visible() should be false for #hidden")
	}
}

func TestElementAttribute(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	input, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	val, exists, err := input.Attribute("type")
	if err != nil {
		t.Fatalf("Attribute() error: %v", err)
	}
	if !exists {
		t.Error("Attribute('type') should exist")
	}
	if val != "text" {
		t.Errorf("Attribute('type') = %q, want %q", val, "text")
	}

	_, exists, err = input.Attribute("nonexistent")
	if err != nil {
		t.Fatalf("Attribute() error: %v", err)
	}
	if exists {
		t.Error("Attribute('nonexistent') should not exist")
	}
}

func TestElementSelectOption(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	sel, err := page.Element("#sel")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := sel.SelectOption("Beta"); err != nil {
		t.Fatalf("SelectOption() error: %v", err)
	}

	val, err := sel.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}
	if val != "b" {
		t.Errorf("Property('value') = %q, want %q", val, "b")
	}
}

func TestElementChildElement(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	parent, err := page.Element("#parent")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	child, err := parent.Element("#child")
	if err != nil {
		t.Fatalf("child Element() error: %v", err)
	}

	text, err := child.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}
	if text != "Child Text" {
		t.Errorf("Text() = %q, want %q", text, "Child Text")
	}
}

func TestElementHTML(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	el, err := page.Element("#child")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	html, err := el.HTML()
	if err != nil {
		t.Fatalf("HTML() error: %v", err)
	}
	if html != `<span id="child">Child Text</span>` {
		t.Errorf("HTML() = %q", html)
	}
}

func TestElementScreenshot(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	data, err := el.Screenshot()
	if err != nil {
		t.Fatalf("Screenshot() error: %v", err)
	}
	if len(data) == 0 {
		t.Error("Screenshot() returned empty data")
	}
}

func TestElementMatches(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	el, err := page.Element("#info")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	matches, err := el.Matches("p")
	if err != nil {
		t.Fatalf("Matches() error: %v", err)
	}
	if !matches {
		t.Error("Matches('p') should be true for #info")
	}

	matches, err = el.Matches("div")
	if err != nil {
		t.Fatalf("Matches() error: %v", err)
	}
	if matches {
		t.Error("Matches('div') should be false for #info")
	}
}

func TestElementEval(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	el, err := page.Element("#info")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	result, err := el.Eval(`() => this.tagName.toLowerCase()`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}
	if result.String() != "p" {
		t.Errorf("Eval() = %q, want %q", result.String(), "p")
	}
}
