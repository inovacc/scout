package gemini

import (
	"os/exec"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

// Doctor runs gemini-side health checks: install target, manifest
// presence, gemini CLI availability.
func (Host) Doctor() aihost.DoctorReport {
	target, _ := Host{}.InstallTarget()
	r := aihost.DoctorReport{Host: "gemini", Target: target}
	add := func(name, verdict, detail, fix string) {
		r.Checks = append(r.Checks, aihost.DoctorCheck{Name: name, Verdict: verdict, Detail: detail, Fix: fix})
	}

	if pathExists(target) {
		add("install_target", "PASS", target, "")
	} else {
		add("install_target", "FAIL", "missing "+target,
			"run `scout plugin install --host gemini`")
	}

	manifest := filepath.Join(target, "gemini-extension.json")
	if pathExists(manifest) {
		add("manifest_present", "PASS", manifest, "")
	} else {
		add("manifest_present", "FAIL", "missing "+manifest,
			"run `scout plugin install --host gemini` to regenerate")
	}

	if _, err := exec.LookPath("gemini"); err != nil {
		add("gemini_cli", "WARN", "gemini CLI not on PATH",
			"install gemini CLI to enable `gemini extensions install` registration")
	} else {
		add("gemini_cli", "PASS", "gemini CLI on PATH", "")
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
