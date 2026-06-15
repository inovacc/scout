package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// locSelector is the shared JSON-schema fragment describing how to find an
// element by a Playwright-style strategy. Embedded into every locator/expect
// tool so AI agents target elements by role/text/label instead of brittle CSS.
const locSelector = `"strategy":{"type":"string","enum":["role","text","label","placeholder","testid","alttext","title","css"],"description":"how to find the element: role (ARIA role like button/link/textbox), text, label (form label), placeholder, testid (data-testid), alttext, title, or css"},"value":{"type":"string","description":"the value to match for the chosen strategy (role name, text, selector, ...)"},"name":{"type":"string","description":"accessible-name filter, mainly for strategy=role (e.g. the button label)"},"exact":{"type":"boolean","description":"require an exact, case-sensitive match instead of case-insensitive substring"}`

// locArgs are the shared locator-selector arguments.
type locArgs struct {
	Strategy string `json:"strategy"`
	Value    string `json:"value"`
	Name     string `json:"name"`
	Exact    bool   `json:"exact"`
}

// locatorFor builds a facade Locator from the selector arguments.
func locatorFor(page *scout.Page, a locArgs) (*scout.Locator, error) {
	var opts []scout.LocatorOption
	if a.Name != "" {
		opts = append(opts, scout.WithName(a.Name))
	}
	if a.Exact {
		opts = append(opts, scout.WithExact(true))
	}

	switch a.Strategy {
	case "role":
		return page.GetByRole(a.Value, opts...), nil
	case "text":
		return page.GetByText(a.Value, opts...), nil
	case "label":
		return page.GetByLabel(a.Value, opts...), nil
	case "placeholder":
		return page.GetByPlaceholder(a.Value, opts...), nil
	case "testid":
		return page.GetByTestID(a.Value), nil
	case "alttext":
		return page.GetByAltText(a.Value, opts...), nil
	case "title":
		return page.GetByTitle(a.Value, opts...), nil
	case "css", "":
		return page.Locate(a.Value), nil
	default:
		return nil, fmt.Errorf("scout: unknown locator strategy %q", a.Strategy)
	}
}

// registerLocatorTools adds Playwright-style locator actions and web-first
// assertions, so MCP clients target elements by role/text/label and assert
// with retrying expectations rather than scripting raw CSS + sleeps.
func registerLocatorTools(server *mcp.Server, state *mcpState) {
	// resolvePage unmarshals selector args and returns the page + locator.
	resolve := func(ctx context.Context, raw json.RawMessage, extra any) (*scout.Locator, locArgs, error) {
		var a locArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, a, err
		}
		if extra != nil {
			if err := json.Unmarshal(raw, extra); err != nil {
				return nil, a, err
			}
		}
		page, err := state.ensurePage(ctx)
		if err != nil {
			return nil, a, err
		}
		loc, err := locatorFor(page, a)
		return loc, a, err
	}

	addTracedTool(server, &mcp.Tool{
		Name:        "locator_click",
		Description: "Click an element located by role/text/label/etc. Auto-waits for it to be actionable.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` + locSelector + `},"required":["strategy","value"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		loc, a, err := resolve(ctx, req.Params.Arguments, nil)
		if err != nil {
			return errResult(err.Error())
		}
		if err := loc.Click(); err != nil {
			return errResult(err.Error())
		}
		return textResult(fmt.Sprintf("Clicked %s=%q", a.Strategy, a.Value))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "locator_fill",
		Description: "Fill an input located by role/text/label/etc. with text. Auto-waits.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` + locSelector + `,"text":{"type":"string","description":"text to fill in"}},"required":["strategy","value","text"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var extra struct {
			Text string `json:"text"`
		}
		loc, a, err := resolve(ctx, req.Params.Arguments, &extra)
		if err != nil {
			return errResult(err.Error())
		}
		if err := loc.Fill(extra.Text); err != nil {
			return errResult(err.Error())
		}
		return textResult(fmt.Sprintf("Filled %s=%q", a.Strategy, a.Value))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "locator_text",
		Description: "Get the text content of an element located by role/text/label/etc.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` + locSelector + `},"required":["strategy","value"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		loc, _, err := resolve(ctx, req.Params.Arguments, nil)
		if err != nil {
			return errResult(err.Error())
		}
		txt, err := loc.TextContent()
		if err != nil {
			return errResult(err.Error())
		}
		return textResult(txt)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "locator_count",
		Description: "Count how many elements match a role/text/label/etc. locator.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` + locSelector + `},"required":["strategy","value"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		loc, _, err := resolve(ctx, req.Params.Arguments, nil)
		if err != nil {
			return errResult(err.Error())
		}
		n, err := loc.Count()
		if err != nil {
			return errResult(err.Error())
		}
		return textResult(fmt.Sprintf("%d", n))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "expect_visible",
		Description: "Web-first assertion: wait until an element (role/text/label/etc.) is visible. Set negate=true to assert it is hidden/absent.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` + locSelector + `,"negate":{"type":"boolean","description":"assert NOT visible"},"timeout_ms":{"type":"integer","description":"max wait in milliseconds (default 5000)"}},"required":["strategy","value"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var extra struct {
			Negate    bool `json:"negate"`
			TimeoutMs int  `json:"timeout_ms"`
		}
		loc, a, err := resolve(ctx, req.Params.Arguments, &extra)
		if err != nil {
			return errResult(err.Error())
		}
		assertion := scout.Expect(loc, expectOpts(extra.TimeoutMs)...)
		if extra.Negate {
			assertion = assertion.Not()
		}
		if err := assertion.ToBeVisible(); err != nil {
			return errResult(err.Error())
		}
		return textResult(fmt.Sprintf("OK: %s=%q visible=%v", a.Strategy, a.Value, !extra.Negate))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "expect_text",
		Description: "Web-first assertion: wait until an element's text equals (or, with contains=true, contains) the expected text.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` + locSelector + `,"text":{"type":"string","description":"expected text"},"contains":{"type":"boolean","description":"substring match instead of full equality"},"timeout_ms":{"type":"integer","description":"max wait in milliseconds (default 5000)"}},"required":["strategy","value","text"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var extra struct {
			Text      string `json:"text"`
			Contains  bool   `json:"contains"`
			TimeoutMs int    `json:"timeout_ms"`
		}
		loc, a, err := resolve(ctx, req.Params.Arguments, &extra)
		if err != nil {
			return errResult(err.Error())
		}
		assertion := scout.Expect(loc, expectOpts(extra.TimeoutMs)...)
		if extra.Contains {
			err = assertion.ToContainText(extra.Text)
		} else {
			err = assertion.ToHaveText(extra.Text)
		}
		if err != nil {
			return errResult(err.Error())
		}
		return textResult(fmt.Sprintf("OK: %s=%q text matches", a.Strategy, a.Value))
	})
}

// expectOpts builds the ExpectOption slice for an optional timeout in ms.
func expectOpts(timeoutMs int) []scout.ExpectOption {
	if timeoutMs > 0 {
		return []scout.ExpectOption{scout.WithExpectTimeout(time.Duration(timeoutMs) * time.Millisecond)}
	}
	return nil
}
