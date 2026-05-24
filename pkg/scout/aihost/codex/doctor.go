package codex

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

// Doctor runs codex-side health checks: install target presence, the
// generated config snippet, whether the user has merged the snippet
// into ~/.codex/config.toml (best-effort grep), and codex CLI on PATH.
func (Host) Doctor() aihost.DoctorReport {
	target, _ := Host{}.InstallTarget()
	r := aihost.DoctorReport{Host: "codex", Target: target}
	add := func(name, verdict, detail, fix string) {
		r.Checks = append(r.Checks, aihost.DoctorCheck{
			Name: name, Verdict: verdict, Detail: detail, Fix: fix,
		})
	}

	if _, err := os.Stat(target); err == nil {
		add("install_target", "PASS", target, "")
	} else {
		add("install_target", "FAIL", "missing "+target,
			"run `scout plugin install --host codex`")
	}

	snippet := filepath.Join(target, configSnippetName)
	if _, err := os.Stat(snippet); err == nil {
		add("config_snippet", "PASS", snippet, "")
	} else {
		add("config_snippet", "FAIL", "missing "+snippet,
			"run `scout plugin install --host codex` to regenerate")
	}

	if configHasMcpServer() {
		add("config_patched", "PASS",
			"~/.codex/config.toml contains [mcp_servers.scout]", "")
	} else {
		add("config_patched", "WARN",
			"~/.codex/config.toml has no [mcp_servers.scout] stanza",
			"cat "+snippet+" >> ~/.codex/config.toml  (manual final step)")
	}

	if _, err := exec.LookPath("codex"); err != nil {
		add("codex_cli", "WARN", "codex CLI not on PATH", "")
	} else {
		add("codex_cli", "PASS", "codex CLI on PATH", "")
	}

	hasFail, hasWarn := false, false
	for _, c := range r.Checks {
		switch c.Verdict {
		case "FAIL":
			hasFail = true
		case "WARN":
			hasWarn = true
		}
	}
	switch {
	case hasFail:
		r.Verdict = "FAILED"
	case hasWarn:
		r.Verdict = "DEGRADED"
	default:
		r.Verdict = "OK"
	}
	return r
}
