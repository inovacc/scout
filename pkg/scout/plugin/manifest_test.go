package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()

	manifest := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"description": "A test plugin",
		"author": "tester",
		"command": "./test-plugin",
		"capabilities": ["scraper_mode", "mcp_tool"],
		"modes": [{"name": "test-mode", "description": "Test mode"}],
		"tools": [{"name": "test-tool", "description": "Test tool"}]
	}`

	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}

	if m.Name != "test-plugin" {
		t.Errorf("name = %q, want %q", m.Name, "test-plugin")
	}

	if m.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", m.Version, "1.0.0")
	}

	if len(m.Capabilities) != 2 {
		t.Errorf("capabilities len = %d, want 2", len(m.Capabilities))
	}

	if len(m.Modes) != 1 {
		t.Errorf("modes len = %d, want 1", len(m.Modes))
	}

	if len(m.Tools) != 1 {
		t.Errorf("tools len = %d, want 1", len(m.Tools))
	}

	if m.Dir != dir {
		t.Errorf("dir = %q, want %q", m.Dir, dir)
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	_, err := LoadManifest(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing plugin.json")
	}
}

func TestLoadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadManifest_Validation(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{"missing name", `{"version":"1.0","command":"./x","capabilities":["mcp_tool"]}`},
		{"missing version", `{"name":"x","command":"./x","capabilities":["mcp_tool"]}`},
		{"missing command", `{"name":"x","version":"1.0","capabilities":["mcp_tool"]}`},
		{"no capabilities", `{"name":"x","version":"1.0","command":"./x","capabilities":[]}`},
		{"bad capability", `{"name":"x","version":"1.0","command":"./x","capabilities":["invalid"]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(tt.manifest), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := LoadManifest(dir)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestManifest_HasCapability(t *testing.T) {
	m := &Manifest{Capabilities: []string{"scraper_mode", "mcp_tool"}}

	if !m.HasCapability("scraper_mode") {
		t.Error("expected HasCapability(scraper_mode) = true")
	}

	if m.HasCapability("extractor") {
		t.Error("expected HasCapability(extractor) = false")
	}
}

func TestManifest_CommandPath(t *testing.T) {
	m := &Manifest{Command: "./my-plugin", Dir: "/home/user/.scout/plugins/test"}
	got := m.CommandPath()
	want := filepath.Join("/home/user/.scout/plugins/test", "my-plugin")

	if got != want {
		t.Errorf("CommandPath() = %q, want %q", got, want)
	}

	absCmd := filepath.Join(os.TempDir(), "plugin")
	m2 := &Manifest{Command: absCmd, Dir: "/tmp"}
	if m2.CommandPath() != absCmd {
		t.Errorf("absolute command should be returned as-is, got %q", m2.CommandPath())
	}
}
