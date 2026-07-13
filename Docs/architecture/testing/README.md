# testing — Architecture Corpus (`internal/testing`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/testing/` (+ `internal/testing/context_harness/`)  
> CLI surface: `nerd test-context` (`cmd/nerd/cmd_test_context.go`)

## Scope

This corpus documents the **Context Test Harness** — the codeNERD subsystem that stress-tests infinite context (turn compression → fact storage → spreading-activation retrieval → checkpoint validation) with multi-turn coding-session simulations.

It is **not**:

- The production context stack (`Docs/architecture/context/`)
- The unit-test strategy for other packages (each package’s `10-TESTING-ALIGNMENT.md`)
- Campaign assault machinery (`.nerd/campaigns/…/assault/`)

It **is** the dual-mode (mock / real) harness that sits beside production and asks: *after N turns of simulated work, can the agent still retrieve the original error / plan / issue file?*

## Role in the north star

| North-star idea | How this package participates |
|-----------------|-------------------------------|
| Logic = executive | Scenarios assert structured facts into `*core.RealKernel`; checkpoints query via activation, not free-text RAG alone |
| LLM = creative center | Optional `--live` mode drives real assistant turns; piggyback control packets feed context-feedback learning |
| JIT prompt atoms | Tracers *observe* JIT selection (today mostly mock snapshots in the simulator path) |
| Constitutional safety | Harness does not execute agent tools; it loads facts and scores retrieval — no `next_action` / VirtualStore loop |

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision for the harness |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory (files, roles, hotspots) |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles for this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs that matter |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | CLI boot, Cortex, engine injection |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, isolation, default-deny adjacency |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Package self-tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | FileLogger, tracers, metrics, report formats |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open questions |
| [_progress.md](_progress.md) | Rebuild log |

## Source layout (quick)

```
internal/testing/
  doc.go                    # package comment only
  harness_subsystem.go      # integration-anchor comment only
  context_harness/          # ALL behavior lives here
    harness.go              # orchestrator
    simulator.go            # turn loop + checkpoints
    mock_engine.go          # fast CI engine
    real_engine.go          # ActivationEngine integration
    scenarios*.go           # mock + integration scenarios
    *tracer*.go / inspector # glass-box observability
    file_logger.go          # .nerd/context-tests sessions
    metrics.go / reporter.go
```

## Verify

```powershell
# Package unit tests (no Cortex boot required)
go test ./internal/testing/...

# Full context harness via CLI (boots Cortex; needs workspace + API key for real/live)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
.\nerd.exe test-context
.\nerd.exe test-context --scenario debugging-marathon --mode=mock
.\nerd.exe test-context --scenario campaign-phase-transition --mode=real
.\nerd.exe test-context --all --format json
```

Logs default to `.nerd/context-tests/session-<timestamp>/` (see [11-OBSERVABILITY.md](11-OBSERVABILITY.md)).

## Quality bar

Modeled on `Docs/architecture/cli/`: real path citations, control-flow diagrams, wiring journals, honest partials — **not** auto-generated inventory stubs.
