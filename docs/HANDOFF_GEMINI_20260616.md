# Handoff — 2026-06-16 — Gemini Headless Agent

> **This file is a self-contained task brief for a Gemini CLI headless agent.**
> Read it fully before writing a single line of code.
> Resume point: read this file, read the spec, then start at **Phase 1**.

---

## Context

**Project:** Scout (`github.com/inovacc/scout`)
**Repo:** `D:/weaver-sync/development/personal/projects/scout`
**Branch:** `main`
**Go version:** 1.26.0
**What this task is doing:** Implement 7 browser automation capability gaps
that currently force users to fall back to Playwright. The goal is to make
Scout the sole tool for authenticated, scheduled browser automation — no
Playwright required.

**Reference use case:** B3 Área do Investidor (Brazilian financial portal).
Cloudflare + Azure AD B2C OAuth + SPA + file downloads + nightly sync.
After this work, a Scout runbook replaces a Playwright script entirely.

---

## Spec — read this first

**Full design spec:**
`docs/superpowers/specs/2026-06-16-scout-playwright-parity-design.md`

**Field report (gap evidence):**
`docs/scout-playwright-gaps.md`

**Prior gap matrix:**
`docs/PLAYWRIGHT-GAP-ANALYSIS.md`

Read the spec completely before writing any code. It contains exact API
signatures, data formats, CLI command names, MCP tool names, runbook action
names, and the B3 reference runbook that must work end-to-end.

---

## Codebase map (critical files)

```
cmd/scout/
  mcp.go              ← MCP server entrypoint; tool count + long description
  session.go          ← existing session CLI (list/reset); extend here
  hijack.go           ← existing hijack CLI; HijackOption already implemented
  vault.go            ← vault CLI; vault package already works

pkg/scout/
  scout.go            ← Browser type + New() constructor
  mcp/                ← ALL MCP tool registrations live here
  runbook/            ← runbook engine (executor, action registry)
  runbooks/           ← built-in preset runbooks
  vault/              ← Argon2id + AES-256-GCM vault; SecretProfileInput
  flow/               ← flow capture/replay (reference for session patterns)

internal/engine/browser/
  ← CDP browser engine (rod-based); Page, WaitLoad, Eval, etc. live here
  ← ADD: networkidle wait, URL-predicate wait, download hooks here

docs/superpowers/specs/
  2026-06-16-scout-playwright-parity-design.md   ← THE SPEC
```

---

## What's done (pre-task)

| Step | File | Notes |
|------|------|-------|
| ✅ Gap analysis | `docs/scout-playwright-gaps.md` | 9 gaps with evidence from B3 session |
| ✅ Design spec | `docs/superpowers/specs/2026-06-16-scout-playwright-parity-design.md` | 4-layer design, approved |
| ✅ Existing eval CLI fix | `d70b4d1` | JS-triggered navigation treated as success (partial) |
| ✅ Existing hijack CLI | `cmd/scout/hijack.go` | `--body` flag works; just needs MCP wrapper |
| ✅ Existing vault | `pkg/scout/vault/` | Fully working; SessionStore will wrap it |

---

## What's broken / blocked

| Issue | Notes |
|-------|-------|
| ❌ eval() MCP primitive return | Returns error for non-function results in MCP tool (not the CLI). Bug is in `pkg/scout/mcp` eval handler. |
| ❌ No storageState | `pkg/scout/session` package does not exist yet |
| ❌ No download interception | No CDP `Page.downloadWillBegin` hook anywhere |
| ❌ No networkidle wait | `wait` MCP tool only accepts CSS selector |
| ❌ No URL-predicate wait | Same — no URL polling mode |
| ❌ hijack_watch not an MCP tool | Only CLI, not in `pkg/scout/mcp` |
| ❌ No runbook loops | `loop`, `if`, `on_failure` don't exist in runbook action registry |
| ❌ No scheduler | `scout schedule` command doesn't exist |

---

## Implementation order — STRICT

Do NOT skip phases or implement out of order. Each phase depends on the one
before it being complete and tested.

---

### Phase 1 — Engine Layer (start here)

**Goal:** Fix the eval bug + add two new wait modes + wire hijack_watch as MCP.
No new packages needed. All changes in existing files.

**Step 1.1 — Fix eval() MCP primitive return**

File: `pkg/scout/mcp/` — find the eval tool handler.
Problem: The handler calls `.apply()` or equivalent on the eval result, which
panics/errors when the JS expression returns a primitive (string, number, JSON
object) instead of a function.
Fix: After `Runtime.evaluate`, check the result type. If it's not a function
reference, serialize the `value` field directly and return it as the tool
result. Do not call any function-invoke CDP method on a primitive result.
Test: `eval` tool with `"location.href"` must return a string. With
`JSON.stringify({a:1})` must return `{"a":1}`. With `42` must return `42`.

**Step 1.2 — Add `wait_for: networkidle` to wait MCP tool**

File: `internal/engine/browser/` — find or add a `WaitNetworkIdle` method.
Implementation:
- Subscribe to CDP `Network.loadingFinished` and `Network.loadingFailed` events
- Start a 500ms debounce timer that resets on each event
- Resolve when the debounce fires without a new network event
- Overall timeout: configurable, default 15000ms
- Wire into the `wait` MCP tool as: `{ "wait_for": "networkidle", "timeout_ms": 15000 }`

**Step 1.3 — Add `wait_for: url` to wait MCP tool**

File: same as 1.2, add `WaitForURL(pattern string, timeout time.Duration)`.
Implementation:
- Poll `Runtime.evaluate("location.href")` every 200ms
- Pattern matching:
  - Plain string → substring contains check
  - Starts with `re:` → compile as Go regex, match against full URL
  - Otherwise → treat `*` as wildcard, compile simple glob
- Resolve when URL matches pattern
- Timeout default: 30000ms
- Wire into `wait` MCP tool as: `{ "wait_for": "url", "url_pattern": "*.b3.com.br*", "timeout_ms": 30000 }`

**Step 1.4 — hijack_watch MCP tool**

File: `pkg/scout/mcp/` — add new tool `hijack_watch`.
Implementation: thin wrapper over existing `scout.HijackOption` /
`scout.WithHijackBodyCapture()`. Tool params:
- `url_pattern` string (default `*`) — URL glob filter
- `collect_for_ms` int (default 5000) — collect duration before returning
- `body` bool (default true) — capture response bodies

Returns: JSON array of `{ url, method, status, requestBody, responseBody, contentType }`.

Also update tool count and description in `cmd/scout/mcp.go` (currently says "18 built-in browser automation tools" — update to 19 after this step, then keep updating per phase).

**Phase 1 done when:**
- [ ] `eval` with `"location.href"` returns a string, not an error
- [ ] `wait` with `wait_for: networkidle` blocks until no requests for 500ms
- [ ] `wait` with `wait_for: url` + `url_pattern` resolves on URL match
- [ ] `hijack_watch` MCP tool returns captured request/response JSON
- [ ] `go build ./...` passes
- [ ] Existing tests pass: `go test -short ./...`

---

### Phase 2 — Storage Layer

**Goal:** Session persistence (vault-backed + Playwright-compat export) +
file download interception.

**Step 2.1 — Create `pkg/scout/session` package**

New package. See spec §5.1 for full API.

Key types:
```go
type SessionMeta struct {
    Name        string
    CapturedAt  time.Time
    ExpiresAt   time.Time   // zero if unknown
    OriginURL   string
}

type SessionStore struct { /* wraps vault */ }

func NewSessionStore(v *vault.Vault) *SessionStore

func (s *SessionStore) Save(ctx context.Context, name string, page Page) error
func (s *SessionStore) Load(ctx context.Context, name string, page Page) error
func (s *SessionStore) Delete(name string) error
func (s *SessionStore) List() ([]SessionMeta, error)
func (s *SessionStore) IsExpired(name string) (bool, error)
func (s *SessionStore) ExportPlaywright(name, outPath string) error
func (s *SessionStore) ImportPlaywright(name, inPath string) error
```

Data captured by Save():
- Cookies: CDP `Network.getAllCookies` filtered to page origin
- localStorage: `Runtime.evaluate` IIFE over `localStorage`
- sessionStorage: `Runtime.evaluate` IIFE over `sessionStorage`
- Metadata: capturedAt, expiresAt (parse from JWT `exp` claim in sessionStorage
  `token` key if present), originURL

Vault storage: serialize session JSON, encrypt via existing vault as a named
secret profile. Key: `session:<name>`.

Load() injection:
- Cookies: CDP `Network.setCookies`
- localStorage: `Runtime.evaluate` IIFE that calls `localStorage.setItem` for each entry
- sessionStorage: same pattern

Playwright export format (see spec §5.1 for exact JSON schema).
sessionStorage: stored in vault but NOT written to Playwright export (inject
via `addInitScript` workaround is out of scope; just omit with a comment).

**Step 2.2 — Add session CLI subcommands**

File: `cmd/scout/session.go` — extend existing file.
Add: `save`, `load`, `export`, `import`, `delete` subcommands.
The `list` command already exists — update it to show expiry status.

**Step 2.3 — Add session MCP tools**

File: `pkg/scout/mcp/` — add 3 new tools:
- `session_save` params: `name string`
- `session_load` params: `name string` → returns `{ "expiring_soon": bool }`
- `session_export` params: `name string`, `format string` ("playwright"), `out string`

**Step 2.4 — DownloadManager**

File: `internal/engine/browser/` — add `DownloadManager` type.
See spec §5.2 for full API.

CDP methods needed:
- `Page.setDownloadBehavior` — call with `behavior: allow`, `downloadPath: <dir>`
- Subscribe to `Page.downloadWillBegin` — capture guid + suggestedFilename
- Subscribe to `Page.downloadProgress` — detect `state: completed`

```go
type DownloadResult struct {
    GUID      string
    Filename  string
    MIMEType  string
    SizeBytes int64
    SavePath  string
    Duration  time.Duration
}

type DownloadManager struct { /* holds dir, active downloads map */ }

func NewDownloadManager(page Page, dir string) (*DownloadManager, error)
func (d *DownloadManager) WaitForDownload(ctx context.Context, timeout time.Duration) (DownloadResult, error)
```

Download directory resolution (priority order):
1. `save_as` param in the tool/action call
2. `--download-dir` CLI flag
3. `SCOUT_DOWNLOAD_DIR` env var
4. `./downloads`

**Step 2.5 — download_wait MCP tool + CLI**

MCP tool `download_wait`: params `timeout_ms int`, `save_as string` (optional).
Returns `DownloadResult` as JSON.

CLI: `scout download watch <url>` — opens page, prints download events as
NDJSON to stdout, saves files to download dir.

**Phase 2 done when:**
- [ ] `session save b3` captures cookies+storage from current page into vault
- [ ] `session load b3` injects them back; page shows as authenticated
- [ ] `session export b3 --format=playwright --out=session.json` writes valid Playwright storageState JSON
- [ ] `session import session.json --format=playwright --name=b3` round-trips correctly
- [ ] `download_wait` MCP tool saves a file triggered by a click
- [ ] `go build ./...` passes
- [ ] `go test -short ./...` passes

---

### Phase 3 — Runbook Layer

**Goal:** Variables, loop primitives, conditionals. All as new runbook action
modifiers + new action types. Existing runbooks must be unaffected.

**Step 3.1 — Variables and `capture_as`**

File: `pkg/scout/runbook/` — runbook executor.

Add top-level `vars` map to runbook schema (JSON/YAML). Populate a variable
scope at execution start. Template interpolation: `{{ var.name }}` and
`{{ env.NAME }}` (reads OS env). `{{ step_output.capture_as_name }}` for
captured step outputs. Also support `{{ now.year }}`, `{{ now.month }}`,
`{{ now.date }}` as built-in time variables.

Expression engine: string interpolation + integer arithmetic only. No eval.
Use a simple template engine (Go `text/template` subset is fine).

`capture_as: varname` modifier: add to the runbook action struct. After step
execution, store the step's string/JSON output as `varname` in the variable
scope.

**Step 3.2 — Loop primitives**

Add `loop` field to the runbook action struct. Three forms (see spec §6.2).
Each form runs the action N times, collecting results into an array.
Support `break_on_error: true` (default false).
Report per-iteration results in the step output.

```go
type LoopConfig struct {
    For    []string `json:"for,omitempty"`    // iterate over list
    As     string   `json:"as,omitempty"`     // loop variable name
    While  string   `json:"while,omitempty"`  // condition expression
    Repeat int      `json:"repeat,omitempty"` // fixed count
    Max    int      `json:"max,omitempty"`    // safety cap for while
    BreakOnError bool `json:"break_on_error,omitempty"`
}
```

**Step 3.3 — Conditional execution**

Add `if` field (string expression, evaluated against variable scope) and
`on_failure` field (sub-sequence of actions) to the runbook action struct.

`if`: skip the step if expression evaluates to false/empty/zero.
`on_failure`: if the step errors, run the sub-sequence. If sub-sequence
succeeds, do not count the parent step as a failure.

**Step 3.4 — Register new runbook actions**

Register all new actions from Phase 1 + Phase 2 in the runbook action registry:

| Action | Calls |
|--------|-------|
| `session_save` | `SessionStore.Save()` |
| `session_load` | `SessionStore.Load()` |
| `download_wait` | `DownloadManager.WaitForDownload()` |
| `wait_networkidle` | `WaitNetworkIdle()` |
| `wait_url` | `WaitForURL()` |
| `hijack_response` | hijack engine with `collect_for_ms` |

**Step 3.5 — Validate with B3 reference runbook**

Write the reference runbook from spec §9 as a test fixture:
`pkg/scout/runbooks/testdata/b3-daily-sync.yaml`

Run `scout runbook plan -f b3-daily-sync.yaml` — must pass validation.
Run `scout runbook validate -f b3-daily-sync.yaml` — must pass.
(Do NOT run `apply` in tests — it requires a live browser against B3.)

**Phase 3 done when:**
- [ ] `{{ var.name }}` interpolation works in navigate URLs and fill values
- [ ] `{{ env.NAME }}` reads OS environment
- [ ] `loop.for` iterates over a list, `capture_as` collects results
- [ ] `loop.while` with `max` safety cap works
- [ ] `loop.repeat` with index works
- [ ] `if` skips step when expression is false
- [ ] `on_failure` runs sub-sequence and suppresses parent failure
- [ ] All 6 new actions registered and resolve correctly in `runbook plan`
- [ ] B3 reference runbook passes plan + validate
- [ ] `go build ./...` passes
- [ ] `go test -short ./...` passes

---

### Phase 4 — Scheduling Layer

**Goal:** `scout schedule` commands + cron daemon + `schedule_run` MCP tool.

**Step 4.1 — Schedule schema**

File: new `pkg/scout/schedule/` package.

```go
type Schedule struct {
    Name    string            `yaml:"name"`
    Cron    string            `yaml:"cron"`
    TZ      string            `yaml:"tz"`
    Runbook string            `yaml:"runbook"`
    Vars    map[string]string `yaml:"vars,omitempty"`
    Timeout duration          `yaml:"timeout,omitempty"`
    OnFailure FailureConfig   `yaml:"on_failure,omitempty"`
}

type FailureConfig struct {
    Notify      string `yaml:"notify"`       // stderr | file | webhook
    NotifyFile  string `yaml:"notify_file,omitempty"`
    NotifyWebhook string `yaml:"notify_webhook,omitempty"`
}

type ScheduleFile struct {
    Schedules []Schedule `yaml:"schedules"`
}
```

Parse from `scout-schedule.yaml` in the working directory. Use a pure-Go cron
parser library (recommend `github.com/robfig/cron/v3` — already common in Go
ecosystem; add as dependency if not present).

**Step 4.2 — Run result persistence**

Write each run result to `~/.scout/runs/<schedule-name>/<ISO-timestamp>.json`:
```json
{
  "schedule": "b3-daily-sync",
  "started_at": "2026-06-16T07:00:01-03:00",
  "finished_at": "2026-06-16T07:03:44-03:00",
  "status": "success",
  "duration_ms": 223000,
  "outputs": {},
  "error": null
}
```

**Step 4.3 — CLI commands**

File: new `cmd/scout/schedule.go`.

```
scout schedule start              # foreground scheduler
scout schedule daemon             # background (use existing daemon_*.go)
scout schedule list               # jobs + next 3 fire times
scout schedule run <name>         # immediate one-shot
scout schedule stop               # signal daemon shutdown
scout schedule history <name>     # last N run results
scout schedule status             # daemon health + last run per job
```

**Step 4.4 — Vault unlock for unattended runs**

Before executing a scheduled runbook, unlock the vault via:
1. `SCOUT_VAULT_PASS` env var
2. `--vault-pass-file <path>` flag

Lock vault after run completes. Zero passphrase from memory immediately after
unlock (follow existing vault pattern in `cmd/scout/vault.go`).

**Step 4.5 — `schedule_run` MCP tool**

File: `pkg/scout/mcp/` — add `schedule_run` tool.
Params: `name string` — schedule name to fire immediately.
Returns: run result JSON.
Implementation: calls the same runbook executor as `scout runbook apply`.
Does NOT require the daemon to be running.

**Phase 4 done when:**
- [ ] `scout schedule list` reads `scout-schedule.yaml` and shows next fire times
- [ ] `scout schedule run <name>` fires a runbook immediately and writes result file
- [ ] `scout schedule start` runs the scheduler loop and fires jobs at correct times
- [ ] `scout schedule history <name>` reads and displays result files
- [ ] `schedule_run` MCP tool fires a named schedule and returns result JSON
- [ ] Vault unlocks before job, locks after
- [ ] `go build ./...` passes
- [ ] `go test -short ./...` passes

---

## Build + test commands

```bash
cd D:/weaver-sync/development/personal/projects/scout

# build
go build ./...

# test (short — skips browser integration tests)
go test -short ./...

# full build with race detector
go build -race ./...

# lint
golangci-lint run --fix ./... --timeout=5m
```

---

## Key decisions (do not second-guess these)

- **Layered implementation order is mandatory.** Phase 2 cannot start until Phase 1 is green. Phase 3 cannot start until Phase 2 is green. Etc.
- **Runbook engine is the single implementation.** MCP tools and the scheduler are thin callers. No logic duplication between surfaces.
- **Vault-encrypted sessions by default.** Plaintext Playwright-compatible export is opt-in via `--format=playwright`. Never write plaintext to disk by default.
- **eval() fix is transparent.** No MCP tool signature change. Just fix the return path for primitive values.
- **wait modes are additive.** The existing CSS selector wait is unchanged. New modes are opt-in via `wait_for` param.
- **Playwright-compat export omits sessionStorage.** Playwright's storageState format doesn't include sessionStorage. Scout stores it in the vault but omits it from export. Document this clearly in the export output or help text.
- **No cross-engine support.** CDP + Chromium only. No Firefox/WebKit.
- **Expression engine is intentionally minimal.** `text/template` subset. No arbitrary code execution in runbooks. Runbooks stay auditable.
- **`now.year`/`now.month`/`now.date` are built-in variables.** Do not make users write `{{ env.CURRENT_YEAR }}` — Scout provides date context automatically.

---

## MCP tool count reference

| After phase | Tool count | New tools |
|-------------|-----------|-----------|
| Baseline | 18 | — |
| Phase 1 | 19 | `hijack_watch` |
| Phase 2 | 22 | `session_save`, `session_load`, `session_export`, `download_wait` |
| Phase 3 | 22 | (no new MCP tools — new runbook actions only) |
| Phase 4 | 23 | `schedule_run` |

Update the tool count in `cmd/scout/mcp.go` long description at each phase.

---

## Files to create (new)

```
pkg/scout/session/          ← new package (Phase 2)
  session.go
  session_test.go
  playwright.go             ← export/import Playwright format
  playwright_test.go

pkg/scout/schedule/         ← new package (Phase 4)
  schedule.go
  scheduler.go
  result.go
  schedule_test.go

cmd/scout/schedule.go       ← new CLI commands (Phase 4)
```

## Files to modify (existing)

```
internal/engine/browser/    ← networkidle, URL wait, download hooks (Phase 1+2)
pkg/scout/mcp/              ← hijack_watch, session_*, download_wait, schedule_run
pkg/scout/runbook/          ← vars, loops, conditionals, new action registry
cmd/scout/session.go        ← new subcommands: save, load, export, import, delete
cmd/scout/mcp.go            ← tool count update
```

---

## Uncommitted files (from this design session — commit before starting)

```
?? docs/scout-playwright-gaps.md
?? docs/superpowers/specs/2026-06-16-scout-playwright-parity-design.md
```

Commit these first:
```bash
git add docs/scout-playwright-gaps.md docs/superpowers/specs/2026-06-16-scout-playwright-parity-design.md
git commit -m "docs: Scout Playwright-parity gap analysis and design spec"
```

---

## Memory entries relevant to this work

- Scout is CDP-only (Chromium family) — no Firefox/WebKit, by design
- Vault uses Argon2id + AES-256-GCM — follow existing patterns in `pkg/scout/vault/`
- Tests use real browser against test fixtures — no mocking of the engine
- Go style: `log/slog` structured JSON logging; `errors.Is`/`errors.As` for error checks
- Existing `hijack watch --body` is the model for response body capture
- B3 financial portal (Brasil) is the reference implementation use case
