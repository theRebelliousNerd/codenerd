# Dark Factory Journal

> Complete 1:1 coverage of `internal/*` — 2026-07-13

## Decision

- **Every** root folder under `internal/` gets a full architecture corpus under `Docs/architecture/<same-name>/`.
- No leaf exclusions.
- Code-grounded honesty (living code status, not pre-impl 0%).
- Full document set per package: foundation 00–04, cross-wiring, testing, deps, failure modes, IMPLEMENTED_SPEC, governance.
- Tier 2–3 only scales optional extra cross-cuts (Mangle/prompt/safety), not whether a full corpus exists.

## Package results (37 total)

| Corpus | Source | Tier | src | tests | mg | % |
|--------|--------|-----:|----:|------:|---:|--:|
| articulation | `internal/articulation` | 2 | 8 | 7 | 0 | 85 |
| autopoiesis | `internal/autopoiesis` | 3 | 37 | 30 | 0 | 85 |
| browser | `internal/browser` | 2 | 3 | 6 | 0 | 90 |
| build | `internal/build` | 2 | 1 | 2 | 0 | 90 |
| campaign | `internal/campaign` | 3 | 44 | 29 | 1 | 85 |
| config | `internal/config` | 2 | 17 | 5 | 0 | 70 |
| context | `internal/context` | 3 | 9 | 11 | 1 | 90 |
| core | `internal/core` | 3 | 78 | 107 | 129 | 88 |
| diff | `internal/diff` | 2 | 1 | 2 | 0 | 90 |
| embedding | `internal/embedding` | 2 | 6 | 7 | 0 | 90 |
| features | `internal/features` | 2 | 1 | 3 | 0 | 90 |
| init | `internal/init` | 3 | 16 | 7 | 1 | 70 |
| jit | `internal/jit` | 2 | 1 | 1 | 0 | 90 |
| logging | `internal/logging` | 2 | 4 | 5 | 0 | 90 |
| mangle | `internal/mangle` | 3 | 21 | 39 | 1 | 90 |
| mcp | `internal/mcp` | 3 | 10 | 16 | 1 | 90 |
| northstar | `internal/northstar` | 2 | 4 | 6 | 0 | 90 |
| observability | `internal/observability` | 2 | 2 | 3 | 0 | 90 |
| perception | `internal/perception` | 3 | 50 | 48 | 0 | 85 |
| persist | `internal/persist` | 2 | 1 | 4 | 0 | 90 |
| prompt | `internal/prompt` | 3 | 25 | 32 | 0 | 90 |
| regression | `internal/regression` | 2 | 1 | 1 | 0 | 90 |
| retrieval | `internal/retrieval` | 2 | 4 | 6 | 0 | 90 |
| session | `internal/session` | 2 | 6 | 14 | 0 | 90 |
| shards | `internal/shards` | 3 | 18 | 24 | 1 | 90 |
| sqlpragmas | `internal/sqlpragmas` | 2 | 1 | 2 | 0 | 90 |
| store | `internal/store` | 3 | 39 | 44 | 0 | 90 |
| system | `internal/system` | 3 | 5 | 11 | 1 | 90 |
| tactile | `internal/tactile` | 2 | 16 | 12 | 0 | 85 |
| testing | `internal/testing` | 3 | 21 | 8 | 0 | 70 |
| tools | `internal/tools` | 3 | 25 | 21 | 0 | 85 |
| transparency | `internal/transparency` | 2 | 8 | 9 | 0 | 90 |
| types | `internal/types` | 2 | 5 | 4 | 0 | 85 |
| usage | `internal/usage` | 2 | 2 | 4 | 0 | 90 |
| ux | `internal/ux` | 2 | 4 | 4 | 0 | 90 |
| verification | `internal/verification` | 2 | 1 | 3 | 0 | 90 |
| world | `internal/world` | 3 | 37 | 31 | 1 | 85 |

## Also present (non-internal)

- `Docs/architecture/cli/` maps to `cmd/nerd/` (CLI surface; not an internal/ root).

## Deviation from earlier spine-only run

- Replaced spine-only + leaf exclusions with full 1:1 internal coverage per user correction.

