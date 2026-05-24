package claude

import (
	"encoding/json"
	"fmt"
)

// Single source of truth for plugin manifest metadata. Changing these
// values changes both the generated .claude-plugin/plugin.json and the
// PluginManifest returned by Manifest().
const (
	Name        = "scout"
	Version     = "1.0.0"
	Description = "Browser automation, web scraping, and site testing via Scout's headless Chrome engine."
	Author      = "inovacc"
	Repository  = "https://github.com/inovacc/scout"
	License     = "BSD-3-Clause"

	// MCP server registration (.mcp.json). Command is the binary on PATH;
	// args are passed verbatim. Keep in sync with cmd/scout/server.go.
	McpServerName = "scout"
	McpCommand    = "scout"
)

var (
	McpArgs  = []string{"mcp", "--headless", "--stealth"}
	Keywords = []string{"browser", "automation", "scraping", "testing", "chrome", "mcp", "headless"}
)

// PluginAuthor matches the object shape Claude Code's plugin loader
// requires: {"name": "..."} (string form fails manifest validation).
type PluginAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// PluginManifest mirrors .claude-plugin/plugin.json shape.
type PluginManifest struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Author      PluginAuthor `json:"author,omitempty"`
	Repository  string       `json:"repository,omitempty"`
	License     string       `json:"license,omitempty"`
	Keywords    []string     `json:"keywords,omitempty"`
}

// Manifest returns plugin manifest synthesised from package consts.
func Manifest() (PluginManifest, error) {
	m := PluginManifest{
		Name:        Name,
		Version:     Version,
		Description: Description,
		Author:      PluginAuthor{Name: Author, URL: Repository},
		Repository:  Repository,
		License:     License,
		Keywords:    Keywords,
	}
	if m.Name == "" || m.Version == "" {
		return PluginManifest{}, fmt.Errorf("plugin: const Name/Version empty")
	}
	return m, nil
}

// PluginJSON returns the bytes written to .claude-plugin/plugin.json.
func PluginJSON() ([]byte, error) {
	m, err := Manifest()
	if err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal plugin.json: %w", err)
	}
	return append(b, '\n'), nil
}

// McpJSON returns the bytes written to plugin's .mcp.json (spec form:
// top-level "mcpServers" map).
func McpJSON() ([]byte, error) {
	doc := map[string]any{
		"mcpServers": map[string]any{
			McpServerName: map[string]any{
				"command": McpCommand,
				"args":    McpArgs,
			},
		},
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal .mcp.json: %w", err)
	}
	return append(b, '\n'), nil
}

// GeneratedFiles returns map of path -> contents for files synthesised
// from Go consts (not embedded). WriteAssets() writes these alongside
// rendered markdown.
func GeneratedFiles() (map[string][]byte, error) {
	pj, err := PluginJSON()
	if err != nil {
		return nil, err
	}
	mj, err := McpJSON()
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		".claude-plugin/plugin.json": pj,
		".mcp.json":                  mj,
	}, nil
}
