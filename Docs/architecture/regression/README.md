# regression — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Language: Go (module `codenerd`)  
> Primary package: `internal/regression/`  
> Scale: **1** non-test Go file (138 lines); **1** test file (102 lines); **0** `.mg`

## Scope

`internal/regression` is a **lightweight YAML regression-battery harness**. It loads a workspace-local task suite (`.nerd/regression/battery.yaml` by convention), runs ordered shell tasks under a parent context with per-task timeouts, and returns structured `Result` values. Fail-fast stops the suite on the first hard failure.

It is **not**:

- the unit/integration test tree (`go test ./…`)
- campaign assault / Nemesis gauntlet orchestration (`internal/campaign`, `internal/shards/nemesis`)
- the Mangle kernel, VirtualStore, or prompt JIT surface
- a CLI subcommand (none registered as of 2026-07-13)

## Role in the north star

codeNERD separates **creative LLM work** from **deterministic executive control**. This package is pure executive machinery for *continuous, declarative workspace smoke checks*: data in (YAML), deterministic shell out, structured results back. It does not call models, assert Mangle facts, or decide policy. Ideal consumers are gauntlets, campaign stages, or operator tooling that want a cheap, file-defined gate — **none of those consumers import this package yet** (wiring gap; see [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md)).

## Source location

| Item | Value |
|------|-------|
| Package root | `internal/regression/` |
| Implementation | `internal/regression/battery.go` |
| Tests | `internal/regression/battery_test.go` |
| Mangle | none |
| Canonical battery path | `{workspace}/.nerd/regression/battery.yaml` via `DefaultBatteryPath` |

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machine |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Registration / callers (honest: none) |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, shell risk |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Existing tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging / metrics / debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild journal |

## Verify

```powershell
go test ./internal/regression/...
go test -race ./internal/regression/...
```

Manual smoke (library-only; no CLI):

```powershell
# Construct a battery.yaml under a workspace, then call from a tiny main or test:
# LoadBattery(DefaultBatteryPath(ws))
# RunBattery(ctx, battery, ws)
```

## Quality bar

Real path citations, control-flow diagrams, honest wiring gaps. No “pre-implementation 0%” banners — code exists. Depth modeled on `Docs/architecture/cli/` seriousness, scaled to a one-file library.
