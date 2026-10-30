---
doc-class: deep-dive
subsystem: session-execution-safety
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 8d373fb6ca05ef03fec59b923c5f1925066d623a
verification-method: source-inspection-only
supersedes: []
---

# Recoverable delete admission must prove the current bytes

**Decision:** preserve automatic deletion of genuinely recoverable files, but stop treating an empty Git status as sufficient evidence that Git can restore the file's current bytes. An index entry marked `assume-unchanged` can hide a modified working-tree file from the exact status command used by the host. The resulting `file_recoverable` fact can satisfy the deletion permission rule without opening the existing human-approval request.

This is a bounded admission-classification finding, not a claim that all deletion lacks protection or that the deleted contents are necessarily lost. The ordinary session path captures a preimage, the tool itself reads the contents before removing the file, and conditional restoration mechanisms exist. Those safeguards matter; they do not make the Git index contain the modified bytes or satisfy the specific evidence contract used to waive approval.

**Evidence status:** source inspection only at the pinned revision. The scenario below is an unexecuted, conditional fixture, not a reproduced incident. No CodeNERD run, Git command, test, inference, training, or implementation was performed. No runtime Git version or frequency of this state was established.

## Scope and predecessor

The existing [request-bound human approval audit][approval-prior] deliberately preserves the tracked-and-clean deletion exception and documents the now-shipped request broker. This proposal does not reopen its former missing-host finding or recommend prompting for every delete. It examines the host measurement that decides whether that broker is needed.

The implementation explicitly promises that files with uncommitted work remain behind approval and calls its predicate a byte-for-byte recovery check. Its two measurements are a nonempty `git ls-files -- <path>` result and an empty `git status --porcelain -- <path>` result. There is no comparison between the current file contents and the recoverable object in that helper. [Recovery contract and implementation][recovery]

Current guidance requires exact current-state authorization and preserves mandatory executive gates. The proposal strengthens the evidence supplied to the existing policy rather than teaching the model to decide which files are safe to delete. [Core guidance][core-guidance]

The main audit inventory and research branch `research-main-20261030-0715` were screened for overlap. Targeted searches found no existing `assume-unchanged` or `skip-worktree` discussion. This is a bounded novelty check, not a claim that every historical document was exhaustively inspected. The recovery helper is identical on the inspected research and cartographer heads; the four other active lane/QA heads retain an identical `gitRestoresExactly` function. No competing fix to this measurement was found in those checks.

## Concrete conditional witness

Use a normal workspace containing an ordinary tracked regular file, `notes.txt`.

1. HEAD and the index contain bytes **A**. There is no staged change, merge conflict, path alias, symlink, submodule, or unusual path syntax.
2. The index entry has its `assume-unchanged` bit set. The existing working-tree file contains different bytes **B**. The bit remains set during admission, and there is no concurrent writer.
3. The file existed before the turn. The same-turn-created-file exception therefore does not explain its admission.
4. An otherwise permitted coding task reaches the ordinary session tool loop with `delete_file` in its effective capability set and proposes `{"path":"notes.txt","confirmed":true}`.
5. Safety checking is enabled; the kernel and Dreamer are usable; the file is neither secret nor protected; no read-only restriction or confirm-first obligation applies, and no human approval has been granted for this call. All independent write, effect, containment, and Dreamer checks permit this particular request. Reading and removing the file succeed.

Under these conditions, Git can name the tracked entry while reporting no change for the path. The host converts that observation into `file_recoverable("notes.txt")`. The constitutional rule can then permit the delete, so the denied-and-approvable branch does not ask the person. The normal handler can remove **B**, although the index contains **A**, not **B**.

The `confirmed:true` argument is an acknowledgement required by the tool schema and supplied in a model tool call; it is not the trusted broker's human decision. A false or missing acknowledgement still blocks this handler. No model behavior is predicted here: the finding begins with an actual proposed call satisfying the stated preconditions. [Tool declaration and handler][delete-handler]

## Why Git status does not establish the claimed proof

Git documents `assume-unchanged` as permission to omit checking a working-tree path and assume it matches the index. The user is expected to clear the bit before relying on change detection. The flag may also be set through `core.ignorestat` workflows; its presence need not originate in a CodeNERD operation. This performance promise is not permission to delete otherwise uncommitted contents. [Git index documentation][git-doc]

The upstream Git 2.52 source provides an immutable reference for the mechanism:

- Status collects working-tree changes through `run_diff_files`. [Status collector][git-status]
- That function skips up-to-date and skip-worktree entries and treats `CE_VALID`, the assume-unchanged flag, as unchanged without inspecting the file for modification. [Diff implementation][git-diff]
- Index refresh also returns an assume-unchanged entry without checking its working-tree contents under the ordinary flag handling. [Index refresh][git-refresh]

These references establish supported Git behavior, not the version installed in any user's environment. The witness selects an operation and state where the flag remains effective. It does not claim that every Git operation ignores such changes. In particular, skip-worktree has distinct sparse-checkout semantics and is not needed for the primary witness.

## Production path and evidence consumption

### The normal executor reaches this decision

The TUI delegates work through `spawnTaskWithContext` and the observed task executor. Depending on policy, the JIT executor uses a task-local inline executor or a subagent; this is not a test-only helper. The coder capability policy includes `delete_file`, but the effective per-request allowlist still has to admit it. [TUI delegation][tui-delegation], [task execution][task-execution], [coder capability][coder-capability]

Native and Piggyback tool batches share `executeAndRecordToolCall`. It normalizes arguments, runs the write guard, records a preimage, and calls `executeToolCall`. [Shared tool path][tool-accounting]

### A host observation becomes permission

`recoverableDeleteFact` invokes `gitRestoresExactly` only for the delete action, with a separate fallback for paths recorded as created by the turn. The safety check asserts the resulting fact for the admission and arranges its retraction. [Fact producer][recovery-fact], [admission assertion][fact-assertion]

The policy requires the pending delete, matching target recoverability, no dangerous target or payload, no restriction, and no outstanding consent gate. Once this rule derives permission, the human-approval predicate's denial condition does not hold. The executor queries the exact action, target, and payload; it only waits for human approval when the verdict is denied and approvable. This is wrong evidence entering an otherwise intentional rule, not a broad action-level consent grant. [Constitution][constitution], [exact permission query][exact-permission], [approval dispatch][approval-dispatch]

### Independent checks and the actual effect remain ordinary

The request still passes executive preflight. Missing or canceled contexts, a missing Dreamer, or an unsafe simulation can stop it. The inspected delete projection emits deletion and critical-path facts; it does not independently compare the current bytes with an index object. The registry also enforces project write protection. The witness explicitly requires these checks to allow, rather than asserting they can be bypassed. [Interactive preflight][preflight], [delete projection][dream-projection], [registry guard][registry-guard]

The modular handler resolves the workspace path, rejects directories, reads the current contents, performs `os.Remove`, and records the changed source. It does not revisit Git recoverability after the admission decision. [Actual deletion][delete-handler]

## Mitigations and limits that must survive the fix

**Preimages are real counterevidence to a data-loss claim.** Before ordinary dispatch, the session records each target's first-write preimage in `ExecutionResult.PreWriteContents`. Known before-state is also exported in observed write records. The delete handler separately reads **B** before removal and refuses if that read fails. [Snapshot][snapshot], [observed writes][observed-writes], [handler][delete-handler]

**Restoration paths exist.** Preimages distinguish absence, empty contents, and unreadable state, and record the turn's last written state to avoid overwriting a later writer during restoration. A repair episode leaving a Go workspace uncompilable can save the attempted patch and restore the turn's files. These are meaningful recovery controls, not evidence that every successful non-code deletion automatically rolls back. [Preimage semantics][preimage], [conditional restoration][restore]

**Other refusals remain effective.** Ordinary dirty status, untracked files, Git errors, and lexical workspace escape fail this helper. Explicit restrictions and confirm-first obligations remain in the constitutional arm. Effective capability, secret-path protections, project rules, executive preflight, acknowledgement, filesystem errors, and later verification retain their own roles. The finding does not imply arbitrary-path deletion, successful final completion, or an ability to forge human approval.

**Existing tests have useful controls.** The inspected recovery suite covers clean tracked files, ordinary modifications, untracked files, missing files, workspace escape, and non-repositories. The proposed flag fixture extends those cases rather than dismissing them. No tests were run. [Existing cases][recovery-tests]

## Proposed admission contract

A host may waive the human-approval obligation for an existing file only when it has a current, target-bound recovery witness for the bytes it is about to remove.

The witness should distinguish at least:

- **Git recovery proven:** a specified recoverable object and restoration representation reproduce the current bytes, with unambiguous path, stage, and relevant mode.
- **Recovery not proven:** the path is dirty, assumed unchanged, unsupported, ambiguous, unreadable, or changed since measurement. Use the existing human-approval route when policy permits it; do not invent permission.
- **Turn-created rollback:** the existing separate exception, supported by owned creation evidence.
- **Host backup proven:** only if a separate, explicitly designed durable recovery contract is adopted. A convenient in-memory snapshot must not silently be relabeled a Git proof.

Bind evidence to the exact canonical target and observed revision. Before removal, reject stale evidence or obtain a new decision; neither a pathname nor a cached clean result establishes current contents. Preserve literal path handling, workspace containment, current policy admission, and the existing broker's immutable request lifecycle.

### Implementation alternatives

**Conservative flag refusal first.** Read index flags and decline the automatic exception for assume-unchanged and other unsupported states. Reuse the existing approval path. This is a small, fail-closed correction for the demonstrated class, but may prompt for flagged files that actually still match. It is not a complete proof against every status blind spot.

**Direct recovery proof.** Compare current contents with a specifically identified recoverable representation rather than trusting status alone. Raw object bytes, checkout bytes, newline conversion, clean/smudge filters, file modes, symlinks, sparse entries, and merge stages need an explicit support contract. Unsupported transformations should require review, not speculative conversion. Do not run arbitrary repository filters merely to establish safety.

**Durable quarantine.** Preserve the exact bytes in a host-owned recovery store before deletion and authorize against that separate capability. This can support legitimate dirty-file removal, but adds crash consistency, retention, restore conflicts, access control, and storage policy. It is a larger product decision, not a necessary repair to the Git witness.

Do not silently clear flags, refresh or rewrite the index just to obtain a cleaner-looking status. Such mutations alter the user's Git state; a stat refresh alone is not a byte-recovery receipt.

## Costs and operational tradeoffs

The current helper performs two Git subprocesses per checked delete. A flag check adds another process unless incorporated into the tracking query. Direct comparison adds object retrieval and work proportional to file size, with latency, memory, filesystem, and cancellation costs that matter for large files and bulk cleanup. Streaming comparison can bound memory; byte hashing still consumes I/O and CPU.

The ordinary tool path already reads a preimage, and the handler reads contents again. Reusing a snapshot could avoid redundant reads only if the snapshot is bound to the exact dispatch revision; the first-write preimage is not automatically the current revision after intervening edits. Avoid authorization caches without a complete identity and invalidation contract.

The flag-only option shifts cost to operator review and can interrupt legitimate cleanup. A direct proof reduces those unnecessary prompts but has more engineering and compatibility surface. Quarantine adds storage, retention cleanup, crash-recovery work, and copies of potentially sensitive contents. None of these measurements inherently requires a model call or external API spend. Additional human-review or agent-recovery turns may still affect end-to-end tokens and elapsed time.

A useful rollout records proof class and bounded failure reasons, not file contents in ordinary logs. Measure added host cost and operator interruption alongside classification accuracy. Do not claim performance or safety improvements before evaluation.

## Fair evaluation plan

All evaluations below are proposed and unrun. Compare the current implementation, conservative flag refusal, and direct-proof design on identical workspaces and fixed Git versions; include a no-automatic-delete control to expose the cost of merely denying everything.

| Fixture | Required observation |
|---|---|
| Ordinary clean tracked file | Preserve automatic admission when the declared recovery contract is satisfied. |
| Ordinary modified file | Existing human-approval requirement remains. |
| Assume-unchanged with index A and worktree B | No Git-proven automatic admission; record the reason, regardless of a captured preimage. |
| Assume-unchanged with equal bytes | Flag-only design may ask; direct-proof design may admit only with a valid receipt. Report the difference as deliberate conservatism. |
| Staged, untracked, missing, non-repository, Git/read failure | Fail closed; no recovery fact inferred from missing evidence. |
| Skip-worktree and sparse-checkout variants | Record actual flag lifecycle and inclusion state; do not assume equivalence to the primary fixture. |
| CRLF, attributes, filters, binary/large files, modes, symlinks, submodules, conflicts | Explicit supported behavior or review; never an undocumented claim of exact restoration. |
| File or index changes after measurement | Stale proof does not authorize new contents; later writers are not overwritten by recovery. |
| Read-only, confirm-first, protected/secret path, denied capability, unsafe or missing Dreamer, cancellation | Independent gates remain effective. |
| Same-turn-created file and exact human approval | Preserve legitimate separate authorization paths and their receipts. |
| Preimage available; later build restoration succeeds or fails | Report recovery independently of the original admission classification. |

At the real session boundary, capture whether approval was requested, the permission witness, the actual target revision, whether removal occurred, and the bytes the claimed recovery source can reconstruct. Do not infer authorization correctness from a successful tool return or final assistant prose.

Report false automatic admissions, unnecessary prompts, legitimate deletions completed, exact recovery success, stale-proof refusal, cancellation latency, subprocess count, bytes read, peak memory, and operator time. For any subsequent end-to-end comparison, hold task, model, budgets, workspace state, and independent safety policy constant. Separate source-predicted behavior from observed outcomes and publish failures as well as improvements.

## Acceptance boundary

The first milestone is narrow: a flagged, modified tracked file must no longer receive the Git-recoverable exception solely because porcelain is empty, while truly recoverable deletion and all independent safeguards continue to work. A snapshot or later successful recovery may reduce harm; it must not hide which admission premise was false.

[approval-prior]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/request-bound-human-approval.md#L203-L212
[recovery]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/delete_recoverable.go#L13-L84
[core-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/agents.md
[git-doc]: https://github.com/git/git/blob/9a2fb147f2c61d0cab52c883e7e26f5b7948e3ed/Documentation/git-update-index.adoc#L337-L363
[git-status]: https://github.com/git/git/blob/9a2fb147f2c61d0cab52c883e7e26f5b7948e3ed/wt-status.c#L615-L638
[git-diff]: https://github.com/git/git/blob/9a2fb147f2c61d0cab52c883e7e26f5b7948e3ed/diff-lib.c#L229-L245
[git-refresh]: https://github.com/git/git/blob/9a2fb147f2c61d0cab52c883e7e26f5b7948e3ed/read-cache.c#L1348-L1359
[tui-delegation]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/delegation.go#L96-L105
[task-execution]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/task_executor.go#L389-L449
[coder-capability]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/intent_routing_rules.mg#L353-L361
[tool-accounting]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/executor_tools.go#L1021-L1068
[recovery-fact]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/delete_recoverable.go#L114-L159
[fact-assertion]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/executor_tools.go#L3101-L3116
[constitution]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/constitution.mg#L65-L104
[exact-permission]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/executor_tools.go#L3136-L3198
[approval-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/executor_tools.go#L2568-L2588
[preflight]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_interactive_gate.go#L159-L205
[dream-projection]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/dreamer.go#L388-L432
[registry-guard]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_write_guard.go#L58-L98
[delete-handler]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/tools/core/file_ops.go#L599-L684
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/executor_tools.go#L1149-L1174
[observed-writes]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/turn_writes.go#L12-L33
[preimage]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/preimage.go#L13-L41
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/buildable_tree.go#L30-L74
[recovery-tests]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/delete_recoverable_test.go#L55-L128

