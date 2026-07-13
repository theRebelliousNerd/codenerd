# 11 — Observability: mangle

> Last verified: 2026-07-13

## Logging

Primary category: **`logging.CategoryKernel`** (via `logging.Kernel`, `KernelDebug`, `Get(...).Info/Warn/Error`).

### Engine

| Event | Level | When |
|-------|-------|------|
| Schema load / parse failures | Error | LoadSchema* |
| Rule recomputation start/complete | Kernel / Debug | RecomputeRules |
| Still recomputing (30s ticker) | KernelDebug | Long eval |
| Derived facts per round | KernelDebug always; Info if ≥100 | evalWithGasLimit |
| Fact limit 85% | Warn | once |
| Query parse/exec/timeout | Debug / Error / Warn | Query |
| Derived counter reset | KernelDebug | |

### FeedbackLoop

| Event | Level | When |
|-------|-------|------|
| Attempt N/M | KernelDebug | Each try |
| Budget exhausted | Warn | CanRetry false |
| LLM failure / timeout | Error / Warn | Complete errors |
| Pre-validation issues | KernelDebug | counts |
| Sanitizer auto-fix / fail | KernelDebug | |
| Compile / schema fail | KernelDebug | |
| Success | Kernel | attempt number |
| JIT predicate selection | Info / Warn | selector path |
| All attempts failed | Warn | terminal |

### Kernel (related, not in package)

- `rebuildProgram` sizes, parse/analyze timers, `debug_program_ERROR.mg` dump on analysis failure.
- Diff invalidate reasons; full vs differential complete logs.

## Metrics / stats APIs

| API | Data |
|-----|------|
| `Engine.GetStats()` | TotalFacts, PredicateCounts, LastUpdate |
| `Engine.GetDerivedFactCount()` | Cumulative derived estimate |
| `ValidationBudget.Stats()` | sessionUsed / sessionBudget |
| `QueryResult.Duration` | Per-query latency |
| mangle-go `EvalStratifiedProgramWithStats` | Strata durations (kernel logs) |

## Debug hooks

| Hook | Purpose |
|------|---------|
| `debug_program_ERROR.mg` (cwd) | Kernel dumps combined program on analyze failure |
| `ProofTreeTracer` | Derivation trees for glass-box |
| `MANGLE_AUTO_EVAL=0` | Disable auto-eval for bulk diagnostics |
| `CODENERD_DIFF_EVAL=0` | Force full eval path |
| LSP diagnostics | IDE-facing syntax/semantic issues |
| `inspect` tests | Program/Decl inspection helpers |

## Glass-box / transparency

- `internal/transparency` imports mangle for explanation.
- Proof trees materialize EDB vs IDB derivation structure for operator-facing “why”.
- Kernel provenance recorder (Codeberg) is a **parallel** path on full eval — not the same object as ProofTreeTracer.

## Noise control

- Derived Info logs gated at ≥100 facts/round to avoid drowning OODA logs.
- Prefer KernelDebug for per-attempt feedback chatter.

## Gaps

- No first-class Prometheus/OpenTelemetry counters in-package.
- Diff path lacks structured “path=diff|full” metric export beyond kernel debug logs.
- Sanitizer failures are debug-level only — may hide chronic repair failure rates in production.
