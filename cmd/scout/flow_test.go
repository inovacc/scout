package main

import (
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
