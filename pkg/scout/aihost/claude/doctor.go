package claude

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

// Doctor runs claude-side health checks: marketplace registration,
// settings.json `enabledPlugins` flip, .mcp.json spec-form presence,
// and `claude` CLI availability.
func Doctor() aihost.DoctorReport {
	target, _ := Host{}.InstallTarget()
	r := aihost.DoctorReport{Host: "claude", Target: target}

	add := func(name, verdict, detail, fix string) {
		r.Checks = append(r.Checks, aihost.DoctorCheck{
			Name: name, Verdict: verdict, Detail: detail, Fix: fix,
		})
	}

	if pathExists(target) {
		add("install_target", "PASS", target, "")
	} else {
		add("install_target", "FAIL", "missing "+target,
			"run `scout plugin install --host claude`")
	}
	if MarketplaceHasEntry() {
		add("marketplace_entry", "PASS", "marketplace.json declares scout", "")
	} else {
		add("marketplace_entry", "FAIL", "marketplace.json missing scout entry",
			"run `scout plugin install --host claude`")
	}
	if SettingsHasEnabled() {
		add("settings_enabled", "PASS",
			"settings.json enabledPlugins[scout@scout]=true", "")
	} else {
		add("settings_enabled", "FAIL",
			"settings.json missing enabledPlugins[scout@scout]",
			`add "scout@scout": true to enabledPlugins in ~/.claude/settings.json`)
	}
	if cmd, ok := McpServersHasScout(); ok {
		add("mcp_registered", "PASS", "mcp command="+cmd, "")
	} else {
		add("mcp_registered", "FAIL", "no scout MCP server registered",
			"check plugin .mcp.json was written during install")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		add("claude_cli", "WARN", "claude CLI not on PATH",
			"required for `claude plugin marketplace add` during install")
	} else {
		add("claude_cli", "PASS", "claude CLI on PATH", "")
	}
	if pathExists(filepath.Join(target, "hooks", "hooks.json")) {
		add("hooks_present", "PASS", "hooks/hooks.json in plugin tree", "")
	} else {
		add("hooks_present", "WARN", "hooks/hooks.json missing from plugin tree",
			"run `scout plugin install --host claude` to write the lifecycle hooks")
	}
	if _, err := exec.LookPath(McpCommand); err != nil {
		add("hook_binary", "WARN", McpCommand+" not on PATH — hooks cannot resolve",
			"install the scout binary on PATH (hook commands run via shell)")
	} else {
		add("hook_binary", "PASS", McpCommand+" on PATH (hooks resolve)", "")
	}
	switch present, total := PermissionsAllowCount(); {
	case present == total:
		add("permissions_allowlist", "PASS",
			fmt.Sprintf("%d/%d low-risk scout tools auto-approved", present, total), "")
	case present > 0:
		add("permissions_allowlist", "WARN",
			fmt.Sprintf("%d/%d scout tools in permissions.allow", present, total),
			"re-run `scout plugin install --host claude` to auto-approve the rest")
	default:
		add("permissions_allowlist", "WARN", "no scout tools auto-approved (prompts per tool)",
			"run `scout plugin install --host claude` to add the scout allowlist")
	}

	r.Verdict = computeVerdict(r.Checks)
	return r
}

func (Host) Doctor() aihost.DoctorReport { return Doctor() }

func computeVerdict(checks []aihost.DoctorCheck) string {
	hasFail := false
	hasWarn := false
	for _, c := range checks {
		switch c.Verdict {
		case "FAIL":
			hasFail = true
		case "WARN":
			hasWarn = true
		}
	}
	switch {
	case hasFail:
		return "FAILED"
	case hasWarn:
		return "DEGRADED"
	default:
		return "OK"
	}
}
