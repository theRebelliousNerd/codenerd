---
doc-class: deep-dive
subsystem: cli
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-10-29
verified-against: 9d26d9c3af600a5d01f904b138974e02ec7838c4
supersedes: []
---

# Campaign messages must remain bound to their originating run

The TUI can accept a second distinct campaign while the first campaign still has a pending listener. Starting the second campaign replaces the model's single campaign slot, but an authentic progress message from the first campaign can subsequently update that replacement slot. The narrow defect is missing message ownership at the TUI campaign boundary, not failed campaign admission or an incorrect execution verdict.

This is a focused addendum applying the existing side-turn owner pattern to the separate background campaign slot. The recommended contract is to bind every campaign message and control operation to an accepted run, then choose explicitly between one managed campaign and a multi-run registry. It is not a claim that ownership fencing is a new architectural idea.

## Evidence and relationship to existing audits

This source-only assessment cites main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, resolved at 19:39 UTC on 29 October 2026. Main advanced to `9d26d9c3af600a5d01f904b138974e02ec7838c4` before the 19:49 recheck. A complete, untruncated tree comparison found the campaign/UI slice and cited guidance unchanged; the intervening commit changes provider transport, configuration, pricing, and related tests and documentation. The finding therefore remains applicable at the newer head, while links retain the inspected immutable source pin. No tests, application runs, model inference, or benchmarks were executed. The interleaving below is derived from control flow, not an observed incident.

The [side-turn ordering audit][side-turn] already discusses unowned terminal messages and a foreground owned-slot design. The [background outcome audit][background] addresses job and session identity. The [campaign admission audit][admission-audit] concerns per-ID publication, generation, and recovery. This addendum addresses a different receiving boundary: the singleton campaign fields in the chat model after an ordinary background campaign has released foreground loading. Those earlier contracts remain applicable.

The research corpus at `75240b6649e9e7fce0c117ac5b03c6f4f0a95840` contained 116 top-level Markdown audits. All were screened for targeted campaign identity and current-message terms; relevant predecessors were read. This did not identify an exact treatment of this campaign-slot interleaving. That is a bounded corpus result, not a universal novelty claim.

At 19:44 UTC, all 13 checked production paths matched main on lanes `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908`. QA `51cfd42649552866d4aa54c53b68acf5740eb09b` differed elsewhere in the command-handler file, but its campaign-command through live-control segment matched. These checks support parity of this mechanism, not entire-branch equivalence.

## How the second campaign becomes current

When campaign A starts, the [started-message handler][start-update] clears `isLoading`, installs `activeCampaign`, `campaignOrch`, progress and event channels, and schedules execution and listeners. That background installation does not enroll a foreground turn. Consequently, in ordinary ChatView, with no modal or clarification intercepting input, `isLoading == false` and `turn.current() == 0` can permit Enter to reach submission. Slash input then reaches command handling. These are explicit witness preconditions, not assertions about every UI state. [Input dispatch][input-dispatch]; [slash routing][slash-routing].

The [campaign-start command][start-command] does not check for an existing `activeCampaign`. A user can therefore submit `/campaign start B` while A remains live. The command marks loading while B is planned. A successful [start operation][campaign-start] passes dependency checks, decomposes the requested campaign, calls `SetCampaign`, and produces a started message.

B must be a distinct, valid campaign whose real planning, admission, and storage succeed. No particular model output is guaranteed. A failed decomposition or rejected admission does not install B and is an essential negative control.

Several existing protections do not close this particular path:

- The [campaign owner lock][campaign-owner] is keyed by campaign ID. It protects competing ownership of the same campaign, not two different IDs.
- [`SetCampaign`][set-campaign] performs inclusion checks, loads facts, and persists the campaign. Successful admission through a fresh orchestrator does not reserve the TUI's singleton slot.
- The [execution guard][execution-guard] rejects an already-running orchestrator. B has a fresh orchestrator, so A's running state does not activate that guard for B.
- Foreground turn dispatch protects its own slot. Making background campaigns occupy that slot indefinitely would block unrelated chat rather than supply the missing campaign ownership contract.

When B's started message arrives, the same handler replaces the model's current campaign pointers and channels. It neither cancels nor joins A before replacement. The campaign-ID lock and durable admission mechanisms can both work correctly while this local UI ownership transition remains unsafe.

## The minimal progress witness

A sufficient source-level witness is:

1. A is live, and a previously issued A progress listener remains pending.
2. The user starts distinct B under the input conditions above.
3. B successfully plans and starts; its started message installs B in the current campaign slot.
4. The pending listener delivers authentic progress from A with counts observably different from B's current counts.

The [listener closure][campaign-listeners] captures its source channel. Replacing the model's channel fields therefore does not retarget a command already listening to A. Progress publication is nonblocking, and [`GetProgress`][get-progress] returns a campaign ID and counts. Delivery is conditional: this argument requires an A observation actually reaching the listener, not every attempted progress send succeeding. A pending observation or heartbeat is sufficient; A need not complete. [Progress publication][progress-publication].

The progress payload [already contains `CampaignID`][progress-type]. The [receiving handler][progress-update] nevertheless assigns the payload to `m.campaignProgress` and copies its counts into the currently active campaign without checking that identity. Under the witness, the active campaign is B. The handler updates the campaign page using that current campaign and rearms a listener from the current model channel, rather than the original listener's captured A channel.

The event order is not a claim about timing observed in a live run. The repository pins [Bubble Tea v1.3.10][tea-version], whose [Batch contract][tea-batch] permits concurrent commands without guaranteeing result order. A's [heartbeat producer][heartbeat] supplies a normal progress source; the witness still requires delivery after B's installation and before any other event supersedes the relevant state.

The resulting defect is concrete: B's current pane or counts can adopt A's progress, and listener continuation follows mutable current state rather than message provenance. Equal counts could conceal the visual symptom; the evaluation must deliberately use distinct counts. None of this reasoning depends on asserting a Go data race.

## Terminal consequences and important limits

A terminal extension is conditional on genuine A terminal evidence being delivered after B becomes current. The [progress listener][campaign-listeners] can produce a completed message carrying its captured A campaign. The [completion handler][completion-update] clears the current campaign fields without proving that the current slot still belongs to A. If B occupies that slot, B's local control handle is lost.

This does **not** falsely render B as completed: the completion message carries A, and the rendered result refers to A. Nor does clearing the TUI fields stop B. Its [execution goroutine][campaign-run] retains its orchestrator, uses the parent shutdown context, participates in goroutine tracking, and defers closure. The [shutdown path][shutdown] cancels the root context and attempts a bounded wait for tracked goroutines, with a five-second warning path. This is not a claim of an unkillable process, work outside global shutdown, or guaranteed completion of every cancellation.

An A hard-risk event has a similar conditional consequence through [risk rendering][risk-render] and the [event handler][event-update]. The finding does not require that risk timing, or A completion, to be reachable in every scheduler arrangement. The progress witness alone establishes the ownership gap.

Current controls provide useful safeguards: [`liveCampaignControlProgress`][live-control] refuses absent, missing, or mismatched live campaign control state. After the conditional slot clear, this refusal prevents accidentally controlling an arbitrary campaign, but leaves the user without B's former local pause or resume handle. The [CLI gap analysis][cli-gap] already requires matching live control and blocking new tasks while paused.

The supported consequences are wrong local presentation and conditional loss of the current run's control handle. They do not establish a permissions bypass, wrong task execution, a false kernel verdict, durable data corruption, broken per-ID locks or journals, or universal campaign failure.

## A campaign message ownership contract

The repair should distinguish a logical `CampaignID` from a unique accepted UI run ID or incarnation. The latter begins with accepted planning and remains stable through execution and settlement. Reusing a logical campaign ID must not make delayed messages from a previous incarnation authoritative.

Every started, progress, event, error, completed, and replan acknowledgement should carry that owner. The current [message shapes][message-types] are uneven: progress has campaign identity, completion carries a campaign pointer, and the error shape carries only an error. The [orchestrator event type][event-type] lacks campaign identity. This is not a claim that identity is absent everywhere; it is a requirement for an explicit, uniform receiving contract.

Before changing loading, pointers, counters, visible state, or channel subscriptions, the UI must validate ownership. A stale message may be recorded under its originating run, but cannot mutate a replacement run. Stale errors must not clear a new run's loading state. A listener must rearm using its captured owner and source channels, never whichever channel happens to be current when its message is handled.

Progress should be an immutable snapshot with a per-run sequence or equivalent ordering rule. Terminal evidence needs an idempotent receipt so duplicates, reordering, and pane changes do not discard a completed outcome. Suppressing stale UI authority is not permission to discard old outcomes. Controls must resolve an explicit owner and its retained handle, not infer ownership from the selected pane.

These are host identity and lifecycle mechanics appropriate for Go. Mangle should continue to own applicable admission and priority policy. The change must preserve authorization, verifiers, work modes, and lifecycle rules; it does not require new model calls or an external broker.

## Choose an admission policy rather than overwriting silently

The smallest coherent policy is one managed campaign reservation, including planning. Reserve it before asynchronous planning starts. Reject or queue a second request with an accurate user-facing explanation, and release it only when the owner settles. This reduces concurrency and may add queue latency, but gives the singleton UI an honest contract. The reservation itself must be atomic with admission; checking only after planning leaves an admission race.

A multi-run registry is the alternative when concurrent campaigns are intended. It separates visible selection from owned execution handles, cancellation and join state, source channels, and outcome history. Every accepted run needs continued draining and bounded retention. A map alone does not qualify shared-kernel or workspace scheduling between campaigns; those interactions require separate evaluation.

Replacing a campaign should therefore require explicit user intent and an explicit cancel-and-join or retain-and-switch policy. Neither silently overwriting pointers nor indiscriminately stopping all campaigns is acceptable. Do not extend the foreground turn slot across a background campaign merely to prevent the witness: users should retain ordinary chat unless an established policy requires otherwise.

Cancellation must not itself release an execution reservation: Go's [CancelFunc contract][go-cancel] requests abandonment but does not wait for work to stop. Establish settlement separately, retain uncertain outcomes, and do not claim a timed-out join means effects have ended.

## Costs and evaluation to run

Owner checks add constant-size message metadata and O(1) identity comparisons. A registry requires O(N) state for active and retained runs, with a documented retention bound. Immutable snapshots add copying and allocation costs. Channel backpressure, terminal receipt retention, history truncation, and cleanup must be designed together so a quiet or hidden run cannot lose its final outcome.

The single-campaign option trades capability and queue latency for simpler ownership. The registry trades implementation and shared-resource complexity for concurrency. Neither has a measured performance advantage here. Measure retries, lost work, human recovery time, and total resource use rather than treating fewer visible errors as proof of lower cost.

Evaluation should exercise the normal production TUI with real Cortex, world, and CodeDOM integration, while deterministic adapters control planning results and delivery order. The ownership mechanism itself needs no LLM inference. Use real control-command dispatch and `Update` ordering rather than only constructing isolated payload structs.

Compare baseline, message fencing alone, single-reservation admission, and a registry under matched tasks, authorization, model settings, and budgets. Proposed cases include:

- A progress after B starts, with distinct counts; A progress while B is planning.
- A completion, risk event, or error after B starts, conditional on authentic producer evidence.
- Same logical ID across different incarnations; duplicate and out-of-order progress.
- Planning or admission failure; same-ID lock rejection; concurrent admission attempts.
- Channel closure, delivery failure, cancellation, quit, and retained terminal evidence.
- One-campaign pause and resume; missing-control refusal; ordinary input without a modal or foreground turn.
- Origin-session separation if supported, without presenting that existing ownership issue as a new finding.

Existing [control tests][control-tests] cover a single live pause and resume path, with separate [missing-transition tests][missing-control-tests]; [risk tests][risk-tests] cover a single blocked event, advisory handling, and ignored non-risk events. They do not demonstrate this overlapping-run contract.

Success requires correct pane ownership, controls addressing only their owner, retention of every accepted outcome, no false completion attribution, and reliable terminal delivery. Evaluate task quality with independent verdicts. Record human resolution latency, total model tokens including failed or canceled attempts, CPU, memory, channel behavior, and wall time. These tests and live evaluations remain proposed and unrun.

[side-turn]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/side-turn-ordering-and-cancellation-ownership.md#L237-L264
[background]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/background-outcome-and-continuation-ownership.md
[admission-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/campaign-generation-admission-and-recovery.md
[start-update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L312-L326
[input-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_key_handler.go#L247-L259
[slash-routing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_handlers.go#L104-L128
[start-command]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers.go#L631-L650
[campaign-start]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign.go#L32-L88
[campaign-owner]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/campaign_owner.go#L17-L65
[set-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_lifecycle.go#L20-L59
[execution-guard]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_execution.go#L23-L27
[campaign-listeners]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign.go#L148-L194
[get-progress]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_control.go#L145-L159
[progress-publication]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_utils.go#L120-L130
[progress-type]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/types.go#L508-L527
[progress-update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L330-L346
[completion-update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L379-L387
[campaign-run]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign.go#L98-L140
[risk-render]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign.go#L201-L218
[event-update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L349-L369
[live-control]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers.go#L765-L790
[cli-gap]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/architecture/cli/03-GAP-ANALYSIS.md#L27-L33
[message-types]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_types.go#L776-L786
[control-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_control_test.go#L70-L178
[risk-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_risk_test.go#L82-L149
[event-type]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_types.go#L132-L139
[shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_lifecycle.go#L31-L61
[missing-control-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_control_test.go#L180-L228
[tea-version]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/go.mod#L11
[tea-batch]: https://pkg.go.dev/github.com/charmbracelet/bubbletea@v1.3.10#Batch
[heartbeat]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_execution.go#L325-L336
[go-cancel]: https://pkg.go.dev/context#CancelFunc

