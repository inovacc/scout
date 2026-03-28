# Scout -- Browser Automation for Claude Code

Scout gives Claude Code full browser automation capabilities through 18 MCP tools, 6 skills, and 3 specialized agents.

## What Scout Does

- **Navigate and interact** with any website (click, type, scroll, wait)
- **Extract data** from pages using CSS selectors or JavaScript
- **Take screenshots** and generate PDFs
- **Crawl websites** to discover pages and broken links
- **Monitor sites** for visual changes
- **Automate forms** and multi-step workflows

## Skills

| Skill | Description |
|-------|-------------|
| `/scout:scrape` | Extract structured data from any URL |
| `/scout:screenshot` | Capture page screenshots |
| `/scout:test-site` | Health-check a website for errors |
| `/scout:gather` | One-shot page intelligence collection |
| `/scout:crawl` | Discover and map site structure |
| `/scout:monitor` | Visual regression detection |

## Agents

| Agent | Specialization |
|-------|---------------|
| **web-scraper** | Data extraction with pagination and dynamic content handling |
| **site-tester** | QA testing, broken links, console errors, accessibility |
| **browser-automation** | Multi-step workflows, login flows, form filling |

## MCP Tools (18)

Scout exposes a full browser automation toolkit over MCP:

| Tool | Purpose |
|------|---------|
| `navigate` | Go to a URL |
| `click` | Click an element by CSS selector |
| `type` | Type text into an input field |
| `extract` | Extract content using CSS selectors |
| `eval` | Run arbitrary JavaScript on the page |
| `back` / `forward` | Browser history navigation |
| `wait` | Wait for an element or condition |
| `screenshot` | Capture a PNG screenshot |
| `snapshot` | Full DOM snapshot |
| `pdf` | Generate a PDF of the page |
| `session_list` | List active browser sessions |
| `session_reset` | Reset a browser session |
| `open` | Open a URL in a visible browser |
| `swarm_crawl` | Distributed multi-page crawl |
| `ws_listen` | Monitor WebSocket traffic |
| `ws_send` | Send WebSocket messages |
| `ws_connections` | List active WebSocket connections |

## How It Works

Scout runs a headless Chrome browser controlled via the Chrome DevTools Protocol. When Claude Code invokes a Scout tool or skill, the MCP server translates the request into browser actions and returns structured results.

All browser sessions include stealth mode by default, making Scout effective for sites with bot detection. Sessions are automatically cleaned up when Claude Code exits.

## Installation

Scout auto-downloads the correct binary for your platform on first use. No Go toolchain required.

Alternatively:

```
npm install -g @inovacc/scout-browser
```

## Requirements

- Chrome, Chromium, Brave, or Edge browser (auto-downloaded if not present)
- macOS, Linux, or Windows (amd64 and arm64)

## Browser Support

| Browser | Status |
|---------|--------|
| Chrome / Chrome for Testing | Full support (default, auto-downloaded) |
| Chromium | Full support |
| Brave | Full support |
| Microsoft Edge | Full support |

## Privacy and Security

- Scout runs entirely on your local machine. No data is sent to external servers.
- Browser sessions are sandboxed in `~/.scout/sessions/` and cleaned up automatically.
- No cookies, credentials, or browsing data persist between sessions unless explicitly configured.

## Links

- [GitHub](https://github.com/inovacc/scout)
- [API Reference](https://github.com/inovacc/scout/blob/main/docs/API.md)
- [Examples](https://github.com/inovacc/scout/tree/main/examples)
- [Architecture](https://github.com/inovacc/scout/blob/main/docs/ARCHITECTURE.md)
