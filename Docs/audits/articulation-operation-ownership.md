---
doc-class: deep-dive
subsystem: chat-articulation-session
implementation-status: planned
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Articulation operation ownership and restriction continuity

## Decision

Give a foreground conversation operation one owner through streaming articulation, specialist consultation and final synthesis. Returning a Bubble Tea message should transfer that ownership rather than end it. Preserve the operation's cancellation handle and recognized restriction across every independently lived context, and settle the operation before releasing its current-turn slot.

The current feature branch has two gaps at the same handoff. First, ordinary streaming articulation outlives the function that owns the foreground cancel; Ctrl+X can consequently display “Stopped” without reaching the stream's context. Second, that stream starts from the session shutdown context rather than the restricted foreground context. A subsequent knowledge consultation can therefore start without a correctly recognized parent restriction. The existing provider terminal checks, response-control admission and session permission gates remain useful and must stay intact.

**Evidence status:** static source and test-definition inspection only. No codeNERD execution, tests, builds, model inference, failure injection, performance measurements or implementation changes were performed. The schedules and fixture outcomes below are proposed source-derived witnesses, not reproduced incidents. This report does not establish the cause of a particular user's failed investigation, the frequency of these outcomes, or a universal forbidden tool effect.

## Source freeze and branch distinctions

The following heads were inspected and rechecked unchanged at approximately 19:10 UTC on October 28, 2026:

| Branch | Source pin | Role |
|---|---|---|
| `main` | [`356f65e0e6bb639adaa90ab36212daa714114871`][main] | Baseline implementation |
| `feat/conversation-brief` | [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`][feature] | Active implementation with foreground cancellation and typed restrictions |
| `codenerd-research-docs` | [`c592ff223cd207fca4c29ae793d5eba57aa4781d`][historical-research] | Historical research and adjacent contract proposals |
| `research-main-20261028` | [`0c38d4fc987213a17f6570cc7d8dae34c87d8ace`][current-research] | Current research inventory |

Unless explicitly labeled main, implementation references below use the feature pin. Main already creates shutdown-rooted streaming contexts. It does not have the feature's foreground `turn.begin` and `turn.end` registration or its typed request-restriction integration. Thus the restriction handoff finding qualifies the feature's new guarantee; it is not a claim that main previously carried a descriptor that this branch removed. The feature's new stop mechanism is also real on the foreground paths that retain its registered context. [Main streaming handoff][main-stream], [feature foreground owner][foreground-owner]

The repository assigns executive decisions to Mangle and requires source-first wiring inspection, spec-first changes and normal-entry validation. The relevant root, chat, perception, articulation, core and session guidance was read. The design below keeps policy decisions in the executive, transport and lifecycle mechanics in Go, and creative response generation in the model. It does not propose a second permission engine or a replacement planner. [Repository contract][guidance], [session guidance][session-guidance], [articulation guidance][articulation-guidance]

## The production operation crosses two lifetimes

The normal chat path does not remain inside one synchronous executor call. `processInput` is a Bubble Tea command. It performs perception and routing, then can launch an articulation goroutine and return a `streamStartMsg`. Update subscribes to that message's channels while the goroutine continues. This deliberate split prevents the command's deferred cancellation from immediately aborting its new stream. [Foreground owner][foreground-owner], [stream lifetime comment and construction][stream-context], [message return][stream-return], [Update subscriptions][stream-update]

The remaining operation can do more than emit text. After successful final response processing, it applies admitted control data. It may then launch up to two specialist consultations, collect their results, and deliver `knowledgeGatheredMsg`. Update persists consultation knowledge and schedules a separate final synthesis call. This makes operation ownership relevant to effects, additional model work and evidence delivery, not just a spinner. [Control and consultation sequence][post-articulation], [consultation implementation][consultation], [synthesis handoff][knowledge-update]

## Finding one ordinary streaming loses its foreground cancel

### The source chain

At command entry, `processInput` derives `ctx, cancel` from `shutdownCtx`, registers `m.turn.begin(cancel)`, and defers both `cancel()` and `m.turn.end()`. The shared `turnState` stores the cancel so foreground stop can reach it. [process.go 70 to 83][foreground-owner]

Before streaming, the command creates another context from `shutdownCtx`, optionally attaches the usage tracker, and captures `streamCancel` only in the new goroutine. It then returns `streamStartMsg`. Returning executes the original deferred cleanup. `turn.end()` clears the shared cancel and working label, even though articulation is still running. The message carries four channels, not the operation owner or stream cancellation handle. [process.go 918 to 937][stream-context], [process.go 1042 to 1047][stream-return], [turnState methods][turn-state], [stream message shape][stream-types]

Ctrl+X checks `m.isLoading`, calls `m.turn.stop()`, asserts `interrupt_requested`, sets loading false, and displays “Stopped.” The handler does not inspect `stop()`'s boolean result. Once `turn.end()` cleared the handle, `stop()` returns false and has no route to the still-live `streamCancel`. Ctrl+X also leaves `isStreaming` unchanged in this branch. [Ctrl+X handler][ctrlx], [turnState stop][turn-state]

### A single operation is sufficient

The minimal conditional schedule is:

1. An ordinary request reaches the articulation path rather than an earlier response or delegated-work return.
2. The provider remains pending after the command returns `streamStartMsg`; its shutdown parent remains alive.
3. The TUI applies that message and the person presses Ctrl+X before the provider completes.
4. The shared foreground cancel has already been cleared. The UI reports a stop, but the provider's `streamCtx` was not canceled by that stop.

No second foreground request, delayed side-turn classification, queue race or cancellation-ignoring provider is needed. The defect is the ownership handoff itself. If a later request is admitted because Ctrl+X made the interface idle, the existing unowned stream messages create additional interference risks, but those races are not necessary to establish this finding.

If the provider subsequently returns a successful terminal result, the goroutine can continue through control application and consultation dispatch. The call to `applyArticulationControlPacket` and the later knowledge-request branch do not consult the Ctrl+X state or an owned stop latch. [Post-articulation sequence][post-articulation]

One bounded example is an otherwise-admitted memory operation. Chat calls the shared scoped memory implementation; `note` can replace a session note, and `promote_to_long_term` can call the durable store. These functions do not receive a cancellation context. Successful application remains contingent on valid controls and a working kernel/store. This is a possible post-stop effect under otherwise-valid controls, not a claim that the model can bypass arbitrary tool authorization. [Control outcome adapter][memory-adapter], [memory effects][memory-effects]

The `interrupt_requested` fact is an existing continuation safeguard. Its relevant policy consumer derives `continuation_blocked(/user_interrupted)` for the CodeDOM continuation policy; it does not cancel the detached stream or intercept the calls just traced. [Continuation interruption policy][continuation-policy]

### The synthesis phase repeats the ownership issue

Knowledge consultation uses `streamCtx`, so it inherits the detached stream's stop problem. After consultation, `handleKnowledgeGatheredMsg` schedules `synthesizeWithKnowledge`. That function creates a fresh shutdown-rooted timeout context and does not register a foreground cancel. Repairing just the first streaming call would leave final synthesis outside the same operation owner. Its configured timeout is useful resource protection, but it does not implement the person's current stop request. [Consultation spawn][consultation], [knowledge result handling][knowledge-update], [synthesis context][synthesis]

## Finding two the streaming context loses the recognized restriction

### The feature has genuine restriction enforcement

Feature `processInput` merges the restriction obtained from perception with the Jev-derived restriction and attaches it to the foreground context. The accompanying comment explicitly says delegations and subagents should inherit it. The immutable restriction type carries supported kinds and merges monotonically. Session execution combines the intent's restriction with its context's restriction, asserts turn-owned facts, narrows the catalog and binds calls to the restricted turn. These mechanisms are implemented and must not be described as absent. [Foreground restriction attachment][restriction-attach], [restriction carrier][restriction-type], [executor merge and publication][executor-restriction], [turn restriction adapter][turn-restriction]

### The consultation receives a different context ancestry

The later stream construction starts from `shutdownCtx` or `context.Background()`, not the `ctx` that carries that restriction. It only reapplies the usage tracker. Assuming the ordinary shutdown root does not independently contain the current request's restriction, `RestrictionFromContext(streamCtx)` is therefore zero. The `intent` value still contains the restriction in the articulation closure, but that does not attach it to the child consultation. [Stream construction][stream-context], [context carrier semantics][restriction-type]

When the completed response contains admitted `knowledge_requests`, chat calls `handleKnowledgeRequests(streamCtx, ...)`. That handler selects each specialist, constructs a synthetic knowledge task, and passes the received context through `spawnTaskWithContext`. The spawn adapter adds model configuration but does not recover the parent request's restriction. [Knowledge dispatch][post-articulation], [consultation spawn][consultation], [spawn adapter][spawn-adapter]

The task request contains its persona/verb and text. Its preset intent has category, verb, target and other classification fields, but no inherited restriction. The inline executor deliberately uses that preset instead of re-perceiving the synthetic task. Session merges the preset restriction and the supplied context restriction; neither supplies the lost parent descriptor on this route. [Task request adapter][task-request], [preset construction][preset], [inline execution][inline], [session merge][executor-restriction]

For an unenforced zero restriction, `assertTurnRestriction` asserts nothing, `restrictionWithheldTools` returns no additional withheld tools, and `bindRestriction` does not attach a restricted-turn identity to a call. This explains why the presence of the new enforcement machinery cannot itself recover a descriptor discarded before entry. [turn_restriction.go][turn-restriction]

### Exact consequence and limits

The established result is loss of the parent's typed restriction before an actual child execution. It does not prove that every consultation writes, executes a command, skips approval or otherwise violates the user instruction. A child's normal envelope may already exclude effects; its own interpretation may independently impose restrictions; ordinary exact permission and approval policy may deny a call; or the model may simply answer without a tool.

The missing invariant is narrower and checkable: a correctly recognized parent restriction must reach the consultation's catalog and final admission without depending on the model repeating it in synthetic task prose. A fixture should observe the child context, catalog and registered-handler invocation counts rather than infer enforcement from a benign answer.

The late synthesis call currently generates text rather than hosting a native tool loop. Its newly rooted context is still an ownership defect, but the specific child-tool restriction consequence above belongs to consultation. This report does not ascribe a tool-execution capability to synthesis that the inspected implementation lacks.

## Existing safeguards and counterexamples

The following behavior should remain explicit during remediation:

- **The original lifetime fix is correct in purpose.** Replacing `streamCtx` with the original foreground `ctx` while preserving its command-return `defer cancel()` would revive the immediate-cancellation failure documented in the source comment.
- **Shutdown remains connected.** The stream derives from `shutdownCtx`; Ctrl+C and Ctrl+D invoke shutdown. That does not make Ctrl+X equivalent to shutdown. [Exit keys][ctrlx]
- **Successful terminal evidence remains mandatory.** Articulation waits for a typed successful terminal result before response processing. Missing, truncated or failed terminal results return an error; they do not follow the successful control path. [Terminal consumer][terminal-consumer]
- **Control proposals remain governed.** The chat consumer filters model observations and validates candidate actions. Memory operations remain scoped with recorded outcomes. This finding does not authorize weakening those checks. [Control consumer][control-consumer], [memory adapter][memory-adapter]
- **Earlier foreground work is a positive control.** Delegation while the command retains its registered context can receive cancellation. The bug is not that feature Ctrl+X has no working path.
- **Absent consultation narrows the consequence.** A stream with no knowledge request has no child-tool restriction handoff, though its ordinary stop ownership still needs repair.
- **Completion before stop is different.** If an effect was already committed when the stop was received, cancellation cannot undo it. The design must report it honestly rather than claim rollback.
- **Other independent failures can stop progress.** A provider error, denied tool, missing backend or shutdown can prevent later work despite the missing foreground stop connection.

## Relationship to existing research

The current and historical research inventories were checked before selecting this boundary. This proposal should extend their existing contracts rather than create a competing lifecycle framework.

| Existing research | Already established boundary | Added evidence here |
|---|---|---|
| [Side turn ordering and cancellation ownership][side-audit] | Delayed stop targets newer work; pre-start cancellation; old cleanup clears newer ownership; queue ordering | A single normally streaming request loses its cancel when its command returns, without any overlapping side turn |
| [Perception restriction continuity][restriction-audit] | Main projection gaps; feature enforcement; detached background and continuation carriers lose restrictions | Foreground articulation creates another detached context, and model-requested consultations receive it |
| [Background outcome and continuation ownership][background-audit] | Background stop latching, successor loss, origin-session ownership | Ordinary foreground articulation and its consultation/synthesis phases, not background job completion |
| [Stream completion contract][stream-audit] | Provider framing and typed terminal completion, with implemented consumer checks | The correct terminal protocol still needs the operation's stop signal connected to its context |
| [Terminal knowledge synthesis grounding][synthesis-audit] | Parent grounding and consultation coverage can disappear from final synthesis | Cancellation ownership and restriction ancestry before consultation; no duplicate grounding proposal |
| [Articulation admission and control fidelity][admission-audit] | Envelope admission and control outcome fidelity | A valid admitted envelope can arrive after a stop that never reached its operation |

This is not an exhaustive claim about every historical document. The inspected adjacent reports do not trace the normal `processInput` return as the point that simultaneously retires foreground cancellation and leaves a differently rooted articulation/consultation operation alive.

## Proposed ownership contract

### Own the accepted operation until settlement

Use the existing foreground ownership work to represent an operation with an immutable identity, accepted/start/stop-requested/settled state, a cancellation handle and a restriction snapshot. Streaming, consultation and synthesis are phases of that operation. The command's return should transfer phase ownership; it should not retire the operation.

The UI should distinguish “stop requested” from terminal settlement. Its owner-qualified terminal event may release the foreground slot or let policy admit other work. A message from an older operation must not change a newer operation's busy flag, stream content, cancel handle or history destination. This extends the existing side-turn ownership contract rather than introducing another scheduler.

### Preserve request data when contexts need independent lifetimes

There are two reasonable implementation shapes to evaluate:

1. **Operation-owned context:** create the cancellable operation before asynchronous dispatch, pass it through every phase, and retire it only when the operation settles. Command-local cleanup must not cancel transferred work.
2. **Explicit phase transfer:** keep independently lived phase contexts, but transfer their cancel ownership without a gap and explicitly reattach the immutable operation restriction and other required request metadata. A stop received during transfer must latch and prevent the new phase from starting normally.

The first shape has fewer ancestry changes; the second can preserve existing channel architecture with a smaller local refactor. Both need an owner-qualified settlement boundary. Merely putting `streamCancel` in one field fixes neither a stop arriving during transfer nor the later synthesis reroot. Merely preserving context values does not establish cancellation ownership.

Do not solve this by indiscriminately discarding cancellation with `WithoutCancel`, by passing a soon-to-be-canceled command context, or by treating a new context as new authority. Parent restrictions can only remain or narrow through child work. The existing exact permission and request-bound approval checks remain necessary even when a restriction is absent.

### Settle stop against effects and dispatch

Define a linearization point for stop acceptance relative to control application and consultation dispatch. After the operation has accepted a stop, it should not admit a not-yet-started consultation or apply a not-yet-committed response control. A provider that returns success despite cancellation must not silently reopen those phases. Preserve the result of any effect already committed before the stop, and avoid replaying effects to repair a failed display or persistence update.

Go should carry identities, invoke cancellation, publish observations and perform the chosen atomic transition. Mangle should derive whether remaining work is stopped, held, resumable or eligible, using owned state. The design should not add fuzzy stop interpretation or a second permission policy inside the stream helper.

### Preserve useful evidence without claiming completion

Keep successfully acquired observations attributable to their operation, even when final synthesis is canceled. Represent completed acquisition, canceled remaining synthesis and not-applied controls separately. Recovery may reuse evidence only under the existing freshness and authority rules; it must not rerun completed mutations merely because the final answer was not delivered.

## Primary foundations and bounded implications

The Go context API specifies that derived contexts inherit cancellation and that canceling a context affects its descendants. Creating a sibling from an earlier ancestor does not inherit values introduced only on another child. `CancelFunc` requests cancellation; it does not wait for work to stop. These semantics support the source trace and the need for a settlement acknowledgement, but they do not prescribe a codeNERD scheduler or prove that a dependency cooperates. [Go context documentation](https://pkg.go.dev/context)

The Go project's context article treats cancellation and request-scoped values as data to propagate across the goroutines performing one request. That is the relevant analogy here. An explicit application-level carrier remains appropriate when a transport command's lifetime is shorter than the logical operation. No measured performance improvement is imported from that article. [Go Concurrency Patterns Context](https://go.dev/blog/context)

The stronger project-specific requirement comes from codeNERD's own restriction and executive contracts: typed restrictions are observations about the request, never permission grants, and the kernel owns executable consequences. [Restriction type][restriction-type], [repository contract][guidance]

## Costs and alternatives

No extra model request is inherently necessary. Correct ownership and metadata transfer are deterministic. Costs include a bounded owner record per live operation, synchronization around handoff and settlement, retained terminal state until delivery, and more explicit control-disposition bookkeeping. Snapshotting immutable restrictions is small relative to model context; this should be measured rather than assumed free.

The likely benefit is avoided post-stop generation, consultation and synthesis cost, plus fewer corrective user turns. Those savings are not measured. Restriction continuity may also replace a seemingly successful but out-of-scope effect with an explicit denial or constrained answer; that is a correctness improvement even if it requires a later authorized request.

Compare the current feature to the two ownership shapes under the same providers, models, tools, world-model context and authority. Keep unrestricted and noncanceled positive controls. Do not make a treatment look safer by removing ordinary tools or making the baseline cheaper by skipping consultation and proof work.

Record the full cost vector:

- provider sends, retries and input/output/cache tokens by perception, articulation, consultation and synthesis;
- tokens produced and child executions admitted after stop acceptance;
- stop receipt to cancellation request, and cancellation request to actual settlement, separately;
- time to first useful evidence, final answer latency and queued-work delay;
- retained observations and terminal records, memory footprint and cleanup latency;
- denied effects, not-applied controls, wrong-owner updates and duplicate settlements;
- user recovery turns, correctly constrained task completion and evidence reuse without effect replay.

A single median latency or tool-call count is inadequate. Report slow-provider tails and failed/canceled cases separately. A responsive UI that incorrectly reports settlement is not a successful latency optimization.

## Evaluation program not executed

### Existing tests establish local properties

`journey_side_turn_test.go` simulates a working foreground by manually registering its cancel. Its Ctrl+X case proves that registered context is canceled; it never reaches the stream handoff that clears it. `stream_terminal_test.go` directly calls `articulateWithConversation` and verifies complete, incomplete and truncated terminal behavior. It does not drive the owning `processInput` command and keyboard handler. [Side-turn test fixture][stop-test], [stream tests][stream-tests]

The real-kernel process-control test is a useful composition fixture: it drives `processInput`, checks that permitted observations and a scoped note land before knowledge consultation, and checks that forbidden model-authored facts do not survive. Its task-executor stub receives `ctx` but its check callback does not inspect inherited restriction or cancellation after the stream message returns. Extend this fixture rather than replacing it with a helper-only proof. [Control roundtrip test][control-test]

### Deterministic normal-entry matrix

Use a barrier-controlled provider, recording task executor and actual production kernel. Drive the ordinary `processInput` to `streamStartMsg` to `Update` path. Use synchronization barriers rather than timing sleeps as the correctness oracle.

| Case | Required observation |
|---|---|
| Stream pending after command return, then Ctrl+X | Provider context receives cancellation; UI says stopping until the matching terminal settlement |
| Stop accepted before provider returns an otherwise-valid envelope | No new response-control effects or consultation dispatch; terminal disposition remains canceled |
| Provider returns success despite canceled context | Owned stop latch still fences later effects and child dispatch |
| Control committed before stop | Effect reported exactly as committed; no fictitious rollback and no replay |
| Stop while consultation is pending | Correct child's context canceled; useful completed sibling evidence retained with attribution |
| Stop between consultation completion and synthesis admission | Final synthesis is not started after accepted stop; retained evidence is available under its owner |
| Stop during synthesis | Synthesis context receives cancellation without stopping a newer operation |
| Recognized read-only or no-changes restriction, admitted consultation | Child receives identical or stricter restriction; prohibited registered handlers are not invoked |
| Recognized confirm-first restriction, consequential consultation action | Existing exact-request personal approval obligation remains intact |
| Child adds another restriction | Union preserved; no widening |
| Unrestricted consultation and ordinary authorized mutation | Existing legitimate capability remains; restriction repair does not deny everything |
| No consultation, ordinary successful stream | One answer, one terminal settlement, normal control outcomes |
| Complete, incomplete, truncated and failed stream terminals | Existing terminal admission guarantees remain unchanged |
| Stop at each phase-transfer boundary | No window in which accepted operation has neither a live cancel owner nor a latched stop |
| New foreground request after old stop | Old chunks, controls, synthesis and terminal messages cannot affect the new owner |
| Shutdown and provider forwarding blocked | Owned work terminates according to documented bounds; report any noncooperative dependency rather than claiming joined cleanup |

The descriptor test should verify the actual child context and turn-owned restriction facts, then observe catalog and exact admission behavior. A model that happens not to request a write is not an enforcement test. The cancellation test should observe the provider/child context, terminal state and effect counts; a hidden spinner alone is not a cancellation test.

### User journey evaluation

After deterministic contracts pass, evaluate a normal conversation that investigates a file, requests specialist evidence, and then decides whether to authorize work. Include read-only and confirm-first instructions, a stop while the answer is streaming, a stop during research, and a later explicitly authorized resumption. Keep normal CodeDOM and world-model context enabled. Judge source-grounded usefulness, preservation of the user's restrictions, and honest disposition of partial evidence, alongside full token and latency costs.

### Checkable exit

Every accepted foreground operation has one owner from admission through its last articulation or consultation phase. Stop reaches the actual outstanding work; a stopped operation cannot admit new effects or child work merely because a provider later reports success; the recognized parent restriction reaches every child catalog and final action boundary; and completed evidence remains attributable without reviving canceled work. Existing authorized, noncanceled journeys retain their capability.

First reconcile the CLI, articulation and session specs with that invariant. Then add the composed failing witnesses, repair lifetime and descriptor transfer, and verify the production journeys. Update existing ownership and restriction audits with this additional exit rather than marking their broader contracts complete from a local helper test.

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[historical-research]: https://github.com/theRebelliousNerd/codenerd/commit/c592ff223cd207fca4c29ae793d5eba57aa4781d
[current-research]: https://github.com/theRebelliousNerd/codenerd/commit/0c38d4fc987213a17f6570cc7d8dae34c87d8ace
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/agents.md
[articulation-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/articulation/agents.md
[main-stream]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L839-L859
[foreground-owner]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L70-L83
[stream-context]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L918-L937
[stream-return]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L1042-L1047
[stream-update]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L96-L133
[turn-state]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/side_turn.go#L36-L98
[stream-types]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_types.go#L738-L753
[ctrlx]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_key_handler.go#L20-L76
[post-articulation]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L984-L1039
[memory-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/control_outcome.go#L27-L53
[memory-effects]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/memory_ops.go#L59-L179
[continuation-policy]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/codedom_continuation.mg#L55-L79
[consultation]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L27-L137
[knowledge-update]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L882-L930
[synthesis]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L146-L193
[restriction-attach]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L213-L219
[restriction-type]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/restriction.go#L11-L193
[executor-restriction]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1169-L1247
[turn-restriction]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/turn_restriction.go#L21-L125
[spawn-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation.go#L64-L74
[task-request]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L352-L358
[preset]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L217-L250
[inline]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L353-L393
[terminal-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_articulation.go#L322-L380
[control-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L1051-L1227
[stop-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/journey_side_turn_test.go#L14-L138
[stream-tests]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/stream_terminal_test.go#L37-L95
[control-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_control_roundtrip_test.go#L71-L230
[side-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/side-turn-ordering-and-cancellation-ownership.md
[restriction-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/perception-restriction-continuity.md
[background-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/background-outcome-and-continuation-ownership.md
[stream-audit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/stream-completion-contract.md
[synthesis-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/terminal-knowledge-synthesis-grounding.md
[admission-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/articulation-admission-and-control-fidelity.md
