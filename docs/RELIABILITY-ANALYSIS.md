# Scout MCP Reliability Analysis — "works once, then disconnects / hangs / unreachable"

**Date:** 2026-06-25
**Scope:** Why Scout, used as a stdio MCP server inside Claude Code, becomes unreliable after a single use.
**Method:** Three parallel code investigations (MCP lifecycle, browser launch hangs, session/lock/process model). Headline claims spot-verified against source.

> This is a findings + backlog document. Nothing here is "fixed." Each item is a hypothesis backed by `file:line` evidence and a proposed change for you to judge and prioritize.

---

## TL;DR — three independent root causes, each enough to break it

The symptoms you described map to three *separate* defects that compound:

| Symptom you reported | Root cause | Evidence |
|---|---|---|
| "use it once, then Claude can't reach it again / disconnects" | **MCP server suicides after 5 min idle.** The idle timer cancels the server's context → the stdio process exits. Claude Code does **not** auto-respawn a dead stdio server mid-session. | `cmd/scout/scout.go:77` (default `--idle-timeout 5m`), `pkg/scout/mcp/server.go:249-256` → `360-376` |
| "hangs" | **Browser launch has no end-to-end deadline.** An untimed `http.Get` on Chrome's CDP debug port (and `ensureBrowser` calling `scout.New` with the context discarded) blocks the tool handler → the stdio stream goes silent → Claude sees an unresponsive server. | `internal/engine/lib/launcher/url_parser.go:123`, `launcher.go:569-579`, `pkg/scout/mcp/server.go:75` |
| "can't reach it again" (after the process is gone) | **MCP creates a non-reusable per-command session under a long-lived server.** When the MCP process exits, the next `scout` CLI run reaps the session dir; on Windows, AV/Search-Indexer/OneDrive file locks wedge cleanup. | `pkg/scout/mcp/server.go:62-75` (no `WithReusableSession`), `internal/engine/session/session_track.go` reaper |

**The architectural mismatch underneath all three:** Scout's model is "sessions are per-command, no daemon" — but an MCP server **is** a long-lived daemon. The MCP entry point bolts a long-lived server onto a lifecycle (idle-suicide, ephemeral session, untimed one-shot launch) designed for one-shot CLI commands.

---

## P0 — each independently makes Scout "single-use"

### P0-1 — MCP stdio server self-terminates on idle (default 5 min)
- **Where:** default `--idle-timeout 5m` at `cmd/scout/scout.go:77`; wired into `ServerConfig.IdleTimeout`; idle callback at `pkg/scout/mcp/server.go:249-256` runs `state.reset()` then `cb()`, where `cb` is the server context's `cancel` (`server.go:360`, `374`). When `Run(ctx, &mcp.StdioTransport{})` (`server.go:376`) sees the cancelled context, the process exits.
- **Why it explains the symptom:** A stdio MCP server is a child process owned by Claude Code. Once it exits, the pipe is closed permanently — Claude Code keeps the tool registered but every call fails with a transport/disconnect error until the *session* is reloaded. So: use Scout once, leave it 5 minutes, it's dead for the rest of your session.
- **Proposed fix:** For the stdio MCP path, **default idle shutdown OFF** (`idle-timeout 0`). The idle timer is appropriate for the headed `scout mcp open` window, not for a client-managed stdio server whose lifecycle the client already owns. Alternatively keep a timer but have it only release the *browser* (call `state.reset()`), never cancel the server context. Recommendation: do both — release the browser on idle, never kill the transport.

### P0-2 — Browser launch can hang forever (no deadline reaches the launch path)
- **Where:**
  - `ensureBrowser` discards its context and calls `scout.New(opts...)` with no timeout — `pkg/scout/mcp/server.go:53,75` (`//nolint:contextcheck`).
  - The launcher resolves the CDP endpoint with an **untimed** `http.Get` on `/json/version` — `internal/engine/lib/launcher/url_parser.go:123` (`//nolint: noctx`).
  - `Launcher.getURL()` blocks on an unbounded `select` over a `WithCancel(context.Background())` context (no deadline) — `internal/engine/lib/launcher/launcher.go:569-579`, ctx created `:136`.
  - Several `Browser.New` side-paths use `context.Background()`: resolve/download (`internal/engine/browser.go:346-348`), VPN connect (`:88`), ADB forward (`:105`).
- **Why it explains the symptom:** If Chrome is slow to expose its debug port — antivirus scanning the binary, a half-crashed Chrome, an offline first-run download, slow disk during unzip — the launch never returns. Because `ensureBrowser` runs inside the tool handler, the stdio response never arrives and Claude reports a hang.
- **Proposed fix:** Thread a real deadline from the MCP tool call through `ensureBrowser` → `scout.New` → launcher. Add an explicit launch timeout (e.g. 45–60s) and put a client timeout on the `/json/version` GET. On timeout, **fail fast with a clear error** ("browser failed to start within Ns; is Chrome installed / is AV blocking it?") instead of blocking the transport.

### P0-3 — MCP session is non-reusable; reaped once the server exits
- **Where:** `ensureBrowser` builds options with no `WithReusableSession` — `pkg/scout/mcp/server.go:62-75`. Non-reusable sessions are removed on `Browser.Close()` and reaped by `CleanStaleSessions()`/`ReapOnce()` on the next CLI invocation when the owning PID is gone — `internal/engine/session/session_track.go` (reaper) and `reaper.go`.
- **Why it explains the symptom:** After the server dies (P0-1) the next `scout` command cleans up the orphaned ephemeral session — so there is nothing to "reach again." This is largely a *consequence* of P0-1; fixing the idle-suicide keeps the server (and its one reused browser, `s.browser` at `server.go:58-59,89`) alive for the whole Claude session, which is the real fix. Still worth making the session reusable or explicitly server-owned so a brief MCP restart can re-attach.
- **Proposed fix:** Make ensureBrowser self-healing (P1-1) and decide ownership explicitly: either mark the MCP session reusable, or document that the browser lifetime == server lifetime and ensure clean teardown on exit.

---

## P1 — resilience gaps that turn a blip into a wedge

### P1-1 — `ensureBrowser` never re-launches a dead browser
- `server.go:58-60` returns the cached `s.browser` whenever it is non-nil, with no liveness check. If Chrome crashed or was killed (Windows reaper, user closed it, OOM), every subsequent tool call uses a dead CDP connection and errors forever — the server is up but wedged.
- **Fix:** Before returning the cached browser, check process/CDP liveness; if dead, drop it and re-launch. Self-heal instead of staying broken.

### P1-2 — Lock-acquire failure is silently ignored
- `registerSession()` proceeds without the lock when `AcquireSessionLock` fails (reported by the session investigation; verify exact lines in `internal/engine/browser.go` / `internal/engine/session/lock.go`). A stale `scout.lock` from a crashed process (Windows `LockFileEx`, `lock_windows.go`) then provides no protection and emits no warning.
- **Fix:** At minimum log a warning with the holder PID; detect-and-break a lock whose owning PID is provably dead (gops `IsScoutProcess`).

### P1-3 — First-run auto-download sits behind a tool call with a 10-min ceiling
- `browser.BestCached()` auto-downloads Chrome for Testing on first use (10-min timeout per the launch investigation, `internal/engine/browser/`). Hundreds of MB with no progress surfaced to the MCP client looks identical to a hang.
- **Fix:** Pre-flight the browser (download on server start, or a `scout browser ensure` step) and/or stream progress; fail fast and actionable when offline.

---

## P2 — Windows-specific cleanup robustness

### P2-1 — Session dirs wedge under AV / Search Indexer / OneDrive
- `internal/engine/session/session_track.go` documents a ~11s retry budget, `StartCleanupRetrier` every 60s, escalating to `forceBreakDir` only after ~20 failures (~20 min). On a machine where Defender/Search-Indexer hold Chrome SQLite+LevelDB, sessions can appear stuck for a long time.
- **Note / scope correction:** Scout's state lives in `%LOCALAPPDATA%\Scout` by default (not the `weaver-sync` synced tree), so OneDrive sync is only a factor if `SCOUT_HOME` is pointed into a synced folder. The foreign-SID "dubious ownership" issue affects git/go-build VCS stamping, **not** Scout's runtime cleanup. Don't conflate the two.
- **Fix:** Consider a faster forced-break path on Windows for non-reusable sessions, and verify the reaper's path-bounded Chrome scan (`reaper.go`) catches processes whose `scout.pid` is missing/corrupt.

### P2-2 — Leaked Chrome when metadata is lost
- The reaper kills the recorded `BrowserPID` and does a path-bounded scan, but a Chrome process with a missing/corrupt `scout.pid` and no data-dir lock can survive (`reaper.go`). Over many crashes these accumulate.
- **Fix:** Periodic gops-based sweep for orphaned Chrome children of dead scout PIDs.

---

## Recommended sequence

1. **P0-1** (disable idle-suicide for stdio MCP) — biggest single win; likely resolves the "use once then gone" report on its own.
2. **P0-2** (launch deadline + fast-fail) — kills the "hangs."
3. **P1-1** (self-healing ensureBrowser) — survives Chrome crashes without a restart.
4. **P0-3 / P1-2 / P1-3**, then the P2 Windows items.

## Verification (per project rule: "done" = proven on your real setup)
A fix is only proven by your own use: run Scout as the Claude Code MCP server, exercise a tool, wait >5 min idle, exercise again — it must still respond; then kill the Chrome process and call a tool — it must re-launch and respond. Tests passing is not evidence here; the failure modes are process/transport/timeout boundaries that tests typically stub.
