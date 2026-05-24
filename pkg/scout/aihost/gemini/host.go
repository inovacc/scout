// Package gemini implements the scout plugin for Google's Gemini CLI.
// Gemini extensions live under ~/.gemini/extensions/<name>/ and use a
// single gemini-extension.json manifest with inline mcpServers. Install
// shells out to `gemini extensions install <local-path>` for
// registration; falls back to a manual hint when the CLI isn't on PATH.
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
	return filepath.Join(home, ".gemini", "extensions", claude.Name), nil
}

func (Host) Walk(fn func(path string, data []byte) error) error {
	return claude.Walk(fn)
}

// ManifestFiles returns the single gemini-extension.json manifest with
// inline mcpServers. Gemini does NOT use a separate .mcp.json — its
// manifest schema embeds the MCP config directly.
func (Host) ManifestFiles() (map[string][]byte, error) {
	pj, err := pluginJSON()
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"gemini-extension.json": pj,
	}, nil
}

// Install satisfies aihost.Installer.
func (Host) Install(target string) (int, error) { return Install(target) }

// Uninstall satisfies aihost.Installer.
func (Host) Uninstall(target string) error { return Uninstall(target) }

// PrintStatus satisfies aihost.Status.
func (Host) PrintStatus(w *os.File) error { return PrintStatus(w) }

// geminiManifest mirrors Gemini CLI's gemini-extension.json schema.
type geminiManifest struct {
	Name        string                       `json:"name"`
	Version     string                       `json:"version"`
	Description string                       `json:"description"`
	MCPServers  map[string]geminiMCPServer   `json:"mcpServers,omitempty"`
}

type geminiMCPServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

func pluginJSON() ([]byte, error) {
	m := geminiManifest{
		Name:        claude.Name,
		Version:     claude.Version,
		Description: claude.Description,
		MCPServers: map[string]geminiMCPServer{
			claude.McpServerName: {
				Command: claude.McpCommand,
				Args:    claude.McpArgs,
			},
		},
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal gemini-extension.json: %w", err)
	}
	return append(b, '\n'), nil
}
