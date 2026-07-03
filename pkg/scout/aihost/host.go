// Package aihost defines the cross-host plugin contract for Scout. Each
// concrete AI host (Claude Code, Codex CLI, Gemini CLI, ...) lives in a
// subpackage and satisfies the Host interface. Optional capabilities
// (Installer, Status, Doctor) are picked up via type assertion so hosts
// can ship lazily.
//
// The CLI dispatcher in cmd/scout/ picks the right host based on
// --host (or legacy --claude flag) and routes install/uninstall/status
// uniformly across all registered hosts.
package aihost

import "os"

// Host is the minimum surface a plugin host implementation must expose
// so cmd/scout/plugin_install.go can drive install/uninstall uniformly.
//
// Concrete hosts: pkg/scout/aihost/claude (full), codex (stub), gemini (stub).
type Host interface {
	// Name returns the short host identifier ("claude", "codex", "gemini").
	Name() string

	// InstallTarget returns the absolute filesystem path where the
	// rendered plugin tree should be written (typically under $HOME).
	InstallTarget() (string, error)

	// Walk yields every rendered plugin asset (commands, skills,
	// agents — markdown). Manifest files come from ManifestFiles().
	Walk(fn func(path string, data []byte) error) error

	// ManifestFiles returns synthesised host-specific manifest payloads
	// keyed by their plugin-tree-relative path (e.g.
	// ".claude-plugin/plugin.json", ".mcp.json").
	ManifestFiles() (map[string][]byte, error)
}

// Installer is an optional capability for hosts that know how to wire
// themselves into their CLI's marketplace / settings layer.
type Installer interface {
	// Install writes plugin files to target and patches any host-side
	// state (marketplace.json, settings.json, etc.). Returns file count
	// written.
	Install(target string) (int, error)
	// Uninstall removes plugin files at target and undoes patches.
	Uninstall(target string) error
}

// Status is an optional capability for hosts that report install
// health checks suitable for a `scout plugin status` CLI subcommand.
type Status interface {
	PrintStatus(w *os.File) error
}

// HookProvider is an optional capability for hosts that emit lifecycle-hook
// manifests binding the host's hook events to `scout hook <event>` subcommands.
// Like ManifestFiles, hook payloads are written OUTSIDE the commands/agents/skills
// stale-sweep so they survive re-install. Only hosts with a native hook surface
// (Claude Code today) implement it; skills-only hosts (codex/gemini) omit it.
type HookProvider interface {
	// Hooks returns hook-manifest payloads keyed by their plugin-tree-relative
	// path (e.g. "hooks/hooks.json").
	Hooks() (map[string][]byte, error)
}

// DoctorCheck is one host-side health check result.
type DoctorCheck struct {
	Name    string `json:"name"`
	Verdict string `json:"verdict"` // PASS | WARN | FAIL
	Detail  string `json:"detail,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

// DoctorReport is the structured output of a host's self-diagnosis.
type DoctorReport struct {
	Host    string        `json:"host"`
	Target  string        `json:"target"`
	Checks  []DoctorCheck `json:"checks"`
	Verdict string        `json:"verdict"` // OK | DEGRADED | FAILED
}

// Doctor is an optional capability that returns host-side health
// findings (marketplace registration, settings flips, CLI presence).
type Doctor interface {
	Doctor() DoctorReport
}
