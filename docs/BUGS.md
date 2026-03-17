# Bug Tracker

## Open Bugs

(No open bugs)

## Resolved Bugs

| Bug                               | Resolution                                                                       | Date    |
|-----------------------------------|----------------------------------------------------------------------------------|---------|
| Page.Race always returns index -1 | Fixed: now matches returned element against selectors to determine winning index | 2026-02 |
| CLI commands fail with mTLS server (EOF) | Fixed: replaced `getClient(addr)` with `resolveClient(cmd)` in inspect, interact, har, network, storage, window, session commands | 2026-02 |
| Server sessions timeout after 30s | Fixed: disabled per-page rod timeout (`WithTimeout(0)`) for server sessions — rod's `Page.Timeout()` creates a one-shot context that expires permanently | 2026-02 |
| Window maximize blank space | Fixed: `setWindowState()` clears `EmulationDeviceMetricsOverride` after maximize/fullscreen | 2026-02 |
| MCP `screenshot`/`navigate` timeout (`context deadline exceeded`) | Fixed: `WithTimeout(0)` disables rod 30s page timeout for MCP; `WaitLoad` made best-effort with 15s cap | 2026-03 |
| MCP session disconnect after `session_reset` | Fixed: close page before browser + 500ms delay for OS port/dir cleanup | 2026-03 |
| Sitemap extract fails on Chrome for Testing after first page | Fixed: stale `Bridge.available` flag never reset between navigations; `ResetReady()` added before each `page.Navigate()` | 2026-03 |
