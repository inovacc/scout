# ADR-0005: Generic Auth Framework

## Status

Accepted

## Date

2026-02-16

## Context

Scout's Slack scraper implements a browser-based authentication flow: launch browser, user logs in, extract tokens/cookies, persist encrypted session. This pattern is specific to Slack (xoxc token + d cookie extraction) but the underlying flow is identical for any web application:

1. Launch browser to login URL
2. Wait for user to authenticate
3. Capture session state (cookies, localStorage, sessionStorage, tokens)
4. Encrypt and persist session
5. Restore session in headless browser for autonomous scraping

As we add more scraper modes (Teams, Discord, Gmail, etc.), duplicating this flow in each package would lead to significant code duplication and inconsistency.

## Decision

Extract the auth-then-scrape pattern into a generic framework at `scraper/auth/` with:

- **`Provider` interface** — each scraper mode implements `Name()`, `LoginURL()`, `DetectAuth()`, `CaptureSession()`, `ValidateSession()`
- **`Registry`** — global registry of providers, allowing CLI commands to work with any registered provider
- **`BrowserAuth`** — generic browser-based auth flow that uses the Provider interface to detect login completion
- **`BrowserCapture`** — "launch browser and capture everything before close" flow for generic use without provider-specific auth detection
- **`OAuthServer`** — local HTTP callback server for OAuth2 PKCE flows
- **`ElectronSession`** — connect to Electron apps via Chrome DevTools Protocol
- **Generic session type** — `auth.Session` replaces provider-specific session types with a unified structure (cookies, tokens, storage, extras)
- **Encrypted persistence** — reuses existing `scraper.EncryptData/DecryptData` (AES-256-GCM + Argon2id)

## Consequences

### Positive

- New scraper modes only need to implement the `Provider` interface (5 methods)
- CLI `scout auth login/capture/status/logout/providers` works for all providers
- `BrowserCapture` enables ad-hoc session capture from any website without writing provider code
- Electron app support is built into the framework
- OAuth2 PKCE support for services that use standard OAuth

### Negative

- Slack's `CapturedSession` type is now partially redundant (kept for backward compat with existing `scout slack capture/load/decrypt` commands)
- Provider-specific validation logic still lives in each provider's package

## Files

| File | Purpose |
|------|---------|
| `scraper/auth/provider.go` | Provider interface, Registry |
| `scraper/auth/browser_auth.go` | BrowserAuth, BrowserCapture flows |
| `scraper/auth/session.go` | Generic encrypted session persistence |
| `scraper/auth/oauth.go` | OAuth2 PKCE local callback server |
| `scraper/auth/electron.go` | Electron CDP connection |
| `scraper/slack/provider.go` | SlackProvider implements auth.Provider |
| `cmd/scout/internal/cli/auth.go` | CLI auth subcommands |
