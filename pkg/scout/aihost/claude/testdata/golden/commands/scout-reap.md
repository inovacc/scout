---
description: Reap orphaned/stale Scout sessions and drain the pending-cleanup queue.
argument-hint: (none)
---
Reclaim leaked Scout browser processes and locked session dirs.

Run via Bash: scout session audit --kill
Then check the pending queue: scout session list --pending

Report what was reaped and any dirs still locked (they are handed to Scout's background retrier). Prefer this targeted reap over a full reset when only orphans need clearing.

<!-- created:2026-05-24 -->
