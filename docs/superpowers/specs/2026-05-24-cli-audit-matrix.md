# CLI Audit Matrix — keep / port-to-mcp / deprecate

Date: 2026-05-24
Status: **Draft for review** — Initiative #1 of the plugin-first OKR (`2026-05-24-plugin-first-okr.md`). No refactor starts until this is approved.
Surface measured: 60 top-level cobra verbs, 188 leaf commands across `cmd/scout/*.go` (79 files).

## Methodology

Every `&cobra.Command{Use: …}` literal was extracted programmatically; parent→child edges were resolved by scanning `<parent>Cmd.AddCommand(...)` calls. Each verb is classified:

- **KEEP** — fundamentally a CLI concern (process management, install, daemon control, interactive shell, the MCP server itself).
- **PORT-TO-MCP** — user-facing workflow that's better invoked from an AI agent than a shell. CLI shim stays (per OKR T1) but the canonical surface becomes the MCP tool.
- **DEPRECATE** — superseded by a more native path; add `Deprecated:` marker, log on use, remove after 60 days (per CLAUDE.md deprecation policy).

Classification roll-up:

| Bucket | Count | Notes |
|---|---|---|
| KEEP | 24 | Mostly process/setup/admin |
| PORT-TO-MCP | 28 | All user-facing browser workflows |
| DEPRECATE | 5 | REST agent + redundant/wrapped surfaces |
| Already MCP | 19 | Today's MCP tool set |
| Hybrid (both surfaces stay first-class) | 3 | session, browser, jobs |

Total: 60 top-level verbs (some span buckets via sub-commands; ratios are leaf-level).

## KEEP (CLI is the right home — do not move)

These verbs cannot or should not be MCP. They manage processes, install state, system identity, or are themselves the MCP transport.

| Verb | Sub-verbs | Why KEEP |
|---|---|---|
| `plugin` | install, uninstall, status, extract, doctor, hosts | Plugin install is the bootstrap; chicken-and-egg with MCP. |
| `subplugin` | install, remove, list, run, search, update, check-updates | Subprocess plugin runtime is system-level — same. |
| `mcp` | (serve) | The MCP server itself. Can't be MCP. |
| `daemon` / `server` / `grpc` | attr, back, block, … (24 sub-verbs) | gRPC daemon control. Scoped to "remote browser, not AI ingress" per OKR T2. CLI is the right surface. |
| `setup` | — | First-run wizard. Interactive. |
| `repl` | — | Interactive shell. Antithesis of MCP. |
| `connect` / `client` | — | CDP/remote-browser connect helpers. CLI scripting. |
| `device` | id, list, pair, trust, discover, remove | Device identity — system-level, OS keychain integration. |
| `vpn` | connect, disconnect, servers, status | System network state. |
| `cloud` | deploy, scale, status, uninstall | DevOps / CI workflow. |
| `bridge` (control surfaces only) | status, observe, tabs, frames | Bridge extension control. Note: bridge `click`/`type`/`dom`/etc. are duplicates of MCP tools → see DEPRECATE. |
| `browser` | download, list | Install management. CLI. |
| `extension` | download, list, load, remove, test | Install management. |
| `mobile` | connect, devices | ADB session setup. |
| `update` | check | Self-update bootstrap. |
| `version` | — | Trivial. (Currently missing — see Quick Wins.) |
| `completion` | — | Shell completion. |
| `aicontext` / `cmdtree` | — | Help-text generators. |
| `swagger` | — | OpenAPI gen for the (deprecating) REST agent — keep until agent is gone, then remove. |
| `logger` | — | Command-logging admin. |
| `ai-job` | list, show, session | LLM-job admin (Scout's own LLM jobs, not user AI). |
| `ollama` | list, pull, status | Ollama model admin. |
| `auth` | login, logout, status, providers, capture, replay, show | Scraper auth admin. Per-platform credential vault. CLI-natural. |

## PORT-TO-MCP (user-facing workflows — ship as MCP tools, keep CLI as thin shim)

For each, the **proposed MCP tool name** mirrors the CLI verb (already convention: `scout swarm start` → `mcp__scout__swarm_crawl`). Cross-surface tests assert the CLI shim and the MCP tool produce byte-identical output.

| CLI verb | Proposed MCP tool | Notes |
|---|---|---|
| `scrape run` | `scrape` | Authenticated scraping. Mode arg picks the platform handler. |
| `crawl` | `crawl` | Single-host crawl (distinct from `swarm`). |
| `gather` | `gather` | Already ported as skill; promote to first-class tool. |
| `test-site` | `test_site` | Already ported as skill; promote to first-class tool. |
| `sitemap extract` | `sitemap` | Sitemap discovery + DOM crawl. |
| `runbook plan` | `runbook_plan` | Dry-run a runbook. |
| `runbook apply` | `runbook_apply` | Execute a runbook. |
| `runbook validate` | `runbook_validate` | Schema check. |
| `runbook create` / `presets` / `run-preset` / `flow` / `fix` / `sample` | `runbook_*` family | Each becomes a tool; or collapse into one `runbook` tool with `op` arg. **Decision needed.** |
| `strategy run` | `strategy_run` | Workflow execution. |
| `strategy init` / `validate` | `strategy_*` | Same family question. |
| `hijack watch` | `hijack_watch` | Streaming output → use MCP notifications. **Streaming proves the prompts/resources expansion path.** |
| `har start` / `stop` / `export` | `har_*` | Network recording control. |
| `form detect` / `fill` / `submit` | `form_*` | DOM forms. |
| `extract meta` / `table` / `ai` | `extract_meta` / `extract_table` / `extract_ai` | Three flavors. Existing `extract` MCP tool is selector-based; these are higher-level. |
| `proxy start` / `routes` | `proxy_start` / `proxy_routes` | Long-running — see streaming note for `hijack`. |
| `report list` / `show` / `delete` / `schedule` | `report_*` | Query/manage saved reports. Read-side perfect for MCP. |
| `github *` (12 sub-verbs) | `github_*` | GitHub scraper. Each sub-verb → one MCP tool. |
| `webmcp discover` / `inspect` / `call` | `webmcp_*` | Discover MCP tools exposed by web pages. Meta. |
| `dom insert` / `remove` | `dom_insert` / `dom_remove` | Bridge-style DOM ops. |
| `window get` + state setters | `window_*` | Window control. |
| `fingerprint generate` / `apply` | `fingerprint_*` | Session-time. |
| `challenge detect` / `solve` | `challenge_*` | Anti-bot. |
| `storage get` / `set` / `list` / `clear` | `storage_*` | localStorage/sessionStorage. Risk: overlaps with `session-capture` agent → coordinate naming. |
| `cookie get` / `set` / `clear` | `cookie_*` | Cookies. Same coordination note. |
| `profile capture` / `load` / `show` / `diff` / `merge` / `session-capture` / `session-load` | `profile_*` | Browser-identity bundle. |
| `inject` | `inject` | Code injection. |
| `record` | `record` | Interaction recording → feeds `flow-porter` agent. |
| `guide` | `guide` | Step-by-step guide builder. Recorder + Markdown renderer. |
| `knowledge` | `knowledge` | Knowledge-base extraction. |
| `swarm start` / `join` / `status` | `swarm_start` / `swarm_join` / `swarm_status` | `swarm_crawl` already exists; add lifecycle ops. |
| `ws listen` (existing in MCP) | — | Already there. |
| `upload file` / `auth` / `status` | `upload_*` | Cloud upload. Auth flow is interactive — KEEP that sub-verb in CLI; port the others. |
| `jobs list` / `status` / `cancel` | `jobs_*` | Async job introspection. **Hybrid** — keep CLI for `cancel` (often used from shell), port read-side to MCP. |
| `session list` / `use` | `session_list` exists; add `session_use` | **Hybrid** — `create`/`reset`/`destroy`/`prune` stay CLI (admin). `list`/`use` already MCP. |
| `fetch` | `fetch` | Lightweight HTTP. |
| `detect` | `detect` | Framework detection. |
| `map` | `map` | Site mapping — overlaps with `site-mapper` agent. **Decision: agent stays, this becomes its underlying tool.** |
| `batch` | `batch` | Multi-URL queue. |
| `research` | `research` | Multi-page research with depth presets. |

**MCP tool count after porting:** 19 (today) + ~35 (this list, after family-collapse decisions) = **~54**. Beats KR2.2 target of ≥35 by ~50%.

## DEPRECATE (60-day removal window per CLAUDE.md policy)

| Verb | Why | Replacement |
|---|---|---|
| `agent serve` | Per OKR T2 — REST + OpenAI/Anthropic schemas is a second AI ingress that competes with MCP. Drift forever. | MCP server (`scout mcp`). Existing `agent` package gets `Deprecated:` marker, slog.Warn on startup. |
| `agent tools` | Same — generates schemas only the REST server consumes. | — |
| `swagger` | OpenAPI generator for the REST agent. Goes with `agent`. | — |
| `bridge click` / `type` / `dom` / `query` / `send` / `clipboard` / `ws-send` / `emit` / `call-exposed` / `events` / `listen` / `record` | Duplicates MCP tools (`mcp__scout__click`, `type`, `eval`, `ws_send`, etc.). The bridge transport is fine; the duplicated user-facing verbs are not. | Existing MCP tools. Keep `bridge status`/`observe`/`tabs`/`frames` (introspection). |
| `pdf-form` | Likely superseded by `pdf` MCP tool with a form-mode arg. **Verify before deprecating** — `pdf-form fields` may have functionality `pdf` lacks. | `pdf` MCP tool with `op: "form_fields" \| "form_fill"`. |
| `mcp screenshot` / `mcp open` | `mcp` subcommand is the server. These sub-verbs are convenience shortcuts that confuse the namespace. | `scout screenshot <url>` (top-level, already exists). |

## Quick wins (out-of-band, would close gaps the audit surfaced)

1. **`scout --version` doesn't exist** — caught earlier in the session. Cheap addition; add a `version` cobra subcommand wired to `runtime/debug.ReadBuildInfo` so `scout --version` and `scout version` both work.
2. **`bridge` namespace cleanup** — 13 bridge sub-verbs are duplicates of MCP tools. Mark them deprecated as a single batch.
3. **`session` namespace** is 9 sub-verbs; only `list` is MCP today. Add `session_use` to MCP — closes the "switch sessions from an AI flow" gap.

## Decisions needed before Initiative #2 starts

| # | Question | Recommendation |
|---|---|---|
| D1 | `runbook` and `strategy` families — one MCP tool per sub-verb (8+3) or one tool with `op` discriminator (2)? | **Per-sub-verb.** Better for MCP `tools/list` discoverability; AI agents do better with narrow tools than `op` switches. |
| D2 | Streaming verbs (`hijack watch`, `proxy start`) — return chunked notifications or block-and-return-on-stop? | **Notifications.** Forces us to validate streaming MCP early — feeds the prompts/resources expansion (Initiative #7) too. |
| D3 | Naming coordination — `storage_*` MCP tools vs `session-capture` agent. Both touch localStorage. | Tools = primitives, agent = orchestrated workflow. Agent uses the tools. Names don't collide. |
| D4 | `pdf-form` deprecation — verify or keep? | Spike: run `scout pdf-form fields` on a sample PDF, compare to what `pdf` can do today. If `pdf` can match, deprecate. |
| D5 | gRPC `daemon`/`server` verbs (24 sub-commands like `daemon click`, `daemon back`, …) — they mirror MCP tools but over gRPC. Do they STAY as the "remote browser" surface, or get deprecated too? | **Stay** per OKR T2. They serve remote browser control (genuine non-AI use case). Doc as such. |

## Next concrete step

Approve the matrix (or push back on classifications/decisions D1–D5). Then Initiative #2 starts: build `pkg/scout/tools/` as the single source of truth, with the first vertical slice being one of the simplest port-to-mcp verbs (recommend `gather` since it's already a skill — completes the loop from skill → underlying tool).
