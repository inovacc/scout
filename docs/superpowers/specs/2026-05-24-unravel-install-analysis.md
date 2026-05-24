# Unravel install process — analysis

Date: 2026-05-24
Scope: trace `unravel plugin install --host claude` end-to-end and note where scout's port (built today) drifts from the reference.

## Call graph

```
cmd/plugin.go            (umbrella: cobra.Command + flag wiring)
  └── cmd/plugin_install.go     (pluginInstallCmd.RunE)
        ├── selectedHost(cmd)        → aihost.ByName(--host | --claude | default)
        ├── h.InstallTarget()        → ~/.claude/plugins/marketplaces/unravel
        ├── inst := h.(aihost.Installer)
        └── inst.Install(target)
              │
              └── pkg/aihost/claude/install.go::Install(target)
                    ├── WriteAssets(target)          ← walks generatedAssets, atomic tmp+rename, sweeps stale
                    ├── PatchMarketplace()           ← writes .claude-plugin/marketplace.json (single-plugin "./" source)
                    ├── PatchSettings()              ← flips ~/.claude/settings.json enabledPlugins[unravel@unravel]=true
                    ├── UnpatchMcpServers()          ← cleans legacy settings.json mcpServers.unravel (now owned by plugin .mcp.json)
                    ├── MigrateLegacyLocalPlugin()   ← removes old ~/.claude/local-plugins/unravel layout
                    └── RegisterLocalMarketplace(target)  ← exec.LookPath("claude") && claude plugin marketplace add <target>
```

After Install returns, the dispatcher in `cmd/plugin_install.go` also:
- Logs `[install] host=… target=… files=N` to stderr.
- If host satisfies `aihost.Status`, prints the status report (PASS/FAIL grid) to stderr.
- Unless `--no-restart-hint`, prints "Restart the host to load the plugin."

## State written / mutated

| Path | Owner | Action |
|---|---|---|
| `~/.claude/plugins/marketplaces/unravel/.claude-plugin/plugin.json` | created | synthesised from package consts (Name/Version/Author/Repo) |
| `~/.claude/plugins/marketplaces/unravel/.claude-plugin/marketplace.json` | created | declares the single "unravel" plugin pointing at "./" |
| `~/.claude/plugins/marketplaces/unravel/.mcp.json` | created | spec-form `{"mcpServers": {"unravel": {"command": "unravel", "args": ["mcp"]}}}` |
| `~/.claude/plugins/marketplaces/unravel/agents/*.md` | created | rendered Asset bodies (text/template with custom delims `<%…%>`) |
| `~/.claude/plugins/marketplaces/unravel/commands/*.md` | created | same |
| `~/.claude/plugins/marketplaces/unravel/skills/*/SKILL.md` | created | same — Skill assets get `created:` injected into frontmatter, others get a trailing `<!-- created:… -->` |
| `~/.claude/settings.json` | mutated | adds `enabledPlugins["unravel@unravel"]=true`, removes legacy `mcpServers.unravel` |
| Stale files under `commands/`, `agents/`, `skills/` | swept | files in target dirs that aren't in the regenerated `wanted` set get `os.Remove`'d (sweep logged to stderr) |
| `~/.claude/local-plugins/unravel/` | migrated | removed if present; legacy marketplace entry and `unravel@local-plugins` enabled-key cleared |

## Atomicity / safety properties

- **Writes are atomic per file** — `os.WriteFile(tmp)` + `os.Rename(tmp, dst)`. No half-written markdown if the process is killed mid-write.
- **JSON patches preserve unrelated keys** — `PatchSettings` reads, unmarshals to `map[string]any`, mutates only `enabledPlugins[<our key>]`, then atomic-writes. The `TestPatchSettingsPreservesUnrelatedKeys` test pins this contract (theme, effortLevel, customKey, and other marketplaces survive).
- **Idempotent** — re-running `Install` with the same assets is a no-op for unchanged files (atomic rename overwrites) and idempotent for marketplace.json. `TestPatchMarketplaceCreatesAndUpdates` verifies this.
- **Stale-asset sweep is bounded** — only under `commands/`, `agents/`, `skills/`. `.claude-plugin/` is owned by `PatchMarketplace` and not swept.
- **`claude` CLI is optional** — if not on PATH, install completes successfully but prints a "run manually" hint instead of registering the marketplace. The plugin still loads on next Claude start because `marketplace.json` is on disk.
- **No network calls** — entire install is local-filesystem + one `exec.Command("claude", ...)`. No package downloads, no GitHub API, no module proxy.

## How `aihost.Host` capabilities are discovered

The dispatcher never calls `Installer.Install` directly on the interface — it type-asserts:

```go
inst, ok := h.(aihost.Installer)
if !ok { return fmt.Errorf("host %q does not yet implement install", h.Name()) }
```

Same for `Status` and `Doctor`. This is why codex/gemini can register a `Host` (Walk + ManifestFiles only) without implementing the full surface — and the CLI surfaces a clear "not yet implemented" message instead of a nil-pointer panic.

## Where scout's port drifts from unravel

The port shipped today (`pkg/scout/aihost/...`) mirrors unravel structurally. Notable deltas:

| Aspect | Unravel | Scout (today) | Verdict |
|---|---|---|---|
| `MigrateLegacyLocalPlugin()` | yes | **no** | Correct — scout never had a `~/.scout/local-plugins/` layout to migrate. Don't port. |
| Tests (`install_test.go`) | yes (3 tests pinning roundtrip, settings preservation, idempotent marketplace) | **no** | Gap. Should port; these tests catch the highest-blast-radius regressions. |
| Install output destination | stderr (clean stdout for piping) | mixed | Drift — scout writes the success line to **stdout** (`cmd.OutOrStdout()`), which means piping `scout plugin install \| jq` would choke. Move all `[install] …` lines to stderr. |
| Auto-print status after install | yes (Status capability check + PrintStatus to stderr) | **no** | Gap. Cheap to add — user gets verification without a second command. |
| `MarketplaceFiles()` separation | manifest files come from a single `GeneratedFiles()` map | same | Match. |
| Sweep set | `commands/`, `agents/`, `skills/` | same | Match. |
| `RegisterLocalMarketplace` log line | stderr | stderr | Match. |
| `--no-restart-hint` flag | yes | yes | Match. |
| `doctor` subcommand | only as MCP tool wrapper iterating `aihost.All()` | exposed as **cobra subcommand** with `--json` | Scout went further — surfaces doctor at the CLI. Cleaner UX. |
| `hosts` subcommand | no | yes | Scout added it. Useful — lists capability matrix per host. |

## Recommended follow-ups for scout

1. **Port `install_test.go`.** Three tests, ~80 lines. Highest defect-prevention ROI in the whole subsystem.
2. **Move install logs from stdout → stderr.** One-line fix in `pluginHostInstallCmd.RunE`: change `cmd.OutOrStdout()` → `cmd.ErrOrStderr()` for the `installed N files…` and restart-hint lines. Keeps stdout reserved for structured output (matters when someone does `scout plugin install --json` someday).
3. **Auto-status after install.** After `inst.Install(target)` succeeds, check `h.(aihost.Status)` and print it. Mirrors unravel's UX.
4. **Decide on `aihost.Doctor` exposure.** Scout exposes it; unravel keeps it internal to the MCP tool surface. Both are valid — but pick one consistently. Recommend keeping scout's surfacing since the CLI is the more discoverable path for a tool whose primary install audience is "people running `scout` from a terminal."

## What's load-bearing if you change the install

Touch these and you break the install contract — change them deliberately:

- `Install(target)` step order. `PatchMarketplace` before `PatchSettings` matters: settings.json's `enabledPlugins[unravel@unravel]` references the marketplace key, and CC's plugin loader checks the marketplace exists before honouring the enabled flag.
- `pluginEnabledKey` format (`"<plugin>@<marketplace>"`). This is CC's schema, not ours.
- The `_ = aihost.TemplateData{}` line at the bottom of `install.go` — silences lint when downstream stubs don't reference `aihost` elsewhere in the file. Don't remove without checking.
- `assetRegistry` ordering doesn't matter at write time (atomic rename per file) but **does** matter for deterministic test output. `AllAssets()` and `AssetsByKind()` sort by path; preserve that.
