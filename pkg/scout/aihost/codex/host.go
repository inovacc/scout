// Package codex implements the scout plugin for OpenAI's Codex CLI.
// Stub: shares the same asset registry as claude but does not yet
// implement Installer/Status. Walks rendered assets and synthesises
// a minimal manifest. Wire Installer/Doctor when codex CLI plugin
// surface stabilises.
package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost"
	"github.com/inovacc/scout/pkg/scout/aihost/claude"
)

func init() {
	aihost.Register(func() aihost.Host { return Host{} })
}

// Host implements aihost.Host for the Codex CLI. Asset bodies are
// shared with claude; manifest shape is codex-specific.
type Host struct{}

func (Host) Name() string { return "codex" }

func (Host) InstallTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".codex", "plugins", claude.Name), nil
}

// Walk re-uses the claude asset set. Codex consumes the same agent /
// skill / command markdown — only the manifest shape differs.
func (Host) Walk(fn func(path string, data []byte) error) error {
	return claude.Walk(fn)
}

// ManifestFiles emits codex's expected layout: a top-level
// `codex.plugin.json` plus the standard `.mcp.json` (spec form).
func (Host) ManifestFiles() (map[string][]byte, error) {
	pj, err := pluginJSON()
	if err != nil {
		return nil, err
	}
	mj, err := claude.McpJSON()
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"codex.plugin.json": pj,
		".mcp.json":         mj,
	}, nil
}

func pluginJSON() ([]byte, error) {
	doc := map[string]any{
		"name":        claude.Name,
		"version":     claude.Version,
		"description": claude.Description,
		"author":      claude.Author,
		"repository":  claude.Repository,
		"license":     claude.License,
		"keywords":    claude.Keywords,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal codex.plugin.json: %w", err)
	}
	return append(b, '\n'), nil
}
