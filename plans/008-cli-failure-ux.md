# Plan 008: CLI failures are visible, distinguishable, and clean up after themselves

> **Executor instructions**: Follow step by step; verify each; honor STOP conditions; update
> `plans/README.md` when done. Steps are independent — land separately if you like.
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- cmd/scout/scout.go cmd/scout/repl.go cmd/scout/gather.go pkg/scout/plugin/command_proxy.go`
> Any change → compare excerpts (STOP on mismatch).

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx (correctness-adjacent)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

The "poor UX" half of the report. Failures are invisible or indistinguishable, and some exit paths
skip cleanup (leaking Chrome). Findings: [75] exit code is always 1; [17] REPL dies silently on
>64KB stdin or any stdin error; [21] `os.Exit` in the plugin command proxy skips logger/interaction
flush and leaks a provisioned browser; [18] `gather` swallows screenshot/HAR write failures while
reporting success; [34] `--help` advertises a "background gRPC daemon" that doesn't exist; [19]
`PersistentPreRunE` swallows a flag-export error and early-returns, silently disabling logging and
interaction capture for that run.

## Current state

```go
// cmd/scout/scout.go:30-31   stale help (contradicts the no-daemon design)
Commands communicate with a background gRPC daemon for session persistence,
or run standalone for one-shot operations.

// scout.go:34-37   PersistentPreRunE swallows the flag-export error and returns early
if err := flags.ExportFlagsToEnv(); err != nil {
    return nil // non-fatal            // ← skips logger init + interaction capture below
}

// scout.go:119-137   exit code is always 1
status := "ok"; if err != nil { status = "error" }
_ = interaction.Close(status)
if err != nil {
    _, _ = fmt.Fprintf(os.Stderr, "scout: %v\n", err)
    return 1                            // ← flag misuse, launch failure, panic all → 1
}
return 0

// cmd/scout/repl.go:58,73   scanner error never checked
scanner := bufio.NewScanner(os.Stdin)  // default 64KB max token
...
if !scanner.Scan() { break }           // ← >64KB or transient error silently ends the REPL

// cmd/scout/gather.go:79-83, 91-93   write failures swallowed
if data, err := decodeBase64(...); err == nil {
    if err := os.WriteFile(ssFile, data, 0o600); err == nil { print "saved" }
}                                       // ← both else-branches: no message, no file

// pkg/scout/plugin/command_proxy.go:141-143   os.Exit skips cleanup
if cmdResult.ExitCode != 0 { os.Exit(cmdResult.ExitCode) }  // ← bypasses Execute() wrapper
```

Conventions: `rootCmd` has `SilenceErrors/SilenceUsage=true`; the `Execute()` wrapper
(`scout.go:~95-138`) is the single place that prints errors and flushes `interaction`/logger, then
`os.Exit`s. Any `os.Exit` elsewhere bypasses that flush. Error prefix `scout: %v`.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Tests | `go test ./cmd/scout/ ./pkg/scout/plugin/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**: the four files above, plus a small exit-code helper. Tests.
**Out of scope**: MCP error text (plan 005 Step 5); the signal-handler *cleanup* correctness beyond
de-conflicting the two handlers (Step 6 is audit-and-unify, not a rewrite).

## Steps

### Step 1: Meaningful exit codes

Introduce a tiny typed-error → exit-code mapping in the `Execute()` wrapper. At minimum distinguish:
`2` = usage/flag error (cobra returns these; detect via `cmd.FlagErrorFunc` or a sentinel), `1` =
runtime error, `130` = interrupted (already via signal). Keep the stderr print. Example:
```go
if err != nil {
    _, _ = fmt.Fprintf(os.Stderr, "scout: %v\n", err)
    return exitCodeFor(err)   // 2 for usage errors, 1 otherwise
}
```
Wire cobra usage errors to code 2 (set `rootCmd.SilenceUsage=false` for flag errors, or classify via
a `UsageError` sentinel). Don't over-engineer — two or three codes is the win.

**Verify**: `scout --nonexistent-flag; echo $?` prints usage guidance and exits `2`; a real runtime
failure exits `1`. Add a `cmd/scout` test asserting the mapping.

### Step 2: PersistentPreRunE must not silently disable logging

Change the swallow to either surface the error or continue to the rest of the hook. The intent was
"non-fatal" — so **log a warning and continue**, don't `return nil` (which skips logger +
interaction init below):
```go
if err := flags.ExportFlagsToEnv(); err != nil {
    slog.Warn("scout: export flags to env failed; continuing", "err", err)
    // fall through — do NOT return; logging/interaction init below must still run
}
```

**Verify**: `go build ./cmd/scout/` → exit 0; confirm by reading that the code below the block still
runs when `ExportFlagsToEnv` errors.

### Step 3: REPL survives large input and reports stdin errors

In `repl.go`, raise the scanner buffer and check `scanner.Err()` after the loop so a >64KB paste or
a transient error doesn't silently kill the session:
```go
scanner := bufio.NewScanner(os.Stdin)
scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)   // allow up to 1MB lines
for { ... if !scanner.Scan() { break } ... }
if err := scanner.Err(); err != nil {
    _, _ = fmt.Fprintf(out, "scout: repl: input error: %v\n", err)
}
```
A single failed command must also not kill the REPL — confirm the per-command dispatch already
`continue`s on error (it prints and loops); if a command error currently `return`s, change it to
print-and-continue. **STOP** and report if a browser-death inside a command wedges the loop — that
overlaps plans 002/003 and should be verified, not patched blindly here.

**Verify**: pipe a >64KB line into `scout repl` — it reports an input error instead of exiting 0
silently. Manual, plus a unit test on the scan-buffer size if the loop is factored out.

### Step 4: `gather` reports write failures

Replace the `err == nil` silent-success branches with explicit error reporting so a read-only dir or
full disk is visible:
```go
data, err := decodeBase64(result.Screenshot)
if err != nil { return fmt.Errorf("scout: gather: decode screenshot: %w", err) }
if err := os.WriteFile(ssFile, data, 0o600); err != nil {
    return fmt.Errorf("scout: gather: write screenshot %s: %w", ssFile, err)
}
_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Screenshot saved: %s\n", ssFile)
```
Same for the HAR branch. Decide whether a write failure should fail the command (recommended) or
warn; if the JSON output also strips the data when saving, ensure it isn't stripped when the save
failed. **STOP** if the data-stripping logic is entangled such that fixing it risks the happy path —
report the shape.

**Verify**: `scout gather --screenshot --save-screenshot -o /some/readonly/x.json` prints an error
and exits non-zero (before: success, no file).

### Step 5: Plugin command proxy returns an error instead of `os.Exit`

In `command_proxy.go`, replace the `os.Exit(cmdResult.ExitCode)` with an error return carrying the
code, so the `Execute()` wrapper flushes logging/interaction and the provisioned browser is closed:
```go
if cmdResult.ExitCode != 0 {
    return &ExitError{Code: cmdResult.ExitCode}   // Execute() maps ExitError → os.Exit(code)
}
```
Add an `ExitError` type and teach `exitCodeFor` (Step 1) to honor it. This composes with plan 002/
005's browser-close so the provisioned Chrome ([22]) is not leaked on plugin-command failure.

**Verify**: a plugin command that exits non-zero causes `scout` to exit with that code AND prints
via the wrapper AND leaves no orphan Chrome. `go test ./pkg/scout/plugin/` → pass.

### Step 6: Fix stale help + audit the competing SIGINT handlers

- Update the root `Long` help (`scout.go:30-31`) to describe the real model: per-command sessions,
  no daemon; MCP via `scout mcp`. Remove the "background gRPC daemon" sentence.
- Audit the two interrupt paths: the global `installSignalCleanup(closeAllLiveCleanup)` in `main()`
  (`scout.go:226`) and any per-command signal handling (e.g. `hijack watch`). They currently race —
  the global one hard-exits 130 and may skip a command's deferred teardown. Unify so Ctrl+C runs the
  command's graceful stop *then* the global cleanup, with one deterministic exit code. **STOP** and
  report the exact handler interaction if it's more tangled than "two handlers, pick one order" — a
  wrong change here can worsen teardown.

**Verify**: `scout --help` no longer mentions a daemon; Ctrl+C on `hijack watch` exits
deterministically (0 or 130 consistently) and flushes the output file.

## Test plan

- Exit-code mapping unit test (usage vs runtime).
- REPL scan-buffer + `Err()` handling (unit on the factored loop, or manual).
- `gather` write-failure surfaces an error (unit with a read-only temp path).
- Plugin command non-zero exit → `ExitError` path, no `os.Exit` (unit).
- Help text no longer mentions a daemon (`grep`).

## Done criteria

- [ ] `grep -n "background gRPC daemon" cmd/scout/scout.go` returns no match.
- [ ] `grep -rn "os.Exit(" pkg/scout/plugin/command_proxy.go` returns no match.
- [ ] `PersistentPreRunE` no longer `return nil` on `ExportFlagsToEnv` error (it warns + continues).
- [ ] REPL sets a larger scanner buffer and checks `scanner.Err()`.
- [ ] `gather` returns/reports an error on screenshot/HAR write failure.
- [ ] Exit codes distinguish usage (2) from runtime (1); test asserts it.
- [ ] `go build ./cmd/scout/ && go build ./pkg/...` exit 0; `go test ./cmd/scout/ ./pkg/scout/plugin/` pass; `task lint` exit 0.
- [ ] `plans/README.md` row updated.

## STOP conditions

- Excerpts drifted from `4ecf689`.
- The REPL per-command error path or the SIGINT handler interaction is more entangled than described
  (Steps 3/6) — report rather than patch blindly (overlaps plans 002/003/005).
- `gather`'s data-stripping is entangled with the save path (Step 4) — report.

## Maintenance notes

- The unifying rule: **one exit chokepoint** (`Execute()`), everything else returns errors. Any new
  `os.Exit`/`log.Fatal` outside the wrapper reintroduces the leak/flush bug — grep for them in review
  (`grep -rn "os.Exit\|log.Fatal" cmd/ pkg/ internal/`).
- [88]'s low-value note (RateLimiter swallows `Wait` error): if touching `internal/engine/ratelimit.go`
  for any reason, surface the error or validate `WithRateLimit`'s argument. Not required here.
- Reviewer: confirm no new `os.Exit` slipped in and that the signal-handler change didn't skip a
  command's deferred `hijacker.Stop()`/output flush.
