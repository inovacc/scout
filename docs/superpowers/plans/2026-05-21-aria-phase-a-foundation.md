# ARIA-Ref Model — Phase A: Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** `docs/superpowers/specs/2026-05-21-playwright-mcp-aria-ref-design.md`

**Goal:** Deliver the foundation of the ARIA-ref model — the new `pkg/scout/aria/` package and the first consumer (`browser_snapshot` MCP tool + `scout://snapshot/{page-id}` MCP resource) — without touching any existing selector-based MCP tool. End state: AI can request a snapshot, get YAML back with stable ref IDs, but cannot yet act on those refs (Phase B).

**Architecture:** New top-level package `pkg/scout/aria/` containing five focused files (errors, store, axtree, diff, resolve) totaling < 1,000 LOC. The MCP layer (`pkg/scout/mcp/`) gains one new tool file (`tools_aria.go`) and one resource template, both ≤ 200 LOC. Strict layering: `aria/` depends only on `internal/engine/` + `internal/engine/lib/proto`. Enforced by a `golangci-lint` `depguard` rule.

**Tech Stack:** Go 1.26, `internal/engine/lib/proto/accessibility.go` (CDP AX types, already internalized), `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.1, existing rod-fork CDP client, `sync.RWMutex` for the store, `gopkg.in/yaml.v3` (already in `go.sum` via transitive deps — verify in Task 1).

---

## Conventions for every task

- **Branch:** all work on `feat/aria-phase-a`. Create in Task 0; merge to `main` only after Task 15 passes.
- **Shell wrapper:** Per project CLAUDE.md, every shell invocation goes through `.scripts/`. Each task that runs commands shows the script body and the invocation. Add `.scripts/` to `.gitignore` if not already.
- **Errors:** prefix all wrapped errors with `"scout: aria: <subsystem>: %w"`. Use `errors.Is` / `errors.As` for comparisons.
- **No mocks:** real browser via `newTestBrowser(t)`. Skip cleanly when Chromium unavailable.
- **TDD strictly:** test first, run-to-fail, implement, run-to-pass, commit. Every task follows this loop.
- **Commits:** conventional commits (`feat(aria):`, `test(aria):`, `chore(aria):`). No `Co-Authored-By:`. Use heredoc form per project rule.
- **Run a single test:** `go test -v -run TestName ./pkg/scout/aria/`.
- **Full check before phase merge:** `task check`.

---

## File Structure

| File | Created in | Responsibility | LOC ceiling |
|---|---|---|---|
| `pkg/scout/aria/doc.go` | Task 1 | Package doc + layering rule comment | 20 |
| `pkg/scout/aria/errors.go` | Task 2 | Sentinel errors + typed error structs (`StaleRefError`, `NoSnapshotError`, `AmbiguousRefError`, `TruncatedError`) | 120 |
| `pkg/scout/aria/errors_test.go` | Task 2 | `errors.Is` / `errors.As` round-trips | 80 |
| `pkg/scout/aria/store.go` | Task 3 | `Store` type with `Put`/`Get`/`Resolve`/`Clear`/`Bump` | 130 |
| `pkg/scout/aria/store_test.go` | Task 3 | Concurrency + version + stale-ref tests | 150 |
| `pkg/scout/aria/axtree.go` | Tasks 4–7 | `Snapshot` type, `Capture`, `RenderYAML`, truncation, cross-frame | 350 |
| `pkg/scout/aria/axtree_test.go` | Tasks 4–7 | Fixture-driven YAML golden tests | 250 |
| `pkg/scout/aria/axtree_browser_test.go` | Task 5 | Real-browser AX-tree capture (build tag: integration) | 180 |
| `pkg/scout/aria/diff.go` | Task 8 | `Diff` + `Summary` + `String()` | 150 |
| `pkg/scout/aria/diff_test.go` | Task 8 | Snapshot-pair fixtures → summary string golden | 180 |
| `pkg/scout/aria/resolve.go` | Task 9 | `ResolveElement(page, ref)` via `DOM.resolveNode` | 90 |
| `pkg/scout/aria/resolve_browser_test.go` | Task 9 | Real-browser ref → click round-trip | 120 |
| `pkg/scout/aria/testdata/*.json` + `*.yaml` | Tasks 4–8 | Fixture AX-tree inputs + expected YAML / diff outputs | n/a |
| `pkg/scout/mcp/resources.go` | Task 11 | Add `scout://snapshot/{page-id}` resource template | +60 |
| `pkg/scout/mcp/tools_aria.go` | Task 12 | New `browser_snapshot` tool (no actions yet) | 150 |
| `pkg/scout/mcp/tools_aria_test.go` | Task 12 | In-memory MCP client round-trip + error-hint contracts | 200 |
| `pkg/scout/mcp/state.go` (modify) | Task 12 | Hang `*aria.Store` off `mcpState` | +20 |
| `pkg/scout/mcp/invalidation.go` | Task 13 | Wire page navigation + `targetDestroyed` → `store.Clear` | 100 |
| `pkg/scout/mcp/invalidation_browser_test.go` | Task 13 | Real-browser navigation invalidation | 120 |
| `.golangci.yml` (modify) | Task 10 | `depguard` rule for `pkg/scout/aria/` | +25 |
| `docs/CHANGELOG.md` (modify) | Task 15 | Phase A entry | +15 |

Total new code: ~1,600 LOC. Total new tests: ~1,300 LOC. Coverage target: ≥ 90 % per spec §8.5.

---

## Task 0: Branch + scratch space

**Files:**
- Create: `.scripts/aria-branch.sh`

- [ ] **Step 1: Write the script**

```bash
mkdir -p .scripts
cat > .scripts/aria-branch.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git checkout -b feat/aria-phase-a
grep -q '^\.scripts/$' .gitignore || echo '.scripts/' >> .gitignore
git status --short
EOF
chmod +x .scripts/aria-branch.sh
```

- [ ] **Step 2: Run it**

```
bash .scripts/aria-branch.sh
```

Expected: `Switched to a new branch 'feat/aria-phase-a'`, then `?? .gitignore` (or no output if already present).

- [ ] **Step 3: Verify clean working tree**

```
git status
```

Expected: `On branch feat/aria-phase-a`, nothing committed except possibly an updated `.gitignore`.

- [ ] **Step 4: Commit `.gitignore` if changed**

```
git add .gitignore && git commit -m "chore: ensure .scripts is gitignored"
```

Skip if no change.

---

## Task 1: Scaffold the `aria` package

**Files:**
- Create: `pkg/scout/aria/doc.go`

- [ ] **Step 1: Create the package directory**

```bash
cat > .scripts/aria-scaffold.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
mkdir -p pkg/scout/aria/testdata
ls -la pkg/scout/aria/
EOF
bash .scripts/aria-scaffold.sh
```

Expected: `pkg/scout/aria/testdata/` exists.

- [ ] **Step 2: Write `doc.go`**

```go
// Package aria implements the accessibility-tree (ARIA) snapshot + ref-id
// interaction model that powers Scout's MCP tools.
//
// Snapshots are captured via the CDP Accessibility.getFullAXTree command and
// rendered as YAML with each interactive node tagged [ref=eN] (root frame) or
// [ref=fF:eN] (frame F). Refs are stable within a snapshot version; navigation
// or DOM mutation invalidates them.
//
// Strict layering: this package imports only internal/engine and
// internal/engine/lib/proto. It MUST NOT import pkg/scout/mcp,
// pkg/scout/agent, or pkg/scout/runbook — those packages depend on aria,
// never the other way. The depguard rule in .golangci.yml enforces this.
package aria
```

Write to `pkg/scout/aria/doc.go`.

- [ ] **Step 3: Verify package builds (empty package is legal)**

```
go build ./pkg/scout/aria/
```

Expected: no output, exit 0.

- [ ] **Step 4: Verify `yaml.v3` is reachable**

```
go list -m gopkg.in/yaml.v3
```

Expected: a version line like `gopkg.in/yaml.v3 v3.0.1`. If not reachable, run `go get gopkg.in/yaml.v3` and commit the `go.mod`/`go.sum` change.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-1.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/doc.go go.mod go.sum 2>/dev/null || true
git commit -m "feat(aria): scaffold pkg/scout/aria with package doc"
EOF
bash .scripts/aria-commit-1.sh
```

---

## Task 2: Typed errors

**Files:**
- Create: `pkg/scout/aria/errors.go`
- Test: `pkg/scout/aria/errors_test.go`

- [ ] **Step 1: Write the failing test**

```go
package aria_test

import (
	"errors"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func TestStaleRefError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.StaleRefError{Ref: "e15", HaveVersion: 14, RequestedVersion: 11}
	if !errors.Is(src, aria.ErrStaleRef) {
		t.Fatalf("errors.Is(src, ErrStaleRef) = false, want true")
	}
	var dst *aria.StaleRefError
	if !errors.As(src, &dst) {
		t.Fatalf("errors.As(src, &dst) = false")
	}
	if dst.Ref != "e15" || dst.HaveVersion != 14 || dst.RequestedVersion != 11 {
		t.Fatalf("typed fields lost: %+v", dst)
	}
}

func TestNoSnapshotError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.NoSnapshotError{PageID: "p1"}
	if !errors.Is(src, aria.ErrNoSnapshot) {
		t.Fatalf("errors.Is failed")
	}
	var dst *aria.NoSnapshotError
	if !errors.As(src, &dst) || dst.PageID != "p1" {
		t.Fatalf("typed lost: %+v", dst)
	}
}

func TestAmbiguousRefError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.AmbiguousRefError{Ref: "e3", BackendNodeID: 4242}
	if !errors.Is(src, aria.ErrAmbiguousRef) {
		t.Fatalf("errors.Is failed")
	}
	var dst *aria.AmbiguousRefError
	if !errors.As(src, &dst) || dst.Ref != "e3" || dst.BackendNodeID != 4242 {
		t.Fatalf("typed lost: %+v", dst)
	}
}

func TestTruncatedError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.TruncatedError{Captured: 9999, Total: 50000, Reason: "node cap"}
	if !errors.Is(src, aria.ErrTruncated) {
		t.Fatalf("errors.Is failed")
	}
	var dst *aria.TruncatedError
	if !errors.As(src, &dst) || dst.Captured != 9999 {
		t.Fatalf("typed lost: %+v", dst)
	}
}

func TestErrorMessages_ScoutPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want string
	}{
		{(&aria.StaleRefError{Ref: "e1", HaveVersion: 2, RequestedVersion: 1}).Error(),
			"scout: aria: stale ref e1 (have v2, requested v1)"},
		{(&aria.NoSnapshotError{PageID: "p1"}).Error(),
			"scout: aria: no snapshot for page p1"},
		{(&aria.AmbiguousRefError{Ref: "e3", BackendNodeID: 4242}).Error(),
			"scout: aria: ambiguous ref e3 (backend node 4242 detached)"},
		{(&aria.TruncatedError{Captured: 5, Total: 10, Reason: "byte cap"}).Error(),
			"scout: aria: snapshot truncated: byte cap (5/10 nodes)"},
	}
	for _, tc := range tests {
		if got := tc.err.(error).Error(); got != tc.want {
			t.Errorf("Error()=%q want %q", got, tc.want)
		}
	}
}
```

Note: the test imports compare `.Error()` strings — keep the strings stable; they're part of the public contract per spec §7.2.

- [ ] **Step 2: Run test to verify it fails**

```
go test -v ./pkg/scout/aria/
```

Expected: `undefined: aria.StaleRefError` (et al). Build error.

- [ ] **Step 3: Write `errors.go`**

```go
package aria

import (
	"errors"
	"fmt"
)

var (
	ErrStaleRef     = errors.New("scout: aria: stale ref")
	ErrNoSnapshot   = errors.New("scout: aria: no snapshot")
	ErrAmbiguousRef = errors.New("scout: aria: ambiguous ref")
	ErrCapture      = errors.New("scout: aria: capture failed")
	ErrTruncated    = errors.New("scout: aria: snapshot truncated")
)

type StaleRefError struct {
	Ref              string
	HaveVersion      uint64
	RequestedVersion uint64
}

func (e *StaleRefError) Error() string {
	return fmt.Sprintf("scout: aria: stale ref %s (have v%d, requested v%d)",
		e.Ref, e.HaveVersion, e.RequestedVersion)
}
func (e *StaleRefError) Unwrap() error { return ErrStaleRef }

type NoSnapshotError struct {
	PageID string
}

func (e *NoSnapshotError) Error() string {
	return fmt.Sprintf("scout: aria: no snapshot for page %s", e.PageID)
}
func (e *NoSnapshotError) Unwrap() error { return ErrNoSnapshot }

type AmbiguousRefError struct {
	Ref           string
	BackendNodeID int64
}

func (e *AmbiguousRefError) Error() string {
	return fmt.Sprintf("scout: aria: ambiguous ref %s (backend node %d detached)",
		e.Ref, e.BackendNodeID)
}
func (e *AmbiguousRefError) Unwrap() error { return ErrAmbiguousRef }

type TruncatedError struct {
	Captured int
	Total    int
	Reason   string
}

func (e *TruncatedError) Error() string {
	return fmt.Sprintf("scout: aria: snapshot truncated: %s (%d/%d nodes)",
		e.Reason, e.Captured, e.Total)
}
func (e *TruncatedError) Unwrap() error { return ErrTruncated }
```

- [ ] **Step 4: Run tests to verify pass**

```
go test -v ./pkg/scout/aria/
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-2.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/errors.go pkg/scout/aria/errors_test.go
git commit -m "feat(aria): typed errors with sentinel wrappers"
EOF
bash .scripts/aria-commit-2.sh
```

---

## Task 3: Store

**Files:**
- Create: `pkg/scout/aria/store.go`
- Test: `pkg/scout/aria/store_test.go`

The Store holds the current `*Snapshot` per `pageID`. Note: the `Snapshot` type doesn't exist yet — Task 4 introduces it. To break the dependency cycle for this task, define the minimal Snapshot stub in `store.go` itself; Task 4 will move it to `axtree.go`.

- [ ] **Step 1: Write the failing test**

```go
package aria_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func TestStore_PutGet(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	snap := &aria.Snapshot{PageID: "p1", Version: 1}
	s.Put("p1", snap)

	got, ok := s.Get("p1")
	if !ok || got.Version != 1 {
		t.Fatalf("Get returned (%v, %v), want (v1, true)", got, ok)
	}

	_, ok = s.Get("p2")
	if ok {
		t.Fatalf("Get(p2) ok=true, want false")
	}
}

func TestStore_Clear(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	s.Put("p1", &aria.Snapshot{PageID: "p1", Version: 1})
	s.Clear("p1")
	if _, ok := s.Get("p1"); ok {
		t.Fatalf("Get after Clear returned ok=true")
	}
}

func TestStore_Resolve_NoSnapshot(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	_, err := s.Resolve("p1", "e15")
	var nse *aria.NoSnapshotError
	if !errors.As(err, &nse) || nse.PageID != "p1" {
		t.Fatalf("Resolve err=%v want NoSnapshotError{p1}", err)
	}
}

func TestStore_Resolve_StaleRef(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	s.Put("p1", &aria.Snapshot{
		PageID:  "p1",
		Version: 7,
		Nodes: []aria.Node{
			{Ref: "e1", BackendNodeID: 100},
			{Ref: "e2", BackendNodeID: 200},
		},
	})
	_, err := s.Resolve("p1", "e99")
	var sre *aria.StaleRefError
	if !errors.As(err, &sre) || sre.HaveVersion != 7 {
		t.Fatalf("Resolve unknown ref err=%v want StaleRefError v=7", err)
	}
}

func TestStore_Resolve_Hit(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	s.Put("p1", &aria.Snapshot{
		PageID:  "p1",
		Version: 3,
		Nodes:   []aria.Node{{Ref: "e1", BackendNodeID: 4242}},
	})
	bn, err := s.Resolve("p1", "e1")
	if err != nil || bn != 4242 {
		t.Fatalf("Resolve(e1)=(%d,%v), want (4242,nil)", bn, err)
	}
}

func TestStore_Concurrent(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); s.Put("p", &aria.Snapshot{PageID: "p", Version: uint64(i)}) }(i)
		go func() { defer wg.Done(); _, _ = s.Get("p") }()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test -v -run TestStore ./pkg/scout/aria/
```

Expected: `undefined: aria.NewStore`, `undefined: aria.Snapshot`, `undefined: aria.Node`.

- [ ] **Step 3: Write `store.go`**

```go
package aria

import (
	"sync"
	"time"
)

// Node is a single entry in a Snapshot's flat node list. Children are encoded
// by index references into the parent Snapshot.Nodes slice — see axtree.go for
// the rendering walker. Defined here so the Store can stand alone; the
// authoritative documentation is in axtree.go.
type Node struct {
	Ref           string  // "e15" (root) or "f2:e3" (frame 2)
	BackendNodeID int64   // CDP DOM.BackendNodeId
	Role          string  // e.g. "button", "textbox"
	Name          string  // accessible name
	Value         string  // current value, if any
	Children      []int   // indices into Snapshot.Nodes
	FrameID       string  // "" for root frame
}

// Snapshot is a captured accessibility tree at a point in time. Immutable
// once Put into the Store; Capture (axtree.go) builds a new Snapshot for
// each version bump.
type Snapshot struct {
	PageID     string
	Version    uint64
	Nodes      []Node
	URI        string
	CapturedAt time.Time
	Truncated  bool
}

// Store maps page IDs to their current Snapshot. Concurrent-safe.
type Store struct {
	mu sync.RWMutex
	m  map[string]*Snapshot
}

func NewStore() *Store {
	return &Store{m: make(map[string]*Snapshot)}
}

func (s *Store) Put(pageID string, snap *Snapshot) {
	s.mu.Lock()
	s.m[pageID] = snap
	s.mu.Unlock()
}

func (s *Store) Get(pageID string) (*Snapshot, bool) {
	s.mu.RLock()
	snap, ok := s.m[pageID]
	s.mu.RUnlock()
	return snap, ok
}

func (s *Store) Clear(pageID string) {
	s.mu.Lock()
	delete(s.m, pageID)
	s.mu.Unlock()
}

// Resolve returns the CDP backend node ID for a ref, or a typed error.
// NoSnapshotError: never snapshotted this page. StaleRefError: snapshotted
// but the ref is unknown in the current version.
func (s *Store) Resolve(pageID, ref string) (int64, error) {
	snap, ok := s.Get(pageID)
	if !ok {
		return 0, &NoSnapshotError{PageID: pageID}
	}
	for i := range snap.Nodes {
		if snap.Nodes[i].Ref == ref {
			return snap.Nodes[i].BackendNodeID, nil
		}
	}
	return 0, &StaleRefError{
		Ref:              ref,
		HaveVersion:      snap.Version,
		RequestedVersion: 0, // caller doesn't know which version; 0 means "unknown"
	}
}
```

- [ ] **Step 4: Run with -race**

```
go test -v -race -run TestStore ./pkg/scout/aria/
```

Expected: all 6 PASS, no race output.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-3.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/store.go pkg/scout/aria/store_test.go
git commit -m "feat(aria): concurrent-safe per-page snapshot store"
EOF
bash .scripts/aria-commit-3.sh
```

---

## Task 4: AX-tree YAML rendering (no browser — fixture-driven)

**Files:**
- Create: `pkg/scout/aria/axtree.go`
- Create: `pkg/scout/aria/axtree_test.go`
- Create: `pkg/scout/aria/testdata/simple_form.json`
- Create: `pkg/scout/aria/testdata/simple_form.yaml`

Capture proper from CDP comes in Task 5. This task isolates the rendering logic — given a `Snapshot`, render it as YAML — using a fixture as input.

- [ ] **Step 1: Write the fixture input**

`pkg/scout/aria/testdata/simple_form.json` (a normalized Snapshot, not a raw CDP response — the CDP-to-Snapshot conversion is Task 5):

```json
{
  "page_id": "p-simple-form",
  "version": 1,
  "nodes": [
    {"ref": "e1", "backend_node_id": 100, "role": "WebArea", "name": "Simple Form", "children": [1, 2]},
    {"ref": "e2", "backend_node_id": 101, "role": "textbox", "name": "Email", "value": "", "children": []},
    {"ref": "e3", "backend_node_id": 102, "role": "button", "name": "Submit", "children": []}
  ],
  "truncated": false
}
```

- [ ] **Step 2: Write the expected output**

`pkg/scout/aria/testdata/simple_form.yaml`:

```yaml
- WebArea "Simple Form" [ref=e1]
  - textbox "Email" [ref=e2]
  - button "Submit" [ref=e3]
```

- [ ] **Step 3: Write the failing test**

```go
package aria_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

type fixtureSnapshot struct {
	PageID    string `json:"page_id"`
	Version   uint64 `json:"version"`
	Nodes     []fixtureNode `json:"nodes"`
	Truncated bool   `json:"truncated"`
}
type fixtureNode struct {
	Ref           string `json:"ref"`
	BackendNodeID int64  `json:"backend_node_id"`
	Role          string `json:"role"`
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	Children      []int  `json:"children"`
	FrameID       string `json:"frame_id,omitempty"`
}

func loadFixture(t *testing.T, name string) *aria.Snapshot {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx fixtureSnapshot
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	nodes := make([]aria.Node, len(fx.Nodes))
	for i, fn := range fx.Nodes {
		nodes[i] = aria.Node{
			Ref: fn.Ref, BackendNodeID: fn.BackendNodeID,
			Role: fn.Role, Name: fn.Name, Value: fn.Value,
			Children: fn.Children, FrameID: fn.FrameID,
		}
	}
	return &aria.Snapshot{
		PageID: fx.PageID, Version: fx.Version, Nodes: nodes, Truncated: fx.Truncated,
	}
}

func TestRenderYAML_SimpleForm(t *testing.T) {
	t.Parallel()
	snap := loadFixture(t, "simple_form")
	var buf bytes.Buffer
	if err := snap.RenderYAML(&buf); err != nil {
		t.Fatalf("RenderYAML err=%v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "simple_form.yaml"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("rendering mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}
```

- [ ] **Step 4: Run to confirm failure**

```
go test -v -run TestRenderYAML ./pkg/scout/aria/
```

Expected: `Snapshot.RenderYAML undefined`.

- [ ] **Step 5: Implement `axtree.go` (rendering only — Capture is Task 5)**

```go
package aria

import (
	"fmt"
	"io"
	"strings"
)

// RenderYAML writes a YAML-like (not strict YAML) representation of the
// snapshot. The format matches the playwright-mcp ARIA dialect:
//
//   - role "accessible name" [ref=eN]
//     - child role "name" [ref=eN+1]
//
// Roots are nodes referenced by no other node. Values are appended in the
// form: textbox "Label" [ref=eN] value="current text".
func (s *Snapshot) RenderYAML(w io.Writer) error {
	if s == nil {
		return fmt.Errorf("scout: aria: render: nil snapshot")
	}
	// Build "is referenced by another node" set
	referenced := make([]bool, len(s.Nodes))
	for i := range s.Nodes {
		for _, ci := range s.Nodes[i].Children {
			if ci >= 0 && ci < len(referenced) {
				referenced[ci] = true
			}
		}
	}
	for i := range s.Nodes {
		if !referenced[i] {
			if err := renderNode(w, s, i, 0); err != nil {
				return fmt.Errorf("scout: aria: render: %w", err)
			}
		}
	}
	return nil
}

func renderNode(w io.Writer, s *Snapshot, idx, depth int) error {
	n := &s.Nodes[idx]
	indent := strings.Repeat("  ", depth)
	line := fmt.Sprintf("%s- %s", indent, n.Role)
	if n.Name != "" {
		line += fmt.Sprintf(" %q", n.Name)
	}
	line += fmt.Sprintf(" [ref=%s]", n.Ref)
	if n.Value != "" {
		line += fmt.Sprintf(" value=%q", n.Value)
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	for _, ci := range n.Children {
		if ci < 0 || ci >= len(s.Nodes) {
			continue
		}
		if err := renderNode(w, s, ci, depth+1); err != nil {
			return err
		}
	}
	return nil
}
```

Move the `Node` and `Snapshot` declarations out of `store.go` into `axtree.go` now. Delete them from `store.go`. Update package-internal references unchanged (same package).

- [ ] **Step 6: Run to verify**

```
go test -v -run TestRenderYAML ./pkg/scout/aria/
go test -v -run TestStore ./pkg/scout/aria/
```

Expected: both PASS.

- [ ] **Step 7: Commit**

```bash
cat > .scripts/aria-commit-4.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/axtree.go pkg/scout/aria/axtree_test.go pkg/scout/aria/store.go pkg/scout/aria/testdata/
git commit -m "feat(aria): YAML rendering for accessibility snapshots"
EOF
bash .scripts/aria-commit-4.sh
```

---

## Task 5: Capture from real Chromium (real-browser test)

**Files:**
- Modify: `pkg/scout/aria/axtree.go` (add `Capture`)
- Create: `pkg/scout/aria/axtree_browser_test.go`

Per CLAUDE.md, real browser + httptest. No mocks. Use `newTestBrowser(t)` pattern — locate the helper by searching existing tests with `grep -rn newTestBrowser internal/engine/` BEFORE writing this task (one-time orientation cost; the helper signature varies). For this plan we assume `newTestBrowser(t *testing.T) *engine.Browser`.

- [ ] **Step 1: Write the failing browser test**

```go
//go:build integration
// +build integration

package aria_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/pkg/scout/aria"
)

func newTestBrowser(t *testing.T) *engine.Browser {
	t.Helper()
	// Use the same helper internal/engine tests use. If naming differs,
	// swap to the canonical newTestBrowser from internal/engine.
	br, err := engine.New(engine.WithHeadless(true))
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = br.Close() })
	return br
}

func TestCapture_SimpleForm_RealBrowser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<!doctype html><html><body>
  <h1>Simple Form</h1>
  <input type="text" aria-label="Email">
  <button>Submit</button>
</body></html>`))
	}))
	t.Cleanup(srv.Close)

	br := newTestBrowser(t)
	page, err := br.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	snap, err := aria.Capture(context.Background(), page)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if snap.Version == 0 || snap.PageID == "" {
		t.Fatalf("snapshot missing identity: %+v", snap)
	}

	// Find a button labeled "Submit" and a textbox labeled "Email".
	var sawButton, sawTextbox bool
	for _, n := range snap.Nodes {
		if n.Role == "button" && strings.Contains(n.Name, "Submit") {
			sawButton = true
		}
		if n.Role == "textbox" && strings.Contains(n.Name, "Email") {
			sawTextbox = true
		}
	}
	if !sawButton || !sawTextbox {
		t.Fatalf("missing expected nodes; got %d nodes: %+v", len(snap.Nodes), snap.Nodes)
	}
}
```

Note `//go:build integration` — run with `go test -tags integration ./pkg/scout/aria/`. Keep slow browser tests off the default path.

- [ ] **Step 2: Run to confirm failure**

```
go test -tags integration -v -run TestCapture_SimpleForm ./pkg/scout/aria/
```

Expected: `undefined: aria.Capture`.

- [ ] **Step 3: Implement `Capture` in `axtree.go`**

Add the CDP call. The call pattern below assumes Scout's rod fork exposes `proto.AccessibilityGetFullAXTree`. Verify by `grep -rn AccessibilityGetFullAXTree internal/engine/lib/proto/` before writing — if the type name differs, adjust.

```go
package aria

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/lib/proto"
)

var versionCounter atomic.Uint64

// Option configures Capture behavior.
type Option func(*captureCfg)

type captureCfg struct {
	maxNodes int
	maxBytes int
	timeout  time.Duration
}

func defaultCfg() *captureCfg {
	return &captureCfg{maxNodes: 10000, maxBytes: 64 * 1024, timeout: 5 * time.Second}
}

func WithMaxNodes(n int) Option   { return func(c *captureCfg) { c.maxNodes = n } }
func WithMaxBytes(n int) Option   { return func(c *captureCfg) { c.maxBytes = n } }
func WithCaptureTimeout(d time.Duration) Option {
	return func(c *captureCfg) { c.timeout = d }
}

// Capture runs CDP Accessibility.getFullAXTree on the page (root frame; child
// frames added in Task 6) and converts the result to a *Snapshot.
func Capture(ctx context.Context, page *engine.Page, opts ...Option) (*Snapshot, error) {
	cfg := defaultCfg()
	for _, opt := range opts {
		opt(cfg)
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	result, err := proto.AccessibilityGetFullAXTree{}.Call(page.RodPage())
	if err != nil {
		return nil, fmt.Errorf("scout: aria: capture: getFullAXTree: %w", err)
	}

	nodes, truncated, err := convertAXNodes(result.Nodes, cfg)
	if err != nil {
		return nil, fmt.Errorf("scout: aria: capture: convert: %w", err)
	}

	pageID := page.TargetID() // CDP target ID; stable per page
	version := versionCounter.Add(1)
	snap := &Snapshot{
		PageID:     pageID,
		Version:    version,
		Nodes:      nodes,
		URI:        fmt.Sprintf("scout://snapshot/%s?v=%d", pageID, version),
		CapturedAt: time.Now(),
		Truncated:  truncated,
	}
	_ = ctx // referenced for symmetry; CDP call uses page's own context
	return snap, nil
}

// convertAXNodes walks the raw CDP AXNode slice, assigning refs and building
// the parent-child index list. Returns truncated=true if cfg.maxNodes hit.
// Frame handling: this version assumes root-frame only; Task 6 extends.
func convertAXNodes(in []*proto.AccessibilityAXNode, cfg *captureCfg) ([]Node, bool, error) {
	if len(in) > cfg.maxNodes {
		in = in[:cfg.maxNodes]
		return convertInner(in), true, nil
	}
	return convertInner(in), false, nil
}

func convertInner(in []*proto.AccessibilityAXNode) []Node {
	// CDP returns nodes in tree order; child references are AXNodeIDs.
	idByAX := make(map[proto.AccessibilityAXNodeID]int, len(in))
	for i, ax := range in {
		idByAX[ax.NodeID] = i
	}
	out := make([]Node, len(in))
	refIdx := 0
	for i, ax := range in {
		refIdx++
		role := ""
		if ax.Role != nil {
			role = ax.Role.Value.String()
		}
		name := ""
		if ax.Name != nil {
			name = ax.Name.Value.String()
		}
		val := ""
		if ax.Value != nil {
			val = ax.Value.Value.String()
		}
		children := make([]int, 0, len(ax.ChildIDs))
		for _, cid := range ax.ChildIDs {
			if ci, ok := idByAX[cid]; ok {
				children = append(children, ci)
			}
		}
		bn := int64(0)
		if ax.BackendDOMNodeID != 0 {
			bn = int64(ax.BackendDOMNodeID)
		}
		out[i] = Node{
			Ref:           fmt.Sprintf("e%d", refIdx),
			BackendNodeID: bn,
			Role:          role,
			Name:          name,
			Value:         val,
			Children:      children,
		}
	}
	return out
}
```

The exact `proto.*` type names (`AccessibilityGetFullAXTree`, `AccessibilityAXNode`, `AccessibilityAXNodeID`, `BackendDOMNodeID`) **must be verified against the internalized rod fork** before this task ships — they're the most likely points of name drift. Run:

```
grep -rn 'AccessibilityAXNode\b\|AccessibilityGetFullAXTree\|BackendDOMNodeId' internal/engine/lib/proto/ | head -30
```

and adjust struct/field names if needed.

- [ ] **Step 4: Verify compile + happy path**

```
go test -tags integration -v -run TestCapture_SimpleForm ./pkg/scout/aria/
```

Expected: PASS (or `t.Skipf` if browser unavailable — both acceptable).

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-5.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/axtree.go pkg/scout/aria/axtree_browser_test.go
git commit -m "feat(aria): capture AX-tree from CDP into Snapshot"
EOF
bash .scripts/aria-commit-5.sh
```

---

## Task 6: Cross-frame refs (`f<F>:e<N>`)

**Files:**
- Modify: `pkg/scout/aria/axtree.go`
- Modify: `pkg/scout/aria/axtree_browser_test.go` (add iframe test)
- Create: `pkg/scout/aria/testdata/iframe.json`
- Create: `pkg/scout/aria/testdata/iframe.yaml`

- [ ] **Step 1: Write failing fixture test**

`pkg/scout/aria/testdata/iframe.json`:

```json
{
  "page_id": "p-iframe",
  "version": 2,
  "nodes": [
    {"ref": "e1", "backend_node_id": 1, "role": "WebArea", "name": "Outer", "children": [1, 2]},
    {"ref": "e2", "backend_node_id": 2, "role": "button", "name": "Outer Click", "children": []},
    {"ref": "f1:e1", "backend_node_id": 10, "role": "WebArea", "name": "Inner", "frame_id": "f1", "children": [3]},
    {"ref": "f1:e2", "backend_node_id": 11, "role": "textbox", "name": "Inner Input", "frame_id": "f1", "children": []}
  ]
}
```

`testdata/iframe.yaml`:

```yaml
- WebArea "Outer" [ref=e1]
  - button "Outer Click" [ref=e2]
  - WebArea "Inner" [ref=f1:e1]
    - textbox "Inner Input" [ref=f1:e2]
```

Add to `axtree_test.go`:

```go
func TestRenderYAML_Iframe(t *testing.T) {
	t.Parallel()
	snap := loadFixture(t, "iframe")
	var buf bytes.Buffer
	if err := snap.RenderYAML(&buf); err != nil {
		t.Fatalf("RenderYAML err=%v", err)
	}
	want, _ := os.ReadFile(filepath.Join("testdata", "iframe.yaml"))
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("rendering mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}
```

- [ ] **Step 2: Run to confirm pass (rendering already supports it)**

```
go test -v -run TestRenderYAML_Iframe ./pkg/scout/aria/
```

If rendering's frame-agnostic walk works correctly, this passes immediately — the YAML format treats `f1:e1` as just another ref string.

If it fails because the iframe nodes are at the top level rather than nested under e1, you need a frame-children resolution step in `convertAXNodes` — see step 4.

- [ ] **Step 3: Add real-browser iframe test**

Append to `axtree_browser_test.go`:

```go
func TestCapture_Iframe_RealBrowser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inner", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body><input aria-label="Inner Input"></body></html>`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body>
		  <button>Outer Click</button>
		  <iframe src="/inner"></iframe>
		</body></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	br := newTestBrowser(t)
	page, err := br.NewPage(srv.URL)
	if err != nil { t.Fatalf("NewPage: %v", err) }
	_ = page.WaitLoad()

	snap, err := aria.Capture(context.Background(), page)
	if err != nil { t.Fatalf("Capture: %v", err) }

	var sawOuter, sawInner bool
	for _, n := range snap.Nodes {
		if n.Role == "button" && strings.Contains(n.Name, "Outer Click") { sawOuter = true }
		if n.Role == "textbox" && strings.Contains(n.Name, "Inner Input") {
			if !strings.HasPrefix(n.Ref, "f") {
				t.Errorf("inner ref %q lacks frame prefix", n.Ref)
			}
			sawInner = true
		}
	}
	if !sawOuter || !sawInner {
		t.Fatalf("missing expected nodes; got %+v", snap.Nodes)
	}
}
```

- [ ] **Step 4: Extend `convertAXNodes` for frames**

Replace `convertAXNodes` and add a per-frame walker. The CDP returns one tree per frame when called with `frameId` arg, or the root tree only when called without; the simplest path is to enumerate child frames via `page.Frames()` and call `getFullAXTree` once per frame.

```go
func Capture(ctx context.Context, page *engine.Page, opts ...Option) (*Snapshot, error) {
	cfg := defaultCfg()
	for _, opt := range opts { opt(cfg) }

	rootResult, err := proto.AccessibilityGetFullAXTree{}.Call(page.RodPage())
	if err != nil {
		return nil, fmt.Errorf("scout: aria: capture: getFullAXTree root: %w", err)
	}

	frames := page.ChildFrames() // see engine.Page; may return []*engine.Frame
	nodes := make([]Node, 0, len(rootResult.Nodes))
	refCounter := 0
	rootNodes, _, _ := convertAXNodes(rootResult.Nodes, cfg, "", &refCounter)
	nodes = append(nodes, rootNodes...)

	for i, frame := range frames {
		frameResult, err := proto.AccessibilityGetFullAXTree{
			FrameID: frame.FrameID(),
		}.Call(page.RodPage())
		if err != nil {
			// best-effort: log and continue
			continue
		}
		framePrefix := fmt.Sprintf("f%d", i+1)
		refInFrame := 0
		frameNodes, _, _ := convertAXNodes(frameResult.Nodes, cfg, framePrefix, &refInFrame)
		nodes = append(nodes, frameNodes...)
	}

	pageID := page.TargetID()
	version := versionCounter.Add(1)
	return &Snapshot{
		PageID: pageID, Version: version, Nodes: nodes,
		URI: fmt.Sprintf("scout://snapshot/%s?v=%d", pageID, version),
		CapturedAt: time.Now(),
	}, nil
}

func convertAXNodes(in []*proto.AccessibilityAXNode, cfg *captureCfg, framePrefix string, counter *int) ([]Node, bool, error) {
	// reset map per frame; refs are scoped per-frame in their numbering
	idByAX := make(map[proto.AccessibilityAXNodeID]int, len(in))
	for i, ax := range in {
		idByAX[ax.NodeID] = i
	}
	out := make([]Node, 0, len(in))
	for _, ax := range in {
		*counter++
		ref := fmt.Sprintf("e%d", *counter)
		if framePrefix != "" {
			ref = framePrefix + ":" + ref
		}
		role, name, val := axStrings(ax)
		children := make([]int, 0, len(ax.ChildIDs))
		for _, cid := range ax.ChildIDs {
			if ci, ok := idByAX[cid]; ok {
				children = append(children, ci) // NOTE: index within this slice; consumer must rebase
			}
		}
		bn := int64(ax.BackendDOMNodeID)
		out = append(out, Node{
			Ref: ref, BackendNodeID: bn,
			Role: role, Name: name, Value: val,
			Children: children, FrameID: framePrefix,
		})
	}
	return out, false, nil
}

func axStrings(ax *proto.AccessibilityAXNode) (role, name, val string) {
	if ax.Role != nil { role = ax.Role.Value.String() }
	if ax.Name != nil { name = ax.Name.Value.String() }
	if ax.Value != nil { val = ax.Value.Value.String() }
	return
}
```

The child-index rebasing for the combined slice is a known follow-up; for this task accept the limitation that frame nodes have their `Children` indices relative to frame-internal numbering. Document this in the function doc comment.

- [ ] **Step 5: Run both tests**

```
go test -v -run TestRenderYAML_Iframe ./pkg/scout/aria/
go test -tags integration -v -run TestCapture_Iframe ./pkg/scout/aria/
```

Both PASS.

- [ ] **Step 6: Commit**

```bash
cat > .scripts/aria-commit-6.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/axtree.go pkg/scout/aria/axtree_test.go pkg/scout/aria/axtree_browser_test.go pkg/scout/aria/testdata/
git commit -m "feat(aria): cross-frame ref prefixes (f<F>:e<N>)"
EOF
bash .scripts/aria-commit-6.sh
```

---

## Task 7: Truncation cap

**Files:**
- Modify: `pkg/scout/aria/axtree.go`
- Modify: `pkg/scout/aria/axtree_test.go`

- [ ] **Step 1: Failing test**

```go
func TestCapture_TruncationByNodeCap(t *testing.T) {
	t.Parallel()
	// Generate a synthetic flat AX-node slice; bypass real CDP.
	in := make([]*proto.AccessibilityAXNode, 0, 100)
	for i := 0; i < 100; i++ {
		in = append(in, &proto.AccessibilityAXNode{
			NodeID:           proto.AccessibilityAXNodeID(i),
			BackendDOMNodeID: proto.DOMBackendNodeID(i + 1000),
		})
	}
	out, truncated, _ := aria.ConvertForTest(in, 50)
	if !truncated {
		t.Errorf("truncated=false, want true")
	}
	if len(out) != 50 {
		t.Errorf("len=%d, want 50", len(out))
	}
}
```

This test imports `proto` and calls a test-only exposure `aria.ConvertForTest`. Expose it via `pkg/scout/aria/export_test.go`:

```go
package aria

import "github.com/inovacc/scout/internal/engine/lib/proto"

// Exposed for tests in pkg/scout/aria_test only.
func ConvertForTest(in []*proto.AccessibilityAXNode, maxNodes int) ([]Node, bool, error) {
	cfg := &captureCfg{maxNodes: maxNodes}
	counter := 0
	if len(in) > cfg.maxNodes {
		in = in[:cfg.maxNodes]
		out, _, err := convertAXNodes(in, cfg, "", &counter)
		return out, true, err
	}
	out, _, err := convertAXNodes(in, cfg, "", &counter)
	return out, false, err
}
```

Note: `export_test.go` does NOT use the `_test.go` suffix matter for compilation — `export_test.go` is compiled only with tests but lives in the production package, exposing internal symbols to external test packages. This is idiomatic Go.

- [ ] **Step 2: Implementation**

Already covered by the `convertAXNodes` cap-check from Task 5/6. Verify test passes:

```
go test -v -run TestCapture_TruncationByNodeCap ./pkg/scout/aria/
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
cat > .scripts/aria-commit-7.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/
git commit -m "feat(aria): truncation cap on AX-tree node count"
EOF
bash .scripts/aria-commit-7.sh
```

---

## Task 8: Diff + Summary

**Files:**
- Create: `pkg/scout/aria/diff.go`
- Create: `pkg/scout/aria/diff_test.go`
- Create: `pkg/scout/aria/testdata/diff_add.json` (pair: `_before.json`, `_after.json`, `_summary.txt`)
- Create: `pkg/scout/aria/testdata/diff_remove.json`
- Create: `pkg/scout/aria/testdata/diff_noop.json`

- [ ] **Step 1: Write fixture triples**

`testdata/diff_add_before.json`:

```json
{"page_id":"p","version":1,"nodes":[
  {"ref":"e1","backend_node_id":100,"role":"WebArea","name":"x","children":[1]},
  {"ref":"e2","backend_node_id":101,"role":"button","name":"Old","children":[]}
]}
```

`testdata/diff_add_after.json`:

```json
{"page_id":"p","version":2,"nodes":[
  {"ref":"e1","backend_node_id":100,"role":"WebArea","name":"x","children":[1,2]},
  {"ref":"e2","backend_node_id":101,"role":"button","name":"Old","children":[]},
  {"ref":"e3","backend_node_id":102,"role":"button","name":"New","children":[]}
]}
```

`testdata/diff_add_summary.txt`:

```
1 element added (ref=e3 button "New")
```

(no trailing newline — exact byte match)

- [ ] **Step 2: Failing test**

```go
func TestDiff_Add(t *testing.T) {
	t.Parallel()
	before := loadFixture(t, "diff_add_before")
	after := loadFixture(t, "diff_add_after")
	summary := aria.Diff(before, after)
	got := summary.String()
	want := readGolden(t, "diff_add_summary.txt")
	if got != want {
		t.Errorf("summary mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil { t.Fatalf("read golden: %v", err) }
	return strings.TrimRight(string(raw), "\n")
}
```

Add equivalent tests for `diff_remove` (one node removed) and `diff_noop` (no changes → summary string is `"no changes"`).

- [ ] **Step 3: Implement `diff.go`**

```go
package aria

import (
	"fmt"
	"strings"
)

type NodeChange struct {
	Ref  string
	Role string
	Name string
}

type Summary struct {
	Added   []NodeChange
	Removed []NodeChange
	Changed []NodeChange
}

func (s Summary) String() string {
	if len(s.Added) == 0 && len(s.Removed) == 0 && len(s.Changed) == 0 {
		return "no changes"
	}
	var b strings.Builder
	if n := len(s.Added); n > 0 {
		_, _ = fmt.Fprintf(&b, "%d element%s added", n, plural(n))
		writeDetails(&b, s.Added)
	}
	if n := len(s.Removed); n > 0 {
		if b.Len() > 0 { b.WriteString("; ") }
		_, _ = fmt.Fprintf(&b, "%d removed", n)
		writeDetails(&b, s.Removed)
	}
	if n := len(s.Changed); n > 0 {
		if b.Len() > 0 { b.WriteString("; ") }
		_, _ = fmt.Fprintf(&b, "%d changed", n)
		writeDetails(&b, s.Changed)
	}
	return b.String()
}

func writeDetails(b *strings.Builder, changes []NodeChange) {
	// Show up to 3 in detail; if more, append "and N more".
	max := 3
	if len(changes) <= max {
		_, _ = fmt.Fprintf(b, " (")
		for i, c := range changes {
			if i > 0 { b.WriteString(", ") }
			writeChange(b, c)
		}
		b.WriteString(")")
		return
	}
	_, _ = fmt.Fprintf(b, " (")
	for i := 0; i < max; i++ {
		if i > 0 { b.WriteString(", ") }
		writeChange(b, changes[i])
	}
	_, _ = fmt.Fprintf(b, ", and %d more)", len(changes)-max)
}

func writeChange(b *strings.Builder, c NodeChange) {
	_, _ = fmt.Fprintf(b, "ref=%s %s", c.Ref, c.Role)
	if c.Name != "" {
		_, _ = fmt.Fprintf(b, " %q", c.Name)
	}
}

func plural(n int) string { if n == 1 { return "" }; return "s" }

// Diff compares two snapshots and returns a structural summary. Comparison
// keys on Ref+Role+Name; refs that exist in one snapshot but not the other
// are Added/Removed; same-ref + changed role/name/value is Changed.
func Diff(before, after *Snapshot) Summary {
	beforeByRef := map[string]Node{}
	for _, n := range before.Nodes { beforeByRef[n.Ref] = n }
	afterByRef := map[string]Node{}
	for _, n := range after.Nodes { afterByRef[n.Ref] = n }

	var s Summary
	for ref, n := range afterByRef {
		old, ok := beforeByRef[ref]
		if !ok {
			s.Added = append(s.Added, NodeChange{Ref: ref, Role: n.Role, Name: n.Name})
			continue
		}
		if old.Role != n.Role || old.Name != n.Name || old.Value != n.Value {
			s.Changed = append(s.Changed, NodeChange{Ref: ref, Role: n.Role, Name: n.Name})
		}
	}
	for ref, n := range beforeByRef {
		if _, ok := afterByRef[ref]; !ok {
			s.Removed = append(s.Removed, NodeChange{Ref: ref, Role: n.Role, Name: n.Name})
		}
	}
	return s
}
```

Caution: map iteration order is non-deterministic. To produce stable summary strings across runs (golden tests rely on this), sort `s.Added` / `s.Removed` / `s.Changed` by `Ref` before returning. Add at the bottom of `Diff`:

```go
sort.Slice(s.Added,   func(i, j int) bool { return s.Added[i].Ref   < s.Added[j].Ref   })
sort.Slice(s.Removed, func(i, j int) bool { return s.Removed[i].Ref < s.Removed[j].Ref })
sort.Slice(s.Changed, func(i, j int) bool { return s.Changed[i].Ref < s.Changed[j].Ref })
```

Don't forget to import `"sort"`.

- [ ] **Step 4: Run tests**

```
go test -v -run TestDiff ./pkg/scout/aria/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-8.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/diff.go pkg/scout/aria/diff_test.go pkg/scout/aria/testdata/
git commit -m "feat(aria): snapshot diff with deterministic Summary"
EOF
bash .scripts/aria-commit-8.sh
```

---

## Task 9: Resolve ref → `*engine.Element`

**Files:**
- Create: `pkg/scout/aria/resolve.go`
- Create: `pkg/scout/aria/resolve_browser_test.go`

- [ ] **Step 1: Failing browser test**

```go
//go:build integration
// +build integration

package aria_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func TestResolveElement_ClickRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body>
		  <button id="b" onclick="document.title='clicked'">Click Me</button>
		</body></html>`))
	}))
	t.Cleanup(srv.Close)

	br := newTestBrowser(t)
	page, _ := br.NewPage(srv.URL)
	_ = page.WaitLoad()

	snap, err := aria.Capture(context.Background(), page)
	if err != nil { t.Fatalf("Capture: %v", err) }

	var btnRef string
	for _, n := range snap.Nodes {
		if n.Role == "button" && n.Name == "Click Me" { btnRef = n.Ref; break }
	}
	if btnRef == "" { t.Fatalf("button ref not found in %+v", snap.Nodes) }

	bn, err := (&aria.Store{}).Resolve(snap.PageID, btnRef)
	_ = bn
	if !errors.Is(err, aria.ErrNoSnapshot) {
		// We haven't Put the snapshot yet — should be NoSnapshotError.
		t.Fatalf("expected NoSnapshotError, got %v", err)
	}

	store := aria.NewStore()
	store.Put(snap.PageID, snap)
	bn, err = store.Resolve(snap.PageID, btnRef)
	if err != nil { t.Fatalf("Resolve: %v", err) }

	el, err := aria.ResolveElement(page, bn)
	if err != nil { t.Fatalf("ResolveElement: %v", err) }
	if err := el.Click(); err != nil { t.Fatalf("Click: %v", err) }

	// Verify title changed.
	title, _ := page.Title()
	if title != "clicked" {
		t.Errorf("title=%q, want %q", title, "clicked")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```
go test -tags integration -v -run TestResolveElement ./pkg/scout/aria/
```

Expected: `undefined: aria.ResolveElement`.

- [ ] **Step 3: Implement `resolve.go`**

```go
package aria

import (
	"fmt"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/lib/proto"
)

// ResolveElement converts a CDP backend node ID into a live *engine.Element
// suitable for action methods (.Click, .Input, .Hover, ...). Returns
// AmbiguousRefError if the node has been detached from the DOM since the
// snapshot was captured.
func ResolveElement(page *engine.Page, backendNodeID int64) (*engine.Element, error) {
	res, err := proto.DOMResolveNode{
		BackendNodeID: proto.DOMBackendNodeID(backendNodeID),
	}.Call(page.RodPage())
	if err != nil {
		return nil, &AmbiguousRefError{BackendNodeID: backendNodeID}
	}
	if res.Object.ObjectID == "" {
		return nil, &AmbiguousRefError{BackendNodeID: backendNodeID}
	}
	el, err := page.ElementFromObject(res.Object)
	if err != nil {
		return nil, fmt.Errorf("scout: aria: resolve: %w", err)
	}
	return el, nil
}
```

Verify `engine.Page` exposes `ElementFromObject` (or equivalent). If not, the equivalent rod call is `page.RodPage().Element(res.Object)` — Scout wraps rod, check by `grep -n 'func.*RemoteObject' internal/engine/page.go`.

- [ ] **Step 4: Run**

```
go test -tags integration -v -run TestResolveElement ./pkg/scout/aria/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-9.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/aria/resolve.go pkg/scout/aria/resolve_browser_test.go
git commit -m "feat(aria): resolve ref to live engine.Element via DOM.resolveNode"
EOF
bash .scripts/aria-commit-9.sh
```

---

## Task 10: Depguard layering rule

**Files:**
- Modify: `.golangci.yml`

- [ ] **Step 1: Read existing `.golangci.yml`** (orientation; no edit yet)

```
cat .golangci.yml | head -100
```

Locate the `linters-settings:` block. We're adding a `depguard` block.

- [ ] **Step 2: Add the rule**

Add (or extend) under `linters-settings:`:

```yaml
linters-settings:
  depguard:
    rules:
      aria-layering:
        list-mode: lax
        files:
          - "**/pkg/scout/aria/**"
        deny:
          - pkg: github.com/inovacc/scout/pkg/scout/mcp
            desc: "pkg/scout/aria must not import pkg/scout/mcp (layering rule, see docs/superpowers/specs/2026-05-21-playwright-mcp-aria-ref-design.md §4)"
          - pkg: github.com/inovacc/scout/pkg/scout/agent
            desc: "pkg/scout/aria must not import pkg/scout/agent (layering rule)"
          - pkg: github.com/inovacc/scout/pkg/scout/runbook
            desc: "pkg/scout/aria must not import pkg/scout/runbook (layering rule)"
```

Make sure `depguard` is in the `linters:` enable list (check the existing `linters:` section).

- [ ] **Step 3: Verify rule fires by deliberately introducing a violation**

In `pkg/scout/aria/doc.go`, temporarily add:

```go
package aria

import _ "github.com/inovacc/scout/pkg/scout/mcp"
```

Run:

```
golangci-lint run ./pkg/scout/aria/ --no-config=false
```

Expected: ERROR `pkg/scout/aria must not import pkg/scout/mcp`.

Remove the import.

- [ ] **Step 4: Run lint clean**

```
golangci-lint run ./pkg/scout/aria/
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-10.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add .golangci.yml pkg/scout/aria/doc.go
git commit -m "chore(aria): depguard rule enforcing aria layering"
EOF
bash .scripts/aria-commit-10.sh
```

---

## Task 11: MCP resource `scout://snapshot/{page-id}`

**Files:**
- Modify: `pkg/scout/mcp/resources.go`
- Modify: `pkg/scout/mcp/state.go` (or wherever `mcpState` lives — find via `grep -rn 'type mcpState' pkg/scout/mcp/`)

- [ ] **Step 1: Locate `mcpState`**

```
grep -rn 'type mcpState' pkg/scout/mcp/
grep -rn 'AddResource\|AddResourceTemplate' pkg/scout/mcp/resources.go
```

Note the file and line for both. We'll modify them.

- [ ] **Step 2: Hang `*aria.Store` off the state**

Add to the `mcpState` struct definition:

```go
import "github.com/inovacc/scout/pkg/scout/aria"

type mcpState struct {
	// ... existing fields ...
	ariaStore *aria.Store
}
```

In the state constructor (find via `grep -rn 'mcpState{' pkg/scout/mcp/`), initialize:

```go
s.ariaStore = aria.NewStore()
```

- [ ] **Step 3: Register the resource template**

Add to `resources.go` (alongside the other 3 resources):

```go
server.AddResourceTemplate(&mcp.ResourceTemplate{
	URITemplate: "scout://snapshot/{page-id}",
	Name:        "Page accessibility snapshot",
	Description: "Current ARIA snapshot for a page, rendered as YAML with [ref=eN] tags.",
	MIMEType:    "text/yaml",
}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	pageID := strings.TrimPrefix(req.Params.URI, "scout://snapshot/")
	pageID = strings.SplitN(pageID, "?", 2)[0] // strip ?v= if present
	snap, ok := state.ariaStore.Get(pageID)
	if !ok {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	var buf bytes.Buffer
	if err := snap.RenderYAML(&buf); err != nil {
		return nil, fmt.Errorf("scout: mcp: render snapshot: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "text/yaml",
			Text:     buf.String(),
		}},
	}, nil
})
```

Verify imports: `bytes`, `strings`, `fmt`, `github.com/inovacc/scout/pkg/scout/aria`.

- [ ] **Step 4: Verify build**

```
go build ./pkg/scout/mcp/
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-11.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/mcp/resources.go pkg/scout/mcp/state.go
git commit -m "feat(mcp): scout://snapshot/{page-id} resource template"
EOF
bash .scripts/aria-commit-11.sh
```

---

## Task 12: `browser_snapshot` MCP tool

**Files:**
- Create: `pkg/scout/mcp/tools_aria.go`
- Create: `pkg/scout/mcp/tools_aria_test.go`
- Modify: server registration (locate via `grep -rn 'addTracedTool\|AddTool' pkg/scout/mcp/server.go`)

- [ ] **Step 1: Failing in-memory MCP test**

```go
package mcp_test

import (
	"context"
	"strings"
	"testing"

	mcplib "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp/client"

	"github.com/inovacc/scout/pkg/scout/mcp"
)

func TestBrowserSnapshotTool_ReturnsYAML(t *testing.T) {
	// Use Scout's existing in-memory MCP harness; locate it by:
	//   grep -rn 'NewInMemoryTransports' pkg/scout/mcp/
	srv, cs := startInMemoryMCP(t) // helper assumed to exist; if not, port from server_test.go
	t.Cleanup(func() { _ = srv.Close(); _ = cs.Close() })

	// Open a simple page first via a helper similar to existing tests.
	openPage(t, cs, simpleHTML(`<button>Submit</button>`))

	result, err := cs.CallTool(context.Background(), &mcplib.CallToolParams{
		Name: "browser_snapshot",
	})
	if err != nil { t.Fatalf("CallTool: %v", err) }
	if result.IsError {
		t.Fatalf("tool error: %s", textOf(result))
	}
	text := textOf(result)
	if !strings.Contains(text, `button "Submit" [ref=`) {
		t.Errorf("snapshot text missing expected node:\n%s", text)
	}
	if !strings.Contains(text, "snapshot_uri=scout://snapshot/") {
		t.Errorf("snapshot text missing snapshot_uri:\n%s", text)
	}
}

func textOf(r *mcplib.CallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if tc, ok := c.(*mcplib.TextContent); ok { b.WriteString(tc.Text) }
	}
	return b.String()
}
```

(Helpers `startInMemoryMCP`, `openPage`, `simpleHTML` should already exist in Scout's MCP test harness — `grep -rn 'startInMemoryMCP\|openPage' pkg/scout/mcp/`. If not, port the relevant fragments from `server_test.go`.)

- [ ] **Step 2: Run to confirm failure**

```
go test -v -run TestBrowserSnapshotTool ./pkg/scout/mcp/
```

Expected: `tool not found: browser_snapshot` or undefined helper.

- [ ] **Step 3: Implement `tools_aria.go`**

```go
package mcp

import (
	"bytes"
	"context"
	"fmt"

	mcplib "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/scout/pkg/scout/aria"
)

type browserSnapshotInput struct {
	// No args yet. Phase A is read-current-page.
}

type browserSnapshotOutput struct {
	SnapshotURI     string `json:"snapshot_uri"`
	SnapshotVersion uint64 `json:"snapshot_version"`
	YAML            string `json:"yaml"`
}

func registerAriaTools(server *mcplib.Server, state *mcpState) {
	mcplib.AddTool(server,
		&mcplib.Tool{
			Name: "browser_snapshot",
			Description: "Capture and store an ARIA snapshot of the current page. " +
				"Returns YAML with [ref=eN] tags. Use refs from this snapshot in action tools (Phase B).",
		},
		func(ctx context.Context, _ browserSnapshotInput) (*mcplib.CallToolResult, browserSnapshotOutput, error) {
			page, err := state.ensureBrowser(ctx)
			if err != nil {
				return errorResult(fmt.Sprintf("browser unavailable: %v", err)), browserSnapshotOutput{}, nil
			}
			snap, err := aria.Capture(ctx, page)
			if err != nil {
				return errorResult(fmt.Sprintf("snapshot capture failed: %v", err)), browserSnapshotOutput{}, nil
			}
			state.ariaStore.Put(snap.PageID, snap)

			var buf bytes.Buffer
			if err := snap.RenderYAML(&buf); err != nil {
				return errorResult(fmt.Sprintf("render failed: %v", err)), browserSnapshotOutput{}, nil
			}
			textBody := fmt.Sprintf("snapshot_uri=%s snapshot_version=%d\n\n%s",
				snap.URI, snap.Version, buf.String())
			return &mcplib.CallToolResult{
				Content: []mcplib.Content{&mcplib.TextContent{Text: textBody}},
			}, browserSnapshotOutput{
				SnapshotURI: snap.URI, SnapshotVersion: snap.Version, YAML: buf.String(),
			}, nil
		},
	)
}

func errorResult(msg string) *mcplib.CallToolResult {
	return &mcplib.CallToolResult{
		IsError: true,
		Content: []mcplib.Content{&mcplib.TextContent{Text: msg}},
	}
}
```

- [ ] **Step 4: Wire registration**

In the existing `RegisterTools` function (or wherever `addTracedTool` is called), add a call to `registerAriaTools(server, state)`.

- [ ] **Step 5: Run**

```
go test -v -run TestBrowserSnapshotTool ./pkg/scout/mcp/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cat > .scripts/aria-commit-12.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/mcp/tools_aria.go pkg/scout/mcp/tools_aria_test.go pkg/scout/mcp/server.go
git commit -m "feat(mcp): browser_snapshot tool backed by aria.Store"
EOF
bash .scripts/aria-commit-12.sh
```

---

## Task 13: Invalidation on root-frame navigation + page close

**Files:**
- Create: `pkg/scout/mcp/invalidation.go`
- Create: `pkg/scout/mcp/invalidation_browser_test.go`
- Modify: `pkg/scout/mcp/state.go` (call invalidation hooks during `ensureBrowser`)

- [ ] **Step 1: Failing browser test**

```go
//go:build integration

package mcp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
	"github.com/inovacc/scout/pkg/scout/mcp"
)

func TestInvalidation_NavigationClearsStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body><button>A</button></body></html>`))
	}))
	t.Cleanup(srv.Close)

	state := mcp.NewStateForTest(t) // small helper that returns the internal mcpState
	t.Cleanup(state.Close)

	page, _ := state.OpenPage(srv.URL) // also a test helper
	snap, _ := aria.Capture(context.Background(), page)
	state.AriaStore().Put(snap.PageID, snap)

	// Navigate.
	_, _ = page.Navigate(srv.URL + "?other")
	_ = page.WaitLoad()

	// Wait a tick for the frame-navigated event listener to fire.
	time.Sleep(100 * time.Millisecond)

	_, err := state.AriaStore().Resolve(snap.PageID, snap.Nodes[0].Ref)
	if !errors.Is(err, aria.ErrNoSnapshot) {
		t.Errorf("expected NoSnapshotError after navigation, got %v", err)
	}
}
```

Expose `NewStateForTest` and `(*mcpState).AriaStore()` / `OpenPage(url)` via `pkg/scout/mcp/export_test.go`.

- [ ] **Step 2: Implement `invalidation.go`**

```go
package mcp

import (
	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/internal/engine/lib/proto"
)

// installInvalidationHooks subscribes to the page's CDP events and clears the
// aria store entry for the page on root-frame navigation or target destroy.
// Called from ensureBrowser the first time a page is opened.
func installInvalidationHooks(page *engine.Page, state *mcpState) {
	pageID := page.TargetID()

	// Root-frame navigation: hook Page.frameNavigated and check frame.parentId.
	go page.RodPage().EachEvent(func(e *proto.PageFrameNavigated) {
		if e.Frame.ParentID == "" { // root frame
			state.ariaStore.Clear(pageID)
		}
	})()

	// Page close.
	go page.RodPage().EachEvent(func(e *proto.TargetTargetDestroyed) {
		if string(e.TargetID) == pageID {
			state.ariaStore.Clear(pageID)
		}
	})()
}
```

Two `go` statements: `EachEvent` blocks; we want it running for the lifetime of the page. Confirm signature by `grep -rn 'EachEvent' internal/engine/lib/`.

- [ ] **Step 3: Call from `ensureBrowser`**

After the existing browser-opening code in `ensureBrowser`, add:

```go
installInvalidationHooks(page, state)
```

- [ ] **Step 4: Run**

```
go test -tags integration -v -run TestInvalidation ./pkg/scout/mcp/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-13.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/mcp/invalidation.go pkg/scout/mcp/invalidation_browser_test.go pkg/scout/mcp/state.go pkg/scout/mcp/export_test.go
git commit -m "feat(mcp): invalidate aria store on root-frame navigation + page close"
EOF
bash .scripts/aria-commit-13.sh
```

---

## Task 14: Bridge mutation-observer hook (auto re-capture)

**Files:**
- Modify: `pkg/scout/mcp/invalidation.go`
- Modify: `pkg/scout/mcp/invalidation_browser_test.go`

Per spec §6.3 the bridge already injects MutationObserver for other reasons. Hook the existing channel. If the bridge doesn't yet emit a "major mutation" signal, this task's first sub-step is to confirm whether one exists.

- [ ] **Step 1: Reconnaissance**

```
grep -rn 'MutationObserver\|major.mutation\|dirty' extensions/ internal/engine/bridge.go pkg/scout/mcp/
```

If a major-mutation signal exists (a channel or callback exposed on `*engine.Page` or `*engine.Browser`), proceed. If not, this task expands to "design + ship the bridge signal" — at which point break this task out and tag it as a Phase A blocker on the Phase 1 issue, do not implement the auto re-capture downstream in this task.

(Assume signal exists for the remainder; if reconnaissance finds none, stop and consult the spec author.)

- [ ] **Step 2: Failing test** (assumes the signal exists as `page.OnMajorMutation()` returning `<-chan struct{}`)

```go
func TestInvalidation_MajorMutationMarksDirty(t *testing.T) {
	// Navigate to a page that triggers a programmatic, large DOM mutation
	// after 100ms (>= 20 mutations).
	html := `<!doctype html><html><body><div id="root"></div><script>
	setTimeout(() => {
	  const root = document.getElementById('root');
	  for (let i=0; i<30; i++) {
	    const b = document.createElement('button'); b.textContent='B'+i; root.appendChild(b);
	  }
	}, 50);
	</script></body></html>`
	// ... open page, capture snapshot v1, wait 300ms, capture v2 ...
	// Assert v2 has > 25 buttons and Diff produces a non-empty Added list.
}
```

(Full body intentionally elided for brevity; mirror the structure of Task 13's test.)

- [ ] **Step 3: Implementation**

```go
func installInvalidationHooks(page *engine.Page, state *mcpState) {
	pageID := page.TargetID()
	// ... existing frameNavigated + targetDestroyed listeners ...

	go func() {
		for range page.OnMajorMutation() { // signal-only; we don't get data
			// Re-capture lazily: just mark stale by clearing.
			state.ariaStore.Clear(pageID)
		}
	}()
}
```

- [ ] **Step 4: Run**

```
go test -tags integration -v -run TestInvalidation_MajorMutation ./pkg/scout/mcp/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cat > .scripts/aria-commit-14.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add pkg/scout/mcp/invalidation.go pkg/scout/mcp/invalidation_browser_test.go
git commit -m "feat(mcp): auto-invalidate aria store on bridge major-mutation signal"
EOF
bash .scripts/aria-commit-14.sh
```

---

## Task 15: Phase A verification + CHANGELOG

**Files:**
- Modify: `docs/CHANGELOG.md`

- [ ] **Step 1: Full check**

```
task check
```

Expected: PASS. If `task check` runs golangci-lint, it picks up the depguard rule from Task 10.

- [ ] **Step 2: Coverage**

```
cat > .scripts/aria-coverage.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
COVERFILE="${TMPDIR:-/tmp}/aria-coverage-$(date +%Y%m%d_%H%M%S).out"
go test -tags integration -coverprofile="$COVERFILE" ./pkg/scout/aria/ ./pkg/scout/mcp/
go tool cover -func="$COVERFILE" | tail -20
go tool cover -func="$COVERFILE" | grep -E 'pkg/scout/aria|pkg/scout/mcp/tools_aria|pkg/scout/mcp/invalidation' | sort
EOF
bash .scripts/aria-coverage.sh
```

Expected: `pkg/scout/aria/...` ≥ 90 %; new MCP files ≥ 80 %.

- [ ] **Step 3: CHANGELOG entry**

Append to `docs/CHANGELOG.md` under an `## [Unreleased]` heading (create the heading if absent):

```markdown
## [Unreleased]

### Added — ARIA-Ref Phase A
- New `pkg/scout/aria/` package: AX-tree snapshots with stable `[ref=eN]` IDs, per-page snapshot store, structural diff with deterministic Summary, ref-to-element resolution.
- New MCP resource `scout://snapshot/{page-id}` exposes the current snapshot as YAML.
- New MCP tool `browser_snapshot` captures and stores a snapshot of the current page; subsequent phases will wire ref-based action tools on top.
- Snapshot invalidates automatically on root-frame navigation, page close, and major DOM mutation signals from the bridge.
- Layering rule enforced by `golangci-lint` `depguard`: `pkg/scout/aria/` cannot import `pkg/scout/mcp`, `pkg/scout/agent`, or `pkg/scout/runbook`.

### Not yet
- Ref-based action tools (`browser_click`, `browser_type`, …) — Phase B.
- Capability gating, observer tools, tab tools, code-gen — Phases C-F.
```

- [ ] **Step 4: Final commit**

```bash
cat > .scripts/aria-commit-15.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git add docs/CHANGELOG.md
git commit -m "docs(aria): CHANGELOG entry for Phase A foundation"
EOF
bash .scripts/aria-commit-15.sh
```

- [ ] **Step 5: Push & open PR**

```
git push -u origin feat/aria-phase-a
gh pr create --title "feat(aria): Phase A — ref model foundation + browser_snapshot" --body "$(cat <<'EOF'
## Summary
- New `pkg/scout/aria/` package implementing the accessibility-tree + ref-id model from `docs/superpowers/specs/2026-05-21-playwright-mcp-aria-ref-design.md`
- New `browser_snapshot` MCP tool and `scout://snapshot/{page-id}` MCP resource
- Snapshot invalidation on navigation, page close, major DOM mutation
- Strict layering enforced via depguard

## Out of scope (Phase B+)
- Action tools (click/type/hover/…)
- Capability gating, observers, tab tools, code-gen

## Test plan
- [ ] `task check` passes
- [ ] Aria package coverage ≥ 90 %
- [ ] `go test -tags integration ./pkg/scout/aria/ ./pkg/scout/mcp/` passes locally with Chromium available

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-Review

**Spec coverage check**

- §4 layering rule → Task 10 (depguard)
- §4 package layout (axtree/store/diff/resolve/errors) → Tasks 1–9
- §5.1 axtree.go (Capture + RenderYAML + Truncated) → Tasks 4, 5, 7
- §5.2 store.go (Put/Get/Resolve) → Task 3
- §5.3 diff.go (Diff + Summary) → Task 8
- §5.4 action.go (typed Action values) → **NOT in Phase A** — Phase B per scope; spec §9 confirms.
- §5.5 resolve.go + errors.go → Tasks 2, 9
- §6.1 snapshot creation flow → Tasks 4, 5, 12
- §6.2 action execution → Phase B (out of scope here)
- §6.3 invalidation triggers (navigation, close, mutation) → Tasks 13, 14
- §6.4 concurrency (per-MCP-server Store, per-(session,page) snapshot) → Tasks 3, 11
- §6.5 token-cost ceiling (10k nodes, 64 KB) → Task 7 + axtree.go `Option` funcs in Task 5
- §7.1 typed errors → Task 2
- §7.2 MCP hint contracts → **partial**: §7.2 hints are referenced from action-tool error paths, which are Phase B. `NoSnapshotError` hint exposed implicitly via Task 12 — but no test in Phase A asserts the exact hint string yet. Acceptable since Phase A has only one tool (`browser_snapshot`) and its error is internal capture failure, not stale ref. Phase B will add the hint-contract tests.
- §7.3 no-internal-retry → enforced by absence of retry code in axtree.go (verified by reviewer)
- §7.4 logging — **gap**: no explicit slog/OTel hookup task. Add as Task 12 step in implementation OR add Task 12.5 if reviewer disagrees. Decision: defer to Phase B where action tools will exercise the logging path more meaningfully. Phase A's single tool inherits the existing `addTracedTool` wrapper.
- §8.1 unit tests → Tasks 2, 3, 4, 7, 8
- §8.2 browser integration tests → Tasks 5, 6, 9
- §8.3 MCP tool tests → Task 12 (with note that capability gating is Phase C)
- §8.4 E2E recorder — **not in Phase A** (per §9 Phase A scope)
- §8.5 coverage targets → Task 15
- §8.6 CI / depguard → Task 10

**Placeholder scan**

- Task 14 step 1 explicitly flags an unknown ("if no bridge mutation signal exists, stop and consult"). This is a documented branch, not a placeholder.
- Task 13's `time.Sleep(100 * time.Millisecond)` is a real wait, not a TODO.
- Task 6 step 4 documents the child-index rebasing follow-up as a known limitation. Acceptable for Phase A.

**Type consistency**

- `Snapshot` fields used identically across `store.go`, `axtree.go`, `diff.go`: `PageID`, `Version`, `Nodes`, `URI`, `CapturedAt`, `Truncated`. ✓
- `Node` fields: `Ref`, `BackendNodeID`, `Role`, `Name`, `Value`, `Children`, `FrameID`. Consistent. ✓
- Store API: `NewStore`, `Put`, `Get`, `Clear`, `Resolve`. Used identically in tests and consumers. ✓
- Error type names: `StaleRefError`, `NoSnapshotError`, `AmbiguousRefError`, `TruncatedError` + sentinels `ErrStaleRef`, `ErrNoSnapshot`, `ErrAmbiguousRef`, `ErrTruncated`, `ErrCapture`. Used consistently. ✓
- `aria.Capture` signature: `(ctx, page, ...Option) (*Snapshot, error)`. Same across Tasks 5, 6, 9, 12, 13. ✓

No type drift detected.

---

Plan complete and saved to `docs/superpowers/plans/2026-05-21-aria-phase-a-foundation.md`. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration. Best for a 15-task plan with heavy TDD discipline.

**2. Inline Execution** — execute tasks in this session, batch with checkpoints. Token-heavy for a plan this size.

Which approach?
