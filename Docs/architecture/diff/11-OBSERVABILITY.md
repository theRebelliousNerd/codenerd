# 11 — Observability: `internal/diff`

> Last verified against codebase: 2026-07-13

## 1. Current state

**Near-zero observability instrumentation.**

| Signal | Present? | Notes |
|--------|----------|-------|
| Structured logs (zap / package logger) | **No** | No logging imports |
| Metrics / counters | **No** | |
| Spans / tracing | **No** | |
| Debug dump hooks | **No** | |
| Panic recovery | **No** | Relies on library + callers |
| Glass box / transparency CLI | **No** | Not a system surface |

This is acceptable for a pure, synchronous library **if** callers observe outcomes
(`IsBinary`, empty hunks, latency). Today, callers generally do not log those either.

## 2. Implicit signals available to callers

| Observable | How |
|------------|-----|
| Binary short-circuit | `FileDiff.IsBinary` |
| New / delete | `IsNew` / `IsDelete` |
| Work size | `len(Hunks)`, sum of line counts |
| Latency | Caller wall-clock around `ComputeDiff` |
| Cache effectiveness | Not exposed; only infer via repeated latency |
| DiffTimeout fired | Not explicitly exposed by this package; sergi may return partial diffs |

## 3. What *should* exist (vision, not implemented)

Optional debug counters on `Engine` (zero cost when unused):

| Counter | Meaning |
|---------|---------|
| `Computes` | Total `ComputeDiff` calls |
| `CacheHits` / `CacheMisses` | Hit rate |
| `BinaryShortCircuits` | NUL gates |
| `Timeouts` | If library exposes or via wall-clock > threshold |
| `CacheEntries` | `sync.Map` size estimate |

A `Stats() EngineStats` method would be enough for glass-box / long-session diagnosis
without forcing logging into the hot path.

## 4. Debugging playbook (today)

1. Reproduce with unit test and fixed strings.  
2. Check `IsBinary` if hunks empty unexpectedly.  
3. Compare `NewEngine()` vs `DefaultEngine` pollution.  
4. Call `ClearCache()` between experiments.  
5. Use benchmarks for large inputs.  
6. If hang suspected: confirm content is not multi-MB single-line; rely on 5s timeout.  
7. For UI issues: distinguish package output vs `diffview` filter/render bugs.

## 5. Relation to codeNERD telemetry

Glass box / transparency (`cmd/nerd` transparency commands) do not currently instrument
this package. Diff remains an opaque pure step inside approval UX.
