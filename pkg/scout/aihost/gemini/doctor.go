package gemini

import (
	"os"
	"os/exec"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

func (Host) Doctor() aihost.DoctorReport {
	target, _ := Host{}.InstallTarget()
	r := aihost.DoctorReport{Host: "gemini", Target: target}
	add := func(name, verdict, detail, fix string) {
		r.Checks = append(r.Checks, aihost.DoctorCheck{Name: name, Verdict: verdict, Detail: detail, Fix: fix})
	}
	if _, err := os.Stat(target); err == nil {
		add("install_target", "PASS", target, "")
	} else {
		add("install_target", "WARN", "missing "+target,
			"gemini installer not implemented; run `scout plugin extract --host gemini` manually")
	}
	if _, err := exec.LookPath("gemini"); err != nil {
		add("gemini_cli", "WARN", "gemini CLI not on PATH", "")
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
