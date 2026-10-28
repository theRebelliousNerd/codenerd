---
doc-class: deep-dive
subsystem: core-mangle
implementation-status: partial
last-verified: 2026-10-28
verified-against: 2e0c54ef204997c36e65c91cbb90bc991e63c8c1
verification-method: static-source-inspection
supersedes: []
---

# Mangle features CodeNERD should actually adopt: executable contracts before more syntax

Date: 2026-10-28. Status: source-grounded design review; **no implementation, test execution, training, or measured performance result**. This is an adoption decision document, not a claim that every available Mangle feature belongs in the kernel.

## 1. Decision in brief

Prioritize three bounded improvements:

1. **Make declarations executable at the host boundary:** typed atom admission that preserves alternative bound tuples and nested types, plus strict static bounds checking for explicitly owned modules.
2. **Use inclusion constraints where the domain is actually closed:** catch dangling producer/consumer references at an epoch or batch boundary, not during arbitrary arrival order.
3. **Finish the proof-data contract before teaching policies to consume proofs:** CodeNERD already records derivations; a second read-only explanation program needs epoch identity, partial-status preservation, origin mapping, and size limits.

Do not enable temporal syntax, deferred evaluation, or merge-based state replacement merely because upstream documents them. The first question is which host path can preserve their semantics. This also applies to familiar features: a parser accepting a declaration is not proof that runtime facts are checked against it.

## 2. Evidence and scope

CodeNERD inspected revision: [`2e0c54ef204997c36e65c91cbb90bc991e63c8c1`](https://github.com/theRebelliousNerd/codenerd/tree/2e0c54ef204997c36e65c91cbb90bc991e63c8c1). Its [`go.mod`](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/go.mod#L9) selects `codeberg.org/TauCeti/mangle-go v0.7.0`. Upstream source references below use the official Google mirror's v0.7.0 commit [`7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`](https://github.com/google/mangle/tree/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb); [the upstream move announcement](https://github.com/google/mangle/discussions/89) explains the Codeberg home and GitHub mirror.

Read this as a delta to the existing [M0 language study](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/journeys/M0-mangle-language-surface.md), [M1 corpus study](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/journeys/M1-mangle-corpus-usage.md), [M2 programming model](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/journeys/M2-mangle-programming-model.md), and [M3 verification report](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/journeys/M3-mangle-verification.md), not a replacement. Those studies contain valuable historical experiments under an older dependency. Their experimental results are not rerun here and must not be silently transferred to v0.7.0.

Negative evidence is deliberately bounded: inspected production entry points below do not call the upstream runtime checkers. Repository search for `CheckTypeBounds`, `NewInclusionChecker`, and `EmitFacts` was supporting reconnaissance, not a complete absence proof; the search index returned an earlier `66ccd529` revision. Critical adapter observations were rechecked directly at the inspected revision.

Freshness check at 02:02 UTC: main advanced from `6ef257c` to `2e0c54e` during this pass. The intervening changes update fact-index maintenance, boot deduplication and context scoring/rendering. The dependency and six central files underpinning this feature review (`kernel_undeclared.go`, `kernel_fact_decl.go`, `kernel_provenance.go`, `kernel_eval.go`, `internal/mangle/engine.go`, and `go.mod`) were fetched again at the new pin; the relevant contracts are unchanged. Those concurrent performance changes are not presented here as missing features. This remains a pinned review, not a claim about later commits.

## 3. Adoption ledger

| Feature | Upstream support at v0.7.0 | Inspected CodeNERD state | Decision |
|---|---|---|---|
| Rich runtime bound checking | `builtin.TypeChecker` checks constants against each alternative tuple | RealKernel's validator reflects over declarations and checks primitive Go `/number` and `/string` shapes; not equivalent | First priority, scoped migration |
| Static bounds checking | `AnalyzeAndCheckBounds` with `ErrorForBoundsMismatch` | Both RealKernel and `internal/mangle.Engine` call `AnalyzeOneUnit`, whose upstream default is `NoBoundsChecking` | Shadow checks, then enforce for owned rule modules |
| Inclusion constraints | `engine.InclusionChecker.CheckFact` checks grounded membership/equality constraints | No integration found in inspected admission/evaluation entry points | Pilot on immutable evidence/catalog batches |
| Structured values and alternative tuples | Declarations and bound handles support richer types | Conversion/coercion is adapter-specific; first tuple is used in relevant conversion paths | Use only after typed boundary contract; not as opaque replacement for indexed relations |
| Full derivation recording | Recorder plus `BuildFromRecording` and `EmitFacts` | RealKernel already exposes enable/disable and `Explain`; disabled by default | Do not call recorder an unused feature; finish a bounded downstream consumer |
| Temporal evaluation | Dedicated upstream temporal interfaces and evaluation options | Inspected RealKernel option assembly does not install a temporal store | Deferred: separate storage, clock, lifecycle and completeness design first |
| Deferred/merge-based evaluation | Covered in upstream and historical M0/M3 studies | Historical failures exist; no fresh execution here | No blanket recommendation; qualify exact binding, convergence and replacement semantics first |

Sources: [runtime checker](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/builtin/builtin.go#L863-L925), [static analyzer](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/analysis/validation.go#L31-L105), [inclusion checker](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/inclusioncheck.go), [declaration reference](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/readthedocs/declarations.md), [temporal reference](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/readthedocs/temporal.md).

## 4. Priority one: declarations must survive the Go boundary

### What the source actually does

[`validateAgainstDeclLocked`](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_undeclared.go#L99) obtains a declaration, flattens reflected bounds into strings, and checks positions. `checkBoundAgainstValue` handles `/number` and `/string`; other bounds are not rejected there. Flattening is not the semantics of a disjunction of whole tuples, nor of a nested list/struct bound. This is a limitation of this boundary, not proof of a currently observed user-visible corrupt fact.

Separately, [`declBoundsLocked`](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_fact_decl.go#L16) selects `decl.Bounds[0].Bounds` for numeric coercion. Its carefully bounded integral-float conversion should not be thrown away: it rejects fractional, nonfinite and out-of-int64-range inputs for simple `/number` slots. The [other engine's conversion path](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/mangle/engine.go#L713) also reads the first tuple when selecting an expected simple type. These are different adapters and need separate migration evidence.

For static rule analysis, [RealKernel line 170](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L170) and [Engine line 357](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/mangle/engine.go#L357) use `AnalyzeOneUnit`. Upstream that delegates to `Analyze`, which explicitly selects `NoBoundsChecking`. Declaration parsing/name validation therefore must not be advertised as full type checking.

### Proposed admission algorithm

Design notation below is a contract, not compiled or tested code:

```text
AdmitBatch(rawFacts, schemaEpoch, producerIdentity):
  pin analyzed declaration map and its program digest
  verify producer owns the allowed predicate/arity set
  check exact predicate arity and that each value can become a ground constant
  convert values through an explicit typed adapter
    preserve Name versus String; reject ambiguous implicit reinterpretation
    preserve the existing safe numeric conversion policy where applicable
  check each converted atom against the declaration's alternative bound tuples
  stage accepted atoms without exposing partial publication
  apply the selected batch inclusion checks against their declared domain snapshot
  publish one epoch, or return bounded structured rejection reasons
```

Use `NewTypeCheckerFromDesugared` only with genuinely desugared declarations, such as the analyzed `ProgramInfo.Decls`; do not pass raw source declarations under that name. Validate arity before calling `CheckTypeBounds`: the pinned `CheckOneBoundDecl` indexes `fact.Args[j]` while walking the bound tuple. Groundness, predicate ownership and byte/depth limits are host responsibilities, not gifts from this API.

The semantic condition is:

\[
\operatorname{valid}(p(a_1,\ldots,a_k)) =
\bigvee_{b\in\operatorname{Bounds}(p)}\bigwedge_{j=1}^k a_j\in\llbracket b_j\rrbracket.
\]

Do not flatten the alternatives into independent per-column unions: that admits illegal cross-combinations. Example: an evidence record allowing `(success, witness)` or `(failure, diagnostic)` must not thereby admit `(success, diagnostic)` merely because each column is individually allowed. For first adoption, keep status and typed payload variants modest and project frequently joined identifiers into separate indexed relations.

This does not make `valid_fact` equivalent to `trusted_fact`. A syntactically well-typed claim from an untrusted producer is still untrusted. Runtime admission should never let learned rules or generated facts grant themselves permission.

### Static-check migration

Start with a named, small policy module and its dependency declarations; record mismatches in a separate shadow analysis. Do not globally switch the boot program to strict bounds and hope historical `/any` declarations become precise. Once a module's producers, conversions and rules agree, require `ErrorForBoundsMismatch` on its candidate composition before admission. Static checking catches rule-level type mistakes; runtime checking catches actual host values. Neither substitutes for the other or proves task correctness.

## 5. Priority two: inclusion constraints for real integration edges

The [upstream checker](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/inclusioncheck.go#L50) substitutes fact arguments into constraints, checks the relevant alternative's bounds, then uses store membership for positive/negative atoms and equality/inequality. It rejects remaining unbound variables. It does not search for missing entities, run a repair workflow, or derive the referenced domain.

Useful first domain: an immutable verification-evidence batch where each evidence identifier must reference an admitted task, revision and check definition. This can catch a producer that emits a correctly shaped result keyed to the wrong run before a completion rule reads it. That is narrower and more useful than vaguely adding “more validation.”

Contract:

* Pin the task/revision/check catalog to the same epoch as the batch.
* Stage all catalog and evidence rows before checking references; arrival order is not a semantic violation.
* If reference membership is derived, obtain the intended complete closure first. Do not validate against a partially evaluated relation.
* Unknown membership is **pending/unresolved**, not false, when an external provider or incomplete snapshot owns the domain.
* Use negative inclusion only for explicitly closed, complete domains. A missing remote check is not proof that no check exists.
* Revalidate dependents or prohibit mutation for the epoch: checking at insertion does not prevent a later referenced-row removal from breaking the relation.
* On failure, quarantine this batch, preserve the last valid epoch, and return producer plus predicate/argument diagnostics. Do not silently delete facts until constraints pass.

This is not a replacement for the existing end-to-end test-obligation design in M2. It is a prerequisite that ensures the identity joins feeding those obligations refer to the same world.

## 6. Priority three: proof facts without overstating proof

CodeNERD already has [EnableProvenance and Explain](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_provenance.go), and [evaluation installs the recorder](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L322-L338). Enabling it alone does not dirty a clean kernel; the source instructs callers to trigger a new evaluation. Resetting a recorder every pass limits retention across passes, not event volume within one large pass.

M2 Pattern F already proposes proof-as-data. The refinement here is the v0.7 integration contract, not a new invention of that pattern. [`EmitFacts`](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/provenance/facts.go) produces `proves`, `uses_rule`, `premise`, `edb_leaf`, `absence_leaf`, bindings and transform/group metadata. It encodes constant arguments directly into list constants. Thus the old M0 warning about converting these constants through display strings should not be copied as a current `EmitFacts` defect.

However, this emitter does **not** export `ProofNode.Partial`, although its human-readable printer does. A policy consumer of emitted facts can therefore lose an important qualification unless the host explicitly carries it. Nor does the emitted schema itself carry CodeNERD's source revision, schema epoch or completeness policy.

Proposed wrapper around the existing output:

```text
ExplanationEnvelope:
  source_revision, program_digest, fact_epoch, requested_ground_goal
  recorder_enabled_for_epoch, extraction_limits, truncation_status
  proof_node_status[node_id] = preserved partial flag + known reason if available
  origin_map[rule_id] = analyzed-rule identity and available source location
  typed proof facts in a separate read-only store
```

A second program may select direct supports, annotate missing origins and assemble concise evidence packets. It must not assert permissions into the execution kernel. Absence leaves establish absence only in the evaluated snapshot's closed-world semantics. An EDB leaf proves store membership, not the truth of an external observation. A partial marker is not automatically “the conclusion is wrong”: it may describe an unrepresented builtin step. Preserve the distinction and disclose the reason when known.

Apply node/edge/serialized-byte caps as well as proof/depth limits; cap extraction without pretending that it capped recording. Retain the exact goal and snapshot so “why?” answers cannot silently mix old proof events with new facts. Restrict prompt injection to the relevant explanation packet rather than dumping the entire proof graph into every turn.

## 7. Version-aware capability manifest for generated rules

A small declarative manifest should say which features a **specific host path** supports, rather than “Mangle supports X” globally. Proposed fields:

```json
{
  "dependency": "codeberg.org/TauCeti/mangle-go@v0.7.0",
  "host_path": "RealKernel",
  "program_digest": "<computed by a future implementation>",
  "features": {
    "runtime_rich_bounds": {"state": "proposed", "required_contract": "typed-ground-atom-admission"},
    "inclusion_constraints": {"state": "proposed", "required_contract": "closed-epoch-domain"},
    "derivation_recording": {"state": "implemented-opt-in", "default_enabled": false},
    "proof_fact_consumer": {"state": "unverified", "required_contract": "epoch-and-partial-status-envelope"},
    "temporal_evaluation": {"state": "not-wired-in-inspected-eval-options"}
  }
}
```

This JSON is illustrative documentation, not an existing configuration schema. JIT rule guidance should only teach a feature as production-ready when the selected adapter, typed boundary, lifecycle and validation evidence agree. Upstream release fixes are useful evidence, but not conformance results for CodeNERD.

The [v0.7 changelog](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/CHANGELOG.md) reports repaired reducer type checking, type unification, factstore hash-collision handling and stable ordering of alternative derivations. Do not propose upgrading from an old version to obtain fixes already present in the pin. Also do not infer that these repairs solve all historical deferred/merge or serialization issues.

## 8. Full cost model and fair evaluation

Let N be admitted atoms, A the maximum number of bound alternatives, V the total inspected value structure per atom, C inclusion conditions per alternative, and L the membership-lookup cost for the chosen store. A conservative planning bound for admission is:

\[
T_{admit}=T_{convert}+T_{desugar/cache}+O(N A V)+O(N A C L)+T_{stage/publish}.
\]

This is not a benchmark. Nested value traversal, hash/equality work and store behavior determine real costs. Cache declaration preparation by program digest; invalidate on schema change. Include staged-store memory, rejection diagnostics, retries and bootstrap revalidation. Do not hide full snapshots or closure recomputation in an “O(1) validation” claim.

For proof use, account separately for recording all selected rule firings, building bounded goal proofs, emitting proof facts, second-program evaluation and final serialization. Small output does not imply cheap recording. Peak memory includes both the execution store and the explanation store until ownership transfers or one is released.

End-to-end token accounting must include setup/context injection, rejected generated rules, retries, repair conversations and human review. Compare tokens **per correctly completed task at a stated false-completion rate**, not tokens per successful-looking response. Better checks can initially increase tokens by revealing errors that a permissive baseline missed.

### Proposed evaluation matrix; none executed here

| Experiment | Fixed inputs and comparison | Required observations |
|---|---|---|
| Admission semantics | Same facts through current boundary vs typed runtime checker; one alternate tuple valid, crossed tuple invalid, nested wrong type, name/string collision, arity mismatch, fractional/out-of-range numeric values | Exact accepted sets, useful rejection attribution, no panic, no partial publication |
| Static analysis | Same candidate programs with current analysis vs shadow/strict bounds; include normal reducers, invalid reducer modes and `/any` cases | True rejection/false rejection rates, analysis time, clear unknown-vs-verified distinction |
| Integration references | Same complete epoch; random arrival order, missing IDs, mixed revisions, later deletion, incomplete external domain | Order-independent valid outcomes; unresolved separated from false; stale joins blocked |
| Proof preservation | Same ground goals and epoch; rule, aggregation, builtin, negative and truncated proofs | Typed identity retained; partial flag retained; no absence interpreted outside closed snapshot |
| Whole harness | Same task set, model, context/turn budgets, permitted tools and correctness evaluator | Correct completion, false completion, tokens, latency, peak RAM, CPU time, retry and human-review burden |

Sweep atom counts and nested widths separately; do not change every dimension at once. Compare both existing store regimes and representative composed policies. Keep training history and cached artifacts fixed for model comparisons. Report failures and fallback costs, not just successful traces.

Acceptance is staged: first semantic equivalence for previously valid inputs, then intentional rejection of known-invalid inputs, then end-to-end benefit with no hidden false-completion increase. No superiority or speedup is established by this document.

## 9. Recommended sequence and stopping conditions

1. Inventory producers and exact typed conversions for one evidence predicate family. Stop if ownership or epoch meaning is ambiguous.
2. Design the runtime checker adapter and static shadow report together. Stop before enforcement if legitimate alternatives or bootstrap facts cannot be represented unambiguously.
3. Add inclusion contracts only to closed reference domains. Leave external/incomplete domains explicitly unresolved.
4. Reuse M2's explanation direction with the missing proof-status envelope and bounded costs. Do not enable whole-session recording by default on the strength of per-pass reset alone.
5. Revisit advanced evaluation features only after a feature-specific host contract and permitted future conformance experiments exist.

The guiding principle: **use Mangle's stronger semantics where CodeNERD can preserve them end to end; do not enlarge the language surface faster than the harness can enforce its meaning.**
