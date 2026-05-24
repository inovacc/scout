package codex

import (
	"os"
	"os/exec"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

// Doctor stub — verifies install target presence and CLI availability.
// Marketplace / settings checks land here when codex stabilises that
// surface.
func (Host) Doctor() aihost.DoctorReport {
	target, _ := Host{}.InstallTarget()
	r := aihost.DoctorReport{Host: "codex", Target: target}
	add := func(name, verdict, detail, fix string) {
		r.Checks = append(r.Checks, aihost.DoctorCheck{Name: name, Verdict: verdict, Detail: detail, Fix: fix})
	}
	if _, err := os.Stat(target); err == nil {
		add("install_target", "PASS", target, "")
	} else {
		add("install_target", "WARN", "missing "+target,
			"codex installer not implemented; run `scout plugin extract --host codex` manually")
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
