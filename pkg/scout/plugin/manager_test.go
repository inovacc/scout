package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Discover(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin directory with manifest.
	pluginDir := filepath.Join(dir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"description": "A test plugin",
		"command": "./test-plugin",
		"capabilities": ["scraper_mode"],
		"modes": [{"name": "test-mode", "description": "Test"}]
	}`

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager([]string{dir}, nil)
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	plugins := mgr.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(plugins))
	}

	if plugins[0].Name != "test-plugin" {
		t.Errorf("name = %q, want %q", plugins[0].Name, "test-plugin")
	}
}

func TestManager_Discover_EmptyDir(t *testing.T) {
	mgr := NewManager([]string{t.TempDir()}, nil)
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	if len(mgr.Plugins()) != 0 {
		t.Error("expected no plugins")
	}
}

func TestManager_Discover_NonexistentDir(t *testing.T) {
	mgr := NewManager([]string{"/nonexistent/path"}, nil)
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	if len(mgr.Plugins()) != 0 {
		t.Error("expected no plugins")
	}
}

func TestManager_GetMode(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.manifests["test"] = &Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Command:      "./test",
		Capabilities: []string{"scraper_mode"},
		Modes:        []ModeEntry{{Name: "test-mode", Description: "Test"}},
	}

	mode, ok := mgr.GetMode("test-mode")
	if !ok {
		t.Fatal("expected to find mode")
	}

	if mode.Name() != "test-mode" {
		t.Errorf("mode.Name() = %q, want %q", mode.Name(), "test-mode")
	}

	_, ok = mgr.GetMode("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent mode")
	}
}

func TestManager_GetExtractor(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.manifests["test"] = &Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Command:      "./test",
		Capabilities: []string{"extractor"},
		Extractors:   []ExtractorEntry{{Name: "test-ext", Description: "Test"}},
	}

	ext, ok := mgr.GetExtractor("test-ext")
	if !ok {
		t.Fatal("expected to find extractor")
	}

	if ext.Name() != "test-ext" {
		t.Errorf("ext.Name() = %q, want %q", ext.Name(), "test-ext")
	}

	_, ok = mgr.GetExtractor("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent extractor")
	}
}

func TestManager_ListModes(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.manifests["p1"] = &Manifest{
		Modes: []ModeEntry{{Name: "a"}, {Name: "b"}},
	}
	mgr.manifests["p2"] = &Manifest{
		Modes: []ModeEntry{{Name: "c"}},
	}

	modes := mgr.ListModes()
	if len(modes) != 3 {
		t.Errorf("got %d modes, want 3", len(modes))
	}
}

func TestManager_ListExtractors(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.manifests["p1"] = &Manifest{
		Extractors: []ExtractorEntry{{Name: "x"}},
	}

	extractors := mgr.ListExtractors()
	if len(extractors) != 1 {
		t.Errorf("got %d extractors, want 1", len(extractors))
	}
}

func TestManager_Close_Empty(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.Close() // should not panic
}
