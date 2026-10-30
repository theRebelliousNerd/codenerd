# Automatic correction learning must follow the answer's request owner

## Finding and decision

The TUI already records which request each answer belongs to. Automatic correction learning still reconstructs that relationship from display position. A background answer to request A can arrive after request B was answered; when the user challenges A's newly displayed answer, the correction input labels B as its `previous_request`. If the correction is admitted, the automatic learner also receives B as the request to learn from.

This is a **consumer-side answer-ownership residual**, not a claim that background delivery or SQLite pairing is still broken. The concrete mismatch is established at main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391). It can occur in one healthy session, without cancellation, a failed save, a session switch, malformed model output, or simultaneous foreground execution.

The narrow repair is to resolve the selected answer through its existing request identity, then carry that immutable association through correction judgment and any admitted learning. Preserve Mangle's correction decision, Jev's negative and uncertainty controls, independent verb-choice gates, and the user's ordinary turn. Unresolved ownership must not silently become the nearest user message.

Static inspection on October 30, 2026 covered latest main, scoped guidance, active branches, and audit inventories through `research-main-20261030-1346`. The schedule received an independent source challenge. No application execution, tests, model inference, benchmarks, or repository writes were performed; this is not an observed production incident.

## Credit the repairs already present

The [queued-history audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/queued-history-projection.md) owns request/answer association in persisted conversation. Current code assigns `Message.Request` to user messages and `Message.Answers` to responses. Background-start acknowledgement is deliberately not the terminal answer: its job receives the request ID, and the finished report receives that owner on delivery. `sessionTurns` groups by those identities rather than adjacency. Preserve all of this. [Identity contract and stamping](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_answers.go#L7-L86), [background completion attribution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L192-L202), [SQLite projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L199-L226).

The [manual-learning audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/manual-learning-evidence-conservation.md) already explains why UI messages and notices cannot simply be relabeled as completed exchanges. Its concrete consumer is the manual `/learn` critic and its other finding concerns durable candidate payloads. This continuation instead concerns the newer `lastExchange` projection used by automatic correction judgment and automatic verb learning; no `/learn` command or candidate confirmation is involved.

The [learned-correction integrity audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/learned-correction-integrity-lifecycle.md) owns qualifier transport, context propagation, persistence outcomes, and worker lifecycle. Those safeguards cannot repair a request chosen incorrectly before persistence. The [background ownership audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/background-outcome-and-continuation-ownership.md) owns job settlement, origin sessions, and successor obligations; the witness uses their ordinary successful same-session route.

## The source mismatch

### The correction input promises an actual exchange

`CorrectionInput` defines `PreviousRequest` as the user message answered by `LastReply`. `CorrectionRequest` sends them as `previous_request` and `codenerd_reply` alongside the new `message`. The question asks whether the user says something in that reply was wrong. Thus the request is not just arbitrary nearby context; the producer assigns it a specific relationship. [Input and question contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/correction.go#L17-L57).

`lastExchange` first skips the trailing current user message when its text equals the current input. It then chooses the latest nonempty assistant message and searches backward for the nearest user message. It never reads the selected assistant's `Answers` or the user's `Request`. `startCorrectionJudgment` immediately supplies this reconstruction to `JudgeCorrection`. [Actual producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/jev_correction.go#L33-L80).

### Ordinary background work makes display order differ from ownership

The routing rule permits a delegated `/query` to run in the background. The processing branch creates the job and returns a `backgroundStartMsg`; its UI handler clears loading and starts the job, leaving the conversation open. A later foreground question can therefore finish before that job's result arrives. This is intended concurrency, not an execution-slot violation. [Background rule](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/routing_arbitration.mg#L311-L317), [job construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L559-L588), [idle chat handoff](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L179-L187).

The origin request ID is carried correctly through `applyAnswer` into the job. When a same-session job completes, the update handler temporarily sets `m.answering` to that job's request ID. Successful completion without a follow-up appends the report through `pushAssistantMsg`, and `addMessage` attributes it. The late report can therefore be the latest assistant message while correctly retaining `Answers=A`. [Job ownership](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_answers.go#L43-L69), [completion and report](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/background.go#L251-L277), [message attribution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L28-L54).

### The wrong subject can proceed to automatic learning

For a normal, non-reused intent, `processInput` starts correction judgment beside perception. After perception succeeds, it records the correction evidence and asks the kernel whether learning is warranted. If so, it calls `learnFromCorrection(lastExchange(...).PreviousRequest, source)`. The same positional reconstruction therefore determines both the labeled judgment input and the request passed onward. [Production call sites](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L226-L274).

The downstream distinction is important: `chooseLearnedMapping` asks Jev to classify **that request's own text**. It does not receive A's reply or the correction C, and does not transfer A's verb onto B. If both verb fit and choice confidence meet the configured threshold, it constructs a learned exemplar for B and attempts persistence. This can be unnecessary learning attributed to the wrong correction and a missed opportunity to learn from A on this pass, even when the selected verb for B is perfectly correct. [Learning, gates, and payload](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/learning_auto.go#L28-L85).

## Controlled normal-entry schedule

Use a live same-session TUI with ordinary dependencies and no pending dream execution, clarification, or held-intent reuse. Assume A is admitted as delegated query review and B as an ordinary foreground explanation. Model classification is a stated route condition, not a guaranteed interpretation of the example wording.

1. User A asks for a review of `auth.py`. The job is started with A's request ID and no automatic follow-up. Keep it pending while the chat is open.
2. User B asks, “Explain what exponential backoff means.” Let the ordinary B answer finish while A is still pending.
3. Allow A's review report to arrive. Its history entry is correctly labeled `Answers=A`.
4. User C says, “Your review of auth.py got that wrong; the guard is on line 12.” This text avoids the existing dream-confirmation/correction phrases and follows normal input processing. Assume perception succeeds and no unrelated assistant message intervenes. [Normal submit and dream checks](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L109-L195), [dream trigger vocabulary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L257-L282).

The relevant ledger order and ownership are:

| Message | Recorded owner |
| --- | --- |
| A review request | `Request=A` |
| Background start acknowledgement | Not A's terminal answer |
| B explanation request | `Request=B` |
| B explanation answer | `Answers=B` |
| Late A review report | `Answers=A` |
| C challenge | `Request=C` |

After skipping C, `lastExchange` selects the late A report but binds it to B, the nearest preceding user. The resulting correction question contains `previous_request=B`, `codenerd_reply=A report`, and `message=C`. An ownership-based join would bind the report to A. This deterministic input discrepancy does not depend on Jev's subsequent answer.

For the learning consequence, additionally require correction judgment at or above the configured yes threshold, a successful kernel decision, a configured Jev client, and an accepted verb choice for B. Durable learning additionally requires an available taxonomy and successful persistence. These are separate conditions, not observations from this audit. Other learning mechanisms may eventually process A; the claim is limited to this automatic correction pass.

There is also a narrower fallback variant that removes dependence on a positive correction judgment: retain a configured Jev client, turn `correction_judgments` off, and use C = `wrong` immediately after the late A report. `startCorrectionJudgment` returns nil, `IsRebuke` recognizes that one-word complaint, and ordinary successful perception reaches the kernel's `learning_correction(/rebuke_phrase)` rule. Under the same healthy kernel and normal-entry conditions, the wrong subject B reaches automatic verb choice without any correction-judge decision. Turning off correction judgments does **not** turn off the independently configured Jev client needed by `ChooseVerb`. Fit/confidence, context, taxonomy, and persistence gates still apply; this is not guaranteed durable learning. The fallback variant sends no correction question, so its precise consequence is the wrong learning subject rather than a mislabeled question. [Switch and fallback evidence](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/jev_correction.go#L33-L42), [recording and kernel read](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/jev_correction.go#L85-L115), [short-complaint detector](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/perception/rebuke.go#L8-L36).

## Protections and uncertainty

- Jev can notice inconsistencies in the report's task label and the user's explicit reference, and return no or uncertainty. A judgment below yes prevents correction learning even when a rebuke phrase exists. With no judgment, the kernel may instead use its rebuke fallback. None of those branches rewrites the already-mislabeled request association. [Kernel truth table](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/jev_learning.mg#L25-L43).
- The correction question is skipped without a last reply; configuration can disable it, its context is bounded, and a failed call is not treated as a negative answer. Automatic verb learning separately refuses empty input, absent Jev, failed choice, weak fit, or low confidence. [Judgment guard](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/correction.go#L59-L80), [learning controls](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/learning_auto.go#L44-L85).
- Learning runs beside the user's turn rather than replacing it with a confirmation dialog or demanding a retry. The current request and full available conversation still reach ordinary perception. A wrong learning subject does not establish that the immediate answer is wrong, a later classifier is corrupted, or any tool permission is bypassed.
- Background reports retain the job/task label, successful raw worker output is stored for follow-up, and session ownership/duplicate settlement remain intact. The evidence is available; this particular consumer does not use its relationship fields.

## Proposed contract and implementation tradeoffs

Resolve one immutable correction subject at admission: selected answer identity, answer-owning request ID, original request text, visible reply text or bounded evidence reference, current correction, and ownership disposition. Use the same subject for the question and any subsequent learning; do not independently rediscover its request after judgment. The existing IDs are enough to fix this same-session witness without a new database or another model call.

For an owned answer, join `Answers` to `Request`. Do not change “last displayed answer” selection into “last user request's answer”: the user can legitimately challenge a late report. Multiple answer parts, side replies, and explicit references to older answers need declared selection semantics. Retain provenance and uncertainty rather than pretending one adjacency heuristic solves all reference resolution.

For unowned notices, imported legacy messages, missing origins, or ambiguous references, preserve the material as unpaired context and withhold automatic request-specific learning unless identity is resolved. Whether to ask the user, omit only `previous_request`, or classify a correction without learning is a policy choice. Do not silently guess a nearest request or discard valid background replies merely because they arrived late. Ordinary alternating exchanges must continue to work.

Carry the admitted subject into learning provenance, so diagnostics can distinguish the corrected answer owner from the new correction request. A successful choice still teaches only the chosen subject's verb under the existing gates. Keep source facts distinct from model judgments; joining IDs does not establish that the complaint is true or that a new exemplar is useful.

Costs include an identity lookup/index or bounded ledger scan, immutable subject storage, legacy and multi-part semantics, provenance plumbing, and cross-route tests. Correct A text may be longer than incorrectly selected B text, affecting judgment or verb-choice cost. Withholding unresolved learning can reduce automatic learning coverage; asking for disambiguation adds friction. Conversely, avoiding unrelated learning can save model/embedding/storage work. None of those net effects has been measured, and the repair should not add a new judge merely to join existing identifiers.

## Fair evaluation, not run

First exercise normal entry, not a manually populated `CorrectionInput`: start a real routed background review with a controlled executor barrier, complete a foreground B, release A, and submit C. Capture actual `Request`/`Answers` fields, the correction question state, the verb-choice request, and any persistence payload. Assert the existing SQLite projection remains correct while the correction consumer also binds A. This isolates ownership without assuming a live provider's classification or probability.

Use paired conditions with fixed returned judgments: yes, no, unsettled, failed/off correction judgment with appropriate fallback, and accepted/rejected verb choices. Verify B is never taught as A's correction subject, and that the user's C turn still runs normally. Then evaluate live correction recognition and held-out learned-intent quality separately; a correct identity join is not proof of improved model accuracy.

Include the explicit fallback schedule with `correction_judgments` off, Jev still configured, and C = `wrong`. Capture that no correction question is sent but the verb-choice request is still A, not B. Pair it with a missing-Jev control and low-fit/low-confidence choices to show that correct attribution does not weaken refusal controls or guarantee persistence.

Cover ordinary alternating exchanges; A arriving before versus after B; two background jobs completing in reverse order; queued messages; multi-part owned answers; unowned status notices; missing/legacy IDs; session navigation; duplicate completion; and correction during active work. Preserve refusal and lifecycle controls. The existing `TestLastExchange` uses an alternating unowned fixture; correction truth-table tests and identity-based session tests cover useful pieces, not this composition. [Current correction tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/jev_correction_test.go), [answer-ownership tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_answers_test.go).

Measure wrong-subject question/learning counts, unresolved ownership, unnecessary persistence, missed eligible correction opportunities, false learned mappings, model/embedding requests and tokens, retained bytes, immediate-answer quality, later task correctness, latency, and total cost per verified successful task. Keep models, authority, ordinary context, and verification obligations fixed. Do not count an extra valid but unrelated exemplar as successful correction learning.

**Acceptance:** an owned late answer remains paired with its actual originating request in correction judgment and any admitted learning, while negative judgments, unknown ownership, normal turn execution, and existing session/background safeguards remain truthful.

## Branch and scope boundary

The `lastExchange` body was identical on the observed three lane heads (`a53cfc30`, `80af1863`, `aa7988de`) and two QA heads (`51cfd426`, `767a3291`). This is a scoped comparison, not whole-branch equivalence. The proposal extends the existing answer-ownership and learning-evidence work; it does not claim a novel identity principle or a reproduced learning-poisoning incident.
