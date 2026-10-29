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

## Post-implementation refinement: stored-fact ownership must support shared kernel clones

**Recommendation:** preserve the implemented clone and predicate-index optimizations, but establish immutable ownership of retained facts at admission and snapshot boundaries. The present implementation shares mutable argument storage with callers. Under an explicit external-API mutation sequence, this can desynchronize EDB rows, canonical index keys and evaluated atoms—and let cone restoration reintroduce a retracted row.

**Evidence boundary:** static source inspection only. No implementation, tests, inference, profiling or benchmarks were executed. **No production caller that mutates an admitted argument or exported snapshot was established.** This is a source-derived API correctness finding, not an observed production incident or authorization bypass.

At the final branch check, main was `356f65e0e6bb639adaa90ab36212daa714114871`, feature `feat/conversation-brief` was `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`, research was `a636eed04b7650ea4037c9494d86e7337d4df6b3`, and historical research was `c592ff223cd207fca4c29ae793d5eba57aa4781d`. All cited kernel ownership/index/evaluation files and the system adapter have identical main/feature blobs. The `Fact` conversion section is also identical, although the surrounding `types.go` file differs.

### What already works—and the missing precondition

The [performance audit’s as-built section](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/mangle-kernel-performance.md#as-built-2026-10-27) records predicate-bucketed EDB access, compiled retractions, cached cone plans and shared stored facts. Those improvements should not be described as missing or discarded without cost evidence.

Their ownership precondition is not fully enforced:

| Boundary | Source-grounded behavior |
|---|---|
| Input admission | `internFact` allocates an argument slice only when a top-level argument is a plain `string` or `MangleAtom`. Otherwise it returns the original slice. Even its copying path leaves nested mutable values shared. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts_intern.go#L19-L63) |
| Retained representations | Admission computes a canonical key, converts the fact, then retains the raw `Fact` in both `k.facts` and the index. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L446-L490) |
| Snapshot export | `GetBaseFacts` copies only the outer `[]Fact`; each `Args` slice still references internal storage. The iterator yields stored `Fact` values directly. [Accessor](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_accessors.go#L5-L16), [iterator](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L1306-L1329) |
| Clone | `cloneLocked` copies outer fact slices; the argument values remain shared. The index clone copies maps, not the nested contents of their `Fact` values. [Kernel clone](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval.go#L507-L580), [index clone](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_edb_index.go#L82-L95) |

The distinction matters: **the kernel not modifying a stored fact does not make that fact immutable if another owner can modify its backing storage.** The Go specification describes slices as references to underlying arrays; copying a struct containing a slice does not independently copy that array. [Go specification](https://go.dev/ref/spec#Slice_types), [Go’s slice explanation](https://go.dev/blog/slices-intro)

### Concrete API witness—unrun

Use a minimal, successfully analyzed kernel with:

- Numeric `probe/1` and `seen/1` declarations.
- A rule deriving `seen(N)` from `probe(N)`.
- No initial `probe` fact.
- Provenance disabled, no intervening program change, and otherwise ordinary cone eligibility.

Then:

1. Assert `probe(1)` using a caller-retained numeric-only argument slice. Materialize the kernel successfully.
2. Optionally clone it while clean.
3. Change the retained argument element from integer `1` to integer `2`, without invoking a kernel mutation method. An independent variant obtains the row through `GetBaseFacts` and changes that returned row’s `Args[0]`.
4. The raw EDB row and index’s stored `Fact` now contain `2`; the index key remains the previously computed `probe(1)`; the evaluated store still contains the atom for `1`. A previously created clone shares the altered argument storage. This external assignment invokes no generation or dirty-state update.
5. Call `RetractExactFact` for `probe(2)`. Its scan matches and removes the raw EDB row. However, compaction attempts to remove the index entry under the row’s **current** canonical key, `probe(2)`. The surviving entry is still keyed by `probe(1)`. Index removal simply returns when the requested key is absent. [Retraction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L917-L949), [compaction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L790-L822), [index removal](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_edb_index.go#L59-L69)
6. On the next eligible cone evaluation, the kernel removes touched store rows and restores EDB facts from the index bucket. The surviving entry supplies the altered `probe(2)`, and the rule can derive `seen(2)`—despite the successful retraction and reduced `FactCount`. [Cone eligibility and restoration](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval_cone.go#L247-L347)

This sequence establishes the conditional mechanism from source; it is **not an executed reproduction**. Forcing a full evaluation is an important control: it repopulates from `k.facts`, not the stale predicate bucket, so it does not demonstrate the same immediate resurrection. It also does not itself establish that the stale dedupe index was repaired.

### Scope and controls

- The numeric-only witness does not depend on malformed values, concurrency, external predicates, failed evaluation or missed reset invalidation.
- Some inputs are already protected at the top level: string/name interning copies their argument array; the special sanitizer copies arguments for three named priority predicates. Neither is general recursive ownership. [Sanitizer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L1195-L1203)
- `Query` and `QueryAll` construct fresh public argument slices from AST atoms. They are **not** the raw aliasing accessor in this finding. [Readback conversion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_query.go#L406-L448)
- Container inputs are genuinely supported: specified map/slice shapes are JSON-encoded into string atoms while the raw container remains in the retained fact.
- Do **not** claim pointers are uniformly rejected. The fallback accepts other JSON-encodable values when their encoding is nonempty and neither `null` nor `{}`; this can include nonnil pointers to exported-field structures. Explicit nil and unsupported/unencodable values are rejected. [Conversion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/types/types.go#L219-L325)
- A top-level argument copy alone does not isolate nested maps, slices, pointers or custom JSON-backed objects.

Real consumers make the contract consequential, without establishing that they currently mutate shared values: Dreamer uses `Clone` for projected safety evaluation; JIT creates private compilation scopes with `CloneWithIdentity`; snapshot export calls `GetBaseFacts`. [Dreamer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dreamer.go#L288-L315), [JIT scope](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_adapters.go#L124-L138), [export](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/cmd_snapshot.go#L144-L175)

### Proposed ownership contract

After successful admission, later changes to caller-owned objects must not change the admitted fact. Exported snapshots must not provide writable access to retained state. Internal clones may then safely share genuinely immutable representations.

Compare two bounded designs:

1. **Owned raw facts:** recursively snapshot the supported input shapes at admission; independently copy mutable output structures at snapshot export. Explicitly define treatment of fallback/custom JSON values rather than assuming arbitrary Go graphs can safely be deep-copied.
2. **Immutable admitted representation:** retain a canonical typed/serialized value as authoritative, and construct public views from it. Preserve Name/String identity, declaration-directed adaptation and existing raw-export compatibility—or document and validate a migration.

Do not merely increment the generation: an untracked external assignment cannot reliably trigger that increment, and a new generation would not repair an already mismatched index key. Do not restore per-fact deep copying on every Dreamer clone as the first response; ownership at admission/export can preserve cheap internal sharing.

### Proposed evaluation—none executed

| Fixture | Required observations |
|---|---|
| Numeric-only caller arguments changed after admission | EDB, evaluated facts, index and identity remain consistent with the admitted value. |
| Returned `GetBaseFacts` arguments changed | Neither parent nor clone changes. |
| Iterator-retained arguments changed after iteration | Same isolation; distinguish replacing the outer `Fact` from changing its backing arguments. |
| String/name input control | Existing top-level protection remains; nested mutable values still receive coverage. |
| Containers and accepted JSON-backed pointers | Capture admitted content once; subsequent caller mutation cannot alter retained truth. |
| Explicit nil, cycles and unencodable values | Existing rejection behavior remains clear; no panic or partial ownership transfer. |
| Exact/first-argument/predicate/batched retractions | No stale bucket entry, resurrection or false duplicate; index agrees with canonical EDB. |
| Clean clone, dirty clone, cone and forced-full paths | Snapshot isolation and successful closure equivalence hold across representations. |
| Normal `Query`/`QueryAll` result mutation | Already-correct readback isolation remains. |

The current iterator “copy” test replaces an outer slice element and checks only the count; it does not mutate nested arguments. The interning test explicitly expects numeric-only arguments to retain their original backing array. These definitions show where future coverage must change; they were not run here. [Snapshot test](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts_gaps_test.go#L456-L470), [interning tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts_intern_test.go)

Measure admission allocations/bytes, container serialization, snapshot-export cost, clone latency, cone preparation, GC and retained memory. Compare current behavior, admission/export ownership and immutable normalized storage separately. Include string-heavy production facts, numeric-only facts, nested values and repeated clones; charge copying where it actually occurs. No speedup or end-to-end token saving is established.

**Nonduplication:** this refines the implemented performance audit’s immutable-fact assumption. It is distinct from its failed-evaluation publication appendix and the fresh JIT audit’s reset/staged/heartbeat generation omissions: here a caller-held alias changes data outside any kernel mutation API.

**Checkable exit:** all admitted values and exported snapshots have explicit ownership semantics; external mutation cannot change retained facts; canonical keys, EDB rows and evaluated facts stay aligned; retractions cannot restore a removed row from an alias-corrupted index; internal clone sharing retains its measured benefits.


## 2026-10-28: make explanation budgets end-to-end, not just envelope targets

**Status:** source-predicted risk; no workload, tests or model execution performed. Main `356f65e0e6bb639adaa90ab36212daa714114871` and feature `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c` share the inspected envelope, provenance, command and renderer blobs. Existing opt-in recording, proof/depth limits, node/edge pruning, partial markers and isolated proof facts are real protections. This refines their scope, not a claim that provenance is absent.

The synchronous [`/explain` handler](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/commands_handlers_misc.go#L67-L125) forces evaluation, requests an envelope and renders it into chat. [Default settings](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/config/explain.go#L5-L27) specify three proofs, depth 32, 400 nodes, 800 edges and 64 KiB of proof-fact strings.

1. [Envelope extraction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_explanation.go#L83-L118) holds the kernel's exclusive lock and builds the upstream derivation before it applies node, edge and byte limits. The [adapter](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_provenance.go#L93-L119) passes proof and depth bounds upstream, not the envelope's other limits. The upstream [aggregate builder](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/provenance/recorder.go#L372-L409) visits its input facts; existing [count policies](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/intelligence.mg#L305-L333) establish relevant shapes, not a measured production workload.
2. The byte budget is a **soft target**. [Bounding](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_explanation.go#L120-L202) may return an oversized store when shrinking reaches one node or its retry ceiling, while setting truncation and reporting actual bytes. The bounded copy also allocates a premise slice at the source width before edge pruning. Even a positive one-byte setting cannot contain the metadata row.
3. The [chat renderer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/cmd_explain.go#L47-L98) recursively prints shared DAG nodes on every occurrence without a visited set or a separate output-byte limit. A bounded distinct-node graph can therefore expand substantially as a tree. The upstream [DAG printer](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/provenance/facts.go#L135-L173) illustrates a print-once-and-reference representation. No observed stall, crash or token cost is asserted.

**Proposed contract.** Separately budget recording, extraction visits/allocations, serialized proof facts and final rendered bytes. Stop extraction before allocating omitted structure, and carry explicit partial/omission metadata. Either make the byte limit hard, including metadata and an indivisible oversized root, or name it as a soft target. Render each shared node once and refer to IDs on later edges. Preserve epoch consistency and source ownership if reducing kernel lock duration; never expose mutable recording state to achieve that reduction.

**Future evaluation, not run.** Use a wide aggregate, oversized single fact and rule, a diamond DAG with repeated sharing, a budget below minimum metadata, multiple proofs sharing support and complete small proofs. Assert extraction visit/allocation limits, truthful omission attribution, hard or explicitly soft byte behavior, rendered-output limits, small-proof parity and unchanged execution facts. Independently account for recording, extraction, pruning retries, serialization, rendering, lock duration, peak memory, output tokens and retrieval/continuation of omitted proof. Compare equal evidence fidelity, not only smaller output. The current [budget test](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_explanation_test.go#L68-L103) checks reduction rather than a strict maximum.

## Follow-up: failed evaluation is not a readable snapshot

**Research status: source-supported failure mode; not implemented or reproduced here.** This follow-up inspected `main` at `356f65e0e6bb639adaa90ab36212daa714114871` and `feat/conversation-brief` at `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c` on 2026-10-28. The latter was 27 commits ahead with the former as its merge base. `kernel_facts.go`, `kernel_eval.go`, `kernel_eval_cone.go`, and `kernel_query.go` have the same contents on those pins. No code, tests, inference, profiling or benchmarks were run.

### What is new, and what is already built

The existing performance audit's **As built** section already records compiled retractions, predicate-bucketed EDB access, cached cone plans, evaluation attribution and the measured decision to retain simple clone stores. Its earlier recommendations must not be presented as still wholly unimplemented. This follow-up concerns a different invariant: **a public query must not report an incomplete evaluation as a successful fact snapshot**. It does not propose reviving retained-IDB incremental evaluation or replacing the measured clone-store choice. [Existing audit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/mangle-kernel-performance.md)

### The failure boundary in the current source

The cone path deliberately edits the current `k.store` in place: it removes touched predicates, restores their EDB, and runs the selected rules in that same store. Its comment explicitly acknowledges that an error leaves the store half rebuilt and relies on full evaluation to repair it. The full fallback builds a fresh store, but installs that store only after successful evaluation. If **both** attempts fail, the old `k.store` reference still names the partially changed cone store. Neither failure branch clears `initialized` or installs an independently checked readable-state marker. [Cone mutation and failure, lines 285–347](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval_cone.go#L285-L347); [fallback and full publication, lines 257–375](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval.go#L257-L375)

This is not a claim that the normal fallback is broken: **successful full fallback repairs the store**. Nor is every failed cone necessarily partially mutated; an eligibility refusal or error before removal has different consequences. The concerning case is a failure after in-place mutation, followed by a failed full fallback. A derivation-limit failure is one candidate for an authorized future fixture; both paths use the configured created-fact limit. No particular production workload is asserted to have hit it.

### Stronger sequential case: staged writes

The smallest source-supported sequence does not require a race:

1. Start with an initialized, successfully evaluated kernel whose `factsDirty` is false and whose store supports cone removal.
2. Admit a **new**, valid fact with `AssertWithoutEval`. It updates EDB and `dirtyPreds`, deliberately leaving `factsDirty` false. A duplicate does not meet this condition.
3. Call `EvaluatePending`. The pending predicate prevents its no-work return, so it evaluates. Cause the cone to fail after mutation and the full fallback to fail too.
4. The call returns an error, but does not set `factsDirty` or invalidate `initialized`. A subsequent ordinary `Query`, `QueryCallback` or `QueryAll` can take `ensureEvaluated`'s false-dirty fast path and return the incomplete store as success.

This is a state-lifecycle defect at the kernel API boundary, not simply a caller failing to inspect the returned evaluation error. A later reader may be a different consumer. [Staging and `EvaluatePending`, lines 720–767](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L720-L767); [`ensureEvaluated`, lines 421–448](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval.go#L421-L448); [query entry, lines 40–62](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_query.go#L40-L62)

### Separate concurrent-read case

Normal `Assert` and transaction writes mark the kernel dirty, so readers that enter **after** those writes normally retry evaluation and receive its error. Nevertheless, there is a narrower interleaving:

1. Reader R finishes `ensureEvaluated` against a clean kernel but has not yet acquired `k.mu.RLock`.
2. A writer admits a normal dirtying mutation; another evaluator obtains the write lock, partially mutates the cone store, fails the full fallback, and releases the lock.
3. R acquires its read lock. It checks `initialized`, not `factsDirty` or a successful materialization epoch, then reads the partial store with no evaluation error.

**Important counterexample:** the mere gap between `ensureEvaluated` and the read lock is not by itself proof of a stale-read bug. If evaluation succeeds, a concurrent query may legitimately linearize before or after the write. The failure above is different: the result can come from a store that was never a completed fixpoint. QueryCallback and QueryAll have the same read handshake. [`QueryCallback`, lines 194–214](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_query.go#L194-L214); [`QueryAll`, lines 332–360](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_query.go#L332-L360)

### Production reachability, and gates that must be credited

`Dreamer.evaluateProjection` uses the exact clone → stage → `EvaluatePending` sequence, but it **checks the error, returns unsafe and does not query the failed clone**. That is a successful caller-side defense, not evidence of a Dreamer permission bypass. The feature branch retains that defense while extending its result shape. [Main lines 288–316](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dreamer.go#L288-L316); [feature lines 295–323](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/dreamer.go#L295-L323)

`Dreamer.assertCriticalPathFactsLocked` also stages facts and calls `EvaluatePending`, this time on the attached kernel rather than a throwaway projection. It leaves `criticalPathsReady` false on staging/evaluation failure, so that Dreamer's simulations remain refused. That flag does **not** mark the general kernel unreadable for other consumers. Constructor/setup timing and whether any new prefix is admitted determine whether the sequential failure condition is reachable in a particular run. [Main lines 132–161](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dreamer.go#L132-L161); [feature lines 138–168](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/dreamer.go#L138-L168)

The concrete shared path is `VirtualStore.getDreamer` → `realKernelForDreamer` → `NewDreamer(realKernel, critical...)` → `assertCriticalPathFacts`. `getDreamer` obtains the production Cortex's catch-all `RealKernel` and passes that same pointer to the constructor; the constructor does not clone it. Its initialization mutex serializes Dreamer creation, not all general kernel readers. The pointer identity is rechecked before publishing the constructed Dreamer, which correctly prevents binding to a replaced kernel but does not repair a failed materialization. [Shared-kernel construction, lines 535–594](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store.go#L535-L594); [constructor, lines 37–48](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dreamer.go#L37-L48)

The outer sharding layer is not a general serialization barrier: `KernelShard.Assert` releases its own mutex before delegating to `RealKernel.Assert`, and `KernelShard.Query` similarly releases it before `RealKernel.Query`. `CortexKernel.EvaluateAll` invokes each shard's `EvaluatePending` and **propagates** an evaluation error. That is another existing defense; the issue is the failed kernel's subsequent readable state, not an assertion that Cortex swallows its error. [Shard delegation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_shard.go#L151-L168); [shard query](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_shard.go#L244-L267); [Cortex evaluation and error propagation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/cortex_kernel.go#L1049-L1089)

The outputs matter beyond diagnostics: chat reads `permitted`, `forbidden` and `safety_warning` for session context; VirtualStore's exact authorization check queries `permitted` and denies on query error. An incorrectly successful partial read removes that error signal. This inspection establishes a relevant consumer and failure contract, **not an observed unauthorized action**: exact action/target/payload matching, Dreamer and other route checks still apply. [Chat context reads](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_session_context.go#L1049-L1105); [exact authorization check](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store.go#L1167-L1237)

### Proposed contract, not an implementation prescription

Track the validity of a materialized snapshot independently from whether automatic evaluation was requested. A failed in-place evaluation must leave the kernel explicitly unreadable until successful repair, or retain a separately published immutable successful snapshot with an explicit stale/error contract. General executive reads must not silently treat an old snapshot as current authorization after a failed new evaluation.

Acquire the read lock and validate the materialization state under that lock before invoking callbacks or exposing rows. A pre-lock fast check can remain an optimization, not the proof of readability. Define staging separately: intentionally suppressing automatic evaluation while collecting a private batch must not suppress invalidation after a failed attempt. Ensure error state and successful publication are consistent across lazy evaluation, `Evaluate`, `EvaluatePending`, clones, reset and policy changes.

Do not solve this by unconditionally copying the entire fact store for every cone. The existing audit measured copying/GC as a dominant large-clone cost and deferred overlays. First compare a bounded validity/error-state plus checked read handshake against staged cone publication; the latter has broader ownership and allocation consequences. Preserve the current full fallback and do not cache authorization verdicts.

### Future verification and full costs

These are proposed checks, none executed here:

| Case | Required evidence |
|---|---|
| Clean kernel → new `AssertWithoutEval` → failed cone and full fallback | Subsequent Query/QueryCallback/QueryAll return an error or explicitly unavailable state; zero callback rows are exposed from the failed evaluation. |
| Controlled reader/evaluator interleaving | Pause after the clean fast check and before RLock using deterministic synchronization; run failed evaluation; resumed reader must not return partial rows. Do not rely on sleeps. |
| Cone failure, successful full fallback | Complete results equal a fresh full fixpoint, with no lingering invalid state. |
| Failure before any mutation; normal successful concurrent writes | No accidental regression in ordinary reads or unnecessary permanent poisoning. |
| Recovery after the failing cause is removed | Readability returns only after a successful complete materialization; the original cause stays attributable. |
| Both store kinds and clean-parent clones | Same semantics; parent isolation remains intact. |
| Actual Dreamer route | Existing unsafe-on-evaluation-error defense remains; attached-kernel setup failure cannot leak a readable partial snapshot to a different consumer. |

The inspected concurrency test checks completion without panic/deadlock and ignores query results/errors; it does not establish this invariant. Existing cone tests establish successful cone/full equivalence, not failed publication. [Concurrency test lines 143–195](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_query_gaps_test.go#L143-L195); [cone tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval_cone_test.go)

Measure total dirty-read latency, lock wait/hold, reader retries, fallback time, EDB/IDB peak memory, and repeated deterministic-failure behavior—not just successful engine duration. An error latch must not create an unbounded busy retry loop; a copy-on-write design must include its allocation and reclamation cost. End-to-end evaluation should report correctness/error visibility and model retries per correctly completed task, not claim token savings merely because failed reasoning returned fewer facts.

## As-built note (2026-10-28)

- Sections 3-7 (retraction plans, EDB buckets and cone plans, evaluation attribution, simple clone stores by measurement) were already built in `00690ca2`; sections 6 (upstream index) and the overlay option are ruled out here: upstream code, and no measured need.
- Failure atomicity is real and fixed: a failed evaluation (cone then full fallback) now latches `evalErr` and raises `factsDirty` (`kernel_eval_failure.go`); Query, QueryCallback, QueryAll and Explain refuse the store under the read lock, and a clone does not inherit it. Tests: `kernel_eval_failure_test.go`.
- Left: Dreamer's direct `kernel.store` projections read without the check (its evaluation path already fails closed); the dirty-read cost measurements were not run.

## As built (2026-10-29): layered strata

- Upstream's per-call cost proportional to the program (§ "Deferred" above) was measured: mangle-go v0.7 `evalStrata` rebuilds, for every stratum, a set of every predicate of the strata before it (then never reads it), so a call costs O(strata × predicates). The corpus has 1,315 components; a full fixpoint over the boot EDB took 122-134 ms, of which the strata themselves took 15 ms.
- `internal/mangle/strata_layers.go`: `mangle.Stratify` (used by the kernel and `mangle.Engine`) layers `analysis.Stratify`'s components by longest dependency path. A layer's components read nothing of each other, by body premise or by top-down evaluation (deferred premises, lattice merges), so each evaluates as it would alone; pairs joined by a top-down read keep their original order. 1,315 strata become 16 layers. Cone plans and inclusion closures keep their own members of a layer.
- Measured: full fixpoint over the boot EDB 122 ms → 18.5 ms with an identical store (6,922 facts); `go test ./internal/core` 403 s → 182 s (`TestConeEvaluationMatchesFullEvaluation` 113.5 → 19.1 s, its lattice variant 15.9 → 3.1 s). Tests: `TestLayerStrataPreservesEvaluation`, `TestLayerStrataShape` (`internal/mangle`), `TestProductionCorpusEvaluatesInLayers` (`internal/core`).
