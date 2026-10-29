# 09 — Safety and Invariants: Northstar

> Last verified against codebase: 2026-07-13  
> Package: `internal/northstar`

## 1. Relationship to constitutional safety

Northstar is **not** the `permitted(...)` constitutional layer. Default deny for tool/actions remains core policy.

Northstar adds **strategic** safety:

- Vision-aligned work vs mission drift
- Campaign risk gate component `/northstar`
- Soft/hard outcomes on alignment scores

A change can be constitutionally permitted and still **blocked** by campaign Northstar gate (goal conflicts with vision). Conversely, a high alignment score never grants tool permission.

## 2. Invariants

### I1 — Vision singleton

At most one vision row (`id=1`). `SaveVision` upserts; `LoadVision` returns `nil, nil` when absent (not an error).

### I2 — Clone isolation

`GetVision` / `GetState` never return live internal pointers. Mutations on returned values must be written back via `UpdateVision` / store APIs.

### I3 — Alignment always returns a check object on soft failure modes

`CheckAlignment` returns `(*AlignmentCheck, nil)` for no vision, no LLM, and LLM errors. Callers must inspect `Result`/`Score`, not only `error`. Campaign hard fail uses **error** when result is blocked.

### I4 — Drift only on failed/blocked

`persistAlignmentOutcome` records drift solely for `AlignmentFailed` and `AlignmentBlocked`. Warnings do not open drift events.

### I5 — Kernel refresh is best-effort per fact

Assert failures log Debug and continue. Retract errors ignored. Nil kernel → no-op.

### I6 — Foreign keys on drift

`drift_events.related_check` references `alignment_checks(id)`. Related check should exist before drift insert (current code records check first).

### I7 — Threshold ordering assumption

Config assumes `BlockThreshold ≤ FailureThreshold ≤ WarningThreshold`. Extreme configs are not validated at construction (tests tolerate massive floats without panic).

### I8 — No vision ⇒ no automatic checks

`ShouldCheckNow` returns false for all automatic triggers when vision is nil; manual still true only when vision exists… actually manual returns true only after the vision nil early-return? Looking at code:

```go
if g.vision == nil {
    return false
}
// ...
case TriggerManual:
    return true
```

So **manual ShouldCheckNow also false without vision**. `CheckAlignment` itself still runs and returns skipped. Campaign start skips check when `!HasVision()` (does not error).

## 3. Concurrency safety

| Resource | Guard |
|----------|-------|
| Guardian fields | `sync.RWMutex` |
| Store DB | `sync.RWMutex` + SQL transactions |
| CampaignObserver maps/counters | `sync.RWMutex` |
| TaskObserver | `sync.Mutex` on complete |

Tests: `TestGuardian_Initialize_Concurrent`, `TestGuardian_ConcurrentAccess`.

## 4. Soft vs hard enforcement map

| Path | Enforcement |
|------|-------------|
| `CheckAlignment` alone | Advisory (returns result) |
| `/alignment` UI | Advisory display |
| Background assessment `block` | Depends on BackgroundObserverManager consumer (outside package) |
| Campaign `StartCampaign` / `OnPhaseStart` | **Hard** error if `AlignmentBlocked` |
| Campaign risk missing observer on protected roots | **Hard** block in risk_scoring |

## 5. Threat / abuse notes

| Risk | Mitigation / residual |
|------|------------------------|
| LLM jailbreak of alignment format | Parser defaults to warning; explicit RESULT can force blocked if model says so — adversarial model could always pass |
| Keyword relevance gaming | Not security-critical; observability only |
| SQL injection | Parameterized queries throughout Store |
| Path traversal in high-impact match | Matching only gates *checks*, not file access |
| Dual store confusion | Residual product risk — operator believes vision set when Guardian empty |

## 6. Mangle Decl discipline

Package does not ship `.mg`. When asserting, facts must match Decl arities in core schema (see dump / defaults). `northstar_defined()` is nullary. Timeline/type fields use `/name` atoms via `"/"+field`.

**Mitigation gap:** free-text mitigation is not asserted; always `/mitigation` atom — policy cannot reason over strategy content.
