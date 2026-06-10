package claude

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errStop is a sentinel used by Walk callback-error tests.
var errStop = errors.New("stop walking")

// setHome points os.UserHomeDir() at a temp dir on both Windows
// (USERPROFILE) and Unix (HOME).
func setHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)
	return tmp
}

func writeSettings(t *testing.T, home string, doc map[string]any) string {
	t.Helper()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return doc
}

// TestWriteAssetsEmptyTargetErrors checks the empty-target guard.
func TestWriteAssetsEmptyTargetErrors(t *testing.T) {
	n, err := WriteAssets("")
	if err == nil {
		t.Fatal("expected error for empty target")
	}
	if n != 0 {
		t.Errorf("count = %d, want 0 on error", n)
	}
}

// TestWriteAssetsSweepsStaleFiles verifies the second WriteAssets run
// removes a stale file we plant under commands/ that is not part of the
// generated set, while keeping a legitimately generated file.
func TestWriteAssetsSweepsStaleFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAssets(dir); err != nil {
		t.Fatalf("first WriteAssets: %v", err)
	}
	stale := filepath.Join(dir, "commands", "zzz-stale-orphan.md")
	if err := os.WriteFile(stale, []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real generated command file we expect to survive.
	survivor := filepath.Join(dir, "commands", "scout-scrape.md")
	if _, err := os.Stat(survivor); err != nil {
		t.Fatalf("precondition: survivor missing: %v", err)
	}
	if _, err := WriteAssets(dir); err != nil {
		t.Fatalf("second WriteAssets: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file not swept (err=%v)", err)
	}
	if _, err := os.Stat(survivor); err != nil {
		t.Errorf("generated file wrongly swept: %v", err)
	}
}

// TestPatchSettingsCreatesFileWhenMissing exercises the os.IsNotExist
// branch: no settings.json yet -> patcher should create it enabled.
func TestPatchSettingsCreatesFileWhenMissing(t *testing.T) {
	home := setHome(t)
	if err := PatchSettings(); err != nil {
		t.Fatalf("PatchSettings: %v", err)
	}
	path := filepath.Join(home, ".claude", "settings.json")
	doc := readSettings(t, path)
	enabled, ok := doc["enabledPlugins"].(map[string]any)
	if !ok {
		t.Fatalf("enabledPlugins missing/not object: %v", doc["enabledPlugins"])
	}
	if enabled[pluginEnabledKey] != true {
		t.Errorf("enabledPlugins[%q] = %v, want true", pluginEnabledKey, enabled[pluginEnabledKey])
	}
}

// TestPatchSettingsRejectsMalformedJSON verifies the unmarshal error
// path is surfaced rather than silently overwriting.
func TestPatchSettingsRejectsMalformedJSON(t *testing.T) {
	home := setHome(t)
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchSettings(); err == nil {
		t.Fatal("expected error on malformed settings.json")
	}
}

// TestUnpatchSettingsRemovesOnlyOurKey verifies UnpatchSettings deletes
// our enabledPlugins entry but preserves sibling entries.
func TestUnpatchSettingsRemovesOnlyOurKey(t *testing.T) {
	home := setHome(t)
	path := writeSettings(t, home, map[string]any{
		"theme": "dark",
		"enabledPlugins": map[string]any{
			pluginEnabledKey: true,
			"other@market":   true,
		},
	})
	if err := UnpatchSettings(); err != nil {
		t.Fatalf("UnpatchSettings: %v", err)
	}
	doc := readSettings(t, path)
	if doc["theme"] != "dark" {
		t.Errorf("unrelated key theme mutated: %v", doc["theme"])
	}
	enabled := doc["enabledPlugins"].(map[string]any)
	if _, present := enabled[pluginEnabledKey]; present {
		t.Errorf("our key not removed: %v", enabled)
	}
	if enabled["other@market"] != true {
		t.Errorf("sibling entry lost: %v", enabled)
	}
}

// TestUnpatchSettingsMissingFileIsNoOp verifies a missing settings.json
// is treated as success (nothing to undo).
func TestUnpatchSettingsMissingFileIsNoOp(t *testing.T) {
	setHome(t)
	if err := UnpatchSettings(); err != nil {
		t.Errorf("UnpatchSettings on missing file should be nil, got %v", err)
	}
}

// TestUnpatchMcpServersRemovesEntry verifies the legacy mcpServers.scout
// entry is deleted while siblings survive.
func TestUnpatchMcpServersRemovesEntry(t *testing.T) {
	home := setHome(t)
	path := writeSettings(t, home, map[string]any{
		"mcpServers": map[string]any{
			Name:    map[string]any{"command": "scout"},
			"other": map[string]any{"command": "thing"},
		},
	})
	if err := UnpatchMcpServers(); err != nil {
		t.Fatalf("UnpatchMcpServers: %v", err)
	}
	doc := readSettings(t, path)
	servers := doc["mcpServers"].(map[string]any)
	if _, present := servers[Name]; present {
		t.Errorf("scout mcpServers entry not removed: %v", servers)
	}
	if _, present := servers["other"]; !present {
		t.Errorf("sibling mcpServers entry lost: %v", servers)
	}
}

// TestUnpatchMcpServersMissingFileIsNoOp covers the not-exist branch.
func TestUnpatchMcpServersMissingFileIsNoOp(t *testing.T) {
	setHome(t)
	if err := UnpatchMcpServers(); err != nil {
		t.Errorf("expected nil for missing file, got %v", err)
	}
}

// TestSettingsHasEnabled covers all three states: enabled true, enabled
// false, and absent file.
func TestSettingsHasEnabled(t *testing.T) {
	tests := []struct {
		name  string
		setup func(home string, t *testing.T)
		want  bool
	}{
		{
			name:  "absent file",
			setup: func(string, *testing.T) {},
			want:  false,
		},
		{
			name: "enabled true",
			setup: func(home string, t *testing.T) {
				writeSettings(t, home, map[string]any{
					"enabledPlugins": map[string]any{pluginEnabledKey: true},
				})
			},
			want: true,
		},
		{
			name: "enabled false",
			setup: func(home string, t *testing.T) {
				writeSettings(t, home, map[string]any{
					"enabledPlugins": map[string]any{pluginEnabledKey: false},
				})
			},
			want: false,
		},
		{
			name: "key absent",
			setup: func(home string, t *testing.T) {
				writeSettings(t, home, map[string]any{
					"enabledPlugins": map[string]any{"other@m": true},
				})
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := setHome(t)
			tt.setup(home, t)
			if got := SettingsHasEnabled(); got != tt.want {
				t.Errorf("SettingsHasEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMarketplaceHasEntry covers absent, present, and present-but-other.
func TestMarketplaceHasEntry(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		setHome(t)
		if MarketplaceHasEntry() {
			t.Error("expected false when no marketplace.json")
		}
	})
	t.Run("present after patch", func(t *testing.T) {
		setHome(t)
		if err := PatchMarketplace(); err != nil {
			t.Fatalf("PatchMarketplace: %v", err)
		}
		if !MarketplaceHasEntry() {
			t.Error("expected true after PatchMarketplace")
		}
	})
	t.Run("only foreign entries", func(t *testing.T) {
		home := setHome(t)
		path := filepath.Join(home, ".claude", "plugins", "marketplaces", pluginMarketplace, ".claude-plugin", "marketplace.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		doc := map[string]any{"plugins": []any{map[string]any{"name": "not-scout"}}}
		b, _ := json.Marshal(doc)
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		if MarketplaceHasEntry() {
			t.Error("expected false when only foreign plugin entries present")
		}
	})
}

// TestMcpServersHasScoutFromPluginMcp verifies the primary lookup path:
// the plugin-shipped .mcp.json spec form.
func TestMcpServersHasScoutFromPluginMcp(t *testing.T) {
	home := setHome(t)
	pluginMcp := filepath.Join(home, ".claude", "plugins", "marketplaces", Name, ".mcp.json")
	if err := os.MkdirAll(filepath.Dir(pluginMcp), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"mcpServers": map[string]any{
			Name: map[string]any{"command": "scout-from-plugin"},
		},
	}
	b, _ := json.Marshal(doc)
	if err := os.WriteFile(pluginMcp, b, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, ok := McpServersHasScout()
	if !ok {
		t.Fatal("expected ok=true from plugin .mcp.json")
	}
	if cmd != "scout-from-plugin" {
		t.Errorf("cmd = %q, want %q", cmd, "scout-from-plugin")
	}
}

// TestMcpServersHasScoutFallbackToSettings verifies the legacy
// settings.json fallback when no plugin .mcp.json exists.
func TestMcpServersHasScoutFallbackToSettings(t *testing.T) {
	home := setHome(t)
	writeSettings(t, home, map[string]any{
		"mcpServers": map[string]any{
			Name: map[string]any{"command": "scout-legacy"},
		},
	})
	cmd, ok := McpServersHasScout()
	if !ok {
		t.Fatal("expected ok=true from settings.json fallback")
	}
	if cmd != "scout-legacy" {
		t.Errorf("cmd = %q, want %q", cmd, "scout-legacy")
	}
}

// TestMcpServersHasScoutNotRegistered verifies the negative case when
// neither source declares scout.
func TestMcpServersHasScoutNotRegistered(t *testing.T) {
	setHome(t)
	if cmd, ok := McpServersHasScout(); ok {
		t.Errorf("expected ok=false, got ok=true cmd=%q", cmd)
	}
}

// TestPathExists covers both branches.
func TestPathExists(t *testing.T) {
	dir := t.TempDir()
	if !pathExists(dir) {
		t.Error("pathExists(existing dir) = false")
	}
	if pathExists(filepath.Join(dir, "nope")) {
		t.Error("pathExists(missing) = true")
	}
}

// TestAtomicWriteJSONRoundtrip verifies the helper creates parent dirs,
// emits indented JSON with a trailing newline, and the result parses
// back to the source document.
func TestAtomicWriteJSONRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "out.json")
	src := map[string]any{"a": "b", "n": float64(3)}
	if err := atomicWriteJSON(path, src); err != nil {
		t.Fatalf("atomicWriteJSON: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Error("expected trailing newline")
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if back["a"] != "b" || back["n"] != float64(3) {
		t.Errorf("roundtrip mismatch: %v", back)
	}
	// no leftover tmp file
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file not renamed away (err=%v)", err)
	}
}

// TestUninstallRemovesTargetAndPatches drives the full uninstall path:
// it must delete the target tree and remove our settings entries.
func TestUninstallRemovesTargetAndPatches(t *testing.T) {
	home := setHome(t)
	target := filepath.Join(home, ".claude", "plugins", "marketplaces", Name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "marker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := writeSettings(t, home, map[string]any{
		"enabledPlugins": map[string]any{pluginEnabledKey: true},
		"mcpServers":     map[string]any{Name: map[string]any{"command": "scout"}},
	})
	if err := Uninstall(target); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target not removed (err=%v)", err)
	}
	doc := readSettings(t, settingsPath)
	enabled, _ := doc["enabledPlugins"].(map[string]any)
	if _, present := enabled[pluginEnabledKey]; present {
		t.Errorf("enabledPlugins entry not removed: %v", enabled)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if _, present := servers[Name]; present {
		t.Errorf("mcpServers entry not removed: %v", servers)
	}
}

// TestPrintStatusWritesReport verifies PrintStatus emits the labelled
// fields reflecting current install state into the provided file.
func TestPrintStatusWritesReport(t *testing.T) {
	home := setHome(t)
	if err := PatchMarketplace(); err != nil {
		t.Fatalf("PatchMarketplace: %v", err)
	}
	if err := PatchSettings(); err != nil {
		t.Fatalf("PatchSettings: %v", err)
	}
	out := filepath.Join(home, "status.txt")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrintStatus(f); err != nil {
		_ = f.Close()
		t.Fatalf("PrintStatus: %v", err)
	}
	_ = f.Close()
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"embedded plugin",
		Name,
		Version,
		"install target",
		"marketplace entry  : true",
		"settings enabled   : true",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("status report missing %q\n---\n%s", want, s)
		}
	}
}
