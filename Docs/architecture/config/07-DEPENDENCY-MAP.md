# 07 — Dependency Map: config

> Last verified: 2026-07-13  
> Direction: who imports `codenerd/internal/config`, and what config imports.

## 1. Upstream (config depends on)

| Package | Why |
|---------|-----|
| `codenerd/internal/features` | `FeaturesConfig` type; `SetActive` / `Summary` on load |
| `codenerd/internal/logging` | Boot/debug logs in Load / LoadUserConfig |
| `codenerd/internal/mcp` | `MCPServerConfig` conversion types |
| stdlib | `os`, `path/filepath`, `encoding/json`, `time`, `slices`, `runtime`, `strings`, `fmt` |
| `gopkg.in/yaml.v3` | YAML Load/Save for `Config` |

**VERIFIED CURRENT.** There is no import of `internal/core`,
`internal/perception`, or `cmd/nerd`; config stays low in the DAG. This import
boundary does not prove its values are wired by those downstream consumers.

## 2. Downstream (imports config) — major clusters

### 2.1 CLI / chat (primary mutators)

| Area | Usage |
|------|--------|
| `cmd/nerd/main.go` | `config.Load` YAML early path |
| `cmd/nerd/chat/session.go` | `FindWorkspaceRoot` |
| `cmd/nerd/chat/session_boot.go` | CoreLimits, scheduler, timeouts |
| `cmd/nerd/chat/commands_handlers*.go` | Load/Save, provider, engine UX |
| `cmd/nerd/chat/config_wizard*.go` | Interactive write |
| `cmd/nerd/chat/process*.go` | `GetLLMTimeouts` for articulation/follow-up |
| `cmd/nerd/cmd_auth.go` | Persist engine/provider |
| `cmd/nerd/cmd_campaign.go` | Limits, scheduler + partial execution projection |
| `cmd/nerd/embedding_cmd.go` | Embedding config |
| `cmd/nerd/cmd_spawn.go` | FollowUpTimeout |
| `cmd/nerd/cmd_transparency.go` | Transparency types |
| `cmd/nerd/cmd_init_scan.go` | Config path / keys |

### 2.2 Runtime libraries

| Package | Usage |
|---------|--------|
| `internal/system` | factory LoadUserConfig at boot |
| `internal/init` | Seed / scanner config |
| `internal/perception` | providers, timeouts, tests |
| `internal/build` | BuildConfig env |
| `internal/transparency` | TransparencyConfig |
| `internal/ux` | Onboarding/preferences/migration |
| `internal/autopoiesis` | LoadUserConfig for tool generation |
| `internal/mangle/feedback` | types referencing config |
| `internal/features` tests | LoadUserConfig roundtrip |

## 3. Import graph (simplified)

```
cmd/nerd ──► internal/config ──► internal/features
    │              │
    │              ├──► internal/logging
    │              └──► internal/mcp (types only)
    │
    ├──► internal/system ──► config
    ├──► internal/perception ──► config
    └──► internal/core  (does NOT import config; uses features + values passed in)
```

## 4. Fact-flow adjacency

```
config ──values──► perception clients ──facts──► user_intent
config ──limits──► core APIScheduler / fact ceilings
config ──budgets─► prompt JIT / context window
config ──allow───► tactile execution (via ExecutionConfig values)
```

**PARTIAL.** The last arrow is complete and validated for shared Cortex. Campaign
start/resume copy binaries/env/directory without the shared timeout/containment
helper, and dormant `cmd/nerd/chat/session_boot.go#performSystemBootLegacy` uses
defaults. See `internal/system/factory_execution.go#executionLayerConfigs`.

## 5. Refresh commands

```powershell
rg "codenerd/internal/config" -g "*.go" --stats
rg "LoadUserConfig|DefaultUserConfigPath|GetActiveProvider" -g "*.go"
```
