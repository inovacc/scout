---
phase: "03"
plan: "02"
subsystem: cli
tags: [search, websearch, cli-consolidation, command-merge]
dependency_graph:
  requires: []
  provides: [CLI-03]
  affects: [cmd/scout/search.go]
tech_stack:
  added: []
  patterns: [functional-flags, cobra-subcommand]
key_files:
  created: []
  modified:
    - cmd/scout/search.go
  deleted:
    - cmd/scout/websearch.go
    - cmd/scout/search_engines.go
decisions:
  - "D-03: websearch merged INTO search (search is the better name)"
  - "D-04: websearch deleted entirely"
  - "D-05: google/bing/duckduckgo subcommands removed; wikipedia kept as unique engine"
metrics:
  duration_minutes: 8
  completed_date: "2026-03-29"
  tasks_completed: 1
  tasks_total: 1
  files_changed: 3
---

# Phase 03 Plan 02: Merge websearch into search Summary

**One-liner:** Consolidated three overlapping search entry points into one `scout search` command with `--engine` and `--fetch` flags.

## What Was Built

`scout search` now covers everything that `scout websearch` and the `scout search google/bing/duckduckgo` subcommands did:

- `--engine google|bing|ddg` selects the search engine (replaces per-engine subcommands)
- `--fetch markdown|text|full` triggers content extraction on result pages (migrated from `websearch`)
- `--max-fetch`, `--main-only`, `--concurrency` support the fetch workflow
- `scout search wikipedia <query>` is retained as the only subcommand (unique engine, not duplicated by `--engine`)

When `--fetch` is set the command routes through `browser.WebSearch()` (the richer API). Without `--fetch` it uses the lighter `browser.Search()` API as before.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Merge websearch --fetch into search.go, delete websearch.go and search_engines.go | 276e2c6 | cmd/scout/search.go (modified), cmd/scout/websearch.go (deleted), cmd/scout/search_engines.go (deleted) |

## Verification

- `go build ./cmd/scout/` passes
- No `websearchCmd` or `AddCommand.*websearch` references remain
- `--fetch` flag present in `search.go`
- `wikipedia` subcommand retained in `search.go`

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Self-Check: PASSED

- `cmd/scout/search.go` exists with new content
- `cmd/scout/websearch.go` does not exist
- `cmd/scout/search_engines.go` does not exist
- Commit `276e2c6` present in git log
