# Scout CLI Command Taxonomy Mapping — Old → New (2026-07-01)

**Date:** 2026-07-01  
**Scope:** Exhaustive mapping of all CLI command paths to the redesigned taxonomy per COMMAND-TAXONOMY.md rules.

## Summary

- **Total leaf commands:** 207
- **Commands changed:** 36
- **Commands unchanged:** 171  
- **Merge collisions:** 5

## Mapping Table

| Old Path | New Path | Source | Changed? | Notes |
|----------|----------|--------|----------|-------|
| aicontext | aicontext | aicontext.go:88 | no | — |
| auth | auth | auth.go:36 | no | — |
| auth capture | auth capture | auth.go:114 | no | — |
| auth login | auth login | auth.go:41 | no | — |
| auth logout | auth logout | auth.go:385 | no | — |
| auth providers | auth providers | auth.go:410 | no | — |
| auth replay | auth replay | auth.go:253 | no | — |
| auth show | auth show | auth.go:306 | no | — |
| auth status | auth status | auth.go:343 | no | — |
| batch | batch | batch.go:30 | no | — |
| bridge | bridge | bridge.go:75 | no | — |
| browser | browser | browser.go:23 | no | — |
| browser download | browser download | browser.go:78 | no | — |
| browser list | browser list | browser.go:28 | no | — |
| call-exposed | call-exposed | bridge.go:787 | no | — |
| capture-host | capture-host | capture.go:166 | no | — |
| capture-host install | capture-host install | capture.go:215 | no | — |
| capture-host keygen | capture-host keygen | capture.go:107 | no | — |
| capture-host uninstall | capture-host uninstall | capture.go:235 | no | — |
| challenge | challenge | challenge.go:11 | no | — |
| challenge detect | challenge detect | challenge.go:16 | no | — |
| challenge solve | challenge solve | challenge.go:61 | no | — |
| clean | clean | session.go:159 | no | — |
| click | click | bridge.go:445 | no | — |
| clipboard | clipboard | bridge.go:655 | no | — |
| cmdtree | cmdtree | cmdtree.go:54 | no | — |
| detect | detect | detect.go:11 | no | — |
| dom | dom | bridge.go:527 | no | — |
| dom insert | dom insert | bridge.go:532 | no | — |
| dom remove | dom remove | bridge.go:574 | no | — |
| emit | emit | bridge.go:837 | no | — |
| events | events | bridge.go:319 | no | — |
| extension | extension | extension.go:29 | no | — |
| extension download | extension download | extension.go:35 | no | — |
| extension list | extension list | extension.go:212 | no | — |
| extension load | extension load | extension.go:72 | no | — |
| extension remove | extension remove | extension.go:55 | no | — |
| extension test | extension test | extension.go:126 | no | — |
| extract | extract | extract.go:21 | no | — |
| extract ai | extract ai | llm.go:61 | no | — |
| extract meta | extract meta | extract.go:95 | no | — |
| extract table | extract table | extract.go:26 | no | — |
| fetch | fetch | fetch.go:20 | no | — |
| fingerprint | fingerprint | fingerprint.go:11 | no | — |
| fingerprint apply | fingerprint apply | fingerprint.go:70 | no | — |
| fingerprint generate | fingerprint generate | fingerprint.go:16 | no | — |
| flow | flow | flow.go:15 | no | — |
| flow analyze | flow analyze | flow.go:48 | no | — |
| flow capture | flow capture | flow.go:17 | no | — |
| flow run | flow run | flow.go:79 | no | — |
| flow verify | flow verify | flow.go:126 | no | — |
| form | form | form.go:26 | no | — |
| form detect | form detect | form.go:31 | no | — |
| form fill | form fill | form.go:93 | no | — |
| form submit | form submit | form.go:153 | no | — |
| frames | frames | bridge.go:886 | no | — |
| github | github | github.go:44 | no | — |
| github code | github code | github.go:341 | no | — |
| github extract-issues | github extract issues | github_extract.go:92 | yes | — |
| github extract-prs | github extract prs | github_extract.go:154 | yes | — |
| github extract-releases | github extract releases | github_extract.go:216 | yes | — |
| github extract-repo | github extract repo | github_extract.go:32 | yes | — |
| github issues | github issues | github.go:115 | no | — |
| github prs | github prs | github.go:180 | no | — |
| github releases | github releases | github.go:294 | no | — |
| github repo | github repo | github.go:59 | no | — |
| github tree | github tree | github.go:399 | no | — |
| github user | github user | github.go:245 | no | — |
| guide | guide | guide.go:21 | no | — |
| hijack | hijack | hijack.go:24 | no | — |
| hijack watch | hijack watch | hijack.go:29 | no | — |
| inject | inject | inject.go:18 | no | — |
| interactions | interactions | interactions.go:21 | no | — |
| interactions list | interactions list | interactions.go:67 | no | — |
| interactions off | interactions off | interactions.go:42 | no | — |
| interactions on | interactions on | interactions.go:26 | no | — |
| interactions status | interactions status | interactions.go:56 | no | — |
| jobs | jobs | jobs.go:31 | no | — |
| jobs cancel | jobs cancel | jobs.go:136 | no | — |
| jobs list | jobs list | jobs.go:36 | no | — |
| jobs status | jobs status | jobs.go:89 | no | — |
| list | list | session.go:27 | no | — |
| list-local | list-local | session.go:65 | no | — |
| listen | listen | bridge.go:189 | no | — |
| ai-job session | llm job | llm.go:410 | yes | Merge collision |
| ai-job | llm job | llm.go:342 | yes | Merge collision |
| ai-job session create | llm job | llm.go:450 | yes | Merge collision |
| ai-job session use | llm job | llm.go:475 | yes | Merge collision |
| ai-job show | llm job | llm.go:384 | yes | Merge collision |
| ai-job session list | llm job | llm.go:415 | yes | Merge collision |
| ai-job list | llm job | llm.go:348 | yes | Merge collision |
| ollama | llm ollama | llm.go:266 | yes | Merge collision |
| ollama list | llm ollama | llm.go:271 | yes | Merge collision |
| ollama pull | llm ollama | llm.go:295 | yes | Merge collision |
| ollama status | llm ollama | llm.go:322 | yes | Merge collision |
| logger | logger | logger.go:14 | no | — |
| mcp | mcp | mcp.go:26 | no | — |
| mcp open | mcp open | mcp_open.go:13 | no | — |
| mcp screenshot | mcp screenshot | mcp_screenshot.go:11 | no | — |
| observe | observe | bridge.go:256 | no | — |
| okf | okf | okf.go:23 | no | — |
| attr | page attr | inspect.go:114 | yes | — |
| eval | page eval | inspect.go:142 | yes | — |
| html | page html | inspect.go:196 | yes | — |
| text | page text | inspect.go:86 | yes | — |
| title | page title | inspect.go:40 | yes | — |
| url | page url | inspect.go:63 | yes | — |
| pdf | pdf | screenshot.go:19 | no | — |
| pdf-form fill | pdf form | pdf.go:94 | yes | Merge collision |
| pdf-form fields | pdf form | pdf.go:32 | yes | Merge collision |
| pdf-form | pdf form | pdf.go:27 | yes | Merge collision |
| plugin | plugin | plugin_host.go:24 | no | — |
| plugin hosts | plugin hosts | plugin_host.go:271 | no | — |
| profile | profile | profile.go:15 | no | — |
| profile capture | profile capture | profile.go:20 | no | — |
| profile diff | profile diff | profile.go:256 | no | — |
| profile load | profile load | profile.go:92 | no | — |
| profile merge | profile merge | profile.go:224 | no | — |
| profile show | profile show | profile.go:147 | no | — |
| proxy | proxy | proxy.go:23 | no | — |
| proxy routes | proxy routes | proxy.go:70 | no | — |
| proxy start | proxy start | proxy.go:29 | no | — |
| prune | prune | session.go:116 | no | — |
| query | query | bridge.go:401 | no | — |
| record | record | bridge.go:928 | no | — |
| record | record | record.go:23 | no | — |
| repl | repl | repl.go:23 | no | — |
| report | report | report.go:17 | no | — |
| report delete | report delete | report.go:63 | no | — |
| report list | report list | report.go:22 | no | — |
| report schedule | report schedule | report_schedule.go:24 | no | — |
| report schedule stop | report schedule stop | report_schedule.go:103 | no | — |
| report show | report show | report.go:49 | no | — |
| research | research | research.go:29 | no | — |
| reset | reset | session.go:214 | no | — |
| rm | rm | session.go:192 | no | — |
| runbook | runbook | runbook.go:67 | no | — |
| runbook apply | runbook apply | runbook.go:72 | no | — |
| runbook create | runbook create | runbook.go:233 | no | — |
| runbook fix | runbook fix | runbook.go:344 | no | — |
| runbook flow | runbook flow | runbook.go:586 | no | — |
| runbook plan | runbook plan | runbook.go:144 | no | — |
| runbook presets | runbook preset list | runbook.go:433 | yes | — |
| runbook run-preset | runbook preset run | runbook.go:469 | yes | — |
| runbook sample | runbook sample | runbook.go:397 | no | — |
| runbook validate | runbook validate | runbook.go:127 | no | — |
| scrape | scrape | scrape.go:60 | no | — |
| scrape auth | scrape auth | scrape.go:85 | no | — |
| scrape list | scrape list | scrape.go:66 | no | — |
| scrape run | scrape run | scrape.go:151 | no | — |
| screenshot | screenshot | mcp_screenshot.go:68 | no | — |
| search | search | search.go:30 | no | — |
| search wikipedia | search wikipedia | search.go:172 | no | — |
| send | send | bridge.go:126 | no | — |
| session | session | session.go:22 | no | — |
| session audit | session audit | session_audit.go:52 | no | — |
| setup | setup | setup.go:17 | no | — |
| crawl | site crawl | crawl.go:23 | yes | — |
| gather | site gather | gather.go:37 | yes | — |
| knowledge | site knowledge | knowledge.go:25 | yes | — |
| sitemap | site map | sitemap.go:30 | yes | Merge collision |
| sitemap extract | site map | sitemap.go:35 | yes | Merge collision |
| map | site map | map.go:24 | yes | Merge collision |
| test-site | site test | testsite.go:27 | yes | — |
| snapshot | snapshot | snapshot.go:21 | no | — |
| status | status | bridge.go:81 | no | — |
| strategy | strategy | strategy.go:52 | no | — |
| strategy init | strategy init | strategy.go:157 | no | — |
| strategy run | strategy run | strategy.go:58 | no | — |
| strategy validate | strategy validate | strategy.go:128 | no | — |
| subplugin | subplugin | plugin.go:45 | no | — |
| subplugin check-updates | subplugin check-updates | plugin.go:575 | no | — |
| subplugin list | subplugin list | plugin.go:50 | no | — |
| subplugin remove | subplugin remove | plugin.go:392 | no | — |
| subplugin run | subplugin run | plugin.go:418 | no | — |
| subplugin search | subplugin search | plugin.go:461 | no | — |
| subplugin update | subplugin update | plugin.go:490 | no | — |
| swagger | swagger | swagger.go:19 | no | — |
| tabs | tabs | bridge.go:615 | no | — |
| type | type | bridge.go:486 | no | — |
| update | update | update.go:23 | no | — |
| update check | update check | update.go:96 | no | — |
| vault | vault | vault.go:13 | no | — |
| vault capture | vault capture | vault.go:165 | no | — |
| vault get | vault get | vault.go:99 | no | — |
| vault import-captures | vault import | capture.go:175 | yes | — |
| vault init | vault init | vault.go:18 | no | — |
| vault capture-key | vault key | capture.go:126 | yes | Merge collision |
| vault capture-key init | vault key | capture.go:131 | yes | Merge collision |
| vault list | vault list | vault.go:85 | no | — |
| vault rm | vault rm | vault.go:233 | no | — |
| vault rotate | vault rotate | vault.go:211 | no | — |
| vault set | vault set | vault.go:37 | no | — |
| vault use | vault use | vault.go:119 | no | — |
| version | version | version.go:17 | no | — |
| vpn | vpn | vpn.go:12 | no | — |
| vpn connect | vpn connect | vpn.go:58 | no | — |
| vpn disconnect | vpn disconnect | vpn.go:81 | no | — |
| vpn servers | vpn servers | vpn.go:100 | no | — |
| vpn status | vpn status | vpn.go:17 | no | — |
| webmcp | webmcp | webmcp.go:20 | no | — |
| webmcp call | webmcp call | webmcp.go:75 | no | — |
| webmcp discover | webmcp discover | webmcp.go:25 | no | — |
| webmcp inspect | webmcp inspect | webmcp.go:128 | no | — |
| ws | ws | websocket.go:24 | no | — |
| ws listen | ws listen | websocket.go:30 | no | — |
| ws-send | ws-send | bridge.go:712 | no | — |
## Merge Collisions

### llm job

Conflicting old paths that map to this new path:
- ai-job
- ai-job list
- ai-job session
- ai-job session create
- ai-job session list
- ai-job session use
- ai-job show

### llm ollama

Conflicting old paths that map to this new path:
- ollama
- ollama list
- ollama pull
- ollama status

### pdf form

Conflicting old paths that map to this new path:
- pdf-form
- pdf-form fields
- pdf-form fill

### site map

Conflicting old paths that map to this new path:
- map
- sitemap
- sitemap extract

### vault key

Conflicting old paths that map to this new path:
- vault capture-key
- vault capture-key init

## Flag Drift / Shorthand Collisions

Per R5, root persistent flags are: -o/--output, -v/--verbose, --format, --session, --browser, --headless, --stealth, --devtools.

Status: No critical collisions identified; detailed flag audit deferred to Phase 6.

---

*Mapping generated per COMMAND-TAXONOMY.md (R1–R7) + design spec structural moves.*
