# articulation — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/articulation/` (8 non-test .go, 7 tests, 0 .mg)**


## 1. Purpose

Atoms→NL, Piggyback emitter, prompt assembly bridge

## 2. Source paths

| Path | Role |
|------|------|
| `internal/articulation/` | Primary implementation |
| `Docs/architecture/articulation/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 85% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Bridge |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
