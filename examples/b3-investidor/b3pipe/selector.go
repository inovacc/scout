package b3pipe

// SelectEngine maps the recon-discovered refresh-token location to a replay engine.
// JS-readable (localStorage/sessionStorage/non-httpOnly cookie) -> A (browserless flow).
// httpOnly-only cookie -> B (headless browser self-refresh). Not found -> fallback.
func SelectEngine(info RefreshTokenInfo) Engine {
	if !info.Found {
		return EngineFallback
	}
	if info.InLocalStorage || info.InSessionStorage || info.InReadableCookie {
		return EngineA
	}
	if info.InHTTPOnlyCookie {
		return EngineB
	}
	return EngineFallback
}
