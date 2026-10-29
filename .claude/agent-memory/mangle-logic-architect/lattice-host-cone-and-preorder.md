---
name: lattice-host-cone-and-preorder
description: Verified 2026-10-28 - cone evaluation dropped merge/deferred rules; analysis never orders strata by a merge predicate; merges must be total preorders; codeNERD first production lattice
metadata:
  type: project
---

Verified on mangle-go v0.7.0, 2026-10-28:
- The engine runs merge predicates and `deferred()` premises top-down from `predToRules`, built from the ProgramInfo it is handed. codeNERD's cone evaluation handed only the cone's rules, so a lattice kept every row as incomparable on the cone path (full eval was right). Fixed: `coneIndex.topDown` in internal/core/kernel_eval_cone.go + kernel_cone_plan.go; differential test `TestConeEvaluationMatchesFullEvaluation_LatticeAndDeferred` fails without it.
- ENGINE trap: analysis stratifies by rule bodies only and never looks at a merge predicate (no `MergePredicate` in analysis/). A merge that reads a derived predicate can run before it exists and keep every tied row (lab merge_strata_test.go: cl_best stratum 0, cl_rank stratum 1). Fix: name what the merge reads in the lattice rule's body. In codeNERD's full eval this showed as a 1-in-4 flaky divergence (map-order strata).
- A merge that answers both ways (non-functional rank table) keeps an order-dependent row: cone vs full diverged until the merge was a total preorder.
- `fn:Pair(/string, /number)` as a lattice column bound passes strict checks; a higher row replaces two tied rows in any arrival order; the engine mutates the incoming fact in place during merge (engine/seminaivebottomup.go mergeDelta).
- Lattice works on every in-memory store in isolation (lab stores_test.go).

**Why:** any new lattice or deferred predicate in codeNERD depends on the cone carrying top-down rules; any other embedder with incremental eval has the same trap.
**How to apply:** for a new lattice, add a cone-vs-full differential case; keep merges total preorders. See [[lattice-cycle-and-proofs]], [[jev-choice-lattice]].
