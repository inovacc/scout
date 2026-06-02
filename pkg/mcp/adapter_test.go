/*
Copyright (c) 2026 Security Research
*/
package mcp

import (
	"context"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAdapter_Sample_HappyPath(t *testing.T) {
	resetSession(t)
	ss := newStubHost(t, func(_ context.Context, _ *gomcp.CreateMessageRequest) (*gomcp.CreateMessageResult, error) {
		return &gomcp.CreateMessageResult{
			Content: &gomcp.TextContent{Text: "hello from adapter"},
			Model:   "test-model",
			Role:    "assistant",
		}, nil
	})
	SetSession(ss, quietLogger())

	a := NewAdapter("test-subsystem")
	body, err := a.Sample(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(body) != "hello from adapter" {
		t.Fatalf("body=%q want %q", body, "hello from adapter")
	}
}

func TestAdapter_Sample_NoSession(t *testing.T) {
	SetSession(nil, quietLogger())
	a := NewAdapter("test-subsystem")
	body, err := a.Sample(context.Background(), "x")
	if err == nil {
		t.Fatal("expected err when no session wired")
	}
	if body != nil {
		t.Fatalf("want nil body, got %q", body)
	}
}

func TestNewAdapter_Name(t *testing.T) {
	a := NewAdapter("my-service")
	if a.name != "my-service" {
		t.Fatalf("name=%q want %q", a.name, "my-service")
	}
}
