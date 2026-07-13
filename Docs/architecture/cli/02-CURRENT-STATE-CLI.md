# 02 — Current State: codeNERD CLI

> Precise inventory as of 2026-07-13. All paths relative to repo root.  
> Totals: **113** non-test Go files ≈ **38,110** lines; **55** test files ≈ **16,755** lines under `cmd/nerd/`.

## 1. Binary & entry point

| Property | Value |
|----------|-------|
| Module path | `./cmd/nerd` |
| Binary name | `nerd` / `nerd.exe` |
| Framework | [spf13/cobra](https://github.com/spf13/cobra) |
| Interactive UI | [Charm Bubble Tea](https://github.com/charmbracelet/bubbletea) + Lipgloss/Glamour |
| Default action | `rootCmd.RunE` → `chat.RunInteractiveChat` after optional `Chdir(workspace)` |
| Version string (help) | Cortex 1.5.0 in root Short (`main.go`) |
| CGO | sqlite-vec headers via `CGO_CFLAGS=-I…/sqlite_headers` (root AGENTS.md) |

**Entry flow (default):**

```
main()
  → rootCmd.Execute()
  → PersistentPreRunE (logger + logging.Initialize for non-interactive)
  → RunE:
       resolve --workspace → os.Chdir
       chat.RunInteractiveChat(chat.Config{DisableSystemShards})
```

**Entry flow (subcommand):**

```
main() → rootCmd.Execute() → <cmd>.RunE
  → typically coresys.GetOrBootCortex / package-specific runners
  → PersistentPostRun: zap Sync + logging.CloseAll
```

Source: `cmd/nerd/main.go` (package index comments, `rootCmd`, `init`, `main`).

## 2. Dual-surface architecture

### Surface A — Cobra CLI (`cmd/nerd/*.go`)

Command registration is centralized in `main.go` `init()` with implementations split across `cmd_*.go`, `dom_*.go`, `embedding_cmd.go`, etc. (documented in the `main.go` file index).

| Area | Files (examples) | Role |
|------|------------------|------|
| Root / globals | `main.go` | Flags, rootCmd, command registration |
| OODA single-shot | `cmd_instruction.go` | `run` |
| Workspace | `cmd_init_scan.go` | `init`, `scan` |
| Query/status | `cmd_query.go` | `query`, `status`, `why` |
| Direct verbs | `cmd_direct_actions.go` | review/fix/test/push/commit/explain/create/refactor/security/analyze/perception |
| Advanced | `cmd_advanced.go` | dream/shadow/whatif/logic/agents/tool/jit |
| Campaign | `cmd_campaign.go` | campaign start/status/pause/resume/list |
| Auth | `cmd_auth.go` | auth claude/codex/grok/status |
| Browser | `cmd_browser.go` | browser launch/session/snapshot |
| Mangle tooling | `cmd_mangle_check.go`, `cmd_mangle_lsp.go` | check-mangle, mangle-lsp |
| Systems | `cmd_systems.go` | mcp/autopoiesis/memory trees |
| Transparency | `cmd_transparency.go` | glassbox, transparency, reflection |
| Sessions | `cmd_sessions.go` | sessions list/load |
| Knowledge | `cmd_knowledge.go` | knowledge list/search |
| North star | `cmd_northstar.go` | northstar show/summary/query/… |
| DOM surgical edit | `dom_cmd.go`, `dom_apply_cmd.go`, `dom_replace_cmd.go` | dom demo/inspect/get/edit/apply/replace |
| Embeddings | `embedding_cmd.go` | embedding set/stats/reembed |
| Debug | `cmd_debug.go` | verbose/dry-run/trace flags |
| Interactive loop | `cmd_interactive.go` | refine/redo/approve meta-commands for direct actions |
| Helpers | `system_results.go`, `stats.go`, `campaign_jit_provider.go` | shared helpers |

### Surface B — Interactive chat TUI (`cmd/nerd/chat/`)

Largest implementation surface. Bubble Tea model owns multi-turn agent loop, slash commands, wizards, campaigns, glass box, multistep decomposition.

| File | ≈Lines | Role |
|------|-------:|------|
| `chat/process.go` | 1085 | Input processing, panic recovery, intent pipeline |
| `chat/session_boot.go` | 1015 | Legacy/full system boot assembly |
| `chat/model_update.go` | 933 | Bubble Tea Update |
| `chat/multistep_corpus.go` | 929 | Multistep corpus handling |
| `chat/review_aggregator.go` | 925 | Review aggregation |
| `chat/model_session_context.go` | 869 | Session context |
| `chat/commands_handlers.go` | 865 | Slash command handlers |
| `chat/northstar_wizard.go` | 743 | North-star wizard |
| `chat/model_handlers.go` | 710 | Model message handlers |
| `chat/multistep_decomposer.go` | 683 | Task decomposition |
| `chat/view.go` | 662 | View rendering |
| `chat/model_types.go` | 657 | Model types |
| `chat/process_dream.go` | 653 | Dream state |
| `chat/delegation.go` | 611 | Delegation |
| `chat/commands.go` | (handlers switch) | Slash command router |

Slash command router: `chat/commands.go` — large `switch` on commands including `/help`, `/query`, `/campaign`, `/jit`, `/glassbox`, `/shadow`, `/whatif`, `/spawn`, `/review`, `/fix`, `/config`, session commands, file ops, etc.

### Surface C — UI components (`cmd/nerd/ui/`)

| File | ≈Lines | Role |
|------|-------:|------|
| `ui/splitpane.go` | 864 | Split pane layout |
| `ui/diffview.go` | 687 | Diff presentation |
| `ui/shard_page.go` | 362 | Shard page |
| `ui/styles.go` | (styles) | Lipgloss styles |
| `ui/campaign_page.go` | campaign UI | Campaign page |
| `ui/jit_page.go` | JIT inspection UI | JIT page |
| `ui/autopoiesis_page.go` | autopoiesis UI | Ouroboros UI |

## 3. Global flags (`main.go`)

| Flag | Default | Purpose |
|------|---------|---------|
| `--verbose` / `-v` | false | Debug zap level |
| `--api-key` | "" | Z.AI key (or `ZAI_API_KEY`) |
| `--workspace` / `-w` | CWD | Project root; chat chdirs here |
| `--timeout` | 25m | Operation timeout (config can override) |
| `--disable-system-shard` | nil | On `run` (and chat config) |

Direct-action commands also take `--interactive` / `-i` and debug flags from `registerDebugFlags`.

## 4. Workspace & config touchpoints

| Path | Role |
|------|------|
| `.nerd/` | Per-project state (created by `init`) |
| `.nerd/config.yaml` or config loader paths | Timeouts and engine config (`main.go` PersistentPreRunE loads config) |
| `.nerd/logs/` | Categorized logs via `internal/logging` |
| `.nerd/campaigns/` | Campaign/assault artifacts (chat + campaign docs) |

## 5. Boot wiring (chat)

`chat/session_boot.go` imports and assembles a wide internal graph: config, core, coreshards, perception, prompt, retrieval, session, shards/system, store, embedding, browser, autopoiesis, northstar, transparency, verification, world, tactile, ux, sqlpragmas, etc.

Comments explicitly note domain shards (coder/tester/reviewer/…) removed from direct import in favor of **JIT clean loop / prompt atoms**.

Legacy path: `performSystemBootLegacy`; newer path referenced as `performSystemBoot` in `session_shared_boot.go`.

## 6. Largest risk hotspots (by size)

| Hotspot | Why it matters |
|---------|----------------|
| `chat/process.go` | Every user turn; panic recovery is mandatory |
| `chat/session_boot.go` | Boot failures = no agent |
| `chat/model_update.go` | TUI correctness / races |
| `cmd_campaign.go` | Long-horizon orchestration |
| `ui/splitpane.go` / `diffview.go` | UX correctness under load |

## 7. What is *not* in this package

- Kernel evaluation engine (`internal/core`, `internal/mangle`)
- Prompt atom YAML corpus (`internal/prompt/atoms`)
- Policy `.mg` files (`internal/core/defaults/policy`)

The CLI **hosts** and **drives** those systems; it does not replace them.
