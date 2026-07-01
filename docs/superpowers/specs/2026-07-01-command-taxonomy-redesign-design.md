# Command Taxonomy Redesign — Design Spec

**Date:** 2026-07-01
**Status:** Draft — awaiting user review gate (Phase 5 complete, Phase 6 not started)
**Surfaces:** Scout CLI (Cobra, `cmd/scout/`) + MCP tools (`pkg/scout/mcp/`)

## 1. Context

The Scout CLI has grown to **64 top-level commands, 215 command definitions,
~193 subcommand relationships, ~75+ flags**. The audit
(`docs/command-taxonomy-audit.md`) confirmed HIGH-severity issues: verb
overloading (`list`/`show` in 15+ places), wide/flat trees (`github`, `vault`,
`runbook`), duplicate traversal commands (`crawl`/`map`/`sitemap`/`knowledge`),
mixed verb-noun ordering, and scattered domains (two `plugin` trees, two LLM
roots, `github` split across two files).

## 2. Scope decisions (locked with user)

| Decision | Choice |
|----------|--------|
| Scope | Full cleanup (all 10 classes) |
| Migration style | **Clean break** — rename in place, no aliases; ship `docs/MIGRATION-command-taxonomy.md`. Justified by the Stabilization-Milestone "breaking changes acceptable" constraint. |
| Surfaces | CLI + MCP, kept 1:1 aligned |
| Shape | Shallow grouping (hot-verb allowlist stays flat; crowded domains get sub-nouns) |
| Traversal cluster | **Full `site` domain** — `crawl`/`gather`/`map`/`knowledge`/`test-site` all move under `site`; `sitemap` merges into `site map` |
| LLM cluster | **`llm` domain** — `ollama`→`llm ollama`, `ai-job`→`llm job` |

## 3. The rules contract

Authoritative rules live in `docs/COMMAND-TAXONOMY.md` (R1–R7 + domain map).
Summary: noun→verb paths; kebab-case; sub-noun layer at ~6+ siblings;
cross-cutting verbs always domain-qualified; one canonical flag name per concept;
MCP tool name = CLI path joined by `_`; a fixed hot-verb allowlist is the only
bare-root exception.

## 4. Structural moves

| # | Problem (audit ref) | Move | Old → New (representative) |
|---|---------------------|------|----------------------------|
| 1 | Duplicate traversal (Issue 1) | `site` domain | `crawl`→`site crawl`, `gather`→`site gather`, `map`→`site map`, `sitemap extract`→`site map`, `knowledge`→`site knowledge`, `test-site`→`site test` |
| 2 | 6 bare root inspect verbs (Issue 5/10) | `page` domain | `title`→`page title`, `url`→`page url`, `text`→`page text`, `attr`→`page attr`, `html`→`page html`, `eval`→`page eval` |
| 3 | Two LLM roots (Issue 6) | `llm` domain | `ollama list`→`llm ollama list`, `ai-job show`→`llm job show`, `ai-job session use`→`llm job session use` |
| 4 | `github` split + wide (Issue 1/5/6) | `github extract` sub-noun | `github extract-repo`→`github extract repo` (×issues/prs/releases) |
| 5 | Two plugin roots (Issue 6) | `plugin host` sub-noun | `plugin-host install`→`plugin host install` (the native-messaging host tree) |
| 6 | `pdf`/`pdf-form` confusion (Issue 1/4) | `pdf form` sub-noun | `pdf-form fill`→`pdf form fill`, `pdf-form fields`→`pdf form fields` |
| 7 | `vault` wide (Issue 5) | sub-nouns | `vault capture-key`→`vault key`, `vault import-captures`→`vault import` |
| 8 | `runbook` wide (Issue 5) | `runbook preset` sub-noun | `runbook presets`→`runbook preset list`, `runbook run-preset`→`runbook preset run` |

**Verb overloading (Issue 3)** is resolved structurally: once every `list`/`show`
is domain-qualified (already mostly true) and the two orphan inspect/LLM clusters
are grouped, no bare cross-cutting verb remains at root.

**Casing (Issue 2)** is already good — normalization only.

**Flag drift (Issue 8)** — enforced by R5. The mapping pass flags every local
shorthand that collides with a root persistent shorthand (the `-o` collisions
already fixed for `okf`/`flow` are the template).

## 5. Merge collisions to watch (for the mapping pass)

- `sitemap extract` **and** `map` both → `site map`. Reconcile flags; `site map`
  must expose sitemap-extraction options as flags, not a separate verb.
- `extract` (root, table/meta/ai) stays flat and is **not** merged into `site` —
  it is single-page table extraction, semantically distinct from traversal.
- `screenshot` (root), `mcp screenshot`, and `pdf` render must stay distinct;
  only `pdf-form` folds into `pdf form`.
- `vault capture` (leaf) vs `vault capture-key` vs `vault import-captures` — three
  different operations; ensure `vault key` / `vault import` do not shadow `vault
  capture`.

## 6. Deliverables produced this phase

- `docs/COMMAND-TAXONOMY.md` — standing rules contract (rev-tagged).
- `docs/superpowers/specs/2026-07-01-command-taxonomy-mapping-cli.md` — exhaustive
  CLI old→new table.
- `docs/superpowers/specs/2026-07-01-command-taxonomy-mapping-mcp.md` — MCP tool
  old→new table + parity check vs CLI.

## 7. Phase 6 staging (approved scope — not yet started)

One concern per PR, each leaving the tree buildable + tests green:
1. Merge scattered roots (`github_extract.go`→`github extract`; `plugin-host`→
   `plugin host`; `ollama`+`ai-job`→`llm`).
2. New grouping domains (`site` — incl. `test-site`→`site test`; `page`).
3. Sub-noun regroup of wide trees (`vault`, `runbook`, `pdf`).
4. Flag-drift + dead-code sweep; regenerate `cmdtree`/`--help` snapshots.
5. **MCP full-parity pass** (user-approved): rename the 8 existing tools to mirror
   the new CLI paths AND build the ~26 missing tools so every browser/data CLI
   leaf has a 1:1 MCP twin (per R6). The 10 MCP-only locator/expect/snapshot tools
   are exempt from CLI parity (see mapping-mcp orphans) and documented as such.
Ship `docs/MIGRATION-command-taxonomy.md` generated from the mapping tables.

## 8. Review-gate resolutions (locked)

- **`test-site` → `site test`** — confirmed rename (not exempted). Docs/skills/CI
  references to `test-site` must be updated in the Phase 6 `site` PR.
- **`ai-job` → `llm job`** — confirmed; `ai-` prefix dropped once nested under `llm`.
- **MCP scope: full parity now** — the ~26 missing tools are built in Phase 6
  step 5, not deferred. 10 orphan tools stay MCP-only by design.
