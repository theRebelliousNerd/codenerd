---
doc-class: deep-dive
subsystem: campaign-execution-recovery
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: source-inspection-only
supersedes: []
---

# Preserve a deleted file's permission mode during campaign rollback

**Decision:** extend the campaign's existing recovery record to retain and restore the supported permission metadata of a pre-existing regular file. Do not replace its scoped rollback, write-time leases or failure gates. A rollback that recovers the bytes but changes an executable script into a non-executable file has not restored the file's operational state.

**Evidence status:** source-supported, conditional production-path finding at [main `388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391). No tests, application execution, inference, training, local Git or repository writes were performed. The fixture and evaluations below are proposed, not reproduced results.

## Finding and scope

The campaign snapshot stores a path, existence flag and content bytes, but no permission mode. Its rollback recreates a deleted pre-existing file using `os.WriteFile(..., 0644)`. On a normal POSIX filesystem, a previously executable regular file that remains absent at that point is restored without its execute bits, even if all rollback IO succeeds. The original task remains failed; the defect is incomplete compensation, not false task completion. [Snapshot representation][snapshot-type], [capture][capture], [restore][restore]

Deletion is load-bearing. Go's documented API preserves permissions when writing an existing file; the supplied permission argument applies when creating a missing file and is filtered by the process umask. Thus ordinary byte-only edits followed by in-place rollback need not lose executability. The primary witness chooses original mode `0755`, successful removal and an absent path at restoration. Creating with `0644` cannot recover those execute bits. Windows has a different permission model; the POSIX conclusion must not be presented as a Windows executable-bit incident. [Go WriteFile contract and implementation][go-write], [platform-specific mode contract][go-chmod]

This memo does not assert changes to ownership, ACLs, extended attributes, hard-link identity or symlinks. Those require separate support decisions. It also does not assert that all deletes are admitted, all failed tasks lose permissions, or recovery is permanently impossible: a trusted retained copy, Git restoration or a human can later repair the mode.

## Existing work and why this is a narrow continuation

- The campaign QA's **metadata-only modification** concern asks whether a legitimate `chmod` is falsely rejected by byte comparison. This finding instead starts with a real file deletion and asks what the already-running compensation reconstructs. It does not require a chmod tool or recommend counting arbitrary metadata changes as successful `/file_modify` work. [Campaign QA §26][campaign-qa]
- Earlier TransactionManager QA raised mode preservation, and current code provides `atomicfile.WriteFilePreservingMode` with a test for editing an existing executable. Credit that work. The helper reads the mode of the path as it exists **now** and uses a fallback if missing; merely substituting it with fallback `0644` would not recover the mode of a file already deleted. The campaign needs the earlier mode, not only a different writer. [Earlier core QA][core-qa], [existing helper][atomic], [mode-preservation test][atomic-test]
- The prior recurse compensation audit concerns **whose writes may be undone**. Current campaign code retains write-time leases and observed outside-snapshot writes. This proposal concerns **which properties are restored** for a file that is unambiguously within this attempt's captured scope. It does not reopen the former outer-diff ownership claim. [Prior ownership audit][ownership-prior], [current failed-attempt coordination][undo]
- The recoverable-delete audit concerns the evidence used to waive human approval, particularly Git index assumptions. This fixture can use an ordinary genuinely clean tracked file: no hidden worktree bytes, unusual index flags or erroneous recovery classification are needed. Correct deletion authorization does not supply the missing metadata to campaign rollback. [Prior delete audit][delete-prior]

Targeted searches of the main audit corpus and QA, plus recent research inventories through `research-main-20261030-1548`, found these predecessors rather than an exact campaign deleted-file mode-restoration audit. The latest branch contains documentation only. The five inspected active non-research heads all retain the same no-mode snapshot structure and `0644` restore statement, although their complete files are not identical to main:

| Branch | Inspected head |
|---|---|
| QA orchestrator | `51cfd42649552866d4aa54c53b68acf5740eb09b` |
| Store lane | `a53cfc308e8836c72370adaa46deba3f7b00180d` |
| Session lane | `80af1863d3a3aa46a722ac8b0c8a0985582d4995` |
| OAuth lane | `aa7988dec4c62116656ea722ff73b687a8509908` |
| QA cartographer | `767a3291d6287b238417f2466592a9002810d05a` |

This is a bounded source screen, not an exhaustive absence proof across every historical document.

## Production chain and concrete trigger

### Admission and worker route

1. A normal TUI campaign is constructed with the live task executor, host executor and workspace, and its background `Run` owns a cancelable lifecycle. [TUI construction][construction], [run startup][chat]
2. The phase worker's `runSingleTask` calls `executeTaskWithRollback`. Mutating task types, including `/file_modify`, require the snapshot; risk refusal or failed capture stops execution first. [Worker boundary][worker], [mutation types][mutation-types], [snapshot wrapper][wrapper]
3. Choose an accepted `/file_modify` task with an explicit existing regular-file target and write set, and an empty explicit shard field. That reaches the normal file handler. The task's coder persona can be granted `delete_file`, but registry/persona membership is not sufficient permission. The actual effective JIT catalog, constitutional/restriction policy, any required human approval, write guards and destructive-action Dreamer preflight still have to admit the exact call. [Task dispatch][dispatch], [file handler][file-handler], [coder capability][coder], [session gates][gates], [executive gate][preflight]
4. The registered delete handler requires `confirmed=true`, resolves containment, refuses directories, reads the preimage and then removes the file. An ordinary clean tracked workspace file can satisfy the separate recoverable-delete permission arm, subject to its remaining policy restrictions. A denied operation is a negative control, not this witness. [Deletion handler][delete-handler], [recovery measurement][recoverable], [permission rule][permission]
5. The observed executor subsequently returns a terminal error before any successful recreation. The verdict loop returns dispatch errors rather than requiring eventual `/done`; the campaign records returned writes and propagates the error. The `/file_modify` handler refuses its create-style fallback. This selects rollback without assuming model prose is accepted as success. [Verdict-loop error][verdict-loop], [observed handoff][observed], [fallback refusal][file-handler]

The deletion and terminal error are explicit conditional observations, not a prediction that a particular prompt makes the model delete a script or that a provider will fail at that point. A provider failure after an admitted tool effect is one possible sequence. An independent failure later in the attempt can also select compensation.

### Minimal unrun fixture

Use a synthetic, ordinary non-Go workspace on a POSIX filesystem:

- `scripts/example.sh` is a regular, tracked, genuinely clean file with known bytes and mode `0755`.
- It is not a test, verifier input, secret, protected project path or mission-protected target. Use an ordinary campaign, not a mission/recurse wrapper.
- The task's exact write set captures this file successfully; storage and directory permissions permit deletion and recreation. There is no concurrent writer, symlink substitution or external chmod.
- The actual admitted coder call removes the file; the task then returns a terminal error. The path is still absent when campaign rollback reaches it.
- No other recovery restores it first. In particular, this fixture has no root `go.mod` or `go.work`, so the separate session `leaveBuildableTree` restoration is not entered. [Build-recovery limit][buildable]

The predicted state sequence is:

| Boundary | Bytes/existence | Ordinary permission bits |
|---|---|---|
| Before attempt | Original bytes, present | `0755` |
| Snapshot | Bytes and existence retained | Not represented |
| Admitted deletion | Absent | No live file |
| Failed attempt's snapshot restore | Original bytes recreated | `0644` filtered by umask, not `0755` |
| Task result | Failure still reported | No separate mode-restoration failure detected |

A later direct invocation relying on execute permission can consequently fail until repaired. This report does not claim that such an invocation was run, that every downstream build calls the script directly, or that invoking it through an interpreter necessarily fails.

## Why existing compensation does not close this boundary

`undoFailedAttempt` first separates observed writes outside the snapshot, undoes those when their content still matches the attempt's result, then restores the snapshot and releases the attempt's leases last. A captured target belongs to the snapshot path even if its deletion was correctly observed. There is no second mode-aware recovery on that branch. [Failed-attempt ordering][undo]

Both exact-file and directory capture inspect filesystem information but retain only bytes/existence. The restore reports IO failures; if `os.WriteFile` succeeds with the wrong permission state, there is no post-restore mode comparison to turn that into a recovery error. [Exact capture][capture], [directory capture][directory], [restore][restore]

The observed outside-snapshot representation also defines `FileState` as content/existence and uses `0644` for recreation. That is an adjacent schema consideration for a repair, not a separate production incident established here. Preserve its unknown-preimage and changed-since-write refusals rather than replacing them with unconditional metadata restoration. [Observed state][file-state], [outside-snapshot undo][outside]

Existing tests explicitly delete captured `0644` files and assert their original bytes return. They are valuable compensation controls, but those fixtures do not distinguish correct metadata restoration from creation using the same default mode. No tests were executed in this review. [Existing deletion restoration test][restore-test]

## Safeguards and nonclaims

- The task is not marked completed on this failure path. The scheduler receives the original error and may retry, replan or stop under its existing policy. No universal false-success or infinite-retry claim follows.
- Snapshot/risk failures prevent the attempt; file membership, path checks, write leases, restrictions, authorization and Dreamer remain required. The proposal grants no new deletion permission.
- No concurrent edit is necessary. Conversely, campaign leases are not atomic ownership over every external editor; adding mode fields alone would not make the whole filesystem transaction serializable.
- Normal in-place writes can preserve mode. The finding must not be stated as “every `0644` write removes executable bits.”
- A worker that never deletes, successfully recreates the original mode before failure, or is denied before dispatch does not meet the primary conditions.
- Existing content recovery, sibling-status isolation and failure diagnostics still work. The specific missed observation is the recreated file's mode.
- The project already has a mode-aware atomic writer for existing paths. The recommended work should reuse its mechanisms where suitable, not invent another unrelated file-writing stack.
- The lower-level rollback is in-memory, not a newly claimed crash-safe filesystem transaction. Broader crash recovery, ACL preservation and ownership semantics remain separate requirements.

## Proposed contract and integration

**Capture an explicit supported recovery state.** For each captured regular file, retain its known permission mode alongside bytes and existence. State the platform contract: initially ordinary permission bits for regular files, not automatic restoration of privileged bits, ACLs, ownership or arbitrary filesystem object types. Unsupported or unavailable metadata must be labeled unknown, not silently replaced by a universal default.

**Restore the captured state, not the current default.** For an owned missing file, recreate the captured bytes and supported mode through the existing atomic-file machinery where applicable. For an existing path, preserve the established ownership/conflict policy and verify both supported content and metadata afterward. A mode assignment can fail independently of the data write; aggregate/report partial recovery rather than declaring the file restored solely because bytes match.

Do not simply change `0644` to `0755`, guess from a filename, or use `WriteFilePreservingMode(path, ..., 0644)` after the path has disappeared. Do not recover a working-tree file from Git without proving Git represents the intended before-state. Do not restore another writer's current metadata over a conflict.

**Carry the same semantics at effect boundaries.** If observed writes are extended for outside-snapshot compensation, add explicit metadata-known/version fields and preserve legacy unknown states. A content-only receipt cannot retroactively prove mode. Keep per-attempt identities, first-before/last-after aggregation and lease release ordering. This is not permission to reinterpret every historical serialized return as a complete file backup.

**Publish recovery evidence.** Go measures capture, write, mode application and final comparison outcomes; Mangle derives whether recovery is complete, requires inspection or holds more work. Keep the original failure distinct from a new recovery failure. The operator should receive paths, supported properties restored and unresolved properties without dumping file contents into ordinary logs.

**Preserve work semantics.** This repair should not relax the intentional `/file_modify` modify-not-delete rule or make a chmod-only operation satisfy a task that owes a content change. The earlier QA's metadata-only task semantics are a separate contract decision. Update the owning campaign/recovery specification before implementation.

## Alternatives and complete costs

| Option | Advantage | Limitation |
|---|---|---|
| Current bytes-only restore plus operator repair | No implementation cost | A failed attempt can leave a functioning script unusable; diagnosis and repair are externalized. |
| Preserve mode only when target still exists | Small, already available helper | Does not solve the deleted-file witness. |
| Add captured mode to the campaign snapshot | Narrowest direct repair | Must handle mode-write failure, post-restore checking and platform semantics; does not automatically fix outside-snapshot receipts. |
| Versioned file-state envelope shared with observed writes | Consistent producer/consumer recovery | Broader schema, migration, retention and compatibility work; avoid expanding the supported-property promise without evidence. |
| Dedicated attempt worktree with conflict-aware promotion | Stronger isolation for several ownership problems | Much larger workspace/index/context integration and disk cost; not required merely to preserve a captured file's mode. |

The narrow capture path already obtains file information, so it may reuse that observation rather than add a separate stat for every captured file. Restoring and verifying metadata adds host filesystem operations and possible failure states. Atomic replacement adds temporary-file, rename, sync and storage overhead where used; directory snapshots already retain file contents, so a mode field is small compared with existing bytes but should still be measured at scale.

No extra model or embedding call is inherently needed. Reduced recovery turns or failed script invocations may save tokens and human time, but no savings are measured. Record added local IO, latency, memory and persistence bytes as well as avoided retries, model calls and operator work. Do not hide conservative holds or legitimate work prevented by a deny-only alternative.

## Fair evaluation — proposed, not run

Use a deterministic observed executor at the normal TUI campaign boundary, while keeping real policy, tool registry, write guards, snapshot and rollback wiring. Where necessary, control the post-delete terminal error with a barrier or injected transport failure rather than depending on a model mistake. First establish actual tool admission and deletion; a fake callback that never traverses the production handler is only a unit control.

Compare current behavior, existing-path mode preservation, and captured-mode restoration with the same task, permissions, filesystem and failure point.

| Fixture | Required observation |
|---|---|
| Captured `0755` regular script, admitted delete, terminal error, absent at restore | Exact bytes and supported original mode recovered, or explicit incomplete recovery; not default-mode success. |
| Original `0644` file deleted | Preserve current byte recovery and intended mode. |
| Original `0755` file modified without deletion | Preserve execute bits; no regression to ordinary in-place rollback. |
| Exact file versus directory snapshot | Both capture paths obey the same metadata contract. |
| Admitted tool effect then canceled/erroring return | Capture partial effects and compensate without requiring `/done`. |
| Tool denied, missing confirmation, unsafe Dreamer, unknown preimage | No new authority; no invented recovery evidence. |
| Mode application or atomic publication fails | Original failure retained, recovery partial state reported and policy hold evaluated. |
| External content/mode change before restore | Do not advertise atomic CAS; obey declared conflict policy and report unresolved ownership. |
| Legacy content-only observed receipt | Unknown metadata remains unknown; no guessed historical mode. |
| Windows and other supported filesystems | Test declared semantics; do not reuse a POSIX executable-bit assertion blindly. |
| Repeated attempts and later legitimate success | No spurious mode churn; failure status, retry budgets and completion gates unchanged. |
| Unsupported symlink/ACL/owner/privileged-bit cases | Explicit exclusion or separately validated behavior, not an implicit full-metadata guarantee. |

Measure bytes and mode before/after, actual deletion, task status, rollback error/receipt, lease lifetime, stale/conflict refusal, retries, host IO/latency/memory, model calls/tokens and operator intervention. A separately authorized behavioral fixture may directly execute a harmless synthetic script to verify operational recovery; no such execution was performed here.

**Acceptance boundary:** on the supported regular-file path, an admitted failed campaign attempt either restores the captured bytes and permission mode or explicitly reports incomplete recovery. Successful byte restoration alone must not conceal an execute-permission regression.

[snapshot-type]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L21-L50
[capture]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L305-L349
[directory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L385-L423
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L489-L504
[wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L52-L108
[worker]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L415-L477
[mutation-types]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_mutation_types.go#L3-L20
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L84-L139
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L90-L153
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L208-L248
[file-handler]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L574-L625
[observed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L58-L112
[verdict-loop]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/verdict_loop.go#L119-L127
[coder]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/intent_routing_rules.mg#L353-L361
[gates]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L2474-L2506
[preflight]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L2568-L2719
[delete-handler]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/core/file_ops.go#L622-L684
[recoverable]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/delete_recoverable.go#L50-L84
[permission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/constitution.mg#L86-L104
[buildable]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/buildable_tree.go#L30-L47
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_attempt_writes.go#L158-L183
[outside]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_attempt_writes.go#L186-L237
[file-state]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/observation/subagent.go#L192-L207
[restore-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction_test.go#L1053-L1083
[campaign-qa]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/2026-10-20_00-20-45_EST.md#L375-L381
[core-qa]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/core/2026-05-04_04-07-20-EST_transaction_manager_boundary_analysis.md#L337-L345
[atomic]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomicfile/atomicfile.go#L226-L246
[atomic-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/transaction_atomic_test.go#L20-L50
[ownership-prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/recurse-compensation-write-ownership.md
[delete-prior]: https://github.com/theRebelliousNerd/codenerd/blob/136e02f0a38897a40931747b1e56a6f0c14feb19/Docs/audits/recoverable-delete-index-assumptions.md
[go-write]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/os/file.go#L934-L948
[go-chmod]: https://pkg.go.dev/os#Chmod

