# 04 — Architectural Principles: Northstar

> Last verified against codebase: 2026-07-13  
> Binding for `internal/northstar/` and its direct integrators

## P1 — Vision is structured data, not prompt residue

Mission, problem, vision statement, personas, capabilities, risks, requirements, and constraints live as typed fields (`Vision` in `types.go`) and SQLite rows. Callers must not treat chat history as the source of truth for vision.

## P2 — Guardian clones under lock; never leak mutable shared state

`GetVision` / `GetState` return clones (`cloneVision`, `cloneGuardianState`). Alignment paths clone vision before releasing the RLock. External packages must treat returned pointers as **private copies**.

## P3 — Soft defaults preserve availability; hard gates are explicit

| Condition | Outcome |
|-----------|---------|
| No vision | `AlignmentSkipped`, score 1.0 |
| No LLM | `AlignmentPassed`, score 0.8 |
| LLM error | `AlignmentWarning`, score 0.7 |
| Parsed score / RESULT | Thresholds → passed/warning/failed/blocked |
| Campaign start/phase with `blocked` | **Error returned** (`CampaignObserver`) |

Integrators must not assume every failed check stops the world.

## P4 — Kernel facts are projected, not dual-written by hand

Only `Vision.ToFacts()` defines the fact shape. `refreshKernelFacts` retracts known predicates then asserts. Integrators should not invent parallel `northstar_*` assert paths with different arity.

## P5 — Observers compose; Guardian owns judgment

`CampaignObserver`, `TaskObserver`, and `BackgroundEventHandler` record and decide *when* to check. Only `Guardian.CheckAlignment` owns *how* to score. Do not reimplement score parsing outside the Guardian.

## P6 — Persistence is transactional for multi-row updates

Store methods that touch both entity tables and `guardian_state` use SQLite transactions (`SaveVision`, `RecordObservation`, `RecordAlignmentCheck`, `RecordDriftEvent`, `ResolveDriftEvent`). New store APIs must keep rollups consistent.

## P7 — Thresholds are configuration, not magic numbers in call sites

`WarningThreshold` (0.7), `FailureThreshold` (0.5), `BlockThreshold` (0.3) live in `GuardianConfig`. Classification goes through `classifyScore`. Do not hardcode score bands in campaign code.

## P8 — High-impact paths are first-class triggers

Default paths include `internal/core/`, `internal/session/`, `internal/perception/`, `cmd/nerd/`, `*.mg`. Matching uses `path.Match` + prefix rules (`matchesHighImpactPath`). Config changes must remain test-covered.

## P9 — Avoid import cycles with shards via adapters

`BackgroundEventHandler` uses local `ObserverAssessment` / event field shapes; chat adapters bridge to `shards.NorthstarHandler`. Do not import `internal/shards` from `internal/northstar`.

## P10 — Logging is categorized, never silent on init without vision

No-vision init emits **Warn** with the store path (`Initialize`). Alignment outcomes log **Info**. Persist failures are **Debug** (non-fatal to the check return path) — callers still get the `AlignmentCheck` value.

## P11 — JIT for LLM-facing product behavior; library prompts are transitional

New *product* alignment/wizard phrasing should prefer `internal/prompt/atoms/northstar/`. Inline Guardian prompts are acceptable only until atomized; do not grow them into multi-page shard prose.

## P12 — Wiring audit before “unused”

`TaskObserver`, `GetAlignmentHistory`, `ResolveDriftEvent`, and `ingested_docs` may look dead from a narrow grep. Audit campaign, chat boot, and future CLI before deleting.
