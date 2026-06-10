package claude

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

// TestKindForPath checks the prefix-based asset classification covers
// every recognised top-level dir plus the unknown fallback.
func TestKindForPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want aihost.Kind
	}{
		{"agent", "agents/web-scraper.md", aihost.KindAgent},
		{"command", "commands/scout-scrape.md", aihost.KindCommand},
		{"skill", "skills/scrape/SKILL.md", aihost.KindSkill},
		{"manifest is unknown", ".claude-plugin/plugin.json", aihost.KindUnknown},
		{"bare file is unknown", "README.md", aihost.KindUnknown},
		{"empty is unknown", "", aihost.KindUnknown},
		{"substring not prefix", "x/agents/foo.md", aihost.KindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kindForPath(tt.path); got != tt.want {
				t.Errorf("kindForPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestTemplateDataMirrorsConsts verifies the render substitution map is
// populated from the package consts (these flow into <%.Name%> etc.).
func TestTemplateDataMirrorsConsts(t *testing.T) {
	d := templateData()
	if d.Name != Name {
		t.Errorf("Name = %q, want %q", d.Name, Name)
	}
	if d.Version != Version {
		t.Errorf("Version = %q, want %q", d.Version, Version)
	}
	if d.Description != Description {
		t.Errorf("Description = %q, want %q", d.Description, Description)
	}
	if d.McpCommand != McpCommand {
		t.Errorf("McpCommand = %q, want %q", d.McpCommand, McpCommand)
	}
}

// TestInitClassifiesAssets verifies init() assigned a non-unknown Kind
// to every registered asset based on its path, and that the kind agrees
// with kindForPath. A misclassified asset would silently drop out of
// Count/CommandNames.
func TestInitClassifiesAssets(t *testing.T) {
	if len(generatedAssets) == 0 {
		t.Fatal("no generated assets registered")
	}
	for _, a := range generatedAssets {
		if a.Kind == aihost.KindUnknown {
			t.Errorf("asset %q left unclassified (KindUnknown)", a.Path)
		}
		if want := kindForPath(a.Path); a.Kind != want {
			t.Errorf("asset %q Kind = %v, want %v", a.Path, a.Kind, want)
		}
	}
}

// TestWalkRendersEveryAsset confirms Walk visits each asset exactly once
// with non-empty rendered bytes, and that template placeholders were
// substituted (no leftover <%...%> delimiters).
func TestWalkRendersEveryAsset(t *testing.T) {
	visited := map[string]int{}
	err := Walk(func(p string, data []byte) error {
		visited[p]++
		if len(data) == 0 {
			t.Errorf("asset %q rendered empty", p)
		}
		if strings.Contains(string(data), "<%") || strings.Contains(string(data), "%>") {
			t.Errorf("asset %q has unrendered template delimiters", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(visited) != len(generatedAssets) {
		t.Errorf("Walk visited %d distinct paths, want %d", len(visited), len(generatedAssets))
	}
	for p, n := range visited {
		if n != 1 {
			t.Errorf("asset %q visited %d times, want 1", p, n)
		}
	}
}

// TestWalkPropagatesCallbackError verifies a callback error aborts Walk
// and is returned to the caller.
func TestWalkPropagatesCallbackError(t *testing.T) {
	sentinel := errStop
	calls := 0
	err := Walk(func(string, []byte) error {
		calls++
		return sentinel
	})
	if err == nil {
		t.Fatal("expected Walk to surface callback error")
	}
	if calls != 1 {
		t.Errorf("expected Walk to stop after first callback, got %d calls", calls)
	}
}

// TestCount verifies per-dir grouping and total match the registered
// asset set, and that the totals are internally consistent.
func TestCount(t *testing.T) {
	counts, total, err := Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != len(generatedAssets) {
		t.Errorf("total = %d, want %d", total, len(generatedAssets))
	}
	sum := 0
	for dir, n := range counts {
		if n <= 0 {
			t.Errorf("dir %q has non-positive count %d", dir, n)
		}
		sum += n
	}
	if sum != total {
		t.Errorf("sum of per-dir counts = %d, want total %d", sum, total)
	}
	for _, dir := range []string{"agents", "commands", "skills"} {
		if counts[dir] == 0 {
			t.Errorf("expected at least one asset under %q", dir)
		}
	}
}

// TestCommandNames verifies the slash-command stems are sorted, stripped
// of the commands/ prefix and .md suffix, and only include flat command
// files (nested paths are skipped).
func TestCommandNames(t *testing.T) {
	names, err := CommandNames()
	if err != nil {
		t.Fatalf("CommandNames: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected at least one command name")
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("command names not sorted: %v", names)
	}
	for _, n := range names {
		if n == "" {
			t.Error("empty command name returned")
		}
		if strings.HasPrefix(n, "commands/") || strings.HasSuffix(n, ".md") {
			t.Errorf("name %q not stripped of prefix/suffix", n)
		}
		if strings.ContainsRune(n, '/') {
			t.Errorf("nested command path leaked into names: %q", n)
		}
	}
	// Every returned stem must correspond to a real command asset.
	want := map[string]bool{}
	for _, a := range generatedAssets {
		if a.Kind == aihost.KindCommand {
			stem := strings.TrimSuffix(strings.TrimPrefix(a.Path, "commands/"), ".md")
			if stem != "" && !strings.ContainsRune(stem, '/') {
				want[stem] = true
			}
		}
	}
	if len(names) != len(want) {
		t.Errorf("CommandNames count = %d, want %d", len(names), len(want))
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected command name %q", n)
		}
	}
}

// TestHostName / TestHostInstallTarget pin the static Host metadata and
// the home-relative install path layout.
func TestHostName(t *testing.T) {
	if got := (Host{}).Name(); got != "claude" {
		t.Errorf("Host.Name() = %q, want %q", got, "claude")
	}
}

func TestHostInstallTarget(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	got, err := (Host{}).InstallTarget()
	if err != nil {
		t.Fatalf("InstallTarget: %v", err)
	}
	want := filepath.Join(tmp, ".claude", "plugins", "marketplaces", Name)
	if got != want {
		t.Errorf("InstallTarget() = %q, want %q", got, want)
	}
}

// TestHostManifestFilesDelegates verifies the Host.ManifestFiles adapter
// returns the same keys as GeneratedFiles().
func TestHostManifestFilesDelegates(t *testing.T) {
	hf, err := (Host{}).ManifestFiles()
	if err != nil {
		t.Fatalf("Host.ManifestFiles: %v", err)
	}
	gen, err := GeneratedFiles()
	if err != nil {
		t.Fatalf("GeneratedFiles: %v", err)
	}
	if len(hf) != len(gen) {
		t.Fatalf("Host.ManifestFiles len = %d, want %d", len(hf), len(gen))
	}
	for k, v := range gen {
		if string(hf[k]) != string(v) {
			t.Errorf("Host.ManifestFiles[%q] diverges from GeneratedFiles", k)
		}
	}
}
