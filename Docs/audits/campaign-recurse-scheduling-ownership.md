# Managed campaign and recurse scheduling ownership

## Decision and bounded finding

**The TUI's single-managed-campaign safeguard does not cover a recurse attempt, although both execute against the same campaign kernel.** Once both campaigns are admitted, several campaign readers select the first global answer rather than the answer belonging to their orchestrator. Shipped policy also contains process-wide existence helpers, and rolling-wave refresh removes whole predicate families before reloading only its own campaign.

This is an integration continuation of the campaign UI ownership repair, not a claim that the repair is absent. The current guard correctly refuses a second ordinary managed campaign and rejects a competing started message. Recurse has a separate slot and a different message family. The narrow proposal is to establish one scheduling-ownership contract across those doors: either reserve a common campaign execution slot before either door publishes scheduler state, or qualify concurrent execution end to end.

The supported consequence is **conditional interference between two legitimately admitted owners**: an orchestrator can ignore its own current phase because a peer's row is first, read a peer's blocking reason, or temporarily lose its canonical phase/task rows during the peer's refresh. This review does not establish inevitable starvation, execution of a foreign task, an authorization bypass, false successful completion, or durable journal corruption.

Source pin: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), reconfirmed as main on 2026-10-30. This is static source analysis and proposed evaluation only. No application, test, host command, model inference, or training was run.

## Existing ownership repairs deserve credit

The prior [campaign UI run-ownership audit][prior-ui] identified cross-run progress and terminal messages overwriting a singleton UI slot. Current `Update` compares the message's orchestrator with the installed owner, and refuses to install a second distinct live orchestrator. The command handler also refuses `/campaign start` while `m.campaignOrch` is non-nil. The corresponding test explicitly covers both the started-message race and the ordinary start command. These are real safeguards, not merely proposed behavior. [Current command guard][start-command], [current installation and event guards][update], [ownership tests][ui-tests].

That prior audit explicitly separated UI registration from qualification of shared-kernel and workspace scheduling. This document supplies a concrete remaining entry combination and traces its scheduler consumers. It does not reuse the old ordinary-campaign A/B message race.

Likewise, [campaign generation admission and recovery][prior-admission] owns durable generation and single-publisher admission for an individual campaign. Current owner locks are keyed by campaign ID; two distinct IDs do not contend for the same lock. They protect each campaign's journal but do not serialize all campaigns sharing a kernel. [Per-ID owner lock][owner-lock].

The [context-seed scheduler audit][prior-seed] concerned stored conclusions supplied by an independent context producer. This witness needs no stale UI seed or fabricated conclusion: normal canonical active campaign rows are sufficient. Earlier QA also noticed first-row phase selection in hypothetical multi-row cases; the incremental contribution here is the concrete managed/recurse admission route and the coupled reader, rule, and refresh contract, not discovery that unordered first-row selection can be dangerous. [Earlier phase QA][prior-qa], [journey's phase-selection inventory][prior-journey].

## The actual two-door route

Use a healthy TUI in ordinary chat mode, with a usable kernel, client, task executor, shard manager, and workspace. Begin recurse in a Git workspace accepted as clean by its tracked-change check, with no unresolved recovery state. There must be actual admitted work: recurse must reach an attempt rather than exit because there is no eligible finding or its gate/mission policy refuses it. Each campaign must pass its own plan, ownership, risk, and cancellation checks. Use distinct campaign, phase, and task IDs and nonconflicting write scopes; file-lock contention is not needed.

1. Start recurse first. Its startup checks required services and whether `m.recurse` already exists. The loop takes the recurse-specific workspace lock, settles prior in-flight work, and checks Git preparation before attempting new work. A dirty tracked checkout can prevent this entry; beginning recurse before the other campaign's writes avoids assuming that guard is absent. [Recurse admission][recurse-start], [loop admission][loop-admission], [Git preparation][git-prepare].
2. Recurse's started message installs only `m.recurse`, clears foreground loading, and schedules its own loop. Each attempt constructs a fresh campaign and orchestrator, calls `SetCampaign`, and runs it directly. It does not install `m.campaignOrch` or pass through `campaignStartedMsg`. [Recurse lifecycle][recurse-loop], [recurse message handling][recurse-messages].
3. While an attempt A remains live, submit `/campaign start <goal>` through the ordinary idle slash-command route. The managed-start check sees no `m.campaignOrch`; it does not check `m.recurse`. A normally admitted campaign B is installed and launched in the background. This is not a queued-command reclassification witness. [Slash dispatch][submit], [managed command guard][start-command], [start and run][managed-start], [installation][update].
4. Both doors use `newCampaignOrchestrator`, whose configuration passes exactly `m.kernel`. Private channels and a fresh orchestrator do not imply a private scheduler fact store. `SetCampaign` loads canonical campaign facts into that supplied kernel. [Shared construction][builder], [canonical publication][set-campaign].

The workspace hold is meaningful counterevidence, but its actual dispatch scope matters. It rejects the listed `/mkdir`, `/write`, `/patch`, `/edit`, and `/append` commands; the separate `/campaign` branch dispatches directly to its handler. Ordinary mutation turns also consult the hold, but that does not intercept this slash path. The recurse lock excludes another recurse loop; it is not the per-campaign owner lock and does not reserve the managed campaign slot. [Write-command hold][hold-dispatch], [campaign dispatch][campaign-dispatch], [ordinary mutation hold][mutation-hold], [hold implementation][workspace-hold], [recurse lock][recurse-lock].

The reverse order is also not rejected by the local slot checks: `/campaign recurse` does not use the managed-start guard, and `startRecurseCampaign` checks only the recurse slot. However, a managed campaign that has already dirtied tracked files can make recurse preparation fail. The recurse-first sequence is the cleaner witness. Neither ordering promises an attempt lasts long enough for the second request, or that a second plan necessarily passes admission. [Recurse command branch][recurse-command].

The deterministic attempt builder gives recurse one phase and one task, an active campaign status, and distinct ID-derived phase/task names. A fix attempt may carry an acceptance witness; an improvement may not. Neither representation makes it a separate kernel domain. [Attempt construction][attempt].

## Stock facts meet unscoped consumers

### Current phase: a local ownership check happens too late

The stock rule derives `current_campaign(C)` for every canonical active campaign. Phase eligibility is already available as `phase_eligible_in_campaign(P,C)`, but the public `phase_eligible(P)` projection discards C. `current_phase(P)` also contains no campaign argument. With two active campaigns, their canonical rows can therefore contribute valid answers to the same queried relation. [Active campaign rule][rules-core], [phase rules][rules-phases].

`getCurrentPhase` reads the complete `current_phase` answer but examines only `facts[0]`. It then searches that one ID in its own campaign. If the first row belongs to B, A returns nil even when a later row identifies A's own current phase. It does not scan for an owned candidate. [Current-phase consumer][phase-reader].

A minimal conditional relation witness is:

- A and B have distinct, valid, active canonical campaign rows and owned phases PA and PB.
- The shipped rules derive current-phase answers for both, for example when both phases are in progress.
- A's query returns PB first and PA later.
- A's in-memory plan still owns PA, and no cancellation or unrelated terminal failure has ended A.

This ordering is a precondition, not a claimed stable property of the engine. No malformed row, custom rule, model misclassification, or alternate-arity injection is needed. If PA is first, this particular lookup succeeds. If A is already inside its phase loop, that loop can continue without immediately consulting `getCurrentPhase`; the defect is at the selection boundary, not an assertion that every running task instantly stops.

When A sees nil, its main loop checks its own completion, then blocking, then tries to start an eligible phase. `startNextPhase` again selects only the first global `phase_eligible` row. A foreign first row is not executed: the subsequent lookup cannot find it in A and returns an error. The main loop records that error and continues. Thus the established immediate consequence is a lost owned selection or failed transition; repeated foreign-first answers can delay progress, but persistent starvation and CPU impact were not measured. [Main-loop consumer][run-selection], [next-phase consumer and refusal][next-phase].

### Blocking: the available campaign key is discarded

`campaign_blocked` is a keyed relation. Nevertheless, `getCampaignBlockReason` reads the first returned row's reason and never compares its first argument with `o.campaign.ID`. The main loop uses a nonempty result to fail its own campaign, and the phase loop also consumes this helper. [Blocking consumer][block-reader], [main-loop failure][run-selection], [phase-loop failure][phase-block].

This makes a **foreign-block propagation conditional**: if A reaches one of those consumers while the query contains a blocking row for B first, and no applicable A-owned block, the helper still supplies B's reason to A. It is not necessary for PA to be selected as a foreign task. The existence and timing of such a B row must be established in any reproduction; this audit does not claim every concurrently running pair derives one. For example, the stock acceptance-exhaustion rule is keyed by its campaign, but the owning run can soon transition to failed and retire its active premise. An evaluation must hold the actual producer/consumer boundary, not inject a permanent impossible B state. [Keyed acceptance-block rule][rules-core].

A stronger finite interleaving avoids relying on the order of multiple blocking answers. Let recurse attempt A have a declared acceptance command, no passing acceptance round, and a newly failed round at the configured exhaustion limit. Its real acceptance runner successfully records the output and publishes the result while A is still active. Let managed campaign B already be in its phase loop with no active worker and only legitimately backed-off pending work, with its own canonical rows present and no B-owned block. Backoff is represented by a real future retry observation and excludes the task from eligibility; the stock backoff helpers classify that wait rather than an all-tasks-blocked condition. [Acceptance result producer][acceptance-producer], [retry producer][retry-producer], [backoff and eligibility rules][backoff-rules].

If B reaches its empty-runnable blocking check after A's failing result is published but before A changes out of active status, the sole blocking answer can be `campaign_blocked(A, /acceptance_failed)`. B's helper takes that reason and B's phase loop calls `failCampaign` for B. This is a source-supported **conditional foreign failure**, not merely a presentation mismatch. It requires that scheduling interleaving, successful result publication, and the stated absence of another B blocker; it is not an observed incident, and no claim is made that every acceptance failure or retry exhibits it. If A retires first, the relevant active premise disappears. If B never reaches that check during the interval, this particular consequence does not follow. [Block consumer][phase-block], [acceptance settlement and keyed status transition][run-selection].

Filtering only these Go readers is insufficient to fully qualify concurrency. `current_phase` for a pending phase uses the global `has_in_progress_phase()`; no-eligible and unverified blocking rules use global existence helpers; `has_next_campaign_task()` is also process-wide. Another campaign's progress or runnable work can therefore influence those derivations before Go filters the answer. The intended semantics—global serialization or independent per-campaign scheduling—need to be declared rather than guessed. [Global phase premise][rules-phases], [global blocking premises][rules-core], [global task helper][rules-tasks].

### Refresh: one owner removes peer source rows

After a phase has passed its applicable checkpoint and been completed, `runPhaseTasks` calls `triggerRollingWave`. The caller invokes the replanner and, on a nil return, retracts **all** `campaign_phase` and `campaign_task` rows before loading only its own campaign. A final phase with no successor is enough to reach this: a healthy replanner returns nil at “no further phases” before making a refinement request, but the caller still performs the broad reload. [Completion-to-refresh][refresh-call], [broad reload][refresh], [no-successor branch][no-next].

This is not a campaign-scoped interpretation of `Retract`. `RealKernel.Retract` removes every row of the named predicate; `CortexKernel.Retract` routes that predicate-wide operation to its owning shard or all shared replicas. Neither takes a campaign ID. [RealKernel semantics][retract-real], [Cortex semantics][retract-cortex].

There is an important repair path: `askWithCampaignState` detects missing owned phase/task IDs and reloads the caller's campaign before completion decisions; `getEligibleTasks` can reload when none of its pending tasks are held. These checks prevent treating missing rows as permission or immediate successful completion in the straightforward case. They can restore B's source state after A's refresh. They do not make A's retraction owner-scoped or fix foreign-first selection when both owners' rows are present. [Completion reload][reload-completion], [eligibility reload][reload-eligibility].

Successful refinement already uses `syncCampaignFacts`, which validates the candidate snapshot and retracts the previous campaign's IDs through a transaction where available. The outer predicate-wide reload is therefore not the only available publication mechanism. Preserve that existing scoped facility rather than replacing it with another global reset. [Scoped sync][scoped-sync], [replanner publication][replan-sync].

## Safety boundaries and nonclaims

- The ordinary second-managed-campaign command and installation guards work. This case depends on the separate recurse door, or another separately proven same-kernel owner—not two ordinary start commands bypassing the current guard.
- Missing services, invalid arguments, no eligible recurse work, risk refusal, plan inclusion failure, owned-campaign lock refusal, cancellation, or an earlier terminal outcome can prevent overlap. A mere visible “Recurse started” line is not proof an attempt reached `SetCampaign`.
- Distinct campaigns have distinct durable owner locks. This review does not show those locks or journal checks failing.
- Phase and task lookup still checks the orchestrator's own in-memory structure. A foreign phase selected first is not simply executed by A.
- Missing-row reload, task eligibility, dependencies, backoff, write-set coordination, verification, and acceptance remain relevant controls. Nothing here authorizes bypassing them.
- A peer's completion, cancellation, cleanup, or reload can end or change interference. Query ordering, kernel publication failures, and interleaving affect the outcome. No hang, false success, or lost file was reproduced.
- Predicate-wide retraction errors are not propagated by this caller, and reload errors are logged. This warrants explicit error handling, but the review does not assume a particular inclusion/rebuild failure or its downstream behavior.
- Separate processes or deliberately separate kernels do not establish this same-fact-store witness. They require their own workspace and resource coordination analysis.
- Existing `derivedFor` and `holdsFor` helpers already show keyed decision consumption. The recommendation is consistency at the remaining boundaries, not abandoning kernel authority for a Go fallback. [Keyed consumers][keyed].

## Proposed contract and implementation choices

Update the campaign capability and lifecycle specification before implementation, as repository guidance requires. Declare whether the execution domain permits one active campaign or multiple independently scheduled owners; do not describe the present UI singleton as a complete workspace or kernel exclusion guarantee.

### Prefer explicit admission until concurrency is qualified

For the current single-managed-campaign product contract, the smallest defensible containment is a **shared scheduling lease** covering managed campaigns and recurse attempts in the same session/kernel:

1. Reserve the execution domain before a door publishes canonical scheduler facts, not merely before the UI installs an owner. Planning or saving may be permitted separately only if it does not inject active execution state into the live scheduler.
2. Identify the lease by execution domain and owner epoch, with campaign/attempt identity as payload. Use the same authority for start, assault, resume, recurse, cancellation, and terminal settlement.
3. Refuse or explicitly queue the second request with its original arguments and status. Do not silently drop it, launch it and hide its pane, or preempt an active owner.
4. Release only after the owned run has settled and its relevant projection has been retired. A recurse loop between attempts needs a stated lease policy: hold across the loop for simple serialization, or yield through an explicit scheduler boundary—not an accidental nil pointer.
5. Preserve stop/status controls for the current owner and document the reduced concurrency. Do not use a naked `Stop` call as terminal cleanup or change a completed campaign back to paused.

This needs no model or embedding call. A frontend-only flag is useful containment but insufficient as a general kernel contract if another production owner can publish to the same scheduler. Admission should be enforced at the common execution-domain boundary as well as reflected in the UI.

### If concurrent campaigns are intended

Qualify the full state machine rather than patching one `facts[0]`:

- Retain campaign identity in selection relations, or join owned phase IDs before choosing. Preserve ordering and dependency policy within that campaign and report ambiguous owned results explicitly.
- Read blocking reasons only for the current campaign; aggregate multiple owned reasons deterministically without borrowing a peer's outcome.
- Scope existence helpers and completion/blocking premises to the intended owner. Explicitly retain only genuinely global constraints, such as a shared resource capacity.
- Replace predicate-wide refresh with owner-scoped, revision-aware publication. Skip no-op refinement publication; propagate publication errors and retain the prior usable snapshot where the kernel contract supports that.
- Reuse and strengthen scoped snapshot transactions, while testing actual Cortex routing and its failure semantics rather than assuming cross-shard atomicity from the word “transaction.”
- Specify ID uniqueness, cleanup, and epoch retirement for attempts. Do not delete another owner's facts or rewrite durable journals simply to make the shared scheduler look singular.
- Keep file-effect leases, verification snapshots, and resource budgets separate from scheduler ownership. A campaign-scoped reader does not itself make concurrent workspace writes safe.

## Costs, failure modes, and fair evaluation

A shared lease is the smaller implementation: common admission state, lifecycle hooks, UI feedback, and queue/refusal tests. It trades parallelism for simpler semantics and can increase waiting time, especially for an unbounded recurse loop. A crash or missed terminal release must not leave a permanent busy slot; stale ownership needs a bounded, evidence-based recovery path. A queue adds argument retention, cancellation, ordering, and memory bounds.

Full concurrent qualification is larger: relation/schema and consumer changes, scoped helper rules, publication semantics, compatibility for callers/tests, owner-aware diagnostics, and resource arbitration. More keys and joins can increase fact volume and kernel evaluation cost; private scheduler kernels increase memory and world-context synchronization cost instead. Neither solution inherently needs new LLM, judge, embedding, or training requests. Extra model spend may arise only indirectly from changed task schedules, retries, or delayed work and must be measured separately.

Measure total completion time and throughput as well as correctness: kernel queries and rebuilds, fact counts, reload count/bytes, mutex and lease wait, poll/error-loop activity, memory, host checks, model requests/tokens, canceled or duplicated work, and time until a stopped owner releases its domain. Include startup, rejected admission, blocked runs, empty recurse cycles, and failure recovery rather than reporting only a favorable concurrent workload.

The following evaluation is proposed, not executed:

| Control or witness | Required observation |
|---|---|
| A managed run followed by recurse, and the reverse order | Under a single-owner design, second work is explicitly refused/queued before scheduler publication. Under a concurrent design, both owners retain independent valid state. |
| Two ordinary managed starts and delayed old UI messages | Preserve the current ownership refusal and stale-message filtering; do not regress the existing tests. |
| Recurse exits before any attempt, fails admission, or is canceled during preparation | No false overlap assertion, leaked lease, or fabricated active campaign. |
| Two valid canonical active plans, with current-phase and eligible-phase answer orders reversed | Each owner selects its own phase or reports its own legitimate wait. A peer-first row does not cause a foreign lookup failure. |
| Both pending, one in progress, both in progress, one backing off, one storage-held | Policy derives the intended per-owner decisions; foreign existence helpers neither suppress nor manufacture an owned decision. |
| A real B blocking producer while A is not blocked, with controlled interleaving | A never fails on B's reason. Also retain genuine A blocking, multiple A reasons, and query-error controls. |
| A final-phase no-successor refresh and a real next-phase refinement while B is active | B's canonical rows and revision stay intact; no broad removal followed by compensating reload is required for correctness. |
| Missing owned rows before selection/completion | Preserve the existing reload-and-reask behavior; do not replace missing evidence with a Go completion or eligibility decision. |
| Failed snapshot validation, transaction/rebuild/load errors, and cancellation at publication boundaries | No silent peer deletion, false success, or unreported owner-state uncertainty. |
| Distinct nonconflicting scopes versus conflicting file scopes | Scheduling ownership is correct in both; existing write coordination still enforces the conflict case. |
| Single ordinary campaign, single recurse attempt, separate kernels, and process restart | Preserve normal throughput, acceptance obligations, durable admission, and recovery semantics. |

Use deterministic adapters and the production policy/consumer composition first, including an actual Cortex-backed fixture and canonical `ToFacts` publication. Do not “prove” the issue only with an impossible permanent block row, a fabricated derived conclusion, or a mock that ignores ownership. For a full acceptance-failure interleaving, produce the blocking row through the real acceptance path and observe its lifetime. Only after those checks should matched end-to-end runs compare current behavior, the shared-lease containment, and fully owner-scoped scheduling. Compare completed authorized work at matched verification obligations, not merely fewer calls caused by refusing all work.

## Review boundary and branch screen

This pass read root/public-repository guidance, campaign execution guidance, the three related ownership audits, phase-selection QA and journey references, and recent research inventories through [`research-main-20261030-1147`][latest-research]. The 1147 branch adds capability-checkpoint, generated-learning publication, multistep literal, and retrieval module-directive audits; none is this scheduling-domain composition.

The active source/QA heads inspected were `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. Tree comparison and targeted source reads found the same shared builder, campaign rule bodies, first-row consumers, recurse-only admission check, and broad refresh mechanism in the relevant older slices. Other files differ; this is not a claim that those branches are wholly identical to main or contain no unrelated improvements. No superseding repair for this contract was identified in that bounded screen.

## Immutable evidence

[prior-ui]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-ui-run-ownership.md
[prior-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-generation-admission-and-recovery.md#L117-L173
[prior-seed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-context-seed-scheduler-ownership.md
[prior-qa]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/2026-07-21_00-29-35_campaign_orchestrator_phases_qa.md
[prior-journey]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/00-journey-map.md
[start-command]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L660-L685
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L325-L387
[ui-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_ownership_test.go#L22-L74
[owner-lock]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/campaign_owner.go#L17-L65
[managed-start]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L29-L148
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L108-L132
[recurse-command]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L697-L725
[recurse-start]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L151-L180
[recurse-loop]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L184-L257
[recurse-messages]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L341-L361
[builder]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L79-L140
[set-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_lifecycle.go#L13-L59
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L33-L134
[rules-core]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_core.mg#L5-L100
[rules-phases]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_phases.mg#L30-L98
[rules-tasks]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L114-L128
[phase-reader]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L14-L45
[run-selection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_execution.go#L173-L257
[next-phase]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L343-L451
[block-reader]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L298-L313
[phase-block]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L296-L309
[refresh-call]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L152-L247
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L354-L413
[no-next]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L440-L474
[retract-real]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_facts.go#L880-L909
[retract-cortex]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/cortex_kernel.go#L517-L537
[reload-completion]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L226-L294
[reload-eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L68-L106
[scoped-sync]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/campaign_fact_sync.go#L34-L79
[replan-sync]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L780-L791
[keyed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_decisions.go#L15-L57
[latest-research]: https://github.com/theRebelliousNerd/codenerd/commit/921628ea7b1af635cb9c99d4055f6feed2a5efeb

[loop-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L329-L392
[git-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L53-L76
[hold-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands.go#L142-L150
[workspace-hold]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/recurse_workspace_hold.go#L16-L21
[recurse-lock]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_control.go#L12-L55
[campaign-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands.go#L238-L241
[mutation-hold]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L401-L409
[acceptance-producer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_acceptance.go#L149-L211
[retry-producer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure.go#L80-L115
[backoff-rules]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L72-L120

