---
name: screenshot
description: Capture a screenshot of a web page. Use when the user wants a visual capture, page preview, or screenshot of a URL.
---

# Screenshot

Capture a screenshot of any web page using Scout's headless browser.

**Input:** `$ARGUMENTS` should be a URL to screenshot.

## Workflow

1. **Navigate** to the URL using `mcp__scout__navigate`.
2. **Wait** for the page to be ready using `mcp__scout__wait`. If a specific element matters, wait for its selector.
3. **Screenshot** the page using `mcp__scout__screenshot`. Request full-page capture if the user wants the entire page, or viewport-only for above-the-fold.
4. **Return** the screenshot image to the user.

## Guidelines

- Default to full-page screenshots unless the user specifies viewport-only.
- If the page has cookie banners or popups, try to dismiss them with `mcp__scout__click` before capturing.
- For pages that load content dynamically, wait a moment after navigation for assets to render.
