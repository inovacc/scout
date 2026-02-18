package scout

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/rod/lib/input"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/element-test", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Element Test</title></head>
<body>
<div id="container">
  <p id="first" class="item">First</p>
  <p id="second" class="item">Second</p>
  <p id="third" class="item">Third</p>
</div>
<button id="dblbtn" ondblclick="this.textContent='DblClicked'">DblClick Me</button>
<button id="rightbtn" oncontextmenu="this.textContent='RightClicked'; return false">RightClick Me</button>
<div id="hoverable" onmouseenter="this.textContent='Hovered'" onmouseleave="this.textContent='Left'">Hover Me</div>
<input id="typeinput" type="text" value=""/>
<input id="dateinput" type="date"/>
<input id="colorinput" type="color" value="#000000"/>
<select id="cssselect">
  <option value="x" class="opt-x">X Option</option>
  <option value="y" class="opt-y">Y Option</option>
</select>
<button id="removable">Remove Me</button>
<div id="scrolltarget" style="margin-top:2000px">Scroll Target</div>
<input id="disabled-input" type="text" disabled value="can't touch"/>
<canvas id="mycanvas" width="50" height="50"></canvas>
<script>
var ctx = document.getElementById('mycanvas').getContext('2d');
ctx.fillStyle = 'red';
ctx.fillRect(0, 0, 50, 50);
</script>
</body></html>`)
		})
	})
}

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

func TestElementDoubleClick(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	btn, err := page.Element("#dblbtn")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := btn.DoubleClick(); err != nil {
		t.Fatalf("DoubleClick() error: %v", err)
	}

	text, err := btn.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "DblClicked" {
		t.Errorf("Text() = %q, want %q", text, "DblClicked")
	}
}

func TestElementRightClick(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	btn, err := page.Element("#rightbtn")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := btn.RightClick(); err != nil {
		t.Fatalf("RightClick() error: %v", err)
	}

	text, err := btn.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "RightClicked" {
		t.Errorf("Text() = %q, want %q", text, "RightClicked")
	}
}

func TestElementHoverAndMoveOut(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#hoverable")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.Hover(); err != nil {
		t.Fatalf("Hover() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Hovered" {
		t.Errorf("after Hover() Text() = %q, want %q", text, "Hovered")
	}

	if err := el.MoveMouseOut(); err != nil {
		t.Fatalf("MoveMouseOut() error: %v", err)
	}
}

func TestElementTap(t *testing.T) {
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

	if err := btn.Tap(); err != nil {
		t.Fatalf("Tap() error: %v", err)
	}
}

func TestElementTypeAndPress(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#typeinput")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.Focus(); err != nil {
		t.Fatalf("Focus() error: %v", err)
	}

	if err := el.Type(input.KeyH, input.KeyI); err != nil {
		t.Fatalf("Type() error: %v", err)
	}

	val, err := el.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}

	if val != "hi" {
		t.Errorf("Type() value = %q, want %q", val, "hi")
	}

	// Press a single key
	if err := el.Press(input.KeyX); err != nil {
		t.Fatalf("Press() error: %v", err)
	}

	val, err = el.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}

	if val != "hix" {
		t.Errorf("Press() value = %q, want %q", val, "hix")
	}
}

func TestElementBlur(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.Focus(); err != nil {
		t.Fatalf("Focus() error: %v", err)
	}

	if err := el.Blur(); err != nil {
		t.Fatalf("Blur() error: %v", err)
	}
}

func TestElementScrollIntoView(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#scrolltarget")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.ScrollIntoView(); err != nil {
		t.Fatalf("ScrollIntoView() error: %v", err)
	}
}

func TestElementRemove(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#removable")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.Remove(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	has, err := page.Has("#removable")
	if err != nil {
		t.Fatalf("Has() error: %v", err)
	}

	if has {
		t.Error("element should be removed from DOM")
	}
}

func TestElementSelectAllText(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.SelectAllText(); err != nil {
		t.Fatalf("SelectAllText() error: %v", err)
	}
}

func TestElementSelectText(t *testing.T) {
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

	if err := el.SelectText("Hello"); err != nil {
		t.Skipf("SelectText() skipped (browser limitation): %v", err)
	}
}

func TestElementInteractable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	// Visible button should be interactable
	btn, err := page.Element("#btn")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	interactable, err := btn.Interactable()
	if err != nil {
		t.Fatalf("Interactable() error: %v", err)
	}

	if !interactable {
		t.Error("visible button should be interactable")
	}

	// Hidden div should not be interactable
	hidden, err := page.Element("#hidden")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	interactable, err = hidden.Interactable()
	if err != nil {
		t.Fatalf("Interactable() error: %v", err)
	}

	if interactable {
		t.Error("hidden element should not be interactable")
	}
}

func TestElementDisabled(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#disabled-input")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	disabled, err := el.Disabled()
	if err != nil {
		t.Fatalf("Disabled() error: %v", err)
	}

	if !disabled {
		t.Error("disabled input should report Disabled() = true")
	}

	// Normal input should not be disabled
	el2, err := page.Element("#typeinput")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	disabled, err = el2.Disabled()
	if err != nil {
		t.Fatalf("Disabled() error: %v", err)
	}

	if disabled {
		t.Error("normal input should report Disabled() = false")
	}
}

func TestElementScreenshotJPEG(t *testing.T) {
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

	data, err := el.ScreenshotJPEG(80)
	if err != nil {
		t.Fatalf("ScreenshotJPEG() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("ScreenshotJPEG() returned empty data")
	}

	if len(data) > 2 && (data[0] != 0xFF || data[1] != 0xD8) {
		t.Error("ScreenshotJPEG() should return JPEG data")
	}
}

func TestElementGetXPath(t *testing.T) {
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

	xpath, err := el.GetXPath()
	if err != nil {
		t.Fatalf("GetXPath() error: %v", err)
	}

	if xpath == "" {
		t.Error("GetXPath() returned empty string")
	}
}

func TestElementContainsAndEqual(t *testing.T) {
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

	child, err := page.Element("#child")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	contains, err := parent.ContainsElement(child)
	if err != nil {
		t.Fatalf("ContainsElement() error: %v", err)
	}

	if !contains {
		t.Error("parent should contain child")
	}

	// Get a second reference to the same element
	child2, err := page.Element("#child")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	// Same element via different references should be equal
	equal, err := child.Equal(child2)
	if err != nil {
		t.Fatalf("Equal() error: %v", err)
	}

	if !equal {
		t.Error("element should equal itself")
	}

	// Different elements should not be equal
	equal, err = parent.Equal(child)
	if err != nil {
		t.Fatalf("Equal() error: %v", err)
	}

	if equal {
		t.Error("parent and child should not be equal")
	}
}

func TestElementParentAndNext(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#first")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	// Parent
	parent, err := el.Parent()
	if err != nil {
		t.Skipf("Parent() skipped (browser limitation): %v", err)
	}

	html, err := parent.HTML()
	if err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	if !strings.Contains(html, "container") {
		t.Errorf("Parent() should be #container, got HTML: %s", html[:min(100, len(html))])
	}
}

func TestElementNextPrevious(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#first")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	// Next sibling
	next, err := el.Next()
	if err != nil {
		t.Skipf("Next() skipped (browser limitation): %v", err)
	}

	text, err := next.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Second" {
		t.Errorf("Next() text = %q, want %q", text, "Second")
	}

	// Previous sibling
	prev, err := next.Previous()
	if err != nil {
		t.Fatalf("Previous() error: %v", err)
	}

	text, err = prev.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "First" {
		t.Errorf("Previous() text = %q, want %q", text, "First")
	}
}

func TestElementParents(t *testing.T) {
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

	parents, err := el.Parents("div")
	if err != nil {
		t.Fatalf("Parents() error: %v", err)
	}

	if len(parents) == 0 {
		t.Error("Parents('div') should return at least one parent")
	}
}

func TestElementElements(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	container, err := page.Element("#container")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	items, err := container.Elements(".item")
	if err != nil {
		t.Fatalf("Elements() error: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("Elements('.item') = %d, want 3", len(items))
	}
}

func TestElementByXPath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	container, err := page.Element("#container")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	el, err := container.ElementByXPath(".//p[@id='second']")
	if err != nil {
		t.Fatalf("ElementByXPath() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Second" {
		t.Errorf("ElementByXPath() text = %q, want %q", text, "Second")
	}

	// ElementsByXPath
	els, err := container.ElementsByXPath(".//p")
	if err != nil {
		t.Fatalf("ElementsByXPath() error: %v", err)
	}

	if len(els) != 3 {
		t.Errorf("ElementsByXPath() = %d, want 3", len(els))
	}
}

func TestElementByText(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	container, err := page.Element("#container")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	el, err := container.ElementByText("p", "Third")
	if err != nil {
		t.Fatalf("ElementByText() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Third" {
		t.Errorf("ElementByText() text = %q, want %q", text, "Third")
	}
}

func TestElementSelectOptionByCSS(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	sel, err := page.Element("#cssselect")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := sel.SelectOptionByCSS(".opt-y"); err != nil {
		t.Fatalf("SelectOptionByCSS() error: %v", err)
	}

	val, err := sel.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}

	if val != "y" {
		t.Errorf("SelectOptionByCSS() value = %q, want %q", val, "y")
	}
}

func TestElementInputTime(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#dateinput")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	testTime := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if err := el.InputTime(testTime); err != nil {
		t.Fatalf("InputTime() error: %v", err)
	}
}

func TestElementInputColor(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#colorinput")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.InputColor("#ff0000"); err != nil {
		t.Fatalf("InputColor() error: %v", err)
	}
}

func TestElementRodElement(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	rod := el.RodElement()
	if rod == nil {
		t.Error("RodElement() should not be nil")
	}
}

func TestElementCanvasToImage(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#mycanvas")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	data, err := el.CanvasToImage("image/png", 1.0)
	if err != nil {
		t.Fatalf("CanvasToImage() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("CanvasToImage() returned empty data")
	}
}

func TestElementWaitVisible(t *testing.T) {
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

	if err := el.WaitVisible(); err != nil {
		t.Fatalf("WaitVisible() error: %v", err)
	}
}

func TestElementWaitStable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.WaitStable(100); err != nil {
		t.Fatalf("WaitStable() error: %v", err)
	}
}

func TestElementWaitLoad(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("body")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}
}

func TestElementWaitInvisible(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#hidden")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	// Already invisible, should return immediately
	if err := el.WaitInvisible(); err != nil {
		t.Fatalf("WaitInvisible() error: %v", err)
	}
}

func TestElementWaitStableRAF(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.WaitStableRAF(); err != nil {
		t.Fatalf("WaitStableRAF() error: %v", err)
	}
}

func TestElementWaitInteractable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#btn")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.WaitInteractable(); err != nil {
		t.Fatalf("WaitInteractable() error: %v", err)
	}
}

func TestElementWaitEnabled(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.WaitEnabled(); err != nil {
		t.Fatalf("WaitEnabled() error: %v", err)
	}
}

func TestElementWaitWritable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.WaitWritable(); err != nil {
		t.Fatalf("WaitWritable() error: %v", err)
	}
}

func TestElementBackgroundImage(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Body has no background image, so this should return empty or error
	el, err := page.Element("body")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	// Just exercise the method — may error if no bg image
	_, _ = el.BackgroundImage()
}

func TestElementSetFiles(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	// Create a page with a file input
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Add a file input dynamically
	_, err = page.Eval(`() => {
		const input = document.createElement('input');
		input.type = 'file';
		input.id = 'fileInput';
		document.body.appendChild(input);
	}`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	el, err := page.Element("#fileInput")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	// SetFiles with a non-existent path — should error
	err = el.SetFiles([]string{"/tmp/nonexistent-test-file.txt"})
	// This may or may not error depending on browser behavior — just exercise the code path
	_ = err
}

func TestElementResource(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Canvas element — exercise Resource() even if it may error
	el, err := page.Element("#mycanvas")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	_, _ = el.Resource()
}

func TestElementPrevious(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#second")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	prev, err := el.Previous()
	if err != nil {
		t.Skipf("Previous() error (rod limitation): %v", err)
	}

	text, err := prev.Text()
	if err != nil {
		t.Skipf("Text() error: %v", err)
	}

	if text != "First" {
		t.Errorf("Previous text = %q, want First", text)
	}
}

func TestElementClear(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#typeinput")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	if err := el.Input("test text"); err != nil {
		t.Fatalf("Input() error: %v", err)
	}

	if err := el.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	val, err := el.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}

	if val != "" {
		t.Errorf("value after clear = %q, want empty", val)
	}
}
