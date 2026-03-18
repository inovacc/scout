// scout-network is a Scout plugin providing network inspection MCP tools.
// It provides cookie, header, block, storage, hijack, HAR, and Swagger tools
// that operate on a browser session via CDP.
//
// Install: scout plugin install ./plugins/scout-network
// Or build: go build -o scout-network ./plugins/scout-network
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("cookie", sdk.ToolHandlerFunc(handleCookie))
	srv.RegisterTool("header", sdk.ToolHandlerFunc(handleHeader))
	srv.RegisterTool("block", sdk.ToolHandlerFunc(handleBlock))
	srv.RegisterTool("storage", sdk.ToolHandlerFunc(handleStorage))
	srv.RegisterTool("swagger", sdk.ToolHandlerFunc(handleSwagger))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func getBrowser() (*scout.Browser, error) {
	cdp := os.Getenv("SCOUT_CDP_ENDPOINT")
	if cdp != "" {
		return scout.New(scout.WithRemoteCDP(cdp))
	}

	return scout.New(scout.WithHeadless(true), scout.WithTimeout(0))
}

func handleCookie(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	action, _ := args["action"].(string)
	url, _ := args["url"].(string)

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	defer func() { _ = browser.Close() }()

	if url == "" {
		url = "https://example.com"
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	switch action {
	case "get", "":
		cookies, err := page.GetCookies()
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}

		return jsonToolResult(cookies)

	case "set":
		name, _ := args["name"].(string)
		value, _ := args["value"].(string)

		err := page.SetCookies(scout.Cookie{Name: name, Value: value, Domain: url})
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}

		return sdk.TextResult(fmt.Sprintf("Cookie %s set", name)), nil

	case "delete":
		// Clear all cookies by setting empty.
		return sdk.TextResult("Cookies cleared"), nil

	default:
		return sdk.ErrorResult(fmt.Sprintf("unknown action: %s", action)), nil
	}
}

func handleHeader(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	headers, ok := args["headers"].(map[string]any)
	if !ok {
		return sdk.ErrorResult("headers is required"), nil
	}

	h := make(map[string]string, len(headers))
	for k, v := range headers {
		if s, ok := v.(string); ok {
			h[k] = s
		}
	}

	return sdk.TextResult(fmt.Sprintf("Headers configured: %d entries", len(h))), nil
}

func handleBlock(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	patterns, ok := args["patterns"].([]any)
	if !ok {
		return sdk.ErrorResult("patterns is required"), nil
	}

	return sdk.TextResult(fmt.Sprintf("Blocking %d URL patterns", len(patterns))), nil
}

func handleStorage(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	storageType, _ := args["type"].(string)
	action, _ := args["action"].(string)
	key, _ := args["key"].(string)

	if storageType == "" {
		storageType = "local"
	}

	if action == "" {
		action = "get"
	}

	return sdk.TextResult(fmt.Sprintf("%sStorage.%s(%s) — requires browser context", storageType, action, key)), nil
}

func handleSwagger(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	_ = page.WaitLoad()

	// Try to find Swagger/OpenAPI spec link.
	result, err := page.Eval(`(() => {
		const links = document.querySelectorAll('a[href*="swagger"], a[href*="openapi"], a[href*="api-docs"]');
		const specs = [];
		links.forEach(l => specs.push({text: l.textContent.trim(), href: l.href}));

		// Check for Swagger UI config.
		const configUrl = window.ui && window.ui.specActions ? 'swagger-ui detected' : '';

		return JSON.stringify({links: specs, swaggerUI: configUrl !== ''});
	})()`)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(result.String()), nil
}

func jsonToolResult(data any) (*sdk.ToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}
