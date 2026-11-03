# Campaign finalization must preserve declared artifacts and acceptance validity

## Finding and scope

**A campaign can pass its declared acceptance command and then move a required, explicitly declared root artifact out of the accepted workspace before marking itself completed.** The completion sweep interprets the task's resolved write set as a collection of basenames. A nonempty directory write set prevents that resolver from consulting the task's artifact declarations. Consequently, a new root artifact can be classified as undeclared even when the task expressly lists it and legitimately produced it.

This is a source-derived, conditional finalization defect, not a reproduced failure or a claim that all campaigns lose their outputs. The strongest witness needs neither concurrent foreign writes nor a falsely successful worker: an ordinary `/file_modify` task really changes existing code, creates a declared configuration file, passes the applicable gates, and passes acceptance. Successful cleanup then changes the file location that acceptance depended on. The bytes remain in the campaign artifact directory; the concern is delivery at the required path and the applicability of the completion evidence, not proven permanent data loss.

Source revision: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), reconfirmed as main on 2026-10-30. This review used source inspection only. No application, command, test, model inference, or training was run. The evaluation below is proposed, not reported as passing.

## Existing design and prior coverage

Root cleanup has a legitimate purpose. The implementation deliberately waits until tasks have finished because inferred write sets can be wrong and later tasks may need earlier output. Its documented scope excludes nested files precisely because those files may be the intended work. That rationale should survive a repair; disabling all cleanup would trade this defect for the known scratch-file problem. [Completion rationale](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_execution.go#L215-L228), [documented root-only scope](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/campaign/WIRING-AND-NOT-BUILT.md#L144-L149).

The existing [recurse compensation audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/recurse-compensation-write-ownership.md) addresses authorship and compensation boundaries; this finding does not require another writer or misattributed work. The [campaign terminal publication audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-terminal-publication.md) concerns terminal delivery and status ownership, not whether finalization preserves an accepted artifact. The newer [deletion microcheckpoint audit](https://github.com/theRebelliousNerd/codenerd/blob/9ac1e81ed2b43d6e8f78af8ee55e2c384aa3de08/Docs/audits/campaign-deletion-microcheckpoint.md) concerns rejection of an otherwise legitimate deletion. Here the task passes that checkpoint and cleanup subsequently relocates an output.

Guidance and research branches through `research-main-20261030-1049` were screened for overlap. Five active development and QA branch heads were also checked: `51cfd426`, `a53cfc30`, `80af1863`, `aa7988de`, and `767a3291`. Their surrounding files differ, but the inspected write-set resolver, cleanup function, and file-target resolver are identical; all retain acceptance before sweep before completion. This is a bounded comparison of the relevant functions, not a claim that the branches are otherwise equivalent or that every historical document was exhaustively searched.

## Concrete conditional witness

Use a small, readable workspace whose root basename is `project`:

- `loader.go` already exists and is included in the task snapshot.
- Root `config.json` does not exist when the campaign records its baseline.
- The admitted task is `/file_modify`, has no explicit shard override, declares artifacts in the order `loader.go`, `config.json`, and has `WriteSet: ["."]`.
- Its authorized instruction is to change the existing loader to consume the new root configuration. The worker actually changes `loader.go` and creates the correct `config.json`; it deletes no pre-existing file.
- No other task's resolved write-set entry has basename `config.json`. There are no glob entries or path aliases in the witness.
- The observed task return is genuinely `/done` without an error; risk, permission, lease, snapshot, task, phase, and cancellation controls allow completion.
- A declared acceptance command genuinely requires root `config.json` and passes. For example, an existing relevant test exercised by `go test -count=1 ./...` can require that file. This is a condition on the checker, not a claim that such a command was executed here.
- The root baseline and final root scan succeed, the artifact destination is available with no conflicting destination file, and the rename succeeds. Persistence must also succeed for a claim about a durable completed checkpoint.

The ordinary planning path preserves an explicit normalized write set and separately stores sanitized artifacts. It infers an artifact-backed write set only when the explicit set is empty. `.` resolves to the workspace root, and the workspace-boundary check accepts that root. A directory scope is therefore representable without granting any additional permission to execute the task. [Plan construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L460-L475), [artifact and write-set handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L539-L578), [normalization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/write_set_lock_manager.go#L259-L311), [boundary check](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/utils.go#L88-L107).

An important rejected witness is a pure `/file_create` task with only the root directory as its write set. Planning retypes it to `/file_modify` because the root already exists; creating only `config.json` would not satisfy the existing-file modification gate. The witness above starts as `/file_modify` and genuinely changes an existing file. It does not evade that safeguard. [Retyping](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L751-L793).

## Production path and mechanism

### Task execution can legitimately succeed

With no explicit shard override, the file task reaches the normal file handler. The first artifact selects `loader.go` as its target, while the task instruction carries the requested work. The host requires an observed executor, installs the task write guard, and uses `RunUntilVerified`; errors and unverified outcomes remain failures. [Dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L208-L248), [target selection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L478-L491), [observed execution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L31-L112).

The transaction wrapper snapshots and checks the attempt. For a directory write set, changing a pre-existing file under that directory satisfies the modification requirement; deletion and read errors still refuse it. Thus the real `loader.go` modification can satisfy the gate while the new configuration remains a legitimate additional output. The microcheckpoint also sees live declared or observed paths. None of this asserts that arbitrary generated work will pass the independent session gates. [Transaction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L52-L107), [directory modification contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L110-L175), [microcheckpoint](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/micro_checkpoint.go#L23-L56).

### Cleanup loses the artifact declaration

The cleanup consumer calls `resolveTaskWriteSet`. That resolver takes nonempty `WriteSet` first and consults artifacts only as a fallback. The sweep then inserts only `filepath.Base` of each resolved path into its declared-name map. For the witness, that name is `project`, not `loader.go` or `config.json`; it is not a directory-membership test or a union with the declared outputs. [Resolver](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L569-L595), [sweep classification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L844-L885).

`loader.go` survives because it was in the baseline. `config.json` is a new root non-directory entry and has no matching declared basename, so the sweep moves it to `.nerd/campaigns/<shortID>/artifacts/config.json`. With the stated successful-rename condition, preserving the bytes at that new path does not preserve a loader or acceptance check that requires the original path. The official Go API documents `Rename` as a move and notes platform-dependent restrictions; failure is a real control, not something this analysis assumes away universally. [Move and error handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L891-L924), [Go Rename contract](https://pkg.go.dev/os#Rename).

### Acceptance certifies the earlier state

After all phases are done, `Run` settles acceptance and handles remediation or blocking before cleanup. The acceptance runner executes the declared command in the workspace, checks cancellation, and records the result. Only afterward does `Run` sweep the root, set `StatusCompleted`, persist, and emit completion. There is no second acceptance invocation between that sweep and the completed return. [Acceptance execution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_acceptance.go#L149-L211), [finalization order](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_execution.go#L173-L235).

The resulting discrepancy is specific: a passing acceptance observation can remain the campaign's completion basis even though a later host-owned move removed a required input from its accepted location. It is not a claim that the checker misreported its own execution.

## Controls that narrow the finding

Existing root files are protected by the baseline. An exact `config.json` write-set entry protects the new file, as does artifact-derived inference when an explicit write set is absent. Another task with the same basename can also mask the witness. Nested files are outside this sweep. A missing baseline, missing campaign, blank workspace, or failed destination creation avoids the move; a rename error leaves that source in place and is logged. Acceptance failure and task refusal stop the successful route. [Baseline and scope](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L751-L773), [baseline persistence](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L807-L841).

Existing tests deliberately retain pre-existing files and an exact declared file while moving two new stray files and checking their preserved contents. They also cover a nil baseline and resumed baseline behavior. Those are valuable controls, not evidence against this directory-scope/artifact composition. The inspected cases do not establish final acceptance validity after this particular relocation. Tests were read, not run. [Sweep tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/root_write_report_test.go#L116-L199), [resume control](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/root_write_report_test.go#L280-L338).

## Proposed contract and bounded remedies

**Declared deliverables must survive finalization at their required locations, and completion evidence must apply to the post-finalization workspace.** Keep four concepts separate: declared output, coordination scope, observed write ownership, and authorized cleanup eligibility. A directory lock is not proof that every new child is disposable, nor permission to move it.

1. Preserve explicit artifacts independently of nonempty write sets. Canonicalize their workspace-relative identities and union their protection into cleanup decisions. This narrowly fixes the concrete witness without retaining all scratch by default.
2. Define file, directory, and pattern semantics instead of projecting everything to a basename. Decide explicitly whether a declared directory protects its descendants. Unknown or conflicting classification should retain the file and report unresolved cleanup, not silently classify it as scratch. Keep this separate from existing mutation permissions.
3. Finalize authorized cleanup before the final acceptance observation, or invalidate affected evidence when finalization changes its subject and obtain an authorized replacement observation. Do not blindly rerun arbitrary acceptance commands: they can cost time or have effects. If safe reconciliation is unavailable, publish an unresolved finalization state rather than reusing evidence for a changed obligation.
4. Record cleanup moves with source identity, version, destination, reason, and outcome. Make resume idempotent and conflict-aware; do not overwrite a newer source or destination during recovery. A historical artifact path or successful rename alone is not permission to restore over subsequent user work.

A conservative report-only mode is a useful rollout alternative. A blanket removal of cleanup, blanket protection of all new files, extension blacklist, or unconditionally repeated acceptance would each avoid part of the witness while introducing different costs or unsafe assumptions.

## Costs and failure modes

Artifact-union protection is primarily local bookkeeping: root enumeration and declaration indexing, with canonical path comparisons. Directory and glob handling adds matching complexity and platform-sensitive case, separator, and symlink questions. Provenance and version checks add filesystem reads; durable move receipts add checkpoint storage, schema migration, and recovery maintenance. Locks can reduce conflicting finalization but increase contention and do not automatically exclude external editors.

Moving cleanup before acceptance may require one final check rather than two, but changes what that check observes. Evidence invalidation can require additional subprocess work, latency, CPU, and potentially side effects. A bounded policy must account for unavailable checks, deadlines, and authorization rather than silently converting them into success. Report-only handling retains more scratch and creates review work. No new model, embedding, or training call is intrinsically needed; optional ambiguity clarification should be separately budgeted. These are engineering cost categories, not measured performance estimates.

## Fair evaluation plan

Compare current behavior, explicit-artifact union, cleanup-before-acceptance, snapshot-aware invalidation, and conservative hold using identical worker outputs, accepted permissions, check commands, and budgets. Inspect both the terminal status and the actual post-finalization file locations.

| Case | Required observation |
| --- | --- |
| Principal directory-scope witness | Existing file really changes; declared configuration remains usable at its required path; final completion does not rely solely on the earlier pass. |
| Pure create with root scope | Retyping and the existing-file modification refusal remain effective. |
| Exact file declaration or inferred artifact write set | Existing successful protection is retained without unnecessary reruns. |
| Pre-existing, nested, and genuine scratch files | Baseline and nested-scope controls remain; proven disposable root scratch can still be relocated with content preserved. |
| Root absolute path, dot, directory, pattern, and basename aliases | Canonical membership and declaration semantics are explicit; no authorization broadening follows from matching. |
| Failed acceptance, cancellation, unreadable baseline, mkdir or rename failure | No fabricated successful observation or unsafe recovery; retained files and unresolved outcomes are accurately reported. |
| Crash, resume, changed source, and destination conflict | Finalization is idempotent and does not overwrite intervening work; completed status requires the declared final-state contract. |

Include a normalizer-to-dispatch-to-finalization integration case, not just a direct helper invocation. The acceptance oracle must genuinely depend on the required root location; checking only the old passing round or the relocated bytes would miss the defect. Measure false relocation of required outputs, retained junk, final-state acceptance validity, false completed outcomes, check count and latency, recovery behavior, and preserved bytes. Record unavailable or ambiguous outcomes separately. No evaluation result is claimed by this audit.
