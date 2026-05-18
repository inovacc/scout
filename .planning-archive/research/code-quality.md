# Code Quality Research: Scout Codebase

**Researched:** 2026-03-29
**Domain:** Code quality hotspots, dead code, panic sites, pattern consistency
**Confidence:** HIGH (direct codebase analysis)

## Summary

The Scout codebase has 128K+ lines of Go (non-test). The primary quality concerns are:
(1) a 1267-line `must.go` where 124 of 134 Must* methods have zero callers outside the file,
(2) 15 panic sites in non-test code (mostly inherited from rod internalization),
(3) exact-duplicate helper functions between `recipe.go` and `runbook.go`,
(4) browser detection logic duplicated across `internal/engine/browser/` and `pkg/scout/browser/`,
(5) inconsistent error handling (657 non-prefixed `fmt.Errorf` calls vs the `"scout: ..."` convention),
and (6) several exported functions with zero external callers.

**Primary recommendation:** Prioritize removing dead Must* methods (saves ~1100 lines), deduplicating recipe/runbook helpers, and converting panics to errors in reachable code paths.

---

## 1. Large Files (>300 lines, excluding tests/generated/proto)

### Critical (>800 lines) -- Scout-authored, splittable

| File | Lines | Assessment |
|------|-------|------------|
| `grpc/server/server.go` | 1398 | **Split by domain.** Already organized with section comments (Session Lifecycle, Navigation, Element Interaction, Query, Capture, Recording, Hijacking, Profile). Each section is a natural file boundary. |
| `internal/engine/must.go` | 1267 | **124 of 134 methods unused.** Delete unused methods (see Section 6). Remaining ~150 lines can stay. |
| `internal/engine/browser/download.go` | 1120 | Split: Chrome/Brave/Edge downloads into separate files + shared `DownloadFile` helper. |
| `internal/engine/page_rod.go` | 1103 | Low-level rod page wrappers. Could split into page_rod_navigation.go, page_rod_eval.go, page_rod_capture.go. |
| `internal/engine/github.go` | 1006 | GitHub automation. Split: auth, repo ops, search, API helpers. |
| `cmd/scout/bridge.go` | 993 | Single `init()` registering many subcommands. Split: bridge_server.go, bridge_client.go, bridge_commands.go. |
| `pkg/scout/scraper/modes/cloud/cloud.go` | 888 | Cloud scraper mode. |
| `internal/engine/browser.go` | 850 | Core Browser type. Could split lifecycle vs configuration. |
| `internal/engine/page.go` | 822 | Core Page type. Could split navigation vs extraction. |
| `pkg/scout/scraper/modes/jira/jira.go` | 814 | SaaS scraper -- acceptable size for self-contained mode. |

### Large but acceptable (500-800 lines, self-contained)

Most scraper modes (salesforce, outlook, grafana, twitter, etc.) are 500-800 lines each. These are self-contained implementations and splitting would reduce cohesion. No action needed.

### Large CLI files (>450 lines)

| File | Lines | Issue |
|------|-------|-------|
| `cmd/scout/recipe.go` | 664 | **Duplicates runbook.go helpers** (see Section 5) |
| `cmd/scout/llm.go` | 662 | Contains `createProvider` marked `//nolint:unused` -- dead code |
| `cmd/scout/runbook.go` | 644 | **Duplicates recipe.go helpers** (see Section 5) |
| `cmd/scout/plugin.go` | 536 | Reasonable for plugin management complexity |
| `cmd/scout/swarm.go` | 527 | Reasonable |
| `cmd/scout/profile.go` | 492 | Reasonable |
| `cmd/scout/client.go` | 487 | Contains `executeREPLCommand` marked `//nolint:maintidx` |
| `cmd/scout/session.go` | 486 | Reasonable |

---

## 2. Panic Sites (ALL 15 in non-test code)

### Inherited from Rod (internal/engine/lib/)

These are from the internalized rod fork. They use `utils.Panic` and `utils.E()` which call `panic()`.

| File:Line | Panic | Reachable from gRPC/MCP/CLI? |
|-----------|-------|------------------------------|
| `lib/utils/utils.go:68` | `var Panic = func(v any) { panic(v) }` | **YES** -- called by `utils.E()` everywhere |
| `lib/utils/utils.go:222` | `panic("all jobs are already done")` | LOW -- internal job queue |
| `lib/utils/utils.go:332-335` | `panic(err)` / `panic(fmt.Sprintf(...))` | YES -- goroutine recovery |
| `lib/cdp/websocket.go:34` | `panic("duplicated connection: " + wsURL)` | MEDIUM -- CDP connect |
| `lib/input/keyboard.go:61` | `panic("key not defined")` | YES -- from `PressKey` gRPC/MCP |
| `lib/launcher/flags/flags.go:66` | `panic("flag name should not contain '='")` | LOW -- init-time |
| `lib/launcher/manager.go:101` | `panic("Must be used with launcher.NewManaged")` | MEDIUM -- launcher config |
| `lib/launcher/manager.go:156` | `panic(http.ErrAbortHandler)` | YES -- HTTP handler abort (idiomatic Go) |
| `lib/defaults/defaults.go:93` | `panic(msg)` | LOW -- env parsing at startup |
| `lib/defaults/defaults.go:201` | `panic("unknown scout env option: " + n)` | LOW -- env parsing at startup |

### Scout-Original

| File:Line | Panic | Reachable? |
|-----------|-------|------------|
| `internal/engine/browser_rod.go:160` | `panic("Browser.Client and Browser.ControlURL can't be set at the same time")` | YES -- misconfiguration |
| `internal/engine/browser_rod.go:380` | `panic("can't use wait function twice")` | YES -- user API misuse |
| `internal/engine/browser/manifest.go:76` | `panic(fmt.Sprintf("browser: parse browser.json: %v", err))` | YES -- browser cache corruption |
| `pkg/scout/identity/identity.go:97` | `panic(fmt.Sprintf("identity: luhnify: %v", err))` | LOW -- identity generation |

### `utils.E()` Callers (indirect panics)

`utils.E(err)` calls `utils.Panic(err)` on non-nil error. Found **30+ call sites** in non-test code:

- `internal/engine/dev_helpers.go` -- 8 calls (ServeMonitor, debug HTTP handlers)
- `internal/engine/lib/cdp/client.go` -- 4 calls (CDP JSON-RPC)
- `internal/engine/lib/cdp/utils.go` -- 2 calls (WebSocket connect)
- `internal/engine/lib/launcher/browser.go` -- 1 call
- `internal/engine/lib/launcher/manager.go` -- 5 calls (browser manager HTTP)
- `internal/engine/lib/launcher/url_parser.go` -- 4 calls (URL parsing)
- `internal/engine/stealth/generate/main.go` -- 2 calls (code gen, not runtime)

**Risk assessment:** The highest-risk panics are in `keyboard.go:61` (reachable from gRPC `PressKey`), `browser_rod.go:160,380` (reachable from any browser creation), and `manifest.go:76` (reachable from browser download). These can crash the gRPC server or CLI.

**Recommendation:** Convert Scout-original panics to `error` returns. For rod-inherited panics, add `recover()` at gRPC/MCP handler boundaries.

---

## 3. Dead Code

### Confirmed Dead: Must* Methods (124 of 134)

Only **10 Must* methods** have callers outside `must.go`:

| Method | Callers | Where |
|--------|---------|-------|
| `MustClick` | 1 | dev_helpers.go |
| `MustElement` | 1 | dev_helpers.go |
| `MustEval` | 7 | dev_helpers.go, stealth |
| `MustEvaluate` | 1 | dev_helpers.go |
| `MustHandle` | 1 | dev_helpers.go |
| `MustHandleDialog` | 1 | (internal) |
| `MustInfo` | 3 | dev_helpers.go |
| `MustInput` | 1 | (internal) |
| `MustPage` | 1 | dev_helpers.go |
| `MustPageFromTargetID` | 1 | dev_helpers.go |
| `MustScreenshot` | 1 | dev_helpers.go |
| `MustStart` | 1 | (internal) |

**124 methods are completely unused** -- approximately 1100 lines of dead code. These are rod API convenience wrappers that were never adopted by Scout's non-Must API surface.

### Confirmed Dead: Exported Functions

| Function | File | Evidence |
|----------|------|----------|
| `FingerprintToProfile` | `internal/engine/fingerprint.go:43` | Only caller is its own test |
| `createProvider` | `cmd/scout/llm.go:607` | Explicitly marked `//nolint:unused`, has "backward compatibility" comment but no callers |

### Suspected Dead (1 non-definition caller each, likely only re-export)

| Function | File | Caller |
|----------|------|--------|
| `NewBridgeFallback` | `internal/engine/bridge_fallback.go:21` | Only re-exported in `pkg/scout/scout.go:368`, no external consumer found |
| `NewBridgeServer` | `internal/engine/bridge_ws.go:68` | Used in cmd/scout, likely alive |

### Commented-Out Code

Minimal. Most `//` prefixed code found is documentation comments, not commented-out logic. The codebase is clean in this regard.

---

## 4. Inconsistent Patterns

### Error Handling

**Convention (from CLAUDE.md):** `fmt.Errorf("scout: action: %w", err)`

**Reality:**
- Files using `"scout: ..."` prefix: ~40 files in `cmd/scout/` follow the convention well
- Files **not** using prefix: **657 `fmt.Errorf` calls** across the codebase lack the `"scout:"` prefix
- Worst offenders: `internal/engine/lib/` (inherited rod code never prefixed), `grpc/server/server.go`, scraper modes
- `errors.New` used in 10 files (minor)

### Logging

Four different logging mechanisms coexist:

| Pattern | Files Using | Where |
|---------|-------------|-------|
| `slog.*` | 19 files | CLI, gRPC, newer code -- **preferred** |
| `fmt.Fprint(os.Stderr, ...)` | 20 files | CLI commands, ad-hoc |
| `log.Printf` | 3 files | Legacy/transitional |
| `utils.Log` | 4 files | Rod-inherited (internal/engine/lib/) |

**Recommendation:** Consolidate to `slog` only. Replace `fmt.Fprint(os.Stderr)` with `slog.Warn`/`slog.Info`.

### Option Pattern

The functional options pattern is used consistently (`type XOption func(*xOptions)` with `WithX()` constructors). Found **20+ option types** all following the same pattern. This is well-standardized.

One outlier: `lib/utils/imageutil.go` uses `type ImgOption struct` (struct, not function) -- acceptable since it's inherited rod code.

---

## 5. Duplicate Code

### Exact Duplicates: recipe.go / runbook.go

Two pairs of **byte-for-byte identical functions** (different names only):

```
cmd/scout/recipe.go:563  -> applyVars()
cmd/scout/runbook.go:543 -> applyRunbookVars()

cmd/scout/recipe.go:576  -> findUnresolvedVars()
cmd/scout/runbook.go:556 -> findUnresolvedRunbookVars()
```

Both operate on `*runbook.Runbook` with identical logic. Should extract to a shared helper in `cmd/scout/helpers.go` or into the `runbook` package itself.

### Overlapping: Browser Detection

Two separate browser detection implementations:

| Package | Functions | Lines |
|---------|-----------|-------|
| `internal/engine/browser/detect.go` | `DetectBrowsers`, `BestDetected`, `BestCached`, `ParseBrowserVersion` | 124 |
| `pkg/scout/browser/detect.go` + platform files | `Detect`, `DetectByType`, `Best`, `ParseVersion` | 258 |

Both packages also have overlapping download logic:
- `internal/engine/browser/download.go` (1120 lines): `DownloadChrome`, `DownloadBrave`, `DownloadEdge`
- `pkg/scout/browser/download.go` (317 lines): `DownloadChrome`, `DownloadBrave`, `DownloadEdge`

**The `pkg/scout/browser/` package appears to be a public API wrapper**, but the function signatures differ (e.g., `pkg` version takes `cacheDir string` param while `internal` version uses global config). This creates maintenance burden -- changes must be made in two places.

**Recommendation:** Have `pkg/scout/browser/` delegate to `internal/engine/browser/` rather than re-implementing.

---

## 6. must.go Deep Analysis

### Overview

- **File:** `internal/engine/must.go` -- 1267 lines
- **Methods defined:** 134 Must* convenience wrappers
- **Methods with external callers:** 10 (all in `dev_helpers.go` or internal rod machinery)
- **Methods with zero callers:** 124

### What Must* Methods Do

Each `Must*` method wraps an error-returning method, calling `utils.E(err)` which **panics** on error. This is the rod-style API that Scout intentionally moved away from in favor of explicit error handling.

### Usage Breakdown

The 10 used methods are called exclusively from:
1. `internal/engine/dev_helpers.go` -- Debug/monitor UI (not production paths)
2. `internal/engine/stealth/` -- Stealth evasion setup
3. `internal/engine/must.go` itself -- Internal chaining (e.g., `MustFind` calls `MustElement`)

### Recommendation

**Phase approach:**
1. **Immediate:** Delete the 124 unused Must* methods (~1100 lines removed)
2. **Next:** Audit the 10 remaining -- most are only used in `dev_helpers.go` which itself uses `utils.E()` pervasively. Consider whether dev_helpers needs Must* or can use error returns
3. **Long-term:** If dev_helpers.go is kept (monitor UI), the remaining Must* methods can stay since they're only used in a panic-tolerant debug context

### Method-by-Method Status

**KEEP (10 methods, ~140 lines):**
MustClick, MustElement, MustEval, MustEvaluate, MustHandle, MustHandleDialog, MustInfo, MustInput, MustPage, MustPageFromTargetID, MustScreenshot, MustStart

**DELETE (124 methods, ~1100 lines):**
MustActivate, MustAdd, MustAddScriptTag, MustAddStyleTag, MustAttribute, MustBackgroundImage, MustBlur, MustCancel, MustCanvasToImage, MustCaptureDOMSnapshot, MustClose, MustConnect, MustContainsElement, MustCookies, MustDescribe, MustDisabled, MustDo, MustDoubleClick, MustDown, MustElementByJS, MustElementFromNode, MustElementFromPoint, MustElementR, MustElementX, MustElements, MustElementsByJS, MustElementsX, MustEmulate, MustEnd, MustEqual, MustEvalOnNewDocument, MustExpose, MustFind, MustFindByURL, MustFocus, MustFrame, MustGet, MustGetCookies, MustGetWindow, MustGetXPath, MustHTML, MustHandleAuth, MustHandleFileDialog, MustHas, MustHasR, MustHasX, MustHover, MustIgnoreCertErrors, MustIncognito, MustInputColor, MustInputTime, MustInsertText, MustInteractable, MustKeyActions, MustLoadResponse, MustMatches, MustMove, MustMoveMouseOut, MustMoveTo, MustNavigate, MustNavigateBack, MustNavigateForward, MustNext, MustObjectToJSON, MustObjectsToJSON, MustPDF, MustPages, MustParent, MustParents, MustPrevious, MustProperty, MustRelease, MustReload, MustRemove, MustResetNavigationHistory, MustResource, MustScreenshotFullPage, MustScroll, MustScrollIntoView, MustScrollScreenshot, MustSearch, MustSelect, MustSelectAllText, MustSelectText, MustSetBlockedURLs, MustSetCookies, MustSetDocumentContent, MustSetExtraHeaders, MustSetFiles, MustSetUserAgent, MustSetViewport, MustSetWindow, MustShadowRoot, MustShape, MustStop, MustStopLoading, MustTap, MustText, MustTriggerFavicon, MustType, MustUp, MustVersion, MustVisible, MustWait, MustWaitDOMStable, MustWaitDownload, MustWaitElementsMoreThan, MustWaitEnabled, MustWaitIdle, MustWaitInteractable, MustWaitInvisible, MustWaitLoad, MustWaitNavigation, MustWaitOpen, MustWaitRequestIdle, MustWaitStable, MustWaitVisible, MustWaitWritable, MustWindowFullscreen, MustWindowMaximize, MustWindowMinimize, MustWindowNormal

---

## Priority Action Items

| Priority | Action | Impact | Effort |
|----------|--------|--------|--------|
| 1 | Delete 124 unused Must* methods | -1100 lines, reduces panic surface | LOW |
| 2 | Add `recover()` at gRPC/MCP boundaries | Prevents server crashes | LOW |
| 3 | Deduplicate recipe.go/runbook.go helpers | -25 lines, DRY | LOW |
| 4 | Convert Scout-original panics to errors (4 sites) | Safer API | LOW |
| 5 | Consolidate logging to slog | Consistency | MEDIUM |
| 6 | Have pkg/scout/browser delegate to internal/engine/browser | -200 lines, single source of truth | MEDIUM |
| 7 | Split grpc/server/server.go by domain | Maintainability | MEDIUM |
| 8 | Prefix rod-inherited fmt.Errorf with "scout:" | Convention compliance | HIGH (657 calls) |
| 9 | Remove `createProvider` dead code in llm.go | Cleanliness | LOW |
| 10 | Remove `FingerprintToProfile` (test-only caller) | Cleanliness | LOW |

---

## Sources

All findings from direct codebase analysis using `grep`, `wc`, `find`, `diff` on the local repository. No external sources consulted -- this is a code investigation, not a technology research task.

## Metadata

**Confidence breakdown:**
- Large files: HIGH -- direct line counts
- Panic sites: HIGH -- exhaustive grep of `panic(` in non-test code
- Dead code (Must*): HIGH -- grep for all 134 method names across entire codebase
- Dead code (exports): MEDIUM -- grep may miss dynamic/reflect-based callers (unlikely in this codebase)
- Duplicates: HIGH -- diff confirmed byte-identical functions
- Pattern inconsistency: HIGH -- quantified counts from grep

**Research date:** 2026-03-29
**Valid until:** Until next major refactor
