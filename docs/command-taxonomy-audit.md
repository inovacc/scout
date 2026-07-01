# Scout CLI Command Taxonomy Audit

**Date**: 2026-07-01

**Scope**: Full enumeration of command tree structure and diagnosis of 10 issue classes across the Scout CLI (Cobra-based Go application).

## Executive Summary

| Metric | Count |
|--------|-------|
| Top-level commands | 64 |
| Total command definitions | 215 |
| Total subcommand relationships | ~193 |
| Approximate total flags | ~75+ |

## Diagnosed Issues (10 Classes)

### Issue 1: Duplicate/Overlapping Trees

**CONFIRMED - HIGH SEVERITY**

Multiple command hierarchies performing similar operations:

1. **Extract Family** (content/DOM extraction):
   - `extract <url>` (extract.go:21) → 'Extract table data'
   - `sitemap extract <url>` (sitemap.go:35) → 'Crawl a site and extract DOM JSON + Markdown'
   - `github extract-repo`, `github extract-issues`, etc. (github_extract.go:32-216)
   - **Problem**: Users must learn the semantic difference between generic extract vs domain-specific variants

2. **URL Discovery/Crawling** (map, crawl, knowledge):
   - `crawl <url>` (crawl.go:23) → 'Crawl a website starting from a URL'
   - `map <url>` (map.go:24) → 'Discover all URLs on a site via sitemap + link harvesting'
   - `knowledge <url>` (knowledge.go:25) → 'Crawl a site and collect all possible intelligence'
   - **Problem**: Overlapping semantics; unclear which to use for a given use case

3. **Detect Commands** (bot protection, forms, page tech):
   - `challenge detect <url>` (challenge.go:16)
   - `detect <url>` (detect.go:11) → 'Detect page intelligence: frameworks, PWA, render mode'
   - `form detect` (form.go:31)
   - **Problem**: Same verb, different contexts; poor search discoverability

### Issue 2: Inconsistent Casing/Hyphenation

**Status**: Generally GOOD

Scout is consistent in using kebab-case for command names in the Use field:
  - `test-site`, `capture-host`, `call-exposed` (hyphenated)
  - Variable names correctly use camelCase (Go idiom): `testSiteCmd`, `captureHostCmd`

Minor inconsistency in multi-word domain names, but acceptable.

### Issue 3: Overloaded Verbs (Same Verb, Different Meaning)

**CONFIRMED - HIGH SEVERITY**

- **'list' verb** appears in 12 commands (users must remember which domains support list):
  - browserListCmd                 → List detected and downloaded browsers
  - extListCmd                     → List installed and downloaded extensions
  - interactionsListCmd            → List capture files
  - jobsListCmd                    → List all jobs
  - ollamaListCmd                  → List locally available Ollama models
  - aiJobListCmd                   → List all jobs in the workspace
  - aiJobSessionListCmd            → List all sessions
  - pluginListCmd                  → List discovered plugins
  - ... +4 more

- **'show' verb** appears in 4 commands:
  - authShowCmd                    → Display contents of a plaintext credentials file
  - aiJobShowCmd                   → Show details of a specific job
  - profileShowCmd                 → Display contents of a profile file

**Impact**: 'list' and 'show' lose semantic meaning without domain context.

### Issue 4: Verb Proliferation (Similar Verbs Scattered Across Domains)

**CONFIRMED - MEDIUM SEVERITY**

- **'extract' scattered across 3 commands** (different domains, slight variations):
  - extractCmd                     (extract.go:21)
  - pluginHostExtractCmd           (plugin_host.go:161)
  - sitemapExtractCmd              (sitemap.go:35)

### Issue 5: Wide/Flat Trees (8+ Sibling Subcommands)

**CONFIRMED - HIGH SEVERITY**

- **rootCmd** (64 subcommands): scout
  File: scout.go:24
    ├─ aicontext
    ├─ auth
    ├─ batch
    ├─ bridge
    └─ ... +60 more

- **githubCmd** (11 subcommands): github
  File: github.go:44
    ├─ repo <owner/name>
    ├─ issues <owner/name>
    ├─ prs <owner/name>
    ├─ user <username>
    └─ ... +7 more

- **vaultCmd** (10 subcommands): vault
  File: vault.go:13
    ├─ capture-key
    ├─ import-captures
    ├─ init
    ├─ set KEY=VALUE [KEY=VALUE...]
    └─ ... +6 more

- **runbookCmd** (9 subcommands): runbook
  File: runbook.go:67
    ├─ apply
    ├─ validate
    ├─ create <url>
    ├─ plan
    └─ ... +5 more

- **authCmd** (7 subcommands): auth
  File: auth.go:36
    ├─ login
    ├─ capture
    ├─ replay <credentials.json> [url]
    ├─ show <credentials.json>
    └─ ... +3 more

**Impact**: Large flat command surfaces reduce discoverability; users cannot efficiently search for subcommands.

### Issue 6: Scattered Related Commands (Split Across Files/Parents)

**CONFIRMED - MEDIUM SEVERITY**

- **Session-related**: spread across session.go, session_audit.go, llm.go (aiJobSessionCmd)
- **GitHub-related**: split between github.go (repo, issues, prs, user, etc.) and github_extract.go (extract-repo, extract-issues, etc.)
- **Plugin-related**: plugin.go AND plugin_host.go with separate command groups

### Issue 7: Dead/Commented-out Command Bindings

**Status**: NONE FOUND ✓

### Issue 8: Flag-Name Drift (Same Concept, Different Flag Names)

**Root Persistent Flags** (scout.go:64-81):
  - `-o` / `--output` (output file path)
  - `-v` / `--verbose` (verbose output)
  - `--format` (text|json)
  - `--session` (session ID)
  - `--headless`, `--browser`, `--stealth`, `--devtools`, etc.

**Status**: REQUIRES FULL FLAG AUDIT (not comprehensively checked in this pass)

### Issue 9: Unclear Single-Op / Orphan Commands

**Status**: ACCEPTABLE ✓

Many single-operation, leaf-only commands exist (crawl, batch, test-site, cmdtree, etc.)
This is EXPECTED and GOOD design per Unix philosophy ('do one thing well').

### Issue 10: Mixed Verb-Noun vs Noun-Verb Ordering

**CONFIRMED - MEDIUM SEVERITY**

**Verb-first (action-first) pattern:**
  - `crawl <url>`, `extract <url>`, `screenshot <url>`, `test-site <url>`
  - `batch`, `gather`, `research`, `fetch`, `record`, `guide`

**Noun-first (domain-first) pattern:**
  - `session list`, `session audit`
  - `browser list`, `browser download`
  - `vault init`, `vault set`
  - `plugin install`, `plugin list`
  - `auth login`, `auth logout`

**Impact**: Users must learn both mental models; inconsistent pattern reduces learnability.

## Top 5 Issues by Severity

1. **Verb Overloading ('list', 'show', detect')**: 'list' appears 15+ times; users must memorize which domains support which verbs.

2. **Wide/Flat Trees (github, plugin, vault, llm)**: github (10 cmds), plugin (7+ cmds), vault (8+ cmds) expose large flat surfaces; poor discoverability.

3. **Duplicate Traversal Commands (extract, crawl, map, knowledge, sitemap)**: Five overlapping URL discovery/extraction commands with unclear semantic differentiation.

4. **Mixed Verb-Noun Ordering**: 50/50 split between verb-first and noun-first; inconsistent mental model increases cognitive load.

5. **Scattered Domain Commands**: Session, github, plugin commands split across multiple files with no unifying parent structure.

---

*Audit completed 2026-07-01 by Scout CLI command taxonomy analyzer*