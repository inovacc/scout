# Interaction Capture (v1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in recorder that writes a redacted, AI-friendly JSONL trace of every CLI invocation, MCP tool call, and REPL browser action to `<scouthome>/captures/<id>.jsonl`.

**Architecture:** A central `internal/interaction` package owns the schema, per-session file writer, and enable gate; secret redaction is extracted into a shared `internal/redact` package; thin emit hooks are added at three existing chokepoints (root `PersistentPreRunE`, `addTracedTool`, the REPL `switch`). The engine is not touched.

**Tech Stack:** Go 1.26, cobra, segmentio/ksuid, the existing `internal/flags` + `internal/engine/scouthome`.

**Execution note:** the working tree currently carries uncommitted security-hardening changes on `main`. Create a dedicated branch first (e.g. `git switch -c feat/interaction-capture`) — or a worktree via `superpowers:using-git-worktrees` — before starting, so this work stays separate.

**Spec:** `docs/superpowers/specs/2026-06-14-interaction-capture-design.md`

---

## File structure

- Create `internal/redact/redact.go` — secret redaction (`Args`, `Map`, `URL`, `Header`, `Body`, `Pattern`, `Placeholder`).
- Create `internal/redact/redact_test.go`.
- Modify `internal/logger/logger.go` — delegate to `redact.Args`; drop the local copy + `regexp` import.
- Delete `internal/logger/redact_test.go` — its subject (`redactArgs`) moves to `internal/redact`.
- Create `internal/interaction/event.go` — `Event`.
- Create `internal/interaction/gate.go` — `Enabled`, `Dir`.
- Create `internal/interaction/recorder.go` — `Recorder`, `Open`, `Emit`, `Close`, default singleton (`Init`/`Default`/`Emit`/`Close`).
- Create `internal/interaction/interaction_test.go` — gate + recorder tests.
- Create `cmd/scout/interactions.go` — `scout interactions on/off/status/list`.
- Create `cmd/scout/interactions_test.go`.
- Modify `cmd/scout/scout.go` — CLI hook (init/emit/close).
- Modify `pkg/scout/mcp/server.go` — MCP hook in `addTracedTool`.
- Modify `cmd/scout/repl.go` — REPL browser-action hook.

---

## Task 1: `internal/redact` package (extract from logger)

**Files:**
- Create: `internal/redact/redact.go`
- Test: `internal/redact/redact_test.go`

- [ ] **Step 1: Write the failing test**

`internal/redact/redact_test.go`:
```go
package redact

import (
	"reflect"
	"testing"
)

func TestArgs(t *testing.T) {
	cases := []struct{ name string; in, want []string }{
		{"separate", []string{"--api-key", "S"}, []string{"--api-key", Placeholder}},
		{"equals", []string{"--token=abc"}, []string{"--token=" + Placeholder}},
		{"untouched", []string{"--url", "http://x"}, []string{"--url", "http://x"}},
		{"flag-at-end", []string{"run", "--secret"}, []string{"run", "--secret"}},
		{"empty", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Args(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Args(%v)=%v want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMap(t *testing.T) {
	in := map[string]any{"url": "http://x", "password": "p", "nested": map[string]any{"token": "t", "ok": 1}}
	got := Map(in)
	if got["url"] != "http://x" || got["password"] != Placeholder {
		t.Fatalf("top-level redaction wrong: %v", got)
	}
	n := got["nested"].(map[string]any)
	if n["token"] != Placeholder || n["ok"] != 1 {
		t.Fatalf("nested redaction wrong: %v", n)
	}
	if in["password"] != "p" {
		t.Fatal("input mutated")
	}
}

func TestURL(t *testing.T) {
	got := URL("https://h/p?token=abc&q=1")
	if got == "https://h/p?token=abc&q=1" || !contains(got, Placeholder) || !contains(got, "q=1") {
		t.Fatalf("URL redaction wrong: %s", got)
	}
	if URL("not a url") != "not a url" {
		t.Fatal("non-url changed")
	}
}

func TestHeaderAndBody(t *testing.T) {
	if Header("Authorization", "Bearer x") != Placeholder {
		t.Fatal("auth header not masked")
	}
	if Header("Accept", "text/html") != "text/html" {
		t.Fatal("benign header masked")
	}
	s, trunc := Body([]byte("hello"), 3)
	if s != "hel" || !trunc {
		t.Fatalf("body cap wrong: %q %v", s, trunc)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/redact/`
Expected: FAIL — package `redact` does not exist.

- [ ] **Step 3: Write minimal implementation**

`internal/redact/redact.go`:
```go
// Package redact masks secret material in values persisted to logs and
// interaction captures. It is best-effort and pattern-based.
package redact

import (
	"net/url"
	"regexp"
	"strings"
)

// Placeholder replaces a redacted value.
const Placeholder = "***REDACTED***"

// Pattern matches flag/field/header/query names whose values are secret.
var Pattern = regexp.MustCompile(`(?i)(api[-_]?key|passphrase|password|secret|token|credential|cookie|bearer|authorization|auth|sig)`)

// Args returns a copy of args with the values of sensitive flags masked. It
// handles "--flag value", "--flag=value" and short "-f value" forms. The input
// is never mutated.
func Args(args []string) []string {
	if len(args) == 0 {
		return args
	}

	out := make([]string, len(args))
	copy(out, args)

	for i := 0; i < len(out); i++ {
		a := out[i]

		dashes := len(a) - len(strings.TrimLeft(a, "-"))
		if dashes == 0 {
			continue
		}

		body := a[dashes:]
		if eq := strings.IndexByte(body, '='); eq >= 0 {
			if Pattern.MatchString(body[:eq]) {
				out[i] = a[:dashes] + body[:eq] + "=" + Placeholder
			}

			continue
		}

		if Pattern.MatchString(body) && i+1 < len(out) && !strings.HasPrefix(out[i+1], "-") {
			out[i+1] = Placeholder
		}
	}

	return out
}

// Map returns a shallow copy of m with values masked for keys matching Pattern;
// nested maps are redacted recursively. The input is never mutated.
func Map(m map[string]any) map[string]any {
	if len(m) == 0 {
		return m
	}

	out := make(map[string]any, len(m))
	for k, v := range m {
		switch {
		case Pattern.MatchString(k):
			out[k] = Placeholder
		default:
			if nested, ok := v.(map[string]any); ok {
				out[k] = Map(nested)
			} else {
				out[k] = v
			}
		}
	}

	return out
}

// URL masks the values of sensitive query params, preserving scheme/host/path.
// A non-URL string is returned unchanged.
func URL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}

	q := u.Query()
	changed := false

	for key := range q {
		if Pattern.MatchString(key) {
			q.Set(key, Placeholder)
			changed = true
		}
	}

	if !changed {
		return raw
	}

	u.RawQuery = q.Encode()

	return u.String()
}

// Header returns value masked when name is a sensitive header.
func Header(name, value string) string {
	if Pattern.MatchString(name) {
		return Placeholder
	}

	return value
}

// Body truncates b to max bytes, returning the string and whether it truncated.
func Body(b []byte, max int) (string, bool) {
	if len(b) <= max {
		return string(b), false
	}

	return string(b[:max]), true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/redact/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/redact/
git commit -m "feat(redact): add shared secret-redaction package"
```

---

## Task 2: Point the logger at `internal/redact`

**Files:**
- Modify: `internal/logger/logger.go`
- Delete: `internal/logger/redact_test.go`

- [ ] **Step 1: Update the logger to delegate**

In `internal/logger/logger.go`:
1. Remove the `"regexp"` import.
2. Add import `"github.com/inovacc/scout/internal/redact"`.
3. Delete the `redactedValue` const, the `sensitiveArgPattern` var, and the entire `redactArgs` function.
4. Replace the two call sites `redactArgs(args)` → `redact.Args(args)` and `redactArgs(l.execution.Args)` → `redact.Args(l.execution.Args)`.

- [ ] **Step 2: Delete the moved test**

```bash
git rm internal/logger/redact_test.go
```

- [ ] **Step 3: Build + test**

Run: `go build ./internal/logger/ && go test ./internal/logger/ ./internal/redact/`
Expected: PASS (no `regexp`/`redactArgs` references remain).

- [ ] **Step 4: Commit**

```bash
git add internal/logger/
git commit -m "refactor(logger): use shared internal/redact for arg redaction"
```

---

## Task 3: `internal/interaction` — Event + gate

**Files:**
- Create: `internal/interaction/event.go`
- Create: `internal/interaction/gate.go`
- Test: `internal/interaction/interaction_test.go` (gate portion)

- [ ] **Step 1: Write the failing gate test**

`internal/interaction/interaction_test.go`:
```go
package interaction

import (
	"path/filepath"
	"testing"
)

func TestGate(t *testing.T) {
	// Assert only the env-enable path; the persisted feature-flag state is
	// machine-dependent and not controllable from a unit test. The disabled
	// behaviour is covered by TestNilRecorderNoOp at the recorder level.
	t.Setenv("SCOUT_INTERACTIONS", "1")
	if !Enabled() {
		t.Fatal("SCOUT_INTERACTIONS=1 should enable")
	}
}

func TestDirDefault(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())
	d, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(d) != "captures" {
		t.Fatalf("Dir()=%s, want .../captures", d)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/interaction/`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/interaction/event.go`:
```go
package interaction

// Event is one JSONL record in a capture file. Kind makes each line
// self-describing; only set fields are written (omitempty).
type Event struct {
	Seq        int            `json:"seq"`
	TS         string         `json:"ts"`
	Kind       string         `json:"kind"`
	Source     string         `json:"source,omitempty"`
	Name       string         `json:"name,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
	Result     string         `json:"result,omitempty"`
	OK         *bool          `json:"ok,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}
```

`internal/interaction/gate.go`:
```go
package interaction

import (
	"os"

	"github.com/inovacc/scout/internal/engine/scouthome"
	"github.com/inovacc/scout/internal/flags"
)

const feature = "interactions"

// Enabled reports whether interaction capture is on (feature flag or env).
func Enabled() bool {
	if flags.IsFeatureEnabled(feature) {
		return true
	}

	switch os.Getenv("SCOUT_INTERACTIONS") {
	case "1", "true", "TRUE", "yes":
		return true
	}

	return false
}

// Dir returns the capture directory: the feature's stored data path if set,
// otherwise <scouthome>/captures.
func Dir() (string, error) {
	if d := flags.GetFeatureData(feature); d != "" {
		return d, nil
	}

	return scouthome.Sub("captures")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/interaction/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/interaction/event.go internal/interaction/gate.go internal/interaction/interaction_test.go
git commit -m "feat(interaction): add Event schema and enable gate"
```

---

## Task 4: `internal/interaction` — Recorder + default singleton

**Files:**
- Create: `internal/interaction/recorder.go`
- Test: `internal/interaction/interaction_test.go` (append recorder tests)

- [ ] **Step 1: Add the failing recorder tests**

Extend the **existing** import block in `internal/interaction/interaction_test.go` to add `"bufio"`, `"encoding/json"`, and `"os"` (do NOT add a second `import` block — Go forbids imports after declarations), then append these test functions:
```go
func TestNilRecorderNoOp(t *testing.T) {
	var r *Recorder
	r.Emit(Event{Kind: "x"}) // must not panic
	if err := r.Close("ok"); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestRecorderRoundtrip(t *testing.T) {
	t.Setenv("SCOUT_INTERACTIONS", "1")
	t.Setenv("SCOUT_HOME", t.TempDir())

	rec, err := Open("cli", "cli-test")
	if err != nil || rec == nil {
		t.Fatalf("Open: rec=%v err=%v", rec, err)
	}

	ok := true
	rec.Emit(Event{Kind: "cli", Source: "cli", Name: "gather", OK: &ok})
	if err := rec.Close("ok"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir, _ := Dir()
	f, err := os.Open(dir + string(os.PathSeparator) + "cli-test.jsonl")
	if err != nil {
		t.Fatalf("open capture: %v", err)
	}
	defer f.Close()

	var kinds []string
	var lastSeq int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("bad jsonl: %v", err)
		}
		if e.Seq != lastSeq {
			t.Fatalf("non-monotonic seq: got %d want %d", e.Seq, lastSeq)
		}
		lastSeq++
		kinds = append(kinds, e.Kind)
	}

	if len(kinds) != 3 || kinds[0] != "session_start" || kinds[1] != "cli" || kinds[2] != "session_end" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/interaction/ -run 'TestNilRecorderNoOp|TestRecorderRoundtrip'`
Expected: FAIL — `Recorder`/`Open` undefined.

- [ ] **Step 3: Write the implementation**

`internal/interaction/recorder.go`:
```go
package interaction

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
)

// Recorder is a per-session append-only JSONL writer. All methods are safe on a
// nil *Recorder (no-op), so callers never branch on Enabled().
type Recorder struct {
	mu     sync.Mutex
	f      *os.File
	w      *bufio.Writer
	seq    int
	closed bool
}

// Open returns a Recorder writing <Dir>/<id>.jsonl, or (nil, nil) when capture
// is disabled. It writes a session_start header. entrypoint ∈ {cli,mcp,grpc,agent}.
func Open(entrypoint, id string) (*Recorder, error) {
	if !Enabled() {
		return nil, nil
	}

	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(dir, id+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}

	r := &Recorder{f: f, w: bufio.NewWriter(f)}
	r.Emit(Event{Kind: "session_start", Source: entrypoint, Extra: map[string]any{"entrypoint": entrypoint, "id": id}})

	return r, nil
}

// Emit appends an event, stamping Seq and TS. Safe on a nil receiver.
func (r *Recorder) Emit(e Event) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	e.Seq = r.seq
	r.seq++

	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}

	b, err := json.Marshal(e)
	if err != nil {
		return
	}

	_, _ = r.w.Write(b)
	_ = r.w.WriteByte('\n')
	_ = r.w.Flush()
}

// Close writes a session_end event and closes the file. Safe on a nil receiver.
func (r *Recorder) Close(status string) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	events := r.seq
	r.mu.Unlock()

	r.Emit(Event{Kind: "session_end", Extra: map[string]any{"status": status, "events": events}})

	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	_ = r.w.Flush()

	return r.f.Close()
}

// --- process-global default recorder (single-session processes) ---

var (
	defMu  sync.Mutex
	defRec *Recorder
)

// Init opens the process-global default recorder with a generated id
// "<entrypoint>-<ksuid>". Returns nil when disabled.
func Init(entrypoint string) *Recorder {
	r, err := Open(entrypoint, entrypoint+"-"+ksuid.New().String())
	if err != nil {
		return nil
	}

	defMu.Lock()
	defRec = r
	defMu.Unlock()

	return r
}

// Default returns the process-global recorder (nil if not initialised/disabled).
func Default() *Recorder {
	defMu.Lock()
	defer defMu.Unlock()

	return defRec
}

// Emit appends to the default recorder (no-op if none).
func Emit(e Event) { Default().Emit(e) }

// Close closes the default recorder (no-op if none).
func Close(status string) error { return Default().Close(status) }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/interaction/`
Expected: PASS (all tests). On Windows the file-mode is not asserted (Windows ignores Unix perms).

- [ ] **Step 5: Commit**

```bash
git add internal/interaction/recorder.go internal/interaction/interaction_test.go
git commit -m "feat(interaction): add Recorder + default singleton"
```

---

## Task 5: `scout interactions` command

**Files:**
- Create: `cmd/scout/interactions.go`
- Test: `cmd/scout/interactions_test.go`

- [ ] **Step 1: Write the failing test**

`cmd/scout/interactions_test.go`:
```go
package main

import "testing"

func TestInteractionsCommandRegistered(t *testing.T) {
	if interactionsOnCmd.Flags().Lookup("dir") == nil {
		t.Fatal("interactions on --dir flag not registered")
	}
	sub := map[string]bool{}
	for _, c := range interactionsCmd.Commands() {
		sub[c.Name()] = true
	}
	for _, want := range []string{"on", "off", "status", "list"} {
		if !sub[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/scout/ -run TestInteractionsCommandRegistered`
Expected: FAIL — `interactionsCmd` undefined.

- [ ] **Step 3: Write the implementation**

`cmd/scout/interactions.go`:
```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/inovacc/scout/internal/flags"
	"github.com/inovacc/scout/internal/interaction"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(interactionsCmd)
	interactionsCmd.AddCommand(interactionsOnCmd, interactionsOffCmd, interactionsStatusCmd, interactionsListCmd)
	interactionsOnCmd.Flags().String("dir", "", "capture directory (default: <scouthome>/captures)")
	_ = flags.IgnoreCommand("interactions") // never capture the management command itself
}

var interactionsCmd = &cobra.Command{
	Use:   "interactions",
	Short: "Capture Scout interactions (CLI, MCP, browser actions) to <scouthome>/captures for analysis",
}

var interactionsOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable interaction capture",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if err := flags.EnableFeature("interactions", dir); err != nil {
			return fmt.Errorf("scout: interactions: %w", err)
		}

		d, _ := interaction.Dir()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "interaction capture ON -> %s\n", d)

		return nil
	},
}

var interactionsOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable interaction capture",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := flags.DisableFeature("interactions"); err != nil {
			return fmt.Errorf("scout: interactions: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "interaction capture OFF")

		return nil
	},
}

var interactionsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show interaction-capture status",
	RunE: func(cmd *cobra.Command, _ []string) error {
		d, _ := interaction.Dir()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "enabled: %v\ndir: %s\n", interaction.Enabled(), d)

		return nil
	},
}

var interactionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List capture files",
	RunE: func(cmd *cobra.Command, _ []string) error {
		d, err := interaction.Dir()
		if err != nil {
			return err
		}

		entries, err := os.ReadDir(d)
		if err != nil {
			if os.IsNotExist(err) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no captures")
				return nil
			}

			return err
		}

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
				names = append(names, e.Name())
			}
		}

		sort.Strings(names)

		for _, n := range names {
			size := int64(0)
			if info, statErr := os.Stat(filepath.Join(d, n)); statErr == nil {
				size = info.Size()
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %8d bytes\n", n, size)
		}

		if len(names) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no captures")
		}

		return nil
	},
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/scout/ -run TestInteractionsCommandRegistered`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/scout/interactions.go cmd/scout/interactions_test.go
git commit -m "feat(cli): add 'scout interactions on/off/status/list'"
```

---

## Task 6: CLI emit hook

**Files:**
- Modify: `cmd/scout/scout.go` (root `PersistentPreRunE` ~line 41-46, and `main()` ~line 89-95)

- [ ] **Step 1: Add the hook**

In `cmd/scout/scout.go`:
1. Add imports `"github.com/inovacc/scout/internal/interaction"` and `"github.com/inovacc/scout/internal/redact"`.
2. In `PersistentPreRunE`, right after the existing `logger.Init`/`StartExecution` block, add:
```go
		if !flags.ShouldIgnoreCommand(cmd.Name()) {
			interaction.Init("cli")
			interaction.Emit(interaction.Event{
				Kind:   "cli",
				Source: "cli",
				Name:   cmd.Name(),
				Input:  map[string]any{"args": redact.Args(args)},
			})
		}
```
3. In `main()`, in the post-`Execute` block next to `logger.EndExecution(err)`, add:
```go
		status := "ok"
		if err != nil {
			status = "error"
		}
		_ = interaction.Close(status)
```

- [ ] **Step 2: Build**

Run: `go build ./cmd/scout/`
Expected: success (`flags` is already imported).

- [ ] **Step 3: Smoke-verify the hook (manual integration)**

Run:
```bash
SCOUT_HOME="$(mktemp -d)" SCOUT_INTERACTIONS=1 go run ./cmd/scout version
SCOUT_HOME="$SCOUT_HOME" go run ./cmd/scout interactions list   # reuse same SCOUT_HOME
```
(Windows PowerShell: set `$env:SCOUT_HOME` to a temp dir and `$env:SCOUT_INTERACTIONS=1`.)
Expected: `interactions list` shows one `cli-*.jsonl`; its contents contain `session_start`, a `cli` event with `"name":"version"`, and `session_end`.

- [ ] **Step 4: Commit**

```bash
git add cmd/scout/scout.go
git commit -m "feat(capture): emit CLI invocation events"
```

---

## Task 7: MCP emit hook

**Files:**
- Modify: `pkg/scout/mcp/server.go` (`addTracedTool`, ~line 152-183)

- [ ] **Step 1: Add the hook**

In `pkg/scout/mcp/server.go`:
1. Add imports `"time"` (if not present), `"encoding/json"` (if not present), `"github.com/inovacc/scout/internal/interaction"`, `"github.com/inovacc/scout/internal/redact"`.
2. In the `addTracedTool` closure, capture `start := time.Now()` before `tracing.MCPToolSpan`, and after the existing `switch { … finish … }` block (before `return result, err`) add:
```go
			ok := err == nil && (result == nil || !result.IsError)

			ev := interaction.Event{
				Kind:       "mcp_tool",
				Source:     "mcp",
				Name:       name,
				OK:         &ok,
				DurationMS: time.Since(start).Milliseconds(),
			}
			if err != nil {
				ev.Error = err.Error()
			}
			// Arguments: tolerate either a decoded map or raw JSON. Confirm the
			// field path against go-sdk/mcp v1.4.1 (mirror how existing tools read
			// req); guarded so it degrades to no-input if the shape differs.
			if req != nil && req.Params != nil {
				switch a := any(req.Params.Arguments).(type) {
				case map[string]any:
					ev.Input = redact.Map(a)
				case json.RawMessage:
					var m map[string]any
					if json.Unmarshal(a, &m) == nil {
						ev.Input = redact.Map(m)
					}
				}
			}

			interaction.Emit(ev)
```

- [ ] **Step 2: Build**

Run: `go build ./pkg/scout/mcp/`
Expected: success. If it fails on `req.Params.Arguments`, grep an existing tool for how it reads arguments (`rg "req.Params" pkg/scout/mcp`) and mirror that accessor.

- [ ] **Step 3: Smoke-verify (optional, manual)**

Start the MCP server with capture on and call any tool; confirm an `mcp_tool` event lands in the `mcp-*.jsonl` file. (Requires an MCP client; build success + the recorder unit tests are the primary gate.)

- [ ] **Step 4: Commit**

```bash
git add pkg/scout/mcp/server.go
git commit -m "feat(capture): emit MCP tool-call events"
```

> Note: when run as the MCP server, the entrypoint must initialise the default recorder. In `pkg/scout/mcp` `ServeSSE`/`Serve` (or the CLI `mcp` command), add `interaction.Init("mcp")` at startup and `defer interaction.Close("ok")` so `addTracedTool` has a recorder. Add this in the same commit.

---

## Task 8: REPL browser-action hook

**Files:**
- Modify: `cmd/scout/repl.go` (after `c := parts[0]`, ~line 78)

- [ ] **Step 1: Add the hook**

In `cmd/scout/repl.go`:
1. Add imports `"github.com/inovacc/scout/internal/interaction"` and `"github.com/inovacc/scout/internal/redact"`.
2. Ensure the REPL initialises a recorder once before the read loop (near where the REPL session starts): `interaction.Init("repl"); defer interaction.Close("ok")`.
3. Immediately after `c := parts[0]`, add:
```go
				switch c {
				case "exit", "quit", "help":
					// meta commands — not captured
				default:
					interaction.Emit(interaction.Event{
						Kind:   "browser_action",
						Source: "repl",
						Name:   c,
						Input:  map[string]any{"args": redact.Args(parts[1:])},
					})
				}
```
(This is a separate `switch` placed *before* the existing command `switch c` — it only emits; it does not alter dispatch.)

- [ ] **Step 2: Build**

Run: `go build ./cmd/scout/`
Expected: success.

- [ ] **Step 3: Smoke-verify (manual)**

```bash
SCOUT_HOME="$(mktemp -d)" SCOUT_INTERACTIONS=1 go run ./cmd/scout repl
# type:  navigate https://example.com  then  exit
SCOUT_HOME="$SCOUT_HOME" go run ./cmd/scout interactions list
```
Expected: a `repl-*.jsonl` containing a `browser_action` event `{"name":"navigate", ...}`.

- [ ] **Step 4: Commit**

```bash
git add cmd/scout/repl.go
git commit -m "feat(capture): emit REPL browser-action events"
```

---

## Task 9: Final verification

- [ ] **Step 1: Full build + vet + fmt + targeted tests**

Run:
```bash
go build ./cmd/scout/ ./pkg/... ./internal/...
gofmt -l internal/redact internal/interaction cmd/scout/interactions.go
go vet ./internal/redact/ ./internal/interaction/ ./cmd/scout/ ./pkg/scout/mcp/ ./internal/logger/
go test ./internal/redact/ ./internal/interaction/ ./internal/logger/ ./cmd/scout/ -run 'Redact|Args|Map|URL|Header|Body|Gate|Dir|Recorder|Interactions'
```
Expected: build clean, `gofmt -l` empty, `vet` exit 0, tests PASS.

- [ ] **Step 2: End-to-end smoke**

Enable, run a few commands, inspect a capture file, disable:
```bash
go run ./cmd/scout interactions on
go run ./cmd/scout version
go run ./cmd/scout interactions list
# open the newest <scouthome>/captures/cli-*.jsonl and eyeball the JSONL
go run ./cmd/scout interactions off
```
Expected: capture files contain redacted, well-formed JSONL framed by session_start/session_end.

- [ ] **Step 3: Update docs**

Add a CLAUDE.md bullet under conventions: "Interaction capture: `scout interactions on/off/status/list` or `SCOUT_INTERACTIONS=1` records redacted JSONL of CLI/MCP/REPL interactions to `<scouthome>/captures/<id>.jsonl` (`internal/interaction` + `internal/redact`)."

```bash
git add CLAUDE.md
git commit -m "docs: document interaction capture"
```

---

## Out of scope (v2/v3 — do not build now)

- gRPC unary/stream interceptors + per-session recorders; agent `/call`; network (HAR link) + console hooks.
- Rotation, retention, `scout interactions prune`, `scout interactions show <id>`.
- MCP tool-argument capture if the SDK field path needs more than the guarded extraction in Task 7.
- Any in-Scout LLM analysis of captures.
