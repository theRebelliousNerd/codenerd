# 00 — Alignment / Vision Review: `internal/types`

> Last verified: **2026-07-13**  
> Scoring: **0–5** against codeNERD north star, with code evidence.

## Summary

| Dimension | Score | One-line evidence |
|-----------|------:|-------------------|
| LLM creative / logic executive split | 5 | Contracts encode split: `LLMClient` vs `Kernel` / facts |
| Constitutional safety surface | 4 | Session constitutional fields + permissions; enforcement not here |
| Import-cycle hygiene | 5 | Package exists explicitly to break cycles |
| Fact integrity (no poison) | 5 | `ToAtom` rejects nil / address-shaped types |
| JIT / prompt-atom friendliness | 4 | `SessionContext`, JIT registrar hooks; no prompt text here |
| Minimal dependencies | 4 | Thin deps; only `logging` for tx panic path |
| API coherence | 3 | Dual `Kernel` / `KernelInterface` debt |
| Testability of contracts | 4 | Strong ToAtom/Extract tests; weaker interface compliance tests |
| Observability | 2 | Intentionally thin; one warn+panic path |
| Documentation honesty | 5 | This corpus rebuild |

**Mean: ~4.1** — strong foundational alignment; main drag is dual kernel APIs and stringly context keys.

---

## Dimension detail

### 1. LLM creative center vs logic executive — **5/5**

North star: model describes; logic decides.

`types` encodes that split in types alone:

- Creative I/O: `LLMClient`, tools, thinking metadata, grounding
- Executive I/O: `Fact`, `Kernel`, transactions, `permitted`-related session fields (`AllowedActions` / `BlockedActions`)

No LLM call is made from this package. No policy is evaluated here. Correct.

### 2. Constitutional safety — **4/5**

Present as **data**:

- `SessionContext.AllowedActions` / `BlockedActions` / `SafetyWarnings`
- `ShardPermission` capability labels
- Dream mode flag (simulation without side effects)

Absent as **enforcement** (correct — enforcement is Mangle `permitted(...)` + VirtualStore). Score 4 because permission enums are not automatically mapped to Decl predicates here (that mapping lives in policy/core).

### 3. Import-cycle hygiene — **5/5**

Package comment and move notes:

- Breaks `core` ↔ `articulation` ↔ `autopoiesis`
- `GraphQuery` moved from `world` to break `core` ↔ `world`
- `VirtualStore` is a marker interface pointing at `*core.VirtualStore` without importing core

This is the primary architectural reason the package exists.

### 4. Fact integrity — **5/5**

`ToAtom` comments document the failure mode of silent `%v` coercion and the deliberate error contract. Tests cover nil, unknown structs, name heuristics, time/duration, containers.

### 5. JIT / prompt atoms — **4/5**

- `PromptLoaderFunc`, `JITDBRegistrar`, `JITDBUnregistrar` for agent knowledge DBs
- `SessionContext` feeds prompt assembly (articulation), not ad-hoc shard prompt strings
- Package does not hold prompt atom content (correct)

### 6. Minimal dependencies — **4/5**

Depends on:

- stdlib
- `codeberg.org/TauCeti/mangle-go/ast` + `analysis`
- `codenerd/internal/logging` (only `transaction.go`)

Could theoretically avoid logging by returning an error from `NewKernelTx` instead of panic+warn — small purity hit.

### 7. API coherence — **3/5**

Honest debt:

- `Kernel` vs `KernelInterface` + `Fact` vs `KernelFact`
- Context keys as plain strings vs typed keys for session
- `VirtualStore` intentionally incomplete (“expand as needed”)

### 8. Testability — **4/5**

Excellent pure function coverage; compile-time `var _ types.X = (*Impl)(nil)` lives in implementers (`core`), not here — acceptable but means interface drift is not tested in-package.

### 9. Observability — **2/5**

Expected for a types package. Only operational log is non-transactor warning before panic.

### 10. North-star purity (no product leakage) — **5/5**

No Vectryx product terms, no app-specific DTOs. General agent runtime contracts only.

---

## Alignment verdict

**Keep and harden.** Do not turn `types` into a kitchen-sink DTO package. Prefer consolidating kernel interfaces over adding more parallel shapes.
