---
name: monitor
description: Perform visual regression monitoring on a web page. Use when the user wants to check for visual changes, compare page states, or detect regressions.
---

# Monitor

Check a web page for visual changes by capturing and comparing screenshots.

**Input:** `$ARGUMENTS` should be a URL to monitor, optionally with a baseline description.

## Workflow

1. **Navigate** to the URL using `mcp__scout__navigate`.
2. **Wait** for the page to fully render using `mcp__scout__wait`.
3. **Screenshot** the page using `mcp__scout__screenshot` for a full-page capture.
4. **Snapshot** the page with `mcp__scout__snapshot` to capture the current DOM state.
5. **Evaluate** key metrics using `mcp__scout__eval`:
   - Element count and structure
   - Visible text content hash
   - Key element positions
6. **Compare** against any previous baseline the user has provided or established:
   - If this is the first run, save the screenshot and snapshot as the baseline reference.
   - If a baseline exists, describe the visual and structural differences.
7. **Report** findings:
   - Visual differences detected (layout shifts, missing elements, new content)
   - DOM structure changes
   - Recommendations (whether changes look intentional or problematic)

## Guidelines

- Focus on meaningful changes, not minor rendering differences.
- Compare structure (element counts, hierarchy) alongside visual appearance.
- Use the Write tool to save baseline data to a local file if the user wants persistent monitoring.
- Suggest setting up periodic checks if the user is monitoring for regressions.
