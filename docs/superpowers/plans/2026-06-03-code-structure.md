# Phase 5 — Code Structure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pay down internal-structure debt so the codebase is maintainable at scale and ready for the Phase 6 Shared Command Executor — split the remaining >1000-line files, consolidate stray logging onto `slog`, make `pkg/scout/browser` delegate to `internal/engine/browser`, and put the `scout:` error prefix on the highest-impact bare-error paths.

**Architecture:** Every task is a **behavior-preserving refactor**. There is no new feature surface and no exported-API change. The safety net is the *existing* test suite + `go build` + `go vet` + `golangci-lint`; a task is correct iff those stay green and exported signatures are byte-identical. File splits move code between files in the *same package* (Go allows a type's methods to live across files), so no import graph changes.

**Tech Stack:** Go 1.26, Taskfile (`task test`, `task check`), `golangci-lint` (`default: all`), `log/slog`. Run Go via `go` on PATH, fallback `C:\Program Files\Go\bin\go.exe`. Commits carry NO AI attribution.

**Scope note (from spec 2026-05-16-05-code-structure-design.md):** Full 657→2203 `fmt.Errorf` migration is **v2 / ERR-01 — OUT OF SCOPE**. Phase 5 establishes the convention and fixes only the highest-impact paths (Task 7). `internal/engine/lib/` (internalized rod) and generated `*.pb.go` are **never** touched.

---

## File Structure (what each split produces)

All splits keep the original file name as the "core" file and add sibling files in the **same package**. No symbol is renamed; no exported signature changes.

**`grpc/server/server.go` (1610) → 4 files (`package server`):**
- `server.go` — `ScoutServer`/`session` types, `New`, lifecycle, idle timer, stats, events, peer tracking, `getSession`, `sanitizeError`.
- `server_session.go` — `CreateSession`, `DestroySession`, `DestroyAllSessions`.
- `server_browser.go` — Navigate, Click, Type, GetText, Eval, Screenshot, PDF, recording, HAR RPCs.
- `server_hijack_stream.go` — hijack RPCs, `hijackEventToProto`, profile capture, `StreamEvents`, `Interactive`, `executeCommand`, `wireEvents`, `mapKey`.

**`internal/engine/browser/download.go` (1121) → 4 files (`package browser`):**
- `download.go` — `CacheDir`, `DownloadFile`, `Resolve`/`ResolveCached`, `copyDir`, `stripFirstDir`, registry-name helpers.
- `registry.go` — `RegistryFile`, `LoadRegistry`, `SaveRegistry`, `RegisterBrowser`, `LookupRegistryBrowser`, `ListDownloaded`, `LatestCachedBin`.
- `download_chromium.go` — `DownloadChromium`, `DownloadChrome`, `latestChromiumRevision`, `latestChromeForTesting`, bin/platform helpers.
- `download_brave_edge.go` — `DownloadBrave`, `DownloadEdge`, `downloadEdgeWindows/Unix`, MSI/PKG extraction, `latestEdgeRelease`/`latestBraveVersion`.

**`internal/engine/page_rod.go` (1103) → 4 files (`package engine`, methods on `*rodPage`):**
- `page_rod.go` — `rodPage` struct, accessors, cookies/headers/UA, `Close`, `String`/`Info`.
- `page_rod_nav.go` — Navigate(Back/Forward), Reload, GetWindow/SetWindow, SetViewport, Emulate.
- `page_rod_capture.go` — Screenshot, `ScrollScreenshot` (+opts), `CaptureDOMSnapshot`, PDF, GetResource, favicon, dialogs.
- `page_rod_wait_eval.go` — Wait* family, `EachEvent`/`WaitEvent`, AddScript/StyleTag, `ElementFrom*`, `ObjectToJSON`, `Call`, `Event`, `initEvents`.

**`internal/engine/github.go` (1006) → 4 files (`package engine`):**
- `github.go` — types (`GitHubRepo/Issue/PR/User/Release/CodeResult`), `GitHubOption`+`With*`, `githubConfig`/defaults.
- `github_repo.go` — `GitHubRepo`, `GitHubReleases`, `GitHubTree`, `GitHubUser`.
- `github_issues_pr.go` — `GitHubIssues`, `GitHubPRs`, `parseGitHubIssues`, `fetchGitHubBody`.
- `github_search.go` — `GitHubSearchCode`, `parseGitHubCodeResults`.

---

## How to execute a file-split task (read once, applies to Tasks 1–4)

A split moves code; it never rewrites logic. For each split:

1. **Baseline:** `go build ./...`-equivalent for the package + its tests are green BEFORE you start. (Use the package's own build/test target — see each task.)
2. **Create the new sibling files** with the same `package` clause and the imports each needs. **Cut** the listed declarations (functions/types/vars/consts, with their doc comments) from the original file and **paste** them verbatim into the target sibling — do not edit bodies.
3. **Fix imports:** run `goimports`/`gofmt`; remove now-unused imports from the original and add needed ones to the siblings. The compiler is the oracle.
4. **Verify gate (all must pass):** `go build ./<pkg>/` · `go vet ./<pkg>/` · `go test ./<pkg>/` (or the documented target) · `golangci-lint run ./<pkg>/` = 0 NEW issues · `gofmt -l` clean.
5. **Exported-signature guard:** `go doc` of the package before vs after must be identical (no symbol added/removed/renamed). A split that changes the public API is a bug — revert and redo.
6. **Commit** with the exact message.

> There is no "write a failing test" step for splits — the existing suite IS the test. If a package has weak coverage, that's noted but NOT expanded here (coverage is a separate /steps:next item).

---

## Task 1: Split `grpc/server/server.go` (1610 → 4 files)

**Files:** Modify `grpc/server/server.go`; Create `grpc/server/server_session.go`, `grpc/server/server_browser.go`, `grpc/server/server_hijack_stream.go`.

- [ ] **Step 1: Baseline green.** Run `go build ./grpc/... && go vet ./grpc/server/ && go test ./grpc/server/` and `golangci-lint run ./grpc/server/`. Record the result. If red, STOP (pre-existing breakage is not this task's to fix — report it).
- [ ] **Step 2:** Create the three new files per the File Structure map. Move `CreateSession`/`DestroySession`/`DestroyAllSessions` → `server_session.go`; the browser-control RPCs (Navigate, Click, Type, GetText, Eval, Screenshot, PDF, recording, HAR) → `server_browser.go`; hijack RPCs + `hijackEventToProto` + profile capture + `StreamEvents` + `Interactive` + `executeCommand` + `wireEvents` + `mapKey` → `server_hijack_stream.go`. Leave types + `New` + lifecycle + `getSession` + `sanitizeError` in `server.go`. Each new file: `package server` + the imports it actually uses.
- [ ] **Step 3:** `goimports -w grpc/server/*.go`; remove dead imports.
- [ ] **Step 4: Verify gate** (build + vet + test + lint + gofmt, per the shared procedure). All green.
- [ ] **Step 5: Exported-signature guard:** `go doc ./grpc/server/` identical to baseline.
- [ ] **Step 6: Commit**

```bash
git add grpc/server/
git commit -m "refactor(grpc): split server.go into session/browser/hijack files"
```

---

## Task 2: Split `internal/engine/browser/download.go` (1121 → 4 files)

**Files:** Modify `internal/engine/browser/download.go`; Create `internal/engine/browser/registry.go`, `download_chromium.go`, `download_brave_edge.go`.

- [ ] **Step 1: Baseline green.** `go build ./internal/engine/browser/ && go vet ./internal/engine/browser/ && go test -short ./internal/engine/browser/` + `golangci-lint run ./internal/engine/browser/`. (Browser-download tests may hit the network / be `-short`-gated — use `-short`; record skips.)
- [ ] **Step 2:** Move registry helpers (`RegistryFile`, `LoadRegistry`, `SaveRegistry`, `RegisterBrowser`, `LookupRegistryBrowser`, `ListDownloaded`, `LatestCachedBin`) → `registry.go`; Chromium/Chrome-for-Testing (`DownloadChromium`, `DownloadChrome`, `latestChromiumRevision`, `latestChromeForTesting`, bin/platform helpers) → `download_chromium.go`; Brave/Edge (`DownloadBrave`, `DownloadEdge`, `downloadEdgeWindows/Unix`, MSI/PKG extraction, `latestEdgeRelease`/`latestBraveVersion`) → `download_brave_edge.go`. Keep `CacheDir`/`DownloadFile`/`Resolve`/`ResolveCached`/`copyDir`/`stripFirstDir` + registry-name helpers in `download.go`.
- [ ] **Step 3:** `goimports -w internal/engine/browser/*.go`. NOTE: this package already has `*_windows.go`/`*_unix.go` platform files — do NOT collide names; the new files are platform-neutral. If a moved function is platform-specific, keep its existing platform file, don't move it.
- [ ] **Step 4: Verify gate** (use `go test -short`). All green; cross-compile check: `GOOS=windows go build ./internal/engine/browser/` AND `GOOS=linux go build ./internal/engine/browser/` (Edge/Brave paths are platform-split).
- [ ] **Step 5: Exported-signature guard:** `go doc ./internal/engine/browser/` identical.
- [ ] **Step 6: Commit**

```bash
git add internal/engine/browser/
git commit -m "refactor(browser): split download.go into registry/chromium/brave-edge"
```

---

## Task 3: Split `internal/engine/page_rod.go` (1103 → 4 files)

**Files:** Modify `internal/engine/page_rod.go`; Create `internal/engine/page_rod_nav.go`, `page_rod_capture.go`, `page_rod_wait_eval.go`.

- [ ] **Step 1: Baseline green.** `go build ./internal/engine/ && go vet ./internal/engine/ && go test -short ./internal/engine/` + `golangci-lint run ./internal/engine/`. (Browser-dependent tests skip under `-short` without Chromium — expected.)
- [ ] **Step 2:** Move nav/window/viewport methods → `page_rod_nav.go`; capture (Screenshot, ScrollScreenshot+opts, CaptureDOMSnapshot, PDF, GetResource, favicon, dialogs) → `page_rod_capture.go`; wait/eval/event family (Wait*, EachEvent/WaitEvent, AddScript/StyleTag, ElementFrom*, ObjectToJSON, Call, Event, initEvents) → `page_rod_wait_eval.go`. Keep `rodPage` struct + accessors + cookies/headers/UA + Close + String/Info in `page_rod.go`. **Reminder:** never name a file `*_js.go` (triggers GOOS=js) — these names are safe.
- [ ] **Step 3:** `goimports -w internal/engine/page_rod*.go`.
- [ ] **Step 4: Verify gate** (`go test -short`). All green.
- [ ] **Step 5: Exported-signature guard:** `go doc ./internal/engine/` shows no diff for `rodPage`-related exported symbols.
- [ ] **Step 6: Commit**

```bash
git add internal/engine/page_rod*.go
git commit -m "refactor(engine): split page_rod.go into nav/capture/wait-eval"
```

---

## Task 4: Split `internal/engine/github.go` (1006 → 4 files)

**Files:** Modify `internal/engine/github.go`; Create `internal/engine/github_repo.go`, `github_issues_pr.go`, `github_search.go`.

- [ ] **Step 1: Baseline green.** `go build ./internal/engine/ && go vet ./internal/engine/ && go test -short -run GitHub ./internal/engine/` + `golangci-lint run ./internal/engine/`.
- [ ] **Step 2:** Move `GitHubRepo`/`GitHubReleases`/`GitHubTree`/`GitHubUser` → `github_repo.go`; `GitHubIssues`/`GitHubPRs`/`parseGitHubIssues`/`fetchGitHubBody` → `github_issues_pr.go`; `GitHubSearchCode`/`parseGitHubCodeResults` → `github_search.go`. Keep all types + `GitHubOption`/`With*` + `githubConfig`/defaults in `github.go`.
- [ ] **Step 3:** `goimports -w internal/engine/github*.go`.
- [ ] **Step 4: Verify gate** (`go test -short`). All green.
- [ ] **Step 5: Exported-signature guard:** `go doc ./internal/engine/` GitHub symbols unchanged.
- [ ] **Step 6: Commit**

```bash
git add internal/engine/github*.go
git commit -m "refactor(engine): split github.go into repo/issues-pr/search"
```

---

## Task 5: Consolidate stray logging onto `slog` (STRUCT-02)

**Files:** Modify `cmd/scout/scout.go`; `pkg/scout/aihost/claude/install.go`, `pkg/scout/aihost/codex/install.go`, `pkg/scout/aihost/gemini/install.go`; `internal/engine/browser/download.go` (or its post-split sibling).

Context: `slog` (incl. `internal/logger` which wraps it) is already the standard. The only strays are 2 `log.Printf` calls and ~20 ad-hoc `fmt.Fprintf(os.Stderr, "[install]…")` progress lines. CLI user-facing `fmt.Fprintf(cmd.OutOrStdout()/ErrOrStderr(), …)` is NOT a logger — leave it.

- [ ] **Step 1: Baseline green.** `go build ./...`-equiv for the affected pkgs + `go vet`.
- [ ] **Step 2:** In `cmd/scout/scout.go`, replace the 2 `log.Printf(...)` (gops agent ~:147, tracing ~:175) with `slog.Warn(...)`/`slog.Error(...)` carrying the same message as a structured field; drop the now-unused `"log"` import if it becomes unused.
- [ ] **Step 3:** In each `pkg/scout/aihost/*/install.go`, replace `fmt.Fprintf(os.Stderr, "[install] …", …)` progress lines with `slog.Info("install: …", "key", val)` (keep the same human text as the message; move interpolated args to fields). Same for the 1 `fmt.Fprintf(os.Stderr, …)` progress line in `browser/download.go`. Remove now-unused `os`/`fmt` imports if applicable.
- [ ] **Step 4: Verify gate:** build + vet + `golangci-lint` (0 new) + `gofmt -l` clean for each touched package. Run `go test -short` on the touched packages.
- [ ] **Step 5: Confirm no strays remain:** `grep -rnE 'log\.(Printf|Print|Fatal|Panic)' cmd/ pkg/ internal/ --include='*.go'` returns only `internal/engine/doc.go` godoc comments (not code).
- [ ] **Step 6: Commit**

```bash
git add cmd/scout/scout.go pkg/scout/aihost/ internal/engine/browser/
git commit -m "refactor(log): route stray log.Printf + install progress through slog"
```

---

## Task 6: Browser-detection consolidation (CLEAN-05)

**Files:** Modify `pkg/scout/browser/*.go` (make it delegate); Verify `cmd/scout/browser.go` (sole consumer).

Context: `pkg/scout/browser` (1030 lines) **reimplements** detection/download with zero import of `internal/engine/browser`. Its only load-bearing external use is `cmd/scout/browser.go` calling `browser.DownloadChrome("")`. Goal: make `pkg/scout/browser` a thin delegation layer over `internal/engine/browser`, removing duplicate detect/path/download logic, while keeping its exported surface (`Detect`, `DetectByType`, `Best`, `ParseVersion`, `DownloadChrome/Brave/Edge`, `Manager`, `BrowserInfo`, errors) source-compatible.

- [ ] **Step 1: Baseline green.** `go build ./pkg/scout/browser/ ./cmd/scout/ && go vet ./pkg/scout/browser/ && go test -short ./pkg/scout/browser/` + `golangci-lint run ./pkg/scout/browser/`. Record which exported funcs have tests.
- [ ] **Step 2: Confirm the consumer contract.** Read `cmd/scout/browser.go`; confirm `DownloadChrome("")` is the only load-bearing call (the mapper found this). List every other importer of `pkg/scout/browser` in non-test code: `grep -rn '"github.com/inovacc/scout/pkg/scout/browser"' --include='*.go' | grep -v _test`. If any importer uses `Detect`/`Best`/etc., keep those delegating (don't drop the symbols).
- [ ] **Step 3: Map engine equivalents.** In `internal/engine/browser`: `DetectBrowsers()`, `BestDetected()`, `ParseBrowserVersion()`, `DownloadChrome(...)`. Rewrite each `pkg/scout/browser` exported func to call the engine equivalent and map `engine/browser.DetectedBrowser` ↔ `browser.BrowserInfo` (a small field-copy adapter). Delete the now-dead reimplemented detect/path bodies (`detect.go`, `detect_unix.go`, `detect_windows.go` private logic) — but KEEP the exported wrappers + `BrowserInfo`/`ManagerOption`/error types + the `Manager` façade (its internals now call the delegating funcs).
- [ ] **Step 4: Verify gate:** build `./pkg/scout/browser/ ./cmd/scout/` + vet + `go test -short ./pkg/scout/browser/ ./cmd/scout/` + `golangci-lint` (0 new) + cross-compile `GOOS=windows` and `GOOS=linux` (detection is platform-split). All green.
- [ ] **Step 5: Exported-signature guard:** `go doc ./pkg/scout/browser/` shows the SAME exported symbols (delegation must not change the public API). Behavior parity: the existing `pkg/scout/browser` tests still pass.
- [ ] **Step 6: Commit**

```bash
git add pkg/scout/browser/ cmd/scout/browser.go
git commit -m "refactor(browser): pkg/scout/browser delegates to internal/engine/browser (CLEAN-05)"
```

---

## Task 7: `scout:` error prefix on highest-impact bare paths (STRUCT-03)

**Files:** Modify `cmd/scout/update.go`; `pkg/scout/browser/download.go`; `pkg/scout/runbook/runbook.go` + `runbook/fix.go`; `pkg/scout/identity/identity.go`. (The 292 `scraper/modes` calls already carry a `mode:` subsystem prefix → **explicitly deferred** to ERR-01/v2; note it, don't touch.)

- [ ] **Step 1: Baseline green.** `go build ./...`-equiv for touched pkgs + `go vet` + `go test -short` on each.
- [ ] **Step 2: `cmd/scout/update.go` (truly bare — highest priority).** Prefix each bare `fmt.Errorf("…")` with `scout: update: ` (e.g. `:119 "build request: %w"` → `"scout: update: build request: %w"`, `:126 "fetch release: %w"` → `"scout: update: fetch release: %w"`, `:131 "github api returned %s"` → `"scout: update: github api returned %s"`, `:136 "decode release json: %w"` → `"scout: update: decode release json: %w"`). Cover all ~15.
- [ ] **Step 3: Prefix-only fixes (mechanical `"<sub>:` → `"scout: <sub>:`):** `pkg/scout/browser/download.go` (`browser:` → `scout: browser:`, ~22), `pkg/scout/runbook/{runbook,fix}.go` (`runbook:` → `scout: runbook:`, ~27), `pkg/scout/identity/identity.go` (`identity:` → `scout: identity:`, ~22). Use a careful search-replace and review each hit (don't double-prefix any that already have `scout:`).
- [ ] **Step 4: Verify gate:** build + vet + `go test -short` on each touched package (assertions that match error *substrings* may need updating — if a test asserts `"browser:"` it still substring-matches `"scout: browser:"`, but a test anchored with `strings.HasPrefix(err, "browser:")` would break; fix any such test to expect the new prefix). `golangci-lint` 0 new + `gofmt -l` clean.
- [ ] **Step 5:** Add a one-line note to `docs/BACKLOG.md` under a `ERR-01` tag: the remaining ~750 non-`scout:` errors (notably `scraper/modes` ×292) are deferred to v2.
- [ ] **Step 6: Commit**

```bash
git add cmd/scout/update.go pkg/scout/browser/download.go pkg/scout/runbook/ pkg/scout/identity/ docs/BACKLOG.md
git commit -m "refactor(errors): scout: prefix on highest-impact paths (update/browser/runbook/identity)"
```

---

## Final verification (after all tasks)

- [ ] No non-generated, non-test, non-`lib/` source file exceeds 1000 lines: `find . -name '*.go' -not -name '*_test.go' -not -path '*/lib/*' -not -path '*/.claude/*' -not -name '*.pb.go' | while read f; do n=$(wc -l <"$f"); [ "$n" -gt 1000 ] && echo "$n $f"; done` → empty.
- [ ] `task test` (or `task test:unit` without Chromium + `task test:full` where available) — green.
- [ ] `task check` (vet + lint) — green; `golangci-lint run ./...` introduces no new issues vs. the pre-Phase-5 baseline.
- [ ] `go doc` of every refactored package is unchanged from baseline (no exported-API drift).
- [ ] `grep` confirms no stray `log.Printf`/`fmt.Fprintf(os.Stderr` progress logging remains outside `internal/logger` bootstrap + CLI UX paths.
- [ ] Use `superpowers:finishing-a-development-branch`.

## Spec coverage check (self-review)

| Spec requirement (2026-05-16-05) | Task |
|---|---|
| STRUCT-01 split files >1000 lines | 1, 2, 3, 4 (must.go already 179 lines — no longer a candidate) |
| STRUCT-02 consolidate logging to slog | 5 |
| STRUCT-03 error-prefix convention (highest-impact only; full = v2) | 7 |
| CLEAN-05 browser detection consolidation | 6 |
| All existing tests pass; no API drift | every task's verify gate + final verification |

**Deviations from the spec's stale figures (flagged):** the spec/ROADMAP listed `must.go` at ~1267 lines as the primary split candidate; it is now **179 lines** (already cleaned) and is dropped. The real >1000-line set is `grpc/server/server.go` (1610), `internal/engine/browser/download.go` (1121), `internal/engine/page_rod.go` (1103), `internal/engine/github.go` (1006). `internal/engine/lib/proto/*` files exceed 1000 lines but are internalized-rod generated code and are exempt per project convention.
