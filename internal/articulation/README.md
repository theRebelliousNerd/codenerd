# internal/articulation/

Piggyback Protocol & Prompt Assembly - Converting LLM output to structured data.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The Articulation layer sits at the exit point of the system, converting LLM output into structured data via the Piggyback Protocol. This is the counterpart to the Perception layer (NL → atoms).

## Architecture

```
LLM Response → ResponseProcessor → PiggybackEnvelope
                                        ↓
                     ┌──────────────────┴───────────────────┐
                     ↓                                      ↓
              surface_response                      control_packet
              (for user display)          (kernel state updates - hidden)
```

## Structure

```
articulation/
├── emitter.go             # Piggyback Protocol implementation
├── prompt_assembler.go    # Dynamic system prompt generation
├── schema.go              # JSON Schema for validation
├── emitter_test.go        # ResponseProcessor tests
└── prompt_assembler_test.go
```

## Key Types

### PiggybackEnvelope

```go
type PiggybackEnvelope struct {
    Control ControlPacket `json:"control_packet"`  // MUST be first (Bug #14)
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

Dual-channel steganographic communication:

| Channel | Purpose | Visibility |
|---------|---------|------------|
| `surface_response` | User-visible text | Shown to user |
| `control_packet` | Kernel state updates | Hidden from user |

**Critical:** `control_packet` MUST appear before `surface_response` in JSON to prevent "Premature Articulation" (Bug #14).

## Components

### ResponseProcessor

Parses LLM output with fallback chain:
1. Full JSON Piggyback parse
2. JSON block extraction
3. Plain text fallback

### PromptAssembler

Generates dynamic system prompts from kernel state with JIT integration.

```go
assembler := NewPromptAssemblerWithJIT(kernel, jitCompiler)
prompt, err := assembler.AssembleSystemPrompt(ctx, promptContext)
```

## Testing

```bash
go test ./internal/articulation/...
```

---

**Last Updated:** December 2024
