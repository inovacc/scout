# SPEC — scout.pid binary consolidation

## Goal

Collapse `active-sessions/` and per-session `.lock/pid` into a single fixed-width binary `<scouthome>/sessions/<uuid>/scout.pid`. Keep top-level `current-session` pointer untouched. Hard cutover — no JSON-read fallback.

## File layout — `scout.pid` v1 (432 bytes, little-endian)

| Off  | Size | Field            | Type    | Notes |
|------|------|------------------|---------|-------|
| 0    | 4    | Magic            | `SCT1`  | ASCII bytes |
| 4    | 2    | Version          | u16     | starts at 1 |
| 6    | 2    | Flags            | u16     | bit0=Reusable, bit1=Headless |
| 8    | 4    | ScoutPID         | i32     | |
| 12   | 4    | BrowserPID       | i32     | |
| 16   | 4    | BrowserParentPID | i32     | |
| 20   | 4    | _reserved        |         | |
| 24   | 8    | CreatedAt        | i64     | unix nano |
| 32   | 8    | LastUsed         | i64     | unix nano |
| 40   | 8    | ExpiresAt        | i64     | unix nano, 0 = no expiry |
| 48   | 32   | Browser          | utf8    | null-padded |
| 80   | 32   | DomainHash       | utf8    | 12 hex chars used |
| 112  | 64   | Domain           | utf8    | eTLD+1 |
| 176  | 128  | Exec             | utf8    | null-padded path |
| 304  | 64   | BuildVersion     | utf8    | null-padded |
| 368  | 64   | BrowserStartToken| utf8    | null-padded |
| 432  | —    | EOF              |         | |

Strings: null-padded, truncate at cap. Reader stops at first `\0`. No length prefix.

## Locking — LockFileEx / flock on scout.pid

- New file: `internal/engine/session/lock_windows.go` — `LockFileEx(handle, EXCLUSIVE|FAIL_IMMEDIATELY, ...)` for write, shared for read.
- New file: `internal/engine/session/lock_unix.go` — `syscall.Flock(fd, LOCK_EX|LOCK_NB)` / `LOCK_SH`.
- Scout process holding session keeps exclusive lock for its lifetime.
- `ReadInfo` (audit, list) takes shared lock.
- Lock released on close OR process exit (OS handles cleanup).
- Eliminates `.lock/` mkdir mutex entirely.

## Hard cutover

- On `main()` startup: detect any session dir whose `scout.pid` is JSON (first byte not `S`). Delete that session dir.
- Delete top-level `active-sessions/` dir unconditionally on startup.
- No legacy reader code path.

## Files touched

- `internal/engine/session/info.go` — rewrite marshal/unmarshal (binary, not JSON)
- `internal/engine/session/lock.go` — replace mkdir mutex API
- `internal/engine/session/lock_windows.go` — NEW
- `internal/engine/session/lock_unix.go` — NEW
- `internal/engine/session/track.go` — drop `.lock/` references, update lock guard
- `internal/engine/session/active.go` (or wherever active-sessions writes happen) — drop
- `internal/engine/session/cleanup.go` — add startup purge of legacy JSON + active-sessions dir
- Any test using `.lock/pid` path directly

## Out of scope

- `current-session` file — untouched
- `job.json` — untouched (per-session job tracking, separate concern)
- gRPC proto changes — none, wire format unchanged

## Tests

- Binary roundtrip (write → read → equal)
- Truncated string fields
- Lock contention (two procs, second fails fast)
- Startup purge of JSON-format scout.pid
- Audit/list reads under shared lock while owner holds exclusive

## Migration risk

Hard cutover deletes all existing sessions on first run. User accepted.
