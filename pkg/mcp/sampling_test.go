/*
Copyright (c) 2026 Security Research
*/
package mcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// newStubHost wires an in-memory transport pair, attaches a CreateMessageHandler
// on the host (client) side, and returns the *ServerSession bound to that host.
// Reference: go-sdk mcp/client_example_test.go Example_sampling.
func newStubHost(t *testing.T, handler func(context.Context, *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error)) *gomcp.ServerSession {
	t.Helper()
	ctx := context.Background()
	ct, st := gomcp.NewInMemoryTransports()

	host := gomcp.NewClient(&gomcp.Implementation{Name: "test-host", Version: "0.0.0"}, &gomcp.ClientOptions{
		CreateMessageHandler: handler,
	})
	srv := gomcp.NewServer(&gomcp.Implementation{Name: "unravel-test", Version: "0.0.0"}, nil)

	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := host.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("host connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Close()
	})
	return ss
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// resetSession clears the package singleton; defer this in every test that
// calls SetSession(non-nil) so other tests start clean.
func resetSession(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetSession(nil, quietLogger()) })
}

func TestSample_HappyPath(t *testing.T) {
	resetSession(t)
	ss := newStubHost(t, func(_ context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		return &gomcp.CreateMessageResult{
			Content: &gomcp.TextContent{Text: "hi"},
			Model:   "test-model",
			Role:    "assistant",
		}, nil
	})
	SetSession(ss, quietLogger())

	body, err := Sample(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(body) != "hi" {
		t.Fatalf("body=%q want %q", body, "hi")
	}
}

func TestSample_NoSession(t *testing.T) {
	SetSession(nil, quietLogger())
	body, err := Sample(context.Background(), "x")
	if err == nil {
		t.Fatal("expected err on no session")
	}
	if !errors.Is(err, errNoSession) {
		t.Fatalf("want errNoSession, got %v", err)
	}
	if body != nil {
		t.Fatalf("want nil body, got %q", body)
	}
}

func TestSample_NonTextContent(t *testing.T) {
	resetSession(t)
	ss := newStubHost(t, func(_ context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		return &gomcp.CreateMessageResult{
			Content: &gomcp.ImageContent{Data: []byte("x"), MIMEType: "image/png"},
			Model:   "test-model",
			Role:    "assistant",
		}, nil
	})
	SetSession(ss, quietLogger())

	body, err := Sample(context.Background(), "p")
	if err == nil {
		t.Fatal("expected err on non-text content")
	}
	if !errors.Is(err, errNonText) {
		t.Fatalf("want errNonText, got %v", err)
	}
	if body != nil {
		t.Fatalf("want nil body, got %q", body)
	}
}

func TestSample_HostError(t *testing.T) {
	resetSession(t)
	ss := newStubHost(t, func(_ context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		return nil, errors.New("denied")
	})
	SetSession(ss, quietLogger())

	body, err := Sample(context.Background(), "p")
	if err == nil {
		t.Fatal("expected err from host")
	}
	if body != nil {
		t.Fatalf("want nil body, got %q", body)
	}
}

func TestSample_Timeout(t *testing.T) {
	resetSession(t)
	// Stub host blocks until its context is done. samplingTimeout (30s) is
	// longer than we want a real test to run, so we override the parent ctx
	// with a short deadline that exercises the same path: ctx cancellation
	// inside Sample() bubbles up as deadline-exceeded.
	ss := newStubHost(t, func(ctx context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	SetSession(ss, quietLogger())

	parent, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	body, err := Sample(parent, "p")
	if err == nil {
		t.Fatal("expected timeout err")
	}
	if body != nil {
		t.Fatalf("want nil body, got %q", body)
	}
}

func TestSample_Ceilings(t *testing.T) {
	resetSession(t)
	var gotMaxTokens atomic.Int64
	var mu sync.Mutex
	var gotRole, gotText string

	ss := newStubHost(t, func(_ context.Context, req *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		gotMaxTokens.Store(req.Params.MaxTokens)
		if len(req.Params.Messages) == 1 {
			mu.Lock()
			gotRole = string(req.Params.Messages[0].Role)
			if tc, ok := req.Params.Messages[0].Content.(*gomcp.TextContent); ok {
				gotText = tc.Text
			}
			mu.Unlock()
		}
		return &gomcp.CreateMessageResult{
			Content: &gomcp.TextContent{Text: "ok"},
			Model:   "test-model",
			Role:    "assistant",
		}, nil
	})
	SetSession(ss, quietLogger())

	if _, err := Sample(context.Background(), "probe-prompt"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := gotMaxTokens.Load(); got != 2000 {
		t.Fatalf("MaxTokens=%d want 2000", got)
	}
	mu.Lock()
	role, text := gotRole, gotText
	mu.Unlock()
	if role != "user" {
		t.Fatalf("Role=%q want user", role)
	}
	if text != "probe-prompt" {
		t.Fatalf("Text=%q want probe-prompt", text)
	}
}

func TestSample_HostDisconnect(t *testing.T) {
	resetSession(t)
	ss := newStubHost(t, func(_ context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		return &gomcp.CreateMessageResult{Content: &gomcp.TextContent{Text: "ok"}, Model: "m", Role: "assistant"}, nil
	})
	SetSession(ss, quietLogger())
	// Close the session before invoking Sample.
	if err := ss.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	body, err := Sample(context.Background(), "p")
	if err == nil {
		t.Fatal("expected err on disconnected session")
	}
	if body != nil {
		t.Fatalf("want nil body, got %q", body)
	}
}
