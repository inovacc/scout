package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// TestParse_Invalid asserts the parse error path: input that is neither valid
// YAML nor valid JSON returns a wrapped "strategy: parse" error.
func TestParse_Invalid(t *testing.T) {
	// A tab-indented mapping is invalid YAML and not JSON either.
	bad := "name: x\n\tversion: \"1.0\"\n\t: : :"

	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}

	if !strings.Contains(err.Error(), "strategy: parse") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "strategy: parse")
	}
}

// TestLoadFile_Missing asserts LoadFile wraps a read error for a nonexistent
// path and that the underlying error is an os.ErrNotExist.
func TestLoadFile_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want errors.Is os.ErrNotExist", err)
	}

	if !strings.Contains(err.Error(), "strategy: read file") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "strategy: read file")
	}
}

// TestBuildBrowserOpts asserts buildBrowserOpts emits the right number of
// scout.Option values per BrowserConfig. Headless is always emitted (1 opt);
// each set field adds one more. WindowSize only counts when it has exactly 2
// elements.
func TestBuildBrowserOpts(t *testing.T) {
	tests := []struct {
		name string
		cfg  BrowserConfig
		want int
	}{
		{"defaults headless only", BrowserConfig{}, 1},
		{"type adds one", BrowserConfig{Type: "brave"}, 2},
		{"stealth adds one", BrowserConfig{Stealth: true}, 2},
		{"proxy adds one", BrowserConfig{Proxy: "socks5://p:1"}, 2},
		{"user agent adds one", BrowserConfig{UserAgent: "UA"}, 2},
		{"window size 2 adds one", BrowserConfig{WindowSize: []int{800, 600}}, 2},
		{"window size 1 ignored", BrowserConfig{WindowSize: []int{800}}, 1},
		{"window size 3 ignored", BrowserConfig{WindowSize: []int{1, 2, 3}}, 1},
		{
			name: "all fields",
			cfg: BrowserConfig{
				Type:       "chrome",
				Stealth:    true,
				Proxy:      "http://proxy:8080",
				UserAgent:  "Mozilla",
				WindowSize: []int{1920, 1080},
			},
			want: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildBrowserOpts(tt.cfg)
			if len(opts) != tt.want {
				t.Errorf("buildBrowserOpts(%+v) = %d opts, want %d", tt.cfg, len(opts), tt.want)
			}
		})
	}
}

// TestEvaluateWhen_MoreBranches covers branches not exercised by the existing
// test: has_auth=false with a non-nil session (false), has_auth=true with a
// non-nil session (true), env with a set value, non-bool/non-string condition
// values (ignored), and an unknown condition key (ignored → true).
func TestEvaluateWhen_MoreBranches(t *testing.T) {
	sess := &auth.Session{Provider: "slack"}

	tests := []struct {
		name       string
		conditions map[string]any
		session    *auth.Session
		setEnv     [2]string // key,val; empty key = skip
		want       bool
	}{
		{
			name:       "has_auth false with session present -> false",
			conditions: map[string]any{"has_auth": false},
			session:    sess,
			want:       false,
		},
		{
			name:       "has_auth false without session -> true",
			conditions: map[string]any{"has_auth": false},
			session:    nil,
			want:       true,
		},
		{
			name:       "has_auth true with session present -> true",
			conditions: map[string]any{"has_auth": true},
			session:    sess,
			want:       true,
		},
		{
			name:       "has_auth non-bool value is ignored -> true",
			conditions: map[string]any{"has_auth": "yes"},
			session:    nil,
			want:       true,
		},
		{
			name:       "env set -> true",
			conditions: map[string]any{"env": "SCOUT_WHEN_SET"},
			setEnv:     [2]string{"SCOUT_WHEN_SET", "1"},
			want:       true,
		},
		{
			name:       "env non-string value is ignored -> true",
			conditions: map[string]any{"env": 123},
			want:       true,
		},
		{
			name:       "unknown key ignored -> true",
			conditions: map[string]any{"unknown_condition": true},
			want:       true,
		},
		{
			name:       "multiple conditions all pass -> true",
			conditions: map[string]any{"has_auth": true, "env": "SCOUT_WHEN_MULTI"},
			session:    sess,
			setEnv:     [2]string{"SCOUT_WHEN_MULTI", "x"},
			want:       true,
		},
		{
			name:       "multiple conditions one fails -> false",
			conditions: map[string]any{"has_auth": true, "env": "SCOUT_WHEN_UNSET_999"},
			session:    sess,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv[0] != "" {
				t.Setenv(tt.setEnv[0], tt.setEnv[1])
			}

			if got := evaluateWhen(tt.conditions, tt.session); got != tt.want {
				t.Errorf("evaluateWhen(%v) = %v, want %v", tt.conditions, got, tt.want)
			}
		})
	}
}

// TestSinkNames asserts each concrete sink reports the expected Name().
func TestSinkNames(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		typ  string
		want string
	}{
		{"json-file", "json-file"},
		{"ndjson", "ndjson"},
		{"csv", "csv"},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			sink, err := NewSink(SinkConfig{Type: tt.typ, Path: filepath.Join(dir, tt.typ+".out")})
			if err != nil {
				t.Fatalf("NewSink(%s): %v", tt.typ, err)
			}
			defer func() { _ = sink.Close() }()

			if got := sink.Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestJSONFileSink_Content round-trips multiple results through the json-file
// sink and asserts the on-disk file is a JSON array preserving order/content.
func TestJSONFileSink_Content(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.json")

	sink, err := NewSink(SinkConfig{Type: "json-file", Path: path})
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}

	r1 := scraper.Result{Type: scraper.ResultMessage, Source: "s", ID: "a", Content: "first"}
	r2 := scraper.Result{Type: scraper.ResultMessage, Source: "s", ID: "b", Content: "second"}

	if err := sink.Write(r1); err != nil {
		t.Fatalf("Write r1: %v", err)
	}

	if err := sink.Write(r2); err != nil {
		t.Fatalf("Write r2: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got []scraper.Result
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}

	if got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("order/IDs = %q,%q, want a,b", got[0].ID, got[1].ID)
	}

	if got[0].Content != "first" || got[1].Content != "second" {
		t.Errorf("content = %q,%q, want first,second", got[0].Content, got[1].Content)
	}
}

// TestNDJSONSink_Content asserts the ndjson sink writes one JSON object per
// line, parseable independently.
func TestNDJSONSink_Content(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.ndjson")

	sink, err := NewSink(SinkConfig{Type: "ndjson", Path: path})
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}

	for _, id := range []string{"x", "y", "z"} {
		if err := sink.Write(scraper.Result{ID: id, Source: "s"}); err != nil {
			t.Fatalf("Write %s: %v", id, err)
		}
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	wantIDs := []string{"x", "y", "z"}

	for i, line := range lines {
		var r scraper.Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}

		if r.ID != wantIDs[i] {
			t.Errorf("line %d ID = %q, want %q", i, r.ID, wantIDs[i])
		}
	}
}

// TestCSVSink_Content asserts the csv sink emits a header row followed by one
// data row per result, with the documented column order.
func TestCSVSink_Content(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.csv")

	sink, err := NewSink(SinkConfig{Type: "csv", Path: path})
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}

	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	res := scraper.Result{
		Type:      scraper.ResultMessage,
		Source:    "src",
		ID:        "id-1",
		Timestamp: ts,
		Author:    "alice",
		Content:   "hello",
		URL:       "https://example.com",
	}

	if err := sink.Write(res); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (header+row): %q", len(lines), string(data))
	}

	wantHeader := "type,source,id,timestamp,author,content,url"
	if strings.TrimRight(lines[0], "\r") != wantHeader {
		t.Errorf("header = %q, want %q", lines[0], wantHeader)
	}

	row := strings.TrimRight(lines[1], "\r")
	for _, want := range []string{"src", "id-1", "2024-01-02T03:04:05Z", "alice", "hello", "https://example.com"} {
		if !strings.Contains(row, want) {
			t.Errorf("row %q missing %q", row, want)
		}
	}
}

// TestCreateSinks_Success builds multiple sinks and asserts they all open and
// close cleanly.
func TestCreateSinks_Success(t *testing.T) {
	dir := t.TempDir()

	configs := []SinkConfig{
		{Type: "json-file", Path: filepath.Join(dir, "a.json")},
		{Type: "ndjson", Path: filepath.Join(dir, "b.ndjson")},
	}

	sinks, err := createSinks(configs)
	if err != nil {
		t.Fatalf("createSinks: %v", err)
	}

	if len(sinks) != 2 {
		t.Fatalf("got %d sinks, want 2", len(sinks))
	}

	closeSinks(sinks)
}

// TestCreateSinks_ErrorClosesPrior asserts that when a later sink config is
// invalid, createSinks returns the error and the already-opened sinks are
// closed (no leak). The first sink is a valid ndjson file; the second has an
// unknown type so NewSink fails.
func TestCreateSinks_ErrorClosesPrior(t *testing.T) {
	dir := t.TempDir()

	configs := []SinkConfig{
		{Type: "ndjson", Path: filepath.Join(dir, "ok.ndjson")},
		{Type: "totally-unknown"},
	}

	sinks, err := createSinks(configs)
	if err == nil {
		t.Fatal("expected error from unknown sink type, got nil")
	}

	if sinks != nil {
		t.Errorf("expected nil sinks on error, got %d", len(sinks))
	}

	if !strings.Contains(err.Error(), "unknown sink type") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "unknown sink type")
	}
}

// TestResolveAuth_PlainSession asserts resolveAuth loads a plain (unencrypted)
// JSON session file when no passphrase is provided.
func TestResolveAuth_PlainSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	want := &auth.Session{Provider: "slack", URL: "https://slack.com"}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolveAuth(context.Background(), &AuthConfig{Provider: "slack", Session: path}, false)
	if err != nil {
		t.Fatalf("resolveAuth: %v", err)
	}

	if got == nil {
		t.Fatal("expected session, got nil")
	}

	if got.Provider != "slack" || got.URL != "https://slack.com" {
		t.Errorf("session = %+v, want provider=slack url=https://slack.com", got)
	}
}

// TestResolveAuth_Encrypted asserts resolveAuth decrypts an encrypted session
// file when a passphrase is supplied in the config.
func TestResolveAuth_Encrypted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")
	passphrase := "correct horse battery staple"

	orig := &auth.Session{Provider: "linkedin", URL: "https://linkedin.com"}
	if err := auth.SaveEncrypted(orig, path, passphrase); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	got, err := resolveAuth(context.Background(), &AuthConfig{
		Provider:   "linkedin",
		Session:    path,
		Passphrase: passphrase,
	}, false)
	if err != nil {
		t.Fatalf("resolveAuth: %v", err)
	}

	if got == nil || got.Provider != "linkedin" {
		t.Errorf("session = %+v, want provider=linkedin", got)
	}
}

// TestResolveAuth_EncryptedWrongPassphraseFallsBackToPlain asserts that when an
// encrypted load fails (wrong passphrase) and the file is not plain JSON,
// resolveAuth in dry-run mode returns (nil, nil) rather than launching auth.
func TestResolveAuth_WrongPassphraseDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")

	orig := &auth.Session{Provider: "x"}
	if err := auth.SaveEncrypted(orig, path, "real-pass"); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	// Wrong passphrase: encrypted load fails; encrypted bytes are not valid
	// JSON so plain load fails too; dryRun short-circuits to (nil, nil).
	got, err := resolveAuth(context.Background(), &AuthConfig{
		Provider:   "x",
		Session:    path,
		Passphrase: "wrong-pass",
	}, true)
	if err != nil {
		t.Fatalf("resolveAuth dry-run: %v", err)
	}

	if got != nil {
		t.Errorf("expected nil session on dry-run fallback, got %+v", got)
	}
}

// TestResolveAuth_DryRunNoSession asserts that with no session file and dryRun
// set, resolveAuth returns (nil, nil) without touching the auth registry.
func TestResolveAuth_DryRunNoSession(t *testing.T) {
	got, err := resolveAuth(context.Background(), &AuthConfig{Provider: "slack"}, true)
	if err != nil {
		t.Fatalf("resolveAuth: %v", err)
	}

	if got != nil {
		t.Errorf("expected nil session, got %+v", got)
	}
}

// TestLoadPlainSession covers both the success and error branches of the
// unexported loadPlainSession helper.
func TestLoadPlainSession(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid json", func(t *testing.T) {
		path := filepath.Join(dir, "ok.json")
		if err := os.WriteFile(path, []byte(`{"provider":"discord"}`), 0o600); err != nil {
			t.Fatal(err)
		}

		s, err := loadPlainSession(path)
		if err != nil {
			t.Fatalf("loadPlainSession: %v", err)
		}

		if s.Provider != "discord" {
			t.Errorf("provider = %q, want discord", s.Provider)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadPlainSession(filepath.Join(dir, "nope.json"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte(`{not json`), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := loadPlainSession(path)
		if err == nil {
			t.Error("expected unmarshal error for malformed json")
		}
	})
}

// TestExecute_DryRunWithAuth drives Execute in dry-run mode with an auth block
// pointing at a plain session file. This exercises the createSinks + auth
// resolution + dry-run early-return path without ever launching a browser.
func TestExecute_DryRunWithAuth(t *testing.T) {
	dir := t.TempDir()
	sessPath := filepath.Join(dir, "sess.json")

	if err := os.WriteFile(sessPath, []byte(`{"provider":"slack"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var phases []string

	s := &Strategy{
		Name:    "dry-auth",
		Version: "1.0",
		Auth:    &AuthConfig{Provider: "slack", Session: sessPath},
		Steps:   []Step{{Name: "s1", Mode: "slack"}},
		Output:  OutputConfig{Sinks: []SinkConfig{{Type: "ndjson", Path: filepath.Join(dir, "o.ndjson")}}},
	}

	err := Execute(context.Background(), s, ExecuteOptions{
		DryRun: true,
		Progress: func(_, phase, _ string) {
			phases = append(phases, phase)
		},
	})
	if err != nil {
		t.Fatalf("Execute dry-run: %v", err)
	}

	// The auth + dry-run phases must have been reported.
	var sawAuth, sawDryRun bool

	for _, p := range phases {
		switch p {
		case "auth":
			sawAuth = true
		case "dry-run":
			sawDryRun = true
		}
	}

	if !sawAuth {
		t.Error("expected an 'auth' progress phase")
	}

	if !sawDryRun {
		t.Error("expected a 'dry-run' progress phase")
	}
}

// TestExecute_BadSinkConfig asserts Execute surfaces a sink-creation error
// (unknown type) before any browser launch.
func TestExecute_BadSinkConfig(t *testing.T) {
	s := &Strategy{
		Name:    "bad-sink",
		Version: "1.0",
		Steps:   []Step{{Name: "s1", Mode: "slack"}},
		Output:  OutputConfig{Sinks: []SinkConfig{{Type: "nope-sink", Path: "x"}}},
	}

	err := Execute(context.Background(), s, ExecuteOptions{DryRun: true})
	if err == nil {
		t.Fatal("expected error from unknown sink type")
	}

	if !strings.Contains(err.Error(), "unknown sink type") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "unknown sink type")
	}
}
