---
doc-class: deep-dive
subsystem: campaign-deliberation-session
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 158be02e0641acd01b85d323405d256ccb6eb308
scope: stopped mission values questions and same-session restoration
---

# Recurse values questions need a durable continuation owner


## Current-main addendum: ownership isolation landed; continuation remains partial

Static review at public main `440276a52c52cbc8b003b5830ca70a5d708e5126`, November 3, 2026. No tests or application paths were run. The original analysis below describes its explicitly pinned pre-fix revision.

**Implemented:** [commit 97a4245](https://github.com/theRebelliousNerd/codenerd/commit/97a4245a50f1159901d2f89bff375fe41e7d5998) tags new mission threads with an owner, retains stopped values questions for that owner, and excludes mission-owned questions from ordinary session `Pending()`. New tagged mission questions no longer produce the historical chat-selector collision. Owner-specific lookup primitives also exist. [Mission settlement](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_mission.go#L244-L286) [Selectors](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/deliberation/persistence.go#L77-L172)

**Legacy records remain an unrun upgrade case:** pre-fix thread records lack owner fields. Current `Load()` unmarshals them without reconciling mission receipts; empty owner fields fail `IsMissionOwned()`. Two such pending mission records can still collide in ordinary `Pending()`; one can still be restored as a chat clarification. This is not a failure of the new-thread correction or an observed incident. [Load and selection](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/deliberation/persistence.go#L58-L108) [Ownership predicate](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/deliberation/types.go#L73-L101)

**Continuation remains partial:** bounded repository-wide searches found `PendingForOwner` and `PendingMissionThreads` in definitions and tests, but no production callers. The inspected session loader calls conversational `Pending()`; mission completion displays the question as text, and the generic design-only answer returns a note without updating or restarting the stopped mission. The new authored fixtures cover tagged isolation and lower-level lookup, not a legacy migration or owner-addressed user journey; this review did not execute them. [Session loader](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/cmd/nerd/chat/session_persistence.go#L251-L285) [Answer handler](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/cmd/nerd/chat/deliberation.go#L70-L93) [Mission fixtures](https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_audits_test.go#L388-L482)

A future migration should require validated receipt-to-thread linkage or explicitly retain unresolved ownership; `DesignOnly` alone is not proof of mission ownership. Test legacy, mixed chat/mission, duplicate-answer and crash-recovery cases. Answering a preference must remain separate from authorizing execution.


## Decision and finding

Keep mission values decisions durable, but do not equate every persisted `awaiting_values` thread with the single active chat clarification. A stopped mission, a live conversational question, and a design-only historical thread need explicit ownership and restoration semantics.

**Source-supported conditional finding:** two sequential, explicitly started recurse missions in the **same TUI session**, each ending on a legitimate values fork, can leave two deliberation threads marked `awaiting_values`. Mission completion settles its execution receipt but does not settle or transfer the thread's question. The later session-list load scans all deliberation files and rejects more than one pending thread; its caller logs that error and restores neither question. The files and mission questions remain on disk. This is an availability and answer-ownership gap, not physical data loss, automatic mission resumption, or an established permission bypass. [Thread producer][mission-thread] [Pending discovery][thread-load] [Session restoration][session-load]

With just one such mission thread, loading the session instead installs it as an ordinary clarification. A valid option answer can finish the design thread, but `DesignOnly` makes the chat resume path return a note; it does not update or resume the stopped mission. That safe no-execution behavior must remain. The missing contract is how the decision is explicitly associated with its stopped mission and, if requested, a newly admitted run. [Answer routing][answer-route] [Design-only return][answer-run]

This review inspected public source and test definitions only. No tests, builds, application execution, model calls, fault injection, or benchmarks were run. The sequences below are source-derived witnesses, not observed production incidents. Root, campaign, and chat guidance were reviewed. All CodeNERD links are pinned to the commit above; external references supply design principles, not evidence that this code was executed.

## Implemented behavior and the seam between owners

The mission specification says a values fork pauses the mission with a saved question instead of silently choosing a preference. The session specification says pending values questions retain their original intent and are restored on session loading. Both are useful contracts individually; their composition presently assumes a single answer owner without enforcing that assumption when missions create threads. [Mission contract][mission-contract] [Session contract][session-contract]

| Boundary | Implemented at the pin | Remaining limitation |
|---|---|---|
| Thread identity and persistence | Thread has ID, session, original request/intent, mode, design-only flag, rounds, question/options/answer and status. Saves write a complete snapshot through a temporary file. | No explicit typed continuation-owner kind or mission-owner field participates in pending discovery. |
| Mission-to-thread relationship | Mission receipt records thread ID/path; the request body also contains mission context. | Those links are not consulted by the session's pending-question selector or generic answer path. |
| Values admission | Mangle selects a values fork and `ask_once`; the conductor validates a concrete question and at least two eligible alternatives. | A legitimate mission question is stored using the same pending status as a resumable chat question. |
| Mission settlement | The mission records its question, stop disposition and receipt; unexecuted pending work is cleared. | The child thread stays `awaiting_values` after the mission run has ended. |
| Session restoration | Exactly one pending thread is restored; multiple candidates are refused rather than arbitrarily selected. | Ambiguity is only logged by the caller, leaving no question-selection or recovery surface in this path. |
| Answer handling | The saved thread, original intent and exact option are reused; design-only results do not execute. | A mission-origin answer is not delivered to a mission continuation owner. |

The thread schema, save/load checks, mission handoff, and answer path establish these distinctions. There is retained provenance; the issue is not that mission and thread have no relationship anywhere, but that the relationship does not govern restoration or answer disposition. [Schema][thread] [Atomic snapshot writer][thread-save] [Mission handoff][mission-thread] [Resume][answer-run]

## Production lifecycle, without concurrency or a crash

### 1. Both missions can use one actual session

The chat service is constructed with `Session: m.sessionID`. Each mission-enabled recurse invocation builds a fresh `RecurseMission` from that service. The recurse driver copies the service to bind the authoritative kernel but preserves its session; it acquires one workspace loop lease and releases it on return. A mission begins with a fresh UUID and its own state file under that session. [Chat service][chat-service] [Mission adapter][mission-adapter] [Driver ownership][cycle-lease] [Fresh mission identity][mission-new]

This witness is **not** two separate CLI processes assumed to share a session. It uses sequential commands in one initialized TUI session, with successful ordinary dependencies, clean/reconciled source, available leases, adequate budgets and known usage. Each invocation reaches a selected attempt and obtains valid proposer/challenger results with two viable values alternatives. No unrelated earlier stop wins the policy's ranked stop decision.

The second invocation need not use the same mission text. Even an explicit new mission with clarified text gets a fresh conductor and new thread; the old question is not superseded by that fact alone.

### 2. The values stop is legitimate and safe

Mission preparation always creates its design thread with `Mode: "propose"` and `DesignOnly: true`. Thus this path does not silently select the recommendation merely because the surrounding chat normally uses yolo. With a first values fork, policy derives `ask_once`. The conductor records `awaiting_values`, `Asked=true`, the question and eligible options, then saves. [Mission thread creation][mission-thread] [Disposition rules][disposition] [Question validation and publication][values-save]

The mission copies the question/options, emits them as progress, asserts the design disposition and asks mission policy again. Under the stated budget conditions, policy selects `/values`. The mission appends its unexecuted receipt, clears `MissionState.Pending` and saves. It does **not** change or resave the deliberation thread's pending status. The mission end routine persists the stopped run and retracts transient mission judgments, not the durable thread. [Values stop][mission-thread] [Stop ranking][mission-policy] [End routine][mission-end]

The recurse attempt returns from failed preparation before calling its configured executor. Its journal records that the design was not executed and its in-flight marker is removed. This demonstrates the mission-attempt execution barrier for the witness, not a blanket assertion that every possible deliberation tool is incapable of effects. [Execution boundary][before-execute]

### 3. A normal explicit next invocation remains possible

Recurse completion sets `m.recurse=nil`. Its result summary prints mission status, state path and the saved question/options as ordinary text; this route does not produce the chat's typed `clarificationMsg`. The next start rejects a still-live recurse or campaign but does not test for an old pending deliberation thread. [Completion][loop-finish] [Summary][mission-summary] [Start checks][restart-admission]

The cross-session interruption guard is real and important: it reads mission state files and refuses a new run if any has a non-nil execution receipt. In this values-stop path the receipt was explicitly settled and cleared, so that guard does not refuse the next mission merely because the old child question remains unanswered. There is no concurrency requirement and no need to delete, corrupt or hand-edit a state file. [Interrupted-work guard][interrupted]

After a second ordinary values stop, the directory therefore contains two different thread files for the same session, both `awaiting_values`, alongside two correctly stopped mission records. Normal successful storage and cleanup suffice.

### 4. Explicit session loading restores neither question

Use the session-list load path after switching away and back to the original session, with its ordinary history available. `loadSelectedSession` loads history and installs the selected session before calling `m.deliberator().Pending()`. Its preceding conversation detach clears the old clarification and resets normal input, so the ambiguity branch does not retain an earlier question as a substitute. [Conversation detach][detach]

`Pending` examines every JSON thread in that session directory, validates its ID/session through `Load`, and selects by status alone. At the second `awaiting_values` thread it returns `multiple pending deliberations in session`. It does not distinguish a mission-owned thread from a chat-owned one, use the receipts to resolve ownership, or return a list. The caller logs the failure and skips the branch that installs clarification state; normal session loading otherwise continues. [Pending selector][thread-load] [Caller behavior][session-load]

This report does not claim every startup path calls this selector, that the entire session fails to load, or that the original question text is erased. The precise observable expectation is failure to restore a selected pending answer interaction through this explicit load path. A legitimate chat clarification created later can also join this ambiguous set; its presence is not required for the two-mission witness.

## Single-question behavior and limits

A single mission-origin pending thread passes the cardinality check and becomes a normal clarification after explicit session loading. Selecting one of its exact saved options loads that thread, supplies the answer, and replays its challenged round. A ready design-only result returns an assistant note. No `RecurseMission` is reconstructed, no mission snapshot is updated by this handler, and no attempt executor is invoked by that design-only return. [Option replay][answer-replay] [Resume handler][answer-run]

That behavior is not evidence of lost user authorization: **a preference answer is not execution permission**. It is also not evidence that the mission has been resumed or its requirement satisfied. The mission remains a historical stopped run unless a separate, explicitly admitted operation is started. New missions may receive newly supplied context, but the inspected adapter does not automatically consume a selected old decision as a typed continuation.

Counterexamples and boundaries matter:

- Ready, failed, unresolved-values, round-cap and other non-`awaiting_values` statuses do not enter this selector's pending set.
- Questions in different sessions do not collide here.
- Answering the one restored thread successfully before creating another pending question can remove that particular collision.
- Ordinary foreground chat deliberation has a working saved-question/option path; this review does not relabel it as absent.
- Earlier budget, source, dependency, storage or policy failures can prevent the two-`/values` sequence. They are controls, not explanations for changing the stated successful prerequisites.
- Workspace serialization prevents simultaneous recurse loops, but serial runs still accumulate pending questions.
- Files remain inspectable and the lower-level service can load a known thread ID. Lack of automatic restoration is not permanent irrecoverability.
- This audit has no claim of remote mutation, weakened host approval, arbitrary tool execution, falsely passing tests, or an accepted mission requirement.

Existing authored tests already cover a normal saved chat question answered without a new model call, its subsequent absence from `Pending`, and continued ordinary work approval. Mission tests cover values stopping before attempt admission, persistence of the question and cleared pending execution state. The inspected tests do not compose two mission values stops with the production session-load selector. They are positive source evidence, **not rerun results**. [Normal continuation test][normal-resume-test] [Mission stop test][values-test] [Interrupted-work control][interrupted-test]

## Relationship to existing work

The portable recurse design proposes durable requirements, bounded human questions and explicit authority separation. This audit narrows that design with an exact implemented lifecycle: mission completion leaves a child pending, and a session-wide singleton selector cannot restore the resulting set. It does not propose another proposer/challenger framework. [Portable design][portable-prior]

The terminal-error clarification audit concerns diagnostic text manufacturing a pause with no actual pending subject. Here a genuine, policy-derived question is durably saved; the problem is its owner and lifecycle after a mission stops. No diagnostic keyword or malformed model output is needed. [Earlier terminal-disposition scope][terminal-prior]

The current audit queue, mission/session specifications, deliberation budget notes and targeted searches across audits, journeys and QA were screened for pending-question cardinality, mission questions, `awaiting_values` and restoration. No exact predecessor was identified in that bounded screen. This is not a claim of exhaustive historical novelty or equivalence across all branches. [Current queue][queue]

## Proposed portable continuation contract

The smallest repair should choose one product meaning explicitly, rather than infer it from status strings:

1. **Stopped-run / historical-question model:** a mission ends at the values boundary. Its thread is durably classified as mission-owned history, not as the active chat clarification. The user can inspect and answer the identified decision; applying that answer requires a new mission run with a new resource envelope and explicit decision lineage.
2. **Paused-owner model:** a mission remains a recoverable, non-executing decision owner. It registers a bounded continuation, and the host exposes a selector when multiple owners need attention. Answering updates that owner only; any subsequent execution still needs fresh admission, budgets and source checks.

Either can be correct. Do not implement a silent newest-file-wins selector, automatically choose a recommendation, or erase older questions to satisfy a singleton API.

A language-neutral durable decision envelope should identify workspace/session, owner kind and owner ID, mission version, thread and question revision, originating node/attempt snapshot, eligible option IDs, response disposition and any explicit supersession link. Existing mission receipts can supply much of the provenance; do not treat a JSON-looking request string or path substring as authority. Store the selected preference separately from permission and from empirical requirement evidence.

Proposed invariants:

- Every pending question has exactly one declared answer owner; every presented answer names that owner and question revision.
- Multiple pending owners are either explicitly disallowed at creation or recoverable through a truthful selection surface. Discovery must not turn an ambiguity into “nothing pending.”
- Ending a run gives its pending decision an explicit disposition: retained-for-owner, transferred, cancelled, superseded or resolved. It never implicitly reopens execution.
- Exact duplicate answers are idempotent; stale, mismatched and already-resolved answers cannot update another owner.
- Source/vision changes invalidate execution reuse, not the historical record that the person expressed a preference.
- Policy continues to decide mission stop/admission and valid alternative selection. Host code validates identities, persists transitions and routes answers. A model or advisory assessment may explain choices but cannot forge acceptance or authority.
- CLI, TUI and future codebase adapters share this decision schema. No Go package path, language-specific test convention, or UI-only singleton is needed to identify an owner.

Because mission and thread snapshots are separate writes, specify partial-update recovery rather than assuming cross-file atomicity. A durable transition intent plus idempotent reconciliation, or an equivalent explicit ownership index, can make a crash between writes recoverable. Unknown legacy ownership must remain visible; migration should not delete evidence or guess from timestamps. This is proposed work, not an implemented transaction protocol.

## Primary grounding

The W3C SCXML recommendation defines state transitions, hierarchical ownership and invoked child lifecycles, including cancellation when their owning state exits. It is useful vocabulary for specifying whether a mission values state is terminal, suspended or transferred. It does not require adopting an XML runtime, and it does not prove this Go implementation correct. [SCXML recommendation](https://www.w3.org/TR/2015/REC-scxml-20150901/)

W3C PROV-DM distinguishes entities, activities, responsible agents, derivation and invalidation. The relevant design inference is to retain the distinction between a preference decision, the run that requested it, and later execution evidence; sharing a session is not equivalent to sharing an owner. [PROV-DM recommendation](https://www.w3.org/TR/2013/REC-prov-dm-20130430/)

Horvitz's primary mixed-initiative UI paper emphasizes efficient clarification, direct invocation/termination, refinement and memory of recent interaction. Applied here, a question should remain both attributable and actionable without forcing users to repeat deliberation or silently restarting work. This is a design rationale, not an empirical claim about CodeNERD's users. [Horvitz, CHI 1999](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/11/chi99horvitz.pdf)

## Cost and fair, unrun evaluation

An ownership field or selector need not add a model call. The cost is chiefly schema/migration work, durable transition I/O, recovery logic, UI attention and retained history. Current discovery reads all JSON thread files, so its work grows with thread count and retained round size; an index may reduce repeated reads but introduces synchronization and rebuild costs. Measure rather than assume an efficiency gain.

A fair evaluation must hold mission text, candidate/assessment content, permissions, source snapshots, budgets, policy, and gate outcomes constant. Compare the current implementation with the selected ownership contract, not with a treatment that succeeds by auto-approving work or dropping unresolved questions. All evaluations below are **proposed and unrun**:

| Fixture | Required evidence |
|---|---|
| Two sequential same-session mission values stops, then explicit session-list reload | Both questions retain correct provenance; the chosen contract restores/selects them or explicitly retires them without ambiguity disappearing into a log. |
| One mission question plus one ordinary chat question | The chat answer cannot accidentally resolve the mission; mission history cannot suppress the active conversation question. |
| One mission question answered after reload | Exact decision is retained for its owner; no mission execution or acceptance follows solely from the answer. |
| Explicit new mission using the prior decision | New run ID, budget and source admission; explicit decision lineage; old mission state remains historically truthful. |
| Different sessions and ordinary ready threads | No cross-session selection; completed history does not become pending. |
| Duplicate, stale and cancelled answers | Idempotent or explicit refusal, with no owner reassignment and no effect dispatch. |
| Crash between mission/thread/index updates | Reconciliation exposes incomplete transitions and preserves the question; no timestamp-based authority invention. |
| Unknown-spend, time-bound, source-drift and interrupted-execution controls | Existing stop/recovery refusals remain intact; no repair of answer routing weakens execution barriers. |
| Go, Python, JS/TS, Rust and a generic configured project | Equivalent decision ownership independent of codebase adapter; unavailable execution capability remains unavailable. |

Start with deterministic conductor outputs and real persistence, mission stop, completion handler and session-load paths. Count actual executor invocations: the values-only and answer-only cases must not invoke mission execution. These fixtures test lifecycle wiring, not a model's ability to identify the right requirement. A separately authorized live study can measure legitimate question quality and user recovery behavior.

Report restored-owner accuracy, unresolved-but-hidden decisions, wrong-owner updates, stale-answer acceptance, prohibited execution attempts, successful authorized resumption/new-run completion, extra user turns, repeated model rounds, input/output tokens, disk reads/writes/bytes, p50/p95 restoration latency and reviewer minutes. Include the cost of refusals and manual recovery. Keep latency, tokens, storage and human time separate unless disclosed conversion assumptions justify a combined monetary estimate.

The acceptance target is modest but consequential: **a legitimate unresolved requirements decision survives its producing run with a known owner, and answering it changes only the state the user actually chose—never execution authority by implication.**

[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L110
[session-contract]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/architecture/session/IMPLEMENTED_SPEC.md#L867-L882
[thread]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/deliberation/types.go#L71-L89
[chat-service]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/deliberation.go#L18-L50
[mission-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/campaign_recurse.go#L196-L218
[mission-new]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission.go#L99-L171
[mission-thread]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission.go#L240-L279
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/core/defaults/policy/mission.mg#L8-L26
[disposition]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/core/defaults/policy/deliberation.mg#L43-L74
[values-save]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/deliberation/outcome.go#L16-L74
[thread-save]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/deliberation/persistence.go#L12-L55
[thread-load]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/deliberation/persistence.go#L58-L107
[mission-end]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission_policy.go#L75-L106
[interrupted]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission_storage.go#L17-L49
[cycle-lease]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L338-L367
[before-execute]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1150-L1172
[mission-summary]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L178-L186
[restart-admission]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/campaign_recurse.go#L157-L189
[loop-finish]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/campaign_recurse.go#L368-L407
[session-load]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/session_persistence.go#L229-L304
[answer-route]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/model_handlers.go#L474-L504
[answer-run]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/deliberation.go#L70-L93
[answer-replay]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/deliberation/service.go#L81-L95
[normal-resume-test]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/deliberation_test.go#L179-L218
[values-test]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission_policy_test.go#L196-L262
[interrupted-test]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission_policy_test.go#L160-L178
[portable-prior]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/portable-recurse-deliberation-crucible.md
[terminal-prior]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/terminal-error-clarification-disposition.md#L1-L19
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/ASTRA_QUEUE.md
[detach]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/session_owner.go#L51-L60

