// Package agent provides AI agent framework integration for Scout.
// It generates tool schemas and adapters for OpenAI, Anthropic, and LangChain.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// Tool describes a Scout capability as an AI agent tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Handler     ToolHandler    `json:"-"`
}

// ToolHandler executes a tool call.
type ToolHandler func(ctx context.Context, args map[string]any) (string, error)

// ToolResult is the result of a tool execution.
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// Provider exposes Scout tools to AI agent frameworks.
type Provider struct {
	tools   []Tool
	browser *scout.Browser
}

// NewProvider creates a tool provider with a shared browser instance.
func NewProvider(browser *scout.Browser) *Provider {
	p := &Provider{browser: browser}
	p.registerBuiltinTools()

	return p
}

// Tools returns all registered tools.
func (p *Provider) Tools() []Tool {
	return p.tools
}

// Call executes a tool by name with the given arguments.
func (p *Provider) Call(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	for _, t := range p.tools {
		if t.Name == name {
			result, err := t.Handler(ctx, args)
			if err != nil {
				return &ToolResult{Content: err.Error(), IsError: true}, nil //nolint:nilerr // intentional: wrap handler error as tool error result
			}

			return &ToolResult{Content: result}, nil
		}
	}

	return nil, fmt.Errorf("agent: unknown tool %q", name)
}

// OpenAITools returns tool schemas in OpenAI function calling format.
func (p *Provider) OpenAITools() []map[string]any {
	tools := make([]map[string]any, 0, len(p.tools))

	for _, t := range p.tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}

	return tools
}

// AnthropicTools returns tool schemas in Anthropic tool_use format.
func (p *Provider) AnthropicTools() []map[string]any {
	tools := make([]map[string]any, 0, len(p.tools))

	for _, t := range p.tools {
		tools = append(tools, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.Parameters,
		})
	}

	return tools
}

// ToolSchemaJSON returns all tool schemas as JSON.
func (p *Provider) ToolSchemaJSON() ([]byte, error) {
	return json.MarshalIndent(p.OpenAITools(), "", "  ")
}

func (p *Provider) registerBuiltinTools() {
	p.tools = []Tool{
		{
			Name:        "navigate",
			Description: "Navigate the browser to a URL and wait for the page to load",
			Parameters:  params("url", "string", "The URL to navigate to", true),
			Handler:     p.handleNavigate,
		},
		{
			Name:        "screenshot",
			Description: "Take a screenshot of the current page",
			Parameters:  params("fullPage", "boolean", "Capture the full scrollable page", false),
			Handler:     p.handleScreenshot,
		},
		{
			Name:        "extract_text",
			Description: "Extract text content from an element using a CSS selector",
			Parameters:  params("selector", "string", "CSS selector for the element", true),
			Handler:     p.handleExtractText,
		},
		{
			Name:        "click",
			Description: "Click an element on the page",
			Parameters:  params("selector", "string", "CSS selector for the element to click", true),
			Handler:     p.handleClick,
		},
		{
			Name:        "type_text",
			Description: "Type text into an input element",
			Parameters: paramsMulti(
				param("selector", "string", "CSS selector for the input", true),
				param("text", "string", "Text to type", true),
			),
			Handler: p.handleType,
		},
		{
			Name:        "markdown",
			Description: "Extract the current page content as Markdown",
			Parameters:  params("mainOnly", "boolean", "Extract only main content", false),
			Handler:     p.handleMarkdown,
		},
		{
			Name:        "eval",
			Description: "Evaluate JavaScript in the page context and return the result",
			Parameters:  params("script", "string", "JavaScript code to evaluate", true),
			Handler:     p.handleEval,
		},
		{
			Name:        "page_url",
			Description: "Get the current page URL",
			Parameters:  emptyParams(),
			Handler:     p.handleURL,
		},
		{
			Name:        "page_title",
			Description: "Get the current page title",
			Parameters:  emptyParams(),
			Handler:     p.handleTitle,
		},
	}
}

func (p *Provider) ensurePage(_ context.Context, url string) (*scout.Page, error) {
	if url != "" {
		page, err := p.browser.NewPage(url)
		if err != nil {
			return nil, err
		}

		_ = page.WaitLoad()

		return page, nil
	}

	pages, _ := p.browser.Pages()
	if len(pages) == 0 {
		return nil, fmt.Errorf("no page open — provide a url")
	}

	return pages[0], nil
}

func (p *Provider) handleNavigate(ctx context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)

	page, err := p.ensurePage(ctx, url)
	if err != nil {
		return "", err
	}

	title, _ := page.Title()

	return fmt.Sprintf("Navigated to %s (title: %s)", url, title), nil
}

func (p *Provider) handleScreenshot(ctx context.Context, args map[string]any) (string, error) {
	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	data, err := page.Screenshot()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Screenshot captured (%d bytes)", len(data)), nil
}

func (p *Provider) handleExtractText(ctx context.Context, args map[string]any) (string, error) {
	selector, _ := args["selector"].(string)

	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	result, err := page.Eval(fmt.Sprintf(`document.querySelector(%q)?.textContent?.trim() || ''`, selector))
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func (p *Provider) handleClick(ctx context.Context, args map[string]any) (string, error) {
	selector, _ := args["selector"].(string)

	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	_, err = page.Eval(fmt.Sprintf(`document.querySelector(%q)?.click()`, selector))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Clicked %s", selector), nil
}

func (p *Provider) handleType(ctx context.Context, args map[string]any) (string, error) {
	selector, _ := args["selector"].(string)
	text, _ := args["text"].(string)

	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	js := fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		if (!el) return 'element not found';
		el.value = %q;
		el.dispatchEvent(new Event('input', {bubbles: true}));
		return 'typed';
	})()`, selector, text)

	result, err := page.Eval(js)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func (p *Provider) handleMarkdown(ctx context.Context, args map[string]any) (string, error) {
	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	var opts []scout.MarkdownOption

	if mainOnly, ok := args["mainOnly"].(bool); ok && mainOnly {
		opts = append(opts, scout.WithMainContentOnly())
	}

	return page.Markdown(opts...)
}

func (p *Provider) handleEval(ctx context.Context, args map[string]any) (string, error) {
	script, _ := args["script"].(string)

	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	result, err := page.Eval(script)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func (p *Provider) handleURL(ctx context.Context, _ map[string]any) (string, error) {
	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	return page.URL()
}

func (p *Provider) handleTitle(ctx context.Context, _ map[string]any) (string, error) {
	page, err := p.ensurePage(ctx, "")
	if err != nil {
		return "", err
	}

	return page.Title()
}

// Schema helpers.
func params(name, typ, desc string, required bool) map[string]any {
	props := map[string]any{
		name: map[string]any{"type": typ, "description": desc},
	}

	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}

	if required {
		schema["required"] = []string{name}
	}

	return schema
}

func param(name, paramType, desc string, required bool) map[string]any {
	return map[string]any{"name": name, "type": paramType, "description": desc, "required": required}
}

func paramsMulti(fields ...map[string]any) map[string]any {
	props := map[string]any{}
	var reqd []string

	for _, f := range fields {
		name := f["name"].(string)
		props[name] = map[string]any{
			"type":        f["type"],
			"description": f["description"],
		}

		if req, ok := f["required"].(bool); ok && req {
			reqd = append(reqd, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}

	if len(reqd) > 0 {
		schema["required"] = reqd
	}

	return schema
}

func emptyParams() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
