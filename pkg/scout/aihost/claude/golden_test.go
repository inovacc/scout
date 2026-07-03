package claude

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGolden rewrites the committed golden tree from the current rendered output.
// Run: go test ./pkg/scout/aihost/claude/ -run TestRenderedTreeGolden -update-golden
var updateGolden = flag.Bool("update-golden", false, "rewrite golden tree fixtures")

// TestRenderedTreeGolden pins the ENTIRE rendered Claude plugin tree (every command,
// agent, skill, plus the synthesised manifests) byte-for-byte against committed
// fixtures under testdata/golden/. It is the drift guard for authoring refactors: any
// change to a rendered body — intended or accidental — fails here until the golden is
// deliberately regenerated with -update-golden.
func TestRenderedTreeGolden(t *testing.T) {
	got := renderFullTree(t)

	dir := filepath.Join("testdata", "golden")

	if *updateGolden {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("clear golden: %v", err)
		}
		for p, d := range got {
			fp := filepath.Join(dir, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fp, d, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("wrote %d golden files to %s", len(got), dir)
		return
	}

	// Every rendered file must match its committed golden.
	for p, d := range got {
		want, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil {
			t.Errorf("no golden for %q — run: go test ./pkg/scout/aihost/claude/ -run TestRenderedTreeGolden -update-golden", p)
			continue
		}
		if string(want) != string(d) {
			t.Errorf("golden drift in %q — rendered output changed; if intended, regenerate with -update-golden", p)
		}
	}

	// Any golden file with no corresponding rendered asset is stale (asset removed).
	_ = filepath.WalkDir(dir, func(path string, dentry os.DirEntry, err error) error {
		if err != nil || dentry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if _, ok := got[filepath.ToSlash(rel)]; !ok {
			t.Errorf("stale golden %q — no rendered asset produces it; regenerate with -update-golden", filepath.ToSlash(rel))
		}
		return nil
	})
}

// renderFullTree returns every tree-relative path -> rendered bytes: Walk'd markdown
// assets plus the synthesised manifest files.
func renderFullTree(t *testing.T) map[string][]byte {
	t.Helper()

	out := map[string][]byte{}
	if err := Walk(func(p string, data []byte) error {
		out[p] = append([]byte(nil), data...)
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}

	gen, err := GeneratedFiles()
	if err != nil {
		t.Fatalf("generated files: %v", err)
	}
	for p, d := range gen {
		out[p] = d
	}

	return out
}
