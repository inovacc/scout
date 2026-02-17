# ADR-0006: Internalize github.com/go-rod/rod

**Status:** Proposed
**Date:** 2026-02-17
**Decision:** Pending

---

## Context

Evaluate whether to internalize `github.com/go-rod/rod` (the browser automation library) into `pkg/rod/` within the Scout project, reducing external dependency risk and enabling deeper customization.

---

## 1. Summary

| Field | Value |
|-------|-------|
| Module | `github.com/go-rod/rod` |
| Version | `v0.116.2` (current in go.mod) |
| License | MIT (Yad Smood, 2019) |
| Total Go files | 113 (excl. tests, examples, fixtures, generators) |
| Total LOC | ~41,686 (excl. tests, examples, fixtures, generators) |
| Packages | 22 |
| Test files | 35 |
| Direct deps | 6 (all `ysmood/*`) |
| Indirect deps | 1 (`ysmood/gop`) |
| CGO / Assembly | None — pure Go |
| Build tags | 2 files (`os_unix.go`, `os_windows.go` in launcher) |

---

## 2. Packages to Internalize

### Rod package structure:

| Package | Purpose | Used by Scout? |
|---------|---------|----------------|
| `rod` (root) | Core Browser/Page/Element API | **Yes** — heavily |
| `lib/launcher` | Browser binary discovery & launch | **Yes** — browser setup |
| `lib/launcher/flags` | Chrome flag constants | **Yes** — flag setting |
| `lib/proto` | CDP protocol bindings (generated) | **Yes** — protocol types |
| `lib/input` | Keyboard/mouse key constants | **Yes** — input simulation |
| `lib/devices` | Device emulation presets | **Yes** — device emulation |
| `lib/cdp` | WebSocket CDP client | **Yes** (indirect, via rod core) |
| `lib/defaults` | Default configuration values | Likely (indirect) |
| `lib/js` | JavaScript helper functions | Likely (indirect, embedded JS) |
| `lib/utils` | Utility functions | Likely (indirect) |
| `lib/assets` | Embedded static assets | Likely (indirect) |
| `lib/docker` | Docker helper binary | No |
| `lib/examples` | Example programs | No |
| `lib/benchmark` | Benchmarks | No |
| `fixtures/` | Test fixtures | No |

---

## 3. Dependencies Brought Along

All 6 direct dependencies are from the `ysmood` ecosystem:

| Dependency | Purpose | Risk |
|-----------|---------|------|
| `github.com/ysmood/gson` | JSON encoding/decoding | **Low** — small, widely used in rod |
| `github.com/ysmood/goob` | Observable/event pattern | **Low** — small utility |
| `github.com/ysmood/leakless` | Process leak prevention | **Medium** — platform-specific binary embedding |
| `github.com/ysmood/fetchup` | HTTP download with progress | **Low** — used only for browser downloads |
| `github.com/ysmood/got` | Testing utilities (only `lib/lcs`) | **Low** — only LCS algorithm used at runtime |
| `github.com/ysmood/gotrace` | Goroutine tracing/debugging | **None** — only used in examples |
| `github.com/ysmood/gop` (indirect) | Pretty printer | **None** — indirect, dev utility |

**Key observation:** Internalizing rod does NOT eliminate these deps — they would still be required as transitive dependencies unless also internalized or replaced.

---

## 4. Import Rewrite Map

```
github.com/go-rod/rod           → github.com/inovacc/scout/pkg/rod
github.com/go-rod/rod/lib/cdp   → github.com/inovacc/scout/pkg/rod/lib/cdp
github.com/go-rod/rod/lib/proto → github.com/inovacc/scout/pkg/rod/lib/proto
github.com/go-rod/rod/lib/launcher       → github.com/inovacc/scout/pkg/rod/lib/launcher
github.com/go-rod/rod/lib/launcher/flags  → github.com/inovacc/scout/pkg/rod/lib/launcher/flags
github.com/go-rod/rod/lib/input   → github.com/inovacc/scout/pkg/rod/lib/input
github.com/go-rod/rod/lib/devices → github.com/inovacc/scout/pkg/rod/lib/devices
github.com/go-rod/rod/lib/defaults → github.com/inovacc/scout/pkg/rod/lib/defaults
github.com/go-rod/rod/lib/js      → github.com/inovacc/scout/pkg/rod/lib/js
github.com/go-rod/rod/lib/utils   → github.com/inovacc/scout/pkg/rod/lib/utils
github.com/go-rod/rod/lib/assets  → github.com/inovacc/scout/pkg/rod/lib/assets
```

**Files requiring rewrite:** 15 (listed in Section 4 analysis)

---

## 5. Minimal Subset Analysis

### Directly used packages (6):
- `rod` (root) — Browser, Page, Element, HijackRouter, Eval, SelectorType
- `lib/launcher` — New, Headless, Bin, Proxy, Launch, etc.
- `lib/launcher/flags` — Flag type
- `lib/proto` — CDP protocol types (extensive usage)
- `lib/input` — Key type
- `lib/devices` — Device type

### Indirectly required (5):
- `lib/cdp` — WebSocket CDP communication (used by rod core)
- `lib/js` — Embedded JavaScript helpers (used by rod core)
- `lib/utils` — Utility functions (used across lib/)
- `lib/defaults` — Default configuration (used by launcher)
- `lib/assets` — Static assets (used by lib/js)

### Excludable (4):
- `lib/docker` — Docker binary, not used
- `lib/examples` — Example code
- `lib/benchmark` — Benchmarks
- `fixtures/` — Test fixtures and generators

### Verdict: Subset copy is NOT practical
The root `rod` package depends on nearly all `lib/` packages internally. Copying only the used packages still requires 11 of 15 packages (~73%). The "subset" would be nearly the full module minus examples/docker/benchmarks.

---

## 6. Risk Assessment

### License Compatibility
- **Rod:** MIT License — **fully compatible** with BSD 3-Clause
- MIT is permissive; only requires preserving copyright notice
- **Action:** Include original LICENSE and copyright in `pkg/rod/`

### Maintenance Burden
- **41,686 LOC** to maintain — this is substantial
- Rod is actively maintained (regular releases, responsive maintainer)
- Internalizing **freezes the version** — no upstream bugfixes, security patches, or Chrome protocol updates
- `lib/proto` contains **generated CDP bindings** (~30,000+ LOC) that must be regenerated when Chrome updates protocol
- The `lib/launcher/revision.go` pins a specific Chrome revision — needs manual updates

### Complexity Concerns
- **Platform-specific code** in launcher (`os_unix.go`, `os_windows.go`) — manageable
- **Generated code** in `lib/proto` — would need generator tooling or manual updates
- **Chrome protocol evolution** — the protocol changes with each Chrome release; rod tracks this
- **`leakless` dependency** — embeds platform-specific binaries for process management

### Breaking Changes Risk
- Freezing at v0.116.2 means no new Chrome DevTools Protocol support
- Chrome updates may break compatibility with frozen CDP bindings
- **High risk** for a browser automation tool that depends on Chrome protocol stability

---

## 7. Recommended Strategy

### **Keep External** (Recommended)

**Justification:**

1. **Size:** 41,686 LOC across 22 packages is too large to maintain as vendored code
2. **Chrome coupling:** Rod's `lib/proto` package tracks Chrome DevTools Protocol changes — internalizing it would require rebuilding their code generation pipeline or manually updating protocol bindings
3. **Active maintenance:** Rod receives regular updates for Chrome compatibility — losing these would degrade Scout over time
4. **Minimal benefit:** Rod's 6 external deps are all small, single-author utilities — the dependency tree is already lean
5. **`go-rod/stealth`** is also used and depends on `go-rod/rod` — internalizing rod would break stealth's compatibility
6. **Platform concerns:** The `leakless` binary embedding adds hidden complexity

### Alternative: Fork & Adapt (If needed)

If customization is truly required (e.g., modifying rod's core behavior), **fork the repository** instead:

1. `gh repo fork go-rod/rod --clone --remote`
2. Replace import path in `go.mod`: `replace github.com/go-rod/rod => github.com/inovacc/rod v0.116.2`
3. Make custom changes in the fork
4. Periodically sync with upstream: `git fetch upstream && git merge upstream/main`

This preserves the ability to pull upstream updates while allowing modifications.

---

## 8. Step-by-Step Execution Plan

### If keeping external (recommended):
No action needed. Current setup is optimal.

### If forking (alternative):

1. Fork: `gh repo fork go-rod/rod --org inovacc`
2. Add replace directive to `go.mod`:
   ```
   replace github.com/go-rod/rod => github.com/inovacc/rod v0.116.2
   ```
3. Run `go mod tidy`
4. Verify: `go build ./...` and `go test ./...`
5. Set up upstream sync schedule (monthly recommended)

### If internalizing (not recommended):

1. Create `pkg/rod/` directory structure mirroring `lib/`
2. Copy all 11 required packages (excluding docker, examples, benchmark, fixtures)
3. Copy `LICENSE` to `pkg/rod/LICENSE`
4. Rewrite all import paths (11 mappings across 113 source files + 15 Scout files)
5. Update `go.mod`: remove `github.com/go-rod/rod`, keep `ysmood/*` deps
6. Handle `go-rod/stealth` — either also internalize or find alternative
7. Run `go mod tidy`
8. Run `go build ./...` and `go test ./...`
9. Set up CDP protocol update process (or accept frozen version)
10. Remove `.tmp/rod`

**Estimated effort:** 2-3 days for initial internalization, ongoing maintenance burden for Chrome protocol tracking.

---

## Decision

**Recommendation: Keep `github.com/go-rod/rod` as an external dependency.**

The module is too large (41K LOC), too tightly coupled to Chrome's evolving protocol, and too actively maintained to justify internalization. The dependency tree is minimal (6 small deps from one author). If customization is needed, a fork with `replace` directive is the better approach.

---

## References

- [go-rod/rod](https://github.com/go-rod/rod) — MIT License
- [ADR-0001: Go Rod Wrapper](ADR-0001-go-rod-wrapper.md) — Original decision to use rod
- [ADR-0002: No ChromeDP](ADR-0002-no-chromedp.md) — Why rod over chromedp
