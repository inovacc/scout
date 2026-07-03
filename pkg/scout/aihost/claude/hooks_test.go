package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

func TestHooksJSON(t *testing.T) {
	b, err := HooksJSON()
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("invalid hooks.json: %v", err)
	}

	// Every CC event binds to its scout hook subcommand, command interpolates McpCommand.
	want := map[string]string{
		"SessionStart": "session-start", "UserPromptSubmit": "prompt-submit",
		"PreToolUse": "pre-tool", "PostToolUse": "post-tool", "Stop": "stop", "SessionEnd": "session-end",
	}
	for event, sub := range want {
		entries, ok := doc.Hooks[event]
		if !ok || len(entries) == 0 || len(entries[0].Hooks) == 0 {
			t.Errorf("hooks.json missing %s", event)
			continue
		}
		h := entries[0].Hooks[0]
		if h.Type != "command" {
			t.Errorf("%s hook type = %q, want command", event, h.Type)
		}
		wantCmd := McpCommand + " hook " + sub
		if h.Command != wantCmd {
			t.Errorf("%s command = %q, want %q", event, h.Command, wantCmd)
		}
		if !strings.HasPrefix(h.Command, McpCommand+" ") {
			t.Errorf("%s command %q must interpolate McpCommand, not hard-code the binary", event, h.Command)
		}
	}

	// Pre/PostToolUse must be scoped to scout's MCP tools; turn/session hooks fire on all.
	if doc.Hooks["PreToolUse"][0].Matcher != "mcp__scout__.*" {
		t.Errorf("PreToolUse matcher = %q, want mcp__scout__.*", doc.Hooks["PreToolUse"][0].Matcher)
	}
	if doc.Hooks["SessionStart"][0].Matcher != "" {
		t.Errorf("SessionStart should have no matcher, got %q", doc.Hooks["SessionStart"][0].Matcher)
	}
}

func TestGeneratedFilesIncludesHooks(t *testing.T) {
	files, err := GeneratedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["hooks/hooks.json"]; !ok {
		t.Errorf("GeneratedFiles missing hooks/hooks.json (%d files)", len(files))
	}
}

func TestHostImplementsHookProvider(t *testing.T) {
	var _ aihost.HookProvider = Host{} // compile-time assertion

	m, err := Host{}.Hooks()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["hooks/hooks.json"]; !ok {
		t.Error("Host.Hooks() missing hooks/hooks.json")
	}
}
