---
doc-class: deep-dive
subsystem: prompt-evolution-session-campaign
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: source-inspection-only
supersedes: []
---

# Prompt trial exposure limits need admission ownership

**Recommendation:** extend the existing exact-version trial lifecycle with ownership of the remaining exposure allowance before a task can carry the candidate. Today the admission rule reads completed, recorded exposure counts. Parallel campaign tasks can both observe the last remaining allowance, both carry the same approved candidate, and subsequently produce two legitimate exposure records against a maximum of one.

This is a continuation of the [prompt evolution lifecycle audit][prior], not a claim that quarantine or versioned trials are absent. Its scope is the declared unit of **recorded turns carrying a candidate**, not provider requests, tokens, or individual tool rounds. The counterexample uses two distinct turns that both finish with host-derived `/done` and are successfully recorded. It needs neither dropped read-only records nor the earlier candidate-identity defects.

**Status:** source-derived, independently challenged, and not runtime-reproduced. All CodeNERD links below pin `388205138ba6d47c1cff0c5e97c896dfa642a391`. No tests, model calls, implementation, or program execution were performed. The likelihood and realized cost of the schedule are unmeasured.

## Existing contract and the new boundary

The configuration describes `TrialMaxExposures` as the number of recorded turns that may carry the candidate before admission stops. The TUI approval response says the trial permits “at most” that number of exposures. These are stronger operator expectations than merely checking whether yesterday's count was below a threshold. [Configuration contract][config-contract], [TUI approval][trial-ui]

The lifecycle audit already distinguishes eligibility, selection, serving, exposure, and credit; its as-built section defines exposure through recorded turns' included atom lists. It also documents the owned recording queue and current candidate-identity limitations. This proposal preserves those distinctions and adds a missing concurrency contract: unfinished tasks that may become recorded exposures must not independently spend the same final allowance. [Lifecycle contract and implementation][prior], [identity continuation][prior-identity]

The current screening-cohort repair concerns which failure evidence buys generation and how those purchases are metered. It does not reserve trial exposure allowance for serving tasks. This is not a newly introduced regression attributed to that repair.

The current audit inventory and both research branches, `research-main-20261030-0715` and `research-main-20261030-0747`, were screened for overlap. Targeted searches found no existing trial-slot reservation finding. At the inspected heads, the trial policy, evolved-atom serving implementation, and recording implementation are byte-identical across both research branches and the five other active lane/QA branches. This check covers those boundaries, not every behavior on those branches.

## One concrete production schedule

Start with an initialized TUI/Cortex and a valid quarantined candidate **C**, version **V**:

- C has a unique ID, no competing canonical-source collision, valid pins matching both tasks, and stable content/selectors throughout the example.
- The operator approves C/V with maximum exposures **1**, current recorded exposure count **0**, and sufficient unexpired lifetime.
- A normal campaign has two eligible, independent coding tasks **A** and **B**. Adaptive concurrency admits at least two; their declared write sets do not conflict; ordinary task, resource, policy, and cancellation checks permit both.
- C actually survives selection, dependencies, conflicts, required-tool checks, and token fitting in both outgoing task prompts. Eligibility alone is not enough.
- Both tasks eventually end in distinct host-derived `/done` turns. Learning is enabled, the recorder accepts and persists both, and C/V remains known. There is no rejection, replacement, queue overflow, database failure, or additional approval.

A legal ordering is:

| Step | A | B | Recorded exposure count |
|---|---|---|---:|
| 1 | Compiles against the live trial facts and includes C/V | Not yet compiled | 0 |
| 2 | Still executing; no finished-turn record | Compiles against the same live allowance and includes C/V | 0 |
| 3 | Finishes and its exposure is persisted | Already received C/V | 1 |
| 4 | Settled | Finishes and its distinct exposure is persisted | 2 |

The compiler need not run concurrently: A's compile can finish before B's compile begins. Only task execution and completed-record publication need overlap. After step 3, withdrawal of future admission works, but it cannot retract C/V from B's already dispatched prompt. At step 4 the ledger correctly preserves B's actual exposure rather than discarding inconvenient evidence. The broken boundary is admission before the exposure, not the subsequent historical count.

The same mechanism allows overshoot at any final remaining allowance; maximum one makes the witness easiest to inspect. This does not establish an unbounded deployment, a harmful generated instruction, or an overall task-verification bypass.

## Source trace from TUI campaign to recorded exposure

### Operator approval creates an envelope

`ApproveTrial` rejects missing and already-promoted candidates, captures the exact candidate hash, writes expiry and maximum exposure count, reads the existing count, and publishes the live trial set. The candidate remains quarantined. The TUI refreshes JIT state after approval. [Approval implementation][approve], [TUI command][trial-ui]

Production boot installs the trial fact sink, registers the evolved-atom manager with the JIT compiler, and installs the finished-turn recorder on the session executor. These are wired production components. [Learning boot][boot]

### Campaign concurrency can reach the shared serving boundary

The in-chat campaign constructor passes its task executor and campaign policy to the orchestrator. The default configured parallelism is three, although actual scheduling remains subject to adaptive limits and eligibility. [TUI construction][campaign-tui], [default policy][campaign-config]

`runPhase` checks the current limit, obtains a write-set lease, marks the task in progress, and starts `runSingleTask` in a goroutine. Thus distinct write sets need not serialize task lifetimes. `spawnTask` dispatches through the observed task executor inside `RunUntilVerified`. [Scheduling][campaign-schedule], [observed dispatch][campaign-dispatch]

The JIT executor's inline route creates a task-local executor and calls `ProcessWithIntent`; `CloneForTask` shares the kernel/compiler dependencies and deliberately inherits the learning recorder. The witness can use ordinary inline coding tasks and does not require a special unverified shard implementation. [Task clone and execution][task-executor], [shared dependencies and recorder][executor-clone]

### Admission reads a count but does not own a slot

The live-set publisher reads ledger counts and excludes expired or exhausted envelopes. The production sink materializes each envelope together with its current `prompt_trial_exposures` count. [Live-set publication][publish], [fact sink][trial-sink]

The shipped Mangle rule admits the exact candidate version when an envelope exists, the recorded count is below its maximum, and the clock is before expiry. The predicate contains no task identity, pending reservation, or consumed admission receipt. [Trial rule][trial-rule]

The evolved-atom manager reads that admission from the compile's private kernel scope. For an exact matching version, it supplies an optional candidate; it does not decrement the ledger or reserve an exposure. Its cache contract includes the snapshot fingerprint, so a later count change can invalidate future cache reuse. The unchanged count in steps 1 and 2 is a legitimate current snapshot, not a stale-cache accident. [Serving and cache boundary][serving]

### Recording follows execution rather than admission

The executor reports the finished turn and included atom IDs to its recorder. The production recorder translates `/done` to a prefilled PASS and submits it to the owned queue. The asynchronous queue is intentional: learning should not put model-judging latency on every foreground turn. [Finished-turn producer][turn-producer], [production recorder][recorder]

Only after the execution record is persisted does `recordExecution` resolve candidate references and invoke `RecordExposures`. That transaction inserts each distinct event's exposure without a quota check. `afterEvidence` then republishes trial facts. [Recording order][recording], [ledger insertion][exposure-insert], [republishing][after-evidence]

For the stable, unique candidate in the witness, the ledger's current-map lookup returns the same V both times. Distinct task IDs produce two rows. After the first row, the live-set filter correctly stops advertising future eligibility; it does not settle an admission B already used.

## Counterevidence and preserved safeguards

**Quarantine still works.** Without an approved exact-version envelope, the candidate is not admitted. A trial copy is optional and grants no new tool or execution permission. Context matching, model/role pins, capability requirements, selection, conflicts, and fitting remain necessary. Both actual inclusions are explicit preconditions, not promised model outcomes. [Serving controls][serving]

**Expiry and exhaustion still work at their observation boundary.** Missing clock or exposure facts fail closed. After a completed exposure is published, a newly created compile scope sees the exhausted trial, and the source fingerprint prevents an old compiled cache entry from simply ignoring the changed state. The proposed repair must retain this behavior.

**Existing tests establish useful sequential behavior.** `TestPromptTrialAdmission_KernelDecidesAndExposureBudgetEndsIt` approves a maximum-one envelope, reads the derived admission through a real compilation scope and the atom manager, calls synchronous `RecordExecution`, and then checks that admission has ended. It does not place two serving tasks before that record. Its helper named `served` examines candidate matching, not a real outgoing provider request; a new end-to-end exposure assertion should observe final inclusion separately. [Existing system test][system-test]

The lifecycle test likewise records two exposures synchronously against a maximum of two and checks withdrawal, reapproval, and promotion. These are controls to preserve, not failed tests or evidence of a concurrent reservation. [Lifecycle test][lifecycle-test]

**Credit and deployment remain separate.** Recording a true exposure does not itself establish that the candidate improved the task. Automatic promotion defaults off. This finding does not claim a default route from the overshoot to automatic deployment. [Promotion default][promotion-default]

**The read-only filter is deliberate and not required here.** The production recorder drops `/unverified` turns to avoid purchasing unnecessary judgment or polluting failure evidence. The existing audit explicitly recognizes that choice. Do not “fix” this counterexample by sending every turn to the judge, and do not recast the declared recorded-turn budget as an undocumented provider-request budget. The primary witness records both turns successfully. [Recorder contract][recorder], [prior lifecycle][prior]

## Proposed contract and repair choices

First decide whether the operator control is a **hard recorded-turn exposure limit** or a **soft stopping threshold**. The current message should not promise the former while implementing only the latter.

For a hard limit, establish a task-owned reservation before the first request carrying C/V is dispatched:

1. Derive trial eligibility using the existing policy and exact version, with declared available allowance that accounts for committed exposure and outstanding ownership.
2. After actual selection/fitting, atomically claim an allowance for the candidate version and stable turn/exposure identity. A compile that merely considers C must not spend it.
3. If the claim fails, omit C and complete a valid prompt without it, or wait under an explicit bounded policy. Do not fail unrelated campaign work merely because an optional trial ran out.
4. Carry the claim with the actual served-candidate receipt. Multiple model calls within the same declared turn must not consume multiple units merely because the transport repeats the prompt.
5. Settle the claim when the recorded turn arrives, linking it idempotently to the exposure row. Credit remains a later independent decision.
6. Release only when the host knows the candidate was not served or that the declared counting contract excludes the turn. A crash after dispatch leaves an unknown outcome requiring reconciliation, not automatically reusable allowance.
7. Recheck expiry, revocation, candidate identity, and policy at the dispatch boundary. Neither a cached prompt nor a previously derived eligibility row is a reusable grant to another turn.

An atomic reservation should be a host-owned mechanical operation, not a model-authored fact. Mangle remains responsible for eligibility and exhaustion policy; the storage layer provides the atomic claim and its evidence. SQLite can serialize writes, but a read followed much later by an independent write is not automatically one claim. Transaction boundaries, conditional updates, uniqueness constraints, busy handling, and rollback need explicit design. [SQLite isolation][sqlite-isolation], [SQLite transactions][sqlite-transactions]

Alternative scopes have different costs:

- **Documented soft threshold:** change UI/spec language to stop admitting after the recorded count is observed at the limit, explicitly allowing already-running tasks. This is low cost, but does not satisfy an operator needing at most N recorded exposures. Quantify overshoot; do not describe it as a hard cap.
- **Single-process reservation owner:** short critical-section ownership per candidate version can close the ordinary same-process campaign schedule. It needs explicit restart and multi-process limits; a mutex around compilation alone does not cover time until recording.
- **Durable reservation ledger:** supports shared workspace and crash reconciliation at greater I/O and lifecycle cost. It can reuse the earlier audit's served-version receipt design without reopening its source-identity issue.
- **Serialize entire trial turns:** simple conceptually, but holds the candidate's opportunity until each turn settles, loses useful campaign parallelism, and still needs correct handling of queue delay and crashes. Do not hold a database transaction or global compiler lock across model/tool execution.

## Costs and failure modes

The hard-limit path adds an admission-time coordination operation for selected trial candidates and a settlement update. Cost is proportional to the number of included trial candidates, not the entire atom corpus. Durable claims add database I/O, lock contention, storage, retention, restart reconciliation, and observable busy/error paths. A short in-memory owner reduces latency but cannot silently claim cross-process or crash guarantees.

Carry compact candidate/version and turn identities rather than full prompt bodies. Serving receipts and pending ownership consume memory and disk; retention must preserve enough evidence to reconcile uncertain dispatch without retaining unnecessary task content. The existing bounded queue should remain bounded, but a rejected or delayed learning record must not silently make already-used allowance free.

Reservation loss after fitting can require prompt reassembly without the optional candidate; account for compiler CPU, cache churn, request latency, and selection changes. Fairness matters when several independent tasks compete for one slot: use a declared order or experiment assignment, not whichever goroutine happens to win forever. Do not change task verification, tool authority, or trial content to make the result look better.

No extra LLM judgment is intrinsically necessary for quota ownership. There may be additional waits, reduced parallelism, or host recovery work; conversely, fewer unintended exposures may reduce token spend. Neither savings nor quality improvement has been measured. Report host resources, provider tokens, elapsed time, and human review separately rather than collapsing them into an unsupported efficiency claim.

## Fair evaluation plan

All proposed evaluations are unrun. Compare current delayed-count behavior, explicit soft-threshold semantics, process-owned reservation, and durable reservation where supported. Keep candidate, version, task, model, tool envelope, verification rules, and token budget fixed.

Use barriers rather than timing sleeps to put two independent campaign tasks after final candidate selection but before either finished-turn record. Both should carry a stable unique candidate and later produce distinct recorded `/done` outcomes. The maximum-one control should distinguish two existing admissions from one owned admission; it must not pass merely by hiding the second exposure row.

| Case | Required observation |
|---|---|
| Maximum one, two admitted parallel campaign turns | At most one turn carries C/V under the hard contract; the other proceeds without it or waits under declared policy. |
| First turn records before the second compiles | Preserve current sequential exhaustion and cache invalidation. |
| Maximum two, two eligible turns | Do not unnecessarily serialize or drop legitimate exposures. |
| Candidate eligible but removed by fitting, conflict, or missing tool | No served exposure and no stranded allowance. |
| Same turn makes several provider calls or retries | Charge the declared recorded-turn unit once; separately account provider work. |
| Duplicate record and conflicting receipt | No duplicate debit; an incompatible receipt is visible rather than silently accepted. |
| Record queue delay, full queue, persistence error, shutdown | Pending ownership remains explicit; no unearned slot release. |
| Cancel before send, uncertain send, crash after send, restart | Release only proven non-exposure; preserve ambiguous outcomes for reconciliation. |
| Expiry, revoke, or candidate replacement during preparation | No obsolete admission; historical exposure remains honest. |
| Wrong pin, no envelope, exhausted count, missing clock, unsupported kernel | Existing fail-closed admission remains. |
| Read-only or learning-disabled turn | Preserve the declared counting policy without buying an unnecessary judge call; specify reservation settlement explicitly. |
| No trial candidates | No mandatory new provider work and negligible coordination overhead. |

Observe final outgoing prompt inclusion, claim identity, turn identity, exposure/credit rows, live trial facts, and final operator counts. Include multiple compiler scopes and cache hits. The existing system test is a valuable starting point, but candidate matching alone does not prove a dispatched exposure.

Measure overshoot, false debits, unused or stranded reservations, successful task completion without the optional candidate, ledger conflicts, queue loss, restart ambiguity, admission and cancellation latency, DB operations, allocations, retained bytes, provider tokens, and human intervention. Preserve both truthful `/done` results in the baseline witness; this audit is not an excuse to weaken verification or discard inconvenient historical records.

**Checkable exit:** the operator's chosen exposure semantics match the UI and source contract. Under a hard maximum of one, two eligible parallel tasks cannot both spend the last allowance, while exact-version quarantine, legitimate campaign concurrency, honest exposure history, and independent execution safety remain intact.

[prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md#L141-L166
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md#L169-L296
[config-contract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/prompt_evolution.go#L47-L52
[trial-ui]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_evolution.go#L262-L303
[approve]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L292-L331
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L144-L173
[campaign-tui]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L112-L133
[campaign-config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/campaign.go#L178-L185
[campaign-schedule]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L250-L293
[campaign-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L85-L98
[task-executor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L395-L449
[executor-clone]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L681-L716
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L352-L380
[trial-sink]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L203-L233
[trial-rule]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/prompt_trials.mg#L1-L15
[serving]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L280-L390
[turn-producer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L2461-L2485
[recorder]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L254-L345
[recording]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_recording.go#L154-L222
[exposure-insert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L150-L175
[after-evidence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_recording.go#L241-L278
[system-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/prompt_trial_admission_test.go#L25-L115
[lifecycle-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/lifecycle_evidence_test.go#L392-L451
[promotion-default]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L88-L100
[sqlite-isolation]: https://www.sqlite.org/isolation.html
[sqlite-transactions]: https://www.sqlite.org/lang_transaction.html

