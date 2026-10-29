# world — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living reference — **code-grounded full corpus**  
> Mode: 1:1 with `internal/world/` (+ `internal/world/lsp/`)  
> **Implementation: ~37 non-test `.go`, ~31 tests, 0 package-local `.mg`** (schemas live in `internal/core/defaults/schemas_world.mg`)

## Role

World model: **filesystem topology + AST / holographic / CodeDOM projection** into Mangle EDB facts. Disk and source are the ground truth; the kernel reasons over the atoms this package emits.

## North-star fit

| Role | Who |
|------|-----|
| Creative center | LLM (uses holographic context, not raw trees) |
| Executive | Mangle kernel / `permitted(...)` |
| Transduction | **`internal/world`** (this package) |

## Source location

- Primary: [`internal/world/`](../../../internal/world/)
- LSP subpackage: [`internal/world/lsp/`](../../../internal/world/lsp/)
- Schema Decl surface: [`internal/core/defaults/schemas_world.mg`](../../../internal/core/defaults/schemas_world.mg)
- Heuristic implementation completeness: **~85–90%**

## Document set

| Doc | Purpose |
|-----|---------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living spec (FS topology, AST, integration) |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores + evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise inventory, hotspots, line roles |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream imports |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, shards, VirtualStore bridges |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, caps |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging category, timers, debug |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [13-MANGLE-SURFACE.md](13-MANGLE-SURFACE.md) | Predicates, Decl sources, replace-set |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Open design questions |
| [_progress.md](_progress.md) | Rebuild log |

### Superseded names (old corpus)

Earlier thin stubs used package-suffixed names (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-WORLD.md`, …). Prefer the table above; superseded files redirect if still present.

## Critical concepts

1. **`file_topology/5`** — portable path identity + SHA-256 + language + nano mtime + test flag  
2. **Fast vs deep AST** — tree-sitter symbols on scan vs Cartographer `code_defines`/`code_calls` on demand  
3. **`ApplyIncrementalResult`** — retract/replace world EDB without full reboot  
4. **FileScope + HolographicCodeScope** — 1-hop CodeDOM + deep facts without core↔world import cycles  
5. **HolographicProvider** — agent-facing package X-ray + impact-prioritized callers  

## Verify

```powershell
go test ./internal/world/...
go test ./internal/world/lsp/...
```

Build binary (if needed) per root `Agents.md` sqlite-vec CGO flags.

## Related packages

| Package | Relation |
|---------|----------|
| `internal/core` | Kernel LoadFacts/Retract; schemas_world.mg |
| `internal/store` | Per-file world fact cache |
| `internal/system` | HolographicCodeScope, factory Scanner |
| `internal/shards/system` | world_model_ingestor |
| `internal/campaign` | Scanner + holographic intelligence |
| `internal/init` | Initial workspace scan |
| `cmd/nerd` | scan, chat sync, campaign, mangle-lsp |
