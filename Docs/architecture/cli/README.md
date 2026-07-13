# cli — Architecture Corpus (`cmd/nerd`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Binary: `nerd` / `nerd.exe`  
> Primary package: `cmd/nerd/` (+ `cmd/nerd/chat`, `cmd/nerd/ui`)

## Scope

This corpus documents the **codeNERD CLI surface**: Cobra command tree, interactive Bubble Tea chat TUI, UI pages, and how the binary boots Cortex (kernel, shards, perception, articulation, stores).

It is **not** a product Spec template set (`Docs/Spec/`) and **not** the kernel itself (`Docs/architecture/core/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION-CLI.md](01-VISION-CLI.md) | Product vision for the CLI/TUI |
| [02-CURRENT-STATE-CLI.md](02-CURRENT-STATE-CLI.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS-CLI.md](03-GAP-ANALYSIS-CLI.md) | Gaps vs vision |
| [04-ARCHITECTURAL-PRINCIPLES-CLI.md](04-ARCHITECTURAL-PRINCIPLES-CLI.md) | Design principles |
| [05-COMMAND-ARCHITECTURE.md](05-COMMAND-ARCHITECTURE.md) | Cobra command tree |
| [06-TUI-CHAT-SURFACE.md](06-TUI-CHAT-SURFACE.md) | Bubble Tea chat + slash commands |
| [07-UI-PAGES-AND-OUTPUT.md](07-UI-PAGES-AND-OUTPUT.md) | ui/ package + rendering |
| [08-DEPENDENCY-MAP.md](08-DEPENDENCY-MAP.md) | Imports into internal/* |
| [09-CONSTITUTIONAL-SAFETY.md](09-CONSTITUTIONAL-SAFETY.md) | Safety at the CLI edge |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests and gaps |
| [11-CROSS-SYSTEM-WIRING-JOURNAL.md](11-CROSS-SYSTEM-WIRING-JOURNAL.md) | Boot → kernel → shards wiring |
| [12-TELEMETRY-OBSERVABILITY.md](12-TELEMETRY-OBSERVABILITY.md) | Logging, glass box, transparency |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Build / run

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
./nerd.exe                    # interactive TUI
./nerd.exe init               # workspace
./nerd.exe run "explain main" # single-shot OODA
go test ./cmd/nerd/...
```

## Quality bar

Modeled on Vectryx `docs/architecture/cli`: real file inventories, command tables, control-flow diagrams, wiring journals, and honest gaps — **not** auto-generated inventory stubs.
