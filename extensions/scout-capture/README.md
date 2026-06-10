# Scout Capture (MV3) — session capture

Captures **this** browser's own session (cookies + web storage) for the **active tab**,
on an explicit click, into your local Scout vault via the `scout capture-host`
native-messaging host. Local only — no network, no passwords (passwords are Phase 3).

## One-time setup

1. **Create the vault capture key + pairing nonce** (prints the nonce):
   ```
   scout vault capture-key init
   ```
2. **(Optional) Pin a stable extension ID.** Run:
   ```
   scout capture-host keygen
   ```
   Copy the printed `"key": "..."` line into `manifest.json`. Skip this to use a
   random dev ID instead.
3. **Load the extension:** Chrome → `chrome://extensions` → enable Developer mode →
   *Load unpacked* → select `extensions/scout-capture/`. Note the **extension ID** shown.
4. **Register the native host for that ID:**
   ```
   scout capture-host install <extension-id>
   ```
5. **Pair:** open the extension popup, paste the pairing nonce from step 1, click
   *Save nonce*.

## Use

- Open the popup on any tab you are logged into → *Capture session for this tab*.
- Drain captures into the vault (prompts the passphrase, shows a review):
  ```
  scout vault import-captures
  ```
- Replay headlessly to prove it worked:
  ```
  scout vault use <site>
  ```

## Security

Least privilege: `nativeMessaging`, `activeTab`, `scripting`, `cookies` — no
`<all_urls>`, no `tabs`/`webRequest`/`debugger`, no host permissions, no remote
code. The popup nonce and capture are the only ways anything leaves a page, and
only for the active tab on your click. Secrets never touch `chrome.storage`
(only the nonce + metadata audit live there); they go straight to the encrypted
spool and then the vault.
