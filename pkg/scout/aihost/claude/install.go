// Claude-specific install ritual. Lives here instead of cmd/ so codex
// + gemini don't have to know about marketplaces, settings.json
// patching, or `claude` CLI shell-outs. The cmd dispatcher calls into
// these exported functions per host.

package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

const (
	pluginMarketplace = "scout"
	pluginEnabledKey  = "scout@scout"
	pluginSourcePath  = "./"
	pluginCategory    = "developer-tools"
)

// WriteAssets walks every Claude asset (rendered) and synthesised
// manifest, writing to target with atomic tmp+rename. Sweeps stale
// files from canonical content dirs that are no longer in the
// generated set. Returns count written.
func WriteAssets(target string) (int, error) {
	if target == "" {
		return 0, fmt.Errorf("target must not be empty")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir target: %w", err)
	}
	wanted := map[string]struct{}{}
	count := 0
	writeOne := func(rel string, data []byte) error {
		dst := filepath.Join(target, filepath.FromSlash(rel))
		wanted[filepath.Clean(dst)] = struct{}{}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", tmp, err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			return fmt.Errorf("rename %s -> %s: %w", tmp, dst, err)
		}
		count++
		return nil
	}
	if err := Walk(writeOne); err != nil {
		return count, err
	}
	gen, err := GeneratedFiles()
	if err != nil {
		return count, fmt.Errorf("generate manifest files: %w", err)
	}
	for rel, data := range gen {
		if err := writeOne(rel, data); err != nil {
			return count, err
		}
	}
	for _, sub := range []string{"commands", "agents", "skills"} {
		subPath := filepath.Join(target, sub)
		_ = filepath.WalkDir(subPath, func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			if _, ok := wanted[filepath.Clean(p)]; !ok {
				if rmErr := os.Remove(p); rmErr == nil {
					fmt.Fprintf(os.Stderr, "[install] swept stale: %s\n", p)
				}
			}
			return nil
		})
	}
	return count, nil
}

// PatchMarketplace writes the owned marketplace.json declaring a
// single-plugin marketplace pointing at "./".
func PatchMarketplace() error {
	path, err := marketplaceJSONPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	doc := map[string]any{
		"name":        pluginMarketplace,
		"description": "scout — browser automation plugin (private marketplace).",
		"owner":       map[string]any{"name": "scout"},
		"plugins": []any{
			map[string]any{
				"name":        Name,
				"description": Description,
				"version":     Version,
				"source":      pluginSourcePath,
				"category":    pluginCategory,
			},
		},
	}
	return atomicWriteJSON(path, doc)
}

// PatchSettings flips ~/.claude/settings.json enabledPlugins[scout@scout]=true.
func PatchSettings() error {
	path, err := settingsJSONPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = []byte("{}")
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse settings.json: %w", err)
	}
	enabled, _ := doc["enabledPlugins"].(map[string]any)
	if enabled == nil {
		enabled = map[string]any{}
	}
	enabled[pluginEnabledKey] = true
	doc["enabledPlugins"] = enabled
	return atomicWriteJSON(path, doc)
}

// UnpatchSettings removes the enabledPlugins entry.
func UnpatchSettings() error {
	path, err := settingsJSONPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if enabled, ok := doc["enabledPlugins"].(map[string]any); ok {
		delete(enabled, pluginEnabledKey)
		doc["enabledPlugins"] = enabled
	}
	return atomicWriteJSON(path, doc)
}

// UnpatchMcpServers removes any legacy settings.json mcpServers entry
// (canonical registration now lives in plugin .mcp.json spec form).
func UnpatchMcpServers() error {
	path, err := settingsJSONPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if servers, ok := doc["mcpServers"].(map[string]any); ok {
		delete(servers, Name)
		doc["mcpServers"] = servers
	}
	return atomicWriteJSON(path, doc)
}

// RegisterLocalMarketplace shells out `claude plugin marketplace add
// <target>` so CC indexes our directory-source marketplace.
func RegisterLocalMarketplace(target string) {
	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[install] claude CLI not on PATH — run manually:\n  claude plugin marketplace add %q\n", target)
		return
	}
	out, err := exec.Command(claudeBin, "plugin", "marketplace", "add", target).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[install] marketplace add said: %s\n", string(out))
		return
	}
	fmt.Fprintln(os.Stderr, "[install] registered marketplace via claude plugin marketplace add")
}

// McpServersHasScout checks the plugin-shipped .mcp.json (spec form)
// for a scout entry. Falls back to legacy settings.json mcpServers.
func McpServersHasScout() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	pluginMcp := filepath.Join(home, ".claude", "plugins", "marketplaces", Name, ".mcp.json")
	if raw, err := os.ReadFile(pluginMcp); err == nil {
		doc := map[string]any{}
		if json.Unmarshal(raw, &doc) == nil {
			if servers, ok := doc["mcpServers"].(map[string]any); ok {
				if entry, ok := servers[Name].(map[string]any); ok {
					cmd, _ := entry["command"].(string)
					return cmd, true
				}
			}
		}
	}
	settingsPath, err := settingsJSONPath()
	if err != nil {
		return "", false
	}
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", false
	}
	doc := map[string]any{}
	if json.Unmarshal(raw, &doc) != nil {
		return "", false
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	entry, ok := servers[Name].(map[string]any)
	if !ok {
		return "", false
	}
	cmd, _ := entry["command"].(string)
	return cmd, true
}

// MarketplaceHasEntry reports whether marketplace.json declares scout.
func MarketplaceHasEntry() bool {
	path, err := marketplaceJSONPath()
	if err != nil {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	doc := map[string]any{}
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	plugins, _ := doc["plugins"].([]any)
	for _, p := range plugins {
		if entry, ok := p.(map[string]any); ok && entry["name"] == Name {
			return true
		}
	}
	return false
}

// SettingsHasEnabled reports whether enabledPlugins flips us on.
func SettingsHasEnabled() bool {
	path, err := settingsJSONPath()
	if err != nil {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	doc := map[string]any{}
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	enabled, _ := doc["enabledPlugins"].(map[string]any)
	v, ok := enabled[pluginEnabledKey].(bool)
	return ok && v
}

// Install runs the full claude-side install ritual: write assets,
// patch marketplace, patch settings, clean legacy mcpServers entry,
// register the marketplace via the claude CLI.
func Install(target string) (int, error) {
	n, err := WriteAssets(target)
	if err != nil {
		return n, fmt.Errorf("write assets: %w", err)
	}
	if err := PatchMarketplace(); err != nil {
		return n, fmt.Errorf("patch marketplace.json: %w", err)
	}
	if err := PatchSettings(); err != nil {
		return n, fmt.Errorf("patch settings.json: %w", err)
	}
	if err := UnpatchMcpServers(); err == nil {
		fmt.Fprintln(os.Stderr, "[install] cleaned: settings.json mcpServers.scout (now owned by plugin .mcp.json)")
	}
	RegisterLocalMarketplace(target)
	return n, nil
}

// Uninstall removes the install target and undoes the JSON patches.
func Uninstall(target string) error {
	if err := UnpatchSettings(); err != nil {
		fmt.Fprintf(os.Stderr, "[uninstall] settings.json: %v (continuing)\n", err)
	}
	if err := UnpatchMcpServers(); err != nil {
		fmt.Fprintf(os.Stderr, "[uninstall] mcpServers: %v (continuing)\n", err)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove %s: %w", target, err)
	}
	fmt.Fprintf(os.Stderr, "[uninstall] removed %s\n", target)
	return nil
}

// PrintStatus writes the install-state report.
func PrintStatus(w *os.File) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	target := filepath.Join(home, ".claude", "plugins", "marketplaces", Name)
	mcpCmd, mcpOk := McpServersHasScout()
	fmt.Fprintf(w, "embedded plugin    : %s v%s\n", Name, Version)
	fmt.Fprintf(w, "install target     : %s\n", target)
	fmt.Fprintf(w, "target exists      : %v\n", pathExists(target))
	fmt.Fprintf(w, "marketplace entry  : %v\n", MarketplaceHasEntry())
	fmt.Fprintf(w, "settings enabled   : %v\n", SettingsHasEnabled())
	fmt.Fprintf(w, "mcp registered     : %v\n", mcpOk)
	if mcpOk {
		fmt.Fprintf(w, "mcp command        : %s\n", mcpCmd)
	}
	return nil
}

func marketplaceJSONPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "plugins", "marketplaces", pluginMarketplace, ".claude-plugin", "marketplace.json"), nil
}

func settingsJSONPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func atomicWriteJSON(path string, doc any) error {
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

var _ = aihost.TemplateData{}
