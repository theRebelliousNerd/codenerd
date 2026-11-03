---
doc-class: deep-dive
subsystem: deliberation-chat-queue
implementation-status: proposed-refinement
audit-status: source-grounded-conditional
last-verified: 2026-11-03
verified-against: 0870640530f537d144b747cad51ff8df688429e1
verification-method: static-source-inspection
---

# Preserve an unanswered values question across queued work

## Finding and decision

**A queued deliberation can replace an unanswered values question while leaving both questions pending on disk.** The first question correctly releases its completed reasoning operation and installs clarification mode. The surrounding update then drains queued work because it checks loading and execution ownership, not the pending interaction. A second queued `/deliberate` command can produce another valid values fork and replace the single visible clarification record. The session's restoration selector subsequently refuses the two conversational pending threads. [Question installation][install] [Queue dispatch][dispatch] [Command entry][entry] [Pending selector][pending]

This is a source-grounded composition defect between the foreground queue and the single-question continuation interface. It requires no overlapping foreground execution, malformed model output, stale source, permission change or misinterpreted slash command. The models can follow their contracts correctly in both deliberations. The conditional witness below supplies two genuine values forks and successful persistence; their occurrence in a live run was not measured.

Preserve the distinction between **a reasoning operation that has settled** and **an interaction that still belongs to an unanswered request**. Under the current single-question interface, defer ordinary queued full turns until that interaction has an explicit disposition. Keep the queued request's identity, command kind and receipt order. Do not fix displacement by discarding queued work, replaying it as a values answer, silently choosing the recommended option, or holding a live model operation open while the person thinks.

The first question is not deleted. Its transcript and saved thread remain. The second question can still be answered through its installed ID; if it completes, a later session load can recover the first as the remaining pending question. The demonstrated boundary is **question displacement and a conflicting restore state**, not irreversible data loss, whole-session load failure or unauthorized implementation. [Answer routing][answer] [Resume][resume] [Restore adapter][restore]

## Source and verification boundary

Public `main` resolved to **0870640530f537d144b747cad51ff8df688429e1**, committed at 17:52:14 UTC on November 3, 2026, and remained unchanged when rechecked at 18:18 UTC. All repository links below use that immutable revision. Cached source files used here were checked against the pin's Git blob identities; newly read queue sources and neighboring audits were checked the same way. [Pinned commit][pin]

This review inspected root and relevant scoped guidance, the conversation-first and session specifications, production deliberation and queue paths, relevant authored tests, the current audit inventory and nearest reports. No codeNERD program, build, test, model inference, benchmark, failure injection, credential access or settings change occurred. All fixtures, acceptance checks and design changes below are **proposed and unrun**.

The session specification labels its deliberation section source-authored with runtime checks pending. That is not evidence of an executed failure or a blanket statement about every later test run. This audit independently establishes the source transition described here and states its runtime preconditions. [Session specification][spec]

## 1 Why this matters to Socratic requirements work

The existing proposer and requirements interrogator already implement the intended division of labor: generate alternatives, examine evidence, distinguish factual and design blockers from personal preferences, and ask the person only when a genuine values fork remains. Mangle owns the design disposition. More debate does not repair the boundary after that disposition asks for an answer. [Proposer][proposer] [Interrogator][interrogator] [Policy][policy]

A useful requirements question is an outstanding obligation attached to a particular request, not merely a message rendered when a worker finishes. The person needs a stable way to answer it. A second request entered earlier is valid queued work, but its arrival does not withdraw the first question or answer it. Both requests should survive, and the system should make their order and ownership explicit.

The conversation architecture intentionally permits typing during work and preserves queued receipt order. The session architecture intentionally retains a values question for restoration. Each is reasonable on its own. Their composition needs an admission rule for the interval where no computation is running but the conversation is waiting for a decision. The current implementation represents that interval in clarification state, yet the queue consumer does not consult it. [Conversation architecture][conversation] [Session continuation][spec]

This is language-independent at the deliberation layer: the witness does not depend on Go, Python, Rust or a repository's build runner. It is currently demonstrated by the shared TUI control flow, not by every possible headless or foreign-host adapter. A portable host contract should describe the unresolved interaction separately from whether its worker is busy.

## 2 The exact production chain

### 2.1 A second command becomes ready queued work

Start a normal design-only command A in propose or explicit mode. Its handler creates a fresh deliberation thread, marks the UI loading, and runs the conductor through the foreground-operation wrapper. While A remains in flight, enter a second command B through ordinary non-paste Enter in ChatView. [Command entry][entry] [Foreground owner][foreground] [Enter dispatch][keys]

Busy submission recognizes a slash-prefixed command before any side-turn classification. It preserves the command text and marks the queue record as a command. The queue insertion assigns receipt order and makes that record ready immediately. Thus B does not depend on a later model judgment to become executable. For the minimal witness there are no earlier queue entries. [Busy submission][busy] [Queue insertion][enqueue]

This path benefits from the existing command-kind repair: B will remain a command rather than becoming natural-language work. That repair should be preserved. The defect here is when the correctly identified command is admitted, not what kind of input it is.

### 2.2 A publishes a valid question and ends its computation

For a values fork, `finish` checks the question form, gathers policy-eligible alternatives, requires at least two displayed choices and saves the thread as awaiting values. The command adapter returns a clarification message containing A's thread ID, question, options and original intent. [Values publication][publish] [Question adapter][question]

The foreground wrapper releases its execution owner on return. This is correct: the model is no longer running. When the UI receives the result, it clears loading and installs A as the one clarification state. The question also enters visible history and session-save scheduling. [Foreground settlement][foreground] [Question installation][install]

### 2.3 The same update starts B before A has an answer

After the core message handler returns, the outer `Update` checks whether loading is false and ready queued work exists. It invokes `startQueuedTurn`. That function's loop checks loading and the current execution owner, but not clarification mode or a pending deliberation owner. Both of its checks now pass. A ready record elsewhere in the queue does not guarantee dequeue if an earlier pending head blocks it; the witness assumes no earlier entries. [Outer drain][update] [Queue dispatch][dispatch] [Queue count and dequeue][queue-count]

B is dequeued and dispatched through the ordinary command handler, which selects the deliberation entry. That entry has no pending-question admission check. It creates a new conversational thread in the same session and reserves a new foreground operation. A's saved awaiting-values record is unchanged. [Command aliases][commands] [Command entry][entry]

This is an ordered sequence. A's computation has ended before B starts; existing execution serialization can work exactly as intended. No thread race is needed to confuse execution availability with availability of the single question interface.

### 2.4 A second legitimate fork replaces the first interaction

Let B independently produce a legitimate values fork and complete its saves. Its returned clarification takes the same installation branch, assigning the model's single clarification state to B. That branch does not retain a list of unanswered conversational questions or explicitly suspend A. Both A and B remain saved as awaiting values. [Installation][install] [Clarification shape][clarification-type] [Persistence][save]

At this point the active input refers to B. The first question may remain in scrollback, but its previous selector is no longer the current answer target. The normal answer handler captures the currently installed deliberation ID and uses that ID for resumption. It does not look up a thread from whichever older question the person happens to be reading. [Answer handler][answer]

The finding does not require someone to give an answer intended for A and have it applied to B. That is a plausible usability risk to measure, not an established incident. Displacement and the two-record restore conflict are sufficient concrete consequences.

### 2.5 Restoration refuses a state normal dispatch can produce

`Pending` intentionally excludes mission-owned threads and accepts one conversational thread awaiting values. On finding another, it returns an error instead of choosing one arbitrarily. This is a useful fail-closed ambiguity check. [Pending selector][pending]

The session-list loader calls that selector. On error it logs a restore failure and does not install either pending question through that branch. Session history loading itself can still succeed. On a fresh normal model, neither question is restored as the active clarification by this call. Do not describe this as a failure of the entire session loader or assume that every previously installed UI state is cleared in the same way. [Session loader][restore]

If the person remains in the original UI and successfully answers B, B can become ready while A remains pending. The selector may then recover A on a later session load. This bounds the claim: the data and a conditional recovery route survive; the ordinary single-question flow has nevertheless displaced A and temporarily created a state its own restoration contract rejects. [Resume path][resume] [Exact answer path][service] [Pending selector][pending]

## 3 Minimal unrun witness and useful controls

Use the normal chat command and update path with healthy kernel, proof, registered roles and persistence; one session; propose or explicit mode; no prior pending question; and no approval modal, error focus, logic-pane interception, patch input or stopped owner. Both commands are design-only. Give them different subjects and distinct question text so the active owner is observable.

1. Submit A, such as `/deliberate Compare export layouts for report readers`. Keep its first role response pending at a controlled fixture barrier.
2. Through the real textarea and Enter handler submit B, such as `/deliberate Compare navigation layouts for settings`. Verify a ready command record exists without invoking side-turn perception.
3. Release A's scripted roles with two viable alternatives and a valid values question. Observe its saved thread, released execution owner and returned clarification message.
4. Deliver A's message through the complete `Update`, not just `updateCore`. Observe whether B is dequeued and its foreground command is scheduled before any answer to A.
5. Let B's scripted roles produce another valid values question. Deliver its result normally. Inspect the installed clarification ID and both durable thread statuses.
6. Before answering either, load the session from a clean normal model through the session-list adapter. Observe the pending-selection error and absence of a restored question, while separately confirming history remains loadable.
7. As a recovery control in the original model, answer B correctly, then load the session again. With no other pending threads, A should remain discoverable.

**Source implication under those conditions:** B is scheduled at step 4, B owns the current clarification at step 5, and the two conversational pending records cause the selector error at step 6. These steps were not executed.

**Proposed corrected behavior:** after A's question is installed, B remains accepted in its original queue position but is not admitted as an ordinary full turn while A owns the pending interaction. After A is answered and its resulting operation settles with an explicit disposition, B can run once under normal admission. An explicit, supported context switch may choose another policy, but must preserve both interaction identities and supply a working multi-question restoration model.

| Control | What it distinguishes |
|---|---|
| A returns a ready design, not a question | The ordinary queue should continue to drain. |
| A asks with no queued work | Preserve the existing single-question experience and reload. |
| A asks; B is submitted after A reserves its foreground slot but before its worker starts, or while A runs | Both accepted-command timings must preserve question ownership. |
| A asks; two queued commands B and C | No drop, reordering or duplicate execution when the interaction later releases. |
| A asks; B's next result is ready or failed | The first question must not become a hidden stale owner or block unrelated recovery forever. |
| A and B are in yolo mode | Current values forks choose a disclosed assumption rather than opening this non-yolo pause; do not claim the same witness. |
| Exact option versus a free-text answer to A | Preserve saved-round selection and bounded refinement, respectively; neither grants work approval. |
| Mission-owned pending records plus one conversational question | Preserve the existing owner separation and do not reopen mission recovery. |
| Quit, withdrawal and local help | Define explicit control behavior; do not use a broad barrier to make exit inaccessible or convert controls into answers. |
| A stops or fails before publishing a question | Do not create a phantom interaction hold. |
| Duplicate or late result delivery | Only the matching active interaction may be installed, retired or replaced. |

The critical test observes the full update composition. A test that calls the service directly, manually installs two files, or invokes only the inner clarification handler would miss the production admission boundary.

## 4 Existing work and nonduplication

The current tree and queue were screened, and the neighboring reports' findings and relevant contracts were read. This is a bounded comparison, not an assertion about every historical branch or private discussion. [Audit directory][inventory] [Audit queue][queue]

- **Values control routing** covers an idle command typed after a question becoming an answer. Its witness explicitly excludes ready queued input. Here B was already accepted during A's execution and correctly remains a command. Control parsing alone cannot stop the outer queue from starting B. [Control routing][control-audit]
- **Queued slash command kind conservation** identifies the former command-to-prose replay error. Current source preserves the kind, and the authored test demonstrates the intended dispatcher path. This report relies on that improvement rather than reopening it. The earlier report's modal-admission caution is relevant precedent; the new contribution is a concrete values-question producer and the incompatible restore state. [Command audit][command-audit] [Command tests][queue-tests]
- **Queued history projection** concerns which request owns an assistant response in persisted conversation history. Perfect answer attribution would not prevent replacement of the active clarification pointer or the two-pending selector error. [History audit][history-audit]
- **Recurse values continuation ownership** concerns mission questions entering the conversational selector, including legacy ownership. The witness here uses two new conversational `/deliberate` threads and no mission; classifying mission ownership correctly cannot fix it. [Mission audit][mission-audit]
- **Answer evaluation recovery** begins after a legitimate answer is submitted and its evaluation fails. This witness needs no answer or resumed evaluation. Its recovery requirements remain a dependency when defining how an interaction hold ends after errors. [Answer recovery][answer-recovery]
- **Campaign intake pause ownership** and **dream plan queue conservation** concern different scheduler holds and plan steps. They establish useful ownership precedents, not the single-values-question admission check traced here. [Campaign pause][campaign-audit] [Dream plan][dream-audit]

Authored tests read at the pin cover queued help after an ordinary completed response, slash-kind preservation while busy, stopped-owner settlement, receipt order, one values fork resumed directly through the service, and multiple mission-owned questions alongside one conversational question. Those are meaningful safeguards. None of these inspected cases combines a real values-question result with a ready second deliberation and the enclosing UI queue drain. No claim is made about uninspected or privately run tests. [Queue tests][queue-tests] [Ownership tests][owner-tests] [Values tests][values-tests] [Persistence tests][persistence-tests]

## 5 Proposed interaction admission contract

The following is a design proposal, not a shipped API or schema.

### One explicit owner for the current question

Treat computation ownership and interaction ownership as separate state. The former can be released when a role or command settles. The latter identifies the session, original request and saved deliberation whose question is currently answerable. Reuse the existing thread ID and original intent; add a question generation only if the interaction can actually be republished or replaced.

For the current single-question product, require:

1. At most one active conversational values-question owner per session.
2. Publishing a question makes that ownership effective before the same update can admit the next queued full turn.
3. Settling the producing computation does not clear its unanswered interaction.
4. A queued request retains its original identity and command kind. It is not a reply to the pending question merely because it dispatches later.
5. An answer targets the installed owner. A terminal disposition, explicit withdrawal or explicit supported context switch resolves its interaction hold.
6. Resuming an answer and admitting a new request cannot both claim the same single interaction surface.
7. Restored ownership is reconstructed before queue draining resumes.

The durable thread status is already the source for pending recovery. Prefer deriving or projecting an owner from that record rather than creating a second independently authoritative pending list. If a compact UI owner index is needed, specify its reconciliation with thread persistence and expose disagreement instead of choosing the newest file silently.

### Keep the executive decision explicit

Go should transport typed events, maintain the event-loop transition, persist state and dispatch work. Following the repository's control split, Mangle should own eligibility to start queued full work when an unresolved interaction exists. A narrowly scoped interaction fact and a queue-admission result can express that rule; their exact predicate names and placement require the owning specification. Do not send another model request to decide whether an already identified question is still awaiting its answer. [Root guidance][guidance] [Architecture guidance][architecture]

The UI must apply the decision at every queue-drain entry, including the loop that continues after a synchronous command. A check only at new input receipt is too early: B is legitimately accepted before A's values disposition is known. A check only in the values renderer is too local: the outer update runs afterward. A check that dequeues B before deciding to wait must retain its identity and position without inventing a new request.

An implementation may initially block ordinary full-turn dispatch while allowing an explicit bounded set of interaction controls. That is the smallest coherent single-question behavior. Allowing independent work to continue is a legitimate alternative, but then the product needs separately addressable questions, explicit focus switching, reply routing, enumeration and restoration of all pending owners. Removing the selector's error and returning an arbitrary record is not that design.

### Failure and compatibility rules

- **Publication failure:** do not display an answerable question whose durable identity could not be saved. Keep the failure truthful and do not strand a queue behind a fabricated owner.
- **Answer evaluation failure:** preserve the answer and recoverable question according to the separate recovery contract, or expose a terminal blocked disposition. Do not silently release and forget unresolved interaction state.
- **Shutdown:** stop computation normally. Retain pending question data for reload unless the user explicitly withdrew it; leaving the app is not an answer.
- **Existing multiple pending records:** report the conflict and provide a deliberate selection or reconciliation path. Do not delete records or prefer modification time as proof of user intent.
- **Stale result:** an old question cannot overwrite a newer active interaction merely because its event arrived later. Match the owner and generation where applicable.
- **Queue conservation:** waiting is neither cancellation nor completion. Releasing an interaction must not replay B twice or reclassify it as a new answer.
- **Permission preservation:** a queue release or values choice only enables the next existing admission step. It does not approve a proposed mutation or enlarge the read-only role envelope.
- **Mission isolation:** preserve mission owner tags and the separate owner selector. This report does not authorize resuming a stopped mission through chat clarification.

Do not keep `isLoading=true` for the entire human pause as a shortcut. The current key handler uses loading to route input into the busy queue; that would also queue the person's intended answer instead of reaching the clarification handler. An interaction hold must leave the answer path usable while withholding unrelated full-turn dispatch. [Key routing][keys] [Answer handler][answer]

## 6 Costs and tradeoffs

The repair does not inherently save B's reasoning cost. If B is still wanted, its proposer, challenger, reads and consultations will occur later. The benefit to measure is preserving A's answerability and avoiding avoidable restarts or mistaken responses, not claiming every deferred call is eliminated.

In the minimal one-round fixture, B starts one proposer role and one challenger role before A is answered. Each role can include additional tool-bearing model exchanges; production provider retries, JIT work, consultations, proof recording and persistence add costs. Count provider requests and tokens, not merely two role dispatches. The fixture should omit optional reads and consultations only to isolate control flow, then include them in subsequent production-path measurements. [Round collection][round] [Role loop][role]

The single-question contract adds a small owner reference and policy admission checks; it need not duplicate candidate text, proof trees or whole thread history. Expected bookkeeping can be constant-size per active conversational interaction, while the existing queue still retains its own entries. Actual query, serialization, allocation and lock costs must be measured. Do not scan every saved thread or rewrite all history on each keystroke merely to decide whether one already known interaction is pending.

Deferring full work increases B's queue latency by the time spent resolving A, which may be long or unbounded if the person leaves. The UI should say what is waiting and provide supported control choices. If responsiveness for independent requests matters more than single-question simplicity, the explicit multi-question alternative has higher implementation, testing, navigation and recovery cost. That tradeoff belongs to product design rather than an accidental drain rule.

For an evaluation, account separately for:

- all role/model input, output, reasoning and retry usage, including failed or restarted deliberations;
- JIT and retrieval work, tool calls, proof generation, saves, reloads and recovery reads;
- time to the first stable answerable question, time waiting for the person, B's added waiting time and time to both final design dispositions;
- human attention: displaced questions, repeated instructions, reply-target corrections, reload navigation and abandoned tasks;
- queue length, age and memory, plus owner-index storage and reconciliation work;
- independent authorization and verification costs when an automatic non-design continuation is evaluated later.

No monetary savings, latency improvement, failure frequency or model-quality uplift has been measured here.

## 7 Primary literature and the bounded design inference

Horvitz's **Principles of Mixed-Initiative User Interfaces** (CHI 1999) treats action timing, user attention, efficient clarification and direct control as part of useful automation. It motivates evaluating interruption and recovery costs alongside throughput. It does not establish that every queued request must wait, or prove this patch's benefit. The proposed single-question barrier is an engineering inference from codeNERD's current interface and restoration invariant. [Horvitz paper][horvitz]

Clark and Brennan's **Grounding in Communication** (1991) separates coordination of what is communicated from coordination of the communication process, and discusses how medium-specific costs shape grounding. That distinction fits this case: both question contents can be sound while their delivery and answer opportunity are poorly coordinated. It supplies a rationale for measuring user repair effort, not a runtime proof of codeNERD behavior. [Clark and Brennan][grounding]

Bohus and Rudnicky's **RavenClaw** work separates domain task logic from reusable dialog control. Its authors' framework description explicitly includes turn-taking, suspension and recovery behavior. The relevant lesson is to make question lifecycle a host responsibility rather than asking each requirements prompt to manage the shared UI. This is design precedent, not a proposal to import that framework or a claim that spoken-dialog results transfer unchanged to coding agents. [RavenClaw authors][ravenclaw]

## 8 Fair evaluation and adoption gates

All evaluation in this section is proposed and unrun.

Start with a deterministic integration fixture using the real keyboard/update dispatcher, real queue records, normal foreground wrapper, real Mangle disposition, real thread persistence and the production session loader. Script only model replies and the timing barrier needed to queue B before A settles. Keep both questions valid and distinct. Do not stub the very queue-admission or installation functions being evaluated.

Compare the pinned baseline with a candidate repair using identical task texts, role replies, policies, tool envelopes, source snapshot and event order. The baseline observation is a regression witness, not evidence about its population failure rate. First prove both conservation and liveness: A remains answerable, B remains queued, A can resolve, B dispatches exactly once, and each saved state has the expected owner. Count question installations, pending files, queue identities, role invocations and user-answer targets.

Exercise the controls in Section 3 and failure cases in Section 5, including admission before/after the values result, duplicate result delivery, queued synchronous commands, a pending queue head, session reload, persistence errors, free-text refinement and cancellation. Add deterministic schedules around question publication, owner settlement and queue selection. Use race instrumentation later where shared state warrants it, but do not require a data race to reproduce this ordered bug.

Only after the structural checks pass should a live study ask whether blocking or explicit multiplexing better serves users. Compare at least:

- current behavior;
- a single-question queue barrier;
- an explicit multi-question interface, if separately implemented and reviewed.

Use matched task pairs and counterbalanced order, hold model/provider and budgets fixed where feasible, and include single-task sessions to measure unnecessary overhead. Report correct answer-to-request binding, completion of both requested designs, rework, abandonment, human actions and full resource costs. Analyze time waiting for the person separately from compute latency. A solution that prevents displacement by indefinitely starving B is not a success; nor is a faster solution that silently answers A's preference or drops B.

Adoption requires an owning specification for the interaction state and its queue policy, the production-path regression and recovery controls, preserved command/FIFO/cancellation behavior, and evidence that propose/explicit permission gates still hold. Update current-state claims only after the relevant checks have actually run. The target is a stable opportunity to answer the question the system chose to ask, with accepted later work conserved until it can safely use the same conversational surface.

## Pinned sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/0870640530f537d144b747cad51ff8df688429e1
[inventory]: https://github.com/theRebelliousNerd/codenerd/tree/0870640530f537d144b747cad51ff8df688429e1/Docs/audits
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/agents.md#L5-L95
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/architecture/AGENTS.md#L1-L120
[conversation]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/architecture/conversation-first-agent.md#L68-L99
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L900
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/prompt/atoms/system/deliberation_proposer.yaml#L1-L48
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/prompt/atoms/system/requirements_interrogator.yaml#L1-L49
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/core/defaults/policy/deliberation.mg#L18-L74
[install]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/model_update.go#L264-L287
[update]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/model_update.go#L24-L36
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/side_turn.go#L597-L632
[enqueue]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/side_turn.go#L238-L249
[busy]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/side_turn.go#L352-L380
[keys]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/model_key_handler.go#L242-L265
[entry]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/deliberation.go#L96-L120
[foreground]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/foreground_operation.go#L13-L50
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/outcome.go#L16-L80
[question]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/deliberation.go#L53-L56
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/commands.go#L112-L113
[clarification-type]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/model_types.go#L192-L200
[save]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/persistence.go#L14-L56
[pending]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/persistence.go#L77-L109
[answer]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/model_handlers.go#L474-L504
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/deliberation.go#L70-L93
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/session_persistence.go#L229-L275
[service]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/service.go#L76-L97
[round]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/round.go#L10-L73
[role]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/read_tools.go#L84-L207
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/ASTRA_QUEUE.md
[control-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/deliberation-values-control-routing.md
[command-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/queued-slash-command-kind-conservation.md
[history-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/queued-history-projection.md
[mission-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/recurse-values-continuation-ownership.md
[answer-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/deliberation-answer-evaluation-recovery.md
[campaign-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/campaign-intake-pause-ownership.md
[dream-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/Docs/audits/dream-plan-queue-conservation.md
[queue-tests]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/queued_command_test.go#L10-L59
[owner-tests]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/side_turn_ownership_test.go#L181-L256
[values-tests]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/deliberation_test.go#L179-L219
[persistence-tests]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/internal/deliberation/deliberation_test.go#L99-L177
[horvitz]: https://www.microsoft.com/en-us/research/wp-content/uploads/2016/11/chi99horvitz-1.pdf
[grounding]: https://web.stanford.edu/~clark/1990s/Clark%2C%20H.H.%20_%20Brennan%2C%20S.E.%20_Grounding%20in%20communication_%201991.pdf
[ravenclaw]: https://www.cs.cmu.edu/~dbohus/ravenclaw-olympus/index-dan.html

[queue-count]: https://github.com/theRebelliousNerd/codenerd/blob/0870640530f537d144b747cad51ff8df688429e1/cmd/nerd/chat/side_turn.go#L307-L335
