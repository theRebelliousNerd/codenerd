# Dark Factory Journal

> Architecture corpus rebuild — docs only — 2026-07-13

## Decision

- **Every** root folder under `internal/` has a matching full architecture corpus under `Docs/architecture/<same-name>/`.
- **cli/** covers `cmd/nerd/` (non-internal) as the original deep-dive quality reference.
- **No application code** was modified during the deep rebuild wave (docs under `Docs/architecture/**` only).
- Quality bar = deep narrative corpora (control flows, wiring honesty, gaps), not auto-inventory stubs.

## Rebuild method (1:1 subagents)

1. Shared contract: `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`
2. **37** general-purpose subagents, one per `internal/*` package, in parallel batches
3. Each agent: research source → overwrite `Docs/architecture/<pkg>/` with full doc set + dense `IMPLEMENTED_SPEC.md`
4. Orchestrator: verify 1:1, size floor, docs-only git status → commit

## Package results (post deep rebuild)

| Package | IMPLEMENTED_SPEC (approx bytes) | Notes |
|---------|--------------------------------:|-------|
| core | 34904 | Kernel, VirtualStore, Dreamer |
| session | 29637 | Executor OODA / tool loop |
| mangle | 27087 | Engine, differential, feedback |
| campaign | 26180 | Orchestrator, assault |
| store | 25755 | Memory tiers |
| world | 25025 | FS topology + AST |
| perception | 24208 | Transducer + clients |
| prompt | 24108 | JIT compiler + atoms |
| autopoiesis | 22937 | Ouroboros |
| shards | 22859 | Registration + system OODA |
| context | 22571 | Activation / window |
| testing | 22372 | Context harness |
| embedding | 21843 | Ollama / GenAI |
| config | 21429 | UserConfig / engines |
| tools | 21378 | Tool registry |
| system | 21263 | GetOrBootCortex |
| tactile | 20620 | Motor cortex / sandbox |
| articulation | 19965 | Piggyback emitter |
| logging | 18216 | Categories + audit |
| cli | 18168 | cmd/nerd reference |
| browser | 17677 | Session manager |
| build | 17427 | Build env factory |
| verification | 16936 | TaskVerifier |
| regression | 16878 | Battery (dormant wire) |
| sqlpragmas | 16642 | SQLite profiles |
| types | 16449 | Shared contracts |
| retrieval | 16215 | Sparse / tiered context |
| transparency | 16178 | Glass box / manager |
| diff | 15987 | Diff engine |
| mcp | 15870 | MCP client / JIT tools |
| persist | 15449 | factsnap (dormant wire) |
| init | 15141 | Cold-start phases |
| features | 15026 | Flag registry |
| observability | 14906 | Metrics + flightrec |
| ux | 14363 | Preferences / journey |
| jit | 13287 | Runtime config types |
| northstar | 12302 | Guardian / alignment |
| usage | 12243 | Token tracker |

**Zero packages below 8KB IMPLEMENTED_SPEC** after deep rebuild.

## Explicit out of scope this wave

- Implementing wiring gaps found in docs
- Filling Docs/Spec 18-file product templates
- Changing Go or Mangle sources

## Regenerate / extend

- New `internal/<pkg>` → spawn one subagent with `_rebuild/SUBAGENT_INSTRUCTIONS.md`
- Quality bar remains `Docs/architecture/cli/` + densest packages (core/session/mangle)
