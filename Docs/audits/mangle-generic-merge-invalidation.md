# Generic-engine rederivation must follow merge-comparator dependencies

## Decision, source pin and scope

Complete the generic `mangle.Engine` invalidation graph with the semantic dependency **merge comparator → lattice head**. The newly enabled lattice implementation correctly marks lattice heads non-monotone, but a change used only inside the comparator cannot currently reach that mark or the lattice's consumers. A reused engine can therefore retain a conclusion about the old winner after its ordering facts change.

Source freeze: CodeNERD main **`ca94da614edff9157acf31bb54f65e4f9695ae92`**, checked 2026-10-29 at 09:16 UTC. The relevant `engine.go`, `engine_rederive.go` and `engine_lattice_test.go` blobs are byte-identical to the earlier inspected active lane `b8b84c6a605c6546a2aa348578b6be4bf6873a14`; that work has now landed on main. This is no longer a branch-only observation.

**Qualification:** the host graph omission is directly established by source. The stale-result witness below is a source-derived prediction, independently challenged, not an executed reproduction. It needs a trusted/custom program whose merge comparator reads replaceable ordering facts and a reused generic engine. The actual stock Orient comparator does not have that shape. No stock Orient failure or production false decision is established.

This review used public source inspection only: no tests, inference, training, implementation, profiling, local Git or computer tasks. The proposed fixture and evaluation plan have not been run.

### Upstream provenance boundary

CodeNERD requires `codeberg.org/TauCeti/mangle-go v0.7.0`. The inspected upstream source is the accessible `google/mangle` v0.7.0 mirror commit **`7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`**, whose module path is also `codeberg.org/TauCeti/mangle-go`. Exact downloaded Codeberg module bytes were not independently retrieved and compared. Consequently, upstream-dependent predictions below are conditional on the deployed v0.7.0 implementation matching that inspected mirror; they are not runtime measurements. [CodeNERD dependency](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/go.mod#L5-L8), [mirror module](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/go.mod#L1-L3).

## 1. Producer-to-consumer evidence

1. **The supported adapter now evaluates removable lattices.** `Engine` owns a `FactStoreWithRemove` under its own read/write mutex, rather than evaluating inside a read-locked `ConcurrentFactStore` scan. `evalWithGasLimit` passes its current full program and store to the stratified evaluator. This removes the earlier lattice lock-upgrade obstacle; it is a useful improvement to preserve. [Store/locking contract](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L61-L78), [evaluation](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L242-L264).
2. **Its graph recognizes non-monotonicity but misses the semantic edge.** `buildRuleGraph` marks each lattice head non-monotone. It then creates dependency edges only from `rule.Premises` to the rule head. It never reads the declared `MergePredicate()` when creating those edges. A relation read inside a comparator reaches the comparator through ordinary body edges, but not the lattice that calls it through its declaration. [Graph construction](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_rederive.go#L59-L93), [lattice recognition](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L1255-L1263).
3. **Named replacement really takes the incremental path.** `ReplaceControlFacts` removes the named predicate's rows, inserts replacements and always rederives, even with `AutoEval=false`. Removal records `dirtyRemoval` and the named dirty predicate, not `dirtyAll`. Therefore a rank-only replacement does not accidentally rescue this case by clearing every IDB. [Replacement contract/path](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L587-L641), [dirty recording](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_rederive.go#L118-L130).
4. **The clearing boundary is exactly the incomplete graph.** `rederiveLocked` computes the downstream set, clears IDB only within it when invalidating, then evaluates the entire program over the remaining store. A subsequent `Evaluate()` also uses `rederiveLocked`; it does not itself force a fresh store. [Downstream traversal and rederivation](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_rederive.go#L96-L166), [Evaluate](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L577-L584).
5. **A changed winner does not retract its old consumers upstream.** In the inspected mirror, the merge calls its comparator top-down, removes a losing lattice row when the store supports removal, and adds the replacement. That replacement does not recursively remove facts previously derived from the loser. Ordinary non-deferred comparator premises are read from the current store. [Merge mechanics](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/seminaivebottomup.go#L438-L510), [premise lookup](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/engine/topdown.go#L142-L168). Host rederivation already exists precisely to handle surviving downstream conclusions; the missing edge leaves this declaration-mediated case outside it.

## 2. Proposed isolated witness — not executed

Use this trusted host-owned schema, with no program initial facts or external callbacks:

```mangle
Decl candidate(K, O) bound [/name, /name].
Decl rank(O, R) bound [/name, /number].
Decl winner(K, O)
    descr [fundep([K], [O]), merge([O], "better")]
    bound [/name, /name].
Decl better(A, B, Best)
    descr [mode('+', '+', '-'), deferred()]
    bound [/name, /name, /name].
Decl chosen(K, O) bound [/name, /name].

winner(K, O) :- candidate(K, O).
better(A, B, A) :- rank(A, RA), rank(B, RB), RA <= RB.
better(A, B, B) :- rank(A, RA), rank(B, RB), RB < RA.
chosen(K, O) :- winner(K, O).
```

1. Create a generic engine with `AutoEval=false`, load this schema, and add correctly typed base facts `candidate(/k, /a)`, `candidate(/k, /b)`, `rank(/a, 1)` and `rank(/b, 2)` through the fact API.
2. Call `Evaluate()` once. This settles the initial `dirtyAll` created by program adoption. Expected settled rows include `winner(/k, /a)` and `chosen(/k, /a)`. [Program adoption resets all derived facts](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_rederive.go#L211-L226).
3. Call `ReplaceControlFacts` with only replacement facts `rank(/a, 3)` and `rank(/b, 2)`, naming only the predicate `rank`. Do not replace candidates or reload the program.
4. Read materialized `winner` and `chosen` with `GetFacts`, which scans the store rather than attempting a separate top-down derivation. [Materialized read](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L1064-L1089).

The graph has `rank → better`, `candidate → winner` and `winner → chosen`, but lacks `better → winner`. The replacement's downstream set is therefore `{rank, better}`. Neither `winner` nor `chosen` is cleared. On the inspected upstream semantics, re-evaluation can replace the old winner `/a` with `/b`, while retaining `chosen(/k, /a)` and adding `chosen(/k, /b)`. An independent fresh engine given the same schema and the final base facts should contain only `chosen(/k, /b)`.

This witness deliberately makes rank **EDB**, already replaced before evaluation starts. It does not depend on an arbitrary late IDB stratum, hidden ordinary-IDB materialization order, missing comparator rules, inclusion projection, time metadata, external caching, non-leading functional-dependency columns or intentionally incomparable values. There is exactly one distinct rank per option in each snapshot, so the comparator is a total minimum within each epoch. The policy's ordering changes between epochs, which is the point of exercising the adapter's exact replacement contract. If mutable comparator inputs are intentionally unsupported, that restriction needs an explicit validated contract rather than an apparently successful replacement with stale consumers.

## 3. Counterevidence and limits

- **Direct lattice dependencies are already handled.** `TestEngine_LatticeEvaluates` adds ordinary `edge`/`tie` facts that reach the lattice through rule bodies, then checks that old `rooted` conclusions disappear. The implementation's new non-monotone marker is appropriate for that case. The missing coverage is comparator-only EDB changes. These tests were read, not run. [Existing fixture and initial check](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_lattice_test.go#L12-L66), [consumer-retraction check](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_lattice_test.go#L68-L94).
- **Stock Orient does not instantiate this comparator-only input.** Both component-label declarations use `orient_lower_ord`; its two rules compare only the supplied numeric ordinals. `doc_tie` and graph inputs appear directly in ordinary lattice rules, so their changes have visible body dependencies. This witness is not evidence that shipped Orient component labels or read sets are wrong. [Declarations](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/orient/schema.mg#L208-L213), [second declaration/comparator mode](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/orient/schema.mg#L241-L248), [pure comparator and read-component rules](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/orient/lineage.mg#L239-L243), [cohort rules](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/orient/timeline.mg#L296-L301).
- **The action kernel already has the correct edge.** `RealKernel`'s cone index explicitly records comparator-to-lattice dependency because changing a comparator read can change the retained row. This is a generic-adapter consistency gap, not a claim that all CodeNERD dependency analysis ignores merges. [Existing host-local precedent](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/core/kernel_eval_cone.go#L151-L168).
- **The new lattice query protection is useful but separate.** `Query` does not rederive a lattice head top-down, avoiding reintroducing all candidates as lattice answers. It does not remove stale facts in another predicate already materialized from a former winner. The witness uses `GetFacts` specifically to isolate invalidation. [Query guard](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine.go#L1001-L1018).
- **Fresh evaluation is a control, not the failing path.** A new engine or program adoption forces full invalidation; replacement of a direct lattice input can also put the lattice in the cone. Neither demonstrates soundness of comparator-only replacement after an initial fixpoint. Base facts asserted deliberately into IDB must remain protected during any repair. [Base-IDB preservation](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/mangle/engine_rederive.go#L169-L202).

### Relationship to prior research

The `research-main-20261029-0850` inclusion audit concerns a **projected smaller program** that omits merge-helper rules and retained-fact metadata. Here the entire program, including the helper, is evaluated; the problem is **retained old IDB outside an incomplete invalidation set**. The shared lesson is declaration-aware dependency extraction, but the producer/consumer boundary, witness and remedy location differ. [Prior inclusion audit](https://github.com/theRebelliousNerd/codenerd/blob/eb353f991ae8bade8d5343a29ce0f4acad3f63ea/Docs/audits/mangle-inclusion-projection-closure.md).

The current performance audit already records plain-store generic evaluation and Orient min-label lattices as implemented work. Its measurements are the repository author's results, not measurements performed here; this refinement preserves those improvements rather than proposing to undo them. Current Mangle guidance likewise says generic lattices are enabled and retraction must not reuse invalid derived state. [As-built record](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/Docs/audits/mangle-kernel-performance.md#L383-L390), [current Mangle guidance](https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/.claude/mangle-project.md#analysis-and-evaluation).

## 4. Proposed contract and remedy

**Contract:** after a successful supported base-fact mutation and rederivation, the materialized conclusions must agree with a fresh engine over the same program and final base facts, including when a declaration invokes a merge comparator that reads those facts. Equality includes downstream conclusions, not just the lattice's current row.

At program adoption, add the semantic edge from each supported merge predicate to its lattice head, preserving the existing non-monotone mark. Ordinary body edges then propagate comparator dependencies transitively, including through deferred helpers. The core cone implementation provides a local precedent. Do not merely mark more predicates non-monotone: without the connecting edge, the affected cone still never reaches the lattice. Likewise, re-running the full evaluator over the retained store is not equivalent to clearing invalid derived rows first.

Consider a shared semantic-dependency extractor only if it preserves the separate consumers' contracts: invalidation, top-down rule inclusion and layer ordering need related information but are not interchangeable algorithms. Unsupported dependency shapes should trigger a documented conservative full-derived reset or a clear rejection. Avoid silently accepting a narrower dependency analysis. Do not add Go-side ranking policy or a custom winner-selection heuristic.

## 5. Costs and trade-offs

- **Graph cost:** one declaration scan and one edge per supported lattice at program adoption; approximately O(declarations) construction work and O(lattices) extra graph storage. The existing traversal still costs the size of the reached dependency graph.
- **Necessary invalidation cost:** comparator changes may now clear a large lattice and all its consumers. Store scans, removals, rederivation, allocations and exclusive lock hold can rise with the affected relation size and fan-out. The current adapter already evaluates the full program after clearing; this proposal changes which old derived rows survive, not a claim that all runtime work becomes delta-sized.
- **Fallback cost:** full-derived clearing is simpler but can be much more expensive than a dependency-complete cone. A fresh-engine oracle is useful for verification, not a recommendation to copy/reload every store on each mutation. Preserve explicit base-IDB facts and evaluation limits.
- **Engineering/compatibility cost:** update the generic graph, targeted regression coverage and the supported comparator contract; check that graph cycles terminate and helper chains are included. A shared extractor adds cross-package maintenance risk and is optional. Declaring comparator inputs immutable instead would narrow currently expressible policy and require validation, documentation and a migration decision for custom consumers.
- **No model cost is intrinsic:** no inference, embeddings, training, provider calls or new persistent cache is required. No token savings, speedup percentage, dollar estimate or production failure rate is claimed. The benefit is correctness of reusable materialized decisions; performance must be measured under the same semantics.

## 6. Fair evaluation plan — proposed, not run

1. Execute the isolated rank-only replacement through public APIs, first with `AutoEval=false` and then with it enabled. Settle the first epoch before replacing rank. Compare canonical full materialized fact sets and errors against an independent fresh engine loaded from final base facts; specifically require removal of the old `chosen` row.
2. Include unchanged-rank replacement, input-order permutations, equal/no-op fact insertion, a direct-body lattice input change, and the existing pure numeric minimum/Orient shape. Add a deferred-helper chain whose terminal read is changed EDB. Missing or non-unique ranks belong in explicit comparator-contract tests, not in this total-order witness.
3. Exercise supported addition, replacement and removal paths separately; distinguish changes that genuinely alter comparator semantics from irrelevant edits. Keep a program-reload/full-reset control and compare base-IDB preservation, evaluation limits and returned errors.
4. A graph-edge unit assertion is necessary but insufficient. Require end-to-end evidence that comparator input → lattice winner → ordinary consumer reflects the new epoch. Use materialized reads so query-time top-down evaluation cannot hide the retained-state defect.
5. Measure warm reused engines with small and large IDB, shallow/deep helper chains, narrow/wide consumer fan-out and unchanged inputs. Report graph build time/memory, cleared rows, total facts, rederive latency, allocations and lock hold. Compare current generic behavior, semantic-edge repair and a full-derived-reset reference only where all produce the same correct final relation; an incorrect retained-store result is not a valid faster baseline.

**Exit condition:** comparator-only changes yield the same materialized conclusions as the fresh-reference engine, direct-input and stock pure-min cases retain their behavior, and the widened invalidation cost is documented. A runtime confirmation against the exact deployed Codeberg module remains necessary before presenting the predicted witness as reproduced.

