# 09 — Safety and Invariants: campaign

> Last verified: **2026-07-13**

## Constitutional position

Campaign is **not** the constitutional permission authority. It schedules work and launches intents through `session.TaskExecutor` / tactile. Mutating tools must still satisfy kernel `permitted(...)` / VirtualStore checks. Campaign-local safety is about:

- plan integrity  
- concurrent file races  
- resume correctness  
- risk-aware start gates  
- path containment for fallbacks  

## Invariants

### I1 — Single running campaign per orchestrator

`Run` returns error if `isRunning`. One cancelFunc, one heartbeat loop per Run.

### I2 — Kernel facts mirror durable plan (eventual)

`SetCampaign` / `LoadCampaign` / status updates assert or retract campaign facts. Rolling-wave reloads phase/task facts after refine. Divergence is a bug; restart recovery rewrites in_progress → pending facts.

### I3 — Snapshot durability is journal-ordered

No committed snapshot without preceding journal request event for that write path; commit event after atomic rename.

### I4 — Write-set exclusivity for concurrent mutators

Tasks with overlapping write sets do not hold leases simultaneously. Timeout yields schedule skip (`ErrWriteSetLockTimeout`), not silent overlap.

### I5 — Workspace containment

Write-set normalization rejects paths outside workspace (production task IDs). File fallback rejects `..` and absolute roots in description-extracted paths.

### I6 — Retry bounds

`MaxRetries` (default 3) prevents infinite failure loops. Backoff uses `NextRetryAt` filtered in eligibility.

### I7 — Checkpoint honesty

Failed verification does not complete the phase; replan_trigger is asserted.

### I8 — Assault batch identity

Batch tasks carry `/assault_batch` artifacts; batch execution fails closed if artifact missing.

### I9 — Risk threshold floor

Threshold clamped to at least `defaultRiskGateThreshold` (70); cannot silently lower below floor via naive config.

### I10 — Pause does not schedule new work

`runPhase` blocks on `pauseCh` before scheduling; closed channel means resumed.

## Risk gates (deterministic)

| Gate | When | Effect |
|------|------|--------|
| Preflight score | `Run` start | May block campaign |
| Advisory | if enabled & board present | Part of evaluation |
| Edge | if enabled & detector present | Part of evaluation |
| Northstar | if enabled & observer present | Phase start may fail |
| Force allow/block | mode override | Operator/test control |

Protected roots (risk heuristics): `internal/core`, `internal/mangle`, `internal/campaign`, `internal/perception`, `internal/articulation`.

## Concurrency hazards & mitigations

| Hazard | Mitigation |
|--------|------------|
| Parallel FS writes | write-set locks |
| Concurrent save | `o.mu` around save/status |
| Heartbeat save vs main | both take `o.mu` for save |
| Result map races | `resultsMu` |
| Journal seq races | `atomic.Uint64` |
| Pause race | snapshot pauseCh under lock |

## Error taxonomy (operator-facing)

Use sentinels in `errors.go` for wrapping. Task failures also assert `task_error` facts with classified types for policy/learning (`campaign_rules.mg` learning sections).

Logic failure escalation may insert diagnostic repro tasks for mutating work — prevents blind retry storms.

## Micro-checkpoints

`micro_checkpoint.go` can run per-task checks after completion paths (light verification). Does not replace phase checkpoints.

## What campaign must never do

1. Mark phase complete when tests/build checkpoint failed.  
2. Drop journal for multi-hour runs.  
3. Execute tasks without TE when TE path is the standard.  
4. Trust LLM plan JSON without kernel load/validation for non-assault campaigns.  
5. Expand assault into one task per file in large monorepos by default.

## Relation to Mangle Decl

All asserted predicates must be Declared in the loaded program. Adding new campaign facts requires schema/policy updates **outside** this package — see [09-MANGLE-SURFACE.md](09-MANGLE-SURFACE.md).
