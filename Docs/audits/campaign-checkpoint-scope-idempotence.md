# Campaign checkpoint remediation scope idempotence

**Status:** source-derived reliability finding and design proposal, not an observed incident or an executed regression. All implementation links are pinned to main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked on 2026-10-30. No application execution, tests, model inference, or implementation were performed.

## Conclusion

A repeated checkpoint failure can broaden a campaign's remediation scope merely because an earlier remediation task exists. The scope helper treats every write-set entry as a file and takes its parent directory. It then stores those directories in the next remediation task's write set. On a later failure, the helper reads that task alongside the original tasks and takes the parent of the already-derived directory.

This is a scope-representation and retry-composition gap. A small repair neighborhood can become a larger snapshot and lease footprint without new target evidence. In a workspace where the larger ancestor exceeds the snapshot limit, the next remediation can fail before its worker runs, even though the intended neighborhood fits.

The repair should preserve the deliberately useful expansion from a file to its containing directory. It should make that expansion stable across retries, rather than applying it again to directory scopes. This is not a proposal to weaken snapshot limits, bypass permissions, or treat the inferred write set as the user's complete authorization.

## The existing capability is intentional

Checkpoint remediation does real useful work. Below the checkpoint-failure cap, the policy can derive `/replan`; the orchestrator adds a file-modification task carrying the failed checkpoint's findings and phase objectives. The phase remains open and is checked again. The implementation replaced a replan that did not carry the findings into actionable work. [Remediation construction][append] [Policy][policy]

The scope deliberately changed from exact declared files to their containing directories. A checkpoint can name a neighboring document that the original tasks omitted, and the remediation should be able to repair that document. The source comment describes this rationale, and the existing integration test expects two declared documents under `Docs` to produce a `Docs` directory scope, excluding campaign reports under `.nerd`. That behavior is not the defect and must remain. [Scope rationale][scope] [Existing neighbor-expansion test][neighbor-test]

Directory write sets are also supported independently of remediation. Snapshot handling recognizes directories, and an existing test exercises a successful directory-scoped modification. Consequently, an already-directory-scoped original task can encounter the same file-versus-directory mismatch on its first checkpoint remediation. [Directory snapshot branch][capture-directory] [Directory task test][directory-test]

## Exact production mechanism

The helper `phaseWriteScope` normalizes each entry, excludes `.nerd`, and unconditionally calculates `path.Dir(file)`. It visits the write sets and artifacts of **every** task in the phase. There is no distinction between an original file target and a directory returned by a previous call. [Scope helper][scope]

`appendCheckpointRemediation` calls that helper before appending a new task, then stores its result unchanged as the task's `WriteSet`. The task is `/file_modify`, with the same planned effect, and remains in the phase after completion. Thus the output of one scope derivation becomes an input to the next derivation. [Task insertion][append]

For a phase with one original file target, the source transformation is:

| Phase contents at scope derivation | Resulting remediation scope |
|---|---|
| Original task writes `docs/component/a.md` | `docs/component` |
| Original task plus the completed first remediation, whose write set is `docs/component` | `docs/component` and `docs` |
| Further below-cap failures with further appended remediation tasks | Can add higher ancestors, subject to later workspace normalization |

The table omits the absolute workspace prefix for readability. The implementation normalizes to absolute paths before taking their parent. Sorting and exact deduplication do not remove this semantic widening: `docs/component` and `docs` are distinct strings.

The loop reaches the next checkpoint only when no task is active and the kernel derives the phase's tasks done. Quality remediation can intervene before checkpoint evaluation. When the checkpoint fails, the policy chooses recheck, remediation, or closure as unverified. Defaults allow three failed checkpoints and enable remediation, so a second failed checkpoint can append the widened task after the first remediation completes. [Production checkpoint loop][phase-loop] [Defaults][defaults]

## A bounded witness with a concrete consequence

Consider an otherwise admitted workspace and phase with these conditions:

1. The original completed task declares only `docs/component/a.md`; no unrelated artifact broadens the initial scope.
2. The component directory contains readable, workspace-admitted regular files small enough for the snapshot limits. The wider `docs` tree contains more than 5,000 such files because of sibling components. The files can all be small; no 256 MiB assumption is needed.
3. The first checkpoint fails, normal fact synchronization and remediation insertion succeed, and the policy permits remediation.
4. The first remediation completes successfully within `docs/component`. Its checkpoint still finds an unmet requirement in that same component; no new broader target is required.
5. The second failure remains below the configured cap, and no cancellation, risk refusal, lease conflict, or other earlier guard prevents scheduling the new task.

The second task's derived write set now includes `docs`. The orchestrator resolves that set for its lease and for mutation snapshot capture. The transaction wrapper captures the snapshot before invoking its task-execution callback. When a resolved path is a directory, it recursively records admitted regular files. [Write-set consumer][resolve] [Lease acquisition][lease] [Pre-execution capture][pre-capture] [Directory capture][capture-directory]

The snapshot helper fails closed after more than 5,000 regular files or more than 256 MiB. In this witness, unrelated sibling files make the wider directory exceed the file-count limit, even though the original component still fits. The wrapper returns a capture error before the remediation worker is invoked. [Snapshot bounds][snapshot-bounds]

The snapshot refusal itself is correct for the scope supplied to it. The upstream scope derivation is what unnecessarily placed the larger tree inside that transaction. Increasing the limit would mask the representation error and increase resource exposure.

This witness does not require a malicious model, concurrent foreign writer, false checkpoint pass, or out-of-workspace path. It is conditional on the stated ordinary execution and failure sequence; no runtime reproduction or failure frequency is claimed.

## What this does and does not establish

**Task failure is visible and recoverable in principle.** The normal task-failure handler records the capture error as a failed attempt and asks the kernel for retry, replan, or failure. A one-shot attempt-cap replan may replace the problematic task and recover. The source therefore does not establish an inevitably blocked campaign, an infinite loop, or permanent inability to repair. [Failure recording and next move][failure] [Replanning control][replan]

**The rejected task has not called its worker.** Costs at this point include host snapshot work and failed-attempt handling, not an automatic worker-model charge for that rejected execution. Earlier checkpoint or remediation calls already occurred under the witness, and later replanning may incur model work; those costs must be attributed separately.

**The write set is not the complete authorization boundary.** It provides coordination and rollback scope. Actual writes pass through the existing write guard, which checks mission scope when applicable and dynamically acquires leases for actual paths. Constitutional and tool permissions remain separate. A broader inferred write set does not by itself prove a permission bypass or unauthorized edit. [Write-time guard][write-guard]

**Workspace containment is not removed.** If a root directory is treated as a file, the helper can form a parent outside the workspace. However, the task's resolved write set is normalized again before lease acquisition and snapshot capture, and outside entries are dropped there. The lock manager separately rejects outside paths when supplied directly. The main witness stays within the workspace and does not depend on this adjacent behavior. [Resolution][resolve] [Normalization and containment][normalization] [Manager refusal][outside-refusal]

**Existing safety controls should remain.** Checkpoint caps and the `/unverified` terminal disposition prevent a failed checkpoint from becoming a successful phase. Turning remediation off selects recheck rather than this append path; a cap of one prevents the repeated-remediation witness. Cancellation stops scheduling and drains active owners. Snapshot bounds, observed write handling, rollback, and subsequent session and campaign verification remain useful defenses. [Checkpoint decisions][phase-loop] [Cancellation handling][cancellation] [Snapshot wrapper][pre-capture]

A widened lease covers a larger area, but this review does not claim that the witness necessarily blocks a concurrently running sibling task: the checkpoint path itself waits for the current phase's tasks to finish. Lease footprint is an observable consequence and evaluation metric, not a demonstrated concurrency incident.

## Proposed contract

The key invariant is:

> Without newly admitted scope evidence, adding a remediation task that carries the already-derived repair neighborhood must not enlarge that neighborhood.

Represent a scope entry as a file target, directory scope, or another explicitly supported kind. Apply file-to-parent expansion once. Preserve a directory as that directory. Do not infer a path's kind solely from its extension or from whether it happens to exist at one moment: new, removed, renamed, globbed, and inaccessible targets need explicit handling.

The design should retain the reason for each scope entry, such as an original task target, a previously admitted repair neighborhood, or an explicit expansion supported by new findings. Host code measures canonical paths, filesystem kind when available, and membership. The kernel can decide whether new evidence justifies expanding repair scope. A checkpoint's prose path should not itself become a broad write grant.

The minimal behavior must preserve all of the following:

- Exact files still widen to their containing directory so neighboring repairs remain possible.
- Existing directory scopes do not widen simply because they pass through the helper again.
- Repeated checkpoints over unchanged targets retain the same effective scope.
- Genuine newly admitted targets can expand scope with an explicit reason.
- Campaign reports under `.nerd` remain excluded from repair scope.
- Workspace containment, write-time authorization, snapshot admission, and failure visibility remain independent.

### Design choices

**Preserve typed scope entries.** Carry file-versus-directory intent through planning, task storage, refinement, and remediation. This offers the clearest general contract but adds schema and migration work. Model declarations describe planned intent; host observations and existing policy still govern execution.

**Record a phase repair neighborhood.** Derive the neighborhood from the original admitted targets once, then reuse it for later checkpoint repairs. New targets require a separate extension decision. This avoids feeding generated tasks back into the derivation, but the owner must distinguish actual new work from inherited scope, including after resume or replanning.

**A focused compatibility correction.** For existing paths, retain known directories rather than applying `Dir` again, and mark generated remediation scope as already normalized. This has a smaller implementation surface but needs clear behavior for missing paths and legacy tasks; filesystem existence alone cannot resolve every case.

Whichever design is chosen, canonicalize equivalent spellings consistently. Collapsing redundant descendant scopes under a parent can reduce duplicate work, but that is a performance optimization after correct semantic scope has been established. It must not silently widen scope.

Legacy journals should retain task history and existing receipts. A migration can mark ambiguous scope kinds and reconstruct the smallest supported neighborhood, or require explicit reconciliation when it cannot. It should not re-execute completed tasks merely to rediscover their target kinds.

## Costs and failure modes

A focused kind-preservation repair adds path-kind checks or a small amount of metadata. A typed design adds serialized fields, migration rules, validation, and reason receipts. Canonicalization and deduplication cost CPU; live filesystem checks add I/O and can fail or race with file changes. These costs should be bounded and reported rather than hidden in a heuristic fallback.

The potential benefit is avoided reads and memory allocation over unrelated subtrees, smaller lease footprints, fewer capture refusals, and fewer retry or replan rounds. These benefits are hypotheses, not measured savings. No new LLM call is inherently required for stable-scope derivation.

A repair that freezes only exact original files would regress the intended neighboring-file capability. A repair that freezes the neighborhood forever could prevent a legitimate new target from being addressed. A repair that treats every failed checkpoint as authority to expand broadly would retain the present problem. Scope extensions should therefore be explicit and inspectable.

Do not increase the snapshot cap, skip capture, proceed after partial capture, or loosen rollback to make the witness pass. A genuinely oversized intended scope should still fail closed or be split through an admitted plan refinement. The remedy is to select the correct transaction scope, not to make an unsafe transaction look executable.

## Fair evaluation plan

These are proposed evaluations; none were run.

Use deterministic checkpoint and worker doubles to exercise the actual `runPhase` path, task insertion, write-set resolution, lease acquisition, and snapshot wrapper. Compare current behavior, a focused directory-preservation correction, and a typed neighborhood design under matched conditions.

| Evaluation | Required distinction |
|---|---|
| One file and one failed checkpoint | Preserve current containing-directory scope and neighboring-file repair. |
| Completed first remediation and second failed checkpoint with unchanged targets | Effective scope remains stable; no ancestor appears solely from the generated task. |
| Original directory write set | First remediation retains the directory rather than its parent. |
| Small component inside a larger tree above the snapshot limit | Correctly scoped capture can proceed; genuinely oversized intended scope still refuses before worker dispatch. |
| Limits just below, at, and above 5,000 files and 256 MiB | Preserve the existing boundary and error reporting. |
| Relative and absolute aliases, Windows case normalization, root scope | Equivalent scope remains consistent and outside paths remain unavailable. |
| Missing, renamed, inaccessible, and globbed targets | Explicit kind or recovery disposition; no guessed broad parent. |
| Newly admitted target outside the prior neighborhood | Expansion remains possible with a recorded reason and unchanged permissions. |
| Remediation disabled, cap one, cap exhausted, or cancellation | Existing policy and lifecycle behavior remain authoritative. |
| Snapshot error followed by retry or replan | Failed attempt is visible; recovery can narrow scope without repeating completed side effects. |
| Resume with legacy or typed scope metadata | Stable interpretation and preserved history across process boundaries. |

Record effective scope entries, files and bytes read for capture, peak allocation, lease footprint and wait time, worker invocation counts, failed attempts, replan rounds, checkpoint calls, model usage by actual caller, wall time, and eventual authorized repair outcome. Measure host costs separately from model costs. Inspect phase status and independent verification; successful scope computation alone is not successful repair.

## Novelty and review boundary

The review covered root and campaign guidance, current campaign wiring, relevant source and existing tests, and audit additions through research branch 0948. The older [checkpoint build-profile audit][profile-audit] concerns verification configuration, not repeated scope derivation. The [recurse compensation audit][compensation-audit] owns foreign-write rollback and is not reopened here. The 0948 [artifact-role audit][artifact-audit] concerns input versus output classification, not directory scope fed back into checkpoint remediation.

Searches for `phaseWriteScope`, checkpoint scope widening, and remediation parent directories found no exact existing audit. This is a bounded novelty check, not proof that no differently worded discussion exists.

The exact remediation source file is byte-identical on the inspected session lane `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, store lane `a53cfc308e8836c72370adaa46deba3f7b00180d`, OAuth lane `aa7988dec4c62116656ea722ff73b687a8509908`, and QA heads `51cfd42649552866d4aa54c53b68acf5740eb09b` and `767a3291d6287b238417f2466592a9002810d05a`. No runtime result is inferred from that source comparison.

[append]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_checkpoint_remediation.go#L12-L96
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_checkpoint_remediation.go#L100-L134
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_decisions.mg#L252-L301
[neighbor-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint_phase_completion_test.go#L428-L464
[directory-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction_test.go#L540-L568
[phase-loop]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L153-L227
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/campaign.go#L164-L179
[resolve]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L569-L594
[lease]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L480-L510
[pre-capture]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L53-L107
[capture-directory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L250-L335
[snapshot-bounds]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L380-L426
[failure]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure.go#L24-L105
[replan]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure.go#L118-L147
[write-guard]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_attempt_writes.go#L137-L155
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/write_set_lock_manager.go#L259-L311
[outside-refusal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/write_set_lock_manager.go#L134-L179
[cancellation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L80-L138
[profile-audit]: https://github.com/theRebelliousNerd/codenerd/blob/07bdcb50873498d22fce498f599073df00ac5750/Docs/audits/campaign-checkpoint-build-profile.md
[compensation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/recurse-compensation-write-ownership.md
[artifact-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4d0c0c64ddc5da4b87587c1bec4368ca7c70edd5/Docs/audits/campaign-analysis-artifact-role-continuity.md

