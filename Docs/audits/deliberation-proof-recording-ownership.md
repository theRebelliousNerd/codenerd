---
doc-class: deep-dive
subsystem: deliberation-provenance
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 9da7715b1d7313b885995ee41949fe97902eb8fc
supersedes: []
---

# Keep required proof recording alive for every active deliberation

## Finding and evidence boundary

A deliberation borrows a shared recorder without recording that borrowing. `Service.Run` enables provenance only if the kernel reports it disabled, and only that caller defers disabling it. A second overlapping run sees recording enabled and acquires no ownership. When the first caller exits, its deferred disable clears the recorder still required by the second caller. The second run can then fail its mandatory proof step even with valid candidates, a complete challenge and a derivable policy decision. [Conductor lifetime][lifetime] [Recorder operations][recorder] [Proof requirement][prove]

**Confirmed static mechanism:** one active conductor can retire another conductor's required shared resource. **Conditional consequence:** a valid read-only ideation or mission-design attempt fails when the stated overlap occurs. This is a lifetime/availability defect, not a demonstrated wrong policy conclusion, false acceptance, permission bypass, corrupt source file or observed model failure. The proof check fails closed and must remain.

The overlap is not limited to a hypothetical caller: the chat recurse mission uses `m.deliberator()` on `m.kernel` in a background command, while an ordinary foreground `/deliberate` uses the same constructor and kernel. The existing foreground slot prevents two foreground tasks running together; it does not own the background mission's proof recorder. [Background constructor][background] [Foreground constructor][chat] [Foreground slot][slot] [Dispatch][dispatch]

Public main resolved to **9da7715b1d7313b885995ee41949fe97902eb8fc**, committed November 3, 2026 at 11:59:25 UTC, and was rechecked unchanged at approximately 12:18 UTC. All repository links use that immutable pin. Public guidance, architecture, code, nearby audits and authored tests were inspected. No repository program, build, test, model inference, benchmark, gate or failure injection ran. No implementation, credential, configuration or setting was changed. All schedules and evaluations below are unrun.

## 1. Why this belongs in the Socratic path

The existing design already assigns creative alternatives to the proposer, concrete objections to the requirements interrogator and executive disposition to Mangle. The session specification explicitly requires recorded proofs and distinguishes a ready design from verified implementation. Its status is source-authored, not a claim that the listed runtime checks passed. Root guidance asks for this logic/model separation to hold in normal production paths, and scoped session guidance requires explicit ownership of shared kernel use and teardown. [Root guidance][guidance] [Session specification][spec] [Shared-state guidance][session-guidance]

This report does not propose a new judge, a stronger model, another voting round or relaxed proof requirements. It addresses whether the existing requirements exchange can finish while an unrelated exchange ends. Additional Socratic refinement cannot repair a recorder that the host has turned off underneath it.

Preserve the current safeguards:

- Candidate and assessment validation, bounded read-only investigation and distinct data/design/values blockers.
- Thread/round-scoped policy facts and exact cleanup.
- Current-generation proof envelopes with bounded extraction.
- Honest failed/stopped outcomes and durable original intent.
- Separate mission source/vision admission, write permissions, test evidence and promotion checks.
- Foreground cancellation ownership and the ability to do useful read-only work while a mission runs.

A correctly retained recorder only supports a logical derivation from admitted premises. It does not prove model-authored premises true or make an unrun success test pass. [Round collection][round] [Projection and proof][prove] [Service][service] [Mission preparation][mission]

## 2. Exact ownership mismatch

### 2.1 The service treats enabled state as sufficient ownership

The service's lifecycle code first calls `IsProvenanceEnabled()`. If false, it calls `EnableProvenance()` and registers `defer DisableProvenance()`. If true, it does neither. There is no run identity, owner count, acquisition token or shared lifetime guard in this sequence. [Lifetime][lifetime]

That strategy preserves recording which was already enabled before an isolated run. It is not safe for a recorder enabled by a different temporary caller. “Enabled now” does not tell the second caller who may turn it off before its next use.

The calls are individually synchronized in `RealKernel`: enable and disable take its mutex, and the state query takes its read lock. Enable is idempotent; disable sets `proofRecorder = nil`. This finding does not require an unsynchronized memory access or a race-detector finding. All calls can occur in an ordinary, fully ordered interleaving. [Recorder implementation][recorder]

There is also a check-then-enable window in which two runs could both observe disabled and both install deferred disables. The simpler witness below does not need that window: B starts after A has definitely enabled the recorder.

### 2.2 Cortex forwarding does not create shared lifetime ownership

The production Cortex wrapper enables or disables every shard and reports enabled only when every shard is recording. These methods do not acquire a conductor-specific reference. Its explanation method forwards to the relevant shard and propagates an error from that shard. Therefore a global disable is visible through the production kernel abstraction rather than only a bare `RealKernel` test. [Cortex recorder forwarding][cortex] [Cortex proof forwarding][cortex-proof]

Per-thread fact identity is useful but independent. Both runs may assert and retract only their own deliberation facts while still sharing the same non-fact recorder state. Giving each thread a unique ID does not own that recorder.

### 2.3 The mandatory consumer correctly refuses missing recording

After collecting and validating a round, the service asserts it, queries `deliberation_decision`, selects the single decision for that thread/round, and calls `prove` before appending the round to durable history. `prove` calls `ExplainEnvelope` and requires a recorded epoch plus at least one proof. [Decision sequence][decision] [Proof guard][prove]

`RealKernel.ExplainEnvelope` evaluates as needed, locks the kernel and immediately returns `ErrProvenanceDisabled` if the recorder is nil. Evaluating a dirty decision with recording disabled does not restore the missing recording. The Cortex method propagates that error. The service's deferred error handler then marks its thread failed and attempts to save it. [Explanation guard][explain] [Failure handling][failure]

The consequence is potentially wasted proposer/challenger work and an unavailable design, not silent proof omission. A repair must not convert this error into successful readiness.

## 3. Production overlap, with limits

The source establishes the following route:

1. `/campaign recurse` schedules `startRecurseCampaign` rather than wrapping the mission lifetime in `foregroundOperation`. The start message stores the running recurse state, clears `isLoading` and launches the run command with its progress listener. [Start dispatch][start-dispatch] [Background launch][launch]
2. With a mission, `runRecurseLoop` constructs `RecurseMission{Deliberator: m.deliberator()}`. The deliberator stores `Kernel: m.kernel`. Mission preparation calls its `Run` method. [Background][background] [Shared kernel][chat] [Mission call][mission]
3. The ordinary Enter handler admits a new foreground command when loading is false and the foreground turn slot is empty. `/deliberate`, `/innovate` and `/clarify` dispatch to `handleCmdDeliberate`. That path creates a design-only thread, starts a foreground operation, and runs another deliberator over the same kernel. [Input gate][input] [Command dispatch][dispatch] [Design command][design]
4. The workspace write hold applies to named mutation commands, not this design-only command. Foreground ownership is still a real safeguard: a second foreground deliberation is queued rather than arbitrarily overlapped. The relevant overlap is **one background mission plus one foreground design**, not two simultaneous foreground submissions. [Write hold and dispatch][dispatch] [Slot][slot]

This route does not guarantee timing: the mission must still be deliberating when the second request starts. It also presupposes an initialized, otherwise functional runtime and valid mission admission. Those conditions are inputs to an unrun evaluation, not facts about a production incident.

The temporary ownership problem also exists for any two supported `Service.Run` callers sharing a kernel. A second OS process with its own kernel is not the same witness. Language-specific build adapters, values-answer parsing and source drift are unnecessary to it.

## 4. Minimal unrun schedule

Use two service instances A and B with one common proof-capable production kernel, distinct thread IDs, valid session/workspace persistence and the same valid routing parameters. Start with provenance disabled. Use design-only threads, adequate bounds and deterministic role callbacks returning valid divergent candidates and a challenge with exactly one ready candidate. No external tool execution, workspace mutation, human answer or malformed response is needed for the core schedule.

Control scheduling with barriers in the callback seam, not guessed sleeps:

| Step | Run A | Run B | Shared recorder |
|---|---|---|---|
| 1 | Enters `Run`, sees disabled, enables and registers deferred disable. Pause during collection. | Not entered. | On. |
| 2 | Still collecting. | Enters, sees enabled, registers no recorder ownership. Pause during collection. | On. |
| 3 | Resume A; let it derive, prove, finish and fully return. | Remains paused. | A's deferred disable makes it off. |
| 4 | Complete. | Resume B with valid proposal/challenge; derive its decision and request the required proof. | Off. |
| 5 | Complete. | `ExplainEnvelope` returns the disabled error; `Run` fails and saves failed status, assuming storage succeeds. | Off. |

The expected failure follows directly from the ordered operations, conditional on the stated infrastructure preconditions. It has not been executed. B can be paused before it receives a challenger result so that A's fact cleanup finishes before B's decision; no simultaneous mutation or epoch ambiguity is needed. A's successful outcome does not authorize any write in this design-only fixture.

Useful variants, also unrun:

- A is canceled or its callback fails after B enters. A's deferred disable still runs, so B's independent lifetime remains vulnerable.
- Both start while disabled and both pass the state check before either enables. Whichever finishes first can retire the recorder.
- B finishes before A, with recording initially off. This ordering should not exhibit the specific loss.
- Recording is enabled before either starts. Neither run installs this deferred disable; the baseline protects this particular schedule.
- Use separate kernels. One run's completion must not affect the other.
- Have B reach a values disposition or an ordinary empirical/design round. The mandatory proof dependency is not unique to final-ready decisions.

Keep failure of JIT compilation, schema analysis, storage, proof truncation, policy selection and role validation distinguishable from this expected lifetime failure.

## 5. Relation to existing audits

The full audit-path inventory and current queue were inspected, along with all deliberation audits and relevant ownership reports. The following prior work remains credited rather than repackaged:

| Existing work | Its boundary | This report's distinct boundary |
|---|---|---|
| Portable recurse deliberation crucible | Requirement contracts, capability limits, criticism-to-refinement and independent acceptance. | Availability of the existing mandatory proof service while independent exchanges overlap. |
| Critical-item retirement | Explicit accounting for earlier design objections. | No omitted or changed objection; valid current-round data can still fail. |
| Progress-observation identity and read-invocation identity | Correctly attributing evidence and deciding whether progress occurred. | No colliding observation or false stall is necessary. |
| Terminal-budget continuation | The appended note is excluded by a copied slice header. | Input fits; the missing resource is shared recording, not context room. |
| Values freshness/option admission | Validity and membership on saved answer replay. | No saved answer or candidate-label parsing is involved. |
| Campaign/recurse scheduling ownership | Shared scheduler facts during overlapping campaigns. | One mission and read-only design can have correct fact identity and still lose recording. |
| Expert-selection temporary-fact ownership | An inserter can retract shared evidence a borrower still needs. | Same general ownership principle, but a different non-fact recorder API, producer/consumer chain and proof failure. |
| Why-query scope preservation | Preserve a selective query through tracing. | No query widening; the required recorder is retired. |

[Crucible][crucible] [Critical retirement][retirement] [Progress][progress] [Read identity][read-audit] [Budget continuation][budget-audit] [Values option admission][values-audit] [Campaign scheduling][scheduling-audit] [Expert ownership][expert-audit] [Why scope][why-audit] [Queue][queue]

A scheduling policy could choose to serialize all uses of this resource, but repairing only campaign task facts or only foreground slots does not establish recorder ownership for every supported caller. A broader serialization decision must be explicit about read-only responsiveness and all entry points.

## 6. Proposed ownership contract

Everything in this section is a prospective design, not an existing API or completed fix.

### Invariant and operation boundary

A mandatory proof consumer must hold a live kernel-owned recording claim from before its relevant evaluation until its final required proof has been copied or the consumer has terminated. Releasing one consumer's claim cannot disable recording required by another. An acquisition must atomically establish ownership rather than separately inspect and then toggle a boolean.

A small API could return an opaque release capability from acquisition. The implementation may use reference counting or a bounded owner set, but it must specify:

- **Identity:** a claim belongs to one kernel instance/generation and one invocation, not merely a thread name that may be reused.
- **Exactly-once release:** success, cancellation, malformed output, assertion failure, proof failure and persistence failure release only that invocation's claim. Repeated release is explicitly harmless or diagnosed, never an underflow that disables others.
- **Existing preference:** distinguish operator/configured recording from temporary mandatory consumers. Completing the last temporary run must not turn off recording that an operator requested before or during it.
- **Disable semantics:** define whether an explicit request to disable while mandatory consumers exist is deferred with truthful status, rejected, or first terminates those consumers. Do not pretend the recorder is off while it remains required, and do not silently strand them.
- **Cortex consistency:** acquisition and release must compose with shard membership/kernel adoption. Partial acquisition rolls back only acquired claims; replacing a kernel either preserves the effective demand or explicitly invalidates/rebinds affected consumers.
- **Lifetime:** claims are in-process resources, not restored by replaying saved historical proofs or pending thread files after a restart.
- **Failure:** inability to establish ownership fails before paid role work when feasible. Missing proof still fails closed.

Do not hold a kernel mutex across model/network calls. Protect the short ownership operations and the necessary evaluation/proof consistency boundary instead. A service-local mutex alone is insufficient because chat creates separate services over the same kernel.

### Bounded alternatives

1. **Shared ownership:** retain concurrency and the current proof API, add explicit lifetime accounting. Cost: state, acquisition/release synchronization, wrapper propagation and lifecycle validation.
2. **Serialization across the shared kernel's mandatory consumers:** simpler to reason about, but queues or rejects read-only ideation while a potentially long mission deliberates. It must cover every consumer, preserve cancellation and bound queued work.
3. **Private proof-capable scope:** avoids sharing this resource, but must supply the correct current program, relevant facts, external-read behavior and policy semantics. Copy, evaluation and staleness costs are material. A clone is not automatically equivalent to the live production kernel.

The preferred first experiment is ownership of the existing resource, with serialization as a measured comparator. This is not a reason to leave global recording permanently enabled or to bypass the proof guard.

## 7. Cost and failure accounting

A recording claim is cheap relative to a model request, but the recorder's effective lifetime is not free. With recording on, the current cone eligibility check returns the provenance reason; full evaluation installs a fresh memory recorder. Consequently retaining recording correctly across B's full lifetime can increase host work relative to a broken baseline that turns it off early. That additional cost may be necessary to produce B's valid proof. Do not report the broken baseline's early failure as an efficiency win. [Cone eligibility][cone] [Evaluation recording][eval]

Measure the whole request group:

- All proposer/challenger and consultation calls, their input/output tokens, failed requests and retries.
- Kernel evaluations by mode, fact/rule counts, time, allocations and peak recorder memory.
- Proof extraction/serialization and thread persistence bytes/time.
- Acquisition/release contention, live owner count, queued time under serialization, cancellation latency and time until the resource becomes idle.
- Foreground time to a usable design and mission time to the same independently accepted endpoint.
- Failed or abandoned designs and repeated reads/model work needed after the lost proof.

A lease should not add model calls. It can nevertheless change throughput by holding full recording longer. Claim-table leaks, missed release, duplicate release, counter underflow, replacement of a kernel while claims remain, manual-toggle interactions and partial Cortex acquisition are explicit failure cases. A process-local owner map cannot be left to grow indefinitely; terminated owners must be released by their actual lifecycle, not guessed idle timeouts during slow inference.

If all requests already run with configured recording on, this defect's exposure may be zero for that deployment. The proposed ownership machinery still has overhead there. Report that negative control rather than assuming a universal benefit. No latency, cost, throughput or success-rate gain has been measured.

## 8. Fair evaluation plan, unrun

### Deterministic correctness layer

Use the actual service and real kernel first, then the production Cortex wrapper. Keep role behavior scripted so the schedule tests host ownership rather than stochastic model quality. Exercise:

- The barrier-controlled A-then-B schedule, and the reversed finish order.
- Two simultaneous acquisitions from disabled state.
- Three overlapping consumers, with each possible last finisher.
- Initially enabled recording, including a separately requested enable during a temporary use.
- Cancellation, error and panic-safe outer cleanup paths, failed acquisition, duplicate release and persistence failure.
- Cortex partial initial state and kernel-adoption cases under the explicitly chosen contract.
- Independent kernels and sequential callers as controls.
- No stale proof reused after a changed program/evaluation; bounds and missing-proof refusals preserved.

For each case assert owner membership, effective recording state, each caller's independent outcome and final idle state. Require positive proof evidence for successful runs, not merely absence of an error. A clean race-detector run alone is insufficient: the ordered schedule can be data-race-free.

The source already contains disabled-by-default/idempotent-toggle tests and a Cortex proof-location test. Those are valuable controls; the inspected tests do not exercise overlapping `Service.Run` lifetimes. This is a claim about the read test files, not a repository-wide proof that no other test exists. None was run here. [RealKernel tests][real-tests] [Cortex tests][cortex-tests] [Service-related tests][deliberation-tests]

### Production-entry and comparative layer

Use the ordinary chat route with one admitted background recurse mission and one design-only `/deliberate`, preserving normal CodeDOM/world-model context and permission gates. Use a disposable repository and an explicitly authorized setup if this evaluation is later run. Force overlap through controlled role barriers before any mission edit so failures cannot be attributed to a changing workspace. After the host schedule is qualified, separately study realistic model timing.

Compare unchanged baseline, shared ownership and complete serialization with identical tasks, provider/model versions, prompt corpus, source state and total budgets. Report outcomes for initially-on and initially-off recording separately. Count every failed attempt and its cost. Benchmark small and large fact stores, short and long role waits, and concurrent unrelated queries. If broader portability claims are desired, repeat on supported OSes and several language ecosystems, but do not infer language-adapter correctness from this language-neutral proof-lifetime result.

Acceptance is narrow: a consumer cannot lose required recording merely because an independent consumer finishes; all successful decisions still carry current bounded proof; existing configured state and exact cleanup survive. It is not proof that the design itself is correct or that the mission improves arbitrary repositories.

## 9. Primary grounding

Herlihy and Wing's original linearizability paper motivates specifying acquisition/release as operations with a clear concurrent-object contract, rather than assuming individually synchronized getters/setters establish a higher-level lifetime. It does not certify this implementation or prescribe a particular lock. [Herlihy and Wing, 1990](https://www.cs.cmu.edu/~wing/publications/HerlihyWing90.pdf)

The Go memory model distinguishes data-race freedom from the application's required behavior. The witness here can be explained as an ordered interleaving of locked calls, so it does not rely on undefined memory behavior. [Go memory model](https://go.dev/ref/mem)

Linux's official reference-counting guidance illustrates acquiring ownership before handing a resource to another execution path and releasing only after the last holder is done. That is an ownership analogy, not a proposal to import Linux kref into Go; a codeNERD recorder also needs operator-state and Cortex-generation semantics. [Linux kref documentation](https://docs.kernel.org/core-api/kref.html)

## Decision

Strengthen the existing proof-resource lifetime at the kernel boundary. Preserve the Socratic roles, policy decisions, read-only scope and fail-closed proof requirement. Validate the overlap with controlled schedules before making performance or reliability claims, and compare the complete cost of a valid design rather than the apparent cheapness of an interrupted one.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/9da7715b1d7313b885995ee41949fe97902eb8fc
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/agents.md#L3-L67
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/session/agents.md
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L900
[lifetime]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/deliberation/service.go#L53-L73
[service]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/deliberation/service.go
[failure]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/deliberation/service.go#L42-L58
[decision]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/deliberation/service.go#L120-L147
[round]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/deliberation/round.go#L10-L72
[prove]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/deliberation/policy.go#L10-L72
[recorder]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/kernel_provenance.go#L26-L56
[cortex]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/cortex_kernel_inspect.go#L64-L93
[explain]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/kernel_explanation.go#L79-L124
[cortex-proof]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/kernel_explanation.go#L278-L307
[background]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/campaign_recurse.go#L193-L222
[start-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/commands_handlers.go#L735-L749
[launch]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/campaign_recurse.go#L368-L379
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/deliberation.go#L18-L51
[design]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/deliberation.go#L95-L120
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/campaign/recurse_mission.go#L184-L310
[slot]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/foreground_operation.go#L14-L48
[input]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/model_key_handler.go#L253-L265
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/commands.go#L109-L145
[cone]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/kernel_eval_cone.go#L310-L336
[eval]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/kernel_eval.go#L372-L398
[real-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/kernel_provenance_test.go#L12-L57
[cortex-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/internal/core/cortex_kernel_inspect_test.go#L89-L117
[deliberation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/cmd/nerd/chat/deliberation_test.go
[crucible]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/portable-recurse-deliberation-crucible.md
[retirement]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/deliberation-critical-item-retirement.md
[progress]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/deliberation-progress-observation-identity.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/deliberation-read-invocation-identity.md
[budget-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/deliberation-terminal-budget-continuation.md
[values-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/deliberation-values-option-admission.md
[scheduling-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/campaign-recurse-scheduling-ownership.md
[expert-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/expert-selection-temporary-fact-ownership.md
[why-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/why-query-scope-preservation.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/9da7715b1d7313b885995ee41949fe97902eb8fc/Docs/audits/ASTRA_QUEUE.md

