# 03 — Gap Analysis: observability

> Last verified against codebase: 2026-07-13  
> Status: Spec vs reality  
> Package: `internal/observability/`

## 1. Method

Compare **vision** ([01-VISION.md](01-VISION.md)) and **comments/claims in source** against **wired behavior**. Prefer evidence over aspiration.

## 2. Spec vs reality matrix

| Capability | Vision / claim | Reality | Gap class |
|------------|----------------|---------|-----------|
| Boot metrics snapshot | Always log host posture | `LogStartupMetrics` in `main` | **Closed** |
| Green Tea status | Detect + warn if off | `greenTeaStatus` + WARN | **Closed** |
| Metric path liveness | Survive Go upgrades | `TestStartupMetricPaths_AllSupported` | **Closed** |
| Flight recorder ring | Process-lifetime window | `StartFlightRecorder` singleton | **Closed** |
| Feature gate | Config + env | `IsFlightRecorderEnabled` / `NERD_FLIGHTREC` | **Closed** |
| Panic dump | Persist ring on crash | `main` defer recover + `DumpFlightRecord` | **Partial** — main goroutine only |
| Dump keeps recorder running | Multiple dumps OK | Lifecycle tests + implementation | **Closed** |
| On-demand dump | `/diag flightrec` (features comment) | No caller outside panic path | **Open** — high product value |
| Tunable ring size/age | Config/flags | Hardcoded `64<<20`, `30s` in main | **Open** — low urgency |
| Workspace-aware dump dir | Dump under active `--workspace` | Uses `ws` from `Getwd` at main start | **Open** — edge-case |
| Graceful-shutdown dump | Optional final snapshot | Not wired | **Open** — optional |
| Mid-session metrics | Status/runtime sample | Not exported as second API | **Open** — optional |
| Trace retention | Rotate/clean `.nerd/traces` | None | **Open** — ops hygiene |
| Chat-recovered panic dump | All panics dump | Chat often recovers without re-panic | **Open** — design choice |
| OTEL export | Sometimes requested generically | Out of scope | **Non-gap** |
| Mangle facts for metrics | N/A | Correctly absent | **Non-gap** |
| Import core | Forbidden by leaf rule | Not imported | **Non-gap** |

## 3. Priorities

### P0 — correctness / operator trust

| Item | Rationale |
|------|-----------|
| Document panic-scope honestly | Operators must know chat recover ≠ dump |
| Do not document `/diag flightrec` as shipped | Features package comment is ahead of code |

### P1 — product completeness

| Item | Rationale |
|------|-----------|
| Wire on-demand dump (slash and/or Cobra) | Features already describes intent; API exists |
| Align dump `nerdDir` with effective workspace | Avoid traces in wrong tree when `--workspace` used |

### P2 — polish

| Item | Rationale |
|------|-----------|
| Config for MaxBytes / MinAge | Power users on constrained hosts |
| Optional dump on graceful exit | Complements panic path |
| Trace retention helper | Long-lived workspaces accumulate files |

### P3 — speculative

| Item | Rationale |
|------|-----------|
| Mid-session metrics API | Only if ops need it beyond logging |
| Integration test from `cmd/nerd` | Heavy; package tests already strong |

## 4. Non-gaps (do not “fix”)

1. **Not on OODA fact-flow** — intentional.  
2. **No Mangle Decl** — intentional.  
3. **No continuous metrics** — package header says one-shot.  
4. **No exported types** — functions-only API is sufficient.  
5. **Prompt manifest naming** — different subsystem; rename only if product confuses operators.

## 5. Completion heuristic

| Layer | Estimate |
|-------|----------|
| Core package behavior | **~95%** of leaf mission |
| Production wiring (boot + panic) | **~85%** |
| Operator-facing controls | **~40%** |
| Ops hygiene (retention/tune) | **~20%** |

**Overall living package health: high.** Gaps are integration and product surface, not missing algorithms inside `internal/observability`.
