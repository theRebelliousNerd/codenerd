---
doc-class: deep-dive
subsystem: browser
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 3441f47316923f134a1b6e06d6385f45d21c059c
verification-method: static-source-inspection
supersedes: []
---

# Browser epoch retirement can precede its last publication

## Finding and decision

**A browser event can be tracked for an epoch, successfully retired before it reaches the kernel, and then asserted with no remaining retirement owner.** `addStreamFacts` appends sanitized facts to `budget.tracked`, releases `budgetMu`, and only then calls the sink. `RollSessionEpoch` can run in that interval, detach the tracked set and successfully retract it. The exact kernel retractor does not remember absent facts: a later assertion is admitted normally. [Publication][stream] [Retirement][roll] [Kernel removal][kernel-retract]

This is a production-reachable ordering gap between the independent navigation and event listeners, not a missing retractor or an unsanitized retraction key. A correctly wired live kernel and successful calls are sufficient. A later session close correctly drains its producers, but cannot recover a fact whose tracking record was already discarded at an earlier navigation. [Concurrent listeners][listeners] [Close ordering][close]

**Required contract:** an epoch must retain responsibility for each admitted publication until its kernel outcome is settled. Retirement must order after all publications admitted to that epoch, or prevent those publications from becoming live. No successful old-epoch assertion may survive successful retirement without an explicit, owned recovery obligation.

The practical consequence established here is a retired-page fact left live and untracked. Its frequency, memory impact and effect on actual tasks are unmeasured. This does not establish an observed incident, a wrong browser effect, or a blanket failure of browser diagnosis. In particular, `browser_reason` defaults to timestamp filtering since the latest navigation, which can hide an older console event from that response even while it remains in the kernel. [Reasoning filter][reason-filter]

## Evidence boundary and relation to existing work

Public main was inspected at [`3441f47316923f134a1b6e06d6385f45d21c059c`][pin] on November 4, 2026, and rechecked unchanged at 15:19 UTC. Repository links below pin that revision. Inspection covered root and scoped guidance, browser architecture, event producers, epoch bookkeeping, live-kernel adapters, exact assertion/retraction semantics, exposed query paths, authored tests and relevant existing audits. Cached source was accepted only when its Git blob hash matched the pinned tree. No repository program, test, build, benchmark, browser fixture, model inference or training was run. All evaluations below are proposed and unrun.

[Root guidance][root] requires deterministic executive control, production-path evidence and current rather than stale world facts. [Architecture guidance][doc-guidance] requires intended capability, implementation and remaining gaps to stay distinct. [Browser guidance][browser-guidance] assigns event lifetimes and session evidence to the manager and live kernel. The fix belongs in those existing owners; it does not require another browser engine or reasoning layer.

The [research-browser ownership and evidence retirement audit][prior-browser] is substantive prior work. Its as-built section records runtime ownership, a fixed sink/kernel binding, retractor wiring, joined read sensors, sanitized tracking, lifecycle-marker retirement and exact retry after retraction failure. Its original contract also calls for joining producers before final retirement. This report preserves those gains and identifies a different remaining boundary: **live navigation turnover while a correctly tracked stream publication is still in flight**. The earlier report does not give this interleaving or an epoch-wide publication fence.

[World refresh publication order][prior-world] and [retrieval delta publication fencing][prior-retrieval] describe analogous ordering principles for graph/cache state. They do not cover browser event admission, the separate CDP listeners, an absent-fact retraction succeeding, or the lost browser retirement record. This is a subsystem-specific residual with a concrete witness, not a claim that publication fencing itself is a new idea. The current browser wiring document's “Fact epoch GC” row correctly establishes reachability; it does not establish atomicity against concurrent stream writers. [Wiring][browser-wiring]

| Capability | Status at the pin | Evidence and limit |
|---|---|---|
| Production browser sink and exact retractor target the live kernel | Implemented | Factory installs both adapters. [Factory][factory] [Write adapter][sink] [Retraction adapter][retractor] |
| Stream facts are sanitized before being retained and published | Implemented | Tracking keeps the same safe slice passed to the sink. [Stream path][stream] [Sink entry][publish] |
| Failed exact retractions retain a retry obligation | Implemented | Only an error adds the batch back to `pendingRetire`. [Retry][retire] |
| Navigation removes previously tracked facts | Implemented for settled publications; incomplete under overlap | Turnover detaches the tracked set before calling the retractor. [Epoch roll][roll] |
| Progressive control-snapshot replacement and epoch roll share serialization | Implemented | Both use `progressiveMu`; the stream path does not. [Snapshot][snapshot] [Roll][roll] |
| Session close drains the manager's event stream before final retirement | Implemented | `closeAndDrainSession` cancels, closes and joins before `forgetSessionBudget`. [Close][close] |
| Stream admission, publication and epoch retirement form one ordered lifecycle | Missing on the inspected path | `budgetMu` does not span the sink call; no epoch ticket reaches it. [Stream][stream] [Publish][publish] |
| Race frequency, throughput effect and task-quality impact | Unverified | Static inspection only; no measurements supplied. |

## Production path and causal witness

### The two writers really can overlap

Factory boot creates `SessionManager` with `browserKernelSink` and installs `NewKernelFactRetractor`. The write adapter calls `AssertBatch`; the removal adapter calls `RetractExactFactsBatch` on the kernel bound to that manager. This is not a test-only fact store. [Factory][factory] [Sink][sink] [Retractor][retractor]

`startEventStream` creates a navigation listener containing `handleNavigation` and a separate listener for console/network/DOM events. It starts their wait functions in different goroutines. A console callback constructs `console_event(SessionID, Level, Message, Timestamp)` and calls `addStreamFacts`. A top-level frame navigation invokes `rollForLoader`, which can call `RollSessionEpoch`. [Listener construction][listener-construction] [Listener goroutines][listeners] [Console producer][console] [Navigation producer][navigation]

Loader deduplication does not serialize these callbacks. It avoids rolling twice for one loader. For a concrete schedule, pause the console callback before the event listener processes the new document request; the independent navigation listener can observe the new frame/loader and perform the roll first. A navigation without a new network request is another intended reason to roll on frame commit. This witness needs a genuine accepted new loader or otherwise qualifying navigation, not a stale frame event rejected by the live-target check. [Loader and navigation checks][navigation] [Request-side roll][document-request]

### Minimal state and schedule

Let the open session be `S`, its current epoch be `E`, and its remaining event allowance be positive. Enable the ordinary live-kernel retractor. Use one fresh, valid fact with a unique message/timestamp so it is not a duplicate of any existing fact:

`F = console_event(S, "error", "old document diagnostic", T)`.

The console schema contains session and timestamp but no epoch argument. The example needs no secret, malformed data, remote attack or sink failure. Flight recording may be disabled; a scheduler pause between ordinary statements is enough. [Schema][event-schema] [Publication][stream]

| Step | Console-event goroutine P | Navigation goroutine N | Resulting ownership/state |
|---|---|---|---|
| 1 | Sanitize F, acquire `budgetMu`, charge the batch and append F to epoch E's tracked set. | | F is tracked, but not yet in the kernel. |
| 2 | Release `budgetMu`; pause before `publishSanitized`. | | The publication has no held epoch fence. |
| 3 | | Enter `RollSessionEpoch`; advance E to E+1, detach the tracked set containing F, clear the current tracked set and release `budgetMu`. | F is now only in the local retirement batch. |
| 4 | | Retract the batch. The kernel finds no F, removes nothing for that key and returns success. | No tombstone is created; no pending retry is retained. |
| 5 | | Publish the new epoch marker and finish the roll. The navigation callback can also publish its navigation facts. | E+1 owns its new facts, not F. |
| 6 | Resume; call the sink and successfully assert F. | | F is live but absent from the current tracked set and pending retirement set. |
| 7 | Finish the callback. | A later roll or close retires only the facts still tracked/pending. | That lifecycle operation has no key for F. |

Steps 1–2 follow `fact_epoch.go:267–303`; steps 3–5 follow `:184–226` and `:140–158`. The kernel serializes each assertion and retraction internally, but accepts this **retract-then-assert** order. Its removal function builds a set of exact keys, filters currently stored facts and returns nil when no matching fact existed. It does not register a future prohibition. Its assertion function accepts valid new facts and marks their predicates dirty for later evaluation. [Tracking/publication][stream] [Roll][roll] [Retirement retry][retire] [Kernel removal][kernel-retract] [Kernel assertion][kernel-assert]

Consequently, this is not solved by making the maps data-race-free. All bookkeeping accesses in the witness can respect their mutexes. The missing guarantee is ordering across the bookkeeping and kernel operations.

### Why existing cleanup and query protections do not disprove it

- **Correct retractor wiring:** necessary, but an exact removal before insertion cannot remove a future insertion. No error is required.
- **Retry on removal failure:** valuable, but no failure occurs in the minimal witness, so `PendingRetirements()` can be zero while F is live. The method reports queued cleanup keys, not an independent census of all residual facts. [Pending counter][pending] [Retry][retire]
- **Close drains callbacks:** this prevents a callback from publishing after that close's final cleanup. Here P already finished after an earlier roll lost its key. Draining P does not reconstruct the discarded ownership record; `forgetSessionBudget` uses only the remaining tracked/snapshot sets and pending batch. [Close][close] [Forget][forget]
- **Progressive observation lock:** it covers the control-snapshot replacement path and epoch roll. `addStreamFacts` never acquires it, so it does not cover this console event. [Snapshot][snapshot] [Stream][stream]
- **Timestamp filters:** default `browser_reason` refreshes page state, finds a navigation timestamp and filters time-bearing result rows. That can prevent this old diagnostic from affecting the default health response. It does not remove F or restore its owner. [Reasoning filter][reason-filter]
- **Bounded reads:** `browser_mangle`'s direct `read`/`query` paths allow session-scoped facts without automatically restricting them to the current navigation. Its scan/output limits bound returned-row enumeration and presentation, not fact lifetime or necessarily lazy kernel evaluation before callbacks. The tool is registered with the research tools. [Registration][registration] [Read dispatch][read-dispatch] [Session filter][query-filter]

The pinned browser rules can derive `root_cause_at` and `user_visible_error` from an error-level console event without an epoch join. Therefore a direct live-kernel query can observe old diagnostic consequences after the roll. This does not establish that every consumer treats them as current, or that any permission gate is bypassed. [Diagnostic rules][diagnosis]

The strongest supported lifetime statement is: **subsequent manager tracking-based retirement cannot remove this unique F unless it acquires its key again through another path.** A separate explicit cleanup, other retention mechanism or kernel teardown could still remove it. No claim of persistence forever or process-wide unbounded growth is needed.

## Required lifecycle contract

The following design is proposed, not implemented by this audit.

### Ownership and ordered settlement

For each session incarnation and epoch, distinguish admission, in-flight publication, settled live keys and retirement obligations. A publication carries the owner it was admitted under; a retirement operation may not discard that owner while a publication can still commit.

1. **Admission cutoff.** Epoch turnover closes E to new E-owned publication. It cannot simply relabel an already admitted E batch as belonging to E+1.
2. **Ordered settlement.** Each admitted batch either settles before retirement removes its keys, or is refused before inserting into the kernel. A post-retirement commit followed by unowned best-effort cleanup is insufficient.
3. **Ownership conservation.** Any facts that may have reached the kernel remain in a live tracked set or an explicit pending cleanup set until their removal is settled. A successful batch must not lose its keys; an error must not be interpreted as proof that nothing landed.
4. **Successful retirement.** After a successful retirement completion, no still-running publication admitted to E can make an E-owned fact live. If cleanup cannot finish, retain the exact keys and expose a pending/degraded outcome rather than claiming a clean boundary.
5. **Exact scope.** Retraction remains exact and uses sanitized keys. Do not compensate with an all-session or all-predicate sweep that could delete valid E+1 or other-owner facts.
6. **All owned producers.** Apply the same boundary to stream facts, caller-side `PublishSessionFacts`, saturation markers and progressive replacements. Preserve the existing close join; do not replace it with an unjoined cancellation signal. [Public sensor entry][public-publish] [Marker path][marker] [Snapshot][snapshot] [Close][close]

`AssertBatch` explicitly permits good facts to land while returning rejection errors for others. A publication design must retain enough ownership to clean up such partial effects. It may conservatively retain the attempted exact set when that is valid under the owner contract, or reconcile a precise admitted subset. Merely moving tracking after a successful return would introduce a different leak on partial failure. Shared identical-key support, if allowed, also needs ownership accounting rather than deleting another owner's support. The minimal witness avoids that separate alias issue. [Batch semantics][kernel-assert]

### Minimal implementation choices and their limits

**Baseline to compare:** serialize each session's full tracked publication and retirement boundary, not only the accounting update. The old writer either completes and is included in retirement, or cannot enter that epoch. A session-local operation lock or ordered command queue can supply this; a single manager-wide lock is simpler but unnecessarily couples independent tabs. This is a proposed baseline, not a claim that inserting one lock without a call-graph review is safe.

Do not hold `budgetMu` across arbitrary sink calls or acquire the manager lock underneath it. The current source expressly avoids that lock inversion. Account for synchronous kernel/event callbacks, flight-recorder I/O and cancellation before choosing lock scope. Waiting on publication must not deadlock the navigation callback that publication needs to finish. [Lock-order warning][stream] [Sink and recorder][publish]

**Alternative:** an epoch owner closes admission, tracks in-flight writers, waits for them to settle, then retracts the complete set. This permits more parallel publication, but must bound waiting/queues and preserve exact ownership on timeout. A generation check alone is not enough if the writer can pass it, pause and commit after retirement. Validation and permission to commit must participate in the same ordered protocol.

**Separate acquisition identity from publication order.** This report's witness starts after F is already admitted to E. A publication fence closes that gap. It does not automatically solve an observation captured from an old page that first calls the publication API after E+1 begins. Where available, capture session/document or loader generation with the observation and reject or explicitly qualify a stale capture at admission. Do not infer document identity solely from event timestamps or move the epoch boundary blindly to frame commit: current networking deliberately starts the new epoch at the main document request so its request/failure evidence survives. Unidentified late CDP events require an explicit attribution policy. [Loader boundary][navigation] [Document request][document-request]

Lifecycle-marker admission needs the same care. A saturation callback currently saves an epoch number before releasing the budget lock, then calls `publishMarker`, which looks up the current tracking set again. The implementation should keep marker ownership and its encoded epoch consistent across turnover; a common owner protocol is preferable to a separate marker-only patch. This is an adjacent acceptance obligation, not a second measured incident. [Saturation][stream] [Markers][marker]

### Executive and driver responsibilities

Go owns callback admission, serialization, cancellation, exact kernel operations and the cleanup receipt. Mangle owns the consequences of stale, incomplete or cleanup-pending evidence for diagnosis, context selection and further work. Do not let the model decide that a missing cleanup receipt is harmless. Preserve redaction, output bounds, current event schemas or explicitly migrate them, the isolated/shared-session policy, and existing action authorization. A retirement repair does not grant browser effects.

The browser corpus should describe this as an additional concurrency condition on epoch GC, with a checkable gap such as **GAP-BROWSER-EPOCH-PUBLICATION**. That identifier is proposed here, not registered as an existing gap. Closure requires the forced overlap witness and production wiring below, not only a successful serial `RollSessionEpoch` test.

## Why this is the right correctness model

Herlihy and Wing define linearizability by relating concurrent operations to legal sequential behavior, with each operation taking effect within its call interval. Applied here, the useful abstract object is the **epoch owner**, not just the budget map or kernel in isolation. If publication is placed before retirement, retirement must remove its settled effect. If placed after retirement, publication must be rejected for the closed epoch or become explicitly owned under a valid new admission. “Live but owned by neither” is not a permitted sequential result. This is an application of their framework, not a formal proof of the whole browser subsystem. [Herlihy and Wing, 1990][linearizability]

The Go memory model supplies ordering for mutex operations. It does not make work after `Unlock` part of the protected critical section. The current `budgetMu` can safely hand the tracked slice to retirement while the caller has not yet asserted it. The proposed fixture should therefore test lifecycle semantics using barriers; a race-detector pass alone would not establish the required ordering. [Go memory model][go-memory]

## Full cost and failure accounting

The repair is a correctness obligation, not an established optimization. Compare designs at equal event completeness, cleanup guarantees and task quality.

| Cost or failure | Current path / proposed tradeoff | What to measure |
|---|---|---|
| Event work | Sanitization and retained exact keys already cost time and memory proportional to admitted payload. Default per-epoch allowance is 20,000 facts; it is not an all-runtime budget. A new owner adds metadata and synchronization. [Budget][budget] | Facts and bytes produced, admitted, dropped, rejected and retained; allocation/peak bytes, not only row counts. |
| Live-state accumulation | Each successful witness can leave additional unique facts beyond the manager's tracked set. The magnitude depends on actual overlapping batches and other cleanup. | Independent kernel census versus owned live/pending keys across repeated navigation and close. |
| Serialization | A simple publication lock can delay navigation behind a slow sink; a global lock can also delay unrelated sessions. | Per-session and cross-session wait distributions, lock hold time, event lag and navigation/close latency. |
| Kernel cleanup | Exact removal constructs keys and filters the kernel fact slice; when facts change it invokes rebuild machinery. Extra compensating removals or fragmented batches add work. [Removal][kernel-retract] [Filtering][kernel-filter] | Removed/attempted keys, kernel facts examined, rebuild/evaluation work and latency under realistic world/CodeDOM load. |
| Recorder I/O | `publishSanitized` records flight evidence before kernel insertion. Holding an owner boundary across this path includes optional recorder work. [Publish][publish] [Recorder][recorder] | With recording off/on: bytes, append latency, failures, contention and total retirement delay. Preserve privacy and diagnostic retention. |
| Concurrent-owner design | Writer counters, tickets and queues can reduce serialization but add storage, backpressure and settlement states. | Maximum admitted/in-flight batches, queued bytes, drain time and cancellation responsiveness. A queue is not free throughput. |
| Partial assertion / cleanup failure | Some facts may land on an assertion error; a retraction error needs exact retry. Cancellation cannot discard ownership. | Conserved keys and explicit pending states after each failure; retry/rebuild cost and recovery latency. |
| End-to-end task cost | Cleaner current evidence might reduce irrelevant diagnosis or repeated investigation, but that has not been measured. Added synchronization may increase latency. | All model input/output/cached-input usage, tool calls, recovery turns, wall time and independently checked outcomes, including unsuccessful tasks. |

Do not report fewer live facts as a win if the implementation silently drops current events, changes the observation budget or hides pending cleanup. Do not treat a lower manager counter as proof of lower kernel retention. Retained diagnostic traces may legitimately outlive an epoch; current executable facts and historical artifacts need different contracts.

## Proposed verification and fair evaluation

Existing authored tests preserve important behaviors: exact sanitized retraction, lifecycle-marker cleanup, saturation history, failed-cleanup retry and rejection of a sensor call made after a session is already closed. They call the publish and retirement operations serially. The kernel-bridge epoch test records which keys were passed to a fake retractor; that is useful adapter coverage, not proof that a real kernel cannot later receive the supposedly retired fact. No test result was rerun or inferred here. [Retirement tests][tests] [Bridge test][bridge-test]

### Deterministic semantic regression

Use one real kernel and one manager with the production-equivalent write/retract adapters. A narrow test barrier pauses the selected F batch after ownership is recorded but before the real `AssertBatch`. It must leave other batches and retraction free to proceed; a fake sink whose own mutex serializes every operation would hide the defect.

1. Open S and establish E. Start one valid, unique F publication and wait for the barrier.
2. Request epoch turnover concurrently. Record whether it completes or waits; release F under a schedule appropriate to the implementation rather than relying on sleeps.
3. Wait for both operations. If F settled in E, successful retirement must leave no F; if it was refused, verify the explicit refusal and no insertion. It must never become an untracked E+1 fact.
4. Query the real kernel directly for F and its diagnostic consequences. Also inspect owned/pending keys. A zero pending count by itself is not the oracle.
5. Publish distinct valid E+1 evidence G, perform another roll, close S and query again. Check that G survives until its own boundary and that F does not survive merely because its original tracking entry is gone.
6. Demonstrate fixture sensitivity: with the publication fence removed or bypassed in a separately authorized mutation check, the chosen schedule must recreate the residual. No mutation check was performed for this report.

### Production-path and negative controls

| Scenario | Required result |
|---|---|
| Normal boot, real kernel, two local pages and a real browser session; old page emits F while its listener is paused | The independent navigation path reaches the overlap. After settlement, old F is absent or explicitly rejected; current-page evidence remains. |
| New document request already rolled the loader before frame commit | Preserve deduplication; do not invent a second epoch or discard the new request/failure evidence. |
| Frame-first processing while the other listener is paused; qualifying no-network navigation | Exercise the alternate production schedules instead of assuming all CDP callbacks are serialized. |
| Serial publish then roll; current-epoch publish without navigation | Existing correct publication and retirement still work; no blanket event suppression. |
| Two tabs, one slow sink batch | Maintain session isolation and exact removal scope. Report whether the chosen design blocks the unrelated tab. |
| Default `browser_reason` versus raw `browser_mangle` query | Preserve timestamp filtering. Check kernel absence separately, without requiring the default health view to expose a stale event that it correctly filters. |
| Partial assertion rejection, failed retraction and repeated recovery | Every possibly landed key stays owned until settled. Retry uses safe exact keys and does not delete new-epoch evidence. |
| Saturation marker racing turnover; progressive snapshot replacement | Marker epoch and owner agree; snapshot fencing still holds; incompleteness receipts are conserved. |
| Close during publication; closed-session sensor call; canceled caller | No detached writer can publish after a successful terminal cleanup. Existing joins and refusal behavior remain; timeout reports pending ownership. |
| Retraction adapter absent | Do not advertise physical collection. Preserve explicit degraded semantics and avoid retaining unnecessary exact copies solely for an unavailable operation. |
| Redacted URL/input content and enabled flight recording | Exact removal uses stored safe identities. No raw sensitive content enters kernel, traces or failure messages. |

A production fixture should use a local deterministic page server, normal manager/tool registration and the real live policy kernel. It should not rely only on package-private direct calls. Keep model-capable background work disabled or use counted fail-on-call model stubs, so the concurrency result requires no paid inference. Hold the world/CodeDOM configuration constant when comparing designs and include its kernel load. Reuse the same event stream, payloads, navigation sequence, budgets and output limits for the unfenced baseline and candidate repair. Report failures and unknown cleanup states, not only successful iterations.

Behavioral acceptance is zero unowned retired-epoch facts under the forced schedules, preservation of current evidence and bounded, explicit settlement under failure. Performance and task-value claims require their own measurements. Passing that contract closes this ordering gap; it does not certify all browser event attribution, runtime memory ceilings or overall browser parity.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/3441f47316923f134a1b6e06d6385f45d21c059c
[root]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/agents.md
[doc-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/Docs/architecture/AGENTS.md
[browser-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/agents.md
[prior-browser]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/Docs/audits/research-browser-ownership-and-evidence-retirement.md
[prior-world]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/Docs/audits/world-refresh-publication-order.md
[prior-retrieval]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/Docs/audits/retrieval-delta-publication-fencing.md
[browser-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/Docs/architecture/browser/WIRING-AND-NOT-BUILT.md#L66-L79
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/system/factory.go#L2051-L2071
[sink]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/system/factory.go#L2094-L2114
[retractor]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/kernel_bridge.go#L80-L102
[stream]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L254-L304
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_redaction.go#L8-L23
[roll]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L176-L227
[retire]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L136-L159
[pending]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L112-L119
[marker]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L161-L174
[public-publish]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L121-L134
[forget]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L229-L252
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/progressive_observe.go#L179-L226
[close]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/session_lifecycle.go#L541-L567
[listener-construction]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/session_manager_dom.go#L242-L269
[listeners]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/session_manager_dom.go#L607-L619
[console]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/session_manager_dom.go#L270-L291
[navigation]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/session_manager_dom.go#L38-L118
[document-request]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/session_manager_dom.go#L292-L305
[kernel-assert]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/core/kernel_facts.go#L690-L756
[kernel-retract]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/core/kernel_facts.go#L999-L1041
[kernel-filter]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/core/kernel_facts.go#L843-L878
[event-schema]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/core/defaults/schemas_browser.mg#L64-L71
[diagnosis]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/core/defaults/policy/browser.mg#L182-L200
[reason-filter]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/tools/research/browser_reasoning.go#L359-L408
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/tools/research/register.go#L8-L32
[read-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/tools/research/browser_reasoning.go#L191-L244
[query-filter]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/tools/research/browser_reasoning.go#L530-L575
[budget]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_epoch.go#L11-L35
[recorder]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/flight_recorder.go#L439-L466
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/fact_retirement_test.go#L45-L219
[bridge-test]: https://github.com/theRebelliousNerd/codenerd/blob/3441f47316923f134a1b6e06d6385f45d21c059c/internal/browser/kernel_bridge_test.go#L106-L139
[linearizability]: https://www.cs.cmu.edu/~wing/publications/HerlihyWing90.pdf
[go-memory]: https://go.dev/ref/mem#locks
