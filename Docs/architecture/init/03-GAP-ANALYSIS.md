# init — Gap Analysis

> Last verified: 2026-08-15

## Spec / vision vs reality

| Target capability | Reality | Gap severity | Notes |
|-------------------|---------|--------------|-------|
| One-command cold start | **Implemented** | — | `nerd init` → `Initializer.Initialize` |
| Accurate multi-language detection | **Implemented** (Go/Rust/TS/Python/Java/Kotlin/C#/Elixir/Ruby) | — | `Framework` is now derived from the dependency set by `detectFrameworkFromDependencies` |
| Monorepo support | **Partial** | Low | Manifests found by a bounded walk (`findManifestFiles`, depth 4, vendor/node_modules skipped); entry points still root-relative and one primary language still wins |
| Researcher-driven agent selection | **Removed / heuristic** | Medium (intentional) | Switch tables in `determineRequiredAgents`; not LLM-chosen |
| Deep KB research | **Partial** | Medium | Context7 when registry works; SkipResearch path; atom-count proxy is now honestly labeled |
| Strategic project “soul” | **Implemented** when LLM present | Low | Falls back / warns without client |
| Interactive agent curation | **Implemented** | — | Phase 6 `curateAgents`; on by default, terminal-gated, `--no-interactive` opts out, choice saved to `agent_selection` |
| Type U user-defined agents | **Implemented** | — | `nerd init --define-agent` -> `InitConfig.TypeUAgents` -> `mergeTypeUAgents` before KB creation |
| Dynamic tool generation | **Measure-only by design** | — | Needs recorded as `missing_tool_for(/project_init, /capability)` facts; generation stays on-demand through `ExecuteOuroborosLoop` |
| Static tool catalog | **Implemented** | — | `GenerateToolsForProject` + JSON |
| Schema migration on re-init | **Implemented** | — | `store.MigrateAllAgentDBs` |
| Post-init validation | **Implemented** | Low | Warnings for low atom counts, not hard fail |
| Constitutional checks during init | **Not applied** | Low for scope | Out of package role; templates only |
| Project prompt atoms in JIT corpus | **Implemented** | — | Phase 5c ingests phase 5b's atoms into `prompts/corpus.db` after reconciliation |
| Analysis phase content | **Stub** | Low (by design) | Phase still in ETA list for timing honesty |
| Package-local Mangle product rules | **N/A** | — | Only generated `profile.mg` + templates; kernel fault dumps go to `.nerd/debug/` and are gitignored |

## Priority backlog (from gaps)

### P0 — correctness / operator trust

1. Ensure embedding failure messaging is clear before mid-pipeline abort.
2. Keep validation + migration paths green on `--force` upgrades.

### P1 — product completeness

3. Monorepo *multi-root profiles*: one `ProjectProfile` per module rather than one merged dependency set.

### P2 — enrichment quality

4. Replace the labeled atom-count population proxy with retrieval-grounded semantic metrics, or remove the legacy fields.
5. Monorepo multi-language primary selection (currently max-count wins).

### P3 — cleanup

6. Relocate session persistence types (`SessionState`, `ChatMessage`, `SessionHistory`) to `internal/session`; they are chat runtime concerns living in `init`.

## Non-gaps (do not “fix” these)

| Observation | Why not a gap |
|-------------|----------------|
| Researcher shard not imported | JIT refactor by design; research tools modular |
| ToolGenerator shard not called | Init measures needs as `missing_tool_for` facts; the kernel decides and Ouroboros builds through its audited entry point |
| No full OODA during init | Init is bootstrap, not session |
| `scan` uses `world` not all of `init` detectors | Lighter path by design |
| Preferences “learned via autopoiesis” mostly hints | Init seeds; learning is other packages |

## Gap matrix summary

```
Implemented solidly ██████████████░░  ~85% cold-start surface
Intentional stubs   ██░░░░░░░░░░░░░░  analysis phase
Wiring incompletes  █░░░░░░░░░░░░░░░  session type ownership
```
