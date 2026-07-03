---
description: Audit Scout browser sessions (classify LIVE/REUSABLE/STALE/ORPHAN); optionally reap.
argument-hint: [--kill]
---
Audit Scout's browser sessions.

Run via Bash: scout session audit $ARGUMENTS

This classifies every ~/.scout/sessions directory against the gops process table. With --kill it reaps stale/zombie/orphan sessions, preserving REUSABLE + HEALTHY ones. Report the classification counts and anything reaped vs preserved. For a deep pass, delegate to the session-janitor agent.

<!-- created:2026-05-24 -->
