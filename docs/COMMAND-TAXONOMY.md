# Scout Command Taxonomy — Rules Contract
<!-- rev:001 -->

Standing rules for how Scout's command surfaces are named and grouped. Every new
command (CLI) and every new tool (MCP) MUST conform. When in doubt, this file
wins over ad-hoc precedent. Companion point-in-time docs: the dated audit
(`docs/command-taxonomy-audit.md`) and the redesign spec/mappings under
`docs/superpowers/specs/`.

## Scope decisions (2026-07-01 redesign)

- **Scope:** full cleanup (all 10 audit issue classes).
- **Migration:** **clean break** — rename in place, no aliases. Justified by the
  project's Stabilization-Milestone constraint ("Breaking changes: Acceptable —
  clean API over backwards compat"), which overrides the global ≥30-day
  deprecation policy. A `docs/MIGRATION-command-taxonomy.md` ships with the change.
- **Surfaces:** CLI (Cobra) **and** MCP tools (`pkg/scout/mcp/`), kept 1:1 aligned.
- **Shape:** shallow grouping — a fixed allowlist of hot verbs stays flat; crowded
  domains get a sub-noun layer.

## Rules

### R1 — Path shape: noun → verb
Commands read `<domain> [<sub-noun>] <verb>`. No bare verbs at root **except** the
hot-verb allowlist (R7). `session list`, not `list-sessions`; `github extract repo`,
not `extract-github-repo`.

### R2 — Casing
Lowercase kebab-case for every path segment. Compound verbs keep their hyphen
(`test-site`, `check-updates`, `capture-host`). No snake_case, no runtogether.

### R3 — Sub-noun threshold
Add a grouping layer only when a domain has **~6+ sibling verbs** or a
self-evident cluster (e.g. the four `github extract-*` commands). Small domains
(≤5 verbs) stay flat under their domain noun.

### R4 — Cross-cutting verbs live under a noun
`list`, `show`, `status`, `detect`, `info`, `get`, `set` are never bare at root
and never ambiguous. They are always qualified by a domain (`plugin list`,
`vpn status`, `page detect`), so the verb's meaning is unambiguous from its path.

### R5 — One canonical flag name per concept
Each concept has exactly one flag name across the whole CLI (e.g. output path is
always `--output`/`-o`). Synonyms are dropped. Commands do **not** re-declare the
root persistent flags (`-o/--output`, `-v/--verbose`, `--format`, `--session`,
`--browser`, `--headless`, `--stealth`, `--devtools`). Local shorthands must not
collide with a global persistent shorthand (see the `okf --out`/`flow --out`
`-o` collision that this rule prevents).

### R6 — MCP mirrors the CLI 1:1
Every CLI leaf command that has a browser/data capability has a matching MCP tool,
and vice versa. The tool name is the CLI path with segments joined by `_`:
`session list` → `session_list`; `github extract repo` → `github_extract_repo`;
`page eval` → `page_eval`. Renaming a CLI path renames its MCP tool in the same
change. Neither surface silently drops a capability — it is renamed, not deleted.

### R7 — The hot-verb allowlist (the only bare-root verbs)
These stay flat at root because they are the primary user actions or unavoidable
meta commands. Nothing else may be added here without updating this list:

- **Actions:** `extract`, `detect`, `scrape`, `fetch`, `research`, `screenshot`,
  `pdf`, `snapshot`, `search`, `batch`, `repl`, `guide`, `record`, `inject`,
  `swagger`, `okf`, `setup`.
- **Meta:** `mcp`, `logger`, `version`, `completion`, `aicontext`, `cmdtree`,
  `update`.

(`extract`, `scrape`, `pdf`, `mcp`, `search`, `update` carry their own
subcommands but keep a flat root because the bare form is a primary action.)

## Domain map (target)

Grouping domains (noun-first, `<domain> [<sub-noun>] <verb>`):

| Domain | Sub-nouns / verbs | Notes |
|--------|-------------------|-------|
| `site` | `crawl`, `gather`, `map`, `knowledge`, `test` | NEW. Absorbs root `crawl`, `gather`, `map`, `knowledge`, `test-site`; `sitemap` merges into `site map`. |
| `page` | `title`, `url`, `text`, `attr`, `html`, `eval` | NEW. Absorbs the 6 bare root inspect verbs. |
| `llm`  | `ollama <list\|pull\|status>`, `job <list\|show>`, `job session <list\|create\|use>` | NEW. Absorbs root `ollama` and `ai-job`. |
| `github` | `repo`, `issues`, `prs`, `user`, `releases`, `tree`, `code`, `extract <repo\|issues\|prs\|releases>` | Merges `github_extract.go` under an `extract` sub-noun. |
| `plugin` | `list install remove run search update check-updates`, `host <install\|uninstall\|status\|extract\|doctor\|hosts>` | Merges the second `plugin-host` root under a `host` sub-noun. |
| `vault` | `init set get list use rm rotate capture`, `key <…>`, `import` | `capture-key`→`vault key`, `import-captures`→`vault import`. |
| `runbook` | `apply validate create plan fix sample flow`, `preset <list\|run>` | `presets`/`run-preset` → `preset` sub-noun. |
| `pdf` | (render, flat) + `form <fill\|fields>` | Folds `pdf-form` under `pdf form`. |
| Conforming domains kept as-is | `auth session browser flow form proxy strategy vpn report webmcp bridge ws challenge fingerprint interactions jobs profile extension scrape capture-host mcp search extract` | Casing + flag normalization only. |

## How to add a command (checklist)

1. Pick the domain noun. If none fits and it is a primary action, consider R7 —
   but adding to the allowlist requires editing this doc.
2. Apply R1–R2 to the path.
3. If the domain now has ≥6 siblings, introduce a sub-noun (R3).
4. Reuse canonical flag names; do not re-declare root persistent flags (R5).
5. Add the matching MCP tool with the `_`-joined name (R6).
6. Update the `--help` snapshot / cmdtree.
