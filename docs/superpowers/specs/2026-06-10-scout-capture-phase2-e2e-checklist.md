# Scout Capture Phase 2 — E2E acceptance checklist

> **Verification status — VERIFIED 2026-06-10.** The automated driver
> `hacks/captureE2E` (commit `075ccb5`) was run fresh against `main` at `4a4cf76`:
> **20 steps, 0 failed, exit 0** (isolated Chrome-for-Testing + isolated
> `SCOUT_HOME`, self-cleaning). It exercises the SHIPPED `extensions/scout-capture`
> (confirming `chrome.storage.local` is available — i.e. the `9dc80ba` storage
> permission fix), the native-messaging capture → sealed spool → `vault
> import-captures` → `vault use` replay (**AUTHENTICATED**), and both negative
> checks. Re-run any time with `go run ./hacks/captureE2E`.
>
> **Automation coverage:** items 1–3, 5, 7–10 are FULLY automated. Items 4 and 6
> are automated at the FUNCTIONAL level (nonce written to `chrome.storage` via CDP;
> `captureSession()` driven over the native-messaging path) but the **rendered
> popup button gesture** (the `activeTab`-granting click in the extension popup) is
> NOT CDP-automatable and remains the one **human-only** residual — standard,
> low-risk Chrome behavior. Run the manual steps below to confirm the popup UX.

Prereqs (manual run): Chrome installed; `go build ./cmd/scout/` produced a `scout`
on PATH; a throwaway test site you can log into (or a local page that sets a
cookie + localStorage).

Legend: `[x]` automated by `hacks/captureE2E`; `[~]` functional path automated,
popup gesture human-only; manual run still validates the popup UX end-to-end.

1. [x] `scout vault capture-key init` → records the pairing nonce. (If no vault
       exists yet, create one first per `scout vault init`.)
2. [x] Load `extensions/scout-capture/` unpacked; copy the extension ID.
3. [x] `scout capture-host install <id>` → "installed native-messaging manifest".
4. [~] Popup → paste nonce → Save nonce → "Nonce saved." *(automated: nonce saved
       to chrome.storage via CDP; popup click gesture is human-only)*
5. [x] Log into the test site in the active tab.
6. [~] Popup → "Capture session for this tab" → expect "Captured ✓ (spool id …)".
       The audit list shows `<site> — <timestamp>` (NO values). *(automated:
       captureSession() over native messaging; popup click gesture is human-only)*
7. [x] Confirm a sealed spool file exists: it is unreadable (sealed box), 0600.
       *(NOTE: on Windows the spool reports mode 0666 — an ACL artifact; sealed/
       not-plaintext is confirmed by the driver.)*
8. [x] `scout vault import-captures` → enter passphrase → review summary shows the
       site + item kinds (no values) → confirm → "imported 1 capture…".
9. [x] `scout vault use <site>` in a headless Scout run → the site loads
       **already authenticated** (cookies + storage replayed). ✅ = pass.
10. [x] Negative checks: with the WRONG nonce saved, capture yields an `error`
        ("origin/nonce rejected") and writes NO spool file. Uninstall
        (`scout capture-host uninstall`) then capture → host unreachable, no spool.
