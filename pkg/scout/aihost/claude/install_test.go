package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteAssetsRoundtrip writes the rendered plugin to a tempdir and
// verifies the required files all landed.
func TestWriteAssetsRoundtrip(t *testing.T) {
	dir := t.TempDir()
	n, err := WriteAssets(dir)
	if err != nil {
		t.Fatalf("WriteAssets: %v", err)
	}
	if n < 17 {
		t.Fatalf("expected >=17 files written, got %d", n)
	}
	required := []string{
		".claude-plugin/plugin.json",
		".mcp.json",
		"agents/browser-automation.md",
		"agents/site-tester.md",
		"agents/web-scraper.md",
		"skills/scrape/SKILL.md",
		"skills/screenshot/SKILL.md",
		"commands/scout-scrape.md",
		"commands/scout-screenshot.md",
	}
	for _, r := range required {
		if _, err := os.Stat(filepath.Join(dir, r)); err != nil {
			t.Errorf("required file missing: %s (%v)", r, err)
		}
	}
}

// TestPatchSettingsPreservesUnrelatedKeys verifies the settings.json
// patcher only touches enabledPlugins[<our key>] and leaves every
// other top-level key + other marketplace entries intact.
func TestPatchSettingsPreservesUnrelatedKeys(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	settingsPath := filepath.Join(tmp, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	pre := map[string]any{
		"theme":       "dark",
		"effortLevel": "medium",
		"enabledPlugins": map[string]any{
			"some-other@marketplace": true,
		},
		"customKey": "must-survive",
	}
	preBytes, _ := json.Marshal(pre)
	if err := os.WriteFile(settingsPath, preBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchSettings(); err != nil {
		t.Fatalf("PatchSettings: %v", err)
	}
	raw, _ := os.ReadFile(settingsPath)
	var post map[string]any
	if err := json.Unmarshal(raw, &post); err != nil {
		t.Fatalf("post parse: %v", err)
	}
	if post["theme"] != "dark" || post["customKey"] != "must-survive" || post["effortLevel"] != "medium" {
		t.Errorf("unrelated keys mutated: %v", post)
	}
	enabled := post["enabledPlugins"].(map[string]any)
	if enabled["some-other@marketplace"] != true {
		t.Errorf("unrelated enabledPlugins entry lost: %v", enabled)
	}
	if enabled[pluginEnabledKey] != true {
		t.Errorf("expected enabledPlugins[%q]=true, got %v", pluginEnabledKey, enabled[pluginEnabledKey])
	}
}

// TestPatchMarketplaceCreatesAndUpdates verifies marketplace creation
// and idempotent re-run (running twice leaves the file in the same
// shape and MarketplaceHasEntry stays true).
func TestPatchMarketplaceCreatesAndUpdates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := PatchMarketplace(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !MarketplaceHasEntry() {
		t.Fatal("entry not present after create")
	}
	if err := PatchMarketplace(); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	if !MarketplaceHasEntry() {
		t.Fatal("entry lost after idempotent re-run")
	}
	path, _ := marketplaceJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read marketplace.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse marketplace.json: %v", err)
	}
	plugins, _ := doc["plugins"].([]any)
	if len(plugins) != 1 {
		t.Fatalf("expected exactly 1 plugin entry, got %d", len(plugins))
	}
	entry, _ := plugins[0].(map[string]any)
	if entry["name"] != Name {
		t.Errorf("plugin name mismatch: want %q got %v", Name, entry["name"])
	}
}
