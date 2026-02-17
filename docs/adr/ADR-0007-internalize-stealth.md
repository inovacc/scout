# ADR-0007: Internalize github.com/go-rod/stealth

**Status:** Proposed
**Date:** 2026-02-17
**Decision:** Pending

---

## Context

Evaluate whether to internalize `github.com/go-rod/stealth` — a tiny anti-bot-detection wrapper for go-rod — into the Scout project.

---

## 1. Summary

| Field | Value |
|-------|-------|
| Module | `github.com/go-rod/stealth` |
| Version | `v0.4.9` (current in go.mod) |
| License | MIT (Rod, 2020) |
| Total Go files | 2 (`main.go`, `assets.go`) |
| Total LOC | ~45 (Go code) + large embedded JS constant |
| Packages | 1 (root only) |
| Test files | 1 (`examples_test.go`) |
| Direct deps | 1 (`github.com/go-rod/rod`) |
| CGO / Assembly | None |
| Build tags | None |
| Generated code | `assets.go` — generated from puppeteer-extra-plugin-stealth JS (v2.7.3) |

---

## 2. Packages to Internalize

| Package | Purpose | Action |
|---------|---------|--------|
| `stealth` (root) | `Page()` and `MustPage()` + embedded JS | **Copy to `pkg/stealth/`** |
| `generate/` | Code generator for assets.go | Skip (only needed to regenerate JS) |

---

## 3. Dependencies Brought Along

| Dependency | Purpose | Impact |
|-----------|---------|--------|
| `github.com/go-rod/rod` | Already a direct dep of Scout | **None** — already present |

**Internalizing stealth adds zero new dependencies.** It only imports `rod` and `rod/lib/proto`, both already in Scout's dependency tree.

---

## 4. Import Rewrite Map

```
github.com/go-rod/stealth → github.com/inovacc/scout/pkg/stealth
```

**Files to update:** 1 (`pkg/scout/browser.go`)

---

## 5. Minimal Subset Analysis

The module is already minimal:
- **`main.go`** (33 LOC): `Page()` function — creates a page and injects anti-detection JS via `EvalOnNewDocument`
- **`assets.go`** (~large): Embedded JS constant from puppeteer-extra-plugin-stealth v2.7.3

Scout uses exactly one function: `stealth.Page(b.browser)`. The entire module is the minimal subset.

---

## 6. Risk Assessment

### License Compatibility
- **Stealth:** MIT — **fully compatible** with BSD 3-Clause
- **Embedded JS:** Based on puppeteer-extra-plugin-stealth (MIT) — compatible
- **Action:** Include original LICENSE in `pkg/stealth/`

### Maintenance Burden
- **Extremely low** — 33 lines of Go code
- The JS blob (`assets.go`) is the bulk — it's a snapshot of puppeteer-extra-plugin-stealth v2.7.3
- Updates to the JS would require running the generator or manually updating the constant
- The upstream repo is **infrequently updated** (last significant change was the JS version bump)

### Complexity
- Zero — no build tags, no CGO, no platform-specific code
- The `Page()` function is trivially simple: create page → inject JS → return

### Breaking Changes Risk
- **Very low** — the API is a single function that wraps rod's existing `Page()` and `EvalOnNewDocument()`
- The embedded JS may become stale as browsers evolve anti-bot detection, but this is independent of internalization
- **Freezing is acceptable** — can manually update the JS constant when needed

---

## 7. Recommended Strategy

### **Full Copy** (Recommended)

**Justification:**

1. **Tiny footprint:** 2 files, 33 lines of actual Go logic
2. **Zero new deps:** Only imports rod, which Scout already uses
3. **Single function used:** `stealth.Page()` — trivial to maintain
4. **Eliminates a dependency:** Removes `github.com/go-rod/stealth` and its indirect dep chain from `go.mod`
5. **Easy to update:** If the stealth JS needs updating, just replace the constant string
6. **Infrequently maintained upstream:** The module rarely changes — no risk of missing important updates

---

## 8. Step-by-Step Execution Plan

1. **Create target directory:**
   ```
   mkdir -p pkg/stealth
   ```

2. **Copy files:**
   ```
   cp .tmp/stealth/main.go pkg/stealth/main.go
   cp .tmp/stealth/assets.go pkg/stealth/assets.go
   cp .tmp/stealth/LICENSE pkg/stealth/LICENSE
   ```

3. **Rewrite package imports in `pkg/stealth/main.go`:**
   - No changes needed — it imports `github.com/go-rod/rod` which Scout already uses

4. **Update Scout's import in `pkg/scout/browser.go`:**
   ```go
   // Old:
   "github.com/go-rod/stealth"
   // New:
   "github.com/inovacc/scout/pkg/stealth"
   ```

5. **Update `go.mod`:**
   ```
   # Remove the stealth dependency
   go mod edit -droprequire github.com/go-rod/stealth
   ```

6. **Tidy:**
   ```
   go mod tidy
   ```

7. **Verify:**
   ```
   go build ./...
   go test ./...
   ```

8. **Cleanup:**
   ```
   rm -rf .tmp/stealth
   ```

---

## Decision

**Recommendation: Full copy into `pkg/stealth/`.**

This is an ideal internalization candidate — 33 lines of Go, zero new dependencies, single function usage, MIT licensed, and infrequently updated upstream. The cost of maintaining it internally is essentially zero.

---

## References

- [go-rod/stealth](https://github.com/go-rod/stealth) — MIT License
- [puppeteer-extra-plugin-stealth](https://github.com/nicedayfor/puppeteer-extra-plugin-stealth) — Source of embedded JS
- [ADR-0006: Internalize Rod](ADR-0006-internalize-rod.md) — Rod itself recommended to keep external
