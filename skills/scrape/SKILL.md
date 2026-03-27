---
name: scrape
description: Extract structured data from a web page. Use when the user wants to scrape, extract, or collect data from a URL.
---

# Scrape

Extract structured data from a web page using Scout's browser automation.

**Input:** `$ARGUMENTS` should be a URL, optionally followed by extraction instructions (selectors, fields, pagination).

## Workflow

1. **Navigate** to the target URL using the `mcp__scout__navigate` tool.
2. **Wait** for the page to load using `mcp__scout__wait` with a reasonable selector or timeout.
3. **Snapshot** the page with `mcp__scout__snapshot` to understand the DOM structure, element selectors, and available content.
4. **Extract** data using `mcp__scout__extract` with CSS selectors identified from the snapshot. For complex extraction, use `mcp__scout__eval` to run JavaScript that collects and structures the data.
5. **Paginate** if requested: use `mcp__scout__click` on the next-page button/link, then repeat steps 2-4 for each page.
6. **Return** the extracted data as structured JSON or a formatted table.

## Guidelines

- Always take a snapshot first to understand the page structure before extracting.
- Prefer CSS selectors over XPath.
- For tables, extract headers and rows separately, then combine.
- For lists of items (products, articles, etc.), identify the repeating container element.
- Handle lazy-loaded content by scrolling with `mcp__scout__eval` if needed.
- If the page requires interaction (dropdowns, filters), use `mcp__scout__click` and `mcp__scout__type` before extracting.
