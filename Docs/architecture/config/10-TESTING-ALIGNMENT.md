# 10 — Testing Alignment: config

> Last verified: 2026-07-13  

## 1. Test inventory

| File | Focus areas |
|------|-------------|
| `config_comprehensive_test.go` | Load missing/malformed/valid YAML; Save roundtrip; Validate providers; duration helpers; shard profiles; MCP helpers; UserConfig provider/engine/CLI defaults; UserConfig load/save; context/embedding/core limits; Context7; DefaultUserConfig |
| `config_test.go` | DefaultConfig; Save/Load; env overrides; Validate; helpers; FindWorkspaceRoot (prefer go.mod, bypass nested .nerd, fallback); DefaultUserConfigPath; GetActiveProvider priority; SetEngine; Codex defaults; effective concurrency; API scheduler policy; Context7 env |
| `config_defaults_test.go` | BuildConfig; ValidateCoreLimits; EnforceCoreLimits; ContextWindow budget; LLM timeout presets; global timeouts; MCP DefaultTimeout; ToMCPServerConfigs; Logging IsCategoryEnabled |
| `env_override_test.go` | LLM key precedence matrix; embedding GenAI/Ollama env; integrations + CODENERD_DB; MCP enabled helpers |
| `ollama_worker_config_test.go` | Ollama defaults; worker; active provider ollama; image model/shard detection; image config aliases |

## 2. Coverage strengths

- Provider priority and config-is-boss cases (including missing key).  
- Engine validation.  
- Workspace root algorithms (critical for multi-project machines).  
- Scheduler policy pointer overrides.  
- Duration parse fallbacks.  
- MCP conversion skips disabled servers.

## 3. Coverage gaps

| Area | Gap |
|------|-----|
| `GetEffectiveJITConfig` clamp edge cases | Thin / absent dedicated tests |
| Full `DefaultUserConfig` field assertions | Only partial “sensible defaults” |
| Transparency / onboarding / guidance helpers | Minimal |
| Features.SetActive side effects | Covered more in `internal/features` tests |
| Concurrent Save/Load | Not stressed |
| Image/worker interaction with real clients | Unit-only string/config level |
| YAML vs JSON default parity | Not asserted as property tests |

## 4. Commands

```powershell
go test ./internal/config/...
go test ./internal/config/ -count=1 -timeout 60s
go test ./internal/config/ -run FindWorkspaceRoot -v
go test ./internal/config/ -run GetActiveProvider -v
go test ./internal/config/ -run APIScheduler -v
```

Related reverse tests:

```powershell
go test ./internal/features/ -run Config -count=1
go test ./internal/build/ -count=1
```

## 5. Alignment to principles

| Principle | Test evidence |
|-----------|---------------|
| Config-is-boss | `TestGetActiveProvider_WhenExplicitProvider_*` |
| Workspace go.mod-first | `TestFindWorkspaceRoot_BypassesStrayNestedNerd` |
| Engine set | `TestSetEngine_WhenInvalid_ShouldReturnError` |
| Scheduler defaults | `TestUserConfig_GetEffectiveAPISchedulerPolicy` |
| Env precedence | `env_override_test.go` |

## 6. Recommended new tests (backlog)

1. Property: for each ValidProviders entry, Validate passes with key (ollama without).  
2. GetEffectiveJITConfig when ReservedTokens ≥ TokenBudget.  
3. Dual-default documentation tests if/when defaults are unified.  
4. Save does not call features.SetActive — document expected re-load.
