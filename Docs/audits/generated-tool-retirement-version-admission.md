# Generated tool retirement must revalidate the approved version

## Finding and decision

Generated-tool retirement binds an approval request to a binary hash, but does not rebuild that subject after the approval wait. It passes the original subject back to the broker and then revokes the tool by name. A fully published replacement under that name can therefore be removed using approval bound to the previous binary.

The narrow repair is to make retirement a version-conditional lifecycle operation: after approval, establish the current authoritative registration and artifact identity, compare it with the reviewed identity, recheck the applicable retirement proposal, and withdraw only that admitted version under the publication ownership contract. A changed version requires a new decision or an explicit stale-request refusal. Do not turn retirement into an automatic response to a tool-value classification.

Source baseline: main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), checked October 30, 2026. This is a static, independently challenged source finding, not a reproduced incident. No application code, tests, model inference, shell commands, or repository writes were performed.

## Existing contract and the precise residual

The earlier [request-bound approval audit][approval-audit] already establishes that approval authorizes the operation reviewed, including its registration version, and calls for rebuilding current state on resume. Its implemented session path actually does that: it constructs a subject before `Await`, then calls `approvalSubject` again after the wait and passes the new value to `Reserve`. Preserve that existing implementation. [Session revalidation][session-approval]

The same audit explicitly defers the external-writer race **between a successful reservation and a later filesystem effect**. This report does not rediscover that general race. Its counterexample completes the replacement **before the human decides and before `Reserve` runs**. There is no writer racing the final deletion. The retirement caller supplies stale input to an otherwise functioning digest comparison.

The retirement method's own contract says it refuses when the approved operation changes between review and execution. The tools specification also describes approval of the exact request, compiled-artifact removal, session-registry withdrawal and retained source. The current implementation supplies the old hash to the request, but never checks whether it is still the hash of the object being removed. [Retirement contract and implementation][retire], [current tools specification][tools-spec]

Recent generated-tool audits address different boundaries: receipt retention, LearningStore multiwriter publication, automatic refinement proposal ownership, aggregate fact supersession, and feedback-only retry repeating Jev accounting. The last can affect a retirement proposal, but it does not cause or repair this defect. Here the initial proposal is legitimate, every accounting observation may be correct, and the person really approves; the removed version still differs from the approved version. [Recent Jev accounting audit][jev-audit], [refinement proposal audit][refinement-audit]

## Production path

### The user command reaches a real approval lifecycle

The TUI's `/tool retire <name>` branch creates an asynchronous command. `retireTool` obtains the session operation context, the current approval broker and the VirtualStore registry, and calls `Orchestrator.RetireGeneratedTool`. This is a live user-directed command, not a dormant helper. It does not run merely because `/tool value` prints a proposal. [Command dispatch][command], [command implementation][chat-retire]

`RetireGeneratedTool` first evaluates `ToolValueReport`, requires a matching `/retire` proposal, obtains a revoker, and reads `RuntimeTool.Hash`. It constructs a subject with workspace, backend, tool name, `ToolVersion`, action, target, reasons and write effect. It then creates a bounded sealed preview and waits for the broker. These are substantial controls. The problem is the next transition:

```text
version = currently registered hash
subject.ToolVersion = version
grant = Await(intent, subject, preview)
Reserve(intent, subject)             // original value, not newly observed state
DispatchStarted()
RevokeTool(name)                     // no expected version argument
```

The source contains no second runtime lookup, artifact digest check, or proposal evaluation between `Await` and revocation. [Exact caller sequence][retire]

### The broker compares what its caller supplies

`Broker.Await` documents that callers must reserve against the operation as it stands after the wait. `Grant.Reserve` computes the supplied subject's digest, checks request state, expiry and intent, compares that digest with the approved request, and atomically claims it in the journal. It does not independently know how to resolve a generated tool from its name. Passing the same unchanged value is expected to match. The `ToolVersion` field is part of the authority-bearing subject; this is not a missing hash field or a hash-collision claim. [Await contract][await], [reservation and dispatch journal][reserve], [subject representation][subject]

### Revocation resolves names and paths rather than an expected version

`OuroborosLoop.RevokeTool(name)` calls `RuntimeRegistry.Revoke(name)`. The registry holds its local mutex, looks up the named tool, validates its canonical name and compiled paths, removes each binary and identity sidecar, and deletes the in-memory registration. Neither API takes the approved identity. The path validator checks the configured name path and regular-file status; it does not compare the file's bytes with the approved binary digest. [Revoker][revoke], [artifact containment][containment]

After successful revocation, retirement unregisters that name from the supplied session registry and marks the tool ledger retired. These updates also use the name. They do not retroactively establish which version the person approved. Errors are handed to `grant.Finish`; the primary witness assumes all these operations succeed. It does not rely on partial deletion or error misclassification. [Post-revocation settlement][retire]

### Same-name replacement is intentionally supported

The duplicate checker explicitly excludes the tool being regenerated: a deliberate same-name generation is treated as a new version, not a duplicate of itself. Ordinary CLI generation reaches the full Ouroboros pipeline and checks that the resulting tool is registered. The witness conditions on a legitimate request resolving to the same canonical tool name and a replacement successfully passing the applicable generation, safety, adversarial, simulation and compilation gates; no particular model response or bypass is assumed. [Same-name semantics][same-name], [CLI generation][cli-generate], [generation wrapper][generate]

`commitTool` acquires a context-aware per-tool operation lock, snapshots existing artifacts, writes source, compiles, registers the result and cleans up rollback copies after success. The compiler writes the binary to the stable name-derived compiled path. `RuntimeRegistry.Register` verifies the resulting binary identity, atomically writes its sidecar, and publishes the registration. A successful replacement therefore gives the same pathname new binary bytes and a new hash. [Publication ownership and rollback][commit], [compiler output path][compile], [identity publication][register]

These publication controls must remain. They are not shared by retirement: `RevokeTool` does not acquire `toolOperations`. The keyed lock is also owned by one `OuroborosLoop`, not a cross-process artifact lock. A registry mutex protects that registry's maps and its deletion sequence; it cannot make a previously captured approval current. [Per-loop lock implementation][operation-lock], [revoker][revoke]

## Bounded source-derived witness

Use an ordinary compiled generated tool `T`, a canonical name, and a workspace with functioning stores and approval UI. Let its existing binary hash be `H1`, and let the current ledger legitimately derive a retirement proposal. Both independently initialized CodeNERD instances use the same configured tools directory. Start both before opening the approval request, so the example does not depend on another broker replaying pending approval records during boot. Only instance A owns this approval request.

| Step | Instance A | Instance B and durable artifacts |
|---|---|---|
| 1 | `/tool retire T` finds the proposal and creates a request containing `ToolVersion=H1`. It waits for a real human decision. | `T` is still the regular compiled artifact with hash `H1`. |
| 2 | The approval remains pending and unexpired. | Separately authorized same-name generation completes all required gates and publishes `T` with hash `H2 != H1` at the normal compiled path, with its matching identity sidecar. The replacement is fully finished. |
| 3 | The human approves the pending request bound to `H1`. `Reserve` receives the unchanged subject containing `H1`, so the broker's digest comparison succeeds. | No further writer runs; `H2` is stable. |
| 4 | `RevokeTool(T)` validates the canonical regular-file path and removes the bytes and sidecar now at that path. The local registration is withdrawn and the ledger is marked retired. | The removed binary is `H2`, not the reviewed `H1`. The source remains. |

Require the decision to arrive before expiry, the operation context to remain live, the broker journal and filesystem operations to succeed, and no running process or OS sharing rule to prevent deletion. This can be an ordinary removable regular artifact; no symlink, corrupt sidecar, malicious name, forged approval, crash, unsupported capability, or tight check-to-unlink timing is needed. Generation is conditional on success, not asserted to succeed for every request.

The two-instance schedule is useful because it does not depend on whether one TUI admits simultaneous generation and retirement commands. In-process callers sharing a loop have the same stale-subject seam if a supported concurrent publication completes while approval waits, but that broader frontend schedule is not required for the finding.

The supported consequence is **wrong-version withdrawal of the compiled artifact**. It is not source-code loss, arbitrary-path deletion, unauthorized code execution, or a claim that every session immediately forgets the tool. Instance B may retain its own registration until its ordinary lifecycle updates it. A subsequent generated execution still verifies the binary and refuses a missing or changed artifact; that fail-closed check limits execution impact but cannot restore the deleted approved replacement. [Binary verification and pre-start refusal][binary-check]

The witness also does not require `H2` to deserve a better tool-value classification than `H1`. Even if the same name remains eligible for retirement, approval bound to one binary is not consent to another. If product policy instead intends name-level consent across future replacements, it needs an explicit preview and authority contract; that is not what the present version-bearing subject promises.

## Proposed admission and retirement contract

1. **Freeze a complete retirement identity.** Retain the relevant workspace, canonical tool name, registration identity, binary path, protocol and digest, together with the proposal evidence used for review. The current compiled-binary hash can be the initial version discriminator; source-only registrations need a separately specified identity rather than an empty hash silently meaning any version. The counterexample above is compiled-only.
2. **Wait without blocking independent work.** Preserve the existing approval broker and cancellation/expiry behavior. Do not hold a registry mutex, kernel lock, or tool publication lock across a human wait. Unrelated tools should remain usable.
3. **Acquire the short retirement ownership boundary after approval.** Coordinate with same-tool publication and execution. Reuse the context-aware keyed lock for in-process participants, but explicitly specify the cross-instance boundary for a shared tools directory. A lock local to one loop is not that boundary.
4. **Reobserve before reserving and deleting.** Resolve the authoritative current registration and artifact, compare them with the approved identity, and rebuild the subject supplied to `Reserve`. A stale in-memory lookup alone is insufficient in the two-instance witness: it can still say `H1` while the durable file and sidecar contain `H2`. Missing, inconsistent or unrecognized identity is a refusal or reconciliation state, not permission to delete by name. Re-evaluate proposal eligibility from the appropriate current evidence; do not confuse merely querying a stale process-local ledger again with observing all writers.
5. **Carry the admitted identity into the revoker.** An expected-identity operation should reject a replacement rather than re-resolve an unqualified name. Keep verification and mutation within the supported owner boundary. If the new subject differs, use the broker's superseded/stale disposition and obtain a new decision when appropriate; do not rewrite the old approved request in place.
6. **Settle the same version across owners.** Registry withdrawal, session-catalog removal and ledger retirement should identify the version actually withdrawn, so a newly registered replacement is not erased by a late name-only completion. Preserve journal dispatch/outcome semantics and expose partial failures. Do not announce a universal all-sessions withdrawal solely because one supplied registry was updated.

There are several implementation choices. A shared advisory per-tool lock can coordinate cooperating CodeNERD publishers and retirement, provided every relevant writer participates and ordering is consistent. Immutable version-specific artifacts plus a conditional active-registration transition can avoid deleting a newer version at a reused pathname and support later garbage collection. A single authoritative lifecycle owner is another option. These choices have different deployment and recovery costs; the report does not prescribe a new distributed service.

Atomic sidecar replacement is valuable but is not a version-conditional unlink of a separate binary. Likewise, a newly added hash read followed by an unlocked removal fixes the stable pre-reservation witness but does not close the already-disclosed post-check external-writer race. State the supported coordination/threat model. Arbitrary external writers require a stronger ownership or storage design, or an honest limitation; ordinary advisory locking does not constrain them.

Preserve the policy's distinction between proposals and actions, built-in refusal, real human or standing-policy authority, canonical compiled-path checks, binary validation, source retention, single-use claims, cancellation, rollback and normal generation gates. Do not weaken generation or widen retirement authority to make the path easier to complete. New explanatory model-facing behavior, if needed, belongs in the existing JIT/spec-first workflow.

## Compatibility and operating costs

Pending requests must not be silently upgraded from `H1` to `H2`. The safest rollout refuses stale or insufficiently identified requests and requires a fresh decision. Existing legacy/source-only registrations need explicit eligibility rules; there is no evidence that an empty historical hash identifies a particular source version. Preserve already completed approval history as history rather than reinterpreting it under the new contract.

Revalidation adds artifact/manifest reads, hashing, policy evaluation and synchronization. Hashing can be proportional to binary size; a trustworthy immutable manifest may reduce repeated work but creates its own integrity and publication obligations. Coordination can delay same-tool generation or execution, and cancellation-aware waiting and deadlock-free lock ordering need tests. A global lock would unnecessarily serialize unrelated tools.

Version-specific artifacts add disk usage, reference tracking, migration and garbage collection. A staged deletion or retirement journal adds recovery work and may retain files longer. Catalog and ledger versioning increases schema and API complexity. Requiring another approval after a genuine update adds human effort but prevents one stale approval from consuming newly generated work. No latency, token, dollar or incident-frequency estimate is established here.

Any evaluation must count replacement-generation cost, approval time, retirement/reconciliation work and later task repair—not only the cost of unlinking two files. A faster stale deletion is not a successful retirement. Conversely, an implementation that never retires anything is not an acceptable safety improvement.

## Evaluation plan not run

The existing retirement tests verify approved proposal withdrawal, no-proposal and no-host refusal, successful artifact removal, and containment. Their synthesizer returns a stable hash and records a name-only revoke; the removal fixture uses fixed artifact bytes. They are useful controls, not a stale-approval replacement test. Existing per-tool coordination tests cover publication/execution and canceled waiters, not retirement participating in that lifecycle. [Retirement tests][retire-tests], [operation-lock tests][lock-tests]

A separately authorized regression should traverse the actual TUI command/broker boundary, pause on a pending request, fully publish a harmless same-name replacement through the normal admitted pipeline, approve the original request, and inspect the artifact, sidecar, both registries, proposal state and approval journal. The repaired result should be a stale refusal with `H2` intact, not a completed retirement of `H2`. Use deterministic barriers, not sleeps that merely increase the chance of a race. A test callback that only changes an isolated mock hash is useful at the caller seam, but is not sufficient evidence for the durable two-instance boundary.

Cover at least:

- unchanged `H1` approval removes exactly the intended artifacts and keeps source;
- `H2` fully published before the decision; replacement before `Reserve`; replacement attempted after reservation under the chosen coordination contract;
- replacement by another instance while A's in-memory registry still reports `H1`;
- distinct-name publication proceeds; same-name waiters cancel promptly;
- missing, corrupt or mismatched identity; regular-path containment refusals; source-only compatibility;
- proposal disappears or changes while waiting; unchanged valid proposal with changed binary;
- rejection, expiry, cancellation, no host, journal failure and a second claim;
- deletion or registry-publication failure after a partial effect, with an honest recoverable outcome rather than a fabricated no-effect success;
- restart and artifact cleanup under the chosen version/ownership design, without replaying a completed approval.

Then compare paired normal tasks at equal tool capability and authority, including tool regeneration followed by user retirement. Measure wrong-version effects, false refusals, successful intended retirements, reapproval rate, hashing/read work, lock wait/cancellation latency, retained artifacts, recovery effort, total model/tool tokens and cost per verified completed task. Run current and proposed paths against identical state transitions. Report failures and canceled work as well as successes. No benchmark or live-model result is supplied by this source review.

## Repository status

Main and the audit queue were unchanged during this pass. The latest research tip, [`30684dc18e934f12e50db05035febe7692174692`][latest-research] (`research-main-20261030-1316`), adds four audit documents and is four commits ahead, zero behind main; it does not implement repairs. Earlier research inventories through that branch were screened for overlap. The three active code lanes and two QA heads were unchanged, and `internal/autopoiesis/tool_retirement.go` was byte-identical to main at each inspected tip (`a53cfc30`, `80af1863`, `aa7988de`, `51cfd426`, `767a3291`). No repair for this boundary was found in those public sources; unpublished work is outside this conclusion.

**Closure criterion:** a successful retirement removes the version authorized by the still-admissible request, or returns an explicit stale/unknown outcome without removing a replacement. The production command, broker, durable artifacts and affected registries must agree on that identity.

[approval-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/request-bound-human-approval.md
[session-approval]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/human_approval.go#L278-L328
[retire]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L131-L222
[tools-spec]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/tools/IMPLEMENTED_SPEC.md#L556-L578
[jev-audit]: https://github.com/theRebelliousNerd/codenerd/blob/30684dc18e934f12e50db05035febe7692174692/Docs/audits/generated-feedback-jev-accounting-idempotency.md
[refinement-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/generated-tool-refinement-candidate-continuity.md
[command]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L889-L908
[chat-retire]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/tool_value.go#L37-L58
[await]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/approval/broker.go#L140-L212
[reserve]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/approval/broker.go#L470-L560
[subject]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/approval/request.go#L101-L164
[revoke]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L76-L125
[containment]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/runtime_registry.go#L174-L203
[same-name]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_jev.go#L101-L121
[cli-generate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_advanced.go#L1035-L1102
[generate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_tools.go#L37-L66
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/ouroboros.go#L850-L981
[compile]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_compiler.go#L177-L237
[register]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/runtime_registry.go#L154-L171
[operation-lock]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_operation_lock.go#L9-L82
[binary-check]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/runtime_registry.go#L382-L448
[retire-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement_test.go#L14-L159
[lock-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_operation_lock_test.go#L17-L182
[latest-research]: https://github.com/theRebelliousNerd/codenerd/commit/30684dc18e934f12e50db05035febe7692174692
