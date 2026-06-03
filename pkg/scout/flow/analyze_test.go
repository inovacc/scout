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

// capturingProvider records the user-prompt (the digest) it receives.
type capturingProvider struct{ lastUser string }

func (c *capturingProvider) Name() string { return "cap" }
func (c *capturingProvider) Complete(_ context.Context, sys, user string) (string, error) {
	c.lastUser = user
	switch {
	case strings.Contains(sys, "classify"):
		return `{"keep":[0,1],"drop":[]}`, nil
	case strings.Contains(sys, "correlate"):
		return `{"chains":[]}`, nil
	default:
		return `{"name":"x"}`, nil
	}
}

func TestAnalyzeRedactsSecretsFromLLMPayload(t *testing.T) {
	capt := &Capture{Version: "1", Entries: []CaptureEntry{
		{Method: "GET", URL: "https://api.x/cb?code=AUTHCODE_SECRET&state=s",
			ReqHeaders: map[string]string{"Authorization": "Bearer LONGTOKENVALUE", "X-Api-Key": "abc123"},
			Status:     200, RespBody: `{"access_token":"SUPER_SECRET_TOKEN","n":7}`},
		{Method: "GET", URL: "https://api.x/me", Status: 200, RespBody: `plain body SECRET_BODY_VALUE`},
	}}
	prov := &capturingProvider{}
	if _, _, err := Analyze(capt, prov, AnalyzeOptions{Name: "x"}); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	for _, secret := range []string{"AUTHCODE_SECRET", "SUPER_SECRET_TOKEN", "LONGTOKENVALUE", "abc123", "SECRET_BODY_VALUE"} {
		if strings.Contains(prov.lastUser, secret) {
			t.Errorf("secret %q leaked into LLM payload: %s", secret, prov.lastUser)
		}
	}
	// structure must survive: the response key name is needed for correlation
	if !strings.Contains(prov.lastUser, "access_token") {
		t.Errorf("redaction destroyed structure (key access_token missing): %s", prov.lastUser)
	}
}

func TestAnalyzeDegradesOnMalformedClassifyJSON(t *testing.T) {
	capt := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: "https://api.x/a", Status: 200}}}
	prov := stubProvider{classify: `{"keep":`, correlate: `{}`, synth: `{}`} // garbage, non-error
	spec, report, err := Analyze(capt, prov, AnalyzeOptions{Name: "x"})
	if err != nil {
		t.Fatalf("Analyze should degrade, not error: %v", err)
	}
	if len(spec.Steps) != 1 || !report.Degraded {
		t.Fatalf("expected degraded skeleton on malformed JSON: %+v report=%+v", spec, report)
	}
}

func TestAnalyzeNotesUnappliedChain(t *testing.T) {
	capt := &Capture{Version: "1", Entries: []CaptureEntry{
		{Method: "POST", URL: "https://api.x/login", Status: 200, RespBody: `{"id":"i"}`},
		{Method: "GET", URL: "https://api.x/me", Status: 200},
	}}
	prov := stubProvider{
		classify:  `{"keep":[0,1],"drop":[]}`,
		correlate: `{"chains":[{"from_entry":0,"from":"response.json","path":"$.id","to_entry":1,"into":"query","name":"id","template":"${id}","var":"id","confidence":0.5}]}`,
		synth:     `{"name":"x"}`,
	}
	_, report, err := Analyze(capt, prov, AnalyzeOptions{Name: "x"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	joined := strings.Join(report.Notes, " | ")
	if !strings.Contains(joined, "into=") {
		t.Fatalf("expected a Note about the unapplied into=query chain, got: %v", report.Notes)
	}
}
