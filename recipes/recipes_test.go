package recipes

import (
	"io/fs"
	"testing"
)

func TestAllReturnsFullCatalog(t *testing.T) {
	all := All()
	if len(all) != len(catalog) {
		t.Fatalf("All() returned %d presets, want %d", len(all), len(catalog))
	}
}

func TestAllPresetsHaveRequiredFields(t *testing.T) {
	for _, p := range All() {
		if p.ID == "" {
			t.Error("preset has empty ID")
		}
		if p.Service == "" {
			t.Errorf("preset %q has empty Service", p.ID)
		}
		if p.Description == "" {
			t.Errorf("preset %q has empty Description", p.ID)
		}
		if p.File == "" {
			t.Errorf("preset %q has empty File", p.ID)
		}
	}
}

func TestCatalogIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range All() {
		if seen[p.ID] {
			t.Errorf("duplicate preset ID %q", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestIndexMatchesCatalog(t *testing.T) {
	if len(index) != len(catalog) {
		t.Fatalf("index has %d entries, catalog has %d", len(index), len(catalog))
	}
	for _, p := range catalog {
		got, ok := index[p.ID]
		if !ok {
			t.Errorf("catalog entry %q missing from index", p.ID)
			continue
		}
		if got != p {
			t.Errorf("index[%q] does not match catalog entry", p.ID)
		}
	}
}

func TestLoadAllPresets(t *testing.T) {
	for _, p := range All() {
		t.Run(p.ID, func(t *testing.T) {
			r, err := Load(p.ID)
			if err != nil {
				t.Fatalf("Load(%q): %v", p.ID, err)
			}
			if r.Version != "1" {
				t.Errorf("version = %q, want %q", r.Version, "1")
			}
			if r.Name == "" {
				t.Error("recipe has empty name")
			}
			if r.Type != "extract" && r.Type != "automate" {
				t.Errorf("type = %q, want extract or automate", r.Type)
			}
			if r.Type == "extract" {
				if r.URL == "" {
					t.Error("extract recipe has empty URL")
				}
				if r.Items == nil {
					t.Error("extract recipe has nil items")
				}
			}
			if r.Type == "automate" && len(r.Steps) == 0 {
				t.Error("automate recipe has no steps")
			}
		})
	}
}

func TestLoadUnknownPreset(t *testing.T) {
	_, err := Load("nonexistent")
	if err == nil {
		t.Fatal("Load(nonexistent) should return error")
	}
}

func TestFSContainsPresetFiles(t *testing.T) {
	fsys := FS()
	for _, p := range All() {
		_, err := fs.Stat(fsys, p.File)
		if err != nil {
			t.Errorf("FS missing %s: %v", p.File, err)
		}
	}
}
