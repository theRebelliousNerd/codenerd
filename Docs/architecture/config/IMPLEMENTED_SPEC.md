# config — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/config/` (complete internal coverage)
> **Implementation: `internal/config/` — 17 non-test .go, 5 tests, 0 .mg**


## 1. Purpose

Configuration loading, engines, limits, user and memory config

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/config/` | Primary implementation |
| `Docs/architecture/config/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **70%** |
| Exported types (sampled) | Implemented | **70%** |
| Tests | Implemented | **70%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 70%** as living package (17 src / 5 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/config/user_config.go` | 1408 | source |
| `internal/config/config.go` | 446 | source |
| `internal/config/llm.go` | 211 | source |
| `internal/config/llm_timeouts.go` | 179 | source |
| `internal/config/ux.go` | 155 | source |
| `internal/config/memory.go` | 130 | source |
| `internal/config/limits.go` | 100 | source |
| `internal/config/integrations.go` | 87 | source |
| `internal/config/jit.go` | 76 | source |
| `internal/config/shard.go` | 56 | source |
| `internal/config/reflection.go` | 53 | source |
| `internal/config/world.go` | 41 | source |
| `internal/config/logging.go` | 32 | source |
| `internal/config/build.go` | 25 | source |
| `internal/config/execution.go` | 16 | source |
| `internal/config/tool_generation.go` | 15 | source |
| `internal/config/mangle.go` | 13 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `BuildConfig` | `internal/config/build.go:6` |
| `Config` | `internal/config/config.go:15` |
| `ExecutionConfig` | `internal/config/execution.go:4` |
| `IntegrationsConfig` | `internal/config/integrations.go:7` |
| `MCPServerIntegration` | `internal/config/integrations.go:14` |
| `JITConfig` | `internal/config/jit.go:8` |
| `CoreLimits` | `internal/config/limits.go:9` |
| `APISchedulerPolicy` | `internal/config/limits.go:34` |
| `EffectiveAPISchedulerPolicy` | `internal/config/limits.go:56` |
| `LLMConfig` | `internal/config/llm.go:14` |
| `ClaudeCLIConfig` | `internal/config/llm.go:29` |
| `CodexCLIConfig` | `internal/config/llm.go:60` |
| `XAIOAuthConfig` | `internal/config/llm.go:126` |
| `GeminiProviderConfig` | `internal/config/llm.go:177` |
| `LLMTimeouts` | `internal/config/llm_timeouts.go:13` |
| `LoggingConfig` | `internal/config/logging.go:4` |
| `MangleConfig` | `internal/config/mangle.go:4` |
| `MemoryConfig` | `internal/config/memory.go:4` |
| `EmbeddingConfig` | `internal/config/memory.go:20` |
| `ContextWindowConfig` | `internal/config/memory.go:55` |
| `ReflectionConfig` | `internal/config/reflection.go:6` |
| `ShardProfile` | `internal/config/shard.go:5` |
| `ToolGenerationConfig` | `internal/config/tool_generation.go:4` |
| `UserConfig` | `internal/config/user_config.go:25` |
| `OllamaLLMConfig` | `internal/config/user_config.go:495` |
| `WorkerLLMConfig` | `internal/config/user_config.go:505` |
| `UIConfig` | `internal/config/ux.go:4` |
| `ExperienceLevel` | `internal/config/ux.go:22` |
| `OnboardingState` | `internal/config/ux.go:32` |
| `TransparencyConfig` | `internal/config/ux.go:59` |
| `GuidanceLevel` | `internal/config/ux.go:100` |
| `GuidanceConfig` | `internal/config/ux.go:110` |
| `WorldConfig` | `internal/config/world.go:6` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `DefaultBuildConfig` | `internal/config/build.go:19` |
| `DefaultConfig` | `internal/config/config.go:61` |
| `Load` | `internal/config/config.go:230` |
| `Save` | `internal/config/config.go:260` |
| `GetLLMTimeout` | `internal/config/config.go:346` |
| `GetQueryTimeout` | `internal/config/config.go:355` |
| `GetExecutionTimeout` | `internal/config/config.go:364` |
| `GetShardProfile` | `internal/config/config.go:377` |
| `SetShardProfile` | `internal/config/config.go:385` |
| `GetSessionTTL` | `internal/config/config.go:393` |
| `Validate` | `internal/config/config.go:405` |
| `IsMCPServerEnabled` | `internal/config/config.go:429` |
| `IsCodeGraphEnabled` | `internal/config/config.go:434` |
| `IsBrowserEnabled` | `internal/config/config.go:439` |
| `IsScraperEnabled` | `internal/config/config.go:444` |
| `DefaultTimeout` | `internal/config/integrations.go:24` |
| `ToMCPServerConfigs` | `internal/config/integrations.go:36` |
| `GetServer` | `internal/config/integrations.go:73` |
| `IsServerEnabled` | `internal/config/integrations.go:84` |
| `UnmarshalJSON` | `internal/config/jit.go:40` |
| `DefaultJITConfig` | `internal/config/jit.go:65` |
| `ValidateCoreLimits` | `internal/config/limits.go:74` |
| `EnforceCoreLimits` | `internal/config/limits.go:92` |
| `DefaultGeminiProviderConfig` | `internal/config/llm.go:203` |
| `DefaultLLMTimeouts` | `internal/config/llm_timeouts.go:84` |
| `FastLLMTimeouts` | `internal/config/llm_timeouts.go:115` |
| `AggressiveLLMTimeouts` | `internal/config/llm_timeouts.go:142` |
| `GetLLMTimeouts` | `internal/config/llm_timeouts.go:171` |
| `SetLLMTimeouts` | `internal/config/llm_timeouts.go:177` |
| `IsCategoryEnabled` | `internal/config/logging.go:20` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Owner |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
