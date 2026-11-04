---
doc-class: deep-dive
subsystem: campaign-mission-sessionlog
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 17e7c434fabba03bedc34c491496dde8553f71ca
verification-method: static-source-inspection
supersedes: []
---

# Session pruning can erase unresolved mission recovery evidence

## Finding and decision

**A normal cold boot can delete an interrupted mission's pending receipt and owned-write sidecar before recurse recovery asks for them.** Mission control state lives inside an ordinary session folder. Session startup deletes the entire folder when its newest file is older than the configured retention, which defaults to 14 days, unless the session is currently protected by the keep set or a fresh heartbeat. Pruning does not inspect mission settlement or references from the independent recurse in-flight record. [Mission storage][mission-location] [Default retention][retention] [Pruning][prune]

The primary consequence is **lost reconciliation evidence while recovery remains blocked**, not automatic permission to repeat an uncertain effect. In the concrete interruption below, `.nerd/recurse/inflight.json` survives and still refuses recovery, but directs the operator to a mission file that startup has just deleted. The refusal is correct; the evidence lifetime is not. [Remaining guard][settle]

**Required contract:** an unresolved mission owns its recovery records and required sidecars until a durable reconciliation disposition releases that obligation. Session inactivity, an expired process heartbeat and an ordinary log-retention deadline are not evidence of settlement. Retain ordinary session retention, but make it subordinate to recovery ownership, or separate authoritative recovery state from disposable session history.

This is a source-derived composition defect, not an observed production loss or a measured incident rate. No implementation, tests, builds, fixtures, model inference or training were run. The recommendations and evaluations below are unimplemented and unrun.

## Evidence boundary and architectural intent

Public main was inspected at [`17e7c434fabba03bedc34c491496dde8553f71ca`][pin] on November 4, 2026, and rechecked unchanged at 15:48 UTC. The investigation read current root and scoped guidance, campaign and logging architecture, the audit inventory and relevant report bodies, normal boot and CLI wiring, mission persistence, attempt cleanup, recovery and authored tests. Local source copies were verified against the pinned tree's Git blob hashes. All repository links below identify that revision.

Root guidance requires durable facts and deterministic executive control. The architecture guidance keeps the required capability separate from its current implementation. The mission specification already requires pending receipts to be reconciled before further effects and explicitly retains full owned-write and check sidecars independently of disposable attempt campaigns. The larger mission proposal already says evidence should retire only after its live and recovery obligations end. This report does not invent that principle: it identifies a reachable deletion path that violates it. [Root][root] [Architecture guidance][architecture-guidance] [Mission recovery contract][mission-contract] [Existing retirement requirement][innovation]

The configuration layer accurately describes session retention as deletion after last activity. The sessionlog implementation is internally consistent with that description; the conflict appears because the mission feature places authoritative recovery material in the same recursively deleted namespace. A configuration warning or a longer default TTL alone would leave that ownership mismatch intact. [Configuration][retention] [Layout][layout]

| Capability | Status at the pin | Scope and evidence |
|---|---|---|
| Save a mission's pending intent before execution | Implemented | Mission preparation persists input digests, the pending receipt and later its executing disposition. [Intent][intent] [Execution admission][execute-admission] |
| Persist returned owned writes independently of the attempt campaign | Implemented | `recordWrites` writes a sidecar and updates the pending receipt's reference. [Owned writes][write-sidecar] |
| Preserve complete measured check output and comparison domains | Implemented after that stage is reached | `recordChecks` writes a separate sidecar. The primary interruption occurs earlier, so it does not assume this sidecar already exists. [Check sidecar][check-sidecar] |
| Exempt the current session, touched sessions and fresh leases from TTL deletion | Implemented | Boot assembles the keep set; pruning checks it and the lease. [Keep set][boot-keep] [Lease][lease] |
| Refuse an unsettled mission-marked in-flight attempt | Implemented | The independent record blocks the ordinary difference-from-HEAD recovery route when no ratchet row settles it. [Recovery][settle] |
| Exempt unresolved mission records and their dependencies from session pruning | Missing on the inspected path | Eligibility is based on keep membership, heartbeat and newest-file time; deletion is recursive. [Pruning][prune] [Activity][activity] |
| Guarantee all possible recovery evidence exists elsewhere | Not established | The attempt campaign is explicitly disposed of; Git and the recurse journal are not copies of the full mission-owned write ledger. [Disposal][release] [Journal schema][journal] |
| Runtime prevalence, recovery effort and performance | Unverified | Static inspection only. |

## Production path and concrete causal witness

### 1. Startup maintenance precedes mission recovery

The CLI's normal `runCampaignRecurse` path calls `GetOrBootCortex` before constructing the mission and before `RunRecurseCycles`. Default Cortex boot includes `initSessionLog`. If no explicit session ID is supplied, that adapter generates a fresh ID, reads the effective session TTL and calls `sessionlog.StartSession`. No debug exchange setting is needed to reach pruning. [CLI order][cli-start] [Boot stages][boot-stages] [Session-log adapter][adapter]

`StartSession` creates or reuses the process store, protects the booting session and its in-process `Touched` set, then prunes. A new process's new store has an empty touched map. That map does not recover all historical mission owners from disk. The old session must independently satisfy the lease or age exemption to survive. [Boot keep set][boot-keep] [Fresh store][store-new]

`Prune` enumerates eligible session directories. For an unprotected directory it checks lease freshness, computes the newest file modification time recursively, and calls `os.RemoveAll(dir)` if that activity exceeds the TTL. Neither `Pending`, `OwnedWritesRef`, `VerificationInputsRef`, nor an external in-flight reference participates. All the required filesystem operations can succeed; no permission error, malformed JSON, exotic filename or race is necessary. [Prune][prune] [Activity][activity]

### 2. The receipt is deliberately more than a diagnostic log

The mission snapshot contains the exact mission, limits, pending attempt, scope, source and verification identity. Before effects, `prepare` writes the verification-input sidecar under the same mission directory and saves `Pending`. After admitted design, it saves the executing state before returning the attempt. [Mission state][mission-types] [Intent publication][intent] [Execution admission][execute-admission]

After the executor returns, the recurse driver calls `recordWrites`. Its payload is `[]observation.FileWrite`, whose elements contain each path and the before/after states, including content when known. This is not merely a list of filenames or hashes. The sidecar is written with owner-only permissions, then its path is committed into the pending mission snapshot. [Driver handoff][write-cut] [Sidecar publication][write-sidecar] [Write representation][write-type]

This evidence has a distinct role: recovery must distinguish owned effects from unrelated user or external-editor changes. The current source tree by itself cannot establish that authorship. Retaining the file's postimage while discarding the recorded preimage and ownership is not equivalent retention.

### 3. The attempt campaign is not an assured backup

The CLI execute callback defers `ReleaseRecurseAttempt`. That defer runs before the callback returns its owned writes to the outer driver. The helper retracts attempt campaign facts and removes matching campaign artifacts, including its snapshot, journal and knowledge base. The mission sidecar is therefore published after the normal attempt cleanup has already run. [CLI callback][cli-execute] [Release contract][release]

The primary witness assumes this cleanup succeeds. It does not require failure of a backup service or claim no external backup can exist. It shows why the normal attempt checkpoint must not be cited as an automatic second copy of mission recovery evidence at the selected boundary.

### 4. Conditional witness, not a reproduction

Use a normal authorized CLI mission in a plain workspace path. The run has valid configuration, a working kernel and an admitted design. Storage operations and attempt cleanup succeed. There is no competing writer, unusual clock behavior or directory-enumeration failure.

1. **Begin and admit attempt A.** The driver publishes `inflight.json` with a `MissionState` reference. Mission preparation durably saves its pending intent and verification inputs. [In-flight publication][flight] [Intent][intent]
2. **Execute and preserve returned effects.** The normal campaign callback completes with at least one kept owned write and runs its cleanup. The outer driver successfully writes `writes-<cycle>.json` and saves the reference in the still-pending mission snapshot. [Callback][cli-execute] [Publication cut][write-cut]
3. **Interrupt the process immediately after `recordWrites` returns.** No after-measurement, adoption or ratchet settlement has occurred for this attempt. This is a hard process interruption, not ordinary cancellation or an error return: Go defers do not execute. The process's in-memory ownership disappears; the durable mission record and sidecar survive at this point. [Compensation boundary][write-cut] [Later settlement][finish-order]
4. **Leave the old session inactive beyond the TTL.** At the next boot, every file under the old session is older than the TTL, its lease is absent or stale, and the process has not adopted or touched that session. The default TTL is 14 days. A short configured positive TTL exercises the same rule without implying that the default is short. [Retention][retention] [Lease][lease] [Activity][activity]
5. **Start a fresh CLI process in the same workspace using a different session ID.** Its normal cold boot reaches session pruning before recurse startup recovery. The old folder qualifies and is removed, including the pending mission snapshot, verification-input sidecar, owned-write sidecar and any other files it contains. [Entry order][cli-start] [Keep set][boot-keep] [Recursive deletion][prune]
6. **Reach recurse recovery.** Because the old pending receipt was deleted, the new mission's `begin` can pass its census and save a fresh mission record. The separately stored in-flight record remains. Because the witness has no ratchet record for A, `settleInFlight` still refuses before any new recurse attempt executes and names the now-missing `MissionState` path. The new mission is then ended with that error. Some new control state has been written, but the source edits are not automatically replayed or reverted by this path. [Census][census] [Fresh mission record][mission-location] [Startup recovery][run-start] [Independent refusal][settle]

The resulting state is internally inconsistent: the guard correctly requires reconciliation while automatic maintenance has removed the evidence it identifies for that reconciliation. The exact lost objects are established by storage and deletion paths. The amount of manual work needed to reconstruct an outcome is not established.

### Countercontrols and limits

- **The independent in-flight guard still matters.** This witness does not establish a permission bypass, a repeated mutation, a false successful mission or automatic loss of the source files themselves. Pruning removes session material, not the workspace's source tree or `.nerd/recurse` directory. [Layout][layout] [Flight path][flight-type] [Refusal][settle]
- **Ordinary cancellation has a different path.** The driver attempts owned compensation and settlement on cancellation. The witness requires abrupt interruption before that path can run. A graceful-cancellation fixture is a negative control, not a substitute for crash qualification. [Cancellation][cancel]
- **Activity means the newest file, not mission creation time.** One sufficiently recent file anywhere below the old session, a fresh heartbeat, explicit protection of that session, or retention set to `off` prevents the stated deletion. [Retention][retention] [Prune][prune] [Activity][activity]
- **Git and the journal remain useful.** Git can establish committed content; the recurse journal retains attempt and ratchet fields. At the chosen cut there is no adopted commit or ratchet row for A, and the journal schema does not contain the full `FileWrite` payload. Neither source proves that every lost detail is unrecoverable from every other source. [Journal][journal] [Adoption ordering][adopt]
- **The newest run is not deleting its own session.** The booting session is protected. The witness concerns an older interrupted mission in another session of the same workspace, exactly the cross-session recovery domain `checkInterrupted` searches. [Keep set][boot-keep] [Census][census]
- **Boot can erase evidence even if later boot fails.** Recurse recovery need not run for the deletion to have occurred. The primary witness assumes successful boot to demonstrate the dangling recovery instruction; no claim that rollback restores a pruned folder is made.

## Nonduplication and source counterevidence

The relevant audit inventory, mission/campaign reports, sessionlog report and targeted repository searches were screened. The following boundaries matter:

- **Mission recovery receipt census:** that report concerns existing records missed by glob syntax or suppressed directory-read errors. Here a plain path and complete, successful enumeration are enough; startup already removed the records. A literal census cannot rediscover deleted evidence. The prior report explicitly does not establish a new retention policy as necessary for its own finding. [Prior census][prior-census]
- **Recurse journal replay coverage:** that report concerns JSONL generation retention, malformed tails and reconstructed refusal/stall history. This witness preserves the journal and in-flight record while deleting mission-owned recovery sidecars in a separate namespace. Both need lifecycle integrity, but the owners and acceptance fixtures differ. [Prior replay][prior-replay]
- **Sessionlog durability and responsiveness:** that report analyzes transcript rewrite costs and progress locking. It includes retention among evaluation dimensions but does not identify pending mission state as an exception to whole-session TTL deletion. [Prior transcript][prior-transcript]
- **Generated-call receipt retention:** that report concerns completed bodies retained too long in two owners, while preserving execution identity. This is the converse lifecycle error: an unresolved body's recovery obligation can outlive its containing session's TTL. Its general distinction between full bodies and durable identity remains useful. [Prior generated receipts][prior-generated]
- **Mission-driven improvement:** the existing proposal already requires recovery-aware evidence retirement. This report is a concrete implementation gap under that requirement, not a new architecture or a claim that evidence lifecycle theory originated here. [Existing design][innovation]

Current authored sessionlog tests intentionally require an expired folder to disappear with everything in it, protect the booting and running sessions, use the newest file for age and preserve all sessions when TTL is zero. These are valuable controls and explain why the current sessionlog behavior is not an accidental failure of its isolated test contract. The inspected tests do not add a pending-mission exception. The mission policy test writes a pending receipt and expects the next mission startup to reject it; it does not run pruning first. Neither set establishes the joined lifecycle. No test was executed. [Retention tests][retention-tests] [Direct mission test][mission-test]

## Proposed retention contract

### One owner for unresolved recovery

Before the first effect is admitted, establish a durable recovery owner for the attempt and the artifacts needed to reconcile it. The current mission receipt and sidecars can remain the representation; a new universal event architecture is not required.

The owner must retain at least the mission/attempt identity, admitted scope and base/source identity, execution disposition and uncertainty, owned-write records, relevant verification inputs/results, and references needed to distinguish committed, compensated and unresolved effects. The exact minimal set should follow the recovery operation, not the convenience of a log writer. Do not substitute a hash for content when a supported compensation operation needs the preimage.

Retirement is a transition from **unresolved** to **durably reconciled**, followed by the applicable archival policy. No timestamp should independently make that transition. A process lease answers whether a producer may still be active; an old lease does not establish that its effects are settled. A journal verdict and a settled mission receipt also must not be conflated when either carries an outstanding publication or evidence obligation.

### Bounded implementation choices

Two compatible directions deserve evaluation:

1. **Protect affected session folders before pruning.** Enumerate and validate mission ownership, including independent in-flight references, and add unresolved owners to the keep decision. This is the smallest initial safety repair. It conservatively retains unrelated history in the same session, so it may retain substantially more bytes than recovery needs.
2. **Separate recovery authority from session history.** Move or publish the required recovery package under an existing durable recurse-owned namespace and make the session entry a diagnostic reference. Publish the destination and its integrity/ownership metadata before releasing the old location. The recovery reader, in-flight pointer and legacy-state migration must agree; moving only the JSON without its sidecars or leaving an old pointer is not a fix.

These are proposed choices, not shipped paths or schema names. Prefer extending existing campaign, recurse and sessionlog owners over adding a new always-running retention service.

Go should measure the eligible directories, ownership records, integrity and referenced artifacts. The executive retention/admission decision should preserve Mangle's authority where integrated into the runtime; models must not decide that an old pending receipt is safe to forget. The current factory creates the kernel before the session-log stage, but a complete new policy surface still requires producer, schema, consumer, failure behavior and tests. Direct leaf callers must not silently bypass the same safety contract. [Boot order][boot-stages] [Architecture guidance][architecture-guidance]

### Failures, concurrency and migration

- A malformed, unreadable or unknown-version mission record is **unknown**, not settled. Skip deletion of the affected recovery owner and surface a bounded diagnostic. Ordinary boot may remain available for inspection; missing evidence must still prevent dependent mission effects.
- Inspection and deletion need a shared ownership protocol. A pre-scan followed by uncoordinated deletion creates a new race with receipt publication, adoption or session reuse. Define a deletion fence/claim and its interaction with all producers, including cross-process writers. The current recurse lock alone does not serialize an earlier session-log boot in another process.
- Publish reconciliation before releasing its pin. Failure between publication and cleanup may safely retain extra data; the opposite ordering must not delete still-needed recovery evidence.
- Validate all paths and owner identities before retaining, moving or deleting a referenced object. A recorded path is not authority to read or remove arbitrary files. Preserve workspace containment, exact identities and owner-only modes.
- An explicit owner decision to discard unresolved evidence must state the lost recovery capability and cannot manufacture a settled outcome or authorize replay. Ordinary log TTL configuration is not such a decision.
- For legacy state already pruned, report missing recovery evidence honestly and keep the appropriate hold. Do not synthesize a receipt from the current tree, delete the in-flight marker to get unstuck, or promote missing history into success.
- If storage pressure prevents safe retention, expose the resource problem and require a legitimate resolution. Silently expiring live obligations is not a bounded-storage solution.

A larger TTL or a touch-on-start workaround only postpones deletion. Disabling all retention retains every log forever. Removing the in-flight guard would turn an evidence-loss problem into unsafe recovery. None meets the contract.

## Primary research and its limited role

Recovery literature treats durable history and partial rollback as part of the recovery protocol, rather than interchangeable diagnostics. ARIES is an established example of write-ahead logging and partial-rollback recovery, including applicability to recoverable file systems. That is background for preserving recovery authority, not an instruction to transplant a database recovery engine into recurse. [Mohan et al., ARIES][aries]

SQLite supplies a precise implementation analogy: its WAL is part of persistent database state after an unclean close, and safe checkpoint/reset ordering requires the data to be persisted before the WAL is reset. Its documented corruption/loss consequences belong to SQLite, not this mission implementation. The transferable design inference is narrower: an artifact's authoritative lifetime follows what still depends on it, not whether the producer process is alive or whether its filename looks like a log. [SQLite WAL documentation][sqlite]

Neither source proves the CodeNERD witness, estimates its frequency or validates the proposed repair. That evidence comes from the pinned source; future runtime qualification remains necessary.

## Full costs and proposed evaluation

### Costs and tradeoffs

Current pruning already walks files beneath eligible sessions to find latest activity and total bytes, then recursively removes selected folders. Let F be inspected files, M mission records, B bytes read to classify them, and R bytes required by unresolved recovery packages. A direct mission-aware pass adds roughly O(M + B) classification work to the existing walk, plus validation and synchronization. It is not free merely because the session tree is already visited. Persisted indexes can reduce repeated scanning but add publication, corruption detection and migration work. [Current walk][activity]

Protecting an entire session retains all of its bytes, not just R. A small unresolved receipt can therefore pin large transcripts and exchanges. Separating recovery packages reduces collateral retention but adds copied or moved bytes, temporary double storage, crash-safe reference changes, checksums and compatibility paths. Full before/after content in `FileWrite` makes payload size depend on changed-file sizes, not just receipt count. [Write format][write-type]

A correct unresolved hold may last indefinitely until someone reconciles it. That is an explicit storage and privacy cost to manage with owner-visible status, compact required state, authorized archival/discard choices and bounded diagnostics. It is not a reason to hide the obligation. Preserve owner-only permissions and avoid placing file contents into telemetry or ordinary status messages.

This repair adds no necessary model call. Its potential value is preserved recovery evidence and less manual reconstruction, not a claimed reduction in tokens or provider latency. Measure cold-boot latency, files/bytes inspected, retained bytes, synchronization waits, partial-deletion failures, restart diagnostics and actual recovery effort. Include failed and abandoned runs; do not benchmark against a baseline that deliberately destroys required state and call the lost durability a speedup.

### Deterministic evaluation matrix, all unrun

| Case | Required observation |
|---|---|
| Pending executing receipt and owned-write sidecar, all old beyond TTL, stale lease, different cold-boot session | Both receipt and required sidecars survive maintenance; recovery reports the unresolved owner with readable evidence. |
| Same data immediately before and immediately after the TTL threshold | Crossing age alone cannot change mission settlement or erase its recovery package. |
| No pending mission and no recovery reference | Ordinary configured session pruning still reclaims the eligible history. |
| Newest-file control, fresh lease, same booting session, touched-session control, TTL off | Existing exemptions still hold; age is measured over every file. |
| Pending snapshot plus independent in-flight reference | The two owners resolve to the same retained package; no dangling instruction after startup. |
| Malformed/unreadable receipt, missing sidecar, unknown version, failed classification | Affected state stays protected or explicitly held as incomplete; no fabricated empty census or settlement. |
| Publication/deletion interleaving, session reuse, another process, reconciliation racing prune | The ownership/deletion fence prevents late deletion of admitted recovery state. Race-detector results alone do not prove this protocol. |
| Reconciliation persisted, retirement publication fails | Extra retained state is safe and retryable; a still-needed source is never removed first. |
| Move/copy interruption during a namespace migration | At least one validated recovery package remains discoverable, and its references identify it without double ownership or silent loss. |
| Old already-pruned legacy package | Honest missing-evidence hold and useful operator diagnostic; no automatic replay or removal of the remaining guard. |

Begin with a deterministic boundary fixture using an injected clock and harmless temporary files. Build a structurally valid pending mission and real owned-write payload, then invoke the actual `StartSession`/prune and recovery functions. Assert that the current pinned implementation takes the deletion path under the stated premises; do not label that expected failure a passing implementation result.

A separate process fixture should establish the full sequence through the normal CLI attempt callback: scripted execution, successful campaign cleanup, durable `recordWrites`, forced interruption at that boundary, cold boot and recovery. No real provider call is required. Disable or replace unrelated model-capable/background work with counted fail-on-call fixtures, rather than assuming a sessionlog unit test qualifies full Cortex boot. Test graceful cancellation separately and confirm the existing compensation and in-flight refusal remain intact.

Run matched baseline/treatment experiments with identical source, activity timestamps, payloads, TTL, session IDs, leases, filesystem conditions and allowed recovery authority. Report each lost or retained object and each admission decision, not only a final error string. For performance, vary number of old sessions, missions per session, write payload size, collateral transcript size and unresolved proportion. Include startup with a late failure after pruning and filesystem deletion errors.

No live mission-quality study is needed to establish the deterministic lifetime defect. A later normal-entry task study may measure practical repair/recovery benefit with the usual world-model and CodeDOM context enabled, but must keep models, budgets, tasks, permitted effects and acceptance oracles comparable. Existing test definitions and cited design prose are not runtime receipts.

## Checkable exit

The gap is closed when the normal cold-boot maintenance path cannot retire an unresolved mission's required recovery evidence, the reader and retirement owner agree on a durable settlement/release transition, and ordinary expired diagnostic history still has a working bounded-retention path. The acceptance evidence must include the after-`recordWrites` crash boundary, legacy/incomplete records, cross-process deletion/publication overlap and source-work preservation.

A clean retention unit test, a surviving JSON filename without its sidecars, or a continued refusal pointing at deleted data does not meet that exit.

[pin]: https://github.com/theRebelliousNerd/codenerd/tree/17e7c434fabba03bedc34c491496dde8553f71ca
[mission-location]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission.go#L127-L157
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/config/logging.go#L29-L68
[prune]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/prune.go#L54-L100
[settle]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1783-L1806
[root]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/architecture/AGENTS.md
[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L185-L215
[innovation]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/innovations/mission-driven-forever-improvement.md#L169-L203
[layout]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/layout.go#L10-L47
[intent]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission.go#L210-L243
[execute-admission]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission.go#L294-L324
[write-sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission_storage.go#L51-L78
[check-sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission_storage.go#L103-L152
[boot-keep]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/boot.go#L39-L95
[lease]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/prune.go#L14-L42
[activity]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/prune.go#L102-L128
[release]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_attempt.go#L252-L284
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_journal.go#L31-L66
[cli-start]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/cmd/nerd/cmd_campaign_recurse.go#L150-L225
[boot-stages]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/system/factory.go#L2892-L2926
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/system/session_log_boot.go#L12-L44
[store-new]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/store.go#L43-L60
[mission-types]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission.go#L46-L97
[write-cut]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1288-L1304
[write-type]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/observation/subagent.go#L192-L207
[cli-execute]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/cmd/nerd/cmd_campaign_recurse.go#L225-L247
[flight]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1238-L1252
[finish-order]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1548-L1574
[census]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission_storage.go#L17-L49
[run-start]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L384-L441
[flight-type]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1109-L1125
[cancel]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1305-L1324
[adopt]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_cycle.go#L1481-L1510
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/audits/recurse-mission-recovery-receipt-census.md
[prior-replay]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/audits/recurse-journal-replay-coverage.md
[prior-transcript]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/audits/sessionlog-durability-progress-responsiveness.md
[prior-generated]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/Docs/audits/generated-call-receipt-retention.md
[retention-tests]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/sessionlog/lifecycle_test.go#L84-L171
[mission-test]: https://github.com/theRebelliousNerd/codenerd/blob/17e7c434fabba03bedc34c491496dde8553f71ca/internal/campaign/recurse_mission_policy_test.go#L160-L178
[aries]: https://research.ibm.com/publications/aries-a-transaction-recovery-method-supporting-fine-granularity-locking-and-partial-rollbacks-using-write-ahead-logging
[sqlite]: https://www.sqlite.org/wal.html
