---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: proposed-refinement
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
scope: eligible failure group to bounded strategy refinement selection
---

# Strategy refinement must apply group scope before its candidate limit

## Decision and evidence boundary

Select refinement candidates from the current failure group's problem type and shard **before** applying the candidate limit. Keep the existing cycle-wide generation/refinement budget, cancellation, evidence admission and group ordering. Do not repair this by giving every group a fresh spending allowance or by retrieving the entire strategy database.

The current path requests the three lowest-success eligible strategies across all groups, then discards those that do not match the current group. Three worse strategies belonging to inactive groups can therefore exclude a relevant fourth strategy even when the current group has reached refinement with call budget remaining. The consequence established here is **missed eligible refinement**, not incorrect execution authorization, unsafe code adoption, or proof that refining the strategy would improve task quality.

Source baseline: public main [`388205138ba6d47c1cff0c5e97c896dfa642a391`][baseline], re-resolved on 2026-10-30 at approximately 08:15 UTC. An independent source challenge corroborated the selection-order defect with the same reachability qualifications. No repository program, test, model inference, experiment or benchmark was run; no implementation was changed. The fixture and acceptance checks below are proposed, not executed results. The store query also exists unchanged at the previous `8d373fb6` baseline: this is not attributed to the newly landed screening-cohort repair.

## Relationship to existing work

The [screening-cohort audit][screening-audit] and its implementation in [`07e08faaf`][screening-fix] already repair mismatched screening/generation evidence, overbroad evidence retirement, repeated screening purchases and missing screening accounting. This report does not reopen those findings or repackage their explicitly disclosed interior-backlog fairness limitation. It concerns the later lookup of **strategy rows**, not selection of failure records for the shared-cause screen.

The [prompt-evolution lifecycle audit][lifecycle-audit] owns candidate quarantine, generation claims, global spend bounds, recording/drain and served-candidate lineage. Those controls remain valuable. The [JIT strategy-context audit][context-audit] and [`261585231`][context-fix] repair the context carried into strategy classification when a prompt is compiled. Neither that classifier call nor the repaired context handoff selects the strategies to refine during an evolution cycle.

A bounded review of the current audit inventory and targeted searches for `GetStrategiesNeedingRefinement`, refinement starvation and strategy filtering found no report owning this exact global-limit-before-group-filter boundary. That is not an exhaustive claim about every historical QA note. The proposed scope is deliberately smaller than redesigning strategy learning or its deployment lifecycle.

## What the current path actually does

### The loop is wired but automatic spending is opt in

The shared factory constructs the evolver when a compiler and judge client are available, registers its strategy provider with JIT, and installs a turn recorder. The evolver defaults to enabled strategy support and a refinement-use threshold of ten; strategy initialization creates a store and seeds defaults. [Factory wiring][factory], [evolver defaults][evolver], [strategy initialization][strategy-init]

The maintenance adapter separately checks the automatic-evolution feature switch and the evolution interval before starting a cycle. It attaches aggregate usage context and applies a two-minute cycle deadline. Thus this report does not claim every normal boot automatically purchases refinement. The witness requires an enabled or explicitly invoked evolution cycle. [Maintenance admission][maintenance], [deadline][deadline]

### A current group must reach refinement first

`RunEvolutionCycle` snapshots configuration, creates one shared cycle budget, obtains unconsumed failure groups and visits them in oldest-evidence order, with a deterministic key tie-break. It stops visiting groups when canceled or when the shared generation-call or atom allowance is exhausted. [Cycle budget and dispatch][cycle], [ordering][order]

For a group, `generateForGroup` checks the serving pin, constructs and saves a bounded evidence cohort, enforces the minimum eligible failure count, obtains screening permission and acquires a generation claim. It charges the atom-generation call against the shared generation budget. A generation error returns before refinement. After candidate handling and publication, it calls `updateStrategies` only when strategies are enabled, the store exists, and generation-call allowance remains. [Group gates and generation][group], [refinement handoff][handoff]

These conditions matter. An insufficient cohort, negative screen, unavailable claim, generation error or exhausted allowance can legitimately prevent refinement. None of those conditions is needed to produce the selection defect below.

### Candidate scope is applied too late

`updateStrategies` requests `GetStrategiesNeedingRefinement(threshold, 3)`. The store query selects rows with sufficient total uses and success rate below `0.6`, orders by success rate ascending and use count descending, and applies `LIMIT`. It has **no problem-type or shard predicate**. Only after those three rows return does the caller compare each row with the current `problemType` and `shardType`. [Store query][query], [caller filter][refine]

The composition is therefore:

```text
current candidates = filter current group (global worst three eligible strategies)
```

That is not equivalent to:

```text
current candidates = worst three eligible strategies within the current group
```

The first expression may be empty even when the second is not. The caller has no continuation query or scoped fallback after rejecting unrelated rows. This is candidate omission before the model call, not a model's refusal to improve an admitted strategy.

## Concrete conditional fixture

Use a functioning strategy store with these four rows. Problem names below refer to the repository's existing problem-type constants; every row uses the same `/coder` shard, so different problem types alone suffice.

| Strategy | Problem type | Successes | Failures | Success rate |
|---|---|---:|---:|---:|
| A | Debugging | 0 | 20 | 0.00 |
| C | Feature creation | 1 | 19 | 0.05 |
| D | Refactoring | 2 | 18 | 0.10 |
| B | Concurrency | 5 | 5 | 0.50 |

All rows meet the default ten-use threshold and have success rate below `0.6`. The differing rates avoid relying on unspecified SQL ordering of ties. These statistics are representable through the normal [strategy outcome writer][outcomes]; the fixture does not require a malformed database.

Assume the following additional state:

1. Concurrency has enough current, unconsumed, versioned agent-failure evidence with a valid serving pin to qualify. A, C and D have historical low success rates but no currently qualifying unconsumed failure groups. Their historical failures may already have been consumed by earlier learning work.
2. The current cycle is authorized to run, is not canceled, and has not spent its generation allowance on earlier groups. The strategy store and all relevant reads/writes are healthy.
3. Concurrency's screen permits generation, its claim is acquired, and atom generation and publication complete normally. Choose a result with one admitted candidate so the fixture also leaves atom capacity; no claim-loss or publication-error behavior is required.
4. With default `MaxCycleGenerationCalls = 3`, the one atom-generation call leaves two logical generation/refinement calls. Strategy refinement is therefore reached with available budget. The default also has separate judge and screening allowances; those are not being silently borrowed here. [Budget defaults][config]

The source-derived sequence is then:

- The global refinement query returns A, C and D. B ranks fourth and never reaches the caller.
- The caller rejects all three because their problem types differ from Concurrency.
- No strategy-refinement model call is made for B, its content/version are unchanged, and `StrategiesRefined` receives no increment from this group. The two remaining generation/refinement calls do not help: no relevant candidate was admitted to the loop that could spend them.
- Atom generation can still succeed and the cycle can finish normally. The defect is not a stalled process or an inability to learn any atom.

This remains a valid omission witness even if there are other groups later in the cycle. Restricting the fixture to one active group merely isolates candidate selection from legitimate competition for the global budget.

### Why the omission can recur

The three unrelated rows are not removed from consideration merely because their groups have no fresh evidence. The SQL query reads their historical use counts and rates, not current failure-group eligibility. In addition, `RefineStrategy` updates content, version, refinement time and source but does not reset those ranking counters; refinement alone is not an automatic rotation mechanism. [Refinement publication][publication]

If new qualifying Concurrency evidence arrives while A, C and D continue to outrank B, a later cycle can repeat the omission. No unconditional or permanent starvation is claimed. Outcomes, deletion, changed configuration or other state changes can reorder the rows; B may eventually rank in the global three. If no new eligible evidence exists, the normal consumed-evidence rules can prevent another generation opportunity entirely. The established property is that **unrelated strategy state can suppress a reached, budget-eligible refinement opportunity**, potentially repeatedly under stable ordering.

## Why the missed work has a real consumer

This is not an unused strategy table. The JIT strategy provider classifies the current prompt context and calls `SelectStrategies` for that problem and shard. Unlike the refinement lookup, this serving query applies problem/shard predicates before its limit. It renders selected strategy bodies as methodology atoms, with normal prompt selection and fitting still applicable. The provider reports source generation for changes to the strategy table. [Serving provider][provider], [scoped serving query][serving]

A successful refinement updates the body/version and invalidates the store's cache; a missed refinement leaves the existing body available. That establishes a learning-to-context connection, not a prediction of how the model will behave. A low empirical success rate is not proof that the strategy caused failures or that the next generated rewrite would help. Nor does this finding establish a bypass of candidate quarantine, execution permission or verification gates.

## Preserve the real global bounds and fairness behavior

There are two different limits here:

- **Three candidate rows** is the argument to the store lookup for one refinement visit. It limits retrieval, but currently limits the wrong population.
- **The shared generation/refinement allowance** is the cycle-wide counter initialized once from configuration. Both atom generation and each attempted refinement decrement it before their model call, including a refinement that fails. Its default is three. It is not three fresh calls per group, and it does not purport to count every model operation in the system. Task judging and shared-cause screening have separate controls. [Cycle budget][cycle], [generation charge][group], [refinement charge][refine], [configuration][config]

Moving group predicates ahead of the candidate limit must not reset or multiply that shared allowance. Keep cancellation checks immediately before refinement calls, existing group order, pin/cohort/screen/claim admission, candidate atom caps and the maintenance deadline. Do not weaken the existing evidence lifecycle to make a previously omitted strategy appear successful.

Oldest-first group ordering is a useful fairness mechanism, not a guarantee that every group receives service in one cycle. An earlier group's admitted refinements consume the same remaining allowance that later atom-generation work could use. Correcting candidate selection can therefore change which later work is deferred even while preserving the global cap. Report that tradeoff explicitly. If a different inter-group allocation is desired, specify and derive that scheduling policy separately rather than disguising it as a parser/query fix.

The intended noninterference property is narrower: **adding an otherwise unrelated strategy must not evict an eligible current-group strategy from that group's candidate window solely because the unrelated row ranks worse globally**. Actual refinement may still be deferred for budget, cancellation or other explicit admission reasons.

## Bounded repair direction

1. **Scope candidate retrieval.** Supply the current problem type and shard to the refinement lookup and apply both predicates before ordering/limiting. Preserve the existing eligibility thresholds and bounded result size. Do not substitute a larger global constant or fetch-and-filter an unbounded table.
2. **Keep policy and measurement distinct.** The store supplies scoped candidate measurements. Any change to executive priorities, thresholds or cross-group allocation should follow the repository's kernel/configuration contract, not introduce another hidden Go ranking heuristic. Merely fixing the relational scope should not silently redefine the current scheduling policy.
3. **Make ordering and outcomes inspectable.** Define a stable tie-break for equal rate/use counts. Distinguish no eligible scoped strategy, intentional budget/cancellation deferral and lookup failure. The current caller discards the lookup error; do not let an implementation comparison count a database error as a successfully empty candidate set. These are supporting acceptance requirements, not claims of an additional reproduced incident.
4. **Do not conflate sampling with proof of improvement.** Keep provenance for which strategy/version and evidence drove a refinement. Improving candidate admission does not settle strategy causal credit, publication fencing, per-request byte bounds or rollout approval. Those require separate contracts and should not inflate this repair's claimed scope.

The existing `(problem_type, shard_type)` index is a starting point for scoped lookup. Whether an additional index helps eligibility/ranking should be measured with representative data rather than assumed. [Store schema and indexes][indexes]

## Costs and fair evaluation

The baseline can look cheaper precisely because it skips intended work. The corrected fixture may make one refinement call where the old path made none. That is not evidence of a regression in efficiency or a demonstrated quality gain: it is restored eligibility within the same authorized cap. Count the extra request, response, latency, persistence and downstream prompt costs, and assess whether the learned change subsequently earns its place.

Measure store rows examined/returned and allocations as the number of unrelated strategies grows. Include query and index-maintenance costs, model usage when reported, unknown-usage states, wall time, version writes/cache invalidation, candidate atom generation, and work deferred to later groups. Logical call counters are not transport-attempt or token counts. Do not hide failed refinements or follow-up costs, and do not claim token savings from a static query change.

An authorized implementation pass should begin with deterministic fixtures and a scripted model client; no paid model is needed to prove selection-order behavior. Keep the same store contents, current failure evidence, configuration, screening answer, generation result and cache state on both sides of the comparison. Test through `RunEvolutionCycle` as well as the scoped query so a handler-only success cannot conceal failure to reach the production handoff.

| Case | Required observation |
|---|---|
| Four-row witness above | B is considered and can refine when its group reaches the handoff with budget; A/C/D do not supply its evidence. |
| Unrelated rows added or made lower-performing | Current-group candidate membership remains unchanged, absent an explicit policy/budget change. |
| Same problem, different shard | Other-shard rows cannot consume the current shard's window. |
| Same shard, different problem | Other-problem rows cannot consume the current problem's window. |
| More than three eligible rows within one group | The scoped candidate limit still holds, with documented deterministic ordering. |
| Use-count and success-rate boundaries | Preserve minimum-use inclusion and the existing strict rate cutoff. |
| One call remains after atom generation | At most one refinement call occurs, even if several scoped rows qualify. |
| Zero calls remain or context is canceled | No refinement call; record intentional deferral rather than treating it as this defect. |
| Refinement call fails | The attempted call still consumes its existing share of the global allowance; no successful version update is claimed. |
| Multiple active groups | Total generation plus refinement calls never exceed the configured cycle allowance; observe which later groups are deferred. |
| Insufficient evidence, rejected screen, unacquired claim or generation error | Preserve the existing path that does not reach refinement. |
| Automatic evolution disabled or strategies disabled | Preserve no automatic purchase and no strategy refinement respectively. |
| Database read error | Surface a distinct lookup failure, not a false no-candidate result. |
| Repeated cycles with new current-group evidence | Demonstrate the scoped opportunity without reusing consumed evidence or promising unconditional fairness. |
| Successful refinement followed by prompt compilation | Verify the intended body/version can reach the existing strategy provider and normal selection; do not equate storage with model consumption. |

Report candidate identity and scope, per-category logical calls, relevant ledger spend, strategy versions, generated atoms, deferrals and unchanged safety outcomes. Quality evaluation should follow only after the deterministic eligibility contract holds, comparing matched tasks with identical total budget and documenting any extra refinement spend. The acceptance criterion is **correct scoped opportunity under unchanged global controls**, not maximum refinement activity.

## Recommended disposition

Implement a narrow scope-before-limit correction with a full-cycle regression fixture. Preserve the recently repaired screening and context-ownership paths. A follow-on fairness policy may be useful, but this finding does not require a new scheduler, wider automatic evolution, or weaker admission to remove the unrelated-row interference.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[screening-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-screening-cohort.md
[screening-fix]: https://github.com/theRebelliousNerd/codenerd/commit/07e08faaf85d8ac368e2084ea1e04f03287d2512
[lifecycle-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[context-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/jit-strategy-context-ownership.md
[context-fix]: https://github.com/theRebelliousNerd/codenerd/commit/26158523140758639d97991f5611790bf1585d40
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L85-L179
[evolver]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L88-L102
[strategy-init]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L318-L330
[maintenance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L374-L407
[deadline]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L78-L83
[cycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L46-L93
[order]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L152-L172
[group]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L175-L257
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L259-L270
[query]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L297-L317
[refine]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L373-L403
[outcomes]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L193-L231
[config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/prompt_evolution.go#L24-L69
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L234-L255
[provider]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L59-L131
[serving]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L149-L190
[indexes]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L72-L100
