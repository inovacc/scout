# ADR: Surfshark VPN Extension Integration

## Status: Proposed

## Context

Scout needs the ability to programmatically change VPN server locations during browser automation sessions. The Surfshark VPN Chrome extension (v4.38.1, MV3) is already available as an unpacked extension at `extract/surfshark-vpn-extension/`. By loading it via `WithExtension()` and controlling it via CDP, Scout can rotate IP addresses per-page or per-session without external VPN software.

## Analysis of Surfshark Extension Internals

### Architecture

```
manifest.json (MV3)
├── background.bundle.js     ← Service worker: proxy control, auth, server list
├── popup.bundle.js          ← UI: server selection, connect/disconnect
├── cs-isolated-world.js     ← Content script: WebRTC leak prevention
├── cs-main-world.js         ← Content script: main world injections
├── cleanweb.bundle.js       ← Ad/tracker/cookie blocker (DNR rules)
└── filter-lists/            ← Declarative net request filter lists
```

### API Endpoints (ext.surfshark.com)

| Endpoint | Purpose |
|----------|---------|
| `POST /v1/auth/login` | Authenticate with email/password |
| `POST /v1/auth/renew` | Refresh auth token |
| `POST /v1/auth/logout` | End session |
| `GET /v1/server/user` | Get user's proxy credentials (username/password) |
| `GET /v5/server/clusters/all` | Full server list (all locations) |
| `GET /v5/server/suggest` | Suggested/optimal server |
| `GET /v5/server/suggest/from` | Suggest from current location |
| `GET /v5/server/suggest/foreign` | Suggest foreign server |
| `GET /v1/server/dedicated` | Dedicated/static IP servers |
| `GET /v1/account/users/me` | Account info |
| `GET /v1/payment/subscriptions/current` | Subscription status |
| `GET /v1/filters/` | Server filter categories |
| `GET /v1/filters/extras` | Extra filter categories |

### Proxy Mechanism

The extension uses **Chrome's `chrome.proxy.settings` API** with `fixed_servers` mode:

```javascript
chrome.proxy.settings.set({
  value: {
    mode: "fixed_servers",
    rules: {
      singleProxy: {
        host: "<server-hostname>",  // e.g. "us-nyc.prod.surfshark.com"
        port: 443,
        scheme: "https"
      }
    }
  }
});
```

- **Protocol**: HTTPS proxy on port 443
- **Authentication**: `chrome.webRequest.onAuthRequired` listener provides `{username, password}` from proxy credentials fetched via `/v1/server/user`
- **Disconnect**: `chrome.proxy.settings.clear()`

### Authentication Flow

1. User logs in → `POST /v1/auth/login` → returns JWT token
2. Token stored in `chrome.storage.session` (key: `"key"`)
3. Token refreshed via `POST /v1/auth/renew` (key: `"auth-renew-token"`)
4. Proxy credentials fetched via `GET /v1/server/user` → returns `{username, password}`
5. `onAuthRequired` listener auto-provides credentials for proxy auth

### Storage Keys

| Key | Storage | Purpose |
|-----|---------|---------|
| `"key"` | session | Encrypted session key |
| `"auth-token"` | local | JWT auth token |
| `"auth-renew-token"` | local | Refresh token |
| `"auth-status"` | local | Login state (`auth-logged-in`, `auth-logged-out`, etc.) |
| `"connection-was-connected"` | local | Was VPN connected before |
| `"connection-count"` | local | Connection counter |
| `"favorite-servers"` | local | User's favorite server list |
| `"setting-autoconnect-enabled"` | local | Auto-connect on startup |
| `"setting-autoconnect-server"` | local | Auto-connect server |
| `"vpn-sessions"` | local | Active VPN session tracking |
| `"vpn-events-queue"` | local | Event queue for analytics |
| `"adblock-enabled"` | local | CleanWeb ad blocking |
| `"breach-alert-enabled"` | local | Breach alert feature |
| `"bypasser-enabled"` | local | Split tunneling |
| `"bypass-list"` | local | Bypassed domains |

### Privacy Features

- **WebRTC leak prevention**: `chrome.privacy.network.webRTCIPHandlingPolicy` + `peerConnectionEnabled`
- **CleanWeb**: `declarativeNetRequest` with ad/cookie/phishing filter lists
- **Bypass (split tunnel)**: Per-domain bypass rules via `bypass-list` storage key

### Connection Flow (step-by-step)

```
1. Load extension via WithExtension()
2. Set auth token in chrome.storage.local via CDP:
   - "auth-token": "<jwt>"
   - "auth-renew-token": "<refresh>"
   - "auth-status": "auth-logged-in"
3. Fetch proxy credentials:
   GET ext.surfshark.com/v1/server/user → {username, password}
4. Fetch server list:
   GET ext.surfshark.com/v5/server/clusters/all → [{host, countryCode, ...}]
5. Select server → set proxy via chrome.proxy.settings.set():
   {mode: "fixed_servers", rules: {singleProxy: {host, port: 443, scheme: "https"}}}
6. onAuthRequired auto-provides username/password
7. All traffic now routed through selected VPN server
8. To switch: repeat step 5 with new host
9. To disconnect: chrome.proxy.settings.clear()
```

## Decision

### Phase 33: VPN Extension Integration

Build a `pkg/scout/vpn.go` module that:

1. **VPN Provider interface** — Pluggable VPN backends (Surfshark first, extensible to NordVPN, ExpressVPN, etc.)
2. **Surfshark provider** — Implements the flow above using CDP to control the loaded extension
3. **Server rotation** — `WithVPNRotation(interval, countries)` rotates servers per-interval or per-page
4. **Go API** — `Browser.VPNConnect(country)`, `Browser.VPNDisconnect()`, `Browser.VPNStatus()`, `Browser.VPNServers()`
5. **CLI** — `scout vpn connect --country=us`, `scout vpn disconnect`, `scout vpn status`, `scout vpn servers`

### Implementation approach

Rather than reverse-engineering the extension's internal message passing, **control it via CDP**:

```go
// Set auth tokens directly in extension storage
page.Eval(`chrome.storage.local.set({"auth-token": "...", "auth-status": "auth-logged-in"})`)

// Or simpler: use chrome.proxy.settings directly (bypass extension entirely)
page.Eval(`chrome.proxy.settings.set({value: {
  mode: "fixed_servers",
  rules: {singleProxy: {host: "us-nyc.prod.surfshark.com", port: 443, scheme: "https"}}
}})`)

// Handle auth via webRequest
page.Eval(`chrome.webRequest.onAuthRequired.addListener(
  (details) => ({authCredentials: {username: "...", password: "..."}}),
  {urls: ["<all_urls>"]}, ["blocking"]
)`)
```

### Alternative: Direct proxy without extension

Since the core mechanism is just an HTTPS proxy with auth, Scout could skip the extension entirely:

```go
// Use rod's built-in proxy support
scout.New(
  scout.WithProxy("https://us-nyc.prod.surfshark.com:443"),
  scout.WithProxyAuth("username", "password"),
)
```

This is simpler but loses CleanWeb, WebRTC leak prevention, and bypass features.

## Consequences

- **Positive**: IP rotation per-page/session, geo-unblocking, bot detection evasion via diverse exit IPs
- **Positive**: Works with any HTTPS proxy VPN (Surfshark, NordVPN, PIA — all use similar proxy infrastructure)
- **Negative**: Requires valid Surfshark subscription and credentials
- **Negative**: Extension approach is fragile (extension updates may break control flow)
- **Recommendation**: Implement both approaches — extension loading for full features, direct proxy for simplicity
