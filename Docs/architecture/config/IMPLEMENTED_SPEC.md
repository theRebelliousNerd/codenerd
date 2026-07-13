# config — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/config/` (17 non-test .go, 4 tests, 0 .mg)**


## 1. Purpose

Config loading, engines, limits, user config

## 2. Source paths

| Path | Role |
|------|------|
| `internal/config/` | Primary implementation |
| `Docs/architecture/config/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **70%** |
| Exported types (sampled) | Implemented | **70%** |
| Tests | Implemented | **70%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 70% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

| Path | Lines |
|------|------:|
| `internal/config/user_config.go` | 1322 | source |
| `internal/config/config.go` | 446 | source |
| `internal/config/llm.go` | 211 | source |
| `internal/config/llm_timeouts.go` | 179 | source |
| `internal/config/ux.go` | 155 | source |
| `internal/config/memory.go` | 130 | source |
| `internal/config/limits.go` | 100 | source |
| `internal/config/integrations.go` | 87 | source |
| `internal/config/jit.go` | 76 | source |
| `internal/config/reflection.go` | 53 | source |
| `internal/config/shard.go` | 53 | source |
| `internal/config/world.go` | 41 | source |

### Sampled types

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
| `UserConfig` | `internal/config/user_config.go:24` |
| `UIConfig` | `internal/config/ux.go:4` |

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Owner |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
