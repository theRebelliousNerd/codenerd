# perception — Architecture Corpus (`internal/perception`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/perception/` (+ `internal/perception/xaioauth/`)

## Scope

This corpus documents the **perception layer**: natural-language → structured intent transduction, semantic (embedding) classification with Mangle fact injection, multi-provider LLM clients, taxonomy learning, and tracing wrappers that feed the OODA fact-flow.

It is **not** the kernel (`Docs/architecture/core/`), **not** articulation/Piggyback emission (`Docs/architecture/articulation/`), and **not** the CLI surface (`Docs/architecture/cli/`).

### Role in fact-flow

```
user input → perception (NL→Intent / Understanding)
  → user_intent fact → kernel next_action
  → VirtualStore / shards / tools → articulation → TUI/stdout
```

Philosophy (from code): **LLM describes → Harness determines.**

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs that matter |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat, kernel, shards wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, fact sanitization |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories, metrics, debug |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

### Superseded thin stubs (names)

Earlier auto-inventory files used alternate names (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-PERCEPTION.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md`). **Canonical names are the table above.** Prefer the numbered docs listed here.

## Verify

```powershell
go test ./internal/perception/...
go test ./internal/perception/xaioauth/...
# Optional: e2e perception contracts
go test ./tests/e2e/ -run Perception -count=1
```

Package-local README (operator notes, slightly stale defaults): `internal/perception/README.md`.

## Quality bar

Modeled on `Docs/architecture/cli/`: real path citations, control-flow diagrams, honest gaps, dense IMPLEMENTED_SPEC — **not** generic inventory stubs.
