---
doc-class: deep-dive
subsystem: deliberation-session
implementation-status: proposed-refinement
evidence-status: static-source-inspection
audit-date: 2026-11-03
verified-against: 81eca338209c4d60beeb8a234cd964d5e73bd9e0
supersedes: []
---

# Retain answered deliberation continuations across failed evaluation

## Finding and decision

An answer to a saved values question can become inaccessible through the ordinary continuation route when evaluating it fails. The chat removes its clarification state before running the continuation. For a free-text answer, the conductor saves the original thread and answer with status `running`; a subsequent participant error overwrites that status with `failed`. Pending discovery selects only `awaiting_values`, and the chat resume entry independently rejects every other status. The original question, answer, request and intent can remain intact on disk while the supported conversation cannot resume that answer. [Answer submission][submission] [Mode clearing][mode] [Resume adapter][resume] [Run transitions][run] [Pending discovery][pending]

This is a **durable continuation availability gap**, with a source-derived failure schedule. It is not physical data loss, proof of an observed outage, an incorrect ready verdict, permission bypass, or evidence that any implementation was executed. The current failure is visible, and refusing to invent a design after a failed model call is correct. The gap is that failure of the evaluation attempt also removes the still-unresolved, already-answered requirements interaction from its normal recovery route.

Require a distinction between **the durable question and answer** and **the outcome of one attempt to evaluate that answer**. A failed or interrupted attempt should either remain discoverable under its original owner with an explicit, non-executing recovery action, or have an explicit terminal disposition explaining that the original request cannot continue. Do not repair this by silently retrying, resetting budgets, reopening a cancelled task, repeating the preference question, or turning the answer into work approval.

### Source and verification boundary

Public main resolved to [81eca338209c4d60beeb8a234cd964d5e73bd9e0][pin], committed at 13:22:57 UTC on November 3, 2026, and remained at that SHA when rechecked at approximately 13:43 UTC. Every repository link below uses this immutable revision.

The review inspected public root guidance, the architecture guidance, session and mission specifications, the complete deliberation package, its chat adapters, relevant authored tests, the audit inventory and nearby reports. No application, test, build, inference, gate, benchmark or failure injection ran. No credential, setting or executable implementation was changed. All proposed witnesses and evaluation outcomes remain unrun.

## Why this matters to the requirements workflow

The existing workflow already contains the useful Socratic structure: a proposer offers distinct readings and explicit success conditions, the requirements interrogator challenges them, and Mangle determines whether to refine, collect evidence, ask one values question, proceed or stop. The proposer treats the person's answer as an added constraint on the original request. The challenger is told not to reopen an answered matter. A human who supplies that constraint has completed their part of the current question; a transport failure is not a new unanswered preference. [Proposer][proposer] [Challenger][challenger] [Session contract][spec]

Root guidance assigns orchestration and completion to the kernel, preserves recoverable working context and demands bounded requests without arbitrary task abandonment. The architecture guidance requires target behavior, current implementation and open gaps to remain distinct. Those principles motivate a recoverable answer lifecycle; they do not establish that such a lifecycle is already implemented. [Root guidance][guidance] [Architecture guidance][architecture]

The session specification explicitly promises retained options and original intent for pending values questions and restoration on session load. It also says infrastructure and malformed-output failures must fail visibly. Both guarantees can coexist: one evaluation attempt may fail visibly while its durable, non-executing answer remains available for an explicitly admitted retry. The current specification does not define that cross-failure state transition. This audit therefore identifies a concrete implementation behavior and a contract gap, rather than claiming a documented unlimited-retry guarantee was violated. [Session continuation and failure contract][spec]

The finding is language-independent. A design question about Go, Rust, Python or a foreign repository follows this same host-side lifecycle. No assumption about a specific package manager, test oracle, shell or operating system is needed for the returned-error witness.

## Implemented behavior worth preserving

- A values fork saves one concrete question, eligible displayed alternatives, the original request and intent, the answer field, completed rounds and owner information. Snapshot publication uses a temporary file, synchronization and rename. [Question publication][finish] [Thread record][types] [Snapshot writer][save]
- Exact option answers replay the existing challenged round rather than making another model call. Free text remains a constraint for refinement. [Fast path and refinement][run]
- A free-text refinement frame retains the original request, intent, previous completed round and answer. [Round frame][round]
- Candidate and assessment validation, proof requirements and policy-owned readiness remain in place. A failed role call returns an error rather than a fallback success. [Collection][round] [Proof][proof]
- Mission-owned questions are excluded from ordinary conversation discovery. This report does not undo that ownership correction. [Owner filtering][pending]
- A chosen design is not permission to edit. The chat retains the original verb and restriction, while design-only outcomes stop at the design surface. [Adoption and continuation][resume]
- The ordinary error handler shows a failure, records it and persists conversation history. The issue is not a hidden error message. [Error handling][errors]

These mechanisms substantially narrow the claim. The missing property is a supported lifecycle for an answered question whose evaluation did not produce an admitted result.

## The exact transition chain

### The interface gives up the question before evaluation finishes

The clarification-submit handler records the response in history, captures its pending intent and deliberation ID, then calls `setInputMode(InputModeNormal)`. That method clears `clarificationState` and resets the selected option. Only afterward does the handler schedule `resumeDeliberation`. [Submission][submission] [Mode state][mode]

Leaving the question picker while work runs is reasonable. It becomes relevant because neither the ordinary error path nor later pending discovery restores an answered-but-unevaluated continuation after the durable status changes below.

### A legitimate custom answer enters the convergent path

`resumeDeliberation` loads the saved thread, requires status `awaiting_values`, assigns `thread.Answer = response`, and invokes `Service.Run`. A non-nil error returns `errorMsg` before design delivery or ordinary work adoption. [Resume adapter][resume]

Within the service, an exact candidate ID or rendered option uses the fast path. Other text proceeds through refinement. The service sets status to `running` and saves the thread before the next round starts. Its next frame includes the answer and the previous completed round, then invokes the proposer. If that role returns an error, `collectRound` returns it; `Run` propagates it. [Run][run] [Round][round]

This is not an invalid-answer scenario. The specification and proposer contract explicitly allow free text to add a constraint. The failure can occur before any model output exists to assess.

### Attempt failure rewrites the continuation status

After the service has installed its error defer, any returned error causes:

```go
t.Status = "failed"
if saveErr := s.Save(t); saveErr != nil {
    err = fmt.Errorf("%w; persist failure: %v", err, saveErr)
}
```

There is no branch preserving an already-answered values continuation, no durable distinction between recoverable evaluation failure and terminal rejection, and no separate evaluation-attempt status. The `Thread` shape retains the answer and question but has only the single status field for this lifecycle. [Error defer][run] [Thread][types]

The primary witness assumes this final save succeeds. It does not depend on failed disk publication or power-loss semantics. If saving fails too, the returned error reports that second failure and the disk can instead retain its previous snapshot; that is a separate persistence outcome, not evidence that the primary transition is harmless.

### Discovery and direct resumption both exclude the saved record

`Pending` loads the session's thread files and selects only `Status == "awaiting_values"` with non-mission ownership. A successfully saved `failed` thread is normal history for this selector, not a pending continuation. The session-list load path only installs a clarification state when this selector returns a thread. [Pending][pending] [Session loader][load-session]

Even if a caller still knows the thread ID, the chat's `resumeDeliberation` entry refuses that same saved record because its status is not `awaiting_values`. This is stronger than the disappearance of a UI button: the normal resume boundary explicitly refuses the record. [Resume guard][resume]

The lower-level `Load` function can still read the file. A developer or a different caller can invoke `Service.Run` with a loaded thread; the service does not categorically refuse all failed states. Therefore **permanent irrecoverability is not the claim**. That lower-level access is not a supported restoration path in the inspected chat flow. Simply running a failed exact-choice thread directly would also miss the existing exact-choice fast path, whose guard requires `awaiting_values`.

## Small source-derived witness

Use the ordinary design-only `/deliberate` or `/innovate` path in non-yolo mode. Keep one conversational thread, ample context, available proof support, successful snapshot writes, stable workspace evidence and sufficient remaining round allowance. No mission, parallel run, malformed participant output, rejected candidate, oversized payload or permissions change is needed.

1. The first valid round yields two viable alternatives and a policy-admitted values question. The thread is saved with `Asked=true`, one completed round and `Status="awaiting_values"`.
2. The user types a legitimate custom preference, for example, “Prefer preserving existing callers, and keep the new behavior opt-in.” It intentionally does not equal either candidate ID or full rendered option string.
3. The submit handler retains the ID locally and clears the active clarification state. The resume adapter loads that same thread and assigns the answer.
4. The service bypasses exact-option replay, saves `running`, then starts the convergent round. The proposer callback returns a temporary transport error before any new proposal is returned.
5. The service's error defer saves `failed`. The chat reports the error and does not adopt a design or launch development work.
6. Reload the same session through the session-list path. Its pending selector skips this intact failed thread. Attempting the same chat resume entry by ID also refuses it.

**Expected source-derived observation:** the saved question, original request, restriction and supplied answer still exist, but neither ordinary discovery nor the guarded resume entry offers a way to continue evaluating that answer. The user must reconstruct/restate the task or use a different, presently unspecified recovery route.

This schedule is proposed, not reproduced. Step 4 is a controlled callback failure for a future fixture, not a claim about any provider's measured error frequency. Use a plain error such as `temporary transport failure` so the legacy generic error-substring clarification branch is not involved. [Error dispatch][error-dispatch]

### Related schedules and counterexamples

| Schedule | Source-derived disposition | Interpretation |
|---|---|---|
| Successful exact offered option, no fault | Existing fast path can return ready without another role call. | Preserve this efficient control. |
| Successful free-text refinement | Can continue under the existing round and readiness rules. | The answer representation itself is not missing. |
| Dependency/config failure before the error defer is installed | Does not pass through this particular status rewrite. | Do not claim every failure consumes the durable question. |
| Exact-option replay fails during assertion or proof | The same outer defer can save failed, despite no new role call. | Availability gap also has a non-inference variant; proof checks must remain. |
| Process exits after the running snapshot and before a terminal save | The durable record can remain running, which Pending also excludes. | A distinct interrupted-attempt case for recovery design; no crash was run. |
| Fact cleanup fails after finish returned a ready outcome | The cleanup defer can set the returned error before the outer defer runs and saves failed. | Keep settlement/cleanup state separate from logical readiness; do not assume cleanup was harmless or safely repeatable. |
| Explicit cancellation or policy round/no-progress stop | Must remain stopped unless the owner explicitly requests an admissible continuation. | Recoverability is not authorization to resume automatically. |
| Final failure save itself fails | Prior disk state and returned persistence error matter. | Reconcile observed state; do not blindly retry or claim the answer was durably saved. |

The returned-error witness deliberately excludes source drift. Current source-validity admission remains a separate unresolved concern, already documented elsewhere. A recovery feature must not make stale historical assessments current merely because the answer survived.

## Relationship to existing audits

The complete tree and audit queue were screened, and the nearest relevant reports were read. No exact predecessor for the **answer submission → failed evaluation → excluded recovery** chain was found in that bounded review. This is not a claim of exhaustive novelty across every branch, historical commit or unindexed note. [Audit queue][queue]

- [Recurse values continuation ownership][owner-audit] concerns mission/chat owner selection, multiple pending records and legacy owner migration. This witness uses one correctly owned conversational thread.
- [Values evidence freshness][freshness-audit] concerns reusing historical factual premises after a pause. This witness keeps the workspace unchanged and fails before a new verdict.
- [Values option admission][option-audit] concerns choosing a candidate outside the displayed admitted set. This witness uses legitimate free text, not a hidden candidate ID.
- [Proof recording ownership][proof-audit] concerns one overlapping run disabling another run's recorder. No overlap or recorder race is needed here. A proof failure can expose this lifecycle gap, but fixing recorder ownership does not address provider failures or interrupted running records.
- [Terminal error clarification disposition][error-audit] concerns interpreting error text as a fresh question. Here the error follows the ordinary failure branch, without either triggering substring.
- [Probe consumption horizon][probe-audit] concerns evidence collected beyond a remaining review round. Here a review round is still allowed, and no conductor probe need run.
- [Selected review handoff][review-audit] concerns omitted important critique after successful selection. Here no successful revised design exists to hand off.

## Proposed contract

The following names describe a proposed contract, not implemented APIs or status values.

### Durable answer ownership

Bind an answer receipt to the existing thread, session/owner, question generation and answer kind. Retain the original question and option set, original request and intent, restriction, `Asked` state, exact answer and completed evidence record. Do not classify the answer as a new request or grant it new authority.

A receipt should distinguish “answer captured” from “answer evaluated.” For an exact option, retain the selected offered-option identity. For custom text, retain it as an added constraint. Existing option-membership and evidence-freshness improvements should be dependencies, not replaced by this lifecycle work.

### Evaluation attempt disposition

Track an attempt separately from the durable interaction. Its disposition should distinguish at least completed evaluation, known failed evaluation, interrupted/unknown completion, user cancellation and policy exhaustion. Preserve enough bounded cause and stage information to choose a recovery path without inventing what happened.

A generic error should not erase the answered interaction's identity. Conversely, a record marked failed must not automatically be treated as safe to retry. Unknown provider consumption, cleanup failure, uncertain snapshot publication and stale evidence require distinct reconciliation.

Mangle should continue to own continuation admission, limits and readiness. Go should transport measured attempt outcomes, validate records and perform durable publication, not independently decide that every error deserves another inference.

### Discovery without automatic execution

Expose an interrupted or failed answered interaction as **answer retained; evaluation incomplete**, associated with its original owner. It can be inspectable and offer a bounded retry or explicit abandonment choice without taking over the user's next unrelated message.

Do not revert it to an unanswered question and ask for the same preference again. Keep `Asked=true`. A recovery action can ask whether to resume the retained work, when authorization is needed; it must not disguise that action as another requirements question or as approval to mutate code.

Mission-owned records remain in their owner's recovery surface. Ordinary chat discovery must not reintroduce the earlier mission-question collision.

### Transition and delivery boundaries

Define observable points for answer receipt, attempt start, evaluated result, delivery, and error/cleanup settlement. Require idempotent handling of the same answer receipt. After a crash, do not infer completion solely from an old process's `running` marker or infer safe repetition from absence of a final message.

The smallest conservative implementation can retain a recoverable record and offer explicit restart of its evaluation, without a background retry worker or cross-file transaction framework. If answer/result/delivery metadata are separate writes, define reconciliation for partial publication; a single atomic snapshot does not make the whole UI-to-evaluation sequence atomic.

### Preserve the existing limits

Retain used rounds, observed costs, restriction and stop reason. No reset to a fresh four-round allowance merely because the prior attempt failed. Preserve applicable usage accounting and admission rules; define unknown-consumption recovery for ordinary chat explicitly rather than assuming mission stop rules already govern it. A genuinely new envelope or changed request requires explicit owner action.

Reusing a completed challenged round may avoid model work only when its exact answer membership, source validity and proof admission are satisfied. Freshness checks must not be waived to make recovery appear cheap. Required proof failure must remain a refusal, never a fallback to model confidence.

### Checkable exit requirement

**GAP-DELIBERATION-ANSWER-RECOVERY:** after a submitted values answer encounters a known evaluation failure or an interrupted attempt, its original owner can inspect the exact retained answer and the incomplete disposition, then explicitly resume or abandon it through a supported path. No duplicate preference question, new task interpretation, automatic execution, budget reset or stale-evidence adoption occurs. Successful no-fault behavior and existing mission isolation remain unchanged.

The requirement is proposed. It is not closed by adding a new status string alone; the chat producer, persistence reader, admission policy and result surface must agree on the transition.

## Evaluation plan that has not been run

Start with deterministic role results and controlled errors through the actual chat submit, resume, persistence and session-load paths. An isolated JSON serialization test cannot establish the composed interaction.

The authored `TestDeliberationValuesForkAsksOnceAndResumesOriginal` exercises a successful persisted exact choice, checks that it makes no further model calls, preserves the original request and still holds propose-mode work. The persistence tests cover round-trip identity and mission/chat isolation. These are useful controls, not evidence for the failed-evaluation schedule: the inspected tests do not inject a role error after answer submission and then exercise session restoration. They were read, not run. [Existing chat control][chat-test] [Existing persistence controls][persistence-tests]

### Baselines and comparisons

Compare the pinned current implementation against a narrowly scoped candidate implementing the recovery contract, with identical initial saved question, workspace, role outputs, mode, proof behavior and resource limits. Keep model/provider choice out of the deterministic phase. Count all attempted dispatches, not only successful returns.

Include three baseline flows: successful exact choice, successful custom refinement, and explicit intentional abandonment. The first should retain its no-new-model-call property; the second should preserve the answer in the challenger/proposer frame; the third must never be resurrected.

### Failure matrix

1. Proposer failure before a new proposal, with the running snapshot already saved.
2. Challenger failure after a valid proposal, including any measured consultation costs.
3. Exact-choice assertion or proof failure, without new inference.
4. Interruption before answer publication, after running publication, after completed-round publication and after ready publication but before delivery.
5. Known failure to write a snapshot versus an uncertain publication outcome.
6. Failed exact-fact cleanup after a selected outcome; preserve quarantine/reconciliation rather than blindly repeating.
7. Duplicate submission of the same answer, stale answer for a replaced question, and two attempted resumptions.
8. Owner cancellation, round exhaustion, no-progress stop, unknown spend and source drift.
9. Correctly isolated mission records and legacy records without sufficient identity.
10. An unrelated new chat request while an older answered interaction is recoverable.

For every case, assert thread/owner identity, exact answer conservation, visible incomplete status, number of preference questions, number of role/tool dispatches, worker launch count, retained limits, source admission and final result provenance. The answer-only and design-only failure cases must not dispatch development work.

Test the real session-list restoration path, not merely `Service.Load`. It is acceptable for a conservatively classified record to require explicit recovery or remain blocked with a reason. It is not acceptable to silently drop it, falsely label it evaluated, or treat recovery as execution approval.

### Fair live qualification

Only after separately authorized deterministic checks should a live study compare user recovery effort and completed design quality. Use the same model, task distribution, time/spend ceilings and externally controlled fault placements in both arms. Include ordinary no-fault runs so the repair cannot win by suppressing useful work or adding disproportionate overhead.

Score requirements preservation and correctness using reviewers blinded to the treatment where practical. A design is not correct merely because it resumed, and a conserved answer is not proof of implementation success. Report small samples, exclusions, provider retry behavior and uncertainty; do not borrow published deliberation benchmark gains as evidence for this host change.

## Full cost and failure accounting

Discovery or displaying a retained answer needs no model call by design. That does not imply zero cost. Account for:

- **Inference:** prior successful rounds, the failing attempt, provider-internal retries, resumed rounds, consultations, output tokens and any unknown consumption. A timeout may still have incurred a charge.
- **Evidence:** repeated reads, revalidation after source drift, tool calls and any remote retrieval. Reuse is only valid when its evidence contract permits it.
- **Kernel work:** assertion/query/proof extraction, exact fact cleanup, admission and reconciliation. A failed proof or cleanup can be expensive without producing a useful result.
- **Persistence:** snapshot serialization, file creation/write/sync/rename, session-history writes and recovery scans. Current Pending scans and loads session thread files; if total retained bytes are S, its read work includes O(S) bytes. New metadata should remain bounded and must not multiply full-history rewrites unnecessarily.
- **Latency:** answer-to-error, recovery discovery, explicit retry decision, completion latency and p50/p95 disk/kernel overhead. Include cold start and restart separately from steady state.
- **Human effort:** retyping constraints, repeated preference questions, locating the saved interaction, deciding whether a retry is safe, and reviewer time checking requirements preservation.
- **Engineering:** schema/version migration, legacy quarantine, UI state integration, owner-aware selection, fault fixtures, concurrency control, observability and support burden.

No numeric savings or failure-rate claim is made. A useful repair should reduce lost-continuation incidents and repeated human input without increasing unauthorized dispatches, exceeding limits or hiding uncertainty. Keep dollars, tokens, time, storage and human minutes separate unless an explicit conversion model is supplied.

Failure risks of the repair include retry storms, repeated billing, duplicate evaluation/delivery, accidental reactivation after cancellation, stale candidate reuse, wrong-owner discovery and a growing queue of unrecoverable history. Keeping all failed records permanently pending would merely replace this gap with availability and attention problems. Explicit classification, retention policy and abandonment semantics are necessary.

## Primary literature and its limits

The W3C [SCXML Recommendation, sections on final states, events and errors](https://www.w3.org/TR/2015/REC-scxml-20150901/#SCXMLEvents) separates an error event from a deliberately selected final-state transition. It provides useful vocabulary for specifying whether an evaluation attempt failed, an interaction remains recoverable, or its owner intentionally ended it. It does not mandate SCXML, automatic retries or any particular Go implementation.

Alvaro, Rosen and Hellerstein's [Lineage-driven Fault Injection, SIGMOD 2015](https://people.ucsc.edu/~palvaro/molly.pdf) starts from desired outcomes and reasons about failures that could prevent them. Here that motivates testing the entire answer-to-restoration dependency chain, rather than treating atomic file publication as proof that the composed interaction survives faults. The paper studies different systems and does not verify codeNERD or imply a measured efficiency gain for this proposal.

## Recommended scope

Add this as one linked lifecycle gap in the existing deliberation/session specification. Keep the current proposer, challenger, JIT atoms, read-only restrictions and Mangle readiness boundary. Implement only after the recovery states, owner binding, cost admission and user-visible transition are agreed and the failure matrix is checkable.

The narrow target is practical: a person should not need to restate a requirements answer merely because the harness failed while considering it. Preserving that answer must never silently authorize another paid attempt, a stale design or code execution.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/81eca338209c4d60beeb8a234cd964d5e73bd9e0
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/agents.md#L1-L72
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/architecture/AGENTS.md
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L898
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/prompt/atoms/system/deliberation_proposer.yaml#L12-L28
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/prompt/atoms/system/requirements_interrogator.yaml#L12-L27
[submission]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/model_handlers.go#L459-L503
[mode]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/model_types.go#L129-L148
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/deliberation.go#L58-L119
[run]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/service.go#L28-L166
[round]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/round.go#L10-L72
[types]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/types.go#L71-L120
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/outcome.go#L16-L86
[proof]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/policy.go#L42-L73
[save]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/persistence.go#L23-L74
[pending]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/persistence.go#L77-L108
[load-session]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/session_persistence.go#L229-L275
[errors]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/model_errors.go#L11-L34
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/ASTRA_QUEUE.md
[owner-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/recurse-values-continuation-ownership.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/deliberation-values-evidence-freshness.md
[option-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/deliberation-values-option-admission.md
[proof-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/deliberation-proof-recording-ownership.md
[error-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/terminal-error-clarification-disposition.md
[probe-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/deliberation-probe-consumption-horizon.md
[review-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/Docs/audits/deliberation-selected-review-handoff.md
[chat-test]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/deliberation_test.go#L179-L219
[persistence-tests]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/internal/deliberation/deliberation_test.go#L99-L177

[error-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/81eca338209c4d60beeb8a234cd964d5e73bd9e0/cmd/nerd/chat/model_update.go#L289-L305
