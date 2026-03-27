---
name: test-site
description: Health-check a website for broken links, errors, and issues. Use when the user wants to test, audit, or validate a site.
---

# Test Site

Run a health check on a website to find broken links, JavaScript errors, and other issues.

**Input:** `$ARGUMENTS` should be a URL to test, optionally followed by depth (e.g., `https://example.com depth:3`).

## Workflow

1. **Navigate** to the target URL using `mcp__scout__navigate`.
2. **Wait** for the page to load using `mcp__scout__wait`.
3. **Snapshot** the page with `mcp__scout__snapshot` to understand the page structure and identify links.
4. **Check console errors** using `mcp__scout__eval` to capture `window.__scout_console_errors` or evaluate `console.error` interceptor results.
5. **Crawl** the site using `mcp__scout__swarm_crawl` with appropriate depth and worker settings to discover pages and detect broken links.
6. **Report** findings in a structured format:
   - Broken links (404s, timeouts)
   - JavaScript errors
   - Pages discovered
   - Load time observations
   - Recommendations

## Guidelines

- Default crawl depth is 2 unless the user specifies otherwise.
- Use 2-3 workers for crawling to balance speed and politeness.
- Flag any mixed-content warnings (HTTP resources on HTTPS pages).
- Note pages with unusually slow load times.
- Present results as a checklist with pass/fail/warn indicators.
