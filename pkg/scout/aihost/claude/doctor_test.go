package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aihost"
)

// TestComputeVerdict pins the precedence: any FAIL -> FAILED, else any
// WARN -> DEGRADED, else OK (including the empty-checks case).
func TestComputeVerdict(t *testing.T) {
	tests := []struct {
		name   string
		checks []aihost.DoctorCheck
		want   string
	}{
		{"empty is OK", nil, "OK"},
		{"all pass", []aihost.DoctorCheck{{Verdict: "PASS"}, {Verdict: "PASS"}}, "OK"},
		{"warn only", []aihost.DoctorCheck{{Verdict: "PASS"}, {Verdict: "WARN"}}, "DEGRADED"},
		{"fail wins over warn", []aihost.DoctorCheck{{Verdict: "WARN"}, {Verdict: "FAIL"}}, "FAILED"},
		{"fail only", []aihost.DoctorCheck{{Verdict: "FAIL"}}, "FAILED"},
		{"unknown verdict ignored", []aihost.DoctorCheck{{Verdict: "PASS"}, {Verdict: "MAYBE"}}, "OK"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeVerdict(tt.checks); got != tt.want {
				t.Errorf("computeVerdict(%v) = %q, want %q", tt.checks, got, tt.want)
			}
		})
	}
}

// checkByName finds a named check in a report.
func checkByName(r aihost.DoctorReport, name string) (aihost.DoctorCheck, bool) {
	for _, c := range r.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return aihost.DoctorCheck{}, false
}

// TestDoctorAllMissing exercises the cold-install path: nothing is set
// up, so install_target / marketplace_entry / settings_enabled /
// mcp_registered should all FAIL and the overall verdict is FAILED.
func TestDoctorAllMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	r := Doctor()
	if r.Host != "claude" {
		t.Errorf("Host = %q, want claude", r.Host)
	}
	if r.Verdict != "FAILED" {
		t.Errorf("Verdict = %q, want FAILED", r.Verdict)
	}
	for _, name := range []string{"install_target", "marketplace_entry", "settings_enabled", "mcp_registered"} {
		c, ok := checkByName(r, name)
		if !ok {
			t.Errorf("missing check %q", name)
			continue
		}
		if c.Verdict != "FAIL" {
			t.Errorf("check %q verdict = %q, want FAIL", name, c.Verdict)
		}
		if c.Fix == "" {
			t.Errorf("failing check %q should carry a Fix hint", name)
		}
	}
}

// TestDoctorAllPresent exercises the warm-install path: target,
// marketplace, settings, and plugin .mcp.json all exist. The four
// install checks PASS. The claude_cli check is environment-dependent
// (PASS or WARN) so the overall verdict is OK or DEGRADED, never FAILED.
func TestDoctorAllPresent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	target := filepath.Join(tmp, ".claude", "plugins", "marketplaces", Name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := PatchMarketplace(); err != nil {
		t.Fatalf("PatchMarketplace: %v", err)
	}
	if err := PatchSettings(); err != nil {
		t.Fatalf("PatchSettings: %v", err)
	}
	// plugin .mcp.json (primary mcp source)
	pluginMcp := filepath.Join(target, ".mcp.json")
	if err := os.WriteFile(pluginMcp, mustMcpJSON(t), 0o644); err != nil {
		t.Fatal(err)
	}

	r := Doctor()
	for _, name := range []string{"install_target", "marketplace_entry", "settings_enabled", "mcp_registered"} {
		c, ok := checkByName(r, name)
		if !ok {
			t.Errorf("missing check %q", name)
			continue
		}
		if c.Verdict != "PASS" {
			t.Errorf("check %q verdict = %q, want PASS (detail=%q)", name, c.Verdict, c.Detail)
		}
	}
	if r.Verdict == "FAILED" {
		t.Errorf("Verdict = FAILED, want OK or DEGRADED when install checks pass")
	}
	if _, ok := checkByName(r, "claude_cli"); !ok {
		t.Error("missing claude_cli check")
	}
}

func mustMcpJSON(t *testing.T) []byte {
	t.Helper()
	b, err := McpJSON()
	if err != nil {
		t.Fatalf("McpJSON: %v", err)
	}
	return b
}
