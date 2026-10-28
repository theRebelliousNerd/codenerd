---
doc-class: deep-dive
subsystem: tools-codedom
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Preserve observed-edit conditions through CodeDOM batching

**Decision:** retain CodeDOM's existing bounded batch editor, syntax checks, optimistic commit checks, and effect receipts. Make a supplied file-read precondition survive the batch adapter. An edit conditional on what the model read must not become an unconditional edit merely because it was grouped with another file.

**Status:** one source-supported adapter finding, plus a separately scoped concurrency limitation. This is a research/design document, not an implementation or reproduced incident. No repository code was changed; no tests, model inference, training, benchmarks, or failure injection were run. The witness sequences and evaluation program below are proposed, not observed outcomes.

## 1. Scope, source freeze, and non-duplication

The user journey is: a normal coding turn reads a file, receives an `obs:fr:` observation handle, proposes line edits, batches distinct files through `apply_edits`, and receives the committed result. The gap is between the supplied observation condition and the batch's private staging representation—not lexical discovery, structural use discovery, or delegated-task settlement.

Source inspected on 2026-10-28:

| Branch | Pinned revision | Qualification |
|---|---|---|
| `main` | `356f65e0e6bb639adaa90ab36212daa714114871` | Primary executable baseline; citations below use this revision unless stated otherwise. |
| `feat/conversation-brief` | `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c` | The fetched `apply_edits.go`, `lines.go`, `commit.go`, and observation `precondition.go` bodies are identical to main. The conversational changes do not repair this adapter. |
| `codenerd-research-docs` | `b802db97f53b913a9995370b9d44d2784fd3d918` | Tree blob identities match main for `apply_edits.go`, `lines.go`, observation `precondition.go`, and `atomicfile.go`. Other executable files differ; do not treat this research branch as a runtime superset of main. |

Read guidance includes root [agents.md][guidance], [session guidance][session-guidance], and [tool boundaries][tool-guidance]. The North Star requires revision-aware world/CodeDOM context, typed policy-mediated operations, and executive decisions owned by Mangle. This proposal neither offers a shell workaround nor expands capability or permission.

Related work was read before selecting this seam:

- [Structure coverage and refactor preconditions][structure-study] concerns structural enumeration, negative evidence, use offsets, and read dependencies of destructive structural operations. Its **As built** section records the implemented `Digest`/`Text` use witnesses and `readDependency` checks. Those are credited here, not reported as missing. A caller's earlier `read_file` observation handle is a different dependency from a fresh index use set.
- [Workspace search/navigation fidelity][search-study] concerns search projection and redeemable navigation addresses, before edit admission.
- [Tool effect receipts and validation completeness][effect-study] concerns the state left by an attempted effect and later validation. Its **As built** section records receipts and post-validator accounting already implemented on main. They do not restore an explicit precondition dropped before execution.
- [Request-visible tool contracts][catalog-study] concerns request catalogs and handler identity. This document concerns a known registered handler's transformation of an argument promised by its same-arguments description.
- [Delegated-turn failure settlement][delegation-study] concerns terminal ownership and recovery, not conditional file mutation.

## 2. Existing capabilities to preserve

| Boundary | Source evidence | Existing guarantee and limit |
|---|---|---|
| Production registration | [CodeDOM `RegisterAll`][registration]; [modular hydration][hydration]; [`factory.go:1657`][factory] | The line verbs and `apply_edits` are registered through the normal modular family. This is not a missing-tool proposal. |
| Production mediation | [`executeToolCall`][dispatch], especially 2570–2596 and 2630 onward | Effectful calls require executive preflight before modular dispatch. Registration and an observation handle are not authority. |
| Observation retention | [`Store`, `Mint`, and `Verify`][precondition-store] | A shared retained-read store preserves the original bytes. `Verify` receives candidate bytes rather than independently reopening the file, and binds the handle to the original canonical absolute path. |
| Region-sensitive protection | [`Verify`][precondition-verify]; [`Enforce`][precondition-enforce] | A changed/moved observed region refuses the edit. An intact observed region with other file changes proceeds with a caveat. Expired, unknown, and wrong-file nonempty handles refuse. Omission is currently permitted. |
| Single-file consumption | [`edit_lines`][line-edit], `insert_lines` at 439–468, `delete_lines` at 555–587 in [lines.go][line-file] | Each line verb calls the shared precondition enforcement against its read bytes before writing. All three expose the `precondition` argument. |
| Bounded batch planning | [`ApplyEditsTool` and parsing][batch-parse]; [snapshots and staging][batch-stage] | Two to sixteen distinct existing files; strict integer addresses; one edit per canonical path; aggregate supplied content limit of 1 MiB; staging and syntax checks precede workspace commit. The content limit is not a bound on total existing source bytes copied. |
| Batch commit protection | [`commitFiles`][commit] | Rechecks planned original bytes under `commitMu`, writes through atomic replacement, and performs best-effort rollback. The coordinator also checks supplied structural `readDependency` callbacks, but `apply_edits` calls it with no such callbacks. Its batch snapshot check protects changes after that snapshot, not automatically the model's earlier observation. |
| Effect accounting | [`executeAndRecordToolCall`][accounting]; [`settleToolEffects`][effects] | Current session code observes effects and settles receipts on terminal paths, including post-validation failure. Preserve this rather than adding a second success-only write ledger. |

`obs:fr:` is a **file-read observation handle**. CodeDOM element `rev` is a separate element revision mechanism ([`checkRevision`][element-revision]). Neither should be silently substituted for the other. A current element body does not establish that an earlier whole-file inference remains current, and a file-read handle does not by itself prove structural use completeness.

The current `Enforce` contract checks the region retained by the read, not whether the requested mutation range lies inside that region. This study does not attribute that additional coverage guarantee to existing code. The witness below deliberately edits the observed region.

## 3. Finding E1: `apply_edits` silently drops a supplied read precondition

The public description says entries use the same arguments as their single-file line verbs ([`apply_edits.go:25`][batch-parse]). Its machine-readable schema only declares an array of generic objects, without explicit nested `precondition` properties. The parity expectation therefore comes from that description and the direct verbs' documented argument—not an explicit batch-entry schema. The batch implementation then:

1. Copies operation, path, line addresses, and content into `parsedEdit`; the type has no precondition field and parsing never reads the supplied handle ([lines 42–52, 111–165][batch-parse]).
2. Reads current workspace bytes into `snapshot.orig` ([lines 175–221][batch-stage]).
3. Constructs fresh maps for the staged line calls, including only path, addresses, and content. None includes `precondition` ([lines 243–269][batch-stage]).
4. Discards staged success strings and returns only `changed` and `operations` after commit ([lines 252–269, 292–307][batch-stage]).

`Enforce` explicitly treats absent/empty handles as no condition. Thus a validly shaped nested entry with a nonempty stale, expired, or wrong-file string handle is not checked by the batch even though the single-file verb would refuse it. This conclusion follows from the handler's argument flow; it is not an inference from a comment or absence of a test.

The optimistic commit check cannot recover the missing semantics. It compares workspace bytes against the **batch's later snapshot**. If the source changed after the model's read but before the batch began, those changed bytes become `orig`; a stable workspace during staging satisfies that check. A syntax check can establish that the replacement parses, not that it was applied to the bytes the caller conditioned it on.

### Unexecuted minimal witness

1. Through the ordinary read route, observe a line such as `const Limit = 10` in `a.go`, retaining its `obs:fr:` handle.
2. Another authorized actor changes the same line to `const Limit = 20` before batch execution. Let the workspace then remain still.
3. Submit two otherwise valid, authorized `apply_edits` entries: replace that line in `a.go` with `const Limit = 30`, carrying the original handle; make an independent valid change in `b.go`.
4. Compare the corresponding single-file call against the batch. The single-file condition should refuse the changed observed region. The current batch's parsed representation contains no handle, so it can stage and commit both edits provided the independent guards pass.

The intended corrected outcome is a precondition refusal naming the batch entry and original path **before any workspace file changes**. This witness deliberately uses parseable source and no concurrent execution during the batch: neither successful parsing nor commit serialization is an answer to E1.

This is conditional-edit integrity, not a claim that all unconditional overwrites are forbidden. Existing callers may omit the optional precondition. The defect is silently weakening the condition a caller supplied.

## 4. Related limitation: atomic replacement is not a serialized read–modify–write

The ordinary line verbs read the whole file, check the optional observation, construct a whole-file candidate, then call `atomicfile.WriteFilePreservingMode` without `commitFiles` ([`edit_lines`, lines 94–157][line-edit]; [other verbs][line-file]). `atomicfile` has striped per-path locks, but they begin inside `WriteFile`, after the caller already read and transformed its snapshot ([`atomicfile.go:121–177`][atomic-lock]). This prevents competing replacement operations from colliding inside that helper; it does not validate an earlier read or merge two independently computed candidates.

A conditional schedule can therefore be: line writer A reads and validates revision r; cooperating writer B commits r→rB; A later atomically replaces the file with a full candidate computed from r. A has no final unchanged-byte check on that path. This remains possible when A and B intend different declarations in the same file, because each candidate contains the rest of the old file. The schedule is not reproduced here.

`commitMu` covers the operations that actually call `commitFiles`, not every operation located in the `codedom` package. Its own description lists batch, element, create, and repoint operations ([`commit.go:17–20`][commit]). The structure audit already defers coordination with writers outside that protocol; this section narrows that boundary: ordinary registered line verbs are also outside it. It is **related scope clarification**, not a second claim that CodeNERD lacks all concurrency protection.

Do not infer that calls inside one native batch execute concurrently. The session executes its tool calls through its accounting path in sequence. Async tasks do exist ([`JITExecutor.executeAsyncInternal`][async]; [`SubAgent.Run`][subagent]), and the campaign package has its own [write-set lock manager][campaign-lock]. Those upper-layer locks may exclude particular schedules. This note does not establish that every production task pair overlaps or that campaign locking is absent. It identifies the lower-level contract to qualify when a path can be reached by overlapping authorized writers.

An operating-system editor, another CodeNERD process, a formatter, or any writer outside the chosen protocol remains a distinct limitation. A process-local mutex must never be presented as universal filesystem compare-and-swap or multi-file isolation.

## 5. Proposed repair contract

### 5.1 Minimal E1 repair: preserve identity before relocating bytes

The smallest repair does not need a new source model, another index, or a replacement transaction manager.

1. Parse the optional `precondition` of every batch entry into the canonical typed edit. Reject a supplied unsupported type or malformed supported field rather than silently erasing it; align the corresponding direct-tool contract deliberately if stricter validation changes compatibility.
2. Resolve the original workspace path using the existing canonical containment path.
3. Once the original snapshot bytes are read, enforce each supplied handle against **that original absolute identity and those exact snapshot bytes**, before staging any candidate workspace writes. Use the shared `precondition` package so changed-region, wrong-file, expired-handle, and intact-region caveat semantics stay aligned with direct calls.
4. Only then transform the validated snapshot under the private staging root. Do **not** naïvely forward the original handle to the staged absolute path: `Verify` rightly treats it as another file. Do not mint a replacement handle from staged bytes, because that would certify the adapter's new read rather than preserve the caller's old observation.
5. Keep the original byte snapshot and existing optimistic check through commit. An observed region that was valid before staging does not authorize overwriting a newer whole-file revision at commit.
6. Return per-entry observation status/caveats with the batch result. A valid region with unrelated file changes is not an error, but its caveat must not disappear because the staged success string is discarded. Keep bounded machine-readable structure and a concise user-facing summary; names below are design notation, not a shipped API.

An illustrative result entry has `path`, `operation`, `precondition_state` (`not_supplied`, `satisfied`, or `satisfied_with_caveat`), and a bounded caveat. Refusal identifies the failing entry and reason; successful entries are not claimed committed when another entry failed preflight. Never label `not_supplied` as a verified observation.

No authority is gained by a valid handle. Capability, exact payload mediation, workspace containment, `nerd.md` protection, syntax checks, and post-edit obligations remain intact. Mangle continues to own executive permission and completion. The host's precondition verification supplies evidence or a refusal; model prose cannot override it. Adding new kernel predicates is unnecessary unless a specified consumer requires them.

### 5.2 Qualify and, if needed, unify the cooperative commit boundary

Treat this as a separately reviewable change, not a prerequisite for fixing E1. Prefer adapting single-file plans to the existing optimistic commit boundary over adding a second lock hierarchy. Preparation can remain outside the critical section; final original-byte validation, conditional mutation, and compensation for cooperating writers must share the same ownership boundary. Preserve scope/path identity and reject stale candidates rather than retrying the effect blindly.

If a process-wide lock proves too costly, evaluate a workspace-keyed or sorted canonical path-set coordinator. Do not ship fine-grained locking without a deadlock order, cancellation behavior, ownership lifetime, and integration with structural read dependencies. A structural read dependency can span files not written by the plan; path-only write locks do not automatically protect that predicate.

Retain the current warning that external writers are not coordinated. Stronger guarantees require a genuine shared protocol or immutable workspace snapshot, not an extra hash read followed by an uncoordinated rename. Preserve best-effort rollback reporting and current effect receipts for partial or uncertain outcomes.

## 6. Primary sources and limits of transfer

- [RFC 9110 §13.1.1, If-Match](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1) gives a precise precedent for a supplied condition being evaluated before a mutation; a false condition must not silently become an unconditional operation. The application here is a local tool contract, not a claim CodeDOM implements HTTP or should replace region-sensitive observations with ETags.
- [Kung and Robinson, *On Optimistic Methods for Concurrency Control* (1981), §§2–4](https://www.eecs.harvard.edu/~htk/publication/1981-tods-kung-robinson.pdf) separates private preparation, validation, and publication, and explains serial validation together with the write phase. The applicable insight is continuity between the read evidence and the actual publication boundary. Database assumptions and serializability do not automatically hold for arbitrary filesystem writers, and the paper does not establish CodeNERD performance.

These are design foundations, not novelty claims or measured speedups.

## 7. Costs, alternatives, and capability value

The useful outcome is fewer stale conditional edits and fewer repair turns at unchanged authorization and task quality—not merely fewer tool calls.

Compare:

| Variant | Intervention | Why evaluate it |
|---|---|---|
| A | Current batch and direct tools, including all existing gates, syntax checks, atomic replacement, commits, and receipts | Honest baseline, not an intentionally weakened editor. |
| B | Preserve and verify original batch-entry preconditions; retain per-entry caveats | Smallest repair for E1. Can validate using existing snapshot bytes without another live file read. |
| C | B plus a shared cooperative commit boundary for ordinary line mutations | Separately addresses the post-read publication window when overlapping writers are permitted. |
| D | C with measured workspace/path-set coordination rather than global serialization | Only justified by contention evidence; added deadlock/ownership complexity can outweigh throughput gain. |

For n entries and B total original bytes, the present batch already copies/stages and rereads source and writes replacements. Observation verification adds retained-record retrieval and scanning/hashing bounded by observed/current bytes, not model inference. The supplied-content cap does not cap B. Reuse each original snapshot buffer for verification; do not introduce an extra reopen per handle. Retained observations already cost space and can expire; refusal on expired handles must remain explicit, with bounded retention rather than pinning every read indefinitely.

Per-entry evidence adds O(n) result metadata and some context tokens, with n already limited to 16. Long caveats or old file bodies must use the existing recoverable-output mechanism rather than unbounded echoing. A broader cooperative coordinator adds lock wait, snapshot revalidation, and potentially conflict/replan costs. The current global `commitMu` can serialize independent workspaces; making more paths join it improves coverage but may expose that contention. Do not assume C is free or D is faster.

Record separately: verified-task rate; rejected stale conditions; unexpected overwrites; false refusals; lost caveats; retry/repair turns; model input/output/reasoning tokens including failures; wall time; bytes read, hashed, staged, and written; parsing CPU; peak source/retention memory; commit-lock wait and hold distributions; rollback and reconciliation work; human intervention. Cost per verified task must include abandoned and unresolved runs. Do not add unlike units without explicit prices.

## 8. Evaluation program — not executed

### Deterministic boundary qualification

| Proposed fixture | Required outcome |
|---|---|
| Two valid entries, fresh original handles | Both changes commit; no wrong-file error from the temporary root; each result identifies the original path. |
| Changed observed region before batch starts | Refuse the whole batch before workspace mutation, even if all candidate source parses. |
| Region moved above/below original coordinates | Same stale-region refusal as direct call; any relocation hint remains advice, not automatic retargeting. |
| Wrong-file, expired, unknown handle | Refuse rather than omit or mint a fresh staged handle. |
| Intact observed region; unrelated file bytes changed | Preserve the direct tool's caveat; do not falsely reject every unrelated edit or claim the whole observation is current. |
| One omitted handle and one valid supplied handle | Preserve documented optionality and distinguish unchecked from checked entries. |
| Supplied non-string or malformed field | Explicitly specified handling; no silent downgrade to omitted protection. |
| Changed workspace bytes after accepted batch snapshot | Existing commit conflict path still refuses or accurately reports rollback; E1 repair must not weaken it. |
| Concurrent direct line edit and batch/element edit of same file — variant C qualification, not a B exit | At an injected read/commit seam, preserve the winner or reject stale candidate; never report both retained changes when one was overwritten. Scope this result to participating writers. Minimal B repairs pre-batch observation continuity and does not close the direct line writer's later publication window. |
| Disjoint files/workspaces | Correct results without unnecessary conflicts; measure coordination wait rather than assuming zero cost. |
| Cancellation during staging or waiting for commit | No unowned background mutation; accurate outcome/effect accounting if mutation began. |
| Partial commit failure or conflicting rollback | Preserve existing best-effort semantics and per-resource receipts; never report universal atomicity. |
| Windows path aliases, symlinks, CRLF, secret/project-protected paths | Original identity and existing containment/protection semantics remain intact; no identity rewritten to the staging root. |

Drive the first stale-handle fixture through normal registry/session dispatch with the real executive adapter and a real read-generated handle, not only a direct helper call. Preserve the actual conditional argument through provider-visible request shape. Existing [line precondition fixtures][line-tests] already define useful positive/negative direct-tool controls; their presence is source evidence only, not a claim they were run here. Add batch parity checks and deterministic seams instead of timing-dependent sleeps.

For the concurrent qualification, separately report same-process participating writers, non-participating same-process writers, and external-process writers. Do not claim the strongest category from the weakest fixture. Campaign write-set locking and sequential native execution should be positive controls where they actually prevent overlap.

### Production task study

Use paired tasks with fixed source snapshots, model/version, context budget, effective tool catalog, authority, and held-out behavioral acceptance criteria. Include multi-file configuration changes, two independent declarations in one file, mid-turn formatter/user edits, long reads whose observation handles expire, and no-interference controls. Randomize A/B initially; add C/D only after defining the authorized overlap domain. Run through the ordinary TUI/session production route with world/CodeDOM enabled.

Report conditional edit integrity and complete task success separately. A refusal that names the stale entry and enables a targeted reread is preferable to a silent overwrite, but is not itself completed work. Likewise, fewer failures obtained by rejecting every edit is not a capability gain. Measure how often the model recovers, how much context is reread, and the total verified-work cost.

### Checkable exits

- **EDIT-COND-01:** every supported supplied batch precondition is preserved, evaluated on original identity/snapshot bytes, and cannot be silently downgraded.
- **EDIT-COND-02:** direct and batch outcomes agree on stale/expired/wrong-file rejection and intact-region caveats; staging identity never substitutes for observation identity.
- **EDIT-COND-03:** cooperative publication guarantees list actual participating mutation paths, and deterministic overlap checks match that scope.
- **EDIT-COND-04:** a production-entry paired study reports task quality, refusal recovery, full costs, and uncertainty; retain B if broader coordination complexity lacks demonstrated value.

Before implementation, reconcile owning tools/session specifications under the repository's spec-first rule. This document does not authorize implementation, repository changes, model calls, or benchmark execution.

## Source references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md
[tool-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/agents.md
[structure-study]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/structure-coverage-and-refactor-preconditions.md#as-built-2026-10-27
[search-study]: https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/workspace-search-navigation-fidelity.md
[effect-study]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/tool-effect-receipts-and-validation-completeness.md#as-built-2026-10-27
[catalog-study]: https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md
[delegation-study]: https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/delegated-turn-failure-settlement.md
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/register.go#L29-L38
[hydration]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store_tools.go#L41-L81
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1657-L1659
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2570-L2654
[precondition-store]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/precondition/precondition.go#L164-L230
[precondition-verify]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/precondition/precondition.go#L230-L311
[precondition-enforce]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/precondition/precondition.go#L448-L505
[line-edit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/lines.go#L94-L166
[line-file]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/lines.go#L439-L595
[batch-parse]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/apply_edits.go#L21-L165
[batch-stage]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/apply_edits.go#L175-L307
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/commit.go#L17-L125
[atomic-lock]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/atomicfile/atomicfile.go#L121-L177
[element-revision]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/element_edit.go#L174-L183
[accounting]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L978-L1060
[effects]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L213-L285
[async]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L451-L499
[subagent]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/subagent.go#L270-L310
[campaign-lock]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/campaign/write_set_lock_manager.go#L24-L86
[line-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/lines_precondition_test.go#L63-L110

## As-built status

Verified and built 2026-10-28 on `main` (after `c28a1212`). Every claim was re-read against current source; the adapter and line-verb files were unchanged from the audited revision.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| E1: `apply_edits` parses no `precondition`, so a supplied stale/expired/wrong-file handle is silently dropped | Real, fixed | Before: `parsedEdit` had no field and the staged verb maps omitted it. Now parsed at `apply_edits.go:133`, a non-string handle is refused, and `precondition.Enforce` runs on the original absolute path and snapshot bytes at `apply_edits.go:247`, before any entry is planned |
| The batch's optimistic commit check compares against its own later snapshot, not the caller's read | Real; unchanged and still needed | `commit.go` `commitFiles` re-checks `plannedWrite.orig`; E1 is closed by the check above, not by this one |
| The staged success strings (and any caveat) are discarded | Real, fixed | Per-entry `preconditions` with `not_supplied` / `satisfied` / `satisfied_with_caveat` and the caveat (`apply_edits.go:311`) |
| Forwarding the handle to the staged path would be refused as another file; minting a fresh one would certify the adapter's read | Confirmed; avoided | The staging directory is gone: entries are planned in memory against the snapshot (`apply_edits.go:280`) |
| §4: line verbs read, transform, then `atomicfile.WriteFilePreservingMode` outside `commitMu`, so a cooperating codedom commit after the read is overwritten by the stale whole-file candidate | Real, fixed for codedom writers | `publishLineEdit` -> `commitLineEdit` -> `commitFiles` (`lines.go:619`, `commit.go:57`); the read bytes are re-checked under `commitMu` (`commit.go:90`) |
| `atomicfile`'s striped locks begin inside `WriteFile`, after the caller's read | Confirmed; not changed | `atomicfile.go:167` |
| Native batch calls in one turn execute sequentially; campaign write-set locks exist above | Confirmed; not changed | `executor_tools.go` accounting path; `campaign/write_set_lock_manager.go` |
| External editors, formatters and other processes are outside every in-process protocol | Confirmed; stated in the code comment and spec | `commit.go` `commitLineEdit` doc |
| `apply_edits` recorded changed elements during staging, before (and regardless of) commit | Found while building, fixed | `RecordChangedSource` now runs per write only after `commitFiles` succeeds (`apply_edits.go:300`) |

### What was built

- Ruling: the batch's caller condition is evaluated on the original identity and the exact bytes the batch transforms, and the batch stops staging through a temp directory. The three line verbs were split into pure planners (`planEditLines`, `planInsertLines`, `planDeleteLines`) and one publisher; `apply_edits` reuses the planners, so direct and batch validation (range, delimiter balance, Go syntax regression) cannot drift.
- Ruling (variant C): single-file line verbs publish through `commitFiles`, the boundary the batch, element, create and repoint verbs already share. No new lock hierarchy. `errOptimisticConflict` makes the refusal typed; the line verb adds "nothing was written, read again".
- Spec: `Docs/architecture/tools/IMPLEMENTED_SPEC.md` §7.2 rewritten as built.

### Tests

`internal/tools/codedom/apply_edits_precondition_test.go`: the audit's witness (stale handle refuses the whole batch, neither file changes, the direct verb refuses the same handle); unknown, non-string and wrong-file handles refuse; per-entry states including the intact-region caveat; each line verb refuses an interleaved codedom commit instead of overwriting it. The precondition tests and the interleave test were each confirmed failing with the change disabled. `go test ./internal/tools/codedom/ ./internal/tools/` pass.

### Left open

- Non-participating writers (another process, an editor, a formatter): no shared protocol exists; stated, not claimed.
- EDIT-COND-04 (production paired study) and the cost measurements in §7: not run.
- Variant D (path-set coordination instead of the one `commitMu`): no contention evidence; not built.
