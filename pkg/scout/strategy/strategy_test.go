package strategy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
)

func TestParse_YAML(t *testing.T) {
	input := `
name: test-strategy
version: "1.0"
browser:
  type: brave
  stealth: true
  headless: false
steps:
  - name: scrape-data
    mode: slack
    targets: [general]
    limit: 10
output:
  sinks:
    - type: json-file
      path: ./out.json
`
	s, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if s.Name != "test-strategy" {
		t.Errorf("Name = %q, want test-strategy", s.Name)
	}

	if s.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0", s.Version)
	}

	if s.Browser.Type != "brave" {
		t.Errorf("Browser.Type = %q, want brave", s.Browser.Type)
	}

	if s.Browser.Stealth != true {
		t.Error("Browser.Stealth = false, want true")
	}

	if s.Browser.IsHeadless() {
		t.Error("Browser.IsHeadless() = true, want false")
	}

	if len(s.Steps) != 1 {
		t.Fatalf("Steps count = %d, want 1", len(s.Steps))
	}

	if s.Steps[0].Mode != "slack" {
		t.Errorf("Step mode = %q, want slack", s.Steps[0].Mode)
	}

	if s.Steps[0].Limit != 10 {
		t.Errorf("Step limit = %d, want 10", s.Steps[0].Limit)
	}

	if len(s.Output.Sinks) != 1 || s.Output.Sinks[0].Type != "json-file" {
		t.Error("Output sink not parsed correctly")
	}
}

func TestParse_JSON(t *testing.T) {
	input := `{
		"name": "json-strategy",
		"version": "1.0",
		"browser": {"type": "chrome"},
		"steps": [{"name": "step1", "mode": "gmaps", "targets": ["pizza"]}],
		"output": {"sinks": [{"type": "ndjson", "path": "./out.ndjson"}]}
	}`

	s, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse JSON: %v", err)
	}

	if s.Name != "json-strategy" {
		t.Errorf("Name = %q, want json-strategy", s.Name)
	}
}

func TestParse_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_PROXY", "socks5://proxy:1080")

	input := `
name: env-test
version: "1.0"
browser:
  proxy: ${TEST_PROXY}
steps:
  - name: step1
    mode: slack
output:
  sinks:
    - type: ndjson
      path: ./out.ndjson
`
	s, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if s.Browser.Proxy != "socks5://proxy:1080" {
		t.Errorf("Proxy = %q, want socks5://proxy:1080", s.Browser.Proxy)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	content := `
name: file-test
version: "1.0"
steps:
  - name: s1
    mode: slack
output:
  sinks:
    - type: json-file
      path: ./out.json
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if s.Name != "file-test" {
		t.Errorf("Name = %q, want file-test", s.Name)
	}
}

func TestValidate_Valid(t *testing.T) {
	s := &Strategy{
		Name:    "valid",
		Version: "1.0",
		Steps:   []Step{{Name: "s1", Mode: "slack"}},
		Output:  OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "./out.ndjson"}}},
	}

	if err := Validate(s); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name string
		s    *Strategy
		want string
	}{
		{
			name: "empty name",
			s:    &Strategy{Version: "1.0", Steps: []Step{{Name: "s", Mode: "x"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "name is required",
		},
		{
			name: "no steps",
			s:    &Strategy{Name: "x", Version: "1.0", Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "at least one step",
		},
		{
			name: "no sinks",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Name: "s", Mode: "x"}}, Output: OutputConfig{}},
			want: "at least one sink",
		},
		{
			name: "step missing name",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Mode: "x"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "name is required",
		},
		{
			name: "step missing mode and url",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Name: "s"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "either mode or url",
		},
		{
			name: "step both mode and url",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Name: "s", Mode: "x", URL: "http://x"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "cannot specify both",
		},
		{
			name: "duplicate step names",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Name: "s", Mode: "a"}, {Name: "s", Mode: "b"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "duplicate step name",
		},
		{
			name: "invalid step timeout",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Name: "s", Mode: "x", Timeout: "xyz"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "invalid timeout",
		},
		{
			name: "json-file sink without path",
			s:    &Strategy{Name: "x", Version: "1.0", Steps: []Step{{Name: "s", Mode: "x"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "json-file"}}}},
			want: "path is required",
		},
		{
			name: "auth without provider",
			s:    &Strategy{Name: "x", Version: "1.0", Auth: &AuthConfig{}, Steps: []Step{{Name: "s", Mode: "x"}}, Output: OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: "x"}}}},
			want: "provider is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.s)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if got := err.Error(); !contains(got, tt.want) {
				t.Errorf("error = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestParseTimeout(t *testing.T) {
	if d := ParseTimeout("5m", time.Minute); d != 5*time.Minute {
		t.Errorf("ParseTimeout(5m) = %v, want 5m", d)
	}

	if d := ParseTimeout("", 3*time.Second); d != 3*time.Second {
		t.Errorf("ParseTimeout('') = %v, want 3s", d)
	}

	if d := ParseTimeout("invalid", 7*time.Second); d != 7*time.Second {
		t.Errorf("ParseTimeout(invalid) = %v, want 7s", d)
	}
}

func TestBrowserConfig_IsHeadless(t *testing.T) {
	// nil → default true
	if b := (BrowserConfig{}); !b.IsHeadless() {
		t.Error("nil headless should default to true")
	}

	f := false
	if b := (BrowserConfig{Headless: &f}); b.IsHeadless() {
		t.Error("explicit false should be false")
	}

	tr := true
	if b := (BrowserConfig{Headless: &tr}); !b.IsHeadless() {
		t.Error("explicit true should be true")
	}
}

func TestStrategy_String(t *testing.T) {
	s := &Strategy{
		Name:  "my-strat",
		Steps: []Step{{Name: "a"}, {Name: "b"}},
	}

	got := s.String()
	if !contains(got, "my-strat") || !contains(got, "a, b") {
		t.Errorf("String() = %q, unexpected", got)
	}
}

func TestEvaluateWhen(t *testing.T) {
	// No conditions → always true.
	if !evaluateWhen(nil, nil) {
		t.Error("nil conditions should return true")
	}

	// has_auth=true with no session → false.
	if evaluateWhen(map[string]any{"has_auth": true}, nil) {
		t.Error("has_auth=true without session should be false")
	}

	// env condition with unset var → false.
	if evaluateWhen(map[string]any{"env": "NONEXISTENT_VAR_12345"}, nil) {
		t.Error("env condition with unset var should be false")
	}

	// env condition with set var → true.
	t.Setenv("TEST_COND", "1")
	if !evaluateWhen(map[string]any{"env": "TEST_COND"}, nil) {
		t.Error("env condition with set var should be true")
	}
}

func TestSinks(t *testing.T) {
	dir := t.TempDir()

	result := scraper.Result{
		Type:      scraper.ResultMessage,
		Source:    "test",
		ID:        "1",
		Timestamp: time.Now(),
		Author:    "user",
		Content:   "hello",
		URL:       "https://example.com",
	}

	tests := []struct {
		name string
		cfg  SinkConfig
	}{
		{"json-file", SinkConfig{Type: "json-file", Path: filepath.Join(dir, "out.json")}},
		{"ndjson", SinkConfig{Type: "ndjson", Path: filepath.Join(dir, "out.ndjson")}},
		{"csv", SinkConfig{Type: "csv", Path: filepath.Join(dir, "out.csv")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink, err := NewSink(tt.cfg)
			if err != nil {
				t.Fatalf("NewSink: %v", err)
			}

			if err := sink.Write(result); err != nil {
				t.Fatalf("Write: %v", err)
			}

			if err := sink.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			data, err := os.ReadFile(tt.cfg.Path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			if len(data) == 0 {
				t.Error("output file is empty")
			}
		})
	}
}

func TestNewSink_UnknownType(t *testing.T) {
	_, err := NewSink(SinkConfig{Type: "unknown"})
	if err == nil {
		t.Error("expected error for unknown sink type")
	}
}

func TestExecute_DryRun(t *testing.T) {
	s := &Strategy{
		Name:    "dry",
		Version: "1.0",
		Steps:   []Step{{Name: "s1", Mode: "test"}},
		Output:  OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: filepath.Join(t.TempDir(), "x.ndjson")}}},
	}

	err := Execute(context.Background(), s, ExecuteOptions{DryRun: true})
	if err != nil {
		t.Errorf("DryRun: %v", err)
	}
}

func TestExecute_CancelledContext(t *testing.T) {
	s := &Strategy{
		Name:    "cancel",
		Version: "1.0",
		Steps:   []Step{{Name: "s1", Mode: "test"}},
		Output:  OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: filepath.Join(t.TempDir(), "x.ndjson")}}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Execute(ctx, s, ExecuteOptions{
		ModeResolver: func(name string) (scraper.Mode, error) {
			return nil, nil
		},
	})

	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestExecute_ValidationFailure(t *testing.T) {
	s := &Strategy{} // Missing everything.

	err := Execute(context.Background(), s, ExecuteOptions{})
	if err == nil {
		t.Error("expected validation error")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
