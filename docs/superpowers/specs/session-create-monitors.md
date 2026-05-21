# SPEC — Fold HAR / hijack / blocking into `scout session create`

## Goal

Make `scout session create` the single entry point for a fully-instrumented
browser session. Today the user has to chain `session create` → `har start`
→ `hijack watch` → manual `block` (not implemented). Move all of it into
one command + persist state inside the session dir so subsequent
`session destroy` can drain the artifacts automatically.

## Proposed flags

```
scout session create --url <u> [--headless=true]
  --har                          # enable HAR recording
  --har-out <path>               # default: <session>/har.json
  --hijack                       # enable passive request/response/WS capture
  --hijack-bodies                # also capture body payloads
  --hijack-out <path>            # default: <session>/hijack.jsonl
  --block <pattern>              # block matching URL, repeatable
  --block-method <m>             # restrict last --block to method (POST, etc.)
  --block-status <code>          # what to return on block (default 444)
  --console                      # capture browser console log to <session>/console.log
  --ws                           # capture WebSocket frames to <session>/ws.jsonl
```

Repeat semantics: `--block` can be repeated; if `--block-method` follows
a `--block`, it modifies only that one (positional). Same for `--block-status`.

## Persistence

Per-session sidecars under `<scouthome>/sessions/<id>/`:

```
scout.pid              # 432-byte binary (existing)
scout.lock             # 0-byte lock target (existing)
monitors.json          # NEW — declared monitors + output paths
har.json               # NEW — HAR recording (if --har)
hijack.jsonl           # NEW — newline-delimited hijack events (if --hijack)
console.log            # NEW — browser console output (if --console)
ws.jsonl               # NEW — WebSocket frames (if --ws)
data/                  # browser profile (existing)
```

`monitors.json` schema:

```json
{
  "har":     {"enabled": true, "path": "har.json"},
  "hijack":  {"enabled": true, "with_bodies": true, "path": "hijack.jsonl"},
  "console": {"enabled": false},
  "ws":      {"enabled": false},
  "blocks":  [
    {"pattern": "**/api/**/reembolsos**", "method": "POST", "status": 444},
    {"pattern": "**/api/**/expenses**",   "method": "POST", "status": 444}
  ]
}
```

Read by `session destroy` to know which artifacts to finalize, by
`session list` to surface monitor status, and by `session resume`
(future) to re-attach.

## gRPC proto additions

`CreateSessionRequest` gains:

```
bool   record_har        = 15;
bool   record_hijack     = 16;
bool   hijack_bodies     = 17;
bool   record_console    = 18;
bool   record_ws         = 19;
repeated BlockRule blocks = 20;

message BlockRule {
  string pattern = 1;
  string method  = 2;   // empty = any
  int32  status  = 3;   // 0 = default (444)
}
```

Server side, the daemon constructs the browser with:

```go
br, err := scout.New(
    scout.WithHeadless(req.Headless),
    scout.WithSessionHijack(...),         // if record_hijack
    scout.WithHARRecorder(...),           // if record_har
    scout.WithConsoleLog(...),            // if record_console
    scout.WithBlockRules(blocks...),      // new option
)
```

Writes `monitors.json` alongside the existing `scout.pid`.

## New engine APIs

1. `WithBlockRules(rules ...BlockRule) Option` — installs a HijackRouter
   that aborts matching requests with the configured status. Captures
   the would-be-sent request (method, URL, headers, body) into the
   HAR / hijack stream BEFORE aborting, so recon use cases still see
   the payload.
2. `WithHARRecorder(path string) Option` — replaces today's separate
   `har start` subcommand path. Auto-flushes on `Browser.Close()`.
3. `WithConsoleLog(path string) Option` — subscribes to CDP
   `Runtime.consoleAPICalled` and writes lines.

## CLI consolidation

- Keep `scout har start/stop/export` working against an already-running
  session (back-compat) but route through `monitors.json`.
- Keep `scout hijack watch` for ad-hoc monitoring of a session that
  didn't enable hijack at create time.
- `session destroy` flushes HAR + hijack + console buffers before
  removing the dir (or, for reusable, before releasing the lock).

## Tests

- e2e: `session create --har --block <pattern>` against httptest server,
  navigate, trigger blocked POST, confirm HAR contains the request
  with method/URL/body and the server saw zero hits.
- Block patterns: glob (`**`, `*`) matched against full URL.
- `monitors.json` persisted and re-read consistently.
- Block-with-status returns the configured code (not just abort).

## Effort

Medium-large. ~5-7 files touched:

- `grpc/proto/scout.proto` + regen
- `grpc/server/server.go` — wire new fields
- `cmd/scout/session.go` — new flags + flag-to-proto mapping
- `internal/engine/option.go` — `WithBlockRules`, `WithHARRecorder`, `WithConsoleLog`
- `internal/engine/browser.go` — install hijacker + sidecar writer
- `internal/engine/session/monitors.go` — NEW; load/save monitors.json
- `internal/engine/hijack/` — extend with block-rule path that captures
  request body before abort
- tests

## Open questions before coding

1. **Patterns** — glob like `**/api/**` (doublestar) or regex (`^/api/.*$`)?
   Glob is friendlier; regex more precise.
2. **Block + capture interaction** — abort happens at CDP `Fetch.failRequest`.
   Headers + body must be sniffed BEFORE the abort. Hijack pipeline already
   captures requests in `requestPaused` — confirm body is available at
   that stage for non-GET methods.
3. **Output formats** — HAR is HTTP Archive 1.2 (JSON). Hijack stream is
   newline-delimited JSON (`.jsonl`). Console is plain text. WS is `.jsonl`.
   OK with all four?
4. **Reusable sessions** — second-time-create on the same reusable session
   should re-apply monitors or keep prior config? Recommend re-apply
   (idempotent: write new monitors.json, overwrite previous).
5. **MCP exposure** — should the MCP server's `create_session`/`open` tools
   also accept these flags? Defer to follow-up.

## Migration

Existing `har start/stop/export` and `hijack watch` keep working against
sessions that didn't enable them at create. No breaking change.
