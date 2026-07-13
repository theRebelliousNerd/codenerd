# 10 — Testing Alignment: articulation

> Last verified: 2026-07-13

## Commands

```powershell
go test ./internal/articulation/...
go test ./internal/articulation/... -count=1 -v
go test ./internal/articulation/ -run TestResponseProcessor -count=1
go test ./internal/articulation/ -fuzz=FuzzResponseProcessor_Process -fuzztime=10s
go test ./internal/articulation/ -bench=Benchmark -benchmem
```

Downstream smoke (not owned by package, but dependent):

```powershell
go test ./internal/session/ -count=1
go test ./tests/e2e/ -run Piggyback -count=1
```

## Test file map

| File | Coverage themes |
|------|-----------------|
| `emitter_test.go` | Direct JSON, markdown, embedded order-agnostic, strict validation, surface truncation, control caps, null fields, type coercion, massive reasoning, recursion depth, duplicate keys, decoy, hallucinated keys, DOS, non-ASCII, brace imbalance, strict unknown fields, **fuzz** |
| `emitter_boundary_test.go` | Boundary nulls/coercion, massive trace, duplicates, extract benchmark, decoy, applyCaps rejects (length/syntax/metachar) |
| `emitter_extra_test.go` | Constitutional override, ProcessLLMResponse, AppendReasoningDirective |
| `emitter_helpers_test.go` | Emitter create/marshal, state helpers, ExtractSurfaceOnly, extractStringField, truncatedEnvelopeMessage |
| `json_scanner_test.go` | Candidates, decoy, deep nest, unicode emoji, depth cap, size limit, post-reset recovery, benchmarks |
| `prompt_assembler_test.go` | Constructors, assemble, context atoms, fallbacks, quick prompt, builders, JIT helpers, toCompilationContext, null inputs, massive context, concurrent mutation, conflicting shard_prompt_base, JIT race, map DI, missing piggyback suffix behavior |
| `stream_parser_test.go` | Chunked surface extraction; **TODO** concurrent ProcessChunk |

## Strengths

- Adversarial posture matches production threat model (decoy, DOS, depth).  
- Caps and mangle filters unit-tested.  
- Assembler race/JIT tests exist.  
- Fuzz entrypoint on `Process`.  

## Gaps

| Gap | Severity | Notes |
|-----|----------|-------|
| StreamParser concurrency | Low | Documented TODO in test file |
| End-to-end control → kernel assert | Med | Lives in e2e/session; keep in sync when fields change |
| Memory op / context_feedback application | Med | Protocol tested; application may lack unit tests in this package |
| Schema constant vs Go type drift | Med | No automated schema↔struct generator check |
| Salvage helpers | Low | Covered partly in helpers tests; more truncated mid-string cases possible |
| Provider schema live path | Low | perception gemini_live_test; may be env-gated |

## Alignment with principles

| Principle | Test signal |
|-----------|-------------|
| Dual channel | Process_JSON, envelope helpers |
| Caps | Process_ControlCaps, applyCaps_* |
| Decoy | ExtractEmbeddedJSON_DecoyInjection |
| JIT fallback | AssembleSystemPromptFallsBackOnNoJIT |
| Constitutional | TestApplyConstitutionalOverride |
| Streaming surface only | TestStreamParser |

## Definition of done for new control fields

1. Add Go fields + Unmarshal tolerances if needed.  
2. Update `schema.go` and `schemaAllowedKeys` in emitter.  
3. Update `PiggybackProtocolSuffix` documentation block.  
4. Cap in `applyCaps`.  
5. Unit tests for parse + cap.  
6. At least one consumer test (session/chat) if field is load-bearing.
