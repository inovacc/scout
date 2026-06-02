/*
Copyright (c) 2026 Security Research

Capability inspection helpers for the package-level sampling singleton.
These are non-blocking accessors over the cached InitializeParams advertised
by the connected host (Claude Code) at MCP handshake time.

D-45-CAPABILITY-PROBE: callers MUST treat a `false` return as authoritative
for the lifetime of the current session — graceful-degrade to NilMCPClient
semantics rather than spinning a retry loop.
*/
package mcp

import "context"

// HasSamplingCapability reports whether the connected MCP host advertised
// the `sampling` client capability during initialize.
//
// Non-blocking, no I/O: this only reads the cached InitializeParams the
// go-sdk stored on the ServerSession during handshake. The ctx parameter
// is accepted for API symmetry with other MCP helpers but is not used.
//
// Returns false in any of the following cases:
//   - SetSession was never called or was cleared (session == nil)
//   - The host did not include a Capabilities block at initialize
//   - The host's Capabilities.Sampling field is nil
//
// A `true` return means the host is willing in principle to handle
// `sampling/createMessage` reverse-RPC; it does not guarantee any specific
// CreateMessage call will succeed (the host may still deny per-prompt).
func HasSamplingCapability(_ context.Context) bool {
	sessionMu.RLock()
	ss := session
	sessionMu.RUnlock()
	if ss == nil {
		return false
	}
	params := ss.InitializeParams()
	if params == nil || params.Capabilities == nil {
		return false
	}
	return params.Capabilities.Sampling != nil
}
