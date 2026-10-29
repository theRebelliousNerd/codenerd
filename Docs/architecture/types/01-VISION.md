# 01 — Vision: Shared Types Layer

> Last verified: **2026-07-13**  
> Target architecture for `internal/types` (contracts + conversion), not every line fully realized — see gap analysis.

## 1. Product / architecture identity

`internal/types` is the **stable ABI inside the monorepo** for Cortex:

- Every fact that crosses package boundaries should be a `types.Fact` (or a documented thin alias).
- Every shard should implement `types.ShardAgent` and take `types.ShardConfig`.
- Every LLM provider should satisfy `types.LLMClient` and opt into capability interfaces via type assertion.
- Session compression should always reify as `types.SessionContext` before shard/prompt injection.

Tagline for this package:

> **Contracts live here. Behavior lives in implementers.**

## 2. Experience principles (for developers using the package)

1. **One fact type** — no parallel private `Fact` structs in domain packages (aliases OK).
2. **Fail loud on poison** — bad args error at assert time with predicate + index.
3. **Optional power via interfaces** — do not bloat `LLMClient` with every provider feature.
4. **Blackboard over prompt stuffing** — `SessionContext` is the structured handoff; prose compression happens upstream.
5. **Atomic multi-fact updates** — callers use `NewKernelTx` for multi-op EDB mutations.
6. **Cycle-free by design** — if a new type would force `types` to import `core`/`session`/`shards`, redesign.

## 3. Target architecture (logical)

```
                    ┌──────────────────────────────┐
                    │       internal/types         │
                    │  Fact | Kernel* | LLM* |     │
                    │  Shard* | SessionContext     │
                    └──────────────┬───────────────┘
           implement               │               consume
    ┌──────────────┬───────────────┼───────────────┬──────────────┐
    ▼              ▼               ▼               ▼              ▼
  core          perception      shards         articulation   campaign
  VirtualStore  LLM clients     agents         prompt asm     fact sync
  CortexKernel  Anthropic/...   manager        SessionContext store
```

### Ideal kernel surface (target)

Long term: **one** kernel interface family:

- `Kernel` for full operations
- Optional `KernelTransactor` for atomicity
- Deprecate or fully alias `KernelInterface` / `KernelFact` once all callers migrate

### Ideal LLM surface (target)

Keep:

- Core `LLMClient`
- Optional: `ToolResultsProvider`, grounding, thinking, piggyback, files, cache, tokens

Avoid:

- Provider-specific concrete types leaking into `types`

## 4. Non-goals

- Business logic of shards, campaigns, or tools
- Mangle Decl / policy authoring
- Storage schemas for sqlite (those belong in `store` / `persist`)
- UI view-models (CLI `ui` may map from types, not vice versa)
- Becoming a second `core` package

## 5. Success metrics

| Metric | Direction | How measured |
|--------|-----------|--------------|
| Packages defining private Fact duplicates | ↓ → 0 | grep for parallel Fact structs |
| ToAtom poison panics in production logs | 0 | assert errors caught at call site |
| Callers using Extract* vs bare Args[i].(string) | ↑ | code search |
| KernelInterface-only packages | ↓ after consolidation | import graph |
| `go test ./internal/types/...` green | 100% | CI |
| `types` import of heavy internal packages | 0 | go list / review |
