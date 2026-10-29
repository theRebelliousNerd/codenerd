# Reflection observation publication in the chat interface

The chat interface can complete reflection recall and then report that none has run. The primary defect is publication of an observation, not failure to perform retrieval: ordinary `processInput` assigns the result to its copied `Model`, but does not return that field to the live model through `Update`. Consequently, the idle `/reflection` diagnostic cannot inspect that completed pass. A related background handoff loses the copied reflection fields when it builds a fresh `SessionContext`; that narrower consequence does not establish total context starvation or incorrect task outcomes.

This addendum recommends an immutable, request-attributed reflection receipt published through the existing update boundary. A separate decision should determine whether background jobs carry the request's reflection snapshot or intentionally use another documented context route. Neither change should rerun recall merely to display its status.

## Source scope and relationship to earlier work

Production citations are pinned to [`9dcb01b9736bf4763aa3ac391d29f133d73c6611`][production]. Main was revalidated on 2026-10-29 at 16:38 UTC. The six central files—`reflection.go`, `process.go`, `model_types.go`, `model_update.go`, `commands_handlers_files.go`, and `background.go`—were unchanged across the reviewed active heads `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908`, and QA head `51cfd42649552866d4aa54c53b68acf5740eb09b`.

The existing [knowledge delivery journey][journey] discusses reflection's kernel and legacy session-context carriers. Its broader delivery analysis is not evidence that every current carrier fails. The [chat session ownership audit][sessionaudit] and [background outcome ownership audit][backgroundaudit] cover lifecycle and settlement protections that this finding does not reopen. The [learning lexical candidate audit][lexicalaudit] addresses selection precision rather than publication of `lastReflection`. A bounded corpus search found no exact publication audit beyond inventories; this is not an exhaustive novelty claim.

## The completed observation stays on a model copy

Ordinary submission returns the current model and `m.processInput(input)`. That method has a value receiver and returns a command closure. Its later assignment of `m.lastReflection` therefore updates the closure's copied receiver, not the model retained by the update loop. The fact that `lastReflection` is a pointer field does not change the ownership of that assignment. The ordinary completion message carries several explicit state updates, but no reflection result, and `Update` applies the fields it receives without publishing reflection. This is a concrete copy-and-message chain, not just an inference from a missing search result. [Submission][submit], [command receiver][receiver], [reflection assignment][assignment], [model field][field], [completion message][assistantmsg], [update handling][update].

The observable witness is deliberately narrow: an initialized, normal chat view accepts an ordinary query that reaches reflection, with reflection enabled, an available store, and a query of at least twelve bytes. The turn completes; the user then submits `/reflection` while idle. This excludes pending busy-queue handling, clarification workflows, open approval modals, and early classification outcomes such as hold, approve, or a perception answer. A future deterministic fixture should control the route rather than presume a particular natural-language query guarantees it. [Early routes][early], [reflection guards][guards].

After those guards, `performReflection` creates a state, records duration, attempts embedding retrieval when available, and uses lexical fallback when the ranked result is empty. It returns the state even when no hits are found. Thus a successful zero-hit pass is still an observation that should be distinguishable from never attempting recall. Default enablement does not override the guards or early routes. [Retrieval and return][retrieval], [configuration defaults][defaults].

The slash-command handler renders the live model's state and returns that model without performing recall. With its initially nil `lastReflection` unchanged, the renderer says, “No reflection recall has run yet in this session.” The completed query, method, duration, and hits are therefore unavailable through this diagnostic. This does not mean all telemetry disappears: reflection warnings can be appended to the ordinary turn response, and other logging remains outside this diagnostic claim. [Command dispatch][dispatch], [handler][handler], [renderer][renderer], [warning propagation][assignment].

## The background boundary loses the copied field carrier

A same-turn foreground delegation using the original worker copy can still read its reflection result. This does not describe a later continuation started from the live model after another message handoff. `buildSessionContext` projects nonempty `ContextHits` into `ReflectionHits` and `ExtraContext["reflection_hits"]`. That projection is a field carrier, not an automatic guarantee that every hit reaches a model prompt. [Foreground context construction][foreground], [session projection][projection].

For the reviewed background route, however, `processInput` returns a `backgroundStartMsg` containing the surface response and job, with no reflection snapshot. `Update` starts that job from the live model, whose reflection field was never updated. The background runner then rebuilds its session context from that copy and passes the constructed context to the observed worker. Conditional on the producer having nonempty context hits, those original hits are not carried through these two session-context fields. The background path neither receives the producer's snapshot nor reruns reflection at this boundary. [Producer handoff][handoff], [job and message types][jobtypes], [background update][backgroundupdate], [start and rebuild][backgroundrun].

There is important counterevidence to any stronger conclusion. Reflection also asserts recall facts into the shared kernel. Current prompt-context policy can turn qualified facts into injectable context. That independent route remains available, subject to its qualification, scope, budget, and context availability; it is not proof of universal rescue either. No claim follows here about guaranteed absence of model context, worse answers, blocked completion, permission changes, safety loss, or failed learning. [Kernel assertions][assertions], [current context policy][policy].

## Publish a bounded receipt through Update

Keep recall work outside `Update`, and keep `/reflection` observational. Publish an immutable receipt through the existing message-ownership pattern rather than mutating the live model from a worker goroutine. Include only the diagnostic projection and any deliberately selected, bounded context references; retaining every trace vector or mutable database object would add unnecessary memory and privacy exposure.

The proposed receipt needs selected-session identity or generation, request or turn sequence, the query snapshot, attempt and completion status, method, elapsed time, warnings, and hit coverage. These are proposed semantics, not an existing status enum. A successful zero-hit result must differ from an unattempted, canceled, or failed pass. The display must also choose between “latest attempt” and “latest turn.” Preserving the previous attempt with its date and turn attribution is reasonable when a later turn is disabled, too short, or handled by an early route; a separate skipped-turn indication avoids erasing history and then claiming nothing ever ran.

Acceptance cannot simply require the receipt's turn identifier to equal the currently running owner. The command's turn-end operation occurs before its returned message is handled. Use a completed-sequence policy compatible with that lifecycle, plus session generation and monotonic last-accepted observation ordering. Reject or separately retain late old-session receipts and older out-of-order turns so they cannot replace newer status. Reuse existing session and background ownership protections rather than introducing another global registry. [Command lifecycle][receiver].

A diagnostic-only repair is a valid first milestone. For background continuity, carry a request-owned snapshot in the job or its envelope, or deliberately derive fresh context under the background request and record that choice. Do not read whichever observation happens to be latest in the UI: another turn may have replaced it. Choose between the existing kernel route and an additional field carrier based on actual serving needs, rather than duplicating context blindly. This is not a full JIT delivery fix and grants no new permissions or cross-request context reuse. Persistence is optional and needs an explicit retention contract; it should not imply replay, inference, or private-content logging.

## Deterministic validation before a model study

The first tests should exercise the actual boundary: ordinary `handleSubmit`, its returned command, the resulting message through `Update`, and an idle `/reflection`. Use a controlled recall store with embeddings disabled or a recording engine. Check query, method, duration, zero-hit and populated-hit rendering, while confirming that the same kernel assertions still occur. Assert that completing the command does not itself mutate the live diagnostic before message application.

Cover disabled reflection, unavailable stores, short queries, early held or perception-answer routes, embedding failure with lexical success, empty results, cancellation, and failure. Ownership cases should include late older receipts, session load or reset, duplicate delivery, an intervening newer turn, and a background job originating in another session. Existing clarification, route outcomes, and authority behavior must remain unchanged.

A background test should traverse message creation, `Update`, `startBackground`, and a recording observed task executor. Under the chosen snapshot contract, assert the original request's expected fields, not whatever the UI currently holds. Pair that test with a control showing that qualified kernel facts can still supply context without those fields. This prevents a field-continuity assertion from being misreported as proof of total context loss.

Compare the current implementation, diagnostic publication only, request-owned background snapshots, and an explicitly kernel-only route using identical inputs, configuration, and source snapshots. Measure truthful status agreement, request freshness, wrong-session attribution, stale overwrites, expected field coverage, receipt size, allocation and retention, copying cost, database and embedding calls, and update or prompt latency. Actual prompt coverage and verified task quality are separate outcomes for a later authorized study, with model, token, and tool usage controlled and recorded. Ordinary-turn resource counts should remain unchanged by diagnostic publication; no extra model calls are needed by design. No numerical savings or quality improvement is established here.

This assessment is source-grounded only. No tests, application execution, or CodeNERD inference were performed.

[production]: https://github.com/theRebelliousNerd/codenerd/tree/9dcb01b9736bf4763aa3ac391d29f133d73c6611
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/journeys/03-knowledge-delivery-and-window.md#L253-L257
[sessionaudit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/chat-session-memory-ownership-and-recovery.md
[backgroundaudit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/background-outcome-and-continuation-ownership.md
[lexicalaudit]: https://github.com/theRebelliousNerd/codenerd/blob/fbed2dde9e73eb88f2fe364c5c7a172de83a4037/Docs/audits/learning-lexical-candidate-window.md
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_handlers.go#L197-L201
[receiver]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L48-L54
[assignment]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L367-L374
[field]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_types.go#L373
[assistantmsg]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_types.go#L734-L743
[update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L138-L162
[early]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L331-L354
[guards]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/reflection.go#L35-L56
[retrieval]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/reflection.go#L71-L131
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/config/reflection.go#L52-L59
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L103-L104
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_files.go#L249-L259
[renderer]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/reflection.go#L134-L159
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_session_context.go#L196-L199
[foreground]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L530-L570
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L497-L526
[jobtypes]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/background.go#L31-L138
[backgroundupdate]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L168-L173
[backgroundrun]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/background.go#L175-L235
[assertions]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/reflection.go#L368-L395
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/prompt_context.mg#L5-L83

