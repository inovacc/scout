package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestToolSchemas(t *testing.T) {
	// Provider without browser — just test schema generation.
	p := &Provider{}
	p.registerBuiltinTools()

	if len(p.Tools()) != 9 {
		t.Errorf("tools count = %d, want 9", len(p.Tools()))
	}

	// OpenAI format.
	openai := p.OpenAITools()
	if len(openai) != 9 {
		t.Errorf("OpenAI tools = %d", len(openai))
	}

	for _, tool := range openai {
		if tool["type"] != "function" {
			t.Errorf("tool type = %v, want function", tool["type"])
		}

		fn, ok := tool["function"].(map[string]any)
		if !ok {
			t.Fatal("function field missing")
		}

		if fn["name"] == nil || fn["name"] == "" {
			t.Error("tool name is empty")
		}
	}

	// Anthropic format.
	anthropic := p.AnthropicTools()
	if len(anthropic) != 9 {
		t.Errorf("Anthropic tools = %d", len(anthropic))
	}

	for _, tool := range anthropic {
		if tool["name"] == nil || tool["name"] == "" {
			t.Error("anthropic tool name empty")
		}

		if tool["input_schema"] == nil {
			t.Error("anthropic input_schema missing")
		}
	}

	// JSON export.
	data, err := p.ToolSchemaJSON()
	if err != nil {
		t.Fatalf("ToolSchemaJSON: %v", err)
	}

	var parsed []any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(parsed) != 9 {
		t.Errorf("JSON tools = %d", len(parsed))
	}
}

func TestParams(t *testing.T) {
	p := params("url", "string", "The URL", true)

	if p["type"] != "object" {
		t.Errorf("type = %v", p["type"])
	}

	props, ok := p["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing")
	}

	if props["url"] == nil {
		t.Error("url prop missing")
	}

	reqd, ok := p["required"].([]string)
	if !ok || len(reqd) != 1 || reqd[0] != "url" {
		t.Errorf("required = %v", p["required"])
	}
}

func TestParamsMulti(t *testing.T) {
	p := paramsMulti(
		param("selector", "string", "CSS sel", true),
		param("text", "string", "Text", true),
	)

	props := p["properties"].(map[string]any)
	if len(props) != 2 {
		t.Errorf("props = %d", len(props))
	}

	reqd := p["required"].([]string)
	if len(reqd) != 2 {
		t.Errorf("required = %d", len(reqd))
	}
}

func TestEmptyParams(t *testing.T) {
	p := emptyParams()

	if p["type"] != "object" {
		t.Errorf("type = %v", p["type"])
	}
}

func TestCallUnknownTool(t *testing.T) {
	p := &Provider{}
	p.registerBuiltinTools()

	_, err := p.Call(context.TODO(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}
