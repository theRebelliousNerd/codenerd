# Progress — Autopoiesis architecture corpus

## 2026-07-13 — Full rebuild (subagent contract)

Rebuilt `Docs/architecture/autopoiesis/` to the **cli/** quality bar per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`.

### Research performed

- Listed `internal/autopoiesis/` and `prompt_evolution/`  
- Read package README, modular split note, types, orchestrator, Ouroboros, checker, delegation, analysis, tools, feedback, kernel, runtime registry, thunderdome/panic, quality/patterns, compiler, Yaegi, profiles/traces, complexity/persistence  
- Grepped exports, reverse imports, factory/chat/process wiring  
- Skimmed prior thin corpus (replaced)

### Documents written (required set)

- README.md  
- IMPLEMENTED_SPEC.md (flagship)  
- 00-ALIGNMENT-VISION-REVIEW.md  
- 01-VISION.md  
- 02-CURRENT-STATE.md  
- 03-GAP-ANALYSIS.md  
- 04-ARCHITECTURAL-PRINCIPLES.md  
- 05-INTERNAL-ARCHITECTURE.md  
- 06-PUBLIC-API-AND-TYPES.md  
- 07-DEPENDENCY-MAP.md  
- 08-WIRING-AND-INTEGRATION.md  
- 09-SAFETY-AND-INVARIANTS.md  
- 10-TESTING-ALIGNMENT.md  
- 11-OBSERVABILITY.md  
- 12-FAILURE-MODES.md  
- TODO.md  
- OPEN-QUESTIONS.md  
- _progress.md (this file)

### Constraints honored

- Docs only under `Docs/architecture/autopoiesis/`  
- No Go/Mangle/test edits  
- No pre-impl “zero code” claims  
- Real paths and honest dual-path / partial wires  

### Note on legacy filenames

Older corpus used names like `01-DOMAIN-MODEL.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md`, `09-CONSTITUTIONAL-SAFETY.md`, `09-MANGLE-SURFACE.md`. The **canonical map is README.md**; prefer the numbered set above if both exist.
