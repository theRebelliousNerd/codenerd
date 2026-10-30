# Dream confirmation must not consume an unrelated request

## Finding and scope

After normal TUI initialization, an idle user can ask **“How do I correct this error?”** without having run a dream consultation. The submission path recognizes the whole word `correct` as dream-learning confirmation, calls the collector with no staged learning, prints a no-pending-learning notice, and returns without scheduling ordinary perception. The supported defect is a request-disposition error before perception, not a model misclassification: a question is consumed by a workflow that has nothing to confirm.

This audit is pinned to public main commit [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), rechecked on 2026-10-30 at 09:12 UTC. It is static source analysis. No application, model inference, build, or test was run. The witness below is derived from reachable branches, not an observed user incident; frequency, downstream task impact, and repair performance remain unmeasured.

The minimal repair is to let ordinary input continue when no applicable dream learning is pending. That is necessary but not sufficient for the broader contract. When candidates are pending, a question containing `correct` still is not consent to learn them. Confirmation should refer to a currently offered, identifiable candidate set and represent an affirmative decision about that set. Availability, intent, and offer identity are separate checks.

## Relationship to earlier work

This is an upstream companion to the [manual-learning evidence audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/manual-learning-evidence-conservation.md), which concerns critic-input association and preservation of proposed fields through candidate confirmation. The [learned-correction integrity audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/learned-correction-integrity-lifecycle.md#as-built-2026-10-28-branch-featconversation-brief) records repairs to qualifier transport, operation context, worker lifetime, and persistence outcomes. Those controls should remain intact. None is an input-dispatch guard for this dream-specific path.

The finding does not reopen the recent delivery-restriction, delivery-answer-ownership, classification-root-identity, or dream-plan foreground-slot reports. Here no worker, continuation, or classification result is needed: the empty confirmation handler ends the submission before `processInput` is scheduled. The learning-confirmation branch and plan-execution branch are also distinct in source; the latter already requires `HasPendingPlan()`. [Submission branches](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L173-L195).

## Reachable production path

### Normal initialization creates an empty collector

The successful component-wiring path constructs `m.dreamCollector = core.NewDreamLearningCollector()`. It separately obtains the router and plan manager from the existing dream subsystem when available. Collector existence therefore does not mean a dream consultation has occurred, that a learning offer was displayed, or that anything is pending. The constructor initializes an empty `staged` map. [TUI wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L787-L800), [collector state and constructor](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/dream_learning.go#L89-L106).

### Idle Enter reaches the dispatcher

In Chat View, an ordinary Enter reaches `handleSubmit` when loading is false and the foreground slot is empty. Clarification input has its own earlier dispatch; Alt+Enter, bracketed paste, and a focused logic pane have separate handling. Within `handleSubmit`, patch mode, slash commands, and wizard modes also have earlier exits. The witness uses none of them. [Keyboard entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_key_handler.go#L232-L265), [submission modes](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L109-L174).

### A lexical match takes ownership before perception

After recording the user message and clearing the input, `handleSubmit` tests only `m.dreamCollector != nil && isDreamConfirmation(input)`. A true result immediately returns `handleDreamLearningConfirmation(input)`. The call to `m.processInput(input, m.lastRequest())` is later in the function and is not reached on that branch. There is no pending-learning or active-offer predicate in this admission decision. [Dispatch and fallthrough](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L134-L195).

`isDreamConfirmation` includes the single-word trigger `correct`. `matchesAnyTrigger` lowercases and trims the input, then matches single-word triggers with `\b` boundaries anywhere in the text. Thus the `correct` in the witness matches as a complete word. Nothing in that helper distinguishes a question, quotation, negation, task instruction, or response to a learning offer. [Matcher and trigger set](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L224-L263).

### Empty confirmation terminates the request

The handler calls `ConfirmLearnings(input)`. That method iterates the staged map; with the normally initialized empty map, it returns no confirmed entries. The handler then emits “No pending learnings to confirm,” sets loading false, and returns a nil command. It does not fall back to normal perception or retain a command that will answer the question. [Empty handler branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L577-L594), [collector confirmation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/dream_learning.go#L335-L375).

The router is invoked only after the nonempty-confirmation branch. Consequently, this witness does not depend on a configured persistence destination or on successful learning writes. It establishes that ordinary request processing is not scheduled, not that any file was changed, a permission check was bypassed, or incorrect knowledge was persisted. [Routing after the guard](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L583-L597).

## Conditions and bounded impact

The complete minimal witness is:

1. Normal successful TUI component wiring has initialized the collector.
2. The user is in ordinary Chat View, not a wizard, patch, or clarification flow.
3. No request is loading and no foreground turn remains in flight.
4. No dream learning is staged; a fresh session is sufficient.
5. The user submits `How do I correct this error?` with ordinary Enter.

The expected result from source is the no-pending-learning branch and no scheduled `processInput` for that submission. An actual normal-entry regression is still needed to verify the complete event/message sequence. The user can recover by rephrasing; the finding is not a permanent application deadlock. Its cost is a lost opportunity to answer the accepted question, a misleading workflow response, and potential retry effort. No incidence rate is inferred from the commonness of the word.

The route matters. Queued ordinary messages are later scheduled directly through `processInput(next.input, next.request)`, rather than replaying this idle `handleSubmit` decision. That makes idle-versus-queued behavior a useful comparison, not proof that every busy submission has identical behavior: busy side-turn admission has its own decisions. [Queue draining](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/side_turn.go#L597-L632).

## Existing controls to preserve

The matcher already protects against substring collisions inside a larger word. The test for `this is incorrect behavior` specifically exercises that repair. Slash-prefixed trigger matching is exact, and input normalization is explicit. This report is not a claim that the earlier regex fix failed at its stated boundary: the witness deliberately contains the separate word `correct`. [Matcher controls](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L224-L254), [false-positive regression source](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers_test.go#L118-L150).

Positive tests document useful intended replies, including `correct!`, `perfect`, and `I think that's right`. The dream response explicitly offers these natural-language learning actions and tells the user they may instead ask something else. Removing all natural-language confirmation would avoid this branch but degrade an existing interaction contract; preserving intentional confirmation belongs in the acceptance criteria. [Positive test definitions](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_test.go#L356-L390), [displayed follow-up choices](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_dream.go#L486-L495).

Collector mutation is protected by a mutex, and confirmation skips already-confirmed entries and removes confirmed entries from staging. Those are real state-management controls, but neither a lock nor successful deduplication establishes that the user's sentence was an affirmative response. Likewise, downstream permission and persistence checks do not recover a question that never reached perception. [Confirmation transition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/dream_learning.go#L337-L375).

## Pending state and offer binding are different repairs

**Pending-state admission** answers whether there is anything this workflow could confirm. The collector already exposes `GetPendingLearnings`; a cheap read-only pending check could avoid materializing the full pointer slice solely to decide admission. When there is no pending learning, a matching word in ordinary text must not consume the request. An explicit, dedicated confirmation action with no applicable offer may return a no-pending diagnostic, but ordinary prose must retain its normal route. [Existing pending accessor](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/dream_learning.go#L414-L424).

**Offer binding** answers which proposed learning the user agreed to and whether this input is agreement at all. A pending-only guard would still route `How do I correct this error?` to confirmation when a prior consultation happened to leave candidates staged. `ConfirmLearnings` accepts a feedback string, not a candidate or offer identity, and iterates every unconfirmed staged entry. The string influences the confidence boost; this method does not independently reject a question or bind the decision to one displayed set. That is a source-level reason not to call the pending-only patch a complete confirmation repair. It is not, in this audit, a measured claim about actual unintended persistence. [Confirmation API and iteration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/dream_learning.go#L335-L375).

The live dream path extracts candidates after formatting the analysis and stages them through the collector; extraction records a latest dream ID and inserts candidate IDs. These identities are useful building blocks, but they are not currently an explicit user-decision argument to confirmation. A repair should identify the offered set rather than infer consent from the mere existence of collector state. [Dream producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_dream.go#L336-L381), [candidate extraction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/dream_learning.go#L109-L150).

## Proposed contract

Begin with the affected chat and dream capability specifications, following the repository's [spec-first and JIT requirements](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md#L92-L110). The following is proposed behavior, not an as-built API:

- **Ordinary input is unconsumed until a workflow accepts it.** Dream feedback admission returns a disposition such as not applicable, accepted confirmation, rejected or stale explicit confirmation, or needs clarification. Not applicable preserves the existing request identity and schedules normal processing once. A no-op learning handler is not successful request completion.
- **Create an offer from an immutable candidate snapshot.** Bind it to session, dream generation, candidate IDs, and the payload being offered. Present enough of that payload for meaningful review. If the product offers “learn all,” define all as that snapshot, not whatever is staged later.
- **Require an affirmative decision about that offer.** Exact UI choices or a dedicated explicit command can carry the offer identity directly. Natural-language replies can remain supported within the applicable interaction, but matching one word anywhere is insufficient. Questions, quotations, negation, and unrelated new requests must not be treated as consent. If model-assisted interpretation is needed, use the existing typed/JIT path rather than an ad hoc extra model call for every submission.
- **Validate and consume atomically.** Revalidate the offer and its exact candidate set at confirmation time; a separate pending check is not a transaction. Replacement, rejection, clearing, and replay must produce explicit outcomes. A stale reply cannot silently approve a newer or larger set. Define when an unrelated request dismisses the offer, consistent with the displayed promise that the user may ask something else.
- **Keep learning and execution distinct.** Confirmation authorizes only the selected learning transition, not execution of a dream plan or future tool use. Preserve ordinary permission gates and honest persistence results. Do not apply a blanket pending-learning requirement to every correction or explicit teaching command; those can legitimately introduce new information without a previous dream and need their own specified admission policy.

A narrow pending guard can ship first with a checkable empty-staging exit. Record offer-bound semantics as still open until the producer, renderer, dispatcher, and collector agree on the same identity. Avoid relabeling a keyword helper as semantic confirmation without changing those consumers.

## Evaluation through the real entry path

The inspected tests are useful lexical controls, not evidence that normal boot plus request admission is correct. A helper returning true for `correct` is compatible with both intended confirmation and this defect. Future regression work should initialize through the normal component-wiring event, submit via the textarea and Enter handler, and drive returned TUI commands/messages. Use a deterministic, context-aware perception dependency to capture admitted requests and a controlled learning sink where persistence is in scope. Do not call only the confirmation handler or initialize the collector to nil: those bypass the production preconditions.

| Scenario | Required observation |
|---|---|
| Fresh normal session, witness question | One ordinary processing admission with the original input; no confirmation transition or no-pending replacement answer |
| Empty staging, `incorrect` and unrelated questions | Existing boundary protection and ordinary routing retained |
| Empty staging, quoted `correct!`, negation, or prose about learning | No workflow claim based only on lexical inclusion |
| Genuine current offer, exact affirmative reply | Intended offered candidates accepted once; ordinary task processing not duplicated |
| Pending candidates, witness question | Question remains a question; candidate state is unchanged or explicitly dismissed under the specified policy |
| Pending candidates, rejection or ambiguous reply | No affirmative learning transition; clarification only when appropriate |
| Offer replaced, cleared, or already consumed | Old decision cannot confirm new candidates or repeat a transition |
| Explicit confirmation action with no offer | Clear diagnostic, distinguishable from an ordinary prose request |
| Same question submitted idle and as an actually queued request | Equivalent request meaning and answer opportunity, with expected scheduling differences |
| Wizard, clarification, slash, patch, and plan-execution controls | Existing dedicated dispatch and permission behavior retained |

For stronger offer binding, exercise the real dream producer with controlled consultation output so staging and displayed offer are established together. Cover zero extracted candidates as well as multiple candidates and generations. Observe collector transitions before any sink call, and separately observe sink success, partial failure, and replay. Do not equate “confirmation admitted” with “learning persisted.” These are proposed tests, not executed results.

Compare baseline, pending-only repair, and offer-bound repair on the same labeled mix of ordinary questions and genuine confirmations. Keep model, prompts, tools, context, and task budgets fixed where model behavior is involved. Include held-out paraphrases and negative examples; do not optimize only for the one witness. Report false consumption of ordinary requests, missed legitimate confirmations, wrong-offer acceptance, duplicate processing, final answer quality, and user retries. A treatment that disables learning entirely cannot win merely by recording zero false confirmations.

## Costs and acceptance criteria

The minimal pending guard need not add a model call. Its cost is state inspection and synchronization; using a full pending snapshot merely to test emptiness may add allocation proportional to pending candidates. Offer binding adds retained identity and payload data, lifecycle transitions, rendering and review effort, and additional regression cases. A semantic interpretation step can add model latency and tokens unless it reuses an already required call. These costs should be measured rather than described as free.

Restoring ordinary processing can increase immediate calls and latency relative to the defective branch, which returns a cheap but irrelevant notice. Count completion of the user's task and retries, not just time to any response. Report a resource vector of model calls and tokens, wall-clock latency, pending-state bytes, persistence operations, and reviewer/user effort; do not add unlike units into a single unexplained cost score. No savings estimate is supported by this inspection.

The minimal exit is precise: a normally initialized empty dream collector cannot consume ordinary input because it contains a confirmation word. The stronger exit is that only an affirmative decision about a valid offered candidate set can make that learning transition, while unrelated requests still receive their normal processing and legitimate dream confirmation remains usable. Close each exit only with normal-entry evidence, retaining the existing regex protections and downstream safeguards.
