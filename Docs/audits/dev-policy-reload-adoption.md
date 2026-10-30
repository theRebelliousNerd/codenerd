---
doc-class: deep-dive
subsystem: dev-policy-reload
implementation-status: implemented
evidence-status: source-only
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Dev policy reload needs coherent adoption and truthful failure reporting

## Decision and scope

Preserve development-mode policy hot reload and its compile-before-install protection, but make adoption cover the participating shard programs, their evaluation readiness and the derivation map as one identified generation. An error after installation must not be reported as proof that the running policy is unchanged. The current implementation has explicit return paths after replacing programs, and a routing-map failure leaves the previous map installed.

This finding is limited to the **opt-in `dev.policy_hot_reload` path in codeNERD's own source tree**. The setting defaults off; the adapter requires a workspace whose `go.mod` names `codenerd` and which contains the policy corpus. It does not establish exposure in ordinary foreign-workspace sessions or a remotely exploitable permission bypass. The observable consequence supported by source is inconsistent adoption/disposition handling under the conditions below, not a reproduced outage or demonstrated unsafe action. [Configuration](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/config/dev.go#L3-L20), [source-tree admission](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/dev_policy.go#L36-L84).

All implementation links are pinned to public main [`fef98230c20a5751e4c32a2fd2757832a9d39cdc`](https://github.com/theRebelliousNerd/codenerd/commit/fef98230c20a5751e4c32a2fd2757832a9d39cdc), revalidated as main at approximately 06:10 UTC on October 30, 2026. The hot-reload implementation was introduced in [`0dac4d12`](https://github.com/theRebelliousNerd/codenerd/commit/0dac4d129bfe7f5ede94533b0792f287e29ffedd). This audit read public source and existing test definitions only. No repository code, tests, model inference, benchmarks, watcher events or failure injections were run. Remediation and evaluation are proposed.

## Research context

[Lamport’s event-ordering work](https://lamport.azurewebsites.net/pubs/time-clocks.pdf) describes why concurrent event ordering must be explicit. It motivates reasoning about a coherent adoption generation here; it does not itself prove a CodeNERD failure or select a repair algorithm.

## Existing protections to retain

`prepareConstitution` assembles the candidate while preserving the kernel's appended schema/policy suffixes and includes its learned rules in compilation. It performs analysis and stratification before returning a swap. A preparation error occurs before this call installs anything. Cortex preparation visits all participating shards before entering its installation loop. These are substantive protections: ordinary malformed or unstratifiable source is not simply copied into the running kernels. [Preparation](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L142-L184), [Cortex prepare phase](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L226-L240).

Installation takes the individual kernel mutex and compares its current schema and policy strings with those captured for that swap. If either changed, that individual installation refuses to overwrite it. On success it updates the constitution prefix, marks policy state dirty and refreshes the schema validator. Credit this coherent per-kernel check; the missing property is a group-wide adoption boundary, not a complete absence of locking. [Individual install](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L187-L205).

The existing core test exercises a valid edit, parse rejection and stratification rejection while retaining an earlier derivation. The system test boots an opted-in source-tree Cortex, applies a valid rule through the watcher, then submits invalid syntax. Those are useful intended contracts. They do not test a failure after one or more installations or during the later map rebuild, and they were not executed for this review. [Core test](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_reload_test.go), [boot and watcher test](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/dev_policy_test.go).

## Where failure no longer means unchanged

### Installation and evaluation are separate

The single-kernel route calls `swap.install()` and then returns `k.Evaluate()`. If evaluation fails, the source strings and constitution prefix have already been replaced; this function has no restoration branch. The source comment promising an untouched program on any failure is broader than the implementation. This does not mean every accepted program fails evaluation, nor does it establish what every evaluator failure does to materialized facts. It establishes that the installed program is not rolled back by this route. [Single-kernel reload](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L208-L220).

The strongest proposed witness needs no concurrent reload: keep a valid accepted baseline, then supply a compile-valid edited rule set whose evaluation over the retained facts exceeds the configured derived-fact limit. Compilation is not evaluation of that workload. The evaluator returns an error on fixpoint failure before publishing its newly built store, but the earlier reload installation has already replaced the program. Existing failure tests define derived-fact-ceiling examples and explicitly exercise `Evaluate` failure; they are evidence of an available test seam, not an executed hot-reload reproduction. [Evaluation error boundary](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/kernel_eval.go#L398-L411), [explicit evaluation](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/kernel_facts.go#L817-L835), [existing failure fixtures](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/kernel_eval_failure_test.go).

Credit the existing fail-closed read protection: evaluation settlement latches the error and marks facts dirty; `readableLocked` refuses the failed evaluation's store, and subsequent reads retry or report failure. This materially narrows the finding. It is **not** evidence that partially evaluated facts are freely served as successful results, nor that every failure causes an unsafe decision. The latch protects reads but does not restore the earlier installed policy, so the unchanged-policy notice remains incorrect for this witness. [Failure settlement and read guard](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/kernel_eval_failure.go#L5-L34).

Cortex similarly installs prepared swaps sequentially, then evaluates shards sequentially. A later installation conflict returns immediately without undoing the earlier successful installs. A later evaluation error returns after the swap loop has completed and may follow successful evaluations of other shards. A replaced/specialized shard is skipped during preparation and reported only after the other swaps and evaluations. Thus even the explicit skipped-shard error can accompany successful adoption elsewhere. [Cortex installation, evaluation and skipped-shard result](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L226-L254).

There is no global adoption/quiescence barrier in this method around preparation, all installs, evaluation and routing-map publication. Per-kernel locks protect their own operations; they do not make the sequence a single cross-shard transition. A mixed observation requires an overlapping consumer or a late failure; this audit does not claim such an interleaving was observed. The stock watcher invokes its own reloads serially in its event loop. The later-install-conflict case is therefore a conditional API-level concurrency case requiring a separate program-changing operation between preparation and installation, not a claim that this watcher routinely launches overlapping reloads. The post-install evaluation witness above does not require that race.

### Preparation does not capture one source generation

Every shard's `prepareConstitution` independently calls `assembleConstitution(constitutionFS())`. Assembly reads the source files from the chosen filesystem; it is not supplied one immutable corpus assembled by the enclosing Cortex operation. If source edits occur between those reads and both resulting candidates compile, compilation alone does not establish that the shards prepared the same corpus. Even a single assembly is a sequence of file reads, not a filesystem snapshot. [Filesystem assembly](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L75-L128), [per-shard read](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/constitution_source.go#L147-L164).

The watcher's debounce reduces bursts of edits but does not lock files during preparation or freeze their contents. It later invokes reload and publishes the resulting error. A subsequent event may repair the state by triggering another attempt; that is possible recovery, not a guarantee that the intermediate attempt was coherent or unchanged. [Watcher and invocation](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/dev_policy.go#L98-L182).

### Routing-map failure retains the old map

The system adapter first calls `ReloadConstitution`. Only after success does it rebuild the derivation map. That rebuild calls `DefaultCorpusText`, which rereads the selected source filesystem instead of consuming the already adopted candidate bytes. A required source becoming unreadable between the two stages is one concrete condition for this later read to fail. The result is returned after the corpus replacement, not before it. No such filesystem transition was executed here. [Adapter ordering](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/dev_policy.go#L187-L200), [map builder](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory.go#L1617-L1635), [corpus reads](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/kernel_types.go#L301-L338).

On that error the adapter does not call `SetDerivationMap(nil)` or otherwise clear the previous map. Its message says shared facts go everywhere until the next reload, but the fallback requires a nil map. With a pre-existing nonnil map, the old map remains. `targetShards` continues to narrow shared-fact replication by that map's consumers. If the new rules change which shards consume a shared predicate, old routing can therefore disagree with the installed program. The precise missed derivation or downstream decision depends on the changed rules and arriving facts; none is claimed as a measured incident. [Nil-map contract and assignment](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/cortex_kernel.go#L167-L191), [routing consumer](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/cortex_kernel.go#L207-L263).

Even a successful later map read needs generation agreement: a valid edit between corpus adoption and map construction could yield a map for different source bytes. Merely returning nil from both stages does not bind them to one generation.

### The visible disposition overstates rollback

`PolicyReloadEvent` carries files and an error, not an adoption disposition or active generation. The publisher treats every error as a failure where the running program stays. The chat handler tells the user the running policy is unchanged for every error. Neither distinguishes pre-install rejection from partial installation, post-install evaluation failure or map-build failure. The stronger assurance is therefore unsupported on the paths just traced. [Event and publisher](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/dev_policy.go#L28-L34), [publisher](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/dev_policy.go#L203-L213), [user-visible message](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L288-L301).

## Proposed adoption contract

Update the owning core/system specifications before code, keeping this as host-side integrity of policy loading. Mangle still supplies executive decisions; Go must not substitute a second decision engine or relax permissions to make reload succeed. The proposal does not authorize codeNERD to edit protected safety or permission policy.

1. **Capture an identified candidate.** Assemble one bounded corpus snapshot and derive a content identity. Use those exact bytes for every participating shard and its routing analysis. A snapshot needs an explicit consistency rule for multi-file edits; debounce alone is not that rule.
2. **Stage all required components.** Preserve current compile/stratification rejection. Define which shard specializations participate and whether any skipped shard makes the attempt ineligible. Include the appended/learned inputs that influence validation in the version check.
3. **Adopt under a coherent boundary.** Choose a quiescent publication point or equivalent generation-aware coordination for readers, mutations, program replacement and routing-map changes. Individual compare-and-swap checks remain useful, but a later conflict must not strand an earlier installation while reporting no change.
4. **Define evaluation failure explicitly.** Prefer an accepted bundle whose required evaluation readiness is established before it becomes authoritative. If staging evaluation is impractical, specify a blocked/degraded state and recoverable reconciliation instead of claiming rollback. Any rollback must preserve concurrent legitimate facts and program changes; blindly copying old fields back can create another ownership defect.
5. **Bind routing and inventory to adoption.** Build the map from the candidate bytes, and account for facts needed by newly consuming shards. Swapping a map controls future routing; it does not by itself prove that previously held shared facts have been hydrated into every new consumer. A nil-map fallback is only one possible degraded policy and needs explicit installation, validity and recovery rules.
6. **Report actual disposition.** Distinguish rejected-before-adoption, adopted-ready, adopted-but-not-ready, partial/conflicted and restored-to-verified-prior-generation. Emit active/candidate identities and the failed stage without dumping policy contents. Say unchanged only when that was established.

The smallest safe implementation may use existing kernel state and synchronization rather than clone every store. This audit does not prescribe a new framework. A message-only correction is worthwhile but does not repair generation coherence; adding rollback without protecting readers and concurrent changes is also insufficient.

## Proposed evaluation and costs

No case in this table has been run for this audit. Use deterministic fixtures and controlled synchronization before any live-session probe. Exercise the actual system adapter and chat projection as well as the core helpers.

| Case | Required result |
|---|---|
| Disabled option or foreign workspace | Existing embedded-policy behavior remains; no new reload authority. |
| Stable valid corpus and multiple participating shards | One source generation, coherent programs and map, preserved supported facts and accurate ready status. |
| Parse or stratification rejection during preparation | No program/map adoption; existing precompile protection remains. |
| Program change on a later shard between prepare and install | Either no group adoption or explicit safe conflicted disposition; never an unsupported unchanged claim after earlier swaps. |
| Compile-valid changed rules exceed the evaluation fact ceiling without concurrency | Prior generation demonstrably restored or explicit not-ready recovery state; fail-closed reads remain protected and the UI does not say unchanged after adoption. |
| Specialized shard whose constitution was replaced | Defined admission/skip policy, with an outcome that identifies what did and did not change. |
| Source changes between shard preparations | Reuse one captured generation or reject/retry the incoherent candidate before publication. |
| Map source read fails after candidate preparation | No authoritative new-program/old-map pairing; accurate disposition and bounded recovery. |
| Source changes before a successful map build | Map identity still equals the adopted corpus identity. |
| Changed shared-predicate consumer with already-present facts | Newly responsible shard receives the needed current inventory, not only future mutations. |
| Concurrent query or fact mutation during adoption | Observations satisfy the selected generation contract; no loss of legitimate concurrent changes. |
| Repeated failed attempts then a valid edit | Recovery reaches one identified ready generation without leaked candidate state or false unchanged notices. |

Capture program identities, active map identity, relevant per-shard facts/derived results, disposition and exact failure stage. Compare failure outcomes with an untouched control rather than checking only an error string. Test late conflicts with barriers or injected seams instead of hoping an uncontrolled timing race occurs. Use a finite, deliberately failing evaluation fixture on the actual pinned engine; do not assume that compile success proves runtime evaluation success or that an arbitrary malformed rule reaches the intended stage.

The resource tradeoff is local CPU, memory, IO and coordination, not extra model inference. Capturing source once can avoid repeated reads, but hashing, retaining candidate/accepted bytes and map construction have costs. Staging evaluation can temporarily retain additional fact/program state; quantify that peak before adopting whole-store copies. A quiescent boundary can increase query/mutation latency, while retries under rapid edits can duplicate compilation. Measure successful and rejected reload latency, preparation/evaluation time, bytes read, allocations, peak retained memory, pause duration, repeated-conflict recovery and event-delivery behavior. No performance or reliability gain is asserted without these measurements.

## Relationship to existing work and exit condition

Root/core/system guidance and the current audit inventory were read. The existing [Mangle schema rollback audit](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/mangle-schema-rollback.md) concerns a rejected fragment in `internal/mangle.Engine`; it is not this new multi-shard RealKernel/Cortex adoption route. The [self-restart failure audit on the research branch](https://github.com/theRebelliousNerd/codenerd/blob/3cb059160df86e0af6e6fd64696955d796077df5/Docs/audits/self-restart-failure-recovery.md) concerns later executable rebuilds after a failed exec attempt, not policy hot reload. Indexed `ReloadConstitution audit` search returned no matches. This is bounded duplicate screening, not proof that no differently worded or unindexed discussion exists.

**Exit condition:** every reload has an identified, coherent disposition for programs, evaluation readiness and routing state; failures before adoption preserve the prior generation; later failures cannot be presented as unchanged without demonstrated restoration; and controlled late-failure/interleaving fixtures establish recovery through the production adapter. Existing malformed-input rejection remains protected. Neither this audit nor source-only review marks those checks passed.


## As built

Rechecked every failure boundary against worktree baseline
`8d373fb6ca05ef03fec59b923c5f1925066d623a` and the authored changes. The historical
analysis above remains a description of the reviewed baseline. Its main findings
were real; none was dismissed as wrong or already repaired. Opt-in admission,
foreign-workspace exclusion, compilation/stratification protection and fail-closed
evaluation reads were already present and are retained.

| Finding | Ruling and resulting contract |
|---|---|
| Single-kernel install precedes fallible evaluation | **Real, repaired.** `internal/core/constitution_reload.go` compiles, validates retained facts and evaluates a private candidate before transferring any live program/evaluation state. Every returned error rejects before publication. |
| Late shard conflict, evaluation failure or skipped specialization can leave partial adoption | **Real, repaired.** Cortex holds an adoption write barrier, its routing mutex and every live kernel mutex in domain order. All candidates must be ready; a specialized shard rejects the whole group. Program suffixes, learned rules, limits and facts remain locked through staging/publication, so there is no late compare-and-swap window to compensate. |
| Shards independently reread mutable source | **Real, repaired.** One candidate capture is reused by every shard. Two matching assembled reads are required; a mismatch/read error rejects. The hash identifies those exact bytes. This is an explicit capture rule, not filesystem atomicity or an editor-transaction guarantee. |
| Routing is rebuilt from a later source generation; a failure leaves the old map | **Real, repaired.** Routing is prepared from staged program bytes before publication, using the installed ownership and consumer routes. `internal/system/dev_policy.go` performs no later corpus read or routing assignment. Common programs include their extensions and learned rules in analysis; distinct programs explicitly install nil-map broadcast/fan-out routing rather than pretending their union is a common program. |
| A new consumer may lack already-held shared facts | **Real inventory gap, repaired.** Candidates replace shared replicas from the catch-all's canonical EDB before evaluation, hydrating newly consuming shards without promoting derived facts or resurrecting stale replicas. |
| Reload errors always claim the running policy is unchanged | **Real, repaired.** `ConstitutionReload` and `PolicyReloadEvent` carry active/candidate corpus identities, stage and rejected/adopted disposition. The chat in `cmd/nerd/chat/self_restart.go` says unchanged only for an explicit rejected disposition. No fallible work follows publication. |

`internal/core/constitution_source.go` retains corpus assembly and adds root-module
attribution markers for routing analysis. The adoption barrier spans route
selection and use in `internal/core/cortex_kernel.go`, inspection in
`internal/core/cortex_kernel_inspect.go` and `internal/core/kernel_explanation.go`,
and routed inclusion checks in `internal/core/kernel_inclusion.go`. Direct kernel
writes serialize on the kernel mutexes; Cortex transactions hold the adoption
read barrier for their routed commit.

Regression definitions in `internal/core/constitution_adoption_test.go` cover
single and late-shard evaluation-ceiling rejection, whole-group specialization
rejection, repeated rejection then recovery, existing-fact hydration, stale-replica
retirement, source instability, no source reread after capture, concurrent routed
queries/fact writes/direct program writes, preserved extensions and conservative
routing for different programs. They reuse existing policy fixtures.
`internal/core/constitution_reload_test.go` retains parse/stratification rejection.
`internal/system/dev_policy_test.go` extends
`TestBootDevMode_HotReloadsThePolicyFromTheSourceTree` through the real watcher and
adapter with generation reporting, evaluation rejection, readable prior state and
later recovery. `cmd/nerd/chat/self_restart_test.go` adds
`TestPolicyReload_OnlyARejectedGenerationClaimsUnchanged`.

The reported failure in
`TestCortexReload_HydratesAlreadyPresentSharedFactsForNewConsumers` exposed a
fixture error: its shards owned no predicates and its probe read only shared
`user_intent`. `internal/core/derivation_map.go` serves that uniform derivation
from the catch-all, so the other shards were not consumers. Production hydration
correctly follows that map. The corrected fixture registers world ownership of
`file_topology`, matching `internal/shards/registration.go`, asserts a file there,
and checks that rebuilt routing requires the world to consume the retained
intent. It verifies the shipped `context_priority` join through a Cortex query,
keeps an unrelated shard empty, and seeds a stale nonconsumer replica to verify
its removal on reload. A separate shared-only regression pins uniform-result
routing through the catch-all. These corrections change Go test setup and
assertions; production hydration and Mangle source remain unchanged. Execution
of the corrected fixtures remains pending under the author-only constraint.

Fail-before evidence is **source-based**, not executed: the baseline installs
before `Evaluate`, swaps earlier shards before a later rejection, captures once
per shard, never hydrates newly consuming shards and unconditionally displays
unchanged. Those branches contradict the corresponding new assertions. The
finite evaluation-limit witness reuses the existing engine limit seam; its
actual failure stage and recovery still require execution on the pinned engine.

The required contracts and current implementation are recorded in
`Docs/architecture/core/INTERNALS.md` and
`Docs/architecture/system/IMPLEMENTED_SPEC.md`. No production `.mg` files,
registration, baseline inventories or prompt atoms changed. Formatting and diff
inspection were performed; compilation, tests, race checks, benchmarks and
commits were not run under the author-only task constraint. Behavioral acceptance
and measurement of candidate-memory/pause costs remain pending. The repository's
private-terms scanner was unavailable in the permitted tree and PATH; its check
remains pending as well. Implementation status does not imply executed acceptance.
