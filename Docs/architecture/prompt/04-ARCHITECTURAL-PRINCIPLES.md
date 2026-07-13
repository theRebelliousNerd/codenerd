# prompt — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for contributors to `internal/prompt/` and atom authors.

## P1 — Atoms first, prose second

New LLM-facing behavior is a **prompt atom** (YAML under `atoms/` or agent `prompts.yaml`), not a new Go string constant in a shard. Config surface for tools/policies is a **ConfigAtom**, not ad-hoc allowlists in session handlers.

## P2 — Skeleton is deterministic; flesh is probabilistic

Identity, protocol, safety, methodology are **never** selected primarily by embeddings. Vector search may only influence **flesh**. Skeleton failure is **CRITICAL**; flesh failure is **degraded operation**.

## P3 — Logic selects; the model describes

Mangle rules (`selected_result`, `blocked_by_context`, …) and `CompilationContext` facts decide inclusion. Prompt text does not authorize actions — `permitted(...)` and tool allowlists do.

## P4 — Budget is absolute truth for inclusion

No atom may blow the total token budget without explicit skip+log. Polymorphism (`standard` → `concise` → `min`) is the compression path, not silent truncation of constitutional meaning without record (manifest/stats).

## P5 — Prefix-cache friendly assembly order

Static categories assemble **before** dynamic tail (`intent`, `world_state`, `context`). Do not reorder casually; it affects provider prefix-cache hit rates and prompt stability.

## P6 — Context is multi-dimensional AND within atom

Atom selectors: empty dimension = wildcard; non-empty = must match. Across dimensions, matching is conjunctive. Frameworks and world states use internal OR.

## P7 — Ephemeral kernel context must retract

`compile_context` facts asserted for a compile must be retracted when `KernelRetracter` is available. Do not leave selection facts poisoning the next turn.

## P8 — Multi-source without single-source lock-in

Embedded corpus is the always-on baseline. Project/agent DBs and evolved atoms layer on top. Features must degrade if a source is missing (except production skeleton without kernel).

## P9 — Observability is part of the product

Every compile should be explainable via `CompilationStats` and `PromptManifest`. Prefer structured fields over string-only logs for flight-recorder UX (`ui/jit_page.go`).

## P10 — Wiring before deletion

Before removing “unused” selection helpers, DB registrars, or baseline paths: grep session, articulation, shards, campaign boot, e2e. This package has intentional dual paths (JIT vs baseline).

## P11 — Tool names are contracts

ConfigAtom tool strings must match registered VirtualStore/tool names. Renames require joint updates of config_defaults, DefaultConfigAtomProvider, and tool registry.

## P12 — Mangle Decl fidelity

Any Go-emitted fact (`prompt_atom`, `atom_selector`, `vector_hit`, `compile_context`) must match arity/order in `schemas_prompts.mg` / `jit_compiler.mg`. Schema-order bugs are silent killers.
