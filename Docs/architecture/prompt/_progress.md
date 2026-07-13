# prompt — Corpus rebuild progress

## 2026-07-13 — Full architecture corpus rebuild

- **Mode:** Docs only under `Docs/architecture/prompt/`  
- **Source of truth:** `internal/prompt/` (+ Mangle JIT rules in `internal/core/defaults/`)  
- **Quality bar:** `Docs/architecture/cli/` depth  

### Produced / overwritten

| File | Role |
|------|------|
| `README.md` | Scope, map, verify commands |
| `IMPLEMENTED_SPEC.md` | Flagship dense living spec |
| `00-ALIGNMENT-VISION-REVIEW.md` | North-star scores |
| `01-VISION.md` | Target vision |
| `02-CURRENT-STATE.md` | Inventory |
| `03-GAP-ANALYSIS.md` | Gaps vs reality |
| `04-ARCHITECTURAL-PRINCIPLES.md` | 12 principles |
| `05-INTERNAL-ARCHITECTURE.md` | Components + flow |
| `06-PUBLIC-API-AND-TYPES.md` | Public surface |
| `07-DEPENDENCY-MAP.md` | Up/downstream |
| `08-WIRING-AND-INTEGRATION.md` | Boot/session/shards |
| `09-SAFETY-AND-INVARIANTS.md` | Safety invariants |
| `10-TESTING-ALIGNMENT.md` | Tests |
| `11-OBSERVABILITY.md` | Stats/logs/UI |
| `12-FAILURE-MODES.md` | FM catalog |
| `13-PROMPT-JIT-DEEP-DIVE.md` | Compiler/selector/budget/resolver narrative |
| `TODO.md` | Backlog |
| `OPEN-QUESTIONS.md` | Open questions |
| `_progress.md` | This file |

### Research notes

- Deep-read: compiler, selector, budget, resolver, atoms, context, assembler, config_factory, loader, baseline, embedded, vector_searcher, predicate_selector, evolved_atoms, sync, manifest, output_mode, compiler_db, options.  
- Reverse deps: session, articulation, shards, cmd/nerd, e2e.  
- Mangle coupling: `jit_compiler.mg`, `schemas_prompts.mg`, `policy/jit_*.mg`.  
- No Go/code changes made.

### Prior thin stubs

Earlier auto-inventory style files replaced by this corpus. Legacy differently-named files rewritten as **supersession stubs** pointing at the canonical set (e.g. `06-PROMPT-JIT-SURFACE.md` → `13-PROMPT-JIT-DEEP-DIVE.md`).
