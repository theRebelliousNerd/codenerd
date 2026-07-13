# 05 — Internal Architecture: articulation

> Last verified: 2026-07-13  
> Sources: all files under `internal/articulation/`.

## Component map

```
┌─────────────────────────────────────────────────────────────────┐
│                     internal/articulation                        │
│                                                                  │
│  protocol_types.go     schema.go                                 │
│  (envelope types)      (JSON Schema constants)                   │
│           │                  │                                   │
│           ▼                  ▼                                   │
│  ┌───────────────── ResponseProcessor (emitter.go) ───────────┐  │
│  │ Process → parseJSON → markdown → extractEmbedded → fallback │  │
│  │ applyCaps → ArticulationResult                              │  │
│  └──────────────┬───────────────────────────┬──────────────────┘  │
│                 │                           │                    │
│                 ▼                           ▼                    │
│         StreamParser                 ProcessLLMResponse*         │
│         (surface stream)             (shard convenience)         │
│                                                                  │
│  json_scanner.go ── findJSONCandidates (depth/size caps)         │
│                                                                  │
│  ┌────────────── PromptAssembler (prompt_assembler.go) ────────┐ │
│  │ AssembleSystemPrompt                                        │ │
│  │   JIT? → JITPromptCompiler.Compile                          │ │
│  │   else → template/baseline + context + session + intent     │ │
│  │ PiggybackProtocolSuffix / fallback templates                │ │
│  └──────────────┬──────────────────────────────────────────────┘ │
│                 │                                                │
│  adapter + mapToPromptContext │ kernel_context helpers           │
│  Emitter / constitutional override / reasoning directives        │
└─────────────────────────────────────────────────────────────────┘
```

## Inbound data flow (LLM response)

```
rawResponse: string
        │
        ▼
ResponseProcessor.Process
        │
        ├─1─ parseJSON (trim, optional unknown-field check, decode)
        │       require non-empty surface_response
        │       strict: intent category+verb
        │
        ├─2─ parseMarkdownWrappedJSON (strip ``` fences)
        │
        ├─3─ extractEmbeddedJSON
        │       findJSONCandidates
        │       pass1: last candidate with surface+control keys
        │       pass2: other candidates last→first
        │
        └─4─ fallback (if !RequireValidJSON)
                looksLikePartialEnvelope?
                  yes → salvageSurfaceFromPartial OR truncatedEnvelopeMessage
                  no  → raw as surface
                Control empty
        │
        ▼
applyCaps (surface, mangle, memory, tools, knowledge, reasoning)
        │
        ▼
ArticulationResult { Surface, Control, ParseMethod, Confidence, Warnings, Raw }
```

### ProcessLLMResponse wrapper semantics

```
ProcessLLMResponse(raw)
  → Process (fallback allowed, LogFallbackAsError=true)
  → if method != "fallback": Control pointer set
  → else Control = nil

ProcessLLMResponseAllowPlain(raw)
  → same but LogFallbackAsError=false  (structured-only shards, intentional plain text)
```

## StreamParser state machine

```
buffer accumulates chunks
        │
        ▼
inSurface == false?
  find `"surface_response"` then `:` then opening `"`
  set lastEmittedIndex after opening quote
        │
inSurface == true
  emit unescaped characters until unescaped `"`
  escapes: \n \r \t \" \\
        │
        ▼
returned delta string → TUI streamChan
full JSON remains in buffer for final ResponseProcessor.Process
```

Not concurrency-safe; one parser per stream.

## Outbound data flow (prompt assembly)

```
input: *PromptContext | map[string]any
        │
        ▼
mapToPromptContext if map
        │
useJIT && jitCompiler?
  yes → toCompilationContext → Compile
         on success: maybe append PiggybackProtocolSuffix
         on fail: assert jit_fallback fact; fall through
  no  → legacy:
         shard_prompt_base? else AssembleEmbeddedBaselinePrompt?
         else getFallbackTemplate
         + injectable_context atoms
         + buildSessionContext
         + buildIntentContext
         + Piggyback suffix if legacy template & not structured-only
        │
        ▼
system prompt string
```

### `toCompilationContext` mappings (high signal)

| From | To (`prompt.CompilationContext`) |
|------|----------------------------------|
| `ShardType` | `"/"+type` |
| stable `ShardID` | strip trailing `-<digits>` instance suffix |
| `SessionCtx.DreamMode` | `/dream` |
| `TestState == /failing` | `/tdd_repair` else `/active` |
| diagnostics / failing tests counts | budget selectors |
| ExtraContext keys | build_layer, init_phase, frameworks, language, … |
| UserIntent | verb/target + SemanticQuery |
| legislator / mangle_repair | tighter token budgets |
| autopoiesis / mangle shards | force `/mangle` language when empty |

## Key type relationships

```
PiggybackEnvelope
  ├── ControlPacket
  │     ├── IntentClassification
  │     ├── []string MangleUpdates
  │     ├── []MemoryOperation
  │     ├── *SelfCorrection
  │     ├── ReasoningTrace string
  │     ├── []KnowledgeRequest
  │     ├── *ContextFeedback
  │     └── []ToolRequest
  └── Surface string

ArticulationResult  ≈ envelope + parse metadata
ProcessedLLMResponse ≈ surface + *Control + method/confidence
ConstitutionalOverride ≈ audit record of surface rewrite + blocked atoms
PromptAssembler ── KernelQuerier
               └── *prompt.JITPromptCompiler (optional)
```

## Emitter path (secondary)

`Emitter` wraps a processor for:

- pretty/compact JSON stdout emit  
- surface-only emit  
- create/marshal envelopes  

Used by CLI instruction paths; primary runtime uses `ResponseProcessor` / convenience funcs directly.

## Kernel query surface (read-only from this package)

| Predicate pattern | Purpose |
|-------------------|---------|
| `shard_prompt_base` | Per-type baseline template string |
| `injectable_context(shard\|*\|/_all, atom)` | Spreading-activation context lines |
| `specialist_knowledge(shard, topic, content)` | Type B knowledge blocks (`BuildContextSection`) |
| `jit_fallback(shardType, reason)` | Written via compiler `AssertFacts` on JIT failure |

## Schema vs runtime tolerance

| Mode | Behavior |
|------|----------|
| Default processor | Unknown fields allowed by decoder; tolerant unmarshals |
| `RequireValidJSON` | `DisallowUnknownFields` + recursive `schemaAllowedKeys` check + required intent fields |
| Provider schema | `PiggybackEnvelopeSchema` for Claude/schema-capable clients |

## Concurrency model

| Object | Thread-safety |
|--------|----------------|
| `ResponseProcessor` | Per-call stats mutation; not documented as concurrent-safe on one instance |
| `PromptAssembler` | `sync.RWMutex` around JIT fields; budget getters use lock |
| `StreamParser` | Single-threaded |
| Package-level constants | Immutable |

## Error philosophy

- Prefer return values over panic.  
- Fallback path returns `nil` error with warnings (non-strict).  
- Strict path returns wrapped `failed to parse Piggyback JSON`.  
- Prompt assembly treats missing context atoms as non-fatal warnings.
