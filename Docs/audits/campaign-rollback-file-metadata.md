---
doc-class: deep-dive
subsystem: campaign-recovery
implementation-status: proposed
last-verified: 2026-11-02
verified-against: 1d9874ef413bc28ed2742777519d11a0aabb24ea
verification-method: static-source-inspection
supersedes: []
---

# Campaign rollback file metadata fidelity

## Decision

Define campaign restoration as an explicit supported file-state contract, not just restoring bytes at a pathname. Capture the original regular-file permission bits before mutation, preserve the existing ownership and unknown-preimage protections, and report incomplete recovery when supported metadata cannot be restored. Handle symbolic links and other file types through explicit policies rather than treating every readable path as an interchangeable byte container.

The narrow verified gap is a failed attempt that deletes a pre-existing file covered by its campaign snapshot. The snapshot retains its path, existence and bytes, but not permissions. If the pathname remains absent when rollback runs, `os.WriteFile(..., 0644)` creates a new regular file. The contents can be restored and the rollback can return no additional error, while an originally executable file is no longer executable. A restrictive original mode can also become less restrictive under particular creation-mask and ACL conditions.

**Evidence status:** source inspection at main [1d9874e](https://github.com/theRebelliousNerd/codenerd/commit/1d9874ef413bc28ed2742777519d11a0aabb24ea), not an executed reproduction or measured incident. No tests, application execution, provider inference or benchmarks were run. No repository files were changed; this is a scratch-only audit. The finding is not a deletion-permission bypass, a claim of universal data loss, or a claim that every failed edit changes permissions.

## Existing work and what this report adds

This boundary is not wholly new:

- Campaign QA already asks whether metadata-only `chmod` counts as modification, and whether symlinks are captured as links or targets. It also proposes concurrent-permission and rollback-error coverage. Those are prior identified questions, not findings first discovered here. [Campaign QA 175–181](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/.quality_assurance/2026-10-20_00-20-45_EST.md#L175-L181), [242](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/.quality_assurance/2026-10-20_00-20-45_EST.md#L242), [375–380](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/.quality_assurance/2026-10-20_00-20-45_EST.md#L375-L380).
- Earlier core transaction QA discusses permission degradation, but its broad assertion that a constant mode passed to `os.WriteFile` strips execute bits from an ordinary existing-file overwrite is not correct under that API's contract. The meaningful campaign witness requires deletion/recreation or another change of the underlying object, not an unchanged existing file. [Core QA 338–345](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/.quality_assurance/core/2026-05-04_04-07-20-EST_transaction_manager_boundary_analysis.md#L338-L345), [Go WriteFile](https://pkg.go.dev/os#WriteFile).
- Existing campaign work repairs actual-write coverage, preserves writes whose resulting content no longer belongs to the attempt, distinguishes unknown from absent preimages, and keeps sibling task state intact. The separate [recurse compensation audit](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/Docs/audits/recurse-compensation-write-ownership.md#L53-L78) concerns who owns bytes selected for compensation. This report concerns what state a correctly selected restoration can reconstruct.
- The separate core transaction manager already uses `atomicfile.WriteFilePreservingMode` for apply and rollback. That is useful counterevidence against a repository-wide “all writes reset permissions” claim, not a repair for uncaptured campaign metadata. [transaction_manager.go 420–421](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/core/transaction_manager.go#L420-L421), [503–515](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/core/transaction_manager.go#L503-L515).

The actionable addition is an exact current production rollback route, a creation-specific POSIX witness, its surviving controls and a proposed end-to-end acceptance contract. It does not substitute for the broader earlier ownership or crash-recovery work.

## The actual campaign execution route

| Boundary | Verified behavior |
|---|---|
| Dispatch and lease | `runPhase` obtains a declared write-set lease before scheduling `runSingleTask`. The task runner releases it on return, calls `executeTaskWithRollback`, and routes any resulting error to task failure rather than completion. [orchestrator_tasks.go 275–292](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_tasks.go#L275-L292), [416–469](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_tasks.go#L416-L469). |
| Recovery wrapper | Mutating tasks enter `withTaskMutationSnapshot`; risk gating and snapshot capture precede `beginAttempt` and the callback. Callback errors and recovered panics invoke `undoFailedAttempt`. A successful callback is followed by the `file_modify` outcome check. [orchestrator_task_transaction.go 53–109](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L53-L109). |
| File task | `file_modify` selects `executeFileTask`, which builds a modify request and invokes `spawnTask`. A returned error for a modify task is propagated, not replaced with the create-style fallback. [orchestrator_task_handlers.go 260–262](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_handlers.go#L260-L262), [585–634](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_handlers.go#L585-L634). |
| Observed execution | `spawnTask` requires an observed task executor, attaches the campaign write guard and runs `RunUntilVerified` over `ExecuteObserved`. Returned write records are retained even when the attempt reports an error. [orchestrator_task_handlers.go 31–110](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_handlers.go#L31-L110). |
| Write boundary | The session write path consults its guard before capturing pre-write contents and dispatching the tool. `delete_file` itself requires `confirmed=true`, resolves workspace containment, refuses directories, reads the existing bytes and only then calls `os.Remove`. These are genuine prerequisites, not bypassed controls. [executor_tools.go 1030–1068](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/session/executor_tools.go#L1030-L1068), [file_ops.go 622–684](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/tools/core/file_ops.go#L622-L684). |
| Undo and release | Failed-attempt handling first conditionally undoes recorded writes outside the snapshot, then restores the snapshot, while retaining the additional write-time leases until cleanup returns. Rollback errors are appended to the original cause. [orchestrator_attempt_writes.go 163–185](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_attempt_writes.go#L163-L185). |

This establishes an ordinary campaign path, not a free-form shell assumption or a special slash-command concurrency scenario. Effective catalog permission, mission restrictions, required approval, write guards and the tool's confirmation contract must all permit the particular deletion. A denied operation does not produce this witness.

## What the preimage actually contains

The declared snapshot type is only:

```text
Path
Exists
Content
```

It has no mode, original file type, link destination, owner, ACL or extended-attribute fields. Exact-path capture calls `os.Stat` and `os.ReadFile`, but stores only existence and content. Directory capture includes admitted regular files and stores the same fields. [Snapshot type](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L21-L25), [exact capture 304–349](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L304-L349), [directory capture 385–426](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L385-L426).

The separate observed-write route is also deliberately content-shaped. Session `PreImage` distinguishes existence, content and unknown state, with last-write content/absence tracking; `withTurnWrites` converts it into `observation.FileState{Known, Exists, Content}`. Neither representation supplies the original mode. [preimage.go 13–42](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/session/preimage.go#L13-L42), [101–112](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/session/preimage.go#L101-L112), [turn_writes.go 19–44](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/session/turn_writes.go#L19-L44), [FileState](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/observation/subagent.go#L193-L207).

This is why simply changing the final write helper cannot recover all missing state: the value to restore was never carried through the contract.

## The narrow deletion and restoration witness

Assume a supported POSIX-style filesystem, an ordinary in-workspace regular file `run.sh` whose actual pre-attempt mode is `0755`, readable contents, a writable containing directory, and a `file_modify` task whose declared snapshot covers the path. Exclude symlinks, ACL complications and concurrent external writers from this primary witness.

1. Risk admission, lease acquisition and snapshot capture succeed. The snapshot records the original bytes and existence but not `0755`.
2. An authorized task operation deletes the file. This may be an admitted `delete_file` call satisfying its confirmation and host permission requirements; no permission bypass is assumed.
3. The task then returns an error, cancellation is propagated as an error, a panic is recovered by the wrapper, or a subsequent micro-checkpoint fails. For the simplest production trace, choose an ordinary error returned through `spawnTask` and `executeFileTask`. Require the file to remain absent until restoration.
4. `undoFailedAttempt` reaches `rollbackTaskExecutionSnapshot`. Because the file existed in the snapshot, rollback ensures the parent directory exists and calls `os.WriteFile(fs.Path, fs.Content, 0644)`.
5. If these operations and the remaining rollback work succeed, the bytes are restored and no additional rollback error is reported. The task's original failure is still returned. The newly created file has no execute bits from the requested creation mode, so direct execution no longer has the original permission state.

[Rollback write](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L489-L504), [error propagation](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_attempt_writes.go#L163-L185).

There is also an explicit deletion detector when the execution callback otherwise returns successfully: `validateFileModifyOutcome` rejects any deleted pre-existing snapshot file, even if another file changed. That is a useful guard, not the defect. Do not use a helper-level success-deletion fixture alone as proof of the whole production route: `executeFileTask` has additional target verification/fallback behavior before that final check. The error-after-deletion witness above avoids depending on those success branches. [Deletion check](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L139-L170), [file-task verification](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_handlers.go#L643-L683).

### Creation mode is conditional on the environment

Go documents that `WriteFile` applies its permission argument when creating a file; an existing file is truncated without changing its permissions. Under the ordinary POSIX creation-mask rule, the new permission bits are the requested bits with masked bits removed. Thus `0644` with umask `0022` yields `0644`, while umask `0077` yields `0600`. Neither restores the execute bits of `0755`. [Go WriteFile](https://pkg.go.dev/os#WriteFile), [POSIX open](https://man7.org/linux/man-pages/man3/open.3p.html).

For an original `0600` file, a confidentiality-relevant widening to `0644` requires conditions such as umask `0022`, no overriding default ACL behavior, no later corrective mode operation and successful recreation. With umask `0077`, this particular widening does not occur. Linux default ACL inheritance can change the creation calculation; it must be specified or excluded in a fixture. Actual access by another user additionally depends on parent-directory traversal and other controls. No such access was established here. [Linux umask and default ACL behavior](https://man7.org/linux/man-pages/man2/umask.2.html).

The primary conclusion is incomplete metadata restoration, not that all restored files become exactly `0644`, all private data becomes readable, or Windows has identical mode semantics.

## Controls and limits that must survive

**Existing-file overwrite is a positive control.** If a file retains its inode and mode throughout the failed edit, the current `os.WriteFile` restore generally preserves that mode. Conversely, if an attempt changed permissions while retaining the file, writing the original bytes does not restore those permissions. Metadata-only task success is a related contract question already raised by QA, not needed for the deletion witness.

**Owned content still matters.** Outside the declared snapshot, `undoAttemptWrites` refuses unknown before-state or unknown/mismatching after-state, names paths left in place and restores known existing preimages with another `os.WriteFile(..., 0o644)`. Its equality check covers existence and bytes, not metadata. Preserve that content-conflict protection; it is not an atomic filesystem compare-and-swap against unrelated external editors. The declared snapshot path has different restoration semantics and must not be falsely described as sharing every outside-write comparison. [orchestrator_attempt_writes.go 191–238](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_attempt_writes.go#L191-L238).

**Capture and rollback can fail visibly.** Snapshot read errors abort capture; directory snapshots have file/byte limits; membership refresh/admission is part of capture. The rollback returns filesystem failures rather than certifying success, and nonstructural rollback restores only the failing task's status instead of replacing sibling campaign state. This report's metadata witness assumes successful byte restoration, not ignored write failure. [Capture and bounds](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L250-L259), [385–426](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L385-L426), [rollback scope](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L519-L605).

**Symlink identity is a separate unresolved scope.** Exact-path capture uses `Stat`/content reading rather than storing link text; directory walking includes regular entries and skips symlink entries. These are not equivalent link-restoration contracts. If a previously captured link pathname were removed, writing its old readable bytes would not reconstruct the link. But the standard delete tool resolves its input to a contained target before removal, so this is not evidence that calling `delete_file` on a link simply unlinks that link. Do not convert this type-fidelity observation into an unverified sandbox escape. [Capture](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L304-L349), [regular-entry filter](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction.go#L393-L420), [workspace resolution](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/tools/workspace_guard.go#L212-L255).

**The atomic helper is not enough alone.** `WriteFilePreservingMode` reads the mode of the file that exists now; if it is missing it uses a fallback. Substituting it into campaign rollback without carrying the old mode would still lack the deleted file's original permissions. Its ordinary existing-file preservation tests are useful, but not proof of deletion-recreation fidelity. [atomicfile.go 226–246](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/atomicfile/atomicfile.go#L226-L246), [transaction_atomic_test.go 20–75](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/core/transaction_atomic_test.go#L20-L75).

## Proposed restoration contract

1. **Name the supported state.** At minimum for regular files: existence, bytes and intended permission bits, bound to the captured resource. State explicitly whether owner/group, ACLs, extended attributes, timestamps, hard-link relationships and special mode bits are preserved, unsupported or require operator recovery. Do not promise full filesystem restoration from a mode field alone.
2. **Capture before mutation.** Retain original metadata with the same owned preimage through declared snapshots and observed writes. Distinguish “metadata unknown” from a legitimate zero-valued field. A later `Stat` after deletion or replacement cannot recover it. Where content and metadata are sampled separately, detect or exclude concurrent object replacement rather than silently combining observations of different files.
3. **Restore supported state before reporting it restored.** Prefer a restrictive staging object, populate its bytes, apply supported metadata and publish through a reviewed platform-specific replacement path. Treat metadata-application failure as incomplete recovery. Avoid publishing a permissive file and narrowing it only afterward. Choosing a replacement primitive must account for symlink following, hard links, Windows sharing and ownership changes.
4. **Preserve mutation ownership.** Keep write leases through restoration and retain content-conflict/unknown-preimage holds. Extend the comparison contract if metadata-only concurrent changes are meaningful. A stat/recheck followed by replacement is not automatically atomic against an uncooperative writer.
5. **Separate recovery outcomes.** Report content restored, metadata restored, conflict held, unsupported object and IO failure distinctly, while retaining the original task failure. A retry must not silently treat a degraded restoration as a pristine baseline. The host supplies facts; policy decides whether mutation may continue.
6. **Keep crash durability separate.** Atomic publication of one file is not an all-files transaction, durable preimage storage, rollback after process death or durable directory publication. Those guarantees require an explicit protocol and corresponding costs.

Recovery of an already affected file requires trusted original metadata or operator intent. Re-running byte restoration cannot infer lost execute bits, owners or ACLs from content alone.

## Costs and fair evaluation

Capturing regular mode bits adds modest state per file and can reuse already obtained information, but reliable object binding may need extra filesystem operations. Rich metadata requires platform-specific APIs, permissions and portability decisions. Staging/atomic replacement adds IO, temporary space and cleanup; preserving ownership may fail without authority. Durable recovery adds journaling and synchronization costs. Longer restoration increases lease hold time and writer waiting. A conservative hold can require human intervention. None of these costs is offset by a measured performance or safety win in this review.

**All evaluations below are proposed and unrun.** Existing campaign deletion tests create `0644` files and assert restored bytes; they correctly cover deletion rejection but do not assert preservation of a different original mode. [Single deletion test](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction_test.go#L1053-L1082), [modified plus deleted test](https://github.com/theRebelliousNerd/codenerd/blob/1d9874ef413bc28ed2742777519d11a0aabb24ea/internal/campaign/orchestrator_task_transaction_test.go#L1085-L1132).

| Evaluation | Controls and acceptance evidence |
|---|---|
| Production failed attempt | Traverse campaign dispatch, real lease/snapshot/write-guard wiring and an observed task executor. Deterministically perform an authorized deletion, then return an error; inspect task failure, exact restored bytes and actual permission bits. A seam may replace model inference, but must not replace the wrapper being evaluated. |
| POSIX recreation | Establish actual `0755` before capture, then delete and fail. Require execute-bit restoration or an explicit incomplete-recovery result. Separately test `0600` with controlled umasks `0022` and `0077`, and state ACL conditions. Set/check modes explicitly so fixture creation umask cannot hide the premise. |
| Process-global mask isolation | Run mask-dependent cases in isolated processes or equivalent harness isolation rather than changing the shared test process umask while parallel tests execute. Record platform and filesystem assumptions. |
| Positive ordinary writes | Failed in-place byte edit with unchanged `0755`; successful authorized edit; `0644` deletion; empty pre-existing file; newly created file removed on failure. Preserve existing success and content-restoration behavior. |
| Error and interruption routes | Task error, propagated cancellation, recovered panic and checkpoint failure. Exercise snapshot-read failure and rollback write/metadata failure; do not count a refused deletion or absent restoration as a passing mode test. |
| Ownership and type limits | Changed foreign contents, metadata-only foreign change, unknown preimage, exact-path symlink and directory-contained symlink. Verify declared outcomes without assuming cooperative leases stop external processes. Do not use an external target or real secrets. |
| Scope and persistence | Verify additional leased writes outside the declared set as well as snapshot-covered files; keep sibling task status intact. Treat restart/crash recovery as a separate capability, not implied by an in-process pass. |
| Cost comparison | Same files, bytes, tool permissions, failure points and concurrency for current and proposed paths. Report metadata fidelity, recovery failures/holds, IO, CPU, memory, temporary space, elapsed time and lease waiting. Include no-mutation and successful-edit controls so refusing all work cannot appear superior. |

Acceptance is not “the bytes compare equal.” It is that the failed attempt restores each supported, owned preimage attribute or accurately reports why it could not, without weakening existing admission, conflict, unknown-state and sibling-isolation protections.

