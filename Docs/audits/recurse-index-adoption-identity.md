# Recurse adoption and Git index ownership

## Finding and decision

**A recurse keep verdict is about the measured working tree, but the final Git commit is made from the shared index.** The helper stages only the supplied changed paths, then invokes a pathless `git commit`. A foreign staged version can therefore be included even when its path is absent from the attempt's changed set, owned-write receipts, and measured working-tree differences.

The strongest source witness is not a last-millisecond edit race. After the initial clean-checkout admission, another Git writer can leave an unrelated tracked file with its original working-tree bytes but a different staged version. That stable index-only difference is invisible to the collector's `git diff HEAD` comparison, yet remains eligible for the pathless commit.

**Recommendation:** bind adoption to an explicit candidate tree and admitted parent revision, with exact owned content and an independently preserved user index. A narrower path-limited commit is a useful containment, but it does not by itself bind published bytes to the verified snapshot.

Source pin: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), reconfirmed as main on 2026-10-30. This is a conditional static-source finding, not an observed bad commit or a test result. No application, host command, test, model inference, or training was executed.

## Prior ownership work and the narrower residual

The [recurse compensation audit][prior-compensation] already owns the general rule that a workspace difference is not proof of effect ownership. It explicitly mentions normal keep/revert adoption as well as cancellation, and proposes exact receipts and isolated worktrees. This document does not claim that broad contract as new.

Current code also repairs the old cancellation path: the executor returns observed writes, cancellation and refusal call `undoOwned`, and that helper preserves changed postimages it cannot attribute or safely restore. Mission attempts additionally inspect returned receipts, known before/after states, current postimages, protected verification inputs, and scope for the collected changed paths. These are substantial controls and are not bypassed by pretending an ordinary unowned working-tree edit is an owned one. [Owned return and cancellation][attempt-return], [mission attribution checks][mission-attribution], [owned compensation][owned-undo].

The narrower defect is **at the final publication consumer even when the collected path set is correct**. A foreign staged version does not have to enter that set. Repairing the old broad diff attribution alone therefore does not establish that the eventual commit contains exactly the admitted effects.

The current [mission specification][mission-spec] expressly disclaims an atomic transaction against arbitrary external editors between its checks and Git adoption. Credit that limit. This witness goes further in a different direction: the index-only version can be present throughout remeasurement and admission, with no subsequent working-tree race. The measured representation and the committed representation disagree before the final check.

The recent [campaign/recurse scheduler-ownership audit][prior-scheduling] concerns coexisting orchestrators and shared kernel relations. This case needs only one recurse loop plus an independent user or process interacting with Git. It does not depend on that scheduling gap.

## Production route

The TUI recurse loop supplies an executor that constructs a normal attempt campaign, builds an orchestrator, admits it with `SetCampaign`, calls `Run`, and returns `orch.KeptWrites()`. The outer loop performs its own measurements and ratchet judgment before adoption. Thus the inner campaign's acceptance and observed effects are not a replacement for the outer Git boundary. [TUI attempt adapter][tui-adapter], [deterministic attempt campaign][attempt-campaign].

Before running attempts, recurse takes a workspace-specific recurse lock, settles pending recovery, and calls Git preparation. Preparation refuses tracked dirt and switches or creates the recurse branch. This protects preexisting staged or unstaged tracked changes at startup, but it is a point-in-time admission test. The recurse lock excludes another recurse loop, not every editor or Git client. [Loop admission][loop-admission], [Git preparation][prepare], [recurse lock][lock].

After the executor returns without cancellation:

1. `changedByAttempt` collects changes using `recurseGit.changed`, then removes loop-ledger paths.
2. The loop remeasures applicable gates and constructs ratchet input from that changed set.
3. Mission mode, if used, checks receipts and scope for each collected changed path and records a workspace snapshot.
4. Only a genuine `ratchetKeep` reaches `r.git.commit(ctx, changed, ...)`.
5. On successful commit, the returned hash is attached to the kept outcome; later journal publication records the cycle.

The finding requires that these existing gates actually allow the owned attempt. Stock policy requires a changed result, a resolved finding or qualified improvement, and no regression or refusal before deriving keep. It does not predict a passing model verdict, invent a keep decision, or propose weakening the ratchet. [Stock ratchet][ratchet-rules] [Collection and measurement][collect-measure], [ratchet and mission snapshot][ratchet-snapshot], [keep publication and recording][keep].

## Concrete stable index-only witness

Use a valid Git workspace at the repository root, a normal recurse attempt, and the ordinary repository index; no inherited `GIT_INDEX_FILE` or alternate repository environment is in use. Let two tracked, regular text files exist:

- `pkg/repair.go`: the attempt's authorized file A;
- `notes/owner-note.md`: unrelated file B, outside the attempt's declared/observed effects and not otherwise touched by it.

At initial preparation both files and the index match HEAD. Then:

| Stage | File A | File B | Required condition |
|---|---|---|---|
| Initial admission | HEAD version | Working tree B0; index B0 | Clean tracked checkout accepted; no unresolved in-flight recovery. |
| Independent staging after admission | Attempt may be running | Another writer stages B1, then restores **only the working-tree bytes** to B0; index remains B1 | No commit, branch move, reset of the staged version, or change by the attempt to B. |
| Post-attempt collection and checks | Owned A1, different from HEAD | Working tree still B0; index still B1 | All required measurements inspect the intended working tree and pass the actual ratchet requirements; no custom gate refuses the index state. |
| Git helper stages supplied paths | A1 added to index | B1 remains staged | The supplied path list contains A, not B; Git add succeeds. |
| Pathless Git commit succeeds | A1 recorded | **B1 also recorded** | No cancellation, lock failure, hook refusal, or other Git error; HEAD and the unrelated staged entry remain stable. |

B1 may be ordinary user work staged for a separate commit. The review need not characterize it as malicious, secret, invalid, or harmful. The failure is adopting that content as part of the recurse cycle without carrying its identity through the owned candidate and verification contract.

This is a legal state of Git's separate working tree and index. Git documents `diff <commit>` as comparing the working tree with that commit, while `--cached` inspects staged changes. A pathless commit records the index. The two command forms do not certify the same bytes. [Git diff documentation](https://git-scm.com/docs/git-diff), [Git commit documentation](https://git-scm.com/docs/git-commit).

The independent staging must happen **after** the initial clean-checkout check; a foreign staged change present before that check is a refusal control, not this witness. It can then remain stable before the changed-set collection and throughout verification. This separates the representation gap from the mission specification's acknowledged check-to-adoption race.

## Exact source mechanism

### The collector compares working-tree bytes, not candidate commit content

`recurseGit.changed` runs `git diff --name-only --relative -z HEAD -- .`, then adds newly untracked paths not present in the attempt's initial untracked-name snapshot. With B's working tree restored to B0, B has no net working-tree change against HEAD. It is already tracked, so the new-untracked pass does not add it. The collector can correctly return only A. `changedByAttempt` merely filters the recurse ledger afterward; it does not inspect the index. [Git collector][changed], [outer filtering][changed-filter].

### Mission controls certify the collected working tree, not an unseen index blob

For mission mode, `last` is built from returned write receipts, and every path in `changed` must be attributed, have known states, match its recorded postimage, and satisfy verification-input, protected-path, and mission-scope rules. The B index blob is not checked by that loop because B is not in `changed`. This is not a failure of the A receipt check. [Mission checks][mission-attribution].

The evidence snapshot hashes file names, modes, and bytes read from the filesystem, excludes Git administration, and does not construct a Git candidate tree. B0 is therefore the snapshot's working-tree value. Its staged B1 does not become part of that evidence just because Git later commits it. Keep the snapshot's intended filesystem contract; do not relabel it as a Git-index certificate. [Snapshot implementation][snapshot].

This also distinguishes the residual from a generic argument to hash more often: repeating the same working-tree snapshot while the index-only state remains stable still observes B0.

### The commit helper widens beyond its argument

The helper receives `paths`, rejects an empty list, and uses NUL-delimited pathspec input for `git add --all`. That is a useful path-selection and filename-handling control. But its next argument vector is only `commit --quiet --file=-` plus optional identity configuration. It contains neither a pathspec nor a private-index selection. The Git runner sets working directory and streams but no environment override; in the stated default environment it uses the repository index. [Commit helper][commit], [Git runner][git-runner].

Thus limiting the preceding add does not limit a later pathless commit. The helper's own comment says it records the supplied paths, and the keep test checks the resulting commit's changed file list, but that test's commit fixture has no unrelated staged difference. Its positive result is compatible with the residual. [Existing keep test][keep-test].

On success the cycle records the commit hash and increments the kept count. The helper obtains the hash from HEAD after committing; it does not compare the created commit's tree or parent with the measured candidate. A durable commit hash is valuable identity evidence, but alone is not evidence that its content equals the admitted output. [Commit result][commit], [kept outcome][keep].

## Consequences and limits

Under the stated successful-commit conditions, the local recurse commit contains a foreign staged B1 that was not in the supplied changed set. Its cycle/finding trailers and kept record attach to the whole commit. The working tree can still hold B0 afterward; publication does not need to overwrite the editor's current bytes to cross this ownership boundary.

No push or remote publication is established by this path. This is not a claim of irreversible loss, privilege escalation, arbitrary command execution, or an automatically exploited security vulnerability. A user may intentionally want all staged work committed, but the current attempt-specific API and measured-snapshot contract do not express that separate authorization.

Other important controls and counterexamples:

- A staged difference present at startup is rejected by the clean-checkout admission.
- The TUI now holds listed write commands and ordinary mutation turns while recurse runs. The witness uses a distinct Git writer, not the old permitted `/write` path. [Current chat hold][chat-hold], [mutation hold][mutation-hold].
- A custom gate or hook that detects/refuses the foreign staged entry can stop publication. That is not a stock guarantee of this helper.
- Without an independently accepted nonempty attempt, `ratchetKeep`, and successful Git operations, the claimed commit does not follow.
- If B remains different in the working tree, it can enter `changed` and mission attribution can reject it. That is a different case and an important positive control.
- If B1 is removed from the index before commit, or commit is already isolated to another index, this witness does not hold.
- Path, task, risk, verification, and mission authority checks remain active for model-originated effects. The foreign version was staged by another owner; this is a publication-boundary issue, not proof the model escaped its write guard.
- A Git index lock can serialize individual Git commands or make a competing operation fail. The witness needs no simultaneous Git command: the foreign index state can be fully established and stable before the helper runs.
- Existing compensation must continue preserving foreign work. Refusing adoption is not permission to reset, unstage, discard, or recommit that work.
- Git hooks, filters, line-ending normalization, modes, symlinks, and submodules affect exact tree identity and require explicit coverage in a repair. No installed Git version or platform behavior was measured here.

## Proposed adoption contract

Update the owning campaign/mission specification first. The host should report an exact candidate identity and publication result; policy should retain authority over whether that candidate may be adopted. No new model judge is needed.

A candidate receipt should bind:

- attempt/cycle identity and the authoritative owner;
- repository/worktree identity, admitted parent commit, and destination ref;
- exact owned file operations, modes, and normalized content/blob identities;
- the materialized candidate tree certified by required checks;
- the observed index/ref conflict disposition;
- resulting commit/tree/parent identity, or an explicit uncertain-publication outcome.

Successful adoption must not contain an extra path or a different content version merely because it was already staged. Preserve the user's independent index and working-tree state. If exact ownership or candidate identity cannot be established, defer adoption visibly rather than broadening the commit or clearing the index.

### Implementation options

**1. Path-limited commit containment.** Passing only the admitted paths to commit can exclude unrelated staged paths; Git documents that path-limited mode leaves other staged content out. Preserve filename-safe transport and verify added/deleted path behavior. This is the smallest response to the foreign-index witness, but it reads current working-tree content and does not by itself close same-path changes after verification, moved HEAD, hook mutations, or ownership ambiguity in the supplied list. Treat it as bounded containment, not full snapshot-bound publication. [Git path-limited commit semantics](https://git-scm.com/docs/git-commit).

**2. Attempt-owned index and exact candidate tree.** Git supports an alternate index via `GIT_INDEX_FILE`. Seed an attempt-owned candidate from the admitted base and apply only verified owned effects. Validate the resulting tree, preserve the user index, and publish against the expected parent/ref with explicit conflict handling. A private index alone is not a sandbox and does not make current working-tree reads immutable; candidate creation, hooks, filters, ref update, and cleanup still need a coherent contract. [Git alternate-index documentation](https://git-scm.com/docs/git).

**3. Isolated attempt worktree plus guarded promotion.** This follows the prior ownership proposal and provides separate working-tree/index state for the entire attempt. It is the larger change: all tools, context, scanners, gates, receipts, and command directories must use the same identity. Promotion into the destination remains a checked operation; shared Git objects/refs and external writers do not vanish.

A pre-commit check of staged paths is useful defense and diagnostics, but a read-then-commit check alone is not atomic against a subsequent Git writer. Conversely, requiring global exclusivity without a real owner protocol can create a false safety claim. Prefer the smallest design that meets the declared contract and explicitly list what remains outside it.

For uncertain commit acknowledgement, retain the adoption intent and reconcile actual commit/tree identity before retrying. Do not automatically reset the repository or assume an error means no commit landed. The current keep path already marks adoption as started before invoking Git, and mission mode preserves ambiguous intent rather than compensating a possible commit; retain that safeguard. [Adoption-start guard][keep], [mission recovery contract][mission-spec].

## Cost and fair evaluation

None of these mechanical protections inherently needs an LLM, judge, embedding, or training request. Their costs are host-side Git work, tree/blob hashing, IO, lock duration, metadata retention, conflict handling, and potentially re-running checks when a candidate changes.

A path-limited containment has relatively low integration cost but still needs compatibility coverage and precise claims. A private index adds candidate lifecycle, base/tree verification, environment management, and recovery artifacts; index/tree processing scales with repository metadata and changed content. A full worktree adds disk usage, checkout/setup time, duplicate caches or synchronization, and changes across tool/context/check routing. Serial adoption can increase wait time but avoid contaminating unrelated user staging.

Measure end-to-end accepted-work cost: Git subprocess count and duration, bytes hashed/materialized, peak disk and memory, time holding ownership/ref coordination, rejected/conflicted publications, recovery effort, rechecks, model calls/tokens indirectly caused by retries, and elapsed time to a genuinely adopted result. Do not count a refusal of every commit as a correctness/performance win.

Proposed evaluation, not executed:

| Fixture | Required result |
|---|---|
| Normal owned modification/addition/deletion with clean index | Same legitimate keep behavior and correct cycle trailers; acceptance obligations unchanged. |
| Foreign staged change exists before startup | Existing preparation refusal remains; no work or destructive cleanup. |
| Primary index-only B1, working-tree B0, established after admission and before measurement | Candidate commit contains only admitted A effects; B's staged version and working-tree bytes remain the user's. |
| Foreign staged entry appears after collection but before commit | No extra publication; conflict/refusal is explicit, with foreign work preserved. |
| Foreign working-tree change is present in `changed` under mission mode | Existing unattributed/protected/out-of-scope refusal still works; do not weaken it to accommodate the new index contract. |
| Same path A changes after verification, or owned receipt postimage no longer matches | Stale candidate is refused or reverified; a path-only allowlist is not treated as content identity. |
| Workspace is a subdirectory of the Git repository; foreign staged file is outside it | Publication remains inside the admitted candidate even though the real index covers the larger repository. |
| Spaces, newlines, leading dashes, adds, deletes, mode changes, filters and CRLF | Exact candidate tree is checked; filename-safe behavior is preserved. |
| Hook refusal or mutation; signing failure; index/ref contention; moved HEAD | No falsely reported keep, no destructive reset, explicit conflict/uncertainty handling. |
| Crash/cancellation before add, between add and commit, after commit before journal, and during acknowledgement | Reconcile once from exact publication identity; preserve user index and already committed work. |
| Revert/refuse/cancel instead of keep | Existing observed-write compensation still leaves foreign versions alone. |
| Ordinary recurse, mission recurse, TUI and CLI adapters | Shared helper contract holds at production entry, not only in a direct helper unit test. |

Begin with deterministic fixtures using the real Git runner, canonical observed returns, and production ratchet composition, with explicit barriers for the independent writer. The primary index-only case should be fully established before the verification stage; a timing sleep is unnecessary. Inspect the actual commit's parent, tree, paths and blobs, not only exit code, kept count, or a log message. Record the user index and working tree independently before and after.

The existing `TestRecurseGit_ChangedKeepCommitRevert` is a positive control for normal owned files and preexisting untracked-file preservation. `WorkspaceInASubdirectory` currently checks collection and revert, not a keep with a foreign staged entry. Neither inspected fixture proves final commit isolation. Preserve them and add the missing dimension rather than replacing them. [Git tests][keep-test], [subdirectory test][subdir-test].

Compare current behavior, path-limited containment, private-index adoption, and isolated-worktree promotion at matched authorized tasks and verification obligations. Separately authorized real-model evaluation is not needed to establish this deterministic boundary, though it may later quantify operational overhead and keep-rate effects.

## Scope and branch screen

Root, public-repository, campaign, and mission guidance were read. The source and ownership screen covered the existing compensation audit, mission integration proposal/specification, campaign recovery and recent audit inventories through [`research-main-20261030-1227`][latest]. The previous audit's broad adoption warning is credited, not relabeled as a new discovery.

Main remained at the cited pin. The active store, session, OAuth, orchestrator-QA, and cartographer-QA heads were inspected at `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, `51cfd42649552866d4aa54c53b68acf5740eb09b`, and `767a3291d6287b238417f2466592a9002810d05a`. Their `recurse_git.go` blob is identical to main. Older caller slices differ but retain the changed-set-to-commit path. This is a bounded source screen, not a claim of complete branch equivalence or exhaustive historical novelty.

## Immutable source links

[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/recurse-compensation-write-ownership.md#L45-L98
[prior-scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/campaign-recurse-scheduling-ownership.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L50-L80
[tui-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L205-L253
[attempt-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L33-L134
[loop-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L329-L392
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L50-L76
[lock]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_control.go#L12-L55
[attempt-return]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1028-L1062
[collect-measure]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1064-L1136
[mission-attribution]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1090-L1128
[ratchet-snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1138-L1189
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1212-L1291
[owned-undo]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1573-L1606
[changed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L80-L122
[changed-filter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1362-L1376
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/evidence/change.go#L169-L246
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L125-L147
[git-runner]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L35-L47
[keep-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git_test.go#L83-L151
[subdir-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git_test.go#L154-L175
[chat-hold]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands.go#L142-L155
[mutation-hold]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L403-L409
[latest]: https://github.com/theRebelliousNerd/codenerd/commit/4585e7fa4732597bd37e0bfe36357f0a3ba71031
[ratchet-rules]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L296-L329

