# Scout Capture — Browser Credential/Session Capture Extension (Roadmap)

**Date:** 2026-06-09
**Status:** Roadmap (pre-brainstorm). Phase 0 must produce an approved design spec + threat model before any code.
**Owner:** project owner, for **self-capture of their own browser sessions and logins** into Scout's local vault.

## 1. Intent & legitimacy boundary

Scout already captures a *headless* browser's session into `pkg/scout/vault` (Argon2id + AES-256-GCM,
`LockedBuffer`/mlock; cookies + web storage). This feature extends that to the user's **own real
browser**: an MV3 extension that, on an explicit user action, hands the active session (and,
later, consented login credentials) to a **local** Scout native-messaging host, which writes them
into the same vault. It is the personal-tooling analog of a password manager's browser integration.

**In scope (legitimate, self-service):** capturing *your own* authenticated sessions and *your own*
logins, on *your own* machine, into *your own* encrypted local vault, over a *local-only* channel.

**Hard non-goals (explicitly excluded — these are abuse patterns, not features):**
- ❌ Silent/background capture, keystroke logging, or form interception without per-event consent.
- ❌ Capturing anyone's credentials but the operator's own (no targeting third parties).
- ❌ Any remote/network transport or third-party endpoint (exfiltration surface).
- ❌ Plaintext persistence of secrets in `chrome.storage` or anywhere outside the vault.
- ❌ Broad `<all_urls>` host access or persistent background scraping.
- ❌ Auto-submit, credential replay to arbitrary sites, or capture inside cross-origin iframes.

If a future change would cross any of these lines, it must be rejected at review.

## 2. Architecture

```
┌─────────────────────────┐   native messaging (length-prefixed JSON over stdio)   ┌──────────────────────┐      ┌──────────────────┐
│  MV3 extension          │ ───────────────────────────────────────────────────►   │ scout capture-host   │ ───► │ pkg/scout/vault   │
│  (popup + content +     │   (OS-mediated; no sockets, no network)                 │ (Go, cmd/scout)      │      │ (Argon2id+AESGCM, │
│   service worker)       │ ◄───────────────────────────────────────────────────   │  - verifies ext ID   │      │  LockedBuffer)    │
└─────────────────────────┘   ack/status only (never secrets back)                  │  - schema+size caps  │      └──────────────────┘
                                                                                     │  - passphrase in-host│
                                                                                     └──────────────────────┘
```

- **Extension** (`extensions/scout-capture/`, MV3): popup UI, a content script injected only on a
  user gesture for the active tab, and a service worker that owns the native-messaging port.
- **Native host** (`scout capture-host`, a `cmd/scout` subcommand): registered via the per-OS native
  messaging manifest with `allowed_origins` pinned to the exact extension ID. Reads length-prefixed
  JSON from stdin, strictly validates, writes to the vault. The vault passphrase is entered **in the
  host** (Scout TUI prompt or a pre-unlocked session) — never sent from or known to the extension.
- **Vault sink:** reuses `SecretProfileInput` (cookies + `OriginStore` web storage from the recent
  profile→vault work) and stores login credentials as vault `Secrets` (`LockedBuffer`), keyed by
  `site|username`.

## 3. Security model (best practices — the core requirement)

| Principle | Mechanism |
|-----------|-----------|
| **Consent-first, visible** | Every capture is user-initiated. Session capture = click in popup for the active tab. Credential capture = a visible "Save this login to Scout?" prompt on form submit, confirmed per-event. No silent capture, ever. |
| **Least privilege (MV3)** | Permissions: `nativeMessaging`, `activeTab`, `scripting`, `cookies` (host-scoped, on gesture). NO `<all_urls>`, NO persistent background, NO `webRequest` body interception. |
| **Local-only transport** | Native messaging (stdio, OS-mediated). The host opens no socket and makes no network call. No `host_permissions` enabling remote fetch. |
| **Authenticated channel** | Native-messaging manifest pins `allowed_origins` to the extension ID; host re-verifies. First-run pairing nonce stored host-side. |
| **Secrets at rest** | Straight into the vault (Argon2id + AES-256-GCM + `LockedBuffer`/mlock). Never in `chrome.storage.local`. In-memory lifetime minimized; buffers zeroed. |
| **Passphrase isolation** | Vault passphrase entered in the host process only. The extension cannot decrypt the vault and never sees the passphrase. |
| **No remote code** | MV3 default CSP; no `eval`; no remotely-hosted scripts; no analytics/telemetry. |
| **Hostile-input hardening** | Host strictly validates JSON schema, caps message size (`io.LimitReader` pattern from HARDENING-V2), rate-limits, rejects unknown origins. |
| **Auditability** | Signed + reproducibly-built extension; every capture logged (metadata only: site/username/timestamp — never values) to Scout's logger; a "captured items" view with revoke. |

### Threat model (STRIDE, abbreviated)
- **Spoofing:** a rogue extension talking to the host → `allowed_origins` pin + host-side ID check + pairing nonce.
- **Tampering:** message tampering on the channel → native messaging is local OS IPC; strict schema validation; reject malformed.
- **Repudiation:** unknown captures → append-only metadata audit log + user-visible captured-items list.
- **Information disclosure:** the #1 risk → no remote transport; secrets only in the vault; passphrase isolated; metadata-only logging; no plaintext at rest.
- **DoS:** oversized/looping messages → size caps + rate limit + bounded reads in the host.
- **Elevation:** extension reaching beyond its tab → no `<all_urls>`, per-gesture `activeTab`, no iframe capture.

## 4. Roadmap (phased; each phase is a Superpowers brainstorm→spec→plan→execute cycle)

> Sequencing rule: ship the **lower-risk session capture before** the credential (password) capture,
> so value lands early and the password surface gets its own dedicated security gate.

### Phase 0 — Design spec + threat model  *(no code)*
Brainstorm → `docs/superpowers/specs/…-scout-capture-design.md`: finalize the consent UX, exact
permission set, the native-messaging wire schema (versioned), the vault schema extension for
credentials, the STRIDE threat model, and the non-goals as enforceable review criteria.
**Gate:** security review of the design before Phase 1.

### Phase 1 — Native messaging host (`scout capture-host`)  *(Go only)*
The host side first, testable without a browser via a stub client. Registers the per-OS native
messaging manifest (install/uninstall command), reads length-prefixed JSON, verifies extension ID,
strict schema + size caps + rate limit, writes session bundles to the vault, passphrase prompt
in-host. **TDD incl. hostile-input/fuzz tests.** Reuses `vault.SecretProfileInput` + `Vault.Set`.

### Phase 2 — MV3 extension: session capture  *(lower-risk bundle)*
Popup + `activeTab` capture of cookies + web storage on explicit user click → native host → vault.
End-to-end proof: capture in the real browser, then `scout vault use` replays the session headless.
Delivers full value with **zero password handling**. Extension permission-minimization audit.

### Phase 3 — Consented credential capture  *(the password surface)*
Password-manager-style: on login-form submit, a visible "Save this login to Scout?" prompt; explicit
per-event confirm; stored as vault `Secrets`. Safeguards: field allow-listing, same-origin only, no
iframe capture, no autofill scraping, no keylogging, debounce, clear which fields are read.
**Gate:** dedicated security review + threat re-assessment specific to credential handling.

### Phase 4 — Hardening, packaging, audit
Extension signing + reproducible build, CSP audit, final permission-minimization audit, per-OS native
host installer, user docs, the captured-items audit/revoke view, and a full security sign-off
(`/security-review`). Verify no remote endpoints, no plaintext at rest, allowed_origins pinned.

### Phase 5 — Firefox parity  *(optional)*
Firefox native messaging + MV3 differences; same security model.

## 5. Risk register
| Risk | Severity | Mitigation |
|------|----------|------------|
| Extension compromise leaks live sessions | High | Local-only transport; vault-only at rest; passphrase isolation; least privilege; signed/auditable build |
| Credential-capture technique repurposed maliciously | High | Consent-first design; non-goals enforced at review; password phase gated + sequenced last; metadata-only logs |
| Native host accepts rogue clients | Med | `allowed_origins` pin + ID re-check + pairing nonce; schema/size/rate limits |
| Passphrase exposure | High | Entered in host only; never crosses the channel |
| Scope creep into silent capture | High | Documented non-goals as hard review blockers; consent UX is load-bearing |

## 6. Reuses / dependencies
- `pkg/scout/vault` (`SecretProfileInput`, `OriginStore`, `Vault.Set/Use`, `CaptureFromPage` model) — the
  profile→vault migration (`feat/profile-vault-phase1`) is the substrate this builds on.
- `extensions/scout-bridge` — existing MV3 extension precedent (embed pattern, build).
- HARDENING-V2 input-bounding patterns (`io.LimitReader`, size caps) for the host.
- `internal/logger` for metadata-only audit logging.

## 7. Definition of done (whole feature)
Local, consent-first capture of the operator's own sessions and logins into the vault; zero remote
transport; zero plaintext at rest; least-privilege MV3; signed extension; passphrase never leaves the
host; captured-items audit + revoke; security sign-off on every phase that touches secrets.
