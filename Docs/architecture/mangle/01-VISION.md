# 01 — Vision: mangle package

> Last verified: 2026-07-13  
> Product/architecture target for `internal/mangle` within codeNERD.

## Role in the product

**Mangle is the deterministic substrate of cognition.**  
The LLM proposes; Mangle *is* the world’s executable theory of how facts become actions.

This package’s vision is not “a Datalog library wrapper.” It is:

> The only legal way creative systems emit or evaluate logic: typed, Decl-gated, gas-limited, reparable, and observable.

## Target capabilities

### 1. Hollow-kernel evaluation API

A single durable API surface for:

- Loading stratified programs (schemas + policy + learned).
- Asserting EDB facts with type/arity enforcement.
- Querying with timeouts.
- Enforcing inference gas (`DerivedFactsLimit` / kernel limits).
- Optional durability via `Persistence`.

### 2. Differential / incremental evaluation as the default hot path

When EDB deltas arrive every OODA tick:

- Re-evaluate only what changed (or amortize via one seminaive call on a unified store).
- Preserve snapshot isolation for simulation / what-if branches.
- Support virtual/lazy predicates without full materialization.

### 3. Closed-loop generation of logic

LLM-facing pipeline that is **budgeted and structured**:

```
prompt (+ JIT predicates) → LLM → extract rule / MangleSynth JSON
  → normalize → pre-validate → sanitize → HotLoad parse
  → schema / forbidden-head gates → accept or structured feedback retry
```

Session budgets prevent infinite repair thrash. Synth mode prefers JSON so models never freehand syntax when avoidable.

### 4. Grammar-constrained decoding (GCD)

Atom-level validation against schema predicates and name constants, with repair prompts — preventing “hallucination of agency” (invented predicates, wrong arities).

### 5. Operator-grade tooling

- `mangle check` / corpus validation for all `.mg` sources.
- LSP for definitions, references, diagnostics, completion.
- Proof trees / derivation traces for glass-box explainability.

### 6. Process-wide parse safety

One chokepoint for ANTLR global state so concurrent kernel boot, hot-load, and CLI never race.

## Non-goals (for this package)

| Non-goal | Owner instead |
|----------|---------------|
| Deciding which action to take | core policy + executive shards |
| Implementing `permitted` rules | `internal/core/defaults/policy/` |
| Prompt atom selection / JIT compiler | `internal/prompt` |
| VirtualStore external tool execution | `internal/core` VirtualStore |
| Full IDE product | external clients + `cmd/nerd` mangle-lsp |
| Fuzzy NL matching in rules | embeddings / perception; assert structured facts |

## Success criteria

1. **No undeclared body predicates** reach learned storage without rejection.
2. **No learned head** may define `permitted` / pipeline spoof predicates.
3. **No unbounded inference** without gas configuration.
4. **No concurrent parse races** under the race detector.
5. **Generation loops** terminate (per-rule + session budgets).
6. **Kernel OODA** can evaluate deltas faster than full rebuild when diff path is safe.
7. **Tests** continuously load real schemas+policy (`mangle_validation_test.go` style).

## Horizon (ordered by dependency, not calendar)

1. Forward eval options (externals + gas) through DifferentialEngine so kernel can stay on diff path more often.
2. True delta-aware evaluation (beyond 2-bucket / unified re-eval).
3. Route all sanitizer/synth parse sites through `ParseUnit`.
4. Promote MangleSynth to default for all autopoiesis writers.
5. Unify proof-tree tracer with provenance recorder for one glass-box story.
