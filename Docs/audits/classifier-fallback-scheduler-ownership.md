---
doc-class: deep-dive
subsystem: perception-scheduler-composition
implementation-status: proposed
last-verified: 2026-11-02
verified-against: 9bb7666f3843a9971573348674b6cdd69edb349e
verification-method: static-source-inspection
---

# A classifier's fallback must not acquire capacity it already holds

## Decision and evidence boundary

Keep the shared classifier and worker-to-main availability fallback, but give their composed call graph one explicit scheduler-ownership contract. At the reviewed main revision, **a configured CLI main engine plus a successfully constructed API worker classifier** produces an outer scheduled classification call whose fallback secondary is the already-scheduled main client. When the worker returns an eligible error, the secondary tries to acquire another slot from the same process-global scheduler while the outer call retains its first slot.

With a fixed effective capacity of one, that secondary cannot start until the outer reservation is released—but the outer call waits for the secondary. Existing slot-acquisition timeout, parent cancellation, and scheduler-stop handling break the wait and release the outer reservation. This is therefore a **bounded self-induced capacity wait and failed fallback**, not a claim of an infinite deadlock or a leaked slot. With spare capacity, fallback can succeed, but its secondary interval holds two scheduler reservations for one sequential fallback chain.

The review is source-only at [`9bb7666f3843a9971573348674b6cdd69edb349e`](https://github.com/theRebelliousNerd/codenerd/tree/9bb7666f3843a9971573348674b6cdd69edb349e). No tests, application execution, inference, benchmarks, provider calls, or repository edits were performed. The sequences below are proposed witnesses, not reproduced incidents. No latency, prevalence, token-saving, or task-quality claim follows from them.

## 1. Scope and prior-work reconciliation

The stream-completion audit already repaired a separate wiring defect: the direct transducer and registered perception firewall now receive the same resolved classifier rather than rebuilding a worker-only client. Its as-built section says that sharing gives one scheduler charge per call. The shared-client repair is present and should remain. The broader one-charge statement, however, needs a topology qualification: sharing an outer object does not ensure its secondary branch contains no inner scheduler. The existing test uses an API/API configuration and verifies the shared client and fallback wrapper; it does not exercise the CLI-main branch under constrained capacity. [Prior assertion and test scope][prior], [factory classifier test][factory-test].

This report is not another stream-terminal, refusal, credential-health, or classification-outcome audit. Typed stream completion, no-splicing rules, provider refusal semantics, and failed-classification policy handling remain valuable controls. The distinct issue is a scheduler dependency introduced by composing individually valid wrappers. The current factory even avoids double wrapping when there is **only** the already-scheduled main secondary; the two-sided worker/fallback branch does not apply the same ownership distinction. [Factory branches][classifier], [fallback implementation][fallback].

Current guidance requires preserving typed provider failure and cancellation identity, respecting explicitly selected providers/models, and keeping Mangle/session policy authoritative over actions. The proposed repair changes neither fallback authorization nor execution permissions. It should make an already-configured availability path runnable without silently changing the model, bypassing the scheduler, or admitting a failed interpretation. [Perception guidance][perception-guidance], [system guidance][system-guidance], [root guidance][root-guidance].

A bounded search of current audit documents found the one-charge statement and related stream/credential work, not a dedicated nested-classifier-scheduling report. This is not a claim of exhaustive historical-branch coverage. [Audit queue][queue], [credential-pool audit][pool-audit].

## 2. Exact factory composition

### 2.1 Main is already scheduled before classifier resolution

`initPerceptionLayer` constructs the selected main client, optionally adds tracing, then creates `core.NewScheduledLLMCall("main", rawLLMClient)`. It installs the main profile router and stores that scheduled object in `bctx.llmClient`. Later it resolves `bctx.classificationClient` and passes it to the direct `UnderstandingTransducer`. The core boot step has already configured the global scheduler from effective user policy. [Main construction][main-build], [scheduler boot configuration][scheduler-config], [classifier injection][classifier-injection].

For `claude-cli` or `codex-cli`, `ProviderConfigFromUserConfig` preserves the CLI engine selection. The main factory constructs the corresponding CLI client. The classification factory deliberately returns `(nil,nil)` for those subscription-engine configurations, meaning “use the main client”; it does not construct a separate raw classification client. [CLI config][cli-config], [CLI main construction][cli-main], [classification CLI exclusion][class-factory].

### 2.2 A valid API worker makes the nested branch reachable

`classificationClientFor` separately attempts a worker classifier using an API-engine provider config, the worker's model, credentials/pool, endpoint, and classification reasoning settings. If that client is constructed, it becomes `primary`. When the CLI main config yields no separate classification client, `secondary` becomes `bctx.llmClient`, the scheduled main object. The two-sided branch then returns another `NewScheduledLLMCall`, named `classification`, around `FallbackClient(primary, secondary)`. [Worker configuration and branch selection][classifier].

The resulting graph is:

```text
Scheduled("classification", scheduler S)
  └─ Fallback
       ├─ primary: configured API worker classifier
       └─ secondary: Scheduled("main", scheduler S)
                       └─ main routing/tracing/metering/CLI client path
```

Both `NewScheduledLLMCall` constructors call `GetAPIScheduler`; that accessor returns the process-global scheduler instance. Distinct labels `classification` and `main` provide state attribution, not separate capacity pools. The scheduler does not interpret the inner call as borrowing the outer reservation. [Wrapper constructor][wrapper-constructor], [global scheduler][global].

Important negative controls:

- With no constructed worker primary, the fallback-to-main-only branch returns the scheduled main client directly; it explicitly avoids another wrapper.
- With a healthy worker response, the secondary is never called, so this nesting does not create a second acquisition on that call.
- With an API main that can construct its classification client, both fallback sides are unscheduled relative to the cooperative scheduler and the outer wrapper is the one scheduler boundary. An empty `classification_model` normally uses the configured `Model`; it is **not**, by itself, evidence that the scheduled-main fallback branch was selected.
- Provider/model routing constraints can refuse a call before acquisition. The minimal witness uses ordinary classification context with no incompatible override.
- Other construction failures can also select the scheduled-main secondary, but the explicit CLI-main/API-worker configuration is the clearest normal configuration witness; no failed constructor is needed.

[Classifier switch][classifier], [classification model semantics][class-factory], [route resolution][route-resolution].

## 3. The real consumer path is non-stream classification

Shared chat boot obtains its Cortex, transducer, and shard manager through `BootCortexWithConfig`. The factory's registry context carries the same `bctx.classificationClient`; the perception-firewall factory installs it, and `ensureTransducer` uses it directly when present. Chat prefers that running firewall and uses the direct transducer when the system path is unavailable. Thus the shared-client repair does not isolate either entrypoint from the composed graph. [Shared boot][chat-boot], [registry context][registry-context], [firewall registration][registration], [firewall transducer][firewall-client], [chat selection][chat-process].

`LLMTransducer.Understand` chooses a method plan and calls `send`. The generic and free-form plans call `CompleteWithSystem`. The fallback wrapper deliberately does not advertise secondary-only thinking/schema capabilities, and the scheduled wrapper forwards the underlying capability state; the ordinary fallback pair follows the non-stream completion path. A call error terminates the selected classification attempt except the explicit unsupported-schema transition, not an unlimited retry loop in the transducer. [Method plan and send][method-plan], [classification error handling][understand], [fallback capabilities][fallback].

`ScheduledLLMCall.CompleteWithSystem` resolves the target, acquires a slot, registers a deferred release, and calls the wrapped client. `FallbackClient.CompleteWithSystem` calls the primary and, if its error is eligible, calls the secondary with the same context. Nothing between those two stages releases the outer scheduler reservation. The inner scheduled main client then performs its own acquisition. [Scheduled call boundary][scheduled-call], [non-stream failover][fallback-call].

The minimal failure is a worker-side availability error that actually reaches `FallbackClient` while the parent context remains alive—for example, a transient failure after any provider-owned retries. This does not assume every HTTP error is retried identically, nor that a retryable provider status necessarily survives its adapter unchanged. Refused, filtered, and truncated outcomes are excluded by the current failover predicate. [Failover eligibility][fallback].

## 4. Conditional behavior at one slot and above

### 4.1 Effective capacity one: fallback waits on its own ancestor

Assume the production composition above, a fixed effective scheduler maximum of one for the observation interval, a live parent context, an admitted outer classification call, and a worker error eligible for fallback. Also assume no independent capacity reconfiguration raises the limit while the inner call waits.

1. The outer `classification` wrapper acquires the only slot.
2. The API worker returns its eligible failure. Its call has returned locally; the outer wrapper is still executing the fallback composite.
3. The fallback invokes scheduled `main`. Main joins the same scheduler's wait queue.
4. No slot can be granted to main while the outer call holds the only reservation. The outer call cannot reach its deferred release until main returns.
5. The inner acquisition's configured waiting timeout, an earlier parent deadline/cancellation, or scheduler stop ends that wait. Main returns an acquisition error without invoking its underlying CLI completion in this fixed-capacity sequence.
6. The fallback returns the secondary error; the outer deferred release runs. The original worker failure is no longer the returned error, though its failover warning was logged.

[Acquire queue/capacity logic][acquire], [grant condition][grant], [outer deferred release][scheduled-call], [secondary error return][fallback-call].

The normal scheduler constructors replace nonpositive acquisition timeouts with a positive default, and `AcquireAPISlot` derives a timeout when the parent has no shorter deadline. The bound applies to the **inner queued acquisition after the worker has failed**, not the entire classification: outer queue time, spacing, worker request/retries, and prompt work precede it. It is not a hard real-time deadline, nor a statement about remote-provider termination or billing. [Scheduler defaults][scheduler-defaults], [bounded wait][acquire].

Capacity one is supported configuration, not an asserted universal default. Effective capacity accounts for core limits and CLI-specific ceilings; Codex's declared default is two. Adaptive policy can also reduce effective capacity toward its floor, but no claim is made that this particular worker failure necessarily causes that reduction or that capacity stays fixed under all concurrent activity. [Effective capacity and policy][capacity], [Codex default][codex-default], [adaptive shrink][adaptive].

### 4.2 Capacity greater than one: success can conceal duplicate reservation

When another slot is available, the inner main call can acquire it and run normally. During that interval the outer classification reservation and inner main reservation coexist. The primary call has already returned, so this is not evidence of two simultaneous local provider invocations in the sequential fallback chain.

At capacity two, one such fallback can occupy both reservations. At larger capacities it still removes an additional slot from unrelated work. With other active calls, the inner request can queue while the outer continues reserving capacity. Priority/FIFO order determines which waiter receives a free slot; it cannot create capacity or resolve the dependency by itself. These are source-derived reservation pressures, not measured throughput reductions or a universal “half capacity” law. [Priority admission][acquire], [slot accounting and release][release].

Minimum call spacing can add another delay at the inner grant because spacing is applied per scheduler acquisition and deliberately holds the granted slot. That behavior is a legitimate pacing policy; the issue is an unintended second scheduler boundary, not the existence of pacing. Increasing the slot limit may mask the one-slot symptom but changes the user's concurrency policy and does not fix ownership. [Grant spacing][spacing].

## 5. Existing failure, cancellation, and outcome protections

The current code does not justify a broad “fallback is unsafe” conclusion:

- **Outcome-sensitive failover:** refused, filtered, and truncated primary errors do not select the secondary. The streaming surface also refuses fallback after emitted content and when the parent is canceled, preserving no-splicing semantics. The observed classification route is non-stream; do not conflate its capacity issue with stream completion. [Fallback guards][fallback].
- **Bounded queue cleanup:** timeout/cancel handling removes a still-waiting request under the mutex; a racing grant is handled explicitly, and scheduler-stop handling reclaims a granted slot. These existing race/cleanup protections remain relevant and should not be weakened. [Acquisition cleanup][acquire].
- **Deferred release:** non-stream wrappers release after target return or their recovered panic path. In the one-slot witness, inner acquisition fails before its own deferred release is installed, while outer release still executes. There is no evidence here of an extra release or persistent leak. [Scheduled completion][scheduled-call].
- **Typed classification failure:** `FailedOutcome` recognizes wrapped cancellation/deadline errors; the understanding adapter can return a compatibility intent with nil Go error, but it carries failed outcome state. The firewall publishes that outcome, and chat's `classificationStop` gate occurs before any execution lane acts on the compatibility intent. Do not resurrect the old claim that a nil error necessarily means an admitted interpretation. [Failure typing][outcome], [adapter failure return][adapter], [firewall publication][firewall-outcome], [chat stop gate][stop-gate].
- **Separate recovery layers:** `ScheduledLLMCall.CompleteWithRetry` has a release-between-retries test, and local-provider semaphore disabling is a distinct concern. Neither establishes that a `FallbackClient` containing an already-scheduled secondary is flattened. [Existing scheduler tests][scheduler-tests].

Non-stream fallback still passes the same context to its secondary after an eligible error, including cancellation; streaming explicitly checks the parent before retry. This audit does not redesign that policy or promise cancellation prevents every race at every wrapper. Its fixed-capacity witness uses a live parent and an ordinary eligible worker failure, then relies on existing acquisition termination to recover local capacity.

## 6. Proposed ownership contract and repair options

### State the invariant at the composition boundary

A sequential fallback chain must not hold one reservation while awaiting another from the **same capacity domain** merely because a selected client was already decorated. The scheduler should account for the intended resource unit—logical completion or provider attempt—rather than accidental wrapper depth. Sharing the classifier instance between entrypoints is necessary wiring consistency, but is not this invariant.

Two defensible designs have different costs:

| Option | Composition | Tradeoff to specify |
|---|---|---|
| One reservation for the logical classification | Schedule one fallback composite whose children have no second reservation against that same scheduler | Retains one grant across primary and secondary; must preserve pacing/attempt accounting if the second provider attempt also requires them |
| One reservation per sequential attempt | Fallback between independently scheduled primary and main, without an outer capacity holder | Releases primary capacity before secondary acquisition; re-enters queue/pacing fairly, but may add legitimate wait and changes total grant counts |

For the first option, factory wiring should retain an explicitly unscheduled main path with necessary semantics intact. Do not recursively strip every `Unwrap` layer: broker metering, tracing, provider/model routing, credential pools, request shaping, and terminal validation are not redundant scheduler locks. The main router must not silently disappear when raw construction is separated from scheduling. Reuse existing configured model identities rather than inventing a new cheap model to avoid the nested branch.

For the second option, ensure all branches are scheduled exactly where intended, including worker-only, main-only, API/API, and CLI/API configurations. Removing the outer wrapper while leaving the worker raw would bypass concurrency control for the primary. Changing to per-attempt ownership also requires reconciling the prior audit's “one scheduler charge” wording: two sequential grants may be intentional, but two nested reservations are not.

An explicit same-scheduler reservation token could support more general composition, but it must have owner, lifetime, and concurrency semantics. A boolean context marker such as “already scheduled” is insufficient: a child can inherit the context and fan out concurrently, use a different scheduler, or outlive the owner. Likewise, label-based or goroutine-based reentrancy is not a sound substitute for call ownership. The narrow factory composition repair may avoid that additional protocol entirely.

### Preserve failure identity and resource truth

Keep availability fallback separate from refusal/filter/truncation. Preserve deadline ownership across attempts; do not reset the parent budget simply to make fallback succeed. Distinguish a provider attempt failure from local slot-acquisition failure in diagnostics, without presenting the original failed attempt as successful or leaking provider response/credential material.

Report logical classifications, provider attempts, scheduler grants, queue wait, and actual usage separately where a consumer needs them. Current `TotalAPICalls` is incremented on scheduler release; it is not an independent billing receipt. Two reservations do not prove double billing, and deleting one wrapper does not by itself prove fewer provider attempts. Existing per-provider/broker accounting must remain intact. [Release accounting][release], [metered client construction][metering].

### Why the timeout is not the repair

Go's context API provides cooperative cancellation/deadline signals, not a successful result or a guarantee that arbitrary work has stopped. The standard semaphore package similarly documents acquisition waiting for capacity or cancellation. CodeNERD uses its own scheduler, so those are contract references, not its implementation. [Go context](https://pkg.go.dev/context#WithTimeout), [Go weighted semaphore](https://pkg.go.dev/golang.org/x/sync/semaphore#Weighted.Acquire).

Python's executor documentation illustrates the general bounded-resource hazard: a worker that waits for work requiring the same exhausted pool prevents that work from starting. This is an analogy only. CodeNERD's timeout breaks its wait; the source-derived failure is that an available configured fallback is never reached before that timeout under the one-slot conditions. [Primary executor documentation](https://docs.python.org/3/library/concurrent.futures.html#threadpoolexecutor).

## 7. Costs and fair evaluation — proposed, unrun

### Full cost model

Current cost includes prompt assembly, outer acquisition/spacing, the worker's provider attempts and backoff, inner acquisition/spacing when selected, CLI invocation/completion when admitted, parsing, outcome publication, and any later user or policy retry. During an inner wait the outer reservation can also delay unrelated work. A scheduler “LLM call duration” spanning the composite includes local fallback queue time, not just provider service time.

Flattening ownership can remove redundant reservation pressure, but it does not erase worker failure cost, CLI startup latency, provider retries, response tokens, or prompt construction. Per-attempt scheduling may improve fairness while adding a queue turn and pacing delay; logical-call scheduling retains capacity across retry/backoff and can be less fair under contention. Broader reservation-token designs add lifecycle complexity and testing burden. Changing the concurrency maximum trades load against latency and is not an equivalent repair.

No extra inference is inherently required to fix composition. Nor is a token/cost saving established: under one slot, a successful repair may newly permit a legitimate fallback provider call that baseline never made, increasing immediate spend while restoring availability. Evaluate successful task completion and total recovery cost rather than treating fewer failed-run tokens as a win.

### Mechanism tests through the actual topology

1. **Construct the normal config route.** Use a configured CLI main and API worker, with local deterministic provider/CLI fixtures and production factory resolution. Confirm the direct transducer and registry firewall share the resolved client, then invoke the same `CompleteWithSystem` classification path. A manually assembled nested wrapper proves the mechanism but not factory reachability.
2. **One-slot witness.** Keep effective capacity fixed at one; choose a short configured acquire timeout for the fixture and a longer live parent. The worker returns a known eligible error only after outer admission. Count whether the underlying CLI is invoked; inspect waiting/active reservations before termination and require both return to zero afterward. Use synchronization barriers, not sleeps as the sole witness. Baseline expectation is an inner acquisition failure; repaired expectation is a reachable, correctly scheduled secondary.
3. **Capacity-two pressure.** Hold the secondary fixture at a barrier after it starts. Observe simultaneous reservation count and whether an unrelated scheduled call can start under the selected ownership contract. Release all work and join it; report wait fairness rather than assuming every schedule has the same order.
4. **Deadline and stop controls.** Parent cancellation before primary, during primary, while inner acquisition waits, and during pacing; scheduler stop; primary failure near parent deadline; timeout/grant races. Retain existing queue cleanup and stream release-before-terminal tests. The acquisition timeout begins at the queued stage, so do not assert it bounds total classification elapsed time.
5. **Configuration controls.** Worker success makes no secondary call; CLI main without a worker has one boundary; valid API main/API worker remains operational; missing optional classification tier takes the intended main path; explicit provider/model overrides remain either correctly routed or visibly refused. Test any adaptive-capacity case separately from fixed-capacity evidence.
6. **Semantic controls.** Refused/filtered/truncated errors never trigger fallback. Streams with content never splice a secondary; pre-output failure retains existing allowed recovery. Invalid classification responses and typed canceled/provider-failed outcomes are not admitted as normal intent. Verify the actual firewall and direct consumer receipts, not only pointer shape.

The existing factory test establishes shared classifier identity with an API/API pair; fallback tests establish primary/secondary behavior with unscheduled fakes; scheduler tests establish acquisition, cleanup, pacing, and retry properties separately. Those are useful controls but do not compose into a proof for the CLI-main nested path. This review inspected test source and ran none. [Factory test][factory-test], [fallback tests][fallback-tests], [scheduler tests][scheduler-tests].

### Compare equivalent work

Use the same provider/CLI fixture responses, fault schedule, effective capacity, spacing, adaptive settings, deadlines, prompt, model/credential configuration, and retry budget before and after. Record success/failure, logical calls, attempted provider requests, CLI starts, maximum reservations, queue wait distributions, reservation-seconds, allocations, goroutines, and cancellation cleanup. Do not compare a timed-out baseline with a successful fallback solely on elapsed time or token count without showing the different achieved outcome.

A later end-to-end study should use ordinary boot and turn entry with world/CodeDOM context and existing permissions enabled, matched tasks/models/budgets, and measured terminal task quality. Account for resumed/retried turns and all provider usage. No live-model study is necessary to establish the scheduling defect itself, and none was performed here.

## Acceptance

- A configured eligible fallback can reach its secondary under a one-slot policy without waiting for a reservation held by its own outer call.
- One sequential fallback chain holds no redundant same-domain reservation; grant/attempt semantics are documented for the chosen design.
- Shared direct/firewall classifier identity, configured providers/models, routing, metering, refusal/stream rules, and typed failure admission remain intact.
- Cancellation, timeout, stop, and races leave no pending waiter or held slot after owned work ends.
- Capacity and cost claims are based on matched outcome-aware measurements, not wrapper counts alone.

**Bottom line:** the shared-classifier repair is real, but the one-charge assertion does not cover a scheduled main client nested beneath a scheduled fallback. Make ownership explicit at composition; preserve the scheduler rather than increasing capacity or bypassing it to hide the wait.

[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/agents.md
[system-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/agents.md
[perception-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/agents.md
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/Docs/audits/stream-completion-contract.md#L203-L231
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/Docs/audits/ASTRA_QUEUE.md
[pool-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/Docs/audits/credential-pool-failure-scope-and-recovery.md
[scheduler-config]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/factory.go#L1072-L1083
[main-build]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/factory.go#L1086-L1157
[classifier-injection]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/factory.go#L1229-L1241
[classifier]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/factory.go#L1259-L1343
[cli-config]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_factory.go#L135-L156
[cli-main]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_factory.go#L505-L521
[class-factory]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_factory.go#L320-L348
[metering]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_factory.go#L830-L871
[wrapper-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/scheduled_llm_client.go#L783-L808
[global]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L881-L891
[route-resolution]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/scheduled_llm_client.go#L41-L91
[scheduled-call]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/scheduled_llm_client.go#L185-L237
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_fallback.go#L17-L202
[fallback-call]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_fallback.go#L101-L140
[chat-boot]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/cmd/nerd/chat/session_shared_boot.go#L89-L116
[registry-context]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/factory.go#L2114-L2138
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/shards/registration.go#L862-L879
[firewall-client]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/shards/system/perception.go#L259-L280
[chat-process]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/cmd/nerd/chat/process.go#L235-L256
[method-plan]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/transducer_llm.go#L253-L283
[understand]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/transducer_llm.go#L137-L164
[acquire]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L292-L424
[grant]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L426-L437
[scheduler-defaults]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L216-L239
[capacity]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/config/user_config.go#L1303-L1385
[codex-default]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/config/llm.go
[adaptive]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L772-L814
[spacing]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L451-L496
[release]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler.go#L553-L585
[outcome]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/classification_outcome.go#L135-L157
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/understanding_adapter.go#L422-L435
[firewall-outcome]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/shards/system/perception.go#L537-L587
[stop-gate]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/cmd/nerd/chat/process.go#L397-L402
[factory-test]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/system/factory_classification_test.go
[fallback-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/perception/client_fallback_test.go
[scheduler-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9bb7666f3843a9971573348674b6cdd69edb349e/internal/core/api_scheduler_test.go#L334-L419
