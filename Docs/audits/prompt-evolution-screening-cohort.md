---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: implemented
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
supersedes: []
---

# Prompt evolution: preserve the screened evidence cohort and account for screening purchases

## Recommendation

Give the existing Jev shared-cause screen an explicit evidence-cohort and purchase identity. The screen, atom generator, generation claim and consumed-evidence ledger currently operate over different projections of one failure group. For a straightforward fifteen-record group, the screen can see the oldest twelve detailed failures while the generator sees only the newest three. A favorable reading of the former therefore need not establish the property required of the latter.

The same boundary affects cost. A negative or unsettled screen leaves the evidence unconsumed and does not decrement the generation-call budget. The next cycle can purchase the same screen again. Jev screening also bypasses the adaptation ledger's metered text client; the ordinary automatic-maintenance context does not carry the usage tracker required by TypeSafe's reporting path.

This is a targeted integration refinement, not a new learning architecture. Preserve shared-boot wiring, quarantine, exact-version trial admission, generation fencing, bounded recording/drain, and idempotent attribution. Do not reopen the already documented served-candidate lineage issue as a new finding.

**Evidence boundary:** read-only static inspection, with main rechecked at 05:44 UTC on 2026-10-30 and still at the pinned revision above. No repository program, test, model inference, profiler or experiment was run. All fixtures and improvements below are proposed, not reproduced outcomes. The relevant collector, generator, cycle, Jev screening, classifier/strategy and TypeSafe usage files were compared individually with research tips `21808fc00ee04762982a056e4e53a98a131d0bae` and `3e433d2d455c48917b6d4326dcd4b1ab105cd243` and matched. The mission-driven proposal on the former research tip matches main; it was not present at the latter tip.

## Related primary research

[Dwork and colleagues study adaptive analysis and holdout reuse](https://arxiv.org/abs/1506.02629), illustrating why repeatedly selecting on the same data requires care. Their results motivate separating development, validation and held-out evidence here; they do not establish a formal guarantee for this proposed cohort contract.

## Existing contracts and scope

The [prompt-evolution lifecycle audit][lifecycle] records the implemented isolation, claim, queue and ledger protections and separately analyzes served-candidate provenance. The [mission-driven improvement proposal][mission] calls for controlled learning with full costs rather than uncontrolled self-editing. The [Jev judgments document][jev-doc] explains the newer shared-cause observation: agent mistakes must still meet the minimum count; a positive screen permits generation, negative or unsettled observations leave the evidence available for later failures, and unavailable judgment falls back to the pre-existing count rule.

These are useful protections. The screen's request is bounded to twelve failures and 2,000 bytes per text field, with omissions disclosed. The ordinary evolution cycle has a global generation/atom budget and the automatic entry has a two-minute context deadline. Automatic evolution remains opt-in; Jev also requires a configured client and enabled atom checks. This audit does not imply that every session purchases screens or that the screen itself promotes a candidate. [Request bounds][screen-request], [cycle budget][cycle-budget], [automatic entry][automatic]

The new issue is **pre-generation evidence identity**: which failure examples justified buying the candidate, which examples the generator actually received, and which observations that purchase retires. Source identity at this stage is distinct from the later question of which candidate version was actually served to a task. The automatic entry's deadline is explicitly defined by the [two-minute timeout constant][automatic-timeout].

## The inspected end-to-end path

| Boundary | Current behavior | Consequence |
|---|---|---|
| Failure retrieval | `GetFailuresByProblemType` reads unconsumed FAIL records and sorts by problem/shard/provider/model, then descending creation time; group construction appends in that order. [Collector][collector] | For records with identical raw grouping fields and distinct timestamps, each group is newest first. |
| Eligibility preparation | `agentMistakes` excludes known non-agent mistakes. `generateForGroup` builds verdict and evidence lists from qualifying records and checks the minimum count. [Group preparation][group-preparation] | The witness must use eligible agent mistakes, not provider outages or missing verdict versions. |
| Shared-cause screen | `SharedCauseRequest` retains the last twelve records when the list is longer. [Request projection][screen-request] | In the stated witness, Jev sees the oldest twelve detailed failures, not the newest twelve. |
| Generator context | `buildMetaPrompt` groups the original verdict list by error category and includes the first three detailed examples per category, plus an omitted count. [Generator projection][generator-projection] | With one error category, the generator sees the newest three detailed failures. Category-map ordering does not affect this one-category witness. |
| Purchase identity | The generation claim key and subsequent call use the full qualifying evidence/verdict lists. [Claim and call][claim-call] | Screening and rendering projections are not represented as distinct, checkable cohort identities. |
| Provenance and retirement | Generated atoms list all supplied source task IDs. Successful publication completes the claim and consumes all evidence references, then persists candidates. A storage failure returns the claim's evidence to the pool. [Source IDs][source-ids], [publication][publication], [ledger transaction][ledger-complete] | Normal successful retirement is broader than either detailed model-visible projection. This is not evidence that all source records were literally disclosed to both models. |

The generator does disclose category counts and omitted examples; the screen likewise discloses an omitted count. The problem is not concealed truncation. An omission count does not establish that omitted examples share the observed cause or that two different projections concern the same actionable pattern.

## Source-derived witness: positive screening of a different cohort

Construct fifteen unconsumed records with:

- one identical problem type, shard, raw provider and raw model;
- one error category, nonempty verdict versions and valid serving provenance;
- distinct timestamps, each record classified as an agent mistake;
- the configured minimum count met and positive remaining cycle budgets;
- Jev configured with atom checks enabled.

Let the oldest twelve records describe recurring cause A. Let the newest three describe different causes B, C and D. Use unique markers in their task/explanation text.

The screen receives A's twelve detailed examples. A scripted positive result for that actual payload allows the cycle to continue. The generator receives detailed B/C/D examples and the aggregate category count, not the screened A examples. If generation and admission produce a valid candidate and publication succeeds, the claim consumes all fifteen task/verdict identities and the generated candidate lists all fifteen task IDs.

This establishes an authorization-of-purchase mismatch: the positive one-cause observation concerns a different detailed cohort than the one used to write guidance. It does **not** establish that a real Jev answer will be positive, that a real generator will write a bad atom, or that any generated atom will reach production. Quarantine and subsequent trial/promotion boundaries still apply.

The counterexample uses fifteen records to make the two detailed sets wholly disjoint. Thirteen or fourteen can already produce nonidentical sets, but partial overlap makes a less clean witness. A group of three is a useful control: no projection omits its examples. Twelve records are another control for the screen cap, although the generator still retains only three examples in a single category.

## Source-derived witness: new evidence can fail to reach the screen

Reverse the arrangement: the oldest twelve describe mixed causes, and the newest three describe one coherent new cause. A scripted negative or unsettled screen returns before claim acquisition or generation. No failure evidence is consumed.

Append more newer qualifying records for that new cause. While the old records remain in the unconsumed group, the last-twelve projection still selects the same oldest twelve. The new records change the full group and omitted count but do not enter the detailed screening examples. The intended opportunity for later failures to change the shared-cause assessment is therefore not guaranteed by leaving evidence unconsumed alone.

This is not a universal starvation claim: verdict revision, retention/removal, changed grouping or another explicit intervention can alter the window. It is a stable retained-backlog witness showing that adding newer relevant evidence need not change the screen's detailed evidence.

## Screening purchase and cost identity

The outer loop checks that generation-call and atom budgets are positive **before** calling `generateForGroup`. The shared-cause call happens after that check but **before** claim acquisition and before decrementing `generationCalls`. A negative or unsettled result returns at that point. Consequently, multiple rejected groups can all purchase screening while the remaining generation budget stays unchanged, until the cycle's context ends or actual generation consumes the available budget. This is not an absent wall-clock bound or a claim that generation exceeds its own counter. [Budget check][cycle-budget], [screen/claim/decrement ordering][claim-call]

A concrete unrun accounting fixture uses three already-judged qualifying groups, positive generation/atom limits of one, and a scripted negative screen for each. All three screens are reachable while the result's generation calls remain zero. `MaxCycleJudgeCalls` bounds the records sent through the task-verdict stage; it does not cap screening of this already-judged backlog. If the groups stay unchanged, a later eligible cycle can buy the same screening again. The existing generation claims correctly prevent duplicate completed generation; they do not cover a screen occurring before a claim. [Verdict-stage bound][judge-stage], [screen behavior][screen-decision]

There are also two missing accounting handoffs:

1. Screening calls `learnjudge.JudgeSharedCause`, which invokes the separate Jev `Asker.Ask` path and labels the operation as task verdict work. It does not pass through the adaptation ledger's `meteredLLM` wrapper. That ledger currently defines judge, generate and refine categories. [Jev call][screen-call], [ledger categories and wrapper][spend-ledger]
2. TypeSafe reports answered-call usage through the tracker/observer in its context. Fresh shared boot starts maintenance from `context.Background()`; maintenance forwards its derived context into evolution without adding the Cortex's usage tracker. The ordinary automatic screening path therefore lacks that accounting destination too. This conclusion is specific to that inspected path; an external caller that explicitly supplies a tracked context can record Jev usage. [Maintenance startup][maintenance-start], [maintenance forwarding][maintenance-forward], [TypeSafe usage][typesafe-usage], [context delivery][usage-context]

The existing request timeout, cycle deadline, and configured enablement remain real controls. Unavailable token counts must remain unknown; do not infer that an absent receipt means a free call. Provider retries and answered logical operations also need separate treatment when defining a call budget.

## Proposed bounded repair

The following names describe contracts, not existing APIs.

1. **Choose a bounded evidence cohort once.** Preserve task ID, verdict version, grouping identity, ordering/tie-break rule, selection-policy version and omitted identities or bounded references. Make selection explainable and age/fairness aware. Do not solve this by sending an unbounded backlog to every model.
2. **Bind screening to the actual generation cohort.** Reuse the exact selected records, or require an explicit checked relationship when the generator receives a smaller projection. A positive result about A must not authorize generating from disjoint B. Include field-level disclosure/omission identity where text bounds matter.
3. **Make retirement match purchased work.** Claims, candidate provenance and consumed evidence must distinguish selected/disclosed evidence from unselected backlog. Preserve deliberate summarized or aggregate consumption only with an explicit contract that does not imply unseen examples were inspected. Unselected records remain eligible or receive a visible deferral disposition.
4. **Give screening its own versioned purchase key.** Key an observation by cohort identity, screening question/version, actual model identity or conservative model-version policy, and relevant thresholds/configuration. Preserve negative and unsettled observations without marking their failure evidence permanently consumed. A changed cohort or explicit versioned reanalysis can justify a new purchase.
5. **Budget all adaptation stages.** Add an explicit screening-work allowance or an overall model-work budget alongside existing judge/generation limits. Define logical calls, transport retries, tokens/bytes and elapsed time separately. The kernel remains the authority for admission and work selection; Go carries the typed observation and executes the admitted work rather than inventing an independent executive policy.
6. **Attach owned accounting context.** Automatic background calls need lifetime cancellation plus the appropriate usage tracker and operation/cycle/cohort identity. Record screening spend and reuse under a distinct category, avoiding duplicate charging when the same provider report reaches both aggregate usage and adaptation reporting. Record failure/cancellation and unknown usage honestly.
7. **Keep unavailable distinct from negative.** Preserve the current documented fallback semantics unless separately reviewed. A cache or budget miss must not silently become a negative judgment, successful generation, or permanent consumption. Deferred, unavailable, negative and unsettled are different outcomes.

Do not retrofit this by weakening quarantine, promoting on a screen alone, removing generation fences, or treating aggregate task success as causal evidence for a candidate.

## Proposed evaluation — not executed

Begin with scripted clients and real collector, request-builder, generator, ledger and factory adapters; no live inference is needed to establish these identities.

| Fixture | Required observation |
|---|---|
| Three same-category failures | Both stages receive the same detailed cohort; one correctly identified purchase. |
| Twelve, thirteen, fourteen and fifteen same-category failures | Explicit screened/rendered identities reveal every omission; the fifteen-record disjoint witness is prevented. |
| Old coherent A, newest unrelated B/C/D | A's positive screen cannot authorize disjoint B/C/D generation. |
| Old mixed causes, appended coherent new failures | Selection policy can expose the new cohort; unchanged observations are reused only for their exact cohort. |
| Multiple categories and tied timestamps | Stable selection; per-category projection cannot silently change purchase identity. |
| Repeated unchanged negative or unsettled groups | No new screening purchase absent authorized/versioned reanalysis; evidence is not permanently consumed. |
| Three rejected pre-judged groups with one unit of screening allowance | Remaining groups defer visibly; generation and screening counters are separate. |
| Revised verdict, changed group/pin, changed question/model/threshold | Reuse invalidates at the declared boundary; old observation remains attributable. |
| Jev disabled, failed, canceled or unavailable | Existing fallback/disposition is explicit; no false negative or zero-cost assertion. |
| Generation/admission/storage failure and retry | Claim fencing, evidence eligibility and spend remain consistent; no duplicate successful retirement. |
| Automatic maintenance with scripted TypeSafe response usage | Aggregate tracker and adaptation receipt contain the same observed usage identity exactly once per reporting view; cancellation stays owned. |
| No Jev / no qualifying failures | Existing low-overhead behavior is preserved. |

Inspect existing tests as controls, not as proof these sequences pass. `TestRunEvolutionCycle_GlobalBudgetOldestFirst` exercises the generator budget with a counting text client; `SharedCauseRequest` tests cover the twelve-record cap and omission count. Neither establishes end-to-end cohort equality or screening purchase accounting. [Existing lifecycle fixture][existing-budget-test], [existing screening fixture][existing-screen-test]

After deterministic identity checks, a separately authorized held-out study can compare current selection with the revised bounded cohort policy at fixed task families, models, tools, verification requirements and total resource budget. Report candidate usefulness and regressions with uncertainty, not just fewer screens or a higher generation acceptance rate. Keep negative, failed, deferred and unconsumed cases in the denominators.

## Full costs and checkable exit

Measure screening, task-verdict judging, candidate generation, admission checks, refinement, trials and independent verification as separate stages. Include successful, failed, retried, canceled, discarded and repeated work. Report provider token categories when available, call/attempt counts, bytes, latency, CPU, database reads/writes, retained artifact growth, migration and review time. Do not add incompatible resource units or treat unpriced/unknown usage as zero.

For G unchanged qualifying groups screened during each of C completed cycles, the current negative/unsettled path can purchase up to C×G logical screens, conditional on deadlines and budgets allowing all groups to be reached. This is a structural conditional count, not measured production spend. A cohort-keyed observation cache targets one purchase per distinct authorized cohort/screener version, plus explicitly accounted retries/reanalysis. New evidence, policy changes and model changes legitimately add purchases; therefore no universal savings ratio follows.

The repair costs include cohort selection and hashing, an observation/receipt schema, indexes and bounded retention, migration/compatibility handling, context plumbing, additional diagnostics and fixtures. Selection fairness has a quality/cost tradeoff: always choosing newest evidence may starve an older important problem; always choosing oldest recreated the demonstrated blind spot. Declare and evaluate the policy rather than hiding it in slice direction.

**Exit:** each generation purchase can name the exact evidence cohort whose shared-cause observation authorized it; claimed, disclosed and consumed identities have an explicit consistent relationship; adding new evidence can reach selection under the declared policy; repeated unchanged screens are intentional and accounted; and automatic-path Jev spend reaches a durable accounting destination or is explicitly reported incomplete. No assertion of improved task accuracy or reduced total cost is warranted until evaluated.

[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/innovations/mission-driven-forever-improvement.md
[jev-doc]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/architecture/jev-turn-judgments.md#L680-L700
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/feedback_collector.go#L532-L571
[group-preparation]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L183-L200
[screen-request]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/learnjudge/sharedcause.go#L28-L62
[generator-projection]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/atom_generator.go#L154-L181
[claim-call]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L174-L227
[source-ids]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/atom_generator.go#L210-L234
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L259-L304
[ledger-complete]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L378-L422
[cycle-budget]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L62-L92
[automatic]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory_learning.go#L378-L392
[automatic-timeout]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory_learning.go#L78-L82
[judge-stage]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L126-L149
[screen-decision]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/jev_evolution.go#L43-L76
[screen-call]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/learnjudge/sharedcause.go#L65-L79
[spend-ledger]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L454-L625
[maintenance-start]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory.go#L363-L367
[maintenance-forward]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory.go#L778-L808
[typesafe-usage]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/typesafe/usage.go#L35-L54
[usage-context]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/usage/usage_tracker.go#L1036-L1051
[existing-budget-test]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/lifecycle_evidence_test.go#L201-L237
[existing-screen-test]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/learnjudge/evolution_test.go#L40-L49


## As built

Source inspection confirmed all three behavioral witnesses and both accounting
handoff findings against the working tree. None was already fixed or ruled wrong.
The existing quarantine, exact-version trial, generation fencing, count fallback,
request timeout and automatic-cycle deadline controls were also confirmed; they
were preserved. The original source-derived examples above describe the audited
implementation, not executed experiments.

| Finding | Ruling and authored repair | Production-path regression |
|---|---|---|
| Screening and generation disclose different cohorts | **Real.** `internal/autopoiesis/prompt_evolution/evidence_cohort.go` selects once. `atom_generator.go` renders the same bounded JSON state that `screening_ledger.go` sends to Jev. | `TestRunEvolutionCycle_ScreenedCohortIsGeneratedAndRetired`, for 3/12/13/14/15 failures. Before the repair the 15-record path screens the oldest twelve but renders only the newest three explanations; the test requires every screened explanation, request and rule in generation. |
| Appended failures need not reach the screen | **Real.** Deterministic alternating newest/oldest ranks expose both ends; Mangle bounds cohort membership. Ties compare task ID and verdict version. Interior fairness under a continuously growing rejected backlog is not claimed. | `TestRunEvolutionCycle_ScreenBudgetCacheAndNewEvidence` and `TestRunEvolutionCycle_CohortTieOrderAndVerdictRevision`. The old tail window excludes appended newest records. |
| Provenance, claims and retirement cover unseen backlog | **Real.** `evolver_cycle.go` claims and consumes only selected task/verdict references. Cohort/claim/screen linkage is durable; `types.go` and `evolver_candidates.go` preserve cohort and observation IDs on candidates and reload. Execution task identity overrides an absent verdict TaskID for provenance. | `TestRunEvolutionCycle_ScreenedCohortIsGeneratedAndRetired` asserts that omitted records remain eligible. `TestRunEvolutionCycle_CohortProjectionIsBoundedAndSurvivesRestart` pins disclosure and persistence. The old path consumes all fifteen. |
| Rejected groups repeatedly purchase screening outside the generation counter | **Real.** Separate configurable logical screening allowance, kernel admission, leased purchase keys and durable exact-key reuse in `screening_ledger.go`. Positive, negative, unsettled and unavailable observations retain distinct data. Reattaching Jev is an explicit conservative reanalysis boundary. | `TestRunEvolutionCycle_ScreenBudgetCacheAndNewEvidence`: three rejected groups, allowance one, one new purchase per cycle, then three cache hits and no new purchase. Previously every cycle can buy all three. |
| Screening bypasses adaptation accounting and uses the task-verdict operation | **Real.** Separate `SpendScreen` category, `jev_shared_cause` operation, cycle/cohort receipts and a usage observer. The receipt captures provider reports without charging the aggregate tracker again. | `TestRunEvolutionCycle_ScreenReceiptAndRetry` and `TestRunEvolutionCycle_UnavailableScreenIsNotNegativeOrFree`. The old ledger has no screening row. |
| Automatic evolution lacks its owned aggregate usage context | **Real.** `internal/system/factory_learning.go` attaches `Cortex.UsageTracker` to the existing cancellable cycle context. `internal/typesafe/client.go` also preserves unknown failed-attempt usage across an answered retry. | `TestAutomaticEvolutionScreeningOwnsUsageContext` uses the actual maintenance adapter and TypeSafe client with scripted HTTP; the old path records no aggregate screening tokens. `TestAsk_RetriedAnswerPreservesUnknownAttemptUsage` catches a successful answer concealing unknown earlier attempt usage. |

The Mangle contract lives in `internal/core/defaults/schemas_evolution.mg` and
`internal/core/defaults/policy/prompt_evolution.mg`. The adaptation component
kernel loads those same sources through `internal/core/defaults/evolution_policy.go`;
`internal/core/kernel_types.go` includes the schema in the action kernel, and
`internal/shards/registration.go` homes every new predicate in the prompt shard.
Go measures ranks and logical calls, executes derived admission, and persists
observations. It has no independent fallback for failed policy evaluation.
`TestPromptEvolutionKernelCohortAndScreenAdmission` and
`TestPromptEvolutionPolicyFiresOnTheShardedKernel` cover the shared corpus and
shard joins. Existing testdata baselines were not enlarged.

Configuration is wired through `internal/config/prompt_evolution.go` and
`EvolverConfigFrom`: `evidence_cohort_size` defaults to 12 and
`max_cycle_screening_calls` defaults to 3. Cohort size must cover the configured
minimum count and fit the twelve-record request protocol. There are no new model
names or prompt atoms. The current contract and remaining design gaps are in
`Docs/architecture/jev-turn-judgments.md`, “Atom checks”.

**Why partial:** this task explicitly forbids compilation and every test command.
The regression assertions and fail-before explanations above come from comparing
the old production flow with the authored tests; no failing-before or passing-after
run is claimed. A disposable `_mangle_lab/screening_cohort/` fixture is authored,
with its verdict pending. The required pinned-engine lab, owning-package tests,
full defaults checks and shard checks must run before accepting the policy design
or calling the behavior verified. No engine-backed CLI was run because it can
build its interpreter. Screening receipts deliberately leave transport attempt
counts and raw response bytes unknown. Storage follows existing append-only
ledger retention; a bounded historical-retention policy and automatic detection
of alias changes within one attachment remain outside this authored repair.
No cost-saving or task-quality outcome is claimed.
