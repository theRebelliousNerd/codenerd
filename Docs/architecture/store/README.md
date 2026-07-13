# store — Architecture Corpus (`internal/store`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/store/`  
> Role: **Multi-tier durable memory** (SQLite + optional sqlite-vec)

## Scope

This corpus documents codeNERD’s **store package**: `LocalStore` memory tiers (vector, graph, cold/archival, session, world, knowledge/prompt atoms, traces), plus satellite stores (`LearningStore`, `ToolStore`, embedded/learned corpora), migrations, reflection re-embed, and integration with boot / VirtualStore / world / prompt.

It is **not** the Mangle kernel, **not** the embedding engine implementation, and **not** a product Spec template set (`Docs/Spec/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture — dense memory-tier deep dive |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture vision for store |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, VirtualStore, CLI wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, durability, safety bounds |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, commands, gaps |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging, timers, debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress |

## Memory tier cheat-sheet

| Tier | Primary home | Notes |
|------|--------------|-------|
| Vector (B) | `knowledge.db` `vectors` + optional `vec_index` | Semantic + keyword |
| Graph (C) | `knowledge_graph` | Relational links + hydrate |
| Cold / archival (D) | `cold_storage` / `archived_facts` | Access-tracked facts |
| Session | `session_history`, `compressed_states`, `activation_log` | Continuity |
| World | `world_files`, `world_facts` | Fast/deep AST cache |
| Atoms | `knowledge_atoms`, `prompt_atoms` | Agent KB + JIT disk |
| Traces / verify | `reasoning_traces`, `task_verifications` | Self-learning |
| Learnings | `.nerd/shards/*_learnings.db` | Autopoiesis |
| Tools | `.nerd/tools.db` | Full tool I/O journal |
| Corpora | embed RO + learned RW | Intent classification |

## Verify

```powershell
# Package tests
go test ./internal/store/...

# Binary with sqlite-vec (PowerShell)
if (Test-Path .\nerd.exe) { Remove-Item .\nerd.exe -ErrorAction SilentlyContinue }
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

## Quality bar

Modeled on the CLI architecture rewrite depth: real path citations, tier diagrams, control-flow, wiring evidence, and honest gaps — **not** auto-inventory stubs.
