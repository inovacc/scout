---
name: crawl
description: Crawl a website to discover pages and map its structure. Use when the user wants to explore, map, or discover all pages on a site.
---

# Crawl

Crawl a website to discover pages, map link structure, and build a sitemap.

**Input:** `$ARGUMENTS` should be a URL to crawl, optionally followed by parameters (e.g., `https://example.com workers:4 depth:3 max:100`).

## Workflow

1. **Crawl** the site using `mcp__scout__swarm_crawl` with:
   - `url`: the target URL from arguments
   - `workers`: number of parallel workers (default 2)
   - `depth`: crawl depth limit (default 2)
   - `maxPages`: maximum pages to visit (default 50)
2. **Report** the crawl results:
   - Total pages discovered
   - Page hierarchy / site structure
   - Broken links found
   - External links
   - Crawl statistics (time, pages/second)

## Guidelines

- Start with conservative defaults (2 workers, depth 2, 50 max pages) to be respectful.
- Increase workers/depth only when the user explicitly requests it.
- Group discovered pages by section or path prefix for readability.
- Highlight any errors or unreachable pages.
