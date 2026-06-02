/*
Copyright (c) 2026 Security Research
*/
package mcp

import (
	"context"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// newStubHostWithCaps wires an in-memory transport pair where the host
// advertises (or omits) the sampling capability via ClientOptions.
func newStubHostWithCaps(t *testing.T, caps *gomcp.ClientCapabilities, withHandler bool) *gomcp.ServerSession {
	t.Helper()
	ctx := context.Background()
	ct, st := gomcp.NewInMemoryTransports()

	opts := &gomcp.ClientOptions{Capabilities: caps}
	if withHandler {
		opts.CreateMessageHandler = func(_ context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
			return &gomcp.CreateMessageResult{
				Content: &gomcp.TextContent{Text: "ok"},
				Model:   "test-model",
				Role:    "assistant",
			}, nil
		}
	}
	host := gomcp.NewClient(&gomcp.Implementation{Name: "test-host", Version: "0.0.0"}, opts)
	srv := gomcp.NewServer(&gomcp.Implementation{Name: "unravel-test", Version: "0.0.0"}, nil)

	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := host.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("host connect: %v", err)
	}
	// host.Connect completed synchronously above, which implies the initialize
	// handshake has been processed and ServerSession.InitializeParams is set.
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Close()
	})
	return ss
}

func TestHasSamplingCapability_NoSession(t *testing.T) {
	SetSession(nil, quietLogger())
	if HasSamplingCapability(context.Background()) {
		t.Fatal("want false when session is nil")
	}
}

func TestHasSamplingCapability_NoSampling(t *testing.T) {
	resetSession(t)
	// Caps without sampling: client has no CreateMessageHandler and supplies
	// an explicit empty capabilities block so sampling is NOT auto-injected.
	ss := newStubHostWithCaps(t, &gomcp.ClientCapabilities{}, false)
	SetSession(ss, quietLogger())
	if HasSamplingCapability(context.Background()) {
		t.Fatal("want false when host has no sampling capability")
	}
}

func TestHasSamplingCapability_WithSampling(t *testing.T) {
	resetSession(t)
	// Auto-injected via CreateMessageHandler (per go-sdk docs).
	ss := newStubHostWithCaps(t, nil, true)
	SetSession(ss, quietLogger())
	if !HasSamplingCapability(context.Background()) {
		t.Fatal("want true when host has sampling capability")
	}
}

// TestHasSamplingCapability_AfterClearSession asserts that SetSession(nil)
// causes the probe to revert to false even if a prior session advertised
// sampling. Locks D-45-CAPABILITY-PROBE — clearing the session MUST clear
// the cached affirmative answer.
func TestHasSamplingCapability_AfterClearSession(t *testing.T) {
	resetSession(t)
	ss := newStubHostWithCaps(t, nil, true)
	SetSession(ss, quietLogger())
	if !HasSamplingCapability(context.Background()) {
		t.Fatal("precondition: expected true after wiring sampling host")
	}
	SetSession(nil, quietLogger())
	if HasSamplingCapability(context.Background()) {
		t.Fatal("want false after SetSession(nil) clears the singleton")
	}
}
