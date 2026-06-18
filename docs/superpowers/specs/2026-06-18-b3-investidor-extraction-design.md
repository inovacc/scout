# B3 Investidor — Capture-Once / Replay-Many Extraction Pipeline

- **Date:** 2026-06-18
- **Status:** Design approved (sections A–F) — pending written-spec review
- **Owner:** Dyam Marcano
- **Related roadmap:** Phase 80 (vault→flow no-expose) — this pipeline is its first real consumer.

## 1. Goal

Extract the user's personal investment data from `https://www.investidor.b3.com.br` (the B3 *Área do Investidor*) and download it for analysis. Authenticate **once** in a headed browser (manual gov.br + MFA login), persist the resulting session, then on **every subsequent run** inject the saved auth and pull the data **headless** — with **auto-refresh** so re-login is rare.

### Locked decisions

| Decision | Choice |
|---|---|
| Data scope | **Everything** the portal exposes for the CPF — Posição, Movimentação, Proventos, and any other sections |
| Output format | **Raw JSON + flattened CSV** per section |
| Auth expiry | **Auto-refresh if possible**; recon decides the mechanism; degrade to headed re-capture only when the *refresh* token dies |
| Replay engine | **Recon-first, auto-pick** — browserless `flow` if the refresh token is JS-readable; headless-browser self-refresh if it is an httpOnly cookie |

### Environment facts (verified 2026-06-18)

- `scout.exe` builds and runs on this machine (`C:\Users\dyamm\go\bin\scout.exe`); git ownership already fixed via `safe.directory`.
- Chrome 149 is cached → headed login needs no download.
- `repl`, `vault`, `flow`, `runbook`, `gather`, `capture-host` subcommands all present.
- The B3 *Área do Investidor* is an **API-backed SPA** — section data arrives as JSON from backend endpoints, which is why API-replay (`flow`) is the primary engine rather than DOM scraping (`runbook`, which additionally **cannot inject auth**).

## 2. Architecture

Three stages. Stage 0 runs occasionally (only when the refresh token dies); Stages 1–2 are the "every run" path.

```
STAGE 0 — Bootstrap + Recon   (headed, you log in; occasional)
  scout headed @ investidor.b3.com.br  ──► you complete gov.br + MFA
        │  (while logged in, walk Posição / Movimentação / Proventos / …)
        ├─► capture.json   = every /api call, headers, refresh endpoint   (flow capture / HAR)
        └─► vault "b3"     = cookies + localStorage + sessionStorage + access/refresh token
                                    │
                                    ▼  RECON decides the engine
                 refresh token JS-readable? ──► Engine A (browserless flow)
                 refresh token httpOnly?     ──► Engine B (headless browser self-refresh)
                 no refresh token at all?    ──► fall back: re-capture headed on expiry

STAGE 1 — Build replay spec   (once, from recon facts)
  capture.json ──► flow.yaml (refresh step + one fetch step per section) + section→endpoint map

STAGE 2 — Replay   (headless, every run)
  load vault "b3" ─► refresh access token ─► fetch every section ─► raw .json + flattened .csv
```

### Engine fork (recon-decided)

- **Engine A — browserless `scout flow run`**: replays B3's JSON APIs directly with the token. Fast, clean, raw JSON. Requires a **JS-readable** refresh token.
- **Engine B — headless browser**: injects saved storage into a headless page, lets the SPA refresh *itself*, and calls the APIs from inside the page (`eval fetch(...)`). Required when the refresh token is an **httpOnly cookie** (JS/flow cannot read it). Heavier (renders the SPA) but robust.

## 3. Components & data flow

| Component | Role | Created / used |
|---|---|---|
| **vault `b3`** | auth store: `Cookies` (B3 + gov.br), `Storage` (per-origin local/session — where the JWT lives), `Secrets` (`b3_refresh`, optionally `b3_access`) | Stage 0: `scout vault capture` + a step to pull tokens from storage into `Secrets` |
| **capture.json** | recon artifact — normalized API log (endpoints, headers, refresh call) | Stage 0: `scout flow capture --filter *api* *graphql*` |
| **flow.yaml** (Engine A) | deterministic replay contract | Stage 1 |
| **section map** | section → endpoint(s) → output filename → primary record-array path | Stage 1 |
| **output writer** | *new* small component: each JSON response → raw `.json` + flattened `.csv`; engine-agnostic | Stage 2 |
| **run-command** | orchestrator (Taskfile targets: `bootstrap`, `run`, `verify`) | Stage 2 |

Data flow: `vault → handle → refresh → for each section: request → JSON → output writer → {json, csv}`.

### flow.yaml shape (Engine A)

Endpoints are **parameters resolved during Stage 0 recon**, not guesses; shown here as placeholders:

```yaml
name: b3-investidor
auth: { profile: <vault-b3-id> }
vars: { api: "https://investidor.b3.com.br/api" }   # confirmed at recon
steps:
  - id: refresh
    request:
      method: POST
      url: "${api}/oauth/refresh"                    # confirmed at recon
      json: { refresh_token: "${secret.b3_refresh}" }
    extract:
      - { var: access, from: response.json, path: "$.access_token" }
  - id: posicao
    request:
      method: GET
      url: "${api}/posicao"
      headers: { Authorization: "Bearer ${access}" }
  - id: movimentacao      # … one step per discovered section, same auth chain
  - id: proventos
```

## 4. Auto-refresh + expiry / re-auth

**Recon classifies the refresh token, which picks the path:**

- **JS-readable** (localStorage / sessionStorage / non-httpOnly cookie) → **Engine A**: the `refresh` step replays it browserless.
- **httpOnly cookie** → **Engine B**: the headless browser holds the refresh cookie; the SPA's own silent-refresh fires; we never read the token.
- **None** → no auto-refresh; degrade to headed re-capture.

**Run-time expiry detection** — the first authenticated response decides:

- Engine A: `refresh` step returns 401 → the *refresh* token itself is dead → print `run "task b3:bootstrap" to re-login (gov.br + MFA)`, exit non-zero.
- Engine B: page lands on the login screen → same re-capture prompt.

**Re-auth = re-run Stage 0** (headed, ~1 min) — the only manual touch, and only when the long-lived *refresh* token dies, not the short access token.

**Token hygiene:** the minted access token stays in-memory (`${access}`), never written to disk. If B3 rotates the refresh token on refresh, the new value is captured from the response and `vault set` back so the chain stays alive.

## 5. Output layout

- **Per-run directory**, timestamped, root configurable, default **outside the repo** (under scout home):
  `<root>/b3-data/2026-06-18T143000/`
- **Per section:** `posicao.json` (raw, full fidelity) + `posicao.csv` (flattened); same for `movimentacao`, `proventos`, and every other discovered section.
- **`_run.json` manifest:** timestamp, scout version, engine used (A/B), sections fetched, per-section HTTP status + row count, refresh outcome.
- **`latest/`** copy pointing at the newest run for "load most recent" in pandas/Excel.
- **Flattening rule:** each section has one primary *record array* (holdings / movements / earnings) → one CSV row per element; nested fields → dotted columns; the array to flatten is declared per section in the section map; top-level scalars/metadata go to `_run.json`.
- **Modes:** files `0o600`, dirs `0o700` (CPF-linked financial data).

## 6. Security

- **No credentials stored, ever** — gov.br CPF/password is only typed by the user in the headed window; only the resulting session *tokens* are persisted.
- **Tokens never plaintext on disk:** refresh token lives only in the encrypted vault (Argon2id+AES-256-GCM, `LockedBuffer`); access token in-memory only; `flow.yaml` references `${secret.b3_refresh}`, never the literal (enforced by flow hygiene tests).
- **`capture.json` is sensitive** (holds a live token snapshot from recon) → `0o600`; scrubbed or deleted after Stage 1; never committed.
- **Nothing sensitive committed:** `b3-data/`, `capture.json`, vault, and the headed profile dir are gitignored / outside the repo. Only the spec and the secret-free `flow.yaml` are git-trackable.

## 7. New code required (small)

1. **Output writer** — JSON→CSV flattener + per-run directory/manifest writer. Pure function over JSON; fully unit-testable, no browser.
2. **`flow run` body persistence** — `scout flow run` currently prints a step count but does not persist response bodies. Either (a) a thin wrapper that captures each step's response, or (b) a `--dump-dir` enhancement to `flow run` in Scout core (preferred long-term; ties into Phase 80 / FLOW v2). v1 may use the wrapper.
3. **Engine-selection helper** — classifies the recon token location → A / B / fallback.
4. **Engine B path** — only if recon shows an httpOnly refresh token: a headless inject-storage + `eval fetch` extractor.

## 8. Testing / verification

- **Status-parity:** `scout flow verify flow.yaml --golden capture.json` (Scout-native).
- **Output-writer unit tests:** flattener over fixture B3-shaped payloads (secrets scrubbed) — CI-safe, no browser.
- **Engine-selection unit test:** recon token-classification input → asserts A / B / fallback.
- **Hygiene test:** no token literal in `flow.yaml` / scrubbed `capture.json` / logs (mirrors Phase 79 `hygiene_test.go`).
- **Manual UAT (acceptance):** after one bootstrap, a headless run produces non-empty json+csv per section, statuses 200, sane row counts — and a **second run works without re-login** (proves auto-refresh). Gated by `verification-before-completion`.

### Acceptance criteria

1. A single headed bootstrap captures auth into vault `b3` and records `capture.json` covering all reachable sections.
2. Recon correctly classifies the refresh-token location and selects engine A / B / fallback.
3. `task b3:run` (headless) writes `posicao`, `movimentacao`, `proventos`, and every other discovered section as raw `.json` + flattened `.csv` under a timestamped run dir, with a `_run.json` manifest.
4. A second consecutive `task b3:run` succeeds **without** re-login (auto-refresh), until the refresh token's natural expiry.
5. No secret literal appears in any git-trackable artifact or log.

## 9. Out of scope (YAGNI)

- Scheduling / cron (run is manual, on demand).
- Parsing/normalizing B3's domain model beyond flatten-to-CSV (downstream analysis owns that).
- Multi-account / multi-CPF.
- Publishing to OKF (Phase 81) — a natural *future* sink for this data, tracked separately.

## 10. Open questions (resolve during planning)

1. **Pipeline placement:** default `examples/b3-investidor/` (flow.yaml + section map + Taskfile + flattener), runtime data outside the repo. Confirm or relocate.
2. **Output writer home:** local helper (v1) vs `flow run --dump-dir` in Scout core (reusable). Decide based on whether this should harden Phase 80 now.
3. **Headed-capture mechanism:** `scout repl` + `eval`/`vault capture` vs the `scout-capture` MV3 extension vs `scout session create --har` — pick the smoothest single-login path during recon.
