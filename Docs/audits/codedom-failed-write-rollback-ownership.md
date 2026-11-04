---
doc-class: deep-dive
subsystem: codedom-failed-write-rollback
implementation-status: partial
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 43e42a89e5037670b49fc6f33f7555eb19a89950
verification-method: static-source-inspection
supersedes: []
---

# CodeDOM failed-write rollback must preserve unrelated bytes

## Finding and decision

**A CodeDOM write that fails before publishing its candidate can overwrite a different writer's completed edit during rollback. A failed creation can instead delete the other writer's newly created file.** The failed-destination recovery branch treats any readable state different from the original as something to restore. It does not establish that the failed operation produced that state. The same function protects its earlier successful writes with a different check: it skips rollback when current bytes differ from the candidate that it wrote. [Failed destination][failed] [Restoration][restore] [Successful destinations][succeeded]

The decisive witness does **not** depend on a writer racing between the recovery read and restoration. The unrelated bytes are already present, readable and different from both the original and the candidate at that read. The recovery branch observes the difference and chooses to overwrite it. This is a concrete ownership error beyond the already documented limitation that an in-process mutex cannot serialize external editors.

**Required contract:** an unsuccessful write does not confer ownership of whatever occupies its destination afterward. Compensation may undo an attributable effect while its ownership conditions still hold. A definite pre-publication failure requires no destination compensation; an uncertain or conflicting destination must be preserved and reported, not reconstructed from the old snapshot merely because it differs.

The proposed change belongs in the existing CodeDOM commit/atomic-write boundary and its effect reporting. Preserve preconditions, optimistic checks, syntax validation, the shared commit coordinator, default-deny execution, and rollback of known owned writes. No new model role or scheduler is needed.

This is a conditional source-derived data-loss path, not an executed incident, prevalence estimate, permission bypass, or claim that every failed edit loses work. A recoverable write failure, an independent writer, a successful recovery read, and a successful compensating overwrite or removal are explicit preconditions.

## Evidence boundary and distinct scope

Public main was resolved to [`43e42a89e5037670b49fc6f33f7555eb19a89950`][pin] on November 4, 2026 and rechecked unchanged at 23:19 UTC. Repository links pin that revision. Root and scoped guidance, the architecture contract, current tools specification, production registration, commit and atomic-write implementations, relevant authored tests, the current audit inventory, and adjacent reports were inspected. Retained source copies were checked against the complete pinned tree's Git blob identities.

No repository implementation was changed or published. No repository executable, test, build, live fault injection, benchmark, project model call or training job was run. All fixtures, comparisons and performance results proposed below are unrun.

Root guidance assigns creative work to models and execution reality to the deterministic host/kernel. Scoped tool guidance requires explicit effects and governed production execution. Architecture guidance requires implemented capabilities, deficiencies and future contracts to remain distinct. The current tools specification correctly records that line and batch edits share `commitFiles` and that external writers do not participate in its mutex. Older neighboring capability summaries are not evidence for the present code. [Root][root] [Tool guidance][tool-guidance] [Architecture guidance][architecture-guidance] [Current line-edit specification][tool-spec]

This report is a CodeDOM consumer-conformance addendum to existing recovery requirements, not a new claim that ownership-aware compensation matters:

- **Edit-observation precondition continuity** established preservation of the caller's read condition and brought line verbs under the cooperative commit lock. Those repairs are present. This witness passes every before-write check and begins at a later failed write. Its third-party bytes are visible to recovery, so another preflight check alone does not fix it. [Prior edit audit][prior-edit]
- **Structure coverage and refactor preconditions** concerns structural discovery, source-bound offsets and dependencies outside the write set. No structural query, stale offset or omitted read dependency is needed here. [Prior structural audit][prior-structure]
- **Tool-effect receipts and validation completeness** already requires preservation of concurrent work and admits that attribution remains incomplete outside a shared ownership protocol. **Effect recovery closure** concerns the later projection of whether recovery is still owed. This finding identifies an earlier destructive consumer: `commitFiles` can erase the competing state before a later receipt can explain it. It does not reassert their original accounting defects. [Prior effect contract][prior-effects] [Prior recovery projection][prior-closure]
- **Recurse compensation write ownership** concerns cancellation/reversion of campaign executor writes. This witness is inside a single CodeDOM tool call and needs no recurse run, campaign cancellation or journal replay. [Prior recurse audit][prior-recurse]

The complete current audit filename inventory and targeted current-main searches for `commitFiles`, `failedIdx`, failed destinations and rollback were checked, along with the adjacent bodies above. No dedicated report of this failed-destination ownership predicate was found. That is a bounded non-duplication claim, not a claim about every historical branch or discussion.

| Capability | Status at the pin | Limit |
|---|---|---|
| Governed production tool registration and execution | Implemented | Catalog registration is not itself permission to execute. [Catalog][catalog] [Hydration][hydration] [Execution][execution] |
| Original-byte checks before commit and immediately before each write | Implemented | These checks precede the failed write and later competing publication. [Checks][checks] |
| CodeDOM commits serialized within one process | Implemented | External editors and other processes are not covered. [Coordinator][coordinator] |
| Candidate written through a temporary file and replacement | Implemented | This does not identify another writer's destination bytes after an error. [Atomic writer][atomic-write] |
| Earlier successful destinations checked against their candidate before rollback | Implemented | This remains a check-then-write protocol, not universal cross-process compare-and-swap. [Successful rollback][succeeded] |
| Failed destination compensated only when the operation owns its current state | Incomplete | Different-from-original, or readable-created-path, is sufficient today. [Failed rollback][failed] |
| Typed publication outcome, stronger ownership recovery and evaluations below | Proposed or unverified | No repair or runtime result is established here. |

## Production path and the asymmetric recovery rule

### The shared helper is reachable from ordinary editing

The catalog includes the CodeDOM family. `HydrateModularTools` installs its families into both the VirtualStore and global registries after binding executive adapters. CodeDOM registration includes `apply_edits`, all three line verbs, element edits, `create_file` and `repoint`. The registry checks the effective allowlist and installed guard before calling the handler. The witness begins only after ordinary authorization and argument/path admission succeed. [Catalog][catalog] [Hydration][hydration] [Registration][registration] [Execution][execution]

`apply_edits` preserves a source snapshot, plans in memory, then calls `commitFiles`; a one-entry batch is supported. Single-file line edits also publish through that helper. `create_file` validates its candidate and calls the helper with `create: true`. Thus neither a multi-file transaction nor a direct test-only helper invocation is necessary to reach the defect. [Batch planning][batch] [Batch bounds][batch-bounds] [Line publisher][line-publisher] [Create publisher][create]

Inside `commitFiles`:

1. `commitMu` excludes other participating CodeDOM commits.
2. Supplied read dependencies and each original snapshot are checked.
3. Each destination is checked again just before writing.
4. A successful `commitWriteFile` call adds the plan to `succeeded`; an error records `failedIdx` and begins recovery.
5. For that failed entry, a readable existing file is restored whenever `cur != orig`. A readable creation target is removed regardless of its bytes.
6. Earlier successful entries are treated more conservatively: current bytes must equal `p.data` before restoration is attempted.

The error path therefore promotes a negative observation, "this is not the original," into a positive ownership claim, "this is ours to undo." The stored plan contains only original bytes, candidate bytes, mode and creation intent. It contains no failed-write publication receipt that establishes the intermediate bytes' authorship. [Plan type][plan-type] [Commit and recovery][commit]

### Atomic replacement does not justify restoring the failed destination

The production write seam is `atomicfile.WriteFile`, not a truncating write. It creates a unique temporary file, writes and syncs it, closes it, sets its mode, and only then attempts destination replacement. Errors during those earlier stages return before `replaceExisting` is called; cleanup targets the temporary file. In a stipulated temporary-file sync failure, for example, this operation has not published its candidate to the destination. [Production seam][coordinator] [Atomic phases][atomic-write]

The primary witness uses that definite pre-publication case. It does not infer that **every** error from the helper means the destination is untouched. Windows replacement has platform-specific behavior, attribute handling and retries; publication uncertainty must be classified at the writer boundary rather than guessed from an error string. The striped atomic-write locks are also in-process locks and do not cover external writers. [Atomic locks][atomic-locks] [Windows replacement][windows]

## Concrete causal witnesses, not executed reproductions

### Existing file: recovery destroys a completed external edit

Use an ordinary admitted text file and a valid one-entry `apply_edits` request. Let:

- `O` be the original bytes, `one\ntwo\n`.
- `P` be the planned candidate, `ONE\ntwo\n`.
- `X` be an independent writer's bytes, `user revision\ntwo\n`.

All three values are distinct. No malformed content, stale handle, symlink, missing source, cross-file dependency or model mistake is required.

| Step | Destination state | Relevant decision |
|---|---|---|
| Plan and preflight | `O` | The batch reads `O`; an optional supplied read condition may validly match it. |
| Both optimistic checks | `O` | All existing before-write checks succeed. |
| Attempted publication | `O` | A temporary-file operation fails before destination replacement. The transient failure does not persist through the later recovery write. |
| Independent publication | `X` | An external writer finishes after the final optimistic check and before the failed-target recovery read. |
| Recovery read | `X` | `cur != orig` is true; no equality to `P` or ownership receipt is required. |
| Recovery write | `O` | `restore` writes `O`, erasing `X`. The readback equals `O`, so this path adds no rollback conflict. |
| Return | `O` | The handler returns the original write error. It does not report a successful edit, but this recovery path need not separately report that it overwrote another writer. |

The external writer can be fully quiescent from the recovery read onward. A stronger final read alone therefore does not solve this witness. The faulty step is the decision to restore already observed unrelated state. [Recovery predicate][failed] [Restore and readback][restore] [Return][return]

The filesystem fault and external interleaving are stipulated conditions, not observed OS behavior in this review. A future deterministic regression can use the existing write seam to represent the schedule, while separate fault-injection tests qualify the production atomic writer. A seam test alone must not be described as proof that every platform reaches the schedule with the same injected OS error.

### Failed creation: unrelated new file is removed

Start with an absent admitted `.go`, `.py` or other supported CodeDOM destination and a valid `create_file` candidate. Both absence checks succeed. A pre-publication write failure occurs. Before recovery reads the destination, an independent writer creates a regular readable file there with unrelated bytes `X`.

Because the failed plan has `create: true`, the readable file enters `restore` regardless of its content. `restore` calls `os.Remove`. If removal succeeds, no conflict is appended. The operation that failed to create its own candidate has deleted the other writer's file. [Create entry][create] [Create recovery][restore] [Failed-entry dispatch][failed]

This is the same ownership error, not a second independent finding. If the destination remains absent, the failed-create read-error branch does nothing; if removal is refused, it records a restoration error. Those controls narrow the witness but do not protect a readable unrelated regular file.

## What existing tests establish, and what they miss

The inspected tests are substantial and should be preserved at their appropriate contracts:

- `TestApplyEdits_RollbackOnWriteFailure` exercises an unchanged failed destination and restoration of an earlier successful write. [Ordinary failure test][test-failure]
- `TestApplyEdits_RollbackConflictReported` changes the **successful first file** before the second write fails. That reaches the guarded `succeeded` branch, not the failed destination's weaker predicate. [Successful-file conflict test][test-success-conflict]
- `TestApplyEdits_InterWriteConflictRollsBackEarlierWrite` changes a later file before its immediate optimistic check. The check rejects it before a failed write is recorded. It does not enter the witness's post-check failure schedule. [Inter-write conflict test][test-between]
- `TestApplyEdits_PartialWriteFailureLeavesNoCorruption` replaces the production seam with a truncating `os.WriteFile` that writes half the candidate and returns an error. Related tests exercise restoration read, write and readback failures. These establish behavior under that artificial writer, not ownership of arbitrary bytes observed after a real atomic-write error. [Partial-write test and recovery cases][test-partial]

The last distinction matters to the repair. Keeping the old test expectation while allowing an unspecified failing writer to leave arbitrary bytes would preserve the unsound assumption. A partial-effect test must supply trustworthy information about what the writer actually published, or expect explicit unresolved recovery. Replacing the test with an always-successful writer would simply remove the difficult case. All these tests were read, not run.

## Proposed recovery contract

### Separate publication outcome from later destination state

The writer should supply a typed outcome meaningful to its implementation: definite no destination publication, attributable publication, or uncertain publication. These are proposed semantics, not existing exported types. Preserve the original error and causal phase without making callers parse formatted error messages. Temporary-file cleanup remains the writer's responsibility; a temporary-file failure is not authority to change the destination.

For each failed destination:

1. **Definite no publication:** do not compensate that destination. If inspection discovers competing state, preserve it. Earlier genuinely successful entries still require their own settlement.
2. **Attributable effect with valid ownership conditions:** compensate only that effect under the existing coordinated boundary or an explicitly supported conditional mutation protocol. Preserve required file metadata and verify the resulting state.
3. **Unknown publication or conflicting state:** do not infer ownership from a mismatch, a candidate prefix, file length, path existence, timestamps, or the operation's error. Preserve the observed state and return an explicit unresolved/conflict disposition with its path.
4. **Failed creation:** absence is the intended precondition, not continuing authority to delete whatever later appears. Removal needs attributable creation plus a still-valid conditional ownership check.

A current-bytes-equal-candidate check is a useful conservative guard, and would reject the two distinct-`X` witnesses. It is **not** by itself proof of authorship or an atomic compare-and-swap. Another writer can independently produce the same bytes, metadata can change, and an uncoordinated writer can still act between comparison and mutation. State the supported coordination domain; do not upgrade a local check into a universal cross-process transaction guarantee.

### Keep recovery and executive reporting consistent

Return the original failure together with per-path settlement: not published, restored, preserved conflict, or unknown/failed recovery. These are proposed result distinctions. A complete rollback claim requires evidence for every attempted effect, including the failed destination. Do not label untouched third-party content as a partial CodeDOM write merely to make the accounting balance.

Keep existing session effect accounting and source-change invalidation rather than creating a second success-only ledger. The host records publication and recovery facts; the existing executive determines whether to retry, inspect, repair or ask for help. A model's explanation cannot authorize overwriting a conflict. Recovery must not silently broaden the original write scope or erase independently owned changes to make the workspace resemble the old snapshot.

### Primary literature and limits of transfer

Garcia-Molina and Salem's original *Sagas* work treats compensation as an explicit part of an interleavable sequence of operations. Its database model motivates distinguishing completion from compensation; it does not prove filesystem atomicity or prescribe a new saga subsystem here. [Original paper][sagas]

Microsoft's compensating-transaction guidance directly warns that restoring original state can overwrite concurrent work and calls for compensation that accounts for it. It also treats compensation failures and progress as information that must remain recoverable. The narrow application here is an ownership-sensitive failed-write contract, not adoption of the article's distributed-cloud architecture. [Primary engineering guidance][compensation]

## Full costs and alternatives

The current successful path already retains original and candidate bytes, checks originals at least twice, performs atomic temporary writes, and holds the global CodeDOM commit mutex across publication and recovery. Failure can add destination reads, restoration writes, file syncs, temporary-file cleanup, and platform retry delays. The batch's 1 MiB supplied-content cap does not bound the existing file bytes it reads, copies or restores. [Batch input bound][batch-bounds] [Commit][commit] [Atomic writer][atomic-write]

Let `B` be total original bytes of the planned files, `P` candidate bytes, `F` attempted destinations, and `R` bytes processed during recovery. The existing source shape has memory proportional to retained `B + P` plus planning copies and I/O proportional to snapshot/check/publication bytes plus `R`. It is not merely proportional to supplied replacement text. These are structural cost terms, not measured bounds on elapsed time or allocations.

Compare four designs:

- **A, current behavior:** no new bookkeeping, but the demonstrated preservation failure remains.
- **B, conservative failed-target guard:** refuse compensation of conflicting or uncertain failed destinations. Low added comparison/diagnostic cost, potentially more unresolved recoveries. It must not silently claim atomic rollback.
- **C, typed writer outcomes and ownership-qualified settlement:** small per-destination metadata and more failure-path logic; may avoid unnecessary rereads/restores after definite pre-publication failures. Platform qualification and receipt maintenance are real engineering costs.
- **D, broader cooperative conditional mutation:** potentially stronger guarantees among participating writers, but adds lock/lease lifecycle, deadlock ordering, cancellation and contention costs. It still cannot constrain arbitrary external editors. Adopt only for a demonstrated need, not as a prerequisite for fixing the observed mismatch predicate.

No model call is necessary for publication classification or byte comparisons. Preserved conflicts may lead to additional reads, user decisions, model turns and validation later. Those costs belong in the end-to-end ledger, alongside bytes read/written, temporary storage, synchronization latency, mutex wait/hold time, recovery duration, retry count and lost-work incidents. A faster rollback that erases independent work is not an efficiency gain. No speedup, token saving or quality improvement is claimed.

## Evaluation program, all unrun

### Deterministic boundary qualification

Use disposable fixtures with controlled writer/failure barriers, no live user data. Assert final bytes and existence, not just returned errors or rollback counts.

| Case | Required observable outcome |
|---|---|
| Existing target, definite pre-publication failure, no other writer | Original remains; no destination restoration is needed. |
| Existing target, independent `X` after the last check but before recovery read | `X` remains byte-for-byte; the failed operation cannot overwrite it. |
| Failed create followed by independent readable creation | The independently created file remains. |
| Earlier successful target still contains its attributable candidate | Its permitted compensation still works; repair must not disable all rollback. |
| Earlier successful target changed by an independent writer | Preserve competing state and report the conflict, retaining existing protection. |
| Write reports failure after a demonstrably owned publication | Typed outcome and ownership conditions govern compensation; do not equate every error with no effect. |
| Writer can leave partial destination bytes but offers no trustworthy effect receipt | Explicit unresolved state, not guessed ownership. |
| Restore read/write/readback failure or cancellation during recovery | Preserve the original error and unresolved path; no false all-restored result. |
| External writer changes state after recovery comparison | Expose the residual consistency boundary or prove the chosen conditional protocol; a pre-read-only test cannot certify this case. |
| Same candidate bytes, metadata-only change, path replacement or alias | Verify the stated identity/metadata scope; byte equality alone must not be described as universal authorship. |

Exercise direct line editing, a one-entry and multi-entry `apply_edits`, and `create_file` through their registered production handlers with an explicit authorized test executive. Keep invalid paths, stale read conditions, syntax failures and refused effects as negative controls. Failure injection must not become a production bypass.

### Writer and platform qualification

Separately qualify actual temporary-file creation, write, sync, close, mode and replacement failures. Run POSIX and Windows cases with appropriate filesystem controls; distinguish pre-publication errors from ambiguous replacement outcomes. Avoid timing sleeps as the only race mechanism. A fake truncating writer must not stand in for the real atomic implementation when claiming publication guarantees. Include cold and warm filesystem state and contention, and report which outcomes could not be induced deterministically.

### Fair end-to-end comparison

Compare A, B and C first on identical file sizes, failure schedules, concurrent edits and admitted tasks. Require preservation of independent work as a correctness gate. Report unresolved recovery frequency separately from task completion: B can preserve data yet shift work to a human, while C may reduce uncertainty without eliminating external races. Add D only if contention or recovery evidence justifies its scope.

For any later model-assisted study, hold provider/model, task prompts, tool surface, permissions and verification budget constant. Count failed and blocked tasks, all recovery rounds, total model/tool cost, user intervention and final behavioral acceptance. Do not reward a candidate for reporting success more readily or restoring the old bytes when those bytes erase legitimate new work.

## Checkable exits

1. Both distinct-`X` witnesses preserve the independent file/content through real handler paths.
2. Definite no-publication, owned publication and unknown publication have distinguishable, platform-qualified recovery behavior.
3. Earlier successful writes retain valid compensation and conflict protection; stale observations and unapproved effects remain refused.
4. Every unsettled path remains explicit in the returned evidence. No complete-rollback claim is made from only original-byte readback after an unowned overwrite.
5. Tests distinguish the production atomic writer from partial-write fakes, and published status names the exercised platforms, coordination domain and remaining races.

Until those exits have evidence, describe CodeDOM as providing preflight validation, cooperative optimistic checks and best-effort recovery, with the failed-destination ownership gap still open.

## References

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/43e42a89e5037670b49fc6f33f7555eb19a89950
[root]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/agents.md
[tool-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/architecture/AGENTS.md
[tool-spec]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/architecture/tools/IMPLEMENTED_SPEC.md#L278-L282
[prior-edit]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/audits/edit-observation-precondition-continuity.md#L213-L245
[prior-structure]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/audits/structure-coverage-and-refactor-preconditions.md
[prior-effects]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/audits/tool-effect-receipts-and-validation-completeness.md
[prior-closure]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/audits/effect-recovery-closure.md
[prior-recurse]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/Docs/audits/recurse-compensation-write-ownership.md
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/catalog/catalog.go#L32-L54
[hydration]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/core/virtual_store_tools.go#L41-L83
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/register.go#L7-L50
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/registry.go#L330-L423
[batch]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/apply_edits.go#L225-L338
[batch-bounds]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/apply_edits.go#L18-L105
[line-publisher]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/lines.go#L614-L625
[create]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/create_file.go#L45-L107
[plan-type]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L36-L45
[coordinator]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L18-L25
[checks]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L86-L150
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L80-L205
[failed]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L175-L185
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L155-L174
[succeeded]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L187-L201
[return]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/commit.go#L202-L205
[atomic-write]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/atomicfile/atomicfile.go#L168-L214
[atomic-locks]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/atomicfile/atomicfile.go#L118-L145
[windows]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/atomicfile/replace_windows.go
[test-failure]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/apply_edits_test.go#L337-L372
[test-success-conflict]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/apply_edits_test.go#L374-L422
[test-between]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/apply_edits_test.go#L661-L695
[test-partial]: https://github.com/theRebelliousNerd/codenerd/blob/43e42a89e5037670b49fc6f33f7555eb19a89950/internal/tools/codedom/apply_edits_test.go#L697-L947
[sagas]: https://www.cs.cornell.edu/andru/cs711/2002fa/reading/sagas.pdf
[compensation]: https://learn.microsoft.com/en-us/azure/architecture/patterns/compensating-transaction
