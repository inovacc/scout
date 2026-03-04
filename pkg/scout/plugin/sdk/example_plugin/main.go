// Command example_plugin demonstrates a minimal Scout plugin.
//
// Build: go build -o example_plugin .
// Install: mkdir -p ~/.scout/plugins/example && cp example_plugin plugin.json ~/.scout/plugins/example/
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

type exampleMode struct{}

func (m *exampleMode) Scrape(_ context.Context, params sdk.ScrapeParams) ([]sdk.Result, error) {
	return []sdk.Result{
		{
			Type:      "post",
			Source:    "example",
			ID:        "1",
			Timestamp: time.Now().Format(time.RFC3339),
			Content:   fmt.Sprintf("Example result for mode %s", params.Mode),
		},
	}, nil
}

type exampleTool struct{}

func (t *exampleTool) Call(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		name = "world"
	}

	return sdk.TextResult(fmt.Sprintf("Hello, %s!", name)), nil
}

func main() {
	s := sdk.NewServer()
	s.RegisterMode("example", &exampleMode{})
	s.RegisterTool("greet", &exampleTool{})

	if err := s.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "plugin error: %v\n", err)
	}
}
