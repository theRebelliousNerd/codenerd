# prompt — Gap Analysis

> Last verified: **2026-07-13**

## Method

Compare **vision** (01) and **north star** (Agents.md) against **implemented behavior** in `internal/prompt/` and primary consumers.

## Spec vs reality matrix

| Capability | Vision | Reality | Gap severity |
|------------|--------|---------|--------------|
| JIT compile from atoms | Required | Full pipeline in `Compile` | **None** |
| Skeleton critical path | Required | Kernel required; hard fail | **None** |
| Flesh degradable | Required | Flesh errors warn-only | **None** |
| Multi-source atoms | Required | Embedded/DB/evolved/kernel/knowledge | **None** |
| Budget + polymorphism | Required | Fit + 3 modes | **None** |
| ConfigFactory tools/policies | Required | Implemented; dual catalogs | **Low–Med** |
| Embeddings for built-ins | Desired | Need SQLite sync for vectors | **Low** (documented) |
| Perfect cache correctness | Desired | Hash omits some dynamic dims | **Medium** |
| Single ConfigAtom source | Desired | Default provider + SimpleRegistry | **Medium** |
| Conflict resolution in Go | Desired | Relies on Mangle primarily | **Low** |
| Package `agents.md` | Root map lists it | May be missing; README carries load | **Low** |
| PredicateSelector first-class session use | Optional vision | Code exists; consumer density uneven | **Low–Med** |
| Cache TTL enforcement | Config has TTL field | LRU size is primary eviction | **Low** |
| Quantized embeddings | TODO in atoms.go | Full float32 | **Low** (perf) |
| Per-request vector weight | TODO in selector | Global on selector | **Low** |
| Full Cobra/TUI JIT inspector parity | Glass-box partial | `ui/jit_page.go` present | **Low** |

## Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| Package has no local `.mg` | Selection rules correctly live in core defaults |
| Flesh may be empty | By design under budget/timeout |
| Baseline path exists | Intentional fallback without kernel |
| Session abstracts JIT behind interface | Correct for testing/mocks |
| Tool names as strings | Matches VirtualStore string routing |

## Priority backlog (from gaps)

### P0 — correctness

1. **Cache key completeness** — include dimensions that change prompt selection without changing Hash (e.g. `PreviousAttemptNoToolCall`, `AvailableTools`, maybe activation).  
2. **ConfigAtom consistency** — one registry or generated dual sync for tool/policy lists.

### P1 — operability

3. Document/predicate-selector wiring map (who calls `PredicateSelector.Select`).  
4. Enforce or remove unused `CacheTTLSeconds`.  
5. Add `internal/prompt/agents.md` if root Working Map continues to cite it.

### P2 — scale

6. Embedding quantization / ann index if corpus grows large.  
7. Streaming fact builder for context assertion (TODO in context.go).

## Regression watchpoints

| Change | Risk |
|--------|------|
| Reorder `prompt_atom` fact args | Breaks Mangle selection silently |
| Rename tool strings | Config allows tools kernel never has |
| Drop safety/identity atoms | Skeleton empty → CRITICAL failures |
| Change skeleton category set | Safety/protocol may leave skeleton |
| Retract predicate rename | Stale compile_context pollution |
