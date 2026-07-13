# articulation — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/articulation/` (complete internal coverage)
> **Implementation: `internal/articulation/` — 8 non-test .go, 7 tests, 0 .mg**


## 1. Purpose

Atoms→NL emission, Piggyback protocol, prompt assembly bridge

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/articulation/` | Primary implementation |
| `Docs/architecture/articulation/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (8 src / 7 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/articulation/prompt_assembler.go` | 1164 | source |
| `internal/articulation/emitter.go` | 1103 | source |
| `internal/articulation/schema.go` | 240 | source |
| `internal/articulation/protocol_types.go` | 239 | source |
| `internal/articulation/kernel_context.go` | 128 | source |
| `internal/articulation/stream_parser.go` | 109 | source |
| `internal/articulation/prompt_assembler_adapter.go` | 108 | source |
| `internal/articulation/json_scanner.go` | 105 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `ResponseProcessor` | `internal/articulation/emitter.go:171` |
| `ProcessorStats` | `internal/articulation/emitter.go:183` |
| `ArticulationResult` | `internal/articulation/emitter.go:192` |
| `Emitter` | `internal/articulation/emitter.go:722` |
| `ConstitutionalOverride` | `internal/articulation/emitter.go:847` |
| `ProcessedLLMResponse` | `internal/articulation/emitter.go:1010` |
| `KernelQuerier` | `internal/articulation/prompt_assembler.go:26` |
| `PromptContext` | `internal/articulation/prompt_assembler.go:31` |
| `PromptAssembler` | `internal/articulation/prompt_assembler.go:49` |
| `PromptAssemblerAdapter` | `internal/articulation/prompt_assembler_adapter.go:17` |
| `PiggybackEnvelope` | `internal/articulation/protocol_types.go:19` |
| `ControlPacket` | `internal/articulation/protocol_types.go:25` |
| `ToolRequest` | `internal/articulation/protocol_types.go:83` |
| `KnowledgeRequest` | `internal/articulation/protocol_types.go:143` |
| `IntentClassification` | `internal/articulation/protocol_types.go:157` |
| `MemoryOperation` | `internal/articulation/protocol_types.go:208` |
| `SelfCorrection` | `internal/articulation/protocol_types.go:215` |
| `ContextFeedback` | `internal/articulation/protocol_types.go:224` |
| `StreamParser` | `internal/articulation/stream_parser.go:9` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewResponseProcessor` | `internal/articulation/emitter.go:209` |
| `Process` | `internal/articulation/emitter.go:224` |
| `GetStats` | `internal/articulation/emitter.go:704` |
| `ResetStats` | `internal/articulation/emitter.go:712` |
| `NewEmitter` | `internal/articulation/emitter.go:731` |
| `Emit` | `internal/articulation/emitter.go:743` |
| `EmitSurface` | `internal/articulation/emitter.go:778` |
| `ParseAndProcess` | `internal/articulation/emitter.go:786` |
| `CreateEnvelope` | `internal/articulation/emitter.go:799` |
| `MarshalEnvelope` | `internal/articulation/emitter.go:818` |
| `ApplyConstitutionalOverride` | `internal/articulation/emitter.go:856` |
| `AppendReasoningDirective` | `internal/articulation/emitter.go:934` |
| `ExtractSurfaceOnly` | `internal/articulation/emitter.go:949` |
| `HasSelfCorrection` | `internal/articulation/emitter.go:970` |
| `HasMemoryOperations` | `internal/articulation/emitter.go:980` |
| `GetMemoryOperationsByType` | `internal/articulation/emitter.go:989` |
| `ProcessLLMResponse` | `internal/articulation/emitter.go:1028` |
| `ProcessLLMResponseAllowPlain` | `internal/articulation/emitter.go:1064` |
| `MustExtractSurface` | `internal/articulation/emitter.go:1100` |
| `GetKernelContext` | `internal/articulation/kernel_context.go:20` |
| `BuildContextSection` | `internal/articulation/kernel_context.go:35` |
| `NewPromptAssembler` | `internal/articulation/prompt_assembler.go:67` |
| `NewPromptAssemblerWithJIT` | `internal/articulation/prompt_assembler.go:80` |
| `AssembleSystemPrompt` | `internal/articulation/prompt_assembler.go:329` |
| `AssembleQuickPrompt` | `internal/articulation/prompt_assembler.go:1037` |
| `WithSessionContext` | `internal/articulation/prompt_assembler.go:1052` |
| `WithIntent` | `internal/articulation/prompt_assembler.go:1058` |
| `WithCampaign` | `internal/articulation/prompt_assembler.go:1064` |
| `WithSemanticQuery` | `internal/articulation/prompt_assembler.go:1070` |
| `JITReady` | `internal/articulation/prompt_assembler.go:1085` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Bridge |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
