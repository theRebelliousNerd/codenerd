# 09 — Safety and Invariants: articulation

> Last verified: 2026-07-13

## Invariants

### I1 — Surface is the only user-facing channel

Integrators must display `Surface` / `ProcessedLLMResponse.Surface`, not raw control JSON. Chat `formatResponse` enforces this. Salvage paths exist specifically so partial envelopes do not dump `control_packet`.

### I2 — Thought-first protocol

Models are instructed to emit `control_packet` before `surface_response`. StreamParser only begins user text after the surface key. Truncation before surface yields friendly message or salvaged partial surface.

### I3 — Empty surface fails structured parse

`parseJSON` rejects envelopes with empty `surface_response`. Fallback may still show raw text when non-strict.

### I4 — Control on fallback is untrusted / absent

`ProcessLLMResponse*` only attaches `Control` when `ParseMethod != "fallback"`. Do not assert mangle updates from fallback paths.

### I5 — Caps always apply after successful structure extract

`applyCaps` runs for all successful Process returns (including fallback surface). New control fields must join this function.

### I6 — Mangle update hygiene

Each update must:

- Be non-empty after trim  
- Length ≤ 1000  
- Contain `(` and end with `.`  
- Not contain shell metacharacters `` `$;| ``  
- Total list ≤ 2000 (then length-truncated before filter)

Invalid entries become warnings and are dropped.

### I7 — Resource bounds on extraction

| Limit | Value | Location |
|-------|------:|----------|
| JSON depth | 200 | `json_scanner.go` |
| Candidate size | 5 MB | `json_scanner.go` |
| Noise candidates without surface key | 50 | scanner |
| Surface max (default) | 50_000 | processor |
| Reasoning trace | 50_000 | applyCaps |
| Tool requests | 20 | applyCaps |
| Knowledge requests | 20 | applyCaps |
| Memory ops | 500 | applyCaps |

### I8 — Decoy resistance

Embedded extraction prefers candidates containing both `"surface_response"` and `"control_packet"`, iterating **backwards** (last-match-wins) to defeat leading decoy envelopes.

### I9 — Constitutional override mutates in place

`ApplyConstitutionalOverride` may rewrite `envelope.Surface` and filter `MangleUpdates`. Callers must treat the pointer as mutated and log/audit the returned `ConstitutionalOverride`.

### I10 — Assembler requires kernel

`NewPromptAssembler*` errors on nil `KernelQuerier`. No silent nil-kernel assembly.

### I11 — Structured-output-only shards skip Piggyback suffix

`shouldAppendPiggybackProtocol` false for `prompt.IsStructuredOutputOnly` and for perception/perception_firewall types — avoids corrupting non-envelope outputs.

## Safety layering (who owns what)

```
LLM output
  → articulation syntactic filters (I5–I8)
  → session/core policy + permitted + VirtualStore
  → tools / assert / memory
```

Articulation is **necessary but not sufficient** for constitutional safety.

## Concurrency hazards

| Risk | Mitigation |
|------|------------|
| Shared `ResponseProcessor` stats races | Prefer per-request processors (current call sites usually allocate new) |
| StreamParser concurrent ProcessChunk | Forbidden; one stream owner |
| JIT field races on assembler | `mu` RWMutex on JIT getters/setters |
| Concurrent EnableJIT during Assemble | Covered by assembler tests; still call Enable carefully |

## Injection / abuse models considered in tests

- Decy JSON before real envelope  
- Deep nesting CPU bombs  
- Catastrophic backtracking (scanner not regex)  
- Oversized reasoning traces  
- Hallucinated unknown keys (strict mode)  
- Shell metacharacters in mangle updates  
- Type coercion on confidence/required  

## What is **not** guaranteed here

- Semantic validity of Mangle predicates vs Decl corpus  
- Authorization of tool names  
- Path sandboxing for tool_args  
- That models always obey thought-first ordering  
- Persistence of memory_operations  

Those remain core/tools/session responsibilities.
