# Marketplace Submission Checklist

## Pre-submission

- [x] plugin.json has name, version, description, author, repository, license, keywords
- [x] .mcp.json valid with scout server config
- [x] 6 skills with SKILL.md and description frontmatter
- [x] 3 agents with description and model frontmatter
- [x] hooks/hooks.json with SessionStart binary check
- [x] scripts/check-scout.sh auto-downloads binary
- [x] Plugin validation passes: `task plugin:validate`
- [x] CI green: Plugin Validate workflow
- [x] v1.0.1 release with binaries for all 6 platforms
- [x] npm package ready: @inovacc/scout-browser

## Submission Info

- **Plugin name:** scout
- **Repository:** https://github.com/inovacc/scout
- **Version:** 1.0.1
- **Author:** inovacc
- **License:** BSD-3-Clause
- **Category:** Developer Tools / Browser Automation
- **Tags:** browser, automation, scraping, testing, chrome, mcp, headless

## Plugin Manifest Summary

| Field | Value |
|-------|-------|
| Name | scout |
| Display Name | Scout - Browser Automation |
| Description | Browser automation for Claude Code with 18 MCP tools, 6 skills, and 3 agents |
| MCP Server | `scout mcp --headless --stealth` |
| Skills | scrape, screenshot, test-site, gather, crawl, monitor |
| Agents | web-scraper, site-tester, browser-automation |
| Hook | SessionStart (binary auto-download) |

## Platform Support

| OS | Architecture | Binary |
|----|-------------|--------|
| macOS | amd64 | scout-darwin-amd64 |
| macOS | arm64 | scout-darwin-arm64 |
| Linux | amd64 | scout-linux-amd64 |
| Linux | arm64 | scout-linux-arm64 |
| Windows | amd64 | scout-windows-amd64.exe |
| Windows | arm64 | scout-windows-arm64.exe |

## Submission URLs

- **Claude.ai:** https://claude.ai/settings/plugins/submit
- **Console:** https://platform.claude.com/plugins/submit

## Post-submission

- [ ] Verify listing appears in marketplace search
- [ ] Test install from marketplace on clean machine
- [ ] Confirm SessionStart hook downloads binary correctly
- [ ] Validate all 6 skills are discoverable via `/scout:`
- [ ] Confirm all 3 agents appear in agent selection
- [ ] Test MCP tools respond correctly after install
