# 11 — Observability: `internal/types`

> Last verified: **2026-07-13**

## 1. Design stance

`internal/types` is intentionally **almost silent**. Observability of facts, shards, and LLM calls lives in:

- `internal/logging` (categories, audit helpers)
- Kernel assert/query paths
- Glass box / transparency / CLI chat activity

Logging from a pure contract package would either spam or create reverse dependencies.

## 2. In-package observability surface

| Location | Behavior |
|----------|----------|
| `transaction.go` `NewKernelTx` | `logging.Get(logging.CategoryKernel).Warn(...)` when kernel lacks `KernelTransactor`, then **panic** |

No metrics, no tracing spans, no structured audit events emitted here.

## 3. Indirect observability (consumers)

| Consumer concern | How types appear in logs |
|------------------|--------------------------|
| Fact dumps | Callers use `Fact.String()` |
| Token usage | `LLMToolResponse.Usage` / `UsageMetadata` fields |
| Thinking / SPL | `ThinkingProvider` getters + `ThoughtSummary` on responses |
| Grounding sources | `GroundingProvider.GetLastGroundingSources` / response fields |
| Tool calls | `ToolCall` / `ToolResult` IDs for pairing |
| Shard lifecycle | Audit APIs in logging package use shard IDs from config |

## 4. Debug hooks useful when working on types

1. Unit tests for `ToAtom` / `String` — primary debug tool.
2. Kernel debug dump `debug_program_ERROR.mg` (system-level) when poisoned facts crash eval — often traceable to bad `Args` types.
3. Compare `Fact.String()` output with engine-stored constants when investigating name vs string mismatches.

## 5. Recommendations (non-blocking)

| Idea | Pros | Cons |
|------|------|------|
| Counter for ToAtom errors at call sites in core | Ops visibility | Not in types; keep here clean |
| Replace panic with error in NewKernelTx | Observable, recoverable | API break; loses fail-closed hardness |
| Debug build flag for heuristic rejections | Tune path-vs-atom | Complexity |

Default recommendation: **keep types quiet**; instrument assert wrappers in `core`.
