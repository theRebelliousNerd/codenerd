---
doc-class: deep-dive
subsystem: workspace-world-census
implementation-status: proposed
last-verified: 2026-11-02
verified-against: b1fa3d09c30b545d16f2fbfff5a319bfae10e251
verification-method: static-source-inspection
---

# Failed membership observation is not evidence of exclusion

## Decision and evidence boundary

Preserve the shared Git-backed workspace authority and its fail-closed traversal policy, but carry **operational failure separately from a completed exclusion decision**. At the reviewed main revision, a failed dynamic `git check-ignore` is logged, converted into `false`, and cached exactly like a successful ignored-path answer. The scanner's error-aware enumeration consequently receives an ordinary skipped file or directory instead of an error. Under a narrow, source-supported timing condition, it can return `Unchanged:true` despite not determining whether a newly observed path belongs to the workspace; the TUI then reports successful completion.

The proposed repair is not “include paths when Git fails.” An unknown directory should remain unopened and an unknown file unread, while the scan becomes incomplete or fails with its prior successful generation preserved. Safety of traversal and completeness of observation are separate obligations.

This is documentation-only inspection of [`b1fa3d09c30b545d16f2fbfff5a319bfae10e251`](https://github.com/theRebelliousNerd/codenerd/tree/b1fa3d09c30b545d16f2fbfff5a319bfae10e251). No application, tests, model inference, benchmarks, or fault injection were run, and no repository files were edited. The sequence below is a conditional witness, not a reproduced incident or measured frequency. It establishes a misleading completeness/result boundary, not a permission bypass, destructive action, or inevitable model error.

## 1. Why this is a distinct residual

The repository already requires complete filesystem generations before retirement/publication, propagation of enumeration and admitted-file failures, and joined workers before returning. Workspace guidance separately requires one authority, pre-descent pruning, Git-safe path handling, and serialization of dynamic checks with refresh. The missing contract lies **between** these otherwise compatible obligations: a leaf membership failure is erased before the scanner's error gate can observe it. [World guidance][world-guidance], [workspace guidance][workspace-guidance].

Existing documents were checked for overlap:

- The workspace ADR and risk register explain Git snapshots, dynamic paths, finite sibling batches, refresh synchronization, and recovery through refreshed snapshots. Their happy-path dynamic-membership evidence does not specify a distinct operational-failure result. This report does not propose replacing Git authority or restoring private directory ignore lists. [Membership ADR][adr], [workspace risks][risks].
- The world incomplete-census work already preserves prior state for errors that reach the scanner. Current implementation and failure tests contain those protections. This report is a residual failure-transport problem, not a claim that the scanner ignores every walk, read, or cancellation error. [Current census][census], [failure tests][failure-tests].
- The structural-coverage audit's as-built section describes explicit deficits and guarded absence/refactoring decisions. Its original problem concerned structure-index read/walk failures and structural edit preconditions. The present witness is the shared membership leaf's dynamic Git failure feeding the fast incremental scanner. Its existence does not establish a new destructive-refactoring bypass. [Structural coverage audit][structure-audit].
- The world publication-order repair is present: observation, application, and publication share scanner ownership with pending-result replay. The current issue occurs earlier, when constructing the purported census. The explicit-deep-scan audit concerns a valid fast no-change result failing to discharge a requested deeper stage; here the fast no-change result itself lacks the required membership observation. [Publication audit][publication-audit], [current refresh owner][refresh], [deep-scan audit][deep-audit].

The audit queue contains earlier world/graph repairs, and a bounded search of current `Docs/audits` found no `check-ignore` report. This is not a claim that every historical research branch was exhaustively searched. [Audit queue][queue].

## 2. Implemented behavior, from Git status to normal pruning

### 2.1 The Git driver initially preserves the important distinction

Git documents `check-ignore` exit status **0** for one or more ignored inputs, **1** when none are ignored, and **128** for fatal error. With `--stdin -z`, paths are NUL-delimited. A nonzero status is therefore not uniformly an execution failure, and a failed execution is not an ignored-path answer. [Git's primary command reference](https://git-scm.com/docs/git-check-ignore#_exit_status), [input/output protocol](https://git-scm.com/docs/git-check-ignore#_options).

CodeNERD's `gitRun` executes Git directly, not through a shell, supplies the workspace via `-C` and `cmd.Dir`, sanitizes repository-selecting environment variables, and adds `GIT_OPTIONAL_LOCKS=0`. It separates an exited process's status from launch/runtime errors. `checkIgnore` treats status 1 as a healthy empty ignored set, parses status-0 output, and returns an error for other statuses or process errors. It also sends a directory's trailing-slash form so directory-only ignore patterns can match. These are protections to retain. [Git invocation][git-run], [dynamic check][check-ignore].

This audit does not strengthen Git's documented exit result into a universal proof that every possible ignore input was read without warnings or that the live filesystem was atomically snapshotted. The bounded finding starts with an operational error that the existing driver **already recognizes**.

### 2.2 `askBatch` loses the error's meaning and stores the loss

`askBatch` holds `refreshMu`, first reuses any `decided` entries, then invokes `checkIgnore` for the remaining paths. On error it logs a warning, but still assigns every needed path:

`value := err == nil && !ignored[r]`

It writes that value to both the returned map and `m.decided`. A failed attempt therefore becomes the same cached `false` as a completed ignored answer. File and directory decisions have separate keys. A batch error affects all still-undecided inputs in that invocation, not only one path. [Decision cache][ask-batch].

This is not entirely silent: the logger receives a warning. However, the public membership decision carries no cause, retryability, or incomplete-coverage state. Moreover, `gitRun` captures stderr but does not include it in its return; an exited command's caller generally sees the synthesized command/root/status error, not the captured Git explanation. Better structured diagnostics could help recovery without dumping configuration or arbitrary stderr unboundedly. [Git result handling][git-run].

`Includes` and `IncludesDir` first honor path containment, universal exclusions, boundaries, patterns, and the successful snapshot. For an unknown path, they read the same boolean decision cache or invoke `ask`. Once an error-derived `false` is stored, a repeat lookup under unchanged freshness state does not itself retry Git. [Includes and IncludesDir][includes].

### 2.3 Error-shaped APIs cannot recover the discarded cause

`Gate` converts a false directory decision to `fs.SkipDir`, or a false file decision to `ErrNotMember`. `Admit` preserves directory `SkipDir` and translates file `ErrNotMember` to `(false, nil)`. Those are correct normal exclusion semantics, but they now also represent the operational-error case. [Gate/Admit][admit].

Go's walk callback contract makes the distinction consequential: `SkipDir` is ordinary traversal control, while another non-nil callback error stops enumeration and is returned. A directory can be pruned before reading its children. That is precisely the desirable exclusion/cost behavior for an excluded or unresolved subtree—but pruning alone cannot certify why the subtree was omitted. Membership is not itself a replacement for action authorization. [Go `WalkDirFunc`](https://pkg.go.dev/io/fs#WalkDirFunc).

The leaf's own `Membership.Walk` also batches unknown siblings and later uses the boolean inclusion methods, so it likewise has no dynamic-Git error to return. **It is not the production incremental scanner's walker.** The scanner directly uses `filepath.WalkDir` followed by `mem.Admit` in its callback; the exact path must be tested rather than substituting the leaf walker. [Leaf walker][walk], [scanner callback][census].

## 3. Production TUI path and the narrow no-change witness

### 3.1 Reachable through normal scanning

Chat boot dispatches `runScan(false)`; the `/scan` handler dispatches `runScan(deep)` after parsing its arguments. `runScan` calls the session scanner's `Refresh` with `SkipWhenUnchanged:true`. A returned error becomes a failed scan message. A nil-error `Unchanged` result instead sets the status to “Workspace unchanged” and returns counts with zero generated facts. The completion handler displays the successful scan table and fresh-codebase-facts prose when its error field is nil. [Boot dispatch][boot], [command dispatch][command], [runScan][chat-scan], [completion rendering][scan-message].

Before each incremental filesystem walk, `Scanner.membership` obtains the root's authority and calls `m.Refresh()`, returning any failure. In a Git workspace this rebuilds the snapshot and clears the dynamic decision cache only after a successful load. This materially narrows the witness: an ordinary pre-existing unignored file that is present in the fresh snapshot never needs the problematic dynamic check. A cached false from an earlier scan also does not survive a successful scan-start refresh. [Per-scan membership refresh][scan-membership], [snapshot replacement][snapshot].

The scanner then enumerates with `filepath.WalkDir`, checks cancellation and walk errors, calls `mem.Admit`, and only records admitted files in `currentFiles`. It later compares that set with the prior manifest. With no changed, new, or deleted files and `SkipWhenUnchanged`, it returns `Unchanged:true` after the remaining successful orientation/cache steps. [Census and admission][census], [delta comparison][comparison], [unchanged branch][unchanged].

### 3.2 Minimal source-derived sequence

Use a genuine Git worktree with a nonempty, healthy prior scanner manifest, current canonical identities, and unchanged previously indexed source. No selected path is under `.git`, `.nerd`, a repository boundary, an explicit exclusion, or an enforced treatment scope.

1. The scan-start membership refresh completes successfully. Its member snapshot contains the old source files.
2. An external editor or other independent writer creates an eligible untracked source file **after that snapshot, but before its parent directory is enumerated**. The new file is absent from both the membership snapshot and the previous scanner manifest. Creating this untracked file need not change the stamped index, HEAD/ref, root `.gitignore`, or `info/exclude`.
3. `filepath.WalkDir` sees the new entry. `mem.Admit` cannot answer it from the snapshot, so the dynamic `check-ignore` attempt runs. That attempt encounters a recognized operational failure, such as a non-0/non-1 exit or a process error. Subsequent stages are otherwise healthy.
4. `askBatch` logs the error and caches `false`; `Admit` returns `(false,nil)` for this file. The scanner omits it from `currentFiles` without receiving a membership error.
5. Every previously indexed file is still observed and unchanged. Because the omitted new file was not in `prevEntries`, it produces neither a new-file delta nor a deletion delta. With no other deltas, the scanner takes the no-change branch.
6. If orientation/repository-fact attachment and kernel application succeed, `Refresh` returns that result; `runScan` reports unchanged and the completion handler uses its success presentation. A workspace without an initialized orientation snapshot is an explicitly supported control, avoiding dependence on extra orientation work for this witness.

[Membership freshness state][snapshot], [dynamic decision collapse][ask-batch], [scanner delta/no-change logic][comparison], [unchanged result][unchanged], [orientation-not-initialized handling][orientation], [refresh return/application][refresh].

For a newly observed directory, the corresponding error becomes normal `SkipDir`, pruning all children. The file witness above is sufficient and avoids assuming which deeper entries would have been enumerated. A file created only after its parent was already read is a different ordinary concurrent-filesystem window, not this finding. Likewise, a file present before the successful membership snapshot usually follows the snapshot fast path, not the dynamic-failure path.

**What the witness does not require:** deletion, parser failure, failed SQL publication, a deep scan, a model call, or adversarial input. It also does not prove those failures occur frequently. Selecting a fault that prevents the earlier snapshot or later required Git stages from succeeding would produce a visible error instead, not this clean no-change outcome.

## 4. State, persistence, and recovery are bounded

The operational-failure answer is stored in the in-process membership object's `decided` map. `For`/`Open` share an authority by canonical root and pattern identity, so consumers of that same object may reuse the false answer until it is invalidated. It is not persisted as an ignore rule and does not rewrite `.gitignore`. It is also not a permanently poisoned world manifest: in the minimal witness the new file was never indexed, so the old manifest simply lacks its entry. The ordinary success branch may save cache updates, but the demonstrated consequence is omitted discovery and a false completeness signal, not corruption of previously stored source bytes. [Authority cache][authority-cache], [snapshot/cache reset][snapshot], [unchanged branch][unchanged].

Recovery already exists. An explicit successful `Refresh` replaces the snapshot and clears `decided`; a subsequent scan always requests that refresh. If Git is healthy and the file remains eligible, the snapshot includes it and the scanner can identify it as new. A relevant control-file stamp change can also trigger automatic refresh. If refresh fails, the previous snapshot is not replaced; scanner entry returns the error rather than claiming that refresh succeeded. Mere recovery of the Git executable or underlying resource, without refresh/invalidation, is not enough to reclassify an already cached false lookup. [Refresh and freshness][snapshot], [Includes cache lookup][includes].

The source also derives deleted paths from prior entries missing in `currentFiles`. A generalized incomplete census is therefore dangerous, but this audit does **not** assert that a normal successfully refreshed, previously indexed file becomes falsely deleted by this specific witness: such a file is usually a snapshot member and bypasses dynamic checking. Any deletion variant needs its own verified setup and should not inflate the stronger, simpler missed-new-file result. [Deletion derivation][comparison].

`Unchanged` also does not mean no subsystem work occurred. The result can carry treatment, repository, or orientation updates; the kernel applier handles these before or within its unchanged return. The configured world publisher is skipped for unchanged source results, by design. The issue is the source-census claim, not that all facts are frozen or that the existing publication receipt is broken. [Incremental application][apply], [publisher condition][refresh].

## 5. Protections that must remain intact

- **Fail-closed traversal:** do not read or descend into unresolved paths merely to avoid an error result. Preserve universal exclusions, gitlink/nested-repository boundaries, operator patterns, tracked-default exceptions, and treatment semantics.
- **Normal Git outcomes:** status 1 remains successful “none ignored”; status-0 mixed batches remain per-path decisions. Empty output alone is not an independent completed-operation receipt.
- **Snapshot failures already surface:** a confirmed worktree whose `ls-files` fails returns an error. The documented missing-Git/non-worktree fallback is a distinct initialization policy, not permission to switch an established Git workspace to fail-open fallback after a dynamic failure. [Construction policy][construction], [Git snapshot loading][load-snapshot].
- **Freshness serialization:** `refreshMu` prevents an in-flight dynamic answer from populating a replacement snapshot. Do not lose that generation ownership when introducing error state. Successful refresh clears prior dynamic answers. [Decision serialization][ask-batch], [snapshot refresh][snapshot].
- **Scanner failure gates:** existing walk/metadata/hash/cancellation failures return errors; full fallback is staged, and admitted delta workers are joined before their errors are checked and publication proceeds. These mechanisms are real and are useful insertion points once membership preserves the cause. [Census][census], [content errors][comparison], [joined-worker barrier][worker-barrier].
- **Refresh publication ownership:** pending application/publication is replayed before another scan advances; this in-memory receipt is not a crash-durable transaction, but this audit does not require expanding it. [Refresh coordinator][refresh].

Existing membership tests cover dynamic success, pruning, nested-ignore refresh, user overlays, root aliases, path whitespace, concurrent refresh, and sibling batching. Existing scanner failure tests seed prior database/cache state and inject walk/stat/cancellation and worker failures. Their presence is not execution evidence from this review. The inspected cases do not establish transport of an actual dynamic `check-ignore` error through `Admit` into the production scan result. [Membership tests][membership-tests], [membership regression tests][membership-regressions], [scanner failure tests][failure-tests].

## 6. Proposed contract: closed admission, explicit incomplete knowledge

### Separate the decision from the attempt

Use an error-aware internal membership result: included, excluded by a completed decision, or unresolved because the authority failed. Preserve the cause, queried path kind, and snapshot/attempt ownership. Compatibility boolean methods may still return false for unresolved paths where conservative admission is required, but enumeration and negative-evidence consumers must use the richer result. Merely adding a log line or a counter does not repair the scanner's result contract.

`Gate`, `Admit`, and `Walk` should propagate operational failure as an error or explicit deficit, not `ErrNotMember`/`SkipDir`. Unresolved directories can still be pruned before descent; the same call must mark the observation incomplete. For the existing fast scanner, returning an error before comparison/publication is the narrowest fit with its staged-generation contract. Supporting useful partial scans is a separate design requiring scoped coverage, last-good state, and consumers that cannot treat absent paths as absent source.

Keep successful negative cache entries separate from failed attempts. Failure throttling may be useful to avoid repeatedly spawning Git during an outage, but its cached result must remain a failure, with bounded retry/backoff semantics and an explicit refresh/recovery route. Do not turn a failure TTL into a negative-membership TTL. Do not immediately retry every path independently during the same outage.

### Completeness must be an admission condition for no-change and retirement

Before returning `Unchanged`, deriving authoritative deletions, retiring old rows, or publishing a replacement manifest, require all necessary membership observations for that scan to have completed. A count of admitted files or empty delta lists cannot substitute for coverage. Where the operation returns an incomplete result rather than an error, the TUI must display that state and the retained generation's age, not ordinary fresh-scan completion.

This is a familiar failure-aware enumeration principle, not a claim that CodeNERD uses rsync: rsync deliberately suppresses destination deletion when the sender reports I/O errors, to avoid interpreting failed source observation as disappearance. Go's `ReadDir` likewise returns partial entries alongside an error rather than declaring the partial list complete. These primary references motivate preserving failure evidence; they do not specify CodeNERD's API or prove its runtime behavior. [rsync deletion safety](https://download.samba.org/pub/rsync/rsync.1#opt--delete), [Go `ReadDir`](https://pkg.go.dev/os#ReadDir).

### Preserve practical process and generation ownership

The existing finite Git invocations avoid a persistent helper lifecycle. An error-preserving repair need not replace them with a long-lived process, one subprocess per cached hit, or an unconditional full parse. Propagate the owning operation's cancellation into future Git process handling if that boundary is changed; current `gitRun` uses `exec.Command`, not `CommandContext`, and the dynamic decision API has no context. Thus the scanner's surrounding cancellation checks do not themselves interrupt an in-flight Git command. Treat improved subprocess cancellation as a related lifecycle obligation, not as a result already established here. [Git command construction][git-run].

Preserve NUL-delimited path fidelity, directory/file distinctions, root aliases, explicit-pattern provenance, and generation serialization. Any wider atomic-filesystem guarantee requires additional machinery; carrying a recognized error correctly does not make a live directory walk an atomic snapshot.

## 7. Fair evaluation proposal — not executed

**Deterministic mechanism test.** Seed a real Git workspace through the real scanner, with a nonempty persisted manifest and prior facts. Use a synchronization barrier after the scan's successful membership refresh but before directory enumeration, then create an eligible new file. Allow real Git for snapshot commands and intercept only the targeted dynamic check to produce a launch error or fatal exit. Assert the exact query occurred and the new entry was seen. Do not merely preseed `decided=false`: that would not test error classification and transport.

Compare current and proposed behavior at four boundaries: the membership outcome, `Admit`, scanner result, and TUI receipt. The baseline source predicts a logged error plus possible `Unchanged`; the proposal should remain closed to the unread path and return a visible incomplete/error state. Assert prior database facts, graph state, and manifest bytes remain unchanged on failure, not just that one error string appeared. Release all barriers and join work on every exit.

**Required controls:**

1. Status 1 with no ignored inputs; status 0 with a mixed ignored/unignored batch; real ignored directories and explicit exclusions stay pruned.
2. The same new file present **before** snapshot refresh: admitted from the snapshot, with no dynamic check required. Also create it after its parent was enumerated to keep that separate race from the audited mechanism.
3. Successful dynamic file and directory decisions; fresh directory failure with a read counter proving children were not opened; failed dynamic attempts remain errors on cache reuse.
4. Failure during initial `ls-files`: visible failure already, not misreported as the new residual. Preserve non-Git fallback and missing-Git initialization controls separately.
5. Restore healthy Git and refresh again: the pending new file must be discovered without restarting the process or deleting the world database. Concurrent refresh must not admit an old failed answer into the new generation.
6. Healthy unchanged, genuine deletion, canonical migration, explicit/implicit ignore patterns, tracked exceptions, gitlinks, nested repositories, path whitespace, and cancellation retain existing behavior.
7. Empty/missing/invalid manifest full fallback and nonempty-manifest incremental paths are qualified separately. Preserve the existing second-census and joined-worker failure controls.

**Use the production route.** After leaf tests, exercise normal session boot/injection and `/scan` through `Scanner.Refresh`, keeping world/CodeDOM context enabled. Verify that nil-error success presentation is not emitted for unresolved census. Keep deep-stage behavior separate; it is not needed to establish this finding. No model inference is required for these mechanism checks.

**Cost and quality controls.** Compare the same repository snapshot, manifest state, Git version, filesystem, ignore configuration, concurrency, event timing, and fault schedule. Include steady healthy scans and repeated outages. Measure Git invocation count, elapsed snapshot/dynamic/walk/hash/parse time, lock waiting, peak memory, retries, database/manifest writes, stale-generation duration, and eventual discovery. Do not label a scan faster when it merely omitted more source.

For any later task study, hold model, prompt/context, permissions, tool set, and success criteria fixed; count extra recovery turns and retries as well as tokens. This mechanism makes no model call itself. There is no evidence here of measured token savings, task-quality uplift, or net speedup.

## 8. Costs and acceptance

Boolean decisions are cheap, compact, and broadly consumed; richer outcomes require API migration, explicit caller handling, diagnostics, and cache-state maintenance. Returning a visible error reduces apparent availability during a transient Git failure, but preserves the previous valid generation instead of certifying an incomplete replacement. Retries add subprocess and filesystem cost; retaining last-good/deficit state adds memory and UI complexity. A full rescan after every failed lookup could be much more expensive than propagating one error and recovering on the existing next scan tick.

The current snapshot fast path, true negative cache, finite sibling batches in `Membership.Walk`, and content-aware incremental scanning are valuable efficiencies. The production `filepath.WalkDir`/`Admit` route does not automatically inherit the leaf walker's sibling batching, so cost claims must measure the actual caller rather than assume batching everywhere. The repair should avoid broad cache disabling or opportunistic parsing of excluded/unknown trees.

Acceptance is a contract, not a throughput target:

- A recognized Git operational failure remains distinguishable from an excluded path at the scanner boundary.
- Unresolved paths remain unread/untraversed, while incomplete observation cannot be reported as healthy `Unchanged` or used as authoritative absence.
- Prior successful world/cache state survives failed enumeration; later healthy refresh discovers eligible files and clears failed-attempt state.
- Existing exclusion, identity, refresh-order, cancellation, worker-join, and publication protections remain intact.
- The UI and metrics describe coverage and recovery truthfully; any added performance or quality claim is backed by matched measurements.

**Bottom line:** fail-closed admission is correct. Caching “Git could not answer” as “not a member,” then letting a scanner call the remaining set unchanged, is not. Preserve the closed boundary and transport the uncertainty to the caller that owns completeness.

[world-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/agents.md
[workspace-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/agents.md
[adr]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/Docs/architecture/workspace/adr/ADR-001-git-ls-files-and-check-ignore-as-membership-authority.md
[risks]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/Docs/architecture/workspace/RISK-REGISTER-AND-DECISION-LOG.md
[structure-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/Docs/audits/structure-coverage-and-refactor-preconditions.md
[publication-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/Docs/audits/world-refresh-publication-order.md
[deep-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/Docs/audits/explicit-deep-scan-admission.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/Docs/audits/ASTRA_QUEUE.md
[git-run]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/git.go#L13-L61
[check-ignore]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/git.go#L250-L287
[load-snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/git.go#L98-L164
[ask-batch]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership.go#L537-L585
[includes]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership.go#L291-L368
[admit]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership.go#L454-L493
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership.go#L182-L289
[authority-cache]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership.go#L64-L149
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership.go#L152-L179
[walk]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/walk.go#L18-L119
[scan-membership]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/fs.go#L51-L74
[census]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/incremental_scan.go#L116-L205
[comparison]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/incremental_scan.go#L279-L323
[unchanged]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/incremental_scan.go#L371-L388
[worker-barrier]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/incremental_scan.go#L652-L699
[orientation]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/incremental_scan.go#L748-L765
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/refresh.go#L77-L146
[apply]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/apply_incremental.go#L12-L39
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/cmd/nerd/chat/model_update.go#L873-L877
[command]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/cmd/nerd/chat/commands_handlers_analysis.go#L101-L123
[chat-scan]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/cmd/nerd/chat/helpers_scan.go#L155-L180
[scan-message]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/cmd/nerd/chat/model_update.go#L1032-L1058
[membership-tests]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership_test.go
[membership-regressions]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/workspace/membership_regression_test.go
[failure-tests]: https://github.com/theRebelliousNerd/codenerd/blob/b1fa3d09c30b545d16f2fbfff5a319bfae10e251/internal/world/incremental_scan_failure_test.go
