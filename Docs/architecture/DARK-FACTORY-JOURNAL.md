# Dark Factory Journal

> Autonomous decisions for architecture corpus generation — 2026-07-13

## Pipeline

1. Skills `arch-propose` + `corpus-build` already ported under `.agents/skills/`.
2. Living packages use **code-grounded** honesty (not pre-impl 0% fiction).
3. Leaf packages excluded from full corpora (listed below).
4. No user checkpoints; tier/scope chosen here.

## Spine freeze (full corpora)

| Name | Source | Tier | Role |
|------|--------|------|------|
| core | `internal/core` | 3 | Mangle kernel, VirtualStore, Dreamer, fact store, shard manager plumbing |
| mangle | `internal/mangle` | 3 | Mangle engine bindings, differential evaluation, feedback loops |
| perception | `internal/perception` | 3 | NL→atoms transduction, semantic classifier, LLM clients |
| articulation | `internal/articulation` | 2 | Atoms→NL, Piggyback emitter, prompt assembly bridge |
| prompt | `internal/prompt` | 3 | JIT prompt compiler, atoms, selector, budget |
| session | `internal/session` | 2 | Clean execution loop / session executor |
| shards | `internal/shards` | 3 | Domain/system shard implementations and registration |
| campaign | `internal/campaign` | 2 | Multi-phase goal orchestration and context paging |
| config | `internal/config` | 2 | Config loading, engines, limits, user config |
| store | `internal/store` | 2 | Memory tiers / persistence stores |
| tools | `internal/tools` | 2 | Tool registry and research/tool integrations |
| cli | `cmd/nerd` | 3 | CLI entrypoints, chat TUI, campaign and system commands |

## Leaf / thin packages (index-only, folded or excluded)

Excluded from full tier corpora this run: `diff`, `types`, `sqlpragmas`, `ux`, `usage`, `verification`, `regression`, `features`, `build`, `observability`, `northstar`, `persist`, `tactile`, `retrieval`, `transparency`, `testing`, `init`, `browser`, `embedding`, `world`, `mcp`, `system`, `autopoiesis`, `context`, `logging`, `jit`.

Rationale: acceptance criterion spine set is binding; leaves are non-goals.

## Generation results

| Corpus | Status | src | tests | mg | heuristic % |
|--------|--------|----:|------:|---:|------------:|
| core | Realized — living code | 78 | 107 | 129 | 88 |
| mangle | Realized — living code | 21 | 39 | 1 | 90 |
| perception | Realized — living code | 49 | 47 | 0 | 85 |
| articulation | Realized — living code | 8 | 7 | 0 | 85 |
| prompt | Realized — living code | 25 | 32 | 0 | 90 |
| session | Realized — living code | 6 | 14 | 0 | 90 |
| shards | Realized — living code | 18 | 24 | 1 | 90 |
| campaign | Realized — living code | 44 | 29 | 1 | 85 |
| config | Realized — living code | 17 | 4 | 0 | 70 |
| store | Realized — living code | 39 | 44 | 0 | 90 |
| tools | Realized — living code | 25 | 21 | 0 | 85 |
| cli | Realized — living code | 113 | 55 | 2 | 70 |

## Deviations from pure pre-impl arch-propose

- Used code-grounded mode for existing `internal/**` and `cmd/nerd` instead of 0% pre-impl banners.
- corpus-build used for surface language/scripts, not full runtime recode.

