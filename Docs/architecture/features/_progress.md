# Progress — features architecture corpus

## 2026-07-13 — Full rebuild (subagent contract)

Replaced thin auto-inventory stubs with a **code-grounded** corpus matching `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md` and the quality bar of `Docs/architecture/cli/`.

### Research performed

- Read entire `internal/features/features.go` (351 lines) and all three test files  
- Grepped reverse imports of `codenerd/internal/features`  
- Grepped all `features.Is*` / `SetActive` / `Summary` call sites  
- Read config install path (`user_config.go`), kernel_eval DiffEval bridge, cortex PerShardFacts, session_boot, main FlightRecorder, world scanner, UX onboarding, UI dark mode  
- Noted TaxonomyFast half-wiring (`cmd/tools/verify_taxonomy` env-only)  
- Noted stale comments (kernel_eval default, PerShardFacts “short-circuit”, SystemShards field env)

### Documents written (required set)

| File | Status |
|------|--------|
| `README.md` | Rebuilt |
| `IMPLEMENTED_SPEC.md` | Rebuilt (flagship deep-dive) |
| `00-ALIGNMENT-VISION-REVIEW.md` | Rebuilt |
| `01-VISION.md` | New name (was domain-model stub style) |
| `02-CURRENT-STATE.md` | New name |
| `03-GAP-ANALYSIS.md` | New name |
| `04-ARCHITECTURAL-PRINCIPLES.md` | New name |
| `05-INTERNAL-ARCHITECTURE.md` | New |
| `06-PUBLIC-API-AND-TYPES.md` | New |
| `07-DEPENDENCY-MAP.md` | Rebuilt |
| `08-WIRING-AND-INTEGRATION.md` | New (replaces thin cross-system stub role) |
| `09-SAFETY-AND-INVARIANTS.md` | New |
| `10-TESTING-ALIGNMENT.md` | New |
| `11-OBSERVABILITY.md` | New |
| `12-FAILURE-MODES.md` | New (replaces thin failure stub role) |
| `TODO.md` | Rebuilt |
| `OPEN-QUESTIONS.md` | Rebuilt |
| `_progress.md` | This file |

### Scope discipline

- **Only** files under `Docs/architecture/features/` modified  
- **No** Go / Mangle / test / config code changes  
- No Vectryx product terms  
- No pre-implementation 0% banners  

### Note on older filenames

Earlier thin stubs used names such as `01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-FEATURES.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md`. The rebuild contract’s canonical set uses the filenames listed above; prefer those for navigation via `README.md`.
