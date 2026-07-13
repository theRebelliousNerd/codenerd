# autopoiesis — Architecture Corpus (`internal/autopoiesis`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/autopoiesis/` (+ `internal/autopoiesis/prompt_evolution/`)

## Scope

This corpus documents codeNERD’s **self-creation** subsystem: detecting when work needs campaigns, tools, or persistent agents; generating and adversarially hardening tools at runtime (Ouroboros); learning from execution quality; and evolving prompt atoms (SPL).

It is **not** the campaign orchestrator (`internal/campaign/`), not the Mangle kernel (`internal/core/`), and not VirtualStore routing (`internal/core/virtual_store.go`). Autopoiesis **feeds** those systems with facts, tools, and analysis.

## North-star fit

| Role | Owner |
|------|--------|
| Creative center (propose tool code, attacks, refinements, atoms) | LLM via `LLMClient` |
| Executive (halt, safety policy, state machine, registration facts) | Mangle engines + parent kernel via `KernelInterface` / `AutopoiesisBridge` |
| Transduction | Heuristic + LLM need detection → structured `ToolNeed` / facts |

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + flows |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, stages, data flow |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, chat, VirtualStore wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety checker, Thunderdome, bounds |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, e2e, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories, traces, stats |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Unresolved design questions |
| [_progress.md](_progress.md) | Rebuild log |

## Verify

```powershell
# Unit + package tests
go test ./internal/autopoiesis/...
go test ./internal/autopoiesis/prompt_evolution/...

# E2E contracts (kernel bridge)
go test ./tests/e2e/ -run Autopoiesis

# Build binary (if exercising CLI surfaces that boot Cortex)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

## Runtime artifact layout

```
.nerd/tools/
├── <tool_name>.go              # Generated source
├── <tool_name>_test.go         # Generated tests (when written)
├── .compiled/                  # Compiled binaries (restore on boot)
├── .learnings/                 # LearningStore persistence
├── .profiles/                  # ToolQualityProfile JSON
└── .traces/                    # ReasoningTrace JSON
.nerd/agents/<name>/
├── agent.json
├── system_prompt.md
├── triggers.json
└── memory/memory.json
```

## Related corpora

- `Docs/architecture/core/` — kernel, `AutopoiesisBridge`, VirtualStore tool hooks  
- `Docs/architecture/campaign/` — multi-phase work triggered by complexity analysis  
- `Docs/architecture/prompt/` — JIT atoms consumed/produced by prompt evolution  
- `Docs/architecture/cli/` — chat/UI surfaces (`/autopoiesis`, tool generate, Alt+A)  
- `Docs/architecture/system/` — `BootCortex` / `initAutopoiesisAndBrowser`

## Quality bar

Modeled on `Docs/architecture/cli/`: real paths, control-flow diagrams, honest gaps, no pre-impl zeroing, no invented wires.
