# Inclusion checking must project a complete executable program

## Decision, pin and evidence boundary

Preserve the new layered-strata and cone optimizations, but tighten the smaller program built for **batch inclusion checking**. It currently omits two different parts of the source program's contract: merge-predicate dependencies and the parallel metadata for retained initial facts.

Source freeze: CodeNERD main **`b2be2799914ec942c90e08a7b487adc405c6428c`**, rechecked 2026-10-29 at 08:46 UTC. The Mangle optimization lane has merged into this main; these findings are **not branch-only**. The relevant `strata_layers.go`, `kernel_eval_cone.go`, `kernel_cone_plan.go` and `kernel_inclusion.go` blobs are identical on inspected active lane **`cfabac55661500defd025f3ce5c686ccac91144d`**. That lane's additional generic-engine lattice and Orient changes do not alter the inclusion projection inspected here.

This is source inspection, including an independent source challenge. **No tests, implementation, inference, training, profiling, local Git, computer tasks or runtime reproductions were performed.** The snippets are proposed fixtures, not executed results.

### Upstream provenance limitation

CodeNERD pins `codeberg.org/TauCeti/mangle-go v0.7.0`. The upstream implementation inspected here is the official GitHub mirror's **v0.7.0** tag, which resolves to **`7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`**; its module path is also `codeberg.org/TauCeti/mangle-go`. The maintainer says Codeberg is the source of truth and commits are mirrored to this GitHub repository. Direct Codeberg raw/API and Go-proxy metadata retrieval were unavailable through the permitted reading tools, so this review did **not** independently verify the bytes of the exact downloaded Codeberg module. Host-side omissions below are definite; upstream-dependent behavioral predictions are explicitly conditional on the deployed v0.7.0 implementation matching the inspected mirror. [CodeNERD dependency](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/go.mod#L5-L8), [mirror module](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/go.mod#L1-L3), [maintainer's migration/mirror statement](https://github.com/google/mangle/discussions/89).

## 1. The implemented contract to preserve

`InclusionReport` admits a complete batch, identifies the relations referenced by its declarations, classifies their backward closure, stages the batch in a scratch `SimpleInMemoryStore`, derives the closed reference domains, and invokes the upstream inclusion checker. Open domains are reported **unresolved**, not treated as false. A violation is returned through `CheckBatchInclusion` so the caller can retain the previous epoch. This is a useful host boundary, not a missing feature to reinvent. [Contract and public API](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_inclusion.go#L19-L47), [staging and checking](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_inclusion.go#L148-L235).

The current production application is campaign task/phase references. Their domain rules are simple projections from the supplied batch. This matters: neither proposed counterexample below establishes that ordinary stock campaign snapshots presently encounter these defects. [Campaign constraints](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/defaults/schemas_campaign.mg#L65-L108), [identity projections](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/defaults/policy/campaign_tasks.mg#L12-L17).

## 2. Merge dependencies disappear from a supposedly closed reference domain

### Producer-to-consumer chain

- `newInclusionClosure` indexes rules and external declarations. `walk` follows only rule-body premises; it never follows a declaration's `merge(...)` predicate. On success, `openReason` marks reached rule heads as the derived set. [Closure construction and walk](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_inclusion.go#L273-L339).
- `derive` retains rules only when their head is in that set. It retains the full declaration map, so a selected relation can still declare a merge predicate whose implementation rules were omitted. [Projected program](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_inclusion.go#L356-L398).
- In the inspected upstream v0.7.0, the engine builds its rule lookup from the **supplied** rules. A top-down query with no matching rules produces no answers. The lattice merge treats no answer as incomparability and retains both rows. The inclusion checker subsequently tests store membership, not whether this reference relation was faithfully derived. [Rule lookup](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/seminaivebottomup.go#L188-L196), [top-down query](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/topdown.go#L36-L77), [merge and incomparability](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/seminaivebottomup.go#L471-L506), [membership checker](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/inclusioncheck.go#L73-L106).

### Proposed isolated witness

Use a trusted host-owned policy with a one-key, one-value numeric maximum lattice:

```mangle
Decl candidate(K, V) bound [/name, /number].
Decl best(K, V)
    descr [fundep([K], [V]), merge([V], "higher")]
    bound [/name, /number].
Decl higher(A, B, Best)
    descr [mode('+', '+', '-'), deferred()]
    bound [/number, /number, /number].
Decl claimed(K, V) bound [/name, /number] inclusion [best(K, V)].

best(K, V) :- candidate(K, V).
higher(A, B, A) :- A >= B.
higher(A, B, B) :- B > A.
```

Supply the batch through Go with correctly typed names and integral numbers:

```text
candidate(/k, 1)
candidate(/k, 2)
claimed(/k, 1)
```

There are **no program initial facts**, external callbacks or ordinary IDB reads hidden inside the comparator. The numeric maximum is a total preorder, the key is leading, and the inclusion scratch store supports removal. Those controls avoid conflating this case with temporal metadata, non-leading-key merge bugs, late materialization order, an intentionally incomparable merge, or the generic engine's separate locking issue.

Source-derived expectation, conditional on the upstream implementation above: the complete program keeps only `best(/k, 2)`, so the claim of value 1 violates inclusion. The projected program contains `best :- candidate` but no `higher` rules; it retains both values and can incorrectly accept the losing claim. Independent source review confirmed this projection chain. This is a latent **trusted custom-policy / host-batch composition** case, not an observed production false acceptance.

### Existing defenses and predecessor

The main cone planner already knows that merge/deferred rules must accompany the consumer, and explicitly adds the top-down rule closure. Its test comment records the no-helper/incomparability problem. The inclusion builder is a different projection and does not reuse that dependency information. Thus this is a cross-boundary conformance refinement, not a claim that merge support or top-down cone wiring is absent throughout CodeNERD. [Cone top-down rule closure](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_cone_plan.go#L103-L111), [existing regression explanation](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_eval_cone_test.go#L221-L226).

## 3. Retained initial facts lose their aligned time metadata

`derive` promises that immutable program facts of the referenced relations participate. It filters `info.InitialFacts` into `initial`, then constructs a new `ProgramInfo` with that slice but **without `InitialFactTimes`**. This omission is present independently of the merge case. [Filtering and construction](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_inclusion.go#L356-L398).

The inspected upstream v0.7.0 reads `InitialFactTimes[i]` for each admitted initial fact **before** checking whether a temporal store is installed. Consequently, if at least one initial fact survives this projection, a nil time slice predicts an index panic rather than a normal inclusion verdict. The ordinary non-temporal case is sufficient under that implementation. This is not a reproduced panic, and the exact-module provenance limitation above applies. [Upstream initial-fact loop](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/seminaivebottomup.go#L266-L284).

A separate proposed fixture, with **no merge**, is:

```mangle
Decl source(X) bound [/name].
Decl reference(X) bound [/name].
Decl claim(X) bound [/name] inclusion [reference(X)].
reference(X) :- source(X).
reference(/builtin).
```

Batch: `source(/ordinary)` and `claim(/builtin)`, using typed name values. The rule makes `reference` a derived closed relation; its program fact survives the filter. A faithful projection should include that fact and accept the claim. The current builder's omitted parallel slice is the concern. Do not replace this with a purely extensional reference missing from the batch: the existing closure intentionally calls such a domain unresolved, which is a different path.

The cone builder provides a useful counterexample: it carries both `InitialFacts` and `InitialFactTimes` into its subprogram. The omission is specific to this builder, not a repository-wide absence of time metadata. [Cone program construction](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/kernel_eval_cone.go#L420-L429).

## 4. Relationship to current guidance and research

The feature-adoption audit already requires complete derived reference domains and documents the implemented batch checker. The present refinement supplies concrete ways its projected program is incomplete; it does not re-propose inclusion constraints. The newly merged layered-strata work changes how that projection selects members of a stratum but does not establish that the selected rule set and initialization metadata are complete. [Original completeness requirement](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/Docs/audits/mangle-feature-adoption-contracts.md#L94-L106), [as-built checker](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/Docs/audits/mangle-feature-adoption-contracts.md#L196-L203), [layered-strata as-built](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/Docs/audits/mangle-kernel-performance.md#L383-L389).

Current capability guidance calls inclusion host-batch-only and explicitly tells generated-rule producers not to declare inclusion constraints; it also does not teach generated deferred/merge declarations. Preserve that scope. None of these observations authorizes model-written rules to widen permissions or makes inclusion membership evidence of trust. [Capability contract](https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/mangle_capabilities.go#L121-L143).

Current main audits and the available `research-main-20261029-0736` delta were checked. The latter contains learned-example, taxonomy-initialization and verification follow-ups, not this inclusion-program projection issue. Earlier external-cache, pushdown, learned-rule comment, metadata identity and cone-readability findings concern different boundaries.

## 5. Proposed contract, implementation choices and full costs

**A projected inclusion program must preserve every executable dependency and every aligned field required to derive its declared closed reference domains.**

1. Include merge/deferred executable dependencies in both **closure classification** and rule selection. Merely appending a comparator's rules is insufficient if it reads an external or otherwise open domain: that must remain unresolved rather than silently borrowing live state. Preserve recursive closure handling, negative premises, declaration identity and per-stratum membership.
2. Filter initial facts and their corresponding time entries **in lockstep**. For plain facts, explicit nil entries still preserve alignment. Do not invent temporal evaluation: CodeNERD's separate temporal capability restrictions remain. If source metadata is malformed, report a bounded structural error rather than depending on an unchecked index.
3. Until a dependency shape is qualified, report its domain as unresolved or return an explicit unsupported-projection error according to the host's contract. Do not label a partial result closed. Do not evaluate the whole live kernel merely to make the error disappear: that would violate batch-domain isolation.
4. Prefer one well-tested dependency/projection description shared where appropriate with the existing cone code over two independently drifting interpretations. Sharing must preserve their different aims: cones reuse proven unchanged facts, whereas inclusion derives a closed domain from the batch alone.

The metadata fix is small O(F) copying for F retained initial facts. Complete top-down closure adds bounded graph traversal, helper rules and possibly newly recognized closed-domain work; worst-case cost may approach the relevant program closure, not just the originally visible body graph. Existing created-fact limits remain essential. Memoization can be keyed by immutable program identity, but the set of batch-present predicates affects closure and must be included; do not cache membership or open/closed verdicts under program identity alone.

Costs include API/diagnostic decisions for unsupported domains, regression fixtures, dependency-graph ownership, parser/analysis compatibility review, and possible increases in validation time, allocations and derived rows. Broader whole-program evaluation or copying every fact would add avoidable cost and may change semantics. No model inference, training, embeddings or additional provider calls are required for these deterministic refinements. No latency or task-quality improvement is claimed; the primary goal is trustworthy admission and explicit failure.

## 6. Fair evaluation plan — unexecuted

- **Establish the deployed module first.** Verify the exact Codeberg v0.7.0 source or downloaded module identity in an authorized environment before treating mirror-derived behavior as reproduced fact. Preserve the distinction in any report.
- **Run the two isolated fixtures separately.** Merge fixture: no initial facts, numeric max, both candidate arrival orders, losing/winning claim controls. Initial-fact fixture: no merge, zero/one/several retained facts, facts both retained and filtered out, aligned nil metadata. Check no panic and the intended verdict.
- **Compare complete relations and verdicts.** Use the complete program over the same batch-only inputs as the reference, not a live kernel containing unrelated facts. Require the projected `best` relation and inclusion report to match, not merely a nil error.
- **Exercise the public boundary.** Test `InclusionReport` and `CheckBatchInclusion`, then the campaign snapshot adapter with unchanged simple-projection controls. On a real violation, require the previous epoch to remain intact.
- **Dependency controls.** Nested deferred helpers, merge helper reading a closed batch relation, helper reaching an external/open relation, negation and recursion, unknown domain, no constrained rows, typed-name versus bare-string tolerance, and bounded derivation failure. Open domains must remain explicitly unresolved.
- **Layer controls.** Compare component strata and layered strata over the same projected complete program. A speed optimization must not compensate for a missing dependency by accidentally exposing extra rules.
- **Cost accounting.** Measure closure construction, retained rules/initial facts, scratch-store bytes, engine time, allocations and total batch-admission latency across batch sizes and helper depths. Include rejection/unresolved workloads and warm/cold program reuse; charge any shared-graph construction and caching.

**Checkable exit:** a closed inclusion reference is derived from an executable, structurally valid batch-scoped projection; a lattice loser cannot become a valid reference merely because its comparator was omitted; retained program facts cannot violate parallel metadata invariants; and stock campaign behavior remains unchanged.

