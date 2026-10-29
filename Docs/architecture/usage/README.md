# usage — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living reference — **code-grounded full corpus**  
> Source: `internal/usage/` (1:1 package root)  
> Scale: **2** non-test Go files · **4** test files · **0** Mangle sources

## Role

`internal/usage` is the **workspace-scoped LLM token accounting** package. It records input/output token counts per call, aggregates them by provider/model/shard-type/operation/session, and persists totals to `.nerd/usage.json`. Call sites attach a `*Tracker` on `context.Context`; perception clients (today: ZAI) pull it via `FromContext` and call `Track` after a successful completion.

This package is **observability / cost hygiene**, not the Mangle executive. It does not assert facts, gate `permitted(...)`, or participate in OODA decide. It sits beside the fact-flow so operators can see what the creative center spent.

```
LLM call succeeds (perception)
        │
        ▼
usage.FromContext(ctx) ──nil?──► no-op
        │ non-nil
        ▼
Tracker.Track(ctx, model, provider, in, out, op)
        │
        ├─ aggregate maps (TotalProject, By*)
        └─ debounced Save → .nerd/usage.json
```

## Scope of this corpus

| In scope | Out of scope |
|----------|--------------|
| `internal/usage/*` types, tracker, context helpers | Pricing tables / live cost APIs |
| Wiring from system boot, CLI, chat, shards, ZAI client | Implementing new providers' Track calls |
| Persistence shape of `usage.json` | Session transcript / campaign billing products |
| Concurrency and failure modes of the tracker | Mangle policy for budget enforcement |

## Document map

| Doc | Purpose |
|-----|---------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living spec — inventory, flows, integration, gaps |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star dimensions scored with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision for usage |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise file inventory, roles, hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding principles specific to this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state, types |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with import evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, chat, shards, perception attach points |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, degradation, no-kernel surface |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Existing tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | What the package *is* vs what it logs |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

## Verify

```powershell
go test ./internal/usage/...
```

Broader consumers (manual / integration):

```powershell
# UI page unit coverage (usage_page uses Tracker.Stats)
go test ./cmd/nerd/ui/ -run UsagePage -count=1
```

## North-star placement

| Concern | usage role |
|---------|------------|
| LLM = creative center | **Meters** creative center spend; does not choose models |
| Kernel = executive | **No** predicates, no `permitted`, no fact injection |
| JIT prompt atoms | **None** — not LLM-facing prose |
| Wiring before “unused” | `UsageEvent` / `Cost` fields look dormant; **do not delete** without auditing UI + future cost work |

## Related packages

- `internal/system` — constructs tracker at boot (`factory.go`)
- `internal/perception` — ZAI client records usage from context
- `internal/core/shards` — `WithShardContext` on spawn
- `cmd/nerd` / `cmd/nerd/chat` — attach tracker to request contexts
- `cmd/nerd/ui` — usage statistics page
