# core — Architecture Corpus (`internal/core`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go + embedded Mangle (`//go:embed defaults/…`)  
> Primary package: `internal/core/` (+ `defaults/`, `shards/`)

## Scope

This corpus documents the **codeNERD executive core**:

- **Mangle kernel** (`RealKernel`, optional `CortexKernel`)
- **VirtualStore** (effect / FFI gateway for `next_action`)
- **Dreamer** (speculative fail-closed safety)
- **Embedded schemas & policy** (`defaults/schemas*.mg`, `defaults/policy/*.mg`)
- **Shard manager plumbing** (`internal/core/shards`)
- Supporting systems: API scheduler, validators, shadow mode, TDD loop, tools, transactions

It is **not** the CLI surface (`Docs/architecture/cli/`), **not** the session OODA loop (`Docs/architecture/session/`), and **not** domain shard implementations (`internal/shards/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target architecture for core |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | On-disk inventory & hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types & constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, DI, reverse deps |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Constitutional + concurrency invariants |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, goldens, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging, audit, glass box, dumps |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [13-MANGLE-SURFACE.md](13-MANGLE-SURFACE.md) | Schemas, policy, Decl surface deep-dive |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

### Legacy filenames (superseded)

Older auto-inventory names (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-CORE.md`, `05-CROSS-SYSTEM-WIRING.md`, …) may still exist on disk. **Prefer the map above.** This rebuild is the authority as of 2026-07-13.

## North star (core’s job)

| Role | Owner |
|------|--------|
| Creative center | LLM (outside this package) |
| Executive | Mangle kernel + policy |
| Effects | VirtualStore under `permitted` + Dreamer |
| Default deny | `permitted(...)` must be derived |

Fact-flow:

```
user_intent → kernel evaluate → next_action / permitted
  → VirtualStore.RouteAction → result facts → articulation
```

## Verify

```powershell
# Package tests
go test ./internal/core/...

# Policy goldens (embedded logic)
go test ./internal/core/defaults/policy/...

# Build binary that embeds core constitution (CGO for sqlite-vec)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd

# If kernel analysis fails at runtime, inspect dump:
#   debug_program_ERROR.mg  (CWD of the process)
```

## Critical paths (always real)

| Path | Why |
|------|-----|
| `internal/core/kernel_types.go` | `RealKernel`, embed FS |
| `internal/core/kernel_init.go` | Boot + `loadMangleFiles` |
| `internal/core/kernel_eval.go` | Program rebuild + evaluate |
| `internal/core/kernel_facts.go` | EDB mutations |
| `internal/core/virtual_store.go` | VS DI + inject |
| `internal/core/virtual_store_routing.go` | RouteAction pipeline |
| `internal/core/dreamer.go` | Speculative safety |
| `internal/core/cortex_kernel.go` | Multi-domain kernel |
| `internal/core/defaults/schemas.mg` | Schema index |
| `internal/core/defaults/policy/constitution.mg` | Default deny |
| `internal/core/defaults/policy/dreamer.mg` | panic_state rules |
| `internal/core/shards/manager.go` | Shard plumbing |

## Quality bar

Modeled on `Docs/architecture/cli/`: real path citations, control-flow diagrams, honest gaps, package-specific narrative — **not** thin auto-inventory stubs.
