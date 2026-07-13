# init — Gap Analysis

> Last verified: 2026-07-13

## Spec / vision vs reality

| Target capability | Reality | Gap severity | Notes |
|-------------------|---------|--------------|-------|
| One-command cold start | **Implemented** | — | `nerd init` → `Initializer.Initialize` |
| Accurate multi-language detection | **Implemented** (Go/Rust/TS/Python/Java/Kotlin/C#/Elixir/Ruby) | Low | Framework field often empty (file detectors do not fully fill `Framework`) |
| Monorepo support | **Partial** | Medium | 2-level globs for language/deps; entry points mostly root-relative |
| Researcher-driven agent selection | **Removed / heuristic** | Medium (intentional) | Switch tables in `determineRequiredAgents`; not LLM-chosen |
| Deep KB research | **Partial** | Medium | Context7 when registry works; SkipResearch path; quality scores naive |
| Strategic project “soul” | **Implemented** when LLM present | Low | Falls back / warns without client |
| Interactive agent curation | **Library only** | Medium | `InteractiveAgentSelection` not called from `runInit` in `cmd_init_scan.go` |
| Type U user-defined agents | **Partial** | Medium | `ParseTypeUAgentFlag*` exist; CLI flag merge into init not evidenced in `runInit` |
| Dynamic tool generation | **Stubbed** | Medium (intentional JIT) | `generateProjectTools` logs only |
| Static tool catalog | **Implemented** | — | `GenerateToolsForProject` + JSON |
| Schema migration on re-init | **Implemented** | — | `store.MigrateAllAgentDBs` |
| Post-init validation | **Implemented** | Low | Warnings for low atom counts, not hard fail |
| Constitutional checks during init | **Not applied** | Low for scope | Out of package role; templates only |
| Project prompt atoms in JIT corpus | **Partial** | Medium | Comment: atoms may only hit `knowledge.db` |
| Analysis phase content | **Stub** | Low (by design) | Phase still in ETA list for timing honesty |
| Package-local Mangle product rules | **N/A** | — | Only generated `profile.mg` + templates; `debug_program_ERROR.mg` is dump noise |

## Priority backlog (from gaps)

### P0 — correctness / operator trust

1. Ensure embedding failure messaging is clear before mid-pipeline abort.
2. Keep validation + migration paths green on `--force` upgrades.

### P1 — product completeness

3. Wire interactive agent selection when `InitConfig.Interactive` is true.
4. Wire Type U definitions from CLI flags into `recommendedAgents` merge.
5. Ensure `populateProjectAtoms` content is visible to JIT (`corpus.db` ingest).

### P2 — enrichment quality

6. Replace atom-count quality scores with retrieval-grounded metrics or drop scores from UX.
7. Framework detection (Gin/Echo/React) beyond dep name maps.
8. Monorepo multi-language primary selection (currently max-count wins).

### P3 — cleanup

9. Remove or gitignore `debug_program_ERROR.mg` from package if accidental.
10. Split `Initialize` into phase functions for maintainability (behavior-preserving).

## Non-gaps (do not “fix” these)

| Observation | Why not a gap |
|-------------|----------------|
| Researcher shard not imported | JIT refactor by design; research tools modular |
| ToolGenerator shard not called | Explicit stub + Ouroboros note |
| No full OODA during init | Init is bootstrap, not session |
| `scan` uses `world` not all of `init` detectors | Lighter path by design |
| Preferences “learned via autopoiesis” mostly hints | Init seeds; learning is other packages |

## Gap matrix summary

```
Implemented solidly ████████████░░░░  ~70% cold-start surface
Intentional stubs   ████░░░░░░░░░░░░  analysis, tool gen
Wiring incompletes  ███░░░░░░░░░░░░░  interactive, Type U CLI
```
