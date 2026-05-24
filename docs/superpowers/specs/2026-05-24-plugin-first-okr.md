# OKR — Scout as a Plugin-First / MCP-First App

Date: 2026-05-24
Status: **Draft for review** — needs approval on load-bearing decisions (§ Tradeoffs) before any code moves.
Horizon: 1 milestone (≈ Q3 2026).

## Why this exists

Scout today is a CLI-first tool with **five** parallel ingress surfaces:

1. Cobra CLI (50+ subcommands)
2. MCP server (`scout mcp`, 18 tools)
3. gRPC daemon (`scout daemon`)
4. REST agent server (`scout agent serve` — OpenAI/Anthropic schemas)
5. Subprocess plugin runtime (`scout subplugin`)
6. (New) AI-host plugin gate (`scout plugin install`)

Five ingresses means five flavors of "the same Scout capability," five places it can drift, five docs surfaces, five things to test. The pivot collapses this to **two officially supported entry points**: the **plugin gate** (CC/Codex/Gemini) and the **MCP server**. Everything else either becomes a thin shim over the same core, or gets deprecated.

This is the same architectural call unravel made (hard "AI is MCP-only" rule, no anthropic-sdk-go outside `internal/ai/` and `internal/mcp/`). Scout adopting the same rule yields: one source of truth for capabilities, one place to add tools, one set of integration tests.

## Objective 1 — Scout's primary distribution becomes the plugin gate

| KR | Target | Today |
|---|---|---|
| KR1.1 | 100% of documented user workflows reachable from a CC slash command, skill, or agent | ~12% (6 skills + 6 commands + 6 agents cover scrape/screenshot/crawl/gather/test/monitor; nothing for session/hijack/runbook/strategy/swarm/electron/mobile) |
| KR1.2 | First-party install path is `scout plugin install` | Today: `go install ./cmd/scout/`. README leads with shell usage. |
| KR1.3 | README & docs lead with plugin install; CLI demoted to "internals/advanced" section | Today: 50+ CLI subcommands documented as primary surface |
| KR1.4 | Time-to-first-tool-call from clean install ≤ 90s | Untested today |

## Objective 2 — Every user-facing capability has an MCP equivalent

| KR | Target | Today |
|---|---|---|
| KR2.1 | CLI audit complete — every Cobra command classified `keep` / `port-to-mcp` / `deprecate` | Not done |
| KR2.2 | MCP tool count: 18 → ≥ 35 | 18 |
| KR2.3 | `pkg/scout/tools/` package owns all tool implementations; both CLI handlers AND MCP handlers delegate to it (no duplication) | Today: CLI handlers call engine API; MCP handlers call engine API independently — drift risk |
| KR2.4 | MCP `prompts/` surface implemented — surfaces runbooks + strategies as user-invocable prompts | Today: prompts/ unused |
| KR2.5 | MCP `resources/` expanded beyond `scout://page/*` to include session state, HAR, hijack stream | Today: 3 resources |

## Objective 3 — Reduce ingress surface area

| KR | Target | Today |
|---|---|---|
| KR3.1 | `pkg/scout/agent/` (REST + OpenAI/Anthropic schemas) **deprecated** with removal date, marketing redirected to MCP | Today: actively maintained as a separate non-MCP AI ingress |
| KR3.2 | gRPC daemon scoped to "remote browser control only" — explicitly NOT an AI ingress, doc'd as such | Today: ambiguous role |
| KR3.3 | Single source of truth for the tool catalog — `pkg/scout/tools/` registers once, MCP server consumes the registry, CLI consumes the same registry for help text | Today: `addTracedTool()` in `pkg/scout/mcp/server.go` is one source; cobra handlers are another |

## Objective 4 — Cross-host parity

| KR | Target | Today |
|---|---|---|
| KR4.1 | All 3 hosts (Claude / Codex / Gemini) have working `Installer` | Claude only; Codex + Gemini are Walk+Manifest stubs |
| KR4.2 | Each host has a Doctor with host-specific checks (not generic stubs) | Same — only Claude is real |
| KR4.3 | `scout plugin install --host all` works | Doesn't exist |

## Tradeoffs — DECIDE BEFORE STARTING

The OKR is sound only if these load-bearing calls are confirmed up-front. Each has a fork.

### T1 — Does CLI survive as thin shims, or get removed?

- **Option A (recommended)**: CLI verbs stay as thin shims over `pkg/scout/tools/`. Scriptability preserved (cron, CI, shell loops). MCP becomes primary; CLI becomes secondary. **Breaking changes are minimal.**
- **Option B**: CLI verbs are removed for ones with MCP equivalents. Forces users into AI agents. Smaller binary, simpler docs. **Breaks every existing user.**

**Recommend A.** A plugin-first app doesn't have to be MCP-only at the user interface — it has to be MCP-only at the *AI-ingress* interface. Cron jobs aren't AI; let them keep working.

### T2 — Does `pkg/scout/agent/` (REST server) get deprecated?

- **Option A (recommended)**: Yes. Mark `Deprecated:` with a 60-day removal window. The agent HTTP server was Scout's pre-MCP answer to AI integration; MCP supersedes it. Two AI ingresses = drift forever.
- **Option B**: Keep it as a "legacy adapter" for non-MCP frameworks.

**Recommend A.** Anthropic, OpenAI, Google all converging on MCP. The REST/OpenAI-schema endpoint will rot. Mark and remove.

### T3 — Tool-unification refactor: do it now, or ship MCP coverage first and refactor later?

- **Option A (recommended)**: Build `pkg/scout/tools/` first as the single source of truth, port CLI and MCP both to consume it, THEN add new tools.
- **Option B**: Port new tools to MCP fast (duplicate engine API calls between CLI and MCP), unify later.

**Recommend A.** Doing B locks in drift; the cost of unwinding it later is the same as doing A now plus the cost of fixing drift bugs in the interim. Refactor cost is paid once; drift cost is paid every release.

### T4 — Skills & agents: do they live in the binary forever?

- **Option A (recommended)**: Yes — Go literals in `pkg/scout/aihost/claude/assets.go` is the right shape (unravel's choice). One binary ships the whole plugin.
- **Option B**: Skills/agents in `~/.scout/plugin-assets/` as editable markdown.

**Recommend A.** Editable assets break atomic upgrades — `scout plugin install` no longer guarantees a known-good state. Keep them as literals; offer `scout plugin extract` for users who want to fork.

### T5 — Naming: keep `scout subplugin` for the subprocess JSON-RPC runtime?

- **Option A (recommended)**: Yes. The name was created today and is honest about what it is.
- **Option B**: Roll the subprocess plugin runtime into the AI-host plugin model.

**Recommend A.** They solve different problems. AI-host plugins ship Scout INTO an AI host; subprocess plugins ship third-party capabilities INTO Scout. Different direction, different lifecycle.

## Initiatives (the work — in order)

1. **CLI audit** — every Cobra subcommand classified: `keep` (sysadmin, can't be MCP'd: install, session reset, daemon start, repl, mcp serve itself), `port-to-mcp` (user-facing workflows: scrape, gather, crawl, test-site, monitor, hijack, runbook, strategy, swarm), `deprecate` (REST agent, anything superseded). **Deliverable:** `docs/superpowers/specs/2026-XX-cli-audit-matrix.md` with the table.

2. **Build `pkg/scout/tools/`** — extract the actual *capability* implementations from CLI handlers and MCP handlers into one package. Each capability is a function with typed input/output, no transport concerns. **Acceptance:** CLI handlers and MCP handlers both ≤ 20 lines (parse + delegate + format).

3. **Port chosen verbs to MCP** — for every `port-to-mcp` entry from the audit, add `pkg/scout/mcp/tools_<group>.go` delegating to the tools package. Tests run both surfaces against the same test fixture and assert byte-identical results.

4. **CLI demotion** — Cobra help text for each ported verb says: "Also available as MCP tool `mcp__scout__<name>`." Update README to lead with plugin install.

5. **Deprecate REST agent server** — add `Deprecated:` to Cobra command, `slog.Warn` on startup, README note, 60-day removal date.

6. **Codex + Gemini Installer** — implement the real install paths once their plugin surfaces stabilise. Until then, ship `extract --host codex|gemini` as the manual fallback.

7. **MCP prompts + resources expansion** — surface runbooks (`pkg/scout/runbook/`) as MCP prompts; surface session state / HAR / hijack stream as MCP resources.

8. **`scout plugin install --host all`** — iterate `aihost.All()`, install into every host that has an Installer.

## Non-goals

- Killing the CLI. Scout still runs from a shell.
- Killing the subprocess plugin runtime (`scout subplugin`). It serves a different purpose.
- Killing the gRPC daemon. It serves remote browser control; that's a legitimate non-AI use case.
- Multi-tenant MCP. Scout's MCP server is per-process today; that stays.

## Success metric (rolled up)

> 90 days after this OKR lands: a new user can install Claude Code, run `scout plugin install`, restart CC, and complete every Scout workflow documented in the README via slash commands / skills / agents — without ever opening a terminal beyond the initial install.

Today: only `scrape`, `screenshot`, `crawl`, `gather`, `test-site`, `monitor` are reachable that way. The other 80% of Scout's capability surface still requires shell.

## Next concrete step

Approve T1–T5 (above). Then I run the CLI audit (initiative 1) and produce the classification matrix. Refactor doesn't start until the audit is reviewed.
