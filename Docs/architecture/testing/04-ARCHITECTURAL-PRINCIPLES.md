# testing — Architectural Principles

> Last verified: 2026-07-13  
> Binding for work in `internal/testing/**` and `cmd/nerd/cmd_test_context.go` changes that touch the harness.

## P1 — Dual-mode seam is the `ContextEngine` interface

All retrieval/compression behavior used by the simulator must go through `ContextEngine`.  
Do not special-case mock vs real inside the turn loop beyond mode flags already on `SimulatorConfig`.  
New production-under-test capabilities → new interface methods (or narrow collaborators), not hard forks in `executeTurn`.

## P2 — Facts are the ground truth, not free text

Validation IDs (`MustRetrieve`, `ShouldAvoid`) and seeded `InitialFacts` are structured.  
Prefer predicate-stable IDs (`turn_N_error_message` via `extractFactID`) over substring matching on full messages.  
Fuzzy match exists to absorb naming drift — it is not permission to abandon IDs.

## P3 — Mock is for CI; real is for fidelity

Mock must remain free of external LLM requirements and fast enough for `go test` / developer loops.  
Real may require Cortex, LocalStore, API keys.  
Never make mock call the network “just a little.” Live LLM is an explicit opt-in (`UseLiveLLM` / `--live`).

## P4 — Enrichment vs compression is first-class

`CompressionRatio` semantics are documented in `types.go`: values &lt; 1 mean enrichment.  
Threshold logic in `meetsExpectations` already branches on this.  
New metrics or scenarios must not silently assume “higher ratio is always better.”

## P5 — Observability is multi-channel, file-first

Glass-box debugging for multi-turn failures requires durable logs (`FileLogger`), not only stdout.  
Tracers accept `io.Writer` — keep them side-effect free beyond writing.  
When adding a new observable subsystem, add: tracer type + FileLogger writer + CLI flag + MANIFEST section.

## P6 — Harness must not become production runtime

No production package under `internal/` (except CLI wiring) should import `context_harness`.  
No agent tools, VirtualStore routes, or policy rules should live here.  
If a helper is needed in production, extract it to the domain package (`context`, `core`, …) and call it from both.

## P7 — Scenario isolation over shared ambient state

Each scenario should behave as if it owns a clean fact universe.  
Engine `Reset()`, factory kernels, and seeder clear paths exist to support this — prefer strengthening them over “hope order is fine.”  
`RunAll` failures must not cascade from leftover facts of a previous scenario.

## P8 — Checkpoint types that exist must eventually mean something

Do not add `ValidateX` fields that the simulator ignores without tracking the gap.  
Either enforce in `validateCheckpoint` or document as deferred with a TODO in this corpus — never pretend pass means validated.

## P9 — Prefer registry completeness

`GetScenario`, `AllScenarios`, `MockScenarios`, `IntegrationScenarios`, and CLI help lists must stay synchronized when scenarios are added.  
A scenario only in one list is a product bug.

## P10 — Measure the memory substrate, not the whole agent

Scope discipline: compression → store → activate → select-within-budget → score.  
Full OODA, tool permission, and multi-shard campaigns belong to other test surfaces.  
Cross-links are fine; ownership transfer is not.

## P11 — Wire before delete

Partial integrations (`TestKernelFactory` schemas, `FactSeeder.Clear`, category flag) look dead — audit CLI and tests before removal.  
codeNERD culture: fix wiring gaps first.

## P12 — Fail loud on kernel type mismatches

CLI already rejects non-`*core.RealKernel` with an explicit error (no silent nil).  
Preserve fail-loud patterns for misconfigured real/live modes.
