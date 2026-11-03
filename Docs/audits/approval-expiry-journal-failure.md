---
doc_class: audit-addendum
subsystem: request-bound approval expiry
status: proposed
last_verified: 2026-11-02
verified_commit: a9990d7fc47110b43d546008517cbf3b32f7dee2
method: static source and documentation inspection only
supersedes: []
---

# Approval expiry must not depend on successfully recording expiry

## Finding and contract

**A request can remain eligible in memory after its deadline because recording its expiry failed. If the immediately following reservation append succeeds, a previously issued grant can be claimed late.** Separately, an undecided request under persistent, promptly returning journal failures can repeatedly retry expiry using already-due timers.

The narrow fault is in `expireLocked`: it calls `transitionLocked(..., StatusExpired, ...)` but ignores the error. The transition helper deliberately leaves the live record unchanged unless its journal append succeeds. `Reserve` then checks the unchanged status, rather than independently refusing the elapsed deadline or propagating the failed expiry attempt. [Expiry and transition ownership](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L275-L296) [Reservation after expiry attempt](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L475-L514)

The governing contract is the repository's own `RequestTTL`: it covers waiting for a decision **and then claiming the approved call**, with a default of 15 minutes. This is not an externally imposed JWT rule. The deadline is set when the broker opens the request and is not renewed by approval. [Decision-and-claim TTL](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/config/approval.go#L15-L31) [Deadline creation](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L253-L271) [Approval preserves that deadline](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L433-L449)

This is a source-derived conditional finding, not a runtime reproduction, observed incident, frequency estimate, or demonstrated exploit. No tests, application/model inference, or repository modifications were performed. Persistent storage failure alone does not authorize a dispatch; the late-claim witness requires a particular failure followed by recovery.

## Existing approval design remains load-bearing

The current broker is not a missing or cosmetic approval gate. It validates the frozen subject and sealed preview; a trusted host supplies actor/channel identity; decisions must match request version and digest; event IDs are deduplicated. Reservation checks execution intent and the current subject digest under a mutex, and a consumed claim cannot simply be claimed again. [Frozen request validation](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L158-L175) [Decision checks](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L396-L449) [Claim checks](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L475-L514)

Approval, reservation and dispatch-start publication each retain commit-before-use checks. The executor must not dispatch when `DispatchStarted` fails. Restart reconciliation does not invent a decision or resume the lost continuation: awaiting/approved records expire, reserved records become admission-failed, and started dispatches become outcome-unknown. [Durable dispatch barrier](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L529-L551) [Restart reconciliation](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/journal.go#L102-L127)

The shipped [request-bound human approval audit](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/Docs/audits/request-bound-human-approval.md#L199-L239) documents those controls and healthy expiry behavior. This addendum concerns the expiry/journal-error composition, not the old broad action-type grants. It is also separate from the [oversized-preview metadata audit](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/Docs/audits/approval-oversized-preview-metadata.md) and [generated-tool retirement identity audit](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/Docs/audits/generated-tool-retirement-revision-identity.md): the witness below uses an unchanged subject and an already constructed preview.

## Production route

Use a normal, eligible modular action that the constitutional gate marks approvable, with a live TUI host and parent execution context. A well-formed deletion requiring review is one concrete route; not every deletion or tool invocation reaches approval.

| Boundary | Current source behavior |
|---|---|
| Boot | `initApprovalBroker` resolves policy and opens the shared workspace journal. A journal-open failure leaves approval unavailable, so approvable denials stay refused. [Broker boot](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/system/approval_boot.go#L16-L43) |
| Wiring | Cortex installs the broker on the session executor. TUI boot attaches its host; decisions go through `Host.Decide`, not model prose. [Executor injection](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/system/factory.go#L2633-L2641) [TUI attachment](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/cmd/nerd/chat/model_update.go#L770-L776) [UI decision handling](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/cmd/nerd/chat/approval_host.go#L147-L168) |
| Admission | The executor routes an approvable, non-generated denial to `awaitHumanApproval`. Hard denial remains denial. [Approval entry and gate rerun](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/executor_tools.go#L2572-L2591) |
| Human wait | The helper freezes subject/preview, invokes `Await` or person-only `AwaitPerson`, and returns a tool error on refusal. [Request and refusal path](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/human_approval.go#L281-L316) |
| Revalidation | After receiving a grant, it rebuilds the subject from current state before invoking `Reserve`. This gives a real interval between a timely grant and the claim; scheduling or revalidation can cross the deadline without changing the subject. [Rebuild then reserve](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/human_approval.go#L317-L328) |
| Execution | Reservation success is followed by constitutional re-admission, project write guards, executive preflight, and successful dispatch-start journaling before the ordinary backend. These remain prerequisites. [Later guards](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/executor_tools.go#L2639-L2724) [Dispatch and backend](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/executor_tools.go#L2757-L2770) |

The per-tool timeout context is created after approval and intervening gates; it is not a separate timeout covering the human wait. The parent context may still cancel the whole operation. The witness explicitly keeps it live. [Tool timeout placement](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/executor_tools.go#L2744-L2755)

## Minimal late-reservation schedule

Let `E` be the request's fixed expiry. Use one request, one valid decision, one grant, matching intent and digest, and no restart or competing broker mutation.

| Step | Time/state | Consequence |
|---|---|---|
| 1 | Before E | The host approves and its append succeeds. `Await` returns the valid grant before E. |
| 2 | Deadline passes | The executor's subject rebuild succeeds with the same digest; the grant has not been reserved. |
| 3 | `Reserve` checks at or after E | Under the broker mutex, `expireLocked` attempts `approved → expired`. |
| 4 | Expiry append fails | `transitionLocked` returns an error without updating the live request; `expireLocked` discards it. Status remains `approved`. |
| 5 | Storage recovers for the next append | The same `Reserve` call accepts the approved status and matching identity, builds `reserved`, and successfully appends it. It returns nil. |
| 6 | Later guards succeed | Successful `DispatchStarted` journaling permits the caller to enter the ordinary backend. |

All critical transitions are visible in [Reserve](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L475-L514) and [the ignored error](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L275-L296). A fresh timestamp is placed in the late claim. There is no second direct deadline refusal after the failed expiry attempt.

The error/recovery ordering is essential. If the expiry append succeeds, reservation returns `ErrExpired`. If storage stays failed, the reservation append returns an error. If recovery occurs before the expiry append, expiry succeeds and the claim is refused. Recovery between these adjacent append attempts is possible at the control-flow level, but its production likelihood was not measured. A deterministic fault seam would establish the branch independently of a particular filesystem's recovery timing.

This is **not resurrection of a committed expired state**: the live state never became expired. Nor is it permission for a new subject, a second claim, or a call with no prior decision. The failure is that an approval valid earlier can pass the claim's freshness boundary after its stated validity has elapsed.

### Late claim is not late completion

The expiry helper intentionally operates only on `awaiting_human` and `approved`; `reserved` is governed by dispatch/admission settlement instead. Therefore a claim validly obtained before E may dispatch or complete after E under the present lifecycle. That alone is not this finding, and a repair should not silently turn the approval TTL into a deadline that aborts already-claimed effects. [States subject to expiry](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L275-L279) [Claim and dispatch transitions](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/request.go#L58-L66)

This witness also avoids the separate specification question of a reservation checked just before E whose durable append completes after E: its eligibility check already occurs at or after E. A revised contract should state the exact claim linearization point rather than infer it from function-return time.

## Persistent fault: expiry retries and cancellation

For an undecided request, failed expiry leaves `StatusAwaitingHuman`. `Await` then allocates a new timer for `ExpiresAt.Sub(now)`, which is nonpositive once the deadline has passed. After that timer fires, the loop attempts the same expiry again. There is no backoff or surfaced expiry-journal error in this path. With quickly returning failures, the result can be a rapid retry loop rather than a terminal expiry response. [Await loop](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L176-L214)

Go's timer implementation schedules nonpositive durations at the current runtime time. This supports the already-due-timer mechanism, not a measured CPU rate or a guarantee of zero scheduling delay. [Go timer implementation](https://go.dev/src/time/sleep.go)

**Cancellation is still present.** The select includes `ctx.Done()`; when selected, it attempts cancellation and returns `ErrCanceled` even if that journal transition fails. The returned snapshot can retain the old state. A ready cancellation has no guaranteed priority over other ready cases, and the function must still obtain the broker mutex and perform synchronous journal work around the select. Thus neither “cancellation is ignored forever” nor a fixed prompt-return bound is justified. [Go select semantics](https://go.dev/ref/spec#Select_statements), [Cancellation branch](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L199-L213)

The broker mutex is released between loop iterations, but held during each transition append. Slow I/O can delay other broker operations; fast failures cause repeated work. Adding a sleep would reduce retry pressure but would not fix expired authority remaining eligible.

Other access paths share the dependency: `Pending` can retain a past-deadline request when expiry recording fails; `Decide` can accept a late decision if its expiry append fails and its following decision append succeeds. A late decision alone does not prove execution, because `Await` and `Reserve` check expiry again. The pre-expiry-grant schedule above is the narrower dispatch witness. [Pending](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L348-L360) [Decision after expiry attempt](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L426-L449)

## Persistence uncertainty, clock, and recovery

Journal append marshals a complete request snapshot, writes it through an `os.File`, then calls `Sync`. A failure does not establish that no bytes were written: a write may be partial, and a sync failure occurs after a successful write. The code does not roll back that append. This is why the witness specifies the returned append error and unchanged live state rather than claiming the expiry record is absent from storage. [Append sequence](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/journal.go#L130-L145), [Go file-write contract](https://pkg.go.dev/os#File.Write), [Linux sync semantics](https://man7.org/linux/man-pages/man2/fsync.2.html)

Repeated successful writes followed by failed syncs can also add repeated expiry snapshots while the live request remains open; zero-byte write failures need not grow the file. Neither persistence extent nor universal crash durability is established by the error alone. Restart retains its conservative reconciliation and does not resume execution merely because it finds an old approval.

The default broker clock is `time.Now`; expiry uses `!Before`, including equality. Live timestamps created by `time.Now` and advanced by `Add` preserve Go's monotonic component for comparisons. No wall-clock manipulation is required. Serialized timestamps lose that component, but restart already reconciles open requests instead of reviving their continuation. Tests using injected clocks should identify that distinction. [Clock selection](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L48-L70), [Go monotonic-clock semantics](https://pkg.go.dev/time#hdr-Monotonic_Clocks)

## Proposed authority and journal contract

**An eligibility check at or after E must refuse a new decision/claim independently of whether recording that refusal succeeds.** Authority status and journal health are different facts.

A focused repair can make expiry return an explicit outcome/error and require each consumer to stop on elapsed validity. Two implementation approaches deserve evaluation:

1. Derive “expired for authorization” directly from status plus deadline, return refusal immediately after the synchronous attempt, and expose any journal error separately.
2. Move the live record to a terminal, fail-closed state even when terminal journaling fails, retaining explicit unsaved-transition/error state for reconciliation.

Either approach needs consistent `Await`, `Pending`, `Decide`, `Reserve` and status-view behavior. A caller must not continue toward approval/reservation merely because the durable expired snapshot is unavailable. Under the second approach, do not falsely describe the terminal state as durably recorded; define retry ownership, ordering and shutdown behavior. Existing `finish` already advances a spent claim's in-memory terminal outcome despite journal failure, while retaining an error reason, demonstrating a related distinction—not a blanket instruction to apply it to every transition. [Existing terminal settlement behavior](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker.go#L565-L586)

Preserve commit-before-use for authority-creating transitions and the dispatch barrier. Do not “fix” expiry by making successful approval or reservation memory-only. Preserve subject revalidation, single claim, trusted host checks and conservative restart handling. If a separate journal repair worker is introduced, it needs bounded queues/backoff, lifecycle ownership and observable incomplete shutdown; a goroutine wrapper alone does not cancel or join blocked file I/O.

This aligns with Saltzer and Schroeder's principle of basing permission on satisfied authorization conditions, rather than the absence of a successfully recorded exclusion. [Fail-safe defaults](https://web.mit.edu/saltzer/www/publications/protection/Basic.html). JWT expiry provides a useful acceptance-time analogy, but RFC 7519 governs JWTs, not this in-process broker; its optional clock-skew leeway does not authorize extending CodeNERD's TTL on journal failure. [RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)

## Costs and proposed evaluations

A direct deadline refusal is cheap, but truthful persistence-health state and reconciliation are not free. They add API/error semantics, status/UI changes, queue ownership if asynchronous, and recovery tests. Keeping synchronous I/O preserves ordering but retains lock-held latency. Asynchronous terminal recording can improve responsiveness only with additional ordering and shutdown guarantees; it must not permit old snapshots to overwrite newer settlement on replay.

The current loop can spend timers, allocations, serialization CPU, writes/sync attempts and shared-broker lock time without model progress. Measure those costs rather than translating them into unsupported token or latency savings. Reduced retries may lower resource pressure, but may delay an audit repair if backoff is introduced. Returning refusal can require the person to review a new request; include that operator effort rather than treating fail-closed behavior as costless.

Existing tests verify healthy expiry and cancellation. Broker expiry tests use no journal; the session expiry case uses a real journal without fault injection. These remain useful positive controls but do not establish behavior when expiry append fails and the next append succeeds. [Broker controls](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/approval/broker_test.go#L303-L348) [Executor expiry control](https://github.com/theRebelliousNerd/codenerd/blob/a9990d7fc47110b43d546008517cbf3b32f7dee2/internal/session/human_approval_test.go#L220-L230)

**Proposed evaluations only; none were run:**

| Case | Controlled schedule | Acceptance evidence |
|---|---|---|
| Healthy timing | Decision/claim before E; claim at E and after E | Timely unchanged subject works once; elapsed eligibility refuses |
| Primary witness | Obtain grant before E; check Reserve after E; fail only expiry append; next append would succeed | No reservation or dispatch; expiry and journal failure both observable |
| Persistent fault | Undecided request past E; repeated prompt errors | Terminal refusal or explicitly bounded reconciliation, not already-due timer churn |
| Cancellation | Cancel during failed-expiry waiting, separately delay I/O | Returned cancellation and worker completion distinguished; no unsupported hard latency bound |
| Storage stages | Zero-byte write failure, partial write, successful write/failed sync | No “error means no record” assumption; replay remains conservative |
| Recovery ordering | Recover before expiry append, between expiry/reservation, and after reservation failure | Recovery never revalidates elapsed authority |
| Late decision | Fail expiry append in Decide, permit subsequent writes | Late decision refused; valid event identity does not override time |
| Identity and replay | Changed subject, wrong intent, duplicate event and concurrent claims | Existing binding and one-shot controls unchanged |
| Already claimed | Reserve validly before E, dispatch/finish after E | Existing claim lifecycle preserved; no new automatic replay or rollback |
| Production route | Real executor/host boundary and ordinary gates, harmless instrumented backend | Dispatch counter stays zero on expired eligibility; healthy control dispatches once |
| Restart and views | Restart after each failure stage; inspect Pending/Get/status/UI | No recovered continuation or invented decision; durable versus live status explicit |
| Cost | Fixed request/preview sizes; varying waiting requests and failure duration | Append/sync counts, timer/allocation counts, CPU, lock wait, bytes, wall time and reviewer effort reported |

Use deterministic clock and narrowly scoped journal fault seams for the exact adjacent-append sequence; do not call that a reproduced filesystem incident. Add bounded integration checks through the real file journal and production entry route separately. Keep authorization scope, source data, preview size, downstream gates, fault stages and runtime conditions identical across current and proposed implementations. No host-disk exhaustion experiment or live destructive tool is necessary.

**Exit criterion:** a failed expiry record cannot preserve eligibility for a late decision or claim, persistent failures do not turn expiry into uncontrolled immediate retries, and the journal's uncertainty is reported without weakening existing approval, dispatch or recovery protections.


## Landed implementation update 2026 11 03

Frozen source review at `86ec71ddeab546141b2def8610418aa6a662ee5f` on 2026-11-03. The original document above is preserved unchanged as historical analysis. The bounded remedy was introduced by [79fde7a0](https://github.com/theRebelliousNerd/codenerd/commit/79fde7a0d87a27ea6d97d80fe988dfc2e95f432c) and reached main through [4b971412](https://github.com/theRebelliousNerd/codenerd/commit/4b971412a29ff94c0d032d861dfce128dcdf58a3). This update inspected source and recorded verification claims; it ran no tests, application/model inference, benchmarks or production operations.

**Implemented scope:** the late-reservation mechanism in finding #1 is remedied in the inspected source. If the expiry transition fails, `expireLocked` now sets the live request to `StatusExpired`, records the journal error in its reason, and notifies waiters. `Reserve` calls expiry before inspecting status and refuses `StatusExpired` before any reservation append. This is a fail-closed live-state change, not a claim that the expiry was durably recorded. [Expiry fallback](https://github.com/theRebelliousNerd/codenerd/blob/86ec71ddeab546141b2def8610418aa6a662ee5f/internal/approval/broker.go#L275-L301) [Reservation gate](https://github.com/theRebelliousNerd/codenerd/blob/86ec71ddeab546141b2def8610418aa6a662ee5f/internal/approval/broker.go#L480-L519).

**Repository-reported verification:** the [queue row](https://github.com/theRebelliousNerd/codenerd/blob/86ec71ddeab546141b2def8610418aa6a662ee5f/Docs/audits/ASTRA_QUEUE.md#L210) marks #1 partially verified-fixed at `4b971412` and records `go test -p=4 -count=1 -run '^(TestReserve_ExpiredApprovalFailsClosedWhenExpiryJournalFails|TestExpireLocked_FailingJournalStillExpiresElapsedRequest)$' ./internal/approval` exiting 0. That receipt was read, not independently reproduced; it covers the named tests, not the full audit exit criterion.

**New fixture, unrun in this review:** [86ec71's regression](https://github.com/theRebelliousNerd/codenerd/blob/86ec71ddeab546141b2def8610418aa6a662ee5f/internal/approval/broker_expiry_journal_failure_test.go#L97-L198) closes the journal, directly invokes expiry, reopens the journal, then checks that `Reserve` returns `ErrExpired` and the request stays expired. This adds the healed-journal scenario omitted by the queue's older receipt. It does not attempt a successful reservation append, exercise later dispatch, or provide an execution receipt for this new test. The inspected queue has no receipt for it.

**Remaining limits:** persistent-failure waiter and status-view behavior, durable/live-state reconciliation, restart, production dispatch, blocked I/O and cost measurements were not independently executed or established by these focused receipts. The historical mechanisms above must be read against their original commit; broader proposals are not automatically implemented or verified by the #1 remedy.
