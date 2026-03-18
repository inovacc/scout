// scout-guide is a Scout plugin providing step-by-step guide recording MCP tools.
//
// Install: scout plugin install ./plugins/scout-guide
// Or build: go build -o scout-guide ./plugins/scout-guide
package main

import (
	"context"
	"log"
	"sync"

	"github.com/inovacc/scout/pkg/scout/guide"
	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

var (
	mu       sync.Mutex
	recorder *guide.Recorder
)

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("guide_start", sdk.ToolHandlerFunc(handleGuideStart))
	srv.RegisterTool("guide_step", sdk.ToolHandlerFunc(handleGuideStep))
	srv.RegisterTool("guide_finish", sdk.ToolHandlerFunc(handleGuideFinish))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func handleGuideStart(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	title, _ := args["title"].(string)
	if title == "" {
		return sdk.ErrorResult("title is required"), nil
	}

	mu.Lock()
	defer mu.Unlock()

	recorder = guide.NewRecorder()

	if err := recorder.Start(title, ""); err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult("Guide started: " + title), nil
}

func handleGuideStep(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	instruction, _ := args["instruction"].(string)
	if instruction == "" {
		return sdk.ErrorResult("instruction is required"), nil
	}

	mu.Lock()
	defer mu.Unlock()

	if recorder == nil {
		return sdk.ErrorResult("no guide in progress — call guide_start first"), nil
	}

	if err := recorder.AddStep("", "", instruction, nil); err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult("Step added: " + instruction), nil
}

func handleGuideFinish(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
	mu.Lock()
	defer mu.Unlock()

	if recorder == nil {
		return sdk.ErrorResult("no guide in progress — call guide_start first"), nil
	}

	g, err := recorder.Finish()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	md, err := guide.RenderMarkdown(g)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	recorder = nil

	return sdk.TextResult(string(md)), nil
}
