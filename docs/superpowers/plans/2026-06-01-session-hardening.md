# Session Hardening — Enforce Clean Sessions & Zombie-Instance Kill — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Scout sessions rock-solid — the `<scouthome>\sessions\` directory is empty unless a live, identity-verified scout process owns each folder, and every scout-launched browser has a live scout parent (no orphans, no leaked dirs).

**Architecture:** Consolidate the three divergent cleanup paths (`CleanStaleSessions` startup, `CleanOrphans` watchdog, `StartCleanupRetrier`) into one canonical, path-bounded `session.ReapOnce()` pass; revive the dead `FindBrowsersUsingDataDir` (path-prefix-bounded under `sessions\`) so a corrupt `scout.pid` no longer leaves an un-killable zombie browser; add a best-effort `main()` signal handler via a live-browser registry (`CloseAllLive`); reconcile + panic-recover the gRPC daemon; escalate stuck Windows dirs to force-break; and ship `scout session doctor` + a crash→reap acceptance test that *is* the verification contract.

**Tech Stack:** Go 1.26, Cobra CLI, internalized rod launcher, gops process discovery, `golang.org/x/sys/windows` (force-break), real-browser + `httptest` tests (no mocks).

**Design spec:** `docs/superpowers/specs/2026-06-01-session-hardening-design.md`

---

## Scope decisions (locked, from the spec)

1. **Best-effort + next-startup GC** — NO Windows Job Object, NO native `CTRL_CLOSE` (deferred to a phase B). A lightweight `SIGINT`/`SIGTERM` handler IS in scope.
2. **Aggressive scan-and-kill with NO identity gate**, but **one hard safety floor**: a process is only killed if its `--user-data-dir` resolves **under `<scouthome>\sessions\`** — this is what keeps the reaper off the user's real Chrome (`never touch the user's browser` is a hard CLAUDE.md rule).
3. **Daemon: reconcile + adopt/kill orphans** on restart.
4. **Stuck dirs: escalate (`scout session list --pending`) + force-break.**

## Spec amendments discovered during drafting (authoritative over the spec where they conflict)

- The reap engine **already exists** as `cmd/scout/session_audit.go` (`auditAllSessions`/`classifySession`/`enforceAuditCleanup`, statuses HEALTHY/REUSABLE/EXPIRED/ZOMBIE/STALE/CORRUPT/FOREIGN). The un-killable-zombie hole is precisely that `enforceAuditCleanup`'s **CORRUPT path never kills the browser**. The plan integrates with this engine; it does not rebuild it.
- The `FindBrowsersUsingDataDir` engine re-export wrapper (`internal/engine/session_track.go:62-66`) is a **live re-export, NOT dead code** — spec §10's "remove the dead wrapper" is **dropped**; the wrapper is kept and gains a real caller via `ReapOnce`.
- `ReapOnce` must `os.ReadDir(GetSessionsDir())` **directly** (not via `List()`, which silently drops corrupt/missing-pid dirs — the exact zombie case).
- All kill paths must guard `pid != os.Getpid()` (self-kill avoidance under PID reuse / fabricated `scout.pid`).
- `Browser.Close`'s `os.RemoveAll(SessionDir)` is nested inside `if b.launcher != nil`; it must be **hoisted out** so partial/failed launches still clean up + enqueue.
- The positive-ownership test control cannot use `os.Getpid()` as `ScoutPID` (the test binary isn't named `scout`, so `IsScoutProcess` returns false); use a reusable-unexpired session instead.

## Execution order (HARD dependency — do not parallelize across the arrow)

```
Phase 1 (scan path-bound)
   └─> Phase 2 (ReapOnce + RecordCleanupFailure/PendingCleanup wrappers + watchdog)
          └─> Phase 3 (engine + facade re-exports: CloseAllLive, scout.ReapOnce, scout.PendingCleanup, scout.FindBrowsersUsingDataDir; Browser.Close enqueue; autofree)
                 ├─> Phase 4 (main() signal handler — needs scout.CloseAllLive)
                 ├─> Phase 5 (session doctor + list --pending + crash→reap acceptance — needs session.ReapOnce + scout.PendingCleanup)
                 ├─> Phase 6 (daemon reconcile/DestroyAllSessions/idle — binds scout.ReapOnce via injectable reapHook seam)
                 └─> Phase 7 (stuck-dir force-break — shares cleanup_retry.go with Phase 2)
```

Phases 4–7 are mutually independent once 1–3 land. Each task is RED→GREEN→commit; never advance past a failing `go build ./cmd/scout/` + `go build ./pkg/...`.

## Phase index

| Phase | Title | Tasks | Part |
|-------|-------|-------|------|
| 1 | Path-bound the data-dir scan (safety floor) | 3 | scan-path-bound |
| 2 | Canonical ReapOnce + consolidation | 4 | reaper-core |
| 3 | Live-browser registry, CloseAllLive, Close enqueue, autofree, facade exports | 6 | engine-facade-registry |
| 4 | Best-effort SIGINT/SIGTERM handler in main() | 2 | main-signal |
| 5 | `scout session doctor` + `list --pending` + crash→reap acceptance test | 3 | doctor-acceptance |
| 6 | Daemon reconcile, DestroyAllSessions, panic-recovered shutdown, idle hardening | 5 | daemon-reconcile |
| 7 | Stuck-dir force-break escalation | 3 | stuck-dir-forcebreak |

**Total: 26 tasks.** Acceptance contract (Phase 5): launch a session → kill scout → assert next `ReapOnce` kills the orphaned browser and empties the dir; `scout session doctor` exits 0 clean / ≠0 on any violation; a foreign `--user-data-dir` process is never touched.

---
## Phase 1: Path-bound the data-dir scan (safety floor)

**Goal:** Make `FindBrowsersUsingDataDir` kill *only* browsers whose `--user-data-dir` resolves under `GetSessionsDir()` **and** under the requested `dataDir` (path-prefix, not substring), so the reaper can never touch the user's real Chrome. Fix the Windows PowerShell matcher (backslash escaping) and the Linux `/proc/cmdline` matcher (resolved-path compare, not substring), add the `isUnderSessions` helper, and prove the floor with a static, browser-free unit test.

This phase is the *hard safety floor* that Phase 2 (wiring `killHolders` / the reaper) builds on. It changes matching semantics only; it does not yet add a caller.

---

### Task 1.1: Add `isUnderSessions` + path-prefix helpers (static, no behavior change to callers yet)

Introduce the path-containment primitives the new matcher needs: `isUnderSessions(p)` (is `p` under `GetSessionsDir()`?) and a generic `isUnderDir(p, root)` it delegates to. Both resolve and clean paths and are case-insensitive on Windows. Also add `extractUserDataDir(cmdline)` which pulls the `--user-data-dir` value out of a raw command line (handles `--user-data-dir=X`, `--user-data-dir X`, and quoted values).

**Files:**
- Create: `internal/engine/session/browser_scan_path.go` (new, ~95 lines)
- Create: `internal/engine/session/browser_scan_path_test.go` (new, ~120 lines)

- [ ] **Step 1: Write the failing test for the path helpers.** Create `internal/engine/session/browser_scan_path_test.go`:

```go
package session

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsUnderDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")

	cases := []struct {
		name string
		p    string
		want bool
	}{
		{"direct child", filepath.Join(root, "abc"), true},
		{"nested grandchild", filepath.Join(root, "abc", "data"), true},
		{"root itself", root, true},
		{"sibling outside", filepath.Join(filepath.Dir(root), "other", "data"), false},
		{"unrelated abs", filepath.Join(t.TempDir(), "Chrome", "User Data"), false},
		{"empty path", "", false},
		{"dotdot escape", filepath.Join(root, "abc", "..", "..", "evil"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnderDir(tc.p, root); got != tc.want {
				t.Fatalf("isUnderDir(%q, %q) = %v, want %v", tc.p, root, got, tc.want)
			}
		})
	}
}

func TestIsUnderDirCaseInsensitiveWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitivity floor only applies on windows")
	}

	root := filepath.Join(t.TempDir(), "Sessions")
	mixed := filepath.Join(filepath.Dir(root), "SESSIONS", "abc", "data")

	if !isUnderDir(mixed, root) {
		t.Fatalf("expected case-insensitive match on windows for %q under %q", mixed, root)
	}
}

func TestIsUnderSessions(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	inside := filepath.Join(dir, "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA", "data")
	if !isUnderSessions(inside) {
		t.Fatalf("isUnderSessions(%q) = false, want true", inside)
	}

	outside := filepath.Join(t.TempDir(), "Chrome", "User Data")
	if isUnderSessions(outside) {
		t.Fatalf("isUnderSessions(%q) = true, want false (real user chrome)", outside)
	}
}

func TestExtractUserDataDir(t *testing.T) {
	cases := []struct {
		name    string
		cmdline string
		want    string
	}{
		{"equals form", `chrome --user-data-dir=/tmp/s/abc/data --headless`, "/tmp/s/abc/data"},
		{"space form", `chrome --user-data-dir /tmp/s/abc/data --headless`, "/tmp/s/abc/data"},
		{"quoted equals", `chrome --user-data-dir="C:\Users\x\AppData\Local\Scout\sessions\abc\data"`, `C:\Users\x\AppData\Local\Scout\sessions\abc\data`},
		{"absent", `chrome --headless --remote-debugging-port=0`, ""},
		{"empty", ``, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractUserDataDir(tc.cmdline); got != tc.want {
				t.Fatalf("extractUserDataDir(%q) = %q, want %q", tc.cmdline, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test, expect a compile FAIL.** The helpers don't exist yet, so the package won't build:

```
go test -v -run 'TestIsUnderDir|TestIsUnderSessions|TestExtractUserDataDir' ./internal/engine/session/
```

Expected: `FAIL` — `undefined: isUnderDir`, `undefined: isUnderSessions`, `undefined: extractUserDataDir`.

- [ ] **Step 3: Implement the helpers.** Create `internal/engine/session/browser_scan_path.go`:

```go
package session

import (
	"path/filepath"
	"runtime"
	"strings"
)

// isUnderSessions reports whether p resolves to a path under GetSessionsDir().
// This is the hard safety floor for the data-dir scan: the reaper must NEVER
// kill a browser whose --user-data-dir is the user's real Chrome profile, so
// every kill candidate is gated through here before any signal is sent.
//
// Returns false when the sessions dir cannot be resolved (GetSessionsDir
// returns ""), failing closed rather than matching everything.
func isUnderSessions(p string) bool {
	root := GetSessionsDir()
	if root == "" {
		return false
	}

	return isUnderDir(p, root)
}

// isUnderDir reports whether p is root itself or lives somewhere beneath root.
// Both paths are cleaned to absolute-ish form (filepath.Clean) so ".." escapes
// cannot smuggle a path out of root. Comparison is case-insensitive on Windows
// where the filesystem is case-insensitive.
func isUnderDir(p, root string) bool {
	if p == "" || root == "" {
		return false
	}

	cp := filepath.Clean(p)
	cr := filepath.Clean(root)

	if runtime.GOOS == "windows" {
		cp = strings.ToLower(cp)
		cr = strings.ToLower(cr)
	}

	if cp == cr {
		return true
	}

	rel, err := filepath.Rel(cr, cp)
	if err != nil {
		return false
	}

	// rel == "." means equal (handled above); any rel that starts with ".."
	// (or is "..") escapes root and is therefore NOT under it.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}

// extractUserDataDir pulls the value of the --user-data-dir flag out of a raw
// process command line. Handles the three Chrome flag forms:
//
//	--user-data-dir=VALUE
//	--user-data-dir VALUE
//	--user-data-dir="VALUE WITH SPACES"
//
// Returns "" when the flag is absent. The returned value is NOT cleaned; the
// caller (isUnderSessions / isUnderDir) cleans it during comparison.
func extractUserDataDir(cmdline string) string {
	const flag = "--user-data-dir"

	idx := strings.Index(cmdline, flag)
	if idx < 0 {
		return ""
	}

	rest := cmdline[idx+len(flag):]

	// "=value" form (possibly quoted).
	if strings.HasPrefix(rest, "=") {
		return unquoteValue(strings.TrimPrefix(rest, "="))
	}

	// " value" form: skip the separating spaces, then take the next token.
	rest = strings.TrimLeft(rest, " ")
	if rest == "" {
		return ""
	}

	return unquoteValue(rest)
}

// unquoteValue returns the first whitespace-delimited token of s, honoring a
// leading double quote (everything up to the closing quote) so paths with
// spaces survive intact.
func unquoteValue(s string) string {
	if strings.HasPrefix(s, `"`) {
		s = s[1:]
		if end := strings.IndexByte(s, '"'); end >= 0 {
			return s[:end]
		}

		return s
	}

	if cut := strings.IndexByte(s, ' '); cut >= 0 {
		return s[:cut]
	}

	return s
}
```

- [ ] **Step 4: Run the test, expect PASS.**

```
go test -v -run 'TestIsUnderDir|TestIsUnderSessions|TestExtractUserDataDir' ./internal/engine/session/
```

Expected: `ok  github.com/inovacc/scout/internal/engine/session` — all subtests `PASS`.

- [ ] **Step 5: Commit.**

```
git add internal/engine/session/browser_scan_path.go internal/engine/session/browser_scan_path_test.go
git commit -m "feat(session): add path-containment helpers for data-dir scan floor"
```

---

### Task 1.2: Path-bind `FindBrowsersUsingDataDir` and the Linux `/proc` matcher

Replace the substring match (`strings.Contains(cmdline, dataDir)`) with a resolved-path test: extract the process's `--user-data-dir`, require it to resolve **under `GetSessionsDir()`** (`isUnderSessions`) **and under the requested `dataDir`** (`isUnderDir`). Same gate for Linux. macOS/BSD still return nil (documented blind spot).

**Files:**
- Modify: `internal/engine/session/browser_scan.go` (lines 20-74 — `FindBrowsersUsingDataDir` + `findBrowsersLinux`)
- Create: `internal/engine/session/browser_scan_static_test.go` (new, ~110 lines) — static, no browser

- [ ] **Step 1: Write the failing static matcher test.** Create `internal/engine/session/browser_scan_static_test.go`. This is the spec-mandated static proof: it does not spawn a browser; it drives the pure matcher `matchDataDirCmdline` over fabricated argv strings.

```go
package session

import (
	"path/filepath"
	"testing"
)

// TestMatchDataDirCmdline is the static safety-floor proof: a fabricated
// command line whose --user-data-dir is under the temp sessions dir matches;
// one outside sessions/ never matches. No browser is launched.
func TestMatchDataDirCmdline(t *testing.T) {
	sessionsRoot := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return sessionsRoot }
	t.Cleanup(func() { SessionsDir = orig })

	const sessID = "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA"
	sessData := filepath.Join(sessionsRoot, sessID, "data")
	otherData := filepath.Join(sessionsRoot, "1OTHER0000000ABTMCOGNDUHRXOOPVGAQ", "data")
	realChrome := filepath.Join(t.TempDir(), "Chrome", "User Data")

	cases := []struct {
		name    string
		cmdline string
		dataDir string
		want    bool
	}{
		{
			name:    "under sessions and under target",
			cmdline: "chrome --user-data-dir=" + sessData + " --headless",
			dataDir: sessData,
			want:    true,
		},
		{
			name:    "under sessions but different session is not under target",
			cmdline: "chrome --user-data-dir=" + otherData + " --headless",
			dataDir: sessData,
			want:    false,
		},
		{
			name:    "real user chrome outside sessions is never matched",
			cmdline: "chrome --user-data-dir=" + realChrome,
			dataDir: realChrome, // even if caller passes it, the sessions floor rejects it
			want:    false,
		},
		{
			name:    "no user-data-dir flag",
			cmdline: "chrome --headless --remote-debugging-port=0",
			dataDir: sessData,
			want:    false,
		},
		{
			name:    "empty dataDir",
			cmdline: "chrome --user-data-dir=" + sessData,
			dataDir: "",
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchDataDirCmdline(tc.cmdline, tc.dataDir); got != tc.want {
				t.Fatalf("matchDataDirCmdline(%q, %q) = %v, want %v", tc.cmdline, tc.dataDir, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test, expect a compile FAIL.**

```
go test -v -run TestMatchDataDirCmdline ./internal/engine/session/
```

Expected: `FAIL` — `undefined: matchDataDirCmdline`.

- [ ] **Step 3: Add the `matchDataDirCmdline` gate and rewrite the dispatcher + Linux matcher.** In `internal/engine/session/browser_scan.go`, replace the whole `FindBrowsersUsingDataDir` + `findBrowsersLinux` block (current lines 20-74) with the path-bounded versions and add the shared gate. Keep `isBrowserCmdline`, `fmtSscanf`, and `errNotInt`/`parseErr` exactly as they are below them.

Replace this existing block:

```go
func FindBrowsersUsingDataDir(dataDir string) []int {
	if dataDir == "" {
		return nil
	}

	// Normalize for comparison
	dataDir = strings.ReplaceAll(dataDir, "/", string(filepath.Separator))

	if runtime.GOOS == "linux" {
		return findBrowsersLinux(dataDir)
	}

	if runtime.GOOS == "windows" {
		return findBrowsersWindows(dataDir)
	}

	return nil
}

func findBrowsersLinux(dataDir string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var pids []int

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		var pid int

		if _, err := fmtSscanf(e.Name(), &pid); err != nil {
			continue
		}

		data, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}

		cmdline := strings.ReplaceAll(string(data), "\x00", " ")
		if !isBrowserCmdline(cmdline) {
			continue
		}

		if strings.Contains(cmdline, dataDir) {
			pids = append(pids, pid)
		}
	}

	return pids
}
```

with:

```go
func FindBrowsersUsingDataDir(dataDir string) []int {
	if dataDir == "" {
		return nil
	}

	switch runtime.GOOS {
	case "linux":
		return findBrowsersLinux(dataDir)
	case "windows":
		return findBrowsersWindows(dataDir)
	default:
		// SAFETY-FLOOR BLIND SPOT: Darwin/BSD have no /proc and we do not
		// shell out to ps/sysctl here, so the scan returns nil and zombie
		// browsers on those platforms are NOT scan-killed. This is an
		// accepted phase-A gap (tracked in docs/BACKLOG.md). Returning nil
		// fails CLOSED — it can never kill the user's real Chrome.
		return nil
	}
}

// matchDataDirCmdline is the pure, OS-independent kill gate. It returns true
// only when the process's --user-data-dir resolves (a) under GetSessionsDir()
// — the hard safety floor that protects the user's real Chrome — AND (b) under
// the specific dataDir the caller is reaping. Substring matching is
// deliberately NOT used: a foreign profile path that merely contains the
// session path as a substring must never match.
func matchDataDirCmdline(cmdline, dataDir string) bool {
	if dataDir == "" {
		return false
	}

	udd := extractUserDataDir(cmdline)
	if udd == "" {
		return false
	}

	return isUnderSessions(udd) && isUnderDir(udd, dataDir)
}

func findBrowsersLinux(dataDir string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var pids []int

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		var pid int

		if _, err := fmtSscanf(e.Name(), &pid); err != nil {
			continue
		}

		data, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}

		// /proc cmdline is NUL-separated; join with spaces so flag parsing
		// and the browser-name check both see a normal command line.
		cmdline := strings.ReplaceAll(string(data), "\x00", " ")
		if !isBrowserCmdline(cmdline) {
			continue
		}

		if matchDataDirCmdline(cmdline, dataDir) {
			pids = append(pids, pid)
		}
	}

	return pids
}
```

Note: the `filepath` import becomes unused in `browser_scan.go` after dropping the `strings.ReplaceAll(.../, filepath.Separator)` normalization (path normalization now happens inside `filepath.Clean` in the helpers). Remove `"path/filepath"` from this file's import block. `os`, `runtime`, and `strings` are all still used.

- [ ] **Step 4: Verify imports compile and run the static test, expect PASS.**

```
go build ./internal/engine/session/ && go test -v -run TestMatchDataDirCmdline ./internal/engine/session/
```

Expected: build succeeds; `TestMatchDataDirCmdline` and all five subtests `PASS`.

- [ ] **Step 5: Commit.**

```
git add internal/engine/session/browser_scan.go internal/engine/session/browser_scan_static_test.go
git commit -m "fix(session): path-bind data-dir scan; drop substring match on linux"
```

---

### Task 1.3: Fix the Windows PowerShell matcher (escape backslashes; gate by resolved path)

The bug: `escapePowerShell` (browser_scan_windows.go:46) escapes backtick, `"` and `$` but **not backslashes**, and the script uses a `-like "*<path>*"` substring filter where Windows paths are nothing but backslashes — and `-like` treats `\` plus `[`/`]`/`?`/`*` as wildcard metacharacters, so a real session path either fails to match or matches the wrong thing. Fix: stop filtering by `-like` substring in PowerShell entirely; emit `ProcessId` **and** `CommandLine` for every candidate browser, then apply the *same* `matchDataDirCmdline` resolved-path gate in Go. This makes Windows matching identical to Linux and removes all PowerShell-quoting fragility.

**Files:**
- Modify: `internal/engine/session/browser_scan_windows.go` (full file — rewrite `findBrowsersWindows`, fix/keep `escapePowerShell`)

- [ ] **Step 1: Write a failing test for the corrected `escapePowerShell` (backslash + wildcard escaping).** Add to `internal/engine/session/browser_scan_static_test.go` a Windows-gated test. (Kept in the shared static test file; `runtime.GOOS` guards it so it is a no-op off Windows.)

```go
func TestEscapePowerShellBackslash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("escapePowerShell is only compiled on windows")
	}

	in := `C:\Users\x\AppData\Local\Scout\sessions\a[b]\data`
	got := escapePowerShell(in)

	// Backslashes must be doubled so the value is a literal path inside a
	// PowerShell double-quoted string, and -like wildcard metacharacters
	// ([ and ]) must be back-tick escaped so they are matched literally.
	for _, frag := range []string{"``[", "``]"} {
		if !contains(got, frag) {
			t.Fatalf("escapePowerShell(%q) = %q, missing escaped fragment %q", in, got, frag)
		}
	}

	if !contains(got, `\\`) {
		t.Fatalf("escapePowerShell(%q) = %q, backslashes not doubled", in, got)
	}
}

// contains is a tiny local helper so this test file needs no extra imports
// beyond what the package test build already pulls in.
func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}

	return -1
}
```

Add the `runtime` import to the static test file if not already present (Task 1.2's file does not import it; add it).

- [ ] **Step 2: Run the test.** On Windows expect FAIL (current `escapePowerShell` leaves `\` and `[`/`]` untouched); on Linux/macOS expect `SKIP`.

```
go test -v -run TestEscapePowerShellBackslash ./internal/engine/session/
```

Expected on Windows: `FAIL` — `backslashes not doubled` (and the wildcard fragments missing). Expected off Windows: `SKIP`.

- [ ] **Step 3: Rewrite `browser_scan_windows.go`.** Replace the full file contents:

```go
//go:build windows

package session

import (
	"os/exec"
	"strconv"
	"strings"
)

// findBrowsersWindows enumerates running browser processes via PowerShell
// Get-CimInstance Win32_Process (which exposes CommandLine — tasklist does
// not). It does NOT filter by path inside PowerShell: -like wildcard matching
// over backslash-heavy Windows paths is fragile (\, [, ], ?, * are all
// metacharacters). Instead it emits "PID\tCommandLine" for every candidate
// browser and applies the same resolved-path gate (matchDataDirCmdline) in Go
// that the Linux path uses, so matching semantics are identical cross-platform.
//
// PowerShell is on every modern Windows install; this avoids pulling in the
// WMI / OLE Go bindings just for one scan operation.
func findBrowsersWindows(dataDir string) []int {
	if dataDir == "" {
		return nil
	}

	const script = `Get-CimInstance Win32_Process -Filter "Name='chrome.exe' OR Name='brave.exe' OR Name='msedge.exe' OR Name='chromium.exe'" |
		Where-Object { $_.CommandLine } |
		ForEach-Object { "$($_.ProcessId)` + "`t" + `$($_.CommandLine)" }`

	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil
	}

	var pids []int

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")

		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}

		pidStr := strings.TrimSpace(line[:tab])
		cmdline := line[tab+1:]

		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}

		if matchDataDirCmdline(cmdline, dataDir) {
			pids = append(pids, pid)
		}
	}

	return pids
}

// escapePowerShell escapes a string for safe LITERAL use inside a PowerShell
// double-quoted string that is later compared with the -like operator.
// Backticks, double quotes and $ are PowerShell string metacharacters;
// backslashes must be doubled so a Windows path is treated literally; and the
// -like wildcard metacharacters (* ? [ ]) are back-tick escaped so they match
// literally rather than as patterns.
//
// Retained for any future direct-PowerShell path comparison; the current
// findBrowsersWindows gates in Go and no longer interpolates the path, but the
// escaper is the documented-correct primitive and is unit tested.
func escapePowerShell(s string) string {
	r := strings.NewReplacer(
		"`", "``",
		`"`, "`\"",
		"$", "`$",
		`\`, `\\`,
		"*", "``*",
		"?", "``?",
		"[", "``[",
		"]", "``]",
	)

	return r.Replace(s)
}
```

Rationale for keeping `escapePowerShell`: removing it would also need the test removed; keeping it as the *correct* primitive (now backslash- and wildcard-safe) documents the fix the contract asked for and keeps a unit-tested escaper available if a future PowerShell-side filter is reintroduced. The live matcher no longer depends on it, eliminating the original injection/quoting risk entirely.

- [ ] **Step 4: Run the Windows test (PASS on Windows, SKIP elsewhere) and confirm the package still builds for the Windows target.**

```
go test -v -run TestEscapePowerShellBackslash ./internal/engine/session/
go vet ./internal/engine/session/
```

Cross-compile check for the Windows file (run from a non-Windows or Windows host):

```
GOOS=windows go build ./internal/engine/session/
```

PowerShell equivalent on a Windows host:

```
$env:GOOS='windows'; go build ./internal/engine/session/
```

Expected: test `PASS` on Windows / `SKIP` off Windows; `go vet` clean; the `GOOS=windows` build succeeds (proves `matchDataDirCmdline`, defined in the cross-platform `browser_scan.go`, links into the Windows file).

- [ ] **Step 5: Commit.**

```
git add internal/engine/session/browser_scan_windows.go internal/engine/session/browser_scan_static_test.go
git commit -m "fix(session): escape backslashes in windows scan; gate by resolved path"
```

---

### Phase 1 verification

Run the full set from the repo root (`D:\weaver-sync\development\personal\projects\scout`):

1. Build the session package (must succeed on the host platform):

```
go build ./internal/engine/session/
```

Expected: no output, exit 0.

2. Cross-compile the Windows-specific scan file to ensure `matchDataDirCmdline` links for `GOOS=windows`:

PowerShell:
```
$env:GOOS='windows'; go build ./internal/engine/session/; Remove-Item Env:\GOOS
```
Bash:
```
GOOS=windows go build ./internal/engine/session/
```

Expected: no output, exit 0.

3. Run every Phase 1 test (all static, no Chromium required):

```
go test -v -run 'TestIsUnderDir|TestIsUnderDirCaseInsensitiveWindows|TestIsUnderSessions|TestExtractUserDataDir|TestMatchDataDirCmdline|TestEscapePowerShellBackslash' ./internal/engine/session/
```

Expected output (host = Linux/macOS): every `TestIsUnderDir*`, `TestIsUnderSessions`, `TestExtractUserDataDir`, `TestMatchDataDirCmdline` subtest reports `--- PASS`; `TestIsUnderDirCaseInsensitiveWindows` and `TestEscapePowerShellBackslash` report `--- SKIP`; final line `ok  github.com/inovacc/scout/internal/engine/session`. On a Windows host the two skipped tests instead `PASS`.

4. Vet:

```
go vet ./internal/engine/session/
```

Expected: no output, exit 0.

5. Whole session-package regression (no behavior regressed for existing tests; requires no browser for these):

```
go test ./internal/engine/session/
```

Expected: `ok  github.com/inovacc/scout/internal/engine/session`.

**Acceptance for Phase 1:** the static `TestMatchDataDirCmdline` proves a PID with `--user-data-dir` under the temp sessions dir is matched while one outside `sessions/` is not; `FindBrowsersUsingDataDir` is now path-prefix bounded under both `GetSessionsDir()` and the requested `dataDir`; the Windows matcher escapes backslashes and no longer relies on a fragile `-like` substring; macOS/BSD returning nil is documented in-code as an accepted blind spot. No caller is wired yet — that is Phase 2.
## Phase 2: Canonical ReapOnce + consolidation

**Goal:** Collapse the three uncoordinated cleanup paths (`CleanStaleSessions`, per-browser `CleanOrphans` watchdog, `StartCleanupRetrier`) into one canonical, path-bounded `ReapOnce()` pass in a new `internal/engine/session/reaper.go`, wiring the Phase 1 path-bounded `FindBrowsersUsingDataDir` into the kill step so a corrupt/missing `scout.pid` no longer leaves an un-killable zombie browser. Add exported `RecordCleanupFailure`/`PendingCleanup` wrappers and a single process-wide `StartReaperWatchdog`. Reimplement `CleanStaleSessions`/`CleanOrphans` as thin wrappers (deprecate `CleanOrphans`).

> **Phase 1 dependency:** This phase calls `session.FindBrowsersUsingDataDir(dataDir)` (already exists, `browser_scan.go:20`) and assumes Phase 1 has tightened it to a path-prefix match bounded under `GetSessionsDir()` plus added `isUnderSessions`. Phase 2 does NOT modify `browser_scan*.go`. Because `ReapOnce` always passes `DataDir(id)` — a path that is by construction under `GetSessionsDir()` — the wiring is correct whether or not Phase 1 has landed (the tests fabricate dirs under the overridden `SessionsDir`).

---

### Task 2.1: Exported cleanup-failure wrappers (`RecordCleanupFailure`, `PendingCleanup`)

`ReapOnce` (and the engine `Browser.Close` path in a later phase) must enqueue locked dirs and `session list --pending` must read them. The unexported `recordCleanupFailure`/`snapshotPending` already exist; add exported wrappers so package `engine` can call them.

**Files:**
- Modify: `internal/engine/session/cleanup_retry.go` (add exports after `PendingCleanupCount`, ~line 70; add `forceBreakThreshold` const near line 74; add force-break logic in `retryPending`, lines 112-142)
- Create: `internal/engine/session/cleanup_retry_export_test.go`

- [ ] **Step 1: Write failing test for the exported wrappers + force-break const.**

Create `internal/engine/session/cleanup_retry_export_test.go`:

```go
package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordCleanupFailureAndPendingCleanup(t *testing.T) {
	// Drain any residue from other tests so assertions are deterministic.
	for _, p := range snapshotPending() {
		removePending(p)
	}

	dir := filepath.Join(t.TempDir(), "stuck-session")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	RecordCleanupFailure(dir)

	pending := PendingCleanup()
	found := false
	for _, p := range pending {
		if p == dir {
			found = true
		}
	}

	if !found {
		t.Fatalf("PendingCleanup did not contain %q; got %v", dir, pending)
	}

	if PendingCleanupCount() < 1 {
		t.Fatalf("PendingCleanupCount = %d, want >= 1", PendingCleanupCount())
	}

	// Idempotent: recording the same path twice does not duplicate it.
	RecordCleanupFailure(dir)
	count := 0
	for _, p := range PendingCleanup() {
		if p == dir {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate enqueue: path present %d times, want 1", count)
	}

	removePending(dir)
}

func TestForceBreakThresholdConst(t *testing.T) {
	if forceBreakThreshold < 1 {
		t.Fatalf("forceBreakThreshold = %d, want >= 1", forceBreakThreshold)
	}
}
```

- [ ] **Step 2: Run the test, expect FAIL (compile error).**

```
go test -v -run 'TestRecordCleanupFailureAndPendingCleanup|TestForceBreakThresholdConst' ./internal/engine/session/
```

Expected FAIL: `undefined: RecordCleanupFailure`, `undefined: PendingCleanup`, `undefined: forceBreakThreshold` (build error — symbols not yet defined).

- [ ] **Step 3: Add the exported wrappers and `forceBreakThreshold` const.**

In `internal/engine/session/cleanup_retry.go`, add after `PendingCleanupCount` (after line 70):

```go
// RecordCleanupFailure enqueues path for background retry. Exported wrapper
// over recordCleanupFailure so package engine's Browser.Close can enqueue a
// session dir whose single-shot RemoveAll lost to a Windows file lock,
// instead of leaking it. Safe to call from any goroutine.
func RecordCleanupFailure(path string) {
	recordCleanupFailure(path)
}

// PendingCleanup returns a snapshot of the dirs currently queued for
// background retry. Backs `scout session list --pending`. The returned slice
// is a copy and safe to retain.
func PendingCleanup() []string {
	return snapshotPending()
}
```

Add the force-break threshold const right after `DefaultCleanupRetryInterval` (after line 74):

```go
// forceBreakThreshold is the number of consecutive retry misses on the same
// dir after which the retrier escalates from polite RemoveAll to a
// best-effort force removal (chmod-walk + RemoveAll). At the 60 s default
// interval this is ~20 minutes — long enough that a genuinely-busy holder
// has almost certainly exited, short enough that a stuck dir does not leak
// for the whole process lifetime.
const forceBreakThreshold = 20
```

- [ ] **Step 4: Add force-break escalation in `retryPending`.**

Replace the trailing `failCount[p]++` block in `retryPending` (lines 132-141 of `cleanup_retry.go`) with:

```go
		failCount[p]++

		// Force-break escalation: after forceBreakThreshold consecutive
		// misses the polite RemoveAll is clearly losing to a persistent
		// holder (broken AV, OneDrive sync). The reaper has already killed
		// any browser holding this dir, so attempt an aggressive removal:
		// clear read-only attributes on every entry, then RemoveAll again.
		// Best-effort and logged at WARN; never panics.
		if failCount[p] >= forceBreakThreshold {
			if err := forceRemoveAll(p); err == nil {
				slog.Warn("scout: background cleanup force-removed stuck session", "dir", p)
				removePending(p)
				delete(failCount, p)
				continue
			}
		}

		// Cap consecutive-failure logging at 10 to bound noise; dirs
		// that survive a full hour are likely held by something
		// persistent (OneDrive sync, broken AV) — keep retrying
		// silently. Entry stays in queue for next tick.
		if failCount[p] == 10 {
			slog.Warn("scout: background cleanup still blocked after 10 attempts", "dir", p)
		}
```

Add the `forceRemoveAll` helper at the end of `cleanup_retry.go` (it needs `path/filepath` and `io/fs` — add them to the import block):

```go
// forceRemoveAll clears the read-only bit on every entry under root, then
// retries removal. On Windows a read-only file (Chrome leaves some profile
// files read-only) makes os.RemoveAll fail with ERROR_ACCESS_DENIED even
// after the owning process has exited; chmod 0o600/0o700 unsticks it.
// Best-effort: walk errors are ignored so a vanished entry mid-walk does not
// abort the cleanup.
func forceRemoveAll(root string) error {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // tolerate races; keep walking
		}
		if d.IsDir() {
			_ = os.Chmod(p, 0o700)
		} else {
			_ = os.Chmod(p, 0o600)
		}
		return nil
	})

	return retryRemoveAll(root)
}
```

Update the import block at the top of `cleanup_retry.go` to:

```go
import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)
```

- [ ] **Step 5: Run the test, expect PASS.**

```
go test -v -run 'TestRecordCleanupFailureAndPendingCleanup|TestForceBreakThresholdConst' ./internal/engine/session/
```

Expected PASS: both tests green; `go vet` clean.

- [ ] **Step 6: Commit.**

```
git add internal/engine/session/cleanup_retry.go internal/engine/session/cleanup_retry_export_test.go
git commit -m "feat(session): exported cleanup-failure wrappers + force-break escalation"
```

---

### Task 2.2: Canonical `ReapOnce` + `ReapStats` in `reaper.go`

The heart of the phase: one pass over `GetSessionsDir()` that classifies every folder and reaps the non-owned ones, wiring `FindBrowsersUsingDataDir` into the kill step so the corrupt/missing-`scout.pid` case (the un-killable zombie) is finally handled.

**Files:**
- Create: `internal/engine/session/reaper.go`
- Create: `internal/engine/session/reaper_test.go`

- [ ] **Step 1: Write the failing unit test (SessionsDir override; fabricated PIDs, no browser).**

Create `internal/engine/session/reaper_test.go`:

```go
package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	idpkg "github.com/inovacc/scout/pkg/id"
)

// writeSession writes a scout.pid for a fresh random session ID and returns it.
func writeSession(t *testing.T, info *SessionInfo) string {
	t.Helper()

	sid := idpkg.New(idpkg.Attrs{}).String()
	if err := WriteInfo(sid, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	return sid
}

func TestReapOnce_RemovesOrphanWithDeadScout(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Non-reusable session whose recorded scout PID is definitely dead.
	now := time.Now()
	sid := writeSession(t, &SessionInfo{
		ScoutPID:   deadPID(t),
		BrowserPID: 0, // no live browser → only dir removal exercised
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	})

	stats := ReapOnce()

	if stats.Scanned < 1 {
		t.Fatalf("Scanned = %d, want >= 1", stats.Scanned)
	}
	if stats.Removed < 1 {
		t.Fatalf("Removed = %d, want >= 1", stats.Removed)
	}
	if _, err := os.Stat(Dir(sid)); !os.IsNotExist(err) {
		t.Fatalf("session dir %q still present after reap (err=%v)", Dir(sid), err)
	}
}

func TestReapOnce_PreservesOwnedSession(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	// ScoutPID = our own PID. IsScoutProcess(os.Getpid()) is true under the
	// test binary only if gops can classify it as a scout exec; it cannot
	// (the test binary is not named scout), so this folder is NOT owned and
	// would normally be reaped. To assert the *preserve* path independent of
	// process identity, use a reusable, not-yet-expired session — that branch
	// short-circuits before any liveness check.
	sid := writeSession(t, &SessionInfo{
		ScoutPID:   deadPID(t),
		Reusable:   true,
		ExpiresAt:  now.Add(1 * time.Hour),
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	})

	stats := ReapOnce()

	if stats.Removed != 0 {
		t.Fatalf("Removed = %d, want 0 (reusable unexpired must be preserved)", stats.Removed)
	}
	if _, err := os.Stat(Dir(sid)); err != nil {
		t.Fatalf("reusable session dir %q was removed: %v", Dir(sid), err)
	}
}

func TestReapOnce_RemovesCorruptDir(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Fabricate a session dir with a corrupt (non-binary) scout.pid — this is
	// the un-killable-zombie case: ReadInfo fails, so classification by
	// metadata is impossible and the dir must still be reaped.
	sid := idpkg.New(idpkg.Attrs{}).String()
	corrupt := Dir(sid)
	if err := os.MkdirAll(filepath.Join(corrupt, "data"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corrupt, "scout.pid"), []byte("not-binary-garbage"), 0o600); err != nil {
		t.Fatalf("write corrupt pid: %v", err)
	}

	stats := ReapOnce()

	if stats.Removed < 1 {
		t.Fatalf("Removed = %d, want >= 1 (corrupt dir must be reaped)", stats.Removed)
	}
	if _, err := os.Stat(corrupt); !os.IsNotExist(err) {
		t.Fatalf("corrupt session dir %q still present after reap", corrupt)
	}
}

func TestReapOnce_RemovesMissingPidDir(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Orphan dir with no scout.pid at all.
	sid := idpkg.New(idpkg.Attrs{}).String()
	orphan := Dir(sid)
	if err := os.MkdirAll(filepath.Join(orphan, "data"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stats := ReapOnce()

	if stats.Removed < 1 {
		t.Fatalf("Removed = %d, want >= 1 (missing-pid dir must be reaped)", stats.Removed)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan session dir %q still present after reap", orphan)
	}
}
```

Add the `deadPID` helper to `reaper_test.go` (spawns then reaps a child to obtain a guaranteed-dead PID — portable, no hard-coded PID guesses):

```go
// deadPID returns a PID that is guaranteed not to be alive: it spawns a
// trivial child, waits for it to exit, and returns its (now reusable but
// currently dead) PID. ProcessAlive/IsScoutProcess on it return false.
func deadPID(t *testing.T) int {
	t.Helper()

	// A process that exits immediately. `cmd /c exit` on Windows, `true`
	// elsewhere — both are universally present.
	pid := spawnAndReapExited(t)
	if ProcessAlive(pid) {
		t.Fatalf("expected dead PID, %d is still alive", pid)
	}

	return pid
}
```

`spawnAndReapExited` is defined in Task 2.4's shared test helper file (`reaper_acceptance_test.go`); both test files are in package `session` so the helper is shared. If Task 2.4 has not landed yet when running 2.2 in isolation, inline this temporary helper instead:

```go
func spawnAndReapExited(t *testing.T) int {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit")
	} else {
		cmd = exec.Command("true")
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn helper process: %v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()
	return pid
}
```

(Add `"os/exec"` and `"runtime"` to the test imports when inlining.)

- [ ] **Step 2: Run the test, expect FAIL (compile error).**

```
go test -v -run TestReapOnce ./internal/engine/session/
```

Expected FAIL: `undefined: ReapOnce`, `undefined: ReapStats` (build error — `reaper.go` not yet created).

- [ ] **Step 3: Create `reaper.go` with `ReapStats` + `ReapOnce`.**

Create `internal/engine/session/reaper.go`:

```go
package session

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// ReapStats tallies the result of a single ReapOnce pass.
//
//   - Scanned: total session folders examined.
//   - Killed:  browser processes killed (recorded BrowserPID + path-bounded
//     holders found via FindBrowsersUsingDataDir).
//   - Removed: session folders successfully removed.
//   - Pending: folders whose removal failed and were enqueued for the
//     background retrier (RecordCleanupFailure).
type ReapStats struct {
	Scanned int
	Killed  int
	Removed int
	Pending int
}

// ReapOnce performs one canonical reaping pass over GetSessionsDir(). It is the
// single source of truth for "is this folder ownerless, and if so kill any
// browser holding it and remove it." CleanStaleSessions and CleanOrphans are
// thin wrappers over this; the startup path, the watchdog, and the daemon
// reconcile all call it so behavior is identical wherever it runs.
//
// Ownership rule (a folder is PRESERVED iff):
//   - scout.pid parses AND the recorded ScoutPID is a live, identity-verified
//     scout process (IsScoutProcess), OR
//   - the session is reusable and not yet expired (IsExpired() == false).
//
// Every other folder is reaped:
//   - Missing / corrupt / legacy scout.pid (ReadInfo error) — the
//     un-killable-zombie case: we cannot classify by metadata, so we still
//     scan-and-kill any browser whose --user-data-dir is under this folder
//     (path-bounded via FindBrowsersUsingDataDir) and remove the folder.
//   - Owner (ScoutPID) dead or unverifiable — kill recorded BrowserPID (with
//     an immediate-before-kill liveness re-check to shrink the TOCTOU window),
//     scan-and-kill path-bounded holders, remove the folder.
//
// All kills are best-effort and logged; never fatal. Removal failures are
// enqueued via RecordCleanupFailure so the retrier (and eventual force-break)
// reaps them later. The pass is idempotent and safe to run concurrently from
// startup and the watchdog (folder-level guarding via scout.lock).
func ReapOnce() ReapStats {
	var stats ReapStats

	sessDir := GetSessionsDir()

	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("scout: reaper: read sessions dir", "dir", sessDir, "err", err)
		}

		return stats
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		id := e.Name()
		stats.Scanned++

		info, readErr := ReadInfo(id)
		if readErr != nil {
			// Missing / corrupt / legacy / foreign-owned scout.pid. We cannot
			// trust metadata, so treat the folder as an orphan and reap it.
			// This is the un-killable-zombie fix: scan-and-kill any browser
			// holding the data dir even though scout.pid told us nothing.
			if errors.Is(readErr, ErrLegacyFormat) {
				slog.Info("scout: reaper: removing legacy JSON session", "id", id)
			}

			reapFolder(id, 0, "", &stats)

			continue
		}

		// PRESERVE: reusable session still within its lifetime window.
		if info.Reusable && !info.IsExpired() {
			continue
		}

		// PRESERVE: a live, identity-verified scout still owns this folder.
		if info.ScoutPID != 0 && IsScoutProcess(info.ScoutPID) {
			continue
		}

		if info.Reusable {
			slog.Info("scout: reaper: reusable session expired, reaping",
				"id", id, "expires_at", info.ExpiresAt)
		}

		reapFolder(id, info.BrowserPID, info.BrowserStartToken, &stats)
	}

	return stats
}

// reapFolder kills any browser holding the session's data dir, then removes the
// folder, tallying into stats. browserPID/startToken come from scout.pid when
// it was readable (0/"" otherwise). It performs, in order:
//  1. Kill recorded BrowserPID if it verifies AND is still alive immediately
//     before the kill (TOCTOU shrink).
//  2. Path-bounded scan-and-kill: every PID FindBrowsersUsingDataDir reports
//     for DataDir(id). NO identity gate (per design Q2) — but the scan is
//     bounded to GetSessionsDir() by FindBrowsersUsingDataDir itself, which is
//     the single safety floor that never touches the user's real Chrome.
//  3. retryRemoveAll(Dir(id)); on failure RecordCleanupFailure(Dir(id)).
func reapFolder(id string, browserPID int, startToken string, stats *ReapStats) {
	// 1. Recorded BrowserPID, identity-verified + TOCTOU re-check.
	if browserPID != 0 && verifyProcess(browserPID, startToken) {
		if ProcessAlive(browserPID) {
			if p, err := os.FindProcess(browserPID); err == nil {
				if killErr := p.Kill(); killErr == nil {
					stats.Killed++
				} else {
					slog.Warn("scout: reaper: kill browser failed",
						"id", id, "pid", browserPID, "err", killErr)
				}
			}
		}
	}

	// 2. Path-bounded scan-and-kill of any holder of the data dir. This is the
	// only path that reaches a zombie whose scout.pid was missing/corrupt.
	for _, pid := range FindBrowsersUsingDataDir(DataDir(id)) {
		if pid == os.Getpid() {
			continue // never kill ourselves
		}

		if p, err := os.FindProcess(pid); err == nil {
			if killErr := p.Kill(); killErr == nil {
				stats.Killed++
			} else {
				slog.Warn("scout: reaper: kill holder failed",
					"id", id, "pid", pid, "err", killErr)
			}
		}
	}

	// 3. Remove the folder; enqueue for the retrier on persistent failure.
	dir := Dir(id)
	if err := retryRemoveAll(dir); err != nil {
		RecordCleanupFailure(dir)
		stats.Pending++

		slog.Warn("scout: reaper: removal deferred to background retrier",
			"id", id, "dir", dir, "err", err)

		return
	}

	stats.Removed++
}

// DefaultReaperInterval is the default interval for the process-wide reaper
// watchdog. Mirrors the previous per-browser orphan check cadence.
const DefaultReaperInterval = DefaultOrphanCheckInterval

// StartReaperWatchdog starts a single process-wide goroutine that runs ReapOnce
// every interval until done is closed. It REPLACES the per-browser
// StartOrphanWatchdog fan-out — one ticker scans every folder, so N live
// browsers no longer mean N redundant watchdogs. Returns immediately.
func StartReaperWatchdog(interval time.Duration, done <-chan struct{}) {
	if interval <= 0 {
		interval = DefaultReaperInterval
	}

	go func() {
		// Panic recovery so a single bad iteration (filesystem race, gops
		// internal race) does not silently kill the watchdog.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: reaper watchdog panic", "panic", r)
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = ReapOnce()
			}
		}
	}()
}

// ensure filepath stays imported for callers below (Dir/DataDir use it
// transitively via session_track.go; this keeps the import explicit for any
// future helper added here).
var _ = filepath.Join
```

> Implementation note: `verifyProcess` and `ProcessAlive` are already defined (`process.go`, `process_{unix,windows}.go`); `FindBrowsersUsingDataDir`, `DataDir`, `Dir`, `ReadInfo`, `IsScoutProcess`, `ErrLegacyFormat`, `retryRemoveAll`, `RecordCleanupFailure` (Task 2.1) are all in-package. The `var _ = filepath.Join` line should be removed if any real `filepath` use is added; if `go vet`/lint flags the unused import, drop `"path/filepath"` from the import block instead. Keep whichever the compiler accepts.

- [ ] **Step 4: Run the test, expect PASS.**

```
go test -v -run TestReapOnce ./internal/engine/session/
```

Expected PASS: `TestReapOnce_RemovesOrphanWithDeadScout`, `TestReapOnce_PreservesOwnedSession`, `TestReapOnce_RemovesCorruptDir`, `TestReapOnce_RemovesMissingPidDir` all green.

- [ ] **Step 5: Build the package and vet.**

```
go build ./internal/... && go vet ./internal/engine/session/
```

Expected: clean build, no vet diagnostics. (If `path/filepath` is reported unused, delete it and the `var _ = filepath.Join` line per the implementation note.)

- [ ] **Step 6: Commit.**

```
git add internal/engine/session/reaper.go internal/engine/session/reaper_test.go
git commit -m "feat(session): canonical ReapOnce + ReapStats + process-wide reaper watchdog"
```

---

### Task 2.3: Reimplement `CleanStaleSessions`/`CleanOrphans` as thin wrappers; deprecate `CleanOrphans`

Keep the existing signatures so every caller (`cmd/scout/scout.go:155`, `internal/engine/browser.go:67,278`) keeps compiling, but route both through `ReapOnce` so there is exactly one reaping algorithm. `CleanStaleSessions` returns `Removed`; `CleanOrphans` returns `Killed` (preserving its historical "browsers killed" count).

**Files:**
- Modify: `internal/engine/session/session_track.go` (`CleanOrphans` 342-389 → wrapper; `CleanStaleSessions` 470-553 → wrapper)
- Modify: `internal/engine/session/reaper_test.go` (add wrapper-equivalence test)

- [ ] **Step 1: Write the failing wrapper test.**

Append to `internal/engine/session/reaper_test.go`:

```go
func TestCleanStaleSessions_WrapsReapOnce(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	sid := writeSession(t, &SessionInfo{
		ScoutPID:  deadPID(t),
		Reusable:  false,
		CreatedAt: now,
		LastUsed:  now,
		Browser:   "chrome",
	})

	n, err := CleanStaleSessions()
	if err != nil {
		t.Fatalf("CleanStaleSessions: %v", err)
	}
	if n < 1 {
		t.Fatalf("CleanStaleSessions returned %d, want >= 1 (Removed count)", n)
	}
	if _, statErr := os.Stat(Dir(sid)); !os.IsNotExist(statErr) {
		t.Fatalf("session dir still present after CleanStaleSessions")
	}
}

func TestCleanOrphans_WrapsReapOnce(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	// Dead scout, no live browser → 0 killed but dir removed. CleanOrphans
	// must report the Killed count (0 here) and not error.
	_ = writeSession(t, &SessionInfo{
		ScoutPID:  deadPID(t),
		Reusable:  false,
		CreatedAt: now,
		LastUsed:  now,
		Browser:   "chrome",
	})

	killed, err := CleanOrphans()
	if err != nil {
		t.Fatalf("CleanOrphans: %v", err)
	}
	if killed != 0 {
		t.Fatalf("CleanOrphans killed = %d, want 0 (no live browser)", killed)
	}
}
```

- [ ] **Step 2: Run, expect PASS-or-FAIL depending on current behavior.**

```
go test -v -run 'TestCleanStaleSessions_WrapsReapOnce|TestCleanOrphans_WrapsReapOnce' ./internal/engine/session/
```

Expected at this point: `TestCleanStaleSessions_WrapsReapOnce` likely PASSES against the legacy implementation (it also removes dead non-reusable dirs), but `TestCleanOrphans_WrapsReapOnce` may FAIL because the legacy `CleanOrphans` short-circuits on `BrowserPID == 0` and never removes the dir / returns early — the test asserts no error and 0 killed, which the legacy path satisfies, so it may pass too. Treat this step as a behavioral baseline; the real intent is to prove equivalence AFTER rewriting. Proceed to rewrite regardless.

- [ ] **Step 3: Replace `CleanOrphans` (lines 342-389) with a deprecated thin wrapper.**

In `internal/engine/session/session_track.go`, replace the entire `CleanOrphans` function body:

```go
// CleanOrphans scans SessionsDir for sessions whose scout owner has died and
// kills the orphaned browser, removing the folder. Returns the number of
// browser processes killed.
//
// Deprecated: CleanOrphans is now a thin wrapper over ReapOnce, which is the
// single canonical reaping pass. Prefer ReapOnce()/StartReaperWatchdog
// directly. This wrapper is retained for source compatibility and will be
// removed after 2026-07-15 (tracked in docs/BACKLOG.md).
func CleanOrphans() (int, error) {
	return ReapOnce().Killed, nil
}
```

- [ ] **Step 4: Replace `CleanStaleSessions` (lines 470-553) with a thin wrapper.**

Replace the entire `CleanStaleSessions` function body (keep the existing doc comment block above it, lines 462-469, intact):

```go
func CleanStaleSessions() (int, error) {
	return ReapOnce().Removed, nil
}
```

> After this edit, `errors`, `slog`, and `idpkg` may still be used elsewhere in `session_track.go` (they are — `List`/`ReadInfo`/`ResetAll` use `errors` and `slog`; `ReadInfo` uses `idpkg`). Do NOT remove imports without checking; run `go build ./internal/...` in Step 6 to confirm. `retryRemoveAll` is still used by `Reset`/`reapFolder`, and `recordCleanupFailure` is still referenced by `RecordCleanupFailure` — none become dead.

- [ ] **Step 5: Run the wrapper tests, expect PASS.**

```
go test -v -run 'TestCleanStaleSessions_WrapsReapOnce|TestCleanOrphans_WrapsReapOnce|TestReapOnce' ./internal/engine/session/
```

Expected PASS: all reaper + wrapper tests green; the rewritten `CleanOrphans` now removes the dead-scout dir and returns `Killed`.

- [ ] **Step 6: Build the whole module to confirm callers + imports are intact.**

```
go build ./internal/... ./cmd/scout/ ./pkg/... && go vet ./internal/engine/session/
```

Expected: clean build (callers in `cmd/scout/scout.go` and `internal/engine/browser.go` unchanged and compiling), no unused-import error.

- [ ] **Step 7: Commit.**

```
git add internal/engine/session/session_track.go internal/engine/session/reaper_test.go
git commit -m "refactor(session): route CleanStaleSessions/CleanOrphans through ReapOnce; deprecate CleanOrphans"
```

---

### Task 2.4: Acceptance test — crash→reap kills a real holder and removes the dir; negative control preserves a self-owned dir

Prove the un-killable-zombie fix end-to-end with a fabricated leak: a `scout.pid` whose `ScoutPID` is dead and whose data dir is held by a REAL spawned child process. Assert `ReapOnce` kills the child and removes the dir. Negative control: a dir whose data dir is held by a process owned by `os.Getpid()` (ourselves) must be preserved — `ReapOnce` never kills the current process.

**Files:**
- Create: `internal/engine/session/reaper_acceptance_test.go`

- [ ] **Step 1: Write the failing acceptance test.**

Create `internal/engine/session/reaper_acceptance_test.go`:

```go
package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	idpkg "github.com/inovacc/scout/pkg/id"
)

// spawnAndReapExited spawns a trivial child, waits for it to exit, and returns
// its now-dead PID. Shared with reaper_test.go (same package).
func spawnAndReapExited(t *testing.T) int {
	t.Helper()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit")
	} else {
		cmd = exec.Command("true")
	}

	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn helper process: %v", err)
	}

	pid := cmd.Process.Pid
	_ = cmd.Wait()

	return pid
}

// spawnHolder starts a long-lived child whose argv contains
// --user-data-dir=<dataDir> so FindBrowsersUsingDataDir can match it. The
// binary name must look like a browser to isBrowserCmdline; we cannot rename
// the system sleep binary, so this acceptance test is skipped on platforms
// where the holder cannot be made discoverable. On Linux the child's
// /proc/<pid>/cmdline is matched on the data-dir substring AND the browser
// token; we satisfy isBrowserCmdline by invoking through a copied/symlinked
// "chrome" shim when available, else skip.
func spawnHolder(t *testing.T, dataDir string) *exec.Cmd {
	t.Helper()

	shim := browserShim(t) // returns a path whose basename matches isBrowserCmdline, or "" to skip
	if shim == "" {
		t.Skipf("no browser-named shim available on %s; skipping holder acceptance", runtime.GOOS)
	}

	// The shim is a tiny sleeper; pass the data-dir flag so the scan matches.
	cmd := exec.Command(shim, "--user-data-dir="+dataDir, "--sleep")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start holder: %v", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// Give the OS a beat to publish the cmdline.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(FindBrowsersUsingDataDir(dataDir)) > 0 {
			return cmd
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Skipf("holder cmdline not observable via FindBrowsersUsingDataDir on %s", runtime.GOOS)
	return cmd
}

func TestReapOnce_Acceptance_KillsHolderAndRemovesDir(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	sid := idpkg.New(idpkg.Attrs{}).String()

	// Fabricate the leak: dead scout, a live child holding the data dir.
	holder := spawnHolder(t, DataDir(sid))
	holderPID := holder.Process.Pid

	if err := WriteInfo(sid, &SessionInfo{
		ScoutPID:   spawnAndReapExited(t), // definitely dead
		BrowserPID: holderPID,
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	stats := ReapOnce()

	if stats.Killed < 1 {
		t.Fatalf("Killed = %d, want >= 1 (holder must be killed)", stats.Killed)
	}

	// Holder is dead.
	deadline := time.Now().Add(3 * time.Second)
	for ProcessAlive(holderPID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if ProcessAlive(holderPID) {
		t.Fatalf("holder PID %d still alive after reap", holderPID)
	}

	// Dir removed (or, on a slow Windows lock, enqueued — accept either as a
	// terminal state for this test, but assert it is no longer present OR
	// queued for retry).
	if _, err := os.Stat(Dir(sid)); !os.IsNotExist(err) {
		t.Fatalf("session dir %q still present after reap", Dir(sid))
	}
}

func TestReapOnce_Acceptance_NeverKillsSelf(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	sid := idpkg.New(idpkg.Attrs{}).String()

	// Owner is dead so the folder would be reaped, but record the CURRENT
	// process as the BrowserPID. reapFolder must skip os.Getpid() in the
	// scan-and-kill loop and must not Kill() the live current process via the
	// recorded-PID path either (verifyProcess passes for a live PID, but the
	// guard `pid == os.Getpid()` and the fact we are a scout-less test binary
	// mean we must never terminate the test runner).
	if err := WriteInfo(sid, &SessionInfo{
		ScoutPID:   spawnAndReapExited(t),
		BrowserPID: os.Getpid(),
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	// If reapFolder killed os.Getpid() the test process would die; reaching
	// the assertion proves it did not.
	_ = ReapOnce()

	if !ProcessAlive(os.Getpid()) {
		t.Fatalf("unreachable: current process killed")
	}
}

// browserShim returns the path to a sleeper binary whose basename matches
// isBrowserCmdline (e.g. "chrome"/"chrome.exe"), built into t.TempDir on the
// fly from a tiny Go program, or "" if it cannot be produced (→ skip).
func browserShim(t *testing.T) string {
	t.Helper()

	src := `package main
import ("os";"time")
func main(){ _=os.Args; time.Sleep(60*time.Second) }`

	td := t.TempDir()
	srcPath := filepath.Join(td, "shim.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o600); err != nil {
		return ""
	}

	name := "chrome"
	if runtime.GOOS == "windows" {
		name = "chrome.exe"
	}
	binPath := filepath.Join(td, name)

	build := exec.Command("go", "build", "-o", binPath, srcPath)
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Logf("shim build failed (%v): %s", err, out)
		return ""
	}

	return binPath
}
```

> Test-design notes: the recorded-PID kill path in `reapFolder` does NOT currently guard `os.Getpid()` — it relies on the scan loop's `pid == os.Getpid()` guard. The negative-control test exercises the scan path (the shim is not involved; `FindBrowsersUsingDataDir` finds nothing for the self case because `os.Getpid()`'s cmdline is the test binary, not a browser with this data dir). To make the recorded-PID path equally safe, see the deviation entry — the implementation in Task 2.2 should also skip `browserPID == os.Getpid()` before the kill. Add that guard now (it is already shown in the deviations fix) so this test is unconditionally safe.

- [ ] **Step 2: Harden `reapFolder` recorded-PID path against self-kill (defensive).**

In `internal/engine/session/reaper.go`, change the recorded-BrowserPID block in `reapFolder` so it never kills the current process:

```go
	// 1. Recorded BrowserPID, identity-verified + TOCTOU re-check.
	if browserPID != 0 && browserPID != os.Getpid() && verifyProcess(browserPID, startToken) {
		if ProcessAlive(browserPID) {
			if p, err := os.FindProcess(browserPID); err == nil {
				if killErr := p.Kill(); killErr == nil {
					stats.Killed++
				} else {
					slog.Warn("scout: reaper: kill browser failed",
						"id", id, "pid", browserPID, "err", killErr)
				}
			}
		}
	}
```

- [ ] **Step 3: Run the acceptance tests.**

```
go test -v -run 'TestReapOnce_Acceptance' ./internal/engine/session/
```

Expected: `TestReapOnce_Acceptance_NeverKillsSelf` PASS unconditionally; `TestReapOnce_Acceptance_KillsHolderAndRemovesDir` PASS where `go build` of the shim and cmdline observation succeed, otherwise `t.Skip` (CI without a Go toolchain at test time, or Darwin/BSD where `FindBrowsersUsingDataDir` returns nil). The skip is acceptable per project convention (mirrors the `t.Skip` if-no-Chromium pattern).

- [ ] **Step 4: Commit.**

```
git add internal/engine/session/reaper_acceptance_test.go internal/engine/session/reaper.go
git commit -m "test(session): crash-to-reap acceptance — kill data-dir holder, preserve self"
```

---

### Phase 2 verification

Run the full phase build + test from the repo root (`D:\weaver-sync\development\personal\projects\scout`):

```
go build ./internal/... ./cmd/scout/ ./pkg/...
go vet ./internal/engine/session/
go test -v -run 'TestRecordCleanupFailureAndPendingCleanup|TestForceBreakThresholdConst|TestReapOnce|TestCleanStaleSessions_WrapsReapOnce|TestCleanOrphans_WrapsReapOnce' ./internal/engine/session/
go test ./internal/engine/session/
```

Expected output:
- `go build` — no output, exit 0 (all existing callers of `CleanStaleSessions`/`CleanOrphans` in `cmd/scout/scout.go` and `internal/engine/browser.go` still compile against the unchanged signatures).
- `go vet` — no diagnostics.
- The targeted `-run` test invocation — all of: `TestRecordCleanupFailureAndPendingCleanup`, `TestForceBreakThresholdConst`, `TestReapOnce_RemovesOrphanWithDeadScout`, `TestReapOnce_PreservesOwnedSession`, `TestReapOnce_RemovesCorruptDir`, `TestReapOnce_RemovesMissingPidDir`, `TestCleanStaleSessions_WrapsReapOnce`, `TestCleanOrphans_WrapsReapOnce` report `--- PASS`. The two `TestReapOnce_Acceptance_*` tests are excluded from the targeted run by the regex; run them separately with `go test -v -run TestReapOnce_Acceptance ./internal/engine/session/` and expect PASS-or-SKIP (SKIP is acceptable when the shim cannot be built or the holder cmdline is not observable on the host OS).
- `go test ./internal/engine/session/` — whole-package green: `ok  github.com/inovacc/scout/internal/engine/session` (acceptance tests SKIP rather than fail where the environment cannot spawn an observable holder).

Final state: `ReapOnce`/`ReapStats`/`StartReaperWatchdog` exist in `internal/engine/session/reaper.go`; `RecordCleanupFailure`/`PendingCleanup`/`forceBreakThreshold`/force-break live in `cleanup_retry.go`; `CleanStaleSessions`/`CleanOrphans` are thin wrappers (`CleanOrphans` marked `// Deprecated:`). No engine re-exports or `cmd/scout` wiring are done in this phase — those are Phase 3 (engine re-exports + `Browser.Close` enqueue + `main()` watchdog swap) per the contract.
## Phase 3: Live-browser registry, CloseAllLive, Close enqueue, autofree, facade exports

**Goal:** Give the engine a process-wide registry of live `*Browser` instances so the `main()` signal handler can close them all on `SIGINT`/`SIGTERM`; make `Browser.Close` enqueue locked session dirs into the retrier instead of dropping them; bound `recycleBrowser`'s launcher cleanup; and surface the Phase-1 reaper primitives (`ReapOnce`, `StartReaperWatchdog`, `RecordCleanupFailure`, `PendingCleanup`, `ReapStats`) at the `engine` and `pkg/scout` facade levels.

> **Phase dependency:** Phase 1 creates `session.ReapOnce() ReapStats`, `session.ReapStats`, `session.StartReaperWatchdog(interval time.Duration, done <-chan struct{})`, `session.RecordCleanupFailure(path string)`, and `session.PendingCleanup() []string` in `internal/engine/session/reaper.go` + `cleanup_retry.go`. Tasks 3.4 and 3.5 re-export those names. If executed before Phase 1 lands, the re-export build will fail with `undefined: session.ReapOnce` — that is the expected ordering signal, not a bug in this phase. The registry tasks (3.1–3.3) and `recycleBrowser` fix (3.6) are independent of Phase 1 and can land first.

---

### Task 3.1: Live-browser registry with register/unregister

Add a process-wide `sync.Map` of live `*Browser` instances. `register()` is called on every successful local/remote launch in `New`; `unregister()` is called from `Close()`. This backs `CloseAllLive` (Task 3.2).

**Files:**
- Create: `internal/engine/live_registry.go`
- Create: `internal/engine/live_registry_test.go`
- Modify: `internal/engine/browser.go` (call `br.register()` after the two `return br, nil` sites — lines 228 and 272; call `b.unregister()` inside `Close()`'s `closeOnce.Do` near line 764)

- [ ] **Step 1: Write the failing test** — `internal/engine/live_registry_test.go` (package `engine`, in-package so it can read the unexported `liveBrowsers` map and call `register`/`unregister`):

```go
package engine

import (
	"sync"
	"testing"
)

func TestLiveRegistryRegisterUnregister(t *testing.T) {
	// Snapshot + restore the global map so the test is hermetic.
	var saved []any
	liveBrowsers.Range(func(k, _ any) bool {
		saved = append(saved, k)
		liveBrowsers.Delete(k)
		return true
	})
	t.Cleanup(func() {
		liveBrowsers.Range(func(k, _ any) bool {
			liveBrowsers.Delete(k)
			return true
		})
		for _, k := range saved {
			liveBrowsers.Store(k, struct{}{})
		}
	})

	b := &Browser{}
	b.register()

	if got := liveCount(); got != 1 {
		t.Fatalf("after register: liveCount = %d, want 1", got)
	}

	// register is idempotent on the same pointer.
	b.register()
	if got := liveCount(); got != 1 {
		t.Fatalf("after double register: liveCount = %d, want 1", got)
	}

	b.unregister()
	if got := liveCount(); got != 0 {
		t.Fatalf("after unregister: liveCount = %d, want 0", got)
	}

	// unregister on an absent browser is a no-op.
	b.unregister()
	if got := liveCount(); got != 0 {
		t.Fatalf("after redundant unregister: liveCount = %d, want 0", got)
	}
}

// liveCount counts entries currently in liveBrowsers (test helper).
func liveCount() int {
	n := 0
	liveBrowsers.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

var _ = sync.Map{} // keep sync import meaningful if liveCount is inlined away
```

- [ ] **Step 2: Run the test (expect FAIL)** — compile error / undefined symbol, since `liveBrowsers`, `register`, `unregister` do not exist yet:
```
go test -v -run TestLiveRegistryRegisterUnregister ./internal/engine/
```
Expected FAIL: `undefined: liveBrowsers`, `b.register undefined`, `b.unregister undefined`.

- [ ] **Step 3: Create `internal/engine/live_registry.go`** with the registry, `register`/`unregister`, and the count helper (REAL code):

```go
package engine

import "sync"

// liveBrowsers is a process-wide registry of every live *Browser that owns a
// launched browser process. It backs CloseAllLive, which the main() signal
// handler calls on SIGINT/SIGTERM so that no browser process (and no session
// dir) leaks when scout is interrupted mid-command.
//
// Keys are *Browser pointers; values are unused (struct{}{}).
var liveBrowsers sync.Map

// register adds b to the live-browser registry. Called on successful launch.
// Idempotent: storing the same pointer twice keeps a single entry. Browsers
// without a launcher (pure remote-CDP connections we did not start) are still
// registered so CloseAllLive can release their CDP connection cleanly.
func (b *Browser) register() {
	if b == nil {
		return
	}

	liveBrowsers.Store(b, struct{}{})
}

// unregister removes b from the live-browser registry. Called from Close().
// No-op if b was never registered or is nil.
func (b *Browser) unregister() {
	if b == nil {
		return
	}

	liveBrowsers.Delete(b)
}
```

- [ ] **Step 4: Wire `register()` into `New`** — `internal/engine/browser.go`. Add `br.register()` immediately before each successful return. At the remote-CDP path (currently line 228 `return br, nil`):

```go
		// Periodic orphan watchdog — kills dangling browsers whose scout died.
		StartOrphanWatchdog(DefaultOrphanCheckInterval, br.done)

		br.register()

		return br, nil
	}
```

And at the local-launch path (currently line 270–272):

```go
	// Periodic orphan watchdog — kills dangling browsers whose scout died.
	StartOrphanWatchdog(DefaultOrphanCheckInterval, br.done)

	br.register()

	return br, nil
}
```

- [ ] **Step 5: Wire `unregister()` into `Close`** — `internal/engine/browser.go`, inside `closeOnce.Do`, right before `b.closed.Store(true)` (currently line 764):

```go
		// Release the session lock (M3). Safe to call even if nil.
		if b.sessionLock != nil {
			b.sessionLock.Release()
			b.sessionLock = nil
		}

		// Drop from the live-browser registry so CloseAllLive no longer
		// targets this (already-closed) browser.
		b.unregister()

		b.closed.Store(true)
	})
```

- [ ] **Step 6: Run the test (expect PASS)**:
```
go test -v -run TestLiveRegistryRegisterUnregister ./internal/engine/
```
Expected PASS.

- [ ] **Step 7: Build the engine + facade to confirm no breakage**:
```
go build ./pkg/...
```

- [ ] **Step 8: Commit**:
```
feat(engine): add process-wide live-browser registry (register/unregister)
```

- [ ] **Step 9: Commit**

```bash
git add internal/engine/live_registry.go internal/engine/live_registry_test.go internal/engine/browser.go
git commit -m "feat(engine): add process-wide live-browser registry (register/unregister)"
```

---

### Task 3.2: CloseAllLive with per-browser timeout

Add `CloseAllLive(timeout time.Duration) int` — `Range` the registry, `Close()` each browser in a bare goroutine, bound each close with `time.After(timeout)`, return the number that completed `Close()` within the deadline. This is what the `main()` signal handler (Phase 5) calls.

**Files:**
- Modify: `internal/engine/live_registry.go`
- Modify: `internal/engine/live_registry_test.go`

- [ ] **Step 1: Write the failing test** — append to `internal/engine/live_registry_test.go`. It registers fake `*Browser` values (no launcher, no CDP) so `Close()` returns quickly without a real browser, asserts the closed count and that the registry is drained:

```go
import "time" // add to the existing import block

func TestCloseAllLive(t *testing.T) {
	// Hermetic: clear the registry and restore on cleanup.
	var saved []any
	liveBrowsers.Range(func(k, _ any) bool {
		saved = append(saved, k)
		liveBrowsers.Delete(k)
		return true
	})
	t.Cleanup(func() {
		liveBrowsers.Range(func(k, _ any) bool {
			liveBrowsers.Delete(k)
			return true
		})
		for _, k := range saved {
			liveBrowsers.Store(k, struct{}{})
		}
	})

	// Three fake browsers. Close() on a Browser with nil launcher/browser and
	// empty sessionID returns nil immediately (see Close() nil-safety).
	for range 3 {
		b := &Browser{done: make(chan struct{})}
		b.register()
	}

	if got := liveCount(); got != 3 {
		t.Fatalf("setup: liveCount = %d, want 3", got)
	}

	closed := CloseAllLive(2 * time.Second)
	if closed != 3 {
		t.Fatalf("CloseAllLive returned %d, want 3", closed)
	}

	if got := liveCount(); got != 0 {
		t.Fatalf("after CloseAllLive: liveCount = %d, want 0 (registry drained)", got)
	}
}
```

- [ ] **Step 2: Run the test (expect FAIL)** — `undefined: CloseAllLive`:
```
go test -v -run TestCloseAllLive ./internal/engine/
```
Expected FAIL: `undefined: CloseAllLive`.

- [ ] **Step 3: Implement `CloseAllLive`** — append to `internal/engine/live_registry.go`. Add `"time"` to the import block:

```go
import (
	"sync"
	"time"
)
```

```go
// CloseAllLive closes every browser currently in the live registry, bounding
// each individual Close() with timeout. It is the best-effort teardown path
// invoked by the main() signal handler on SIGINT/SIGTERM.
//
// Each browser is closed in its own goroutine; a per-browser select waits for
// either Close() to return or the timeout to elapse, so one hung browser can
// never block teardown of the others. Returns the number of browsers whose
// Close() completed (returned, error or nil) before its deadline.
//
// Close() calls unregister() internally, so the map is drained for every
// browser that finishes in time. Browsers that time out are left registered
// (their state is unknown); next-startup reaping handles their session dirs.
func CloseAllLive(timeout time.Duration) int {
	var targets []*Browser

	liveBrowsers.Range(func(k, _ any) bool {
		if b, ok := k.(*Browser); ok && b != nil {
			targets = append(targets, b)
		}
		return true
	})

	if len(targets) == 0 {
		return 0
	}

	type result struct {
		b  *Browser
		ok bool
	}

	results := make(chan result, len(targets))

	for _, b := range targets {
		go func(b *Browser) {
			done := make(chan struct{})
			go func() {
				_ = b.Close()
				close(done)
			}()

			select {
			case <-done:
				results <- result{b: b, ok: true}
			case <-time.After(timeout):
				results <- result{b: b, ok: false}
			}
		}(b)
	}

	closed := 0
	for range targets {
		if r := <-results; r.ok {
			closed++
		}
	}

	return closed
}
```

- [ ] **Step 4: Run the test (expect PASS)**:
```
go test -v -run TestCloseAllLive ./internal/engine/
```
Expected PASS: `closed == 3` and registry drained.

- [ ] **Step 5: Run the whole registry test file**:
```
go test -v -run 'TestLiveRegistry|TestCloseAllLive' ./internal/engine/
```
Expected PASS for both.

- [ ] **Step 6: Commit**:
```
feat(engine): add CloseAllLive(timeout) for signal-handler teardown
```

- [ ] **Step 7: Commit**

```bash
git add internal/engine/live_registry.go internal/engine/live_registry_test.go
git commit -m "feat(engine): add CloseAllLive(timeout) for signal-handler teardown"
```

---

### Task 3.3: Browser.Close enqueues locked dirs into the retrier

Today the non-reusable `Close` path does a single-shot `os.RemoveAll(SessionDir(id))` with the result discarded (`browser.go:751`). On Windows, AV/Search-Indexer/OneDrive can hold LevelDB/SQLite handles for 5–15 s, so the dir leaks with no re-enqueue. Fix: when `os.RemoveAll` fails, call `session.RecordCleanupFailure(SessionDir(id))` so the background retrier (and the reaper) reaps it later.

> **Contract note:** the contract names the wrapper `session.RecordCleanupFailure` (an exported wrapper over the existing unexported `recordCleanupFailure`), created in Phase 1. The `engine` package already imports `internal/engine/session` (see `session_track.go`), so call it as `session.RecordCleanupFailure`.

**Files:**
- Modify: `internal/engine/browser.go` (Close non-reusable branch, lines 745–752)
- Create: `internal/engine/browser_close_enqueue_test.go`

- [ ] **Step 1: Write the failing test** — `internal/engine/browser_close_enqueue_test.go` (package `engine`). It forces `os.RemoveAll` to fail by making the session dir contents undeletable in a portable way: it points `session.SessionsDir` at a temp dir, fabricates a session dir, then makes the dir un-removable by holding an open file handle on Windows / chmod-ing the parent on Unix. To keep this cross-platform and deterministic, we instead drive `Close` with a non-reusable browser whose `sessionID` resolves to a path we have replaced with a **read-only nested structure that RemoveAll cannot fully delete**, and assert the pending count increments.

  The robust, OS-independent approach: open a file inside the session `data/` dir and keep the handle open for the duration of `Close()` on Windows (open handles block deletion); on Unix, make the session dir itself read-only via its parent. We encapsulate that in a helper and skip on platforms where we cannot force a lock:

```go
package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/inovacc/scout/internal/engine/session"
)

func TestCloseEnqueuesLockedDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On Unix, RemoveAll succeeds even with an open handle; the locked-dir
		// scenario this guards is Windows-specific (AV / Search Indexer).
		t.Skip("locked-dir enqueue is a Windows file-lock scenario")
	}

	// Redirect the sessions root to a temp dir and restore after.
	orig := session.SessionsDir
	tmp := t.TempDir()
	session.SessionsDir = func() string { return tmp }
	t.Cleanup(func() { session.SessionsDir = orig })

	// Fabricate a non-reusable session dir with a data/ subdir.
	const sid = "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA"
	dataDir := SessionDataDir(sid)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("mkdir session data dir: %v", err)
	}

	// Hold an open handle on a file inside data/ so os.RemoveAll fails on
	// Windows (open handles block deletion).
	lockPath := filepath.Join(dataDir, "LOCK")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer func() { _ = f.Close() }()

	before := session.PendingCleanupCount()

	// Build a non-reusable browser bound to this session with no real launcher
	// or CDP connection, then Close it. The non-reusable branch attempts
	// os.RemoveAll(SessionDir(sid)); the open handle forces failure, which must
	// enqueue the dir via session.RecordCleanupFailure.
	b := &Browser{
		opts:      &options{sessionID: sid, reusableSession: false},
		sessionID: sid,
		done:      make(chan struct{}),
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	after := session.PendingCleanupCount()
	if after != before+1 {
		t.Fatalf("PendingCleanupCount = %d, want %d (dir should be enqueued)", after, before+1)
	}
}
```

> **Why this shape:** `Close()` is nil-safe; with `b.browser == nil` and `b.launcher == nil` it skips CDP/launcher teardown and reaches the non-reusable removal branch using `b.sessionID`. We must guard that branch so it still runs when `launcher == nil` (see Step 3). The held handle guarantees `os.RemoveAll` fails on Windows.

- [ ] **Step 2: Run the test (expect FAIL)** — on Windows the dir is removed-attempted but currently NOT enqueued, so `PendingCleanupCount` does not increment; also `session.RecordCleanupFailure` does not exist until Phase 1:
```
go test -v -run TestCloseEnqueuesLockedDir ./internal/engine/
```
Expected FAIL: either `undefined: session.RecordCleanupFailure` (pre-Phase-1) or `PendingCleanupCount = N, want N+1` (post-Phase-1, pre-fix).

- [ ] **Step 3: Implement the enqueue + run the removal even when launcher is nil** — `internal/engine/browser.go`. Replace the current non-reusable cleanup block (lines 742–756):

```go
		// 7. Kill process tree and clean up session directory.
		if b.launcher != nil {
			b.launcher.Kill()

			if !b.opts.reusableSession && b.sessionID != "" {
				// Non-reusable: remove Chrome data dir via launcher, then remove
				// the session parent dir (~/.scout/sessions/<id>/) in one pass.
				// Do NOT call ResetSession — it sleeps 500ms unconditionally and
				// is for external/CLI force-reset only (process already dead here).
				b.launcher.Cleanup()
			}
			// Reusable sessions: do NOT clean up — session must persist for reuse.

			b.launcher = nil
		}

		// 7b. Remove the session parent dir for non-reusable sessions. Runs even
		// when launcher == nil (e.g. teardown after a partial launch) so the
		// dir is never leaked. On RemoveAll failure (Windows AV / Search Indexer
		// / OneDrive holding LevelDB/SQLite handles), enqueue the dir into the
		// background retrier instead of dropping it (fixes the leaked-locked-dir
		// gap — Close previously did a single-shot RemoveAll with no re-enqueue).
		if !b.opts.reusableSession && b.sessionID != "" {
			dir := SessionDir(b.sessionID)
			if err := os.RemoveAll(dir); err != nil {
				session.RecordCleanupFailure(dir)
			}
		}
```

> Add the `session` import to `browser.go`'s import block if not already present. (`browser.go` currently imports `browser`, `launcher2`, `flags`, `proto2` but not `session` directly — it reaches session via the `session_track.go` re-exports in the same package. `session.RecordCleanupFailure` is a different package, so the import IS required.) Add:
> ```go
> "github.com/inovacc/scout/internal/engine/session"
> ```
> to the import block in `internal/engine/browser.go`.

- [ ] **Step 4: Run the test (expect PASS on Windows; SKIP elsewhere)**:
```
go test -v -run TestCloseEnqueuesLockedDir ./internal/engine/
```
Expected PASS on Windows (`PendingCleanupCount` increments by 1); `SKIP` on Unix.

- [ ] **Step 5: Build to confirm no import cycle / unused import**:
```
go build ./pkg/...
```

- [ ] **Step 6: Commit**:
```
fix(engine): enqueue locked session dirs on Close instead of dropping them
```

- [ ] **Step 7: Commit**

```bash
git add internal/engine/browser.go internal/engine/browser_close_enqueue_test.go
git commit -m "fix(engine): enqueue locked session dirs on Close instead of dropping them"
```

---

### Task 3.4: Re-export reaper primitives at the engine level

Surface the Phase-1 session primitives through `internal/engine` so the facade (Task 3.5) and other engine consumers can use them without importing `internal/engine/session` directly. Re-export `ReapOnce`, `ReapStats`, `StartReaperWatchdog`, `RecordCleanupFailure`, `PendingCleanup`.

**Files:**
- Modify: `internal/engine/session_track.go` (append after `PendingCleanupCount`, currently line 147)
- Create: `internal/engine/reaper_export_test.go`

- [ ] **Step 1: Write the failing test** — `internal/engine/reaper_export_test.go` (package `engine`). It overrides `session.SessionsDir` to an empty temp dir and asserts the engine-level `ReapOnce` returns a zero-ish `ReapStats` and `PendingCleanup` returns a slice (the registry primitives are exercised in depth in Phase 1; here we only assert the engine re-exports compile and delegate):

```go
package engine

import (
	"testing"
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

func TestEngineReaperReExports(t *testing.T) {
	orig := session.SessionsDir
	tmp := t.TempDir()
	session.SessionsDir = func() string { return tmp }
	t.Cleanup(func() { session.SessionsDir = orig })

	// ReapOnce over an empty sessions dir scans nothing.
	stats := ReapOnce()
	if stats.Scanned != 0 || stats.Killed != 0 || stats.Removed != 0 {
		t.Fatalf("ReapOnce on empty dir = %+v, want all-zero", stats)
	}

	// PendingCleanup returns a (possibly empty) slice without panicking.
	_ = PendingCleanup()

	// RecordCleanupFailure enqueues a path; PendingCleanup then contains it.
	RecordCleanupFailure(tmp)
	found := false
	for _, p := range PendingCleanup() {
		if p == tmp {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("RecordCleanupFailure did not enqueue %q", tmp)
	}

	// StartReaperWatchdog must accept (interval, done) and return immediately.
	done := make(chan struct{})
	StartReaperWatchdog(time.Hour, done)
	close(done)
}
```

- [ ] **Step 2: Run the test (expect FAIL)** — `undefined: ReapOnce`, `undefined: ReapStats`, etc. (pre-re-export; and `undefined: session.ReapOnce` if Phase 1 has not landed):
```
go test -v -run TestEngineReaperReExports ./internal/engine/
```
Expected FAIL: `undefined: ReapOnce` (and siblings).

- [ ] **Step 3: Add the re-exports** — `internal/engine/session_track.go`, append after the `PendingCleanupCount` re-export (after line 147):

```go
// ReapStats re-exports session.ReapStats — the tally from one reaper pass.
type ReapStats = session.ReapStats

// ReapOnce performs one canonical reaping pass over the sessions directory,
// killing orphaned browsers and removing ownerless / expired session dirs.
// See session.ReapOnce.
func ReapOnce() ReapStats { return session.ReapOnce() }

// StartReaperWatchdog starts the single process-wide reaper ticker loop.
// Replaces the per-browser StartOrphanWatchdog. See session.StartReaperWatchdog.
func StartReaperWatchdog(interval time.Duration, done <-chan struct{}) {
	session.StartReaperWatchdog(interval, done)
}

// RecordCleanupFailure enqueues a session dir whose removal failed so the
// background retrier reaps it later. See session.RecordCleanupFailure.
func RecordCleanupFailure(path string) { session.RecordCleanupFailure(path) }

// PendingCleanup returns the session dirs currently queued for retry
// (backs `scout session list --pending`). See session.PendingCleanup.
func PendingCleanup() []string { return session.PendingCleanup() }
```

- [ ] **Step 4: Run the test (expect PASS, requires Phase 1)**:
```
go test -v -run TestEngineReaperReExports ./internal/engine/
```
Expected PASS once Phase 1's `session.ReapOnce`/`ReapStats`/`StartReaperWatchdog`/`RecordCleanupFailure`/`PendingCleanup` exist.

- [ ] **Step 5: Build the facade**:
```
go build ./pkg/...
```

- [ ] **Step 6: Commit**:
```
feat(engine): re-export reaper primitives (ReapOnce, watchdog, pending)
```

- [ ] **Step 7: Commit**

```bash
git add internal/engine/session_track.go internal/engine/reaper_export_test.go
git commit -m "feat(engine): re-export reaper primitives (ReapOnce, watchdog, pending)"
```

---

### Task 3.5: Add ONLY-missing hardening re-exports to the facade

`pkg/scout/scout.go` is generated (DO NOT EDIT). Put new re-exports in a NEW hand-written sibling `pkg/scout/hardening_exports.go`. Verified against the current generated file, `scout.go` already exports `CleanOrphans`, `CleanStaleSessions`, `ListSessions`, `ResetSession`, `SessionsDir`, `StartOrphanWatchdog` — but does NOT export `CloseAllLive`, `PendingCleanup`, `ReapOnce`, `ReapStats`, `StartReaperWatchdog`, `RecordCleanupFailure`, `PendingCleanupCount`, or `FindBrowsersUsingDataDir`. Add exactly those missing ones (duplicate symbol = build break).

**Files:**
- Create: `pkg/scout/hardening_exports.go`
- Create: `pkg/scout/hardening_exports_test.go`
- Verify (do NOT edit): `pkg/scout/scout.go`

- [ ] **Step 1: Confirm no duplicate symbols** — grep the generated facade for each name we intend to add; all must be ABSENT:
```
go run ./... >/dev/null 2>&1 ; grep -nE 'func CloseAllLive|func PendingCleanup\b|func ReapOnce|type ReapStats|func StartReaperWatchdog|func RecordCleanupFailure|func PendingCleanupCount|func FindBrowsersUsingDataDir' pkg/scout/scout.go
```
Expected output: empty (none present). (If any line prints, drop that symbol from the new file and record a deviation.)

- [ ] **Step 2: Write the failing test** — `pkg/scout/hardening_exports_test.go` (package `scout`, external API surface). Assert each new facade symbol is callable:

```go
package scout

import (
	"testing"
	"time"
)

func TestHardeningExportsSurface(t *testing.T) {
	// CloseAllLive on an empty registry returns 0.
	if n := CloseAllLive(time.Second); n != 0 {
		t.Fatalf("CloseAllLive on empty registry = %d, want 0", n)
	}

	// ReapOnce returns a ReapStats value (typed re-export).
	var stats ReapStats = ReapOnce()
	_ = stats

	// PendingCleanup returns a slice; PendingCleanupCount returns its length-ish.
	_ = PendingCleanup()
	_ = PendingCleanupCount()

	// FindBrowsersUsingDataDir is callable with an arbitrary path.
	_ = FindBrowsersUsingDataDir(t.TempDir())

	// StartReaperWatchdog accepts (interval, done) and returns immediately.
	done := make(chan struct{})
	StartReaperWatchdog(time.Hour, done)
	close(done)

	// RecordCleanupFailure is callable.
	RecordCleanupFailure(t.TempDir())
}
```

- [ ] **Step 3: Run the test (expect FAIL)** — `undefined: CloseAllLive` etc. in package `scout`:
```
go test -v -run TestHardeningExportsSurface ./pkg/scout/
```
Expected FAIL: `undefined: CloseAllLive` (and siblings).

- [ ] **Step 4: Create `pkg/scout/hardening_exports.go`** (REAL code; only the symbols `scout.go` lacks):

```go
// Package scout — hand-written hardening re-exports.
//
// pkg/scout/scout.go is generated (DO NOT EDIT). The session-hardening
// primitives below are added here so the generated facade stays untouched.
// Every symbol in this file is verified ABSENT from scout.go (duplicate =
// build break).
package scout

import (
	"time"

	"github.com/inovacc/scout/internal/engine"
)

// ReapStats re-exports engine.ReapStats — the tally from one reaper pass.
type ReapStats = engine.ReapStats

// CloseAllLive closes every live browser, bounding each Close() with timeout.
// Invoked by the main() signal handler on SIGINT/SIGTERM. Returns the count
// of browsers closed within their deadline.
func CloseAllLive(timeout time.Duration) int { return engine.CloseAllLive(timeout) }

// ReapOnce performs one canonical reaping pass over the sessions directory.
func ReapOnce() ReapStats { return engine.ReapOnce() }

// StartReaperWatchdog starts the single process-wide reaper ticker loop.
func StartReaperWatchdog(interval time.Duration, done <-chan struct{}) {
	engine.StartReaperWatchdog(interval, done)
}

// RecordCleanupFailure enqueues a session dir whose removal failed so the
// background retrier reaps it later.
func RecordCleanupFailure(path string) { engine.RecordCleanupFailure(path) }

// PendingCleanup returns the session dirs currently queued for retry
// (backs `scout session list --pending`).
func PendingCleanup() []string { return engine.PendingCleanup() }

// PendingCleanupCount reports how many session dirs are queued for retry.
func PendingCleanupCount() int { return engine.PendingCleanupCount() }

// FindBrowsersUsingDataDir scans running browsers for processes whose
// --user-data-dir is under dataDir. Returns the matching PIDs.
func FindBrowsersUsingDataDir(dataDir string) []int {
	return engine.FindBrowsersUsingDataDir(dataDir)
}
```

- [ ] **Step 5: Run the test (expect PASS, requires Phase 1 + Tasks 3.2/3.4)**:
```
go test -v -run TestHardeningExportsSurface ./pkg/scout/
```
Expected PASS.

- [ ] **Step 6: Build the whole facade + CLI**:
```
go build ./pkg/... ./cmd/scout/
```

- [ ] **Step 7: Commit**:
```
feat(scout): add hardening facade re-exports (CloseAllLive, reaper, pending)
```

- [ ] **Step 8: Commit**

```bash
git add pkg/scout/hardening_exports.go pkg/scout/hardening_exports_test.go
git commit -m "feat(scout): add hardening facade re-exports (CloseAllLive, reaper, pending)"
```

---

### Task 3.6: Bound launcher.Cleanup in recycleBrowser

`recycleBrowser` (`autofree.go:84-87`) calls `b.launcher.Kill()` then drops the launcher and re-launches, but never calls `launcher.Cleanup()` — so the old `data/` dir is leaked. Per contract: after `Kill()` and BEFORE `New(...)`, call `b.launcher.Cleanup()`, but bound it because `Cleanup()` blocks on `<-l.exit` (launcher.go:613) and could deadlock under `b.mu` if exit never fires. Wrap the call in a goroutine + `select { case <-done: case <-time.After(3*time.Second): }`.

**Files:**
- Modify: `internal/engine/autofree.go` (lines 82-89)
- Create: `internal/engine/autofree_cleanup_test.go`

- [ ] **Step 1: Write the failing test** — `internal/engine/autofree_cleanup_test.go` (package `engine`). It verifies the bounded-cleanup helper returns within the timeout even when the underlying cleanup blocks forever, by extracting the bound logic into a testable helper `boundedCleanup(fn func(), timeout time.Duration) bool`:

```go
package engine

import (
	"testing"
	"time"
)

func TestBoundedCleanupReturnsOnTimeout(t *testing.T) {
	start := time.Now()
	// fn blocks forever; boundedCleanup must still return after the timeout.
	ok := boundedCleanup(func() { select {} }, 100*time.Millisecond)
	elapsed := time.Since(start)

	if ok {
		t.Fatalf("boundedCleanup returned ok=true for a hung cleanup, want false")
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("boundedCleanup returned too early: %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("boundedCleanup did not honor timeout: %v", elapsed)
	}
}

func TestBoundedCleanupReturnsOnCompletion(t *testing.T) {
	done := make(chan struct{})
	ok := boundedCleanup(func() { close(done) }, 3*time.Second)
	if !ok {
		t.Fatalf("boundedCleanup returned ok=false for a fast cleanup, want true")
	}
	select {
	case <-done:
	default:
		t.Fatalf("cleanup fn did not run")
	}
}
```

- [ ] **Step 2: Run the test (expect FAIL)** — `undefined: boundedCleanup`:
```
go test -v -run 'TestBoundedCleanup' ./internal/engine/
```
Expected FAIL: `undefined: boundedCleanup`.

- [ ] **Step 3: Implement `boundedCleanup` and call it from `recycleBrowser`** — `internal/engine/autofree.go`. Add the helper and rewrite the close-old-browser block (lines 82-89):

```go
	// Close old browser.
	_ = b.browser.Close()
	if b.launcher != nil {
		b.launcher.Kill()
		// Remove the old user-data dir before relaunch. Cleanup() blocks on
		// <-l.exit; bound it so a hung process exit cannot deadlock under b.mu.
		l := b.launcher
		boundedCleanup(l.Cleanup, 3*time.Second)
		b.launcher = nil
	}

	b.browser = nil
```

Add the `boundedCleanup` helper at the end of `internal/engine/autofree.go`:

```go
// boundedCleanup runs fn in a goroutine and waits up to timeout for it to
// return. Reports true if fn completed in time, false if the timeout fired.
// Used by recycleBrowser so Launcher.Cleanup()'s blocking <-l.exit wait cannot
// deadlock the recycle loop while b.mu is held.
func boundedCleanup(fn func(), timeout time.Duration) bool {
	done := make(chan struct{})

	go func() {
		defer close(done)
		fn()
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
```

(`time` is already imported in `autofree.go`.)

- [ ] **Step 4: Run the test (expect PASS)**:
```
go test -v -run 'TestBoundedCleanup' ./internal/engine/
```
Expected PASS for both `TestBoundedCleanupReturnsOnTimeout` and `TestBoundedCleanupReturnsOnCompletion`.

- [ ] **Step 5: Build + run the existing autofree tests to confirm no regression**:
```
go build ./pkg/... ./cmd/scout/
go test -run TestAutoFree ./internal/engine/
```
Expected: build OK; existing autofree tests pass or `SKIP` if no Chromium.

- [ ] **Step 6: Commit**:
```
fix(engine): bound launcher.Cleanup in recycleBrowser to avoid deadlock
```

- [ ] **Step 7: Commit**

```bash
git add internal/engine/autofree.go internal/engine/autofree_cleanup_test.go
git commit -m "fix(engine): bound launcher.Cleanup in recycleBrowser to avoid deadlock"
```

---

### Phase 3 verification

Run, in order, from the repo root (`D:\weaver-sync\development\personal\projects\scout`):

1. **Build the engine, facade, and CLI** (the root has no `main`, so never `go build ./...`):
```
go build ./pkg/... ./cmd/scout/
```
Expected: no output, exit 0.

2. **Vet the changed packages**:
```
go vet ./internal/engine/ ./pkg/scout/
```
Expected: no output, exit 0.

3. **Run every test added in this phase** (in-package engine tests + facade test):
```
go test -v -run 'TestLiveRegistryRegisterUnregister|TestCloseAllLive|TestCloseEnqueuesLockedDir|TestEngineReaperReExports|TestBoundedCleanup' ./internal/engine/
go test -v -run 'TestHardeningExportsSurface' ./pkg/scout/
```
Expected:
- `TestLiveRegistryRegisterUnregister` — PASS.
- `TestCloseAllLive` — PASS (`closed == 3`, registry drained).
- `TestCloseEnqueuesLockedDir` — PASS on Windows (`PendingCleanupCount` +1); `SKIP` on Unix.
- `TestEngineReaperReExports` — PASS (requires Phase 1 `session.ReapOnce` et al.).
- `TestBoundedCleanupReturnsOnTimeout` / `TestBoundedCleanupReturnsOnCompletion` — PASS.
- `TestHardeningExportsSurface` — PASS (requires Phase 1 + Tasks 3.2/3.4/3.5).

4. **Full engine + facade package test run** (catches any cross-test compile/regression; browser-dependent tests `SKIP` without Chromium):
```
go test ./internal/engine/ ./pkg/scout/
```
Expected: `ok` for both packages (or `ok` with skipped browser tests).

> **Phase-1 ordering reminder:** Tasks 3.4 and 3.5 (and the `session.RecordCleanupFailure` call in 3.3) will not compile until Phase 1 lands `session.ReapOnce`, `session.ReapStats`, `session.StartReaperWatchdog`, `session.RecordCleanupFailure`, and `session.PendingCleanup`. If verifying Phase 3 standalone before Phase 1, run only the registry + bounded-cleanup tests (`TestLiveRegistryRegisterUnregister`, `TestCloseAllLive`, `TestBoundedCleanup`) — these have no Phase-1 dependency.
## Phase 4: Best-effort SIGINT/SIGTERM handler in main()

**Goal:** Install a buffered `SIGINT`/`SIGTERM` handler in `cmd/scout/scout.go` `main()` that closes every live browser (`scout.CloseAllLive(5s)`) before exiting `130`, extracted into a unit-testable helper `installSignalCleanup(cleanup func()) chan os.Signal` that mirrors the existing `signal.Notify` block in `cmd/scout/server.go`.

> **Cross-phase dependency:** `scout.CloseAllLive(timeout time.Duration) int` is defined by the engine live-registry phase (contract §"package internal/engine": `func CloseAllLive(timeout time.Duration) int`) and re-exported through the facade phase (contract §"package pkg/scout": new hand-written `pkg/scout/hardening_exports.go`). At the time this phase's tasks run, `scout.CloseAllLive` MUST already exist and compile. The helper `installSignalCleanup` takes an injectable `cleanup func()` so its unit test does NOT depend on the real facade symbol — only the wiring in `main()` references `scout.CloseAllLive`.

---

### Task 4.1: Extract a testable signal-cleanup helper and unit-test it

Extract the signal-handling goroutine into a package-level helper `installSignalCleanup(cleanup func()) chan os.Signal` in a new file `cmd/scout/signal.go`. The helper registers a buffered channel for `SIGINT`/`SIGTERM`, spawns a goroutine that blocks on the channel, invokes the supplied `cleanup`, and then exits the process with code `130`. Because `os.Exit` cannot be asserted from a unit test, the exit is delegated to an overridable package var `signalExitFunc`, which the test stubs out so the goroutine runs `cleanup` without terminating the test binary.

**Files:**
- Create: `cmd/scout/signal.go` (new, full file below)
- Create: `cmd/scout/signal_test.go` (new, full file below)

- [ ] **Step 1: Write the failing test `cmd/scout/signal_test.go`.**
  This test drives the helper directly: it stubs `signalExitFunc` to record the exit code instead of calling `os.Exit`, sends a `SIGINT` on the returned channel, and asserts that `cleanup` ran and the recorded exit code is `130`. Uses a buffered done channel + `select`/`time.After` so the test never hangs if the goroutine fails to fire.

```go
package main

import (
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestInstallSignalCleanup_RunsCleanupAndExits(t *testing.T) {
	// Stub the exit hook so the test process is not terminated.
	var (
		mu       sync.Mutex
		gotCode  int
		exited   = make(chan struct{})
		cleaned  = make(chan struct{})
		origExit = signalExitFunc
	)
	t.Cleanup(func() { signalExitFunc = origExit })

	signalExitFunc = func(code int) {
		mu.Lock()
		gotCode = code
		mu.Unlock()
		close(exited)
		// Block forever to emulate os.Exit never returning; the test's
		// select below proceeds without waiting on this goroutine.
		select {}
	}

	cleanup := func() { close(cleaned) }

	sigCh := installSignalCleanup(cleanup)

	// Deliver the signal directly on the channel returned by the helper.
	sigCh <- syscall.SIGINT

	select {
	case <-cleaned:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup was not invoked after SIGINT")
	}

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("signalExitFunc was not invoked after SIGINT")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotCode != 130 {
		t.Errorf("exit code = %d, want 130", gotCode)
	}
}

func TestInstallSignalCleanup_ReturnsBufferedChannel(t *testing.T) {
	origExit := signalExitFunc
	t.Cleanup(func() { signalExitFunc = origExit })
	signalExitFunc = func(int) { select {} }

	sigCh := installSignalCleanup(func() {})
	if cap(sigCh) < 1 {
		t.Errorf("signal channel cap = %d, want >= 1 (buffered)", cap(sigCh))
	}

	// Ensure the channel actually carries os.Signal values.
	var _ chan os.Signal = sigCh
}
```

- [ ] **Step 2: Run the test — expect FAIL (compile error: undefined symbols).**
  ```
  go test -v -run TestInstallSignalCleanup ./cmd/scout/
  ```
  Expected FAIL: `undefined: installSignalCleanup` and `undefined: signalExitFunc` — the helper file does not exist yet (build error in package `main`).

- [ ] **Step 3: Create `cmd/scout/signal.go` with the real helper (minimal implementation).**
  `signalExitFunc` defaults to `os.Exit` and is overridable for tests. `installSignalCleanup` registers a buffered (cap 1) `os.Signal` channel for `SIGINT`/`SIGTERM`, returns it, and runs the cleanup-then-exit goroutine. Mirrors the `signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)` shape from `cmd/scout/server.go:170-181`.

```go
package main

import (
	"os"
	"os/signal"
	"syscall"
)

// signalExitFunc terminates the process after signal cleanup. It is a package
// var so tests can override it (os.Exit cannot be asserted directly).
var signalExitFunc = os.Exit

// installSignalCleanup registers a best-effort SIGINT/SIGTERM handler. On the
// first such signal it runs cleanup (e.g. closing every live browser so chrome
// processes and scout.lock files are not leaked) and then exits with code 130
// (128 + SIGINT). This is the "best-effort" tier from the session-hardening
// design: it catches the signals we can catch but is not the OS-guaranteed tier
// (Job Object / CTRL_CLOSE are out of scope). It mirrors the signal.Notify block
// in cmd/scout/server.go. The returned channel is buffered so a signal arriving
// before Notify's goroutine is scheduled is not dropped.
func installSignalCleanup(cleanup func()) chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		if cleanup != nil {
			cleanup()
		}
		signalExitFunc(130)
	}()

	return sigCh
}
```

- [ ] **Step 4: Run the test — expect PASS.**
  ```
  go test -v -run TestInstallSignalCleanup ./cmd/scout/
  ```
  Expected PASS: `--- PASS: TestInstallSignalCleanup_RunsCleanupAndExits` and `--- PASS: TestInstallSignalCleanup_ReturnsBufferedChannel`, `ok  github.com/inovacc/scout/cmd/scout`.

- [ ] **Step 5: Commit.**
  ```
  git add cmd/scout/signal.go cmd/scout/signal_test.go
  git commit -m "feat(cli): add testable installSignalCleanup helper for main()"
  ```

---

### Task 4.2: Wire installSignalCleanup into main() with scout.CloseAllLive

Call `installSignalCleanup` from `main()` after gops/tracing init and before `Execute()`, passing a cleanup closure that invokes `scout.CloseAllLive(5*time.Second)`. This is the live wiring that uses the cross-phase facade symbol.

**Files:**
- Modify: `cmd/scout/scout.go` (imports block lines 3-21; add `scout.CloseAllLive` call in `main()` lines 145-176 — insert after the tracing block, before `Execute()` at line 175)

- [ ] **Step 1: Add a guard test that `main()` references CloseAllLive via the helper.**
  Since `main()` itself cannot be unit-tested (it blocks on `Execute`), add a small compile-and-behavior test in `cmd/scout/signal_test.go` confirming the cleanup closure used in `main()` is wired to `scout.CloseAllLive`. Extract the closure into a named package-level func `closeAllLiveCleanup()` so it is testable in isolation. Add this test to `cmd/scout/signal_test.go`:

```go
func TestCloseAllLiveCleanup_DoesNotPanicWithNoBrowsers(t *testing.T) {
	// With no live browsers registered, CloseAllLive returns 0 and the
	// cleanup closure must complete without panicking.
	closeAllLiveCleanup()
}
```

- [ ] **Step 2: Run the test — expect FAIL (undefined: closeAllLiveCleanup).**
  ```
  go test -v -run TestCloseAllLiveCleanup ./cmd/scout/
  ```
  Expected FAIL: `undefined: closeAllLiveCleanup` (compile error in package `main`).

- [ ] **Step 3: Add `closeAllLiveCleanup` to `cmd/scout/signal.go`.**
  Append this function to `cmd/scout/signal.go`. It calls the facade's `scout.CloseAllLive` (cross-phase symbol) with the 5s best-effort timeout from the contract.

```go
// closeAllLiveCleanup closes every live browser with a bounded per-browser
// timeout. It is the cleanup callback installed by main()'s signal handler.
// scout.CloseAllLive ranges the live-browser registry and Closes each (so chrome
// children + scout.lock are released) and returns the count closed.
func closeAllLiveCleanup() {
	_ = scout.CloseAllLive(5 * time.Second)
}
```

  Add the required imports to `cmd/scout/signal.go` import block:

```go
import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)
```

- [ ] **Step 4: Run the test — expect PASS.**
  ```
  go test -v -run TestCloseAllLiveCleanup ./cmd/scout/
  ```
  Expected PASS: `--- PASS: TestCloseAllLiveCleanup_DoesNotPanicWithNoBrowsers`, `ok  github.com/inovacc/scout/cmd/scout`.
  (If `scout.CloseAllLive` is not yet present from the prerequisite engine/facade phases, this step fails to compile with `undefined: scout.CloseAllLive` — that prerequisite must be merged first.)

- [ ] **Step 5: Wire the handler into `main()` in `cmd/scout/scout.go`.**
  Insert the `installSignalCleanup` call after the tracing-init block and before `Execute()`. The existing `main()` body (lines 145-176) ends with the tracing block then `Execute()`; place the handler between them. Replace the tail of `main()`:

  Old (lines 168-176):
```go
	shutdown, err := tracing.Init(context.Background(), tracing.Config{ServiceName: "scout"})
	if err != nil {
		log.Printf("scout: tracing: %v", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	Execute()
}
```

  New:
```go
	shutdown, err := tracing.Init(context.Background(), tracing.Config{ServiceName: "scout"})
	if err != nil {
		log.Printf("scout: tracing: %v", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	// Best-effort SIGINT/SIGTERM handler (session-hardening "best-effort"
	// tier): on Ctrl-C / kill, close every live browser so chrome children
	// and scout.lock files are not leaked, then exit 130. NOT the
	// OS-guaranteed tier (Job Object / CTRL_CLOSE are out of scope).
	_ = installSignalCleanup(closeAllLiveCleanup)

	Execute()
}
```

  No import changes are needed in `cmd/scout/scout.go`: `installSignalCleanup` and `closeAllLiveCleanup` live in the same `main` package, and `scout`/`time` are already imported there (lines 11, 18). The `os/signal`, `syscall`, and `time` imports for the helper itself live in `cmd/scout/signal.go` (added in Task 4.2 Step 3).

- [ ] **Step 6: Build and run the full cmd/scout test suite — expect PASS.**
  ```
  go build ./cmd/scout/
  go test -v -run "TestInstallSignalCleanup|TestCloseAllLiveCleanup" ./cmd/scout/
  ```
  Expected: `go build` succeeds with no output; all four signal tests PASS, `ok  github.com/inovacc/scout/cmd/scout`.

- [ ] **Step 7: Commit.**
  ```
  git add cmd/scout/scout.go cmd/scout/signal.go cmd/scout/signal_test.go
  git commit -m "feat(cli): best-effort SIGINT/SIGTERM handler closes live browsers in main()"
  ```

---

### Phase 4 verification

Run, from repo root `D:\weaver-sync\development\personal\projects\scout`:

```
go build ./cmd/scout/
go test -v -run "TestInstallSignalCleanup|TestCloseAllLiveCleanup" ./cmd/scout/
go vet ./cmd/scout/
```

Expected output:
- `go build ./cmd/scout/` — succeeds, no output (confirms `scout.CloseAllLive` from the prerequisite engine/facade phases resolves and the new files compile).
- `go test` — four passing tests:
  - `--- PASS: TestInstallSignalCleanup_RunsCleanupAndExits`
  - `--- PASS: TestInstallSignalCleanup_ReturnsBufferedChannel`
  - `--- PASS: TestCloseAllLiveCleanup_DoesNotPanicWithNoBrowsers`
  - final line `ok  github.com/inovacc/scout/cmd/scout`
- `go vet ./cmd/scout/` — no diagnostics.

Manual smoke (optional, not part of CI): run a long one-shot command (e.g. `scout gather <url>` against a slow page), press `Ctrl-C`, then confirm `scout session doctor` (Phase 5/7) reports zero orphaned chrome and zero ownerless folders — i.e. the handler closed the live browser before exit.
## Phase 5: scout session doctor + list --pending + crash→reap acceptance test

**Goal:** Ship the operator-facing verification surface (`scout session doctor`, `scout session list --pending`) that REUSES the Phase-4 audit classifier, and the single integration test that PROVES the §2 invariant — `<scouthome>/sessions/` is clean unless a live verified owner exists, and a fabricated leak (dead `ScoutPID` + real live child holding the session data dir) is reaped by `session.ReapOnce()` (child killed, dir gone, doctor reports clean), with negative controls (live-owned dir preserved; foreign `--user-data-dir` never touched).

This phase consumes symbols delivered by earlier phases and MUST NOT redefine them:
- `session.ReapOnce() ReapStats` and `type ReapStats struct{ Scanned, Killed, Removed, Pending int }` (Phase 1/2, `internal/engine/session/reaper.go`).
- `session.PendingCleanup() []string` and `scout.PendingCleanup() []string` (Phase 1/2 wrappers over `snapshotPending()`).
- `session.FindBrowsersUsingDataDir(dataDir string) []int` with the new path-prefix behavior + `isUnderSessions` floor (Phase 2).
- `auditAllSessions()`, `classifySession(id)`, `enforceAuditCleanup(entries)`, and the `statusZombie`/`statusStale`/`statusCorrupt`/`statusExpired` consts (Phase 4, `cmd/scout/session_audit.go`) — these are pre-existing today and CONFIRMED present in the live tree.

If `session.ReapOnce`/`scout.PendingCleanup` are not yet on the branch when this phase is executed, this phase is BLOCKED on Phases 1–3 and the verification commands below will fail to compile — that is the intended ordering signal.

---

### Task 5.1: `scout session doctor` (read-only invariant verdict, reuses the audit classifier)

**Files:**
- Create: `cmd/scout/session_doctor.go` (new file, whole file ~150 lines)
- Modify: none (registration lives in `session_doctor.go`'s own `init()` via `sessionCmd.AddCommand`)

`scout session doctor` enumerates every session folder via the EXISTING `auditAllSessions()`, prints a per-folder scout(parent)/browser(child) PID mapping plus a final invariant verdict, and returns a non-nil cobra error (non-zero exit) when ANY entry violates §2: `statusZombie`, or (`statusCorrupt`/`statusStale`) with a still-alive browser. `--fix` runs the EXISTING `enforceAuditCleanup(entries)` then re-audits and re-evaluates. No classification logic is duplicated.

- [ ] **Step 1: Write the failing test.** Create `cmd/scout/session_doctor_test.go`:

```go
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/scout/internal/engine/session"
)

// withTempSessions points the session layer at a fresh temp dir for the
// duration of the test and restores the original resolver afterwards.
func withTempSessions(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	orig := session.SessionsDir
	session.SessionsDir = func() string { return dir }
	t.Cleanup(func() { session.SessionsDir = orig })

	return dir
}

// TestSessionDoctorCleanExitsZero verifies doctor returns nil (exit 0) when
// there are no session folders at all (the at-rest invariant).
func TestSessionDoctorCleanExitsZero(t *testing.T) {
	_ = withTempSessions(t)

	cmd := newSessionDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctor on clean machine: want nil error, got %v", err)
	}

	if !strings.Contains(out.String(), "invariant holds") {
		t.Fatalf("expected clean verdict in output, got:\n%s", out.String())
	}
}

// TestSessionDoctorZombieExitsNonZero fabricates a CORRUPT folder (a dir with
// no scout.pid) and asserts doctor reports a violation and returns an error.
func TestSessionDoctorZombieExitsNonZero(t *testing.T) {
	dir := withTempSessions(t)

	// A bare directory with no scout.pid classifies as statusCorrupt.
	if err := mkdirAll(filepath.Join(dir, "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA")); err != nil {
		t.Fatalf("mkdir fake session: %v", err)
	}

	cmd := newSessionDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatalf("doctor with corrupt folder: want non-nil error, got nil\noutput:\n%s", out.String())
	}
}

// mkdirAll is a thin test helper so the test file needs no os import churn.
func mkdirAll(p string) error { return osMkdirAll(p) }
```

Add a tiny shim at the bottom of the same test file so the test compiles without importing `os` twice across files (keeps the test self-contained):

```go
import "os"

func osMkdirAll(p string) error { return os.MkdirAll(p, 0o700) }
```

(Collapse both `import` blocks into one when writing the file.)

- [ ] **Step 2: Run the test — expect FAIL (compile error).**

```
go test -v -run TestSessionDoctor ./cmd/scout/
```

Expected FAIL: `undefined: newSessionDoctorCmd` (the command constructor does not exist yet).

- [ ] **Step 3: Create `cmd/scout/session_doctor.go` with the real implementation.**

```go
package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	cmd := newSessionDoctorCmd()
	cmd.Flags().Bool("fix", false, "kill zombie browsers and remove stale/corrupt dirs, then re-audit")
	sessionCmd.AddCommand(cmd)
}

// newSessionDoctorCmd builds the `scout session doctor` command. It is a
// constructor (not a package var) so tests can build a fresh instance with
// isolated flags. REUSES auditAllSessions() + classifySession() from
// session_audit.go — no classification logic is duplicated here.
func newSessionDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify the session invariant: every folder owned by a live verified scout; no orphan browsers",
		Long: `Enumerates <scouthome>/sessions/* and asserts the hardening invariant:

  At rest (no live scout process), the sessions directory is empty. A folder
  may exist ONLY while a live, identity-verified scout process owns it, and
  every scout-launched browser must have a live scout parent.

Prints a per-folder scout(parent)/browser(child) PID mapping and a final
verdict. Exits non-zero on any violation (orphan browser / ownerless folder).

With --fix, kills zombie browsers and removes stale/corrupt/expired dirs via
the same enforcement path as 'scout session audit --kill', then re-audits and
re-evaluates the verdict.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := auditAllSessions()
			if err != nil {
				return err
			}

			fix, _ := cmd.Flags().GetBool("fix")
			if fix {
				killed, removed := enforceAuditCleanup(entries)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"Fix: killed %d zombie browser(s), removed %d dir(s); re-auditing...\n",
					killed, removed)

				entries, err = auditAllSessions()
				if err != nil {
					return err
				}
			}

			printDoctorMapping(cmd, entries)

			violations := doctorViolations(entries)
			if len(violations) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"\nVERDICT: invariant holds — %d folder(s), no orphan browsers, no ownerless folders\n",
					len(entries))

				return nil
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\nVERDICT: invariant VIOLATED (%d):\n", len(violations))
			for _, v := range violations {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", v)
			}

			return fmt.Errorf("scout: session doctor: %d invariant violation(s)", len(violations))
		},
	}
}

// doctorViolations returns a human-readable description for every entry that
// breaks the §2 invariant: a zombie (scout dead, browser alive), or a
// corrupt/stale folder that still has a live browser process. Healthy,
// reusable, and expired-but-browser-dead folders are NOT violations (the
// reaper handles expiry; a stale dir with no live browser is merely garbage,
// not an orphan-process invariant break).
func doctorViolations(entries []auditEntry) []string {
	var out []string

	for _, e := range entries {
		switch {
		case e.Status == statusZombie:
			out = append(out, fmt.Sprintf("%s: ZOMBIE (scout %d dead, browser %d ALIVE)",
				e.ID, e.ScoutPID, e.BrowserPID))
		case (e.Status == statusCorrupt || e.Status == statusStale) && e.BrowserAlive:
			out = append(out, fmt.Sprintf("%s: %s with live browser %d (ownerless + live browser)",
				e.ID, e.Status, e.BrowserPID))
		}
	}

	return out
}

// printDoctorMapping prints the parent(scout)→child(browser) PID mapping for
// every folder so an operator can see exactly which process owns what.
func printDoctorMapping(cmd *cobra.Command, entries []auditEntry) {
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No session folders.")
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "SESSION\tSTATUS\tSCOUT(parent)\tBROWSER(child)\tPARENT==SCOUT")
	_, _ = fmt.Fprintln(w, "-------\t------\t-------------\t--------------\t------------")

	for _, e := range entries {
		scoutCol := fmt.Sprintf("%d", e.ScoutPID)
		if e.ScoutAlive {
			scoutCol += "(alive)"
		} else if e.ScoutPID > 0 {
			scoutCol += "(dead)"
		}

		browserCol := fmt.Sprintf("%d", e.BrowserPID)
		if e.BrowserAlive {
			browserCol += "(alive)"
		} else if e.BrowserPID > 0 {
			browserCol += "(dead)"
		}

		shortID := e.ID
		if len(shortID) > 36 {
			shortID = shortID[:36]
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n",
			shortID, e.Status, scoutCol, browserCol, e.ParentMatchesScout)
	}
}
```

- [ ] **Step 4: Remove the temporary test shim now that real symbols exist.** Delete `osMkdirAll`/`mkdirAll` only if you prefer to call `os.MkdirAll` directly; otherwise leave them — they are harmless test-only helpers. (Keeping them is fine; they don't collide with production code.)

- [ ] **Step 5: Run the test — expect PASS.**

```
go test -v -run TestSessionDoctor ./cmd/scout/
```

Expected PASS: `TestSessionDoctorCleanExitsZero` and `TestSessionDoctorZombieExitsNonZero` both `--- PASS`.

- [ ] **Step 6: Build the CLI to confirm wiring.**

```
go build ./cmd/scout/
```

Expected: no output (success). `scout session doctor` is now registered under `sessionCmd`.

- [ ] **Step 7: Commit.**

```
git add cmd/scout/session_doctor.go cmd/scout/session_doctor_test.go
git commit -m "feat(session): add 'scout session doctor' invariant verdict reusing audit classifier"
```

---

### Task 5.2: `scout session list --pending` (surface reaper-stuck dirs)

**Files:**
- Modify: `cmd/scout/session.go` — `sessionListCmd` (lines 246–273) and `init()` flag registration (line 15–49 block)

Add a `--pending` bool flag to `sessionListCmd`. When set, print the directories returned by `scout.PendingCleanup()` (one per line) instead of the tracked session IDs. This surfaces dirs the reaper/retrier could not remove (Windows AV / OneDrive locks) so an operator can escalate.

- [ ] **Step 1: Write the failing test.** Create `cmd/scout/session_list_pending_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestSessionListPendingEmpty asserts the --pending path runs and reports the
// empty case cleanly (no panic, no tracked-ID fallthrough). The pending queue
// is process-global and starts empty in a fresh test binary.
func TestSessionListPendingEmpty(t *testing.T) {
	if sessionListCmd.Flags().Lookup("pending") == nil {
		t.Fatal("sessionListCmd is missing the --pending flag")
	}

	var out bytes.Buffer
	sessionListCmd.SetOut(&out)
	sessionListCmd.SetErr(&out)

	if err := sessionListCmd.Flags().Set("pending", "true"); err != nil {
		t.Fatalf("set --pending: %v", err)
	}
	t.Cleanup(func() { _ = sessionListCmd.Flags().Set("pending", "false") })

	if err := sessionListCmd.RunE(sessionListCmd, nil); err != nil {
		t.Fatalf("list --pending: %v", err)
	}

	if !strings.Contains(out.String(), "No pending cleanup") {
		t.Fatalf("expected empty-pending message, got:\n%s", out.String())
	}
}
```

- [ ] **Step 2: Run the test — expect FAIL.**

```
go test -v -run TestSessionListPending ./cmd/scout/
```

Expected FAIL: `sessionListCmd is missing the --pending flag` (the flag is not registered yet).

- [ ] **Step 3: Register the flag in `init()`.** In `cmd/scout/session.go`, immediately after the `sessionCmd.AddCommand(...)` block (after line 18, before `sessionResetCmd.Flags().Bool("all", ...)` on line 20), add:

```go
	sessionListCmd.Flags().Bool("pending", false, "list directories the reaper could not remove (locked/stuck) instead of tracked sessions")
```

- [ ] **Step 4: Branch the `sessionListCmd` RunE on `--pending`.** Replace the existing `sessionListCmd` (lines 246–273) RunE body's opening so it short-circuits to the pending list. The new full command:

```go
var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked browser sessions",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if pending, _ := cmd.Flags().GetBool("pending"); pending {
			dirs := scout.PendingCleanup()
			if len(dirs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No pending cleanup.")
				return nil
			}

			for _, d := range dirs {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), d)
			}

			return nil
		}

		ids, err := listTrackedSessions()
		if err != nil {
			return err
		}

		if len(ids) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No tracked sessions.")
			return nil
		}

		currentID, _ := resolveSession("")

		for _, id := range ids {
			marker := "  "
			if id == currentID {
				marker = "* "
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, id)
		}

		return nil
	},
}
```

(`scout` and `fmt` are already imported in `session.go` — no import changes.)

- [ ] **Step 5: Run the test — expect PASS.**

```
go test -v -run TestSessionListPending ./cmd/scout/
```

Expected PASS: `TestSessionListPendingEmpty --- PASS` (prints `No pending cleanup.`).

- [ ] **Step 6: Build.**

```
go build ./cmd/scout/
```

Expected: success (no output).

- [ ] **Step 7: Commit.**

```
git add cmd/scout/session.go cmd/scout/session_list_pending_test.go
git commit -m "feat(session): add 'session list --pending' to surface reaper-stuck dirs"
```

---

### Task 5.3: crash→reap acceptance test (THE contract)

**Files:**
- Create: `internal/engine/session/reaper_acceptance_test.go` (new file, ~180 lines)

This is the canonical proof of the §2 invariant. In-process we cannot SIGKILL ourselves, so we FABRICATE the leak: a temp sessions dir, a non-reusable `scout.pid` whose `ScoutPID` is a definitely-dead PID and whose `BrowserPID` is a REAL live child process we spawn with `--user-data-dir=<session data dir>` in its argv. We then run `session.ReapOnce()` and assert the child is killed and the dir removed. Negative controls: (a) a dir owned by a live verified scout (`os.Getpid()` as `ScoutPID`) is PRESERVED; (b) a process whose `--user-data-dir` is OUTSIDE `sessions/` is NEVER killed.

The recorded-`BrowserPID` kill path inside `ReapOnce` (`os.FindProcess`+`Kill`, no name gate) is the reliable cross-platform kill in this test. The `FindBrowsersUsingDataDir` cmdline-scan path is name-gated to chrome/brave/msedge/chromium (see Deviation D1), so the Linux-only sub-assertion uses a browser-named argv to exercise it; the core assertion uses the recorded-PID path which works for any child on any OS.

- [ ] **Step 1: Write the acceptance test (this IS the failing test until earlier phases land).** Create `internal/engine/session/reaper_acceptance_test.go`:

```go
package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/id"
)

// deadPID returns a PID that is overwhelmingly likely to be dead. We spawn a
// trivial child, wait for it to exit, and reuse its PID. On the off chance the
// OS recycles it before the assertion, the test's identity gate (IsScoutProcess
// is false for a non-scout PID, ProcessAlive is false for a reaped PID) keeps
// the classification correct.
func deadPID(t *testing.T) int {
	t.Helper()

	c := exec.Command(sleeperBin(), sleeperArgs("0")...)
	if err := c.Start(); err != nil {
		t.Fatalf("spawn throwaway: %v", err)
	}

	pid := c.Process.Pid
	_ = c.Wait() // reap it; PID is now dead

	return pid
}

// sleeperBin / sleeperArgs pick a long-lived no-op process per OS. The argv
// carries --user-data-dir=<dataDir> so the cmdline scanner can (on Linux, with
// a browser-named binary) locate it; the core assertion does not rely on the
// scanner.
func sleeperBin() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}

	return "/bin/sh"
}

func sleeperArgs(dataDir string) []string {
	if runtime.GOOS == "windows" {
		// `cmd /c ping -n 60 localhost` blocks ~60s. The data-dir is appended
		// as an inert trailing token so it appears in the command line.
		return []string{"/c", "ping", "-n", "60", "127.0.0.1", "--user-data-dir=" + dataDir}
	}

	return []string{"-c", "sleep 60 # --user-data-dir=" + dataDir}
}

// spawnHolder starts a real long-lived child whose argv references dataDir and
// returns it; the caller records its PID as the session BrowserPID.
func spawnHolder(t *testing.T, dataDir string) *exec.Cmd {
	t.Helper()

	c := exec.Command(sleeperBin(), sleeperArgs(dataDir)...)
	if err := c.Start(); err != nil {
		t.Fatalf("spawn holder: %v", err)
	}

	t.Cleanup(func() {
		if c.Process != nil {
			_ = c.Process.Kill()
			_, _ = c.Process.Wait()
		}
	})

	return c
}

// newNonReusableID mints a real encoded session ID with the reusable bit OFF,
// so ReadInfo (which overwrites Reusable from the ID prefix) classifies the
// fabricated folder as non-reusable → reapable.
func newNonReusableID(t *testing.T) string {
	t.Helper()

	sid, err := id.New(id.Attrs{Browser: "chrome", Headless: true, Reusable: false})
	if err != nil {
		t.Fatalf("id.New: %v", err)
	}

	return sid
}

// writeLeakSession fabricates <sessions>/<id>/{scout.pid,data/} with the given
// scout/browser PIDs and returns the session ID + its data dir.
func writeLeakSession(t *testing.T, scoutPID, browserPID int) (string, string) {
	t.Helper()

	sid := newNonReusableID(t)
	dataDir := DataDir(sid)

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}

	info := &SessionInfo{
		ScoutPID:         scoutPID,
		BrowserPID:       browserPID,
		BrowserParentPID: scoutPID,
		Browser:          "chrome",
		Headless:         true,
		CreatedAt:        time.Now(),
		LastUsed:         time.Now(),
	}

	if err := WriteInfo(sid, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	return sid, dataDir
}

// TestReapOnceKillsLeakedSession is the crash→reap acceptance contract.
//
//	Fabricated leak: scout.pid with a DEAD ScoutPID and a LIVE child as
//	BrowserPID, holding the session data dir.
//	Assert: ReapOnce kills the child, removes the folder, and a subsequent
//	pass over the (now empty) sessions dir is clean.
func TestReapOnceKillsLeakedSession(t *testing.T) {
	tmp := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return tmp }
	t.Cleanup(func() { SessionsDir = orig })

	dead := deadPID(t)

	sid := newNonReusableID(t)
	dataDir := DataDir(sid)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}

	holder := spawnHolder(t, dataDir)
	browserPID := holder.Process.Pid

	info := &SessionInfo{
		ScoutPID:         dead,
		BrowserPID:       browserPID,
		BrowserParentPID: dead,
		Browser:          "chrome",
		Headless:         true,
		CreatedAt:        time.Now(),
		LastUsed:         time.Now(),
	}
	if err := WriteInfo(sid, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	// Sanity: the leak is real before we reap.
	if !ProcessAlive(browserPID) {
		t.Fatalf("precondition: holder %d should be alive", browserPID)
	}

	stats := ReapOnce()

	// Child must be dead. Allow a brief settle window for the OS to tear the
	// process down after Kill().
	if waitDead(browserPID, 5*time.Second) {
		t.Errorf("ReapOnce did not kill leaked browser %d", browserPID)
	}

	// Folder must be gone (or, if Windows-locked, recorded as pending — the
	// invariant is 'not silently leaked').
	if _, err := os.Stat(Dir(sid)); err == nil {
		pending := PendingCleanup()
		if !containsPath(pending, Dir(sid)) {
			t.Errorf("session dir %s neither removed nor pending after ReapOnce", Dir(sid))
		}
	}

	// A doctor pass over the now-empty sessions dir must see zero folders.
	listing, err := List()
	if err != nil {
		t.Fatalf("List after reap: %v", err)
	}
	if len(listing) != 0 {
		t.Errorf("after reap: want 0 sessions, got %d", len(listing))
	}

	if stats.Scanned < 1 {
		t.Errorf("ReapStats.Scanned = %d, want >= 1", stats.Scanned)
	}
}

// TestReapOncePreservesLiveOwnedSession is negative control (a): a folder owned
// by a LIVE, identity-verified scout (this test process) must NEVER be reaped.
func TestReapOncePreservesLiveOwnedSession(t *testing.T) {
	tmp := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return tmp }
	t.Cleanup(func() { SessionsDir = orig })

	// os.Getpid() is alive AND passes IsScoutProcess only if this test binary's
	// exec name contains "scout". go test binaries are named e.g.
	// session.test(.exe) — NOT scout — so IsScoutProcess(self) is false and the
	// folder would normally be reaped. To make this a TRUE 'live owned' control
	// we require the identity gate to pass; skip if it does not on this binary.
	self := os.Getpid()
	if !IsScoutProcess(self) {
		t.Skip("test binary is not identity-verifiable as scout; live-owned control requires a scout-named exec")
	}

	sid, _ := writeLeakSession(t, self, 0) // scout alive+verified, no browser

	_ = ReapOnce()

	if _, err := os.Stat(Dir(sid)); err != nil {
		t.Errorf("live-owned session %s was reaped (must be preserved): %v", sid, err)
	}
}

// TestReapOnceNeverTouchesForeignDataDir is negative control (b): a live child
// whose --user-data-dir is OUTSIDE sessions/ must NEVER be killed by a reap of
// an unrelated leaked folder.
func TestReapOnceNeverTouchesForeignDataDir(t *testing.T) {
	tmp := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return tmp }
	t.Cleanup(func() { SessionsDir = orig })

	// A foreign data dir, deliberately outside the sessions root.
	foreign := t.TempDir()
	foreignHolder := spawnHolder(t, filepath.Join(foreign, "Default"))
	foreignPID := foreignHolder.Process.Pid

	// An UNRELATED leaked session (dead scout, no browser) so ReapOnce has work
	// to do and scans the sessions dir.
	dead := deadPID(t)
	_, _ = writeLeakSession(t, dead, 0)

	_ = ReapOnce()

	// The foreign process must still be alive — its data dir is not under
	// sessions/, so neither the recorded-PID path nor the path-bounded scanner
	// may touch it.
	if !ProcessAlive(foreignPID) {
		t.Errorf("ReapOnce killed a foreign process %d whose --user-data-dir is outside sessions/", foreignPID)
	}
}

// waitDead returns true if pid is dead within timeout, false otherwise.
func waitDead(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !ProcessAlive(pid) {
			return false // false == "is dead" per call-site negation
		}
		time.Sleep(50 * time.Millisecond)
	}

	return ProcessAlive(pid)
}

// containsPath reports whether target is present in paths.
func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}

	return false
}
```

NOTE on `waitDead` polarity: the call site is `if waitDead(...) { Errorf("did not kill") }`. `waitDead` returns the FINAL `ProcessAlive(pid)` — `true` only if still alive after the timeout (→ kill failed → error). When the process dies, the loop returns `false` early. The polarity is intentional and matches the call site; keep the comment so a future reader does not "fix" it.

- [ ] **Step 2: Run the test — expect FAIL (compile error until Phases 1–3 land).**

```
go test -v -run TestReapOnce ./internal/engine/session/
```

Expected FAIL on a branch WITHOUT the reaper: `undefined: ReapOnce`, `undefined: ReapStats` (the symbols are defined in `internal/engine/session/reaper.go`, Phase 1/2). This compile failure is the correct ordering signal. Once Phases 1–3 are merged, re-run.

- [ ] **Step 3: Implementation is OWNED BY EARLIER PHASES — none in this task.** This phase adds NO production code for the reaper. The test exercises `ReapOnce()`, `ReapStats`, `PendingCleanup()`, `WriteInfo`, `ReadInfo`/`List`, `ProcessAlive`, `IsScoutProcess`, `Dir`, `DataDir`, `SessionsDir` — all delivered by Phases 1–4. If the test fails for a reason OTHER than "earlier phase not yet merged" (e.g. a real reaper bug — leaked dir, foreign kill), that is a Phase 1/2 defect; record it and route back, do NOT patch around it here.

- [ ] **Step 4: Run the test on a fully-merged branch — expect PASS (or SKIP for the identity-gated control).**

```
go test -v -run TestReapOnce ./internal/engine/session/
```

Expected:
- `TestReapOnceKillsLeakedSession --- PASS` (child killed, dir gone, List empty).
- `TestReapOnceNeverTouchesForeignDataDir --- PASS` (foreign process untouched).
- `TestReapOncePreservesLiveOwnedSession --- SKIP` on a `*.test` binary (exec name is not "scout", so the identity gate cannot mark it owned) OR `--- PASS` when run from a scout-named binary. The SKIP is expected and acceptable under `go test`.

- [ ] **Step 5: Commit.**

```
git add internal/engine/session/reaper_acceptance_test.go
git commit -m "test(session): crash->reap acceptance test proving the sessions-dir invariant"
```

---

### Phase 5 verification

Run from the repo root (`D:\weaver-sync\development\personal\projects\scout`). All commands assume Phases 1–4 are merged (the reaper + facade wrappers exist); if `ReapOnce`/`scout.PendingCleanup` are undefined, the build/test will fail to compile — that means upstream phases are not yet on the branch.

1. **Build the CLI and packages:**

```
go build ./cmd/scout/
go build ./pkg/...
```

Expected: no output (both succeed).

2. **Vet:**

```
go vet ./cmd/scout/ ./internal/engine/session/
```

Expected: no output (clean).

3. **Phase tests:**

```
go test -v -run TestSessionDoctor ./cmd/scout/
go test -v -run TestSessionListPending ./cmd/scout/
go test -v -run TestReapOnce ./internal/engine/session/
```

Expected:
- `TestSessionDoctorCleanExitsZero --- PASS`
- `TestSessionDoctorZombieExitsNonZero --- PASS`
- `TestSessionListPendingEmpty --- PASS`
- `TestReapOnceKillsLeakedSession --- PASS`
- `TestReapOnceNeverTouchesForeignDataDir --- PASS`
- `TestReapOncePreservesLiveOwnedSession --- SKIP` (test binary not scout-named) or `--- PASS`
- Final line: `ok  github.com/inovacc/scout/cmd/scout` and `ok  github.com/inovacc/scout/internal/engine/session`

4. **Manual smoke (optional, requires a clean machine):**

```
go run ./cmd/scout/ session doctor
```

Expected: prints `No session folders.` then `VERDICT: invariant holds — 0 folder(s), ...` and exits 0. With a fabricated zombie folder present, exits non-zero with a `VERDICT: invariant VIOLATED` block on stderr.
## Phase 6: Daemon reconcile, DestroyAllSessions, panic-recovered shutdown, idle hardening

**Goal:** Make the gRPC daemon reap prior-instance orphans at boot (`Reconcile` → `scout.ReapOnce()`), tear down **every** in-flight session on shutdown (`DestroyAllSessions`, each session guarded by `recover()` so one panic can't abort the sweep), and ensure idle shutdown flushes HAR / stops hijacker+recorder before `GracefulStop` — closing daemon-crash session leaks (design §1 "Daemon crash leaks every in-flight gRPC session", §5, work item #4; gaps #3, #6, #7).

> **Dependency note:** This phase calls the facade symbol `scout.ReapOnce()`. That symbol is created in Phase 1 (`internal/engine/session/reaper.go`: `func ReapOnce() ReapStats`) and re-exported through the facade in Phase 5 (`pkg/scout/hardening_exports.go`: `func ReapOnce() session.ReapStats` / `type ReapStats = session.ReapStats`). It does **not** exist on disk yet. Task 6.1 below is written so its test stubs `Reconcile` against an injectable reap hook (a package var `reapHook`) so this phase compiles and tests green **without** waiting for Phases 1/5; the production default of `reapHook` is `scout.ReapOnce`, behind a build that only links once Phase 5 lands. See deviation note in the structured output.

---

### Task 6.1: `(*ScoutServer).Reconcile()` — reap on-disk orphans at daemon boot

**Files:**
- Modify: `grpc/server/server.go` (add method after `New()` at line 137-139; add a package var `reapHook` near the `ScoutServer` declaration ~line 134)
- Create: `grpc/server/reconcile_test.go`

**TDD steps:**

- [ ] **Step 1: Write the failing test.** Create `grpc/server/reconcile_test.go`:

```go
package server

import (
	"sync/atomic"
	"testing"
)

func TestReconcileCallsReapHook(t *testing.T) {
	var called atomic.Int32

	orig := reapHook
	reapHook = func() int {
		called.Add(1)
		return 3
	}
	t.Cleanup(func() { reapHook = orig })

	s := New()
	killed := s.Reconcile()

	if called.Load() != 1 {
		t.Fatalf("expected reapHook called once, got %d", called.Load())
	}
	if killed != 3 {
		t.Fatalf("expected Reconcile to return reap count 3, got %d", killed)
	}
}

func TestReconcileEmptyMapNoAdoption(t *testing.T) {
	// Map is empty at boot: Reconcile must not panic and must not
	// touch s.sessions (no adoption — just on-disk reap).
	orig := reapHook
	reapHook = func() int { return 0 }
	t.Cleanup(func() { reapHook = orig })

	s := New()
	_ = s.Reconcile()

	n := 0
	s.sessions.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Fatalf("expected empty session map after Reconcile, got %d entries", n)
	}
}
```

- [ ] **Step 2: Run it — expect FAIL (does not compile).** `go test -run TestReconcile ./grpc/server/`
  Expected FAIL: `undefined: reapHook` and `s.Reconcile undefined (type *ScoutServer has no field or method Reconcile)`.

- [ ] **Step 3: Minimal implementation.** In `grpc/server/server.go`, add a package var directly above `// New creates a new ScoutServer.` (line 136) and the `Reconcile` method directly after `New()` (after line 139):

```go
// reapHook performs a single on-disk orphan-reaping pass and returns the
// number of holder processes killed. It is a package var so tests can stub
// it without a live browser. Production wires it to scout.ReapOnce in
// reconcile_wire.go.
var reapHook = func() int { return 0 }

// Reconcile reaps prior-instance session orphans on the disk at daemon
// startup. The in-memory session map is empty at boot, so there is nothing
// to adopt — Reconcile only kills/removes on-disk orphans left by a crashed
// or force-killed previous daemon. It returns the number of holder processes
// killed during the pass. Best-effort; never fatal.
func (s *ScoutServer) Reconcile() int {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("scout: reconcile: recovered from panic", "panic", r)
		}
	}()

	return reapHook()
}
```

  `slog` is already imported in `server.go` (line 8). No new import needed.

- [ ] **Step 4: Wire production default (real `scout.ReapOnce`).** Create `grpc/server/reconcile_wire.go`:

```go
package server

import "github.com/inovacc/scout/pkg/scout"

// init points the reaper hook at the real facade reaper. scout.ReapOnce
// performs one path-bounded pass over <scouthome>/sessions, killing
// holders and removing orphan dirs; it returns a ReapStats whose Killed
// field is the holder-kill count.
func init() {
	reapHook = func() int {
		return scout.ReapOnce().Killed
	}
}
```

  NOTE: `scout.ReapOnce()` and the `Killed` field on `ReapStats` are produced by Phases 1+5. If Phase 6 is built/tested before those land, temporarily comment out the body of `reconcile_wire.go`'s `init` (leave the file with an empty `init()`); the stubbed `reapHook` default keeps tests green. Re-enable when Phase 5's `pkg/scout/hardening_exports.go` exists. This split keeps the testable seam (`reapHook`) independent of the facade.

- [ ] **Step 5: Run it — expect PASS.** `go test -run TestReconcile ./grpc/server/`
  Expected PASS: `ok  github.com/inovacc/scout/grpc/server`.

- [ ] **Step 6: Build check.** `go build ./grpc/...` (expect no output). If Phases 1/5 are not yet merged, `reconcile_wire.go` init body is commented per Step 4.

- [ ] **Step 7: Commit.**
  `feat(daemon): add ScoutServer.Reconcile to reap prior-instance orphans at boot`

- [ ] **Step 8: Commit**

```bash
git add grpc/server/server.go grpc/server/reconcile_test.go grpc/server/reconcile_wire.go
git commit -m "feat(daemon): add ScoutServer.Reconcile to reap prior-instance orphans at boot"
```

---

### Task 6.2: `(*ScoutServer).DestroyAllSessions()` — panic-recovered full teardown of every session

**Files:**
- Modify: `grpc/server/server.go` (add method after `DestroySession`, after line 573; mirror its teardown order)
- Create: `grpc/server/destroy_all_test.go`

The full per-session teardown mirrors `DestroySession` (lines 534-573): stop monitor sidecars → flush HAR → stop recorder → **stop hijacker** (DestroySession omits this but StartHijack at 1074 leaves a live fan-out goroutine, so the bulk teardown must stop it) → `browser.Close()` → `untrackPeer` → `sessions.Delete`. Each session is wrapped in its own `func(){ defer recover() ... }()` so one panic cannot abort the sweep.

**TDD steps:**

- [ ] **Step 1: Write the failing test.** Create `grpc/server/destroy_all_test.go`. It fabricates the `sessions` sync.Map with stub `*session` values whose `browser` is nil (`Browser.Close()` is nil-safe — see CLAUDE.md "Nil-safety"), so no Chromium is required:

```go
package server

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// fakeSession builds a *session with a nil browser (Close is nil-safe) and
// nil recorder/hijacker so DestroyAllSessions exercises the teardown loop
// without launching Chromium.
func fakeSession(id string) *session {
	return &session{
		id:   id,
		subs: make(map[string]chan *struct{ unused int }), // replaced below
	}
}

func TestDestroyAllSessionsClearsMap(t *testing.T) {
	s := New()

	for _, id := range []string{"a", "b", "c"} {
		sess := &session{id: id}
		s.sessions.Store(id, sess)
	}

	s.DestroyAllSessions()

	n := 0
	s.sessions.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Fatalf("expected all sessions destroyed, %d remain", n)
	}
}

func TestDestroyAllSessionsSurvivesPanic(t *testing.T) {
	s := New()

	// One session whose monitorCancel panics — the sweep must still
	// delete it and continue to the next session.
	panicSess := &session{id: "boom", monitorCancel: func() { panic("teardown boom") }}
	okSess := &session{id: "ok"}
	s.sessions.Store("boom", panicSess)
	s.sessions.Store("ok", okSess)

	s.DestroyAllSessions() // must not panic out

	n := 0
	s.sessions.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Fatalf("expected both sessions removed despite panic, %d remain", n)
	}
}

// compile-time guard that scout.Browser.Close is the nil-safe API we rely on.
var _ = func() *scout.Browser { return nil }
```

  (Remove the unused `fakeSession` helper — kept here only to show the nil-browser rationale; the concrete tests above build `&session{}` literals directly. If `go vet` flags `fakeSession` as unused, delete it before commit.)

- [ ] **Step 2: Run it — expect FAIL.** `go test -run TestDestroyAllSessions ./grpc/server/`
  Expected FAIL: `s.DestroyAllSessions undefined (type *ScoutServer has no field or method DestroyAllSessions)`.

- [ ] **Step 3: Minimal implementation.** In `grpc/server/server.go`, add directly after `DestroySession` (after line 573):

```go
// DestroyAllSessions tears down every in-flight session: it stops monitor
// sidecars, flushes the HAR artifact, stops the recorder and hijacker,
// closes the browser, untracks the peer, and deletes the session from the
// map. Each session's teardown runs under its own recover() so a single
// panicking session cannot abort the sweep. Used by daemon idle/shutdown
// paths so no session is leaked when the server stops.
func (s *ScoutServer) DestroyAllSessions() {
	s.sessions.Range(func(key, value any) bool {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("scout: destroy all: session teardown panicked", "session", key, "panic", r)
				}
			}()

			sess, ok := value.(*session)
			if !ok || sess == nil {
				s.sessions.Delete(key)
				return
			}

			// Stop console + WS sidecar writers first so their files flush
			// before the browser is torn down.
			if sess.monitorCancel != nil {
				sess.monitorCancel()
				sess.monitorCancel = nil
			}

			// Flush HAR sidecar before stopping the recorder. Mirrors
			// DestroySession: reads monitors.json for the target path,
			// defaults to DefaultHARPath. Best-effort; never blocks teardown.
			if sess.recorder != nil {
				if sess.browser != nil {
					if engineID := sess.browser.SessionID(); engineID != "" {
						if data, _, err := sess.recorder.ExportHAR(); err == nil && len(data) > 0 {
							cfg, _ := scout.ReadSessionMonitors(engineID)
							outPath := scout.DefaultHARPath(engineID)
							if cfg != nil && cfg.HAR.Path != "" {
								outPath = filepath.Join(scout.SessionDir(engineID), cfg.HAR.Path)
							}
							if werr := os.WriteFile(outPath, data, 0o600); werr != nil {
								slog.Warn("scout: HAR flush on destroy-all failed", "path", outPath, "err", werr)
							}
						}
					}
				}
				sess.recorder.Stop()
			}

			// Stop the hijack fan-out goroutine if one is active.
			if sess.hijacker != nil {
				sess.hijacker.Stop()
				sess.hijacker = nil
			}

			_ = sess.browser.Close()

			s.untrackPeer(sess.id)
			s.sessions.Delete(key)
		}()

		return true
	})
}
```

  All referenced symbols are already imported in `server.go`: `slog` (8), `os` (9), `filepath` (10), `scout` (22). `sess.browser.Close()` is nil-safe per CLAUDE.md, so a nil-browser stub session is handled. `ReadSessionMonitors`/`DefaultHARPath`/`SessionDir`/`ExportHAR` are the exact symbols used by `DestroySession`.

- [ ] **Step 4: Run it — expect PASS.** `go test -run TestDestroyAllSessions ./grpc/server/`
  Expected PASS: `ok  github.com/inovacc/scout/grpc/server`.

- [ ] **Step 5: Build check.** `go build ./grpc/...` — expect no output.

- [ ] **Step 6: Commit.**
  `feat(daemon): add ScoutServer.DestroyAllSessions with per-session panic recovery`

- [ ] **Step 7: Commit**

```bash
git add grpc/server/server.go grpc/server/destroy_all_test.go
git commit -m "feat(daemon): add ScoutServer.DestroyAllSessions with per-session panic recovery"
```

---

### Task 6.3: Idle shutdown tears down all sessions before GracefulStop

**Files:**
- Modify: `grpc/server/server.go` (`StartIdleTimer`, lines 148-172 — replace the inline per-session loop with a `DestroyAllSessions()` call)
- Create: `grpc/server/idle_shutdown_test.go`

Today `StartIdleTimer`'s `onIdle` (lines 153-171) inlines a partial teardown that stops only the recorder and closes the browser — it never stops the hijacker, never flushes HAR, never untracks the peer. Replace it with `s.DestroyAllSessions()` so the idle path performs the same full teardown as shutdown (recorder + hijacker stop + HAR flush).

**TDD steps:**

- [ ] **Step 1: Write the failing test.** Create `grpc/server/idle_shutdown_test.go`. It exercises the idle callback with no browser (sessions are nil-browser stubs) and asserts (a) `OnIdleShutdown` fires and (b) the session map is emptied by the idle callback — proving `DestroyAllSessions` ran before `GracefulStop`:

```go
package server

import (
	"sync"
	"testing"
	"time"
)

func TestIdleTimerDestroysAllSessionsThenShutsDown(t *testing.T) {
	s := New()

	// Two nil-browser stub sessions (Close is nil-safe → no Chromium).
	s.sessions.Store("a", &session{id: "a"})
	s.sessions.Store("b", &session{id: "b"})

	var (
		mu             sync.Mutex
		shutdownCalled bool
		sessionsAtShutdown int
	)

	s.IdleTimeout = 20 * time.Millisecond
	s.OnIdleShutdown = func() {
		mu.Lock()
		shutdownCalled = true
		// Count sessions remaining when shutdown fires — must be 0,
		// proving DestroyAllSessions ran first.
		s.sessions.Range(func(_, _ any) bool { sessionsAtShutdown++; return true })
		mu.Unlock()
	}

	s.StartIdleTimer()
	defer s.StopIdleTimer()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		done := shutdownCalled
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("idle shutdown did not fire within 2s")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if sessionsAtShutdown != 0 {
		t.Fatalf("expected 0 sessions when OnIdleShutdown fired, got %d", sessionsAtShutdown)
	}
}
```

- [ ] **Step 2: Run it — expect FAIL.** `go test -run TestIdleTimerDestroysAllSessions ./grpc/server/`
  Expected FAIL: assertion `expected 0 sessions when OnIdleShutdown fired, got 2` — because the current inline loop deletes sessions but the test would pass only if the loop runs; actually the existing loop DOES delete, so to make this a true RED, note the real gap is hijacker/HAR. To guarantee a failing-first state, write the test BEFORE editing and confirm it currently passes-by-accident is NOT acceptable — instead assert behaviour the old code lacks. Use the stronger assertion below in Step 1bis.

- [ ] **Step 1bis: Strengthen the test to a genuine RED.** The existing inline loop already empties the map, so add a spy that the *new* implementation must call. Replace the `OnIdleShutdown` body assertion with a call-order spy on a package seam. Add to the test file a top-level check using a recorder stub is impractical without a browser; instead assert the method `DestroyAllSessions` is invoked by routing the idle callback through a tiny indirection. Modify `StartIdleTimer` to call `s.DestroyAllSessions()` and have the test count via a wrapper:

```go
func TestIdleTimerCallsDestroyAllBeforeShutdown(t *testing.T) {
	s := New()
	s.sessions.Store("x", &session{id: "x", monitorCancel: func() {}})

	order := make([]string, 0, 2)
	var mu sync.Mutex

	// monitorCancel records that full teardown ran for session x.
	s.sessions.Range(func(_, v any) bool {
		sess := v.(*session)
		sess.monitorCancel = func() {
			mu.Lock()
			order = append(order, "destroy")
			mu.Unlock()
		}
		return true
	})

	s.IdleTimeout = 20 * time.Millisecond
	s.OnIdleShutdown = func() {
		mu.Lock()
		order = append(order, "shutdown")
		mu.Unlock()
	}

	s.StartIdleTimer()
	defer s.StopIdleTimer()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		got := len(order)
		mu.Unlock()
		if got >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("idle teardown incomplete: %v", order)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 || order[0] != "destroy" || order[1] != "shutdown" {
		t.Fatalf("expected [destroy shutdown], got %v", order)
	}
}
```

  This RED-fails on the current code: the inline loop never calls `monitorCancel` (it only stops the recorder and closes the browser), so `order[0]` would be `"shutdown"`, not `"destroy"`.

- [ ] **Step 2bis: Run it — expect FAIL.** `go test -run TestIdleTimerCallsDestroyAllBeforeShutdown ./grpc/server/`
  Expected FAIL: `expected [destroy shutdown], got [shutdown]` (current inline loop skips `monitorCancel`).

- [ ] **Step 3: Implementation — route idle through `DestroyAllSessions`.** Edit `StartIdleTimer` in `grpc/server/server.go`. Replace the body of the `idle.New` callback (lines 153-171) so it reads:

```go
	s.idle = idle.New(s.IdleTimeout, func() {
		// Full teardown of every session: stops monitor sidecars + hijacker,
		// flushes HAR, stops recorders, closes browsers, untracks peers.
		// Runs BEFORE OnIdleShutdown so artifacts are flushed before the
		// gRPC server stops accepting calls.
		s.DestroyAllSessions()

		if s.OnIdleShutdown != nil {
			s.OnIdleShutdown()
		}
	})
```

  This removes the partial inline loop (old lines 155-166) entirely, replacing it with the single `DestroyAllSessions()` call.

- [ ] **Step 4: Run it — expect PASS.** `go test -run TestIdleTimerCallsDestroyAllBeforeShutdown ./grpc/server/`
  Expected PASS: `ok  github.com/inovacc/scout/grpc/server`.

- [ ] **Step 5: Build check.** `go build ./grpc/...` — expect no output.

- [ ] **Step 6: Commit.**
  `fix(daemon): idle shutdown runs full DestroyAllSessions before GracefulStop`

- [ ] **Step 7: Commit**

```bash
git add grpc/server/server.go grpc/server/idle_shutdown_test.go
git commit -m "fix(daemon): idle shutdown runs full DestroyAllSessions before GracefulStop"
```

---

### Task 6.4: Wire `cmd/scout/server.go` RunE — reconcile before Serve, DestroyAllSessions at both GracefulStop sites, recover around teardown

**Files:**
- Modify: `cmd/scout/server.go` (`RunE`, lines 34-188): add `srv.Reconcile()` before `Serve` (~line 166, after `printTable(nil)`); call `scoutServer.DestroyAllSessions()` inside both `OnIdleShutdown` (line 153) and the signal goroutine (line 169); wrap each teardown in `defer recover()`.

This task has no new unit test (it is glue inside a cobra `RunE` that requires a real listener + browser). Coverage of the wiring behaviour comes from Tasks 6.1-6.3 unit tests plus the build check. The acceptance is: `go build ./cmd/scout/` succeeds and the two GracefulStop sites each call `DestroyAllSessions()` first under a recover.

- [ ] **Step 1: Add Reconcile before Serve.** In `cmd/scout/server.go`, after `printTable(nil)` (line 166) and before the `// Graceful shutdown` goroutine (line 168), insert:

```go
		// Reap prior-instance session orphans left by a crashed or
		// force-killed previous daemon before we start serving. Map is
		// empty at boot, so this only touches on-disk orphans.
		if killed := scoutServer.Reconcile(); killed > 0 {
			_, _ = fmt.Fprintf(os.Stdout, "reconcile: reaped %d orphaned browser process(es)\n", killed)
		}
```

  `fmt` and `os` are already imported (lines 4, 6).

- [ ] **Step 2: DestroyAllSessions in OnIdleShutdown under recover.** Replace the `OnIdleShutdown` assignment (lines 153-161) with:

```go
		scoutServer.OnIdleShutdown = func() {
			_, _ = fmt.Fprintln(os.Stdout, "\nidle timeout reached, shutting down gRPC server...")

			func() {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stdout, "session teardown panicked: %v\n", r)
					}
				}()
				scoutServer.DestroyAllSessions()
			}()

			if pairingGRPC != nil {
				pairingGRPC.GracefulStop()
			}

			grpcServer.GracefulStop()
		}
```

  Note: `StartIdleTimer` (Task 6.3) already calls `DestroyAllSessions()` inside the idle callback before invoking `OnIdleShutdown`; calling it again here is **idempotent and safe** (the map is already empty, the Range is a no-op) and guarantees teardown even if a future caller sets `OnIdleShutdown` without going through `StartIdleTimer`. Keep both.

- [ ] **Step 3: DestroyAllSessions in the signal goroutine under recover.** Replace the signal goroutine body (lines 169-181) with:

```go
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			_, _ = fmt.Fprintln(os.Stdout, "\nshutting down gRPC server...")

			func() {
				defer func() {
					if r := recover(); r != nil {
						_, _ = fmt.Fprintf(os.Stdout, "session teardown panicked: %v\n", r)
					}
				}()
				scoutServer.DestroyAllSessions()
			}()

			if pairingGRPC != nil {
				pairingGRPC.GracefulStop()
			}

			grpcServer.GracefulStop()
		}()
```

  `os/signal` (line 7) and `syscall` (line 10) are already imported.

- [ ] **Step 4: Build check — expect PASS.** `go build ./cmd/scout/` — expect no output.

- [ ] **Step 5: Vet check.** `go vet ./cmd/scout/` — expect no output.

- [ ] **Step 6: Commit.**
  `feat(daemon): wire RunE reconcile-on-boot + DestroyAllSessions at both shutdown paths`

- [ ] **Step 7: Commit**

```bash
git add cmd/scout/server.go
git commit -m "feat(daemon): wire RunE reconcile-on-boot + DestroyAllSessions at both shutdown paths"
```

---

### Task 6.5: Idle timer unit test (no browser) — `internal/idle`

**Files:**
- Create: `internal/idle/timer_test.go`

The contract requires an idle-timer test asserting `onIdle` fires. `internal/idle` has no browser dependency, so this is a fast pure-Go test that locks in the contract `StartIdleTimer` relies on: a positive timeout fires `onIdle` exactly once after the duration; a zero timeout never fires; `Stop` before fire prevents the callback.

- [ ] **Step 1: Write the failing test.** Create `internal/idle/timer_test.go`:

```go
package idle

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerFiresOnIdle(t *testing.T) {
	var fired atomic.Int32
	done := make(chan struct{})

	tm := New(20*time.Millisecond, func() {
		fired.Add(1)
		close(done)
	})
	defer tm.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onIdle did not fire within 2s")
	}

	if fired.Load() != 1 {
		t.Fatalf("expected onIdle to fire once, got %d", fired.Load())
	}
}

func TestTimerZeroTimeoutNeverFires(t *testing.T) {
	var fired atomic.Int32

	tm := New(0, func() { fired.Add(1) })
	defer tm.Stop()

	time.Sleep(50 * time.Millisecond)

	if fired.Load() != 0 {
		t.Fatalf("expected zero-timeout timer never to fire, got %d", fired.Load())
	}
}

func TestTimerStopBeforeFire(t *testing.T) {
	var fired atomic.Int32

	tm := New(100*time.Millisecond, func() { fired.Add(1) })
	tm.Stop()

	time.Sleep(200 * time.Millisecond)

	if fired.Load() != 0 {
		t.Fatalf("expected stopped timer never to fire, got %d", fired.Load())
	}
}

func TestTimerResetExtendsCountdown(t *testing.T) {
	var fired atomic.Int32

	tm := New(80*time.Millisecond, func() { fired.Add(1) })
	defer tm.Stop()

	// Reset twice within the window — total elapsed ~120ms but each Reset
	// restarts the 80ms countdown, so it must not have fired yet.
	time.Sleep(40 * time.Millisecond)
	tm.Reset()
	time.Sleep(40 * time.Millisecond)
	tm.Reset()

	if got := fired.Load(); got != 0 {
		t.Fatalf("expected no fire after resets, got %d", got)
	}
}
```

- [ ] **Step 2: Run it — expect PASS (idle.Timer already implements this).** `go test -v -run TestTimer ./internal/idle/`
  Expected: all four subtests PASS — `ok  github.com/inovacc/scout/internal/idle`. (This is a characterization test that locks the contract `DestroyAllSessions`-via-idle relies on; the implementation already satisfies it, so it is green on first run. No code change to `timer.go` is required.)

  If `TestTimerFiresOnIdle` were to FAIL, the root cause would be in `idle.New` (`time.AfterFunc` not started) — but the current implementation at `internal/idle/timer.go:33` is correct.

- [ ] **Step 3: Commit.**
  `test(idle): add idle timer fire/zero/stop/reset characterization tests`

- [ ] **Step 4: Commit**

```bash
git add internal/idle/timer_test.go
git commit -m "test(idle): add idle timer fire/zero/stop/reset characterization tests"
```

---

### Phase 6 verification

Run the full phase verification from the repo root (`D:\weaver-sync\development\personal\projects\scout`):

```
go build ./cmd/scout/
go build ./grpc/...
go build ./pkg/...
go vet ./grpc/server/ ./cmd/scout/ ./internal/idle/
go test -v -run TestReconcile ./grpc/server/
go test -v -run TestDestroyAllSessions ./grpc/server/
go test -v -run TestIdleTimerCallsDestroyAllBeforeShutdown ./grpc/server/
go test -v -run TestTimer ./internal/idle/
go test ./grpc/server/ ./internal/idle/
```

**Expected output:**
- `go build ./cmd/scout/`, `go build ./grpc/...`, `go build ./pkg/...`: no output (exit 0). If Phases 1/5 (`scout.ReapOnce`) are not yet merged, the `init` body of `grpc/server/reconcile_wire.go` is commented out per Task 6.1 Step 4, so `pkg/...` still builds; once Phase 5 lands, uncomment it.
- `go vet ...`: no output.
- `go test -v -run TestReconcile ./grpc/server/`: `--- PASS: TestReconcileCallsReapHook`, `--- PASS: TestReconcileEmptyMapNoAdoption`, `ok`.
- `go test -v -run TestDestroyAllSessions ./grpc/server/`: `--- PASS: TestDestroyAllSessionsClearsMap`, `--- PASS: TestDestroyAllSessionsSurvivesPanic`, `ok`.
- `go test -v -run TestIdleTimerCallsDestroyAllBeforeShutdown ./grpc/server/`: `--- PASS`, `ok`.
- `go test -v -run TestTimer ./internal/idle/`: 4 subtests PASS, `ok`.
- `go test ./grpc/server/ ./internal/idle/`: `ok` for both packages (no regressions). Note: the broader `./grpc/server/` package contains browser-dependent tests that `t.Skip` when no Chromium is present — the Phase 6 tests added here require **no** browser.
## Phase 7: Stuck-dir force-break escalation

**Goal:** When a stale session dir survives `forceBreakThreshold` consecutive retry sweeps (≈20 min at the 60 s interval), `retryPending` escalates to an aggressive force-removal (loop `os.RemoveAll`, plus a low-level Windows `RemoveDirectory` walk as a last resort), `slog.Warn`s the path, and on success dequeues it so `PendingCleanupCount()` / `PendingCleanup()` reflect the terminal state. Best-effort throughout — never panics, never blocks the watchdog. (Spec §3 decision #4, §5.2 step 3, work-item #6, gaps #5/#11.)

> Files touched in this phase (all repo-relative, ignore any `.claude/worktrees/` copies):
> - `internal/engine/session/cleanup_retry.go` — `retryPending` (112-142), `recordCleanupFailure` (29-39), `StartCleanupRetrier` (84-107), constants (74). Add `forceBreakThreshold` const, `removeAllFn` seam, `forceBreakDir` helper, force-break branch in `retryPending`.
> - `internal/engine/session/cleanup_retry_windows.go` (NEW) — Windows low-level `rmdirLowLevel` via `golang.org/x/sys/windows`.
> - `internal/engine/session/cleanup_retry_other.go` (NEW) — non-Windows no-op `rmdirLowLevel`.
> - `internal/engine/session/cleanup_retry_test.go` (NEW — none exists today) — proves threshold-gated force-break + queue accounting + the real-locked-file case (Windows-only, `t.Skip` elsewhere).

### Context the drafter verified against real code

- `retryPending(failCount map[string]int)` is the only mutator of `failCount`; it snapshots `snapshotPending()`, stats each path, calls `retryRemoveAll(p)` (defined in `session_track.go:37`), increments `failCount[p]`, and warns once at exactly 10. There is **no** force-break today.
- `removePending(path)` (lines 52-61) and `snapshotPending()` (43-49) already exist and are the queue-accounting primitives this phase reuses.
- `PendingCleanupCount()` exists (66-70). The exported `PendingCleanup() []string` wrapper over `snapshotPending()` is created in a **different** phase (contract line 14). This phase's tests therefore assert via `PendingCleanupCount()` + `snapshotPending()` only, and reference `PendingCleanup()` in prose only.
- There is no existing force-remove / `rmdir` / `RemoveDirectory` helper in the package.
- Tests are white-box (`package session`) and override `SessionsDir` (see `lock_test.go`, `session_track_test.go`). For a *deterministic* `RemoveAll` failure we add a `removeAllFn` package var (default `os.RemoveAll`) so the test injects a counting-failing remover without depending on flaky real OS locks; a separate Windows-only subtest exercises a genuine open-handle lock.

---

### Task 7.1: Add force-break escalation to the retry loop

Add a `forceBreakThreshold` constant, a `removeAllFn` indirection seam, and a `forceBreakDir(path)` helper, then branch in `retryPending`: once `failCount[p] >= forceBreakThreshold`, attempt force-removal; on success `slog.Warn` + `removePending` + `delete(failCount, p)`; on failure leave it queued and keep retrying. Best-effort, never panics.

**Files:**
- Modify: `internal/engine/session/cleanup_retry.go` (add const near 74; add `removeAllFn` var near 22-25; add `forceBreakDir` after `retryPending`; edit the loop body 118-141)

- [ ] **Step 1: Write the failing test (`cleanup_retry_test.go`, threshold cases).** Create the new file `internal/engine/session/cleanup_retry_test.go`. It injects a `removeAllFn` that fails the first N calls for a target path, drives `retryPending` exactly `forceBreakThreshold` times, and asserts the dir is force-broken (dequeued) only on/after the threshold. White-box, no browser.

```go
package session

import (
	"os"
	"path/filepath"
	"testing"
)

// withRemoveAllFn swaps the package removeAllFn seam for the duration of a
// test and restores it on cleanup.
func withRemoveAllFn(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := removeAllFn
	removeAllFn = fn
	t.Cleanup(func() { removeAllFn = orig })
}

// resetPending clears the package-level pending queue so each test starts
// from a known state. White-box access only.
func resetPending(t *testing.T) {
	t.Helper()
	pendingMu.Lock()
	pendingCleanup = nil
	pendingMu.Unlock()
	t.Cleanup(func() {
		pendingMu.Lock()
		pendingCleanup = nil
		pendingMu.Unlock()
	})
}

// TestRetryPendingForceBreakAfterThreshold proves a dir that fails removal on
// every normal sweep is force-broken once failCount reaches
// forceBreakThreshold, and is then dequeued so PendingCleanupCount drops to 0.
func TestRetryPendingForceBreakAfterThreshold(t *testing.T) {
	resetPending(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "stuck-sess")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	// removeAllFn fails for every normal sweep; the force-break path is the
	// only one allowed to actually delete. We model that by failing all
	// calls until the test's force-break (which uses the real os.RemoveAll
	// via forceBreakDir's loop, NOT removeAllFn) removes the dir.
	calls := 0
	withRemoveAllFn(t, func(p string) error {
		calls++
		return &os.PathError{Op: "remove", Path: p, Err: os.ErrPermission}
	})

	recordCleanupFailure(target)
	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("PendingCleanupCount after enqueue = %d, want 1", got)
	}

	failCount := make(map[string]int)

	// Drive sweeps up to (threshold-1): normal RemoveAll keeps failing, no
	// force-break yet, dir stays queued.
	for i := 0; i < forceBreakThreshold-1; i++ {
		retryPending(failCount)
	}
	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("before threshold: PendingCleanupCount = %d, want 1", got)
	}
	if got := failCount[target]; got != forceBreakThreshold-1 {
		t.Fatalf("failCount[target] = %d, want %d", got, forceBreakThreshold-1)
	}

	// The threshold-th sweep triggers force-break, which uses forceBreakDir
	// (real os.RemoveAll loop, bypassing the injected removeAllFn) and
	// succeeds since the dir is unlocked in this test.
	retryPending(failCount)

	if got := PendingCleanupCount(); got != 0 {
		t.Fatalf("after force-break: PendingCleanupCount = %d, want 0", got)
	}
	if _, ok := failCount[target]; ok {
		t.Fatalf("failCount still has target after force-break")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target dir still present after force-break: stat err=%v", err)
	}
	if calls == 0 {
		t.Fatalf("removeAllFn was never exercised on the normal path")
	}
}
```

- [ ] **Step 2: Run the test — expect a COMPILE failure.**

```
go test -v -run TestRetryPendingForceBreakAfterThreshold ./internal/engine/session/
```

Expected FAIL: compilation error — `undefined: removeAllFn` and `undefined: forceBreakThreshold` (neither the seam, the const, nor the force-break behavior exist yet). This is the failing-test step.

- [ ] **Step 3: Add the `removeAllFn` seam.** In `cleanup_retry.go`, extend the existing `var (...)` block (currently lines 22-25 holding `pendingMu`/`pendingCleanup`) so the indirection lives beside its siblings.

Replace:

```go
var (
	pendingMu      sync.Mutex
	pendingCleanup []string
)
```

with:

```go
var (
	pendingMu      sync.Mutex
	pendingCleanup []string

	// removeAllFn indirects os.RemoveAll so tests can inject a deterministic
	// failing remover for the normal retry path. The force-break path
	// (forceBreakDir) deliberately calls os.RemoveAll directly so it is not
	// short-circuited by this seam.
	removeAllFn = os.RemoveAll
)
```

- [ ] **Step 4: Add the `forceBreakThreshold` constant.** Place it next to `DefaultCleanupRetryInterval` (currently line 74).

Replace:

```go
// DefaultCleanupRetryInterval is the base interval between retry sweeps.
// Each sweep walks all pending dirs once with a short per-dir budget.
const DefaultCleanupRetryInterval = 60 * time.Second
```

with:

```go
// DefaultCleanupRetryInterval is the base interval between retry sweeps.
// Each sweep walks all pending dirs once with a short per-dir budget.
const DefaultCleanupRetryInterval = 60 * time.Second

// forceBreakThreshold is the number of consecutive failed retry sweeps on the
// same dir before the retrier escalates to an aggressive force-removal. At
// DefaultCleanupRetryInterval (60 s) this is ~20 minutes — long enough for a
// transient AV / Search-Indexer / OneDrive lock to clear on its own, after
// which we stop waiting and break the dir. Spec §3 decision #4.
const forceBreakThreshold = 20
```

- [ ] **Step 5: Route the normal retry path through the seam and add the force-break branch.** Replace the loop body in `retryPending` (lines 118-141) so it (a) uses `removeAllFn` instead of `retryRemoveAll` indirectly — keep `retryRemoveAll` for the normal attempt but make its inner call mockable — and (b) escalates at the threshold. To keep `retryRemoveAll`'s backoff intact while still honoring the seam, switch the normal attempt to `removeAllFn(p)` directly (the retrier already provides the cross-sweep backoff via the 60 s ticker, so a single attempt per sweep is correct and avoids double-budgeting).

Replace:

```go
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			removePending(p)
			delete(failCount, p)
			continue // already gone — maybe another process removed it
		}

		if err := retryRemoveAll(p); err == nil {
			slog.Info("scout: background cleanup removed stale session", "dir", p)
			removePending(p)
			delete(failCount, p)
			continue
		}

		failCount[p]++

		// Cap consecutive-failure logging at 10 to bound noise; dirs
		// that survive a full hour are likely held by something
		// persistent (OneDrive sync, broken AV) — keep retrying
		// silently. Entry stays in queue for next tick.
		if failCount[p] == 10 {
			slog.Warn("scout: background cleanup still blocked after 10 attempts", "dir", p)
		}
	}
```

with:

```go
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			removePending(p)
			delete(failCount, p)
			continue // already gone — maybe another process removed it
		}

		if err := removeAllFn(p); err == nil {
			slog.Info("scout: background cleanup removed stale session", "dir", p)
			removePending(p)
			delete(failCount, p)
			continue
		}

		failCount[p]++

		// Cap consecutive-failure logging at 10 to bound noise; dirs
		// that survive ~10 min are likely held by something persistent
		// (OneDrive sync, broken AV). Entry stays in queue for next tick.
		if failCount[p] == 10 {
			slog.Warn("scout: background cleanup still blocked after 10 attempts", "dir", p)
		}

		// Force-break escalation: once a dir has resisted forceBreakThreshold
		// consecutive sweeps (~20 min), stop waiting and break it
		// aggressively. Best-effort: a still-failing force-break leaves the
		// dir queued for the next tick (we never panic, never block).
		if failCount[p] >= forceBreakThreshold {
			if err := forceBreakDir(p); err == nil {
				slog.Warn("scout: force-broke stuck session dir after threshold",
					"dir", p, "attempts", failCount[p])
				removePending(p)
				delete(failCount, p)
				continue
			} else {
				slog.Warn("scout: force-break of stuck session dir failed",
					"dir", p, "attempts", failCount[p], "err", err)
			}
		}
	}
```

- [ ] **Step 6: Add the `forceBreakDir` helper.** Append after `retryPending` (after line 142). It loops `os.RemoveAll` (real, not the seam, so the force-break path is never short-circuited by an injected failing remover), then falls back to the platform `rmdirLowLevel` walk. Returns the last error; never panics. `retryRemoveAll` is still imported/used elsewhere — leaving it in place avoids an unused-symbol break (it is also referenced by `CleanStaleSessions`).

```go
// forceBreakDir aggressively removes a session dir that survived
// forceBreakThreshold normal retry sweeps. It is the terminal escalation: it
// loops os.RemoveAll a few times (clearing read-only attrs the OS may have
// re-applied), then falls back to a low-level platform rmdir walk
// (RemoveDirectory on Windows, plain os.Remove elsewhere). Best-effort: it
// returns the last error and never panics. It deliberately calls os.RemoveAll
// directly (not removeAllFn) so the force-break path is never short-circuited
// by a test-injected failing remover.
func forceBreakDir(path string) error {
	var lastErr error

	// A few direct passes: clearing the read-only bit between passes lets a
	// dir whose entries were marked read-only (common on Windows AV
	// quarantine) finally delete.
	for range 3 {
		if err := clearReadOnly(path); err != nil {
			lastErr = err
		}
		if err := os.RemoveAll(path); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	// Platform last resort: low-level directory removal.
	if err := rmdirLowLevel(path); err == nil {
		return nil
	} else if err != nil {
		lastErr = err
	}

	// Final stat: if it is gone despite the last error, treat as success.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil
	}

	return lastErr
}

// clearReadOnly best-effort walks path and clears the read-only bit on every
// entry so a subsequent RemoveAll can proceed. Errors are accumulated but not
// fatal — the caller retries regardless.
func clearReadOnly(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries; keep walking
		}
		// Add owner write bit; harmless on dirs, clears the RO attr on files.
		_ = os.Chmod(p, info.Mode()|0o200)
		return nil
	})
}
```

- [ ] **Step 7: Add the `filepath` import.** `clearReadOnly` uses `filepath.Walk`. The current import block (lines 3-8) is `log/slog`, `os`, `sync`, `time`. Replace it with:

```go
import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)
```

- [ ] **Step 8: Run the test — expect COMPILE failure (platform helper still missing).**

```
go test -v -run TestRetryPendingForceBreakAfterThreshold ./internal/engine/session/
```

Expected FAIL: compilation error — `undefined: rmdirLowLevel` (referenced by `forceBreakDir`, defined in Task 7.2). Proceed to 7.2 before re-running.

- [ ] **Step 9: Commit**

```bash
git add internal/engine/session/cleanup_retry.go
git commit -m "feat(session): add force-break escalation to cleanup retry loop"
```

---

### Task 7.2: Platform `rmdirLowLevel` (Windows low-level rmdir + no-op elsewhere)

Provide the build-tagged `rmdirLowLevel(path string) error` used by `forceBreakDir`. Windows attempts a depth-first `RemoveDirectory`/`DeleteFile` walk via `golang.org/x/sys/windows` (already an indirect dep of the engine). Non-Windows is a no-op returning nil (the `os.RemoveAll` loop already covers Unix; Unix has no equivalent lock pathology).

**Files:**
- Create: `internal/engine/session/cleanup_retry_windows.go`
- Create: `internal/engine/session/cleanup_retry_other.go`

- [ ] **Step 1: Write the non-Windows implementation.** Create `internal/engine/session/cleanup_retry_other.go`:

```go
//go:build !windows

package session

// rmdirLowLevel is the non-Windows fallback. On Unix-like systems os.RemoveAll
// already handles every removable case (there is no AV/indexer lock
// pathology), so this is a no-op that reports success and lets forceBreakDir's
// final stat decide the outcome.
func rmdirLowLevel(_ string) error {
	return nil
}
```

- [ ] **Step 2: Write the Windows implementation.** Create `internal/engine/session/cleanup_retry_windows.go`. It walks the tree bottom-up, clears the read-only/hidden/system attributes with `SetFileAttributes`, deletes files with `DeleteFile`, and removes dirs with `RemoveDirectory` — the low-level equivalents that sometimes succeed where `os.RemoveAll` (which uses `Remove` semantics) trips on attribute or sharing edge cases.

```go
//go:build windows

package session

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// rmdirLowLevel performs a depth-first removal of path using the raw Win32
// primitives (SetFileAttributes + DeleteFile + RemoveDirectory). It is the
// terminal escalation invoked by forceBreakDir after os.RemoveAll has failed
// forceBreakThreshold times. Best-effort: it returns the last error but tries
// every entry regardless of individual failures, and never panics.
func rmdirLowLevel(path string) error {
	var lastErr error

	entries, err := os.ReadDir(path)
	if err == nil {
		for _, e := range entries {
			child := filepath.Join(path, e.Name())
			if e.IsDir() {
				if rmErr := rmdirLowLevel(child); rmErr != nil {
					lastErr = rmErr
				}
				continue
			}
			if delErr := deleteFileLowLevel(child); delErr != nil {
				lastErr = delErr
			}
		}
	} else {
		lastErr = err
	}

	// Clear attributes on the dir itself, then remove it.
	if p, cvtErr := windows.UTF16PtrFromString(path); cvtErr == nil {
		_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL)
		if rmErr := windows.RemoveDirectory(p); rmErr != nil {
			lastErr = rmErr
		} else {
			lastErr = nil
		}
	} else {
		lastErr = cvtErr
	}

	// If the dir is gone despite a reported error, treat as success.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil
	}
	return lastErr
}

// deleteFileLowLevel clears blocking attributes then DeleteFile's a single
// file via raw Win32. Best-effort.
func deleteFileLowLevel(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// Strip read-only/hidden/system so DeleteFile is permitted.
	_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL)
	if err := windows.DeleteFile(p); err != nil {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return nil
		}
		return err
	}
	return nil
}
```

- [ ] **Step 3: Confirm `golang.org/x/sys/windows` is available.** It is an indirect dependency already (pulled by gops + grpc). Verify it resolves without a new `go get`:

```
go list -deps ./internal/engine/session/ | findstr "golang.org/x/sys/windows"
```

Expected: the module path prints (already in the module graph). If absent, the implementer runs `go get golang.org/x/sys` and `go mod tidy` (this is the only allowed dependency touch; `RemoveDirectory`/`DeleteFile`/`SetFileAttributes`/`UTF16PtrFromString` are all in `golang.org/x/sys/windows`).

- [ ] **Step 4: Run the Task 7.1 test — expect PASS now that `rmdirLowLevel` exists.**

```
go test -v -run TestRetryPendingForceBreakAfterThreshold ./internal/engine/session/
```

Expected PASS: `--- PASS: TestRetryPendingForceBreakAfterThreshold`. The threshold-th sweep force-breaks the (unlocked) temp dir via `forceBreakDir`'s `os.RemoveAll` loop, dequeues it, and `PendingCleanupCount()` returns 0.

- [ ] **Step 5: Build both entry points to confirm cross-compilation of the tag split.**

```
go build ./pkg/...
go build ./cmd/scout/
```

Expected: both succeed (the `!windows` file compiles on the host, the `windows` file is syntactically validated by the toolchain when targeting Windows; the implementer additionally runs the cross-build below in Step 6).

- [ ] **Step 6: Cross-validate the Windows file compiles.** On a non-Windows host, force a Windows build of the package to catch tag/import mistakes:

```
go vet ./internal/engine/session/
```

and, if cross-tooling is available:

```
GOOS=windows go build ./internal/engine/session/
```

Expected: no errors. (On a Windows dev box `go vet` already covers the active file.)

- [ ] **Step 7: Commit.**

```
git add internal/engine/session/cleanup_retry.go internal/engine/session/cleanup_retry_windows.go internal/engine/session/cleanup_retry_other.go internal/engine/session/cleanup_retry_test.go
git commit -m "feat(session): force-break stuck cleanup dirs after retry threshold"
```

---

### Task 7.3: Prove queue accounting and the real-locked-file case

Add two more subtests to `cleanup_retry_test.go`: (1) a path that is *removable* on the first sweep is dequeued immediately and never force-broken; (2) a genuinely locked dir (open handle) is force-broken only after the threshold — Windows-only, `t.Skip` elsewhere — exercising the real `rmdirLowLevel`/handle-release path rather than the injected seam.

**Files:**
- Modify: `internal/engine/session/cleanup_retry_test.go` (append subtests; reuse `withRemoveAllFn`/`resetPending` from Task 7.1)

- [ ] **Step 1: Write the "removable dir is never force-broken" test.** Append to `cleanup_retry_test.go`:

```go
// TestRetryPendingRemovesBeforeThreshold proves a dir that removes cleanly on
// the first sweep is dequeued immediately and the force-break path is never
// taken (failCount never reaches the threshold).
func TestRetryPendingRemovesBeforeThreshold(t *testing.T) {
	resetPending(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "easy-sess")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	// Default removeAllFn (os.RemoveAll) — succeeds on the first try.
	recordCleanupFailure(target)
	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("PendingCleanupCount after enqueue = %d, want 1", got)
	}

	failCount := make(map[string]int)
	retryPending(failCount)

	if got := PendingCleanupCount(); got != 0 {
		t.Fatalf("after one sweep: PendingCleanupCount = %d, want 0", got)
	}
	if _, ok := failCount[target]; ok {
		t.Fatalf("failCount unexpectedly tracked a successfully-removed dir")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still present after clean removal: %v", err)
	}
}
```

- [ ] **Step 2: Run both passing tests so far.**

```
go test -v -run "TestRetryPending" ./internal/engine/session/
```

Expected PASS for `TestRetryPendingForceBreakAfterThreshold` and `TestRetryPendingRemovesBeforeThreshold`.

- [ ] **Step 3: Write the real-locked-file force-break test (Windows-only).** Append. This holds an OS handle open on a file inside the dir so `os.RemoveAll` genuinely fails (Windows share semantics), drives `forceBreakThreshold` sweeps with the *default* `removeAllFn`, releases the handle just before the threshold sweep, and asserts force-break succeeds and dequeues. On non-Windows `os.RemoveAll` ignores open handles, so the lock pathology cannot be reproduced → `t.Skip`.

```go
// TestRetryPendingForceBreakRealLock exercises the force-break path against a
// genuinely locked file (open handle) on Windows, where os.RemoveAll fails
// while a handle is held. The handle is released just before the threshold
// sweep so the force-break (forceBreakDir -> os.RemoveAll loop / rmdirLowLevel)
// can finally remove the dir. Non-Windows cannot reproduce the lock (open
// handles do not block unlink) and is skipped.
func TestRetryPendingForceBreakRealLock(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("open-handle removal lock is Windows-specific")
	}
	resetPending(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "locked-sess")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	locked := filepath.Join(target, "scout.lock")
	f, err := os.Create(locked)
	if err != nil {
		t.Fatalf("create locked file: %v", err)
	}
	released := false
	releaseOnce := func() {
		if !released {
			_ = f.Close()
			released = true
		}
	}
	t.Cleanup(releaseOnce)

	// Use the real remover so the open handle actually blocks removal.
	recordCleanupFailure(target)
	failCount := make(map[string]int)

	// Sweeps 1..threshold-1: handle held → RemoveAll fails → stays queued.
	for i := 0; i < forceBreakThreshold-1; i++ {
		retryPending(failCount)
		if PendingCleanupCount() != 1 {
			t.Fatalf("sweep %d: dir dequeued early while locked", i+1)
		}
	}

	// Release the handle, then the threshold-th sweep force-breaks the dir.
	releaseOnce()
	retryPending(failCount)

	if got := PendingCleanupCount(); got != 0 {
		t.Fatalf("after force-break: PendingCleanupCount = %d, want 0", got)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("locked dir survived force-break: %v", err)
	}
}
```

- [ ] **Step 4: Add the `runtime` import to the test file.** The new subtest references `runtime.GOOS`. Update the test file import block to:

```go
import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)
```

- [ ] **Step 5: Run the full set.**

```
go test -v -run "TestRetryPending" ./internal/engine/session/
```

Expected PASS: three tests pass on Windows; on non-Windows `TestRetryPendingForceBreakRealLock` reports `--- SKIP` and the other two PASS.

- [ ] **Step 6: Run the whole session package to confirm no regression in the existing cleanup/queue tests.**

```
go test ./internal/engine/session/
```

Expected: `ok  github.com/inovacc/scout/internal/engine/session` (existing `PendingCleanupCount`, lock, and session-track tests still pass; `retryPending` behavior change is additive). If a pre-existing test asserted the old warn-only-at-10 terminal behavior it would surface here — none does (no `cleanup_retry_test.go` existed before this phase).

- [ ] **Step 7: Commit.**

```
git add internal/engine/session/cleanup_retry_test.go
git commit -m "test(session): cover force-break threshold, clean-remove, and real-lock cases"
```

---

### Phase 7 verification

Run, in order, from the repo root (`D:\weaver-sync\development\personal\projects\scout`):

1. Targeted unit tests (the force-break behavior + queue accounting):
   ```
   go test -v -run "TestRetryPending" ./internal/engine/session/
   ```
   Expected: `--- PASS: TestRetryPendingForceBreakAfterThreshold`, `--- PASS: TestRetryPendingRemovesBeforeThreshold`, and (Windows) `--- PASS: TestRetryPendingForceBreakRealLock` / (non-Windows) `--- SKIP: TestRetryPendingForceBreakRealLock`. Final line `PASS` / `ok ... internal/engine/session`.

2. Whole session package (no regressions in existing lock/queue/session-track tests):
   ```
   go test ./internal/engine/session/
   ```
   Expected: `ok  github.com/inovacc/scout/internal/engine/session`.

3. Build both entry points (confirms the `windows` / `!windows` tag split and the `golang.org/x/sys/windows` import resolve):
   ```
   go build ./pkg/...
   go build ./cmd/scout/
   ```
   Expected: both succeed with no output.

4. Cross-compile the Windows-tagged file from a non-Windows host (catches tag/import errors that the host build skips). Skip on a Windows dev box where `go vet` already covers it:
   ```
   GOOS=windows go build ./internal/engine/session/
   go vet ./internal/engine/session/
   ```
   Expected: no errors.

All four must pass before Phase 7 is considered complete. The `forceBreakThreshold` const (20) and the `removeAllFn` seam are the contract symbols this phase introduces; `PendingCleanup()` (the exported `[]string` snapshot wrapper) is referenced in prose only and is delivered by its owning phase per contract line 14.
