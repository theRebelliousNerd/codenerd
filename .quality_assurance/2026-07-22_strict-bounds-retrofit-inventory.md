# Strict-Bounds Retrofit Inventory (kernel type enforcement)

**Date:** 2026-07-22
**Status:** Inventory only — retrofit NOT started. The kernel still analyzes with `NoBoundsChecking`.

## Why this matters

`rebuildProgram` (internal/core/kernel_eval.go) calls `analysis.AnalyzeOneUnit`, which resolves to
`AnalyzeAndCheckBounds(..., NoBoundsChecking)`. Declared `bound [...]` blocks are therefore **never
enforced**: a String asserted into an Atom-bounded slot is accepted and silently fails to unify with
`/name` joins — the classic silent-join-failure class (see the Top-30 Mangle errors list, #1).
`TestRuleCourt_AtomStringDissonance` (internal/core/rule_court_gaps_test.go) pins this lax behavior
as a canary and documents the upgrade path.

Flipping to `analysis.ErrorForBoundsMismatch` would eliminate that entire bug class at analyze time
— but requires every `Decl` in the embedded corpus to carry a `bound` block, and will then surface
latent rule/decl type mismatches that each need case-by-case fixes.

## Trial result (2026-07-22)

Flipping the mode directly fails boot immediately:

```text
kernel failed to boot embedded constitution: failed to analyze program:
no bound decls in {checkpoint_due() ...}
```

Even zero-arity Decls need an explicit `bound []`.

## Debt size

- **1,507** `Decl` lines across `internal/core/defaults/**/*.mg`
- **224** lacked any `bound` block at inventory time
- **Step 1 already done (2026-07-22):** all 54 zero-arity Decls got `bound []` (inert under
  NoBoundsChecking, boot-verified). **170 typed Decls remain** for the retrofit campaign.

Top offenders (count / file):

| Count | File |
| ----- | ---- |
| 30 | internal/core/defaults/reviewer.mg |
| 25 | internal/core/defaults/chaos.mg |
| 14 | internal/core/defaults/schemas_coder.mg |
| 13 | internal/core/defaults/tester.mg |
| 13 | internal/core/defaults/taxonomy.mg |
| 13 | internal/core/defaults/jit_compiler.mg |
| 13 | internal/core/defaults/benchmarks.mg |
| 11 | internal/core/defaults/policy/jit_logic.mg |
| 10 | internal/core/defaults/schemas_shards.mg |
| 8 | internal/core/defaults/policy/schemas_perception_latency.mg |
| 7 | internal/core/defaults/policy/taxonomy_inference.mg |
| 7 | internal/core/defaults/policy/codedom_safety.mg |

(remaining ~60 spread across ~20 files; regenerate the full list with
`grep -rn "^Decl " internal/core/defaults --include="*.mg" | grep -v bound`)

## Retrofit plan (future session / campaign)

1. **Mechanical pass:** add `bound []` to all zero-arity Decls (safe, inert under NoBoundsChecking).
2. **Typed pass:** for each remaining Decl, derive arg types from producers (Go assert sites and
   rule heads) — `/name` for atoms, `/string`, `/number`. Wrong bounds fail boot loudly, so verify
   with a kernel-boot test per batch (any `TestRuleCourt_*` boots the full embedded corpus).
3. **Flip trial:** switch `AnalyzeOneUnit` → `AnalyzeAndCheckBounds(..., ErrorForBoundsMismatch)`
   behind a temporary local build and fix surfaced mismatches until boot + `go test ./internal/core
   ./internal/system` is green.
4. **Land the flip** and rewrite `TestRuleCourt_AtomStringDissonance` to assert the rejection
   (the canary is designed to fail when this lands — that failure is the reminder).

This is a good candidate for a codeNERD campaign (`/campaign` audit type) — mechanical, verifiable
per-batch, and the payoff is kernel-wide type enforcement.
