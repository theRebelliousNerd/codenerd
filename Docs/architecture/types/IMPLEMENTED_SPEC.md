# types — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/types/` (complete internal coverage)
> **Implementation: `internal/types/` — 5 non-test .go, 4 tests, 0 .mg**


## 1. Purpose

Shared type definitions used across packages

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/types/` | Primary implementation |
| `Docs/architecture/types/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (5 src / 4 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/types/types.go` | 455 | source |
| `internal/types/interfaces.go` | 379 | source |
| `internal/types/extract.go` | 210 | source |
| `internal/types/shard.go` | 157 | source |
| `internal/types/transaction.go` | 85 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `Kernel` | `internal/types/interfaces.go:10` |
| `LLMClient` | `internal/types/interfaces.go:36` |
| `Message` | `internal/types/interfaces.go:48` |
| `ToolResult` | `internal/types/interfaces.go:58` |
| `ToolResultsProvider` | `internal/types/interfaces.go:77` |
| `ToolDefinition` | `internal/types/interfaces.go:86` |
| `ToolCall` | `internal/types/interfaces.go:93` |
| `UsageMetadata` | `internal/types/interfaces.go:100` |
| `LLMToolResponse` | `internal/types/interfaces.go:109` |
| `ShardAgent` | `internal/types/interfaces.go:129` |
| `ShardFactory` | `internal/types/interfaces.go:143` |
| `PromptLoaderFunc` | `internal/types/interfaces.go:146` |
| `JITDBRegistrar` | `internal/types/interfaces.go:149` |
| `JITDBUnregistrar` | `internal/types/interfaces.go:152` |
| `ReviewerFeedbackProvider` | `internal/types/interfaces.go:155` |
| `LimitsEnforcer` | `internal/types/interfaces.go:164` |
| `ShardLearning` | `internal/types/interfaces.go:171` |
| `LearningStore` | `internal/types/interfaces.go:180` |
| `VirtualStore` | `internal/types/interfaces.go:187` |
| `GraphQuery` | `internal/types/interfaces.go:200` |
| `GroundingProvider` | `internal/types/interfaces.go:215` |
| `GroundingController` | `internal/types/interfaces.go:234` |
| `PiggybackToolProvider` | `internal/types/interfaces.go:259` |
| `ThinkingProvider` | `internal/types/interfaces.go:279` |
| `ThoughtSignatureProvider` | `internal/types/interfaces.go:322` |
| `FileProvider` | `internal/types/interfaces.go:334` |
| `CacheProvider` | `internal/types/interfaces.go:353` |
| `TokenCounter` | `internal/types/interfaces.go:376` |
| `ShardType` | `internal/types/shard.go:12` |
| `ShardState` | `internal/types/shard.go:22` |
| `ShardPermission` | `internal/types/shard.go:32` |
| `ModelCapability` | `internal/types/shard.go:46` |
| `ModelConfig` | `internal/types/shard.go:55` |
| `StartupMode` | `internal/types/shard.go:61` |
| `ShardConfig` | `internal/types/shard.go:71` |
| `ShardResult` | `internal/types/shard.go:99` |
| `ShardInfo` | `internal/types/shard.go:107` |
| `SpawnPriority` | `internal/types/shard.go:115` |
| `KernelTransaction` | `internal/types/transaction.go:8` |
| `KernelTransactor` | `internal/types/transaction.go:29` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `ExtractString` | `internal/types/extract.go:32` |
| `ExtractName` | `internal/types/extract.go:67` |
| `ExtractInt64` | `internal/types/extract.go:80` |
| `ExtractFloat64` | `internal/types/extract.go:97` |
| `ExtractBool` | `internal/types/extract.go:115` |
| `ExtractTime` | `internal/types/extract.go:143` |
| `ExtractDuration` | `internal/types/extract.go:157` |
| `ArgString` | `internal/types/extract.go:171` |
| `ArgName` | `internal/types/extract.go:180` |
| `ArgInt64` | `internal/types/extract.go:189` |
| `ArgFloat64` | `internal/types/extract.go:198` |
| `StripAtomPrefix` | `internal/types/extract.go:208` |
| `String` | `internal/types/shard.go:132` |
| `NewKernelTx` | `internal/types/transaction.go:40` |
| `Retract` | `internal/types/transaction.go:50` |
| `RetractFact` | `internal/types/transaction.go:55` |
| `RetractExactFact` | `internal/types/transaction.go:60` |
| `RetractPredicateSet` | `internal/types/transaction.go:65` |
| `Assert` | `internal/types/transaction.go:70` |
| `LoadFacts` | `internal/types/transaction.go:76` |
| `Commit` | `internal/types/transaction.go:83` |
| `WithSessionContext` | `internal/types/types.go:27` |
| `GetSessionContext` | `internal/types/types.go:32` |
| `String` | `internal/types/types.go:111` |
| `ToAtom` | `internal/types/types.go:146` |
| `ToFact` | `internal/types/types.go:257` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
