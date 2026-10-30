---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: proposed-refinement
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
scope: consumed task and verdict identity across screening snapshots and generation purchase
---

# Consumed failure evidence needs transactional generation admission

## Decision and scope

Revalidate and reserve the selected failure evidence transactionally when admitting a generation purchase. A screening observation can remain valid while the evidence it describes has already been consumed by another process. Keep conservative rescreening on a new Jev attachment; do not let the resulting distinct screening identity implicitly authorize another generation from a stale pre-consumption snapshot.

The current path filters consumed task/verdict identities only when it first reads failure groups. It then screens a saved cohort, constructs a generation key that includes the screening identity, and claims that key without checking the evidence's current consumption state. Two normal Jev attachments have different epochs and therefore different claim keys. One worker can finish and consume the evidence while another is screening the older snapshot; the second worker can subsequently buy generation and complete its own claim. Completion uses `INSERT OR REPLACE`, replacing the first consuming claim recorded for those same task/verdict identities. [Initial exclusion][collector], [screen and claim handoff][handoff], [claim and completion][ledger]

This is a generation-admission and retirement-ownership residual, not an assertion that two screening purchases are forbidden. The established consequences are a duplicate same-evidence generation purchase and, conditional on successful candidate generation/admission/storage, a second publication with replaced consumption ownership. No measured frequency, model-quality regression, monetary loss or unsafe execution is claimed.

Public main was re-resolved at approximately 09:42 UTC on 2026-10-30 and remained [`388205138ba6d47c1cff0c5e97c896dfa642a391`][baseline]. Independent source challenge corroborated the stale-snapshot path. This is static source analysis: no application, repository program, test, database experiment, model call or benchmark was executed, and no implementation was modified. The schedules and acceptance checks below are proposed and unrun.

## Existing work and the boundary it leaves

The [prompt-evolution lifecycle audit][lifecycle] establishes consumed-version accounting, generation claims, per-cycle global spending bounds and lifecycle ownership. The [screening-cohort audit][screen-audit] and its repair correctly align the screened/generated cohort, preserve omitted evidence, bound screening separately and retain durable screening receipts. This report concerns the subsequent concurrency boundary between that immutable cohort and the current durable consumption state.

The latest [strategy refinement publication audit][strategy-audit] is different. Its worker loses an existing claim after a configured short lease, correctly discards atoms, and then continues into strategy refinement. Here neither worker loses its claim: the claim keys differ. No lease expiry, strategy row, refinement threshold or role-matching workaround is required. The default 600-second claim lease can remain unchanged, and the complete witness can fit inside the normal 120-second cycle deadline. [Lease defaults][config], [automatic deadline][deadline], [manual deadline][manual]

The proposal does not reopen shared-cause cohort fairness, source/version exposure attribution, strategy candidate filtering or cross-process strategy-cache coherence. Nor does it remove the screen identity to make unrelated observations appear equivalent. A bounded inventory/source review found no existing report owning this exact stale-snapshot-to-new-claim boundary; that is not an exhaustive claim about every historical note.

## Two identities with different responsibilities

### Screening identity is deliberately attachment specific

`newJevUse` assigns a new `screenEpoch` whenever Jev is attached. The screening purchase hash includes that epoch, the configured model, thresholds, questions, procedure version and cohort identity. The documented reason is conservative treatment of moving model aliases: reattachment, including at restart, permits a new observation rather than trusting an earlier attachment's answer. This is intentional behavior to retain. [Attachment construction][attachment], [screening key][screen-key], [documented rescreening policy][screen-contract]

An affirmative screen is evidence about the selected cohort's shared cause. The Mangle policy derives `evolution_generation_allowed` from the observation and threshold; it is not a transactional reservation of the task/verdict rows. Neither the component policy adapter nor that rule reads the durable consumed-evidence table. [Policy adapter][policy-adapter], [screening policy][policy]

### Consumption identity is independent of that attachment

`consumed_evidence` has primary key `(task_id, verdict_version)` and stores a consuming `claim_key`. It is not keyed by screening epoch. `GetFailuresByProblemType` excludes any matching consumed task/verdict pair, and its contract explicitly describes such a failure as unavailable to buy the next generation. A revised verdict is a new evidence version. [Schema][schema], [collector contract and query][collector]

Consequently, a new attachment does not normally make already-consumed failures eligible again: a cycle that starts its group read after consumption sees none of those records. The problematic worker differs only because it retained a group snapshot obtained before consumption. Permitting conservative rescreening is therefore not, by itself, a reason to accept this timing-dependent regeneration.

The generation-key helper separately documents deliberate generator-version reanalysis. If the project wants explicit generation reanalysis of consumed evidence, that authority and its evidence domain should be specified and recorded directly. It should not depend on which concurrent worker happened to read first. This report does not prescribe a ban on deliberate reanalysis. [Generation identity contract][generation-key]

## Production handoff and missing check

The factory creates each evolver under the workspace's `.nerd` directory, attaches its configured Jev client and uses the existing recording/compilation integration. Evolver mutexes serialize one instance, while durable claims are the documented cross-process mechanism. Two processes on one workspace therefore share the evidence database but have distinct attachment epochs and mutexes. Automatic cycles remain opt-in; an operator can also explicitly invoke a cycle. [Factory][factory], [lock contract][locks], [automatic admission][automatic]

The relevant sequence is:

1. `RunEvolutionCycle` reads unconsumed failures once, orders the resulting groups and creates a shared cycle budget. [Cycle snapshot][cycle]
2. `generateForGroup` validates the serving pin, selects a deterministic bounded cohort, checks the minimum count, saves the cohort and obtains screening permission. [Cohort and screen][handoff]
3. Its generation key is derived from group identity, `atomGeneratorVersion`, cohort ID, screening ID and selected task/verdict references. Distinct attachment-specific screen IDs make distinct generation keys even when the selected evidence, generator and group are otherwise unchanged. [Claim-key composition][handoff]
4. `EvidenceLedger.Claim` transactionally inserts or takes over the claim row for that key. It receives only an evidence count, not the selected references; it cannot inspect their consumption status through this API. The transaction checks this claim's status/lease, not whether another key already consumed the evidence. [Claim implementation][ledger]
5. After successful claim admission, the caller decrements the shared generation-call allowance and invokes the generator on the saved cohort. There is no intervening fresh evidence read. [Purchase][handoff]
6. `Complete` checks ownership of this claim, then replaces consumption rows for its references. Ownership of a distinct claim is sufficient; an existing consumption row pointing at another successful claim does not reject completion. Atom storage follows successful completion. [Completion][ledger], [publication][publication]

The existing owner fence is not broken for equal keys. A completed claim cannot be reacquired under the same key, and a worker whose owner token was replaced is rejected. The problem is that those checks do not arbitrate two different keys that consume the same evidence.

## Concrete source derived schedule

Use two initialized instances A and B on the same `.nerd` directory. Give them unchanged, already-judged eligible failures: at least the configured minimum, with stable task IDs/verdict versions, the same group and a valid serving pin. Three eligible failures suffice for the default minimum; no cohort truncation is needed. Both instances have Jev atom checks enabled with the same configured model and thresholds but their naturally distinct attachment epochs. The cycle budgets are positive and both contexts remain live.

For a controlled future fixture, use positive screening replies and healthy storage. If asserting second atom publication as well as the duplicate purchase, supply distinct, valid candidate IDs that pass the actual admission path; do not introduce a candidate-ID collision or assume arbitrary generated output is accepted.

1. B reads the unconsumed failures, selects cohort C and begins its allowed screen under screening key SB. Hold that response.
2. A reads the same still-unconsumed failures and selects the same cohort C. Its attachment creates a different key SA. Let A's screen permit generation.
3. A acquires generation claim KA, generates, completes and publishes successfully. The selected `(task_id, verdict_version)` pairs now point to KA in `consumed_evidence`.
4. Let B's still-live screen return a permitted result. Its screening receipt is legitimate: it describes what B actually screened, and no other worker owned SB.
5. B constructs KB. Because SB differs from SA, KB differs from KA. `Claim(KB, ...)` succeeds as a new claim without inspecting the evidence consumed at step 3.
6. B purchases generation using C. This is the central residual: the purchase starts after those exact evidence versions have already funded A's completed generation.
7. If B's generation and publication succeed, `Complete(KB, ...)` completes B's correctly owned claim and replaces the consumption mappings with KB. A's completed claim and generated candidates are not thereby undone. The database can now contain two successful claims for the same selected evidence, while the one-row-per-evidence mapping retains only the later consuming claim.

No timeout, lease takeover, failed fence, extra failure, changed verdict, changed generator procedure or manual deletion of consumption rows is needed. Both workers stay within their individual screening, generation and atom allowances. This is not proof of an exceeded per-cycle cap or a claim that the system promises one workspace-wide aggregate cycle budget.

Useful negative controls establish the boundary:

- Start B's failure-group read only after A completes. The collector excludes the consumed evidence even with B's new attachment epoch.
- Use the same cohort and stable count-fallback identity with atom checks off. B's later claim has the same key as A's completed claim and is correctly refused.
- Make B's screen negative/unsettled, exhaust its allowance or cancel its context before continuation. Its generation does not follow.
- Give the workers disjoint evidence. Independent valid generations should remain admissible.

## Transactional purchase and retirement contract

### Contain the demonstrated post consumption purchase

Pass the selected evidence references into generation admission and, in the same transaction that admits the claim, reject any reference already consumed outside an explicitly authorized reanalysis domain. An unprotected re-read before `Claim` is insufficient: another worker can consume the evidence between the read and claim insertion.

This atomic consumed-state check closes the exact schedule above. Alone, it does not prevent two distinct keys from both being admitted while their overlapping evidence is still unconsumed. State that limitation rather than calling it complete cross-process evidence ownership.

### Reserve evidence before buying generation

For the stronger single-purchase contract, acquire an owner/fence-bound reservation for every selected task/verdict identity transactionally with the generation claim. Admit only when the required evidence is both unconsumed and available to this operation. Use a defined reanalysis domain if deliberate versioned regeneration is supported; ordinary new screening receipts must not create that domain accidentally.

An all-or-none cohort reservation is the simplest contract to evaluate. If another worker consumed or reserved any member, defer/refuse the stale cohort and let a later bounded cycle select fresh evidence. Do not silently drop occupied members and generate from the remainder under the old positive screen: that would recreate the screening/generation cohort mismatch the existing repair correctly removed. Any changed cohort needs its own identity and the appropriate admission/screening treatment, without a fresh unlimited budget.

A coarser durable group lease is another option, but it serializes nonoverlapping work within that group and must still revalidate the snapshot after acquisition. Compare its simplicity and contention with bounded per-evidence reservation; do not assume one approach is free. Merely making generation keys independent of screening would contain the equal-cohort epoch example but would not settle differently composed overlapping cohorts.

### Retire only evidence owned by this purchase

Completion should atomically verify the current claim/reservation owner and transition its evidence to consumed without replacing another completed purchase's ownership. A conflict must produce an observable rejected/stale publication disposition, not a successful `EvidenceConsumed` increment. A conditional insert/update with checked affected rows can form a last publication barrier, but checking only at completion is too late to prevent a duplicate model purchase.

Release or publication-failure handling must free only reservations/consumption belonging to that failed owner and operation. Preserve the current successful winner and its evidence if a late loser fails. Existing `PublishFailed` deletes consumption by its claim key; any reservation extension must retain equally explicit ownership rather than deleting by task ID alone. [Failure retirement][failure-retirement]

Keep model calls outside database transactions and metadata locks. Lease recovery must fence old owners and retain visible spent-but-discarded work; exactly-once external model execution cannot be promised merely by adding a database transaction. Preserve request cancellation, deadlines, all current allowances, atom admission/quarantine and explicit screening receipt accounting. The database enforcement should faithfully implement the declared evidence-ownership contract, not introduce an unrelated Go policy for judging whether failures share a cause.

## Costs and migration

The selected cohort is already bounded to at most twelve records. Admission would add indexed evidence checks and, for full reservation, a bounded set of reservation writes plus release/completion updates. Measure transaction duration and query plans rather than asserting a latency benefit. The costs include schema/API changes, cross-process contention, stale-owner recovery, crash reconciliation, diagnostics and receipt retention.

Contending workers may legitimately pay for separate screens before one loses generation admission. Keep both valid screening receipts and their costs; refusing duplicate generation does not make the preceding screen free. Moving reservation before screening can reduce duplicate screening work but holds evidence ownership longer across a model call. That tradeoff needs a declared lease/deferral policy and evaluation, not an automatic change to the permitted rescreening semantics.

Legacy `consumed_evidence` rows should remain consumed. Do not drop them or reset all evidence to make a schema migration convenient. Historical duplicate claims may require reconciliation/reporting; the audit does not authorize deleting historical candidates or guessing which generated instruction is better. Record an explicit authority for any deliberate reanalysis of old evidence.

A rolling deployment must account for older writers that still claim without evidence reservation and complete with `INSERT OR REPLACE`. Upgrading only readers cannot establish the new invariant. Use an explicit compatible storage protocol, reject incompatible writers, or coordinate a drained/single-writer migration window. No claim of net token savings or improved task outcomes is justified until measured.

## Fair evaluation and exit criteria

Existing tests are useful controls, not execution evidence for this residual. `TestEvidenceLedger_ClaimFencing` exercises equal-key ownership. `TestRunEvolutionCycle_ScreenBudgetCacheAndNewEvidence` explicitly expects rescreening after reattachment for unconsumed negative/unsettled evidence. The restart fixture checks that a later collector read excludes retired evidence. None of those assertions traverses B's pre-consumption snapshot through a distinct later claim after A completes. [Equal-key test][fence-test], [reattachment test][reattach-test], [restart exclusion test][restart-test]

Proposed checks, all unrun:

| Fixture | Required observation |
|---|---|
| B screen paused; A consumes same cohort; B screen resumes under a different attachment | B's valid screen receipt remains; no second generation purchase from consumed evidence. |
| Same cohort and same claim key | Existing active/completed-key exclusion and owner fencing remain effective. |
| New attachment after an unconsumed negative/unsettled observation | Permitted rescreening remains available within screening limits. |
| New attachment whose group read occurs after successful consumption | No resurrection of consumed task/verdict evidence. |
| Two distinct claims concurrently target overlapping unconsumed evidence | Under the reservation contract, only the authorized nonconflicting purchase proceeds; the other is deferred/refused without silent cohort editing. |
| Disjoint cohorts and groups | Independent work is not unnecessarily serialized by a global mutex. |
| Lease expiry, crash, retry and late completion | Recovery is possible; stale owners cannot replace winner-owned consumption or release its reservation. |
| Negative screen, unavailable screen, cancellation and exhausted allowance | Existing distinct dispositions, count fallback and spend limits remain intact. |
| Model returns no usable candidate or publication storage fails | Follow the declared consumption/release contract; preserve spend and winner ownership. |
| Explicit revised verdict or authorized generator reanalysis | New eligibility follows the declared version/authority contract rather than incidental attachment timing. |
| Restart and mixed-version writers | Consumption/reservation state remains durable; legacy writes cannot silently bypass the new invariant. |

Use two real evolver instances sharing temporary SQLite stores and deterministic barriers around screening/generation. Assert actual screening IDs, claim IDs, selected references, purchase counts, candidate publication, consumed mappings and error dispositions. Do not reduce this to a direct `Complete` unit test: the defect includes a normal stale snapshot buying a new call. Use controlled replies and valid production admission inputs; no timing sleeps or live model quality judgment is necessary to establish ownership behavior.

Measure duplicate generation purchases, successful/blocked claims, evidence ownership replacements, recovery latency, screening work paid before deferral, transaction/lock latency and retained receipt volume. At fixed workloads, report all costs and failed/deferred attempts, not only winners. A later authorized capability study should hold task sets, model/tool access, budgets and acceptance criteria fixed and include downstream repair cost; fewer calls alone does not establish better learned instructions.

**Checkable exit:** permitted screening reanalysis remains intact, but a stale cohort cannot purchase generation after its evidence is consumed; overlapping purchases are arbitrated under a declared transactional contract; completion never silently transfers another successful purchase's evidence; equal-key fencing, bounded spending, cancellation and candidate lifecycle safeguards remain effective.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md#as-built-2026-10-27
[screen-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-screening-cohort.md
[strategy-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b7fe0e7e57df31ccd3dc5b51a4f292bb197c491c/Docs/audits/strategy-refinement-claim-publication.md
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/feedback_collector.go#L527-L571
[attachment]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/jev.go#L31-L62
[screen-key]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/screening_ledger.go#L75-L165
[screen-contract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/jev-turn-judgments.md#L720-L742
[policy-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolution_policy.go#L55-L68
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/prompt_evolution.mg#L14-L23
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L75-L94
[generation-key]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L324-L335
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L338-L403
[failure-retirement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L406-L422
[cycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L34-L93
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L175-L235
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L277-L319
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L112-L173
[locks]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L216-L226
[automatic]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L374-L407
[deadline]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L79-L83
[manual]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_evolution.go#L33-L44
[config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/prompt_evolution.go#L42-L69
[fence-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/lifecycle_evidence_test.go#L239-L269
[reattach-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/screening_cohort_test.go#L164-L202
[restart-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/screening_cohort_test.go#L283-L326
