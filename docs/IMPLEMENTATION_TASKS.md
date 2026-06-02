# Implementation Tasks

Granular tasks broken down by domain from `docs/ROADMAP.md` (Phase 75+ Future), `docs/BACKLOG.md` (open items), and known stabilization gaps.

> Source: ROADMAP Phase 75+, BACKLOG open items, stale `coverage.out` from main.

## Domain 1 — Stabilization (post-archive .planning)

| ID | What | Status |
|----|------|--------|
| 1.1 | Regenerate `coverage.out` | DONE — `task test:cover` (2026-05-21) — total 36.1% under `-short`; full suite remains multi-minute |
| 1.2 | Add `task test:full` target | DONE — Taskfile `test:full` runs the multi-minute suite |
| 1.3 | Audit `testing.Short()` skip coverage | DONE — browser-dependent tests gated; `task test:unit` runs without Chromium |
| 1.4 | Document `.planning-archive/` policy | DONE (CLAUDE.md updated 2026-05-20) |

## Domain 1B — Session Hardening (v1.0.4) [DONE]

All 14 findings from `docs/quality/SESSION_HARDENING.md` closed (12 fixed, 2 LOW deferred with rationale). See ROADMAP Phase 76 + MILESTONES v1.0.4.

## Domain 1C — Session Monitors & Encoded IDs (v1.0.5) [IN PROGRESS]

Encoded session ID (`pkg/id`), binary `scout.pid` + sibling `scout.lock`, `monitors.json` sidecar, `WithBlockRules`, `scout session audit`, AV-resilient cleanup retrier, and toolchain bump to Go 1.26. See ROADMAP Phase 77 + MILESTONES v1.0.5.

## Domain 2 — Mobile Expansion (BACKLOG P3)

| ID | What | Files | Environment | Depends | Effort |
|----|------|-------|-------------|---------|--------|
| 2.1 | Spike: `ios-webkit-debug-proxy` connection lifecycle from Go | `hacks/ios-spike/` | Go spike | — | Medium |
| 2.2 | `WithiOSDevice(opts)` option mirroring `WithMobile` | `internal/engine/option.go`, `internal/engine/mobile_ios.go` | Go | 2.1 | Large |
| 2.3 | `scout mobile ios-devices` + `scout mobile ios-connect` CLI subcommands | `cmd/scout/mobile.go` | Go / Cobra | 2.2 | Medium |
| 2.4 | iOS Safari fingerprint preset (UA, touch points, screen profile) | `internal/engine/fingerprint/presets_ios.go` | Go | 2.2 | Small |
| 2.5 | macOS-only build tag + skip on linux/windows; CI matrix entry | `internal/engine/mobile_ios_darwin.go`, `.github/workflows/test.yml` | Go / CI | 2.2 | Small |
| 2.6 | E2E test against connected iPhone Simulator | `tests/e2e/ios_test.go` | Go | 2.3 | Medium |

## Domain 3 — Marketplace Distribution (BACKLOG P3)

| ID | What | Files | Environment | Depends | Effort |
|----|------|-------|-------------|---------|--------|
| 3.1 | Run `scripts/validate-plugin.sh` against the 7 check categories; capture report | `.claude-plugin/`, validation log | Bash / docs | — | Small |
| 3.2 | Add screenshots/preview images to `.claude-plugin/plugin.json` | `.claude-plugin/plugin.json`, `assets/marketplace/*.png` | Plugin manifest | 3.1 | Small |
| 3.3 | Submission package: README excerpt, license, versioning policy | `docs/marketplace/SUBMISSION.md` | docs | 3.2 | Medium |
| 3.4 | Open submission PR to `anthropic-experimental/claude-code-plugins` (or current marketplace repo) | upstream PR | external | 3.3 | Small |
| 3.5 | Wire marketplace badge into `README.md` once accepted | `README.md` | docs | 3.4 | Small |

## Domain 4 — Coverage Maintenance

| ID | What | Files | Environment | Depends | Effort |
|----|------|-------|-------------|---------|--------|
| 4.1 | Capture fresh per-package coverage table; compare against v1.0.1 baselines (agent 91.4%, plugin 84.4%, metrics 100%, hijack 97.4%) | `docs/ROADMAP.md` (Test Coverage section) | Go | 1.1 | Small — DONE 2026-05-21: agent 96.5%, plugin 88.7%, metrics 100%, hijack 97.4%, fingerprint 90.6%; total 36.1% under `-short` |
| 4.2 | Backfill tests for packages without `*_test.go` (run audit after 1.1) | `internal/**/`, `pkg/scout/**/` | Go | 4.1 | Large |
| 4.3 | Add benchmark guard: regression-flag if `BenchmarkPageCreation` slows >25% | `.github/workflows/test.yml`, `scripts/bench-diff.sh` | CI / Bash | 4.1 | Medium |

## Suggested Implementation Order

```
1.1 → 1.2 → 1.3 → 4.1 → 4.2 → 4.3   (stabilization + coverage first)
       └→ 3.1 → 3.2 → 3.3 → 3.4 → 3.5   (marketplace can run in parallel)
2.1 → 2.2 → 2.4 → 2.3 → 2.5 → 2.6   (iOS chain, gated on someone with a Mac)
```

Cross-reference: each task ID is quotable from `ROADMAP.md` / `BACKLOG.md` / `MILESTONES.md` entries planning the next minor release.
