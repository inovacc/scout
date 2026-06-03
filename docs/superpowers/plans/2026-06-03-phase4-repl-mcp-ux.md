# Phase 4: REPL & MCP UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the remaining Phase 4 UX work — readline in the REPL, the two missing MCP tools, the truncate cap bug, and description polish — without re-doing what Phases 5/6 already shipped.

**Architecture:** REPL keeps routing browser logic through `pkg/scout/tools/` (Phase 6); only its input loop changes from `bufio.Scanner` to `github.com/peterh/liner` (history + tab-completion + line editing). Two new MCP tools (`reload`, `tabs`) register the already-existing `tools.Reload`/`tools.Tabs` verbs. A shared truncate helper caps output length safely.

**Tech Stack:** Go 1.26, `github.com/peterh/liner` (new dep), `pkg/scout/tools/` verbs, `pkg/scout/mcp` (`addTracedTool`), Cobra REPL.

**Spec:** `docs/superpowers/specs/2026-05-16-04-repl-mcp-ux-design.md` (see 2026-06-03 Amendment). Already DONE (do NOT touch): REPL `html`/`cookies`/`reload`/`tabs`/`markdown` commands; MCP `html`/`cookies`/`markdown` tools.

---

## Task 1: MCP-04 — truncate cap bug

**Files:**
- Modify: `cmd/scout/helpers.go` (`truncate` ~line 162), `pkg/scout/tools/websocket.go` (`truncateWS` ~line 149)
- Test: `cmd/scout/helpers_test.go` (create or append)

- [ ] **Step 1: Write the failing test** (`cmd/scout/helpers_test.go`)

```go
package main

import "testing"

func TestTruncateNeverExceedsMaxAndNeverPanics(t *testing.T) {
	cases := []struct {
		in  string
		max int
	}{
		{"hello world", 5},
		{"hi", 5},
		{"hello", 2},
		{"hello", 0},
		{"hello", 3},
		{"hello", 1},
		{"", 4},
	}
	for _, c := range cases {
		got := truncate(c.in, c.max) // must not panic
		if c.max >= 0 && len(got) > c.max {
			t.Errorf("truncate(%q, %d) = %q (len %d) exceeds max", c.in, c.max, got, len(got))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestTruncateNeverExceeds' ./cmd/scout/`
Expected: FAIL (panic: slice bounds out of range for `max=1`/`max=2`, since `s[:maxLen-3]` goes negative).

- [ ] **Step 3: Fix both functions**

Replace `cmd/scout/helpers.go` `truncate` with:

```go
// truncate shortens s to at most maxLen runes-worth of bytes, never panicking
// and never returning a string longer than maxLen.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
```

Then READ `pkg/scout/tools/websocket.go` `truncateWS` and apply the SAME safe logic (cap at `maxLen`, no panic, append `"..."` only when `maxLen > 3`). If `truncateWS` currently does `s[:maxLen] + "..."` (the "exceeds maxLen" bug the spec flags), this fixes it.

- [ ] **Step 4: Test + commit**

```bash
go test -run 'TestTruncateNeverExceeds' ./cmd/scout/ && go test ./pkg/scout/tools/ && golangci-lint run ./cmd/scout/ ./pkg/scout/tools/
git add cmd/scout/helpers.go cmd/scout/helpers_test.go pkg/scout/tools/websocket.go
git commit -m "fix(ux): truncate/truncateWS cap output at maxLen and never panic"
```

---

## Task 2: MCP-02 remainder — `reload` + `tabs` tools

**Files:** Modify `pkg/scout/mcp/tools_browser.go` (register near the other page tools).

- [ ] **Step 1:** Read how existing zero-arg tools are registered (e.g. the `markdown`/`page_url` twins added in Phase 6, and `back`/`forward`) to copy the exact `addTracedTool` + `state.ensurePage`/`state.ensureBrowser` + result-helper style. Note the JSON result helper used for structured output (likely `textResult` with a JSON-marshaled string, or a `jsonResult` helper — use whatever the Tabs-like tools use).

- [ ] **Step 2:** Add the two tools:

```go
	addTracedTool(server, &mcp.Tool{
		Name:        "reload",
		Description: "Reload the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		if _, err := tools.Reload(ctx, page, tools.ReloadInput{}); err != nil {
			return errResult(err.Error())
		}
		return textResult("reloaded")
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "tabs",
		Description: "List open browser tabs (index, URL, title)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		b, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		res, err := tools.Tabs(ctx, b, tools.TabsInput{})
		if err != nil {
			return errResult(err.Error())
		}
		data, err := json.Marshal(res.Tabs)
		if err != nil {
			return errResult(err.Error())
		}
		return textResult(string(data))
	})
```

- [ ] **Step 3:** Update the parity manifest `pkg/scout/tools/parity_test.go`: confirm `Reload` and `Tabs` rows now have `InMCP: true` (they were `InREPL: true, InMCP: true` already if listed — verify; Tabs was `InREPL true, InMCP true`? It became MCP-exposed only now, so set `InMCP: true`). Keep `TestVerbParity` passing.

- [ ] **Step 4: Verify + commit**

```bash
go build ./pkg/scout/mcp/ && go vet ./pkg/scout/mcp/ && go test -run 'TestVerbParity|TestCheckURL' ./pkg/scout/tools/ ./pkg/scout/mcp/ && golangci-lint run ./pkg/scout/mcp/
git add pkg/scout/mcp/tools_browser.go pkg/scout/tools/parity_test.go
git commit -m "feat(mcp): add reload + tabs tools (route to tools.Reload/Tabs)"
```

---

## Task 3: REPL-01 — readline via peterh/liner

**Files:** Modify `cmd/scout/repl.go`.

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/peterh/liner@latest` then `go mod tidy`. Confirm it appears in `go.mod`.

- [ ] **Step 2: Replace the input loop**

In `cmd/scout/repl.go` `RunE`, READ the current loop (it uses `bufio.NewScanner(os.Stdin)` + `for { ... scanner.Scan() ... }`). The command `switch` BODY stays exactly as-is (already routed through `tools.*`). Replace ONLY the scanner setup + the read mechanism:

```go
	// (near the top of RunE, after `out := cmd.OutOrStdout()`)
	rl := liner.NewLiner()
	defer func() { _ = rl.Close() }()
	rl.SetCtrlCAborts(true)

	replCommands := []string{
		"navigate", "go", "nav", "eval", "click", "type", "extract", "screenshot",
		"markdown", "md", "html", "cookies", "url", "title", "wait", "back", "forward",
		"reload", "tabs", "tab", "newtab", "health", "help", "exit", "quit",
	}
	rl.SetCompleter(func(in string) []string {
		var c []string
		low := strings.ToLower(in)
		for _, cmd := range replCommands {
			if strings.HasPrefix(cmd, low) {
				c = append(c, cmd)
			}
		}
		return c
	})

	// Best-effort persistent history.
	histPath := replHistoryPath()
	if histPath != "" {
		if f, err := os.Open(histPath); err == nil {
			_, _ = rl.ReadHistory(f)
			_ = f.Close()
		}
		defer func() {
			if f, err := os.Create(histPath); err == nil {
				_, _ = rl.WriteHistory(f)
				_ = f.Close()
			}
		}()
	}

	for {
		prompt := "> "
		if page != nil {
			if u, err := page.URL(); err == nil {
				if parsed, err := url.Parse(u); err == nil {
					prompt = fmt.Sprintf("[%s%s] > ", parsed.Host, parsed.Path)
				}
			}
		}

		input, err := rl.Prompt(prompt)
		if errors.Is(err, liner.ErrPromptAborted) || errors.Is(err, io.EOF) {
			_, _ = fmt.Fprintln(out, "Bye!")
			return nil
		}
		if err != nil {
			return fmt.Errorf("scout: repl: read line: %w", err)
		}

		line := strings.TrimSpace(input)
		if line == "" {
			continue
		}
		rl.AppendHistory(line)

		parts := strings.SplitN(line, " ", 3)
		c := parts[0]

		switch c {
		// ... EXISTING cases unchanged ...
		}
	}
```

Add a helper at the end of `repl.go`:

```go
// replHistoryPath returns the REPL history file path, or "" if the home dir is
// unavailable (history is then in-session only).
func replHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".scout_repl_history")
}
```

- [ ] **Step 3: Fix imports**

Add `"github.com/peterh/liner"`, `"errors"`, `"path/filepath"`. REMOVE `"bufio"` (no longer used). Keep `io`, `net/url`, `os`, `strings`, `encoding/json`, `fmt`, `scout`, `cobra`, `tools` (verify each is still referenced; run `goimports`/`gofmt`).

- [ ] **Step 4: Verify behavior**

```bash
go build ./cmd/scout/ && go vet ./cmd/scout/
# Non-interactive piped input still works (liner reads from a non-tty as plain lines):
printf 'navigate https://example.com\ntitle\nexit\n' | go run ./cmd/scout/ repl
```
Expected: navigates, prints the title, exits with "Bye!". (liner degrades to line-reading on a non-tty pipe; interactive history/completion only apply on a real terminal.)
`golangci-lint run ./cmd/scout/` → no NEW issues.

- [ ] **Step 5: Commit**

```bash
go mod tidy
git add cmd/scout/repl.go go.mod go.sum
git commit -m "feat(repl): readline via peterh/liner (history, tab-completion, line editing)"
```

---

## Task 4: REPL-04 + MCP-03 — help + description polish

**Files:** Modify `cmd/scout/repl.go` (`printREPLHelp`), `pkg/scout/mcp/tools_browser.go` (a few tool descriptions).

- [ ] **Step 1: REPL help** — update `printREPLHelp` to add `reload` and `tabs` (if missing from the list) and a short "Tip: use ↑/↓ for history, Tab to complete commands." line. Keep it concise.

- [ ] **Step 2: MCP descriptions** — improve the THINNEST tool descriptions for unambiguous LLM tool-selection. Concretely, change these (only if currently terse):
  - `navigate`: "Navigate the browser to a URL and wait for the page to load."
  - `click`: "Click the first element matching a CSS selector on the current page."
  - `type`: "Type text into the first element matching a CSS selector."
  - `extract`: "Extract the visible text of the first element matching a CSS selector."
  - `eval`: "Evaluate a JavaScript expression in the page and return its result. The expression should be a function, e.g. `() => document.title`."
  - `reload`/`tabs`: as set in Task 2.
  Keep each `InputSchema` unchanged (wire contract stable).

- [ ] **Step 3: Verify + commit**

```bash
go build ./cmd/scout/ ./pkg/scout/mcp/ && golangci-lint run ./cmd/scout/ ./pkg/scout/mcp/
git add cmd/scout/repl.go pkg/scout/mcp/tools_browser.go
git commit -m "docs(ux): clearer REPL help + MCP tool descriptions"
```

---

## Final review (whole feature)

After all tasks: `go build ./cmd/... ./pkg/... && go test ./cmd/scout/ ./pkg/scout/tools/ ./pkg/scout/mcp/`. Confirm the amended success criteria:
- REPL has readline (history/completion/editing) on a real terminal; piped input still works.
- MCP exposes `reload` + `tabs` (plus the Phase-6 `html`/`markdown`/`cookies`).
- `truncate`/`truncateWS` never exceed `maxLen` and never panic.
Then `superpowers:finishing-a-development-branch` to merge `feat/phase4-repl-mcp-ux` into `main`.

**Note:** Tasks 1, 2, 4 are small/mechanical (good fast subagents). Task 3 (liner) is the substantive one — verify piped/non-tty input still works (it's how MCP-less scripts and tests drive the REPL).
