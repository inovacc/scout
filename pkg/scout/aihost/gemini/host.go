// Package gemini implements the scout plugin for Google's Gemini CLI.
// Stub: shares the same asset registry as claude. Walk + ManifestFiles
// only. Wire Installer/Doctor when Gemini CLI plugin surface stabilises.
package gemini

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

type Host struct{}

func (Host) Name() string { return "gemini" }

func (Host) InstallTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".gemini", "plugins", claude.Name), nil
}

func (Host) Walk(fn func(path string, data []byte) error) error {
	return claude.Walk(fn)
}

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
		"gemini-extension.json": pj,
		".mcp.json":             mj,
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
		return nil, fmt.Errorf("marshal gemini-extension.json: %w", err)
	}
	return append(b, '\n'), nil
}
