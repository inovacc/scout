package runbook

import (
	"strings"
	"testing"
)

func TestPlan_NilRunbook(t *testing.T) {
	_, err := Plan(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil runbook")
	}

	if !strings.Contains(err.Error(), "nil runbook") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPlan_UnknownType(t *testing.T) {
	// A nil browser is fine here because the type switch hits the default
	// branch before any browser access.
	_, err := Plan(nil, &Runbook{Name: "x", Type: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}

	if !strings.Contains(err.Error(), `unknown type "bogus"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecutionPlan_String_Extract(t *testing.T) {
	p := &ExecutionPlan{
		Runbook: "products",
		Type:    "extract",
		URL:     "https://example.com/list",
		Valid:   true,
		Selectors: []SelectorCheck{
			{Name: "container", Selector: ".product", Status: PlanOK, Count: 12, Score: SelectorScore{Tier: "good", Score: 0.8}},
			{Name: "field:title", Selector: "h3", Status: PlanMissing, Count: 0, Score: SelectorScore{Tier: "fair", Score: 0.5}},
			{Name: "field:noop", Selector: "", Status: PlanSkipped, Count: 0, Score: SelectorScore{Tier: "fragile", Score: 0.1}},
		},
		SampleCount: 12,
		Errors:      []string{`field "title" selector "h3" not found`},
		Warnings:    []string{"fragile selector for field:noop"},
	}

	out := p.String()

	// Header.
	if !strings.Contains(out, "Runbook: products (extract)") {
		t.Errorf("missing header line:\n%s", out)
	}

	if !strings.Contains(out, "URL: https://example.com/list") {
		t.Errorf("missing URL line:\n%s", out)
	}

	// Status icons: ok=+, missing=-, skipped=~.
	if !strings.Contains(out, "+ container") {
		t.Errorf("expected '+ container' (ok icon):\n%s", out)
	}

	if !strings.Contains(out, "- field:title") {
		t.Errorf("expected '- field:title' (missing icon):\n%s", out)
	}

	if !strings.Contains(out, "~ field:noop") {
		t.Errorf("expected '~ field:noop' (skipped icon):\n%s", out)
	}

	// Extract summary uses SampleCount.
	if !strings.Contains(out, "Plan: 12 items to extract") {
		t.Errorf("expected extract summary:\n%s", out)
	}

	if !strings.Contains(out, "1 error(s)") {
		t.Errorf("expected error count:\n%s", out)
	}

	if !strings.Contains(out, "1 warning(s)") {
		t.Errorf("expected warning count:\n%s", out)
	}

	if !strings.Contains(out, `Error: field "title" selector "h3" not found`) {
		t.Errorf("expected error detail:\n%s", out)
	}

	if !strings.Contains(out, "Warning: fragile selector for field:noop") {
		t.Errorf("expected warning detail:\n%s", out)
	}
}

func TestExecutionPlan_String_Automate(t *testing.T) {
	p := &ExecutionPlan{
		Runbook: "login",
		Type:    "automate",
		URL:     "https://example.com/login",
		Valid:   true,
		Steps: []StepCheck{
			{Index: 0, Action: "navigate", Status: PlanOK},
			{Index: 1, Action: "type", Selector: "#user", Status: PlanOK},
			{Index: 2, Action: "click", Selector: "#missing", Status: PlanMissing},
			{Index: 3, Action: "eval", Status: PlanSkipped},
		},
	}

	out := p.String()

	if !strings.Contains(out, "Steps:") {
		t.Errorf("expected Steps section:\n%s", out)
	}

	// navigate step has no selector, so detail falls back to the action name.
	if !strings.Contains(out, "[0] navigate") {
		t.Errorf("expected navigate step line:\n%s", out)
	}

	// type step shows its selector as detail.
	if !strings.Contains(out, "#user") {
		t.Errorf("expected selector detail for type step:\n%s", out)
	}

	// missing step uses '-' icon.
	if !strings.Contains(out, "- [2] click") {
		t.Errorf("expected missing-step icon:\n%s", out)
	}

	// skipped step uses '~' icon.
	if !strings.Contains(out, "~ [3] eval") {
		t.Errorf("expected skipped-step icon:\n%s", out)
	}

	// Automate summary counts steps, not SampleCount.
	if !strings.Contains(out, "Plan: 4 steps to execute") {
		t.Errorf("expected automate summary:\n%s", out)
	}

	// No errors/warnings -> these phrases must be absent.
	if strings.Contains(out, "error(s)") || strings.Contains(out, "warning(s)") {
		t.Errorf("did not expect error/warning counts:\n%s", out)
	}
}
