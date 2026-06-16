# Scout MCP — Practical Gaps vs Playwright (Field Report)

**Source:** B3 Área do Investidor automation session, 2026-06-16  
**Use case:** Authenticated session capture + full document/data download from a Brazilian financial portal (Cloudflare + Azure AD B2C OAuth, SPA, dynamic content)  
**Related:** See `PLAYWRIGHT-GAP-ANALYSIS.md` for the broader architectural comparison.

This document records gaps hit during *real automation work* — not a feature matrix.
Each gap has a concrete example, the workaround used, and a priority signal.

---

## 1. Session persistence — storageState save/restore

**What Playwright does:**
```js
// save after login
await context.storageState({ path: '.secrets/session.json' });

// restore on next run (skips login entirely)
const context = await browser.newContext({ storageState: '.secrets/session.json' });
```

**What Scout does:**  
`cookies` tool reads cookies from the live page. There is no way to *save* the full browser state (cookies + localStorage + sessionStorage) to a file and *restore* it in a future session.

**Workaround used:**  
After Scout captured the session, the raw tokens were manually extracted via `eval()` and written to a `.secrets/b3-session.json` file. A Playwright script was then written to own the persistence layer.

**Impact:** High. Any automation that needs to avoid re-login between runs hits this wall immediately. OAuth flows (like B2C / Azure AD) issue short-lived tokens (~1h); without storageState you re-login every run.

**Priority:** 🔴 P0 — most real-world authenticated automation needs this.

---

## 2. File download interception

**What Playwright does:**
```js
const [download] = await Promise.all([
  page.waitForEvent('download', { timeout: 15000 }),
  page.locator('button:has-text("Exportar")').click(),
]);
await download.saveAs('/path/to/file.pdf');
```

**What Scout does:**  
Clicking a button that triggers a file download works at the click level, but there is no tool to intercept the `download` event, capture the stream, or save it to a local path.

**Workaround used:**  
Playwright script handles all download collection. Scout was limited to navigation and data extraction via `eval()` / `cookies`.

**Impact:** High. Document download is a primary use case for financial portals, data exports, and report automation.

**Priority:** 🔴 P0 — no workaround within Scout alone.

---

## 3. URL predicate waiting (waitForURL with function)

**What Playwright does:**
```js
// Wait until URL matches a dynamic condition (e.g. post-OAuth redirect)
await page.waitForURL(url => {
  const u = url.toString();
  return u.includes('investidor.b3.com.br') && !u.includes('/login');
}, { timeout: 180_000 });
```

**What Scout does:**  
`wait` accepts a CSS selector or a generic load state. No predicate-based URL wait.

**Workaround used:**  
Called `page_url` tool in a polling loop (manual, one tool call at a time). Fragile for OAuth redirects with unpredictable URL shapes.

**Impact:** Medium-High. OAuth / SSO flows nearly always need predicate URL waiting — the redirect URL contains state/code params that make exact string matching impossible.

**Priority:** 🟠 P1 — high value, likely a one-day addition.

---

## 4. eval() JavaScript limitations

**What Scout does:**  
The `eval` tool wraps the expression and calls `.apply()` on the result. This means:
- Multi-statement expressions without an explicit return fail
- The final value must be a callable (functions work; plain objects/strings returned from IIFEs sometimes fail)
- Error: `TypeError: JSON.stringify(...).apply is not a function` when the return value is a primitive

**Example that failed:**
```js
// ❌ Failed in Scout eval
JSON.stringify({ls: Object.keys(localStorage), origin: location.origin})

// ✅ Worked — wrapped in IIFE returning a function
(function(){ return JSON.stringify({ls: Object.keys(localStorage)}); })
```

**What Playwright does:**  
`page.evaluate(expr)` evaluates any expression and returns the serialized result. No `.apply()` wrapping. Works with primitives, objects, arrays, and async expressions.

**Impact:** Medium. Causes confusing trial-and-error when doing data extraction. Increases iteration time per eval call.

**Priority:** 🟠 P1 — fix the eval wrapper to not call `.apply()` on the result.

---

## 5. Autonomous / scheduled execution

**What Playwright does:**  
A Playwright script is a plain Node/Python script. Run it with `node script.js`, `cron`, GitHub Actions, a Docker container, etc. Zero dependencies on an interactive session.

**What Scout does:**  
Scout MCP tools require an active Claude Code session. There is no way to invoke a saved Scout runbook autonomously from a cron job or CI pipeline without an LLM host running.

**Workaround used:**  
Playwright script at `scripts/scrapeB3Investidor.js` runs on demand (`npm run scrape-b3`) or can be scheduled via Task Scheduler / cron. Scout was used only for initial exploration and session bootstrapping.

**Impact:** High for any "run this nightly" use case.

**Priority:** 🔴 P0 for headless/cron use. Scout runbooks can address this if they can be executed by a lightweight runner without the full MCP stack.

---

## 6. waitForLoadState('networkidle')

**What Playwright does:**
```js
await page.goto(url, { waitUntil: 'networkidle' });
// or post-navigation
await page.waitForLoadState('networkidle', { timeout: 15000 });
```

**What Scout does:**  
`navigate` triggers a navigation and returns when the basic load event fires. No `networkidle` option. SPAs (like B3 Investidor, which makes 8-12 XHR calls after the shell loads) appear "ready" in Scout while data is still being fetched.

**Workaround used:**  
Added `wait` calls with a known selector that only appears after data loads, plus fixed `waitForTimeout` sleeps — both fragile.

**Impact:** Medium-High. Every SPA hits this. Data scraped before XHR completes returns an empty or partial result.

**Priority:** 🟠 P1 — add `networkidle` as an option to `navigate`/`wait`.

---

## 7. Programmatic loops and conditional logic

**What Playwright does:**  
Full JS/Python — `for` loops, `if/else`, `try/catch`, `Promise.all`, retry logic, pagination loops.

**What Scout does:**  
Each tool call is a single discrete action driven by the LLM. Looping over 12 months of paginated data, or retrying a flaky API call, requires the LLM to make N sequential tool calls — expensive, slow, and context-consuming.

**Workaround used:**  
Data collection logic was embedded into a Playwright script where a `for` loop runs over brokers and date ranges programmatically.

**Impact:** Medium. Scout works well for exploration; it becomes costly for repetitive iteration (pagination, bulk download).

**Priority:** 🟡 P2 — Scout runbooks (YAML scripts) partially address this; needs loop/iteration primitives in the runbook format.

---

## 8. Request/response interception for API discovery

**What Playwright does:**
```js
page.on('response', async res => {
  if (res.url().includes('/api/')) {
    console.log(res.url(), await res.json());
  }
});
```

**What Scout does:**  
`ws_listen` monitors WebSocket frames. HTTP responses can only be observed by calling `performance.getEntriesByType('resource')` via `eval` after the fact — which gives URLs but not response bodies.

**Workaround used:**  
Used `eval` + `performance.getEntriesByType('resource')` to discover API endpoint URLs, then re-fetched them with `eval` + `fetch()` using the stored Bearer token.

**Impact:** Medium. The two-step workaround works but is slow and misses responses that are no longer in the performance buffer.

**Priority:** 🟡 P2 — network interception is in `PLAYWRIGHT-GAP-ANALYSIS.md` as a known gap.

---

## 9. Multiple isolated browser contexts

**What Playwright does:**
```js
const ctx1 = await browser.newContext({ storageState: 'user1.json' });
const ctx2 = await browser.newContext({ storageState: 'user2.json' });
// run in parallel
```

**What Scout does:**  
One active session at a time. `session_reset` clears everything.

**Impact:** Low for this use case, but blocks multi-account or A/B scenarios.

**Priority:** 🟡 P2.

---

## Summary table

| Gap | Scout today | Playwright | Priority |
|-----|------------|------------|----------|
| storageState save/restore | ❌ cookies read-only | ✅ full save/load | 🔴 P0 |
| File download interception | ❌ | ✅ | 🔴 P0 |
| Headless/scheduled execution | ❌ needs LLM host | ✅ standalone | 🔴 P0 |
| URL predicate waiting | ❌ CSS selector only | ✅ function predicate | 🟠 P1 |
| eval() primitives bug | 🟡 workaround needed | ✅ any expression | 🟠 P1 |
| networkidle wait | ❌ | ✅ | 🟠 P1 |
| Programmatic loops | ❌ LLM-driven only | ✅ | 🟡 P2 |
| Response body interception | ❌ URLs only via perf API | ✅ | 🟡 P2 |
| Multiple isolated contexts | ❌ one session | ✅ | 🟡 P2 |

## Where Scout won in this session

- **Anti-bot / Cloudflare bypass** — B3 uses Cloudflare. Scout's stealth mode passed without friction. A headless Playwright default would have been challenged.
- **Interactive exploration** — Scout's tool-per-action model was ideal for initial discovery: take screenshot → identify elements → click → observe result. Much faster than writing Playwright code to explore an unknown SPA.
- **ARIA snapshot** — `browser_snapshot` gave a clean semantic map of the login form in seconds, making locator selection trivial.
- **OAuth token extraction** — `eval` + `cookies` together captured the full Azure B2C token set without writing any code.

## Recommended hybrid pattern (used in this session)

```
Scout MCP (exploration + bootstrap)
  → navigate, screenshot, snapshot, click, fill, eval, cookies
  → discover API endpoints, extract auth tokens, understand page structure
  → export storageState-equivalent JSON manually

Playwright (production automation)
  → storageState restore (skip re-login)
  → programmatic loops over date ranges / brokers
  → download event interception
  → networkidle waits
  → scheduled/headless runs
```

Scout is the **exploration and bootstrap** layer. Playwright is the **production execution** layer. The gap to close for Scout to own the full cycle is primarily: storageState, downloads, and headless/cron execution.
