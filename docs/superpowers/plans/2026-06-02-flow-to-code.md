# Scout Flow (Flow-to-Code) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `pkg/scout/flow` + `scout flow capture|analyze|run|verify` — record a browser flow's network traffic, let an LLM turn it into a reviewed declarative `flow.yaml`, and replay the underlying REST/GraphQL calls deterministically with no browser, sourcing auth from a vault profile.

**Architecture:** A normalized `Capture` artifact (from the existing `internal/engine/hijack` capture) feeds a staged-LLM `Analyze` that emits a `FlowSpec` (`flow.yaml`) + a human-review report. A deterministic `Run` executes the spec via `net/http` with a `${var}`/`${secret.NAME}` interpolation layer (extract response fields → inject into later requests), resolving secrets from `pkg/scout/vault` at send time. `Verify` re-runs and diffs against the golden capture.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3` (in go.mod), `encoding/json` + a tiny custom JSON path-walker (no new dep), `net/http`, `internal/engine/hijack` (capture), `internal/engine/llm` (analyze), `pkg/scout/vault` (auth), Cobra CLI, `httptest`-based tests.

---

## Conventions for every task

- **Package:** `pkg/scout/flow` (import path `github.com/inovacc/scout/pkg/scout/flow`). It MAY import `github.com/inovacc/scout/internal/engine/hijack`, `.../internal/engine/llm`, `github.com/inovacc/scout/pkg/scout`, and `github.com/inovacc/scout/pkg/scout/vault` (same module — verified).
- **Error wrapping:** `fmt.Errorf("scout: flow: <action>: %w", err)`.
- **Run a single test:** `go test ./pkg/scout/flow/ -run TestName -v`. Windows fallback if `go` unresolved: `& 'C:\Program Files\Go\bin\go.exe' test ./pkg/scout/flow/ -run TestName -v`.
- **Do NOT** run `go build ./...` (root has no main). Scope to `./pkg/scout/flow/` and `./cmd/scout/`.
- **Browser-dependent tests** (only Task 11) use `//go:build integration` + a `t.Skipf`-on-no-browser helper; everything else is pure `httptest`/unit and always runs.
- **Commit** after every task. NO AI attribution (no `Co-Authored-By` / "Generated with").

## File structure (created by this plan)

| File | Responsibility |
|------|----------------|
| `pkg/scout/flow/capture_artifact.go` | `Capture`/`CaptureEntry` normalized types + `SaveCapture`/`LoadCapture` |
| `pkg/scout/flow/jsonpath.go` | `ExtractPath(data, "$.a.0.b")` — encoding/json path-walker (no dep) |
| `pkg/scout/flow/interp.go` | `Interpolate` — `${var}`/`${var.X}`/`${secret.X}` resolution + `SecretResolver` |
| `pkg/scout/flow/spec.go` | `FlowSpec` & friends + `LoadFile`/`Parse`/`Validate` (mirrors `strategy`) |
| `pkg/scout/flow/runtime.go` | `Run(ctx, *FlowSpec, RunOptions)` — REST+GraphQL send, extract, expect |
| `pkg/scout/flow/secrets.go` | `VaultResolver` adapting `vault.Handle` to `SecretResolver` |
| `pkg/scout/flow/analyze.go` | `Analyze(*Capture, llm.Provider, AnalyzeOptions)` — staged passes → spec + `Report` |
| `pkg/scout/flow/verify.go` | `Verify(ctx, *FlowSpec, golden *Capture, RunOptions)` — re-run + diff |
| `pkg/scout/flow/capture.go` | `CaptureFlow(ctx, *scout.Page, CaptureOptions)` — drive hijacker → `Capture` |
| `cmd/scout/flow.go` | `scout flow capture/analyze/run/verify` |

Build order: types & pure logic first (1–4), then runtime (5–7), analyze (8), verify (9), browser capture (10), CLI (11), hygiene+docs (12).

---

## Task 1: Capture artifact type

**Files:** Create `pkg/scout/flow/capture_artifact.go`; Test `pkg/scout/flow/capture_artifact_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/flow/capture_artifact_test.go
package flow

import (
	"path/filepath"
	"testing"
)

func TestCaptureSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	in := &Capture{
		Version: "1",
		Name:    "demo",
		Entries: []CaptureEntry{{
			Method: "POST", URL: "https://api.example.com/login",
			ReqHeaders: map[string]string{"Content-Type": "application/json"},
			ReqBody:    `{"u":"a"}`,
			Status:     200,
			RespHeaders: map[string]string{"X-CSRF-Token": "csrf-1"},
			RespBody:   `{"access_token":"tok-1"}`,
			MimeType:   "application/json",
		}},
	}
	if err := SaveCapture(path, in); err != nil {
		t.Fatalf("SaveCapture: %v", err)
	}
	out, err := LoadCapture(path)
	if err != nil {
		t.Fatalf("LoadCapture: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].RespBody != `{"access_token":"tok-1"}` {
		t.Fatalf("round-trip mismatch: %+v", out.Entries)
	}
}

func TestLoadCaptureMissing(t *testing.T) {
	if _, err := LoadCapture(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing capture")
	}
}
```

- [ ] **Step 2: Run — expect FAIL** `go test ./pkg/scout/flow/ -run TestCapture -v` → `undefined: Capture`.

- [ ] **Step 3: Implement**

```go
// pkg/scout/flow/capture_artifact.go

// Package flow records a browser flow's network traffic, turns it (via an LLM
// analyzer) into a reviewed declarative FlowSpec, and replays the underlying
// REST/GraphQL calls deterministically with no browser.
package flow

import (
	"encoding/json"
	"fmt"
	"os"
)

// Capture is the flow pipeline's normalized record of a captured flow: one
// CaptureEntry per HTTP request/response pair (bodies preserved). It is the
// input to Analyze and the golden for Verify.
type Capture struct {
	Version string         `json:"version"`
	Name    string         `json:"name,omitempty"`
	Entries []CaptureEntry `json:"entries"`
}

// CaptureEntry is a single request/response pair.
type CaptureEntry struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	ReqHeaders  map[string]string `json:"req_headers,omitempty"`
	ReqBody     string            `json:"req_body,omitempty"`
	Status      int               `json:"status"`
	RespHeaders map[string]string `json:"resp_headers,omitempty"`
	RespBody    string            `json:"resp_body,omitempty"`
	MimeType    string            `json:"mime_type,omitempty"`
}

// SaveCapture writes c as pretty JSON with owner-only permissions (0o600). A
// capture is the RAW recording of a browser flow: its headers/bodies contain
// live secrets (auth tokens, cookies, CSRF, session IDs), so it MUST be 0o600.
// (Only the derived FlowSpec is secret-free, via ${secret.*} refs.)
func SaveCapture(path string, c *Capture) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: flow: marshal capture: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: flow: write capture: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil { // tighten a pre-existing world-readable file
		return fmt.Errorf("scout: flow: chmod capture: %w", err)
	}
	return nil
}

// LoadCapture reads a capture.json from disk.
func LoadCapture(path string) (*Capture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: read capture: %w", err)
	}
	var c Capture
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("scout: flow: parse capture: %w", err)
	}
	return &c, nil
}
```

- [ ] **Step 4: Run — expect PASS** `go test ./pkg/scout/flow/ -run TestCapture -v` and `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/capture_artifact.go pkg/scout/flow/capture_artifact_test.go
git commit -m "feat(flow): normalized Capture artifact (save/load)"
```

---

## Task 2: JSON path-walker

**Files:** Create `pkg/scout/flow/jsonpath.go`; Test `pkg/scout/flow/jsonpath_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/flow/jsonpath_test.go
package flow

import "testing"

func TestExtractPath(t *testing.T) {
	data := []byte(`{"data":{"cart":{"id":"c-1","total":42}},"items":[{"sku":"A"},{"sku":"B"}]}`)
	cases := []struct {
		path, want string
		wantErr    bool
	}{
		{"$.data.cart.id", "c-1", false},
		{"$.data.cart.total", "42", false},
		{"$.items.1.sku", "B", false},
		{"$.data.cart", `{"id":"c-1","total":42}`, false}, // object → compact JSON
		{"$.missing.key", "", true},
		{"$.items.9.sku", "", true},
	}
	for _, c := range cases {
		got, err := ExtractPath(data, c.path)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", c.path)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.path, err)
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.path, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: ExtractPath`).

- [ ] **Step 3: Implement**

```go
// pkg/scout/flow/jsonpath.go
package flow

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExtractPath walks a dotted JSON path against data and returns the value as a
// string. Supported syntax: a leading "$." then dot-separated keys; a numeric
// segment indexes an array (e.g. "$.items.1.sku"). Scalars stringify naturally;
// objects/arrays return compact JSON. A missing key or out-of-range index errors.
func ExtractPath(data []byte, path string) (string, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("scout: flow: jsonpath: parse: %w", err)
	}
	segs := strings.Split(strings.TrimPrefix(path, "$."), ".")
	cur := root
	for _, seg := range segs {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return "", fmt.Errorf("scout: flow: jsonpath: key %q not found", seg)
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(node) {
				return "", fmt.Errorf("scout: flow: jsonpath: index %q out of range", seg)
			}
			cur = node[i]
		default:
			return "", fmt.Errorf("scout: flow: jsonpath: cannot descend into %q", seg)
		}
	}
	return stringify(cur)
}

func stringify(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case float64:
		// JSON numbers decode to float64; render integers without a trailing .0.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10), nil
		}
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	case nil:
		return "", nil
	default: // object or array → compact JSON
		b, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("scout: flow: jsonpath: marshal value: %w", err)
		}
		return string(b), nil
	}
}
```

- [ ] **Step 4: Run — expect PASS** + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/jsonpath.go pkg/scout/flow/jsonpath_test.go
git commit -m "feat(flow): dependency-free JSON path-walker for extract"
```

---

## Task 3: Interpolation + SecretResolver

**Files:** Create `pkg/scout/flow/interp.go`; Test `pkg/scout/flow/interp_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/flow/interp_test.go
package flow

import (
	"errors"
	"testing"
)

type fakeSecrets struct{ m map[string]string }

func (f fakeSecrets) Secret(name string) ([]byte, error) {
	v, ok := f.m[name]
	if !ok {
		return nil, errors.New("no such secret")
	}
	return []byte(v), nil
}

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"token": "tok-1", "baseURL": "https://x"}
	sec := fakeSecrets{m: map[string]string{"PASSWORD": "hunter2"}}

	got, err := Interpolate("${baseURL}/u?t=${var.token}&p=${secret.PASSWORD}", vars, sec)
	if err != nil {
		t.Fatalf("Interpolate: %v", err)
	}
	if got != "https://x/u?t=tok-1&p=hunter2" {
		t.Fatalf("got %q", got)
	}
}

func TestInterpolateUnknownVarErrors(t *testing.T) {
	if _, err := Interpolate("${nope}", map[string]string{}, nil); err == nil {
		t.Fatal("expected error for unknown var")
	}
}

func TestInterpolateSecretWithoutResolverErrors(t *testing.T) {
	if _, err := Interpolate("${secret.X}", map[string]string{}, nil); err == nil {
		t.Fatal("expected error: secret ref but no resolver")
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: Interpolate`).

- [ ] **Step 3: Implement**

```go
// pkg/scout/flow/interp.go
package flow

import (
	"fmt"
	"regexp"
	"strings"
)

// SecretResolver yields a secret value by name. Implemented at run time by a
// vault-backed resolver (secrets.go); nil when a spec uses no ${secret.*}.
type SecretResolver interface {
	Secret(name string) ([]byte, error)
}

var placeholderRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// Interpolate replaces ${...} placeholders in s. Namespaces:
//   - ${secret.NAME} → secrets.Secret(NAME) (errors if secrets is nil)
//   - ${var.NAME} or bare ${NAME} → vars[NAME]
// An unknown placeholder is an error (fail closed — never emit a literal ${...}).
func Interpolate(s string, vars map[string]string, secrets SecretResolver) (string, error) {
	var firstErr error
	out := placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		name := placeholderRe.FindStringSubmatch(match)[1]
		if rest, ok := strings.CutPrefix(name, "secret."); ok {
			if secrets == nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("scout: flow: interp: ${secret.%s} but no vault profile bound", rest)
				}
				return match
			}
			b, err := secrets.Secret(rest)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("scout: flow: interp: secret %q: %w", rest, err)
				}
				return match
			}
			return string(b) // transient; the surrounding request bytes are discarded after send
		}
		key := strings.TrimPrefix(name, "var.")
		v, ok := vars[key]
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("scout: flow: interp: unknown variable %q", key)
			}
			return match
		}
		return v
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}
```

- [ ] **Step 4: Run — expect PASS** + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/interp.go pkg/scout/flow/interp_test.go
git commit -m "feat(flow): namespaced ${var}/${secret} interpolation"
```

---

## Task 4: FlowSpec types + Load/Parse/Validate

**Files:** Create `pkg/scout/flow/spec.go`; Test `pkg/scout/flow/spec_test.go`. Mirrors `pkg/scout/strategy` (yaml.v3 + JSON fallback, standalone `Validate`).

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/flow/spec_test.go
package flow

import (
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
		{ID: "", Request: Request{Method: "GET", URL: "u"}},        // empty id
		{ID: "dup", Request: Request{Method: "", URL: ""}},          // missing method+url
		{ID: "dup", Request: Request{Method: "GET", URL: "u"}},      // duplicate id
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
```

> Add a tiny test-only shim so the test compiles without importing `os` twice: in `spec_test.go` add `import "os"` and define `func osWriteFile(p string, b []byte) error { return os.WriteFile(p, b, 0o644) }`. (Or inline `os.WriteFile`.)

- [ ] **Step 2: Run — expect FAIL** (`undefined: FlowSpec`).

- [ ] **Step 3: Implement** (mirror `pkg/scout/strategy/strategy.go` Parse: yaml.v3 first, `os.Expand` for env `${VAR}` is NOT used here — flow uses its own `${...}` namespaces resolved at run time, so Parse must NOT expand. Just unmarshal.)

```go
// pkg/scout/flow/spec.go
package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type FlowSpec struct {
	Version string            `yaml:"version" json:"version"`
	Name    string            `yaml:"name" json:"name"`
	Auth    *AuthRef          `yaml:"auth,omitempty" json:"auth,omitempty"`
	Vars    map[string]string `yaml:"vars,omitempty" json:"vars,omitempty"`
	Steps   []FlowStep        `yaml:"steps" json:"steps"`
}

type AuthRef struct {
	Profile string `yaml:"profile" json:"profile"`
}

type FlowStep struct {
	ID      string    `yaml:"id" json:"id"`
	Request Request   `yaml:"request" json:"request"`
	Extract []Extract `yaml:"extract,omitempty" json:"extract,omitempty"`
	Expect  *Expect   `yaml:"expect,omitempty" json:"expect,omitempty"`
}

type Request struct {
	Method  string            `yaml:"method" json:"method"`
	URL     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	JSON    map[string]any    `yaml:"json,omitempty" json:"json,omitempty"`
	GraphQL *GraphQL          `yaml:"graphql,omitempty" json:"graphql,omitempty"`
}

type GraphQL struct {
	OperationName string         `yaml:"operationName,omitempty" json:"operationName,omitempty"`
	Query         string         `yaml:"query" json:"query"`
	Variables     map[string]any `yaml:"variables,omitempty" json:"variables,omitempty"`
}

// Extract binds a named var from a step's response.
// From is one of: response.json | response.header | response.body.
type Extract struct {
	Var  string `yaml:"var" json:"var"`
	From string `yaml:"from" json:"from"`
	Path string `yaml:"path" json:"path"` // JSONPath for response.json; header name for response.header; regex for response.body
}

type Expect struct {
	Status int `yaml:"status,omitempty" json:"status,omitempty"`
}

// LoadFile reads a flow spec from disk (YAML or JSON).
func LoadFile(path string) (*FlowSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: read file: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals a flow spec. YAML is a superset of JSON, so yaml.v3 handles
// both; on YAML failure it retries encoding/json for a clearer error. Parse does
// NOT validate (call Validate separately) and does NOT expand ${...} (those are
// resolved at run time against vars + the vault).
func Parse(data []byte) (*FlowSpec, error) {
	var f FlowSpec
	if err := yaml.Unmarshal(data, &f); err != nil {
		if jErr := json.Unmarshal(data, &f); jErr != nil {
			return nil, fmt.Errorf("scout: flow: parse: %w", err)
		}
	}
	return &f, nil
}

// Validate collects all problems and returns them as one error.
func Validate(f *FlowSpec) error {
	var errs []string
	if f.Version == "" {
		errs = append(errs, "version is required")
	}
	if len(f.Steps) == 0 {
		errs = append(errs, "at least one step is required")
	}
	seen := map[string]bool{}
	for i, s := range f.Steps {
		where := fmt.Sprintf("step %d", i+1)
		if s.ID != "" {
			where = fmt.Sprintf("step %d (%q)", i+1, s.ID)
		}
		if s.ID == "" {
			errs = append(errs, where+": id is required")
		} else if seen[s.ID] {
			errs = append(errs, where+": duplicate step id")
		}
		seen[s.ID] = true
		if s.Request.Method == "" {
			errs = append(errs, where+": request.method is required")
		}
		if s.Request.URL == "" {
			errs = append(errs, where+": request.url is required")
		}
		for j, e := range s.Extract {
			if e.Var == "" || e.From == "" || e.Path == "" {
				errs = append(errs, fmt.Sprintf("%s: extract %d: var/from/path are all required", where, j+1))
			}
			switch e.From {
			case "response.json", "response.header", "response.body":
			default:
				errs = append(errs, fmt.Sprintf("%s: extract %d: from must be response.json|response.header|response.body", where, j+1))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("scout: flow: validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
```

- [ ] **Step 4: Run — expect PASS** + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/spec.go pkg/scout/flow/spec_test.go
git commit -m "feat(flow): FlowSpec types + Load/Parse/Validate (yaml.v3)"
```

---

## Task 5: Runtime core (REST send + extract + expect)

**Files:** Create `pkg/scout/flow/runtime.go`; Test `pkg/scout/flow/runtime_test.go`

- [ ] **Step 1: Write the failing test** (real `httptest` server; proves extract→inject chaining deterministically, no LLM/browser)

```go
// pkg/scout/flow/runtime_test.go
package flow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunRESTExtractInjectChain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-CSRF-Token", "csrf-9")
		_, _ = w.Write([]byte(`{"access_token":"tok-9"}`))
	})
	var sawAuth, sawCSRF string
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawCSRF = r.Header.Get("X-CSRF-Token")
		_, _ = w.Write([]byte(`{"id":"u-1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := &FlowSpec{Version: "1", Name: "t", Vars: map[string]string{"baseURL": srv.URL},
		Steps: []FlowStep{
			{ID: "login", Request: Request{Method: "POST", URL: "${baseURL}/login"},
				Extract: []Extract{
					{Var: "token", From: "response.json", Path: "$.access_token"},
					{Var: "csrf", From: "response.header", Path: "X-CSRF-Token"},
				}, Expect: &Expect{Status: 200}},
			{ID: "me", Request: Request{Method: "GET", URL: "${baseURL}/me",
				Headers: map[string]string{"Authorization": "Bearer ${token}", "X-CSRF-Token": "${csrf}"}},
				Extract: []Extract{{Var: "uid", From: "response.json", Path: "$.id"}}},
		}}
	if err := Validate(f); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), f, RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sawAuth != "Bearer tok-9" || sawCSRF != "csrf-9" {
		t.Fatalf("injection failed: auth=%q csrf=%q", sawAuth, sawCSRF)
	}
	if res.Steps[1].Extracted["uid"] != "u-1" {
		t.Fatalf("final extract failed: %+v", res.Steps[1].Extracted)
	}
}

func TestRunExpectStatusMismatchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "x", Request: Request{Method: "GET", URL: srv.URL}, Expect: &Expect{Status: 200}}}}
	if _, err := Run(context.Background(), f, RunOptions{}); err == nil {
		t.Fatal("expected expect-mismatch error")
	}
}

// guard for unused imports in some toolchains
var _ = io.Discard
var _ = json.Marshal
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: Run`).

- [ ] **Step 3: Implement** (REST only here; GraphQL added in Task 6, secrets in Task 7)

```go
// pkg/scout/flow/runtime.go
package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RunOptions configures a flow run.
type RunOptions struct {
	Client  *http.Client    // nil → a default 30s client
	Secrets SecretResolver  // nil → ${secret.*} is an error
	Vars    map[string]string // overrides merged over FlowSpec.Vars
}

// RunResult is the per-step outcome of a run.
type RunResult struct {
	Steps []StepResult
}

// StepResult records one step's response + the vars it bound.
type StepResult struct {
	ID        string
	Status    int
	Body      string
	Extracted map[string]string
}

// Run executes the flow's steps in order, threading extracted vars into later
// requests. It does NOT touch a browser. Secrets are resolved per-request via
// opts.Secrets and never retained.
func Run(ctx context.Context, f *FlowSpec, opts RunOptions) (*RunResult, error) {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	// vars: spec.Vars overlaid with opts.Vars, then grown by extracts.
	vars := map[string]string{}
	for k, v := range f.Vars {
		vars[k] = v
	}
	for k, v := range opts.Vars {
		vars[k] = v
	}

	res := &RunResult{}
	for _, step := range f.Steps {
		sr, err := runStep(ctx, client, step, vars, opts.Secrets)
		if err != nil {
			return res, fmt.Errorf("scout: flow: step %q: %w", step.ID, err)
		}
		res.Steps = append(res.Steps, *sr)
	}
	return res, nil
}

func runStep(ctx context.Context, client *http.Client, step FlowStep, vars map[string]string, secrets SecretResolver) (*StepResult, error) {
	url, err := Interpolate(step.Request.URL, vars, secrets)
	if err != nil {
		return nil, err
	}
	body, contentType, err := buildBody(step.Request, vars, secrets)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, step.Request.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range step.Request.Headers {
		hv, hErr := Interpolate(v, vars, secrets)
		if hErr != nil {
			return nil, hErr
		}
		req.Header.Set(k, hv)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if step.Expect != nil && step.Expect.Status != 0 && resp.StatusCode != step.Expect.Status {
		return nil, fmt.Errorf("expected status %d, got %d", step.Expect.Status, resp.StatusCode)
	}

	sr := &StepResult{ID: step.ID, Status: resp.StatusCode, Body: string(raw), Extracted: map[string]string{}}
	for _, e := range step.Extract {
		val, eErr := applyExtract(e, resp.Header, raw)
		if eErr != nil {
			return nil, eErr
		}
		vars[e.Var] = val // bind for later steps
		sr.Extracted[e.Var] = val
	}
	return sr, nil
}

// buildBody returns the request body reader + Content-Type. REST uses Request.JSON;
// GraphQL is added in Task 6.
func buildBody(r Request, vars map[string]string, secrets SecretResolver) (io.Reader, string, error) {
	if r.JSON == nil {
		return http.NoBody, "", nil
	}
	interp, err := interpolateJSON(r.JSON, vars, secrets)
	if err != nil {
		return nil, "", err
	}
	data, err := json.Marshal(interp)
	if err != nil {
		return nil, "", fmt.Errorf("marshal json body: %w", err)
	}
	return bytes.NewReader(data), "application/json", nil
}

// interpolateJSON walks a decoded JSON value, interpolating ${...} in every string.
func interpolateJSON(v any, vars map[string]string, secrets SecretResolver) (any, error) {
	switch t := v.(type) {
	case string:
		return Interpolate(t, vars, secrets)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			iv, err := interpolateJSON(val, vars, secrets)
			if err != nil {
				return nil, err
			}
			out[k] = iv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			iv, err := interpolateJSON(val, vars, secrets)
			if err != nil {
				return nil, err
			}
			out[i] = iv
		}
		return out, nil
	default:
		return v, nil
	}
}

func applyExtract(e Extract, header http.Header, body []byte) (string, error) {
	switch e.From {
	case "response.json":
		return ExtractPath(body, e.Path)
	case "response.header":
		return header.Get(e.Path), nil
	case "response.body":
		return extractRegex(e.Path, body)
	default:
		return "", fmt.Errorf("unknown extract source %q", e.From)
	}
}
```

> Add `extractRegex` in `runtime.go` (first capture group of a regex over the body):
> ```go
> func extractRegex(pattern string, body []byte) (string, error) {
> 	re, err := regexp.Compile(pattern)
> 	if err != nil {
> 		return "", fmt.Errorf("compile regex: %w", err)
> 	}
> 	m := re.FindSubmatch(body)
> 	if len(m) < 2 {
> 		return "", fmt.Errorf("regex %q: no capture group match", pattern)
> 	}
> 	return string(m[1]), nil
> }
> ```
> and add `"regexp"` to the import block.

- [ ] **Step 4: Run — expect PASS** `go test ./pkg/scout/flow/ -run TestRun -v` + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/runtime.go pkg/scout/flow/runtime_test.go
git commit -m "feat(flow): deterministic REST runtime (send + extract + expect)"
```

---

## Task 6: GraphQL step support

**Files:** Modify `pkg/scout/flow/runtime.go` (`buildBody`); Test add to `pkg/scout/flow/runtime_test.go`

- [ ] **Step 1: Add the failing test**

```go
// append to runtime_test.go
func TestRunGraphQL(t *testing.T) {
	var gotOp, gotVar string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotOp = body.OperationName
		if v, ok := body.Variables["id"].(string); ok {
			gotVar = v
		}
		_, _ = w.Write([]byte(`{"data":{"cart":{"total":7}}}`))
	}))
	defer srv.Close()

	f := &FlowSpec{Version: "1", Vars: map[string]string{"cartId": "c-7"}, Steps: []FlowStep{
		{ID: "cart", Request: Request{Method: "POST", URL: srv.URL, GraphQL: &GraphQL{
			OperationName: "Cart", Query: "query Cart($id:ID!){cart(id:$id){total}}",
			Variables: map[string]any{"id": "${cartId}"}}},
			Extract: []Extract{{Var: "total", From: "response.json", Path: "$.data.cart.total"}}}}}
	res, err := Run(context.Background(), f, RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotOp != "Cart" || gotVar != "c-7" {
		t.Fatalf("graphql body wrong: op=%q id=%q", gotOp, gotVar)
	}
	if res.Steps[0].Extracted["total"] != "7" {
		t.Fatalf("extract: %+v", res.Steps[0].Extracted)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`gotOp` empty — GraphQL body not built).

- [ ] **Step 3: Extend `buildBody`** — handle GraphQL before the JSON branch:

```go
// in buildBody, at the top (before the r.JSON == nil check):
	if r.GraphQL != nil {
		vars := map[string]any{}
		for k, v := range r.GraphQL.Variables {
			iv, err := interpolateJSON(v, varsMap, secrets)
			if err != nil {
				return nil, "", err
			}
			vars[k] = iv
		}
		payload := map[string]any{"query": r.GraphQL.Query}
		if r.GraphQL.OperationName != "" {
			payload["operationName"] = r.GraphQL.OperationName
		}
		if len(vars) > 0 {
			payload["variables"] = vars
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, "", fmt.Errorf("marshal graphql body: %w", err)
		}
		return bytes.NewReader(data), "application/json", nil
	}
```

> Note: `buildBody`'s `vars` parameter is named `vars` in Task 5; rename the param to `varsMap` (and update the two `interpolateJSON(..., vars, ...)` calls already inside `buildBody` to `varsMap`) so it doesn't shadow the local `vars := map[string]any{}` above. Keep `runStep`'s call `buildBody(step.Request, vars, secrets)` unchanged.

- [ ] **Step 4: Run — expect PASS** (`TestRunGraphQL` + the Task-5 tests still green).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/runtime.go pkg/scout/flow/runtime_test.go
git commit -m "feat(flow): GraphQL step support in runtime"
```

---

## Task 7: Vault-backed secret resolution

**Files:** Create `pkg/scout/flow/secrets.go`; Test `pkg/scout/flow/secrets_test.go`

- [ ] **Step 1: Write the failing test** (uses the real vault — create a profile, resolve through the flow runtime end-to-end with `httptest`)

```go
// pkg/scout/flow/secrets_test.go
package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

func TestVaultResolverInjectsSecretHeader(t *testing.T) {
	vp := filepath.Join(t.TempDir(), "vault.bin")
	v, err := vault.Create([]byte("pw"), vault.WithPath(vp))
	if err != nil {
		t.Fatal(err)
	}
	id, err := v.Set(vault.SecretProfileInput{Name: "svc", Secrets: map[string][]byte{"API_KEY": []byte("sk-secret")}})
	if err != nil {
		t.Fatal(err)
	}
	_ = v.Close()

	v2, err := vault.Open([]byte("pw"), vault.WithPath(vp))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v2.Close() }()
	h, err := v2.Use(id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()

	var sawKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("X-API-Key")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "call", Request: Request{Method: "GET", URL: srv.URL,
			Headers: map[string]string{"X-API-Key": "${secret.API_KEY}"}}}}}
	if _, err := Run(context.Background(), f, RunOptions{Secrets: NewVaultResolver(h)}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sawKey != "sk-secret" {
		t.Fatalf("secret not injected: %q", sawKey)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: NewVaultResolver`).

- [ ] **Step 3: Implement**

```go
// pkg/scout/flow/secrets.go
package flow

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// VaultResolver adapts a vault.Handle to the SecretResolver interface so the
// runtime can resolve ${secret.NAME} at send time. The handle owns the buffers;
// the caller closes it (which zeros them) after the run.
type VaultResolver struct {
	h *vault.Handle
}

// NewVaultResolver wraps an opened vault.Handle.
func NewVaultResolver(h *vault.Handle) *VaultResolver {
	return &VaultResolver{h: h}
}

// Secret returns a COPY of the named secret's bytes. The copy is short-lived
// (consumed immediately by Interpolate into a request that is sent and discarded);
// the vault's own buffer stays locked/zeroable.
func (r *VaultResolver) Secret(name string) ([]byte, error) {
	lb, err := r.h.Secret(name)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: vault secret %q: %w", name, err)
	}
	return append([]byte(nil), lb.Bytes()...), nil
}
```

- [ ] **Step 4: Run — expect PASS** `go test ./pkg/scout/flow/ -run TestVaultResolver -v` + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/secrets.go pkg/scout/flow/secrets_test.go
git commit -m "feat(flow): vault-backed ${secret.*} resolution at run time"
```

---

## Task 8: Analyzer (staged LLM passes → FlowSpec + Report)

**Files:** Create `pkg/scout/flow/analyze.go`; Test `pkg/scout/flow/analyze_test.go`. The `llm.Provider` is injected so the test is deterministic (a stub returns canned JSON).

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/flow/analyze_test.go
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
	cap := &Capture{Version: "1", Entries: []CaptureEntry{
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
	spec, report, err := Analyze(cap, prov, AnalyzeOptions{Name: "login-flow"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if spec.Name != "login-flow" || len(spec.Steps) != 2 {
		t.Fatalf("spec wrong: %+v", spec)
	}
	// the inferred chain must produce an extract on step 0 and an injected header on step 1
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
	cap := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: "https://api.x/a", Status: 200}}}
	spec, report, err := Analyze(cap, errProvider{}, AnalyzeOptions{Name: "x"})
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

var errTest = &analyzeTestErr{}

type analyzeTestErr struct{}

func (*analyzeTestErr) Error() string { return "boom" }
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: Analyze`).

- [ ] **Step 3: Implement** — a skeleton-first design: build a deterministic skeleton spec from the capture (one step per entry, no chains), then ask the LLM (in staged passes) to (a) de-noise, (b) infer correlations, (c) name; apply the parsed results onto the skeleton. On ANY LLM/parse failure, return the skeleton with `report.Degraded=true` and everything flagged — never error.

```go
// pkg/scout/flow/analyze.go
package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/internal/engine/llm"
)

// AnalyzeOptions configures analysis.
type AnalyzeOptions struct {
	Name    string
	Timeout time.Duration // per-pass; 0 → 60s
}

// Report is the human-review surface emitted alongside the spec.
type Report struct {
	Degraded bool       `json:"degraded"` // true if the LLM failed and a raw skeleton was emitted
	Dropped  []int      `json:"dropped"`  // capture entry indexes classified as noise
	Chains   []Chain    `json:"chains"`   // inferred correlations (review these)
	Notes    []string   `json:"notes"`
}

// Chain is one inferred value correlation: extract a value from from_entry's
// response and inject it into to_entry's request.
type Chain struct {
	FromEntry  int     `json:"from_entry"`
	From       string  `json:"from"`  // response.json | response.header
	Path       string  `json:"path"`
	ToEntry    int     `json:"to_entry"`
	Into       string  `json:"into"`  // header | query | json
	Name       string  `json:"name"`  // header/field name on the target
	Template   string  `json:"template"` // e.g. "Bearer ${token}"
	Var        string  `json:"var"`
	Confidence float64 `json:"confidence"`
}

// Analyze turns a Capture into a FlowSpec + Report using staged LLM passes.
// The provider is injected (tests pass a stub). On LLM/parse failure it returns
// a deterministic skeleton with Report.Degraded=true — never an error from the
// LLM path.
func Analyze(cap *Capture, provider llm.Provider, opts AnalyzeOptions) (*FlowSpec, *Report, error) {
	if cap == nil || len(cap.Entries) == 0 {
		return nil, nil, fmt.Errorf("scout: flow: analyze: empty capture")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	spec := skeleton(cap, opts.Name)
	report := &Report{}

	// Pass 1 — classify (de-noise): which entries to keep.
	keep, dropped, ok := passClassify(provider, cap, timeout)
	if !ok {
		report.Degraded = true
		report.Notes = append(report.Notes, "LLM classify failed; emitted raw skeleton — review every step")
		return spec, report, nil
	}
	report.Dropped = dropped
	spec = skeletonKeep(cap, opts.Name, keep)

	// Pass 2 — correlate: infer value chains.
	chains, ok := passCorrelate(provider, cap, keep, timeout)
	if ok {
		report.Chains = chains
		applyChains(spec, keep, chains)
	} else {
		report.Notes = append(report.Notes, "LLM correlate failed; no chains inferred — add extract/inject manually")
	}

	// Pass 3 — name (best-effort; failure is non-fatal).
	if name, ok := passName(provider, cap, timeout); ok && name != "" {
		spec.Name = name
	}
	return spec, report, nil
}

// skeleton builds one step per capture entry with no chains.
func skeleton(cap *Capture, name string) *FlowSpec {
	idx := make([]int, len(cap.Entries))
	for i := range idx {
		idx[i] = i
	}
	return skeletonKeep(cap, name, idx)
}

func skeletonKeep(cap *Capture, name string, keep []int) *FlowSpec {
	f := &FlowSpec{Version: "1", Name: name}
	for n, ei := range keep {
		e := cap.Entries[ei]
		step := FlowStep{
			ID:      fmt.Sprintf("step%d", n+1),
			Request: Request{Method: e.Method, URL: e.URL, Headers: cloneHeaders(e.ReqHeaders)},
		}
		if e.Status != 0 {
			step.Expect = &Expect{Status: e.Status}
		}
		f.Steps = append(f.Steps, step)
	}
	return f
}

func cloneHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// applyChains maps inferred chains onto the spec: an Extract on the source step
// and a templated header/value on the target step.
func applyChains(spec *FlowSpec, keep []int, chains []Chain) {
	pos := map[int]int{} // capture-entry index → step index
	for stepIdx, entryIdx := range keep {
		pos[entryIdx] = stepIdx
	}
	for _, c := range chains {
		src, okS := pos[c.FromEntry]
		dst, okD := pos[c.ToEntry]
		if !okS || !okD {
			continue
		}
		spec.Steps[src].Extract = append(spec.Steps[src].Extract, Extract{Var: c.Var, From: c.From, Path: c.Path})
		if c.Into == "header" {
			if spec.Steps[dst].Request.Headers == nil {
				spec.Steps[dst].Request.Headers = map[string]string{}
			}
			spec.Steps[dst].Request.Headers[c.Name] = c.Template
		}
		// query/json chain injection: backlog (note in spec §9).
	}
}

const classifySys = "You classify captured HTTP entries. Drop static assets/telemetry; keep API calls. Return JSON: {\"keep\":[indexes],\"drop\":[indexes]}."
const correlateSys = "You correlate captured HTTP entries. Find values from one response reused in a later request (auth tokens, CSRF, IDs). Return JSON {\"chains\":[{from_entry,from,path,to_entry,into,name,template,var,confidence}]}."
const nameSys = "Name this API flow in kebab-case. Return JSON {\"name\":\"...\"}."

func passClassify(p llm.Provider, cap *Capture, timeout time.Duration) (keep, dropped []int, ok bool) {
	out, err := complete(p, classifySys, captureDigest(cap), timeout)
	if err != nil {
		return nil, nil, false
	}
	var r struct {
		Keep []int `json:"keep"`
		Drop []int `json:"drop"`
	}
	if json.Unmarshal([]byte(out), &r) != nil || len(r.Keep) == 0 {
		return nil, nil, false
	}
	return r.Keep, r.Drop, true
}

func passCorrelate(p llm.Provider, cap *Capture, keep []int, timeout time.Duration) ([]Chain, bool) {
	out, err := complete(p, correlateSys, captureDigest(cap), timeout)
	if err != nil {
		return nil, false
	}
	var r struct {
		Chains []Chain `json:"chains"`
	}
	if json.Unmarshal([]byte(out), &r) != nil {
		return nil, false
	}
	return r.Chains, true
}

func passName(p llm.Provider, cap *Capture, timeout time.Duration) (string, bool) {
	out, err := complete(p, nameSys, captureDigest(cap), timeout)
	if err != nil {
		return "", false
	}
	var r struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(out), &r) != nil {
		return "", false
	}
	return r.Name, true
}

func complete(p llm.Provider, sys, user string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Complete(ctx, sys, user)
}

// captureDigest renders a compact, SECRET-REDACTED view of the capture for the LLM
// (header/cookie/token VALUES are replaced with "<redacted:N>"; names + structure kept).
func captureDigest(cap *Capture) string {
	type de struct {
		I      int               `json:"i"`
		Method string            `json:"method"`
		URL    string            `json:"url"`
		Req    map[string]string `json:"req_headers,omitempty"`
		Status int               `json:"status"`
		Resp   string            `json:"resp_excerpt,omitempty"`
	}
	var ds []de
	for i, e := range cap.Entries {
		ds = append(ds, de{I: i, Method: e.Method, URL: e.URL, Req: redactHeaders(e.ReqHeaders), Status: e.Status, Resp: excerpt(e.RespBody)})
	}
	b, _ := json.Marshal(ds)
	return string(b)
}
```

> Add the two redaction helpers to `analyze.go`:
> ```go
> func redactHeaders(in map[string]string) map[string]string {
> 	if len(in) == 0 {
> 		return nil
> 	}
> 	out := make(map[string]string, len(in))
> 	for k, v := range in {
> 		if len(v) > 8 {
> 			out[k] = fmt.Sprintf("<redacted:%d>", len(v))
> 		} else {
> 			out[k] = v
> 		}
> 	}
> 	return out
> }
> func excerpt(s string) string {
> 	const max = 400
> 	if len(s) > max {
> 		return s[:max]
> 	}
> 	return s
> }
> ```

- [ ] **Step 4: Run — expect PASS** `go test ./pkg/scout/flow/ -run TestAnalyze -v` + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/analyze.go pkg/scout/flow/analyze_test.go
git commit -m "feat(flow): staged-LLM analyzer (skeleton + classify/correlate/name; degrades safely)"
```

---

## Task 9: Verify (re-run + diff vs golden capture)

**Files:** Create `pkg/scout/flow/verify.go`; Test `pkg/scout/flow/verify_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/flow/verify_test.go
package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyParity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "a", Request: Request{Method: "GET", URL: srv.URL}}}}
	golden := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: srv.URL, Status: 200}}}

	rep, err := Verify(context.Background(), f, golden, RunOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !rep.OK || len(rep.Steps) != 1 || !rep.Steps[0].StatusMatch {
		t.Fatalf("expected parity: %+v", rep)
	}
}

func TestVerifyDetectsDrift(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	f := &FlowSpec{Version: "1", Steps: []FlowStep{{ID: "a", Request: Request{Method: "GET", URL: srv.URL}}}}
	golden := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: srv.URL, Status: 200}}}
	rep, err := Verify(context.Background(), f, golden, RunOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if rep.OK || rep.Steps[0].StatusMatch {
		t.Fatalf("expected drift detected: %+v", rep)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: Verify`).

- [ ] **Step 3: Implement** — Verify runs the flow with `Expect` cleared (so a status mismatch doesn't abort), then diffs each step's actual status against the golden entry's status.

```go
// pkg/scout/flow/verify.go
package flow

import (
	"context"
	"fmt"
)

// VerifyReport is the parity result of re-running a flow vs a golden capture.
type VerifyReport struct {
	OK    bool
	Steps []StepDiff
}

// StepDiff compares one replayed step to its golden entry.
type StepDiff struct {
	ID             string
	ExpectedStatus int
	ActualStatus   int
	StatusMatch    bool
}

// Verify re-runs f and diffs each step's status against golden.Entries by index.
// Expect assertions are ignored during verify (we want to observe drift, not abort).
func Verify(ctx context.Context, f *FlowSpec, golden *Capture, opts RunOptions) (*VerifyReport, error) {
	// Clone f with Expect cleared so a mismatch doesn't stop the run.
	clone := *f
	clone.Steps = make([]FlowStep, len(f.Steps))
	for i, s := range f.Steps {
		s.Expect = nil
		clone.Steps[i] = s
	}
	res, err := Run(ctx, &clone, opts)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: verify: %w", err)
	}

	rep := &VerifyReport{OK: true}
	for i, sr := range res.Steps {
		var exp int
		if i < len(golden.Entries) {
			exp = golden.Entries[i].Status
		}
		match := exp == 0 || exp == sr.Status
		if !match {
			rep.OK = false
		}
		rep.Steps = append(rep.Steps, StepDiff{ID: sr.ID, ExpectedStatus: exp, ActualStatus: sr.Status, StatusMatch: match})
	}
	return rep, nil
}
```

- [ ] **Step 4: Run — expect PASS** `go test ./pkg/scout/flow/ -run TestVerify -v` + `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/verify.go pkg/scout/flow/verify_test.go
git commit -m "feat(flow): verify (re-run + status parity vs golden capture)"
```

---

## Task 10: Browser capture driver

**Files:** Create `pkg/scout/flow/capture.go`; Test `pkg/scout/flow/capture_integration_test.go` (build-tagged)

> **VERIFY-BEFORE-CODING:** the exact hijack option names. The fact-gather shows `Page.NewSessionHijacker(opts ...HijackOption) (*SessionHijacker, error)`, `Events() <-chan HijackEvent`, `Stop()`, and event types `hijack.CapturedRequest{Method,URL,Headers,Body}` / `hijack.CapturedResponse{Status,Headers,Body,MimeType}` with a discriminated `Event{Type, Request, Response, Frame}`. Confirm the engine-level option names with `grep -n "func WithHijack\|func WithSessionHijack\|NewSessionHijacker" internal/engine/*.go` and use `WithSessionHijack`/body-capture as available. The `scout.Page` facade aliases `engine.Page`, so `page.NewSessionHijacker(...)` is reachable.

- [ ] **Step 1: Write the failing integration test**

```go
//go:build integration

// pkg/scout/flow/capture_integration_test.go
package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func newCaptureBrowser(t *testing.T) *scout.Browser {
	t.Helper()
	b, err := scout.New()
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func TestCaptureFlowRecordsAPICall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><script>fetch('/api/data')</script></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := newCaptureBrowser(t)
	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = page.Close() }()

	cap, err := CaptureFlow(context.Background(), page, CaptureOptions{
		URL: srv.URL, Name: "t", URLFilter: []string{"*api*"},
	})
	if err != nil {
		t.Fatalf("CaptureFlow: %v", err)
	}
	found := false
	for _, e := range cap.Entries {
		if e.URL == srv.URL+"/api/data" && e.RespBody != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("api call not captured: %+v", cap.Entries)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** `go test -tags integration ./pkg/scout/flow/ -run TestCaptureFlow -v` (`undefined: CaptureFlow`).

- [ ] **Step 3: Implement** — drive the hijacker, navigate, drain events for a settle window, correlate request+response by RequestID into `CaptureEntry`s.

```go
// pkg/scout/flow/capture.go
package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// CaptureOptions configures a capture.
type CaptureOptions struct {
	URL       string        // page to open
	Name      string        // capture name
	URLFilter []string      // glob patterns for which URLs to keep (e.g. "*api*")
	Settle    time.Duration // how long to keep recording after load; 0 → 3s
}

// CaptureFlow opens URL in page, records HTTP traffic via the session hijacker,
// and returns a normalized Capture. Body capture is enabled so responses are
// preserved for analysis.
func CaptureFlow(ctx context.Context, page *scout.Page, opts CaptureOptions) (*Capture, error) {
	settle := opts.Settle
	if settle == 0 {
		settle = 3 * time.Second
	}

	// VERIFY the option names against internal/engine (see VERIFY-BEFORE-CODING note);
	// use the body-capture + URL-filter hijack options the engine exposes.
	hj, err := page.NewSessionHijacker(hijackOpts(opts.URLFilter)...)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: capture: start hijack: %w", err)
	}
	defer hj.Stop()

	type pair struct {
		req *CaptureEntry
	}
	pending := map[string]*CaptureEntry{}
	order := []string{}

	done := make(chan struct{})
	go func() {
		for ev := range hj.Events() {
			switch {
			case ev.Request != nil:
				ce := &CaptureEntry{Method: ev.Request.Method, URL: ev.Request.URL,
					ReqHeaders: ev.Request.Headers, ReqBody: ev.Request.Body}
				if _, ok := pending[ev.Request.RequestID]; !ok {
					order = append(order, ev.Request.RequestID)
				}
				pending[ev.Request.RequestID] = ce
			case ev.Response != nil:
				if ce, ok := pending[ev.Response.RequestID]; ok {
					ce.Status = ev.Response.Status
					ce.RespHeaders = ev.Response.Headers
					ce.RespBody = ev.Response.Body
					ce.MimeType = ev.Response.MimeType
				}
			}
		}
		close(done)
	}()

	if err := page.Navigate(opts.URL); err != nil {
		return nil, fmt.Errorf("scout: flow: capture: navigate: %w", err)
	}
	_ = page.WaitLoad()
	select {
	case <-time.After(settle):
	case <-ctx.Done():
	}
	hj.Stop()
	<-done

	cap := &Capture{Version: "1", Name: opts.Name}
	for _, id := range order {
		if ce := pending[id]; ce != nil {
			cap.Entries = append(cap.Entries, *ce)
		}
	}
	return cap, nil
}
```

> Implement `hijackOpts(filters []string) []scout.HijackOption` (or the engine's option type) in `capture.go` per the VERIFY note — return the body-capture option plus a URL filter for each pattern. If the engine option type isn't re-exported on the `scout` facade, import `github.com/inovacc/scout/internal/engine` (same module) and use `engine.WithHijack*` directly, taking `page.RodPage()`-level access only if required. Confirm `ev.Request`/`ev.Response` field access matches the `HijackEvent` alias of `hijack.Event`.

- [ ] **Step 4: Run — expect PASS (or SKIP without Chromium)** `go test -tags integration ./pkg/scout/flow/ -run TestCaptureFlow -v`. Also confirm the non-integration suite still builds: `go build ./pkg/scout/flow/` and `go vet ./pkg/scout/flow/`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/capture.go pkg/scout/flow/capture_integration_test.go
git commit -m "feat(flow): browser capture driver (hijack -> normalized Capture)"
```

---

## Task 11: CLI — `scout flow capture|analyze|run|verify`

**Files:** Create `cmd/scout/flow.go`; Test `cmd/scout/flow_test.go`

> Mirror `cmd/scout/strategy.go` registration. Use `baseOpts(cmd)` for the capture browser, `readPassphraseBytes` + `SCOUT_VAULT_PASSPHRASE` for `run --profile`. The LLM provider for `analyze` is selected via a `--llm` flag → construct `internal/engine/llm` provider (mirror how `cmd/scout/llm.go` builds one — grep `NewOllamaProvider|NewOpenAIProvider|NewAnthropicProvider` in `cmd/scout/`).

- [ ] **Step 1: Write the failing test** (pin the two pure helpers the CLI owns)

```go
// cmd/scout/flow_test.go
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
```

- [ ] **Step 2: Run — expect FAIL** `go test ./cmd/scout/ -run 'TestRenderFlow|TestRenderVerify' -v` (`undefined: renderFlowReport`).

- [ ] **Step 3: Implement `cmd/scout/flow.go`**

```go
// cmd/scout/flow.go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/flow"
	"github.com/inovacc/scout/pkg/scout/vault"
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{Use: "flow", Short: "Capture a browser flow, analyze it to a spec, and replay it without a browser"}

var flowCaptureCmd = &cobra.Command{
	Use:   "capture <url>",
	Short: "Record a flow's network traffic into a capture.json",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		filter, _ := cmd.Flags().GetStringSlice("filter")
		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()
		page, err := b.NewPage("about:blank")
		if err != nil {
			return err
		}
		cap, err := flow.CaptureFlow(context.Background(), page, flow.CaptureOptions{URL: args[0], Name: "capture", URLFilter: filter})
		if err != nil {
			return err
		}
		if out == "" {
			out = "capture.json"
		}
		if err := flow.SaveCapture(out, cap); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "captured %d entries -> %s\n", len(cap.Entries), out)
		return nil
	},
}

var flowAnalyzeCmd = &cobra.Command{
	Use:   "analyze <capture.json>",
	Short: "Turn a capture into a reviewed flow.yaml + report (LLM)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		name, _ := cmd.Flags().GetString("name")
		cap, err := flow.LoadCapture(args[0])
		if err != nil {
			return err
		}
		provider, err := flowProvider(cmd) // builds an internal/engine/llm provider from --llm
		if err != nil {
			return err
		}
		spec, report, err := flow.Analyze(cap, provider, flow.AnalyzeOptions{Name: name})
		if err != nil {
			return err
		}
		if out == "" {
			out = "flow.yaml"
		}
		if err := writeFlowSpec(out, spec); err != nil { // marshal yaml + write 0o644
			return err
		}
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), renderFlowReport(report))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (review it, then: scout flow run %s)\n", out, out)
		return nil
	},
}

var flowRunCmd = &cobra.Command{
	Use:   "run <flow.yaml>",
	Short: "Replay a flow deterministically (no browser)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile, _ := cmd.Flags().GetString("profile")
		f, err := flow.LoadFile(args[0])
		if err != nil {
			return err
		}
		if err := flow.Validate(f); err != nil {
			return err
		}
		opts := flow.RunOptions{}
		if profile == "" && f.Auth != nil {
			profile = f.Auth.Profile
		}
		if profile != "" {
			pass, perr := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
			if perr != nil {
				return perr
			}
			defer zeroBytesCLI(pass)
			v, verr := vault.Open(pass)
			if verr != nil {
				return verr
			}
			defer func() { _ = v.Close() }()
			h, herr := v.Use(profile)
			if herr != nil {
				return herr
			}
			defer func() { _ = h.Close() }()
			opts.Secrets = flow.NewVaultResolver(h)
		}
		res, err := flow.Run(context.Background(), f, opts)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ran %d steps\n", len(res.Steps))
		return nil
	},
}

var flowVerifyCmd = &cobra.Command{
	Use:   "verify <flow.yaml> --golden <capture.json>",
	Short: "Re-run a flow and diff status against the golden capture",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		goldenPath, _ := cmd.Flags().GetString("golden")
		if goldenPath == "" {
			return fmt.Errorf("scout: flow: --golden <capture.json> is required")
		}
		f, err := flow.LoadFile(args[0])
		if err != nil {
			return err
		}
		golden, err := flow.LoadCapture(goldenPath)
		if err != nil {
			return err
		}
		rep, err := flow.Verify(context.Background(), f, golden, flow.RunOptions{})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), renderVerifyReport(rep))
		if !rep.OK {
			return fmt.Errorf("scout: flow: verify: parity drift detected")
		}
		return nil
	},
}

// renderFlowReport formats the analysis report. MUST NOT print secret values
// (Report carries none — only indexes, var names, paths, confidence).
func renderFlowReport(r *flow.Report) string {
	var sb strings.Builder
	if r.Degraded {
		sb.WriteString("[degraded] LLM analysis failed — review EVERY step.\n")
	}
	if len(r.Dropped) > 0 {
		fmt.Fprintf(&sb, "dropped entries: %v\n", r.Dropped)
	}
	for _, c := range r.Chains {
		fmt.Fprintf(&sb, "chain: %s %s -> ${%s} into %s/%s (conf %.2g)\n", c.From, c.Path, c.Var, c.Into, c.Name, c.Confidence)
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&sb, "note: %s\n", n)
	}
	return sb.String()
}

func renderVerifyReport(r *flow.VerifyReport) string {
	var sb strings.Builder
	for _, s := range r.Steps {
		status := "OK"
		if !s.StatusMatch {
			status = "DRIFT"
		}
		fmt.Fprintf(&sb, "%-6s %s: expected %d got %d\n", status, s.ID, s.ExpectedStatus, s.ActualStatus)
	}
	return sb.String()
}

func init() {
	flowCaptureCmd.Flags().StringP("out", "o", "", "output capture path (default capture.json)")
	flowCaptureCmd.Flags().StringSlice("filter", []string{"*api*", "*graphql*"}, "URL glob filters to keep")
	flowAnalyzeCmd.Flags().StringP("out", "o", "", "output flow spec path (default flow.yaml)")
	flowAnalyzeCmd.Flags().String("name", "captured-flow", "name for the generated flow")
	flowAnalyzeCmd.Flags().String("llm", "", "LLM provider (ollama|openai|anthropic; default from env)")
	flowRunCmd.Flags().String("profile", "", "vault profile id for auth (default: spec auth.profile)")
	flowVerifyCmd.Flags().String("golden", "", "golden capture.json to diff against")
	flowCmd.AddCommand(flowCaptureCmd, flowAnalyzeCmd, flowRunCmd, flowVerifyCmd)
	rootCmd.AddCommand(flowCmd)
}
```

> Implement the two CLI helpers referenced above:
> - `writeFlowSpec(path string, f *flow.FlowSpec) error` — `yaml.Marshal(f)` (import `gopkg.in/yaml.v3`) then `os.WriteFile(path, data, 0o644)`, wrapped `scout: flow: write spec: %w`.
> - `flowProvider(cmd *cobra.Command) (llm.Provider, error)` — read `--llm`; construct the matching `internal/engine/llm` provider (grep `cmd/scout/llm.go` for the existing construction + env handling and reuse it; default to the env-configured provider). Import `github.com/inovacc/scout/internal/engine/llm`.
> `zeroBytesCLI` already exists (added by the vault CLI).

- [ ] **Step 4: Run — expect PASS + build** `go test ./cmd/scout/ -run 'TestRenderFlow|TestRenderVerify' -v` and `go build ./cmd/scout/`.

- [ ] **Step 5: Commit**

```bash
git add cmd/scout/flow.go cmd/scout/flow_test.go
git commit -m "feat(flow): scout flow CLI (capture/analyze/run/verify)"
```

---

## Task 12: Secrets-hygiene guard + docs

**Files:** Create `pkg/scout/flow/hygiene_test.go`; Modify `CLAUDE.md`, `README.md`, `docs/BACKLOG.md`

- [ ] **Step 1: Write the hygiene test** — a `FlowSpec` round-trip must never serialize a raw secret value (only `${secret.*}` refs + `auth.profile`).

```go
// pkg/scout/flow/hygiene_test.go
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
	// A real secret value must never appear — these placeholders are the only auth surface.
	for _, leak := range []string{"hunter2", "sk-live", "Bearer eyJ"} {
		if strings.Contains(s, leak) {
			t.Fatalf("spec leaked a secret-like value: %q", leak)
		}
	}
}
```

- [ ] **Step 2: Run — expect PASS** `go test ./pkg/scout/flow/ -run TestFlowSpecNever -v` (it passes immediately — it documents + guards the contract).

- [ ] **Step 3: Run the full gate**

Run: `go test ./pkg/scout/flow/... ./cmd/scout/ -v` (browser capture test skips without `-tags integration`/Chromium) — all PASS.
Run: `go vet ./pkg/scout/flow/... ./cmd/scout/` — clean.
Run: `golangci-lint run ./pkg/scout/flow/... --timeout=5m` — 0 issues (fix any modernize/unparam nits inline, mirroring the vault feature).

- [ ] **Step 4: Docs**

Add a `CLAUDE.md` Conventions bullet:
```markdown
- **Flow capture→replay**: `pkg/scout/flow` records a browser flow (`scout flow capture`), an LLM analyzer turns it into a reviewed `flow.yaml` (`scout flow analyze`), and a deterministic runtime replays the REST/GraphQL calls with no browser (`scout flow run`), chaining values via `${var}`/`${secret.NAME}` (secrets resolved from a `pkg/scout/vault` profile at send time, never embedded). `scout flow verify --golden` diffs status parity. The reviewed spec is the deterministic contract; the LLM runs only at analyze time.
```

Add a `README.md` `### Flow capture → replay` section with the `scout flow capture/analyze/run/verify` example.

Append to `docs/BACKLOG.md`:
```markdown
- FLOW v2: WebSocket replay, multipart/file-upload, SSE/streaming bodies, query/json chain-injection (analyzer currently emits header chains only), a fully-automatic no-review analyze mode, and `scout flow export --go` (standalone Go module emitter).
```

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/hygiene_test.go CLAUDE.md README.md docs/BACKLOG.md
git commit -m "test(flow): secret-embedding hygiene guard; docs + backlog"
```

---

## Final verification (after all tasks)

- [ ] `go build ./cmd/scout/ ./pkg/...` — whole tree builds.
- [ ] `go test ./pkg/scout/flow/... ./cmd/scout/ -v` — green (capture integration test skips without Chromium / `-tags integration`).
- [ ] `go test -tags integration ./pkg/scout/flow/ -run TestCaptureFlow -v` — passes with Chromium (or skips cleanly).
- [ ] `go vet ./pkg/scout/flow/... ./cmd/scout/` + `golangci-lint run ./pkg/scout/flow/...` — clean.
- [ ] **Manual smoke:** `scout flow capture https://httpbin.org/json -o cap.json` → `scout flow analyze cap.json -o flow.yaml` (review report) → `scout flow run flow.yaml` → `scout flow verify flow.yaml --golden cap.json`.
- [ ] Confirm a generated `flow.yaml` contains only `${secret.*}` refs + `auth.profile` — no raw secret values.
- [ ] Use `superpowers:finishing-a-development-branch`.

---

## Spec coverage check (self-review)

| Spec requirement (design §) | Task(s) |
|---|---|
| §2 scout-hosted job artifact, declarative spec, AI analyzer, REST+GraphQL, vault auth | 4,5,6,7,8 |
| §3 capture→analyze→run→verify pipeline | 8,9,10,11 |
| §4 components (`pkg/scout/flow/*` + `cmd/scout/flow.go`) | all |
| §5 FlowSpec schema (extract/inject, `${secret.*}`/`${var}`, GraphQL, auth.profile) | 3,4,6 |
| §6 4-verb CLI | 11 |
| §7 error handling (analyze degrades safely; per-step wrap; secrets zeroed) | 7,8 |
| §8 testing (httptest runtime, stubbed-provider analyze, parity verify, browser capture, secret-hygiene) | 5–12 |
| §9 scope (REST+GraphQL; WS/multipart/SSE backlog) | 6,12 |
| §10 security (secret-free specs, redacted analysis digest, vault buffers zeroed) | 7,8,12 |
| §11 success criteria | 5–11, final smoke |

**Scope/format deviation from spec (flagged):** the spec called the golden a "HAR"; this plan uses the flow's normalized **`capture.json`** as both the analyze input and the verify golden, because `internal/engine/hijack` has an HAR *exporter* but no reader. HAR export remains available via the engine for interop; adding a HAR reader + `--golden file.har` is a small backlog follow-up. Also: the analyzer v1 emits **header** chains (auth/CSRF); query/json chain-injection is in the FLOW-v2 backlog.
