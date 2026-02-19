# Bug Tracker

## Open Bugs

### Window maximize blank space

- **Severity:** Medium
- **Reported:** 2026-02-18
- **Description:** Browser window maximize (`WithWindowState(WindowMaximized)` or `scout window max`) leaves blank/white space in the viewport instead of filling the entire screen area.
- **Reproduction:** Launch browser with `WithHeadless(false)`, call `window.SetMaximized()` or use CLI `scout window max`.
- **Suspected cause:** Chrome window state transition timing — viewport may not resize to match the maximized window bounds.

## Resolved Bugs

| Bug                               | Resolution                                                                       | Date    |
|-----------------------------------|----------------------------------------------------------------------------------|---------|
| Page.Race always returns index -1 | Fixed: now matches returned element against selectors to determine winning index | 2026-02 |
| CLI commands fail with mTLS server (EOF) | Fixed: replaced `getClient(addr)` with `resolveClient(cmd)` in inspect, interact, har, network, storage, window, session commands | 2026-02 |
| Server sessions timeout after 30s | Fixed: disabled per-page rod timeout (`WithTimeout(0)`) for server sessions — rod's `Page.Timeout()` creates a one-shot context that expires permanently | 2026-02 |
