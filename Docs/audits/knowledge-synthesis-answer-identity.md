---
doc-class: deep-dive
subsystem: chat-knowledge-synthesis-session-history
implementation-status: planned
last-verified: 2026-10-30
verified-against: fa6877b932322bb6ca75826289366836d442c051
verification-method: static-source-inspection
---

# Preserve answer identity through terminal knowledge synthesis

## Decision and evidence boundary

Carry a chat request's existing identity through the asynchronous knowledge-synthesis command and its result. The current request-ID repair correctly attributes ordinary queued answers, but specialist consultation crosses another boundary: the interim response is attributed to the request, while the later synthesized answer is not. The visible and JSON transcripts can retain the final answer even though the SQLite exchange projection excludes it.

This is a narrow, source-derived residual of the queued-history repair, not a claim that request IDs are absent, that all session history is lost, or that the new terminal grounding carrier is ineffective. A second independent static review challenged the chain and confirmed this narrower boundary. No tests, builds, application runs, model inference, fault injection, or performance measurements were performed. This document is a research draft, not an implementation or reproduced-incident report.

The inspected public main head was [`fa6877b932322bb6ca75826289366836d442c051`][baseline], rechecked unchanged at approximately 06:40 UTC on October 30, 2026. Root and relevant chat, perception and articulation guidance were read. The two currently listed research branches, `research-main-20261030-0544` and `research-main-20261030-0614`, were compared with main: their additional audits address restart recovery, verifier coverage, prompt-evolution cohorts, policy reload, JIT strategy context and recurse-gate contracts. None adds this terminal-answer identity witness. Public repository sources only were used.

## Primary framework reference

[Bubble Tea’s implementation](https://github.com/charmbracelet/bubbletea/blob/v1.3.10/tea.go) defines the model, messages and commands used by this path. The request-ownership requirement is CodeNERD’s own contract; the framework reference does not establish the observed source-level omission by itself.

## Relationship to existing work

The [queued-history audit][queued-audit] identified a positional pairing defect: two submissions could leave the first without an exchange and pair the second with its queue acknowledgement. The current [audit queue][queue] records that repair as fixed by `2aa65dfda`. Credit it. `request_answers.go` now gives user messages request IDs, carries them through ordinary work and stamps answering messages; `sessionTurns` pairs by identity rather than location. Its direct queued-request regression is meaningful. [Request ownership][ownership] [Queued regression][ownership-tests]

The narrower residual here requires **no queue at all**. An owned message schedules another asynchronous answer-producing command, and the ownership wrapper does not follow that command's result. The same general ownership contract was already proposed; this report supplies a distinct current-main production witness missed by the repaired ordinary path, not a competing history architecture.

The [terminal knowledge synthesis audit][grounding-audit] concerns the evidence and obligations delivered to the final answering request. Its as-built section credits `knowledgeParent` for freezing recent conversation, rendered workspace evidence and compressed context; failures, empty results and requests beyond the consultation cap are now represented in synthesis. Those repairs remain intact. This report begins after those inputs have reached synthesis and asks whether the answer it produces remains associated with its originating request.

The [articulation operation ownership audit][operation-audit] covers cancellation and restriction continuity across independently lived contexts. That is adjacent, not necessary to this finding. The witness below uses one session, no stop, no competing foreground work, no session switch and successful calls and persistence. It does not refile streaming cancellation, cross-session publication, prompt budgeting, or compressor lane coverage.

## Current production chain

### 1. The request enters with an identity

`addMessage` calls `attribute`; a user message without an ID receives `m.turn.newRequest()`. Normal submission passes `m.lastRequest()` to `processInput`. The answer association is therefore available before any model call. [Message insertion][insertion] [Submission][submission] [Identity assignment][ownership]

After initial articulation succeeds, chat applies its control packet before consultation. When there are admitted knowledge requests and the re-entrancy guard permits consultation, the stream goroutine sends:

```go
answerTo(request, publishReflection(m.handleKnowledgeRequests(...), ranReflection))
```

Thus the consultation result does carry the original request ID. This is not a missing-ID-at-admission defect. The ordinary nonconsultation answer is also explicitly wrapped, providing a useful negative control. [Control settlement and branch][articulation-return]

### 2. The wrapper covers the interim handler, not its future result

`answerMsg` contains the request ID and the inner message. `applyAnswer` sets `m.answering` while it runs `m.updateCore(inner)`, then restores the prior value on the returned model and returns its command unchanged. Its background-start special case explicitly transfers ownership into `backgroundJob.RequestID`; there is no corresponding transfer for knowledge synthesis. [Ownership wrapper][ownership]

`handleKnowledgeGatheredMsg` adds a nonempty `InterimResponse` to history during that wrapped Update invocation. Since `m.answering` is set at that moment, the interim receives `Answers = R`. The handler then returns a batch containing a persistence command and `m.synthesizeWithKnowledge(...)`. It does not attach the request identity to the future command's result. [Interim and synthesis handoff][knowledge-update]

### 3. Synthesis returns an unattributed terminal message

`synthesizeWithKnowledge` makes its single `CompleteWithSystem` call and returns `assistantMsg{Surface: response}`. It does not call `answerTo`, and neither its parameters nor `knowledgeParent` contain a request ID. [Synthesis][synthesis] [Parent carrier][parent]

The fact that the command closes over a model copy does not implicitly preserve response ownership. The command does not append the answer itself or stamp a message field from that captured copy. It returns a bare message for the live model's later Update invocation. The original `applyAnswer` already restored `m.answering`; under the ordinary single-request schedule its value is zero.

The `assistantMsg` handler calls `pushAssistantMsgWithThought`, which reaches `addMessage` and `attribute`. With no explicit `Answers` and no active `m.answering`, the terminal answer receives `Answers = 0`. It is displayed, increments the normal turn metrics and schedules persistence, but remains unattributed. [Assistant handler][assistant-handler] [Insertion][insertion] [Attribution][ownership]

### 4. The durable exchange excludes that answer

`sessionTurns` first collects assistant content only where `Answers != 0`, then joins messages whose `Answers` match each user's `Request`. `syncSessionToSQLite` writes that projection. Consequently, the request's interim appears in the exchange but its later synthesized answer does not. A subsequent full-history save repeats the same projection; ordered successful saves cannot repair the missing identity. [Projection and writer][projection]

`StoreSessionTurn` accepts the nonempty projected response on upsert; it does not infer a terminal answer from raw history. `QuerySession` later exposes the stored response as the fourth argument of `session_turn`. [Upsert][upsert] [Queryable history][query-session]

This establishes wrong/incomplete **owned queryable history**, not necessarily a wrong future model answer. The inspected continuity rule consumes the user-input column and ignores the response column. Raw JSON history serializes both messages and their ownership fields, so loading that transcript can still recover the final prose. Neither fact makes the SQLite exchange complete. [Continuity rule][continuity] [Raw serialization][serialization]

## Minimal conditional witness

Use a fresh ordinary natural-language request with a nonzero request ID, local SQLite wired, successful normal routing to articulation, and a valid admitted response that requests one specialist. Keep one session throughout and allow persistence to finish in order. No delegated parent task that independently writes the same exchange is needed; a scripted specialist result can isolate the chat projection.

Give the initial surface and final surface distinct sentinels, for example:

| Event | Content | Source-derived ownership |
|---|---|---|
| Accepted user request | The question | `Request = R` |
| Initial articulation | `INTERIM-581: Let me consult the specialist.` | Later appended with `Answers = R` |
| Specialist result | Useful fixed evidence | Consultation data, not the terminal answer |
| Terminal synthesis | `FINAL-ANSWER-742` | Appended with `Answers = 0` |

After terminal Update and persistence, the in-memory/JSON transcript contains `FINAL-ANSWER-742`, but the SQLite response projected for R contains `INTERIM-581` and excludes the final sentinel. The same omission follows from `sessionTurns` before database IO, so it does not depend on an SQLite race or failed transaction.

An empty interim variant yields no owned final answer; it is a supplementary boundary fixture, not the canonical production witness. An empty projected response preserves any response already stored by the upsert, so this variant must not be described as necessarily erasing an existing row. The nonempty admitted interim above is sufficient and avoids relying on empty-surface admission behavior.

## Existing protections and counterexamples

- An ordinary direct terminal answer is wrapped at the original return and keeps its identity; do not describe all articulation as broken.
- Background work has an explicit request-ID handoff in `applyAnswer`; its result ownership is a separate existing control, not evidence that every asynchronous handoff is covered.
- A valid current `knowledgeParent` still delivers the earlier evidence, coverage and conversation to synthesis. The provider can produce a correct, fully grounded final answer and this projection defect still occurs.
- First-phase controls remain applied before consultation and are recorded through the already-applied path. Repairing attribution must not replay them.
- Visible and raw serialized history retain the final answer. No total transcript loss, unauthorized effect, wrong next model answer, or observed production incident is asserted.
- With no local database, the wrong association still exists in the message metadata, but this witness does not claim a persisted SQLite consequence.
- Automatic/test invocations with request ID zero intentionally bypass `answerTo`. They cannot establish preservation of an actual accepted user's nonzero identity.

## Why the inspected tests do not cover the composed boundary

`TestSessionHistory_QueuedRequestsKeepTheirOwnAnswers` drives two directly answered `/converse` requests through admission, Update and SQLite, which supports the queue repair. It does not request specialist knowledge. [Owned-answer regression][ownership-tests]

`TestSynthesizeWithKnowledge_OneLLMCallNoReentry` verifies exactly one synthesis call and explicitly expects a bare `assistantMsg`. `TestKnowledgeSynthesis_KeepsTheTurnsEvidence` uses a real `processInput` evidence route, but passes request ID **zero**, extracts the gathering result and invokes synthesis directly; it inspects the prompt rather than delivering the terminal result through Update and checking its saved exchange. These are useful local tests, not an end-to-end answer-identity witness. None was run for this report. [Synthesis test][synthesis-tests] [Grounding test][grounding-tests]

## Smallest coherent correction

Update the owning chat/session-history specifications first. Carry the existing origin request identity explicitly across the knowledge-synthesis command/result boundary, including success, returned error and recovered panic. Preserve the single synthesis call and current parent-grounding carrier. The result must not depend on the live model's last user message, current request, or temporary Update-local attribution.

Treat `knowledgeGatheredMsg` as a handoff, not proof that the request has received its terminal answer. Preserve useful interim content, but distinguish its role from terminal settlement. Whether an exchange includes both interim and final prose is a presentation/storage contract; in either choice, it must include the correct final result and must not treat the interim alone as completion. Retention and recall decisions remain the policy layer's responsibility; Go only needs to preserve known identity and perform the IO.

Do not repair this with positional pairing, a match on “let me look that up,” a second perception pass, or replay of the original control packet. Do not assign old unattributed records to a request by guessing. If historical repair is desired, do it only from trustworthy retained ownership evidence and otherwise retain explicit legacy/unknown status.

## Proposed evaluation, not executed

First use fixed scripted provider and specialist outcomes with the real normal admission, articulation, consultation, Update, synthesis, persistence and `QuerySession` boundaries. Preserve world-model and CodeDOM context in complete-system fixtures; helper-only constructor tests are supplementary. No model sampling is required to establish the identity transformation.

The primary oracle is exact association, not successful execution of a save: after R's terminal result arrives, its owned exchange contains the terminal sentinel, and no other request acquires it. Check raw history and SQLite separately. Run current main and the narrow correction on identical schedules and payloads, without changing prompts, model, permissions, consultation count or answer quality.

| Fixture | Required observation |
|---|---|
| One owned request, nonempty interim, successful synthesis | Final content belongs to the originating request in both transcript metadata and queryable exchange |
| Ordinary direct answer without consultation | Existing attribution remains unchanged |
| Two queued requests, first consults | First synthesis remains with the first request; second's queue notice and answer remain separate |
| Unrelated side/background output between phases | Display order does not change answer ownership |
| Interim absent/empty where admitted | Final answer does not require an interim message to acquire identity |
| Synthesis fails or is canceled | Interim is not falsely recorded as terminal success; the terminal disposition retains its owner |
| Save failure then authorized retry | Projection can be retried without rerunning consultation or controls |
| Repeated full-history persistence | Final answer remains included, without duplicating owned content merely because of another save |
| No local database | Transcript ownership remains correct; no false persistence claim |
| Legacy request-ID-zero input | No invented ownership |
| Session change during outstanding work | Explicit separate session-owner test; do not substitute it for the single-session witness |

Capture raw `Request`/`Answers` fields, projected rows, the final `QuerySession` facts, provider-call count, and control-effect count. Retain the existing one-call synthesis, evidence-grounding and queue tests. A later separately authorized recall-quality evaluation would be needed before claiming improved downstream answer accuracy.

## Full costs and acceptance

No speedup, token saving or implementation-time estimate has been measured. The narrow repair should require no additional provider call: the request identity already exists. Carrying that identity has small bounded metadata cost, but preserving the final answer adds its real bytes to the durable exchange and potentially to subsequent reads. Counting the current omission as an efficiency benefit would reward losing the user's answer.

Measure command/result bookkeeping, message and snapshot bytes, projection CPU, SQLite upsert/read latency, raw-history and exchange storage, hydration volume, and save retry/diagnostic work. Include the unchanged initial articulation, specialist and terminal-synthesis model/tool costs in whole-request comparisons. If a richer phase/disposition schema is introduced, account separately for schema migration, legacy handling and retention work; do not make a new framework a prerequisite for carrying an already-known ID.

The checkable exit is narrow: a terminal synthesis result is associated with the same accepted request through Update and durable queryable projection, independently of interim text or display interleaving. Existing grounding, one-call synthesis, exactly-once control application and queue protections remain intact. A correct answer on screen is necessary, but it is not proof that the saved exchange contains that answer.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/fa6877b932322bb6ca75826289366836d442c051
[queued-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/Docs/audits/queued-history-projection.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/Docs/audits/ASTRA_QUEUE.md
[grounding-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/Docs/audits/terminal-knowledge-synthesis-grounding.md
[operation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/Docs/audits/articulation-operation-ownership.md
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/request_answers.go#L24-L78
[ownership-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/request_answers_test.go#L45-L124
[insertion]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/model_handlers.go#L26-L73
[submission]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/model_handlers.go#L150-L195
[articulation-return]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/process.go#L1070-L1122
[knowledge-update]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/model_update.go#L987-L1035
[synthesis]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/process_knowledge.go#L228-L267
[parent]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/process_knowledge.go#L38-L58
[assistant-handler]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/model_update.go#L144-L164
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/session_persistence.go#L179-L226
[upsert]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/store/local_session.go#L80-L126
[query-session]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/core/virtual_store_predicates.go#L354-L383
[continuity]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/core/defaults/policy/knowledge.mg#L76-L80
[serialization]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/session_persistence.go#L70-L86
[synthesis-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/knowledge_synthesis_test.go#L13-L65
[grounding-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/knowledge_grounding_test.go#L19-L63
