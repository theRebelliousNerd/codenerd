# config — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/config/` (17 non-test .go, 4 tests, 0 .mg)**


## Source package

`internal/config/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 0 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| — | 0 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Config loading, engines, limits, user config**

## Data & control concepts

- Primary language surface: Go under `internal/config/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
