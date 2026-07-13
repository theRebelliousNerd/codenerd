# 02 — Current State: articulation

> Last verified: 2026-07-13  
> Inventory is **on-disk truth** for `internal/articulation/`.

## Package metrics

| Class | Count | Notes |
|-------|------:|-------|
| Non-test `.go` | **8** | Production sources |
| Test `.go` | **7** | Unit, boundary, fuzz, benchmarks |
| Package `README.md` | 1 | Architecture overview (slightly incomplete file list) |
| Local `.mg` | **0** | Kernel queried via interface only |
| Approx. non-test LoC | **~3,200** | Dominated by assembler + emitter |

## File inventory (non-test)

| Path | ≈ Lines | Role |
|------|--------:|------|
| `internal/articulation/prompt_assembler.go` | 1165 | JIT/legacy system prompt assembly, Piggyback suffix, fallback templates |
| `internal/articulation/emitter.go` | 1103 | `ResponseProcessor`, caps, Emitter, constitutional override, `ProcessLLMResponse*`, salvage |
| `internal/articulation/schema.go` | 240 | `PiggybackEnvelopeSchema`, `SimpleEnvelopeSchema`, `GetPiggybackSchema` |
| `internal/articulation/protocol_types.go` | 239 | Envelope/control types + tolerant `UnmarshalJSON` helpers |
| `internal/articulation/kernel_context.go` | 128 | `GetKernelContext` / `BuildContextSection` for shard prompt augmentation |
| `internal/articulation/stream_parser.go` | 109 | Incremental `surface_response` extraction for streaming TUI |
| `internal/articulation/prompt_assembler_adapter.go` | 108 | Perception-safe adapter + `mapToPromptContext` for DI maps |
| `internal/articulation/json_scanner.go` | 105 | `findJSONCandidates` state machine with depth/size caps |

## Test inventory

| Path | Focus |
|------|--------|
| `emitter_test.go` | Happy path parse, markdown, caps, strict mode, fuzz, adversarial |
| `emitter_boundary_test.go` | Null fields, type coercion, decoy, applyCaps filters, benchmark |
| `emitter_extra_test.go` | Constitutional override, `ProcessLLMResponse`, reasoning directives |
| `emitter_helpers_test.go` | Emitter marshal, helpers, truncation messages |
| `json_scanner_test.go` | Candidate scan, decoy, depth/size limits, unicode, benchmarks |
| `prompt_assembler_test.go` | Assembly, JIT flags, budgets, races, map DI, piggyback suffix |
| `stream_parser_test.go` | Chunked surface streaming (+ noted concurrency gap TODO) |

## Hotspots (where change is expensive)

1. **`ResponseProcessor.Process`** — every chat/session/shard parse path funnels here or through thin wrappers.  
2. **`ControlPacket` / schema** — field additions require Go types, JSON schema, Piggyback suffix text, and consumer updates.  
3. **`PromptAssembler.AssembleSystemPrompt`** — dual path JIT vs legacy; budget and Piggyback append rules.  
4. **`applyCaps`** — security-relevant filters on mangle updates.  
5. **`findJSONCandidates` + last-match-wins** — correctness under prompt-injection / decoy attacks.

## Behavioral summary (what works today)

### Inbound (LLM → structure)

- Full dual-channel parse with multi-stage recovery.  
- Partial envelope detection and surface salvage / friendly truncation message.  
- Type-tolerant unmarshaling for confidence, required flags, and single-string `mangle_updates`.  
- Streaming surface-only parser for TUI.  
- Convenience APIs for shards: `ProcessLLMResponse`, `ProcessLLMResponseAllowPlain`, `MustExtractSurface`.

### Outbound (state → prompt)

- `PromptAssembler` with kernel querier, optional JIT, env default `USE_JIT_PROMPTS` on.  
- Context mapping from `types.SessionContext` and `types.StructuredIntent` into `prompt.CompilationContext`.  
- Kernel predicates: `shard_prompt_base`, `injectable_context` (shard + `*` + `/_all`), `specialist_knowledge`.  
- Piggyback protocol suffix + reasoning directives.  
- Adapter for perception / import-cycle isolation.

### Safety / formatting

- Surface length default 50k; control list caps; reasoning 50k.  
- Mangle atom validation (paren, trailing `.`, length, shell metacharacters `` `$;| ``).  
- Constitutional override prepends safety notice and drops blocked atoms.
- Shared `ResponseProcessor` statistics are mutex-protected; byte caps preserve
  valid UTF-8 for surface and reasoning output.

### Verified repair receipt

`VERIFIED CURRENT` — `internal/articulation/emitter.go#ResponseProcessor` now
serializes statistics snapshots/updates, and `applyCaps` cuts strings only at a
valid UTF-8 boundary. Regressions:
`internal/articulation/emitter_test.go#TestResponseProcessor_ConcurrentStats` and
`#TestResponseProcessor_Process_SurfaceTruncationPreservesUTF8`. Focused tests and
their `-race` run passed on 2026-07-13.

## What this package does **not** implement (callers do)

| Concern | Typical owner |
|---------|----------------|
| Streaming transport + thoughts channel | `cmd/nerd/chat/helpers_articulation.go` |
| Asserting `mangle_updates` after permission | `internal/session/executor.go` |
| Spawning specialists from `knowledge_requests` | chat process_knowledge / orchestrator |
| Tool catalog injection into prompts | session executor Piggyback++ path |
| Schema-enforced provider calls | `internal/perception` `CompleteWithSchema` |

## Status classification

| Component | Status |
|-----------|--------|
| Piggyback types + schema | **Implemented** |
| ResponseProcessor + salvage | **Implemented** |
| StreamParser | **Implemented** |
| PromptAssembler + JIT bridge | **Implemented** |
| Kernel context helpers | **Implemented** |
| Constitutional override helper | **Implemented** (thin) |
| Package-local Mangle rules | **N/A** |
| End-to-end “all control fields always applied” | **Partial (callers)** |

**Overall:** production, load-bearing package — **not** pre-implementation.
