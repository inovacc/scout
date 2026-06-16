# Scout Playwright-Parity Design Spec
**Date:** 2026-06-16  
**Status:** Approved for implementation planning  
**Source:** B3 Área do Investidor automation session (field report: `docs/scout-playwright-gaps.md`)  
**Scope:** All 7 P0–P2 gaps. Design only — implementation plan to follow.

---

## 1. Goal

Make Scout the de facto tool for authenticated, scheduled browser automation — eliminating the need for Playwright as a secondary layer. The B3 financial portal scraping use case (Cloudflare + Azure AD B2C OAuth, SPA, file downloads, nightly sync) is the reference implementation that must work end-to-end in Scout alone after these changes.

---

## 2. Non-goals

- Cross-engine support (Firefox, WebKit) — CDP-only remains by design.
- Multi-language bindings (Python, Java, .NET).
- Full Playwright Test runner (fixtures, reporters, UI mode, trace viewer).
- Component testing.

---

## 3. Architecture

Implementation follows a strict bottom-up layering. Each layer is independently testable and the scheduling layer is impossible to build correctly without the layers beneath it.

```
┌─────────────────────────────────────────────────────┐
│                  MCP Tools (+7 new)                  │  LLM surface
│  session_save  session_load  session_export          │
│  download_wait  hijack_watch                         │
│  wait(networkidle/url)  schedule_run                 │
└────────────────────┬────────────────────────────────┘
                     │ thin wrappers
┌────────────────────▼────────────────────────────────┐
│          Runbook Engine  (pkg/scout/runbook)         │  automation lingua franca
│  New actions: session_save  session_load             │
│               download_wait  loop  while  repeat     │
│               wait_url  wait_networkidle             │
│               hijack_response                        │
│  New modifiers: loop  if  on_failure  capture_as     │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│          Storage Layer  (pkg/scout/session)          │
│  SessionStore (vault-backed)  DownloadManager        │
│  Playwright-compat export/import                     │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│          Engine Layer  (internal/engine/browser)     │
│  eval fix  networkidle  URL-predicate wait           │
│  CDP download hooks  response body capture           │
└─────────────────────────────────────────────────────┘
```

**Key principle:** The runbook engine is the single implementation for all capabilities. MCP tools and the cron scheduler are thin callers of the runbook engine — no logic is duplicated between surfaces.

---

## 4. Layer 1 — Engine Layer

### 4.1 eval() primitive return fix

**Problem:** The `eval` MCP tool wrapper calls `.apply()` on the expression result. When the expression returns a primitive (string, number, plain JSON object), this throws `TypeError: result.apply is not a function`.

**Fix:** In `pkg/scout/mcp` (eval tool handler), check `reflect.TypeOf(result).Kind()` before calling `.apply()`. If the result is not a function, serialize it directly and return. Zero API change — purely a bug fix.

**Affected file:** `pkg/scout/mcp` eval tool registration.

### 4.2 Wait enhancements

Add two new modes to the existing `wait` MCP tool and `wait` runbook action via a `wait_for` parameter:

**`wait_for: networkidle`**
- Subscribes to CDP `Network.loadingFinished`, `Network.loadingFailed`, `Page.frameStoppedLoading` events.
- Starts a 500ms debounce timer on each event; resolves when the timer fires without a new event.
- `timeout_ms` param controls overall timeout (default 15000ms).
- Mirrors Playwright's `waitForLoadState('networkidle')`.

**`wait_for: url`** + `url_pattern: "<pattern>"`
- Polls `Runtime.evaluate('location.href')` on a 200ms tick.
- Pattern matching supports: plain substring, glob (`*` wildcards), and full Go regex (prefix `re:`).
- Resolves when current URL matches pattern.
- `timeout_ms` default 30000ms.
- Solves OAuth/SSO redirect detection (B3 Azure B2C post-login redirect).

Both modes are additive — the existing CSS selector `wait` mode is unchanged.

### 4.3 Response body capture

**Problem:** `hijack watch --body` exists in the CLI but is not exposed as an MCP tool.

**Solution:** Add `hijack_watch` MCP tool. Parameters:
- `url_pattern` — URL glob to filter (default `*`)
- `collect_for_ms` — duration to collect before returning (default 5000ms); returns all captured request/response pairs as a JSON array
- `body` — bool, whether to capture response bodies (default `true`)

Implementation: thin wrapper over existing `scout.HijackOption` / `scout.WithHijackBodyCapture()` infrastructure. No new engine code required.

**Affected files:** `pkg/scout/mcp` (new tool registration), `cmd/scout/mcp.go` (tool count update in docs).

---

## 5. Layer 2 — Storage Layer

### 5.1 SessionStore (`pkg/scout/session`)

New package. Wraps the existing `pkg/scout/vault` with a browser-state-specific API.

**Data captured per session:**
- Cookies: via CDP `Network.getAllCookies` → filtered to target origin
- localStorage: via `Runtime.evaluate` IIFE iterating `localStorage`
- sessionStorage: via `Runtime.evaluate` IIFE iterating `sessionStorage`
- Metadata: `captured_at` (RFC3339), `expires_at` (optional, parsed from auth token if present), `origin_url`

**Vault storage:** Each session is a named vault secret profile. The session JSON is AES-256-GCM encrypted via the existing vault. Key: `session:<name>`.

**API surface:**
```
SessionStore.Save(ctx, name string, page Page) error
SessionStore.Load(ctx, name string, page Page) error
SessionStore.Delete(name string) error
SessionStore.List() ([]SessionMeta, error)
SessionStore.IsExpired(name string) (bool, error)
SessionStore.ExportPlaywright(name, outPath string) error
SessionStore.ImportPlaywright(name, inPath string) error
```

**Playwright-compatible export format:**
```json
{
  "cookies": [
    { "name": "...", "value": "...", "domain": "...", "path": "/",
      "expires": -1, "httpOnly": false, "secure": false, "sameSite": "Lax" }
  ],
  "origins": [
    {
      "origin": "https://www.investidor.b3.com.br",
      "localStorage": [{ "name": "key", "value": "val" }]
    }
  ]
}
```
sessionStorage is stored in Scout's vault but omitted from Playwright export (Playwright's format doesn't include sessionStorage — it's injected via `addInitScript` instead).

**CLI commands added:**
```
scout session save   <name>             # capture from current page
scout session load   <name>             # inject into current page
scout session export <name> [--format=playwright] [--out=file.json]
scout session import <file> [--format=playwright] [--name=name]
scout session list                      # already exists, now shows expiry
scout session delete <name>
```

**MCP tools added:** `session_save`, `session_load`, `session_export` (3 new tools).

**Token expiry detection:** After `Load`, if `expires_at` is within 5 minutes of now, the tool returns a warning field `"expiring_soon": true` so the LLM or runbook can decide to re-authenticate.

### 5.2 DownloadManager (`internal/engine/browser`)

**CDP hooks used:**
- `Page.setDownloadBehavior` — set to `allow` with a `downloadPath`
- `Page.downloadWillBegin` — emitted when download starts (provides `guid`, `suggestedFilename`, `url`)
- `Page.downloadProgress` — progress events (`receivedBytes`, `totalBytes`, `state`)

**DownloadManager API:**
```
DownloadManager.SetDir(path string)
DownloadManager.WaitForDownload(ctx context.Context, timeout time.Duration) (DownloadResult, error)
DownloadManager.SaveAs(guid, destPath string) error
```

`DownloadResult`: `{ GUID, Filename, MIMEType, SizeBytes, SavePath, Duration }`.

**Download directory resolution (priority order):**
1. `save_as` param in runbook action or MCP tool call
2. `--download-dir` CLI flag
3. `SCOUT_DOWNLOAD_DIR` environment variable
4. `./downloads` (relative to working directory)

**Runbook action:**
```yaml
- action: download_wait
  timeout_ms: 15000
  save_as: "./b3/downloads/informe-2025.pdf"   # optional rename
  capture_as: download_result                   # captures DownloadResult as var
```

**MCP tool:** `download_wait` — same params. Returns `DownloadResult` JSON.

**CLI command:** `scout download watch <url>` — opens page, streams download events to stdout as NDJSON, saves files.

---

## 6. Layer 3 — Runbook Layer

### 6.1 Variables

Top-level `vars` block in runbook JSON/YAML:
```yaml
vars:
  cpf: "708.101.202-75"
  base_url: "https://www.investidor.b3.com.br"
  year: 2025
```

Interpolated anywhere via `{{ var.name }}` or `{{ step_output.capture_as_name }}`. Expression engine: string interpolation + integer arithmetic only. No eval, no scripting — runbooks stay auditable.

`capture_as: varname` on any step stores that step's output (string, JSON, DownloadResult) into the variable scope for downstream steps.

### 6.2 Loop primitives

**`for` — iterate over a list:**
```yaml
- action: navigate
  loop:
    for: ["2023", "2024", "2025"]
    as: year
  url: "{{ base_url }}/relatorios?ano={{ year }}"
```

**`while` — repeat until condition false:**
```yaml
- action: extract
  loop:
    while: "{{ page_num }} <= {{ total_pages }}"
    max_iterations: 100
  selector: ".data-row"
  capture_as: rows
```

**`repeat` — fixed N times with index:**
```yaml
- action: click
  loop:
    repeat: 5
    as: i
  selector: ".load-more"
```

All loop forms:
- Emit results as a JSON array in `capture_as`
- Support `break_on_error: true` (default `false`)
- Report iteration count in run output

### 6.3 Conditional execution

```yaml
- action: session_load
  name: b3_session
  on_failure:
    - action: navigate
      url: "{{ base_url }}/login"
    - action: fill
      selector: "[placeholder*='CPF']"
      value: "{{ var.cpf }}"
    - action: click
      selector: "button:has-text('Entrar')"
    - action: session_save
      name: b3_session
```

**`if: "{{ condition }}"` modifier** — skips the step when false:
```yaml
- action: download_wait
  if: "{{ menus.informeRendimentos == true }}"
  timeout_ms: 10000
```

**`on_failure`** — runs a sub-sequence if the step errors. Does not count as a step failure if `on_failure` succeeds.

### 6.4 New runbook action registry

| Action | Layer | Description |
|--------|-------|-------------|
| `session_save` | Storage | Save current browser state to vault |
| `session_load` | Storage | Restore session from vault |
| `download_wait` | Storage | Intercept next file download, save to disk |
| `wait_networkidle` | Engine | Wait until network goes quiet (500ms debounce) |
| `wait_url` | Engine | Wait until URL matches pattern |
| `hijack_response` | Engine | Capture API response bodies matching URL pattern |

Loop/conditional modifiers (`loop`, `if`, `on_failure`, `capture_as`) apply to **any** action, not just new ones.

---

## 7. Layer 4 — Scheduling Layer

### 7.1 Schedule definition

`scout-schedule.yaml` in the working directory, or embedded as a top-level `schedule:` block inside a runbook:

```yaml
schedules:
  - name: b3-daily-sync
    cron: "0 7 * * 1-5"          # weekdays 7am
    tz: "America/Sao_Paulo"
    runbook: runbooks/b3-sync.json
    vars:
      download_dir: "./b3/downloads"
    timeout: 10m
    on_failure:
      notify: stderr             # stderr | file | webhook
      notify_file: "./logs/b3-failures.ndjson"
      notify_webhook: ""         # POST JSON on failure

  - name: b3-monthly-informe
    cron: "0 9 1 * *"            # 1st of month at 9am
    tz: "America/Sao_Paulo"
    runbook: runbooks/b3-informe.json
    timeout: 15m
```

Cron syntax: standard 5-field. Timezone-aware. Parsed by a pure-Go cron library (no system cron dependency).

### 7.2 CLI commands

```
scout schedule start              # foreground scheduler, reads scout-schedule.yaml
scout schedule daemon             # background via existing daemon infrastructure
scout schedule list               # show all jobs + next 3 fire times
scout schedule run   <name>       # immediate one-shot fire (for testing)
scout schedule stop               # signal daemon shutdown
scout schedule history <name>     # tail recent run results
scout schedule status             # daemon health + last run per job
```

### 7.3 Run result persistence

Each run writes to `~/.scout/runs/<schedule-name>/<ISO-timestamp>.json`:
```json
{
  "schedule": "b3-daily-sync",
  "started_at": "2026-06-16T07:00:01-03:00",
  "finished_at": "2026-06-16T07:03:44-03:00",
  "status": "success",
  "duration_ms": 223000,
  "outputs": { "download_result": { "filename": "informe-2025.pdf", ... } },
  "error": null
}
```

`scout schedule history <name>` reads and pretty-prints the last N result files.

### 7.4 Vault integration for unattended runs

The scheduler unlocks the vault for the duration of each job run. Passphrase supplied via:
1. `SCOUT_VAULT_PASS` environment variable (recommended for cron/systemd)
2. `--vault-pass-file /path/to/passfile` (chmod 600)

The vault is locked again after each run completes. Passphrase is zeroed from memory immediately after unlock (existing vault pattern).

### 7.5 MCP tool: `schedule_run`

Triggers immediate execution of a named schedule entry and returns the run result JSON. Allows Claude to fire a scheduled job on demand without starting the daemon. No new engine code — calls the same runbook executor as `scout runbook apply`.

---

## 8. New MCP tool surface (complete list)

| Tool | Layer | Description |
|------|-------|-------------|
| `session_save` | Storage | Save current browser state to vault |
| `session_load` | Storage | Restore session from vault |
| `session_export` | Storage | Export session (Playwright-compatible JSON) |
| `download_wait` | Storage | Intercept next file download event, save to disk |
| `hijack_watch` | Engine | Capture request/response bodies matching URL pattern |
| `wait` (updated) | Engine | Existing tool + `wait_for: networkidle/url` modes |
| `schedule_run` | Scheduler | Fire a named schedule entry immediately |

`eval` fix is transparent — no tool signature change.

Total MCP tools after: **25** (18 existing + 7 new).

---

## 9. Reference implementation: B3 daily sync runbook

This is the end-state runbook that replaces the Playwright script entirely:

```yaml
name: b3-daily-sync
vars:
  base_url: "https://www.investidor.b3.com.br"
  cpf: "{{ env.B3_CPF }}"           # from environment, not hardcoded
  download_dir: "./b3/downloads"

steps:
  - action: session_load
    name: b3_session
    on_failure:
      - action: navigate
        url: "{{ base_url }}/login"
      - action: wait_networkidle
      - action: fill
        selector: "[placeholder*='CPF']"
        value: "{{ var.cpf }}"
      - action: click
        selector: "button:has-text('Entrar')"
      - action: fill
        selector: "input[type='password']"
        value: "{{ env.B3_PASS }}"
      - action: click
        selector: "button:has-text('ENTRAR')"
      - action: wait_url
        url_pattern: "investidor.b3.com.br"
        timeout_ms: 180000
      - action: session_save
        name: b3_session

  - action: navigate
    url: "{{ base_url }}/proventos/recebidos"
  - action: wait_networkidle

  - action: hijack_response
    url_pattern: "*extrato-eventos-provisionados*"
    collect_for_ms: 5000
    capture_as: proventos_raw

  - action: navigate
    url: "{{ base_url }}/relatorios/informe-rendimentos"
  - action: wait_networkidle

  - action: click
    selector: "button:has-text('Exportar')"
  - action: download_wait
    timeout_ms: 15000
    save_as: "{{ var.download_dir }}/informe-{{ now.year }}.pdf"

  - action: session_save
    name: b3_session
```

Schedule entry:
```yaml
schedules:
  - name: b3-daily-sync
    cron: "0 7 * * 1-5"
    tz: "America/Sao_Paulo"
    runbook: runbooks/b3-sync.yaml
    on_failure:
      notify: file
      notify_file: "./logs/b3-failures.ndjson"
```

---

## 10. Phasing and dependencies

```
Phase 1 — Engine Layer      (no dependencies)
  ├── eval() fix             [tiny]
  ├── wait_networkidle       [small]
  ├── wait_url               [small]
  └── hijack_watch MCP tool  [small — wraps existing code]

Phase 2 — Storage Layer     (depends on Phase 1 browser primitives)
  ├── pkg/scout/session      [medium]
  ├── DownloadManager        [medium]
  ├── session MCP tools      [small — wraps SessionStore]
  └── download_wait MCP tool [small — wraps DownloadManager]

Phase 3 — Runbook Layer     (depends on Phase 2 actions being registered)
  ├── vars + capture_as      [small]
  ├── loop primitives        [medium]
  ├── if / on_failure        [medium]
  └── new action registry    [small — wires Phase 1+2 into runbook executor]

Phase 4 — Scheduling Layer  (depends on Phase 3 runbooks working)
  ├── scout-schedule.yaml    [small]
  ├── scheduler daemon       [medium]
  ├── CLI commands           [small]
  ├── run result persistence [small]
  └── schedule_run MCP tool  [small]
```

Estimated total: ~4 weeks at one engineer, phases sequentially. Each phase ships independently and is usable before the next phase begins.

---

## 11. Testing strategy

- **Phase 1:** Unit tests for eval fix; integration tests for networkidle/URL wait against a local HTTP server with controlled response timing.
- **Phase 2:** SessionStore unit tests with an in-memory vault. DownloadManager integration test with a local file-serving endpoint. Playwright-compat round-trip test (export → import → verify).
- **Phase 3:** Runbook integration tests for each new action and each loop form. Existing runbook test suite must pass unchanged.
- **Phase 4:** Scheduler unit tests for cron parsing + next-fire-time calculation. Integration test: schedule fires a runbook against a local test server, verifies run result file is written.

No mocking of the browser engine in integration tests — use a real browser against test fixtures (existing Scout testing pattern).
