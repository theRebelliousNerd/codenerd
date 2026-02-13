# internal/articulation - Piggyback Protocol & Prompt Assembly

This package implements the Articulation layer: LLM output → structured data extraction via the Piggyback Protocol, and dynamic system prompt assembly from kernel state.

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - Kernel state for prompt assembly
- [internal/prompt](../prompt/CLAUDE.md) - JIT compiler for context-aware prompts
- [internal/perception](../perception/CLAUDE.md) - Transducer (NL→atoms counterpart)

## Architecture

The articulation package provides:
- **Piggyback Protocol**: Dual-channel steganographic control (surface + control packet)
- **ResponseProcessor**: LLM output parsing with fallback chain
- **PromptAssembler**: Dynamic system prompt generation with JIT integration

## File Index

| File | Description |
|------|-------------|
| `emitter.go` | Piggyback Protocol implementation with dual-channel JSON schema. Exports `PiggybackEnvelope` (control_packet first per Bug #14), `ControlPacket`, `IntentClassification`, `MemoryOperation`, `SelfCorrection`, and `ResponseProcessor` with fallback parsing. |
| `prompt_assembler.go` | Dynamic system prompt generation from kernel state with JIT integration. Exports `PromptAssembler`, `PromptContext`, `KernelQuerier` interface, `NewPromptAssembler()`, and `NewPromptAssemblerWithJIT()` for context-aware prompt compilation. |
| `schema.go` | JSON Schema constant for Piggyback Protocol validation with Claude CLI. Exports `PiggybackEnvelopeSchema` for structured output guarantees via --json-schema flag. |
| `emitter_test.go` | Unit tests for ResponseProcessor parsing and Piggyback envelope extraction. Tests fallback parsing chain and self-correction detection. |
| `prompt_assembler_test.go` | Unit tests for PromptAssembler with mock kernel and JIT compiler. Tests context conversion and system prompt generation. |

## Key Types

### PiggybackEnvelope
```go
type PiggybackEnvelope struct {
    Control ControlPacket `json:"control_packet"` // MUST be first (Bug #14)
    Surface string        `json:"surface_response"`
}

type ControlPacket struct {
    IntentClassification IntentClassification `json:"intent_classification"`
    MangleUpdates        []string             `json:"mangle_updates"`
    MemoryOperations     []MemoryOperation    `json:"memory_operations"`
    SelfCorrection       *SelfCorrection      `json:"self_correction,omitempty"`
    ReasoningTrace       string               `json:"reasoning_trace,omitempty"`
}
```

### PromptContext
```go
type PromptContext struct {
    ShardID    string
    ShardType  string
    SessionCtx *types.SessionContext
    UserIntent *types.StructuredIntent
    CampaignID string
}
```

## Piggyback Protocol

The dual-channel protocol separates:
- **Surface Response**: User-visible text
- **Control Packet**: Kernel state updates (hidden from user)

Key principle: control_packet MUST appear before surface_response in JSON to prevent "Premature Articulation" (Bug #14).

## Dependencies

- `internal/prompt` - JIT compiler integration
- `internal/types` - SessionContext, StructuredIntent
- `internal/logging` - Structured logging

## Testing

```bash
go test ./internal/articulation/...
```

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*