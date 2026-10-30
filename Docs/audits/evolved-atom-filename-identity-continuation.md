---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: proposed-refinement
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
scope: distinct evolved candidate IDs sharing a lifecycle storage filename
---

# Distinct evolved atom identities need distinct storage ownership

## Decision and evidence boundary

Extend the existing prompt-evolution lifecycle contract to the candidate's durable storage identity. Two distinct, admitted atom IDs must not address the same file, and a lifecycle transition for one candidate must not remove or replace another candidate's durable body.

Current filename construction replaces every `/` in an atom ID with `_`. The distinct IDs `evolved/a_b/c` and `evolved/a/b_c` therefore both address `evolved_a_b_c.yaml`. A generation can successfully store both logical candidates in memory and report both as generated, while the second file write replaces the first. Reload sees only the surviving body. Promotion and rejection reuse the same filename mapping, so a transition requested for one ID can also remove or move another ID's pending file. [Storage and naming][storage], [promotion and rejection][transitions]

This is a scoped continuation of [prompt evolution lifecycle and evidence accounting][lifecycle], not a proposal for another learning system. Main was rechecked at approximately 10:43 UTC on October 30, 2026 and remained [`388205138ba6d47c1cff0c5e97c896dfa642a391`][main]. Independent static challenge corroborated the distinct-ID alias and the precise promotion behavior described below. No repository program, test, model inference, fault injection, or benchmark was run. No implementation was changed. The witnesses are source-derived and conditional, not observed incidents or frequency estimates.

## Credit the existing identity work

The lifecycle audit already documents same-ID replacement and served-version lineage. Its appendix calls for compilation-owned receipts so an ID collision or later replacement cannot credit an unserved version. The March QA report also explicitly describes repeated generation with the **same** ID overwriting the file and map. Those findings should not be republished as new. [Lifecycle and lineage][lifecycle], [historical same-ID finding][same-id]

Here A and B have different IDs, different bodies, and separate in-memory entries. No same-ID policy, source precedence decision, or compilation receipt can make two independently owned candidates fit into one mutable file. Even a future rejection of duplicate logical IDs would not reject this pair. The missing boundary is the noninjective translation from logical ID to storage location.

The 1018 [duplicate serving-domain audit][scope-audit] owns another boundary: content coverage plus provider/model containment before refusing a candidate. This witness does not require a false duplicate reading. Use distinct instructions that both pass admission, or the supported within-bound unjudged path with atom checks disabled. Correcting provider/model containment would not change the filename collision. [Configured and unjudged admission][atomcheck]

## The ordinary producer admits the two IDs

`atomDefinition` has an explicit YAML `id`. `convertToPromptAtom` keeps a supplied ID and generates an `evolved/...` UUID only when that field is empty. It validates/maps the category and stamps the source role and serving pin, but does not allocate a separate unique storage key. The pair above consists only of ordinary lower-case letters, underscores, and slashes; no traversal, platform-specific separator, invalid UTF-8, or same-ID collision is needed. [Generator][generator]

The evolved manager later validates each wrapper's atom. `PromptAtom.Validate` requires nonempty ID and content, a known category, and no self-dependency/self-conflict; it does not reject either example ID. The canonical authored parser's additional whitespace check also does not distinguish this pair. Use a known category such as `methodology`, valid provenance, compatible selectors, and nonempty bounded content. [Atom validation][validation], [authored ID check][authored-id], [manager validation][manager-validation]

The normal factory installs atom admission, creates the workspace evolver, and registers its evolved-atom reader with JIT. Automatic evolution remains opt-in; successful boot alone does not purchase generation. A valid witness explicitly runs an admitted cycle or enables the existing automatic route. It still requires qualifying unconsumed evidence, valid pin provenance, screening permission where configured, an owned claim, and sufficient generation/atom budget. [Factory][factory], [automatic admission][automatic], [cycle gates][gates]

`admitGenerated` returns the retained candidates. The cycle applies the per-group and remaining cycle caps, then calls publication. For the minimal case, let at least two atom slots remain and let one valid generation return A followed by B. Both survive their normal admission checks. Nothing in this construction needs extra calls beyond that admitted generation. [Admission][admission], [cap and publication handoff][handoff]

## Minimal sequential witness

Let A have ID `evolved/a_b/c` and content A, and B have ID `evolved/a/b_c` and distinct content B. Both are new pending candidates, with valid source role/provider/model provenance. Start with no preexisting file at their shared pending filename. Assume the filesystem writes succeed and no cancellation or other unrelated gate interrupts the admitted publication.

1. `publishGeneration` gathers both logical IDs and completes the owned generation claim with that ID list. It records consumption of the selected evidence.
2. Storing A writes its YAML wrapper to `pending/evolved_a_b_c.yaml`, then inserts `pe.evolvedAtoms[A.ID] = A`.
3. Storing B writes its YAML wrapper to the same path. The existing file is replaced by B's bytes. The map then gains `pe.evolvedAtoms[B.ID] = B`, leaving A's separate entry intact.
4. Both writes return success. The cycle decrements two atom slots, increments `AtomsGenerated` twice, and appends both IDs to its result. No storage error triggers `PublishFailed`.
5. The active evolver's inventory can show both A and B. A new evolved manager or a reload reads the one durable file, whose wrapper names B; it cannot reconstruct A from the consumed-evidence record or the successful publication's ID list. On restart, the evolver's own directory loader also sees only B. [Publication][publication], [storage][storage], [manager reload][reload], [evolver reload][evolver-reload]

This is silent loss of one durable candidate relative to the acknowledged publication, not an exception suppressed by error handling. The storage-failure safeguard is useful but does not fire because both file operations succeeded.

The generation purchase itself remains correctly charged and its claim remains owned. Consuming an unproductive purchase ordinarily prevents endlessly buying the same output again. The consequence here is that one reported candidate is missing despite successful writes, and the next ordinary serial cycle excludes the already-consumed task/verdict versions. Do not repair the filename defect by clearing all consumed evidence or making every publication retry indefinitely. [Consumption][consumption], [unconsumed query][unconsumed]

## Lifecycle operations make the ownership mismatch concrete

The following variants begin after the two-pending-candidate sequence above, while the original evolver still holds both A and B in memory.

### Promoting A writes A and removes B pending

`promoteAtomLocked(A.ID)` looks up **A in memory**, sets A's promotion timestamp, and serializes **A's wrapper** to the promoted directory. It then removes the pending path derived from A's ID. That pending path currently contains **B's wrapper**. The resulting files contain promoted A and no pending B, although B remains in the in-memory map until some other operation or restart changes it. [Promotion][transitions]

It would be incorrect to say this operation promotes B's bytes under A's name. It does not read the pending file to obtain the promoted body; it writes A from memory. The defect is the collateral removal of B's pending file and disagreement between logical inventory and durable state.

If B is subsequently promoted in the same process, B's promoted filename aliases A's promoted filename too. The B write can replace durable A; removal of the now-missing pending path is tolerated. Both logical entries may still look promoted in memory, but reload retains only B. This additional operation requires its own legitimate promotion decision; the finding does not imply unauthorized automatic promotion.

### Rejecting A moves B rather than checking the file owner

`RejectAtom(A.ID)` derives the shared filename and renames that pending file into the rejected directory, then deletes A's map entry and ends A's trial. The moved wrapper is B. Provided the destination is absent and the rename succeeds, B is durably rejected while the remaining map entry is B, not A. No file-body identity check connects the requested transition to the object being moved. [Rejection][transitions]

This is a wrong-object lifecycle action within the learning store, not a demonstrated tool-permission bypass, arbitrary filesystem write, or exposure of unapproved instructions. The two candidates remain subject to existing serving and trial policy. Even if a retained candidate is eligible, actual prompt inclusion still depends on context, source precedence, selection, and budget.

### Other writers share the mapping

Stored-pin migration rewrites candidate files and moves unpinned records using the same helper. A fix restricted to initial generation would therefore leave other lifecycle operations on the old addressing rule. Preserve the migration's good behavior—derive pins from each record's saved provenance and quarantine unknown provenance—but make those operations identify their actual durable object. [Pin migration][pin-migration]

## A durable identity and transition contract

The required invariant is simple:

> Distinct admitted candidate identities must have independently owned durable storage, and a transition may replace, move, or delete only the candidate identity and version that authorized it.

Separate logical identity from the filesystem representation. Two reasonable implementation directions are:

- **A versioned, injective filename encoding of the admitted ID.** Define the supported ID domain and filesystem limits. Use an encoding whose distinct inputs remain distinct on supported case-insensitive and normalization-sensitive filesystems; do not merely substitute another ambiguous delimiter. Address component length, reserved names, and bounded size explicitly.
- **A bounded opaque storage key with a unique logical-ID mapping.** An index or transactional store can bind logical ID, candidate version, lifecycle state, and owned storage key. A digest-based filename is practical but should still verify the stored full identity and define collision/conflict behavior; a finite digest is not a mathematical injective encoding of arbitrary strings.

Keep exact logical IDs and candidate hashes stable where possible. Giving every generation a new random ID can reduce one producer's risk, but it does not repair legacy files or every promotion/rejection/migration path. Restricting all IDs to remove slashes or underscores would break established identities and needs a much larger migration than the demonstrated defect requires.

Publication must establish storage ownership before replacing bytes. If a destination belongs to another ID, report a conflict and leave that object intact. Checking the destination and then issuing an unconditional write is not sufficient when multiple processes can write the workspace. State the atomicity mechanism: a transactionally unique mapping, a coordinated writer/lease protocol, or another operation that cannot race between ownership validation and publication.

Use atomic file replacement for a candidate's own update so readers do not observe partial YAML, but do not confuse that with ownership safety. Atomically renaming B over A is still the wrong transition. Similarly, a process-local mutex does not establish cross-process exclusion. Keep model calls outside filesystem/database transactions and preserve the existing generation-owner fence.

Lifecycle transitions should bind the source and destination to the expected ID and, where the reviewed body matters, its candidate version. Delete the source only after durable publication of that same authorized candidate and verification that the source still belongs to it. Define interrupted-promotion recovery without sacrificing the existing rule that a legitimate promoted copy wins over a stale pending copy of the **same** ID. Distinct IDs are not stale copies of one another.

A filesystem plus SQLite design must make partial publication explicit rather than promise an atomic transaction spanning components it does not actually transact. Retain the current owned claim and failed-publication accounting; reconcile incomplete candidate writes and lifecycle receipts under an idempotent recovery contract. Do not count a mere successful file syscall as proof that every named candidate remains durably addressable.

## Migration and recovery

Version the storage layout and migrate all lifecycle directories together: pending, promoted, rejected, and unpinned. Enumerate existing wrappers and use the embedded logical ID and preserved provenance to derive the new key. **Never reverse underscores into slashes to guess an old ID**; the old filename has already discarded that information.

Before modifying a legacy location, verify the actual wrapper identity and record the intended mapping. Write the new object durably, record migration progress, and retire the old location only if it still contains the object being migrated. Preserve backups and resumable outcomes for interruption, corrupt wrappers, ambiguous multiple copies, unavailable storage, and ownership conflicts. Mixed-version writers must be drained or prevented from continuing the old lossy layout while migration runs.

A single overwritten file cannot reveal both original bodies. If only B remains, do not fabricate A from its name in a generation receipt or relabel B as A. Recover A only from an actual retained body, backup, or other authenticated artifact; otherwise report it missing. Any intentional regeneration is a separate bounded and auditable purchase, not an excuse to erase all consumption history. A live in-memory A may help an explicitly coordinated migration, but is not a crash-durable recovery source.

Changing only physical storage keys should not silently broaden pins, promote pending candidates, or manufacture fresh trial allowance. Preserve logical IDs, immutable version ownership, existing approval envelopes, exposure/credit evidence, source-failure receipts, and prior spend when their meaning is unchanged. If recovery changes a candidate's body or admission-affecting fields, treat it as a changed version and apply existing review/trial rules. Refresh the reader's source generation when publishing a migrated set; cache changes are not a substitute for fixing storage identity.

## Proposed controls and fair evaluation

All checks below are proposed and unrun. Start with deterministic generator and judge replies through the normal evolution path, using real temporary lifecycle directories and ledger storage. A direct helper test is necessary but insufficient: validate the returned generation receipt, file bodies, restart inventory, and lifecycle consumer together.

| Case | Required observation |
| --- | --- |
| Distinct slash/underscore IDs A and B in one admitted generation | Both have independent durable bodies, or the conflicting publication is explicitly refused without altering the first body; never report two durable successes backed by one file |
| Same pair across two eligible cycles | Later publication does not silently retire the earlier distinct candidate |
| Reload and restart after both writes | IDs, contents, source evidence, pins, and lifecycle state agree with acknowledged publication |
| Promote A after both pending writes | A's authorized bytes become promoted; B remains pending and durable |
| Subsequently promote B | Both reviewed identities remain independently durable; no replacement of A merely because filenames once aliased |
| Reject A while B is pending | Only A moves to rejected; B and its trial/credit records remain owned by B |
| Existing same-ID promoted/pending crash duplicate | Existing reviewed-copy precedence remains; do not confuse a recovery duplicate with two distinct IDs |
| Ordinary write failure | No false successful in-memory publication; preserve the existing failure behavior and evidence recovery |
| Interrupted write, promotion, rejection, or migration | Restart reaches an explicit consistent or recoverable state without deleting a different object |
| Two processes publishing colliding legacy names or the same logical ID | Atomic ownership/conflict behavior, not a check-then-write race |
| Valid pin, missing pin, approved trial, rejected and unpinned records | Existing pin and lifecycle admission remain authoritative |
| Legacy lost body and mixed-version writer | Honest missing/conflict disposition; no guessed recovery or silent reauthorization |
| Long IDs, case variants, Unicode and supported filesystems | Storage encoding/mapping obeys its declared bounded identity contract |

The existing [failed-write test][write-test] correctly requires that a failed persistence operation not leave the atom in memory. Keep it. The alias witness adds a successful-write case in which the two representations disagree; weakening the failure test would not address it.

Measure independently addressable candidates, wrong-object moves/deletions, unexpected body replacements, logical/durable inventory disagreement, completed and failed publication receipts, and evidence consumption. Then measure bytes, directory/index size, serialization and validation work, read/write/rename counts, lock contention, migration time, and recovery overhead. Report model purchases separately from host storage work: establishing this identity discrepancy requires no live inference.

If evaluating downstream task quality later, hold tasks, serving models, tools, verification requirements, trial policy, and budgets matched. More retained atoms are not automatically useful atoms, and a restored pending candidate is not proof of exposure or improved output. Report candidate availability, authorized exposure, and task outcomes separately.

## Costs and checkable exit

An injective encoding can lengthen filenames and require explicit ID bounds. An opaque key/index design adds schema, lookup, integrity checks, and transactional contention. Atomic replacement and migration add temporary files, I/O, recovery records, backups, and disk-space requirements. Collision checks may deliberately reject already-purchased generated output; expose that cost instead of hiding it as success.

Preserving both candidates can increase legitimate storage, review, and trial work compared with silently losing one. Do not claim a token, latency, or quality improvement without measurement. No additional model call is inherently required to make storage identity faithful.

**Checkable exit:** every successfully published candidate remains durably addressable by its exact logical identity; acting on A cannot move, replace, or delete distinct B; restart and migration preserve or honestly report identity; existing pins, quarantine, approvals, claim fencing, bounded spend, and failed-publication recovery remain intact.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[same-id]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/autopoiesis/2026-03-28_10-00-EST_prompt_evolver_boundary_analysis.md#L360-L369
[scope-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b42816320d9164c824aba7da0c54bc27df2b4279/Docs/audits/atom-duplicate-serving-domain-containment.md
[storage]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L45-L90
[transitions]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L115-L179
[generator]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/atom_generator.go#L168-L265
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L713-L742
[authored-id]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atom_schema.go#L507-L514
[manager-validation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L199-L252
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L112-L173
[automatic]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L374-L395
[atomcheck]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/admission.go#L52-L126
[gates]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L175-L235
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/admission.go#L51-L92
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L238-L265
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L273-L319
[reload]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L148-L197
[evolver-reload]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L219-L275
[consumption]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L378-L403
[unconsumed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/feedback_collector.go#L527-L571
[pin-migration]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/pin_migration.go#L33-L98
[write-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/prompt_evolution_test.go#L920-L940
