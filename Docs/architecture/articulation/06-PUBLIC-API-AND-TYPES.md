# 06 — Public API and Types: articulation

> Last verified: 2026-07-13  
> Package: `codenerd/internal/articulation`  
> Only **exported** symbols that matter to integrators are listed.

## Protocol types (`protocol_types.go`)

### `PiggybackEnvelope`

| Field | JSON | Notes |
|-------|------|-------|
| `Control` | `control_packet` | Must be first in protocol |
| `Surface` | `surface_response` | Required non-empty for successful parse |

### `ControlPacket`

| Field | JSON | Notes |
|-------|------|-------|
| `IntentClassification` | `intent_classification` | Category/verb/target/constraint/confidence |
| `MangleUpdates` | `mangle_updates` | Tolerates single string via custom unmarshal |
| `MemoryOperations` | `memory_operations` | Cold storage directives |
| `SelfCorrection` | `self_correction` | Optional pointer |
| `ReasoningTrace` | `reasoning_trace` | Optional |
| `KnowledgeRequests` | `knowledge_requests` | Specialist / research |
| `ContextFeedback` | `context_feedback` | Usefulness signal |
| `ToolRequests` | `tool_requests` | Structured tools |

### Nested types

- `ToolRequest` — `id`, `tool_name`, `tool_args`, `purpose`, `required` (bool tolerant of string/int)  
- `KnowledgeRequest` — `specialist`, `query`, `purpose`, `priority`  
- `IntentClassification` — confidence accepts float or high/medium/low strings  
- `MemoryOperation` — `op`, `key`, `value` (`promote_to_long_term|forget|store_vector|note`)  
- `SelfCorrection` — `triggered`, `hypothesis`  
- `ContextFeedback` — usefulness + helpful/noise facts + missing_context  

## Processing API (`emitter.go`)

### `ResponseProcessor`

```go
NewResponseProcessor() *ResponseProcessor
(*ResponseProcessor) Process(raw string) (*ArticulationResult, error)
(*ResponseProcessor) GetStats() ProcessorStats
(*ResponseProcessor) ResetStats()
```

Configurable fields: `RequireValidJSON`, `AllowMarkdownWrapped`, `MaxSurfaceLength`, `LogFallbackAsError`.

### Results

- `ArticulationResult` — Surface, Control, ParseMethod, Confidence, Warnings, RawResponse  
- `ProcessorStats` — TotalProcessed, SuccessfulParses, FallbackParses, ValidationFailures, SelfCorrections  
- `ProcessedLLMResponse` — Surface, *Control, ParseMethod, Confidence  

### Convenience

| Func | Behavior |
|------|----------|
| `ProcessLLMResponse` | Tolerant parse; fallback logs error; Control nil on fallback |
| `ProcessLLMResponseAllowPlain` | Same without error-level fallback log |
| `MustExtractSurface` | Surface only via ProcessLLMResponse |
| `ExtractSurfaceOnly` | New processor, return surface or raw |

### Emitter

```go
NewEmitter() *Emitter
Emit, EmitSurface, ParseAndProcess, CreateEnvelope, MarshalEnvelope
```

### Safety / helpers

```go
ApplyConstitutionalOverride(envelope *PiggybackEnvelope, blocked []string, reason string) *ConstitutionalOverride
HasSelfCorrection(envelope PiggybackEnvelope) bool
HasMemoryOperations(envelope PiggybackEnvelope) bool
GetMemoryOperationsByType(envelope PiggybackEnvelope, opType string) []MemoryOperation
AppendReasoningDirective(systemPrompt string, full bool) string
```

### Constants

- `ReasoningTraceDirective`  
- `ShardReasoningDirective`  

## Schema (`schema.go`)

| Symbol | Role |
|--------|------|
| `PiggybackEnvelopeSchema` | Full draft-07 schema string |
| `SimpleEnvelopeSchema` | Minimal required pair |
| `GetPiggybackSchema(strict bool) string` | Selector |

## Streaming (`stream_parser.go`)

```go
NewStreamParser() *StreamParser
(*StreamParser) ProcessChunk(chunk string) string
(*StreamParser) GetFullBuffer() string
```

## Prompt assembly (`prompt_assembler.go`, adapter, kernel_context)

### Interfaces / context

```go
type KernelQuerier interface {
    Query(predicate string) ([]types.Fact, error)
}

type PromptContext struct {
    ShardID, ShardType, CampaignID string
    SessionCtx *types.SessionContext
    UserIntent *types.StructuredIntent
    SemanticQuery string
    SemanticTopK int
}
```

Builders: `WithSessionContext`, `WithIntent`, `WithCampaign`, `WithSemanticQuery`.

### Constructors

```go
NewPromptAssembler(kernel KernelQuerier) (*PromptAssembler, error)
NewPromptAssemblerWithJIT(kernel KernelQuerier, jit *prompt.JITPromptCompiler) (*PromptAssembler, error)
NewPromptAssemblerAdapter(assembler *PromptAssembler) *PromptAssemblerAdapter
AssembleQuickPrompt(ctx, kernel, shardID, shardType) (string, error)
GetKernelContext(kernel, shardID) (string, error)
```

### Methods

| Method | Role |
|--------|------|
| `AssembleSystemPrompt(ctx, input any)` | Main entry; `*PromptContext` or `map[string]any` |
| `BuildContextSection(shardID)` | Injectable + specialist sections |
| `JITReady` / `EnableJIT` / `SetJITCompiler` / `GetJITCompiler` / `IsJITEnabled` | JIT control |
| `SetJITBudgets(...)` | Token/top-k overrides |
| Adapter `AssembleSystemPrompt(ctx, shardID, shardType)` | Perception simplified API |
| Adapter `JITReady()` | Feature probe |

### Protocol string

- `PiggybackProtocolSuffix` — mandatory output protocol for user-facing shards  

## Parse method string contract

Callers should treat these as stable:

| `ParseMethod` | Meaning |
|---------------|---------|
| `json` | Direct JSON |
| `json_markdown` | Fenced markdown |
| `json_extracted` | Embedded candidate |
| `fallback` | Plain / salvage; no trusted control |
| `unknown` | Initial placeholder (should not ship) |

## Integration snippets (patterns, not prescriptions)

**Shard / executor:**

```go
processed := articulation.ProcessLLMResponse(raw)
// display processed.Surface
// if processed.Control != nil { route tools / mangle }
```

**Chat stream:**

```go
parser := articulation.NewStreamParser()
delta := parser.ProcessChunk(chunk)
// ... after stream
result, err := articulation.NewResponseProcessor().Process(parser.GetFullBuffer())
```

**Boot assembler:**

```go
pa, err := articulation.NewPromptAssemblerWithJIT(kernel, jitCompiler)
// or NewPromptAssembler + SetJITCompiler + SetJITBudgets + EnableJIT
transducer.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(pa))
```
