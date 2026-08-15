# campaign — Architecture Corpus (`internal/campaign`)

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/campaign/`  
> Policy surface (outside package): `internal/core/defaults/campaign_rules.mg`, policy Section 19, `build_topology.mg`

## Scope

This corpus documents **multi-phase campaign orchestration**: goal decomposition (LLM + Mangle validation), phase/task execution, context paging, checkpoints, adaptive replan, write-set locking, risk gating, intelligence preflight, and adversarial assault campaigns.

It is **not**:

- the single-shot OODA loop (`internal/session/`)
- the Mangle kernel itself (`internal/core/`)
- the CLI command tree (see `Docs/architecture/cli/`) — though Cobra/chat **wire** into this package

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + dense inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding principles for this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | CLI, chat, JIT, boot wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, risk gates |
| [09-MANGLE-SURFACE.md](09-MANGLE-SURFACE.md) | **Extra** — campaign predicates & rules |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging, events, journal |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

### Legacy filenames (redirects)

Older rebuild stubs used different names. Prefer the map above. Short redirects remain for:

- `01-DOMAIN-MODEL.md` → vision + internal architecture  
- `02-CURRENT-STATE-CAMPAIGN.md` → `02-CURRENT-STATE.md`  
- `03-GAP-ANALYSIS-CAMPAIGN.md` → `03-GAP-ANALYSIS.md`  
- `04-INVARIANTS-AND-GATES.md` → safety + principles  
- `05-CROSS-SYSTEM-WIRING.md` → `08-WIRING-AND-INTEGRATION.md`  
- `06-TESTING-STRATEGY.md` → `10-TESTING-ALIGNMENT.md`  

## Fact-flow placement

Campaigns sit **beside** the single-turn OODA loop as a long-horizon executive:

```
user goal /docs /assault
   → Decomposer (LLM propose) + kernel LoadFacts/validate
   → Orchestrator.Run
        → kernel Query: current_phase, eligible_task, next_campaign_task, campaign_blocked
        → ContextPager ActivatePhase / Prefetch / Compress
        → TaskExecutor / tactile (per TaskType)
        → CheckpointRunner → Replanner (on fail)
   → artifacts under .nerd/campaigns/
```

Constitutional safety for mutating work still flows through VirtualStore / `permitted(...)` when shards or tools act; the orchestrator **does not** replace the kernel.

## Build / test

```powershell
# Package tests (from repo root)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/campaign/...

# Related e2e
go test ./tests/e2e/ -run Campaign -count=1

# Operator surface
go build -o nerd.exe ./cmd/nerd
./nerd.exe campaign start "Build REST API" --docs ./specs/
./nerd.exe campaign status
# Chat: /campaign assault ...
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real paths, control-flow diagrams, wiring journals, honest partials — **not** auto-inventory stubs. IMPLEMENTED_SPEC is the dense living source of truth for this package.
