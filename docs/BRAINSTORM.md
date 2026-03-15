# Brainstorm

## Competitive Landscape

### 1. mcp-browser-agent (imprvhub)
- **What**: Python MCP server giving Claude Desktop autonomous browser control
- **Stack**: Python, Playwright, Claude Desktop integration
- **Tools**: DOM manipulation, JS execution, API requests, screenshot
- **Strength**: Tight Claude Desktop integration, autonomous agent loop
- **Gap vs Scout**: No Go SDK, no gRPC, no stealth/fingerprint, no session persistence, no plugin system, single-browser only

### 2. BrowserMCP (browsermcp.io)
- **What**: Chrome extension + MCP server — connect AI apps to your real browser
- **Stack**: Chrome extension, local MCP server, accessibility snapshots
- **Tools**: Navigate, click, type, screenshot, snapshot, drag & drop, console logs, press key, hover, wait
- **Strength**: Uses your real browser profile (logged-in sessions, real fingerprint), local/private, fast
- **Gap vs Scout**: Extension-only (no headless/CI), no scraper framework, no runbooks, no gRPC, no research presets
- **Idea to steal**: Accessibility snapshot as a first-class MCP tool — lightweight alternative to full screenshot for AI reasoning

### 3. web-scout-mcp (pinkpixel-dev)
- **What**: Node.js MCP server for web search + content extraction via DuckDuckGo
- **Stack**: TypeScript, DuckDuckGo API, Cheerio for extraction
- **Tools**: DuckDuckGoWebSearch, UrlContentExtractor (single + batch)
- **Strength**: Simple, privacy-focused search, parallel URL extraction, rate limiting
- **Gap vs Scout**: No browser automation at all — search + scrape only, no JS rendering, no interaction
- **Idea to steal**: Built-in web search as MCP tool — Scout could add a `search` tool that combines DuckDuckGo/Google results with our browser-rendered extraction

## Feature Ideas

### A. Step-by-Step Guide Generator (from idea.txt)

**Concept**: An MCP tool/command that lets AI generate interactive how-to guides by:
1. Taking user inputs (URL, goal, parameters)
2. Navigating and capturing screenshots at each step
3. Executing API calls (curl, JSON payloads) and capturing responses
4. Combining screenshots + terminal output + AI narration into a final document

**Implementation thoughts**:
- New MCP tool: `guide_create` — orchestrates a multi-step flow
- Each step = navigate + action + screenshot + AI description
- Output formats: Markdown doc with embedded images, HTML, PDF
- Could reuse `Gather()` for page intelligence at each step
- Runbook integration: a runbook could define the steps, guide generator adds the captures/narration layer
- Terminal capture: leverage existing `internal/logger/` KSUID-based capture

**MVP scope**:
- `scout guide record --url <url>` — starts recording mode
- Each navigation/action auto-captures screenshot + DOM snapshot
- `scout guide stop` — ends recording, generates markdown guide
- MCP tool: `guide_start`, `guide_step`, `guide_finish`

### B. Accessibility Snapshot Tool (inspired by BrowserMCP)

- Add `snapshot_accessibility` MCP tool returning the accessibility tree
- Much lighter than screenshots for AI reasoning about page structure
- We already have `Snapshot()` — expose it as a dedicated MCP tool with structured output

### C. Web Search Integration (inspired by web-scout-mcp)

- Add `web_search` MCP tool (DuckDuckGo or SearXNG)
- Combine with our browser-rendered content extraction
- Flow: search → get URLs → navigate + extract with full JS rendering
- Differentiator: other tools do search OR browser — Scout does both

### D. Real Browser Profile Mode (inspired by BrowserMCP)

- We already have `WithRemoteCDP()` — document/promote connecting to user's real browser
- Add `scout connect` CLI command for easy pairing with running Chrome
- Enables logged-in session automation without credential management

## Priority Ranking

| # | Idea | Effort | Impact | Priority |
|---|------|--------|--------|----------|
| A | Guide Generator | High | High — unique differentiator | P2 |
| B | Accessibility Snapshot MCP tool | Low | Medium — quick win | P1 |
| C | Web Search MCP tool | Medium | High — completes the loop | P2 |
| D | Real Browser Profile docs/CLI | Low | Medium — low-hanging fruit | P1 |
