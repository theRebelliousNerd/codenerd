# 08 — Wiring and Integration: articulation

> Last verified: 2026-07-13  
> How articulation is **registered and called** at runtime.

## 1. Cortex boot (`internal/system/factory.go`)

During Cortex assembly, after JIT compiler exists:

1. `articulation.NewPromptAssembler(kernel)`  
2. `pa.SetJITCompiler(jitCompiler)`  
3. `pa.SetJITBudgets(tokenBudget, reservedTokens, semanticTopK, fallbackRatio)` from JIT config  
4. `pa.EnableJIT(jitCfg.Enabled)`  
5. Store on boot context / Cortex: `PromptAssembler *articulation.PromptAssembler`  
6. `transducer.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(pa))`  
7. `poiesis.SetPromptAssembler(pa)` when present  
8. System/campaign shards receive adapter via `SetPromptAssembler(...)`

This is the **canonical** long-lived assembler for the process.

## 2. Chat session boot (`cmd/nerd/chat/session_boot.go`)

Interactive chat mirrors factory wiring:

- Builds `PromptAssemblerWithJIT(kernel, jitCompiler)` when available.  
- Injects adapter into the perception transducer.  
- Chat model holds kernel/JIT for campaign JIT providers that construct assemblers on demand.

## 3. Chat articulation phase (`cmd/nerd/chat/helpers_articulation.go`)

Primary user-facing response path:

```
articulateWithConversation
  → build system + conversation + findings prompt
  → stream: CompleteWithStreaming + StreamParser.ProcessChunk → streamChan
  → non-stream: CompleteWithSystem
  → NewResponseProcessor().Process(fullRaw)
  → ArticulationOutput {
        Surface, Envelope, MemoryOperations, MangleUpdates,
        SelfCorrection, KnowledgeRequests, ContextFeedback, ...
    }
```

`formatResponse` returns **only** `payload.Surface` to the TUI.

Knowledge / compressor / dream paths consume control fields from `ArticulationOutput`.

## 4. Session clean executor (`internal/session/executor.go`)

Piggyback++ tool loop:

1. Log schema length via `GetPiggybackSchema(false)`  
2. LLM `CompleteWithSystem`  
3. `ProcessLLMResponse` → build envelope  
4. `parseToolRequestsFromControl` → tool execution  
5. `processMangleUpdatesFromEnvelope` → filter → assert; `ApplyConstitutionalOverride` on blocked  
6. Finalization: `processPiggybackControlPacket` uses `ProcessLLMResponseAllowPlain` for self-correction and memory op logging/routing  

## 5. Perception integration

| Site | Wiring |
|------|--------|
| `client_schema.go` | Parses `PiggybackEnvelopeSchema` into provider schema payload |
| `transducer.go` | Type alias `PiggybackEnvelope = articulation.PiggybackEnvelope` |
| Transducer prompt assembly | Adapter `AssembleSystemPrompt(ctx, shardID, shardType)` pulls session from context |

## 6. System shards (`internal/shards/system/*`)

- `BaseSystemShard` stores assembler as `any`; type-asserts `*articulation.PromptAssembler` when building prompts with full `PromptContext`.  
- Planner / perception / legislator / mangle_repair process LLM outputs with `ProcessLLMResponse` or equivalent.  
- Registration paths in `internal/shards/registration.go` and factory constructors inject assemblers.

## 7. Campaign JIT providers

`cmd/nerd/campaign_jit_provider.go` and `cmd/nerd/chat/campaign_jit_provider.go`:

- Hold `*articulation.PromptAssembler`  
- Build `PromptContext` for campaign phases  
- Call `AssembleSystemPrompt` for assault / campaign prompts  

## 8. CLI instruction path (`cmd/nerd/cmd_instruction.go`)

Uses classic `NewEmitter()`:

- Emit full envelopes or construct payloads for instruction workflows  
- Demonstrates surface/control separation outside the TUI  

## 9. Fact-flow diagram (articulation-centric)

```
                    ┌──────────────┐
                    │  Perception  │
                    │  user_intent │
                    └──────┬───────┘
                           ▼
                    ┌──────────────┐
                    │   Kernel     │── next_action / permitted
                    └──────┬───────┘
                           ▼
              VirtualStore / tools / shards
                           ▼
                    ┌──────────────┐
         ┌─────────│     LLM      │─────────┐
         │         └──────────────┘         │
         │ raw text                         │ needs system prompt
         ▼                                  ▼
  ResponseProcessor                  PromptAssembler
  StreamParser                       (+ JIT compiler)
         │                                  │
         ▼                                  │
  surface → TUI/stdout                      │
  control → mangle assert / tools /         │
            knowledge / memory              │
         ▲                                  │
         └──────── protocol enforced by ────┘
                   PiggybackProtocolSuffix
```

## 10. Wiring checklist (for auditors)

Before claiming “articulation unused”:

1. Grep `ProcessLLMResponse`, `NewResponseProcessor`, `StreamParser`, `PromptAssembler`, `PiggybackEnvelope`.  
2. Confirm factory `promptAssembler` non-nil under normal boot.  
3. Confirm chat helpers still call `Process` after stream buffer complete.  
4. Confirm session executor still filters mangle updates before assert.  
5. Prefer **fixing wiring** over deleting dual-channel types.

## 11. Environment / config knobs

| Knob | Effect |
|------|--------|
| `USE_JIT_PROMPTS=false` | Default assembler starts with JIT off (`defaultUseJIT`) |
| JIT config on Cortex | Budgets + EnableJIT from factory |
| `RequireValidJSON` | Per processor instance (callers rarely set true in prod) |
| `LogFallbackAsError` | ProcessLLMResponse true; AllowPlain false |
