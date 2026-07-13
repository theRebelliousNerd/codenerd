# 08 — Wiring and Integration: config

> Last verified: 2026-07-13  
> How configuration is registered, loaded, and passed through boot.

## 1. Boot wiring journal

### 1.1 Early CLI (`cmd/nerd/main.go`)

- May call `config.Load(configPath)` for YAML-shaped early settings (logging path, etc.).
- Interactive chat path then re-resolves workspace and JSON UserConfig.

### 1.2 Chat session (`cmd/nerd/chat/session.go`)

```
FindWorkspaceRoot()
  → set workspace root for session
  → DefaultUserConfigPath() implied for later loads
```

### 1.3 Cortex assembly (`session_boot.go`, `internal/system/factory.go`)

Typical pattern observed in consumers:

1. `LoadUserConfig(DefaultUserConfigPath())` (errors often soft-ignored with empty config)
2. `GetCoreLimits()` → fact ceilings / shard concurrency
3. `GetEffectiveAPISchedulerPolicy()` or manual scheduler fields + `GetLLMTimeouts().SlotAcquisitionTimeout`
4. `GetActiveProvider()` / `GetEngine()` → construct perception client
5. `GetEmbeddingConfig()` → embedding engine
6. `GetJITConfig` / transparency / logging → observability surfaces

### 1.4 Features install (side effect of load)

```
LoadUserConfig
  → features.SetActive(cfg.Features)
  → kernel/world/main consult features without importing config
```

## 2. Mutating surfaces

| Surface | Action |
|---------|--------|
| Config wizard (`chat/config_wizard*.go`) | Load → edit fields → Save |
| Slash / command handlers | Provider/engine/theme updates + Save |
| `cmd_auth.go` | Engine/OAuth-related persistence |
| `nerd init` / `internal/init` | Seed `DefaultUserConfig`-like JSON |
| Manual edit of `.nerd/config.json` | Next process load picks up |

No inotify/watch loop — changes require restart or explicit reload by caller.

## 3. Campaign / long-horizon

`cmd_campaign.go` reloads UserConfig and reapplies core limits + scheduler so campaigns inherit the same ceilings as interactive sessions.

## 4. Embedding management

`embedding_cmd.go` loads UserConfig and uses `GetEmbeddingConfig()` so reembed / status commands share boot’s model name.

## 5. MCP integrations

```
UserConfig.Integrations
  → GetIntegrations()
  → ToMCPServerConfigs()
  → mcp layer connects enabled servers
```

Env URLs for code_graph/browser/scraper apply on **YAML Config** path via `applyEnvOverrides`, not automatically on JSON unless mirrored in file.

## 6. Timeouts wiring

Widespread pattern:

```go
ctx, cancel := context.WithTimeout(parent, config.GetLLMTimeouts().ArticulationTimeout)
```

Also: FollowUpTimeout, SlotAcquisitionTimeout, ShardExecutionTimeout (via callers). `SetLLMTimeouts` should only run at startup.

## 7. Constitutional path (data only)

```
ExecutionConfig.AllowedBinaries / AllowedEnvVars
        │
        ▼
tactile / VirtualStore action execution
        │
        ▼
kernel permitted(...) still final executive gate
```

Config allowlists are **inputs**, not substitutes for `permitted(...)`.

## 8. Wiring risks

| Risk | Mitigation |
|------|------------|
| Soft `_` ignore of LoadUserConfig errors | Can hide malformed JSON; prefer logging |
| Dual Load (YAML + JSON) | Document which fields each path owns |
| Features not reinstalled after mid-session Save | Save does not call SetActive; restart or re-load needed for Features block |
| Stray nested .nerd | FindWorkspaceRoot go.mod-first |

## 9. Integration checklist for new knobs

1. Add field on `UserConfig` with json tag.  
2. Add `GetX` with defaults.  
3. Wire one boot consumer (factory or session_boot).  
4. Add test in `config_comprehensive_test.go` or focused file.  
5. Grep for hard-coded old constants at call sites.  
6. Update this corpus if behavior is non-obvious.
