package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/flow"
)

func TestRenderFlowReportNoSecretLeak(t *testing.T) {
	rep := &flow.Report{Dropped: []int{2}, Chains: []flow.Chain{{Var: "token", From: "response.json", Path: "$.access_token", Confidence: 0.9}}}
	out := renderFlowReport(rep)
	if !strings.Contains(out, "token") || !strings.Contains(out, "0.9") {
		t.Fatalf("report missing chain info: %s", out)
	}
}

func TestRenderVerifyReport(t *testing.T) {
	rep := &flow.VerifyReport{OK: false, Steps: []flow.StepDiff{{ID: "a", ExpectedStatus: 200, ActualStatus: 403, StatusMatch: false}}}
	out := renderVerifyReport(rep)
	if !strings.Contains(out, "403") || !strings.Contains(out, "a") {
		t.Fatalf("verify render missing drift: %s", out)
	}
}

// TestWriteFlowSpecPermsAreOwnerOnly guards that a generated flow.yaml is written
// 0o600. A generated spec is meant to be secret-free, but the perms are a defensive
// backstop. POSIX perms are not enforced on Windows, so skip there.
func TestWriteFlowSpecPermsAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions are not enforced on Windows")
	}
	p := filepath.Join(t.TempDir(), "flow.yaml")
	if err := writeFlowSpec(p, &flow.FlowSpec{Version: "1"}); err != nil {
		t.Fatalf("writeFlowSpec: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("flow.yaml perms = %o, want 600", perm)
	}
}
