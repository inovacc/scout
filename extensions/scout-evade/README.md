# scout-evade

A Chrome MV3 extension that injects Scout's anti-bot-detection stealth evasions
into every page at `document_start` in the **MAIN world** (page JS context),
before the page's own scripts run.

## What it does

`evasions.js` is a faithful port of all 17 evasions from
`internal/engine/stealth/stealth_extra.go`, plus `navigator.webdriver = false`
from Scout's main stealth layer. Each evasion is individually try/catch guarded
so one failure cannot abort the rest, and an idempotency guard prevents
double-injection if the extension is loaded multiple times.

| # | Evasion |
|---|---------|
| 0 | `navigator.webdriver` → `false` |
| 1 | Canvas `toDataURL` / `getImageData` — subtle RGB noise |
| 2 | `AudioContext.createOscillator` — micro gain-node noise |
| 3 | WebGL `UNMASKED_VENDOR/RENDERER` → Intel Inc. / Intel Iris OpenGL Engine |
| 4 | `navigator.connection` → 4G / 10 Mbps / 50 ms RTT |
| 5 | `Notification.permission` → `"default"` |
| 6 | WebRTC `createOffer` SDP / `onicecandidate` — strip local-IP candidates |
| 7 | `document.fonts.check` / `.forEach` — normalise to common font set |
| 8 | `screen.width/height/availWidth/availHeight/colorDepth/pixelDepth` — match viewport |
| 9 | `navigator.getBattery` → charging, level 1.0 |
| 10 | Remove ChromeDriver leak globals (`cdc_*`, `$cdc_*`, `callPhantom`, `_phantom`, `__nightmare`, `domAutomation*`) |
| 11 | `navigator.hardwareConcurrency` → 8, `deviceMemory` → 8, `vendor` → "Google Inc." |
| 12 | `document.hasFocus()` → always `true` |
| 13 | `window.outerWidth/outerHeight` — fix headless zero values |
| 14 | `navigator.languages` → `["en-US","en"]`, `navigator.language` → `"en-US"` |
| 15 | `navigator.plugins` / `mimeTypes` — inject 5 PDF-viewer entries |
| 16 | `Intl.DateTimeFormat` — force timezone to `America/New_York` when UTC |
| 17 | `Function.prototype.toString` — make overridden functions look native |

### Static defaults

Several evasions hard-code values that Scout's Go layer injects dynamically
via CDP. Where a static default was chosen, the code has a `// TODO: parameterize`
comment:

- **Timezone** (evasion 16): defaults to `America/New_York`
- **Locale** (evasion 14): defaults to `en-US`
- **WebGL strings** (evasion 3): defaults to `Intel Inc.` / `Intel Iris OpenGL Engine`

To make these dynamic in an extension context you would need a background
service worker that reads user preferences and passes them to the content script
via `chrome.storage` before injection — that is out of scope for v0.1.

## Honest limitations

This extension operates at the **JavaScript layer only**. It cannot change:

- TLS fingerprint / JA3 signature
- HTTP/2 fingerprint (ALPN, frame order)
- Chrome Privacy Attestation tokens (PAT / Private State Tokens)
- Low-level CDP-detectable automation markers (requires Scout's CDP-level patches)
- **Web Worker / Service Worker `navigator` and User-Agent** — MV3 content scripts do
  **not** execute in dedicated/shared/service Worker scopes (in *any* `world`), so this
  extension cannot patch a worker's `navigator`/UA. A worker leaking the real
  `HeadlessChrome` UA or unpatched `navigator` values (the `hasInconsistentWorkerValues`
  bot signal) must be fixed at the CDP layer — Scout sets the UA as a process-global
  `--user-agent` switch and (planned) injects the evasions into each worker session.

For full bot evasion, pair this extension with Scout's built-in stealth layer
(`WithStealth()` or `SCOUT_STEALTH=true`), which applies additional CDP-level
patches that a browser extension cannot reach.

## Loading the extension

### Option A — normal Chrome (developer mode)

1. Open `chrome://extensions`
2. Enable **Developer mode** (toggle, top-right)
3. Click **Load unpacked**
4. Select this directory (`extensions/scout-evade/`)

### Option B — Scout library (`WithExtension`)

```go
import "github.com/inovacc/scout/pkg/scout"

b, err := scout.New(
    scout.WithExtension("extensions/scout-evade"),
    scout.WithHeadless(false), // extensions require non-headless or --disable-extensions=false
)
```

### Option C — Scout CLI (`scout extension load`)

The Scout CLI exposes a dedicated `extension` command group. Use `scout extension load`:

```sh
scout extension load --path extensions/scout-evade --url https://example.com
```

This opens a non-headless browser with the extension loaded and navigates to
the given URL. Press Ctrl+C to exit. There is no bare `--extension` flag on
other commands (e.g. `scout scrape`); you must use either the `extension load`
subcommand or the Go library's `WithExtension()` option.

## Chrome version requirement

`"world": "MAIN"` in MV3 content scripts requires **Chrome 111 or later**.
The Scout target is Chrome 149 — no issue.

### Fallback for Chrome < 111 (ISOLATED-world injection)

If you need to support an older Chrome that does not honour `"world": "MAIN"`,
implement a two-file approach:

1. Keep `evasions.js` as a [web accessible resource](https://developer.chrome.com/docs/extensions/reference/manifest/web-accessible-resources).
2. Add a second content script (`loader.js`, `"world": "ISOLATED"`) that
   creates a `<script>` element pointing at `chrome.runtime.getURL("evasions.js")`
   and appends it to `document.documentElement` before `DOMContentLoaded`.

```js
// loader.js (ISOLATED world, document_start)
const s = document.createElement('script');
s.src = chrome.runtime.getURL('evasions.js');
s.async = false;
(document.head || document.documentElement).prepend(s);
```

```json
// manifest.json additions
"web_accessible_resources": [{
  "resources": ["evasions.js"],
  "matches": ["<all_urls>"]
}],
"content_scripts": [{
  "matches": ["<all_urls>"],
  "js": ["loader.js"],
  "run_at": "document_start",
  "all_frames": true
}]
```

This fallback is **not implemented** here — `world: "MAIN"` is used directly
because Scout targets Chrome 111+.
