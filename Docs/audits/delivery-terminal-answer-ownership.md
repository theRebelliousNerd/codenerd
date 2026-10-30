---
doc-class: deep-dive
subsystem: cli
implementation-status: proposed-continuation
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
continues: Docs/audits/knowledge-synthesis-answer-identity.md
scope: request-delivery continuation results to owned session-history projection
---

# Delivery terminal results must answer the request that started the chain

## Decision

Continue the existing answer-identity work at a different asynchronous producer: a foreground request that becomes a delivery continuation. Its initial verified surface is correctly attributed to the user request, but the later continuation's completion, failure or unverified summary is not. The summary remains visible and can be saved in raw JSON history while the SQLite exchange projection excludes it.

Carry the originating request's existing identity through the continuation lifecycle and wrap its actual results before Update handles them. Preserve the safe projection's refusal to guess ownership. Do not repair this by pairing replies positionally, inferring an owner from the latest user message, or treating every message emitted while a chain is active as its answer.

**Evidence boundary:** public main was re-resolved at approximately 08:11 UTC on 2026-10-30 to [388205138][main]. This is static source inspection plus an independent source challenge. No CodeNERD program, test, model inference, failure injection or benchmark was run. The witness below is a proposed deterministic fixture, not an observed production incident. No implementation code was changed during this review.

## Preserve the repairs already present

Commit [3d5699872][synthesis-fix] fixes the knowledge-synthesis producer: `applyAnswer` transfers the enclosing request into `knowledgeGatheredMsg`, synthesis carries it to a typed terminal result, and terminal errors clear consultation state rather than being reinterpreted as clarification instructions. The [existing audit's as-built section][synthesis-audit] describes that implementation and its evidence limits. This finding does not reopen that fixed producer.

Other controls are correct and necessary:

- Ordinary queued answers retain request IDs; background jobs explicitly receive their originating request. `applyAnswer` supplies temporary Update-local attribution and restores the previous value afterward. Restoration prevents unrelated later output from inheriting an old request. [Answer ownership][answer-owner]
- `sessionTurns` includes only assistant messages with explicit nonzero `Answers`. An unowned message is not assigned to the nearest user message. [Safe projection][projection]
- Continuation outcomes distinguish completed, unverified, failed and interrupted; their zero value is unverified. The ledger retains earlier unverified work and does not infer success merely because no step remains. [Message contract][messages] [Ledger][ledger]
- Terminal continuation handling explicitly schedules a save, including failure and unverified work. The persistence command snapshots history off the event loop. [Terminal save][terminal-save] [Snapshot][snapshot]
- Raw JSON history preserves message content and ownership fields, including unowned messages. The established defect is narrower than disappearing UI output or total transcript loss. [JSON serialization][json-history]

The residual lies before these consumers: the continuation producer does not supply the owner they correctly require.

## Production trace

### Owned first response, unowned asynchronous successor

Normal `processInput` receives the user request ID and uses a deferred `answerTo` on the returned message. On the verified mutation route, it requires a successful verifier result. It then opens request delivery and checks whether another step is needed. When a next step exists, it returns `continuationInitMsg` containing the completed first surface, first worker result, next step and delivery chain. [Original ownership][process-owner] [Verified handoff][verified-handoff]

`applyAnswer` handles that message with `m.answering = Request`, so `handleContinuationInitMsg`'s first `pushAssistantMsg` receives the correct owner. The handler then calls `offerOrDispatch`, which can schedule `executeSubtask`. After this handler returns, `applyAnswer` restores `m.answering` on the live Model. Its special handoff cases cover background and consultation, not `continuationInitMsg`. [Ownership restoration][answer-owner] [Initialization][continuation-init] [Step scheduling][step-scheduling]

The scheduled command can capture a Model value while attribution is set. That alone does not transport attribution back to the live Model: its returned message must carry it. `executeSubtask` returns a bare `continuationDoneMsg` on error, a bare `continueMsg` when more work derives, or a bare terminal message otherwise. It does not wrap these returns with the original request's `answerMsg`. [Actual producers][producer]

None of `continueMsg`, `continuationInitMsg`, `continuationDoneMsg` or `Subtask` has an answer-owner field. `deliveryChain.request` is a separate generated Mangle delivery identity, not the user's `Message.Request` ID, and the chain has no mapping back to that request ID. It must not be cast or guessed into one. [Message fields][messages] [Subtask fields][subtask] [Delivery identity][delivery-start]

### The terminal outcome is rendered and saved, but not in its exchange

When the bare terminal message arrives in an otherwise ordinary session with `m.answering == 0`, `handleContinuationDone` adds its failure, interruption, verified or unverified summary. `attribute` fills absent `Answers` from that zero value. The terminal branch then requests persistence. [Attribution][answer-owner] [Terminal rendering][ledger] [Save scheduling][terminal-save]

The save serializes all messages into JSON and separately synchronizes the exchange projection to SQLite. `sessionTurns` groups only messages whose `Answers` matches the user's nonzero `Request`. The initial surface qualifies; the unowned continuation summary does not. `StoreSessionTurn` faithfully writes the supplied nonempty response and preserves an existing response on an empty update; it cannot infer the missing ownership. `QuerySession` returns the stored response as the fourth `session_turn` argument. [Dual persistence][dual-save] [Projection][projection] [Upsert][upsert] [Queryable result][query]

Repeatedly saving the same history does not recover that association. Fixing storage by guessing would regress the earlier queued-history repair.

## Exact conditional witness

Use a single ordinary user request with a nonzero assigned ID, an initially idle model and no session switch. No queue, side turn, overlap, crash or save reordering is necessary.

1. The request asks for a code option and its README documentation. It is routed to verified foreground delegation rather than consultation or an approved dream plan. The first worker returns a nonempty report and an accepted verifier verdict after implementing the option but omitting the documentation.
2. Request delivery is enabled: a kernel and Jev client exist, instruction coverage is on, and fragments are present. No ordinary pending obligation is selected ahead of the delivery check. The judge identifies documentation as requested and missing, not unsettled, cut evidence, a blocker or a stalled round. The kernel derives a next step.
3. `processInput` returns its owned `continuationInitMsg`. Update attributes the initial surface to request R, then dispatches the gap step in yolo mode. Propose/explicit mode can also reach the next step after the person accepts the offered work, but is unnecessary for this witness.
4. The gap worker returns an error containing a distinctive public sentinel, such as `documentation fixture could not complete`. `executeSubtask` creates a bare failed `continuationDoneMsg`; the live Model has restored its Update-local attribution to zero.
5. Update displays the failure and schedules persistence. With a configured local DB, an open selected session, usable workspace/session paths and successful save operations, raw history and JSON contain the failure. R's SQLite response contains its attributed initial surface but not the failure sentinel. `QuerySession` exposes that same incomplete exchange.

The existing [delivery journey test][delivery-test] exercises the “first result implements option, continuation documents README” route in yolo mode. It supports the intended route, not proof that this review ran it or that any live model produced the witness.

**Established consequence:** a persisted/queryable exchange can omit the request's later terminal outcome despite that outcome being visibly reported and explicitly saved. The first saved surface is not necessarily false about the first worker's work. The problem is that the exchange lacks the later qualification. Downstream misleading recall or bad decisions remain conditional; no answer-quality degradation rate is established.

## Limits, conditions and safeguards

Delivery is not unconditional. `startDelivery` returns nil without the necessary kernel/client/configuration; the policy declines pushback for blocking or stalled rounds, and cut evidence cannot support “missing.” Existing pending obligations take priority. A verifier can reject incomplete work before this handoff. These guards can make a proposed input fail to reach the witness; retain them. [Delivery construction][delivery-start] [Selection and offer][delivery-selection] [Delivery policy][delivery-policy]

Terminal rendering remains honest: errors stop the chain; failed/unverified work is not automatically changed to verified completion. Tool permissions, human approval, typed restrictions, cancellation and worker verification are independent of the ownership issue and remain necessary. The prior [delivery-restriction draft][restriction-draft] concerns enforcement input, not answer attribution. Fixing one field does not fix the other.

Persistence is conditional. No local DB means no SQLite projection; an inactive session skips its deferred save; write failures are logged or reported under the existing contracts. The primary witness assumes successful persistence and a stable session so these independent boundaries are not confused with deterministic exclusion from `sessionTurns`. [Save preconditions][save-preconditions]

Do not claim:
- that final text is absent from the visible transcript or JSON history;
- that the database silently changes a correctly supplied response;
- that request ownership itself authorizes another step;
- that every automatic task must have a user-request owner;
- or that a continued request's final kernel verdict is necessarily incorrect.

## Why current tests do not close this producer

The terminal persistence test includes continuation success, unverified, failed and interrupted cases. Crucially, it calls:

```go
m.Update(answerMsg{Request: request, Msg: tc.msg})
```

The test supplies the missing wrapper itself. It correctly demonstrates that rendering, attribution and persistence work **when a terminal owner is supplied**; it does not show that the real `executeSubtask` producer supplies one. Keep this useful consumer test rather than weakening its assertions. [Cases and injected owner][persist-test]

The new knowledge-synthesis identity tests cover their repaired producer, including terminal failure and save retry. The ordinary request-answer tests protect queued association and unknown-owner exclusion. A passing result in either family would not establish continuation producer coverage. This review inspected test definitions only; it makes no fresh pass/fail claim. [Knowledge tests][knowledge-tests] [Request tests][request-tests]

## Proposed contract

Treat this as another explicit exit under the existing answer-identity capability, not a competing history architecture.

1. **Capture the owner at the handoff.** When an owned request creates a continuation, bind the originating request ID to that chain before leaving Update. Keep it separate from the generated delivery/task identity. Use one authoritative immutable owner, with the same origin-session semantics the existing request work requires.
2. **Carry ownership through the real producer.** Every request-contributing result from `executeSubtask`—next-step progress, success, unverified completion, error and interruption—must reach Update with that binding. Reuse `answerTo` or an equivalent explicit typed contract. Merely adding a field without using it at the returned message is insufficient.
3. **Preserve isolation.** Keep `applyAnswer`'s scoped restoration. Do not leave `m.answering` set across asynchronous work or assign `lastRequest()` at completion. A queued request, side reply or unrelated status must not acquire this chain's owner.
4. **Retain ownership through pauses.** Offered work and the resumption command must refer to the same request owner even if other user messages arrive. If a new request supersedes the chain, record that transition rather than silently rebinding the old result. Ownerless standalone/legacy work remains explicitly ownerless; do not fabricate an origin.
5. **Attribute only the chain's answer.** The ledger can finish one request and offer a paused approved plan. Do not blanket-wrap unrelated plan notices or a successor plan's execution as the just-finished request's answer. Define the message boundary for each contribution instead of relying on all output from a broad handler.
6. **Preserve outcomes and single application.** Ownership changes must not replay controls, rerun perception, rerun the worker, turn an error into clarification, or increment success merely to create an answer row. Keep exact safety and completion semantics unchanged.

No historical ownership repair is implied. Existing unattributed records can be repaired only from trustworthy retained identity evidence; otherwise leave their status explicit.

## Costs and fair comparison

The narrow repair needs no extra model or embedding call, no new permission mechanism and no change to SQLite's response-upsert semantics. It does add owner transport and lifecycle handling across messages, offered steps, termination and possible resumption. If an owner is added to durable queued state, count the version/migration and compatibility work; do not assume every older saved item has one.

Correct exchange projection increases stored answer bytes by retaining real terminal/progress content. That is not a storage regression to hide by removing the initial surface or silently replacing useful context. Measure duplicate content, bytes per exchange, persistence time, allocations and human recovery turns as well as exact association. Do not claim a latency or quality improvement without measurement.

Compare current main and a narrowly corrected producer under identical scripted outputs, routing policy, work mode, model configuration and workspace state. Count all provider calls and commands. A patch that disables continuation, discards interim output or guesses positional pairing cannot pass merely because it avoids the missing association.

## End-to-end evaluation: proposed, not run

Drive normal user admission, `processInput`, the returned Update command, actual `executeSubtask`, terminal Update and its scheduled persistence. Use deterministic providers/executors and the real projection/store. **Do not manually wrap the continuation terminal result in the test.** Record the admitted request ID, raw returned message type/owner, history `Answers`, JSON and SQLite/query results.

| Case | Required outcome |
|---|---|
| Accepted first attempt → missing README → failed continuation | Failure sentinel appears in R's exchange as well as raw/JSON history; no successful terminal claim |
| Same route, successful or unverified continuation | Correct terminal sentinel and status association; interim surface retained |
| More than one continuation round | Every contributing result has the same request owner; no missing final qualification |
| Propose/explicit pause and later Enter | Resume retains the offered chain's owner, not the newest request |
| Unrelated queued request or side message | No terminal result or notice crosses request ownership |
| Chain ends while an approved plan remains paused | Finishing request and plan offer/execution retain distinct ownership |
| Interruption before dispatch and cancellation/error during work | Correct request owns the terminal/held result; no rerun to obtain an owner |
| Unknown/zero owner and standalone automatic task | Explicitly unowned; no position-based reconstruction |
| No DB, failed save and retry of the same snapshot | Honest persistence status; retry does not rerun the worker or change attribution |
| Existing knowledge, background and queued-answer controls | Landed ownership repairs remain intact |
| Repeated successful saves | Same complete exchange, without duplicate contributions caused by saving alone |

The primary oracle is exact association, not “the save command ran.” The observed exchange for R must include the terminal sentinel actually returned by its producer and exclude unrelated messages. Raw transcript, JSON, SQLite and `QuerySession` are separate assertions.

**Acceptance:** the real delivery continuation supplies its original request owner for every answer-producing exit, and successful persistence preserves its terminal outcome in that request's exchange without weakening safe projection, lifecycle, approval or completion rules. This closes the delivery producer only; session-switch races, save ordering, historical migration and other asynchronous producers require their own evidence.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[synthesis-fix]: https://github.com/theRebelliousNerd/codenerd/commit/3d56998728af0182a5d44ade310d57f1ce4a4885
[synthesis-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-synthesis-answer-identity.md#as-built
[answer-owner]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_answers.go#L24-L86
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L177-L226
[messages]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_types.go#L853-L888
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/continuation_ledger.go#L144-L244
[terminal-save]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L423-L440
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L1171-L1201
[json-history]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L75-L90
[process-owner]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L71-L89
[verified-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L599-L686
[continuation-init]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L1083-L1105
[step-scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/continuation_ledger.go#L63-L98
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_continuation.go#L44-L120
[subtask]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_types.go#L171-L177
[delivery-start]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L29-L69
[dual-save]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L117-L126
[upsert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_session.go#L80-L126
[query]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store_predicates.go#L354-L383
[delivery-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery_test.go#L143-L182
[delivery-selection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L110-L286
[delivery-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/request_delivery.mg#L45-L115
[restriction-draft]: https://github.com/theRebelliousNerd/codenerd/blob/136e02f0a38897a40931747b1e56a6f0c14feb19/Docs/audits/delivery-continuation-restriction.md
[save-preconditions]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L29-L51
[persist-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update_persist_test.go#L189-L234
[knowledge-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/knowledge_answer_identity_test.go#L191-L360
[request-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_answers_test.go#L49-L119

