# Scout Capture Phase 2 — manual E2E acceptance checklist

Prereqs: Chrome installed; `go build ./cmd/scout/` produced a `scout` on PATH;
a throwaway test site you can log into (or a local page that sets a cookie +
localStorage).

1. [ ] `scout vault capture-key init` → records the pairing nonce. (If no vault
       exists yet, create one first per `scout vault init`.)
2. [ ] Load `extensions/scout-capture/` unpacked; copy the extension ID.
3. [ ] `scout capture-host install <id>` → "installed native-messaging manifest".
4. [ ] Popup → paste nonce → Save nonce → "Nonce saved."
5. [ ] Log into the test site in the active tab.
6. [ ] Popup → "Capture session for this tab" → expect "Captured ✓ (spool id …)".
       The audit list shows `<site> — <timestamp>` (NO values).
7. [ ] Confirm a sealed spool file exists: it is unreadable (sealed box), 0600.
8. [ ] `scout vault import-captures` → enter passphrase → review summary shows the
       site + item kinds (no values) → confirm → "imported 1 capture…".
9. [ ] `scout vault use <site>` in a headless Scout run → the site loads
       **already authenticated** (cookies + storage replayed). ✅ = pass.
10. [ ] Negative checks: with the WRONG nonce saved, capture yields an `error`
        ("origin/nonce rejected") and writes NO spool file. Uninstall
        (`scout capture-host uninstall`) then capture → host unreachable, no spool.
