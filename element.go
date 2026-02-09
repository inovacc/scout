package scout

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Element wraps a rod element with a simplified API.
type Element struct {
	element *rod.Element
}

// Click performs a left mouse click on the element.
func (e *Element) Click() error {
	if err := e.element.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("scout: click: %w", err)
	}

	return nil
}

// DoubleClick performs a double-click on the element.
func (e *Element) DoubleClick() error {
	if err := e.element.Click(proto.InputMouseButtonLeft, 2); err != nil {
		return fmt.Errorf("scout: double click: %w", err)
	}

	return nil
}

// RightClick performs a right mouse click on the element.
func (e *Element) RightClick() error {
	if err := e.element.Click(proto.InputMouseButtonRight, 1); err != nil {
		return fmt.Errorf("scout: right click: %w", err)
	}

	return nil
}

// Hover moves the mouse over the element.
func (e *Element) Hover() error {
	if err := e.element.Hover(); err != nil {
		return fmt.Errorf("scout: hover: %w", err)
	}

	return nil
}

// MoveMouseOut moves the mouse out of the element.
func (e *Element) MoveMouseOut() error {
	if err := e.element.MoveMouseOut(); err != nil {
		return fmt.Errorf("scout: move mouse out: %w", err)
	}

	return nil
}

// Tap simulates a touch tap on the element.
func (e *Element) Tap() error {
	if err := e.element.Tap(); err != nil {
		return fmt.Errorf("scout: tap: %w", err)
	}

	return nil
}

// Input focuses the element and inputs the given text.
func (e *Element) Input(text string) error {
	if err := e.element.Input(text); err != nil {
		return fmt.Errorf("scout: input: %w", err)
	}

	return nil
}

// InputTime inputs a time value into a date/time input element.
func (e *Element) InputTime(t time.Time) error {
	if err := e.element.InputTime(t); err != nil {
		return fmt.Errorf("scout: input time: %w", err)
	}

	return nil
}

// InputColor inputs a color value (e.g. "#ff0000") into a color input element.
func (e *Element) InputColor(color string) error {
	if err := e.element.InputColor(color); err != nil {
		return fmt.Errorf("scout: input color: %w", err)
	}

	return nil
}

// Clear removes all text from the element by selecting all and replacing with empty string.
func (e *Element) Clear() error {
	if err := e.element.SelectAllText(); err != nil {
		return fmt.Errorf("scout: clear (select all): %w", err)
	}

	if err := e.element.Input(""); err != nil {
		return fmt.Errorf("scout: clear (input empty): %w", err)
	}

	return nil
}

// Type simulates keyboard key presses on the element.
func (e *Element) Type(keys ...input.Key) error {
	if err := e.element.Type(keys...); err != nil {
		return fmt.Errorf("scout: type: %w", err)
	}

	return nil
}

// Press simulates pressing a single key on the element.
func (e *Element) Press(key input.Key) error {
	if err := e.element.Type(key); err != nil {
		return fmt.Errorf("scout: press: %w", err)
	}

	return nil
}

// SelectOption selects option elements in a <select> element by their text.
func (e *Element) SelectOption(selectors ...string) error {
	if err := e.element.Select(selectors, true, SelectorText); err != nil {
		return fmt.Errorf("scout: select option: %w", err)
	}

	return nil
}

// SelectOptionByCSS selects option elements in a <select> element by CSS selector.
func (e *Element) SelectOptionByCSS(selectors ...string) error {
	if err := e.element.Select(selectors, true, SelectorCSS); err != nil {
		return fmt.Errorf("scout: select option by css: %w", err)
	}

	return nil
}

// SetFiles sets the file paths for a file input element.
func (e *Element) SetFiles(paths []string) error {
	if err := e.element.SetFiles(paths); err != nil {
		return fmt.Errorf("scout: set files: %w", err)
	}

	return nil
}

// Focus sets focus on the element.
func (e *Element) Focus() error {
	if err := e.element.Focus(); err != nil {
		return fmt.Errorf("scout: focus: %w", err)
	}

	return nil
}

// Blur removes focus from the element.
func (e *Element) Blur() error {
	if err := e.element.Blur(); err != nil {
		return fmt.Errorf("scout: blur: %w", err)
	}

	return nil
}

// ScrollIntoView scrolls the element into the visible area.
func (e *Element) ScrollIntoView() error {
	if err := e.element.ScrollIntoView(); err != nil {
		return fmt.Errorf("scout: scroll into view: %w", err)
	}

	return nil
}

// Remove removes the element from the DOM.
func (e *Element) Remove() error {
	if err := e.element.Remove(); err != nil {
		return fmt.Errorf("scout: remove: %w", err)
	}

	return nil
}

// SelectAllText selects all text in the element.
func (e *Element) SelectAllText() error {
	if err := e.element.SelectAllText(); err != nil {
		return fmt.Errorf("scout: select all text: %w", err)
	}

	return nil
}

// SelectText selects text matching the regex in the element.
func (e *Element) SelectText(regex string) error {
	if err := e.element.SelectText(regex); err != nil {
		return fmt.Errorf("scout: select text: %w", err)
	}

	return nil
}

// Text returns the visible text content of the element.
func (e *Element) Text() (string, error) {
	text, err := e.element.Text()
	if err != nil {
		return "", fmt.Errorf("scout: text: %w", err)
	}

	return text, nil
}

// HTML returns the outer HTML of the element.
func (e *Element) HTML() (string, error) {
	html, err := e.element.HTML()
	if err != nil {
		return "", fmt.Errorf("scout: html: %w", err)
	}

	return html, nil
}

// Attribute returns the value of the named attribute.
// The bool return indicates whether the attribute exists.
func (e *Element) Attribute(name string) (string, bool, error) {
	val, err := e.element.Attribute(name)
	if err != nil {
		return "", false, fmt.Errorf("scout: attribute %q: %w", name, err)
	}

	if val == nil {
		return "", false, nil
	}

	return *val, true, nil
}

// Property returns the value of a DOM property as a string.
func (e *Element) Property(name string) (string, error) {
	val, err := e.element.Property(name)
	if err != nil {
		return "", fmt.Errorf("scout: property %q: %w", name, err)
	}

	return val.String(), nil
}

// Visible returns true if the element is visible on the page.
func (e *Element) Visible() (bool, error) {
	visible, err := e.element.Visible()
	if err != nil {
		return false, fmt.Errorf("scout: visible: %w", err)
	}

	return visible, nil
}

// Interactable returns true if the element can be interacted with.
func (e *Element) Interactable() (bool, error) {
	_, err := e.element.Interactable()
	if err != nil {
		return false, nil //nolint:nilerr // expected: non-interactable returns error
	}

	return true, nil
}

// Disabled returns true if the element is disabled.
func (e *Element) Disabled() (bool, error) {
	disabled, err := e.element.Disabled()
	if err != nil {
		return false, fmt.Errorf("scout: disabled: %w", err)
	}

	return disabled, nil
}

// Screenshot captures a PNG screenshot of the element.
func (e *Element) Screenshot() ([]byte, error) {
	data, err := e.element.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	if err != nil {
		return nil, fmt.Errorf("scout: element screenshot: %w", err)
	}

	return data, nil
}

// ScreenshotJPEG captures a JPEG screenshot of the element with the given quality.
func (e *Element) ScreenshotJPEG(quality int) ([]byte, error) {
	data, err := e.element.Screenshot(proto.PageCaptureScreenshotFormatJpeg, quality)
	if err != nil {
		return nil, fmt.Errorf("scout: element screenshot jpeg: %w", err)
	}

	return data, nil
}

// GetXPath returns the XPath of the element.
func (e *Element) GetXPath() (string, error) {
	xpath, err := e.element.GetXPath(true)
	if err != nil {
		return "", fmt.Errorf("scout: get xpath: %w", err)
	}

	return xpath, nil
}

// Matches checks if the element matches the CSS selector.
func (e *Element) Matches(selector string) (bool, error) {
	matches, err := e.element.Matches(selector)
	if err != nil {
		return false, fmt.Errorf("scout: matches %q: %w", selector, err)
	}

	return matches, nil
}

// ContainsElement checks if the target element is equal to or inside this element.
func (e *Element) ContainsElement(target *Element) (bool, error) {
	contains, err := e.element.ContainsElement(target.element)
	if err != nil {
		return false, fmt.Errorf("scout: contains element: %w", err)
	}

	return contains, nil
}

// Equal checks if two elements refer to the same DOM node.
func (e *Element) Equal(other *Element) (bool, error) {
	eq, err := e.element.Equal(other.element)
	if err != nil {
		return false, fmt.Errorf("scout: equal: %w", err)
	}

	return eq, nil
}

// CanvasToImage returns the image data of a canvas element.
func (e *Element) CanvasToImage(format string, quality float64) ([]byte, error) {
	data, err := e.element.CanvasToImage(format, quality)
	if err != nil {
		return nil, fmt.Errorf("scout: canvas to image: %w", err)
	}

	return data, nil
}

// BackgroundImage returns the CSS background image data.
func (e *Element) BackgroundImage() ([]byte, error) {
	data, err := e.element.BackgroundImage()
	if err != nil {
		return nil, fmt.Errorf("scout: background image: %w", err)
	}

	return data, nil
}

// Resource returns the "src" content of the element (e.g. image data).
func (e *Element) Resource() ([]byte, error) {
	data, err := e.element.Resource()
	if err != nil {
		return nil, fmt.Errorf("scout: resource: %w", err)
	}

	return data, nil
}

// Parent returns the parent element.
func (e *Element) Parent() (*Element, error) {
	obj, err := e.element.Eval(`() => this.parentElement`)
	if err != nil {
		return nil, fmt.Errorf("scout: parent: %w", err)
	}

	if obj.Value.Nil() {
		return nil, fmt.Errorf("scout: parent: no parent element")
	}

	parent, err := e.element.Page().ElementFromObject(obj)
	if err != nil {
		return nil, fmt.Errorf("scout: parent: %w", err)
	}

	return &Element{element: parent}, nil
}

// Parents returns all ancestor elements matching the optional CSS selector.
// Pass empty string to match all ancestors.
func (e *Element) Parents(selector string) ([]*Element, error) {
	page := e.element.Page()

	els, err := page.ElementsByJS(rod.Eval(`(sel) => {
		const els = [];
		let el = this.parentElement;
		while (el) {
			if (!sel || el.matches(sel)) els.push(el);
			el = el.parentElement;
		}
		return els;
	}`, selector).This(e.element.Object))
	if err != nil {
		return nil, fmt.Errorf("scout: parents: %w", err)
	}

	return wrapElements(els), nil
}

// Next returns the next sibling element.
func (e *Element) Next() (*Element, error) {
	obj, err := e.element.Eval(`() => this.nextElementSibling`)
	if err != nil {
		return nil, fmt.Errorf("scout: next: %w", err)
	}

	if obj.Value.Nil() {
		return nil, fmt.Errorf("scout: next: no next sibling")
	}

	next, err := e.element.Page().ElementFromObject(obj)
	if err != nil {
		return nil, fmt.Errorf("scout: next: %w", err)
	}

	return &Element{element: next}, nil
}

// Previous returns the previous sibling element.
func (e *Element) Previous() (*Element, error) {
	obj, err := e.element.Eval(`() => this.previousElementSibling`)
	if err != nil {
		return nil, fmt.Errorf("scout: previous: %w", err)
	}

	if obj.Value.Nil() {
		return nil, fmt.Errorf("scout: previous: no previous sibling")
	}

	prev, err := e.element.Page().ElementFromObject(obj)
	if err != nil {
		return nil, fmt.Errorf("scout: previous: %w", err)
	}

	return &Element{element: prev}, nil
}

// ShadowRoot returns the shadow root of this element.
func (e *Element) ShadowRoot() (*Element, error) {
	sr, err := e.element.ShadowRoot()
	if err != nil {
		return nil, fmt.Errorf("scout: shadow root: %w", err)
	}

	return &Element{element: sr}, nil
}

// Frame creates a Page that represents the iframe content.
func (e *Element) Frame() (*Page, error) {
	frame, err := e.element.Frame()
	if err != nil {
		return nil, fmt.Errorf("scout: frame: %w", err)
	}

	return &Page{page: frame}, nil
}

// Element finds the first child element matching the CSS selector.
func (e *Element) Element(selector string) (*Element, error) {
	page := e.element.Page()

	el, err := page.ElementByJS(rod.Eval(`(sel) => this.querySelector(sel)`, selector).This(e.element.Object))
	if err != nil {
		return nil, fmt.Errorf("scout: child element %q: %w", selector, err)
	}

	return &Element{element: el}, nil
}

// Elements finds all child elements matching the CSS selector.
func (e *Element) Elements(selector string) ([]*Element, error) {
	page := e.element.Page()

	els, err := page.ElementsByJS(rod.Eval(`(sel) => this.querySelectorAll(sel)`, selector).This(e.element.Object))
	if err != nil {
		return nil, fmt.Errorf("scout: child elements %q: %w", selector, err)
	}

	return wrapElements(els), nil
}

// ElementByXPath finds the first child matching the XPath expression relative to this element.
func (e *Element) ElementByXPath(xpath string) (*Element, error) {
	page := e.element.Page()

	el, err := page.ElementByJS(rod.Eval(`(xpath) => {
		const result = document.evaluate(xpath, this, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		return result.singleNodeValue;
	}`, xpath).This(e.element.Object))
	if err != nil {
		return nil, fmt.Errorf("scout: child element xpath %q: %w", xpath, err)
	}

	return &Element{element: el}, nil
}

// ElementsByXPath finds all children matching the XPath expression relative to this element.
func (e *Element) ElementsByXPath(xpath string) ([]*Element, error) {
	page := e.element.Page()

	els, err := page.ElementsByJS(rod.Eval(`(xpath) => {
		const result = document.evaluate(xpath, this, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null);
		const nodes = [];
		for (let i = 0; i < result.snapshotLength; i++) nodes.push(result.snapshotItem(i));
		return nodes;
	}`, xpath).This(e.element.Object))
	if err != nil {
		return nil, fmt.Errorf("scout: child elements xpath %q: %w", xpath, err)
	}

	return wrapElements(els), nil
}

// ElementByText finds the first child element matching the CSS selector whose text matches the regex.
func (e *Element) ElementByText(selector, regex string) (*Element, error) {
	page := e.element.Page()

	el, err := page.ElementByJS(rod.Eval(`(sel, regex) => {
		const re = new RegExp(regex);
		const els = this.querySelectorAll(sel);
		for (const el of els) {
			if (re.test(el.textContent)) return el;
		}
		return null;
	}`, selector, regex).This(e.element.Object))
	if err != nil {
		return nil, fmt.Errorf("scout: child element by text %q %q: %w", selector, regex, err)
	}

	return &Element{element: el}, nil
}

// WaitVisible waits until the element becomes visible.
func (e *Element) WaitVisible() error {
	if err := e.element.WaitVisible(); err != nil {
		return fmt.Errorf("scout: wait visible: %w", err)
	}

	return nil
}

// WaitInvisible waits until the element becomes invisible.
func (e *Element) WaitInvisible() error {
	if err := e.element.WaitInvisible(); err != nil {
		return fmt.Errorf("scout: wait invisible: %w", err)
	}

	return nil
}

// WaitStable waits until the element's shape stops changing for the given duration.
func (e *Element) WaitStable(d time.Duration) error {
	if err := e.element.WaitStable(d); err != nil {
		return fmt.Errorf("scout: wait stable: %w", err)
	}

	return nil
}

// WaitStableRAF waits until the element's shape is stable for 2 animation frames.
func (e *Element) WaitStableRAF() error {
	if err := e.element.WaitStableRAF(); err != nil {
		return fmt.Errorf("scout: wait stable raf: %w", err)
	}

	return nil
}

// WaitInteractable waits for the element to become interactable.
func (e *Element) WaitInteractable() error {
	if _, err := e.element.WaitInteractable(); err != nil {
		return fmt.Errorf("scout: wait interactable: %w", err)
	}

	return nil
}

// WaitEnabled waits until the element is not disabled.
func (e *Element) WaitEnabled() error {
	if err := e.element.WaitEnabled(); err != nil {
		return fmt.Errorf("scout: wait enabled: %w", err)
	}

	return nil
}

// WaitWritable waits until the element is not readonly.
func (e *Element) WaitWritable() error {
	if err := e.element.WaitWritable(); err != nil {
		return fmt.Errorf("scout: wait writable: %w", err)
	}

	return nil
}

// WaitLoad waits for the element to finish loading (e.g. <img>).
func (e *Element) WaitLoad() error {
	if err := e.element.WaitLoad(); err != nil {
		return fmt.Errorf("scout: wait load: %w", err)
	}

	return nil
}

// Eval evaluates JavaScript with this element as `this`.
func (e *Element) Eval(js string, args ...any) (*EvalResult, error) {
	obj, err := e.element.Eval(js, args...)
	if err != nil {
		return nil, fmt.Errorf("scout: element eval: %w", err)
	}

	return remoteObjectToEvalResult(obj), nil
}

// RodElement returns the underlying rod.Element for advanced use cases.
func (e *Element) RodElement() *rod.Element {
	return e.element
}
