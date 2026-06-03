package flow

import (
	"context"
	"strings"
	"testing"
)

// stubProvider implements llm.Provider with canned, prompt-keyed responses.
type stubProvider struct{ classify, correlate, synth string }

func (s stubProvider) Name() string { return "stub" }
func (s stubProvider) Complete(_ context.Context, sys, _ string) (string, error) {
	switch {
	case strings.Contains(sys, "classify"):
		return s.classify, nil
	case strings.Contains(sys, "correlate"):
		return s.correlate, nil
	default:
		return s.synth, nil
	}
}

func TestAnalyzeEmitsSpecAndReport(t *testing.T) {
	capt := &Capture{Version: "1", Entries: []CaptureEntry{
		{Method: "POST", URL: "https://api.x/login", Status: 200,
			RespHeaders: map[string]string{"X-CSRF-Token": "csrf"}, RespBody: `{"access_token":"t"}`},
		{Method: "GET", URL: "https://api.x/me", Status: 200,
			ReqHeaders: map[string]string{"Authorization": "Bearer t"}, RespBody: `{"id":"u"}`},
	}}
	prov := stubProvider{
		classify:  `{"keep":[0,1],"drop":[]}`,
		correlate: `{"chains":[{"from_entry":0,"from":"response.json","path":"$.access_token","to_entry":1,"into":"header","name":"Authorization","template":"Bearer ${token}","var":"token","confidence":0.9}]}`,
		synth:     `{"name":"login-flow"}`,
	}
	spec, report, err := Analyze(capt, prov, AnalyzeOptions{Name: "login-flow"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if spec.Name != "login-flow" || len(spec.Steps) != 2 {
		t.Fatalf("spec wrong: %+v", spec)
	}
	if len(spec.Steps[0].Extract) == 0 || spec.Steps[0].Extract[0].Var != "token" {
		t.Fatalf("chain extract missing: %+v", spec.Steps[0].Extract)
	}
	if got := spec.Steps[1].Request.Headers["Authorization"]; got != "Bearer ${token}" {
		t.Fatalf("chain inject missing: %q", got)
	}
	if report == nil || len(report.Chains) != 1 {
		t.Fatalf("report missing chains: %+v", report)
	}
}

func TestAnalyzeDegradesOnLLMError(t *testing.T) {
	capt := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: "https://api.x/a", Status: 200}}}
	spec, report, err := Analyze(capt, errProvider{}, AnalyzeOptions{Name: "x"})
	if err != nil {
		t.Fatalf("Analyze should degrade, not error: %v", err)
	}
	if len(spec.Steps) != 1 || !report.Degraded {
		t.Fatalf("expected degraded skeleton: %+v report=%+v", spec, report)
	}
}

type errProvider struct{}

func (errProvider) Name() string { return "err" }
func (errProvider) Complete(_ context.Context, _, _ string) (string, error) {
	return "", errTest
}

var errTest = &analyzeTestError{}

type analyzeTestError struct{}

func (*analyzeTestError) Error() string { return "boom" }
