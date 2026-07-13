# world — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for changes inside `internal/world/`.

## P1 — World is transducer, not executive

Emit facts. Do not decide `next_action` or `permitted`. Policy lives in Mangle; world only describes disk and structure.

## P2 — Topology before structure

Always be able to answer “does this path exist / hash / language / test?” even if AST fails. `file_topology` outranks symbol completeness.

## P3 — Portable identity

Prefer workspace-relative forward-slash paths in facts. Never invent machine-local identity when Rel succeeds. New emitters must use the same canonicalization helper as the full scanner.

## P4 — Layered cost

| Layer | Cost | When |
|-------|------|------|
| Topology + hash | Cheap | Always |
| Fast AST | Medium | Non-test, under size cap |
| Deep Cartographer | Expensive | On-demand / scope / campaign |
| Holographic bodies | Expensive | Priority-capped agent context |

Do not run deep map on every file every tick.

## P5 — Soft failure for enhancement facts

Dataflow, git, optional LSP projection must not abort topology. Log and continue (Cartographer already treats dataflow errors as non-fatal).

## P6 — Incremental is the steady state

Full scan bootstraps cache/DB. Steady-state edits go through delta retract/assert. Fingerprints use size + **UnixNano** mtime.

## P7 — Stratified language facts

CodeDOM: language-specific Stratum-0 facts + shared `code_element` shape. Cross-language meaning is **Mangle bridge rules**, not Go switch sprawl.

## P8 — Import-cycle discipline

- Prefer `internal/types` for shared Fact/GraphQuery.
- Keep `core` free of `world` imports; put bridges in `system` (e.g. `HolographicCodeScope`).
- Avoid new reverse deps from world → session/cli.

## P9 — Bounded concurrency

Semaphore before spawn on walks. Pool parsers. Cap package parse counts and caller body injection.

## P10 — Ignore poison trees by default

`node_modules`, `vendor`, `.git`, `.nerd`, build outputs are not world ground truth for agent reasoning unless explicitly configured.

## P11 — Predicate Decl before emission

New predicates need Decl in `schemas_world.mg` (or appropriate schema module) **and** consideration for `WorldPredicates` if they participate in full replace.

## P12 — Wiring audit before deletion

Before removing “unused” parsers, grep: chat helpers, init, campaign, system factory, shards, codedom, tests. This package has **dormant-looking multi-lang paths that tests exercise**.
