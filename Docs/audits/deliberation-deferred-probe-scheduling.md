---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: cc9dce8e2412a8cb6e688841738e3b212e630378
supersedes: []
---

# Carry deferred data probes across deliberation rounds

## Finding and decision

An empirical round can contain more critical data probes than its call allowance. The conductor attempts the first allowed prefix, saves each result or error, and returns when the allowance is spent. On the next empirical round it starts at the beginning of the new assessment again. It does not consult earlier attempts, carry a deferred-item cursor, or require a retry disposition before buying the same reads again. A stable failing prefix can therefore consume successive rounds while an already-requested, permitted later read is never attempted. [Dispatch][dispatch] [Collection][collect] [Record contract][records]

Under the defaults, a seven-item candidate whose first six reads return stable errors can produce **18 failed probe dispatches across three completed rounds, zero attempts of item seven, and a no-progress stop before round four**, if the participants retain that candidate and item order. This is a source-derived conditional schedule, not an executed result or a claim about normal model frequency. [Defaults][defaults] [Fingerprint][fingerprint] [Loop][loop] [Stop policy][stops]

**Classification: a bounded scheduling and continuation contract gap.** The current implementation does not promise fairness among pending probes, so this is a proposed strengthening of the requirements workflow, not an allegation that an explicit existing fairness guarantee was violated. The source-confirmed mechanism is repeated prefix selection. Whether a live participant would maintain the witness's item order, or whether the unattempted read would lead to a better design, remains unmeasured.

The narrow refinement is to preserve a bounded probe frontier: distinguish a new pending read from an earlier failed attempt, derive any retry priority explicitly, and retain a reason for every deferred item. Keep the same round, tool, context and permission limits. Do not erase an unresolved requirement, treat a failed read as evidence of success, raise the budget, or add another model judge.

### Pin and execution boundary

The initial public main observation resolved to `0e792aa7a7ed487418a3ce68b1a18076c0f94457`. A final read at approximately 16:22 UTC on November 3, 2026 resolved to **cc9dce8e2412a8cb6e688841738e3b212e630378**, committed at 16:15:36 UTC. The complete four-commit comparison changes only four unrelated campaign/delivery audit status appendices. None of the cited implementation, guidance, architecture, authored tests or neighboring deliberation reports changed. Source copies were checked against their Git blob identities; the comparison establishes their applicability at the final pin. Repository citations use that final immutable revision. [Comparison][comparison] The review inspected public root and scoped guidance, current architecture, the deliberation package and production adapters, relevant authored tests, the audit inventory and neighboring reports. [Pin][pin] [Root guidance][guidance] [Architecture guidance][architecture]

No repository program, build, test, model inference, benchmark, gate or failure injection ran. No implementation, credential, configuration, permission or setting was changed. All witnesses, state extensions, proposed evaluations and benefit claims below are unrun.

## 1. Why this matters to Socratic requirements work

The existing design has the needed roles: a proposer offers distinct readings, scope, assumptions and success conditions; a requirements interrogator tests them against the complete request and workspace evidence. Missing data is supposed to cause a concrete read, not a question to the user. Stable candidate IDs and objection keys let refinement preserve the same underlying requirements. [Proposer contract][proposer] [Interrogator contract][challenger]

That stability should not force stable execution of an unsuccessful list prefix. “The requirement remains unresolved” and “repeat its same evidence request before inspecting any remaining request” are different decisions. A foreign repository can have several missing or moved paths and one already-requested discovery read that would reveal its layout. Honest preservation of the unanswered requirements should not itself determine which failed lookups consume the next allowance.

The repository puts orchestration in Mangle and allows bounded work to continue while it makes useful progress. The current session specification documents measured probes feeding later frames, errors retained in history, explicit stopping and durable round records. These controls are compatible with a small scheduling refinement; neither unbounded retries nor automatic execution of every model-proposed probe follows from them. [Executive contract][guidance] [Current session specification][spec]

Preserve the implemented distinctions:

- A candidate can be empirical without being ready.
- Critical data, design and values objections have different meanings.
- The call allowance is checked by policy; reads still pass the existing registry guards.
- Probe errors are retained, not reported as successful observations.
- A stopped thread is not labeled a completed design.
- Design-only chat does not start a development worker.
- Recurse mission preparation has independent source, permission, budget and execution admission.

This audit changes neither readiness nor authority. It asks how the existing evidence allowance is allocated among already-declared data questions. [Policy][policy] [Read adapter][read] [Chat command][chat-command] [Mission admission][mission-admission]

## 2. Exact current producer-to-consumer chain

### 2.1 More data items than calls are admitted

`Assessment.Items` is an ordinary item slice. `validateRound` checks nonempty unique item keys, closed severity/kind vocabulary and a nonnil probe for every critical data item. It does not cap the item count at `DeliberationToolCalls`. The candidate and consultation caps are separate controls. Seven short data items are structurally admissible under the default six-call allowance. [Item shape][items] [Validation][validation] [Proposal admission][collect] [Defaults][defaults]

This is not itself a defect. A truthful assessment should be able to describe more open requirements than can be investigated immediately. Refusing the whole assessment merely because its complete obligation inventory exceeds one batch would hide useful work. The missing contract is what happens to the remainder.

The empirical rule requires a critical item and no critical non-data item. It does not require a prior success assertion. The policy selects the best-ranked eligible candidate; the host then calls `runProbes` for an empirical decision. Thus the witness can retain honest `success: false`, with all seven data requirements open. [Empirical eligibility][eligibility] [Decision dispatch][loop]

### 2.2 The host chooses a fresh prefix every round

`runProbes` creates a round-specific actor and a local `calls := 0`. It iterates the selected candidate's `Items` in supplied order. For each critical data item, it calls `AllowCall`; a refusal returns nil from the entire probe function. For an admitted item, it increments the counter before calling the probe adapter. [Dispatch][dispatch]

The admission rule is `N < Cap`. A failed execution consumes a slot because its increment already occurred. That is a reasonable cost policy: failed attempts still consume host work and may consume provider or remote service resources. The gap is not charging errors. [Call policy][call-policy] [Admission helper][allow]

The probe's ordinary error is copied into `ProbeResult.Error`, appended to `Round.Probes`, persisted, and projected as a digest receipt. It is not returned as a fatal service error. Therefore a round can complete normally after all six probe attempts fail. Persistence or kernel failures are different: those do propagate and are excluded from the principal witness. [Result handling][dispatch]

No part of this selector consults earlier rounds' probe results or subtracts previously attempted invocations. It has no next-item index, unattempted queue, last-attempt state or retry priority. A round-specific actor avoids one kind of counter collision; it is not a cross-round pending-work record.

### 2.3 The remainder survives as a request, not a scheduling state

The saved challenge still contains the seventh item and its exact probe. Its omission from `Round.Probes` can be discovered by comparing the challenge and result list. This audit does **not** claim physical loss of that request. [Round record][records] [Persistence][save]

However, the return-on-cap branch records no explicit “not attempted because batch allowance was spent” result for the remainder. The next frame contains the preceding round and receipts, but the conductor asks for a new proposal/challenge and again traverses the new selected assessment from its beginning. Neither a preserved original item nor a retained failure automatically changes dispatch priority. [Collector][collect] [Dispatch][dispatch]

Both participants can inspect the prior round, reorder the items, revise paths, remove an actually resolved objection, or use their own read tools. Those are real escape routes. They limit the finding to **the conductor's behavior for otherwise valid participant output**, not impossibility of progress for the whole system. Nevertheless, requiring a participant to rediscover and repair list scheduling differs from executive continuation of already-declared evidence work. [Role-local reads][agent] [Previous-round frame][collect]

### 2.4 Repetition reaches the existing, valid stop

The progress fingerprint uses critical item identities and serialized probe results. With stable keys, kinds, outputs and errors, it is identical across the witness's rounds. The service records unchanged counts 0, 1 and 2, then the pre-round policy stops before round four. [Fingerprint][fingerprint] [Loop][loop] [Stop policy][stops]

Here the no-progress decision accurately describes the observations the host actually produced. The problem is earlier allocation: the same failed reads were repeatedly selected while a different pending read remained available. Changing the fingerprint to treat a new round number or attempt number as progress would conceal the stall and increase waste; it is not the proposed repair.

## 3. Production reach and limits

The chat deliberator uses `ReadTools.Execute` as its probe callback and marshals frames for the registered roles. `/deliberate` and `/innovate` call this service with a design-only thread. On a stopped result, the ordinary surface returns the status rather than an adopted design. [Chat adapter][chat-adapter] [Chat command][chat-command] [Outcome rendering][note]

The recurse-mission constructor also supplies `ReadTools.Execute` to the same service. Preparation creates a mission-owned design-only thread, saves its reference, and maps non-proceed, non-question outcomes to stopped. Execution remains behind the owning mission's independent admission and source checks. This finding may prevent a useful mission design from advancing; it does not demonstrate an unauthorized write, a bad retained patch, a weakened test gate or a falsely verified mission result. [Mission constructor][mission-constructor] [Mission preparation][mission-preparation] [Mission admission][mission-admission]

The mechanism is language-independent list scheduling. Its impact on unfamiliar layouts is plausible, but no cross-language, provider or operating-system benefit was measured. The concrete file example below uses ordinary workspace paths and built-in tools rather than an ecosystem-specific parser.

The repository's live-exercise ledger records a fifth deliberation run converging in two rounds after earlier defects were corrected. Preserve that positive, owner-reported evidence. This audit did not rerun it, and that receipt does not exercise a stable failed prefix with an unattempted tail. The generic session text's older runtime-pending wording should not be used to erase the more specific ledger. [Live exercise][live]

## 4. Minimal unrun witness

### Preconditions

Use the default four-round, six-probe-call and two-unchanged-transition bounds. Use a fresh thread with an empty fingerprint, live context, stable workspace, functioning JIT, proof support and successful persistence. Keep all strings, errors and frames comfortably within the configured byte bound. No consultations, participant-local reads, values questions, malformed JSON, concurrent activity or source changes are needed.

The scripted participants retain two valid candidates across three rounds:

- **A**, first in rank and recommended, has seven critical `data` items, keys `d1` through `d7`, in that order. Every item has a valid offered read call. A remains `success: false`.
- **B** has a single stable critical `design` item and remains `success: false`.
- Briefs, targets and success tests are ordinary nonblank strings. IDs and keys are unique in their required domains.
- The first six calls always return the same short error for their respective invocation. The seventh is independently executable and would return useful new information. Nothing requires executing that seventh call to make the first six error outcomes deterministic.

A built-in realization to test later could have `d1` through `d6` read six expected files under nonexistent ordinary subdirectories, while `d7` requests `list_files` on an admitted existing directory. The missing-file helper returns a stable error and specifically suggests discovering the correct path. A successful directory listing could orient the next refinement. This is an illustrative fixture, not a claim that the roles should keep proposing obviously mistaken paths, nor that the listing proves the requirements satisfied. [Missing-path handling][missing-path] [Read handling][file-read] [Directory tool][file-list]

### Source-predicted trace

| Step | Probe work | Saved result | Next action |
|---|---|---|---|
| Round 1 | Attempt d1–d6; count reaches six before d7. | Six error observations; all seven requested items remain in the challenge. | Set fingerprint, unchanged = 0. |
| Round 2 | Restart at d1; attempt d1–d6 again. | Six more corresponding errors; d7 still has no attempt. | Same fingerprint, unchanged = 1. |
| Round 3 | Restart at d1; attempt d1–d6 a third time. | Six more corresponding errors; d7 still has no attempt. | Same fingerprint, unchanged = 2. |
| Before round 4 | No dispatch. | Thread saved as no_progress. | Return stopped. |

The resulting **18 failed dispatches, six distinct attempted reads and one never-attempted pending read** are static control-flow arithmetic for these premises. They are not runtime measurements. Three calls to each of six probes are not 18 distinct evidence acquisitions.

The trace needs no argument-identity collision: the repeated calls really are the same. It needs no last-round probe: d7 could have been attempted in round two and considered in round three without adding a round. It needs no oversized evidence: even empty outputs with short errors suffice. It needs no mixed-kind candidate: A has only data blockers. [Dispatch][dispatch] [Collection order][collect] [Loop][loop]

Do not append a hypothetical “therefore ready” conclusion. The directly checkable scheduling endpoint is that d7 receives an attempt or explicit justified deferral before equivalent unsupported retries consume the remaining allowance. All unresolved requirements must remain visible. A useful listing might improve the design, fail to help, reveal a new blocker, or justify stopping.

### Controls and counterexamples

- With at most six items, the first round can attempt all of them; this specific suffix gap is absent.
- If the challenger moves d7 ahead of a repeated item in round two, the current code can attempt d7. The conditional witness depends on retained order.
- If a role performs d7 itself, the overall conversation may obtain the information even though the conductor never executes that pending request.
- If an earlier failure becomes recoverable, a retry may be the best use of a slot. The proposed contract must allow policy-admitted retries rather than permanently suppress all failures.
- If every unattempted call is disallowed, dependent on missing information, no longer relevant, or lacks a review opportunity, deferral or stopping may be correct. Being listed is not permission or proof of usefulness.
- If the workspace changes or the effective probe arguments change, an earlier attempt may no longer apply. Stable item keys alone cannot justify reuse or suppression.

## 5. Novelty relative to existing audits

This report builds on the prior bounded-crucible direction, rather than presenting a new agent framework. The concrete missing boundary is **attempt-aware selection inside an already admitted empirical batch across rounds**.

| Existing report | Its boundary | Why this witness remains after that boundary is fixed |
|---|---|---|
| Mixed-objection probe admission | A candidate with both data and design blockers lacks empirical dispatch. | Candidate A here already qualifies as empirical; six calls dispatch each round. |
| Progress observation identity | Different executed invocations can collapse into the same fingerprint. | The prefix invocations here are genuinely identical; d7 is not executed at all. |
| Probe consumption horizon | A final-round read can lack a following review opportunity. | d7 has a round-two slot and round-three consumer in the unchanged envelope; prefix retries occupy the slot. |
| Probe frame capacity | Admitted observations can exceed the next role's encoded frame. | All witness records are short; no capacity rejection is necessary. |
| Read invocation identity | Role-local results omit requested arguments. | The witness uses conductor calls with their source challenge intact; exact attempt identity alone does not select a deferred item. |
| Critical-item retirement | A later round may drop or downgrade earlier blockers without accounting. | Every blocker is retained. Removing a failed requirement to advance the tail would be the wrong repair. |

Sources: [Mixed objections][prior-mixed], [progress identity][prior-progress], [horizon][prior-horizon], [frame capacity][prior-capacity], [read identity][prior-read], [retirement][prior-retirement], [portable crucible][prior-crucible].

The mixed-objection audit already asks for explicit deferred work, unique item selection, retry/lifecycle handling and an evaluation that prevents accidental iteration-order priority. Credit that broader scheduling requirement directly. This report's distinct contribution is the all-data repeated-prefix witness, its exact cross-round arithmetic, and an attempt-aware selection refinement for the already eligible empirical path. It is not the first identification of iteration-order priority as a general concern. Those shared requirements should be reused rather than counted as separately discovered defects. [Earlier scheduling design and evaluation][prior-mixed]

## 6. Proposed contract, not implemented

### 6.1 Keep requirements separate from attempt state

Retain the challenge's complete open-item inventory. Add or derive an attempt-aware frontier under the owning thread and candidate revision. A compact state record should distinguish:

- Requested and not yet attempted.
- Admitted for a specific attempt.
- Completed with an observation, including failed execution.
- Deferred with a named reason, such as current allowance, missing prerequisite, no review capacity or unchanged failed invocation.
- Superseded or withdrawn through an explicit assessment disposition.

These are proposed semantics, not existing fields or committed predicate names. A failed attempt must leave the underlying requirement unresolved unless an admitted later assessment explains otherwise. A deferred request is not a completed read. An observation is not automatically an accepted requirement.

The identity must bind the thread/owner, candidate and revision, blocker key, effective tool plus arguments, workspace/evidence validity, and attempt. Stable keys support continuity; changing the key, transport call ID, item position or round must not automatically reset attempt history. Conversely, reusing a key with materially different arguments must not suppress a genuinely new request. [Current limited result shape][items] [Earlier identity work][prior-read]

### 6.2 Derive selection instead of repairing list order in a Go heuristic

The host should measure and validate the complete request and attempt inventory, publish typed facts, and execute only the next policy-admitted probe. Mangle should own selection, retry eligibility and any deferral. Do not implement a hidden Go “best next read” ranking that bypasses the architecture's executive split. Model judgments about information value can be inputs, but need an explicit trust class and bounded effect on selection. [Executive contract][guidance]

For an initial conservative treatment, keep the current selected candidate and ordinary permission envelope. Within that candidate:

1. Prefer an eligible unattempted invocation over a retry of a previously failed invocation with unchanged effective arguments and applicable source state, unless policy admits the retry first.
2. Permit a bounded retry when its declared conditions hold, for example a transient failure whose retry interval elapsed or changed evidence affecting its applicability.
3. Persist and expose why the other pending calls were deferred.
4. Stop honestly if no useful, authorized, reviewable acquisition remains.

This is a proposed policy, not an assertion of optimal information allocation. Strict round-robin and highest-estimated-value selection are alternatives, not additional mandatory mechanisms. A cursor alone is a useful baseline but cannot handle reordered items, changed arguments, source drift or intentional priority retries.

### 6.3 State a bounded, conditional service property

Do not promise that every requested probe eventually runs under finite budgets. A useful checkable requirement is:

> While an unattempted probe remains eligible, a repeated failed invocation must not consume its place solely because it appears earlier in the assessment. It needs a distinct policy-admitted retry priority, or the unattempted probe needs a policy-valid deferral reason independent of list position. Spending the allowance on that same failed prefix is not such a reason.

For a finite stable frontier with equal eligibility, no priority retries or other valid deferrals, and sufficient admitted slots plus review capacity, require work-conserving first-attempt selection: fill each available slot from eligible unattempted invocations until either that frontier or the batch allowance is exhausted. Each invocation then receives an initial attempt before any unchanged failure repeats. With n items and K positive slots per empirical round, first-attempt coverage takes at most ceil(n/K) admitted batches under those additional assumptions. Sufficient total slots alone does not establish this bound if the scheduler can idle them. This is a target contract, not a guarantee of the current implementation, semantic correctness, or eventual design readiness.

Requirements that exceed the remaining envelope receive explicit deferred/unresolved results. Increasing a cap, opening another thread, inventing progress from attempt counters, or silently converting a read into a command are not acceptable ways to satisfy the property.

### 6.4 Compose with evidence consumption and capacity

An admitted read must have an allowed consumer opportunity and a bounded, recoverable representation. Frontier repair must not recreate the existing horizon or frame-capacity gaps. Preserve the current byte bound and record partial evidence. If a useful next read cannot fit or cannot be reviewed in the remaining envelope, record that specific limitation instead of retrying something else merely to use the allowance. [Horizon requirement][prior-horizon] [Capacity requirement][prior-capacity]

Only host-measured new evidence or an admitted assessment transition should affect progress. Dispatching a different item, changing a record state, or issuing a new attempt ID is not itself proof of useful advancement.

### 6.5 Recovery and ownership

Keep frontier ownership with the existing conversation or mission thread. Save completed outcomes before advancing their scheduling disposition. After an interruption, distinguish “never started,” “known failed,” “completed but not yet recorded” and “outcome unknown” where the adapter supports those distinctions. An unknown outcome is not automatic permission to repeat the read, particularly for remote tools whose effect classification needs separate care.

A lost or unreadable frontier must not be reconstructed as “all probes new” without disclosure. A cancelled thread must not restart simply because pending items remain. Old saved threads need an explicit compatibility policy: retained history can establish known attempts, but absent identity or source-validity data is unknown, not proof of safe reuse.

This audit does not authorize implementation, automatic resume, retry, migration or broader tool access.

## 7. Costs, tradeoffs and failure modes

### Current source-derived costs

Let K be the conductor call allowance and R the number of completed empirical rounds in the schedule. A stable prefix with at least K items can consume K × R attempts of the same K invocations. The defaults and witness give 6 × 3 = 18, not a general count for every deliberation.

There are two participant invocations per completed round, so this fixture invokes the proposer and challenger six times. A participant invocation is not necessarily one paid provider call: the production role has its own evidence loop, closing behavior and possible provider retries. The scripted fixture intentionally removes those extra actions to isolate scheduling; it cannot supply an invoice estimate. [Role loop][agent] [Collector][collect]

On the all-successful-persistence witness path, the service makes **26 thread-save calls**: one initial running save; for each of three rounds, one completed-round save, six probe-result saves and one fingerprint save; then one stop save. Each save serializes the complete accumulated thread and uses a temporary file, write, file sync, close and rename. This is source-derived call arithmetic, not observed I/O latency or a new durability finding. [Loop saves][loop] [Probe saves][dispatch] [Save implementation][save]

The relevant total cost includes:

- Role input, output and reasoning usage where available; JIT and tool-definition context; consultations and retries.
- Every successful, failed and refused tool request, including registry checks, filesystem or remote-service work.
- Kernel assertion/query/provenance work and cleanup.
- Repeated full-thread serialization, bytes written, sync/rename latency and retained history.
- Consumer re-reading of repeated errors, remaining useful evidence collection, and later review.
- Cancellation, interrupted-work reconciliation, operator attention and any separately authorized implementation/verification.

Six calls is neither a universal dollar cap nor a token or wall-time cap.

### Proposed costs and tradeoffs

A frontier adds state, admission checks, dependency bookkeeping and persistence. Naively comparing every new request with every prior attempt can add quadratic work; a typed identity index can reduce matching work but creates schema, canonicalization and migration obligations. The correct complexity and memory bounds depend on the eventual representation and must be measured, not assumed.

Prefer unattempted work only under a declared policy. It may delay a transient retry that would immediately unblock the best design. It may also select a low-value tail before a high-value retry. Record those regressions in evaluation. A less repetitive trace is useful only if requirement quality, truthful incompleteness and total cost are at least as good under the intended task distribution.

| Failure mode in a refinement | Required response |
|---|---|
| Key-only cache treats changed arguments as already attempted. | Bind effective invocation and applicability, not only item key. |
| New keys or call IDs reset retry limits. | Separate semantic invocation/requirement identity from transport identity. |
| Successful stale result suppresses a needed reread. | Revalidate source and design applicability. |
| Every failure is permanently suppressed. | Keep bounded, justified retry states. |
| Repeated permission refusal becomes “retry until allowed.” | Preserve ordinary permission and denial handling; no frontier rule grants authority. |
| Unattempted low-value work displaces urgent recovery. | Admit and expose justified priority, then measure the tradeoff. |
| Item reordering skips or duplicates work. | Match state by identity; do not rely only on array offset. |
| A read shared by several objections closes them all automatically. | Reuse the observation only; assess each requirement separately. |
| Frontier entries silently vanish from the model frame. | Preserve complete durable records and an explicit bounded remaining-work view. |
| Novel-attempt count resets no-progress forever. | Keep an actual evidence/assessment progress contract and hard envelope limits. |
| New read lands without a following review or fitting context. | Defer with the correct capacity reason; do not borrow extra work. |
| Interrupted dispatch is assumed not to have happened. | Reconcile the adapter's known outcome before retry. |

No speedup, saved tokens, improved acceptance rate or portability gain is established by this static review.

## 8. Fair evaluation plan, entirely unrun

### A. Deterministic control-flow contract

Use the real conductor and policy with scripted participant replies and measured read adapters. Pin fixture sources, configuration, prompts and catalog. Establish:

1. The exact seven-item witness gives the current repeated-prefix trace.
2. The proposed policy attempts d7 by a later admitted batch or emits its specific justified deferral; it does not silently drop d1–d6.
3. For the equal-eligibility variant, no unchanged failed invocation repeats while an eligible unattempted invocation remains.
4. A source-changed or argument-changed retry is not incorrectly suppressed.
5. A declared transient retry can take priority and is charged against the same allowance.
6. With too few remaining slots or no review capacity, the result is explicit incompleteness, not a budget increase or invented ready outcome.

Use a complete attempt census: requested identities, actual dispatch order, observed result/error, eligibility/retry reason, deferred remainder, frame inclusion and terminal disposition. Only checking the final text or total call count is insufficient.

The existing authored tests cover successful empirical continuation, configured no-progress/round stops, malformed rounds and fingerprint invariance under rewording. Those are useful positive controls. Their inspected definitions do not exercise more critical data items than the probe allowance with a stable failed prefix. No test was run here. [Chat tests][chat-tests] [Unit tests][unit-tests]

### B. Required countercases

| Case | What must be established |
|---|---|
| K−1, K, K+1, and several multiples of K pending items | Correct first-attempt/defer accounting without truncating requirement inventory. |
| Empty output, deterministic missing path, transient timeout, refusal | Outcome status and retry policy remain distinct. |
| Item reorder, insertion, removal, stable key with new arguments | State follows the intended request identity. |
| Same read supports two separate requirements | Avoid unnecessary repeat execution without inventing both requirement resolutions. |
| Source change, permission change, changed tool implementation | Old attempt state does not become current authority or evidence. |
| Success in an early item; useful tail; all probes genuinely blocked | Preserve successful cases and honest stopping. |
| Cap one with two items; no-progress one; no review space | Do not claim unconditional fairness under an impossible envelope. |
| Cancel/persistence failure before and after dispatch | Recover known state without hidden retry or revived work. |
| Windows/Linux paths, unfamiliar repository layouts, remote read adapters | Identity and applicability obey the actual tool semantics. |
| Legitimate high-value retry before low-value new work | Priority is explicit and its benefit/cost is recorded. |

### C. Behavioral value, separate from the mechanical witness

Compare at least:

- Current prefix scheduling.
- A minimal attempt-aware deterministic frontier.
- A prompt-only “reorder deferred probes” baseline, with its prompt tokens and extra reasoning charged.
- Optionally, a richer priority policy, varied separately rather than bundled with stronger models or bigger budgets.

Use identical repository snapshots, original requirements, permitted effects, model/provider versions and effort, JIT sources, candidate/round/context limits, and tool catalogs where feasible. Apply matched **full-cost** budgets, not merely equal conductor probe counts. Counterbalance fixture order and hold out repository/task families and failure layouts.

Include cases where new evidence matters, retrying first is genuinely better, missing information cannot be recovered, and no extra probe is necessary. Independently judge requirement satisfaction and unsupported readiness without seeing the variant label. Report both quality at equal total budget and cost to reach a matched accepted endpoint.

Primary measures should include:

- Share of eligible distinct requests attempted or explicitly deferred.
- Time and total cost to the first useful tail observation.
- Repeated failed-attempt rate, broken down by justified versus unjustified retry.
- Evidence incorporated into an actual subsequent assessment.
- Independently accepted design rate and unsupported-ready rate.
- Honest unresolved outcomes, permission violations, cancellation/recovery correctness.
- Complete provider, host, storage, review and any separately authorized verification costs.

Keep failed, stopped and abandoned runs in the denominator. Cluster uncertainty by repository/task family; repeated seeds on one fixture do not establish broad portability. Do not equate dispatch coverage with solving the design.

### D. Normal production entry paths

After focused evaluation and separate authorization, exercise ordinary `/deliberate` or `/innovate` and the recurse mission design route, with normal world-model/CodeDOM context and installed policy. Use harmless read-only fixtures, including an unfamiliar layout. Record the actual source/configuration identity, available tools, attempts, consumer frame and terminal result.

A successful synthetic callback test does not establish production wiring. A successful design-only run does not establish mutation permissions, gate quality, requirement verification or safe mission retention. Those remain separate endpoints and costs.

## 9. Primary research and its limited role

**Hay, Russell, Tolpin and Shimony, Selecting Computations: Theory and Applications (UAI 2012)** distinguishes choosing which internal computations to perform from the eventual external decision, and studies selection under costs in specified statistical settings. It motivates making probe allocation a separate explicit decision. It does not prove that unattempted-first scheduling is optimal for this conductor or justify importing a bandit algorithm without a fitting model and evaluation. [Primary publication](https://proceedings.mlr.press/r10/hay12a.html)

**Yao et al., ReAct (ICLR 2023)** studies interleaving reasoning and actions so observations inform subsequent plans. That supports checking the concrete observation-to-refinement path here. The paper does not establish fairness for bounded tool batches, this repository's permission model, or the success rate of the proposed frontier. [Primary paper](https://arxiv.org/abs/2210.03629)

**Madaan et al., Self-Refine (NeurIPS 2023)** evaluates iterative generation, feedback and refinement without additional training. It supplies useful context for preserving the current feedback loop and for including a prompt-only baseline. It does not show that repeated rounds with unchanged failed evidence will resolve a requirement, nor that additional critique is worth its cost on this workload. [Primary paper](https://arxiv.org/abs/2303.17651)

The source establishes the dispatch mechanism. These papers motivate explicit computation selection and evidence feedback; they are not closure evidence for this audit.

## 10. Closure criteria

This item should close only when the chosen bounded scheduling contract is explicit, its implementation retains unresolved requirements and attempt identity, and a source-matched test establishes the failed-prefix/unattempted-tail distinction. The ordinary single-probe and configured-stop cases must remain valid. Production-route evidence must show that the new selection reaches its consumer under unchanged permissions and resource limits.

The desired outcome is not “all probes always run.” It is that the executive can account for the unattempted evidence work and does not repeatedly spend its finite allowance on the same failures merely because they occur first in a list.

## Source map

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/cc9dce8e2412a8cb6e688841738e3b212e630378
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/agents.md#L1-L65
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/architecture/AGENTS.md#L1-L116
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L895
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/prompt/atoms/system/deliberation_proposer.yaml
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/prompt/atoms/system/requirements_interrogator.yaml
[records]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/types.go#L54-L115
[items]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/types.go#L26-L59
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/types.go#L144-L181
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/round.go#L10-L72
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/outcome.go#L138-L181
[allow]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/outcome.go#L184-L201
[fingerprint]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/outcome.go#L118-L135
[loop]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/service.go#L96-L165
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/core/defaults/policy/deliberation.mg
[eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/core/defaults/policy/deliberation.mg#L18-L36
[stops]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/core/defaults/policy/deliberation.mg#L38-L40
[call-policy]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/core/defaults/policy/deliberation.mg#L60
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/config/routing.go#L16-L32
[read]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/read_tools.go#L57-L86
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/read_tools.go#L88-L208
[save]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/persistence.go#L24-L56
[note]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/outcome.go#L88-L116
[chat-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/cmd/nerd/chat/deliberation.go#L18-L54
[chat-command]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/cmd/nerd/chat/deliberation.go#L107-L131
[mission-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission-preparation]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/campaign/recurse_mission.go#L212-L286
[mission-admission]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/campaign/recurse_mission.go#L288-L318
[missing-path]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/tools/core/file_ops.go#L75-L135
[file-read]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/tools/core/file_ops.go#L207-L238
[file-list]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/tools/core/file_ops.go#L688-L841
[live]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/.claude/skills/codenerd-dogfood/references/component-ledger.md#L6333-L6342
[chat-tests]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/cmd/nerd/chat/deliberation_test.go#L230-L268
[unit-tests]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/internal/deliberation/deliberation_test.go#L51-L120
[prior-mixed]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/deliberation-mixed-objection-probe-admission.md
[prior-progress]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/deliberation-progress-observation-identity.md
[prior-horizon]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/deliberation-probe-consumption-horizon.md
[prior-capacity]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/deliberation-probe-frame-capacity.md
[prior-read]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/deliberation-read-invocation-identity.md
[prior-retirement]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/deliberation-critical-item-retirement.md
[prior-crucible]: https://github.com/theRebelliousNerd/codenerd/blob/cc9dce8e2412a8cb6e688841738e3b212e630378/Docs/audits/portable-recurse-deliberation-crucible.md


[comparison]: https://github.com/theRebelliousNerd/codenerd/compare/0e792aa7a7ed487418a3ce68b1a18076c0f94457...cc9dce8e2412a8cb6e688841738e3b212e630378
