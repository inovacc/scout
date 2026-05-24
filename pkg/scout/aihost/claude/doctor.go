package claude

import (
	"os/exec"

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
