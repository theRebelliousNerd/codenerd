---
doc-class: research-proposal
subsystem: deliberation-values-control
implementation-status: proposed
audit-status: source-grounded-conditional
last-verified: 2026-11-03
verified-against: daa3e2b60f42bdfe67c04f17aca7c6af76120803
verification-method: static-source-inspection
---

# Keep exit and withdrawal controls out of values answers

## Finding and decision

**A valid values pause changes the meaning of an existing application exit command: typing `/quit` and pressing Enter resumes deliberation instead of entering the quit-command branch.** The paused UI sends every nonempty typed submission to the saved thread as `Answer`. If it does not match a candidate and another round is allowed, the conductor saves the thread as running and invokes its proposer. No participant has to misunderstand the text for this routing mismatch to occur. [Enter dispatch][enter] [Normal command routing][submit] [Quit command][commands] [Answer submission][answer] [Resume][resume] [Conductor][run]

At that pause, Ctrl+X's cancellation branch is inactive because the clarification handler sets `isLoading=false`; Escape in ChatView is explicitly a no-op. **Ctrl+C and Ctrl+D still exit globally.** The finding is therefore not an inescapable UI, an uncancellable running role, or evidence that code was edited after a stop request. It is a mismatch between ordinary command semantics and values-answer routing, together with an undeclared task-withdrawal boundary in the inspected values flow. [Pause installation][pause] [Keyboard controls][keys]

Strengthen the existing continuation contract with three distinct acts: **answer this question, leave the application, and withdraw this deliberation**. Route an explicit control act before it becomes a candidate answer or starts further reasoning. Preserve the original request for genuine answers. Do not solve the problem by reclassifying all answers as new work, by interpreting arbitrary slash-prefixed paths as commands, or by making a model reason its way out of a UI control request.

## Source and verification boundary

Public `main` resolved to **daa3e2b60f42bdfe67c04f17aca7c6af76120803**, committed at 17:20:28 UTC on November 3, 2026, and remained there when rechecked at approximately 17:48 UTC. All repository links below use that immutable revision. Source copies used for this review were checked against that revision's Git blob identities, including reused cached files. [Pinned commit][pin]

The review covered root and relevant scoped guidance; the session, shard, conversation-first and mission design; the deliberation conductor, persistence, policy and role atoms; production keyboard, clarification, command and session-loading adapters; relevant authored tests; and the current audit tree and nearest reports. Inspection is not runtime validation. No codeNERD program, build, test, model inference, benchmark, failure injection, credential access or settings change was performed. The witness and evaluation plan below are **proposed and unrun**.

The session specification describes its deliberation changes as source-authored with original runtime checks pending. That is an evidence boundary, not proof of a current failing test. Existing authored cases cover values selection, permission preservation, configured stops and running Ctrl+X behavior; none of the inspected cases exercises an application control submitted while a real values question is idle. [Session specification][spec] [Values tests][values-tests] [Running-stop tests][stop-tests]

## 1. Why this belongs to Socratic requirements work

A Socratic workflow is useful because the system investigates uncertainty before involving the person, offers viable alternatives when a genuine preference remains, and carries the person's answer back into the original design problem. codeNERD already implements much of that shape. Its proposer preserves the complete request and explicit constraints; its challenger distinguishes factual, design and preference blockers; Mangle owns readiness, selection and stopping. [Root guidance][guidance] [Proposer][proposer] [Interrogator][interrogator] [Policy][policy]

The person may nevertheless decide that the question, the task, or the current session is no longer wanted. Continuing the original request is appropriate only for an actual answer. Preserving intent must not become unconditional preservation of a task after an explicit control act.

The architecture's existing stop and ownership mechanisms show that stopping is not supposed to depend on a creative role choosing to cooperate. Foreground operations have an owner and a cancellation path. The missing composition is the interval **after the reasoning operation settled and before the person answers**, where the UI has a durable pending question but no running operation for Ctrl+X to stop. [Foreground owner][foreground] [Conversation-first architecture][conversation]

The current specification promises original-intent preservation and pending-question restoration. It does not explicitly specify a values-question withdrawal action or the precedence of slash controls over modal answers. This report therefore separates:

- **Implemented, source-confirmed behavior:** the existing `/quit` command is routed into a values answer under the stated pause conditions.
- **Proposed design contract:** a discoverable, owner-bound withdrawal action should end the pending deliberation without another model call and should survive session reload.
- **Not established:** whether a live model would obey natural-language cancellation, whether the person would choose Ctrl+C instead, or whether any unwanted implementation could subsequently pass ordinary admission.

That distinction matters: a missing interaction contract should be made explicit, not presented as a previously documented API that has already been implemented.

## 2. Exact production source chain

### 2.1 A legitimate preference fork creates the modal pause

For `ask_once`, `finish` validates a concrete question, collects eligible alternatives, requires at least two displayed options, sets `Status="awaiting_values"` and `Asked=true`, retains the question/options and saves the thread. The chat converts this result to a `clarificationMsg` carrying the original request, intent and deliberation ID. [Question publication][finish] [Question adapter][question]

The UI's `clarificationMsg` case sets `isLoading=false`, installs `InputModeClarification`, saves the ID and question, chooses option zero initially and offers either option selection or a custom answer. A settled pause is not an executing foreground role. [Pause installation][pause]

### 2.2 Idle Enter selects the answer handler before ordinary submission

When the UI is not loading and no turn owns the foreground slot, Enter checks `InputModeClarification` first. If set, it invokes `handleClarificationResponse` and returns immediately. It does not invoke `handleSubmit`. [Enter dispatch][enter]

In normal input mode, `handleSubmit` recognizes a leading slash and calls `handleCommand`. That command dispatcher already has explicit `/quit`, `/exit` and `/q` branches returning `tea.Quit`. These are existing controls, not invented spellings used to manufacture a witness. [Normal submission][submit] [Exit commands][commands]

At the pause, `handleClarificationResponse` accepts typed nonempty text verbatim after outer whitespace trimming. It records that text in user history, captures the saved deliberation ID, clears the clarification UI, sets loading, and schedules `resumeDeliberation`. There is no command/control branch before this conversion. [Answer submission][answer]

### 2.3 The saved continuation treats the control text as preference evidence

`resumeDeliberation` loads the thread, requires `awaiting_values`, assigns the supplied string to `thread.Answer`, and calls `Service.Run`. It does not distinguish a values answer from an application control. [Resume adapter][resume]

The service recognizes exact candidate IDs or rendered option labels as a fast-path selection. All other text follows the refinement path: set running, save, check the next round's stop predicates, and collect a new round. The proposer frame carries the original request, original intent, prior round and this answer. [Exact recognition][choice] [Run transitions][run] [Frame construction][round]

For ordinary candidates `c1` and `c2`, `/quit` matches neither. With one completed round, a four-round cap and no exhausted no-progress counter, the next allowed round reaches `RunShard(..., Proposer, frame)`. The role then uses the configured JIT/model path, subject to its ordinary dependencies and bounds. The firm conclusion is **new reasoning is admitted before the requested exit command is interpreted as an application command**. A later model response is not needed to establish that boundary. [Round collection][round] [Role execution][agent]

This report does not assert that the model will produce a ready design. It might recognize the wish to stop, fail to produce valid JSON, return unresolved blockers or hit a bound. Those outcomes do not restore the lost command routing, and their frequency was not measured.

### 2.4 Available keys do not supply an in-chat withdrawal disposition

The global key handler checks Ctrl+C/Ctrl+D before other views and returns quit after shutdown. Keep this positive control: the user can exit. Ctrl+X cancels only inside `if m.isLoading`. The clarification handler has just made that false. Escape in ChatView returns immediately with no state change. [Keyboard controls][keys] [Pause state][pause]

The thread's supplied outcomes cover ready, waiting, unresolved values, policy stops and failure. The inspected chat adapter exposes resumption but no separate owner-bound values-withdrawal operation. That is the bounded scope of the missing contract claim. It does not claim that arbitrary filesystem edits, process termination or future undocumented integrations could not remove a thread.

### 2.5 Leaving the application and withdrawing a task are not the same act

`Pending` selects saved conversational threads still marked `awaiting_values`. The session-list loader uses it to restore the question. That persistence is useful and intentional; a person who merely closes the application may want the question later. [Pending discovery][pending] [Session restore][restore]

Do **not** fix the command mismatch by marking every unanswered thread cancelled on application shutdown. Conversely, a new explicit withdrawal action cannot merely hide the picker: if it leaves the same durable waiting record intact, a later load can restore it. The new action needs a terminal, owner-bound disposition if it promises to abandon the task.

## 3. Minimal unrun witness and controls

Use the normal chat entry path with initialized dependencies, an available foreground slot, ChatView and textarea focus, with no ready queued input and no focused error or logic pane. Keep source, permissions and mode unchanged. Choose propose or explicit mode, and obtain a genuine first-round values fork containing `c1` and `c2`. Use a four-round cap, zero unchanged count, normal proof/persistence operations and no simultaneous cancellation. The published question must have settled into `InputModeClarification`.

1. Put the exact string `/quit` in the input box and deliver a normal, non-paste Enter key event.
2. Observe the actual UI dispatch, returned scheduled commands and saved thread transition.
3. Record whether `handleCommand`'s quit branch was reached; whether a `tea.Quit` result was produced; whether the string entered `Thread.Answer`; and whether the proposer runner was called for round two.
4. Stop observation at that boundary. Do not require an unwanted edit or fabricate a favorable subsequent model answer.

**Current source implication, conditional on the listed dependencies succeeding:** the clarification handler schedules continuation; `/quit` becomes an answer; the thread is saved running; the round-two proposer is invoked. The immediate quit-command branch is not reached.

**Proposed required result:** the supported exit control remains a control at a values pause, schedules no further deliberation, and follows the product's ordinary exit semantics. Whether pending work is retained on exit must remain explicit and distinct from withdrawal.

Controls prevent exaggerated conclusions:

| Case | Expected distinction |
|---|---|
| Normal input mode, exact `/quit` | Existing command dispatcher returns quit. |
| Same values pause, Ctrl+C or Ctrl+D | Existing global exit remains available; no claim of entrapment. |
| Running deliberation, Ctrl+X | Preserve current owner cancellation and truthful stopping/settled messages. |
| Paused deliberation, Ctrl+X | Current branch does not cancel the paused question; proposed behavior needs an explicit product decision. |
| Legitimate displayed option | Continue the no-new-role-call exact-choice path and normal downstream admission. |
| Genuine free-text preference | Continue bounded refinement with original request and restrictions. |
| Literal path `/tmp/cache` or a discussion containing “quit” | Preserve as answer content unless the user chose an explicit control form. |
| Next round already forbidden by policy | No new role call is expected, but a policy-limit result is still not execution of the exit command. |
| Provider/JIT unavailable | An exit control must not become a provider-dependent error merely because it was routed as an answer. |
| Design-only `/deliberate` or `/innovate` | Exercise the same pause without any development worker; the routing finding still applies. |

The last case is useful because it isolates the control error from execution authority entirely. For automatically triggered deliberation, keep propose/explicit admission and original restrictions as additional negative controls. An answer, a withdrawal, and an exit must never manufacture approval.

## 4. Distinction from existing audits

The current tree and queue were screened, and the nearest relevant reports were read. This is a bounded deduplication claim, not a claim about every private discussion or historical branch. [Audit inventory][queue]

- **Queued slash-command identity** concerns a command accepted while busy and later dispatched as natural-language input; it explicitly excludes clarification mode. This case is an idle, correctly owned values pause where the control becomes a saved answer directly, without queueing. Fixing queue redispatch alone would not repair it. [Queued command audit][queued-audit]
- **Answer-evaluation recovery** starts with a real submitted values answer and a subsequent failed/interrupted evaluation. Its issue is losing the supported recovery route. Here the input was an application control before any legitimate answer evaluation was requested. Keeping failed answers discoverable would not repair that routing. Its explicit requirement not to reactivate cancellation is complementary. [Recovery audit][recovery]
- **Answered-context continuity** concerns retaining the question that gives a genuine answer meaning in later frames. This report stops before an application control should enter any frame. Carrying more history does not supply a control act. [Answered-context audit][context-audit]
- **Values-option admission** concerns selecting an unoffered candidate through the exact-match path. This witness uses text matching no candidate and follows free-text refinement. Offered-set validation does not fix it. [Option audit][option-audit]
- **Terminal-error clarification disposition** concerns a terminal diagnostic incorrectly creating a pause. Here the producer legitimately created a values pause; the defect is routing the person's next control submission into its answer channel. [Error-disposition audit][error-audit]
- **Side-turn cancellation ownership** and **foreground slot admission** concern cancellation while an execution exists. Here no foreground execution is active at the question, and submitting the control starts one. Preserve those existing ownership mechanisms rather than relitigating them. [Side-turn audit][side-audit] [Foreground operation][foreground]
- **Recurse values ownership** concerns choosing the correct mission or chat continuation. Use one correctly owned conversational thread here. Withdrawal must preserve mission-owner isolation, but changing discovery ownership alone does not create the missing control route. [Owner audit][owner-audit]

There is one core finding: modal answer capture lacks a distinct control branch. Durable withdrawal is the proposed completion of that boundary, not an additional claimed live defect.

## 5. Proposed contract, not implemented behavior

Start in the owning session/chat capability specification. Preserve the existing conductor, JIT roles, bounds, exact-choice replay, policy proofs and normal work admission. No new debating role, generic agent layer or background retry engine is needed.

### 5.1 Separate the three intents at the input boundary

Specify a small set of explicit, discoverable controls that remain available while a question is waiting. The existing exit command is a concrete compatibility requirement to decide and test. A dedicated “cancel this deliberation” action is the proposed additional capability.

An explicit UI control should create a typed event bound to the current session, question/thread and expected waiting state. Do not put its payload in `Answer`. Do not invoke the proposer, challenger, consultation manager or empirical probes to decide what a known button, key binding or exact reserved control means.

Do not send every slash-prefixed answer through arbitrary command execution. Absolute paths and command examples can be legitimate requirements data. Define a narrowly reserved control set and an explicit literal-input mechanism if a legitimate answer collides with one. A visible button/key is an alternative that avoids expanding slash parsing.

Natural-language “never mind” can be supported through the existing intent/judgment architecture, with a typed owner-scoped control result and conservative ambiguity handling. That is an optional extension, not a prerequisite for the deterministic control fix. A hand-written substring bank is not the recommended executive.

### 5.2 Keep exit, suspension and withdrawal distinct

The contract should state:

- **Exit:** stop the application's activity using its ordinary shutdown path. Retaining an unanswered question for a later session load is allowed and must not be described as cancellation.
- **Suspend or leave the question, if offered:** retain the continuation deliberately and make its status visible; define whether unrelated conversation can proceed.
- **Withdraw:** end this pending deliberation without selecting a design or authorizing its original work. Keep its history for explanation, but prevent normal pending discovery and exact-answer replay from reactivating it.

This report recommends explicit withdrawal for in-chat control; it does not choose an undocumented meaning for Escape or Ctrl+X. Assign the gesture deliberately and document it in the visible question controls.

### 5.3 Preserve host evidence and policy ownership

Go should record the actual user control, its owner and the current continuation state. Mangle should derive the terminal disposition from those facts, consistently with the repository's executive split. The role model must neither invent the control event nor decide whether it overrides a recognized withdrawal.

A proposed terminal record needs only bounded identity and disposition metadata: the existing session/thread, the question or continuation generation where needed, the user-act identity and the resulting state. Keep the original request, question, candidates and accumulated cost/history as historical evidence. None becomes approval or a new execution mandate.

A late answer or stale UI event must be checked against the current continuation before admission. Once withdrawal has been durably acknowledged, replay must not restore `running` or `ready` from an older waiting copy. Serialize the transition with resume through the existing owner where possible; add a storage revision check only where independent writers actually require it. Do not introduce a broad database or event framework without evidence it is necessary.

### 5.4 Settlement and durability must be honest

A paused thread normally has no participant operation to interrupt. Withdrawal can therefore be a local state transition rather than starting a model job and immediately cancelling it.

If an answer and withdrawal race, define the admission boundary. A withdrawal accepted before the next role starts blocks that start. If an operation already owns the slot, use its existing cancellation/settlement contract and prevent downstream adoption after revocation. Cancellation requested, operation settled and terminal state persisted are distinct observations.

If persistence fails, do not report that withdrawal will survive reload. In-memory admission should remain blocked for that owner while the UI reports that durability could not be established and offers an explicit recovery choice. Do not silently return the thread to answerable state or run a role to solve the storage failure.

Repeated controls should be idempotent. A stale event for an older thread must not withdraw the new question, a different session's task or a mission-owned continuation. Existing ownership filtering and work restrictions are invariants to preserve.

### 5.5 Legacy and recovery behavior

Old waiting records contain no withdrawal receipt. Their absence is not evidence that the user cancelled. Continue to restore them under existing ownership rules unless a supported terminal event says otherwise.

The answer-recovery proposal must not reopen a withdrawn thread merely because an evaluation was also interrupted. Recovery should distinguish “answer evaluation failed” from “the person ended the task.” A new explicit request may start a fresh deliberation; it does not erase the old disposition or retroactively mark it successful.

## 6. Failure modes and costs

No measured frequency, latency or saving is claimed. The current cost depends on where control routing fails and how quickly the user notices.

Let `B` be the bytes in the saved thread, `S` the total thread bytes scanned on restore, and `m` the length of submitted text.

- **Input dispatch:** checking a small exact control vocabulary costs O(m) parsing/normalization in host code. It need not add any model call to genuine values answers. A dedicated control widget can avoid textual ambiguity entirely.
- **Withdrawal persistence:** reusing the current snapshot writer costs O(B) serialization/write work plus its file sync and rename, not O(1) disk work merely because the new metadata is small. Additional metadata can be O(1) per terminal event if old history is not recopied into new records. Repeated controls should not repeatedly rewrite large history unnecessarily. [Current snapshot writer][save]
- **Restore:** the existing discovery path scans and loads thread files, with read work including O(S) bytes. A terminal filter must preserve correctness without claiming that this proposal makes restoration indexed or constant-time. [Pending scan][pending]
- **Kernel and integration:** the proposed event/disposition adds bounded facts and policy evaluation plus UI and persistence integration. Measure actual derivation/proof overhead rather than assuming it is free.
- **Misrouted control today:** a nonmatching answer with round budget can incur a full resumed proposer/challenger cycle, their JIT compilation, reads, consultations, empirical probes, policy/proof work and thread/session persistence. Later rounds and recovery may add more. A provider error may prevent those later costs; do not charge every possible call to every occurrence.
- **Model accounting:** count all participant and consultation calls, provider-reported reasoning and output, repeated bounded input, failed calls and retries actually made. The role tool-call cap does not by itself establish a total token or nested consultation-cost bound. Existing terminal-budget and consultation audits remain relevant. [Role loop][agent] [Round consultation][round]
- **Human cost:** count corrective submissions, searching for an escape, restarting/loading a session, and time until the requested control is accurately reflected. Do not score a model-written “cancelled” sentence as successful withdrawal without durable state and admission evidence.
- **Engineering/maintenance:** the price is a small control/state protocol, ownership checks, gesture documentation, race handling, literal-input compatibility and migration/recovery integration. A patch which routes all slash input to commands would be cheap but could create a new authority error and break valid answers.

Specific risks of a repair are false withdrawal of answer text, accidental execution of unrelated slash commands, loss of intentionally suspended work, cancelled-thread resurrection on reload, stale controls targeting a successor, repeated persistence/notifications, and falsely claiming a durable cancellation after a failed save. These are part of the acceptance contract, not optional polish.

## 7. Primary research and its limits

Horvitz's CHI 1999 paper treats efficient direct termination as a distinct requirement alongside dialogue for resolving uncertain goals. It supports the design distinction here: richer reasoning does not remove the need for direct control of the reasoning service. Its treatment of action cost and uncertainty also argues against spending another deliberation round to interpret a known control. This is a design application, not an experiment on codeNERD. [Principles of Mixed-Initiative User Interfaces, principle 6][horvitz]

Amershi and colleagues' CHI 2019 work separately identifies efficient dismissal, correction and communicating the consequences of actions. Those distinctions suggest separate evaluation outcomes for leaving a session, correcting a candidate and ending a pending task. The authors evaluated the guidelines across AI-infused products; their findings do not establish a failure rate, preferred key binding or performance improvement for this CLI. [Guidelines for Human-AI Interaction][amershi]

Neither paper substitutes for tracing the production path or testing cancellation, persistence and normal answers in this implementation.

## 8. Fair evaluation plan, entirely unrun

Compare the pinned implementation with two deliberately separate changes:

1. **Minimal control routing:** a narrow fix preserving the existing application-exit control at a values pause.
2. **Complete question control:** the same fix plus an explicit, owner-bound durable withdrawal operation.

Do not bundle better prompts, a different model, larger budgets or broader evidence tools into the treatment. Match model/provider settings, routing configuration, work mode, source revision, CodeDOM/world-model state, original request, saved question, candidate set, filesystem health and user input.

### Deterministic production-path cases

Use scripted participant seams only to obtain a known legitimate values fork. Drive the real input box, key handler, clarification state, scheduled continuation, persistence and session-load adapters. Inspect both the UI result and thread state; a unit test of a new cancellation helper alone does not show that the user can reach it.

| Proposed case | Required evidence |
|---|---|
| `/quit`, `/exit`, `/q` during a real values pause | Defined exit semantics; no values-answer assignment and no new participant/consult/probe call. |
| Global exit keys during the same pause | Existing working behavior retained. |
| Explicit withdraw while idle | Correct owner reaches terminal withdrawal; chat is usable; no role call and no selected design. |
| Reload after successful withdrawal | No pending picker restoration for that thread; historical record remains intelligible. |
| Ordinary exit without withdrawal | Pending question is retained or handled according to the separately documented exit contract. |
| Legitimate option or free-text preference | Original intent, restrictions, bounds and normal work admission preserved. |
| Literal command-looking answer | Explicit literal mechanism keeps it data; no false exit or unrelated command execution. |
| Duplicate withdraw / stale answer | Idempotent terminal result; no extra model work or resurrection. |
| Old question's control after new question/session | No cross-owner state change. |
| Answer admitted before withdrawal and reverse order | Defined ordering; no post-withdrawal adoption; existing running cancellation remains truthful. |
| Save/write/sync/rename failure | Honest durability result, no automatic answer continuation and no false durable-cancel claim. |
| Legacy record and mission-owned record | No inferred historical cancellation, no mission/chat ownership regression. |
| Read-only/design-only and propose/explicit/yolo controls | No new authority; no unrequested worker launch; yolo's normal no-question path remains a separate control. |

Use provider-unavailable and offline conditions for known controls. A deterministic exit/withdrawal control should not require a functioning inference service. Do not run live models merely to prove the handler dispatch.

### Behavioral study only after structural acceptance

A later, separately authorized user-facing evaluation could assess discoverability and ambiguous language such as “not that,” “skip this,” or “never mind the new API; keep compatibility.” Prelabel the intended act independently; distinguish withdrawing the whole task from rejecting one alternative. Measure false cancellations and unnecessary continuations, not just the rate at which the assistant says it stopped.

For end-to-end cost, count the original deliberation that produced the question plus all subsequent control handling, unintended/resolved reasoning, persistence and recovery. Also report the incremental cost from the pause to the correct control outcome. Include failures and user-abandoned trials. The target is the requested control outcome, not candidate readiness; cancelling a task successfully must not be scored as a failed design search.

### Checkable exit criterion

> A person at a genuine values pause can invoke the documented application exit and a distinct documented task-withdrawal control without either being converted into a preference answer. Exit follows its declared retention semantics. Acknowledged withdrawal prevents further reasoning/adoption for that continuation and remains terminal after reload. Genuine choices and constraints still resume the original task under unchanged admission rules.

This criterion is proposed. It has not been implemented or evaluated in this pass.

## Source links

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/daa3e2b60f42bdfe67c04f17aca7c6af76120803
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/agents.md#L1-L73
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L901
[conversation]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/architecture/conversation-first-agent.md#L44-L94
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/prompt/atoms/system/deliberation_proposer.yaml#L13-L30
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/prompt/atoms/system/requirements_interrogator.yaml#L13-L29
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/core/defaults/policy/deliberation.mg#L18-L74
[keys]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/model_key_handler.go#L20-L100
[enter]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/model_key_handler.go#L232-L265
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/model_handlers.go#L122-L146
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/commands.go#L42-L95
[pause]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/model_update.go#L264-L287
[answer]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/model_handlers.go#L441-L498
[question]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/deliberation.go#L53-L55
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/deliberation.go#L58-L93
[foreground]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/foreground_operation.go#L14-L49
[run]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/service.go#L74-L123
[round]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/round.go#L10-L73
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/outcome.go#L34-L74
[choice]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/outcome.go#L77-L86
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/read_tools.go#L82-L211
[save]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/persistence.go#L24-L56
[pending]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/internal/deliberation/persistence.go#L77-L108
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/session_persistence.go#L230-L275
[values-tests]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/deliberation_test.go#L179-L219
[stop-tests]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/cmd/nerd/chat/ctrlx_stopping_test.go#L23-L75
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/ASTRA_QUEUE.md
[recovery]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/deliberation-answer-evaluation-recovery.md
[context-audit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/deliberation-answered-context-continuity.md
[option-audit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/deliberation-values-option-admission.md
[error-audit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/terminal-error-clarification-disposition.md
[side-audit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/side-turn-ordering-and-cancellation-ownership.md
[owner-audit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/recurse-values-continuation-ownership.md
[horvitz]: https://www.microsoft.com/en-us/research/wp-content/uploads/2016/11/chi99horvitz.pdf
[amershi]: https://www.microsoft.com/en-us/research/wp-content/uploads/2019/01/Guidelines-for-Human-AI-Interaction-camera-ready.pdf


[queued-audit]: https://github.com/theRebelliousNerd/codenerd/blob/daa3e2b60f42bdfe67c04f17aca7c6af76120803/Docs/audits/queued-slash-command-kind-conservation.md#L3-L33
