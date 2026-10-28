---
doc-class: deep-dive
subsystem: core-mangle
implementation-status: partial
last-verified: 2026-10-28
verified-against: 2e0c54ef204997c36e65c91cbb90bc991e63c8c1
verification-method: static-source-inspection
supersedes: []
---

# Improving Mangle kernel performance without weakening executive truth

**Recommendation:** optimize the work around the existing conservative rederivation strategy before replacing it. Preserve argument indexes when cloning a large evaluated kernel, remove whole-world scans from cone preparation, and compile batched retractions into one pass. Measure cloning, preparation, evaluation, external calls and lock holding separately. These are source-supported opportunities, not measured bottleneck rankings or promised speedups; static inspection does not establish a soundness proof.

**Evidence boundary:** no implementation, tests, inference, training, profiling or benchmarks were executed for this document. All current CodeNERD claims use the pinned revision above. The dependency is `codeberg.org/TauCeti/mangle-go v0.7.0` (`go.mod`, require block, [line 9](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/go.mod#L9)). Upstream store inspection uses the corresponding accessible `google/mangle` v0.7.0 commit `7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`; do not infer the behavior of a newer engine from this source.

## 1 The workload to improve

The root vision assigns executive decisions to the Mangle fixpoint, not to Go heuristics or model discretion (`agents.md`, **The Vision**, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/agents.md#the-vision-steve-2026-09-18--read-before-any-architectural-change)). Performance work must make that decision process cheaper while preserving its conclusions, failures, external observations and provenance contract. It must not make the system look faster by skipping a safety derivation.

This document concerns `RealKernel`, its speculative clones and the mutations routed to it. It does not claim all Mangle instances share one runtime or one data store. `CortexTransaction.Commit` groups changes by target shard and creates one transaction per shard (`internal/core/cortex_kernel.go:897–985`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/cortex_kernel.go#L897-L985)); whole-system accounting must include that fanout.

Existing research is important but historical. `Docs/journeys/M3-mangle-verification.md`, **Harness**, pins its probes to an older v0.5.1-derived dependency ([source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/journeys/M3-mangle-verification.md#1-harness)). Its measurements are not fresh v0.7.0 results. `Docs/journeys/impl/S23-differential-path.md`, **Open**, records why performance pressure must not revive unsound retained-IDB evaluation ([source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/journeys/impl/S23-differential-path.md#open)). The proposals here refine the present implementation rather than rediscovering those investigations.

## 2 What the kernel already does

| Mechanism | Inspected source | Consequence for the proposal |
|---|---|---|
| Shared analyzed program cache | `rebuildProgram`, `internal/core/kernel_eval.go:103–130,195–215`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L103-L215) | Program analysis and stratification are already reused; do not propose recompiling less as though every query reparses the corpus. Sandbox candidates deliberately bypass the shared cache. |
| Dirty-epoch lazy evaluation | `ensureEvaluated`, `internal/core/kernel_eval.go:414–443`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L414-L443) | A singleflight lock coalesces dirty reads. Counting assertions is not counting evaluations. |
| Selective cone rederivation | `evaluateConeLocked`, `internal/core/kernel_eval_cone.go:210–312`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval_cone.go#L210-L312) | Deletes touched predicate rows, restores their EDB and rederives dependent rules. It is not naive append-only incremental evaluation. |
| External-dependent invalidation | `buildConeIndex` and `closure`, `internal/core/kernel_eval_cone.go:65–137`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval_cone.go#L65-L137) | External-premise heads and downstream consumers join every cone. Preserve this conservative treatment until an external revision contract exists. |
| Indexed large full evaluations | `newEvaluationFactStore`, `internal/core/kernel_eval.go:654–659`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L654-L659) | At 1,024 EDB facts the full path chooses the multi-indexed array store. Indexing is already implemented, not an unused engine capability. |
| Safe fallback and optional proofs | `evaluate`, `internal/core/kernel_eval.go:257–268,320–352`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L257-L352) | Cone ineligibility/failure falls back to a fresh full evaluation. Provenance is recorded on the full path; the cone path refuses eligibility when a proof recorder is active. |
| Existing closure comparison | `TestIndexedEvaluationPreservesProductionClosure`, `internal/core/kernel_indexed_store_test.go:13–56`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_indexed_store_test.go#L13-L56) | There is already a production-corpus indexed-versus-simple comparison definition, including a retract. This study did not run it. Extend its scope, do not claim the invariant has never been considered. |

## 3 Preserve indexes across speculative clones

`Clone` shares immutable analyzed program/strata/cone information and, when the parent is clean, copies its evaluated store to avoid a fresh full fixpoint (`internal/core/kernel_eval.go:507–547`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L507-L547)). That is an existing optimization worth preserving. But `copyFactStore` always constructs `SimpleInMemoryStore` and copies every atom into it (`:664–677`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L664-L677)). The parent may therefore use argument indexes while its clean speculative child uses predicate scans.

This has a real consumer: `Dreamer.evaluateProjection` clones the kernel, stages projected facts and calls `EvaluatePending` (`internal/core/dreamer.go:289–313`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/dreamer.go#L289-L313)). On an eligible clone, cone rules execute against the copied simple store. This is not a claim that every dream is expensive: a narrow cone with small input relations may benefit more from cheap copying than from rebuilding indexes.

The upstream difference is concrete. `SimpleInMemoryStore.GetFacts` scans a predicate's atoms and applies `Matches` (`factstore/factstore.go:163–173`, [source](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L163-L173)). `MultiIndexedArrayInMemoryStore.GetFacts` uses the first bound argument's hash bucket, then still checks matches (`:812–835`), and that store implements removal (`:848–854`, [source](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L812-L854)). A removable indexed copy is therefore an available building block, not a request for an entirely new engine.

### Proposed first experiment

Compare the existing clone strategy with a size/workload-aware indexed copy. Preserve all copied atoms, not just EDB; the optimization depends on the parent already being evaluated. Retain the dirty-parent full-evaluation path. Choose based on materialized relation size and expected lookup patterns, not blindly on the current EDB threshold, because materialized IDB can greatly exceed EDB.

For a clone, model the cost as:

```text
T_clone = T_copy_EDB_Go_facts + T_copy_evaluated_store + T_build_indexes
          + T_cone_prepare + T_rederive + T_query + T_cleanup
```

An indexed clone wins only when the saved lookup work exceeds its extra index-construction and memory cost. Measure this with tiny and large cones and sparse and dense joins. Do not report lookup-only speed while excluding the copy/build phase paid on every speculative request.

### Later option requiring a stronger design

Immutable parent snapshots plus a private overlay could remove repeated whole-store copying, but a naive merged store is not sufficient. Upstream `MergedStore.Remove` only removes from its write store (`factstore/factstore.go:240–264`, [source](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L240-L264)). A cone needs to hide old derived rows inherited from the parent. An overlay must support predicate tombstones or exact masking, deduplicate reads, and never mutate parent indexes. External calls, source revisions, lifetime ownership and failed child evaluation also need explicit handling. Keep this option behind evidence that indexed copying is still the dominant cost.

Do not cache authorization verdicts to avoid the clone. `Dreamer.SimulateAction` explicitly refuses such reuse without a complete fact/policy/payload/external identity contract (`internal/core/dreamer.go:165–170`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/dreamer.go#L165-L170)). Faster computation and stale permission reuse are not equivalent optimizations.

## 4 Make cone preparation scale with the touched domain

The cone reduces evaluated rules, but its preparation still touches global structures:

- It lists all store predicates, selecting touched ones (`evaluateConeLocked`, `internal/core/kernel_eval_cone.go:232–247`).
- It scans every `k.facts` EDB row to restore touched predicates (`:249–261`).
- It scans every analyzed rule and stratum to build a subprogram (`:263–287`).

[All three source ranges](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval_cone.go#L232-L287) are mechanical overhead even if the affected logical cone is small. Let N be EDB rows, R rules, P predicates and C the touched predicate set. Current preparation includes O(N + R + P) selection work, plus removal/restoration and derivation. This is an asymptotic source model, not a profile result.

### Proposed maintained access paths

1. Alongside canonical EDB identity, maintain a predicate-to-fact bucket index. Every admitted assertion, retraction, load, reset and clone must update it under the same mutation boundary. Use immutable atoms or stable IDs rather than slice offsets that become invalid during compaction.
2. At program compilation, build rule-head-to-rule and predicate-to-stratum member indexes. These are immutable with the analyzed program and can be shared with clones. A policy/schema change creates a new index identity.
3. Construct a cone plan by unioning reachable predicates and selecting their rule/stratum buckets. Preserve the original stratum order and complete strongly connected components. No reordering across negation or aggregation dependencies is authorized by this optimization.
4. Restore EDB only from touched buckets. Remove all old rows of every affected predicate before rederivation, including predicates that can also contain EDB. Preserve program initial facts and external-dependent closure.
5. Cache only immutable cone plans under `(program identity, canonical written predicate set, evaluation option profile)`, with bounded storage. Do not cache result facts or permission answers under that key.

The intended preparation cost becomes proportional to reachable dependency edges, affected rule/stratum entries and touched EDB rows, plus unavoidable engine work. This does not claim total evaluation becomes O(delta): a single changed fact can invalidate a large recursive relation or aggregate. Large cones may still favor the existing full path. Any adaptive choice must change only physical execution, not which obligations are derived.

### Correctness guardrail

The current index includes negative and temporal premises in dependency extraction (`premisePredicates`, `internal/core/kernel_eval_cone.go:52–62`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval_cone.go#L52-L62)). A new planner must preserve those dependencies and widen conservatively for any unsupported expression shape. Cache invalidation includes changed learned rules, declarations, engine options and external contracts. The safe fallback remains complete recomputation, not an empty or partial answer.

## 5 Compile transaction retractions into one traversal

**Fresh-source reconciliation:** main advanced during this research. At 02:02 UTC, `2e0c54ef204997c36e65c91cbb90bc991e63c8c1` included `da587e9`'s incremental dedupe-index maintenance: `compactFactsLocked` now deletes only removed facts' canonical keys, transactions no longer rebuild the entire dedupe index after retracts, and boot facts enter through deduplication (`kernel_facts.go:814–850`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_facts.go#L814-L850); `kernel_init.go:62–77`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_init.go#L62-L77)). The new `TestFactIndexFollowsRetractionsIncrementally` defines parity checks against a rebuilt index ([source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/fact_index_incremental_test.go#L10-L66)); it was not executed here. Credit that implemented improvement. The remaining proposal below concerns repeated EDB selection traversals, not the now-removed index rebuild. Other cited kernel evaluation, clone and cone source files were unchanged between the initial inspection and this pin.

`KernelTransaction.Commit` uses one kernel lock and marks one lazy rebuild, but it does not make all mutation work one-pass. Each full-predicate, first-argument and exact-row retraction invokes `compactFactsLocked` separately (`internal/core/kernel_transactions.go:111–150`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_transactions.go#L111-L150)). With K retraction conditions and N facts, the worst-case selection work is O(KN), before assertions and later evaluation. Repeated no-match retractions still scan.

This pattern has a concrete producer: orientation `RefreshResult.Apply` queues one exact retraction for each removed row (`internal/orient/snapshot.go:362–379`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/orient/snapshot.go#L362-L379)). Cortex preserves individual exact retractions when routing into each shard transaction (`internal/core/cortex_kernel.go:956–983`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/cortex_kernel.go#L956-L983)). The volume and observed time of these batches remain unmeasured here.

### Proposed removal plan

Compile buffered retracts into three sets: whole predicates, predicate-plus-first-argument identities, and exact fact identities. Within the existing locked commit, traverse the EDB once and keep a fact only if none of those sets match. Update affected dedupe and predicate indexes during that traversal and apply assertions in their existing phase.

There is already a useful local precedent: `RetractExactFactsBatch` creates a canonical-key set then filters once (`internal/core/kernel_facts.go:972–1009`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_facts.go#L972-L1009)). Reuse its identity semantics where equivalent; do not call a public locking method recursively from inside `Commit`.

The expected selection work is O(N + K) hash-table operations, plus key construction/comparison costs, assuming a compatible equality encoding. The qualification matters: current first-argument and exact comparison use `argsEqual`/`argsSliceEqual`, including cross-Go-type cases (`internal/core/kernel_facts.go:1053–1158`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_facts.go#L1053-L1158)). A convenient new hash key cannot silently change those semantics. Validate equivalence before replacing a scan, or retain a bounded equality-checked bucket fallback for noncanonical composite arguments.

Preserve the transaction's actual contract: rejected assertions are collected while valid assertions can land; retractions precede assertions; the event bus publishes asserted predicates after unlocking (`KernelTransaction.Commit`, `internal/core/kernel_transactions.go:157–222`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_transactions.go#L157-L222)). This performance proposal does not silently turn it into an all-or-nothing database transaction or change cross-shard atomicity.

## 6 A selective upstream index opportunity

The pinned multi-indexed array lookup picks the **first** nonvariable argument, not the smallest available bound bucket (`GetFacts`, upstream `factstore/factstore.go:819–832`, [source](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L819-L832)). A query binding a low-selectivity early argument and highly selective later argument can therefore examine more candidates than necessary.

A candidate upstream refinement is to maintain cheap bucket-cardinality estimates and choose the smallest bound bucket, while checking every argument with the existing matcher. With b bound positions and selected bucket size m, target selection/scan work is O(b + m), excluding hashing and equality costs. Computing exact bucket lengths by traversing every candidate bucket would erase the intended saving; update counts on insert/remove or choose a simpler approximate estimator. All-variable queries still require enumeration. Collision handling, zero-arity predicates, removal and repeated-variable semantics remain unchanged.

This is a secondary experiment, not a prerequisite to fixing clone representation. A synthetic skewed relation can show the mechanism, but only the production query mix establishes value. Indexing every argument already has write and memory costs; an additional cardinality table must earn its overhead.

## 7 Measure the complete path before choosing the next optimization

`EvaluationStats` currently exposes only input fact count and total duration (`internal/core/kernel_types.go:121–132`, [source](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_types.go#L121-L132)). `evaluate` receives per-stratum engine stats but sums/logs durations on the full path; cone evaluation discards returned engine stats (`internal/core/kernel_eval.go:337–361` and `kernel_eval_cone.go:301–311`, [full](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval.go#L337-L361), [cone](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/core/kernel_eval_cone.go#L301-L311)). This supports finer attribution, not a claim there is no existing logging.

Proposed bounded sample records should distinguish caller and shard, full/cone mode, eligibility/fallback reason, EDB/materialized sizes, dirty predicates, touched rows, selected rules/strata, clone bytes, store kind, preparation time, engine time, external time/call count, lock wait/hold, allocations and peak memory. Use stable program fingerprints rather than logging all fact payloads. Sample traces, bound cardinality, redact sensitive values, and include telemetry overhead in comparisons.

For an end-to-end action, use disjoint stage timings where possible:

```text
T_action = T_admission + T_projection + T_clone + T_cone_preparation
           + T_fixpoint + T_result_read + T_effect_and_validation + T_other
```

Nested spans cannot be summed as independent costs. If measured fraction f belongs to the optimized stage and that stage improves by factor s, the familiar upper-bound model is `1 / ((1-f) + f/s)` before added overhead. [Amdahl's original paper](https://www.cs.cmu.edu/~18742/papers/Amdahl1967.pdf) motivates considering unaffected work; its historical numerical estimates do not describe CodeNERD. Kernel time savings do not automatically reduce model tokens. Token gains require a separate causal mechanism, such as less timeout-induced rework at unchanged decisions.

## 8 Evaluation and rollout contracts

No checks below were executed. Compare each proposed variant with both the current optimized implementation and a complete fresh-EDB reference evaluation. Keep policy, source data, engine version, external observations, gas limits and requested results identical.

| Variant | Distinct question |
|---|---|
| Current clone and cone | What is the real baseline, including existing caches and indexes? |
| Indexed clone only | Do saved joins outweigh per-clone index building? |
| Predicate/rule-bucket cone only | How much is preparation rather than fixpoint work? |
| Compiled retraction sets only | Does realistic refresh batch size justify the change? |
| Combined narrow changes | Do the independent improvements compose without shifting semantics? |
| Snapshot overlay or selective bound bucket | Do higher-complexity extensions improve the measured remaining bottleneck? |

Vary EDB size around the current threshold and through tens/hundreds of thousands of facts, IDB expansion ratio, rule count, cone fraction, recursive depth, relation skew, exact/first-argument/predicate deletion mixes, no-op changes, clone concurrency and shard fanout. Include cold boot, warm steady state, policy change, dirty-parent clone and proof-enabled full fallback. Use representative persisted-state shapes without publishing private payloads.

Correctness witnesses must include: negated premise becoming true; aggregate value replacement; an alternate derivation surviving deletion; program facts in touched predicates; empty cone; external output change; policy/schema replacement; name/string and numeric identity; clone mutation isolation; canceled/failed evaluation; and fact-limit exhaustion. Compare complete fact sets and error classes, not only one desired answer. Preserve proof completeness when requested and verify no external callback disappears or gains an unauthorized side effect. Test definitions are a plan, not evidence of a pass.

Measure p50/p95/p99 latency, throughput under concurrent readers/writers, peak memory, copied bytes, parser/engine/GC CPU and lock contention. For complete coding tasks, keep model, budget, approvals, tools and held-out behavioral acceptance fixed. Report verified-task rate, actionable refusals, human repair, model input/output totals and non-additive cache/reasoning subcounts. Do not improve a cost-per-success metric by silently dropping hard cases.

A staged future rollout should first compare selected pure/replayable workloads with the full oracle, then admit production use only after equivalence evidence. Do not shadow-run live side-effecting external callbacks twice. Capture a permitted immutable observation fixture or use pure test callbacks for dual evaluation. Any mismatch fails the optimization closed and records a minimal authorized diagnostic.

## 9 Research foundations and implementation order

[Gupta, Mumick and Subrahmanian, *Maintaining Views Incrementally* (1993)](https://www.cs.columbia.edu/~gravano/Qual/Papers/13%20-%20Maintaining%20Views%20Incrementally.pdf) provides the relevant foundation: deletion requires accounting for lost and surviving derivations; recursive maintenance can delete a superset and rederive. This does not prove CodeNERD's cone implementation is DRed, nor that tuple-level maintenance is automatically cheaper. Its own introduction notes workloads where recomputation is preferable. The immediate proposals optimize physical access around the existing conservative rederivation boundary; a new incremental engine would require a separate correctness and applicability argument.

| Local work item | Proposed next step | Checkable exit |
|---|---|---|
| KERNEL-PERF-01 | Attribute real action cost | Representative run separates clone, preparation, engine and lock cost with bounded measurement overhead. |
| KERNEL-PERF-02 | Preserve useful clone indexing | Full closure, isolation and failure parity hold; end-to-end speculative action latency and allocations reported against current clones. |
| KERNEL-PERF-03 | Index cone preparation | Small-cone preparation stops scanning unrelated EDB/rules; complete-corpus/retraction/external equivalence maintained. |
| KERNEL-PERF-04 | Compile removal batches | Mixed retraction batches preserve existing equality and transaction outcomes; operation count scaling and whole refresh cost reported. |
| KERNEL-PERF-05 | Evaluate selective bound lookup | Skewed and realistic workloads establish whether smallest-bucket selection earns its maintenance overhead. |
| KERNEL-PERF-06 | Consider immutable overlays only if needed | Parent isolation, masking, deduplication, external observation and memory lifetime contracts demonstrated before claiming copy avoidance. |

These are research work items, not adopted repository gap IDs. The practical order is measurement plus the smallest index-preserving clone experiment, then preparation and retraction access paths. Keep the executive in Mangle. Make it do less irrelevant work, not less required reasoning.

## As built (2026-10-27)

Already on main before this pass: `9dc23e1e` (clean-parent clones copy the evaluated store; dream checks use `EvaluatePending`) and `da587e91` (incremental dedupe-index removal on retract).

**Implemented**

- **KERNEL-PERF-04, compiled retractions.** `internal/core/kernel_retract_plan.go`: `compileRetractions` turns a transaction's whole-predicate, predicate-set, first-argument and exact retractions into one `retractionPlan`; `KernelTransaction.Commit` (`kernel_transactions.go`) compacts the EDB once with `plan.keeps`, and skips the pass when no named predicate has rows. Hash keys (`valueKey`, allocation-free) only select candidates; every match is confirmed with `argsEqual`/`argsSliceEqual`, so the cross-type semantics are unchanged (unkeyable values go to a linear list; `-0` keys with `0`). Transaction contract unchanged: retractions before assertions, partial assertion rejection, publish after unlock.
- **KERNEL-PERF-03, cone preparation.** `kernel_edb_index.go`: the dedupe index is now `edbIndex`, bucketed by predicate (canonical key → stored `Fact`) with a monotone per-name arity history, maintained by every admit, retract, heartbeat replace, boot inject, clear and clone. `kernel_cone_plan.go`: `coneIndex` (built after stratification, shared through the compiled-program cache) carries `rulesByHead`, `strataByPred` and per-name program symbols, and a bounded (256) cache of immutable `conePlan`s keyed by the written-predicate set. `evaluateConeLocked` removes old rows by symbol (program symbols ∪ EDB arity history) instead of `ListPredicates`, restores only touched EDB buckets, and hands the engine the plan's rules and strata in original program order.
- **KERNEL-PERF-02, clone representation.** `Clone` shares the immutable stored facts (the per-fact deep copy is gone) and clones the index structure; `copyFactStore(src, dst)` copies into a simple store.
- **KERNEL-PERF-01, attribution.** `EvaluationStats` (`kernel_types.go`) records mode, store kind, prepare vs engine time, touched/removed/restored counts, rules, strata, and why a named write fell back to the full path (`coneIneligibilityLocked`). `Dreamer.evaluateProjection` logs clone time plus these per check.
- Found along the way: `rebuildProgram` cleared `policyDirty` and installed the new `programInfo` before stratifying, so a non-stratifiable program left new rules beside old strata. It now installs nothing until stratification succeeds.

**Measured** (`internal/core/kernel_perf_bench_test.go`, `-benchtime=30x`, median of 3, shared workstation; before = this branch's base, after = this commit):

| Benchmark | Before | After |
|---|---|---|
| `TransactionMixedRetractions` 60k EDB, 1,000 exact + 20 first-arg + 1 predicate | 538 ms | 6.9 ms |
| same, 50 exact | 34.1 ms | 6.1 ms |
| `ConeSmallWrite` one-rule cone, 60k EDB | 754 µs | 126 µs |
| `ConeSmallWrite`, 1.5k EDB | 80 µs | 73 µs |
| `DreamCloneConeEvaluation` clone + stage + cone + read, 60k EDB | 75.4 ms, 141k allocs | 67.5 ms, 81k allocs |
| same, 1.5k EDB | 1.03 ms, 3.9k allocs | 0.93 ms, 2.4k allocs |

Equivalence, not just speed: `kernel_perf_test.go` checks cone results against a fresh full fixpoint after asserts, a negated premise turning true and retractions, on both store kinds and on clones (`TestCloneStoreKindAndConeEquivalence`), the retracted-undeclared-arity case (`TestConeRemovesRetractedUndeclaredArity`), plan selection against a whole-program scan on the production corpus (`TestConePlanMatchesProgramScanOnProductionCorpus`), compiled retractions against sequential passes over a cross-type value zoo with index parity (`TestCompiledRetractionsMatchSequentialPasses`), and transaction fixpoint equivalence.

**Ruled differently**

- Clone stores stay simple. `BenchmarkCloneStoreKind` (copy + cone + read, timed together) measured an indexed copy at 1.6 vs 0.8 ms (synthetic 1.5k EDB), 96 vs 63 ms (synthetic 60k EDB, a bound join that the index does help), and 26.8 vs 13.3 ms (production corpus, dream projection). Building argument indexes per clone costs more than the joins it saves.
- Plans cache the physical selection only (rules, strata, touched names). They don't cache results or permission answers. Program identity is implicit: the cache lives on that program's `coneIndex`. The bound is an internal constant like `compiledProgramCacheMax`, not a user setting.
- `RetractExactFactsBatch` keeps its canonical-key identity; the transaction keeps `argsEqual` identity. Each public API's semantics are unchanged.

**Deferred**

- KERNEL-PERF-06 overlays: the 60k-EDB profile now shows store copy plus GC dominating a dream check (the cone itself is small), which is the evidence §3 asked for. The masking, tombstone and lifetime design is still unbuilt.
- KERNEL-PERF-05 smallest-bound-bucket lookup: this is an upstream `factstore` change. Also upstream, `EvalStratifiedProgramWithStats` copies every Decl and rebuilds per-stratum EDB sets on each call, a cost proportional to the program rather than to the cone.
- Lock wait/hold, peak memory and external-call timing in `EvaluationStats`. Per-stratum engine durations are summed, not exposed.
