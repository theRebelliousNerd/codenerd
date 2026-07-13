# 04 — Architectural Principles: mangle

> Binding principles for `internal/mangle` and its subpackages. Violations should be called out in review.

## P1 — Logic is executive; this package is substrate

`internal/mangle` evaluates and validates. It does **not** decide user goals, pick tools, or grant permissions. Permission remains `permitted(...)` in core policy (default deny).

## P2 — Decl before use

No fact insert, no query, no learned body predicate without a declaration in the loaded program. Undeclared predicates fail closed (`predicate not declared`, SchemaValidator errors).

## P3 — Forbidden heads stay core-owned

Learned / LLM rules must never define constitutional or pipeline-spoof predicates (`permitted`, `safe_action`, `pending_action`, …). Enforced in `SchemaValidator.forbiddenLearnedHeads`.

## P4 — Gas limits on inference

Every production eval path should bound derived facts. Engine exposes `DerivedFactsLimit`; kernel full path uses `WithCreatedFactLimit`. Diff path must not silently omit limits once option-forwarding lands (track as gap until then).

## P5 — Serialize ANTLR parse globally

All parse entry points go through `ParseUnit` / `ParseAtom`. No package may call `parse.Unit` / `parse.Atom` in concurrent contexts without that lock. New code must not reintroduce unlocked parse.

## P6 — LLM output is untrusted until validated

Generation flows: pre-validate → sanitize → HotLoad → schema gate → budgeted retry. Never hot-load raw model text into the live constitution without this pipeline (or equivalent).

## P7 — Prefer structured synthesis over freehand syntax

When models emit logic, prefer `mangle_synth_v1` (`SynthModePrefer` / `Require`) so syntax is compiled, not hallucinated.

## P8 — Differential is opt-in and correctness-gated

Diff evaluation is a performance feature. Correctness gates (externals, provenance, policy rebuild, retract) may force full eval. Do not weaken gates to chase benchmarks.

## P9 — Encoding skew is a bug class

Fact→Atom conversion must be consistent on a given path. Kernel uses `ApplyAtomDelta` + `types.Fact.ToAtom()` to avoid Engine auto-atomizer skew. Document any new conversion path.

## P10 — Stratification choices are performance contracts

The 2-bucket scheme in DifferentialEngine is intentional. Changing it requires re-benchmarking kernel differential tests, not style preference.

## P11 — JIT predicates for feedback, not entire schemas in prompts

FeedbackLoop should use `PredicateSelectorInterface` when available; dump-all-predicates is a fallback only (capped).

## P12 — No product-specific sibling-platform/foreign-product-surface terms

Keep this package general-purpose for codeNERD’s logic substrate. Client-app-specific features stay out of mangle core.
