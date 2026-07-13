# northstar — Architecture Corpus (`internal/northstar`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/northstar/`  
> Scale: **4** non-test Go files ≈ **2,196** lines; **6** test files ≈ **3,135** lines; **0** `.mg` in-package

## Scope

This corpus documents the **Northstar Guardian** subsystem: project vision storage, LLM-backed alignment checks, observation/drift persistence, campaign and task observers, and Mangle fact injection for vision predicates (`northstar_mission`, `northstar_defined`, …).

It is **not**:

- The interactive Northstar **wizard** UI (`cmd/nerd/chat/northstar_*.go`) — that is a CLI/chat surface that *feeds* vision into this package (or into `.nerd/northstar.json`).
- The **prompt atom library** (`internal/prompt/atoms/northstar/`) — JIT atoms for wizard/alignment phrasing; cited here only as an integration surface.
- Campaign risk-gate policy itself (`internal/campaign/risk_scoring.go`) — that *consumes* `CampaignObserver`.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | Scores vs codeNERD north star |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision for this package |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat, campaign, CLI wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, soft vs hard gates |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging category and debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Quick facts

| Property | Value |
|----------|-------|
| Role in fact-flow | Side-channel guardian: asserts vision facts into kernel; alignment is **not** the primary OODA path |
| Durable store | `.nerd/northstar_knowledge.db` (SQLite via `NewStore`) |
| Wizard/CLI artifact | `.nerd/northstar.json` / `.nerd/northstar.mg` (chat + `cmd_northstar.go`; **parallel** to DB) |
| Logging | `logging.CategoryNorthstar` (`"northstar"`) |
| Kernel predicates | `northstar_mission`, `northstar_problem`, `northstar_vision`, personas/needs, capabilities, risks, requirements, constraints, `northstar_defined` |

## Verify

```powershell
go test ./internal/northstar/...
go test -race ./internal/northstar/...
```

Build (when binary needed for CLI northstar commands):

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
./nerd.exe northstar show
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring journals, honest dual-store and soft-gate gaps — **not** auto-generated type tables alone.
