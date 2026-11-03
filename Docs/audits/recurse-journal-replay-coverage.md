---
doc-class: deep-dive
subsystem: campaign-recurse-recovery
implementation-status: proposed
last-verified: 2026-11-03
verified-against: f279fefa3f2603be7f959b4ad2475f99df02faa2
verification-method: static-source-inspection
---

# Recurse journal replay coverage after rotation and torn tails

## Decision

Give recurse a recovery-log contract distinct from best-effort measurement retention. Its restart path uses a JSONL reader that intentionally returns only the current generation and one backup, tolerates malformed tails, and reports that incompleteness separately. Recurse discards the malformed-generation count and has no checkpoint covering an overwritten prefix. The live policy and a restarted policy can therefore disagree about which findings remain refused or stalled. A damaged live generation can also accept later writes that ordinary status and recovery never decode.

These are **source-supported conditional findings**, not executed reproductions. No application, model inference, tests, benchmark, failure injection, implementation, repository write, credential access, or settings change was performed. Only public source reads and this scratch report were used.

The principal correction is not to make every telemetry log permanent. Preserve bounded measurement logs, but require recoverable state before retiring the events that establish recurse decisions, and require a valid append boundary before continuing a damaged journal.

## Scope, source freeze, and prior work

Public main was resolved and rechecked at [`f279fefa3f2603be7f959b4ad2475f99df02faa2`](https://github.com/theRebelliousNerd/codenerd/commit/f279fefa3f2603be7f959b4ad2475f99df02faa2), with the final check at approximately 21:45 UTC on November 3, 2026. Reviewed guidance included [root agent requirements][root-guidance], [campaign requirements][campaign-guidance], campaign architecture and wiring, the forever-loop journey, current implementation, relevant test bodies, and the audit inventory. The root architecture requires Mangle to own executive decisions, source-grounded evidence, and explicit long-horizon continuity; this proposal preserves those choices.

The neighboring work matters:

- [Independent-writer JSONL rotation][prior-jsonl] concerns a fresh record written through a stale descriptor after another writer rotates twice. This report needs **one writer**, whose writes and rotations all succeed. Intentional retirement itself loses still-required control history.
- [Recurse recovery attempt identity][prior-recovery] concerns matching a foreign commit by local cycle number. Here the primary witnesses need no foreign repository, copied ledger, matching commit, branch race, or unresolved adoption. An exact attempt ID would not reconstruct a forgotten refusal.
- [Prior reason budgeting][prior-budget] bounds model-facing history rendering. Its current-main addendum expressly leaves journal retention/replay as separate questions. This report concerns replay authority, not a return of the repaired reason-only prompt overflow.
- [Campaign generation admission and recovery][prior-campaign] already treats damaged-tail admission and repair in the ordinary orchestrator journal. Recurse does not call that journal implementation: its wrapper directly uses `internal/jsonl`. This is a distinct live recovery path that needs equivalent attention, not a claim that the campaign repair is absent.

A bounded audit-inventory review and searches for recurse journal, rotation, truncation, refusal, and `readRecurseJournal` did not identify this exact paired replay-coverage mechanism. That is a novelty screen, not an exhaustive claim about all history or branches.

## 1. The contract crossing

### Recurse treats the log as decision history

The journal describes itself as the loop's ledger and restart/status input. Git is separately authoritative for kept commits, but that does not make Git an archive of refusals, failed signatures, visited nodes, or rejected approaches. The wrapper calls `jsonl.Open` without changing retention; `append` checks only whether the appender's failure counter increased. `readRecurseJournal` explicitly discards the reader's second return value. [Journal wrapper][journal].

Startup reads the journal, settles any in-flight attempt, prepares the Git checkout, opens the journal again, rereads it, and passes the result into `resume`. There is no intervening validation or repair of a malformed journal suffix. `resume` reconstructs pass/visit position, the maximum cycle, policy attempts, latest kept-node observations, and prior rejected approaches from the records it receives. [Startup][startup], [replay][replay].

The live path projects non-kept fix outcomes into `recurse_attempt`; it intentionally retains the full attempt history. A kept change updates the node's latest kept observation. The rules derive refusal from an existing refused-attempt fact, and exclude that finding from candidacy; the last two matching failed signatures derive a stall unless an appropriately newer kept-node change exists. The refusal comment says lifting it belongs to the owner. [Policy projection][projection], [stall/refusal/selection rules][rules].

Both CLI and chat use `RunRecurseCycles`; this is not merely an unused helper. CLI launches actual attempt campaigns, and chat describes restarting recurse from the journal. [CLI route][cli], [chat route][chat].

### The storage primitive promises a recent sample

`jsonl.Appender` explicitly describes measurement data rather than a ledger of record. It appends without fsync, rotates at a default 20 MiB threshold, and preserves only one backup. A second rotation replaces the older backup. `Dropped()` tracks overwritten generations on that appender object, but recurse never consults it, and opening a new object does not reconstruct this counter from durable metadata. [Appender and rotation][appender], [loss counter][loss-counter].

`jsonl.Read` reads only `path.1` followed by `path`. For each generation, its decoder returns the valid prefix at the first malformed object, marks that generation truncated, and returns no ordinary error. This is a reasonable approximate-measurement policy. It is not proof of complete execution-history coverage. [Reader][reader].

The forever-loop journey calls for restart continuity while expressly allowing one-generation rotation. Its statement about retaining only two kernel attempts is stale relative to the current projection and test, which intentionally retain all attempts and derive the two-failure window in policy. The decisive issue is the current producer/consumer mismatch, not that stale wording. [Journey][journey], [full-history test][history-test].

## 2. Witness A: a torn pass-end hides later successful appends

This witness does not require reaching the rotation threshold.

1. Run ordinary, non-mission recurse in an admitted clean workspace. Let the current pass finish its attempted work normally: effects are settled, ratchet rows are written, and any in-flight file has been removed.
2. An interruption leaves a malformed **partial JSON object** at the end of the live journal while appending that pass's `pass_end`. Assume no unresolved in-flight file and no unrelated Git or filesystem admission failure. A partial object is an explicit failure condition, not an assertion that every process termination tears a write.
3. Restart with a fresh kernel and enough pass budget to continue. The JSONL reader returns the valid prefix plus a positive truncation count. Recurse discards that count. With no in-flight file, settlement returns; the normal clean-checkout path can proceed.
4. `jsonl.Open` reopens the same bytes in append mode. The next successful append writes a new serialized object after the malformed partial object; it does not truncate, seal, repair, or start a clean generation unless the byte threshold happens to require rotation.
5. Choose the ordinary partial-object case that remains invalid after another JSON object is appended. The stream decoder always stops there. Subsequent complete `pass_start`, attempt, ratchet, and visit rows in that generation remain beyond the decoding boundary even though their writes succeeded.
6. Before rotation, read status or restart again. The reader still exposes the old prefix, without an error propagated by recurse.

The ordering is directly visible: pass-end publication is after visits; successful cycle completion removes the in-flight file; missing in-flight state returns immediately; startup then opens the existing journal; the appender extends it unchanged; the reader cannot resynchronize. [Pass end][pass-end], [cycle completion][cycle-end], [settlement][settlement], [startup][startup], [appender][appender], [reader][reader].

The minimal established consequence is **silent invisibility of newly appended state to the normal status/replay path**. If those hidden rows contain a new refusal or a stall's determining failures, a subsequent fresh replay cannot derive those guards from them. If the hidden work is still an eligible measured finding, it can be proposed again. Actual repeated model work requires those additional conditions. The report does not assert that every hidden kept commit is repeated or lost: current source measurement, Git contents, in-flight reconciliation, and other gates can independently change or block the next action.

If the first new append rotates the damaged generation, new rows land in a clean file and this particular hidden-new-rows interval ends. That is a control, not repair of the lost or unparsed state. A complete JSON object missing only its trailing newline is also not this witness; the JSON stream decoder can accept adjacent complete objects. The witness requires a genuinely malformed partial object.

## 3. Witness B: clean retention expires a live refusal

This witness needs neither corruption nor a crash during a write.

1. An ordinary recurse attempt on a stable finding ID reaches a forbidden path and is correctly refused and compensated. Its settled refused ratchet row enters the oldest generation and its fact remains in the live kernel.
2. The same authorized long-running loop continues useful work or ordinary pass/visit activity elsewhere. Enough successfully serialized journal data causes two rotations, so the generation containing that refusal is replaced. No owner lifts the refusal and no later refused row for that finding enters the retained generations.
3. The live kernel still holds the refused-attempt fact. Its current policy continues excluding that finding.
4. Stop cleanly at a settled boundary, then restart with a fresh kernel containing no earlier recurse attempt facts. Replay sees only the retained generations. It reconstructs no refused-attempt fact for this finding.
5. Let the finding remain open, preserve its identity, and give it a normal visit where the other candidate guards allow it. The current selection rules no longer exclude it for the aged-out refusal.

The same logical loss can remove the only two failures establishing a still-applicable stall, provided no relevant kept-node change legitimately lifts it. A refuse case is simpler: the current refusal rule has no ordinary age expiry or kept-change exception. [Live projection][projection], [rules][rules], [rotation][appender], [replay][replay].

This is a **restart-dependent change of executive state**, not a permission bypass. A newly attempted forbidden write still encounters existing scope, permission, ratchet, and compensation controls. The practical risk is re-admitting work the loop said it would not retry, losing causal history, and spending work again without an owner decision.

A first rotation alone normally preserves the earlier row in the backup. A short run, unchanged process/kernel, a retained later refusal, changed finding identity, legitimate stall reset, missing gates, or failed admission can prevent this exact outcome. Mission bounds may stop an invocation earlier; the witness deliberately uses ordinary recurse and does not claim unlimited mission execution.

No fixed number of cycles is claimed: the threshold is measured in serialized bytes, and records have variable detail/patch sizes. The rotations are normal retention behavior documented and tested by the JSONL package, not arbitrary deletion introduced by the witness. [Rotation tests][rotation-tests].

## 4. User-visible and policy consequences

`ReadRecurseStatus` uses the same incomplete reader, totals its retained ratchets, and formats attempts/outcomes and a pass trend without a coverage field. It can therefore present a recent or damaged-prefix count without saying which history is missing. The code comments describe totals over the journal, so this report does not assume the UI explicitly promises a lifetime total; the omission matters because the same apparently usable journal also drives recovery. [Status][status].

Git protects a different kind of truth: a refused/no-op attempt creates no kept commit from which its control evidence can be reconstructed. Fresh source remeasurement correctly discovers what is currently failing, but cannot establish that policy previously refused an attempt under the owner’s forbidden-path rules or that repeated identical attempts already stalled.

The existing workspace lock remains valuable and does not repair either witness. Both use one writer. Nor does a successful JSONL `Failures()` check establish recoverability: rotation is intentional, and a full successful write after a malformed prefix need not increment a failure counter.

## 5. Proposed recovery contract

Define the invariant before selecting an implementation:

> For an admitted ledger incarnation and policy version, replay from a durable checkpoint plus its validated contiguous suffix must reproduce the executive observations needed for the same refusal, stall, visit, and sequence decisions as uninterrupted execution.

This is an architectural proposal, not a claim that the current implementation satisfies it.

### Admission before extension

- Read a structured coverage report: ledger/incarnation identity, retained sequence range, checkpoint coverage, validated append offset, malformed/unknown suffix, and unresolved attempt/adoption references.
- Never extend a generation known to contain a malformed suffix. Under the existing owner lock, preserve the original evidence and perform an explicitly successful repair or seal the generation and start a linked clean one. Repair failure must hold execution instead of silently discarding uncertainty.
- Propagate coverage to status. Separate a readable recent history from an admissible recovery source. Missing files in a genuinely new workspace need not be treated as corruption, but missing covered generations in an existing ledger must not silently mean no history.

### Safe retirement

A checkpoint should preserve source observations, not just currently derived booleans: refusal provenance, the failed attempts relevant to the current policy window, latest kept-node evidence, cycle/pass high-water marks, current-pass visits, unresolved effects/adoption, and references needed for bounded rejection memory. Include a schema/policy version and a verified suffix boundary.

Current code deliberately keeps all attempt facts. Replacing them with two rows or a boolean in Go is not automatically compatible with the logic-first contract. Either snapshot the required fact history, or specify and prove a policy-owned compact representation that is observationally equivalent for supported consumers and future appends. A saved stalled boolean alone is insufficient because later kept-node changes can lift the stall.

Publish and validate the checkpoint before deleting its covered prefix. A failed publication, sync, replacement, or checkpoint read must leave a recoverable previous state or an explicit hold. Separate large human-readable evidence retention from compact executive state; shortening historical patches must not expire a still-active refusal.

Options to compare:

1. **Small containment:** surface malformed-tail admission failure and refuse destructive retirement until a valid checkpoint is available. This avoids silent state loss but can stop long runs at a storage boundary.
2. **Checkpoint plus segmented journal:** retain bounded replay suffixes after durable source-state publication. This keeps file-based tooling but adds sequence, migration, repair, and retirement ownership.
3. **Transactional control store plus recent log:** store executive observations in a durable transactional representation and keep JSONL for recent inspection. This adds storage/schema/WAL lifecycle and must not create a competing authority for Mangle decisions.

Merely retaining more generations postpones the clean-retention witness. Merely adding fsync does not repair malformed append boundaries or preserve retired control facts. Merely adding UUIDs solves neither coverage gap, though attempt identity should compose with the earlier recovery-identity work.

For old ledgers with an already missing prefix, do not invent absent refusal history or silently assert completeness. Preserve readable evidence and expose the migration/reconciliation decision. Explicit owner-authorized resets and archival policy must be distinguished from automatic forgetting.

## 6. Costs and failure modes

- **Storage and memory:** executive state scales with outstanding/distinct obligations and whatever provenance the contract retains, not necessarily a constant. A checkpoint cannot honestly promise arbitrary forever-history semantics in fixed space. The current full-attempt in-memory history is a separate growth concern; this report does not claim checkpointing alone fixes it.
- **Work:** checkpoint serialization, validation, and durability barriers add CPU, bytes written, and latency; compaction may temporarily require both generations. Incremental checkpoints reduce pauses at the price of more bookkeeping and recovery states.
- **Recovery:** validation and migration can increase restart latency. Successful admission can reduce repeated campaign/model work, but no savings are established without matched experiments.
- **Failure cases:** disk full, interrupted checkpoint publication, checkpoint/suffix mismatch, duplicate sequence, copied ledger, root relocation, policy-version changes, missing archives, owner-lock loss, and repair preservation failure require explicit behavior.
- **Scope:** do not impose heavyweight durable-ledger semantics on every existing measurement caller merely because recurse reused its helper. Keep telemetry's intentional loss contract explicit.

The state-preserving compaction idea is established systems practice, not a novel storage algorithm. Ongaro and Ousterhout's [Raft extended paper, section 7](https://raft.github.io/raft.pdf) describes replacing a committed prefix only after a snapshot records state and its log boundary. That motivates the checkpoint/coverage invariant here; codeNERD does not need distributed consensus to apply it. Mohan et al.'s [ARIES paper](https://www.cs.cmu.edu/~15849g/readings/mohan92.pdf), also [cataloged by IBM Research](https://research.ibm.com/publications/aries-a-transaction-recovery-method-supporting-fine-granularity-locking-and-partial-rollbacks-using-write-ahead-logging), motivates explicit recovery sequencing and retaining compensation/restart evidence. Neither paper establishes this repository's correctness, filesystem guarantees, or performance.

## 7. Fair proposed evaluation, not run

The existing tests are useful controls:

- The forbidden-write fixture checks one refusal and no retry across two passes. [Refusal fixture][refusal-test].
- The policy-history fixture deliberately keeps hundreds of attempts and verifies the refusal fact remains. [History fixture][history-test].
- The recurse soak defaults to 100 cycles, compares live attempt facts against returned journal rows, checks cycle-fact cleanup and a disk-size bound, but does not restart after forced rotation. Its equality can itself expose divergence once older attempts age out; no such run was performed. [Soak fixture][soak-test].
- JSONL tests deliberately confirm oldest-generation retirement and report a damaged tail, but do not exercise recurse's cold replay and subsequent append path. [Rotation][rotation-tests], [tail fixture][tail-test].

Use deterministic executors and actual policy derivation first. Compare current main, containment, and checkpoint/transactional alternatives with identical source findings, write guards, verification obligations, and replay inputs. Do not claim a quality or cost win from simply refusing every task.

| Proposed case | Required observation |
|---|---|
| Healthy log, no rotation, clean cold restart | Same current policy eligibility and pass/cycle position; existing successful restart behavior preserved. |
| First rotation, then second rotation, old active refusal | Cold replay retains the refusal or explicitly refuses incomplete recovery; no implicit owner reset. |
| Old equal-signature failures, with and without a newer kept-node change | Stall decision matches uninterrupted policy, including legitimate release. |
| Torn pass-end with no in-flight attempt, then successful append | Either safe admission refusal or verified repair; new rows remain readable. |
| Complete object without newline | Do not misclassify valid JSON stream input as the malformed-object witness. |
| Malformed live versus malformed backup; rotation immediately on reopen | Correct per-generation coverage, no hidden uncertainty or accidental whole-ledger reset. |
| Interrupt each checkpoint/retirement boundary; disk-full and repair failure | No admitted suffix with missing prerequisite state; original evidence preserved or precise hold reported. |
| Explicit reset, changed policy, relocated/copied ledger, legacy incomplete log | Versioned, documented migration semantics; no invented completion or refusal provenance. |
| Real CLI and chat cold process restart | Verify kernel facts, executor calls, status coverage, in-flight lifecycle, Git/workspace contents, and cleanup together. |

For focused fixtures, lower the appender threshold through test-only injection to cross both rotations cheaply, then add a default-threshold integration case. Do not spend live model calls merely to generate tens of MiB of history. Preserve genuine ratchet/refusal producers; a helper test that fabricates only a final boolean does not prove the production contract.

Measure decision divergence, re-admitted refused/stalled work, silently unreadable successful appends, duplicate settled cycles, and operator-visible uncertainty. Account separately for checkpoint/append latency, startup time, memory/fact count, retained/written bytes, sync operations, CPU, host verification, repeated model requests/tokens, and intervention time. Include failed and held runs in the denominator. No runtime pass, incidence rate, benchmark, savings, or implementation completion is claimed.

[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/agents.md
[prior-jsonl]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/Docs/audits/jsonl-independent-writer-rotation.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/Docs/audits/recurse-recovery-attempt-identity.md
[prior-budget]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/Docs/audits/recurse-prior-reason-budget.md
[prior-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/Docs/audits/campaign-generation-admission-and-recovery.md
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_journal.go#L11-L100
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_cycle.go#L420-L447
[replay]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_cycle.go#L1840-L1883
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_policy.go#L103-L128
[rules]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/core/defaults/policy/recurse.mg#L193-L245
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/cmd/nerd/cmd_campaign_recurse.go#L225-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/cmd/nerd/chat/campaign_recurse.go#L23-L28
[appender]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/jsonl/jsonl.go#L24-L145
[loss-counter]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/jsonl/jsonl.go#L172-L178
[reader]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/jsonl/jsonl.go#L196-L255
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/Docs/journeys/10-forever-loop.md#L161-L177
[history-test]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_policy_test.go#L245-L298
[pass-end]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_cycle.go#L605-L612
[cycle-end]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_cycle.go#L1536-L1572
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_cycle.go#L1783-L1805
[rotation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/jsonl/jsonl_test.go#L95-L163
[status]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_control.go#L107-L178
[refusal-test]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_cycle_test.go#L200-L226
[soak-test]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/campaign/recurse_soak_test.go#L59-L178
[tail-test]: https://github.com/theRebelliousNerd/codenerd/blob/f279fefa3f2603be7f959b4ad2475f99df02faa2/internal/jsonl/jsonl_test.go#L185-L218

