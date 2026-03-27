---
name: gather
description: Collect comprehensive intelligence about a web page in one shot. Use when the user wants a full analysis, overview, or reconnaissance of a URL.
---

# Gather

One-shot page intelligence collection. Gathers everything about a URL in a single pass.

**Input:** `$ARGUMENTS` should be a URL to analyze.

## Workflow

1. **Navigate** to the URL using `mcp__scout__navigate`.
2. **Wait** for the page to fully load using `mcp__scout__wait`.
3. **Collect page metadata** by reading MCP resources:
   - `scout://page/url` for the current URL (after redirects)
   - `scout://page/title` for the page title
   - `scout://page/markdown` for the page content as markdown
4. **Snapshot** the page with `mcp__scout__snapshot` to get the full DOM structure and accessibility tree.
5. **Screenshot** the page with `mcp__scout__screenshot` for a visual capture.
6. **Evaluate** JavaScript to gather additional metadata:
   - `document.querySelector('meta[name="description"]')?.content` for meta description
   - `performance.timing` for load metrics
   - Framework detection hints (React, Vue, Angular, etc.)
   - Cookie information via `document.cookie`
7. **Compile** a comprehensive report including: URL, title, description, content summary, technologies detected, screenshot, key links, and metadata.

## Guidelines

- This is a read-only intelligence gathering operation. Do not click or interact with the page.
- Summarize the markdown content rather than returning it verbatim if it's very long.
- Identify the page's purpose (landing page, article, product page, dashboard, etc.).
- Note any authentication requirements or access restrictions.
