# 07 — Dependency Map: articulation

> Last verified: 2026-07-13  
> Evidence via imports of `codenerd/internal/articulation` and package imports of articulation itself.

## Upstream (what articulation imports)

| Package | Why |
|---------|-----|
| `encoding/json`, `fmt`, `strings`, `strconv` | Parse / format |
| `context`, `os`, `path/filepath`, `sort`, `sync`, `maps` | Assembler / env / concurrency |
| `codenerd/internal/logging` | `CategoryArticulation`, timers |
| `codenerd/internal/prompt` | JIT compiler, compilation context, embedded baseline, structured-only check |
| `codenerd/internal/types` | `Fact`, `SessionContext`, `StructuredIntent`, extract helpers |

**Does not import:** `core` (kernel concrete), `perception`, `session`, `shards` — cycle avoidance by design.

## Downstream (who imports articulation)

### CLI / chat (primary UX)

| Path | Use |
|------|-----|
| `cmd/nerd/chat/helpers_articulation.go` | StreamParser, ResponseProcessor, envelope types, ArticulationOutput |
| `cmd/nerd/chat/helpers.go` | ProcessLLMResponseAllowPlain for display cleanup |
| `cmd/nerd/chat/delegation.go` | ProcessLLMResponseAllowPlain |
| `cmd/nerd/chat/process_knowledge.go` | Knowledge request types / processing |
| `cmd/nerd/chat/process_dream_delegation.go` | ResponseProcessor |
| `cmd/nerd/chat/session_boot.go` | PromptAssemblerWithJIT + Adapter → transducer |
| `cmd/nerd/chat/campaign.go` / `campaign_assault.go` | Assembler with JIT |
| `cmd/nerd/chat/campaign_jit_provider.go` | PromptAssembler + PromptContext |
| `cmd/nerd/chat/model_types.go` | Type references |
| `cmd/nerd/cmd_instruction.go` | Emitter + envelopes |
| `cmd/nerd/cmd_campaign.go` | Assembler with JIT |
| `cmd/nerd/campaign_jit_provider.go` | Same pattern as chat provider |

### Runtime core path

| Path | Use |
|------|-----|
| `internal/system/factory.go` | Boot PromptAssembler, budgets, adapter on transducer/shards, Cortex field |
| `internal/session/executor.go` | Schema length log, ProcessLLMResponse*, tool_requests, mangle updates + constitutional override, memory/self-correction helpers |
| `internal/perception/transducer.go` | Type alias `PiggybackEnvelope` |
| `internal/perception/client_schema.go` | Unmarshals `PiggybackEnvelopeSchema` for schema clients |
| `internal/perception/gemini_live_test.go` | Process + schema e2e tests |

### Shards

| Path | Use |
|------|-----|
| `internal/shards/registration.go` | Wiring |
| `internal/shards/system/base.go` | Set/Get PromptAssembler, PromptContext assembly |
| `internal/shards/system/planner.go` | ProcessLLMResponse, PromptAssembler |
| `internal/shards/system/perception.go` | Holds `*PromptAssembler` |
| `internal/shards/system/legislator.go` | Assembler / process |
| `internal/shards/system/mangle_repair.go` | Assembler / process |
| `internal/shards/requirements_interrogator.go` | Assembler usage |

### Tests / e2e

| Path | Use |
|------|-----|
| `tests/e2e/piggyback_executor_full_boundary_test.go` | Envelope + ProcessLLMResponse boundary |
| Various `*_test.go` under perception/shards | Integration |

### Soft dependency notes

- `internal/autopoiesis` documents `PromptAssembler`-shaped interface without hard import cycle (`autopoiesis_types.go` comments). Factory injects concrete `*articulation.PromptAssembler`.

## Dependency direction diagram

```
                  internal/prompt
                  internal/types
                  internal/logging
                        ▲
                        │
              internal/articulation
                        ▲
        ┌───────────────┼───────────────────┐
        │               │                   │
 internal/session   internal/system     internal/perception
 internal/shards      (factory)          (schema + alias)
        │               │                   │
        └───────────────┴───────────────────┘
                        ▲
                 cmd/nerd (+ chat)
                 tests/e2e
```

## Blast radius

Changing:

| Change | Breaks |
|--------|--------|
| JSON field renames | Schema clients, all parsers, model prompts |
| ParseMethod strings | Chat/session branching |
| applyCaps filtering | Kernel assert content |
| AssembleSystemPrompt signature | Shards, factory, campaigns |
| KernelQuerier shape | All assembler construction |

## Verify reverse deps

```powershell
rg "codenerd/internal/articulation" -g "*.go" --glob "!*_test.go"
```
