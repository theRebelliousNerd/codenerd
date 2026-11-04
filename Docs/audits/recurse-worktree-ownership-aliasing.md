# Recurse live ownership must follow the Git worktree, not the selected subdirectory

**Status: static source-derived integration finding and proposed contract.** Public `theRebelliousNerd/codenerd` main was inspected at **`30bdd99c0404578a20f1d6a07e108a69abde55a2`**, observed November 4, 2026 at 09:38 UTC and rechecked unchanged at 09:47 UTC. All CodeNERD links below pin that revision. No application, test, build, gate, Git mutation, model inference or failure injection was run. Existing tests were read as source, not executed. No implementation, repository, credential or setting was changed. The schedules and evaluations are conditional and unrun.

## Decision in brief

**Two ordinary recurse processes can acquire different “one loop” locks while operating on the same Git worktree.** The selected workspace may be a repository subdirectory. Recurse canonicalizes that directory and locks its own `.nerd/recurse/lock`; Git preparation accepts the containing repository but discards the returned top-level path. A run rooted at `/repo` and one rooted at `/repo/service` therefore have different lock files but the same checkout, HEAD and index. [Workspace producer][cli-root] [Canonicalization][canonical] [Owner admission][admission] [Lock][lock] [Git preparation][git-prepare]

This is a live ownership-domain mismatch, not another recovery-cycle collision. Its minimal consequence is simultaneous admission of cooperating recurse owners to one checkout. A stronger ordinary-mode schedule can cause the parent-root run to commit both processes' independently valid edits, after which the child-root run sees no remaining diff, derives revert and restores its own preimage into the newly committed tree. The kept commit remains in history, but the worktree no longer matches it. This consequence requires the explicit execution and scheduling conditions below; no runtime incident or frequency is claimed.

**Recommended contract:** reserve the mutable Git worktree before reconciliation, branch preparation or attempt execution, while preserving the narrower requested workspace as the source/write scope. Separate source scope, live worktree ownership, campaign identity and durable attempt identity. Do not widen tools to the repository root merely to make locks contend.

## 1. Architectural intent and prior-work boundary

Root guidance gives the model creative work and Mangle executive decisions. The recurse driver similarly states that host code measures and executes while policy decides what runs and what is kept. The owner lock is a deterministic admission mechanism supporting that division, not something an ideation partner or requirements interrogator can establish from prose. A portable monorepo workflow needs correct resource ownership before spending on design and verification. [Guidance][guidance] [Driver contract][driver] [Campaign architecture][architecture] [Portable proposal][portable]

The bounded nonduplication screen reviewed the current audit inventory, all current `recurse-*.md` reports, the portable deliberation proposal, and the campaign/recurse scheduling report. Closest work:

- **Recovery attempt identity** already observes that sibling subworkspaces have separate locks, journals and cycle counters. Its witness is explicitly sequential and repairs the local-cycle-to-shared-history join. It says repository serialization alone cannot solve that witness. The present finding does not claim discovery of separate local state; it traces the unqualified **simultaneous** admission and its live keep/compensate consumers. Unique attempt IDs do not make two live locks contend. [Prior recovery][prior-recovery]
- **Compensation write ownership** traces an uncoordinated TUI writer and introduced owned-write restoration. Current source does preserve those receipts. This witness uses two ordinary recurse owners with separate processes/kernels, and demonstrates that byte-correct compensation can still undo bytes another admitted owner has just published. [Prior compensation][prior-compensation] [Owned restoration][undo]
- **Campaign/recurse scheduling ownership** concerns two doors sharing a TUI kernel and unscoped scheduler relations. Here the processes have independent kernels and campaigns, so none of those relation-ordering conditions is needed. [Prior scheduling][prior-scheduling]
- **Index adoption identity** concerns foreign staged content, and the current path-limited commit is a real containment. Here both files are already in the parent root's observed working-tree difference set before staging; path-limited commit faithfully includes both. No index-only edit, pathspec wildcard or post-verification producer is required. [Prior index][prior-index] [Current commit][commit]
- **Parallel baseline interference** concerns checks deliberately launched by one admitted owner. This report concerns two admitted owners even if each owner's checks are individually serial and read-only. [Prior baseline][prior-baseline]

No earlier reviewed report supplied this live worktree-lease witness and closure criterion. This is a bounded source/audit screen, not an exhaustive assertion about every historical branch or issue.

## 2. Production source trace

### 2.1 Selected source scope remains a subdirectory

The CLI's `recurseWorkspace` returns the explicit global workspace or current directory. It passes that value to `RunRecurseCycles`; chat passes its own workspace to the same driver. `CanonicalWorkspaceRoot` resolves absolute paths, symlinks and existing ancestors. It does not replace a real subdirectory with a containing Git root. This behavior is appropriate for tool containment. [CLI selection][cli-root] [CLI run][cli-run] [Chat run][chat-run] [Canonicalization][canonical]

Subdirectory support is not merely speculative input: `TestRecurseGit_WorkspaceInASubdirectory` creates a `svc` workspace beneath a repository and checks that collection/reversal leave the other subtree alone. The test is valuable coverage for **scope**, not for two live owners. [Subdirectory fixture][subdir-test]

### 2.2 The lock key is source scope, not the shared mutable resource

The driver calls `acquireRecurseLock(root)` and holds its release until return. The helper opens `root/.nerd/recurse/lock` and applies the platform's file lock. Identical canonical workspace roots contend; distinct directories open distinct files. Neither the helper nor the driver resolves an enclosing worktree owner. Existing lock tests cover repeated acquisition for the same root. [Admission][admission] [Lock][lock] [POSIX implementation][posix-lock] [Windows implementation][windows-lock] [Lock fixtures][lock-tests]

This is not a failure of `flock` or `LockFileEx`. Both locks can operate exactly as specified on different files. Symlink canonicalization also remains useful; a parent and its actual child are different directories, not two spellings of one directory.

### 2.3 Git operates in the containing worktree

`recurseGit.run` sets `cmd.Dir` to the selected workspace. `prepare` invokes `git rev-parse --show-toplevel` but only tests its error; it neither checks that the answer equals the workspace nor uses the answer as a shared lock identity. It then reads tracked status and selects/creates the recurse branch. [Git execution and preparation][git-prepare]

Git documents `--show-toplevel` as the worktree's top-level path, and provides `--absolute-git-dir` and `--git-path` for administrative identity. Git's worktree documentation distinguishes real linked worktrees, which have their own HEAD and index, from subdirectories inside one worktree. A different process working directory does not create another worktree. [Git rev-parse](https://git-scm.com/docs/git-rev-parse) [Git worktree](https://git-scm.com/docs/git-worktree)

The tracked-clean admission is a point-in-time check. Both processes can finish it before either makes an edit. The witness uses the same already-selected default `nerd/recurse` branch, so no competing branch switch is required. The exported `Branch` configuration and branch preparation would also need the same shared ownership contract, but the CLI need not expose a branch flag for the primary finding.

### 2.4 Ordinary adoption and compensation consume different identities

After an executor returns, the outer driver collects differences from the **current HEAD** under its workspace, plus newly untracked names. That census is not a join with the returned owned-write receipts. In mission mode, the separate block checks every changed path against owned postimages and admitted scope; ordinary mode does not enter that block. [Changed census][changed] [Outer collection and mission guard][outer]

On keep, the host passes the entire collected set into the path-limited Git commit. On revert, it calls `undoOwned`, which restores the callback's actual owned preimages while current bytes still match the callback's postimages. The restoration does not ask whether another owner has committed those bytes since capture. That omission is safe only under an adequate ownership/adoption protocol. [Keep and revert][keep-revert] [Owned restoration][undo] [Outer undo][outer-undo]

Per-campaign publisher locks are scoped to campaign IDs and local campaign directories; the write-set manager keeps process-memory ownership maps. They do not replace a common cross-process worktree owner. The scenario can use disjoint file targets, so a per-file conflict is unnecessary. [Campaign owner][campaign-owner] [Write-set manager][write-manager]

## 3. Conditional witness A: two live owners

Use one ordinary Git worktree `/repo` with a child project `/repo/service`. Neither directory is a separate repository or linked worktree. Configure their required runtime/project inputs normally, ignore both runtime `.nerd` trees, and provide valid independently selected work. Use separate processes, kernels and campaign state. No mission is enabled. No external editor, alternative index, malformed paths, recovery intent or corrupted state is needed.

1. Start ordinary recurse A at `/repo`. It acquires `/repo/.nerd/recurse/lock`.
2. Before any tracked edit, start ordinary recurse B at `/repo/service`. It acquires `/repo/service/.nerd/recurse/lock`.
3. Let both complete Git preparation while the tracked worktree is clean and already on `nerd/recurse`.
4. Both can continue toward their real graph, gate and attempt paths. There is no shared recurse lock to reject B.

The source-derived result is **double ownership admission**, not a claim that every pair reaches model execution. All independent boot, graph, gate, campaign and permission checks still apply. If B starts only after A has dirtied tracked files, preparation may reject it; that does not cover the clean admission interval.

## 4. Conditional witness B: an adopted peer edit is subsequently restored

Continue A/B with two tracked files under the child workspace:

- `service/a/a.py`, initial content A0, edited only by A to A1.
- `service/b/b.py`, initial content B0, edited only by B to B1.

The names and contents are generic. The Python source walker can acquire both directories from either selected root; this avoids assuming discovery across nested Go module boundaries. Provide valid owner-declared read-only checks with accurate findings and the required scope. Each edit is a legitimate in-scope repair of its own selected finding. Both callbacks return complete, accurate owned-write observations. They need not edit the same file. Hold line/test metrics and protected inputs unchanged, and require that A's actual ratchet permits its repair in the resulting A1/B1 tree. A can focus `service/a` while B focuses `b`; the collector still uses the parent workspace scope. These are conditions for the witness, not assumed outcomes of a live model. [Graph acquisition][graph]

A finite schedule:

| Boundary | Source-derived state |
|---|---|
| Both complete startup before writes | Both locks remain held; HEAD contains A0/B0. |
| A's callback produces A1; B's callback produces B1 | Each owns only its own write. Both complete any inner acceptance before the relevant return boundary. |
| B's completed callback has not yet returned to its outer driver | B1 remains uncommitted and B's precise B0→B1 receipt remains available. |
| A returns, collects changes and passes its real ratchet | Its root-relative set contains both tracked paths. Ordinary mode has no owned-write filter at this point. |
| A's path-limited commit succeeds | HEAD now contains A1/B1. The collected set includes both, so path limiting does not exclude B. No simultaneous Git subprocess is needed. |
| A finishes or pauses without another source edit; B returns | B's diff from current HEAD is empty because B1 is already committed. Runtime files remain ignored. |
| B judges its attempt | `Changed=false`; stock policy cannot derive keep and derives revert absent a separate refusal. |
| B applies its accurate owned restoration | B's file still equals B1, so the postimage check succeeds and `undoAttemptWrites` writes B0. HEAD remains A1/B1 while the worktree contains A1/B0. |

The no-change rule and restoration are explicit code paths. [Policy][ratchet-policy] [Collection/keep/revert][outer] [Keep/revert][keep-revert] [Undo][undo]

The resulting mismatch is not irreversible data loss: B1 is recoverable from A's commit. It is nevertheless a concrete correctness failure. A can report a kept repair while B's legitimate cleanup has returned that file to its older content; B records a reverted attempt despite its repair residing in another run's commit. Subsequent checks may rediscover the defect, or a later startup may refuse the dirty checkout. Those later safeguards do not make the live ownership/adoption accounting correct.

The stronger consequence is conditional on the callback schedule, A's real keep verdict and successful Git/restore operations. An inner campaign can independently block either attempted edit; a custom check can detect interference; a mission's ownership guard can reject A's foreign B change. The report does not assert those controls are bypassed or that all recurse runs exhibit the schedule.

## 5. Required contract and bounded correction

Update the owning campaign specification before implementation:

1. **Preserve source scope separately.** The requested workspace determines admitted reads/writes, project policy, gates and local history. Resolving worktree ownership must not silently widen it.
2. **Identify the actual mutable worktree.** Obtain and canonicalize the applicable Git worktree/admin identity through supported Git queries. Handle ordinary repositories, linked worktrees and gitfiles deliberately. A shared object store is not the same as a shared working tree/index.
3. **Acquire one cooperating owner before shared effects.** Cover in-flight reconciliation, branch selection, discovery/checks with effects, attempt execution, adoption, compensation and final settlement. Avoid acquiring only immediately around individual Git commands: the A/B schedule uses sequential Git operations.
4. **Retain exact owner metadata.** Identify the active workspace, run and generation sufficiently for a useful refusal/status result. Do not publish absolute private paths in commit trailers merely to solve local coordination.
5. **Fail closed on unresolved ownership identity.** A failed repository lookup or lock error must not fall back to independent workspace locks while retaining a guarantee of exclusive checkout ownership.
6. **Coordinate migration and controls.** Old and new binaries using unrelated lock domains could both run. Define compatibility or an explicit version barrier. Define which active owner status/stop commands discover without converting an arbitrary child-root stop into unbounded authority over unrelated work.
7. **Keep evidence/adoption defenses.** A correct shared lease contains this cooperating-owner witness, not arbitrary editor races. Preserve owned writes, scope guards, literal path handling, exact candidate verification and ambiguous-adoption reconciliation. Do not replace receipt-aware compensation with a blanket reset.

The smallest policy is one live recurse owner per Git worktree. It conservatively serializes sibling source scopes too because they still share HEAD/index and branch state. A future parallel design needs isolated worktrees or a qualified shared transaction protocol, including current-base comparison, exact owned publication and compensation that cannot undo an already-adopted peer effect. Merely assigning different branch names in one worktree or adding process-local mutexes is insufficient.

A new model/JEV decision is not needed to decide whether two scopes share one worktree. Host observations should establish identity and lease facts; Mangle retains attempt selection, permission and verdict decisions. More Socratic rounds cannot repair a lock keyed to the wrong resource.

## 6. Costs and failure modes

- **Identity work:** extra Git discovery, canonicalization and local lock/metadata I/O at startup. Cache only within a valid worktree identity/lifetime; quantify cold starts and network-mounted paths.
- **Serialization:** same-worktree runs may wait or be refused, reducing apparent throughput. Report owner wait and total completed verified work, not only one worker's latency. Reusing separate linked worktrees adds disk, checkout and cold tool-cache costs.
- **Lifecycle:** error handling must release only owned leases and preserve unresolved effects. Never unlink a live lock file as a “stale PID” repair: replacement can split lock identity while the original holder still owns its open file.
- **Scope mistakes:** promoting the source root to the Git root can grant unintended reads/writes and change gate selection. Conversely, locking only a selected package keeps the defect.
- **Topology mistakes:** a shared common Git directory can over-serialize distinct linked worktrees; locking unrelated submodules together can obscure separate repositories. Under-serialization and over-serialization need separate measurements.
- **Compatibility:** mixed versions, worktree relocation, case aliases, symlinks, separate Git directories, unsupported network lock semantics and alternate Git environment require explicit outcomes. Do not claim universal filesystem transactionality.
- **Remaining independent work:** sequential recovery identity, foreign-editor protection and exact adopted-tree evidence remain necessary. None is repaired merely by this lease.

No performance, cost saving, production prevalence or success-rate result is claimed.

## 7. Fair evaluation plan, entirely unrun

Begin with deterministic temporary repositories and separate processes, real OS locks, shipped recurse policy, real Git and fixed receipt-producing callbacks. Use barriers at the source boundaries rather than probabilistic sleeps. Compare the pinned baseline with a minimal worktree-scoped lease and any proposed isolated-parallel alternative under identical gates, source bytes, permissions and scheduling.

| Case | Required observation |
|---|---|
| Same canonical root twice | Preserve current refusal and release/reacquire behavior. |
| Parent/child roots, both clean before writes | Only one live shared-worktree owner enters recovery/preparation/execution. |
| Strong A/B receipt schedule above | No peer edit is adopted under another attempt and then restored after publication; worktree, commit and ledgers agree. |
| Disjoint siblings in one worktree | Honest shared-resource admission, not presumed isolation from disjoint file paths. |
| First owner finishes or dies before second starts | Correct release and existing recovery behavior, no permanent false hold. |
| Separate repositories, nested real repository, separate linked worktrees | Apply declared resource policy without unintentionally widening scope or banning qualified independence. |
| Symlink/case alias, spaces/non-ASCII paths, gitfile/separate Git directory | Equivalent owner identity or explicit unsupported outcome. |
| Repository discovery/lock/read failure | No silent fallback that admits two owners. |
| Mission and ordinary mode | Preserve mission ownership/snapshot/oracle guards; avoid claiming the ordinary contamination schedule necessarily works in mission mode. |
| Status and stop from parent/child roots | Active owner and requested target are identified truthfully under the declared control contract. |
| Existing pending intent, crash before/after adoption | Lease and recovery boundaries compose without false kept attribution or blind repeated effects. |
| Single ordinary successful repair | Still completes normally; a fix that refuses all work is not success. |
| Noncooperating editor during one owner | Existing protections remain; the new lease is not advertised as an external-editor sandbox. |

Observe lock paths/identities, acquired owner count, callback entry, selected source scope, both journals, HEAD/index trees, residual diff and exact pre/postimage receipts. Count false admissions, peer-file adoption, false holds, restoration after peer commit and recovery ambiguity separately. Measure process launches, lock hold/wait time, bytes copied/hashed, verification reruns, elapsed time and any model retries caused by conflict.

Then qualify normal CLI and chat routes on the same small projects with their real campaign acceptance, world-model context and guards. The deterministic callback study establishes the host boundary, not full model-driven end-to-end behavior. Any later model study should hold model/version, budgets and task population comparable and charge orchestration/serialization cost to the design.

**Closure criterion:** different selected subdirectories of one mutable Git worktree cannot become independent live recurse owners through separate local lock paths; a kept candidate and later compensation must remain consistent with the declared owner and adoption state. Preserve legitimate subdirectory scope and single-run progress.

## Pinned references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/architecture/campaign/INTERNALS.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/audits/portable-recurse-deliberation-crucible.md
[driver]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_cycle.go#L28-L47
[cli-root]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/cmd/nerd/cmd_campaign_recurse.go#L120-L126
[cli-run]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/cmd/nerd/cmd_campaign_recurse.go#L226-L264
[chat-run]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/cmd/nerd/chat/campaign_recurse.go#L218-L279
[canonical]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/tools/workspace_guard.go#L58-L88
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_cycle.go#L356-L445
[lock]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_control.go#L12-L76
[posix-lock]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_lock_other.go#L1-L26
[windows-lock]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_lock_windows.go#L1-L29
[lock-tests]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_control_test.go#L11-L52
[git-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_git.go#L35-L77
[subdir-test]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_git_test.go#L154-L177
[changed]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_git.go#L93-L124
[outer]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_cycle.go#L1324-L1391
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_git.go#L125-L149
[keep-revert]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_cycle.go#L1479-L1534
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/orchestrator_attempt_writes.go#L186-L237
[outer-undo]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_cycle.go#L1886-L1919
[ratchet-policy]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/core/defaults/policy/recurse.mg#L296-L329
[campaign-owner]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/campaign_owner.go#L17-L65
[write-manager]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/write_set_lock_manager.go#L25-L40
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/internal/campaign/recurse_workspace.go#L457-L486
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/audits/recurse-recovery-attempt-identity.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/audits/recurse-compensation-write-ownership.md
[prior-scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/audits/campaign-recurse-scheduling-ownership.md
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/audits/recurse-index-adoption-identity.md
[prior-baseline]: https://github.com/theRebelliousNerd/codenerd/blob/30bdd99c0404578a20f1d6a07e108a69abde55a2/Docs/audits/recurse-parallel-baseline-interference.md

