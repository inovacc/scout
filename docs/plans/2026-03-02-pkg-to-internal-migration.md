# pkg/scout → internal/engine Migration Plan

## Goal

Restructure the monolithic 101-file `pkg/scout/` into:
- **`pkg/scout/`** — thin public facade re-exporting only the types/methods external consumers need
- **`internal/engine/`** — domain sub-packages holding all implementation logic (~15 packages)

External import path stays `github.com/inovacc/scout/pkg/scout`. Internal consumers (`cmd/scout/`, `grpc/`, `pkg/scout/mcp/`, etc.) import `internal/engine/*` directly.

## Architecture

```
pkg/scout/                    ← Public facade (re-exports from internal/engine)
  scout.go                    ← New(), Browser, Page, Element, Option, core result types
  option.go                   ← WithHeadless, WithStealth, etc. (delegates to internal)
  types.go                    ← Public result types (GatherResult, CrawlResult, etc.)

internal/engine/              ← Core browser/page/element implementation
  core/                       ← Browser, Page, Element, option parsing, launcher
  bridge/                     ← Bridge extension (WS, commands, events, fallback, record, scout API)
  detect/                     ← Framework, tech stack, render mode, PWA, challenge detection
  extract/                    ← Struct extraction, form detection, table extraction
  fingerprint/                ← Generation, rotation, store, data pools
  gather/                     ← Gather, knowledge, healthcheck pipelines
  hijack/                     ← Session hijacker, HAR recorder, network interception
  inject/                     ← JS injection helpers, templates, EvalOnNewDocument
  llm/                        ← Provider interface, Ollama, OpenAI, Anthropic, workspace, review
  nav/                        ← Navigation, bypass, wait_smart, WaitFrameworkReady
  paginate/                   ← PaginateByClick, PaginateByURL, PaginateByInfiniteScroll
  research/                   ← Research agent, cache, presets
  screenshot/                 ← Screenshot, screenrecord, visual diff, PDF
  search/                     ← Multi-engine search, Wikipedia, GitHub
  session/                    ← Session tracking, storage, profiles, credentials, cookie jar
  vpn/                        ← VPN interface, Surfshark, proxy chain, rotation
```

## Migration Phases

### Phase 49: Prepare — Extract internal/engine/core (Largest, do first)

**What moves:**
- `browser.go` → `internal/engine/core/browser.go` (unexported struct, exported via pkg/scout facade)
- `page.go` → `internal/engine/core/page.go`
- `element.go` → `internal/engine/core/element.go`
- `option.go` → `internal/engine/core/option.go`
- `eval.go` → `internal/engine/core/eval.go`
- `autofree.go` → `internal/engine/core/autofree.go`
- `browser_detect*.go`, `browser_download.go`, `browser_path*.go` → `internal/engine/core/`
- `electron*.go` → `internal/engine/core/`
- `window.go` → `internal/engine/core/`

**pkg/scout/ facade:**
```go
package scout

import "github.com/inovacc/scout/internal/engine/core"

type Browser = core.Browser
type Page = core.Page
type Element = core.Element
type Option = core.Option
type BrowserType = core.BrowserType

func New(opts ...Option) (*Browser, error) { return core.New(opts...) }
```

**Risk:** Highest risk phase — touches the foundation. All other phases depend on this.

**Effort:** Large (2-3 days)

### Phase 50: Extract internal/engine/bridge

**What moves:** `bridge.go`, `bridge_commands.go`, `bridge_events.go`, `bridge_fallback.go`, `bridge_record.go`, `bridge_scout_api.go`, `bridge_ws.go`

**Public exports needed:** `BridgeRecorder`, `NewBridgeRecorder`, `RecordAll`, `ToRecipe` (via facade)

**Effort:** Medium (half day)

### Phase 51: Extract internal/engine/detect

**What moves:** `detect.go`, `challenge.go`, `challenge_solver.go`, `challenge_captcha.go`, `challenge_cloudflare.go`, `challenge_service.go`, `wait_smart.go`

**Public exports needed:** `FrameworkInfo`, `TechStack`, `ChallengeInfo`, `ChallengeType`, `DetectTechStack`, `DetectFrameworks`, `WaitFrameworkReady`

**Effort:** Medium (half day)

### Phase 52: Extract internal/engine/hijack

**What moves:** `hijack_session.go`, `hijack_har.go`, `network.go`

**Public exports needed:** `SessionHijacker`, `HijackRecorder`, `HijackEvent`, `CapturedRequest`, `CapturedResponse`, `HijackRouter`, `Cookie`

**Effort:** Medium (half day)

### Phase 53: Extract internal/engine/extract

**What moves:** `extract.go`, `extract_all.go`, `form.go`, `markdown.go`, `readability.go`, `snapshot.go`, `snapshot_script.go`, `selector.go`

**Public exports needed:** `ExtractOption`, `TableData`, `MetaData`, `Form`, `FormField`

**Effort:** Medium (half day)

### Phase 54: Extract internal/engine/fingerprint

**What moves:** `fingerprint.go`, `fingerprint_data.go`, `fingerprint_rotation.go`, `fingerprint_store.go`

**Public exports needed:** `Fingerprint`, `FingerprintRotationConfig`, `FingerprintStore`, `GenerateFingerprint`

**Effort:** Quick (2-3 hours)

### Phase 55: Extract internal/engine/llm

**What moves:** `llm.go`, `llm_anthropic.go`, `llm_ollama.go`, `llm_openai.go`, `llm_review.go`, `llm_workspace.go`

**Public exports needed:** `LLMProvider`, `LLMOption`

**Effort:** Quick (2-3 hours)

### Phase 56: Extract internal/engine/session

**What moves:** `session_track.go`, `session_track_*.go`, `storage.go`, `profile.go`, `capture.go`, `cookie_jar.go`

**Public exports needed:** `SessionState`, `UserProfile`, `CapturedCredentials`

**Effort:** Medium (half day)

### Phase 57: Extract internal/engine/vpn

**What moves:** `vpn.go`, `vpn_rotation.go`, `vpn_surfshark.go`, `vpn_surfshark_connect.go`, `proxy_chain.go`

**Public exports needed:** `VPNProvider`, `VPNConnection`, `ProxyChain`

**Effort:** Quick (2-3 hours)

### Phase 58: Extract internal/engine/search

**What moves:** `search.go`, `search_wikipedia.go`, `github.go`, `github_extract.go`, `websearch.go`, `webfetch.go`, `webmcp.go`, `map.go`

**Public exports needed:** `SearchResult`, `SearchEngine`, `SearchOption`

**Effort:** Medium (half day)

### Phase 59: Extract internal/engine/gather

**What moves:** `gather.go`, `knowledge.go`, `knowledge_option.go`, `knowledge_writer.go`, `healthcheck.go`, `healthcheck_option.go`, `swagger.go`

**Public exports needed:** `GatherResult`, `GatherOption`, `KnowledgeResult`, `KnowledgePage`, `HealthReport`, `HealthIssue`

**Effort:** Medium (half day)

### Phase 60: Extract remaining (inject, paginate, research, screenshot, nav)

**What moves:**
- `inject.go`, `inject_helpers.go`, `inject_templates.go` → `internal/engine/inject/`
- `paginate.go` → `internal/engine/paginate/`
- `research.go`, `research_cache.go` → `internal/engine/research/`
- `screenshot.go`, `screenrecord.go`, `visual_diff.go`, `upload.go` → `internal/engine/screenshot/`
- `navigate_bypass.go` → `internal/engine/nav/`
- `crawl.go`, `sitemap.go`, `batch.go`, `jobs.go`, `tabgroup.go`, `recorder.go` → `internal/engine/core/`

**Effort:** Medium (half day)

### Phase 61: Update all consumers

**What changes:**
- `cmd/scout/*.go` — update imports from `pkg/scout` to `internal/engine/*` where needed
- `grpc/server/*.go` — same
- `pkg/scout/mcp/*.go` — same
- `pkg/scout/runbook/*.go` — same
- `pkg/scout/scraper/*.go` — same
- `examples/*.go` — keep using `pkg/scout` (public facade)

**Effort:** Large (1-2 days)

### Phase 62: Cleanup & verify

- Remove dead code from `pkg/scout/`
- Verify `go build ./...` (all packages)
- Verify `go test ./...` (all tests)
- Verify examples still compile against public facade
- Update CLAUDE.md architecture section
- Update README import examples

**Effort:** Medium (half day)

## Key Decisions

1. **Type aliases vs wrappers:** Use `type Browser = core.Browser` (alias) for zero-cost re-export. No wrapper overhead.

2. **Method attachment:** Methods attached to `Browser`/`Page` in sub-packages need to use interfaces or be attached at the `core` level with delegation. Alternative: sub-packages expose functions like `gather.Run(page, opts...)` instead of methods.

3. **Circular dependency prevention:** `internal/engine/core` must not import any sibling package. All sub-packages import `core` for types. `core` defines interfaces that sub-packages implement.

4. **Test migration:** Tests move with their implementation files. Integration tests that span multiple packages stay in a top-level `internal/engine/integration_test.go` or remain in `pkg/scout/`.

5. **rod sub-package:** `pkg/scout/rod/` stays as-is (already separate). `internal/engine/core` imports it.

## Dependency Graph

```
pkg/scout (facade)
  └── internal/engine/core (Browser, Page, Element, options)
        ├── pkg/scout/rod (CDP wrapper)
        └── [no sibling imports]

internal/engine/bridge     → core
internal/engine/detect     → core
internal/engine/extract    → core
internal/engine/fingerprint → core
internal/engine/gather     → core, hijack, detect, extract
internal/engine/hijack     → core
internal/engine/inject     → core
internal/engine/llm        → core
internal/engine/nav        → core, detect
internal/engine/paginate   → core, extract
internal/engine/research   → core, llm, search
internal/engine/screenshot → core
internal/engine/search     → core
internal/engine/session    → core
internal/engine/vpn        → core
```

## Risk Mitigation

- **Phase 49 (core) is the gate:** If this works cleanly, the rest is mechanical.
- **Feature-flag the migration:** Keep old `pkg/scout/` working until each phase is verified.
- **One phase per PR:** Each phase is independently reviewable and revertible.
- **Tests must pass at every phase boundary.** Never merge a phase with broken tests.

## Estimated Total Effort

| Category | Phases | Effort |
|----------|--------|--------|
| Core extraction | 49 | 2-3 days |
| Domain sub-packages | 50-60 | 5-6 days |
| Consumer updates | 61 | 1-2 days |
| Cleanup & verify | 62 | half day |
| **Total** | **14 phases** | **~10-12 days** |
