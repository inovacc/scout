package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// NavigateInput targets a URL on the caller's page.
type NavigateInput struct {
	URL string `json:"url" jsonschema:"the URL to navigate to"`
}

// NavigateOutput reports the landed page after load.
type NavigateOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Navigate drives the given page to in.URL and waits for load (best-effort).
func Navigate(_ context.Context, p *scout.Page, in NavigateInput) (*NavigateOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: navigate: nil page")
	}

	if in.URL == "" {
		return nil, fmt.Errorf("tools: navigate: url required")
	}

	if err := p.Navigate(in.URL); err != nil {
		return nil, fmt.Errorf("tools: navigate: %w", err)
	}

	_ = p.WaitLoad()

	url, _ := p.URL()
	title, _ := p.Title()

	return &NavigateOutput{URL: url, Title: title}, nil
}

// BackInput, ForwardInput and ReloadInput take no fields (operate on the page).
type (
	BackInput    struct{}
	ForwardInput struct{}
	ReloadInput  struct{}
)

// EmptyOutput is returned by verbs with no payload.
type EmptyOutput struct {
	OK bool `json:"ok"`
}

// Back navigates the page back in history.
func Back(_ context.Context, p *scout.Page, _ BackInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: back: nil page")
	}

	if err := p.NavigateBack(); err != nil {
		return nil, fmt.Errorf("tools: back: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// Forward navigates the page forward in history.
func Forward(_ context.Context, p *scout.Page, _ ForwardInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: forward: nil page")
	}

	if err := p.NavigateForward(); err != nil {
		return nil, fmt.Errorf("tools: forward: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// Reload reloads the current page.
func Reload(_ context.Context, p *scout.Page, _ ReloadInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: reload: nil page")
	}

	if err := p.Reload(); err != nil {
		return nil, fmt.Errorf("tools: reload: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// WaitInput waits for page load (Selector empty) or for an element to appear.
type WaitInput struct {
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector to wait for; empty waits for page load"`
}

// Wait blocks until the page finishes loading or the selector resolves.
func Wait(_ context.Context, p *scout.Page, in WaitInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: wait: nil page")
	}

	if in.Selector == "" {
		_ = p.WaitLoad()
		return &EmptyOutput{OK: true}, nil
	}

	if _, err := p.Element(in.Selector); err != nil {
		return nil, fmt.Errorf("tools: wait: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// ClickInput selects an element to click.
type ClickInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector to click"`
}

// TypeInput selects an input element and the text to type into it.
type TypeInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector of the input"`
	Text     string `json:"text"     jsonschema:"text to type"`
}

// ExtractInput selects an element to read text from.
type ExtractInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector to extract text from"`
}

// ExtractOutput holds the extracted text.
type ExtractOutput struct {
	Text string `json:"text"`
}

// EvalInput carries a JavaScript expression to evaluate.
type EvalInput struct {
	Expression string `json:"expression" jsonschema:"JavaScript expression to evaluate"`
}

// EvalOutput holds the stringified evaluation result.
type EvalOutput struct {
	Result string `json:"result"`
}

// Click clicks the first element matching the selector.
func Click(_ context.Context, p *scout.Page, in ClickInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: click: nil page")
	}

	if in.Selector == "" {
		return nil, fmt.Errorf("tools: click: selector required")
	}

	el, err := p.Element(in.Selector)
	if err != nil {
		return nil, fmt.Errorf("tools: click: %w", err)
	}

	if err := el.Click(); err != nil {
		return nil, fmt.Errorf("tools: click: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// Type inputs text into the first element matching the selector.
func Type(_ context.Context, p *scout.Page, in TypeInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: type: nil page")
	}

	if in.Selector == "" {
		return nil, fmt.Errorf("tools: type: selector required")
	}

	el, err := p.Element(in.Selector)
	if err != nil {
		return nil, fmt.Errorf("tools: type: %w", err)
	}

	if err := el.Input(in.Text); err != nil {
		return nil, fmt.Errorf("tools: type: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// Extract returns the text content of the first matching element.
func Extract(_ context.Context, p *scout.Page, in ExtractInput) (*ExtractOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: extract: nil page")
	}

	if in.Selector == "" {
		return nil, fmt.Errorf("tools: extract: selector required")
	}

	text, err := p.ExtractText(in.Selector)
	if err != nil {
		return nil, fmt.Errorf("tools: extract: %w", err)
	}

	return &ExtractOutput{Text: text}, nil
}

// Eval evaluates a JavaScript expression and returns its stringified result.
func Eval(_ context.Context, p *scout.Page, in EvalInput) (*EvalOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: eval: nil page")
	}

	if in.Expression == "" {
		return nil, fmt.Errorf("tools: eval: expression required")
	}

	res, err := p.Eval(in.Expression)
	if err != nil {
		return nil, fmt.Errorf("tools: eval: %w", err)
	}

	return &EvalOutput{Result: res.String()}, nil
}

// HTMLInput takes no fields (reads the current page).
type HTMLInput struct{}

// HTMLOutput holds the page HTML.
type HTMLOutput struct {
	HTML string `json:"html"`
}

// MarkdownInput takes no fields (reads the current page).
type MarkdownInput struct{}

// MarkdownOutput holds the page rendered as Markdown.
type MarkdownOutput struct {
	Markdown string `json:"markdown"`
}

// CookiesInput takes no fields (reads the current page).
type CookiesInput struct{}

// CookiesOutput holds the page cookies. The concrete type is the engine
// cookie slice, carried as any to avoid importing it here.
type CookiesOutput struct {
	Cookies any `json:"cookies"`
}

// URLInput takes no fields (reads the current page).
type URLInput struct{}

// URLOutput holds the current page URL.
type URLOutput struct {
	URL string `json:"url"`
}

// TitleInput takes no fields (reads the current page).
type TitleInput struct{}

// TitleOutput holds the current page title.
type TitleOutput struct {
	Title string `json:"title"`
}

// HTML returns the full HTML of the current page.
func HTML(_ context.Context, p *scout.Page, _ HTMLInput) (*HTMLOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: html: nil page")
	}

	h, err := p.HTML()
	if err != nil {
		return nil, fmt.Errorf("tools: html: %w", err)
	}

	return &HTMLOutput{HTML: h}, nil
}

// Markdown returns the current page rendered as Markdown.
func Markdown(_ context.Context, p *scout.Page, _ MarkdownInput) (*MarkdownOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: markdown: nil page")
	}

	m, err := p.Markdown()
	if err != nil {
		return nil, fmt.Errorf("tools: markdown: %w", err)
	}

	return &MarkdownOutput{Markdown: m}, nil
}

// Cookies returns the cookies visible to the current page.
func Cookies(_ context.Context, p *scout.Page, _ CookiesInput) (*CookiesOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: cookies: nil page")
	}

	c, err := p.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("tools: cookies: %w", err)
	}

	return &CookiesOutput{Cookies: c}, nil
}

// URL returns the current page URL.
func URL(_ context.Context, p *scout.Page, _ URLInput) (*URLOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: url: nil page")
	}

	u, err := p.URL()
	if err != nil {
		return nil, fmt.Errorf("tools: url: %w", err)
	}

	return &URLOutput{URL: u}, nil
}

// Title returns the current page title.
func Title(_ context.Context, p *scout.Page, _ TitleInput) (*TitleOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: title: nil page")
	}

	t, err := p.Title()
	if err != nil {
		return nil, fmt.Errorf("tools: title: %w", err)
	}

	return &TitleOutput{Title: t}, nil
}
