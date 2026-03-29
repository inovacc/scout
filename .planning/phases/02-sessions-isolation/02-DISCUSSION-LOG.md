# Phase 2: Sessions & Isolation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 02-sessions-isolation
**Areas discussed:** Session reuse policy, Windows process detection

---

## Session Reuse Policy

| Option | Description | Selected |
|--------|-------------|----------|
| No implicit reuse | Every New() gets fresh session. Opt-in via WithReuseSession() | ✓ |
| Keep reuse, fix detection | Keep deterministic hash but fix stale PID detection | |
| Random session IDs always | UUID-based dirs, no deterministic hashing | |

**User's choice:** No implicit reuse (Recommended)
**Notes:** Fresh sessions by default. Explicit WithReuseSession() for persistence.

---

## Windows Process Detection

| Option | Description | Selected |
|--------|-------------|----------|
| WaitForSingleObject with 0 timeout | Accurate zombie detection, immediate return | ✓ |
| Exit code check | GetExitCodeProcess — simpler but less reliable | |
| Increase file lock retries | Keep OpenProcess, bump retries — treats symptoms | |

**User's choice:** WaitForSingleObject with 0 timeout (Recommended)

---

## Claude's Discretion

- Rod fallback elimination strategy
- Session cleanup implementation details
- Browser isolation enforcement approach
- File lock retry timing
- WithReuseSession() API design

## Deferred Ideas

None — discussion stayed within phase scope.
