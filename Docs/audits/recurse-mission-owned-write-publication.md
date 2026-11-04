---
doc-class: deep-dive
subsystem: campaign-recurse-mission-owned-write-publication
implementation-status: partial
evidence-status: source-only
last-verified: 2026-11-04
main-rechecked: 2026-11-04T21:16:34Z
verified-against: 8a8aebcdc54eb4901d81e0ce25b01641c072842b
verification-method: static-source-inspection
supersedes: []
---

# Publish mission owned writes before releasing their execution owner

## Finding and decision

**A recurse mission can leave source edits and a durable executing intent after a hard process interruption, but never publish the content-bearing receipt intended to support owned-effect reconciliation.** The normal CLI and chat callbacks return the orchestrator's in-memory write ledger only after deferred campaign cleanup has run. The outer driver persists that ledger afterward. A process stop between those operations therefore leaves the mission's `OwnedWritesRef` empty, no `writes-<cycle>.json` sidecar, and the disposable campaign already removed. [CLI callback][cli] [Chat callback][chat] [Outer publication][handoff] [Sidecar writer][sidecar]

The restart guards correctly refuse to continue. This finding is **missing recovery material at an effect handoff**, not automatic replay, permission bypass, false acceptance, or a claim that the source files themselves are lost. Git, transcripts or external backups may assist an operator; their existence does not make the missing content-bearing mission receipt available through its declared contract. [Pending census][census] [Independent recovery guard][settle] [Write representation][write-type]

Require a durable, attempt-bound ownership handoff before retiring the executor's recovery responsibility. Separately close or explicitly qualify the earlier interval while execution is still changing files. Moving cleanup later is useful, but cannot make an in-memory write ledger survive a crash during execution. Preserve the current fail-closed recovery, conditional compensation, mission permissions, oracle protection and Mangle ratchet. Do not replace them with an automatic reset to `HEAD`.

**Evidence boundary:** public main resolved to [8a8aebcdc54eb4901d81e0ce25b01641c072842b][pin]. Root and scoped guidance, campaign architecture, the current audit inventory, neighboring reports, implementation and relevant authored tests were inspected. Retained source bytes were verified against the pinned tree's Git blob identities. No repository application, test, build, gate, fixture, benchmark, model inference or training was run. This is a source-derived conditional trace, not an observed incident. The proposed changes and evaluations are unimplemented and unrun. Only this documentation draft was prepared.

## Architectural purpose and current status

The root architecture puts creative design with the model and executive control with the deterministic kernel. The mission specification saves intent before execution, retains returned owned writes independently of disposable campaigns, and requires reconciliation before more effects after interruption. It also deliberately makes no blanket atomicity promise against arbitrary external editors. The proposed correction serves that existing evidence and ownership design; it does not create another autonomous loop. [Root guidance][root] [Campaign guidance][campaign-guidance] [Mission storage contract][mission-contract]

The relevant distinctions are:

- A **pending intent** establishes which mission and attempt were admitted. It does not enumerate which writes actually happened.
- An **owned-write record** carries a path and known before/after file contents. Current compensation uses those contents to distinguish its own remaining postimage from a later writer's state.
- A **ratchet outcome** records whether a candidate was kept, reverted, refused or unverified. It is not yet present at the chosen interruption.
- A **cleanup completion** says disposable execution artifacts were released. It does not establish that another owner durably accepted their recovery obligation. [Mission receipt][receipt] [Compensation][undo] [Ratchet publication][finish-order] [Cleanup][release]

| Boundary | Status at the pin | What it establishes |
|---|---|---|
| Mission intent and executing disposition | Implemented | Pending mission and verification identity are saved before the callback runs. [Intent][intent] [Execution admission][execute-admission] |
| Per-turn and per-campaign owned writes | Implemented in memory | Session returns and successful task records carry content-bearing write observations to the outer executor. [Turn projection][turn-writes] [Campaign collection][kept-writes] |
| Independent mission write sidecar | Implemented after callback completion | `recordWrites` publishes the payload, then its reference in the mission state. [Sidecar][sidecar] |
| Disposal of the attempt campaign | Implemented inside the callback | Deferred cleanup precedes the caller's sidecar publication. [CLI][cli] [Chat][chat] |
| Graceful cancellation and pre-adoption error compensation | Implemented once returned ownership is available | The driver can undo its returned writes conditionally. [Compensation boundary][handoff] [Cancellation][cancel] |
| Refusal of unresolved mission effects on restart | Implemented | Both fresh mission admission and ordinary recurse recovery preserve the unresolved boundary. [Census][census] [Recovery][settle] |
| Durable owned-effect handoff across the selected process interruption | Missing on the inspected path | No mission-owned write sidecar has been created at that cut. |
| Recovery-complete recording during each file effect | Not established by the present return-based ledger | A post-execution publisher cannot cover effects before it is reached. |
| Runtime frequency and recovery effort | Unverified | No execution or incident study was performed. |

The opportunity is to preserve the evidence already gathered by the host. An additional proposer, critic or model-generated reconstruction would not turn guessed authorship into a measured write receipt.

## The content-bearing record is process local until the outer handoff

### Session observations preserve the right information in memory

`withTurnWrites` builds a `FileWrite` for every recorded written path. It reads the final file state and obtains the earlier state from `ExecutionResult.PreWriteContents`. The return therefore has the material used for conditional restoration, including whether the earlier file existed and whether the read was known. [Turn projection][turn-writes]

`observation.Return.Writes` is explicitly marked `json:"-"`. Its comment says the content is kept for the process that ran the turn and is never serialized through the ordinary return representation. `FileWrite` itself contains `Path`, `Before` and `After`; each `FileState` includes `Known`, `Exists` and `Content`. That serialization boundary is important counterevidence to the assumption that a retained structured worker return automatically backs up the full ledger. [Return boundary and types][write-type]

The separate `EffectReceipt` can be serialized, but it intentionally retains content identities rather than content. Its resource effect records a path, before/after identity and state. Those are useful provenance and validation observations, not interchangeable bytes for an operation that restores a recorded preimage. This report does not recommend removing that privacy boundary or copying every file into general-purpose transcripts. [Effect receipt contract][effect-type]

### Campaign success retains the ledger without making it durable

The orchestrator stores active attempts and kept writes in private in-memory fields. A successful mutating task ends its attempt record, appends its writes to `keptWrites`, releases its leases and returns. `KeptWrites` returns a copy of that slice. These are appropriate live ownership transfers, but none of these functions publishes the write sequence to a recovery file. [Orchestrator fields][orch-fields] [Task completion][task-complete] [Collection and return][kept-writes]

The ordinary campaign snapshot is a different object: `saveCampaign` marshals `o.campaign`, then records snapshot request and completion events around its publication. The journal metadata at this boundary describes campaign status, task counts and snapshot identity; it does not serialize the orchestrator's private kept-write field. The mutation snapshot used for task rollback is also a local object whose file content fields are not an argument to that campaign snapshot writer. [Campaign persistence][campaign-save] [Mutation snapshot][task-snapshot]

This is a bounded claim about the cited formats and paths. A tool transcript could contain some source text, an external backup could retain old bytes, and a specialized tool could have its own recovery evidence. Those possibilities must not be presented as an assured reconstruction of every `FileWrite`, in order and bound to this mission attempt.

### The callback releases before its caller publishes

Both production entry points construct a normal attempt campaign and install a deferred `ReleaseRecurseAttempt`. They execute the campaign and return `KeptWrites()`. Their deferred close and cleanup run before control returns to the outer driver. The primary witness assumes successful acquisition and successful cleanup, so the separate rejected-acquisition and literal-path cleanup defects are not needed. [CLI][cli] [Chat][chat]

`ReleaseRecurseAttempt` retracts campaign facts and removes matching campaign artifacts. Its documented purpose is to prevent indefinite growth of disposable snapshots, journals and knowledge stores. That bounded-retention purpose is sound. What is missing here is a prerequisite showing that an independent owner has already accepted the effect-bearing recovery record. [Release][release]

Only after `r.cfg.Execute(ctx, a)` returns does the driver call `Mission.recordWrites(owned)`. That method marshals the full returned sequence into `writes-<cycle>.json`, sets `Pending.OwnedWritesRef`, and saves the mission state. The sidecar uses the existing atomic-file writer and owner-only file mode. Atomic publication once the method is entered does not cover an interruption before the method is called. [Driver order][handoff] [Sidecar publication][sidecar] [Atomic writer][atomic]

## Concrete process interruption witness

Use an ordinary authorized mission, one admitted source node, one successful campaign task and at least one real owned file modification. Use plain workspace and campaign names, valid storage, complete receipt discovery and no competing writer. The mission is not at a reused cycle with a preexisting settling ratchet row. The old session remains recent, so retention is not involved. No final recurse adoption has occurred.

The following sequence is a proposed causal witness, not a captured run:

1. **Publish the attempt and mission intent.** The driver writes the in-flight record, including the mission-state path. Mission preparation saves verification inputs and the pending receipt, then durably records the `executing` disposition after design admission. [Flight][flight] [Intent][intent] [Executing][execute-admission]
2. **Complete an owned source change.** The normal session/campaign path produces a known before/after `FileWrite`. The orchestrator retains it as a successful task's kept write. The source file contains its new bytes. [Turn writes][turn-writes] [Task completion][task-complete]
3. **Finish the executor callback.** `KeptWrites()` supplies the return value; deferred close and release succeed. The disposable campaign is gone before the outer driver receives `owned`. [Callbacks][cli] [Release][release]
4. **Terminate the process immediately after the execute call returns, before `recordWrites`.** No ordinary error return or context cancellation is required. The process-local slice disappears. No write sidecar or reference was published for this cycle. The later compensation defer and the normal cancellation branch cannot execute after process termination. [Exact cut][handoff]
5. **Inspect the persisted state.** The pending mission still says `executing`; its owned-write reference is empty. Verification-input evidence and the in-flight record can be intact, while the source has the unadopted modification. The outer journal has an attempt record, but no ratchet outcome for this attempt. Its schema does not contain the full owned-write sequence. [Receipt fields][receipt] [Sidecar][sidecar] [Journal shape][journal] [Later judgment][finish-order]
6. **Restart through each supported route.** A new mission reaches the pending-receipt census and refuses the unsettled cycle. Ordinary recurse reaches the mission-marked in-flight guard and refuses, directing the owner to reconcile against the mission state. These occur before the later Git clean-checkout preparation; a dirty-tree diagnostic is not the primary mechanism. [Startup order][startup] [Pending refusal][census] [Independent refusal][settle]

The exact result is a recovery obligation with no published mission-owned write payload. The safe refusal remains valuable, but the operator has less evidence than the sidecar design intends to preserve.

### What the witness does not establish

- It does not show a new model turn or another mutation after restart. Both guards remain closed.
- It does not show a falsely kept commit. The interruption precedes after-measurement and adoption.
- It does not prove that the edited content or every earlier byte is irrecoverable. A tracked file's base content can exist in Git; manual inspection may resolve a simple case.
- It does not show all ordinary campaign crashes have the same outcome, or that every successful callback necessarily changes a file. An actual nonempty owned write is an explicit premise.
- It does not rely on machine power loss, a torn write, unsupported directory synchronization, a race in cleanup, or the accuracy of a wall-clock failure rate. A process-level interruption at the stated boundary is sufficient.
- It does not treat retention exemptions as a repair. This receipt was never published, so keeping every session folder forever would still leave this cut open.

### Why current source and Git are not the ownership ledger

The compensation function restores a path only when its previous state is known and its current bytes still match the attempt's recorded final state. This is how it avoids overwriting a later writer. `HEAD` and the current worktree can show a net difference, but do not by themselves identify the attempt's last postimage or distinguish a later editor's contribution. [Conditional compensation][undo]

The no-concurrency witness already establishes the absent sidecar. A later-writer control makes the value of preserving it explicit: if the attempt writes B and a user later writes C, an intact receipt naming B lets existing compensation leave C untouched. Reconstructing a receipt by declaring C to be the attempt's postimage would erase that safeguard. This is a warning about a proposed recovery shortcut, not a claim that current guarded recovery performs it.

## Neighboring findings and the distinct contribution

This report deliberately credits the ordering already described in earlier audits. It does not claim that cleanup-before-publication is newly discovered or that this architecture has never discussed crash recovery.

| Existing report | Boundary it establishes | What remains different here |
|---|---|---|
| Mission retention ownership | A write sidecar has been published; later session TTL pruning deletes unresolved recovery evidence. | This witness stops before sidecar creation. No pruning or age threshold is needed, and a retention exemption cannot fix it. [Retention report][prior-retention] |
| Attempt cleanup ownership | Cleanup can retire state without acquiring the intended campaign, and pattern interpretation can select the wrong artifacts. | Acquisition and literal targeting are ordinary and successful here. The report explicitly leaves the full write-preimage publication question unestablished. [Cleanup report][prior-cleanup] |
| Mission recovery receipt census | Existing receipts are missed through glob/path semantics or incomplete enumeration. | Discovery is complete and correctly refuses; the missing object is the effect payload that was never published. [Census report][prior-census] |
| Compensation write ownership | Live cancellation must undo only its own writes and preserve other writers. | The live owned-write sequence already exists in the witness, then disappears with the process before the durable handoff. Its conditional semantics must be preserved. [Compensation report][prior-compensation] |
| Recovery attempt identity | Inadequate identity can assign an earlier adoption to a later attempt. | There is no matching settled ratchet record or adopted commit in this witness. Stronger identity alone would still identify an attempt with no published ledger. [Identity report][prior-identity] |
| Campaign generation and recovery | A persisted campaign generation must be admitted and reconciled correctly. | Campaign control-state serialization is not serialization of the private source-effect ledger. [Generation report][prior-generation] |

The distinction is buildable: **effects to process memory to durable ownership handoff** is a different acceptance surface from **durable ownership to retention**, or **acquisition to permitted deletion**. The same end-to-end design should satisfy all three; duplicate prose about the general importance of durability is not the deliverable.

## Proposed recovery and handoff contract

### Preserve one recoverable owner throughout an effect lifecycle

Before a supported mission can leave an effect outside process memory, require recovery material bound to that mission and attempt, or an explicit unsupported/incomplete-recovery disposition that prevents a stronger promise. The material must support the operation the system actually offers. For conditional file restoration, that means sufficient preimage data, exact path identity, known/absent distinctions, ordering and an attributable postimage or unresolved-effect state. A content hash can prove equality but cannot reconstruct a missing preimage by itself.

The executor retains recovery responsibility until the destination owner acknowledges a complete durable handoff. A function return, a campaign `/completed` status or a successful cleanup call is not that acknowledgment. Publish a reference with stable attempt identity, schema version, content integrity and completeness status. Readers must distinguish a published empty write set from a write set that has not yet been observed or published.

Do not infer completion from the absence of a sidecar, invent postimages from the current worktree, or delete `Pending` to make startup succeed. Keep the current refusal when evidence is missing, corrupt, incomplete, cross-attempt or inconsistent with current source. An owner-assisted resolution can remain a supported outcome, but it should receive the available recovery package and a precise account of what is missing.

### Separate a narrow handoff repair from full execution recovery

Two increments have different guarantees:

1. **Completed-callback handoff.** Persist the returned content-bearing ledger under the durable mission owner and confirm publication before releasing execution artifacts or marking handoff complete. If publishing the sidecar succeeds but publishing its reference does not, retain a discoverable, integrity-bound orphan or an explicit incomplete transfer. Do not destructively retry from an uncertain state. The public callback contract must permit acknowledgment before retirement, rather than requiring the outer caller to receive an already-disposed executor.
2. **Interruption during execution.** Extend the existing guarded mutation/observation path to record sufficient recovery information before each effect can outlive the process, and record its result or uncertainty afterward. A write-ahead record, protected preimage object, or another recoverable staging design can meet this obligation. A finished-turn return and a final callback copy are both too late for effects that preceded them.

The first increment addresses the selected handoff cut only if it actually persists the ledger before that boundary. Merely moving `ReleaseRecurseAttempt` later keeps artifacts longer but does not put the private `keptWrites` slice into them. The second increment is required before claiming crash-complete owned-effect recovery for an entire mission. Document partial support rather than marking both complete after a cleanup reorder.

Keep changes at the existing session observation, campaign owner and mission receipt seams. Host code should publish facts about the durable recovery owner, effect identity, observed content state and publication outcome. Mangle remains responsible for execution admission, continuation, evidence disposition and keep/revert decisions. No model may certify that an unrecorded write was its own or authorize restoring over a conflicting user state.

### Preserve privacy and bounded retention

Do not remove `json:"-"` from ordinary worker returns as a shortcut. That would copy source contents into every consumer that serializes a return, changing exposure and storage without defining a recovery owner. The mission's existing owner-only sidecar is a more appropriate attachment seam, subject to the unresolved-retention contract already required by the earlier audit. [Return privacy][write-type] [Existing sidecar][sidecar] [Retention contract][prior-retention]

Persist only the content and metadata needed for supported recovery. If representation compacts repeated writes, it must preserve the original preimage, attributable final postimage and any unresolved gaps needed to avoid claiming another writer's state. Deduplication, compression and content addressing can reduce repeated bytes, but their integrity, garbage collection and reference lifetime must be part of the contract. Unknown content cannot be compacted into known content.

Retire the recovery package only after adoption or compensation is durably reconciled and all consumers have released it. Repeated failure during compensation must retain both remaining work and the outcomes already established. An orphan collector must not use elapsed time alone as permission to remove unresolved evidence.

## Primary literature and limits

Mohan and colleagues' **ARIES** describes write-ahead logging as an ordering rule between recovery information and persistent updates, not merely an atomic write of a log after work finishes. The transferable principle is that recovery data has to survive before the corresponding effect can leave recovery dependent on process memory. This report does not propose importing ARIES page recovery or claim that CodeNERD implements its transaction model. [ARIES, Section 1.1][aries]

Garcia-Molina and Salem's **Sagas** explicitly records commands and compensation parameters, and records compensation progress so another interruption can be recovered. It also discusses preserving the information needed by a higher-level execution coordinator when individual transactions complete. The useful analogy is layered execution ownership: a lower-level success does not remove the higher-level need to know how to reconcile a partial operation. CodeNERD source edits are not database transactions; external editors, tool-specific effects and file metadata require their own contract. [Sagas, Sections 3, 4 and 7][sagas]

These works support the proposed ordering and recovery distinctions. The actual defect is established by the pinned CodeNERD producer, cleanup, publication and restart paths, not by treating a database paper as proof of this application's behavior.

## Costs and failure modes

Let W be the number of recorded file effects and B the sum of required preimage/postimage content bytes. The current post-execution sidecar already serializes a content-bearing sequence of size proportional to B plus W path/metadata records. Moving its publication earlier changes lifecycle and critical-path I/O. Recording throughout execution can add up to one or more persistence operations per effect, additional hashing/copying, temporary-file operations and recovery metadata. Neither cost should be hidden in a token-only comparison.

Measure write latency, synchronization count, bytes read and written, peak memory, retained disk bytes, directory operations, lock hold time, recovery scan time and restart latency. Rewriting the full accumulated sidecar after every effect can amplify writes far beyond B; append-only or per-effect records avoid that particular rewrite pattern but introduce framing, completion and compaction obligations. Batching can reduce synchronization overhead only if effects do not cross the promised durability boundary ahead of their recovery record.

The ordinary `FileState` representation carries content rather than complete filesystem object metadata. Do not advertise general restoration of permissions, links or every tool effect solely because the same structure becomes durable. Existing metadata and compensation audits remain separate constraints. A bounded first release can explicitly support the current regular-file content operation and refuse stronger claims.

| Failure or tradeoff | Required handling |
|---|---|
| Recovery storage unavailable before an effect | Refuse the effect or use an explicitly supported safe alternative; do not continue with an empty success receipt. |
| Effect outcome uncertain after a durable intent | Preserve uncertainty and inspect the actual resource before replay or compensation. |
| Sidecar complete but reference save fails | Retain attempt-bound discoverability; a retry must not erase a valid payload or adopt a different attempt's file. |
| Later writer changes the same path | Preserve the existing current-state match requirement and require owner resolution on conflict. |
| Crash during compensation | Retain remaining obligations and established progress; repeated recovery must not blindly undo twice. |
| Large files or repeated edits | Define bounded admission and storage behavior without dropping the recovery-required suffix silently. |
| Secret or proprietary source in preimages | Keep narrow permissions, authorized local ownership and controlled retention; do not leak through ordinary logs or model context. |
| Cleanup or garbage collection fails | Retain named cleanup debt and exact ownership, rather than widening deletion scope. |
| A legacy pending mission lacks the new ledger | Surface incomplete recovery explicitly; do not synthesize trustworthy ownership from filenames or timestamps. |

No model call is inherently required by the proposed host correction. It may nevertheless increase I/O and reduce write throughput. It may save manual reconstruction and repeated work after interruption, but no latency, reliability, token, cost or quality benefit was measured here.

## Fair evaluation plan entirely unrun

Compare the current implementation, a completed-callback durable handoff and the fuller effect-level design separately. Use the same source files, permitted scope, kernels, executor outputs, configuration, filesystem and process-interruption points. Include ordinary success and failure cases so an implementation that never admits a write does not appear superior merely because it loses no recovery evidence.

### Establish the exact handoff behavior

Use disposable workspaces and a controlled process harness. Drive the normal CLI/chat callback composition with scripted model responses and real host-owned writes, keeping live providers disabled. Synchronize at named boundaries rather than hoping a timed sleep happens to hit the gap. Stop the process without unwinding deferred cleanup, then inspect the filesystem from a new process.

Required cases:

| Interruption or control | Required observation |
|---|---|
| Before any admitted mutation | No source effect is attributed; pending intent is classified accurately. |
| After a real write but before turn return | Either durable recovery material covers the effect, or the explicit supported limitation remains visible; a final-return-only repair must not claim this case passed. |
| After task success but before callback completion | The effect-level design retains a recovery owner independent of the dying process; the completed-callback-only increment must report this earlier cut as outside its guarantee. |
| Primary post-callback, post-cleanup, pre-sidecar cut | A corrected handoff preserves a complete attempt-bound owned-write package; the baseline's absent sidecar is observable. |
| Sidecar published, mission reference not yet saved | Recovery finds or reports the incomplete transfer without guessing ownership or losing the payload. |
| Sidecar and reference published | The ordinary post-publication recovery control retains the payload and refuses uncertain replay. |
| Graceful cancellation at comparable work | Existing conditional restoration remains correct; this is not a substitute for the hard-interruption case. |
| Another writer edits an owned path afterward | Recovery leaves the conflicting bytes intact and reports the unresolved ownership boundary. |
| Crash during compensation and repeated restart | Previously settled actions are not blindly repeated, and remaining obligations remain visible. |
| Kept commit with delayed acknowledgment | Existing adoption reconciliation is preserved; compensation must not undo an ambiguously committed change. |
| Empty returned write set and failed callback | Empty, unpublished and incomplete are observably distinct. |
| Legacy pending receipt, unavailable storage and corrupt payload | Fail closed with the exact missing evidence, without automatic deletion or permission expansion. |

For the primary cut, assert the exact source bytes, mission ID, attempt identity, `Pending` state, sidecar contents/reference, campaign artifact disposition, in-flight marker and absence of a new adopted commit. Restart once as a mission and once as ordinary recurse from equivalent independent fixtures. Verify that neither route silently treats missing evidence as permission to continue. Do not remove the pending marker in the test merely to make the next invocation start.

### Preserve the current controls

The existing cancellation fixture returns explicit owned writes and checks that unrelated or subsequently changed files survive. The interrupted-mission fixture manually creates pending state and checks refusal from both mission admission and ordinary recovery. The release fixture checks normal deletion of one disposable campaign while another remains. These are useful authored controls; they do not execute the primary callback-to-sidecar interruption and demonstrate retained content-bearing ownership afterward. None was run in this audit. [Cancellation fixture][cancel-test] [Interrupted-mission fixture][interrupted-test] [Release fixture][release-test]

A deterministic handoff fixture can establish preservation without evaluating model intelligence. A later end-to-end study should hold model/provider, prompt, mission, budget and acceptance gates fixed, inject the same interruption distribution, and count all outcomes: correct retained work, correct compensation, conflicting user edits preserved, blocked incomplete recovery, operator interventions, repeated paid work and total recovery cost. Report unsupported effect types and failure cases rather than averaging them into successful resumes.

**Checkable exit:** at the demonstrated completed-callback cut, a source effect cannot survive without its complete mission-owned recovery package or a still-retained durable producer from which that exact package is recoverable. Execution and compensation interruption support must be qualified separately. Correct restart refusal, literal acquired cleanup ownership, privacy boundaries, source conflicts, scope and ratchet outcomes remain intact. A cleanup reorder or a passing graceful-cancellation test alone does not meet this exit.

## References

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/8a8aebcdc54eb4901d81e0ce25b01641c072842b
[root]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/agents.md#L1-L68
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/agents.md#L1-L15
[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L184-L235
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_mission.go#L66-L97
[intent]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_mission.go#L211-L243
[execute-admission]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_mission.go#L291-L323
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/cmd/nerd/cmd_campaign_recurse.go#L225-L246
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/cmd/nerd/chat/campaign_recurse.go#L218-L266
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_cycle.go#L1282-L1304
[sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_mission_storage.go#L51-L78
[census]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_mission_storage.go#L17-L48
[settle]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_cycle.go#L1783-L1840
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_cycle.go#L378-L432
[flight]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_cycle.go#L1239-L1253
[cancel]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_cycle.go#L1305-L1323
[finish-order]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_cycle.go#L1481-L1571
[release]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_attempt.go#L256-L285
[turn-writes]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/session/turn_writes.go#L11-L46
[write-type]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/observation/subagent.go#L172-L206
[effect-type]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/observation/effects.go#L8-L74
[orch-fields]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/orchestrator_types.go#L20-L46
[task-complete]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/orchestrator_task_transaction.go#L100-L124
[task-snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/orchestrator_task_transaction.go#L25-L57
[kept-writes]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/orchestrator_attempt_writes.go#L109-L129
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/orchestrator_attempt_writes.go#L186-L239
[campaign-save]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/orchestrator_lifecycle.go#L62-L111
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_journal.go#L35-L59
[atomic]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/atomicfile/atomicfile.go#L159-L222
[prior-retention]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/audits/recurse-mission-retention-ownership.md#L45-L99
[prior-cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/audits/recurse-attempt-cleanup-ownership.md#L133-L175
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/audits/recurse-mission-recovery-receipt-census.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/audits/recurse-compensation-write-ownership.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/audits/recurse-recovery-attempt-identity.md
[prior-generation]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/Docs/audits/campaign-generation-admission-and-recovery.md
[cancel-test]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_owned_undo_test.go#L13-L75
[interrupted-test]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_mission_policy_test.go#L160-L178
[release-test]: https://github.com/theRebelliousNerd/codenerd/blob/8a8aebcdc54eb4901d81e0ce25b01641c072842b/internal/campaign/recurse_soak_test.go#L183-L227
[aries]: https://www.cs.cmu.edu/~15849g/readings/mohan92.pdf
[sagas]: https://www.cs.cornell.edu/andru/cs711/2002fa/reading/sagas.pdf
