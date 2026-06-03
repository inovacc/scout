package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleYAML = `
version: "1"
name: checkout
auth: { profile: "p-123" }
vars: { baseURL: "https://api.example.com" }
steps:
  - id: login
    request:
      method: POST
      url: "${baseURL}/login"
      json: { username: "${secret.USERNAME}" }
    extract:
      - { var: token, from: response.json, path: "$.access_token" }
    expect: { status: 200 }
`

func TestParseAndValidate(t *testing.T) {
	f, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Name != "checkout" || len(f.Steps) != 1 || f.Steps[0].ID != "login" {
		t.Fatalf("unexpected spec: %+v", f)
	}
	if f.Steps[0].Extract[0].Var != "token" {
		t.Fatalf("extract not parsed: %+v", f.Steps[0].Extract)
	}
	if err := Validate(f); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateCatchesErrors(t *testing.T) {
	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "", Request: Request{Method: "GET", URL: "u"}},
		{ID: "dup", Request: Request{Method: "", URL: ""}},
		{ID: "dup", Request: Request{Method: "GET", URL: "u"}},
	}}
	err := Validate(f)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, want := range []string{"step 1", "method", "url", "duplicate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("validation error missing %q: %v", want, err)
		}
	}
}

func TestLoadFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "flow.yaml")
	if err := writeFile(t, p, sampleYAML); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(p); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
}

func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return osWriteFile(path, []byte(content))
}

func osWriteFile(p string, b []byte) error { return os.WriteFile(p, b, 0o644) }
