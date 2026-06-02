/*
Copyright (c) 2026 Security Research

Package mcp provides a reusable MCP sampling library. It lets any Go MCP
server ask its connected client (e.g., Claude Code) for an AI completion via
the standard sampling/createMessage reverse-RPC, without depending on any
provider SDK.

# Pattern

Wire the session once in the AfterConnect hook of your MCP server:

	mcptools.Serve(ctx, mcptools.ServerConfig{
	    AfterConnect: func(ss *gomcp.ServerSession) {
	        mcp.SetSession(ss, logger)
	    },
	})

Then call Sample from any tool handler:

	body, err := mcp.Sample(ctx, "Summarise the following code:\n"+src)
	if err != nil {
	    // No session wired or host denied — degrade gracefully.
	    return fallback(ctx, src)
	}

Or use the Adapter wrapper when you need a named client bound to a subsystem:

	client := mcp.NewAdapter("forensic")
	body, err := client.Sample(ctx, prompt)

# Capability probe

Before calling Sample, you can check whether the connected host actually
advertises the sampling capability:

	if !mcp.HasSamplingCapability(ctx) {
	    return fallback(ctx, src) // host won't handle createMessage
	}

# Fallback / no-session behaviour

Sample returns a non-nil error when no session is wired (SetSession was never
called or was cleared). Callers should treat this the same as any other
transient error and degrade to an alternative path (subprocess, heuristic,
etc.). Do NOT retry in a loop — D-45-CAPABILITY-PROBE applies.

# What this package does NOT contain

  - No github.com/anthropics/anthropic-sdk-go dependency.
  - No Claude-specific anything — this is generic MCP sampling (JSON-RPC
    over the go-sdk transport), and will work with any MCP client that
    implements sampling/createMessage.
  - No domain-specific adapters (forensic, frida, classify, enrich) — those
    live in unravel/internal/mcp to avoid import cycles with domain packages.
*/
package mcp
