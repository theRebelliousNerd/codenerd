# 06 — Public API and Types: `internal/types`

> Last verified: **2026-07-13**  
> All paths under `internal/types/`.

## 1. Fact conversion

| Symbol | Kind | File | Notes |
|--------|------|------|-------|
| `MangleAtom` | type | `types.go` | Explicit name constant |
| `Fact` | struct | `types.go` | `Predicate string`, `Args []any` |
| `Fact.String` | method | `types.go` | Debug/Datalog-ish text |
| `Fact.ToAtom` | method | `types.go` | → `ast.Atom` or error |
| `KernelFact` | struct | `types.go` | Bridge shape |
| `KernelFact.ToFact` | method | `types.go` | Conversion |

## 2. Session / intent / summaries

| Symbol | File | Role |
|--------|------|------|
| `WithSessionContext` | `types.go` | Attach to `context.Context` |
| `GetSessionContext` | `types.go` | Retrieve or nil |
| `StructuredIntent` | `types.go` | Perception output shape |
| `ToolInfo` | `types.go` | Registered tool metadata |
| `ShardSummary` | `types.go` | Prior shard execution summary |
| `KnowledgeSummary` | `types.go` | Specialist knowledge handoff |
| `ToolExecutionSummary` | `types.go` | Lightweight tool run for context |
| `AmbientContext` | `types.go` | IDE cursor/selection/diagnostics |
| `SessionContext` | `types.go` | Blackboard payload |

## 3. Kernel contracts

| Symbol | File | Role |
|--------|------|------|
| `Kernel` | `interfaces.go` | Full kernel operations |
| `KernelInterface` | `types.go` | Narrow assert/query bridge |
| `KernelTransaction` | `transaction.go` | Buffered ops |
| `KernelTransactor` | `transaction.go` | `Transaction()` factory |
| `KernelTx` | `transaction.go` | Convenience wrapper |
| `NewKernelTx` | `transaction.go` | Requires `KernelTransactor` or panics |

### `Kernel` methods

`LoadFacts`, `Query`, `QueryAll`, `Assert`, `AssertBatch`, `Retract`, `RetractFact`, `UpdateSystemFacts`, `GetProgramInfo`, `Reset`, `AppendPolicy`, `RetractExactFactsBatch`, `RemoveFactsByPredicateSet`.

### `KernelInterface` methods

`AssertFact`, `AssertFactBatch`, `QueryPredicate`, `QueryBool`, `RetractFact`.

## 4. LLM contracts

| Symbol | File |
|--------|------|
| `LLMClient` | `interfaces.go` |
| `Message` | `interfaces.go` |
| `ToolResult` | `interfaces.go` |
| `ToolResultsProvider` | `interfaces.go` |
| `ToolDefinition` | `interfaces.go` |
| `ToolCall` | `interfaces.go` |
| `UsageMetadata` | `interfaces.go` |
| `LLMToolResponse` | `interfaces.go` |
| `GroundingProvider` | `interfaces.go` |
| `GroundingController` | `interfaces.go` |
| `PiggybackToolProvider` | `interfaces.go` |
| `ThinkingProvider` | `interfaces.go` |
| `ThoughtSignatureProvider` | `interfaces.go` |
| `FileProvider` | `interfaces.go` |
| `CacheProvider` | `interfaces.go` |
| `TokenCounter` | `interfaces.go` |

## 5. Shard contracts

| Symbol | File |
|--------|------|
| `ShardAgent` | `interfaces.go` |
| `ShardFactory` | `interfaces.go` |
| `PromptLoaderFunc` | `interfaces.go` |
| `JITDBRegistrar` / `JITDBUnregistrar` | `interfaces.go` |
| `ReviewerFeedbackProvider` | `interfaces.go` |
| `LimitsEnforcer` | `interfaces.go` |
| `ShardLearning` | `interfaces.go` |
| `LearningStore` | `interfaces.go` |
| `ShardType` + consts | `shard.go` |
| `ShardState` + consts | `shard.go` |
| `ShardPermission` + consts | `shard.go` |
| `ModelCapability` + consts | `shard.go` |
| `ModelConfig` | `shard.go` |
| `StartupMode` + consts | `shard.go` |
| `ShardConfig` | `shard.go` |
| `ShardResult` | `shard.go` |
| `ShardInfo` | `shard.go` |
| `SpawnPriority` + consts + `String` | `shard.go` |
| `CtxKeyPriority` | `shard.go` |
| `CtxKeyModelCapability` | `shard.go` |
| `CtxKeyModelName` | `shard.go` |

### Permission constants

`PermissionReadFile`, `PermissionWriteFile`, `PermissionExecCmd`, `PermissionNetwork`, `PermissionBrowser`, `PermissionCodeGraph`, `PermissionAskUser`, `PermissionResearch`.

## 6. Marker / query interfaces

| Symbol | File | Role |
|--------|------|------|
| `VirtualStore` | `interfaces.go` | `ReadFile`, `WriteFile`, `Exec`, `ReadRaw` |
| `GraphQuery` | `interfaces.go` | `QueryGraph(queryType, params)` |

## 7. Extract API

| Symbol | File |
|--------|------|
| `ExtractString`, `ExtractName` | `extract.go` |
| `ExtractInt64`, `ExtractFloat64` | `extract.go` |
| `ExtractBool`, `ExtractTime`, `ExtractDuration` | `extract.go` |
| `ArgString`, `ArgName`, `ArgInt64`, `ArgFloat64` | `extract.go` |
| `StripAtomPrefix` | `extract.go` |

## 8. Intentionally unexported

| Symbol | File | Role |
|--------|------|------|
| `sessionContextKeyType` / `sessionContextKey` | `types.go` | Typed context key |
| `isValidMangleNameConstant` | `types.go` | Name heuristic |
| `hasFileExtension` | `types.go` | Path vs atom |

## 9. External types referenced in signatures

| External type | Package | Where used |
|---------------|---------|------------|
| `analysis.ProgramInfo` | `mangle-go/analysis` | `Kernel.GetProgramInfo` |
| `ast.Atom` | `mangle-go/ast` | `Fact.ToAtom` return |
| `context.Context` | stdlib | LLM, shards, session helpers |
