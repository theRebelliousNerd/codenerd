# init — Architecture Corpus (`internal/init`)

> Last verified against codebase: 2026-08-15
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/init/`  
> CLI entry: `cmd/nerd/cmd_init_scan.go` (`nerd init`, `nerd scan`)

## Scope

This corpus documents **codeNERD cold-start workspace initialization**: creating `.nerd/`, scanning the project, building a `ProjectProfile`, generating Mangle profile facts, seeding specialist knowledge bases, registering Type-3 agents, session state helpers, tool definition catalogs, and post-init validation.

It is **not** the interactive chat boot path (`cmd/nerd/chat/session_boot*.go`), **not** the world scanner implementation (`internal/world`), and **not** the kernel itself (`internal/core`). Init **uses** those packages to materialize a durable workspace identity the rest of the system boots from.

## Role in fact-flow

```
nerd init
  → Initializer.Initialize (22 phases)
  → world.Scanner + file detectors → ProjectProfile
  → profile.mg facts + knowledge.db / shard KBs
  → agents.json + prompts.yaml → later chat boot / VirtualStore

At runtime (after init):
  user input → perception → user_intent → kernel next_action
    → VirtualStore / shards (profiles registered at init) → articulation
```

Init is the **substrate bootstrap**: logic executive and specialists need a workspace, profile facts, and KBs before OODA is useful.

The completion contract is deliberately split: required artifact failures make
`InitResult.Success` false, including failed or empty structural KB validation,
while optional LLM enrichment failures are counted and reported as degraded.
The legacy `QualityScore` JSON fields are presented as atom-count population
proxies, not semantic quality.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product vision for cold-start |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Phases, components, data flow |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and functions |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | CLI, chat, session consumers |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, Mangle hygiene |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging, progress, ETA |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [09-MANGLE-SURFACE.md](09-MANGLE-SURFACE.md) | Extra: generated profile.mg + templates |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Build / verify

```powershell
# Package tests (no full binary required for pure unit coverage)
go test ./internal/init/...

# Init command via CLI (needs sqlite-vec CGO for full KB path)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
./nerd.exe init
./nerd.exe init --force
./nerd.exe scan
./nerd.exe init --cleanup-backups
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real file inventories, phase tables, control-flow diagrams, wiring journals, and honest gaps — **not** auto-generated inventory stubs.
