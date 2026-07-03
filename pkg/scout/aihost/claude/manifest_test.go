package claude

import (
	"encoding/json"
	"testing"
)

// TestManifestPopulatesFromConsts verifies Manifest() copies every
// package const into the manifest struct, including the nested author
// object shape Claude Code's loader requires.
func TestManifestPopulatesFromConsts(t *testing.T) {
	m, err := Manifest()
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if m.Name != Name {
		t.Errorf("Name = %q, want %q", m.Name, Name)
	}
	if m.Version != Version {
		t.Errorf("Version = %q, want %q", m.Version, Version)
	}
	if m.Description != Description {
		t.Errorf("Description = %q, want %q", m.Description, Description)
	}
	if m.Author.Name != Author {
		t.Errorf("Author.Name = %q, want %q", m.Author.Name, Author)
	}
	if m.Author.URL != Repository {
		t.Errorf("Author.URL = %q, want %q", m.Author.URL, Repository)
	}
	if m.Repository != Repository {
		t.Errorf("Repository = %q, want %q", m.Repository, Repository)
	}
	if m.License != License {
		t.Errorf("License = %q, want %q", m.License, License)
	}
	if len(m.Keywords) != len(Keywords) {
		t.Fatalf("Keywords len = %d, want %d", len(m.Keywords), len(Keywords))
	}
	for i := range Keywords {
		if m.Keywords[i] != Keywords[i] {
			t.Errorf("Keywords[%d] = %q, want %q", i, m.Keywords[i], Keywords[i])
		}
	}
}

// TestPluginJSONShape verifies PluginJSON() emits valid JSON with the
// nested author OBJECT (not a string), a trailing newline, and the
// const-sourced fields. The object-vs-string author shape is
// load-bearing — Claude's manifest validator rejects the string form.
func TestPluginJSONShape(t *testing.T) {
	b, err := PluginJSON()
	if err != nil {
		t.Fatalf("PluginJSON: %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("PluginJSON must end with a trailing newline")
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("PluginJSON not valid JSON: %v", err)
	}
	if doc["name"] != Name {
		t.Errorf("name = %v, want %q", doc["name"], Name)
	}
	if doc["version"] != Version {
		t.Errorf("version = %v, want %q", doc["version"], Version)
	}
	author, ok := doc["author"].(map[string]any)
	if !ok {
		t.Fatalf("author must be an object, got %T (%v)", doc["author"], doc["author"])
	}
	if author["name"] != Author {
		t.Errorf("author.name = %v, want %q", author["name"], Author)
	}
	if author["url"] != Repository {
		t.Errorf("author.url = %v, want %q", author["url"], Repository)
	}
}

// TestMcpJSONShape verifies McpJSON() emits the spec-form .mcp.json:
// top-level "mcpServers" map with the scout entry carrying command +
// verbatim args.
func TestMcpJSONShape(t *testing.T) {
	b, err := McpJSON()
	if err != nil {
		t.Fatalf("McpJSON: %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("McpJSON must end with a trailing newline")
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("McpJSON not valid JSON: %v", err)
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers must be an object, got %T", doc["mcpServers"])
	}
	entry, ok := servers[McpServerName].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers[%q] entry", McpServerName)
	}
	if entry["command"] != McpCommand {
		t.Errorf("command = %v, want %q", entry["command"], McpCommand)
	}
	args, ok := entry["args"].([]any)
	if !ok {
		t.Fatalf("args must be an array, got %T", entry["args"])
	}
	if len(args) != len(McpArgs) {
		t.Fatalf("args len = %d, want %d", len(args), len(McpArgs))
	}
	for i := range McpArgs {
		if args[i] != McpArgs[i] {
			t.Errorf("args[%d] = %v, want %q", i, args[i], McpArgs[i])
		}
	}
}

// TestGeneratedFilesKeysAndPayloads verifies GeneratedFiles() returns exactly the
// three synthesised manifest paths (plugin.json, .mcp.json, hooks/hooks.json) and
// their payloads match the dedicated generators byte-for-byte.
func TestGeneratedFilesKeysAndPayloads(t *testing.T) {
	gen, err := GeneratedFiles()
	if err != nil {
		t.Fatalf("GeneratedFiles: %v", err)
	}
	if len(gen) != 3 {
		t.Fatalf("expected 3 generated files, got %d (%v)", len(gen), keysOf(gen))
	}
	wantPaths := []string{".claude-plugin/plugin.json", ".mcp.json", "hooks/hooks.json"}
	for _, p := range wantPaths {
		if _, ok := gen[p]; !ok {
			t.Errorf("missing generated path %q (have %v)", p, keysOf(gen))
		}
	}
	pj, _ := PluginJSON()
	if string(gen[".claude-plugin/plugin.json"]) != string(pj) {
		t.Errorf("plugin.json payload diverges from PluginJSON()")
	}
	mj, _ := McpJSON()
	if string(gen[".mcp.json"]) != string(mj) {
		t.Errorf(".mcp.json payload diverges from McpJSON()")
	}
	hj, _ := HooksJSON()
	if string(gen["hooks/hooks.json"]) != string(hj) {
		t.Errorf("hooks/hooks.json payload diverges from HooksJSON()")
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
