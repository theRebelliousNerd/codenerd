# Campaign requirement intake must not restore an obsolete pause state

## Status and scope

Source review at [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), the main head inspected on 2026-10-30. This is a proposed control-ownership repair, not an implementation or a reproduced incident. No application, tests, model inference, benchmark, or repository mutation was run.

**Finding:** requirement intake temporarily changes the same `isPaused` Boolean and `pauseCh` used by explicit operator controls, then restores a captured `wasPaused` value. An operator can resume an already-paused campaign while its requirement-planning call is pending. When intake subsequently returns—even with an ordinary provider error and no accepted plan—its defer restores `isPaused=true` after `Resume` has closed the channel and published `/active`. The scheduler then repeatedly enters a supposedly blocking pause branch whose receive is immediately ready. It does not proceed to new scheduling through that branch, despite the active status and successful resume acknowledgement. [Temporary pause][intake] [Controls][controls] [Main wait][main-wait] [Phase wait][phase-wait]

The narrow contract is: **an operation may release its own temporary hold, but must not reinstate an earlier operator decision or corrupt the wakeup state.** This is not a claim that campaign permissions, verification, cross-process ownership, or all pause mechanisms are absent.

## Why this is a distinct continuation

The existing campaign UI ownership and terminal-publication audits cover which run owns a message and how its terminal receipt reaches the UI. The generation-admission audit covers trusted durable campaign snapshots. Their protections remain valuable and do not distinguish an operator pause from an intake-owned hold inside one still-live orchestrator. [UI ownership][prior-ui] [Terminal publication][prior-terminal] [Durable generations][prior-generation]

Historical replanner QA already discusses stale plan snapshots, concurrent campaign mutation, and handling late task results during replanning. Those are important predecessors, but the witness here requires no successful plan mutation, stale task result, task deletion, or unsynchronized simultaneous access: the pause, resume, and deferred restoration can acquire the mutex in a fully ordered sequence. The same QA's completed-phase injection concern was screened and is **not** presented as a new finding here. [Snapshot QA][prior-qa] [In-flight mutation QA][prior-qa-inflight] [Completed-phase QA][prior-qa-phase]

Earlier execution QA also explicitly raised pause-channel state corruption and proposed a concurrent pause/cancel double-close check. Credit that general invariant and negative-test concern. The incremental contribution here is a concrete current production producer of the inconsistent state—requirement-intake restoration after an acknowledged resume—and its exact slash-command ordering, including a planner-error path. It is not a claim to have discovered channel lifecycle safety as a new topic. [Channel-state QA][prior-channel] [Proposed double-close check][prior-close]

The audit inventory and research branches through `research-main-20261030-1417` were screened. The latest branch adds scope deduplication, correction-answer ownership, manual alignment coverage, and NUL recall documents, not this temporary-hold lifecycle. The relevant `HandleNewRequirement`, `Pause`, and `Resume` function bodies were also compared against the five active non-research heads: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. Those bodies match main; other parts of the files differ. This is a targeted competing-fix check, not a claim that every branch was exhaustively audited. [Latest inventory](https://github.com/theRebelliousNerd/codenerd/tree/6d97defb8901c0320d7561d4f15d4c13ee799bc9/Docs/audits)

## Production route and exact state transition

### The controls are concurrently reachable

A started chat campaign clears the foreground loading flag and launches its orchestrator in the background. Ordinary slash submission routes to the command handler. `/campaign add` verifies that a matching live orchestrator exists, returns an asynchronous requirement command, and does not set the foreground loading flag. Its worker uses the session operation context, usage tracker, and campaign broker purpose. `/campaign resume` is a separate command that checks the live campaign's paused status before invoking `Resume` and confirming active status. [Background installation][install] [Slash dispatch][submit] [Add and control commands][commands] [Requirement command][command-worker]

Use ordinary ChatView with no foreground turn, modal approval, recurse run, or competing campaign. Assume an already admitted campaign with remaining work, healthy policy/kernel/storage, and a live execution context. These preconditions keep the witness on the actual local controls and avoid the separate cross-process `.pause` protocol.

### A legal schedule, without an accepted planner proposal

| Step | Action | `isPaused` | `pauseCh` | Canonical campaign status |
|---|---|---:|---|---|
| 1 | Operator pauses the running campaign normally | true | open channel P | `/paused` |
| 2 | `/campaign add <requirement>` begins; intake captures `wasPaused=true` | true | same open P | `/paused` |
| 3 | The planning call remains pending; operator submits `/campaign resume` | false | P is closed | `/active` |
| 4 | Planning returns an ordinary error while the session/run remain live | false before cleanup | closed P | `/active` |
| 5 | Intake's defer restores captured `wasPaused` | **true** | **closed P** | **`/active`** |

At step 1, `Pause` creates a new open channel only when the Boolean was false, sets it true, updates canonical status, and saves. At step 2, intake observes true and therefore does not replace P. At step 3, `Resume` closes P, clears the Boolean, and updates status. At step 5, the defer's channel-close branch is skipped because `wasPaused` is true, but its unconditional assignment reinstates that true value. It does not restore or otherwise update canonical status. [Controls][controls] [Temporary pause][intake]

The provider-error choice is deliberate. `ReplanForNewRequirement` returns a completion error before parsing, cloning, or publishing a revised plan. Thus the primary witness does not depend on a particular model response, accepted JSON, task retyping, plan-validation weakness, or a stale clone replacing live campaign state. The error is logged and returned to the TUI, which reports that the requirement was not added; that report does not repair the pause/channel combination. [Planner error boundary][planner-error] [Failure and cleanup][intake] [User-facing result][command-worker]

Keep the campaign from reaching a terminal state between steps 3 and 5—for example, an already-admitted legitimate task can remain outstanding while the provider error is delivered. This is a scheduling precondition, not a prediction about provider latency or model behavior. Some work can legitimately run after the user's resume and before intake cleanup; this audit does not claim that pause terminates already-owned workers.

### Actual consumer consequence

Both execution loops check cancellation, then read `isPaused` and `pauseCh`. If paused, they select on context cancellation or the channel and then `continue`. Once step 5 has occurred, receiving from P is immediately possible, so the loop does not block and does not reach the subsequent scheduling branch. In the phase loop this also precedes draining completed-task results. This is a source-derived nonblocking retry loop with no intended pause wait—not a measured CPU percentage, a guaranteed hung process, or proof that in-flight workers stop. [Main wait][main-wait] [Phase wait][phase-wait]

The Go specification independently establishes the two relevant language semantics: a closed-channel receive is ready, and closing an already-closed channel panics. These are language rules, not an assertion about a particular installed Go binary. [Receive operator](https://go.dev/ref/spec#Receive_operator) [Close](https://go.dev/ref/spec#Close)

**Conditional second-close consequence:** after step 5, an immediate TUI `/campaign resume` is rejected because canonical status is active. If the operator instead pauses and then resumes the same still-live run, `Pause` sees `isPaused=true`, leaves P closed, and publishes paused status. The subsequent allowed `Resume` reaches `close(P)` again. Source therefore predicts a channel-close panic for that additional schedule. No runtime crash, panic presentation, or process-wide recovery behavior was observed here. [Command status checks][commands] [Controls][controls]

## Safeguards and competing explanations

- **A normally ordered pause/resume pair works.** The close guard relies on the Boolean and channel remaining consistent. This finding is about the intake cleanup breaking that invariant, not an unconditional double-close in `Resume`.
- **No overlapping control, no witness.** A paused campaign whose intake returns before the operator resumes preserves its original open channel. An originally active campaign that receives no intervening controls also follows the intended temporary-pause restoration.
- **Canonical status matters.** Merely clearing `isPaused` after a later explicit pause does not prove unauthorized dispatch: `/paused` removes the stock `current_campaign` premise. The selected witness instead has canonical `/active` and a reinstated host pause, so it concerns lost progress/control consistency, not bypass of an operator stop. [Campaign status policy][status-policy]
- **The run must still be alive.** A settled run, context cancellation, or an independent failure transition changes the sequence. Main and phase loops retain cancellation checks. `Stop` invokes the run's cancel function; the durable cross-process pause watcher separately cancels the run. Do not weaken either route to repair local hold composition. [Stop][stop] [Durable pause][durable-pause] [Loop checks][phase-wait]
- **Fresh construction differs from object reuse.** A fresh orchestrator initializes a consistent resumed Boolean/channel pair, so admitted recovery through a fresh object need not inherit this in-memory corruption. Reusing the same corrupted object is not an equivalent repair: after its prior run ends, `Run`'s startup reset also closes any non-nil channel when `isPaused` remains true, so it can reach the same second-close condition. Actual recovery still requires the ordinary ownership and generation checks. [Construction][construction] [Run reset][run-reset] [Durable generations][prior-generation]
- **The planning error is visible.** Error logging, the failed requirement event, and the TUI failure message survive. The missing information is that intake cleanup has contradicted the acknowledged control state; this is not a wholly silent provider failure.
- **Existing safety is not evaluated away.** Risk admission, task eligibility, write leases, permissions, checkpoints, and acceptance remain independent. No new unauthorized effect or false verified task result is established by this trace.
- **Prior in-flight plan-mutation concerns remain separate.** A temporary scheduling hold is not a worker join. This proposal must not claim to solve concurrent plan replacement merely by fixing pause ownership. [Earlier QA][prior-qa]

## Proposed contract and implementation choices

### Separate intent from temporary holds

Represent the operator's requested pause separately from temporary operation-owned holds. An intake should acquire a token bound to the live run incarnation and release exactly that token on every outcome. A later operator resume clears the operator hold; it does not erase an intake hold that still protects a plan operation. Conversely, releasing an intake token cannot recreate a user hold that the operator already cleared.

Publish these as bounded host observations and let the kernel derive dispatch admission and the reason for a hold. Go should own synchronized lifecycle bookkeeping and wakeups, not create a second permission policy. Preserve the existing distinction between an operator control, a temporary planner operation, storage uncertainty, and cross-process cancellation. The repository's executive contract and campaign guidance are the design constraints, not permission to add a generic unconditional retry. [Repository contract][guidance] [Campaign guidance][campaign-guidance]

### Maintain a checkable wakeup invariant

Use a single transition helper or equivalent owner-controlled mechanism:

- effective transition from runnable to held installs a fresh open wait channel;
- held to runnable closes that channel exactly once;
- acquiring or releasing one of several holds does not overwrite another owner;
- an old operation cannot release a newer run's hold;
- cancellation remains observable while held;
- duplicate cleanup is harmless and does not change newer operator intent.

A captured Boolean is not an ownership token. A mutex makes individual writes exclusive but does not make restoring a historical value semantically current. A generation check can be a smaller repair, but only if it also specifies what happens when resume is requested during an active intake and how multiple intake operations compose. Simply suppressing a second `close`, recreating a channel in the defer, or adding a sleep treats symptoms while potentially losing the user's resume or releasing work too early.

### Make the displayed state truthful

Choose and document the user-visible contract. For example, resume can acknowledge that the operator pause is cleared while stating that requirement integration still temporarily holds dispatch. Alternatively, the command may explicitly defer the resume until the current owned operation settles. Do not say fully resumed while the host gate remains closed without exposing its reason.

Plan success and failure receipts should include the operation/run identity and hold disposition, not duplicate full plan or prompt content. A provider error must release only that operation's hold and preserve the latest operator request. Retain the existing provider-error message and terminal owner fence.

### Preserve recovery and safety boundaries

Temporary in-process holds need bounded cleanup and an explicit restart interpretation; do not persist anonymous holds that can survive without an owner. Preserve durable operator pause requests and campaign generation admission. If plan mutation additionally needs worker quiescence, implement and evaluate that distinct contract rather than pretending a Boolean pause joins active workers. Do not cancel accepted work, reopen completed phases, or widen task authority merely to make this fixture advance.

## Costs and fair comparison

The minimum state repair needs no additional model, embedding, search, or verification request. A run-scoped hold set adds identity/state storage proportional to simultaneous operations, synchronized transitions, and wakeup handling. A derived hold policy adds fact publication/retirement and fixpoint work; durable receipt choices add journal bytes and recovery complexity. A generation-only approach is smaller but has weaker composition unless every control path observes the generation contract.

Measure scheduler iterations and CPU, channel/lock contention, allocations, retained hold count, kernel mutation/evaluation work, heartbeat/log volume, pause-to-quiescence and resume-to-eligible-dispatch latency, and receipt persistence cost. Include normal no-overlap campaigns: preventing a pathological spin does not establish a general speedup. Count provider input/output/cache tokens and retries separately; a repair should not need an extra planning call merely to restore control state. Include canceled and failed intakes, abandoned operations, human recovery effort, and downstream wasted work rather than reporting only successful campaign throughput.

Compare the current code, a generation-fenced minimal correction, and explicit owner-scoped holds under identical command schedules, task state, provider outcomes, kernel, and persistence conditions. Evaluate legitimate parallelism and responsiveness as well as correctness. Serializing all UI input behind a slow planner may avoid this schedule but prevents timely controls and is not automatically an acceptable tradeoff.

## Proposed evaluation — not run

Use barriers around the production planning adapter and the existing TUI command path, not arbitrary sleeps or an assumed model response. Preserve the normal kernel and owner lifecycle. First establish the exact baseline state tuple and loop behavior; then credit a repair only against the same fixture.

| Control or witness | Required evidence |
|---|---|
| Paused → add pending → resume → ordinary provider error | Latest operator resume retained; intake hold retired; runnable state and wakeup channel coherent; no nonblocking pause loop |
| Same schedule with valid successful plan | Plan receipt and control receipt both correct; no operation-owned hold leaks |
| Provider parse/validation/publication error | Explicit failure preserved; only the failing operation's hold released |
| Intake completes before resume | Ordinary paused state remains until explicit resume; no regression of current behavior |
| Initially active intake, no external controls | Temporary hold ends exactly once and normal work continues |
| Explicit pause during intake | Pause remains effective after intake cleanup; do not infer dispatch solely from the host Boolean |
| Resume while intake remains live | Documented hold semantics; no premature release of the intake's protection |
| Pause then resume after the broken-state fixture | No second-close panic; acknowledgement corresponds to actual admission state |
| Two overlapping intakes, reversed completion order | Each releases only its own hold; latest operator intent and surviving hold remain |
| Stop, session quit, deadline, durable `.pause` request | Existing cancellation and worker-join behavior retained; no restart of canceled work |
| Delayed cleanup from old run after replacement | New owner unaffected; bounded stale receipt/release disposition |
| Kernel/storage failure and restart | Honest unresolved/held state; no invented successful durability or orphaned permanent hold |

The existing planning-tier fixture verifies real-kernel intake routing, persistence, and later trigger consumption, but it calls intake sequentially on an active campaign. The durable-pause test exercises the separate control-file cancellation path. Those are useful safeguards, not evidence for the same-process control interleaving above. Both were inspected, not executed. [Planning fixture][planning-test] [Durable pause fixture][durable-test]

## Evidence links

[intake]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_utils.go#L324-L372
[controls]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_control.go#L7-L45
[stop]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_control.go#L47-L84
[main-wait]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_execution.go#L124-L174
[phase-wait]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L83-L156
[install]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L325-L353
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L109-L132
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L726-L819
[command-worker]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L48-L77
[planner-error]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L264-L324
[status-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_core.mg#L1-L16
[durable-pause]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/pause_durable.go#L12-L67
[planning-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/planning_tier_test.go#L440-L505
[durable-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/pause_durable_test.go#L15-L79
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/agents.md
[prior-ui]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-ui-run-ownership.md
[prior-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-terminal-publication.md
[prior-generation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-generation-admission-and-recovery.md
[prior-qa]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/campaign/2026-04-05_04-30-EST_replanner_boundary_analysis.md#L121-L132
[prior-qa-inflight]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/campaign/2026-04-05_04-30-EST_replanner_boundary_analysis.md#L377-L381
[prior-qa-phase]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/campaign/2026-04-05_04-30-EST_replanner_boundary_analysis.md#L212-L216
[prior-channel]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/orchestrator_execution_qa_2026-06-26_04-27-EST.md#L30-L42
[prior-close]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/orchestrator_execution_qa_2026-06-26_04-27-EST.md#L191-L194
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_init.go#L39-L65
[run-reset]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_execution.go#L72-L96
