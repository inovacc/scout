# Backlog

## Priority Levels
| Priority | Timeline |
|----------|----------|
| P1 | First month |
| P2 | First quarter |
| P3 | Future |

## Items

### Test Coverage Gaps

- **Priority:** P1
- **Description:** Page methods at 0% coverage: NavigateForward, ScrollScreenshot, ScreenshotPNG, PDF, PDFWithOptions, ElementByJS, ElementByText, ElementFromPoint, Search, HasXPath, WaitStable, WaitDOMStable, WaitIdle, WaitXPath, SetWindow, Emulate, SetDocumentContent, AddScriptTag, AddStyleTag, StopLoading, Activate, HandleDialog, Race
- **Effort:** Large

### Element Method Test Coverage

- **Priority:** P1
- **Description:** Element methods at 0% coverage: DoubleClick, RightClick, Hover, MoveMouseOut, Tap, InputTime, InputColor, Type, Press, SelectOptionByCSS, SetFiles, Focus, Blur, ScrollIntoView, Remove, SelectAllText, SelectText, Interactable, Disabled, ScreenshotJPEG, GetXPath, ContainsElement, Equal, CanvasToImage, BackgroundImage, Resource, Parent, Parents, Next, Previous, ShadowRoot, Frame, all Wait* methods
- **Effort:** Large

### EvalResult Type Conversion Tests

- **Priority:** P1
- **Description:** EvalResult.Float(), JSON(), and Decode() have 0% coverage. String(), Int(), Bool() only partially covered.
- **Effort:** Small

### Network Accessor Tests

- **Priority:** P2
- **Description:** HijackContext.Request(), ContinueRequest(), LoadResponse(), Skip(), HijackRequest methods (Method, URL, Header, Body), HijackResponse.SetHeader(), Fail() — all at 0% coverage.
- **Effort:** Medium

### Missing LICENSE File

- **Priority:** P1
- **Description:** No LICENSE file in the repository. Required for open-source distribution.
- **Effort:** Small

### gRPC Server Test Coverage

- **Priority:** P2
- **Description:** The `grpc/server/` package has 0% test coverage. All 25+ RPCs (CreateSession, Navigate, Click, Type, Screenshot, etc.) are untested. Consider integration tests against a local httptest server with a real browser.
- **Effort:** Large

### GoDoc Examples

- **Priority:** P2
- **Description:** Add `Example*` test functions for key API entry points: New, Browser.NewPage, Page.Element, Page.Eval, Page.Hijack, Element.Click, Element.Input, NetworkRecorder, KeyPress.
- **Effort:** Medium

### Remove Legacy Taskfile Tasks

- **Priority:** P3
- **Description:** `Taskfile.yml` still contains legacy template tasks that don't apply: `proto:generate` (references non-existent `internal/database/proto/`), `sqlc:generate`, `generate`, `build:dev`, `build:prod`, `run` (depends on generate), `release`, `release:snapshot`, `release:check` (goreleaser not configured). Valid tasks (`proto`, `grpc:*`, `test`, `check`, `lint`, `fmt`, `vet`, `deps`) work correctly.
- **Effort:** Small

## Resolved Items

| Item | Resolution | Date |
|------|------------|------|
| Missing Git Remote | Remote configured at `github.com/inovacc/scout.git` | 2025 |
| Taskfile Cleanup | Legacy template tasks replaced with valid proto/grpc tasks | 2025 |
