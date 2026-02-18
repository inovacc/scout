# ADR-0002: Skip chromedp as dual backend

## Status

Accepted

## Context

During planning for scout's scraping toolkit expansion, adding chromedp as an alternative CDP backend was evaluated. This would theoretically give users a choice of underlying engine.

## Decision

Do **not** add chromedp as a dual backend. Keep go-rod as the sole engine.

## Rationale

### Effort vs. Value

- An interface abstraction would need to cover 142+ methods across Browser, Page, and Element
- chromedp uses an action-pipeline pattern (`chromedp.Run(ctx, actions...)`) that is fundamentally incompatible with rod's OOP fluent API (`page.Element("sel").Click()`)
- Bridging these paradigms would require adapter types for every operation

### Missing chromedp equivalents

Rod provides features with no chromedp counterpart:

- **Stealth mode** via `go-rod/stealth` (critical for scraping)
- **WaitStable / WaitDOMStable / WaitRequestIdle** (essential for dynamic pages)
- **HijackRequests** with modify-in-flight support
- **Race** (first-element-wins pattern)
- Auto browser download and management

### Escape hatch exists

Users needing raw CDP access can use `RodPage()` and `RodElement()` to drop down to rod's full API, which itself supports arbitrary CDP commands via `proto` types.

## Consequences

### Positive

- Development effort focused on high-value scraping features (extraction, forms, pagination, search, crawling)
- No interface overhead or adapter complexity
- Simpler codebase, easier to maintain

### Negative

- Users who prefer chromedp cannot swap backends
- If rod development stalls, migration would require rewriting (mitigated by rod's active maintenance and MIT license)
