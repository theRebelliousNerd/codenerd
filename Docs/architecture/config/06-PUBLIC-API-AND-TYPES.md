# 06 — Public API and Types: config

> Last verified: 2026-07-13  
> Package import path: `codenerd/internal/config`

## 1. Aggregates

| Type | File | Purpose |
|------|------|---------|
| `UserConfig` | `user_config.go` | Broad JSON aggregate used by primary runtime consumers; mutable and not strictly validated |
| `Config` | `config.go` | Legacy YAML aggregate still used for the Cobra timeout and env override path |

## 2. Load / save / workspace

| Symbol | File | Notes |
|--------|------|-------|
| `FindWorkspaceRoot()` | user_config.go | go.mod-first |
| `DefaultUserConfigPath()` | user_config.go | `{root}/.nerd/config.json` |
| `LoadUserConfig(path)` | user_config.go | strict JSON + features.SetActive |
| `(*UserConfig).Save(path)` | user_config.go | MarshalIndent + private atomic writer |
| `GlobalConfig()` | user_config.go | Load default path |
| `DefaultUserConfig()` | user_config.go | Fully populated seed |
| `DefaultConfig()` | config.go | YAML defaults |
| `Load(path)` | config.go | YAML + env |
| `(*Config).Save(path)` | config.go | YAML marshal |
| `(*Config).Validate()` | config.go | key + provider |
| `ValidProviders` | config.go | string slice |

## 3. Engines and LLM structs

| Type / const | File |
|--------------|------|
| `LLMConfig` | llm.go |
| `ClaudeCLIConfig` | llm.go |
| `CodexCLIConfig` | llm.go |
| `XAIOAuthConfig` | llm.go |
| `GeminiProviderConfig` | llm.go |
| `DefaultGeminiProviderConfig()` | llm.go |
| `DefaultCodexExecSkillName` | llm.go |
| `DefaultCodexMaxConcurrentCalls` | llm.go |
| `DefaultXAIOAuthMaxConcurrentCalls` | llm.go |
| `OllamaLLMConfig` | user_config.go |
| `WorkerLLMConfig` | user_config.go |
| `ImageLLMConfig` | user_config.go |
| `DefaultImageModel` | user_config.go |
| `IsImageGenerationModel` | user_config.go |
| `IsImageShardType` | user_config.go |

### Engine resolvers on UserConfig

| Method | Returns |
|--------|---------|
| `GetEngine` / `SetEngine` | engine string |
| `HasExplicitLLMSelection` | whether file config owns LLM routing |
| `GetActiveProvider` | (provider, apiKey) |
| `GetClaudeCLIConfig` | *ClaudeCLIConfig defaults |
| `GetCodexCLIConfig` | *CodexCLIConfig defaults |
| `GetXAIOAuthConfig` | *XAIOAuthConfig defaults |
| `GetGeminiConfig` | *GeminiProviderConfig |
| `GetOllamaLLMConfig` | OllamaLLMConfig |
| `GetWorkerLLMConfig` | *WorkerLLMConfig or nil |
| `GetImageLLMConfig` | ImageLLMConfig |
| `GetEffectiveMaxConcurrentAPICalls` | int |
| `GetEffectiveAPISchedulerPolicy` | EffectiveAPISchedulerPolicy |

## 4. Limits and timeouts

| Type / func | File |
|-------------|------|
| `CoreLimits` | limits.go |
| `APISchedulerPolicy` | limits.go |
| `EffectiveAPISchedulerPolicy` | limits.go |
| `DefaultSubscriptionMinCallSpacingMs` etc. | limits.go |
| `(*Config).ValidateCoreLimits` | limits.go |
| `(*Config).EnforceCoreLimits` | limits.go |
| `(*UserConfig).GetCoreLimits` | user_config.go |
| `LLMTimeouts` | llm_timeouts.go |
| `DefaultLLMTimeouts` / `FastLLMTimeouts` / `AggressiveLLMTimeouts` | llm_timeouts.go |
| `GetLLMTimeouts` / `SetLLMTimeouts` | llm_timeouts.go |
| `(*Config).GetLLMTimeout` / `GetQueryTimeout` / `GetExecutionTimeout` / `GetSessionTTL` | config.go |

## 5. Memory, embedding, context, reflection

| Type / func | File |
|-------------|------|
| `MemoryConfig` | memory.go |
| `EmbeddingConfig` | memory.go |
| `ContextWindowConfig` | memory.go |
| `ContextWindowConfig.TotalContextWindow` | memory.go |
| `ContextWindowConfig.EffectiveInputBudget` | memory.go |
| `DefaultContextWindowConfig` | memory.go |
| `ReflectionConfig` | reflection.go |
| `DefaultReflectionConfig` | reflection.go |
| `(*UserConfig).GetContextWindowConfig` | user_config.go |
| `(*UserConfig).GetEmbeddingConfig` | user_config.go |
| `(*UserConfig).GetReflectionConfig` | user_config.go |

## 6. Shards, execution, world, build, tools

| Type / func | File |
|-------------|------|
| `ShardProfile` | shard.go |
| `(*Config).GetShardProfile` / `SetShardProfile` | config.go |
| `(*UserConfig).GetShardProfile` | user_config.go |
| `ExecutionConfig` | execution.go |
| `(*UserConfig).GetExecution` | user_config.go |
| `WorldConfig` / `DefaultWorldConfig` | world.go |
| `(*UserConfig).GetWorldConfig` | user_config.go |
| `BuildConfig` / `DefaultBuildConfig` | build.go |
| `(*UserConfig).GetBuildConfig` | user_config.go |
| `ToolGenerationConfig` / `DefaultToolGenerationConfig` | tool_generation.go |
| `(*UserConfig).GetToolGenerationConfig` | user_config.go |
| `MangleConfig` / `DefaultDerivedFactLimit` | mangle.go |

## 7. Integrations

| Type / func | File |
|-------------|------|
| `IntegrationsConfig` | integrations.go |
| `MCPServerIntegration` | integrations.go |
| `DefaultTimeout(serverID)` | integrations.go |
| `(*IntegrationsConfig).ToMCPServerConfigs` | integrations.go |
| `GetServer` / `IsServerEnabled` | integrations.go |
| `(*Config).IsMCPServerEnabled` / CodeGraph/Browser/Scraper helpers | config.go |
| `(*UserConfig).GetIntegrations` | user_config.go |

## 8. JIT, logging, UX

| Type / func | File |
|-------------|------|
| `JITConfig` / `DefaultJITConfig` | jit.go |
| `(*UserConfig).GetJITConfig` / `GetEffectiveJITConfig` | user_config.go |
| `LoggingConfig` / `IsCategoryEnabled` | logging.go |
| `(*UserConfig).GetLogging` | user_config.go |
| `UIConfig` / `DefaultUIConfig` | ux.go |
| `ExperienceLevel` constants | ux.go |
| `OnboardingState` / `DefaultOnboardingState` | ux.go |
| `TransparencyConfig` / `DefaultTransparencyConfig` | ux.go |
| `GuidanceLevel` / `GuidanceConfig` / `DefaultGuidanceConfig` | ux.go |
| `GetOnboardingState` / `GetTransparencyConfig` / `GetGuidanceConfig` | user_config.go |
| `IsOnboardingComplete` / `GetExperienceLevel` / `ShouldShowTransparency` | user_config.go |
| `GetLearningCandidateThreshold` / `GetLearningCandidateAutoPromote` | user_config.go |
| `GetContext7APIKey` / `AutoDetectContext7APIKey` | user_config.go |

## 9. Consumption guidance

Prefer **UserConfig + Get\*** for new code. Prefer **GetLLMTimeouts()** for operation deadlines rather than hardcoding durations. Prefer **GetEmbeddingConfig()** over constructing `EmbeddingConfig` literals.

## 10. Contract cautions

- **PARTIAL:** there is no full `ValidateUserConfig`; `LoadUserConfig` rejects
  unknown/trailing JSON but publishes feature side effects after syntax-only
  decode.
- **PARTIAL:** `UserConfig.Save` and `Config.Save` now use the atomic/private
  writer, but secret references, writer conflicts, rollback and platform ACL
  behavior are not a complete verified transaction.
- **PARTIAL:** effective getters do not all return deep copies. Treat returned
  maps, slices and pointers as potentially aliased until an immutable snapshot
  contract exists.
- **VERIFIED CURRENT:** the Codex backend ignores configured sandbox/shell
  relaxation, always emits read-only plus shell-disabled arguments, and admits
  only reasoning/verbosity/personality overrides. Hostile-config coverage is
  `internal/perception/codex_cli_client_test.go#TestCodexCLIClient_buildCLIArgs_FiltersEffectOverrides`.
- **VERIFIED CURRENT:** `GetActiveProvider` returns `(explicitProvider, "")` for
  a missing matching key, not `("", "")`.
