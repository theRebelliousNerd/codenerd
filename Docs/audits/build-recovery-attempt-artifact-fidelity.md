---
title: Build failure recovery needs reconstructible attempt artifacts
subsystem: session-build-recovery
status: source-supported; proposed contract; evaluations unrun
source_commit: 895e234f638e892904067a636c42303e85db4d50
review_date: 2026-11-05
---

# Build failure recovery needs reconstructible attempt artifacts

## Finding and decision

**A successfully saved attempt patch does not necessarily preserve the attempt that automatic build recovery removes.** When a build or test repair episode gives up and the workspace still fails its build check, `leaveBuildableTree` saves `turnDiffPatch` and restores the turn's owned files. That patch is the repair prompt's display representation with its size budgets disabled. A NUL-containing file is represented only by its path, existence note and before/after byte counts. The attempted bytes are not included, even when writing the patch succeeds. [Recovery owner][recovery] [Shared renderer][renderer] [Binary projection][binary]

The concrete defect is **reconstruction from the advertised `.patch.md` artifact after compensation**, not binary display quality, a missing truncation marker, a hash collision, or a claim that all copies of the attempted work disappear. The binary note honestly says that the diff is not shown. What does not follow is the stronger claim that this unbounded display record preserves the whole attempt for restoration. The architecture explicitly assigns it that role. [Architecture contract][architecture]

**Proposed decision:** separate the bounded review projection from an exact recovery representation. Before describing an attempt as preserved, establish a complete, revision-bound record from which its supported file states can actually be reconstructed. Keep the buildability objective, mandatory verification, Mangle-owned repair decisions, exact permissions and conflict protections. This report does not silently change the existing buildability-first choice when storage fails; the desired failure policy needs an explicit specification.

This is a static source deduction at public main [`895e234f638e892904067a636c42303e85db4d50`][pin]. No CodeNERD execution, tests, builds, runtime reproduction, failure injection, inference, training or benchmark was performed. All evaluations below are proposals. No production occurrence, frequency, universal data loss or measured efficiency gain is claimed.

## Intended behavior and implementation status

The root requirements make the host responsible for truthful observations and the kernel responsible for executive control. They require recoverable omitted context and revision-aware evidence, while the session guidance preserves exact admission, effect accounting and verification. The failed-build owner adds a specific local objective: restore an uncompilable workspace while keeping the failed attempt available for a person. These are compatible requirements when recovery bytes and prompt summaries are separate artifacts. [Root guidance][root] [Session guidance][session-guidance] [Recovery rationale][recovery-rationale]

| Status | Current evidence and limit |
| --- | --- |
| Implemented | Build verification distinguishes affirmative failure from cancellation and indeterminate outcomes. Attributed build failure can enter repair; a gave-up build/tests episode can enter final restoration. |
| Implemented | Known preimages distinguish existing empty files, absent files and unknown states. Restoration checks recorded last-write identity, leaves detected conflicts untouched, and reports them. |
| Implemented | Repair-prompt diffs are budgeted, disclose omission, and name binary edits. The saved patch removes the byte budgets. |
| Partial | “The attempt is saved as a patch” establishes that a display artifact was written, not that every supported afterimage can be reconstructed from it. Binary content is deliberately absent from that artifact. |
| Proposed | A separate exact recovery artifact, completeness and storage receipts, a versioned recovery contract, and byte-for-byte reconstruction tests through the final restoration route. |
| Unverified | Runtime incidence, recoverability through every optional transcript or backup, crash durability, the proposed implementation and all cost or task-outcome measurements. |

Current source, scoped guidance, the diff and session architecture, the main audit inventory and targeted repository searches were inspected before selecting this topic. Existing prose is evidence of intended behavior; the pinned implementation is evidence of the behavior traced here. There is no assumption that an architecture file's historical test receipt was rerun in this review.

## Production source trace

### Repair enters recovery through the ordinary session path

`verifyAndRepairBuild` observes the build, respects the configured verification switch and attribution gate, constructs a build repair specification, and calls `repairLoop`. An indeterminate initial verification does not become a failure merely to enter this path. When the repair loop gives up, only its build/tests kinds call `leaveBuildableTree`; the forcing rounds have separate snapshots and recovery behavior. [Build repair][build-repair] [Final repair disposition][repair-exit]

`leaveBuildableTree` requires a nonempty written set and workspace, a root `go.mod` or `go.work`, and an affirmative failed build verification. It then:

1. Computes `turnDiffPatch` from written paths, retained preimages and current disk contents.
2. Writes the resulting string to `.nerd/attempts/attempt_<timestamp>.patch.md`.
3. Calls `turnFiles{pre: map[string]PreImage{}}.restore` on the result.
4. If the save returned a path and restoration did not return an error, reports that the attempt is saved as a patch at that path, with conflicts identified separately. [Recovery and publication][recovery] [Patch writer][save]

This is not a dormant helper-only concern. The helper is called by the shared repair owner, and the existing recovery test exercises saving and restoring an uncompilable text attempt. The new binary case remains unrun. [Existing recovery test][recovery-test]

### Disabling a display budget does not change its representation

Both `turnDiffSection` and `turnDiffPatch` call `renderTurnDiff`; the latter passes an empty budget. `renderTurnDiff` reads each current file and creates a `fileChange` containing before and after strings. It then calls `renderFileDiff`. [Renderer entry][renderer]

The diff engine checks for a NUL byte in **either** input before cache lookup or diff computation. It sets `IsBinary` and returns an empty hunk list. `renderFileDiff` converts that result to a note of this form:

```text
asset.dat (binary: 2 bytes before, 2 after; diff not shown)
```

There is no payload, encoded byte string, binary delta or link to a separately retained blob in that section. Increasing the prompt budget, disabling the cache or removing all truncation limits does not change the branch. [Engine binary admission][engine] [Binary note][binary]

For normal text, the same renderer builds Markdown-fenced line diffs. The `.patch.md` suffix is therefore accurate about its presentation format; this report does not assume it claims to be directly accepted by `git apply`. The stronger problem holds even for manual recovery: byte counts cannot identify the binary bytes that were removed.

### Restoration retires the live afterimage

The restore function first plans the affected files and refuses missing or unknown preimages. For a path whose current contents still match the turn's recorded last write, it either removes a file that did not exist before or writes its preimage back. It then rebuilds `WrittenPaths` from the snapshot's prior set plus unrestored paths and detected conflicts. In this final-build call the snapshot's prior set is empty, so successfully restored paths leave that list. [Restoration plan and effect][restore] [Write identity][preimage]

These are useful ownership safeguards, not the finding. The primary witness has an exact owned version, no conflicting writer and successful restoration. The attempted afterimage is intentionally replaced in the workspace. Its preservation must therefore be established by the recovery artifact rather than by pointing back to the live path.

## A concrete nonreconstructible artifact

Use an isolated Go module and an ordinary regular data file, `asset.dat`. The example values below are bytes, not literal backslash text:

| State | Hex bytes |
| --- | --- |
| Before the turn, B | `00 41` |
| Attempted afterimage A1 | `00 42` |
| Alternative attempted afterimage A2 | `00 43` |

Let the same authorized turn also make a syntactically valid Go edit that produces an unresolved symbol. This allows parsing safeguards to remain effective while the build can genuinely fail. The turn's write records contain known preimages and matching last-write identities for its files. All capability, constitutional, workspace, project and write-set checks admit the actual operations. The final failed-build recovery is reached after repair gives up, with `asset.dat` still containing the attempted version. All acquisition, patch-write and restoration operations succeed. There is no external writer, permission failure, process crash, Git race or omitted recorded path.

The general write handler accepts string content, resolves the workspace path, retains existing line-ending behavior, applies its Go/build-exclusion checks and writes the resulting bytes. A regular data-file witness does not require malformed Go source, a raw shell escape or a new binary-specific tool. A future full-path fixture must still establish the actual admission and receipt conditions rather than fabricating permission. [Typed write handler][write]

For either A1 or A2, the source predicts:

1. NUL detection takes the binary branch.
2. The asset section in the saved patch is the same two-byte-before/two-byte-after note. Keep all other file changes equal between the paired cases.
3. `saveAttemptPatch` successfully returns a path.
4. Owned restoration writes B back to `asset.dat`.
5. The returned message names the saved patch as the record of the attempt.

The saved patch's **contents** are identical for both alternative afterimages. A timestamped filename does not encode which content was attempted. Given B and that patch, no deterministic decoder can recover both A1 in the first case and A2 in the second. This is a representation counterexample, independent of a particular model's behavior or a parser's interpretation of the Markdown.

The intended reconstruction condition can be written as design notation:

```text
decode(base, saved_attempt(base, after)) = after
```

The current binary projection violates that condition because distinct `after` values map to the same saved artifact. A created binary file is a related case: its section gains a creation note, then successful restoration removes the file. A binary-to-text replacement also takes the binary branch because the **before** side contains NUL. These variants strengthen the supported-domain question, but the existing-file two-byte case is sufficient.

The witness concerns the final state present when this recovery runs. It does not promise a history of every intermediate repair edit; earlier backtracking can already have returned files to an episode snapshot. Retaining all intermediate attempts would be an additional requirement.

## What does and does not supply another recovery route

**Preimages and hashes:** `PreImage.Content` preserves the original bytes. `LastWriteHash` helps identify the owned current version; it is not an encoding from which the attempted bytes can be decoded. Restoration updates that identity to its own restored version. [Preimage record][preimage]

**Returned file observations:** `withTurnWrites` reads the current disk state at return and iterates only the remaining `WrittenPaths`. It does not archive every pre-restoration afterimage. The successful-restoration path above removes those paths before this projection can serve as such an archive. [Return projection][return-writes] [Path retirement][restore]

**Effect and repair receipts:** tool-effect receipts carry before/after identities and validation/recovery state, not content. The repair record carries outcomes, cost, filenames, tool outcomes and failure excerpts, not a file-body archive. Those are appropriate audit records; making them larger is not the only possible repair. [Effect receipts][effects] [Repair record][repair-record]

**Transcripts and independent backups:** a complete retained tool-call argument, external backup or another snapshot may allow recovery of a particular attempted file. This review does not prove those sources absent. They are not referenced or verified by `saveAttemptPatch`, and replaying an original tool operation is not necessarily equivalent to restoring the observed final bytes. The conclusion is therefore not “all binary work is irretrievably lost.” It is “the named recovery artifact does not preserve the supported afterimage it is presented as preserving.”

**Conflict protection:** if another writer changes the file and the last-write comparison detects it, restoration leaves that file in place. That control breaks the overwrite part of this witness and must remain. Likewise, unknown preimages refuse restoration, a compiling workspace is left alone, and a non-Go root does not enter this recovery. The finding does not override any of those boundaries.

**Save failures:** the current writer explicitly allows restoration to proceed when it cannot save the patch. That is an existing buildability-first policy, not the cause of the successful-save witness. Storage failure should have an explicit preservation outcome, but this report does not count that separate known behavior as an additional discovery. [Save policy][save]

## Relationship to prior work and primary sources

- The diff architecture's closed binary-visibility work ensures a repair prompt mentions a binary edit. Its bounded-diff work ensures text omitted from a prompt is disclosed and the saved text patch has no prompt cap. Those are useful implemented controls. Neither establishes reconstructibility after the live attempted bytes are removed. [Diff queue][diff-todo] [Binary visibility test][binary-test] [Text budget test][budget-test]
- [Tool effect receipts and validation completeness][prior-effects] separates permission, mutation and validation; [effect recovery closure][prior-closure] separates historical receipts from current obligations. This report assumes the write is recorded and compensation is properly owned. The gap is the content representation saved before that compensation.
- [Recurse compensation write ownership][prior-recurse] concerns whether outer cleanup can undo unrelated writes. Here the ordinary session restores its own uncontested file, with no recurse wrapper or competing user operation.
- [Recoverable delete admission][prior-delete] concerns proving Git can restore the **current pre-deletion** bytes before granting a deletion exception. Here permission is already valid and original bytes are known; the missing object is the **attempted afterimage** after automatic build recovery.
- [Verifier change evidence coverage][prior-verifier] concerns a judge's access to retained before/after evidence. This report concerns a person-facing recovery artifact after the workspace was restored. No judge or progressive-disclosure request is needed.

Targeted searches for `turnDiffPatch`, `saveAttemptPatch`, `leaveBuildableTree`, `.patch.md` and binary recovery found no matching audit of this composed preservation boundary. This is a bounded overlap check, not a claim about all historical branches or unpublished work. The report does not claim to newly discover general diff limitations or the value of snapshots.

The technical distinction has established precedents. Git's own manual distinguishes human-oriented one-way text conversions from applicable patches, and separately documents binary diff output that `git apply` can consume. This supports keeping presentation and reconstruction contracts separate; merely switching to a Git command would still need exact scope, base identity, untracked-file handling, filter policy and verification. [Git diff manual][git]

Saltzer, Reed and Clark's *End-to-End Arguments in System Design* (1984), in its careful-file-transfer example, places the final integrity check at the application endpoints, including reading the destination copy and comparing its checksum before acknowledging success. Applied here, that is an architectural analogy: successfully writing a summary is not proof that the preserved artifact can reproduce the observed file. The paper does not establish this CodeNERD bug or require a particular storage format. [Primary paper][end-to-end]

## Proposed preservation contract

The following is proposed design, not a shipped API or new authorization.

1. **Separate the two products.** Keep `turnDiffSection` as a bounded, readable review projection with its current binary and omission notices. Produce the recovery artifact from a frozen set of owned file states, not from those display sections. A human-readable report can accompany the exact data.
2. **Declare reconstruction scope.** For every included regular file, record canonical workspace-relative identity, existence, final bytes or a lossless delta with its required base, content digest, and capture status. A deletion is an explicit absent state, not an empty payload. Define mode and other metadata support rather than implicitly promising full filesystem restoration. Unsupported directories, symlinks or other objects require an explicit outcome.
3. **Bind the artifact to the attempted version.** The captured state must be the version the turn owns and proposes to replace, not a later foreign version. Preserve existing conflict checks and use an appropriate coordination mechanism for capture and compensation. A hash read followed by an uncoordinated write does not create an atomic compare-and-swap guarantee against arbitrary external writers.
4. **Make completeness machine-checkable.** Enumerate the intended capture set and every successful, absent, unavailable or unsupported member. Do not let an unreadable file silently disappear while the aggregate artifact is labeled complete. An artifact containing only metadata is a record of a change, not preservation of its content.
5. **Verify before claiming preservation.** For the promised durability class, publish the payload and manifest using a defined commit protocol, then verify identity and reconstruction through the supported reader. Distinguish artifact publication, crash durability, successful compensation and successful later recovery. A digest detects wrong bytes only when the bytes remain available.
6. **Give failures an executive outcome.** The host reports capture, publication and restore results. Mangle derives the appropriate preservation/hold/recovery obligation under a documented policy. If buildability-first restoration is retained when exact capture fails, report that failure explicitly and do not claim the attempt was preserved. A policy that instead requires quarantine or a hold is a substantive decision to make before implementation, not a hidden consequence of this audit.
7. **Keep recovery bounded and authorized.** Bound retained artifacts by an explicit lifetime and storage policy that does not silently invalidate a newly promised artifact. Preserve workspace access controls and prevent a new raw-content archive from broadening disclosure or retaining sensitive source indefinitely. Restoration from an archive must recheck destination identity and current permission; an old archive is not authority to overwrite later user work. Do not replay a model/tool effect to recreate missing bytes.

Two reasonable implementation candidates are a versioned manifest with raw afterimage blobs, or a proven lossless patch format with retained exact bases. Full afterimages simplify binary and missing-final-newline recovery; deltas can save space but add decoder, base-lifetime and corruption dependencies. A proven existing snapshot service could be reused if its scope, availability and lifetime satisfy this contract. The finding does not require a new database, model call or changes to the prompt budget.

## Evaluation plan with no runs claimed

Start with deterministic host fixtures and scripted tool responses. A fabricated `ExecutionResult` is useful for a focused unit witness, but the acceptance case must also use the registered write path, ordinary verification/repair exit, save and restoration. Do not grant permission inside the fixture in a way the normal host never would.

| Case | Required observation |
| --- | --- |
| Same-length binary A1/A2, fixed base | Current artifacts collide as predicted; a proposed exact artifact reconstructs the respective bytes after restoration. Compare byte arrays, not just names or marker text. |
| Created binary file | Recovery preserves its attempted bytes and existence even though compensation removes the live path. |
| Binary-to-text and text-to-binary | Both input classifications retain the exact attempted final state. |
| Text with or without final newline, repeated blank lines, CRLF and non-ASCII | Recovery reproduces exact bytes. Existing “some changed lines exist” assertions are not round-trip evidence. Do not count earlier documented text-diff limitations as new discoveries. |
| Deleted file and existing empty file | Recover the attempted absent state separately from an attempted empty file; retain the declared base and metadata semantics. |
| Unknown preimage | Existing refusal remains; no invented empty base or destructive compensation. |
| Foreign write before capture or restoration | Preserve foreign work and report conflict; no archive may silently adopt that version as the turn's. |
| Successful capture with write/readback/corruption failures | Preservation status reflects the exact failed stage, and compensation follows the specified failure policy. |
| Failure or crash between payload, manifest and compensation | Recovery identifies complete versus partial artifacts without fabricating a preserved attempt. This requires separately authorized fault injection. |
| Build passes, verification indeterminate/canceled, non-Go root | Preserve existing no-restoration controls. A passing test suite is not required merely to exercise file serialization. |
| Text-only normal attempt and bounded repair prompt | Existing readable diffs, omission notices and prompt byte ceilings remain; exact archival data is not injected into the model window. |
| Later user edit followed by requested archive restoration | Refuse or reconcile stale destinations under current authority; do not overwrite merely because the archive is valid. |

The decisive oracle is reconstruction of the captured attempted state in a separate temporary destination, with exact bytes, existence and every metadata field the chosen format claims to preserve. Then verify the original workspace's compensation result independently. An archive test that checks only that a file exists, contains the Go error token, or lacks `diffTruncatedMarker` is insufficient.

Inspecting present tests found useful but narrower coverage: the buildable-tree test checks restored text, removal of a created Go file, one saved patch and a text token; the binary test checks the display marker; the budget test checks that the text patch reaches its last generated line. None of those assertions establishes the binary reconstruction property. This is a description of the read test bodies, not a claim that the suite was run or that no other test exists. [Recovery test][recovery-test] [Binary test][binary-test] [Budget test][budget-test]

## Full costs and fair comparison

Let `n` be the number of captured paths, `A` the total attempted afterimage bytes, `B` the required base bytes, and `D` the existing rendered Markdown size. The current route already reads the attempted files into memory, renders the display sections, writes `D`, reads current files for restoration checks and writes preimages. Binary output can be tiny even when `A` is large; that is information omitted from the archive, not a measured efficiency improvement. Text diff computation, copying, sorting and caching also consume resources beyond the final file size.

A raw-afterimage design requires approximately `A` plus manifest and filesystem overhead for retained content, and ordinarily additional write and verification-read I/O proportional to `A`. Streaming can bound transient memory but does not remove disk bytes, hashing or publication work. A delta design trades retained base storage `B`, delta bytes and decoder work against those costs. Deduplication saves repeated content only if reference tracking and retention preserve every needed blob. Compression adds CPU and can expand incompressible small inputs; benchmark rather than assuming a saving.

Use the same owned changes, repair policy, verifier configuration and failure schedule when comparing the current Markdown path, a lossless snapshot candidate, and a scoped binary-capable patch candidate. Keep prompt budgets and model authority equal. A “never restore” variant avoids losing the live afterimage but fails the buildability objective; a “never save” variant is smaller but fails preservation. Neither is an equivalent successful baseline.

Report separately:

- exact reconstruction and capture-completeness rates, unresolved/unsupported files, foreign-write protection and compensation results;
- capture/publication/reconstruction latency distributions, bytes read and written, peak memory, CPU, hashing/codec time, filesystem synchronization and contention;
- retained storage over the full artifact lifetime, garbage collection, orphan cleanup, failed publication and repair/migration work;
- complete task costs: initial and repair model input/output/reasoning and cache-token classes where known, tool calls, build/test work, failed or canceled attempts, user recovery time and any later regeneration;
- no-fault overhead separately from fault-conditioned recovery benefit, with every attempted case and failed preservation in the denominator.

The minimal repair needs no additional inference. It can leave the model-facing diff unchanged and add a small recovery reference/status to the result. Any claimed token benefit would come from avoiding later recovery or regeneration and must be measured over completed tasks, not inferred from smaller archives. No resource dimensions should be summed without a declared common unit and pricing assumptions.

## Acceptance boundary

Close this gap only when the normal final-build recovery route can preserve and reconstruct the supported attempted file states, reports incomplete preservation honestly, and retains the existing permission, attribution, verification and conflict controls. Keep the readable `.patch.md` if useful, but make it a view of verified recovery evidence rather than the only preservation mechanism.

The source-supported conclusion is narrow and practical: **an unbounded display diff is still a lossy representation, and successful publication of it does not prove that the attempt being undone was saved.**

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/895e234f638e892904067a636c42303e85db4d50
[root]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/agents.md#north-star
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/agents.md
[recovery-rationale]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/buildable_tree.go#L14-L29
[recovery]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/buildable_tree.go#L30-L74
[save]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/buildable_tree.go#L77-L104
[renderer]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/turn_diff.go#L75-L122
[binary]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/turn_diff.go#L223-L256
[engine]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/diff/diff.go#L240-L263
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/architecture/diff/WIRING-AND-NOT-BUILT.md#L20-L46
[build-repair]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/build_verify.go#L241-L324
[repair-exit]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/repair_loop.go#L362-L380
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/change_gates.go#L587-L668
[preimage]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/preimage.go#L13-L112
[write]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/tools/core/file_ops.go#L363-L432
[return-writes]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/turn_writes.go#L11-L45
[effects]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/observation/effects.go#L8-L39
[repair-record]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/repair_loop.go#L22-L72
[recovery-test]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/buildable_tree_test.go#L28-L69
[binary-test]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/turn_diff_test.go#L60-L79
[budget-test]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/internal/session/turn_diff_test.go#L106-L158
[diff-todo]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/architecture/diff/TODO.md#L12-L31
[prior-effects]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/audits/tool-effect-receipts-and-validation-completeness.md
[prior-closure]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/audits/effect-recovery-closure.md
[prior-recurse]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/audits/recurse-compensation-write-ownership.md
[prior-delete]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/audits/recoverable-delete-index-assumptions.md
[prior-verifier]: https://github.com/theRebelliousNerd/codenerd/blob/895e234f638e892904067a636c42303e85db4d50/Docs/audits/verifier-change-evidence-coverage.md
[git]: https://git-scm.com/docs/git-diff
[end-to-end]: https://web.mit.edu/Saltzer/www/publications/endtoend/endtoend.pdf
