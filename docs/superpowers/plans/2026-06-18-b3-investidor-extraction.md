# B3 Investidor Extraction Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a capture-once / replay-many pipeline that logs into `investidor.b3.com.br` headed once, then on every run injects the saved session and pulls all sections (Posição, Movimentação, Proventos, …) headless to raw JSON + flattened CSV, auto-refreshing the token.

**Architecture:** A new advanced example at `examples/b3-investidor/` with a small testable Go package (`b3pipe`) — CSV flattener, run-dir/manifest writer, `sections.yaml` loader, engine-selector, and an Engine-B fetcher (Scout library: headless browser + vault inject + in-page `eval fetch`). A thin `main.go` wires them. Engine B (real browser, SPA self-refreshes) is the always-correct baseline; Engine A (browserless `scout flow run --dump-dir`, a small Scout-core addition) is a conditional fast-path built only when recon shows a JS-readable refresh token.

**Tech Stack:** Go 1.26, `github.com/inovacc/scout/pkg/scout` + `pkg/scout/vault`, Taskfile, real Chrome (cached 149) for integration, `httptest` for unit-level browser tests. YAML via the module's existing yaml dep.

## Global Constraints

- **Go 1.26** — no language changes (`go.mod`).
- **No mocks** — tests use real browser + `httptest`; any test that calls `scout.New(...)` / needs Chrome MUST `t.Skip` under `testing.Short()` (mirror `newTestBrowser`). `task test:unit` / `go test -short` must stay green with no Chrome.
- **Error wrapping** — `fmt.Errorf("scout: <subsystem>: %w", err)` (the `b3pipe` package uses prefix `b3:`).
- **File modes** — runtime data files `0o600`, dirs `0o700` (CPF-linked financial data).
- **No secret literals** — refresh token only in the encrypted vault; access token in-memory only; `flow.yaml` uses `${secret.*}`; no token value in any git-trackable file or log.
- **Placement** — pipeline code under `examples/b3-investidor/`; all runtime data (`b3-data/`, `capture.json`, headed profile dir) written OUTSIDE the repo (under scout home) and gitignored.
- **Commits** — Conventional Commits; **NO** `Co-Authored-By` / AI attribution (per project CLAUDE.md).
- **Build note** — module root has no main; build/test this work with `go test ./examples/b3-investidor/...` and (for the core change) `go build ./cmd/scout/`.

---

## File Structure

```
examples/b3-investidor/
  README.md                      operator runbook (bootstrap, run, verify, security)
  Taskfile.yml                   targets: bootstrap, recon, run, verify
  .gitignore                     ignores runtime data
  sections.example.yaml          template section map (committed); real sections.yaml is gitignored
  main.go                        package main — Stage 2 orchestrator (wires b3pipe)
  b3pipe/
    sections.go  sections_test.go    SectionsConfig loader
    flatten.go   flatten_test.go     JSON → (header, rows) flattener
    writer.go    writer_test.go      per-run dir + raw json + csv + _run.json + latest/
    selector.go  selector_test.go    refresh-token location → engine A/B/fallback
    fetch.go     fetch_test.go       Engine-B fetcher (+ pure buildFetchJS, unit-tested)

pkg/scout/flow/runtime.go          MODIFY — add RunOptions.DumpDir + write step bodies
cmd/scout/flow.go                  MODIFY — add `--dump-dir` flag to `flow run`
```

Shared types (defined in `b3pipe`, referenced across tasks):

```go
package b3pipe

type AuthConfig struct {
    Mode            string `yaml:"mode"`              // "cookie" or "bearer"
    TokenStorageKey string `yaml:"token_storage_key"` // localStorage key holding the access JWT (bearer mode)
}

type Section struct {
    ID         string `yaml:"id"`          // e.g. "posicao"
    Endpoint   string `yaml:"endpoint"`    // absolute or base-relative URL
    Output     string `yaml:"output"`      // base filename (no extension), e.g. "posicao"
    RecordPath string `yaml:"record_path"` // dot path to the record array, e.g. "data.items"
}

type SectionsConfig struct {
    BaseURL  string     `yaml:"base_url"`
    Auth     AuthConfig `yaml:"auth"`
    Sections []Section  `yaml:"sections"`
}

type RefreshTokenInfo struct {
    Found            bool
    InLocalStorage   bool
    InSessionStorage bool
    InReadableCookie bool
    InHTTPOnlyCookie bool
}

type Engine string

const (
    EngineA        Engine = "A"        // browserless scout flow run
    EngineB        Engine = "B"        // headless browser self-refresh
    EngineFallback Engine = "fallback" // no refresh token; re-capture headed on expiry
)
```

---

## Task 1: Scaffold the example directory

**Files:**
- Create: `examples/b3-investidor/.gitignore`
- Create: `examples/b3-investidor/sections.example.yaml`
- Create: `examples/b3-investidor/README.md`
- Create: `examples/b3-investidor/b3pipe/doc.go`

**Interfaces:**
- Produces: the `b3pipe` package path `github.com/inovacc/scout/examples/b3-investidor/b3pipe`.

- [ ] **Step 1: Create `.gitignore`**

```gitignore
# runtime data + secrets — never commit
b3-data/
capture.json
flow.yaml
sections.yaml
*.profile/
.scripts/
```

- [ ] **Step 2: Create `sections.example.yaml`** (template; real `sections.yaml` authored at recon)

```yaml
base_url: "https://www.investidor.b3.com.br"
auth:
  mode: "bearer"            # "cookie" | "bearer" — set from recon
  token_storage_key: ""     # localStorage key holding the access JWT (bearer mode)
sections:
  - id: "posicao"
    endpoint: "https://investidor.b3.com.br/api/posicao"   # confirmed at recon
    output: "posicao"
    record_path: "data"     # dot path to the record array in the JSON response
  # add movimentacao, proventos, … from capture.json
```

- [ ] **Step 3: Create `b3pipe/doc.go`**

```go
// Package b3pipe implements the reusable pieces of the B3 Investidor
// capture-once / replay-many extraction pipeline: section config loading,
// JSON->CSV flattening, run-output writing, engine selection, and the
// Engine-B (headless browser) fetcher.
package b3pipe
```

- [ ] **Step 4: Create `README.md`** (operator runbook — fill the body in Task 8; minimal stub now)

```markdown
# B3 Investidor Extraction Pipeline

Capture-once / replay-many extraction of your B3 *Área do Investidor* data.
See `docs/superpowers/specs/2026-06-18-b3-investidor-extraction-design.md` for design.

Operator runbook (bootstrap / run / verify) — see Task 8 / Taskfile.
```

- [ ] **Step 5: Verify it compiles & commit**

Run: `go build ./examples/b3-investidor/...`
Expected: builds (empty package is valid).

```bash
git add examples/b3-investidor/
git commit -m "feat(b3): scaffold investidor extraction example"
```

---

## Task 2: `sections.yaml` loader

**Files:**
- Create: `examples/b3-investidor/b3pipe/sections.go`
- Test: `examples/b3-investidor/b3pipe/sections_test.go`

**Interfaces:**
- Produces: `LoadSections(path string) (*SectionsConfig, error)`; the shared types above.
- Consumes: the module's YAML dependency. Verify which is in use first: `grep -R "yaml.v3\|ghodss/yaml\|goccy/go-yaml" go.mod`. Use whatever `go.mod` already lists (likely `gopkg.in/yaml.v3`). The code below assumes `gopkg.in/yaml.v3` — adjust the import to the one in `go.mod`.

- [ ] **Step 1: Write the failing test**

```go
package b3pipe

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadSections(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "sections.yaml")
    content := `base_url: "https://www.investidor.b3.com.br"
auth:
  mode: "bearer"
  token_storage_key: "accessToken"
sections:
  - id: "posicao"
    endpoint: "https://investidor.b3.com.br/api/posicao"
    output: "posicao"
    record_path: "data"
`
    if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
        t.Fatal(err)
    }
    cfg, err := LoadSections(p)
    if err != nil {
        t.Fatalf("LoadSections: %v", err)
    }
    if cfg.BaseURL != "https://www.investidor.b3.com.br" {
        t.Errorf("base_url = %q", cfg.BaseURL)
    }
    if cfg.Auth.Mode != "bearer" || cfg.Auth.TokenStorageKey != "accessToken" {
        t.Errorf("auth = %+v", cfg.Auth)
    }
    if len(cfg.Sections) != 1 || cfg.Sections[0].ID != "posicao" || cfg.Sections[0].RecordPath != "data" {
        t.Errorf("sections = %+v", cfg.Sections)
    }
}

func TestLoadSectionsRejectsEmpty(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "sections.yaml")
    if err := os.WriteFile(p, []byte("base_url: \"\"\nsections: []\n"), 0o600); err != nil {
        t.Fatal(err)
    }
    if _, err := LoadSections(p); err == nil {
        t.Fatal("expected error for empty base_url / sections")
    }
}
```

- [ ] **Step 2: Run test → FAIL**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestLoadSections -v`
Expected: FAIL — `undefined: LoadSections`.

- [ ] **Step 3: Implement**

```go
package b3pipe

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// LoadSections reads and validates a sections.yaml file.
func LoadSections(path string) (*SectionsConfig, error) {
    raw, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("b3: read sections: %w", err)
    }
    var cfg SectionsConfig
    if err := yaml.Unmarshal(raw, &cfg); err != nil {
        return nil, fmt.Errorf("b3: parse sections: %w", err)
    }
    if cfg.BaseURL == "" {
        return nil, fmt.Errorf("b3: sections: base_url is required")
    }
    if len(cfg.Sections) == 0 {
        return nil, fmt.Errorf("b3: sections: at least one section is required")
    }
    for i, s := range cfg.Sections {
        if s.ID == "" || s.Endpoint == "" || s.Output == "" {
            return nil, fmt.Errorf("b3: sections[%d]: id, endpoint, output are required", i)
        }
    }
    return &cfg, nil
}
```

- [ ] **Step 4: Run test → PASS**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestLoadSections -v`
Expected: PASS (both cases).

- [ ] **Step 5: Commit**

```bash
git add examples/b3-investidor/b3pipe/sections.go examples/b3-investidor/b3pipe/sections_test.go
git commit -m "feat(b3): sections.yaml loader with validation"
```

---

## Task 3: JSON → (header, rows) flattener

**Files:**
- Create: `examples/b3-investidor/b3pipe/flatten.go`
- Test: `examples/b3-investidor/b3pipe/flatten_test.go`

**Interfaces:**
- Produces: `Flatten(raw []byte, recordPath string) (header []string, rows [][]string, err error)`. Deterministic: header = sorted union of dotted keys across all records; nested objects → dotted columns; nested arrays → compact-JSON string cell. `recordPath` is a dot path (e.g. `"data.items"`, `""` means the top-level value is itself the array).

- [ ] **Step 1: Write the failing test**

```go
package b3pipe

import (
    "reflect"
    "testing"
)

func TestFlattenSimpleArray(t *testing.T) {
    raw := []byte(`{"data":[{"ticker":"PETR4","qtd":100},{"ticker":"VALE3","qtd":50,"corretora":"XP"}]}`)
    header, rows, err := Flatten(raw, "data")
    if err != nil {
        t.Fatalf("Flatten: %v", err)
    }
    // union of keys, sorted: corretora, qtd, ticker
    wantHeader := []string{"corretora", "qtd", "ticker"}
    if !reflect.DeepEqual(header, wantHeader) {
        t.Fatalf("header = %v, want %v", header, wantHeader)
    }
    wantRows := [][]string{
        {"", "100", "PETR4"},
        {"XP", "50", "VALE3"},
    }
    if !reflect.DeepEqual(rows, wantRows) {
        t.Fatalf("rows = %v, want %v", rows, wantRows)
    }
}

func TestFlattenNestedAndTopLevelArray(t *testing.T) {
    raw := []byte(`[{"a":{"b":1},"tags":["x","y"]}]`)
    header, rows, err := Flatten(raw, "")
    if err != nil {
        t.Fatalf("Flatten: %v", err)
    }
    if !reflect.DeepEqual(header, []string{"a.b", "tags"}) {
        t.Fatalf("header = %v", header)
    }
    if !reflect.DeepEqual(rows, [][]string{{"1", `["x","y"]`}}) {
        t.Fatalf("rows = %v", rows)
    }
}

func TestFlattenRecordPathNotArray(t *testing.T) {
    if _, _, err := Flatten([]byte(`{"data":{"x":1}}`), "data"); err == nil {
        t.Fatal("expected error: record_path is not an array")
    }
}
```

- [ ] **Step 2: Run test → FAIL**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestFlatten -v`
Expected: FAIL — `undefined: Flatten`.

- [ ] **Step 3: Implement**

```go
package b3pipe

import (
    "encoding/json"
    "fmt"
    "sort"
    "strconv"
    "strings"
)

// Flatten parses raw JSON, walks recordPath (dot-separated; "" = root) to a
// JSON array, and flattens each element into a row. Header is the sorted union
// of dotted keys across all records. Nested arrays/objects without scalar leaves
// are emitted as compact JSON strings.
func Flatten(raw []byte, recordPath string) (header []string, rows [][]string, err error) {
    var root any
    if err := json.Unmarshal(raw, &root); err != nil {
        return nil, nil, fmt.Errorf("b3: flatten: parse json: %w", err)
    }
    node := root
    if recordPath != "" {
        for _, key := range strings.Split(recordPath, ".") {
            m, ok := node.(map[string]any)
            if !ok {
                return nil, nil, fmt.Errorf("b3: flatten: record_path %q: %q is not an object", recordPath, key)
            }
            node, ok = m[key]
            if !ok {
                return nil, nil, fmt.Errorf("b3: flatten: record_path %q: key %q not found", recordPath, key)
            }
        }
    }
    arr, ok := node.([]any)
    if !ok {
        return nil, nil, fmt.Errorf("b3: flatten: record_path %q does not point to an array", recordPath)
    }

    flat := make([]map[string]string, len(arr))
    keySet := map[string]struct{}{}
    for i, el := range arr {
        m := map[string]string{}
        flattenValue("", el, m)
        flat[i] = m
        for k := range m {
            keySet[k] = struct{}{}
        }
    }
    header = make([]string, 0, len(keySet))
    for k := range keySet {
        header = append(header, k)
    }
    sort.Strings(header)

    rows = make([][]string, len(flat))
    for i, m := range flat {
        row := make([]string, len(header))
        for j, h := range header {
            row[j] = m[h] // missing key -> ""
        }
        rows[i] = row
    }
    return header, rows, nil
}

func flattenValue(prefix string, v any, out map[string]string) {
    switch t := v.(type) {
    case map[string]any:
        for k, sub := range t {
            child := k
            if prefix != "" {
                child = prefix + "." + k
            }
            flattenValue(child, sub, out)
        }
    case []any:
        b, _ := json.Marshal(t) // nested array -> compact JSON cell
        out[prefix] = string(b)
    case float64:
        out[prefix] = strconv.FormatFloat(t, 'f', -1, 64)
    case bool:
        out[prefix] = strconv.FormatBool(t)
    case nil:
        out[prefix] = ""
    default:
        out[prefix] = fmt.Sprintf("%v", t)
    }
}
```

- [ ] **Step 4: Run test → PASS**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestFlatten -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add examples/b3-investidor/b3pipe/flatten.go examples/b3-investidor/b3pipe/flatten_test.go
git commit -m "feat(b3): deterministic JSON->CSV flattener"
```

---

## Task 4: Run-output writer (raw JSON + CSV + manifest + latest/)

**Files:**
- Create: `examples/b3-investidor/b3pipe/writer.go`
- Test: `examples/b3-investidor/b3pipe/writer_test.go`

**Interfaces:**
- Produces:
  - `type Manifest struct { Timestamp string; ScoutVersion string; Engine string; Sections []SectionResult }`
  - `type SectionResult struct { ID string; Status int; Rows int }`
  - `func NewRun(root string, ts time.Time) (*Run, error)` — creates `root/b3-data/<ts>/` (0o700).
  - `func (r *Run) WriteSection(id string, raw []byte, header []string, rows [][]string) error` — writes `<id>.json` + `<id>.csv` (0o600).
  - `func (r *Run) WriteManifest(m Manifest) error` — writes `_run.json` (0o600).
  - `func (r *Run) UpdateLatest() error` — refreshes `root/b3-data/latest/` (copy of this run dir).
  - `func (r *Run) Dir() string`

- [ ] **Step 1: Write the failing test**

```go
package b3pipe

import (
    "encoding/csv"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestRunWriteSectionAndManifest(t *testing.T) {
    root := t.TempDir()
    ts := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)
    run, err := NewRun(root, ts)
    if err != nil {
        t.Fatalf("NewRun: %v", err)
    }
    raw := []byte(`{"data":[{"ticker":"PETR4"}]}`)
    if err := run.WriteSection("posicao", raw, []string{"ticker"}, [][]string{{"PETR4"}}); err != nil {
        t.Fatalf("WriteSection: %v", err)
    }
    // raw json present
    jsonPath := filepath.Join(run.Dir(), "posicao.json")
    if b, _ := os.ReadFile(jsonPath); string(b) != string(raw) {
        t.Errorf("posicao.json = %q", b)
    }
    // csv has header + row
    f, err := os.Open(filepath.Join(run.Dir(), "posicao.csv"))
    if err != nil {
        t.Fatal(err)
    }
    defer func() { _ = f.Close() }()
    recs, _ := csv.NewReader(f).ReadAll()
    if len(recs) != 2 || recs[0][0] != "ticker" || recs[1][0] != "PETR4" {
        t.Errorf("csv = %v", recs)
    }
    // mode 0o600
    info, _ := os.Stat(jsonPath)
    if info.Mode().Perm() != 0o600 {
        t.Errorf("mode = %v", info.Mode().Perm())
    }
    // manifest + latest
    if err := run.WriteManifest(Manifest{Timestamp: ts.Format(time.RFC3339), Engine: "B",
        Sections: []SectionResult{{ID: "posicao", Status: 200, Rows: 1}}}); err != nil {
        t.Fatalf("WriteManifest: %v", err)
    }
    if err := run.UpdateLatest(); err != nil {
        t.Fatalf("UpdateLatest: %v", err)
    }
    if _, err := os.Stat(filepath.Join(root, "b3-data", "latest", "_run.json")); err != nil {
        t.Errorf("latest/_run.json missing: %v", err)
    }
}
```

- [ ] **Step 2: Run test → FAIL**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestRunWrite -v`
Expected: FAIL — `undefined: NewRun`.

- [ ] **Step 3: Implement**

```go
package b3pipe

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

type SectionResult struct {
    ID     string `json:"id"`
    Status int    `json:"status"`
    Rows   int    `json:"rows"`
}

type Manifest struct {
    Timestamp    string          `json:"timestamp"`
    ScoutVersion string          `json:"scout_version"`
    Engine       string          `json:"engine"`
    Sections     []SectionResult `json:"sections"`
}

type Run struct {
    dir  string
    root string
}

// NewRun creates root/b3-data/<RFC3339-compact>/ with 0o700 perms.
func NewRun(root string, ts time.Time) (*Run, error) {
    stamp := ts.UTC().Format("2006-01-02T150405")
    dir := filepath.Join(root, "b3-data", stamp)
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return nil, fmt.Errorf("b3: run dir: %w", err)
    }
    return &Run{dir: dir, root: root}, nil
}

func (r *Run) Dir() string { return r.dir }

func (r *Run) WriteSection(id string, raw []byte, header []string, rows [][]string) error {
    if err := os.WriteFile(filepath.Join(r.dir, id+".json"), raw, 0o600); err != nil {
        return fmt.Errorf("b3: write %s.json: %w", id, err)
    }
    f, err := os.OpenFile(filepath.Join(r.dir, id+".csv"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
    if err != nil {
        return fmt.Errorf("b3: write %s.csv: %w", id, err)
    }
    defer func() { _ = f.Close() }()
    w := csv.NewWriter(f)
    if err := w.Write(header); err != nil {
        return fmt.Errorf("b3: csv header: %w", err)
    }
    if err := w.WriteAll(rows); err != nil {
        return fmt.Errorf("b3: csv rows: %w", err)
    }
    w.Flush()
    return w.Error()
}

func (r *Run) WriteManifest(m Manifest) error {
    b, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return fmt.Errorf("b3: manifest marshal: %w", err)
    }
    if err := os.WriteFile(filepath.Join(r.dir, "_run.json"), b, 0o600); err != nil {
        return fmt.Errorf("b3: write manifest: %w", err)
    }
    return nil
}

// UpdateLatest refreshes root/b3-data/latest/ as a copy of this run's files.
func (r *Run) UpdateLatest() error {
    latest := filepath.Join(r.root, "b3-data", "latest")
    if err := os.RemoveAll(latest); err != nil {
        return fmt.Errorf("b3: clear latest: %w", err)
    }
    if err := os.MkdirAll(latest, 0o700); err != nil {
        return fmt.Errorf("b3: latest dir: %w", err)
    }
    entries, err := os.ReadDir(r.dir)
    if err != nil {
        return fmt.Errorf("b3: read run dir: %w", err)
    }
    for _, e := range entries {
        if e.IsDir() {
            continue
        }
        b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
        if err != nil {
            return fmt.Errorf("b3: copy %s: %w", e.Name(), err)
        }
        if err := os.WriteFile(filepath.Join(latest, e.Name()), b, 0o600); err != nil {
            return fmt.Errorf("b3: write latest %s: %w", e.Name(), err)
        }
    }
    return nil
}
```

- [ ] **Step 4: Run test → PASS**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestRunWrite -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add examples/b3-investidor/b3pipe/writer.go examples/b3-investidor/b3pipe/writer_test.go
git commit -m "feat(b3): per-run output writer (json+csv+manifest+latest)"
```

---

## Task 5: Engine-selector

**Files:**
- Create: `examples/b3-investidor/b3pipe/selector.go`
- Test: `examples/b3-investidor/b3pipe/selector_test.go`

**Interfaces:**
- Consumes: `RefreshTokenInfo`, `Engine` (shared types).
- Produces: `func SelectEngine(info RefreshTokenInfo) Engine`. Rules: not found → fallback; httpOnly cookie (and not JS-readable) → B; JS-readable (localStorage / sessionStorage / readable cookie) → A.

- [ ] **Step 1: Write the failing test**

```go
package b3pipe

import "testing"

func TestSelectEngine(t *testing.T) {
    cases := []struct {
        name string
        in   RefreshTokenInfo
        want Engine
    }{
        {"none", RefreshTokenInfo{Found: false}, EngineFallback},
        {"localStorage", RefreshTokenInfo{Found: true, InLocalStorage: true}, EngineA},
        {"sessionStorage", RefreshTokenInfo{Found: true, InSessionStorage: true}, EngineA},
        {"readable cookie", RefreshTokenInfo{Found: true, InReadableCookie: true}, EngineA},
        {"httpOnly only", RefreshTokenInfo{Found: true, InHTTPOnlyCookie: true}, EngineB},
        {"httpOnly + localStorage prefers A", RefreshTokenInfo{Found: true, InHTTPOnlyCookie: true, InLocalStorage: true}, EngineA},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            if got := SelectEngine(c.in); got != c.want {
                t.Errorf("SelectEngine(%+v) = %q, want %q", c.in, got, c.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run test → FAIL**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestSelectEngine -v`
Expected: FAIL — `undefined: SelectEngine`.

- [ ] **Step 3: Implement**

```go
package b3pipe

// SelectEngine maps the recon-discovered refresh-token location to a replay engine.
// JS-readable (localStorage/sessionStorage/non-httpOnly cookie) -> A (browserless flow).
// httpOnly-only cookie -> B (headless browser self-refresh). Not found -> fallback.
func SelectEngine(info RefreshTokenInfo) Engine {
    if !info.Found {
        return EngineFallback
    }
    if info.InLocalStorage || info.InSessionStorage || info.InReadableCookie {
        return EngineA
    }
    if info.InHTTPOnlyCookie {
        return EngineB
    }
    return EngineFallback
}
```

- [ ] **Step 4: Run test → PASS**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestSelectEngine -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add examples/b3-investidor/b3pipe/selector.go examples/b3-investidor/b3pipe/selector_test.go
git commit -m "feat(b3): engine selector from refresh-token location"
```

---

## Task 6: Engine-B fetcher (headless browser + vault inject + in-page fetch)

**Files:**
- Create: `examples/b3-investidor/b3pipe/fetch.go`
- Test: `examples/b3-investidor/b3pipe/fetch_test.go`

**Interfaces:**
- Consumes: `AuthConfig`, `Section`; `github.com/inovacc/scout/pkg/scout` (`*scout.Page`), `github.com/inovacc/scout/pkg/scout/vault`.
- Produces:
  - `func buildFetchJS(endpoint string, auth AuthConfig) string` — pure; returns the async-IIFE JS that fetches `endpoint` and returns `{status, body}`. In `bearer` mode it reads the token from `localStorage[auth.TokenStorageKey]` and sets `Authorization: Bearer <tok>`; in `cookie` mode it relies on `credentials:'include'`.
  - `type FetchResult struct { Status int; Body []byte }`
  - `func FetchSection(page *scout.Page, s Section, auth AuthConfig) (FetchResult, error)` — runs the JS via `page.Eval`, decodes `{status, body}`.
- Note for the integration helper (used by `main.go`, Task 7): `func OpenAuthedPage(profileID string, pass []byte, baseURL string) (*scout.Browser, *scout.Page, *vault.Handle, error)` — headless `New` → `NewPage(about:blank)` → `ApplyToPage` → `Navigate(baseURL)` → `WaitLoad` → `ApplyStorageToPage` → **`Navigate(baseURL)` again → `WaitLoad`** (reload so the SPA boots with seeded storage). Caller closes browser + handle.

- [ ] **Step 1: Write the failing test (pure JS builder — CI-safe, no browser)**

```go
package b3pipe

import (
    "strings"
    "testing"
)

func TestBuildFetchJSBearer(t *testing.T) {
    js := buildFetchJS("https://investidor.b3.com.br/api/posicao",
        AuthConfig{Mode: "bearer", TokenStorageKey: "accessToken"})
    if !strings.Contains(js, "localStorage.getItem('accessToken')") {
        t.Errorf("bearer JS missing token read: %s", js)
    }
    if !strings.Contains(js, "Authorization") || !strings.Contains(js, "Bearer ") {
        t.Errorf("bearer JS missing Authorization header: %s", js)
    }
    if !strings.Contains(js, "/api/posicao") {
        t.Errorf("JS missing endpoint: %s", js)
    }
}

func TestBuildFetchJSCookie(t *testing.T) {
    js := buildFetchJS("https://investidor.b3.com.br/api/posicao", AuthConfig{Mode: "cookie"})
    if !strings.Contains(js, "credentials") || !strings.Contains(js, "include") {
        t.Errorf("cookie JS must use credentials:'include': %s", js)
    }
    if strings.Contains(js, "Authorization") {
        t.Errorf("cookie JS should not set Authorization: %s", js)
    }
}
```

- [ ] **Step 2: Run test → FAIL**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestBuildFetchJS -v`
Expected: FAIL — `undefined: buildFetchJS`.

- [ ] **Step 3: Implement `buildFetchJS` + `FetchSection` + `OpenAuthedPage`**

```go
package b3pipe

import (
    "encoding/json"
    "fmt"

    "github.com/inovacc/scout/pkg/scout"
    "github.com/inovacc/scout/pkg/scout/vault"
)

type FetchResult struct {
    Status int    `json:"status"`
    Body   []byte `json:"-"`
}

// buildFetchJS returns an async IIFE that fetches endpoint and resolves to
// {status, body}. body is the raw response text. In bearer mode it injects
// Authorization from localStorage[auth.TokenStorageKey]; in cookie mode it sends
// credentials:'include'. Returns the literal "null"/string handled by Eval as JSON.
func buildFetchJS(endpoint string, auth AuthConfig) string {
    headers := ""
    if auth.Mode == "bearer" {
        headers = fmt.Sprintf(
            "const tok = localStorage.getItem(%q); "+
                "const headers = tok ? {'Authorization': 'Bearer ' + tok} : {};",
            auth.TokenStorageKey)
    } else {
        headers = "const headers = {};"
    }
    return fmt.Sprintf(`async () => {
  %s
  const r = await fetch(%q, { headers, credentials: 'include' });
  const body = await r.text();
  return { status: r.status, body };
}`, headers, endpoint)
}

// FetchSection runs the section's fetch JS in the page and returns status+body.
func FetchSection(page *scout.Page, s Section, auth AuthConfig) (FetchResult, error) {
    res, err := page.Eval(buildFetchJS(s.Endpoint, auth))
    if err != nil {
        return FetchResult{}, fmt.Errorf("b3: fetch %s: %w", s.ID, err)
    }
    var payload struct {
        Status int    `json:"status"`
        Body   string `json:"body"`
    }
    if err := json.Unmarshal(res.JSON(), &payload); err != nil {
        return FetchResult{}, fmt.Errorf("b3: decode %s: %w", s.ID, err)
    }
    return FetchResult{Status: payload.Status, Body: []byte(payload.Body)}, nil
}

// OpenAuthedPage opens a headless authenticated page for the given vault profile.
// Sequence mirrors `scout vault use` plus a reload so a localStorage-JWT SPA boots
// with the seeded storage present.
func OpenAuthedPage(profileID string, pass []byte, baseURL string) (*scout.Browser, *scout.Page, *vault.Handle, error) {
    v, err := vault.Open(pass)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("b3: vault open: %w", err)
    }
    h, err := v.Use(profileID)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("b3: vault use: %w", err)
    }
    b, err := scout.New(scout.WithHeadless(true))
    if err != nil {
        _ = h.Close()
        return nil, nil, nil, fmt.Errorf("b3: browser: %w", err)
    }
    page, err := b.NewPage("about:blank")
    if err != nil {
        b.Close()
        _ = h.Close()
        return nil, nil, nil, fmt.Errorf("b3: new page: %w", err)
    }
    if err := h.ApplyToPage(page); err != nil { // cookies + headers
        b.Close()
        _ = h.Close()
        return nil, nil, nil, fmt.Errorf("b3: apply cookies: %w", err)
    }
    if err := page.Navigate(baseURL); err != nil {
        b.Close()
        _ = h.Close()
        return nil, nil, nil, fmt.Errorf("b3: navigate: %w", err)
    }
    _ = page.WaitLoad()
    if err := h.ApplyStorageToPage(page); err != nil { // localStorage/sessionStorage on origin
        b.Close()
        _ = h.Close()
        return nil, nil, nil, fmt.Errorf("b3: apply storage: %w", err)
    }
    if err := page.Navigate(baseURL); err != nil { // reload so SPA boots authed
        b.Close()
        _ = h.Close()
        return nil, nil, nil, fmt.Errorf("b3: re-navigate: %w", err)
    }
    _ = page.WaitLoad()
    return b, page, h, nil
}
```

- [ ] **Step 4: Run pure test → PASS**

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestBuildFetchJS -v`
Expected: PASS.

- [ ] **Step 5: Add a `-short`-gated integration test (real browser + httptest SPA)**

```go
package b3pipe

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestFetchSectionIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("requires browser; skipped under -short")
    }
    // Minimal page that serves a token in localStorage + an API echoing it.
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte(`<html><body><script>localStorage.setItem('accessToken','T123')</script></body></html>`))
    })
    mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "Bearer T123" {
            w.WriteHeader(401)
            return
        }
        _, _ = w.Write([]byte(`{"data":[{"x":1}]}`))
    })
    srv := httptest.NewServer(mux)
    defer srv.Close()

    b, err := scout.New(scout.WithHeadless(true))
    if err != nil {
        t.Skipf("no browser: %v", err)
    }
    defer b.Close()
    page, err := b.NewPage(srv.URL)
    if err != nil {
        t.Fatal(err)
    }
    _ = page.WaitLoad()
    res, err := FetchSection(page, Section{ID: "data", Endpoint: srv.URL + "/api/data"},
        AuthConfig{Mode: "bearer", TokenStorageKey: "accessToken"})
    if err != nil {
        t.Fatalf("FetchSection: %v", err)
    }
    if res.Status != 200 || string(res.Body) != `{"data":[{"x":1}]}` {
        t.Fatalf("res = %d %q", res.Status, res.Body)
    }
}
```

Run: `go test ./examples/b3-investidor/b3pipe/ -run TestFetchSection -v` (full, not -short) → PASS with Chrome; `go test -short ./examples/b3-investidor/...` → SKIP.

- [ ] **Step 6: Commit**

```bash
git add examples/b3-investidor/b3pipe/fetch.go examples/b3-investidor/b3pipe/fetch_test.go
git commit -m "feat(b3): Engine-B headless fetcher with vault inject + in-page fetch"
```

---

## Task 7: Run-command `main.go` (Stage 2 orchestrator, Engine B)

**Files:**
- Create: `examples/b3-investidor/main.go`

**Interfaces:**
- Consumes: all of `b3pipe`; env `SCOUT_PASSPHRASE` (vault passphrase), `SCOUT_HOME` (output root resolution).
- Behavior: load `sections.yaml` → `OpenAuthedPage` → for each section `FetchSection` → if status==401 or login-redirect detected, print re-auth message and exit non-zero → else `Flatten` + `WriteSection` → `WriteManifest` + `UpdateLatest`.
- Flags: `--profile <vault-id>` (required), `--sections <path>` (default `sections.yaml`), `--out <root>` (default: current dir).

- [ ] **Step 1: Write `main.go`**

```go
// Command b3-investidor replays a saved B3 investor session headless and writes
// each section to raw JSON + flattened CSV. See README.md and the design spec.
package main

import (
    "flag"
    "fmt"
    "os"
    "time"

    "github.com/inovacc/scout/examples/b3-investidor/b3pipe"
)

func main() {
    profile := flag.String("profile", "", "vault profile ID for the B3 session (required)")
    sectionsPath := flag.String("sections", "sections.yaml", "path to sections.yaml")
    outRoot := flag.String("out", ".", "root directory for b3-data output")
    flag.Parse()

    if *profile == "" {
        fmt.Fprintln(os.Stderr, "error: --profile is required")
        os.Exit(2)
    }
    pass := []byte(os.Getenv("SCOUT_PASSPHRASE"))
    if len(pass) == 0 {
        fmt.Fprintln(os.Stderr, "error: set SCOUT_PASSPHRASE for the vault")
        os.Exit(2)
    }

    if err := run(*profile, pass, *sectionsPath, *outRoot); err != nil {
        fmt.Fprintf(os.Stderr, "b3: %v\n", err)
        os.Exit(1)
    }
}

func run(profile string, pass []byte, sectionsPath, outRoot string) error {
    cfg, err := b3pipe.LoadSections(sectionsPath)
    if err != nil {
        return err
    }
    browser, page, handle, err := b3pipe.OpenAuthedPage(profile, pass, cfg.BaseURL)
    if err != nil {
        return err
    }
    defer browser.Close()
    defer func() { _ = handle.Close() }()

    runOut, err := b3pipe.NewRun(outRoot, time.Now())
    if err != nil {
        return err
    }
    var results []b3pipe.SectionResult
    for _, s := range cfg.Sections {
        res, err := b3pipe.FetchSection(page, s, cfg.Auth)
        if err != nil {
            return err
        }
        if res.Status == 401 || res.Status == 0 {
            return fmt.Errorf("section %s returned %d — session expired; run `task b3:bootstrap` to re-login (gov.br + MFA)", s.ID, res.Status)
        }
        header, rows, err := b3pipe.Flatten(res.Body, s.RecordPath)
        if err != nil {
            // still write raw json for debugging; surface the flatten error
            _ = runOut.WriteSection(s.Output, res.Body, []string{"_raw"}, [][]string{{string(res.Body)}})
            return fmt.Errorf("section %s: %w", s.ID, err)
        }
        if err := runOut.WriteSection(s.Output, res.Body, header, rows); err != nil {
            return err
        }
        results = append(results, b3pipe.SectionResult{ID: s.ID, Status: res.Status, Rows: len(rows)})
        fmt.Printf("✓ %-16s %d  (%d rows)\n", s.ID, res.Status, len(rows))
    }
    if err := runOut.WriteManifest(b3pipe.Manifest{
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Engine:    string(b3pipe.EngineB),
        Sections:  results,
    }); err != nil {
        return err
    }
    if err := runOut.UpdateLatest(); err != nil {
        return err
    }
    fmt.Printf("\nwrote %d sections to %s\n", len(results), runOut.Dir())
    return nil
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./examples/b3-investidor/`
Expected: builds.

- [ ] **Step 3: Verify `-short` suite still green (no Chrome needed)**

Run: `go test -short ./examples/b3-investidor/...`
Expected: PASS (integration test skips).

- [ ] **Step 4: Commit**

```bash
git add examples/b3-investidor/main.go
git commit -m "feat(b3): Stage 2 run-command orchestrator (Engine B)"
```

---

## Task 8: Stage 0 bootstrap + recon (operator-driven) + Taskfile + README

**Files:**
- Create: `examples/b3-investidor/Taskfile.yml`
- Modify: `examples/b3-investidor/README.md`

**Interfaces:**
- Consumes: the built `scout` CLI (`scout flow capture`, `scout vault capture`, `scout vault set`, `scout repl`).
- Produces: operator commands `task b3:bootstrap`, `task b3:recon`, `task b3:run`, `task b3:verify`.
- **Human-in-the-loop:** gov.br login + MFA is performed by the operator in the headed window; it cannot be automated.

- [ ] **Step 1: Write `Taskfile.yml`**

```yaml
version: "3"

vars:
  URL: "https://www.investidor.b3.com.br"
  PROFILE_DIR: '{{.SCOUT_HOME | default (printf "%s/.scout-b3" .HOME)}}'

tasks:
  bootstrap:
    desc: "Headed login (gov.br + MFA) + capture auth into vault profile 'b3'"
    cmds:
      - echo "A headed Chrome will open. Log in via gov.br (CPF + MFA), then walk Posição / Movimentação / Proventos so the SPA fires every API call. Close the window when done."
      - scout repl {{.URL}} --headless=false --user-data-dir "{{.PROFILE_DIR}}"
      - echo "Now capturing cookies + storage into vault profile 'b3'..."
      - scout vault capture b3 {{.URL}} --user-data-dir "{{.PROFILE_DIR}}"
      - echo "Vault profile 'b3' created. Note the printed profile ID."

  recon:
    desc: "Capture API traffic to capture.json; inspect token location to pick the engine"
    cmds:
      - echo "Replays a headed session and records *api*/*graphql* calls. Log in if prompted."
      - scout flow capture {{.URL}} --out capture.json --filter '*api*' --filter '*graphql*'
      - echo "Inspect capture.json for: the data endpoints (-> sections.yaml), the refresh endpoint, and where the access/refresh token lives (localStorage vs httpOnly cookie). Then author sections.yaml from sections.example.yaml."

  run:
    desc: "Headless extraction of all sections to b3-data/ (Engine B)"
    cmds:
      - go run . --profile {{.PROFILE | default "REPLACE_WITH_PROFILE_ID"}} --sections sections.yaml --out "{{.PROFILE_DIR}}"

  verify:
    desc: "Re-run and confirm auto-refresh worked (no re-login)"
    cmds:
      - task: run
      - echo "Open {{.PROFILE_DIR}}/b3-data/latest/_run.json and confirm every section status is 200."
```

- [ ] **Step 2: Write the README operator runbook** (replace the stub)

```markdown
# B3 Investidor Extraction Pipeline

Capture your B3 *Área do Investidor* session once (headed, gov.br + MFA), then pull
all sections headless on every run to raw JSON + flattened CSV.

## One-time bootstrap
1. `task b3:bootstrap` — a headed Chrome opens; log in via gov.br (CPF + MFA) and
   walk Posição / Movimentação / Proventos. Close the window. Auth is sealed into
   vault profile **b3** (note the printed profile ID). You'll be asked for a vault
   passphrase — remember it; export it as `SCOUT_PASSPHRASE` for runs.
2. `task b3:recon` — records the API traffic to `capture.json`. From it, author
   `sections.yaml` (copy `sections.example.yaml`): one entry per section with its
   `endpoint` and the `record_path` to its record array, and set `auth.mode`
   (`bearer` if the JWT is in localStorage, `cookie` if it's an httpOnly cookie)
   and `token_storage_key` for bearer mode.

## Every run
```bash
export SCOUT_PASSPHRASE='…'
task b3:run PROFILE=<your-b3-profile-id>
```
Output lands in `$SCOUT_HOME/.scout-b3/b3-data/<timestamp>/` (and `latest/`):
`posicao.json` + `posicao.csv`, etc., plus `_run.json`.

## When it stops working
A `session expired` message means the **refresh token** died (not the short access
token). Re-run `task b3:bootstrap` (~1 min) and runs resume.

## Security
- Your gov.br password is never stored — you type it in the headed window.
- The session token lives only in the encrypted vault; `b3-data/`, `capture.json`,
  and the profile dir are gitignored and written outside the repo.
- After authoring `sections.yaml`, delete `capture.json` (it holds a live token).
```

- [ ] **Step 3: Commit**

```bash
git add examples/b3-investidor/Taskfile.yml examples/b3-investidor/README.md
git commit -m "feat(b3): operator bootstrap/recon/run/verify Taskfile + runbook"
```

---

## Task 9 (CONDITIONAL — only if recon shows a JS-readable refresh token): Engine A fast-path

Build this ONLY when `task b3:recon` shows the refresh token in localStorage / sessionStorage / a non-httpOnly cookie (selector → `EngineA`). Otherwise skip; Engine B already covers the case.

### 9a — Add `--dump-dir` to `scout flow run` (Scout core; reusable)

**Files:**
- Modify: `pkg/scout/flow/runtime.go` (`RunOptions` struct ~line 15; the `Run` loop where each `*StepResult` is produced)
- Modify: `cmd/scout/flow.go` (`flowRunCmd` flags ~line 219; `RunE` ~line 114)
- Test: `pkg/scout/flow/runtime_test.go`

**Interfaces:**
- Produces: `RunOptions.DumpDir string`; when set, `Run` writes each step's response body to `DumpDir/<step-id>.json` (0o600). CLI: `scout flow run flow.yaml --profile <id> --dump-dir <dir>`.

- [ ] **Step 1: Failing test** (httptest server; no browser)

```go
func TestRunDumpDir(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte(`{"ok":true}`))
    }))
    defer srv.Close()
    spec := &FlowSpec{
        Name:  "t",
        Steps: []FlowStep{{ID: "one", Request: FlowRequest{Method: "GET", URL: srv.URL}}},
    }
    dir := t.TempDir()
    _, err := Run(context.Background(), spec, RunOptions{DumpDir: dir})
    if err != nil {
        t.Fatalf("Run: %v", err)
    }
    b, err := os.ReadFile(filepath.Join(dir, "one.json"))
    if err != nil || string(b) != `{"ok":true}` {
        t.Fatalf("dump = %q err=%v", b, err)
    }
}
```

(Confirm the exact `FlowSpec`/`FlowStep`/`FlowRequest` field names against `pkg/scout/flow/spec.go` before running — adjust literals if they differ.)

- [ ] **Step 2: Run → FAIL** (`RunOptions has no field DumpDir`).

- [ ] **Step 3: Implement** — add the field and the write. In `runtime.go`:

```go
type RunOptions struct {
    Client  *http.Client
    Secrets SecretResolver
    Vars    map[string]string
    DumpDir string // when set, each step's response body is written to DumpDir/<id>.json
}
```

In `Run`, immediately after a `*StepResult` (`sr`) is obtained from `runStep` in the step loop:

```go
if opts.DumpDir != "" && sr != nil {
    if err := os.WriteFile(filepath.Join(opts.DumpDir, sr.ID+".json"), []byte(sr.Body), 0o600); err != nil {
        return nil, fmt.Errorf("scout: flow: dump %s: %w", sr.ID, err)
    }
}
```

Add `"os"` and `"path/filepath"` imports if missing.

- [ ] **Step 4: Run → PASS.** Then wire the CLI in `cmd/scout/flow.go`:

```go
// in flowRunCmd setup (~line 219):
flowRunCmd.Flags().String("dump-dir", "", "directory to write each step's response body (<id>.json)")

// in RunE before flow.Run (~line 114):
if d, _ := cmd.Flags().GetString("dump-dir"); d != "" {
    opts.DumpDir = d
}
```

Build: `go build ./cmd/scout/` → success.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/flow/runtime.go cmd/scout/flow.go pkg/scout/flow/runtime_test.go
git commit -m "feat(flow): flow run --dump-dir writes step response bodies"
```

### 9b — Author `flow.yaml` + wire Engine A in `main.go`

- [ ] **Step 1:** Author `flow.yaml` from `capture.json` (refresh step + one GET per section, auth via `${secret.b3_access}`/`${secret.b3_refresh}`), per spec §3. Copy the refresh token into the vault `Secrets`: `scout vault set b3_refresh=<value> --id <b3-profile-id>` (value read from the recon capture; never commit it).
- [ ] **Step 2:** In `main.go`, add `--engine A|B` (default B). For `A`: shell out to `scout flow run flow.yaml --profile <id> --dump-dir <tmp>`, then read `<tmp>/<id>.json` for each section and feed `Flatten`/`WriteSection` (reuse Task 7's writer path). Add a unit test for the "read dump-dir → sections" wiring using fixture JSON files (no browser).
- [ ] **Step 3:** `task b3:verify` for Engine A also runs `scout flow verify flow.yaml --golden capture.json` (status parity).
- [ ] **Step 4: Commit** `feat(b3): Engine-A browserless fast-path via flow run --dump-dir`.

---

## Task 10: Verification / acceptance (UAT)

**Files:** none (operator + checklist). Gate with `superpowers:verification-before-completion`.

- [ ] **Step 1:** `go test -short ./examples/b3-investidor/... ./pkg/scout/flow/...` → all PASS (CI-safe units green without Chrome).
- [ ] **Step 2:** `go test ./examples/b3-investidor/b3pipe/ -run Integration` (with Chrome) → fetcher integration PASS.
- [ ] **Step 3 (operator):** `task b3:bootstrap` → vault profile `b3` created; `task b3:recon` → `capture.json` covers all sections; author `sections.yaml`.
- [ ] **Step 4 (operator):** `task b3:run` → every configured section yields non-empty `<id>.json` + `<id>.csv`; `_run.json` shows status 200 + sane row counts.
- [ ] **Step 5 (operator):** Run `task b3:run` **again** → succeeds with NO re-login (auto-refresh proven), until the refresh token's natural expiry.
- [ ] **Step 6:** `git grep` the working tree for any token literal in committed files → none (no-expose). Confirm `b3-data/`, `capture.json`, `flow.yaml`, `sections.yaml` are gitignored.

### Acceptance criteria (from spec §8)
1. One headed bootstrap captures auth into vault `b3` + records `capture.json` for all sections. ✔ Task 8 / 10.3
2. Recon classifies the refresh-token location → selects A/B/fallback. ✔ Task 5 / 8.
3. `task b3:run` writes every section as raw `.json` + flattened `.csv` under a timestamped dir + `_run.json`. ✔ Task 4 / 7 / 10.4.
4. A second consecutive run succeeds without re-login (auto-refresh). ✔ Task 10.5.
5. No secret literal in any git-trackable artifact or log. ✔ Task 1 (.gitignore) / 10.6.

---

## Self-Review

**Spec coverage:** §1 decisions → Tasks 5,7,8; §2 stages → Tasks 6/7 (replay), 8 (bootstrap/recon); §3 components → Tasks 2–7; §4 auto-refresh/expiry → Task 6 (reload) + Task 7 (401 detection); §5 output → Task 4; §6 security → Task 1 + Task 10.6; §7 new code → Tasks 3,4,5,6,9; §8 testing/acceptance → Task 10. No uncovered section.

**Placeholder scan:** endpoints/`record_path`/token keys are recon-derived config values authored in Task 8 (legitimately data-driven, not code placeholders); the one `REPLACE_WITH_PROFILE_ID` is a Taskfile var the operator overrides via `PROFILE=`. Task 9 explicitly notes the `FlowSpec` field-name confirmation before running. No code TODOs.

**Type consistency:** `SectionsConfig`/`Section`/`AuthConfig`/`RefreshTokenInfo`/`Engine` defined once (File Structure) and used identically in Tasks 2,5,6,7. `Flatten` signature `(raw []byte, recordPath string) ([]string, [][]string, error)` consistent across Tasks 3,7. `FetchResult{Status,Body}`, `Manifest`/`SectionResult` consistent across Tasks 4,6,7. `OpenAuthedPage`/`FetchSection`/`buildFetchJS` signatures match between Task 6 (def) and Task 7 (use). `RunOptions.DumpDir` consistent across Task 9a (def) and 9b (use).
