// Codex-specific install ritual. Codex uses TOML config
// (~/.codex/config.toml) which we deliberately do not patch in-place —
// round-trip-preserving TOML editing would mean pulling in a TOML
// library and writing tests to pin "unrelated keys survive" the way
// claude's install.go does for settings.json.
//
// Instead this Installer writes (a) all rendered assets to the install
// target, (b) a ready-to-paste config snippet next to them, and prints
// the manual final step. This is a real install (assets land on disk,
// skills are usable) with one honest manual step. Full TOML
// round-tripping is tracked as a follow-up.

package codex

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/scout/pkg/scout/aihost"
	"github.com/inovacc/scout/pkg/scout/aihost/claude"
)

const (
	configSnippetName = "codex-config-snippet.toml"
)

// Install satisfies aihost.Installer for Codex.
func Install(target string) (int, error) {
	if target == "" {
		return 0, fmt.Errorf("target must not be empty")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir target: %w", err)
	}
	count := 0
	writeOne := func(rel string, data []byte) error {
		dst := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		count++
		return nil
	}
	h := Host{}
	if err := h.Walk(writeOne); err != nil {
		return count, err
	}
	gen, err := h.ManifestFiles()
	if err != nil {
		return count, err
	}
	for rel, data := range gen {
		if err := writeOne(rel, data); err != nil {
			return count, err
		}
	}
	snippet, err := configSnippet(target)
	if err != nil {
		return count, err
	}
	if err := writeOne(configSnippetName, snippet); err != nil {
		return count, err
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "[install] codex: manual step required —")
	fmt.Fprintf(os.Stderr, "  cat %s >> ~/.codex/config.toml\n", filepath.Join(target, configSnippetName))
	fmt.Fprintln(os.Stderr, "  (or open the snippet and merge into your existing config)")
	return count, nil
}

// Uninstall removes the install target. Does NOT roll back the
// config.toml lines — the user added them manually; they remove them
// manually. This matches the install side of the contract.
func Uninstall(target string) error {
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove %s: %w", target, err)
	}
	fmt.Fprintf(os.Stderr, "[uninstall] codex: removed %s\n", target)
	fmt.Fprintf(os.Stderr, "[uninstall] codex: REMINDER — remove any [mcp_servers.scout] and skills.config entries from ~/.codex/config.toml\n")
	return nil
}

// PrintStatus reports install-state for Codex.
func PrintStatus(w *os.File) error {
	target, _ := Host{}.InstallTarget()
	fmt.Fprintf(w, "embedded plugin    : %s v%s\n", claude.Name, claude.Version)
	fmt.Fprintf(w, "install target     : %s\n", target)
	fmt.Fprintf(w, "target exists      : %v\n", pathExists(target))
	fmt.Fprintf(w, "config snippet     : %s\n", filepath.Join(target, configSnippetName))
	fmt.Fprintf(w, "config patched     : %v  (manual — check ~/.codex/config.toml for [mcp_servers.%s])\n",
		configHasMcpServer(), claude.McpServerName)
	return nil
}

// configSnippet returns the TOML fragment the user pastes into
// ~/.codex/config.toml. Includes [mcp_servers.scout] plus one
// skills.config entry per skill in the install target.
func configSnippet(target string) ([]byte, error) {
	skillsDir := filepath.Join(target, "skills")
	entries, _ := os.ReadDir(skillsDir)
	var b bytes.Buffer
	fmt.Fprintf(&b, "# scout — append this block to ~/.codex/config.toml\n")
	fmt.Fprintf(&b, "# Generated %s. Removing scout: delete these lines.\n\n", claude.Version)
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", claude.McpServerName)
	fmt.Fprintf(&b, "command = %q\n", claude.McpCommand)
	fmt.Fprintf(&b, "args = [")
	for i, a := range claude.McpArgs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", a)
	}
	b.WriteString("]\n\n")
	if len(entries) > 0 {
		fmt.Fprintf(&b, "# Skills — paths point at the install target above.\n")
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillPath := filepath.ToSlash(filepath.Join(target, "skills", e.Name()))
			fmt.Fprintf(&b, "[[skills.config]]\n")
			fmt.Fprintf(&b, "path = %q\n", skillPath)
			fmt.Fprintf(&b, "enabled = true\n\n")
		}
	}
	return b.Bytes(), nil
}

// configHasMcpServer best-effort grep of ~/.codex/config.toml for our
// mcp_servers stanza. Used by status/doctor as a hint, not a guarantee.
func configHasMcpServer() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		return false
	}
	needle := fmt.Sprintf("[mcp_servers.%s]", claude.McpServerName)
	return bytes.Contains(data, []byte(needle))
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// silence unused-import warnings when the rest of the package stubs out.
var _ = aihost.TemplateData{}
var _ = strings.Builder{}
