# @inovacc/scout-browser

Browser automation, web scraping, and site testing via Scout's headless Chrome engine.

## Install

```bash
npm install -g @inovacc/scout-browser
```

This downloads the correct `scout` binary for your platform from GitHub Releases.

## Usage

```bash
# Start MCP server (for Claude Code / AI agent integration)
scout mcp

# Take a screenshot
scout screenshot https://example.com

# Health-check a website
scout test-site https://example.com

# Start agent HTTP server
scout agent serve

# Interactive browser REPL
scout repl https://example.com
```

## Claude Code Plugin

Scout is also available as a Claude Code plugin:

```bash
claude plugin install scout
```

## Supported Platforms

| OS      | Architecture |
|---------|-------------|
| macOS   | x64, arm64  |
| Linux   | x64, arm64  |
| Windows | x64, arm64  |

## License

BSD-3-Clause
