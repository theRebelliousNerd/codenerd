---
doc-class: deep-dive
subsystem: campaign-chat-integration
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection
---

# Campaign context seeding and scheduler ownership

**Status:** Source-grounded audit and proposed contract. No implementation or runtime validation.

**Reviewed:** 29 October 2026. Primary source: CodeNERD commit [`9dcb01b9736bf4763aa3ac391d29f133d73c6611`][baseline]; main remained at this commit when rechecked at 18:48 UTC.

**Decision:** Retire the TUI's independent production of scheduler conclusions. Preserve campaign-aware prompting through an owner-provided, read-only context view, and make phase completion idempotent.

## Finding

The TUI's `seedCampaignFacts` writes `current_phase(P)` into the scheduler's shared kernel as an extensional database fact, or EDB fact: stored input rather than a conclusion whose validity disappears with its premises. The function exists to improve activation and prompt selection, but it asserts names that the campaign schema describes as derived execution state. It provides no corresponding retirement operation. [Seeder][seed] [Derived declarations][schema-derived]

Consequently, a context seed accepted while phase P is unfinished can remain true after the authoritative `campaign_phase` row says P is completed. The scheduler selects the matching campaign phase without checking its status. For a one-phase campaign, the stale row can therefore select the completed phase again, bypassing the *branch that performs terminal settlement*, not bypassing acceptance checks to report success. If its checkpoint passes again, phase completion accounting can increment again. [Phase selection][select] [Execution loop][execution] [Phase completion][complete]

This is a **conditional source-derived liveness and accounting defect**, not an observed production incident. It does not establish duplicate task execution, unauthorized work, false acceptance, data loss, pause bypass, or an unavoidable infinite loop. The useful boundary is between presentation context and authoritative scheduling evidence.

## How presentation state reaches the scheduler

The TUI installs the active campaign, clears its loading state, and starts orchestration in the background. The orchestrator receives `m.kernel`, the same kernel used by chat processing. Campaign admission already checks and loads the canonical `Campaign.ToFacts()` representation through `SetCampaign`; a second UI producer is not required to establish execution state. [TUI start][tui-start] [Orchestrator wiring][wiring] [Campaign admission][admission]

A normal chat turn can reach the seeder after classification. This is not every input: classification-stop, hold, approval, and perception-answer paths return before that call. The witness requires a valid classified, non-fast-path turn while a campaign remains active. It does not rely on `/campaign status`, nor assert that a particular greeting or natural-language prompt guarantees the necessary classification. [Chat entry point][entry]

The seeder checks only that the kernel and active campaign are non-nil. It selects the first phase whose status is not completed, then asserts campaign, phase, next-task, objective, and artifact context. It neither scopes the asserted conclusions to a chat turn nor replaces an earlier snapshot, and it ignores the `LoadFacts` return value. “First not completed” is also not the policy's complete eligibility calculation. [Seeder][seed]

This is not an isolated prompt-only kernel. Cortex routes predicates to their owning and consuming shards; campaign registration explicitly owns `current_phase`. `RealKernel.LoadFacts` adds accepted facts to its EDB and normally defers reevaluation until a read. Stored facts survive independently of whether the policy could still derive them. Deduplication of an identical assertion does not supply expiration. [Shard ownership][ownership] [Cortex routing][routing] [EDB loading][edb-load] [EDB storage][edb-store]

The policy normally derives the current phase from an in-progress phase or an eligible phase. Updating the canonical phase to completed removes those derivational reasons, but not the independent EDB assertion. `completePhase` updates the canonical phase row without retracting `current_phase`. Rolling-wave refresh retracts phase and task rows, not that seeded conclusion; context compression retires phase-context atoms, not scheduler state. [Phase policy][policy] [Phase completion][complete] [Rolling-wave refresh][rolling] [Compression][compression]

## Minimal one phase witness

Use one admitted campaign C with one phase P and at least one actual task. This avoids relying on unordered selection among multiple current-phase rows.

1. Boot a healthy production kernel, pass the applicable planning and risk gates, and admit C through the normal owner path. Hold an actual task or otherwise keep P non-completed long enough for a chat turn.
2. A valid non-fast-path TUI turn reaches the seeder. Its `current_phase(P)` assertion is accepted into the shared EDB. No malformed fact, fabricated campaign, or direct test-only injection is necessary.
3. Let all of P's tasks finish and its checkpoint pass. The normal path compresses context, marks P completed, and increments `CompletedPhases`. The canonical completed phase row is successfully installed. [Phase finalization][finalize] [Phase completion][complete]
4. Assume no reset, explicit retirement, or intervening replan removes the seeded row or P itself. Keep the context alive and the campaign neither paused nor failed. On the next outer iteration, `getCurrentPhase` finds P by ID and returns it without a status filter. With only P present, no result-order assumption is needed. [Phase selection][select]
5. Because that pointer is non-nil, execution does not enter the branch that checks whole-campaign completion and settles acceptance. It invokes the phase path again. The task loop rebinds to the live phase, so this witness does not depend on an orphaned pointer. [Execution loop][execution] [Live phase binding][live]
6. With no active workers, `phaseTasksDone` asks policy using actual phase and task state. The rule requires a phase row and no incomplete tasks, not an unfinished phase status. It can therefore be true for completed P. The checkpoint and compression path runs again; another successful checkpoint reaches the unguarded completion increment again. [Task completion query][tasks-done] [Completion rule][done-rule] [Phase finalization][finalize]

The repeated step remains conditional on retained facts and successful subsequent checks. A later failed checkpoint, cancellation, pause, timeout, failure transition, or state replacement can change the outcome. The claim is repeated completed-phase processing and obstruction of ordinary terminal settlement under the stated conditions, not proof of an unbounded execution.

## Impact and existing protections

**Task dispatch remains guarded.** The scheduler skips a task unless it is pending, and the loop rebinds the live phase. This trace does not redispatch completed tasks merely because P was selected again. Cancellation and pause checks remain in both relevant loops; configured campaign timeout and Stop still apply. Failed checkpoints retain their bounded policy-controlled handling. [Task dispatch][dispatch] [Live phase binding][live] [Controls][controls] [Timeout][timeout] [Phase finalization][finalize]

**Successful finalization can still be duplicated.** The checkpoint routine has no already-passed completion receipt check at this boundary: it runs the configured objective verifications and appends checkpoint records. Objectives with `VerifyNone` skip verification. Compression builds accomplishments from completed tasks and can request another summary when clients and prompts are available; otherwise it can fall back locally. Removing old context atoms is not a summary-call cache. Thus repeated paid model calls are possible, not guaranteed. [Checkpoint routine][checkpoint] [Compression][compression]

Likely costs include repeated verification work, summary tokens, scope-refresh I/O, events, persistence work, duplicate completion accounting, and delayed completion. The initial seed also scans phases, objectives, and artifacts and may dirty the EDB for reevaluation. None of these costs was measured here; there is no latency, token, dollar, or incident-frequency estimate. The source establishes another `CompletedPhases++` transition, not a particular displayed count or permanently inflated value.

A smaller duplication defect supports retiring the producer: it emits `phase_objective/3` and `task_artifact/3`, whereas declarations and canonical `ToFacts` use four arguments. Undeclared arities are stored with warnings but are not queryable through the declared program. That does **not** atomically reject the entire load or neutralize the valid `current_phase/1` assertion. Nor does it mean all campaign goals disappear: canonical facts already provide the correct shapes. [Seeder][seed] [Objective schema][schema-objective] [Artifact schema][schema-artifact] [Canonical objectives][canonical-objective] [Canonical artifacts][canonical-artifact] [Undeclared facts][undeclared]

## Proposed ownership contract

Specify the contract before changing code:

- **One authoritative producer.** The campaign owner publishes admitted source state. The UI obtains a typed snapshot or read-only derived view for prompting; it never asserts `current_phase`, `current_campaign`, or `next_campaign_task` as execution evidence merely to make a prompt campaign-aware. Preserve canonical `ToFacts` and useful contextual content.
- **Separate presentation focus when necessary.** If activation requires facts, use distinctly named presentation predicates keyed by session, campaign, revision, and owner epoch. Replace each owned snapshot atomically, surface publication failures, and define retirement on completion, replan, reset, and pause as appropriate to the display contract. A presentation fact must not become an authorization premise for execution.
- **Respect ownership during migration.** Remove only legacy seeded EDB facts whose provenance is known. Do not globally retract every current-phase fact or suppress the predicate. Where provenance cannot distinguish producers, rebuild the campaign projection from admitted state at an owner-controlled barrier rather than guessing which rows to erase.
- **Make contradictions explicit.** A completed canonical phase plus a purported current-phase input should produce a bounded repair or hold through Mangle policy. Go should not silently choose a different next phase and become a competing scheduler. A local status filter is useful defense in depth, not a complete producer fix for other consumers.
- **Make completion effects idempotent.** Identify successful completion by campaign, phase, and admitted generation. Duplicate delivery must not increment counters or replay successful completion side effects. Preserve policy-authorized failed-checkpoint retries and distinguish genuine reopening or a new revision from repetition of an old completion. Return explicit stale, duplicate, and error outcomes.

This requires a coherent boundary across UI publication and scheduler consumption, not just a mutex around one slice. Carry expected generations and receipts through state replacement. Adding the missing fourth arguments would leave the main defect intact; clearing facts after each chat risks races; filtering only `getCurrentPhase` leaves polluted evidence available elsewhere.

Compare three designs fairly: removing the ad hoc seeder while reading existing context, a typed owner-provided read-only projection, and a defensive consumer guard. Judge prompt usefulness and correctness at matched context quality, not merely the lower cost of dropping campaign awareness. Prefer the smallest design that enforces ownership end to end.

The implementation cost is not zero: an owner snapshot needs a stable schema and error contract; an epoch-scoped projection needs lifecycle hooks and storage; legacy cleanup needs provenance or a coordinated rebuild; and completion receipts need ordering, retention, and genuine-reopen semantics. A consumer-only guard is smaller but leaves the producer and other readers inconsistent. None of these mechanisms requires a new model, embedding, or judge request. Measure kernel mutation and evaluation CPU, snapshot and fact bytes, retained receipt count, lock/wait time, host verification runs, summary requests and tokens, journal writes, and end-to-end settlement latency separately. Include failed and canceled runs and ordinary small campaigns, not just a favorable repeated-completion example.

## Evaluation plan and evidence limits

No tests, application execution, live model calls, or benchmarks were run for this audit. The proposed regression must exercise the production TUI path, real Cortex routing, and normal world and CodeDOM wiring with deterministic fake providers and explicit barriers, not only inject a stale row into a mock kernel.

Hold a one-phase task, deliver the qualifying chat turn, record the actual seed's EDB origin and derived rows, then release the task. Capture task dispatches, checkpoints, compression attempts, completion counters, and whether required final acceptance executes. Demonstrate the baseline failure before crediting a repair. Avoid sleeps and unspecified model classifications as the synchronization mechanism.

Controls should include no chat; a fast-answer chat that never seeds; CLI execution without the TUI; multiple phases; skipped or blocked work; stale planner state; passing and failing checkpoints; pause/resume, cancellation, and deadlines; genuine reopen in a changed revision; duplicate or delayed UI snapshots; projection failure; concurrent campaigns; and custom providers. Require exactly one successful completion accounting event per admitted generation, unchanged legitimate dispatch, required acceptance still checked, bounded read-only presentation focus, and zero additional model calls from the repair mechanism itself.

One inspected fixture, `ActiveCampaign_ContextDoesNotHijackChat`, checks greeting routing with `activeCampaign` left nil and a manually inserted two-argument campaign fact. It does not exercise a running campaign, this seeder, and subsequent phase completion. That limits the evidence supplied by this fixture; it is not a claim about all repository test coverage. [Inspected routing test][test]

The review compared the nine-file seed/scheduler slice against four active code and QA snapshots: `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `51cfd42649552866d4aa54c53b68acf5740eb09b`. That slice was identical. The research-branch screen at [`d08d4645f6435d6031db56e95c70aedfe9334ceb`][research] examined 112 top-level audits for the seeder and relevant phase terms without finding an exact predecessor. Existing [campaign generation admission][prior-admission] and [campaign resume return evidence][prior-return] audits concern different durable recovery or return boundaries. [Earlier QA][prior-qa] discussed hypothetical multiple current phases; this finding identifies a concrete live producer and a one-phase witness. These bounded checks support distinctness, not a claim that nobody previously recognized stale phases as undesirable.

## Pinned sources

[baseline]: https://github.com/theRebelliousNerd/codenerd/tree/9dcb01b9736bf4763aa3ac391d29f133d73c6611
[research]: https://github.com/theRebelliousNerd/codenerd/tree/d08d4645f6435d6031db56e95c70aedfe9334ceb
[prior-admission]: https://github.com/theRebelliousNerd/codenerd/blob/d08d4645f6435d6031db56e95c70aedfe9334ceb/Docs/audits/campaign-generation-admission-and-recovery.md
[prior-return]: https://github.com/theRebelliousNerd/codenerd/blob/d08d4645f6435d6031db56e95c70aedfe9334ceb/Docs/audits/campaign-resume-return-evidence.md
[prior-qa]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/.quality_assurance/qa_journal_2026-08-01.txt#L100-L103
[seed]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_seed.go#L112-L173
[schema-derived]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/schemas_campaign.mg#L310-L318
[select]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_phases.go#L13-L44
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_execution.go#L130-L294
[complete]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_phases.go#L514-L583
[tui-start]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L312-L326
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_planning.go#L84-L135
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_lifecycle.go#L20-L58
[entry]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L331-L364
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/registration.go#L325-L365
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/cortex_kernel.go#L211-L278
[edb-load]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L64-L137
[edb-store]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L446-L491
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/campaign_phases.mg#L47-L55
[rolling]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_tasks.go#L342-L399
[compression]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/context_pager.go#L218-L318
[finalize]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_tasks.go#L152-L238
[live]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_tasks.go#L99-L138
[tasks-done]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_phases.go#L322-L338
[done-rule]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/campaign_core.mg#L97-L105
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_tasks.go#L242-L273
[controls]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_control.go#L7-L71
[timeout]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_execution.go#L103-L108
[checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_utils.go#L49-L117
[schema-objective]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/schemas_campaign.mg#L46-L49
[schema-artifact]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/schemas_campaign.mg#L114-L116
[canonical-objective]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/types.go#L637-L642
[canonical-artifact]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/types.go#L751-L758
[undeclared]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_undeclared.go#L14-L90
[test]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/task_routing_arbitration_e2e_test.go#L599-L630
