# session — Architecture Corpus (`internal/session`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/session/`  
> Scale: **6** non-test Go files ≈ **3,149** lines; **14** test files; **0** local `.mg`

## Scope

This corpus documents codeNERD’s **Universal Execution Loop**: the clean, JIT-driven agent runtime that replaced large hardcoded domain shards.

It covers:

1. **`Executor`** — observe → orient → JIT prompt/config → LLM ↔ tools → articulate  
2. **`Spawner` / `SubAgent`** — context-isolated parallel workers with memory compression  
3. **`TaskExecutor` / `JITExecutor`** — migration surface used by Cortex, campaigns, chat delegation  
4. **Constitutional safety** on the interactive tool path (`pending_action` / `permitted` / executive gate)

It is **not** the CLI TUI (`Docs/architecture/cli/`), not the Mangle kernel (`Docs/architecture/core/`), and not the prompt atom library (`Docs/architecture/prompt/`). Session **consumes** those systems.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + executor deep dive |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, Cortex, VirtualStore, campaign, chat |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Constitutional gate, concurrency, isolation |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Unit + e2e coverage map |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | `CategorySession` logging surface |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Fact-flow position

```
user input
  → perception.Transducer (or preset intent for delegated tasks)
  → user_intent fact asserted on kernel
  → JIT prompt + EffectiveAgentRuntimeConfig
  → LLM (native tools or Piggyback)
  → Executor tool loop
      → checkSafety → pending_action / permitted
      → InteractiveExecutiveGate (preflight + post-validate)
      → tools.Global() modular tools | Ouroboros ToolRegistry
  → articulation (surface text + control packet)
  → response + optional SessionPersister
```

## Verify

```powershell
go test ./internal/session/...
go test ./tests/e2e/ -run Session -count=1
```

Build the binary (when needed) with sqlite-vec CGO flags from root `Agents.md`.

## Quality bar

Modeled on `Docs/architecture/cli/`: real path citations, control-flow diagrams, wiring journals, dense executor behavior, honest partials — **not** inventory stubs.
