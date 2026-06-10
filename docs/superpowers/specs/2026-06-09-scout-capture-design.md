# Scout Capture — Design Spec (Extension Phase 0)

**Date:** 2026-06-09
**Status:** Approved design (brainstorming complete; ready for the implementation plan of Phase 1 = the native host).
**Roadmap:** `docs/superpowers/specs/2026-06-09-credential-capture-extension-roadmap.md`
**Scope:** The operator's **own** self-capture tooling. Local-only. No remote transport.

## Locked decisions

1. **Capture scope:** session bundle (cookies + web storage) **and** raw login credentials — but **consent-first / password-manager-style only** (visible per-event "Save this login to Scout?" prompt). Never silent interception, keylogging, or background scraping.
2. **Transport:** native-messaging host (local, OS-mediated stdio). No network, no remote endpoint.
3. **Unlock flow:** **encrypted spool / append-only inbox.** The browser-facing host holds only the vault's public key and can encrypt-to but never decrypt the vault or the spool.
4. **Vault mapping:** **per-site profile** — one vault profile per site holds its session + login.

## 1. Architecture (three isolated units)

```
┌ extensions/scout-capture/ (MV3) ┐  native msg   ┌ scout capture-host ┐  sealed-box   ┌ <scouthome>/captures/spool/*.cap ┐
│ popup (capture/list/settings)   │ ───(stdio)──► │ validate + encrypt │ ───append───► │ 0600 encrypted inbox (0700 dir)  │
│ background.js (native port)     │ ◄──ack only──  │ to vault PUBKEY    │               └──────────────┬───────────────────┘
│ content_consent.js (on gesture) │               │ NO passphrase/priv │   interactive `scout vault import-captures`
└─────────────────────────────────┘               └────────────────────┘   (enter passphrase) ─► per-site vault profile
```

- **Extension** (`extensions/scout-capture/`): UI + consent + capture. Thinnest, riskiest surface; ships last.
- **Native host** (`scout capture-host`, `cmd/scout/capture_host.go`): validate + encrypt + spool. Browser-free TDD.
- **Vault keypair + import** (`pkg/scout/vault` additions + `cmd/scout` `vault import-captures`): decrypt + per-site upsert.

Each unit has one responsibility, a well-defined interface, and is independently testable.

## 2. Unlock flow — the spool inbox

- **Keypair:** a one-time interactive `scout vault capture-key init` (works on a new **or existing**
  vault; prompts the passphrase to unlock) generates an **X25519 keypair** via
  `golang.org/x/crypto/nacl/box` (already in the dep tree). Public key written to
  `<scouthome>/captures/capture.pub` (it is public; `0644`). Private key stored **inside the vault**
  as a Secret (`LockedBuffer`), encrypted under the passphrase like any other vault secret. Idempotent:
  re-running reports the existing key unless `--rotate` is passed.
- **Host-side encrypt:** the host `box.SealAnonymous`-encrypts each capture payload to the public key
  (anonymous-sender sealed box: Curve25519 + XSalsa20-Poly1305, authenticated) → one spool file
  `<scouthome>/captures/spool/<ksuid>.cap` (`0600` in a `0700` dir). The host needs **only** the
  public key; it cannot open the spool.
- **Drain:** `scout vault import-captures` (interactive — prompts the passphrase) unlocks the vault,
  loads the private key, `box.OpenAnonymous`-decrypts each spool file, presents a **review summary**
  (site / username / item kinds — never values), upserts on confirm, then **secure-deletes** the
  spool file (overwrite + remove). A decrypt/parse failure quarantines that file and continues.

## 3. Native-messaging wire protocol (versioned)

Frame format (the native-messaging standard): `[uint32 little-endian length][UTF-8 JSON]`, **≤ 1 MiB**
per message (the browser-side limit). The host reads length-prefixed, enforces the cap, strictly
validates, and reads all input as hostile.

Messages (extension → host), each `{ "v": 1, "type": ... }`:
- `hello` `{ ext_id, nonce }` → host verifies origin + pairing nonce → replies `hello_ack { host_version }`.
- `capture_session` `{ site, cookies: [...], storage: { local: {k:v}, session: {k:v} }, at }` → `ack { id }`.
- `capture_login` `{ site, username, password, at }` → `ack { id }`.

Host → extension: `hello_ack` | `ack { id }` | `error { code, message }`. **Never returns secret material.**
Validation: version allowlist (`v==1`), type allowlist, required-field + type checks, size cap,
per-connection rate limit, reject unknown/malformed with `error` (message echoes no secret value).

## 4. Vault credential schema (per-site, no type change)

A capture for `site` upserts the vault profile with `Name == site` (create if absent, else update via its ID):
- `Cookies`  ← session cookies (`vault.Cookie` = `scout.Cookie`)
- `Storage`  ← web storage (`map[string]OriginStore`)
- `Secrets["login:<username>"]` ← the password bytes (becomes a `LockedBuffer`)

This reuses `SecretProfileInput` exactly — **no new vault type**. The username is encoded in the
Secret key; the Secret value is the password. `scout vault use <site>` replays the full authenticated
state (cookies + storage via the Phase-1 `ApplyToPage`/`ApplyStorageToPage`); the login Secrets are
available via `Handle.Secret("login:<username>")`.

## 5. Consent UX

- **Session capture:** explicit — the user clicks "Capture session for this tab" in the popup. One
  deliberate, visible action; reads cookies (host-scoped, on the `activeTab`) + web storage via an
  injected snapshot script.
- **Login capture (password-manager-style):** the user first toggles "watch this tab for logins"
  in the popup (a per-tab user gesture → injects `content_consent.js` via `chrome.scripting` into the
  **active tab only**). On the **next login-form submit**, an in-page banner appears:

  ```
  ┌──────────────────────────────────────────────┐
  │ Save this login to Scout?                     │
  │ site: example.com   user: alice@example.com   │
  │ [ Save to Scout ]        [ Not now ]          │
  └──────────────────────────────────────────────┘
  ```

  The password leaves the page **only** when the user clicks "Save to Scout". Rules: values are read
  **once, at submit, after consent** (no keylogging); **same-origin top frame only**; **no cross-origin
  iframes**; the "watch" state is per-tab and cleared on navigation/close.

## 6. MV3 hardening (manifest)

- `permissions`: `["nativeMessaging", "activeTab", "scripting", "cookies"]`. **No** `tabs`, `webRequest`,
  `debugger`, or `<all_urls>` content script.
- `host_permissions`: none declared statically. Cookie/script access is per-`activeTab` on a user gesture.
- `key`: a pinned public key in the manifest → **stable extension ID**, so the native-host manifest's
  `allowed_origins` can pin `chrome-extension://<id>/`.
- `content_security_policy`: MV3 default — no remote code, no `eval`.
- The native-host manifest pins `allowed_origins` to the exact extension ID; the host **re-verifies**
  the origin on `hello` and binds a **first-run pairing nonce** (generated host-side, stored `0600`,
  presented once for the user to enter into the popup, then required on every `hello`).

## 7. Passphrase & at-rest isolation

- The vault passphrase is entered **only** in the interactive `scout vault import-captures` (a terminal
  Scout process). It is never in the extension, never in the native host, never on the wire.
- The native host has **no private key and no passphrase** — it cannot decrypt the spool or the vault.
- At rest: spool = sealed-box (authenticated) `0600`; final store = vault (Argon2id + AES-256-GCM +
  `LockedBuffer`/mlock). `chrome.storage` holds **only** non-secret prefs (which tabs are "watched",
  UI state) — never secrets.

## 8. Threat model (STRIDE) — non-goals are enforceable review blockers

| Threat | Mitigation |
|--------|------------|
| **Spoofing** (rogue extension/host) | `allowed_origins` pin + host-side origin re-check + first-run pairing nonce |
| **Tampering** (message/spool) | Strict schema validation; sealed-box AEAD integrity on every spool file |
| **Repudiation** (unknown captures) | Append-only spool; `import-captures` review + metadata-only audit log |
| **Information disclosure** (#1 risk) | Host cannot decrypt; sealed-box at rest; passphrase isolated; ack-only responses; metadata-only logs |
| **DoS** | 1 MiB frame cap; per-connection rate limit; bounded spool size; reject malformed |
| **Elevation of privilege** | No `<all_urls>`; `activeTab` + gesture; same-origin top-frame only; no iframe capture |

**Non-goals (hard review blockers — reject any change that introduces them):** silent/background
capture, keystroke logging, form interception without per-event consent, remote/network transport,
third-party-credential targeting, plaintext secrets at rest, auto-submit/credential replay to
arbitrary sites, cross-origin iframe capture.

## 9. Testing strategy

- **Native host (Phase 1 — primary):** browser-free TDD with a stub stdio client feeding
  length-prefixed JSON. Hostile inputs: oversized frame, truncated length, bad/garbage JSON, unknown
  type/version, wrong origin, missing pairing nonce, replay. Assert: valid → spool file written
  (sealed); invalid → `error` + no spool, no secret echoed.
- **Spool crypto:** seal-with-pubkey → open-with-privkey round-trip; a tampered ciphertext fails to
  open; the host path never has the private key.
- **Import:** spool → per-site vault profile merge (cookies + storage + `login:` Secret); secure-delete
  after; quarantine on decrypt failure.
- **Extension:** documented manual test plan (consent prompt appears only post-toggle + on submit;
  password leaves page only on confirm; no iframe capture); later, a Scout-driven E2E for the consent
  flow.

## 10. Build order (per the roadmap)

1. **Phase 1 — native host + spool + vault keypair + `import-captures`** (Go only; testable without a
   browser via a stub). Delivers the secure backend end-to-end.
2. **Phase 2 — MV3 extension: session capture** (cookies + storage; no passwords). Full value, lower risk.
3. **Phase 3 — consented credential (password) capture.** Gated by its own security review.
4. **Phase 4 — hardening / signing / packaging / audit.**
5. **Phase 5 — Firefox parity (optional).**

This spec covers Phases 1–3's contracts; each phase still gets its own implementation plan. The
**first** implementation plan (next step) covers **Phase 1 only — the native host, spool, vault
keypair, and `import-captures`** — the secure backend, no browser.
