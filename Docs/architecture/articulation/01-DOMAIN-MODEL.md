# articulation — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/articulation/` (complete internal coverage)
> **Implementation: `internal/articulation/` — 8 non-test .go, 7 tests, 0 .mg**


## Package

`internal/articulation/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Atoms→NL emission, Piggyback protocol, prompt assembly bridge**
