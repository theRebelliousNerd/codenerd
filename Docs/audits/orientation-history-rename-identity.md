# Orientation history needs revision-scoped identities, not transitive path aliases

## Status, scope and finding

**Supported by static source tracing at `f1632329be07b92e86ae07b1c9e96777e656651b`; not reproduced by execution.** Orientation's newest-first Git-history parser treats a historical pathname as a timeless alias for another pathname. With two distinct files that exchange names through three separately committed renames—`a.md → tmp.md`, `b.md → a.md`, then `tmp.md → b.md`—the aliases conflate their histories.

The parser can assign both files' older events to current `a.md`, while current `b.md` retains only the final rename event. This changes `repo_file_history` first timestamps, commit counts and active-day counts, the daily witnesses, and monthly file/document-arrival counts. Those are inputs to real timeline, lineage and attention-selection rules, not merely a diagnostic display.

The bounded consequence is incorrect orientation evidence and potentially incorrect derived document lineage/read prioritization. It is not a demonstrated wrong model answer, authorization bypass, file mutation or universal initialization failure. The source files remain unchanged; the error is their historical attribution. No repository code, tests or reproductions were run or changed in this review.

## 1. The intended contract and the Git boundary

`ScanHistory` explicitly promises one whole-tree log pass, following renames so a moved document retains history under its current path. It runs `git log --name-status --find-renames` with commit hash and committer timestamp markers, then passes that stream to `factsFromGitLog`. [Scan entry and command][scan].

Git normally emits log entries in reverse chronological order. Rename detection reports a relationship between deleted and added file entries in a particular comparison; it is not a permanent global equivalence relation among pathname strings. The witness below uses a linear history with increasing committer timestamps, one unambiguous unchanged-content rename per commit and ordinary ASCII names. This avoids merges, timestamp disorder, unusual-name parsing and similarity ambiguity as explanations. [Official git-log documentation](https://git-scm.com/docs/git-log), [Git's diffcore rename design](https://git-scm.com/docs/gitdiffcore#_diffcore_rename_for_detecting_renames_and_copies).

The operational contract should be:

> Within the declared history model, assign an event to the file lineage occupying that path at that revision; map that lineage to its current display path only after preserving its identity.

Two distinct lineages may occupy `a.md` at different times. Conversely one lineage may occupy several paths. A path's historical binding and a stable lineage identifier cannot safely be represented by the same recursively resolved string.

This report uses “identity” to mean the lineage justified by the particular detected rename events. It does not claim Git stores a perfect intrinsic file identity, that content similarity proves semantic sameness, or that a path's earliest visible event in a shallow clone is its true creation.

## 2. Exact parser mechanism

The parser maintains:

- `canon map[string]string`, from old path to a newer path;
- `files map[string]*fileRec`, with first/last timestamps, commit-hash set and active-day set;
- a pending commit containing parsed status changes.

`resolve(p)` repeatedly follows `canon[p]`. `link(old, neu)` first resolves `neu`; if the resolved value equals `old`, it returns to avoid a cycle; otherwise it records `canon[old] = head`. For each commit, all rename links are installed before any touched path is resolved. Rename/copy events count their destination; other events count their named path. [Resolver, link and commit processing][aliases].

Commit hashes and days are deduplicated within the resulting path record. The records already accumulated in `files` are not rekeyed when a later alias entry changes the meaning of their pathname. At the end, the parser emits daily witnesses and summaries directly from those records. [Accumulation and fact emission][facts].

The status parser accepts tab-separated `R100\told\tnew` and `A\tpath` shapes. Simple names such as `a.md`, `b.md`, and `tmp.md` survive its cleanup unchanged. No malformed input is necessary. [Status parsing][parse].

The cycle guard is useful for the existing single-file rename-away-and-back fixture, but it does not distinguish “the same file returned to a name” from “a different file now occupies that name.”

## 3. Static five-commit witness

Let two different documents have lineages **A** and **B**. Keep each document's bytes unchanged through its renames, and commit each move separately:

| Commit, chronological order | Date, distinct UTC days | Change | Tree after change |
|---|---|---|---|
| C0 | January 1 | Add A as `a.md` | `a=A` |
| C1 | February 1 | Add B as `b.md` | `a=A, b=B` |
| C2 | March 1 | Rename `a.md → tmp.md` | `tmp=A, b=B` |
| C3 | April 1 | Rename `b.md → a.md` | `tmp=A, a=B` |
| C4 | May 1 | Rename `tmp.md → b.md` | `b=A, a=B` |

Use a single year, strictly increasing committer times and stable HEAD. The source invokes rename detection, and the one deleted/added unchanged pair in each rename commit supplies the ordinary `R100` case described by Git's rename documentation. The witness is conditional on those detected records, not a claim that every arbitrary move/edit sequence must be reported as a rename. [Git rename representation](https://git-scm.com/docs/gitdiffcore#_diffcore_rename_for_detecting_renames_and_copies).

By the implementation's declared rename-following contract, the expected current-path histories are:

- Current `a.md` is B: C1 and C3, February through April, two commits on two days.
- Current `b.md` is A: C0, C2 and C4, January through May, three commits on three days.

The newest-first source trace instead is:

| Parser step | Alias action | Resolved touched path |
|---|---|---|
| C4: `tmp → b` | `canon[tmp] = b` | `b`; create its record with C4 |
| C3: `b → a` | `canon[b] = a` | `a`; create its record with C3 |
| C2: `a → tmp` | `resolve(tmp)` follows `tmp → b → a`; equals old `a`, so link is skipped | `resolve(tmp) = a`; add C2 to `a` |
| C1: add `b` | No rename link | `resolve(b) = a`; add C1 to `a` |
| C0: add `a` | No rename link | `a`; add C0 to `a` |

Predicted summaries from this manual trace:

| Current path | Expected first / last | Expected commits / days | Current parser first / last | Current parser commits / days |
|---|---|---|---|---|
| `a.md` | February / April | 2 / 2 | January / April | 4 / 4 |
| `b.md` | January / May | 3 / 3 | May / May | 1 / 1 |

The two last timestamps happen to remain correct in this witness. Do not describe every field as corrupted. The global commit count and per-month commit counts also remain correct: each parsed commit still increments those counters once.

The file/document-arrival counters do change. `monthFacts` assigns each record to the month of its first timestamp; it counts A's current `b.md` record as newly appearing in May and loses B's February arrival. Both files are Markdown, so document-arrival counts move with file-arrival counts. [Monthly facts][months].

Both current path records still exist. This is not simply a missing row, and checking that there are two files or five total commit touches would not expose it. The invariant must compare attribution per lineage/current path.

## 4. Concrete consumers and production reachability

### Normal initialization measures this history

The initializer's phase sequence invokes phase 3 orientation; that phase calls `runOrientation`, which calls `orient.Measure`. `Measure` invokes `ScanHistory`, appends its facts to document/ecosystem measurements, and checks that HEAD stayed stable. `Snapshot.Engine` asserts those facts and evaluates the embedded policy. A stable HEAD witness prevents acquisition across revisions, but does not correct a deterministic identity error within a stable history. [Initializer phase][init], [phase wrapper][phase], [orientation entry][run], [measurement and evaluation][measure].

### Incorrect first timestamps can reverse an evolution edge

`lineage.mg` derives `time_ordered_similar(Old, New)` from sufficient document similarity and `FirstOld < FirstNew`. It then derives `doc_evolved_into` when both documents exist and their themes are compatible, or one lacks a theme. [Time-ordered lineage][lineage].

For the same pair of current documents in the table:

- Correct birth ordering is `b.md` before `a.md`.
- The parser's ordering is `a.md` before `b.md`.

If the ordinary similarity/theme premises hold, the chronological orientation of that edge reverses. Similarity and theme conditions remain necessary; the history defect alone does not manufacture a similarity relation. Nor does a reversed evolution edge necessarily imply `doc_superseded`: that rule additionally requires a live successor and quiet predecessor. [Live/superseded rules][superseded].

### Attention and the Northstar brief consume the result

`chain_latest` depends on evolution predecessors/successors. It supplies a weighted read reason, and budgeted ranking derives `orient_read_candidate`. First timestamps also feed document generation and birth cohorts; commit/day counts feed burst predicates. [Chain tips][chain], [generation][generation], [burst inputs][bursts], [read reasons][reasons], [budgeted selection][selection].

The initialization consumer actually queries `orient_read_candidate`, selects the associated measured document bodies and sends eligible pending documents for classification. Later, `orientationBrief` queries `doc_evolved_into` and `doc_superseded` and populates typed evolution links. These are live evidence-consumption paths. [Read-set consumer][readconsumer], [brief consumer][brief].

The defensible claim is that incorrect history can change eligibility, reasons, weights and lineage evidence reaching those consumers. A changed final selected set depends on budgets, competing documents, duplicate/cohort grouping, other signals and configuration. A changed model answer is not established by this review.

### Refresh does not make the full-history algorithm safe

When a saved HEAD changes, `Refresh` can choose a full history pass for detected endpoint renames, rewinds, missing witnesses or related conditions. That pass is the same `ScanHistory`. Thus “fall back to a full scan on rename” is not a remedy for this parser defect. A newly initialized repository after the swap already exercises the full path without relying on refresh selection. [Refresh choice][refresh].

History delta parsing also shares `factsFromGitLog`. Endpoint diff detection and joining a partial history introduce additional identity questions, but this report does not need to prove a particular refresh split to establish the initial full-scan witness. Full and incremental equivalence belongs in the evaluation contract. [Delta parser][delta].

The projected predicate allowlist includes the affected raw summaries, daily/monthly observations and derived judgments. Incremental scan code calls `orient.Refresh`; applying the scan result publishes the returned orientation delta. This is not an isolated unused parser. [Projection surface][projection], [scan refresh adapter][scanrefresh], [application][apply].

## 5. Limits, controls and duplicate review

**Important controls**

- A simple one-way rename chain with no name reuse can work under the current map.
- A single lineage renamed away and back is explicitly covered by existing source tests; that is not the two-lineage permutation.
- No detected rename means this exact alias sequence does not occur. It does not establish that deletion/recreation is handled correctly.
- Shallow history is marked and `history_usable` gates chronological judgments. The wrong raw attribution may still exist, but this report uses a complete history for the downstream lineage witness. [Shallow-history gate][shallow].
- Equal first timestamps cannot support the strict time-order rule. Distinct creation dates are deliberate.
- Missing similarity, incompatible themes or independent ranking reasons can prevent the particular derived consequences.
- Changing filesystem mtimes is irrelevant; the parser uses committer epochs.
- The five-commit witness avoids merge ordering, copy detection, unusual filenames, dirty working trees and simultaneous renames. Those remain separate robustness obligations.

**Existing tests and documentation**

`TestFactsFromGitLog_RenameChainAndBack` checks an ordinary chain and one file's round trip. `TestScanHistory_Repo` exercises an actual Git repository and a single rename. Those are useful positive controls, but neither fixture exchanges two surviving files' names. They should remain passing under any remedy. These are source observations, not test-run results. [Chain/back fixture][tests], [Git-backed fixture][gittest].

Focused searches for `factsFromGitLog`, `ScanHistory`, `repo_file_history` and rename/swap terms found no existing audit or QA analysis of this exact conflation. The architecture timeline specification describes intended chronology/lineage rather than this counterexample. The existing orientation-treatment audit concerns acquisition/disclosure policy, and the symbol-focus audit concerns resolving present-day user targets to code symbols. Neither analyzes revision-scoped rename identity in the history sensor. [Timeline specification][spec], [treatment audit][treatmentaudit], [symbol-focus audit][symbolaudit].

This is a targeted duplicate screen, not an assertion that all historical branches or issues were exhaustively searched.

## 6. Proposed contract and remediation boundary

A focused repair should preserve file-lineage identities independently from display paths:

1. **Revision-scoped bindings.** At a particular traversal cut, map the path occupied there to an opaque lineage identity. Do not recursively interpret that identity as another mutable path.
2. **Rename transitions, not unions.** Walking backward across a detected rename transfers the destination's identity to the source binding at the parent revision. It must not globally equate both pathnames for every time.
3. **Commit-wide consistency.** Capture bindings for a commit's changes before updating them, so multiple renames cannot depend on output-row order.
4. **Lifetime boundaries.** Define handling of additions, deletions, copies and path reuse explicitly. An older occupant of a reused path is not automatically the same lineage. Copying identical bytes does not merge identities.
5. **Aggregation after attribution.** Accumulate timestamps, commit hashes and days against stable identities; project to the intended current/historical path view under an explicit policy.
6. **Ambiguity is evidence.** Detected renames are heuristic observations. Preserve their basis and report unsupported/ambiguous identity rather than claiming semantic continuity from pathname or content equality alone.
7. **State the history model.** A single linear-cut mapping is not by itself a solution for arbitrary merge DAGs. Per-parent edges, traversal order and merge counting need a declared contract and independent coverage.

A content hash alone is not a stable logical-file key: two independent documents can contain identical bytes, and an edited document can retain lineage while changing its hash. Likewise, deleting the cycle guard or merely compressing the alias chains is not a demonstrated fix. The current error is precisely that transitively resolving an old terminal path changes its referent.

Preserve the package's architectural boundary: Go measures faithful history and uncertainty; Mangle derives attention and lineage judgments. Do not compensate by adding repository-specific filename exceptions or silently changing attention policy.

## 7. Costs and fair evaluations — all unrun

**Costs.** The current approach streams one Git process and retains distinct commit/day witnesses plus path aliases. It also allocates a visited map during each transitive resolution. A lineage table can keep constant-time expected path-to-identity lookup for a supported linear traversal, but adds identity/binding state and explicit lifecycle management. Do not promise a speedup: maintaining parent-specific bindings for branches can cost substantially more memory and processing. Preserve bounded acquisition, cancellation and honest partial-history status.

No additional model or embedding calls are intrinsically needed to fix historical attribution. Corrected facts can change the selected documents and therefore downstream classification/token costs; compare those end-to-end costs rather than presenting fewer selected documents as automatic efficiency.

| Evaluation | Required result |
|---|---|
| Five-commit two-lineage swap | Exact per-current-path first/last, commit sets and day sets match the table's intended histories. |
| Positive controls | One-way chain, single-file rename-back, unchanged history and ordinary modification retain their existing semantics. |
| Unique-content real Git fixture | Capture actual name-status events from the normal command and verify parser attribution; distinguish Git detection from parser interpretation. |
| Conditional policy integration | With controlled sufficient similarity and compatible themes, chronological edge direction follows the correct births; inspect chain-tip and read reasons independently of final ranking. |
| Budgeted consumer comparison | Hold model, configuration, similarity facts and document bodies fixed; compare chosen paths, omission reasons and Northstar brief links. Do not claim answer quality from a fact-level pass. |
| Refresh split points | Measure from scratch and refresh across every boundary of the same history; compare normalized per-lineage facts and projection results. |
| Renames in one commit | Reordering equivalent status records cannot change attribution; batch transitions are evaluated from one revision cut. |
| Lifecycle controls | Delete/recreate, copy, same-content independent files and repeated temporary-name reuse do not silently merge unrelated lifetimes. |
| Graph and clock cases | Branch/merge histories, clock skew and shallow clones satisfy the declared model or disclose unsupported completeness. |
| Metamorphic checks | Rename only the temporary placeholder, or consistently relabel all paths: lineage counts remain equivalent after relabeling. |
| Performance | Equal event counts and correctness requirements; report Git time, parser allocations, binding/witness memory, full/delta latency and downstream selection cost separately. |

These are proposed evaluations, not executed tests or benchmark findings. A fair baseline must follow the same identity contract: a faster flat-path aggregation that assigns events to the wrong document is not an equivalent implementation.

## Conclusion

The static witness turns a valid sequence of three detected renames into a temporal identity collision. A pathname that once identified A later identifies B; transitive alias resolution forgets that time boundary. The resulting facts can reverse chronological lineage even while global commit counts and both current path rows look plausible. The narrow repair target is revision-scoped event attribution, with existing attention policy and permissions left intact.

[scan]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history.go#L28-L90
[aliases]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history.go#L172-L234
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history.go#L235-L321
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history.go#L337-L375
[months]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history.go#L388-L421
[init]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/init/initializer.go#L557-L581
[phase]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/init/phases.go#L158-L171
[run]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/init/phase_orient.go#L130-L160
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/snapshot.go#L180-L233
[lineage]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/lineage.mg#L16-L52
[superseded]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/timeline.mg#L420-L441
[chain]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/lineage.mg#L54-L64
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/timeline.mg#L123-L249
[bursts]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/timeline.mg#L327-L354
[reasons]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/timeline.mg#L600-L653
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/timeline.mg#L723-L762
[readconsumer]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/init/phase_orient.go#L165-L230
[brief]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/init/phase_orient.go#L384-L398
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/snapshot.go#L497-L524
[delta]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history_delta.go#L12-L36
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/snapshot.go#L269-L329
[scanrefresh]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/world/incremental_scan.go#L748-L772
[apply]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/world/apply_incremental.go#L106-L115
[shallow]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/timeline.mg#L30-L35
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history_test.go#L95-L145
[gittest]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/internal/orient/history_test.go#L177-L249
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/Docs/architecture/orient/05-TIMELINE-AND-LINEAGE.md
[treatmentaudit]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/Docs/audits/orientation-treatment-disclosure-contract.md
[symbolaudit]: https://github.com/theRebelliousNerd/codenerd/blob/f1632329be07b92e86ae07b1c9e96777e656651b/Docs/audits/symbol-focus-identity-continuity.md

