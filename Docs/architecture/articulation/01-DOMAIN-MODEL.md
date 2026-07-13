# articulation — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/articulation/` (8 non-test .go, 7 tests, 0 .mg)**


## Source package

`internal/articulation/`

## Exported / primary types (sampled)

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

This package's position: **Atoms→NL, Piggyback emitter, prompt assembly bridge**

## Data & control concepts

- Primary language surface: Go under `internal/articulation/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
