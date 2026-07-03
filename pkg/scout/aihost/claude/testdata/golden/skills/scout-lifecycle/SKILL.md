---
name: scout-lifecycle
description: Use when reasoning about Scout's session/browser lifecycle — why sessions leak, how the reaper preserves vs reaps, what the plugin's Claude Code hooks do, and how to keep ~/.scout/sessions clean. The model behind the session-janitor agent and the lifecycle hooks.
created: 2026-05-24
---
# Scout lifecycle

Scout has no daemon — every invocation opens a browser session, works, and tears down. The stdio MCP server (scout mcp) is a long-lived server bolted onto that per-command engine, so it inherits the same teardown gaps.

## Session model
- Each session lives in ~/.scout/sessions/<hash>/ with scout.pid (owner PID, browser PID, a PID-reuse start-token, Reusable, ExpiresAt), job.json, and monitor sidecars.
- The reaper (ReapOnce) is the always-on safety net. It PRESERVES a session iff its ScoutPID owner is live and identity-verified, OR it is Reusable and not expired. Everything else is reaped: kill the recorded browser PID (start-token verified), path-bounded kill of any chrome holding the data dir, then remove the dir.

## Where sessions leak (why this plugin exists)
- Scout's own teardown is best-effort: SIGINT-tier signal cleanup only, and the MCP path has no defer state.reset(), so a clean Claude Code exit or crash can orphan headless Chrome + session dirs.
- The plugin's Claude Code hooks are the guaranteed OUTER lifecycle Scout cannot give itself:
  - SessionStart -> reap prior leaks + inject a clean baseline.
  - Stop -> end-of-turn reclaim of idle/orphaned sessions.
  - SessionEnd -> the guaranteed teardown (scout session reset / reap).
  - PreToolUse -> advisory guardrail for the scout MCP tools (opt-in hard-deny via SCOUT_DENY_TOOLS / SCOUT_ALLOW_TARGETS).

## Keeping it clean
- Audit: scout session audit (classify), scout session doctor (health + pending queue).
- Reclaim: scout session audit --kill (targeted, respects Reusable) or scout session reset --all (full teardown).
- The session-janitor agent automates this; the scout-doctor agent diagnoses install + session health.
- REUSABLE + HEALTHY sessions are preserved by every reap — only a full reset drops them.
