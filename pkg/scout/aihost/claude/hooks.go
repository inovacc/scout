package claude

import (
	"encoding/json"
	"fmt"
)

// hookBindings map each Claude Code lifecycle-hook event to the `scout hook <sub>`
// subcommand (cmd/scout/hook.go) that handles it. Pre/PostToolUse are scoped by
// matcher to Scout's own MCP tools so they never fire on unrelated tool calls; the
// session/turn hooks fire unconditionally. The command interpolates McpCommand — it
// NEVER hard-codes "scout" — so a renamed/relocated binary stays consistent with the
// MCP registration (.mcp.json). Phase 2 of the lifecycle-governance plugin.
var hookBindings = []struct {
	event   string // Claude Code hook event
	sub     string // scout hook subcommand
	matcher string // tool matcher ("" = all tools); scopes Pre/PostToolUse to scout's MCP tools
}{
	{"SessionStart", "session-start", ""},
	{"UserPromptSubmit", "prompt-submit", ""},
	{"PreToolUse", "pre-tool", "mcp__scout__.*"},
	{"PostToolUse", "post-tool", "mcp__scout__.*"},
	{"Stop", "stop", ""},
	{"SessionEnd", "session-end", ""},
}

// HooksJSON returns the bytes written to hooks/hooks.json — the plugin's Claude Code
// lifecycle-hook manifest. Emitted via GeneratedFiles(), so it lands OUTSIDE the
// commands/agents/skills stale-sweep and survives re-install.
func HooksJSON() ([]byte, error) {
	hooks := make(map[string]any, len(hookBindings))

	for _, b := range hookBindings {
		entry := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": McpCommand + " hook " + b.sub},
			},
		}
		if b.matcher != "" {
			entry["matcher"] = b.matcher
		}
		hooks[b.event] = []any{entry}
	}

	out, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal hooks.json: %w", err)
	}

	return append(out, '\n'), nil
}

// Hooks implements the optional aihost.HookProvider capability: the hook-manifest
// payloads keyed by tree-relative path. Introspected by doctor and used for per-host
// portability decisions; the tree write itself goes through GeneratedFiles().
func (Host) Hooks() (map[string][]byte, error) {
	hj, err := HooksJSON()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{"hooks/hooks.json": hj}, nil
}
