# Northstar projection repair must survive an unchanged save

## Finding and decision

**A failed durable Northstar projection can survive the advertised recovery path while the next save forgets that it is pending.** After SQLite and `northstar.json` have adopted vision V1 but replacement of an existing `northstar.mg` fails, the first save correctly reports projection-pending and defers its requested campaign. Reopening the wizard reconciles the equal SQLite/JSON pair but checks only whether the Mangle export exists. An existing V0 export therefore survives. Saving V1 again returns `VisionUnchanged` before any projection work, with no pending projection errors.

The narrow correction is to distinguish **unchanged semantic authority** from **verified projection convergence**. Preserve no-op edits, compare-and-swap admission and atomic file replacement; add an explicit, generation-bound repair/check outcome for already-committed authority. Do not require users to make a fake semantic edit merely to retry a failed export.

This is a continuation of the existing lossless-strategic-editing work, not a claim that its authority transaction is broken or that cross-resource atomicity was ever promised. The initial warning and deferral already work. The residual is their recovery contract.

**Source pin:** main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), rechecked October 30, 2026 at approximately 14:45 UTC. Static source and test-definition inspection only; no application execution, tests, builds, model inference, benchmarks or repository changes. The control-flow conclusion received an independent source challenge. The schedule below is conditional on an ordinary reported export failure, not an observed incident. No unsafe campaign action or lasting stale live-kernel state is established.

## Status, novelty and existing work

Main, repository guidance and `ASTRA_QUEUE.md` were unchanged in this pass. The latest inspected research branch, `research-main-20261030-1417`, tip `6d97defb8901c0320d7561d4f15d4c13ee799bc9`, is four documentation commits ahead of main: campaign refinement scope deduplication, correction-learning answer ownership, manual alignment snapshot/coverage continuity, and working-context NUL recall. None implements this recovery path. The active code/QA tips inspected remained unchanged as well. [Research tip](https://github.com/theRebelliousNerd/codenerd/commit/6d97defb8901c0320d7561d4f15d4c13ee799bc9).

The [lossless strategic editing audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/northstar-lossless-strategic-editing.md) already owns canonical-view preservation, base-bound admission, deeply detached snapshots, explicit projection-pending outcomes and later durable recovery. Its as-built section explicitly leaves cross-process atomicity/outbox work deferred and says a pending projection is repaired by boot reconciliation or a re-save. This note does **not** claim the broad need for an outbox as new. It demonstrates a specific gap in those two advertised existing recovery routes: an existing stale Mangle export is not checked for equality, and an unchanged save does not retry it.

The prior [manual alignment continuation](https://github.com/theRebelliousNerd/codenerd/blob/6d97defb8901c0320d7561d4f15d4c13ee799bc9/Docs/audits/manual-alignment-snapshot-and-coverage-continuity.md) concerns a missing fact-reader dependency and dropped module-coverage metadata. Here the canonical vision is committed and readable, and the issue is convergence of its durable projection. The [effect-recovery closure audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/effect-recovery-closure.md) concerns historical tool receipts incorrectly rendered as current debt; it does not repair Northstar's equality/repair decision.

This is a bounded duplicate check of current guidance, relevant Northstar audit/specification material, source references and recent research inventories through 1417, not a claim to have exhaustively searched every historical branch.

## The production contract already has useful safeguards

The wizard's summary handles `save` and `campaign` through the same `saveNorthstar` function. `commitNorthstarVision` builds a candidate from its retained base, acquires the shared Guardian, binds the parent kernel, and calls `UpdateVisionFromBase`. Successful results rebase the wizard onto the admitted vision. [Summary dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_wizard.go#L873-L880), [canonical commit adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_edit.go#L195-L225).

`UpdateVisionFromBase` validates before effects and invokes `SaveVisionIfBase` for a changed candidate. The store compares authority content inside its transaction rather than trusting the wizard's age. A rejected or stale edit does not proceed to publication. This protects semantic authority and must remain intact. [Update contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/edit.go#L438-L478), [store admission](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/store.go#L208-L230).

After the authority commits, `publishVision` updates the Guardian's cached vision, exports JSON and Mangle, and refreshes kernel facts. Each failed export or kernel write is returned as a projection error. `UpdateVisionFromBase` converts a nonempty error list into `VisionProjectionPending`, not “not saved.” [Publication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L328-L360), [outcome construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/edit.go#L480-L494).

The wizard reports those errors and defers a requested campaign when that outcome is pending. Its actual recovery instruction is to reopen `/northstar` and save again before starting the campaign. Thus this is a supported user-visible recovery path, not a synthetic caller expectation. [Pending display and instruction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_persistence.go#L56-L103).

Atomic writes are also a mitigation, not the defect. `WriteVisionMangle` renders `Vision.ToFacts` and calls `atomicfile.WriteFile`. That helper uses a unique adjacent temporary file, writes and syncs it, and replaces the destination; errors are reported rather than silently accepting a partial result. Its Windows path already retries sharing violations for two seconds. A handle held beyond that interval can still cause an explicit failure, as the implementation documents. This audit does not propose deleting or truncating the old good export to make a retry appear successful. [Mangle writer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/bridge.go#L345-L376), [atomic writer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomicfile/atomicfile.go#L147-L212), [bounded Windows retry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomicfile/replace_windows.go#L26-L30), [retry decision](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomicfile/replace_windows.go#L86-L145).

## Where repair disappears

### Equality of two authorities is treated as enough for the third surface

Opening `/northstar` calls `loadExistingNorthstar`, which obtains `SnapshotVision`. That method calls `Initialize` and then reads the stored authority. Initialization runs `SyncVisionAuthority`, loads vision/state, and attempts a kernel refresh. [Slash-command entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L945-L968), [wizard open](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_edit.go#L26-L49), [snapshot](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/edit.go#L497-L508), [initialization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L170-L220).

In the branch where stored and JSON visions are semantically equal, reconciliation:

1. selects the stored vision;
2. calls `os.Stat` on `northstar.mg`;
3. writes the Mangle projection only if that stat returned an error;
4. returns a no-op result.

It never compares an existing export's bytes or source generation with the current stored vision. Its missing-file repair is real, but does not cover a valid old export left by a failed replacement. [Equal-vision branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/bridge.go#L515-L522).

### The no-op edit result contains no repair status

`UpdateVisionFromBase` returns `VisionUnchanged` immediately when the candidate's content digest equals the stored vision. This deliberately performs no writes and does not call `publishVision`. It supplies no projection errors from a previous attempt. The implementation has correctly answered “is this a semantic edit?” but has not answered “are the committed vision's projections current?” [Equality shortcut](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/edit.go#L454-L480).

The wizard then suppresses its normal projection/knowledge work on unchanged outcomes and defers a requested campaign **only** for the current call's `VisionProjectionPending`. A second `campaign` save can therefore return a campaign-start command without repairing the existing stale export. This is a disappearance of this particular readiness condition, not proof that downstream campaign admission or execution is unsafe. [Outcome handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_persistence.go#L38-L58), [deferral and start](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_persistence.go#L91-L118).

The explicit CLI recovery surface uses the same reconciliation. With a nonnil vision and no-op direction it prints `Already in sync.`, without validating that existing Mangle artifact. Its short description emphasizes JSON versus store, but its long description names JSON and Mangle import/export surfaces. The shared recovery logic should identify the narrower state instead of implying all projections were checked. [CLI sync](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_northstar.go#L590-L628).

## Conditional, sequential witness

No concurrent canonical writer, malformed vision or model response is required.

1. Begin with valid V0 in SQLite, JSON and an existing regular `northstar.mg` export.
2. In the ordinary wizard, make an authorized semantic change to V1 and request `campaign`.
3. Condition on SQLite admission and JSON export succeeding, but the subsequent Mangle replacement failing **before replacement**, leaving V0 at that path. One concrete source-supported environment is Windows with another reader holding this destination without delete-sharing beyond the existing replacement retry window. Release that reader afterward. Other reported pre-replacement I/O failures that preserve the old file have the same state.
4. The first call correctly returns projection-pending, shows the Mangle error, and does not start its requested campaign. V1 remains committed; do not roll it back or call this a rejected edit.
5. Follow the displayed recovery instruction: reopen `/northstar`. SQLite and JSON both represent V1. `os.Stat` succeeds on the old regular Mangle export, so reconciliation does not rewrite it.
6. Save without another semantic change. The candidate equals stored V1, so the result is unchanged, no publication occurs, and the old export remains V0. Choosing `campaign` at this save no longer triggers the wizard's pending-projection deferral.
7. Running the explicit `northstar sync` route over this same equal pair also leaves the existing export untouched and selects its “Already in sync” message.

The pre-replacement failure is an explicit condition, not a claim that every failed atomic write leaves old bytes or that any given platform will encounter it. The Windows example needs no internal writer race; the cross-process file reader is an environmental cause already recognized by the atomic-file implementation. No such handle or filesystem condition was created during this review.

### Important limits and recovery controls

- **The current live TUI kernel may be correct.** Reopening invokes `Initialize`, which attempts `refreshKernelFacts` independently of whether the Mangle export was repaired. It may successfully project V1. The finding is not “all reads still see V0.”
- **SQLite remains V1 and JSON is V1.** Their semantic data is not lost. The first warning remains in history/logs. The defect concerns the durable Mangle projection and the absence of verified recovery status on subsequent calls.
- **A missing export is a different, better-covered case.** The equal-pair branch attempts to recreate it. A successful new semantic edit also reaches publication again. Neither makes the unchanged recovery route reliable for an existing stale export.
- **Campaign behavior has other controls.** The returned campaign-start command still passes through its ordinary setup, observer, risk and execution logic. In particular, `BuildCampaignObserver` binds the kernel and calls `Initialize`, which reads the stored vision and can repair live facts. This audit proves only that the wizard's explicit projection-pending deferral is no longer applied; it does not prove a wrong plan or unsafe action. [Observer initialization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/campaign_observer.go#L44-L78).
- **The artifact has a real reader, but that is not a downstream incident claim.** RealKernel initialization reads `.nerd/northstar.mg` and appends its logic when present. Later Guardian initialization can refresh facts. The kernel-load path establishes why durable export convergence matters, not that every boot ultimately retains stale facts. [Boot artifact consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_init.go#L404-L429).
- **Do not generalize across projection families.** Kernel state, JSON reconciliation and knowledge-atom writes have different owners and recovery paths. This witness requires only the existing stale Mangle artifact.

## Proposed scoped repair

### Separate semantic edit outcome from projection outcome

Retain `unchanged` as an accurate authority result. Add a separate per-projection readiness/repair report, or an explicit repair operation invoked by the documented recovery path. A healthy no-op must still avoid rewriting canonical authority, changing timestamps or churning already-correct JSON. An unchanged authority with a stale or unverified export must not imply successful convergence.

For this deterministic generated export, a small repair can render the current committed vision, compare the expected bytes with the existing regular artifact, and atomically replace it only when needed. An existence check is not evidence of current generation. An unreadable or unsupported destination is an explicit pending/unknown projection, not a no-op success.

A durable pending record/outbox is an alternative if the owner needs crash-resumable work across more projection types. It should be keyed by workspace, authority generation, projection kind and renderer/version identity, with bounded current state and a deliberate audit-retention policy. Do not build a general distributed transaction solely to fix the demonstrated artifact check.

### Repair the admitted generation, not the stale candidate

Recovery must re-read the authoritative generation and reconcile any intentional JSON edit under the established import rules before projecting. It must not overwrite a newer canonical change with the candidate retained by an older wizard. Preserve base-bound edit refusal and immutable snapshots.

If multiple publishers are supported, define generation-safe publication and acknowledgement: a V1 repair cannot mark V2's export ready, and an old writer must not silently replace a newer projection after its check. A process-local lock or atomic file rename alone is not a cross-process generation fence. This is a cost/contract for the proposed repair, not a newly demonstrated concurrent race.

Keep operator-authored JSON distinct from generated Mangle output. Do not discard a rejected JSON edit to make all surfaces match; existing validation and rejection semantics must remain. Do not delete an existing export merely to force the missing-file branch. Observe destination type and the existing path-admission/atomic-write policy.

### Expose what was actually checked

The repair result should distinguish authority unchanged, artifact current, artifact repaired, repair pending/failed and coverage not checked. The CLI should not describe an unchecked third surface as synchronized. If the wizard continues to require a projection before beginning a campaign, consume that current generation's repair result rather than treating absence of this call's errors as completion.

If product policy instead decides the Mangle export is not required for a particular live campaign, make that an explicit, documented dependency decision. It should not change implicitly because the user repeated a no-op save. Keep the initial committed/pending distinction and the existing independent campaign controls.

Update the prior audit's re-save recovery claim and the operator runbook to name the actual supported repair. Preserve the historical audit and its successful lossless-editing work; add a dated continuation rather than recasting that entire repair as unfixed.

## Costs and rollout

A compare-before-repair of an export with B bytes requires rendering and reading/hashing proportional to that artifact's size, plus allocation and filesystem work. It is more expensive than `stat`, and should be measured on complete open/save/sync operations. It needs no model call. A cache is useful only with trustworthy source-generation and destination invalidation; an existence or mtime-only shortcut recreates the same evidence gap.

Actual repair pays temporary-file creation, writing, file synchronization, atomic replacement and platform retry costs. Preserve the existing bounded Windows retries; an unavailable destination should remain visibly pending rather than spin forever. Do not weaken atomic replacement to save latency.

A durable per-projection record adds schema/migration, locking, recovery, cleanup and cross-process ordering work. Existing installations have no trustworthy acknowledgement; mark them unverified until checked, rather than fabricating successful publication from file existence. If direct comparison is sufficient for the one generated artifact, it may avoid that migration.

Readiness must not become an excuse to rewrite healthy exports on every no-op or boot. Compare the proposed implementation against both a healthy no-op and the actual pending-repair case, including repeated retries after recovery. Record whole-operation latency, bytes read/written, fsync/retry work and user-visible failure rates. No performance, quality or operational reliability improvement is measured here.

## Fair evaluation plan — not run

1. **Normal entry, not only helper tests:** drive wizard edit → `campaign` save → reported Mangle failure → reopen → unchanged save. Capture the authority digest, JSON semantic value, exact Mangle bytes, messages and returned campaign command at each boundary.
2. **Controlled pre-replacement failure:** preserve an existing V0 regular export, fail only V1's Mangle publication after successful authority/JSON writes, then remove the failure. The repaired implementation must publish V1 without changing the authority again. A Windows non-delete-sharing handle can later exercise the platform path; deterministic fault injection can cover the logical boundary without timing dependence.
3. **Failure persists:** repeated recovery must retain visible pending status and the intended dependency disposition, not convert it into success by exhaustion or equality.
4. **Healthy no-op:** retain the existing tests' unchanged authority timestamp and JSON mtime. Add a current Mangle artifact check that performs no rewrite.
5. **Missing versus stale versus unreadable:** verify all three separately. The existing missing-file repair is a positive control, not evidence for stale-content repair.
6. **CLI parity:** run the same state through the actual `northstar sync` handler and inspect both artifact contents and output; successful JSON/SQLite comparison alone must not claim artifact verification.
7. **Live kernel control:** allow initialization to repair V1 facts while keeping V0 on disk. The report must distinguish these states, neither claiming total failure nor complete convergence.
8. **Generation movement:** introduce a newer admitted V2 during repair and require explicit retry/conflict/current-generation publication, not a V1 acknowledgement applied to V2. Keep invalid/stale candidate refusal and authorized JSON reconciliation intact.
9. **Restart and receipt migration, if implemented:** pending repair survives restart, a successful acknowledgement corresponds to checked bytes/generation, and a crash does not replay the semantic edit.
10. **Consumer limits:** demonstrate any claimed downstream boot effect separately. Do not use a stale export alone as evidence of unsafe campaign execution or of model judgment quality.

Existing regression definitions separately verify no-op preservation and first-attempt projection-pending classification. Preserve them and add the sequence connecting those states; neither isolated assertion proves recovery. [No-op Guardian test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/edit_test.go#L306-L330), [first failed projection test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/edit_test.go#L426-L444), [wizard no-op preservation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/northstar_edit_test.go#L131-L155).

**Completion criterion:** an unchanged save remains semantically unchanged, but a prior failed durable projection is either verified/repaired for the current authority or remains explicitly unresolved. The documented recovery path must not lose the obligation merely because the canonical edit has already succeeded.

