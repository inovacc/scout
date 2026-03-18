// scout-forms is a Scout plugin providing form automation MCP tools.
//
// Install: scout plugin install ./plugins/scout-forms
// Or build: go build -o scout-forms ./plugins/scout-forms
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
	srv.RegisterTool("form_detect", sdk.ToolHandlerFunc(handleFormDetect))
	srv.RegisterTool("form_fill", sdk.ToolHandlerFunc(handleFormFill))
	srv.RegisterTool("form_submit", sdk.ToolHandlerFunc(handleFormSubmit))

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

func handleFormDetect(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
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

	forms, err := page.DetectForms()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return jsonToolResult(forms)
}

func handleFormFill(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	values, ok := args["values"].(map[string]any)

	if !ok || len(values) == 0 {
		return sdk.ErrorResult("values is required"), nil
	}

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

	formValues := make(map[string]string, len(values))
	for k, v := range values {
		if s, ok := v.(string); ok {
			formValues[k] = s
		}
	}

	// Fill form fields via JS eval.
	valuesJSON, _ := json.Marshal(formValues)
	selector, _ := args["selector"].(string)
	if selector == "" {
		selector = "form"
	}

	js := fmt.Sprintf(`(() => {
		const form = document.querySelector(%q);
		if (!form) return 'form not found';
		const vals = %s;
		let filled = 0;
		for (const [name, value] of Object.entries(vals)) {
			const el = form.querySelector('[name="'+name+'"], #'+name);
			if (el) { el.value = value; el.dispatchEvent(new Event('input', {bubbles:true})); filled++; }
		}
		return 'filled ' + filled + ' fields';
	})()`, selector, string(valuesJSON))

	result, err := page.Eval(js)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(result.String()), nil
}

func handleFormSubmit(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	selector, _ := args["selector"].(string)

	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	if selector == "" {
		selector = "form"
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

	// Submit via JS click on submit button.
	js := fmt.Sprintf(`(() => {
		const form = document.querySelector(%q);
		if (!form) return 'form not found';
		const btn = form.querySelector('[type=submit], button:not([type=button])');
		if (btn) { btn.click(); return 'clicked submit'; }
		form.submit();
		return 'submitted';
	})()`, selector)

	_, _ = page.Eval(js)

	_ = page.WaitLoad()

	finalURL, _ := page.URL()

	return sdk.TextResult(fmt.Sprintf("Form submitted. Final URL: %s", finalURL)), nil
}

func jsonToolResult(data any) (*sdk.ToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}
