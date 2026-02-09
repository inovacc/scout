# ADR-0001: Wrap go-rod with a simplified API

## Status
Accepted

## Context
Building a browser automation library in Go requires choosing between:
1. Using the Chrome DevTools Protocol (CDP) directly
2. Using an existing Go automation library (go-rod, chromedp, playwright-go)
3. Wrapping an existing library with a simpler API

go-rod provides comprehensive CDP coverage but exposes low-level types from `proto` and requires verbose setup code. Many use cases need only a subset of features with a simpler interface.

## Decision
Wrap go-rod with thin type wrappers (`Browser`, `Page`, `Element`, `EvalResult`) that provide a Go-idiomatic API. Use the functional options pattern for configuration. Re-export rod types only where necessary (e.g., `SelectorType`, `devices.Device`).

Expose the underlying rod instances via escape hatches (`RodPage()`, `RodElement()`) so users can drop down to the full rod API when the wrapper is insufficient.

## Consequences

### Positive
- Simpler API for common use cases (fewer imports, no proto types in user code)
- Consistent error handling with `scout:` prefix wrapping
- Nil-safe operations reduce boilerplate in caller code
- Escape hatches prevent the wrapper from being a limitation
- go-rod handles browser download, CDP protocol, and concurrency

### Negative
- Adds an abstraction layer that must track go-rod API changes
- New rod features require explicit wrapping to be accessible through the scout API
- Users familiar with rod must learn a slightly different API surface

## Alternatives Considered

### chromedp
More established but lower-level, callback-heavy API. go-rod's fluent builder pattern is more Go-idiomatic.

### playwright-go
Port of Playwright's API. Heavier dependency, less Go-native feel.

### Direct CDP
Maximum flexibility but requires managing the protocol, browser lifecycle, and serialization manually. Unreasonable effort for the target use cases.
