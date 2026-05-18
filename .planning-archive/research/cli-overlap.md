# CLI Command Overlap Analysis - Research

**Researched:** 2026-03-29
**Domain:** Scout CLI command structure (`cmd/scout/`)
**Confidence:** HIGH (direct codebase analysis)

## Summary

Scout's CLI has **~60 top-level commands** registered on `rootCmd`, totaling **~200 cobra.Command definitions** across 80 source files. The command surface is extremely large for a single binary. Several clear overlap clusters exist where multiple commands provide similar or identical functionality through different interfaces (standalone vs gRPC daemon, recipe vs runbook, search vs websearch, multiple auth-capture flows).

The biggest structural issue is the split between **standalone commands** (launch browser directly via `baseOpts` + `scout.New()`) and **daemon commands** (talk to gRPC server via `resolveClient`). Many commands exist in both forms without clear naming to distinguish them.

**Primary recommendation:** Consolidate overlapping commands into grouped subcommands, remove deprecated `recipe` (deadline 2026-04-15), and unify the three auth-capture flows under a single `auth` parent.

---

## Command Inventory

### Top-Level Commands Registered on rootCmd

Total `rootCmd.AddCommand` calls: **~60** (some files register multiple, e.g., `inspect.go` registers 6, `interact.go` registers 7, `navigate.go` registers 4, `network.go` registers 3, `screenshot.go` registers 2, `extract.go` registers 2, `llm.go` registers 3).

#### Standalone Commands (launch own browser via `scout.New()` + `baseOpts`)

| Command | File | Description |
|---------|------|-------------|
| `batch` | batch.go | Batch scrape multiple URLs concurrently |
| `crawl` | crawl.go | Crawl a website starting from a URL |
| `detect` | detect.go | Detect frameworks, PWA, render mode, tech stack |
| `fetch` | fetch.go | Fetch URL and extract structured content |
| `gather` | gather.go | One-shot page intelligence collector |
| `guide` | guide.go | Record step-by-step guide with screenshots |
| `inject` | inject.go | Open page with injected JavaScript |
| `knowledge` | knowledge.go | Crawl site, collect all intelligence |
| `map` | map.go | Discover URLs via sitemap + link harvesting |
| `markdown` | markdown.go | Convert web page to Markdown |
| `mcp open` | mcp_open.go | Open URL in visible browser |
| `mcp screenshot` | mcp_screenshot.go | Take screenshot of a URL |
| `record` | record.go | Record browser screen as animated GIF |
| `research` | research.go | Search + fetch + LLM synthesis |
| `search` | search.go | Search web (Google/Bing/DDG) |
| `sitemap extract` | sitemap.go | Crawl + extract DOM JSON + Markdown |
| `snapshot` | snapshot.go | Accessibility tree snapshot |
| `swagger` | swagger.go | Detect/extract Swagger/OpenAPI spec |
| `test-site` | testsite.go | Health check (broken links, JS errors) |
| `websearch` | websearch.go | Search web + optionally fetch results |
| `repl` | repl.go | Interactive local browser shell |

#### gRPC Daemon Commands (talk to server via `resolveClient`)

| Command | File | Description |
|---------|------|-------------|
| `screenshot` | screenshot.go | Screenshot via gRPC session |
| `pdf` | screenshot.go | PDF via gRPC session |
| `navigate` | navigate.go | Navigate via gRPC |
| `back` | navigate.go | Back via gRPC |
| `forward` | navigate.go | Forward via gRPC |
| `reload` | navigate.go | Reload via gRPC |
| `click` | interact.go | Click via gRPC |
| `type` | interact.go | Type via gRPC |
| `select` | interact.go | Select via gRPC |
| `hover` | interact.go | Hover via gRPC |
| `focus` | interact.go | Focus via gRPC |
| `clear` | interact.go | Clear via gRPC |
| `key` | interact.go | Keypress via gRPC |
| `title` | inspect.go | Get title via gRPC |
| `url` | inspect.go | Get URL via gRPC |
| `text` | inspect.go | Get text via gRPC |
| `attr` | inspect.go | Get attribute via gRPC |
| `eval` | inspect.go | Eval JS via gRPC |
| `html` | inspect.go | Get HTML via gRPC |
| `cookie get/set/clear` | network.go | Cookie management via gRPC |
| `header` | network.go | Set header via gRPC |
| `block` | network.go | Block URLs via gRPC |
| `window get/min/max/full/restore` | window.go | Window state via gRPC |
| `storage get/set/list/clear` | storage.go | Web storage via gRPC |

#### Grouped Parent Commands (with subcommands)

| Parent | Subcommands | File(s) |
|--------|-------------|---------|
| `agent` | serve, tools | agent.go |
| `auth` | login, capture, status, logout, providers | auth.go |
| `bridge` | status, send, listen, observe, events, query, click, type, dom (insert/remove), tabs, clipboard, ws-send, call-exposed, emit, frames, record | bridge.go |
| `browser` | list, download | browser.go |
| `challenge` | detect, solve | challenge.go |
| `cloud` | deploy, status, scale, uninstall | cloud.go |
| `credentials` | capture, replay, show | credentials.go |
| `device` | id, trust, list, remove, pair, discover | device.go |
| `extension` | download, remove, load, test, list | extension.go |
| `fingerprint` | generate, apply | fingerprint.go |
| `form` | detect, fill, submit | form.go |
| `github` | repo, issues, prs, user, releases, code, tree, extract-repo, extract-issues, extract-prs, extract-releases | github.go, github_extract.go |
| `har` | start, stop, export | har.go |
| `hijack` | watch | hijack.go |
| `jobs` | list, status, cancel | jobs.go |
| `mcp` | (stdio server), open, screenshot | mcp.go, mcp_open.go, mcp_screenshot.go |
| `mobile` | devices, connect | mobile.go |
| `ollama` | list, pull, status | llm.go |
| `ai-job` | list, show, session (list/create/use) | llm.go |
| `pdf-form` | fields, fill | pdf.go |
| `plugin` | list, install, remove, run, search, update, check-updates | plugin.go |
| `profile` | capture, load, show, merge, diff, session-capture, session-load | profile.go |
| `proxy` | start, routes | proxy.go |
| `recipe` | run, validate, test, create, fix, sample, presets, run-preset, flow | recipe.go (DEPRECATED) |
| `report` | list, show, delete, schedule (stop) | report.go, report_schedule.go |
| `runbook` | apply, validate, plan, create, fix, sample, presets, run-preset, flow | runbook.go |
| `scrape` | list, auth, run | scrape.go |
| `search` | google, bing, duckduckgo, wikipedia | search.go, search_engines.go |
| `session` | create, destroy, list, use, list-local, prune, clean, rm, reset | session.go |
| `sitemap` | extract | sitemap.go |
| `strategy` | run, validate, init | strategy.go |
| `swarm` | start, join, status | swarm.go |
| `update` | check | update.go |
| `upload` | auth, file, status | upload.go |
| `vpn` | status, connect, disconnect, servers | vpn.go |
| `webmcp` | discover, call, inspect | webmcp.go |
| `window` | get, minimize, maximize, fullscreen, restore | window.go |
| `ws` | listen | websocket.go |

#### Other Top-Level (no subcommands)

| Command | File | Description |
|---------|------|-------------|
| `aicontext` | aicontext.go | Generate AI context for coding agents |
| `client` | client.go | Interactive gRPC client REPL |
| `cmdtree` | cmdtree.go | Display command tree visualization |
| `completion` | completion.go | Shell completion scripts |
| `connect` | connect.go | Connect to running browser via CDP |
| `extract-ai` | llm.go | LLM-based structured extraction |
| `logger` | logger.go | Configure command logging |
| `open` | mcp_open.go | (under mcp) |
| `server` | server.go | Start gRPC server |
| `setup` | setup.go | Configure Scout as AI assistant plugin |
| `table` | extract.go | Extract table data |
| `meta` | extract.go | Extract page metadata |
| `version` | version.go | Print version |

---

## Overlap Clusters

### 1. CRITICAL: `recipe` is a full duplicate of `runbook` (DEPRECATED)

**Files:** `recipe.go` (666 lines) vs `runbook.go` (similar structure)

`recipe` has `Deprecated: "Use 'scout runbook' instead. Will be removed after 2026-04-15."` -- only 17 days away.

Both share identical subcommand structure: `run/validate/create/test(plan)/fix/sample/presets/run-preset/flow`. The `recipe` commands internally call `runbook.LoadFile()` -- they are pure wrappers.

**Action:** Remove `recipe.go` entirely after 2026-04-15.

### 2. HIGH: Three Authentication/Credential Capture Flows

**Files:** `auth.go`, `credentials.go`, `profile.go`

| Command | What it captures | Storage format | Encryption |
|---------|-----------------|----------------|------------|
| `auth login` | Provider-specific login (Google, GitHub, etc.) | AES-encrypted session file | Yes (passphrase) |
| `auth capture` | Generic URL - cookies + localStorage + sessionStorage | AES-encrypted session file | Yes (passphrase) |
| `credentials capture` | Generic URL - cookies + localStorage + sessionStorage | JSON file | No (plaintext) |
| `credentials replay` | Restores captured session | N/A | N/A |
| `profile capture` | Cookies + localStorage + sessionStorage + UA + viewport | `.scoutprofile` file | Optional |
| `profile load` | Restores profile | N/A | N/A |

**Overlap:** `auth capture`, `credentials capture`, and `profile capture` all do essentially the same thing: open a browser, let user interact, capture browser state on close. The differences are:
- `auth` encrypts by default, `credentials` doesn't, `profile` is optional
- `profile` also captures UA/viewport metadata and supports merge/diff
- `credentials` has a `--on-close` flag for automatic capture

**Recommendation:** Merge `credentials` into `auth` (add `--plaintext` flag). Keep `profile` separate as it serves a distinct "portable identity" purpose with merge/diff.

### 3. HIGH: Search Command Fragmentation

**Files:** `search.go`, `search_engines.go`, `websearch.go`, `research.go`

| Command | Engine | Fetches pages? | LLM? |
|---------|--------|---------------|------|
| `search <query>` | google/bing/ddg (via `--engine`) | No | No |
| `search google <query>` | Google only | No | No |
| `search bing <query>` | Bing only | No | No |
| `search duckduckgo <query>` | DDG only | No | No |
| `search wikipedia <query>` | Wikipedia | No | No |
| `websearch <query>` | google/bing/ddg (via `--engine`) | Yes (optional `--fetch`) | No |
| `research <query>` | google/bing/ddg (via `--engine`) | Yes | Yes (LLM synthesis) |

**Overlap:** `search` and `websearch` are nearly identical -- `websearch` is `search` + optional page fetching. The engine-specific subcommands (`search google`, `search bing`, etc.) duplicate what `search --engine=X` already does.

**Recommendation:** Merge `search` into `websearch` (it's a superset). Remove `search_engines.go` subcommands (redundant with `--engine` flag). Keep `research` separate (LLM is a distinct capability).

### 4. MEDIUM: Crawling / Site Discovery Fragmentation

**Files:** `crawl.go`, `map.go`, `sitemap.go`, `knowledge.go`, `swarm.go`, `test-site.go`

| Command | What it does | Output |
|---------|-------------|--------|
| `crawl <url>` | BFS crawl, follows links | URL + depth + link count per page |
| `map <url>` | Sitemap.xml + link harvesting | URL list |
| `sitemap extract <url>` | Crawl + extract DOM JSON + Markdown per page | Files per page |
| `knowledge <url>` | Crawl + gather everything (md, HTML, links, meta, cookies, screenshots, HAR, tech stack, etc.) | Directory tree or JSON |
| `swarm start <url>` | Distributed crawl with multiple workers | Title + links per page |
| `test-site <url>` | Crawl checking for broken links, JS errors | Health report |

**Overlap:** `crawl`, `map`, and `sitemap extract` all crawl websites but extract different amounts of data. `knowledge` is a superset of all three. `swarm` is `crawl` distributed.

**Recommendation:** Consider grouping under `crawl` parent:
- `scout crawl <url>` (current crawl behavior, default)
- `scout crawl map <url>` (URL discovery only)
- `scout crawl extract <url>` (current sitemap extract)
- `scout crawl deep <url>` (current knowledge)
- `scout crawl swarm <url>` (distributed)

Keep `test-site` separate -- it's a health check tool, not a crawl tool.

### 5. MEDIUM: Screenshot Dual Implementation

**Files:** `screenshot.go` (gRPC), `mcp_screenshot.go` (standalone)

| Command | Mode | Takes URL? |
|---------|------|-----------|
| `scout screenshot` | gRPC daemon | No (uses current session page) |
| `scout mcp screenshot <url>` | Standalone | Yes |

The root `screenshot` command requires a running daemon session. The `mcp screenshot` command is standalone and takes a URL. These serve different use cases but have confusing naming.

**Recommendation:** Add `--url` flag to root `screenshot` for standalone mode, or rename `mcp screenshot` to `screenshot` with `--url` for standalone use.

### 6. MEDIUM: `fetch` vs `markdown` vs `gather`

**Files:** `fetch.go`, `markdown.go`, `gather.go`

| Command | What it returns |
|---------|----------------|
| `fetch --mode=markdown` | Markdown of page |
| `markdown --url <url>` | Markdown of page |
| `fetch --mode=full` | Markdown + HTML + meta + links |
| `gather <url>` | DOM + HAR + links + screenshots + cookies + metadata + console log + frameworks + accessibility |

`markdown` is a subset of `fetch --mode=markdown`. They produce the same output.

**Recommendation:** Deprecate `markdown` in favor of `fetch --mode=markdown`. Add alias for backward compat.

### 7. LOW: `table` and `meta` as Top-Level Commands

**File:** `extract.go`

`table` and `meta` are top-level commands that extract table data and page metadata respectively. They would fit better under a parent like `extract table` and `extract meta` (alongside `extract-ai` which is currently also top-level).

### 8. LOW: Bare Interaction Commands at Root Level

**Files:** `inspect.go`, `interact.go`, `navigate.go`

These register **17 bare commands** at root level: `title`, `url`, `text`, `attr`, `eval`, `html`, `click`, `type`, `select`, `hover`, `focus`, `clear`, `key`, `navigate`, `back`, `forward`, `reload`. All require the gRPC daemon.

These clutter `--help` significantly. Could be grouped under `page` or `browser` parent, but this would be a breaking change to the CLI interface.

### 9. LOW: `client` vs `repl`

**Files:** `client.go`, `repl.go`

| Command | Mode | Requires daemon? |
|---------|------|-----------------|
| `client` | Interactive gRPC client REPL | Yes |
| `repl` | Interactive local browser shell | No |

Both are REPLs but for different modes. Naming is clear enough but could confuse new users.

### 10. LOW: GitHub Duplicate Subcommands

**Files:** `github.go`, `github_extract.go`

`github.go` has: `repo`, `issues`, `prs`, `user`, `releases`, `code`, `tree`
`github_extract.go` adds: `extract-repo`, `extract-issues`, `extract-prs`, `extract-releases`

The `extract-*` variants provide "detailed metadata" and "unified format" but overlap with the base commands. These could be merged with a `--detailed` or `--extract` flag.

---

## Deprecated Commands

| Command | Deprecation Message | Removal Date | Status |
|---------|-------------------|--------------|--------|
| `recipe` (and all subcommands) | "Use 'scout runbook' instead" | 2026-04-15 | 17 days remaining |
| `--addr` flag (root) | "deprecated, use --target" | Not specified | No removal date |

---

## Shared Helpers Analysis (`helpers.go`)

**File:** `cmd/scout/helpers.go` (144 lines)

Functions provided:
- `writeOutput(cmd, data, defaultName)` -- write bytes to `--output` or stdout
- `readPassphrase(w, prompt)` -- secure passphrase input (env var or terminal)
- `readPassphraseConfirm(w)` -- double passphrase entry with match check
- `isHeadless(cmd)` -- read `--headless` flag
- `browserOpt(cmd)` -- `WithBrowser` from `--browser` flag
- `stealthOpts(cmd)` -- `WithStealth` from `--stealth` flag
- `baseOpts(cmd)` -- combines headless + no-sandbox + browser + stealth + system-browser + electron flags
- `truncate(s, maxLen)` -- string truncation

### Patterns

**Consistent:**
- All standalone commands use `baseOpts(cmd)` for browser launch options
- All commands use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` for output
- Error wrapping follows `fmt.Errorf("scout: action: %w", err)` pattern
- Browser cleanup uses `defer func() { _ = b.Close() }()`

**Inconsistent:**
- `fetch` and `markdown` use `--url` as a flag; most other commands use positional args for URL
- `fetch` accepts both `--url` and positional; `markdown` only accepts `--url`
- Some commands have `--format` flag for JSON output; others use the root `--format` persistent flag; some ignore it entirely
- `writeOutput` is used inconsistently -- some commands handle file output themselves
- `domainFromURL` in `knowledge.go` is a local helper that could be in `helpers.go`

### Missing from helpers:
- No shared `resolveClient`/`resolveSession` helper visible in helpers.go (likely in `client.go` or `daemon.go`)
- No URL normalization helper (each command does its own `strings.HasPrefix(url, "http")` check)
- No shared progress/spinner helper

---

## Command Grouping Proposal

### Phase 1: Remove deprecated (immediate, post-2026-04-15)

- Delete `recipe.go` entirely

### Phase 2: Merge clear duplicates (low risk)

1. **Merge `credentials` into `auth`:**
   - `auth capture` already exists; add `--plaintext` flag
   - `auth replay` (from credentials replay)
   - `auth show` (from credentials show, auth status already similar)
   - Deprecate `credentials` with 30-day notice

2. **Merge `search` into `websearch`:**
   - `websearch` is a superset of `search`
   - Remove `search_engines.go` subcommands (redundant with `--engine`)
   - Rename `websearch` to `search` (the better name)
   - Keep `search wikipedia` as it's a unique engine

3. **Deprecate `markdown`:**
   - `fetch --mode=markdown` already does this
   - Add deprecation notice pointing to `fetch`

### Phase 3: Regroup crawling (medium risk)

Create `crawl` parent with subcommands:
```
scout crawl <url>              # current crawl (default)
scout crawl map <url>          # current map
scout crawl extract <url>      # current sitemap extract
scout crawl deep <url>         # current knowledge
```
Keep `swarm` and `test-site` as separate top-level commands.

### Phase 4: Group extraction commands (higher risk, breaking)

Create `extract` parent:
```
scout extract table <url>      # current table
scout extract meta <url>       # current meta
scout extract ai <url>         # current extract-ai
```

### Phase 5: Consider grouping gRPC commands (future)

Group daemon-dependent commands under `page` parent:
```
scout page title
scout page url
scout page click <selector>
scout page type <selector> <text>
scout page screenshot
```
This is the most breaking change and should be approached carefully.

### Proposed Final Structure (post-consolidation)

```
scout
  agent (serve, tools)
  auth (login, capture, replay, show, logout, providers)  # merged credentials
  bridge (17 subcommands)
  browser (list, download)
  challenge (detect, solve)
  client
  cloud (deploy, status, scale, uninstall)
  cmdtree
  completion
  connect
  crawl (default, map, extract, deep)  # merged map/sitemap/knowledge
  detect
  device (id, trust, list, remove, pair, discover)
  extension (download, remove, load, test, list)
  extract (table, meta, ai)  # grouped extraction
  fetch  # absorbed markdown
  fingerprint (generate, apply)
  form (detect, fill, submit)
  gather
  github (repo, issues, prs, user, releases, code, tree)  # merged extract-* with --detailed
  guide
  har (start, stop, export)
  hijack (watch)
  inject
  jobs (list, status, cancel)
  logger
  mcp (stdio, open, screenshot)
  mobile (devices, connect)
  ollama (list, pull, status)
  page (title, url, text, attr, eval, html, click, type, select, hover, focus, clear, key, navigate, back, forward, reload, screenshot, pdf, cookie, header, block, storage, window)  # grouped gRPC commands
  pdf-form (fields, fill)
  plugin (list, install, remove, run, search, update, check-updates)
  profile (capture, load, show, merge, diff, session-capture, session-load)
  proxy (start, routes)
  record
  repl
  report (list, show, delete, schedule)
  research
  runbook (apply, validate, plan, create, fix, sample, presets, run-preset, flow)
  scrape (list, auth, run)
  search (default, google, bing, duckduckgo, wikipedia)  # absorbed websearch
  server
  session (create, destroy, list, use, list-local, prune, clean, rm, reset)
  setup
  snapshot
  strategy (run, validate, init)
  swagger
  swarm (start, join, status)
  test-site
  update (check)
  upload (auth, file, status)
  version
  vpn (status, connect, disconnect, servers)
  webmcp (discover, call, inspect)
  ws (listen)
```

This reduces top-level commands from ~60 to ~45 and eliminates the most confusing overlaps.

---

## Sources

All findings from direct codebase analysis of `cmd/scout/*.go` files. No external sources needed.

## Metadata

**Confidence breakdown:**
- Command inventory: HIGH -- direct grep of cobra.Command definitions
- Overlap analysis: HIGH -- code comparison of implementations
- Grouping proposal: MEDIUM -- subjective recommendations, breaking change risk varies
- Deprecation timeline: HIGH -- explicit dates in source code

**Research date:** 2026-03-29
**Valid until:** Until next major CLI restructuring
