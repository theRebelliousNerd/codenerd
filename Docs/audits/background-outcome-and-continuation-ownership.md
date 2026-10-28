---
doc-class: deep-dive
subsystem: cli
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: ac2f1fd3425dab9c14ff9066f9bf10cad02acc75
verification-method: static-source-inspection
supersedes: []
---

# Background outcomes need an owner after the worker finishes

**Decision:** preserve the feature branch's job-owned result delivery, but give each background chain an originating session, a latched control disposition, and a retained successor obligation. A worker finishing, a result reaching the chat, and permission to begin the next step are three different events. A busy UI must defer an owed successor rather than erase it; a stop must suppress not-yet-dispatched continuation without falsifying completed evidence; a session switch must not adopt another session's pending work.

**Source freeze:** feature `feat/conversation-brief` at [`ac2f1fd3425dab9c14ff9066f9bf10cad02acc75`](https://github.com/theRebelliousNerd/codenerd/commit/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75), committed 2026-10-28 09:20:06 UTC and re-resolved at approximately 09:22 UTC. Main remained [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871) at the 09:10 UTC check. Main has no `cmd/nerd/chat/background.go`; these are feature-path findings, not claims that the new path already shipped on main.

**Verification limit:** source inspection only. No codeNERD runtime, tests, model inference, failure injection, benchmark, implementation, or GitHub write was performed. The schedules below are source-derived witnesses to validate, not reproduced incidents. Local source copies are under `chat_evidence_delivery_20261028/ac2f1fd/`; they are research inputs, not a modified repository.

## 1. What the feature already gets right

The new background path is a real improvement for preliminary investigation. [Routing](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/core/defaults/policy/routing_arbitration.mg#L312-L324) derives background work only for a delegated `/query`. The chat returns promptly, names the running task through `background_job`, and delivers the result through a message carrying the specific job. Turns classified as `/mutation` are excluded by this routing rule, and the subsequent full-turn work-admission policy still applies. That category exclusion is not, by itself, proof of an independently enforced read-only tool envelope.

The result is not recovered by comparing the size of two global fact sets. [`backgroundDoneMsg`](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L113-L120) directly carries the job, raw result, rendered surface, result facts and execution error. [Completion](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L171-L189) distinguishes an execution error from a normal return and stores successful raw output for follow-up questions. Reviewer/tester output includes the raw result even if automatic interpretation fails. These properties should survive the repair.

The self-diagnosis path is also materially grounded: [task construction](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/self_diagnosis.go#L57-L108) reads the problems log, selects kernel-derived hot problems, optionally triages them and writes their full sample into the task. It records a `FollowUp` asking for fixes with tests. The intended chain is analysis followed by a new full turn, with work mode deciding whether that fix runs, is proposed or waits for approval.

The branch also advanced during this audit: the current `process.go` snapshots system-result identities again after stale-fact cleanup, adds gathered evidence to articulation, and merges request restrictions into the foreground context. Those changes repair adjacent paths and deserve credit; they do not change the background completion/continuation branches examined here. [Result baseline](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L251-L255), [restriction context](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L213-L219), [gathered evidence](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L842-L854), [articulation handoff](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L895-L905).

This report does **not** re-report:

- the earlier residual TUI count-baseline and local-read delivery findings in [TUI evidence turn ownership](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/tui-evidence-turn-ownership.md);
- the session executor's loss of partial execution state on a hard loop error in [delegated-turn failure settlement](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/delegated-turn-failure-settlement.md);
- the older loss of the original user question in synthetic task rewriting;
- side-turn intent classification, side-turn queue errors or delayed foreground-stop ownership.

Those are adjacent contracts. Here the background worker can return successfully with useful evidence, and the consumer still loses, misroutes or revives the remaining obligation.

## 2. The current lifecycle, precisely

| Boundary | Current behavior | Source |
|---|---|---|
| Route | A delegated query derives `route_background`; a mutation does not. | [routing_arbitration.mg:312–324](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/core/defaults/policy/routing_arbitration.mg#L312-L324) |
| Build job | Job contains ID, shard, task, start time and optional follow-up string. It has no originating session or request generation. | [background.go:26–44](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L26-L44), [process.go:493–509](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L493-L509) |
| Launch | Update makes the chat idle and schedules the job. A shared registry retains its cancellation function; the job context derives from the model shutdown context. | [model_update.go:168–178](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update.go#L168-L178), [background.go:136–145](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L136-L145) |
| Execute and interpret | The task adapter returns an observed result. Background code copies `ret.Output`, constructs a done message with the spawn error, then optionally checks and interprets the result. | [background.go:145–165](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L145-L165) |
| Stop request | A derived background stop invokes every registered cancel. The registry does not record a stop disposition separate from the cancel function. | [jev_turn.go:106–117](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/jev_turn.go#L106-L117), [background.go:76–89](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L76-L89) |
| Consume result | The current Model removes the job, attempts fact publication, records a terminal job status and appends a message to its current history. | [background.go:171–189](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L171-L189) |
| Continue chain | A nonempty follow-up is submitted only if `m.isLoading` is false at that instant. No pending successor is stored on the busy branch. | [background.go:190–197](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L190-L197) |

The important distinction is not foreground versus background as a label. It is **execution ownership versus completion-consumer ownership**. A job ID is sufficient to identify which worker returned, but does not identify which conversation should receive it or whether its remaining chain is still authorized.

## 3. B1 — finishing while the chat is busy silently loses the successor

This is a direct control-flow loss, not a probabilistic routing concern.

The self-diagnosis job sets [`FollowUp: selfDiagnosisFollowUp`](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/self_diagnosis.go#L102-L108). At completion, `finishBackground` first removes the job from the running registry, stores the result, marks it done and presents it. It then returns without a command when either no follow-up exists **or** the chat is loading. The two conditions have different meanings, but the code gives both the same terminal action.

### Source-derived schedule

1. Session A starts self-diagnosis job J. The chat is intentionally left open.
2. The user starts a separate foreground turn T.
3. J completes normally while T leaves `m.isLoading == true`.
4. `finishBackground(J)` displays the diagnosis and retires J, then takes the busy branch at line 190.
5. T completes later. There is no queued record of J's fix step to consume.

The existing full-turn queue does not rescue this schedule. [Update's queue check](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update.go#L24-L32) can only dequeue inputs actually stored in `turnState.queued`; background completion never enqueues its follow-up. A `background_job(...,/done)` row contains no successor payload, and the shown rule corpus has no done-job recovery rule.

**Impact:** the analysis result is not lost from the current in-memory chat, but the promised analysis-to-fix/proposal step is lost. A person must notice the absent continuation and manually ask again. This contradicts the feature's own “keep talking” journey: talking at the wrong instant changes whether the chain exists.

### Two jobs make the loss deterministic in a natural schedule

Suppose two diagnosis jobs complete close together while the chat was idle. The first completion starts its follow-up and sets `isLoading = true`. The second completion can then drop its own follow-up. Completion timing, not an executive choice about priority or supersession, selects which diagnosis proceeds.

This is not a recommendation to execute both fix requests immediately. A correct policy may merge compatible findings, supersede duplicate work or require approval. It must retain each result and derive an explicit disposition rather than interpreting UI occupancy as cancellation.

**Required contract:** after result settlement, every nonempty successor is either retained as pending, dispatched once, explicitly superseded, rejected or canceled. “Busy right now” is a scheduling condition, not a terminal disposition.

## 4. B2 — a stop after execution can still leave an eligible follow-up

The feature adds a genuine natural-language background-stop path. [The policy](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/core/defaults/policy/jev_evidence.mg#L84-L90) derives `stop_background_requested` when a configured Jev judgment says the user wants to stop and a job is running. [The caller](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L213-L222) invokes cancellation and adds a stopping warning.

But cancellation is stored only as a function to call. `cancelAll` does not mark the job or chain stopped, and `backgroundDoneMsg.Err` was fixed to the task-execution error **before** the later review-check/interpretation phase.

[Automatic interpretation](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process_dream_delegation.go#L418-L473) uses the same context for a model call. If that call fails, including cancellation, `formatInterpretedResult` returns a useful fallback string plus raw output; it does not propagate the error. Keeping already-completed raw evidence is appropriate. Treating the same fallback as permission to start the next step is not.

### Source-derived schedule

1. J's read-only executor returns normally. `done.Err` is nil.
2. J is still in the registry while review checks, interpretation or message delivery remain pending.
3. A recognized stop calls J's cancellation function.
4. The interpretation may return a canceled error and become fallback display text; alternatively the result message may already be queued. Either way, the done message can retain `Err == nil`.
5. The stop turn finishes before Update consumes J's done message.
6. `finishBackground` has no recorded stop disposition to inspect. It follows its normal-return branch and can submit J's `FollowUp` as a fresh turn.

The schedule does **not** establish that a file is changed. The fresh turn still passes perception, work mode, permission and verification gates. Nor should the system falsify the completed analysis by rewriting its execution outcome to “nothing happened.” The demonstrated seam is that an accepted stop does not necessarily prevent the undispatched successor from being submitted, and the job can be labeled finished without acknowledging the stop.

This is specifically the **background natural-language stop** path. [Ctrl+X](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_key_handler.go#L60-L77) calls the foreground `turn.stop()` while loading; it is not the background cancellation mechanism under discussion. Jev disabled, a failed judgment or a stop not recognized by policy is also outside this witness.

**Required contract:** latch stop intent by job/chain identity before invoking context cancellation. Preserve the completed observation, mark remaining delivery or work as canceled as appropriate, and require a new explicit resume/admission decision before dispatching a still-pending successor. Cancellation is a request about remaining work; it is not merely a guess about what error a downstream function will eventually return.

## 5. B3 — late completion adopts the selected session

A background job is owned by a shared registry and the model shutdown context, not by the session that launched it. Neither the [job](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L26-L44) nor its [done message](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background.go#L113-L120) carries an origin session or request generation.

Session switching is a normal UI operation: [the sessions list's Enter handler](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_key_handler.go#L92-L105) calls `loadSelectedSession`. [That method](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/session_persistence.go#L185-L243) saves the current session, replaces `m.sessionID`, replaces history and turn count, rehydrates the selected session's compressor state, and returns. It does not cancel, rebind or detach background jobs.

Later Update passes a done message into `finishBackground` on the **then-current** Model. The method unconditionally publishes its facts, updates the shard blackboard, appends the result to current history and, when idle, starts the follow-up.

### Source-derived schedule

1. Session A starts J, an ordinary review or self-diagnosis.
2. While J runs, the user selects an existing session B.
3. J completes successfully.
4. B receives A's background result in its history. [`storeShardResult`](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_lifecycle.go#L263-L299) records the result using B's current turn count and makes it `lastShardResult`.
5. If J has a follow-up and B is idle, the synthetic follow-up is processed using B's current conversational state.

The worker need not have read another workspace for this defect: session selection in the inspected path stays within the same workspace. The failure is conversation ownership and control provenance, not a demonstrated cross-workspace read or security breach. Likewise, a current session's permission gate is not bypassed; it is asked about work whose provenance was not carried into this boundary.

The same ownership issue is reachable through [`/new-session`](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/commands_handlers_files.go#L122-L136), which replaces history and session ID without retiring the background registry.

A completion notification in the currently visible view can be a good product choice. **Adopting the originating session's evidence and next-step request as the current session's work is a separate choice.** If background work is intentionally workspace-scoped, the UI should make that scope explicit and the next action should still preserve its originating request. The present message lacks the identity needed to make that distinction.

**Required contract:** terminal results settle to their originating owner even after navigation. An inactive session may receive a durable result plus an unobtrusive notification; it must not silently hand a mutation/proposal successor to the active session. Choosing to resume the old chain is a new visible transition, not a side effect of viewing another conversation.

## 6. Preserve these adjacent facts while repairing the lifecycle

These are narrower integration observations, not additional claims that every background answer is wrong.

### Structured observations are richer than the completion message

The background path invokes an observed executor but keeps only its output string and error. [The existing observed adapter](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/session/observed_return.go#L47-L84) can carry the kernel outcome, missing evidence, stage, snapshot and acceptance information. The foreground continuation path already [distinguishes complete, incomplete, failed and unverified](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process_continuation.go#L249-L276).

A repair should retain the existing observation rather than reconstruct completeness from raw prose or equate a nil transport error with validated findings. This does not mean a read-only query needs the mutation verifier or that every normal background return currently carries an incomplete verdict.

### Fact publication and display are separate deliveries

The background consumer logs a `LoadFacts` failure but still stores and displays the result. That is useful best-effort presentation, not proof that every later model can query the evidence. Preserve the raw result, record a separate publication failure, and retry publication without re-running the investigation. Do not hide a useful answer merely because a secondary projection failed; do not mark that projection delivered when it was not.

### Completion does not schedule the normal session save

The background-start branch batches a session save, but [the done branch returns directly](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update.go#L168-L178) without the save used by normal assistant/response messages. A later event or explicit session action can persist current history, so this is not proof that a normally closed session always loses the result. It is a missing immediate save trigger at the new completion boundary.

Also, [SQLite synchronization](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/session_persistence.go#L147-L183) assumes strict alternating pairs at offsets 0,2,4,… and skips any pair with other roles. Given an otherwise valid prefix `[user A, assistant start A]`, an unsolicited `assistant done A` before `user B` shifts the pairs: `[assistant done A, user B]` is skipped, and later ordinary alternating turns remain offset until another extra message changes parity. JSON history serialization does not use this pairing shortcut and can still preserve the messages when invoked. The new async path should not promise queryable session continuity solely because a JSON save was added.

Both persistence observations reinforce the same design: explicit message/request identity is more reliable than adjacency or the current turn count. They do not justify a second independent history database.

## 7. Proposed ownership and admission contract

Follow the root [North Star](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/agents.md#north-star): Go owns observation transport and synchronization; Mangle derives executive choices. Do not repair the issue with another LLM planner, a textual “remember to continue” instruction or unconditional Go dispatch whenever the spinner stops.

### Minimal record to keep

Reuse the existing job/task/observation and session-persistence owners. A record needs, conceptually:

- immutable job/chain identity and the originating session/request identity;
- immutable analysis task and the original request, including its structured restrictions and admitted scope, rather than only the synthetic shard task;
- the observed return and its execution outcome;
- independently tracked result-delivery destinations;
- latched stop/supersession disposition;
- successor identity, evidence dependency and admission disposition.

The exact schema is a design decision, not an implemented API. Do not make raw result bytes a duplicated large Mangle fact merely to gain identity: a retained observation handle plus typed metadata can carry the same authority with bounded prompt cost.

### Three state dimensions, not one “done”

1. **Execution:** running → observed terminal outcome.
2. **Delivery:** pending → delivered/failed for chat, facts and persistence independently.
3. **Successor:** absent/pending → admitted/dispatched/held/canceled/superseded/rejected.

A successful execution can coexist with a canceled successor and a failed fact projection. That combination is honest, not an inconsistent state to flatten into one boolean.

### Derived dispatch conditions

Before a successor begins, the executive needs evidence that:

- its dependency result exists and its scope is suitable;
- the originating request is not stopped or superseded;
- the chosen execution owner/session is still valid;
- foreground capacity is available or a deliberate queue policy selected this work;
- work mode, permission and verification admission have been satisfied;
- this successor has not already been dispatched.

A busy event records “pending,” not “no successor.” An inactive session records “available there,” not “owned here.” A stop prevents a not-yet-admitted successor without discarding legitimate analysis output. A duplicate completion can be acknowledged without appending another result or launching a second successor.

### Tradeoffs to decide explicitly

| Choice | Advantage | Cost / required guard |
|---|---|---|
| Keep background work alive across navigation | Avoid wasted investigation; return to results later. | Origin-scoped settlement and notifications; never implicit successor adoption. |
| Cancel background work on navigation | Simpler active-session model. | Navigation becomes a work-control event; disclose it and preserve completed/partial observations. |
| Queue every successor FIFO | Simple, predictable recovery from busy completion. | Can run stale or overlapping fixes; still needs supersession, current-source checks and work admission. |
| Derive successor coalescing/priority | Fewer redundant investigations and conflicting fix attempts. | Requires explicit evidence joins and visible dispositions for merged or superseded work. |
| Show raw result before interpretation finishes | Lower time to first evidence; cancellation need not conceal useful output. | Versioned/updatable presentation or a second message, without changing execution/admission state. |

The recommended starting point is origin-scoped retained obligations with explicit cancellation. More sophisticated coalescing can follow evidence from actual concurrent use.

## 8. Validation plan — proposed, not executed

Use the ordinary TUI Update/command path, real embedded routing/work-admission policy and deterministic executor/provider doubles. The important variable is message ordering, not model quality. Control the executor, interpreter and Update delivery independently.

| Scenario | Required observation |
|---|---|
| One diagnosis, idle completion | Result appears once; successor remains subject to normal work mode; no duplicate execution. |
| Diagnosis completes during a foreground turn | Result is retained; successor remains pending and is reconsidered after the foreground settles. |
| Two diagnosis jobs complete while idle | Neither successor disappears; policy explicitly queues, merges, supersedes or holds them. |
| Stop before task execution finishes | Stop disposition latches; resulting error/evidence is preserved; no automatic successor. |
| Stop after execution but during interpretation | Completed analysis remains visible; canceled interpretation is labeled; successor remains canceled. |
| Stop after done message is produced but before consumption | Result delivery honors the latched stop and does not revive continuation. |
| Stop and completion in both Update orders | A documented linearization point determines the remaining-work outcome; timing is not inferred from the final error string. |
| Session A launches, user selects B, then A completes | A owns its result; B receives at most a labeled notification; no A follow-up starts in B. |
| Return to A after inactive completion | Result and pending/held/canceled disposition are recoverable without re-running J. |
| Duplicate done-message delivery | One result settlement and at most one successor dispatch. This is a robustness requirement, not a claim the current Tea runtime normally duplicates commands. |
| Fact publication failure | Result remains accessible; publication failure is explicit; repair retries projection, not the task. |
| Session save failure / application exit | Delivery is not marked durable before the required write; recovery does not falsely infer a pending chain succeeded. |
| Unsolicited result between normal turns | JSON and SQLite projections preserve explicit identities; later user turns are not dropped by pair parity. |
| Missing/unverified observed verdict | Useful raw evidence is retained with uncertainty; admission does not invent a complete diagnosis from an empty error. |
| Source revision changes before proposed fix | The successor refreshes/revalidates cited evidence; old findings are not silently treated as current. |

Inspect the current [background test definitions](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/background_test.go#L19-L120) as the starting fixtures: they cover routing, mutation exclusion, a visible successful result, an execution failure, self-diagnosis selection and complete problem samples. They do not exercise a busy `FollowUp`, stop after spawn success, or a session switch between launch and completion. Existing [persistence-command tests](https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update_persist_test.go) validate asynchronous save construction and history snapshot isolation, not this new completion scheduling or nonalternating message sequence. These tests were read, not run.

## 9. Acceptance and implementation sequence

The useful success measure is not merely “the background footer disappeared.” Measure:

- one retained terminal result per admitted job, with the correct originating session;
- zero successors lost because the UI was busy;
- zero not-yet-dispatched successors revived by a latched stop;
- one explicit disposition for every successor dependency;
- result availability after navigation and persistence/reload;
- time to first useful evidence, time until the eligible successor is admitted, and cancellation-to-settlement delay;
- projection/persistence retries separately from task re-execution, including their token/tool cost.

No baseline or performance improvement was measured here.

A scoped implementation should first update the CLI capability/gap spec with this lifecycle contract, then add origin and successor identity to the existing background boundary, then make settlement/stop/dispatch idempotent under controlled ordering. Integrate the existing turn queue rather than create an unrelated scheduler, but do not reduce a structured successor to an unowned string. Connect result persistence and fact-publication retries to the same settlement. Finally verify the production journey with both normal foreground work and session navigation.

The desired behavior is simple for the person: **keep talking, receive the investigation you asked for, and never lose—or unexpectedly resume—the work that follows from it.** The implementation may be complex; the obligations should be explicit.


## As-built status

Verified and built 2026-10-28 on `main` after `c28a1212`, where `background.go` now ships (the feature branch is merged). The background lane had gained `Target`/`Constraint` and `withPartialState` on failure since the audited pin; the continuation and ownership branches below were unchanged.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| Section 1: job-owned delivery, raw result kept, mutations excluded | Confirmed, kept | `backgroundDoneMsg` (`background.go:136`); `route_background` unchanged |
| B1: `FollowUp == "" \|\| m.isLoading` drops the next step when busy | Real, fixed | A busy chat (loading or an owned foreground slot) enqueues the next step in the turn queue and says so (`finishBackground`, `background.go:215`) |
| B1: two chains finishing together lose one next step | Real, fixed | The second is queued; `TestBackground_TwoChainsFinishingTogetherKeepBothNextSteps` |
| B2: `cancelAll` latches nothing; a stop after the work returned still starts `FollowUp` | Real, fixed | `cancelAll` latches `stopped[id]`; `settle` returns it; a stopped job shows its result and does not start its next step (`background.go:78`, `:96`, `backgroundReport` `:257`) |
| B3: job and done message carry no origin session; a late result is adopted by the open session | Real, fixed | `backgroundJob.Session` set by `startBackground` (`background.go:170`); `settleElsewhere` appends the report to the origin session's saved history and shows only a notice (`background.go:279`, `appendSessionMessage` `:290`); no blackboard write and no next step in the open session |
| Duplicate done message settles twice | Real (robustness), fixed | `settle` reports `wasRunning`; a second done is ignored |
| Structured observation reduced to `ret.Output` and the error | Partly superseded, rest left open | Failure now carries `withPartialState`; a successful return still keeps only `Output` |
| `LoadFacts` failure logged while the result is displayed | Confirmed, left as is | Display is not withheld; no retry of projection was built |
| Done branch schedules no session save | Real, fixed | `model_update.go:178` batches `saveSessionStateCmd` |
| SQLite pairing assumes strict alternation; an unsolicited message shifts later turns | Real, fixed | `sessionTurns` pairs each user message with the next assistant reply (`session_persistence.go:173`); the store upserts by turn |

### What was built

- Rulings: (1) a busy chat's owed next step goes into the existing turn queue as the same string the idle path submits, so it passes the same perception and work admission when it runs; no new scheduler. (2) A latched stop cancels the next step but keeps the completed result and marks the job `/done` (its work did finish); a stop that interrupted the work keeps `/failed`. (3) Background work survives navigation; its result settles to the starting session's saved history, and resuming the chain there is a new request. (4) A job with no recorded session (built outside `startBackground`) settles in the open session.
- `cmd/nerd/chat/background.go`, `model_update.go`, `session_persistence.go`; spec `Docs/architecture/conversation-first-agent.md` (background section).

### Tests

`cmd/nerd/chat/background_ownership_test.go`: busy completion queues the next step with the result; two chains keep both next steps; a stop latched after the work returned shows the result and starts nothing; a duplicate done settles once; a job from session A finishing in B lands in A's saved history with B showing only a notice and adopting nothing; completion schedules a save; SQLite pairing survives an unsolicited result. Existing background and Jev stop journeys pass.

### Left open

- Queued next steps and queued messages are not session-scoped: a message queued in A runs in B after a switch (shared with the side-turn queue).
- No per-destination delivery state (chat, facts, persistence) and no retry of a failed fact projection; a failed append to the origin session falls back to showing the result in the open one.
- The successful observation (`observation.Return` verdict, missing evidence) is not carried to the completion message; source-revision revalidation of a queued next step is left to the turn that runs it.
- Policy-derived coalescing or supersession of concurrent chains was not added; both are queued in completion order.
