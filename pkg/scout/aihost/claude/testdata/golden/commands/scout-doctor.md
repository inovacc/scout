---
description: Diagnose Scout install + session health and surface fixes.
argument-hint: (none)
---
Diagnose Scout's health.

Run via Bash, in order:
- scout doctor
- scout plugin doctor --host claude
- scout session doctor

Summarize each PASS/WARN/FAIL in plain language, identify the most likely root cause if anything is failing, and propose the minimal fix. For deep diagnosis delegate to the scout-doctor agent.

<!-- created:2026-05-24 -->
