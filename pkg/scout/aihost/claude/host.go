// Package claude implements the scout plugin for Anthropic's Claude
// Code host. Asset bodies live in assets.go as aihost.Asset literals;
// each registers itself into the shared aihost registry on init().
// Manifest files (.claude-plugin/plugin.json + .mcp.json) come from
// manifest.go via synthesised consts. Satisfies aihost.Host.
package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

func init() {
	aihost.Register(func() aihost.Host { return Host{} })
	for i, a := range generatedAssets {
		generatedAssets[i].Kind = kindForPath(a.Path)
	}
	aihost.RegisterAsset(generatedAssets...)
}

func kindForPath(p string) aihost.Kind {
	switch {
	case strings.HasPrefix(p, "agents/"):
		return aihost.KindAgent
	case strings.HasPrefix(p, "commands/"):
		return aihost.KindCommand
	case strings.HasPrefix(p, "skills/"):
		return aihost.KindSkill
	default:
		return aihost.KindUnknown
	}
}

// templateData returns the substitution map every Claude asset is
// rendered with at install time.
func templateData() aihost.TemplateData {
	return aihost.TemplateData{
		Name:        Name,
		Version:     Version,
		Description: Description,
		McpCommand:  McpCommand,
	}
}

// Host implements aihost.Host for Claude Code.
type Host struct{}

func (Host) Name() string { return "claude" }

func (Host) InstallTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".claude", "plugins", "marketplaces", Name), nil
}

func (Host) Walk(fn func(path string, data []byte) error) error { return Walk(fn) }

func (Host) ManifestFiles() (map[string][]byte, error) { return GeneratedFiles() }

func (Host) Install(target string) (int, error) { return Install(target) }

func (Host) Uninstall(target string) error { return Uninstall(target) }

func (Host) PrintStatus(w *os.File) error { return PrintStatus(w) }

// Walk visits every Claude-origin asset's rendered bytes.
func Walk(fn func(path string, data []byte) error) error {
	d := templateData()
	for _, a := range generatedAssets {
		data, err := a.Render(d)
		if err != nil {
			return err
		}
		if err := fn(a.Path, data); err != nil {
			return err
		}
	}
	return nil
}

// Count returns asset counts grouped by top-level dir.
func Count() (map[string]int, int, error) {
	counts := map[string]int{}
	total := 0
	err := Walk(func(p string, _ []byte) error {
		parts := strings.SplitN(p, "/", 2)
		counts[parts[0]]++
		total++
		return nil
	})
	return counts, total, err
}

// CommandNames returns sorted list of slash-command stems.
func CommandNames() ([]string, error) {
	var names []string
	for _, a := range generatedAssets {
		if a.Kind != aihost.KindCommand {
			continue
		}
		stem := strings.TrimSuffix(strings.TrimPrefix(a.Path, "commands/"), ".md")
		if stem == "" || strings.ContainsRune(stem, '/') {
			continue
		}
		names = append(names, stem)
	}
	sort.Strings(names)
	return names, nil
}
