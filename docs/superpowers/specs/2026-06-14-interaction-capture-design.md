# Interaction Capture — Design Spec

**Date:** 2026-06-14
**Status:** approved (brainstorm), pending implementation plan
**Owner:** scout

## 1. Goal & motivation

Provide an opt-in capability that records a structured, correlated trace of every
interaction with Scout — what is *passed to* Scout (CLI invocations, MCP tool
calls, gRPC/agent requests) and what Scout *does* (browser actions, network) — to
local, AI-friendly files under `<scouthome>/captures/`
(`C:\Users\dyamm\AppData\Local\Scout\captures\` on Windows). The captured data is
for later analysis and enhancement of Scout itself; analysis is performed
externally (e.g. by pointing Claude at the files). Scout performs no LLM analysis
of captures in v1.

## 2. Locked decisions

| Dimension | Choice |
|---|---|
| Scope | Full sessions: control-plane (CLI / MCP / gRPC / agent) + browser actions + network |
| Deliverable | Capture-only, AI-friendly JSONL; no in-Scout LLM (v1) |
| Redaction | Redact secrets, keep payloads (body cap 64 KB; files `0o600`, dir `0o700`) |
| Architecture | Central recorder (`internal/interaction`) + thin emit hooks at existing chokepoints |
| Output | `<scouthome>/captures/<id>.jsonl` |
| Enablement | Feature flag (`scout interactions on`) + `SCOUT_INTERACTIONS=1` |
| Phasing | v1: CLI + MCP + browser actions · v2: gRPC + agent + network + console · v3: rotation/prune + `capture show` |

## 3. Architecture

### 3.1 Package: `internal/interaction`

Named `interaction` to avoid colliding with the existing native-messaging
`pkg/scout/capture` package. It is `internal/` so every layer (cmd, pkg, grpc)
can import it; the engine stays decoupled (it never imports it — see §3.5).

Public surface:

```go
// Enabled reports whether capture is on (feature flag or SCOUT_INTERACTIONS env).
func Enabled() bool

// Dir returns the capture directory (GetFeatureData("interactions") or
// scouthome.Sub("captures")).
func Dir() (string, error)

// Recorder is a per-session/per-invocation append-only JSONL writer. All methods
// are safe on a nil *Recorder (no-op), so callers never branch on Enabled().
type Recorder struct { /* unexported */ }

// Open returns a Recorder writing <Dir>/<id>.jsonl, or (nil, nil) when capture is
// disabled. It writes a session_start header event. entrypoint ∈ {cli,mcp,grpc,agent}.
func Open(entrypoint, id string) (*Recorder, error)

func (r *Recorder) Emit(e Event)
func (r *Recorder) Close(status string) error

// Process-global default recorder for single-session processes (CLI, MCP, agent).
func Init(entrypoint, id string) *Recorder   // sets + returns the default
func Default() *Recorder                       // nil if not initialised
func Emit(e Event)                             // Default().Emit(e)
func Close(status string) error                // Default().Close(status)
```

Single-session processes (one CLI invocation, one MCP server, one agent server)
use the process-global default (`Init`/`Emit`/`Close`). The multi-session gRPC
daemon (v2) creates one `Recorder` per session ID instead.

### 3.2 Event schema (JSONL)

One normalized record per line; `kind` makes each line self-describing.

```go
type Event struct {
    Seq        int            `json:"seq"`
    TS         string         `json:"ts"`                  // RFC3339Nano
    Kind       string         `json:"kind"`                // session_start|cli|mcp_tool|browser_action|grpc|agent_call|network|console|session_end
    Source     string         `json:"source,omitempty"`    // cli|mcp|grpc|agent|browser
    Name       string         `json:"name,omitempty"`      // command/tool/verb/method
    Input      map[string]any `json:"input,omitempty"`     // redacted args/params
    Result     string         `json:"result,omitempty"`    // short, capped summary
    OK         *bool          `json:"ok,omitempty"`
    Error      string         `json:"error,omitempty"`
    DurationMS int64          `json:"duration_ms,omitempty"`
    Extra      map[string]any `json:"extra,omitempty"`     // kind-specific: url, status, bytes, exit, version, os...
}
```

Example file:

```jsonl
{"seq":0,"ts":"2026-06-14T17:40:00.1Z","kind":"session_start","source":"cli","extra":{"scout_version":"dev","entrypoint":"cli","os":"windows/amd64","id":"cli-2abc..."}}
{"seq":1,"ts":"2026-06-14T17:40:00.2Z","kind":"cli","source":"cli","name":"gather","input":{"args":["https://x","--har"]},"ok":true,"duration_ms":1240,"extra":{"exit":0}}
{"seq":2,"ts":"2026-06-14T17:40:00.3Z","kind":"mcp_tool","source":"mcp","name":"navigate","input":{"url":"https://x"},"ok":true,"duration_ms":52}
{"seq":3,"ts":"2026-06-14T17:40:00.4Z","kind":"browser_action","source":"browser","name":"click","input":{"selector":"#btn"},"ok":true,"duration_ms":11}
{"seq":4,"ts":"2026-06-14T17:40:01.5Z","kind":"session_end","extra":{"status":"ok","events":4,"duration_ms":1500}}
```

### 3.3 Recorder lifecycle & files

- File: `<Dir>/<id>.jsonl`, opened `O_CREATE|O_APPEND|O_WRONLY`, mode `0o600`; dir created `0o700`.
- IDs: CLI → `cli-<ksuid>`; MCP server → `mcp-<ksuid>`; agent → `agent-<ksuid>`; gRPC session (v2) → the engine session id (`pkg/id`).
- `Open` writes `session_start`; `Close(status)` writes `session_end` (event count + total duration) and flushes/closes.
- Buffered writer; flush per Emit (durability over throughput; capture is low-rate) — revisit if hot.
- Monotonic `Seq` per recorder, guarded by a mutex (Emit is concurrency-safe — MCP/agent are multi-goroutine).

### 3.4 Emit hooks

Each hook is ~3 lines at an existing chokepoint. v1 hooks in **bold**.

| Surface | Chokepoint | Event kind | Phase |
|---|---|---|---|
| **CLI** | root `PersistentPreRunE` + post-run (`cmd/scout/scout.go`, beside logger) | `cli` | **v1** |
| **MCP** | `addTracedTool` wrapper (`pkg/scout/mcp/server.go`) | `mcp_tool` | **v1** |
| **Browser actions** | `pkg/scout/tools/` verbs (page/form/tabs/crawl/…) | `browser_action` | **v1** |
| gRPC | unary + stream interceptor (`grpc/server`) | `grpc` | v2 |
| agent | `/call` handler (`pkg/scout/agent`) | `agent_call` | v2 |
| network | reuse HAR/hijack recorder; emit per request or link the HAR artifact | `network` | v2 |
| console | console monitor | `console` | v2 |

- **CLI**: the root `PersistentPreRunE` already calls `logger.StartExecution`; add `interaction.Init("cli", id)` + emit a `cli` event (command, redacted args) on completion in `main()` next to `logger.EndExecution`. Honor `flags.ShouldIgnoreCommand` (don't capture the `capture`/`logger` management commands themselves).
- **MCP**: `addTracedTool` wraps every tool; add an `interaction.Emit` alongside the span with tool name, redacted args, ok/error, duration.
- **Browser actions**: emit from the `tools/` verbs (the shared REPL+MCP executor). The `tools.TestVerbParity` manifest is the checklist of verbs to cover.

### 3.5 Engine decoupling

`internal/engine` does **not** import `internal/interaction`. v1 browser-action
capture lives entirely in the `pkg/scout/tools/` layer (above the engine). v2
network/console capture reuses the engine's existing HAR/hijack/console recorders
(their artifacts are linked or summarized into the capture) or, if an in-engine
emit is unavoidable, via an injected `func(Event)` sink set through a `With…`
option — the same decoupling pattern as `Browser.InstallRequestFilter`.

## 4. Enablement

New command `scout interactions` — resolved to this name (not `scout capture`)
because the `capture` namespace is already taken (`cmd/scout/capture.go` defines
`capture-host`/`capture-key`, plus `flow/profile/vault capture` subcommands). The
output directory is still `captures/`.

- `scout interactions on [--dir <path>]` → `flags.EnableFeature("interactions", dir)` (empty dir → default `scouthome.Sub("captures")`).
- `scout interactions off` → `flags.DisableFeature("interactions")`.
- `scout interactions status` → enabled?, dir, file count/size.
- `scout interactions list` → recent capture files with size.

`flags.ExportFlagsToEnv()` (already called in `PersistentPreRunE`) propagates the
flag to `SCOUT_INTERACTIONS=1`, so the daemon and subprocesses inherit it. Ad-hoc:
`SCOUT_INTERACTIONS=1 scout …`. `Enabled()` = `flags.IsFeatureEnabled("interactions") || truthy(os.Getenv("SCOUT_INTERACTIONS"))`.

## 5. Redaction & safety

Extract a shared `internal/redact` package (currently `redactArgs` lives in
`internal/logger`):

- `redact.Args([]string) []string` — moved verbatim from the logger; logger calls it.
- `redact.Map(map[string]any) map[string]any` — masks values whose key matches the sensitive pattern (password/token/secret/key/cookie/bearer/auth/credential). Used for MCP/browser `input`.
- `redact.URL(string) string` — keeps scheme+host+path, masks sensitive query params (`token`, `key`, `secret`, `access_token`, `sig`, `password`, …).
- `redact.Header(name, value string) string` — masks `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, …
- `redact.Body([]byte, max int) (string, bool)` — truncates to `max` (default 64 KB), reports truncation.
- Shared `redact.Pattern` (regexp) + `redact.Placeholder` (`***REDACTED***`).

The logger's existing `redactArgs` test moves to `internal/redact`. The logger
keeps behaviour identical (delegates to `redact.Args`).

Safety:
- Files `0o600`, dir `0o700` (reuse the hardening convention).
- Body/result fields capped (64 KB) with a truncation marker.
- Local-only — captures are never transmitted anywhere.

## 6. Rotation & retention (v3)

- Per-file size cap (default 64 MB) → roll to `<id>.1.jsonl`, `<id>.2.jsonl`, …
- Directory budget: keep the most recent N files (default 500) or total size (default 1 GB); prune oldest.
- `scout capture prune [--max-files N | --max-size SIZE | --older-than DUR]`.

## 7. Phasing

- **v1** — `internal/interaction` (Recorder, Event, gate, default singleton); `internal/redact` extraction; `scout capture on/off/status/list`; hooks for **CLI, MCP, browser actions**; session framing.
- **v2** — gRPC unary/stream interceptors (per-session recorders), agent `/call`, network (HAR link/summary), console.
- **v3** — rotation + retention + `scout capture prune` + `scout capture show <id>` pretty-printer.

## 8. Testing strategy

Real browser + httptest, no mocks (project convention).

- `internal/redact`: `Args` (existing cases) + `Map`, `URL`, `Header`, `Body`.
- `internal/interaction`:
  - Recorder roundtrip: `Open` → `Emit`×N → `Close`; read the JSONL back; assert framing (`session_start`/`session_end`), monotonic `seq`, counts.
  - No-op safety: `Open` returns `(nil,nil)` when disabled; nil-receiver `Emit`/`Close` don't panic.
  - File mode `0o600` (unix-gated; Windows doesn't honor Unix perms).
  - Size-cap behaviour (v3).
- Integration:
  - CLI: run a command with `SCOUT_CAPTURE=1` → assert a `<id>.jsonl` with `session_start` + `cli` + `session_end`.
  - MCP: a wrapped tool call emits the expected `mcp_tool` event (unit via `addTracedTool` with a test recorder).
  - Browser action: a `tools/` verb emits a `browser_action` event (browser-gated; skips without Chromium).

## 9. Out of scope (YAGNI)

- In-Scout LLM analysis of captures (capture-only; analysis is external).
- Capturing Go library-API calls (no public entry point to hook).
- Telemetry / off-machine upload.
- Replay of captures (that is `pkg/scout/flow`'s responsibility).

## 10. Risks & open items

- **Command-name collision** — resolved: the command is `scout interactions` (the `capture` namespace is taken). Output dir remains `captures/`.
- **Volume** — full-session capture (esp. browser actions + future network) can be large; the size caps + retention (v3) bound this; v1 ships without rotation, so document the unbounded-growth caveat until v3.
- **Redaction completeness** — redaction is best-effort pattern-based; the "keep payloads" choice means the capture dir can still contain sensitive business data (not secrets) — `0o600`/local-only is the control. Documented.
- **Double-capture (CLI)** — the logger and capture both record CLI command/args; acceptable (different consumers), but capture does not duplicate full stdout/stderr by default (only a capped summary) to avoid bloat.
