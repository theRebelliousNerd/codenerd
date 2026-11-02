---
doc-class: deep-dive
subsystem: generated-tool-retirement
implementation-status: proposed
last-verified: 2026-11-02
verified-against: bed7cde961483023ac106cc2c9b922c78f6ec83a
verification-method: static-source-inspection
---

# Generated-tool retirement must revoke the reviewed revision, not its current namesake

**Decision:** bind retirement to a host-owned registration identity and perform conditional revocation under the same ownership boundary as publication. Rebuilding an approval subject after review is necessary, but an unlocked recheck followed by name-based deletion is insufficient.

**Evidence status:** static inspection of CodeNERD main `bed7cde961483023ac106cc2c9b922c78f6ec83a`. The finding is a conditional producer/consumer composition, not a reproduced incident, exploit demonstration or measured failure rate. No tests, application execution, model inference, generated binaries or repository edits were performed. This document proposes a repair and unrun evaluations.

## 1. The residual boundary

`RetireGeneratedTool` correctly asks the approval broker about a subject containing the tool name and its observed `RuntimeTool.Hash`. After approval, however, it passes **the original captured subject** to `Grant.Reserve`, then invokes `RevokeTool(name)`. The revoker resolves the current registration by name and deletes its compiled artifacts. If a legitimate same-name replacement publishes a different hash while the approval waits, an approval recorded for the old hash can withdraw the replacement. [Approval subject and dispatch][retire-admission]; [revocation][revoke].

The broker is not failing to compare supplied subjects: `Reserve` computes the digest of the `current` subject its caller supplies, checks it against the request and grant, and supersedes a mismatch. The retirement adapter supplies no newly resolved state. This is an integration gap between current-state measurement and an otherwise useful exact-request broker. [Broker reservation][reserve].

There are three separate contracts:

1. **Consent identity:** the approved registration revision is the one being retired.
2. **Current eligibility:** the policy still permits/proposes retirement under whatever post-review policy contract the product adopts.
3. **Mutation ownership:** the registration cannot change between successful validation and the destructive operation, and cleanup cannot remove a later incarnation.

The primary witness violates the first and third without needing a change in policy eligibility. A stale policy reason is an adjacent concern: the retirement path computes its report only before waiting. Do not substitute re-running that report for comparing registration identity.

## 2. Existing contracts and bounded overlap

Generated-tool guidance already makes registration host-owned and names workspace, binary path, digest and protocol as identity inputs. The implemented tools specification says retirement requires a derived generated-tool proposal and approval of the exact request. The retirement method's own comment promises refusal when the approved operation changes between review and execution. [Guidance][guidance]; [tools specification][tools-spec]; [method contract][retire-admission].

The earlier `request-bound-human-approval` audit establishes immutable request subjects, current-state reservation, one-shot dispatch and explicit uncertain outcomes. Its as-built executor path rebuilds the subject before reservation; it also discloses an external-writer race after reservation. This report does not re-propose the broker or claim that its implemented controls are absent. It identifies a concrete later retirement adapter that reuses a pre-review snapshot and a name-only backend; the minimal witness needs no external filesystem writer. [Prior approval contract][approval-audit].

The residual generation-restriction audit concerned whether a requester admitted generation's write/execute effects. Current `generation_admission.go` has request-restriction admission. This finding assumes independently admitted generation and successful existing safety checks, not a generation-permission bypass. Existing operation locks also serialize publication and execution per tool; retirement simply does not participate in that protocol. [Restriction admission][generation-admission]; [operation locks][operation-lock].

Inspected retirement tests cover approved proposal withdrawal, no-proposal refusal, no-host refusal, artifact removal and containment. Their synthesizer returns a fixed hash and does not replace the registration during a human wait. This is test-definition inspection, not an assertion that those tests were executed or that no other relevant test exists. [Retirement test definitions][retire-tests].

## 3. Production route and an independent replacement producer

### Retirement is a real TUI action

`/tool retire <name>` schedules `retireTool`, which creates a session operation context and calls the orchestrator with the shared approval broker and VirtualStore's executor-facing registry. It returns an error on refusal/failure or a withdrawal message on success. The command does not automatically retire proposals merely because the report lists them. [Command][command]; [TUI closure][tui].

`ToolValueReport` evaluates the current ledger and registered generated-tool list. `RetireGeneratedTool` refuses if no `retire` proposal matches the requested name or the synthesizer cannot revoke. It reads the loop under the orchestrator lock, releases that lock, snapshots the runtime hash and builds the subject. No orchestrator or tool-operation lock remains held across `Await`. Waiting without an execution lock is desirable; the missing operation is safe reacquisition and current-state validation afterward. [Snapshot/report][report]; [retirement sequence][retire-admission].

### Dream-driven generation can outlive foreground input

The DreamRouter routes only confirmed, not-already-persisted learnings. A tool-need learning obtains its name from `metadata["tool_name"]` or inference, records a kernel fact, and attempts a nonblocking enqueue. Successful enqueue is conditional on an attached queue with capacity; a kernel-only result is **not** proof that generation ran. [Producer filtering][dream-filter]; [tool-need queue][dream-producer].

Factory boot installs a 16-element queue, connects it to the DreamRouter and starts a goroutine consuming needs on the Cortex's Ouroboros lifetime. Each dequeued need preserves its name and description in an autopoiesis `ToolNeed`, then calls the same orchestrator's `ExecuteOuroborosLoop`. That background lifetime is independent of a retirement command's wait; Cortex shutdown cancels it. TUI boot adopts that Cortex orchestrator, queue and approval broker. Therefore a previously admitted/enqueued generation can still be producing a replacement while a retirement approval waits. [Factory wiring and consumer][dream-consumer]; [shared TUI components][shared-boot].

The full loop can reject, reuse another tool, fail compilation, halt or be canceled; queue receipt does not guarantee publication. Same-name replacement itself is intentional: `duplicateCandidates` expressly omits the name being regenerated, calling it a deliberate new version. A different existing tool can still cause reuse. The witness requires actual successful publication with changed binary bytes, not an assumed model response. [Same-name rule][same-name]; [full-loop reuse and bounds][loop-entry].

Publication sanitizes the name, obtains the existing per-tool operation lock, snapshots prior source/test/binary/identity files, writes and compiles, then calls `RuntimeRegistry.Register`. Successful compiled registration verifies the binary, atomically writes its identity file and replaces `identities[name]` and `tools[name]`; failed write/compile/register paths attempt snapshot rollback. No duplicate-name refusal prevents the intended replacement. [Publication and rollback][commit]; [registration][register].

There is also a code-supplied action route: `VirtualStore.handleGenerateTool` forwards the requested target and code to `GenerateToolFromCode`, which performs its applicable checks and uses the same commit path. This is corroborating reachability, not a claim that any caller is authorized to invoke it. `CompileTool` is not used as the production witness here; a source helper's existence alone would be insufficient. [VirtualStore route][vs-generate]; [code-supplied generation][from-code].

**Foreground counterexample:** stepping aside from the approval modal does not by itself permit a second `/tool generate` to run concurrently. Normal Enter while loading goes through `submitWhileBusy`, which queues slash commands. The witness uses independently owned background generation, not a second foreground slash or a claim that the foreground slot is generally broken. [Modal step-aside][aside]; [Enter routing][keys]; [busy slash queue][busy].

## 4. Exact H1 → H2 sequence

Assume a normal compiled generated tool `T` is registered with hash `H1`, a valid retirement proposal exists, a person-capable broker is attached, and all journal/file operations in the final withdrawal succeed. Use one Cortex/loop and one completed replacement; no cross-process coordination, hostile path swap, simultaneous file write or policy change is necessary.

1. Retirement evaluates the proposal for `T`, reads `H1`, and opens a request whose subject contains `Tool=T`, `ToolVersion=H1`, action `/retire_generated_tool`, target `T`, workspace and proposal reasons.
2. While that request waits, the independently admitted dream-generation job for `T` finishes its normal pipeline. It publishes a replacement with **`H2 != H1`**, using the per-tool publication lock, then releases it. The replacement is current in the runtime registry. The retirement request has not been superseded by another retirement request.
3. The person approves the original request before expiry. Retirement passes the unchanged H1 subject into `Reserve`. The broker's digest comparison succeeds because it compares that supplied snapshot with the same approved snapshot; it does not inspect the tool registry.
4. `DispatchStarted` is recorded. `RevokeTool(T)` calls `RuntimeRegistry.Revoke(T)`, which locks the registry and resolves **current** `tools[T]`, now H2. It validates artifact containment, removes those binary/identity paths, then deletes the current runtime and identity entries.
5. If the executor-facing registry holds `T`, retirement unregisters it by name, finishes the grant and marks the name's ledger retired. Under these healthy-operation assumptions the caller can report success, even though the reviewed H1 revision was not the revision resolved for withdrawal.

The registry mutex prevents concurrent map mutation during `Revoke`; it does not make an approval taken earlier apply to a new registration. The established operation lock cannot prevent step 2 during the human wait, nor should it hold that wait hostage. A proper post-wait conditional mutation is required. [Name-resolved revocation][revoke]; [name-resolved session removal][session-unregister].

A second, narrower window remains after a hypothetical fix that merely rereads H1 before `Reserve`: another publication can complete **after the reread but before name-based `Revoke`**. Correctness requires a compare-and-revoke operation, not just a more recent comparison. This is the familiar check/use problem described by [CWE-367](https://cwe.mitre.org/data/definitions/367.html), used here as a design classification rather than proof of exploitability or severity.

## 5. Controls, scope and partial failure

The following protections remain valid:

- No matching generated-tool retirement proposal means refusal; built-ins are proposed only for flagging, not this retirement path.
- No broker, no host in nonautonomous mode, rejection, expiration and cancellation while waiting prevent this dispatch. Autonomous approval is an existing standing-instruction policy, not a forged human event; the long human-wait witness does not depend on it.
- The broker binds request/decision digest and intent, accepts one reservation, and requires a durable dispatch-start transition before the effect. Rebuilding a genuinely changed subject would make `Reserve` reject it. A new awaiting request for the same intent with a changed digest can supersede the old one.
- Canonical-name and compiled-directory checks constrain artifact deletion; the witness stays inside those boundaries and makes no arbitrary-file deletion claim.
- Compiled execution already acquires the per-tool operation lock and checks the requested identity against current registration. Preserve that execution-side control and snapshot rollback during publication. [Broker wait/open][await]; [reservation/dispatch][reserve]; [execution identity fence][execution].

Revocation itself removes artifacts sequentially, then deletes map entries. An I/O error can leave partial effects; it is returned and passed to the broker's unknown-outcome transition, not treated as “nothing happened.” That transition also depends on successful journal persistence. The later session-registry operation may fail after runtime removal. Do not promise transactionality across files, multiple registries, ledger persistence and journal simply by adding a lock. Likewise, removal does not prove that an already-started process was killed, every other session stopped offering the name, or source code was erased: source is intentionally retained. Those are separate retirement-scope and recovery contracts.

Identity also needs more than a display name or a universally assumed nonempty binary hash. Current source-only registration explicitly clears `rt.Hash`. A robust design distinguishes **name**, **registration incarnation**, **content revision** and **owning workspace/runtime**. The demonstrated compiled case deliberately uses different H1/H2 values; it does not rely on hash collision, missing hashes or metadata-only changes. [Source-only registration][register].

## 6. Repair direction: conditional retirement owned by the runtime

1. **Capture a retirement identity.** Resolve an immutable host-owned identity for the active registration, with an incarnation/revision that works for compiled and source-only tools. Include the relevant workspace, protocol/artifact ownership and policy reasons in the request contract. Decide explicitly whether recreating identical bytes is the same approved object; a name/hash pair alone cannot express every incarnation policy.
2. **Wait without mutation locks.** Keep the current broker/host workflow, expiry, rejection and cancellation semantics. No registry-wide or tool-operation lock should be held throughout human review.
3. **Acquire the tool's ownership boundary after approval.** Re-resolve the current registration and rebuild the subject while publication is excluded. Revalidate current proposal/admission requirements. A changed identity should supersede/refuse the old approval; re-review is not automatic permission for H2.
4. **Reserve and conditionally revoke that identity.** Have the owning backend accept an expected identity, compare it against current state, and remove only that identity within one linearizable mutation boundary. Hold the relevant publication exclusion through the comparison and artifact/map transition, or use an equivalent generation-aware protocol. Establish a lock order for operation, registry and broker locks to avoid inversion; preserve durable `DispatchStarted` before destructive effects.
5. **Propagate revision-aware cleanup.** Session-catalog withdrawal and ledger retirement must not later erase or label a newly published incarnation by name alone. If cleanup spans components, carry the retired identity and track pending/failed settlement; do not replay revocation indiscriminately after an unknown outcome.
6. **Declare the supported coordination domain.** A per-loop in-memory lock coordinates only participants that use it. Cross-process writers and external file replacement need additional ownership or immutable/versioned artifacts; registry atomicity does not supply filesystem compare-and-delete. Define in-flight execution behavior separately, such as draining existing users and denying new ones, rather than implying deletion terminates work.

HTTP's `If-Match` provides a useful conditional-mutation analogy: state-changing requests must not apply when the current strong validator fails the precondition. This proposal does not require HTTP or claim RFC conformance. [RFC 9110 §13.1.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1). A backend operation should have a clearly identified point at which the expected registration is atomically claimed for retirement, following the reasoning discipline of [Herlihy and Wing's linearizability definition](https://www.cs.cmu.edu/~wing/publications/HerlihyWing90.pdf); that does not make all subsequent external effects atomic.

## 7. Costs and unrun evaluation

No efficiency improvement is measured. Identity resolution can be cheap if it reuses trustworthy registration metadata; rehashing full binaries costs O(binary bytes) I/O and CPU. Persisted incarnation IDs require migration and restore semantics. Revision-aware catalogs and ledger settlement add schema/API work, memory, diagnostics and reconciliation. Extra broker journal transitions and failures add storage/sync work. Revalidation can produce more stale-request refusals and human re-review. These are costs of preserving the same consent boundary, not evidence of worse task intelligence.

Extending per-tool exclusion can delay generation and execution, especially if retirement waits for running users or performs slow filesystem cleanup. Preserve unrelated-tool concurrency and context-aware waits; a global lock is a useful conservative comparison, not a free solution. Avoid performing model judgments or human review while holding publication locks. Rejected stale retirements may leave tools available and require another review; repeat generation can waste model tokens, compile CPU and wall time. Measure all of it, including failed attempts and cleanup.

All evaluations below are **proposed and unrun**. Compare baseline, subject-reread-only, and conditional-revocation candidates using the same policy/ledger, binary revisions, broker mode, source outputs and admitted generation work. Use deterministic barriers and scripted generation before any live-model study.

| Fixture | Required evidence |
|---|---|
| H1 request; H2 fully publishes during human wait; approve H1 | Capture request subject/digest, current registration, actual deleted artifacts and grant outcome. Fixed path preserves H2 and reports stale admission. |
| H2 publication attempts between post-review check and delete | Demonstrate why reread-only is insufficient; conditional mutation either retires H1 before publication or refuses against H2, never deletes H2 under H1 approval. |
| Confirmed DreamRouter need and real factory consumer | Show enqueue versus kernel-only disposition, same-loop ownership, successful normal pipeline publication and TUI retirement coupling. Do not manufacture a foreground slash overlap. |
| Stable H1 positive control | Exact approved retirement succeeds; no unnecessary refusal, orphan registry entry or lost ledger settlement. |
| Existing safety controls | No proposal, built-in, no host, reject, expired/canceled wait, changed supplied subject, duplicate approval/resume, journal failure, containment refusal and generation rejection. |
| Broader identity/lifecycle controls | Source-only tools, identical bytes/new incarnation, metadata/protocol change, recreation after prior retirement, active execution, same/different tool names and late catalog synchronization. |
| Partial failure and recovery | Deny individual artifact removals, catalog retraction and ledger persistence; restart after dispatch starts; verify no claim of no effect and no unconditional destructive retry. |

Report stale-approved-revision withdrawals, stale refusals, correct stable retirements, duplicate dispatches, orphan artifacts/identities, cleanup completeness, source retention and reopened state. Measure writer/operation lock hold and wait, blocked execution time, hash bytes, journal/file operations, memory, CPU, cancellation-to-settlement, human review time, generation/model calls, tokens and total verified-task cost. A zero-retirement implementation is not a correctness win; success must include authorized unchanged-revision withdrawal. Cross-process and platform-specific filesystem evaluation should be labeled separately from the single-loop contract.

**Acceptance:** an approval for registration H1 cannot withdraw H2 merely because H2 now owns the same name; a stable approved registration remains retireable; failure and cleanup state stay explicit; existing broker, generation, execution and containment protections continue to hold.

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/agents.md
[tools-spec]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/Docs/architecture/tools/IMPLEMENTED_SPEC.md#15a-tool-value-does-each-tool-earn-its-place-as-built-2026-10-28
[approval-audit]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/Docs/audits/request-bound-human-approval.md#as-built-2026-10-27
[generation-admission]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/generation_admission.go#L13-L70
[operation-lock]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/tool_operation_lock.go#L9-L100
[retire-tests]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/tool_retirement_test.go#L14-L158
[command]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/cmd/nerd/chat/commands_handlers.go#L889-L908
[tui]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/cmd/nerd/chat/tool_value.go#L37-L56
[report]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/tool_retirement.go#L40-L74
[retire-admission]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/tool_retirement.go#L128-L222
[revoke]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/tool_retirement.go#L76-L126
[reserve]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/approval/broker.go#L470-L563
[await]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/approval/broker.go#L140-L272
[dream-filter]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/core/dream_router.go#L77-L114
[dream-producer]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/core/dream_router.go#L185-L254
[dream-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/system/factory.go#L2808-L2888
[shared-boot]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/cmd/nerd/chat/session_shared_boot.go#L108-L332
[same-name]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/tool_jev.go#L101-L120
[loop-entry]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/ouroboros.go#L355-L449
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/ouroboros.go#L850-L981
[register]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/runtime_registry.go#L63-L171
[vs-generate]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/core/virtual_store_workflows.go#L283-L330
[from-code]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/ouroboros.go#L1342-L1423
[aside]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/cmd/nerd/chat/approval_host.go#L121-L185
[keys]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/cmd/nerd/chat/model_key_handler.go#L253-L265
[busy]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/cmd/nerd/chat/side_turn.go#L352-L380
[session-unregister]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/core/tool_registry.go#L305-L345
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/bed7cde961483023ac106cc2c9b922c78f6ec83a/internal/autopoiesis/autopoiesis_profiles.go#L87-L130
