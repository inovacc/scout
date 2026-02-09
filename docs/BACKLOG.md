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

### Missing Git Remote

- **Priority:** P1
- **Description:** No git remote configured. Repository needs to be pushed to GitHub at `github.com/inovacc/scout` (as declared in go.mod).
- **Effort:** Small

### Taskfile Cleanup

- **Priority:** P3
- **Description:** Taskfile.yml contains tasks for protobuf generation (`proto:generate`), sqlc (`sqlc:generate`), goreleaser builds (`build:dev`, `build:prod`), and `run` — none of which apply to this library package. These appear to be copied from a template.
- **Effort:** Small

### GoDoc Examples

- **Priority:** P2
- **Description:** Add `Example*` test functions for key API entry points: New, Browser.NewPage, Page.Element, Page.Eval, Page.Hijack, Element.Click, Element.Input.
- **Effort:** Medium
