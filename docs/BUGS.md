# Bug Tracker

## Open Bugs

### Page.Race always returns index -1
- **Severity:** Medium
- **Status:** Open
- **File:** page.go:478
- **Description:** `Page.Race()` returns `-1` as the matched selector index regardless of which selector won. The return value `el, -1, nil` is hardcoded.
- **Expected:** Should return the 0-based index of the winning selector.

## Resolved Bugs

| Bug | Resolution | Date |
|-----|------------|------|
| (none yet) | | |
