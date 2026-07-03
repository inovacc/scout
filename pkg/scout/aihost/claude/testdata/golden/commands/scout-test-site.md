---
description: Run a health check on a website (broken links, JS errors, etc.).
argument-hint: <url> [options]
---
Test site `$ARGUMENTS` — health-check the URL.

Use the `test-site` skill: crawl with `mcp__scout__swarm_crawl`, check console errors via `mcp__scout__eval`, then produce a pass/warn/fail report grouped by severity.

<!-- created:2026-05-24 -->
