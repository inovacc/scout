package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestParseHookInput(t *testing.T) {
	in := parseHookInput([]byte(`{"hook_event_name":"SessionStart","session_id":"abc","source":"startup"}`))
	if in.HookEventName != "SessionStart" || in.SessionID != "abc" || in.Source != "startup" {
		t.Fatalf("parse mismatch: %+v", in)
	}

	// Malformed / empty input must yield the zero value, never panic (fail-open).
	for _, bad := range [][]byte{nil, {}, []byte("not json"), []byte("{")} {
		if got := parseHookInput(bad); got.HookEventName != "" || got.SessionID != "" {
			t.Errorf("parseHookInput(%q) = %+v, want zero value", bad, got)
		}
	}
}

func TestHookURLArg(t *testing.T) {
	if got := hookURLArg(json.RawMessage(`{"url":"https://example.com/x"}`)); got != "https://example.com/x" {
		t.Errorf("hookURLArg = %q", got)
	}
	if got := hookURLArg(json.RawMessage(`{"selector":"#a"}`)); got != "" {
		t.Errorf("hookURLArg (no url) = %q, want empty", got)
	}
	if got := hookURLArg(nil); got != "" {
		t.Errorf("hookURLArg(nil) = %q, want empty", got)
	}
}

func TestBrowserDied(t *testing.T) {
	for _, s := range []string{`"EOF"`, `{"error":"connection refused"}`, `"websocket: close 1006"`, `"context deadline exceeded"`} {
		if !browserDied(json.RawMessage(s)) {
			t.Errorf("browserDied(%s) = false, want true", s)
		}
	}
	for _, s := range []string{`"ok"`, `{"result":"navigated"}`, ``} {
		if browserDied(json.RawMessage(s)) {
			t.Errorf("browserDied(%s) = true, want false", s)
		}
	}
}

func TestPreToolDeny(t *testing.T) {
	// Default (no env) → always allow (advisory).
	t.Setenv("SCOUT_DENY_TOOLS", "")
	t.Setenv("SCOUT_ALLOW_TARGETS", "")
	if r := preToolDeny(hookInput{ToolName: "mcp__scout__eval"}); r != "" {
		t.Errorf("advisory default should allow, got deny: %q", r)
	}

	// Denylist matches the tool.
	t.Setenv("SCOUT_DENY_TOOLS", "mcp__scout__eval,mcp__scout__open")
	if r := preToolDeny(hookInput{ToolName: "mcp__scout__eval"}); r == "" {
		t.Error("SCOUT_DENY_TOOLS should deny mcp__scout__eval")
	}
	if r := preToolDeny(hookInput{ToolName: "mcp__scout__navigate"}); r != "" {
		t.Errorf("SCOUT_DENY_TOOLS should not deny navigate, got: %q", r)
	}

	// Allowlist: off-list host denied; on-list allowed.
	t.Setenv("SCOUT_DENY_TOOLS", "")
	t.Setenv("SCOUT_ALLOW_TARGETS", "example.com,localhost")
	if r := preToolDeny(hookInput{ToolName: "mcp__scout__navigate", ToolInput: json.RawMessage(`{"url":"https://evil.test/"}`)}); r == "" {
		t.Error("off-allowlist URL should be denied")
	}
	if r := preToolDeny(hookInput{ToolName: "mcp__scout__navigate", ToolInput: json.RawMessage(`{"url":"https://example.com/ok"}`)}); r != "" {
		t.Errorf("on-allowlist URL should be allowed, got: %q", r)
	}
}

// TestHookPreToolEndToEnd exercises the pre-tool subcommand's stdin→stdout wiring: with a
// denylist set, a matching tool yields a permissionDecision:deny JSON on stdout.
func TestHookPreToolEndToEnd(t *testing.T) {
	t.Setenv("SCOUT_DENY_TOOLS", "mcp__scout__open")

	out := captureHookStdout(t,
		`{"hook_event_name":"PreToolUse","tool_name":"mcp__scout__open","tool_input":{"url":"http://x"}}`,
		func() { hookPreToolCmd.Run(hookPreToolCmd, nil) })

	var resp struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("output not valid JSON: %q (%v)", out, err)
	}
	if resp.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %q (out=%q)", resp.HookSpecificOutput.PermissionDecision, out)
	}
}

// TestHookPreToolAllowsByDefault: no env policy → allow → NO output.
func TestHookPreToolAllowsByDefault(t *testing.T) {
	t.Setenv("SCOUT_DENY_TOOLS", "")
	t.Setenv("SCOUT_ALLOW_TARGETS", "")

	out := captureHookStdout(t,
		`{"hook_event_name":"PreToolUse","tool_name":"mcp__scout__eval","tool_input":{}}`,
		func() { hookPreToolCmd.Run(hookPreToolCmd, nil) })

	if out != "" {
		t.Errorf("advisory default should emit no output (allow), got %q", out)
	}
}

func TestHookSubcommandsRegistered(t *testing.T) {
	want := map[string]bool{
		"session-start": false, "prompt-submit": false, "pre-tool": false,
		"post-tool": false, "stop": false, "session-end": false,
	}
	for _, c := range hookCmd.Commands() {
		delete(want, c.Name())
	}
	if len(want) != 0 {
		t.Errorf("missing hook subcommands: %v", want)
	}

	// The historical `scout hook sessionstart` (setup.go) must resolve via the alias.
	c, _, err := hookCmd.Find([]string{"sessionstart"})
	if err != nil || c.Name() != "session-start" {
		t.Errorf("`hook sessionstart` should resolve to session-start via alias (got %v, err=%v)", c.Name(), err)
	}
}

// captureHookStdout runs fn with the given stdin payload and returns its captured stdout.
func captureHookStdout(t *testing.T, stdin string, fn func()) string {
	t.Helper()

	oldIn, oldOut := os.Stdin, os.Stdout
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = inR
	go func() { _, _ = inW.WriteString(stdin); _ = inW.Close() }()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = outW

	fn()
	_ = outW.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(outR)
	_ = outR.Close()

	return buf.String()
}
