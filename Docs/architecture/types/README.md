# types — Architecture Corpus (`internal/types`)

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Language: Go (module `codenerd`)  
> Primary package: `internal/types/`  
> Role: **Foundational shared contracts** that break import cycles and define the fact / kernel / LLM / shard / session surface used across Cortex

## Scope

`internal/types` is **not** an executable subsystem. It is the **contract package**:

- Canonical `Fact` + `ToAtom()` bridge into Mangle AST
- Kernel / VirtualStore / GraphQuery / LearningStore interfaces (implementations live elsewhere)
- `LLMClient` plus optional capability interfaces (grounding, thinking, piggyback, cache, files)
- `ShardAgent`, `ShardConfig`, permissions, spawn priority
- `SessionContext` blackboard payload
- Safe fact-argument extractors
- Atomic kernel transaction wrappers (`KernelTx`, `TransactorOf`)
- Typed context keys for spawn priority and per-call model hints (`ctxkeys.go`)
- Operator-visibility contract for long-lived subsystems (`TransparencyManager`)
- Two **repo-wide ratchet tests** that keep fact-construction conventions and
  `KernelTransactor` conformance from decaying (`fact_conventions_guard_test.go`,
  `kernel_transactor_guard_test.go`)

This corpus documents what the package **defines**, who **implements** it, who **consumes** it, and the invariants that keep fact poisoning and import cycles out of the runtime.

It is **not** the kernel (`Docs/architecture/core/`), not Mangle syntax (`Docs/architecture/mangle/`), and not product Spec templates (`Docs/Spec/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture, inventory, deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture for the shared-types layer |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, dual-interface debt, test gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding principles for this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, conversion pipeline |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | How types wire into boot, kernel, shards, CLI |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Fact safety, panic policy, concurrency notes |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Existing tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging touchpoints (minimal by design) |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

## Verify

```powershell
go test ./internal/types/...
```

Targeted deep checks:

```powershell
go test ./internal/types/ -count=1 -v
go test ./internal/core/ -run 'KernelTx|ToAtom|Fact' -count=1
```

Build is not required for this pure library package; `go test` is the gate.

## Quality bar

Modeled on `Docs/architecture/cli/`: real path citations, conversion pipelines, dual-interface honesty, reverse-dependency evidence — **not** auto-inventory stubs.

## North-star placement

```
user input → perception → user_intent (types.StructuredIntent)
  → kernel (types.Kernel / KernelInterface) → next_action
  → VirtualStore (types.VirtualStore) / shards (types.ShardAgent)
  → articulation (SessionContext blackboard) → response
```

`types` owns the **shapes and interfaces** of that flow; it does not execute them.
