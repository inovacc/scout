# Scout MCP Tool Surface: Old→New Mapping & Parity Check

**Date:** 2026-07-01  
**Scope:** Enumeration of all 40 registered MCP tools; mapping to new names per R6 (CLI path with segments joined by `_`); parity verification against redesigned CLI per domain map.

## Summary

- **Total MCP tools:** 40
- **Renamed:** 8 (page domain moves + site domain moves)
- **Unchanged:** 32
- **Parity gaps (CLI paths lacking a tool):** 11+
- **Orphan tools (MCP tools with unclear/absent CLI twin):** 6+

---

## Master Tool Mapping Table

*Sorted by new tool name. All 40 currently-registered tools.*

| Old tool | New tool | Source (file:line) | Mirrors CLI path | Changed? |
|----------|----------|-------------------|------------------|----------|
| back | back | tools_browser.go:148 | back (interaction) | no |
| browser_snapshot | browser_snapshot | tools_aria.go:16 | **[ORPHAN]** browser snapshot? | no |
| click | click | tools_browser.go:47 | click (interaction) | no |
| cookies | page_cookies | tools_browser.go:277 | page cookies | yes |
| crawl | site_crawl | tools_crawl.go:15 | site crawl | yes |
| eval | page_eval | tools_browser.go:123 | page eval | yes |
| expect_text | expect_text | tools_locator.go:170 | **[ORPHAN]** expect text (locator?) | no |
| expect_visible | expect_visible | tools_locator.go:147 | **[ORPHAN]** expect visible (locator?) | no |
| extract | extract | tools_browser.go:96 | extract (root hot verb, table/meta) | no |
| form_detect | form_detect | tools_form.go:16 | **[ORPHAN]** form detect? | no |
| forward | forward | tools_browser.go:165 | forward (interaction) | no |
| gather | site_gather | tools_gather.go:16 | site gather | yes |
| hijack_watch | hijack_watch | tools_hijack.go:15 | **[ORPHAN]** hijack watch? | no |
| html | page_html | tools_browser.go:241 | page html | yes |
| locator_click | locator_click | tools_locator.go:82 | **[ORPHAN]** locator click (domain?) | no |
| locator_count | locator_count | tools_locator.go:131 | **[ORPHAN]** locator count | no |
| locator_fill | locator_fill | tools_locator.go:97 | **[ORPHAN]** locator fill (domain?) | no |
| locator_text | locator_text | tools_locator.go:115 | **[ORPHAN]** locator text | no |
| markdown | page_markdown | tools_browser.go:259 | page markdown | yes |
| navigate | navigate | tools_browser.go:16 | navigate (root hot verb) | no |
| open | open | tools_session.go:58 | open (root hot verb OR session open?) | no |
| page_title | page_title | tools_browser.go:318 | page title (already in page domain) | no |
| page_url | page_url | tools_browser.go:300 | page url (already in page domain) | no |
| pdf | pdf | tools_capture.go:81 | pdf (root hot verb) | no |
| report_delete | report_delete | tools_report.go:58 | report delete | no |
| report_list | report_list | tools_report.go:15 | report list | no |
| report_show | report_show | tools_report.go:28 | report show | no |
| runbook_apply | runbook_apply | tools_runbook.go:42 | runbook apply | no |
| runbook_plan | runbook_plan | tools_runbook.go:16 | runbook plan (or runbook validate?) | no |
| screenshot | screenshot | tools_capture.go:16 | screenshot (root hot verb) | no |
| session_list | session_list | tools_session.go:15 | session list | no |
| session_reset | session_reset | tools_session.go:47 | session reset | no |
| sitemap | site_map | tools_sitemap.go:14 | site map | yes |
| snapshot | snapshot | tools_capture.go:47 | **[ORPHAN]** snapshot vs. screenshot? | no |
| test_site | site_test | tools_testsite.go:14 | site test | yes |
| type | type | tools_browser.go:71 | type (interaction) | no |
| wait | wait | tools_browser.go:182 | wait (interaction) | no |
| ws_connections | ws_connections | tools_websocket.go:70 | ws connections | no |
| ws_listen | ws_listen | tools_websocket.go:16 | ws listen | no |
| ws_send | ws_send | tools_websocket.go:41 | ws send | no |

---

## Parity Analysis

### Gaps: CLI Leaf Commands Without MCP Tools

Authoritative CLI paths from the **domain map** (COMMAND-TAXONOMY.md) that SHOULD have an MCP tool per R6 but do not:

**`site` domain (NEW):**
- `site knowledge` — NOT FOUND (no MCP tool registered)

**`page` domain (NEW):**
- `page text` — NOT FOUND
- `page attr` — NOT FOUND

**`llm` domain (NEW):**
- `llm ollama list` — NOT FOUND
- `llm ollama pull` — NOT FOUND
- `llm ollama status` — NOT FOUND
- `llm job list` — NOT FOUND
- `llm job show` — NOT FOUND
- `llm job session list` — NOT FOUND
- `llm job session create` — NOT FOUND
- `llm job session use` — NOT FOUND

**`github` domain (wide, with sub-noun):**
- `github extract repo` — NOT FOUND (design spec §4, move 4)
- `github extract issues` — NOT FOUND
- `github extract prs` — NOT FOUND
- `github extract releases` — NOT FOUND

**`plugin` domain (with `host` sub-noun):**
- `plugin host install` — NOT FOUND (design spec §4, move 5)
- `plugin host uninstall` — NOT FOUND
- `plugin host status` — NOT FOUND
- `plugin host extract` — NOT FOUND
- `plugin host doctor` — NOT FOUND
- `plugin host hosts` — NOT FOUND

**`pdf` domain (with `form` sub-noun):**
- `pdf form fill` — NOT FOUND (design spec §4, move 6)
- `pdf form fields` — NOT FOUND

**`vault` domain (with `key` and `import` sub-nouns):**
- `vault key <...>` — NOT FOUND (design spec §4, move 7)
- `vault import` — NOT FOUND

**`runbook` domain (with `preset` sub-noun):**
- `runbook preset list` — NOT FOUND (design spec §4, move 8)
- `runbook preset run` — NOT FOUND

**Total gaps:** 26 CLI leaf paths without corresponding MCP tools.

---

### Orphan Tools: MCP Tools Without Clear CLI Twin

The following registered MCP tools have **no obvious or documented CLI command equivalent**. They require clarification:

| Tool | Source | Issue |
|------|--------|-------|
| `snapshot` | tools_capture.go:47 | Differs from `screenshot`? Redundant? Orphan. |
| `browser_snapshot` | tools_aria.go:16 | Differs from both `snapshot` and `screenshot`? Possible duplicate. Orphan. |
| `form_detect` | tools_form.go:16 | No `form` domain in current design spec; unclear purpose. Orphan. |
| `hijack_watch` | tools_hijack.go:15 | Likely internal/utility; no documented CLI path. Orphan. |
| `locator_click` | tools_locator.go:82 | Part of a `locator` sub-domain? Not in domain map. Orphan. |
| `locator_fill` | tools_locator.go:97 | Part of a `locator` sub-domain? Not in domain map. Orphan. |
| `locator_text` | tools_locator.go:115 | Part of a `locator` sub-domain? Not in domain map. Orphan. |
| `locator_count` | tools_locator.go:131 | Part of a `locator` sub-domain? Not in domain map. Orphan. |
| `expect_visible` | tools_locator.go:147 | Part of a `locator` sub-domain? Not in domain map. Orphan. |
| `expect_text` | tools_locator.go:170 | Part of a `locator` sub-domain? Not in domain map. Orphan. |

**Total orphans:** 10 tools with unclear CLI mapping or absent from domain map.

---

## Notes & Caveats

1. **Interaction verbs (`click`, `type`, `wait`, `back`, `forward`):** These are driver/browser-control verbs. Per the audit and design spec, they are **not** listed as leaf commands in the CLI domain map. Their MCP registration is correct; their absence from the CLI map is intentional (the CLI may re-expose them under a higher-level grouping like `page interact`, or they remain MCP-only). No renaming required.

2. **`open` (tools_session.go:58):** Marked as a hot verb (R7) and stays flat. Could be clarified as `session open` if the domain map specifies. Current name is correct per R7.

3. **Locator tools (`locator_*`, `expect_*`):** These appear to be **playwright/browser-locator utilities** not yet grouped under a formal CLI domain. They should either be:
   - Added to the domain map under a `locator` sub-domain, or
   - Renamed to clarify scope (e.g., `page locator_click`), or
   - Left as MCP-only tools if the CLI does not expose them.

4. **`snapshot` vs. `screenshot` vs. `browser_snapshot`:** Three related tools; need clarification on purpose and intended usage. Possible consolidation.

5. **Phase 6 follow-up:** The **26 gap** tools (new CLI commands like `llm ollama list`, `github extract repo`, etc.) require **new MCP tool implementations** during Phase 6, not just renames of existing tools.

---

## Mapping Summary by Category

### Renamed (site + page domain moves): 8
- `crawl` → `site_crawl`
- `gather` → `site_gather`
- `sitemap` → `site_map`
- `test_site` → `site_test`
- `eval` → `page_eval`
- `html` → `page_html`
- `markdown` → `page_markdown`
- `cookies` → `page_cookies`

### Unchanged (includes hot verbs, already-correct names): 32
- Root hot verbs: `navigate`, `click`, `type`, `extract`, `back`, `forward`, `wait`, `screenshot`, `pdf`, `open`
- Already domain-qualified: `page_url`, `page_title`, `session_list`, `session_reset`, `report_list`, `report_show`, `report_delete`, `runbook_plan`, `runbook_apply`, `ws_listen`, `ws_send`, `ws_connections`
- Orphans (no mapping): `snapshot`, `browser_snapshot`, `form_detect`, `hijack_watch`, `locator_click`, `locator_fill`, `locator_text`, `locator_count`, `expect_visible`, `expect_text`
