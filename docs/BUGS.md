# Bug Tracker

## Open Bugs

(none)

## Resolved Bugs

| Bug                               | Resolution                                                                       | Date    |
|-----------------------------------|----------------------------------------------------------------------------------|---------|
| Page.Race always returns index -1 | Fixed: now matches returned element against selectors to determine winning index | 2026-02 |
| CLI commands fail with mTLS server (EOF) | Fixed: replaced `getClient(addr)` with `resolveClient(cmd)` in inspect, interact, har, network, storage, window, session commands | 2026-02 |
| Server sessions timeout after 30s | Fixed: disabled per-page rod timeout (`WithTimeout(0)`) for server sessions — rod's `Page.Timeout()` creates a one-shot context that expires permanently | 2026-02 |
