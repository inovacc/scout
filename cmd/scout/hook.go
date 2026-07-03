package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

// hookCmd is the direct-binary target for Claude Code lifecycle hooks, invoked by the
// plugin's hooks.json (NO shell scripts). Each subcommand reads a CC hook JSON payload
// on stdin, composes an existing scout lifecycle verb, and writes a hook JSON response
// on stdout.
//
// Contract: hooks fire on every turn, so they are FAST and FAIL-OPEN — every subcommand
// exits 0 and never blocks Claude, even on a parse error, unknown event, or engine error.
// The subcommands are thin compositions of existing engine verbs; they add no new state.
var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Claude Code lifecycle hook handlers (invoked by the plugin's hooks.json)",
	Hidden: true,
}

// hookInput is the subset of the Claude Code hook payload the scout hooks read. Unknown
// events or malformed JSON unmarshal to the zero value, which every handler treats as a
// safe no-op.
type hookInput struct {
	HookEventName string          `json:"hook_event_name"`
	SessionID     string          `json:"session_id"`
	Source        string          `json:"source"`        // SessionStart: startup|resume|clear
	Reason        string          `json:"reason"`        // SessionEnd
	ToolName      string          `json:"tool_name"`     // Pre/PostToolUse
	ToolInput     json.RawMessage `json:"tool_input"`    // Pre/PostToolUse
	ToolResponse  json.RawMessage `json:"tool_response"` // PostToolUse
}

// readHookInput reads and parses the hook payload from stdin, bounded to 1 MiB. Any
// read/parse failure yields the zero value (safe no-op) — hooks never fail closed.
func readHookInput() hookInput {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return hookInput{}
	}

	return parseHookInput(data)
}

// parseHookInput parses hook payload bytes; malformed/empty input yields the zero value.
func parseHookInput(data []byte) hookInput {
	var in hookInput
	if len(data) > 0 {
		_ = json.Unmarshal(data, &in)
	}

	return in
}

// emitContext writes a hookSpecificOutput response that injects additionalContext into
// the Claude Code session (used by SessionStart / UserPromptSubmit). Empty context = no output.
func emitContext(event, ctx string) {
	if ctx == "" {
		return
	}

	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": ctx,
		},
	})
}

// liveSessionCount returns the number of tracked sessions (best-effort; 0 on error).
func liveSessionCount() int {
	sessions, err := scout.ListSessions()
	if err != nil {
		return 0
	}

	return len(sessions)
}

func init() {
	rootCmd.AddCommand(hookCmd)
	hookCmd.AddCommand(
		hookSessionStartCmd,
		hookPromptSubmitCmd,
		hookPreToolCmd,
		hookPostToolCmd,
		hookStopCmd,
		hookSessionEndCmd,
	)
}

// SessionStart — pre-flight reap of browsers/dirs leaked by a prior session, then inject
// a clean baseline into context. Composes CleanStaleSessions (respects the Reusable flag).
var hookSessionStartCmd = &cobra.Command{
	Use:     "session-start",
	Aliases: []string{"sessionstart"}, // back-compat with the historical `scout setup` emission
	Short:   "SessionStart hook: reap stale sessions, inject a clean baseline",
	Run: func(_ *cobra.Command, _ []string) {
		_ = readHookInput()

		reaped, _ := scout.CleanStaleSessions()

		emitContext("SessionStart", fmt.Sprintf(
			"Scout lifecycle: pre-flight reap cleared %d stale session(s); %d live session(s) now. "+
				"Browser access is same-machine-only via mcp__scout__* tools or the `scout` CLI; "+
				"never touch the user's real browser without --system-browser.",
			reaped, liveSessionCount()))
	},
}

// UserPromptSubmit — inject current session state so Claude knows what's already open.
var hookPromptSubmitCmd = &cobra.Command{
	Use:   "prompt-submit",
	Short: "UserPromptSubmit hook: inject live session state into context",
	Run: func(_ *cobra.Command, _ []string) {
		_ = readHookInput()

		if n := liveSessionCount(); n > 0 {
			emitContext("UserPromptSubmit", fmt.Sprintf("Scout: %d browser session(s) currently open.", n))
		}
	},
}

// PreToolUse — advisory guardrail for mcp__scout__* (and Bash `scout`) tools. ADVISORY by
// default: it only hard-denies when the opt-in env allowlist/denylist is set, so it never
// false-positives on legitimate automation.
//
//	SCOUT_DENY_TOOLS   — comma-separated tool names to deny (e.g. "mcp__scout__eval,mcp__scout__open")
//	SCOUT_ALLOW_TARGETS — comma-separated host substrings; navigate/open to a URL matching NONE is denied
var hookPreToolCmd = &cobra.Command{
	Use:   "pre-tool",
	Short: "PreToolUse hook: advisory guardrail for scout tools (opt-in hard-deny via env)",
	Run: func(_ *cobra.Command, _ []string) {
		in := readHookInput()

		if reason := preToolDeny(in); reason != "" {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":            "PreToolUse",
					"permissionDecision":       "deny",
					"permissionDecisionReason": reason,
				},
			})
		}
		// No output = allow (advisory default).
	},
}

// preToolDeny returns a non-empty deny reason only when an opt-in env policy matches.
func preToolDeny(in hookInput) string {
	if deny := os.Getenv("SCOUT_DENY_TOOLS"); deny != "" {
		for _, t := range strings.Split(deny, ",") {
			if strings.TrimSpace(t) != "" && strings.EqualFold(strings.TrimSpace(t), in.ToolName) {
				return fmt.Sprintf("scout policy: tool %q is denied by SCOUT_DENY_TOOLS", in.ToolName)
			}
		}
	}

	if allow := os.Getenv("SCOUT_ALLOW_TARGETS"); allow != "" {
		if url := hookURLArg(in.ToolInput); url != "" {
			ok := false
			for _, host := range strings.Split(allow, ",") {
				if h := strings.TrimSpace(host); h != "" && strings.Contains(url, h) {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Sprintf("scout policy: %q not in SCOUT_ALLOW_TARGETS allowlist", url)
			}
		}
	}

	return ""
}

// hookURLArg extracts a "url" field from a tool_input JSON object, if present.
func hookURLArg(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var m struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(raw, &m)

	return m.URL
}

// PostToolUse — if a tool reported the browser died (EOF / connection lost — the plan-005
// liveness gap), reap so the dead session dir + chrome are reclaimed before the next call.
var hookPostToolCmd = &cobra.Command{
	Use:   "post-tool",
	Short: "PostToolUse hook: reclaim a dead browser session on EOF/connection loss",
	Run: func(_ *cobra.Command, _ []string) {
		in := readHookInput()

		if browserDied(in.ToolResponse) {
			_, _ = scout.CleanStaleSessions()
		}
	},
}

// browserDied heuristically detects a lost-browser sentinel in a tool response.
func browserDied(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	s := strings.ToLower(string(raw))

	return strings.Contains(s, "eof") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection lost") ||
		strings.Contains(s, "websocket: close") ||
		strings.Contains(s, "context deadline exceeded")
}

// Stop — end-of-turn reclaim: reap idle/orphaned sessions so a browser doesn't linger
// between turns (a deterministic substitute for scout's 5-min idle timer). Respects Reusable.
var hookStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop hook: end-of-turn reap of idle/orphaned sessions",
	Run: func(_ *cobra.Command, _ []string) {
		_ = readHookInput()
		_, _ = scout.CleanStaleSessions()
	},
}

// SessionEnd — the highest-value hook: the guaranteed teardown scout structurally lacks
// (its own is best-effort SIGINT-tier, and the MCP path has no defer state.reset()).
// Default: reap dead-owner + non-reusable + expired sessions (respects the Reusable flag).
// Opt-in aggressive: SCOUT_SESSION_END_ALL forces a full reset of every session.
var hookSessionEndCmd = &cobra.Command{
	Use:   "session-end",
	Short: "SessionEnd hook: guaranteed teardown (reap; opt-in full reset via SCOUT_SESSION_END_ALL)",
	Run: func(_ *cobra.Command, _ []string) {
		_ = readHookInput()

		if os.Getenv("SCOUT_SESSION_END_ALL") != "" {
			_, _ = scout.ResetAllSessions() // aggressive: also drops reusable sessions
		}

		_, _ = scout.CleanStaleSessions()
	},
}
