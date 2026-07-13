# core — Architecture Corpus Progress

## 2026-07-13 — Full rebuild (docs-only)

**Agent task:** Rebuild `Docs/architecture/core/` to CLI-depth quality.  
**Source:** `internal/core/`, `internal/core/defaults/`, `internal/core/shards/`.  
**Constraint:** No Go/Mangle/test/code modifications.

### Produced / overwritten (authority set)

| File | Status |
|------|--------|
| `README.md` | Rebuilt |
| `IMPLEMENTED_SPEC.md` | Dense flagship rebuilt |
| `00-ALIGNMENT-VISION-REVIEW.md` | Rebuilt |
| `01-VISION.md` | New authority name |
| `02-CURRENT-STATE.md` | New authority name |
| `03-GAP-ANALYSIS.md` | New authority name |
| `04-ARCHITECTURAL-PRINCIPLES.md` | New authority name |
| `05-INTERNAL-ARCHITECTURE.md` | New authority name |
| `06-PUBLIC-API-AND-TYPES.md` | New authority name |
| `07-DEPENDENCY-MAP.md` | Rebuilt |
| `08-WIRING-AND-INTEGRATION.md` | New authority name |
| `09-SAFETY-AND-INVARIANTS.md` | New authority name |
| `10-TESTING-ALIGNMENT.md` | New authority name |
| `11-OBSERVABILITY.md` | New authority name |
| `12-FAILURE-MODES.md` | Rebuilt as failure catalog |
| `13-MANGLE-SURFACE.md` | Extra deep-dive (critical package) |
| `TODO.md` | Rebuilt |
| `OPEN-QUESTIONS.md` | Rebuilt |
| `_progress.md` | This file |

### Research notes used

- Read `kernel_types.go`, `kernel_init.go` (loadMangleFiles), `kernel_eval.go`, `kernel_facts.go` (API surface), `kernel_policy.go`
- Read `virtual_store.go`, `virtual_store_routing.go`, `virtual_store_constitution.go`, `virtual_store_types.go`, `virtual_store_actions.go`
- Read `dreamer.go`, `cortex_kernel.go`, `shards/manager.go`
- Read `defaults/schemas.mg`, `schemas_safety.mg`, `policy/constitution.mg`, `policy/dreamer.mg`, `policy/system_core.mg`
- Grepped constructors, types, reverse imports
- Compared quality bar to `Docs/architecture/cli/`

### Explicit non-actions

- Did not modify any files outside `Docs/architecture/core/`
- Did not update `internal/core/README.md` (code tree) — noted staleness in current-state doc instead

### Legacy thin filenames

Redirect stubs written for: `01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-CORE.md`, `03-GAP-ANALYSIS-CORE.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-CONSTITUTIONAL-SAFETY.md`, `06-MANGLE-SURFACE.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md` (legacy path), `09-CONSTITUTIONAL-SAFETY.md`, `09-MANGLE-SURFACE.md` → point at rebuild authority names.

### Follow-ups

- Engineering backlog remains in `TODO.md` / `OPEN-QUESTIONS.md`
- Optional later: delete redirect stubs once no external links remain
