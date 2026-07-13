# features — Architecture Corpus (`internal/features`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded full corpus  
> Language: Go (module `codenerd`)  
> Primary package: `internal/features/`  
> Scale: **1** non-test Go file (351 lines); **3** test files; **0** Mangle sources

## Scope

This corpus documents the **leaf-level feature-toggle registry** that lets modernization flags (DifferentialEngine, FlightRecorder, Provenance, system shards, dark mode, scan tunables, …) be read from low-level packages **without** importing `internal/config`.

It is **not**:

- the full user-config surface (`Docs/architecture/config/`)
- the kernel evaluation engine itself (`Docs/architecture/core/`)
- product Spec templates under `Docs/Spec/`

## Why this package exists

`internal/config` pulls in store/logging/world-adjacent deps. Core, world, CLI boot, and UX need flag reads on hot or early paths. Putting toggles in a **zero-internal-import leaf** breaks the cycle:

```
internal/config (LoadUserConfig)  ──SetActive──►  internal/features
internal/core | world | ux | cmd/nerd  ──IsXXX()──►  internal/features
```

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture for feature toggles |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, resolve flow, state |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and accessors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, kernel, consumer call sites |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, leaf purity, safety |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Boot Summary, logging ownership |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

## Verify

```powershell
go test ./internal/features/...
go test ./internal/core/ -run Features
# Full binary (when CGO headers present):
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real path citations, control-flow diagrams, honest gaps, package-specific narrative — **not** thin auto-inventory stubs.
