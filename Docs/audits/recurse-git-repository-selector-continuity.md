# Recurse Git operations must stay bound to the selected workspace

**Status: static source-derived integration finding and proposed contract.** Public `theRebelliousNerd/codenerd` main was inspected at **`90353b5c3effbaa45e7647834fe442527e6acb2f`**, observed November 4, 2026 at 10:07 UTC and rechecked unchanged at 10:16 UTC. All CodeNERD links below pin that revision. No application, test, build, gate, Git command, model inference or failure injection was run. Existing tests were read as source. No implementation, repository, credential or setting was changed. The witnesses and evaluations below are conditional and unrun.

## Decision in brief

**A recurse run selected for workspace A can prepare and switch an unrelated repository B before executing any repair.** The Git helper sets its subprocess working directory to A but leaves the environment inherited. Absolute `GIT_DIR` and `GIT_WORK_TREE` values selecting B therefore remain effective. Preparation asks Git for its top-level directory but discards the answer, checks B's cleanliness, and switches or creates the recurse branch in B. Meanwhile the driver has loaded A's project policy and acquired A's recurse lock. [Git invocation and preparation][git-run] [Driver admission][startup]

This is a target-binding failure, not another concurrent-owner or staged-content witness. One process and two separate ordinary repositories suffice. The minimal consequence is a local ref/HEAD mutation in B under an A-scoped run. A successful repair, keep verdict, model mistake, crash, malformed filename, external writer or race is unnecessary.

**Recommended contract:** derive an admitted Git repository/worktree context from the explicitly selected source workspace, establish that the workspace belongs to that context, and keep recovery, cleanliness checks, branch preparation, census, adoption and reversal bound to it. Ambient repository selectors must not silently choose a second target. Preserve supported subdirectory workspaces rather than requiring the selected directory to equal the Git top level.

## 1. Architectural intent and existing protection

Root guidance assigns creative work to the model and executive control to Mangle, with workspace-aware tools and deterministic safety. The recurse CLI promises to improve the selected workspace using its imports and gates; the Git helper describes itself as acting in the owner's checkout. Choosing which repository receives host effects is a deterministic admission fact, not a judgment the proposer or requirements interrogator can establish from prose. [Guidance][guidance] [CLI contract][cli-contract] [Git contract][git-contract]

The repository already handles the relevant environment distinction in a neighboring subsystem. `internal/workspace/git.go` constructs a child environment that removes `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY` and `GIT_NAMESPACE`, and supplies it to the membership Git subprocess. Its comment expressly says this prevents the parent process's selected repository from overriding the requested workspace. The current membership audit credits that behavior as an existing protection. [Membership environment][membership-env] [Prior membership audit][prior-membership]

That protection is local to the child command. It does not unset variables in the CodeNERD process and is not called by `recurseGit.run`. Membership can consequently read A while the recurse Git lifecycle operates on B. The minimal startup witness does not require graph acquisition to finish: the foreign branch operation occurs earlier. [Membership invocation][membership-env] [Recurse invocation][git-run] [Startup order][startup] [Graph producer][graph]

## 2. Exact production trace

### 2.1 The user-selected workspace remains A

`recurseWorkspace` returns the explicit global workspace or the current directory. The CLI passes it as `RecurseCycleConfig.Workspace`; the chat route passes `m.workspace` into the same driver. The ordinary CLI first needs its actual Cortex/kernel/executor setup to succeed. The witness assumes those prerequisites, rather than claiming that every launch reaches the driver. The `--plan` and `--dry-run` path returns separately and does not enter this mutating preparation route. [CLI selection][cli-root] [CLI preconditions and dispatch][cli-run] [Chat dispatch][chat-run]

`RunRecurseCycles` canonicalizes that filesystem path, loads its project document, constructs `recurseGit{root: root}`, and acquires its local recurse lock. Filesystem canonicalization resolves aliases; it does not define or clear a subprocess's Git selection environment. The lock resides under the selected root's `.nerd/recurse/lock`. [Driver setup][startup] [Canonicalization][canonical] [Lock][lock]

The driver then reads that workspace's journal and settles any in-flight record before `git.prepare`. With no pending record, settlement immediately returns. Git preparation precedes `r.passes`, whose later pass function derives the workspace graph and detects gates. [Startup order][startup] [No-intent recovery][settle] [Pass acquisition][pass]

### 2.2 The Git helper inherits a second target

`recurseGit.run` uses `exec.CommandContext(ctx, "git", args...)`, assigns `cmd.Dir = g.root`, and configures streams. It supplies neither `cmd.Env` nor an explicit admitted Git directory/worktree pair. Go specifies that a nil `Cmd.Env` inherits the current process environment. Thus assigning a working directory is not environment isolation. [Helper][git-run] [Go subprocess environment](https://pkg.go.dev/os/exec#Cmd)

Git documents `GIT_DIR` as overriding the default repository administration directory and `GIT_WORK_TREE` as selecting the working-tree root. The example below uses absolute paths so their interpretation does not depend on CodeNERD's changed working directory. Explicit selection is supported even when the initial current directory is elsewhere; the versioned Git setup implementation changes directory to the selected worktree for worktree-requiring commands. [Git repository environment](https://git-scm.com/docs/git#_environment_variables) [Git setup source](https://github.com/git/git/blob/v2.51.0/setup.c#L459-L481)

This is a real integration condition, not an assertion of common prevalence. Git's hook documentation warns that repository-selection variables exported to hooks must be cleared when invoking Git for a foreign repository or worktree. A wrapper, development shell or embedding process can similarly retain selection state. The witness does not require a hook, a recursive invocation from a committing hook, or an attacker. [Git hook environment](https://git-scm.com/docs/githooks#_description)

### 2.3 Preparation verifies existence but not correspondence

The first call is `rev-parse --show-toplevel`; its return value is discarded. A successful lookup therefore establishes only that the inherited Git context provides a top level, not that this is the repository containing A. The subsequent `status --porcelain --untracked-files=no` and branch lookup use the same helper/environment. They can consistently describe B. [Preparation][prepare]

If B's current branch differs from the desired branch, preparation either switches to B's existing local branch or creates the branch in B. No source-scope/returned-worktree comparison intervenes. Git documents that switching updates the selected working tree and index to the target branch, and `--create` creates and checks out a branch on success. [Preparation][prepare] [Git switch](https://git-scm.com/docs/git-switch#_description)

The environment is not merely used for a preliminary check: `untracked`, `changed`, `commit`, `revert` and `keptCycles` use the same runner. Repairing only the initial diagnostic would therefore leave later operations able to retarget. Their further outcomes depend on path and execution details; they are not needed to establish the minimal foreign startup mutation. [Census and adoption][git-rest] [Reversal and recovery lookup][git-recovery]

## 3. Conditional witness: A owns the run; B receives branch preparation

Use two separate ordinary non-bare Git repositories at canonical absolute paths A and B. Neither is a child directory, linked worktree or alias of the other. Their `.git` paths are ordinary directories. Both are accessible to the process. A has valid normal runtime/project inputs and no pending recurse intent. Let the CLI boot/admission prerequisites succeed. Use ordinary recurse, not mission mode, with its default `nerd/recurse` branch.

The process environment already contains these two selectors:

- `GIT_DIR` names B's absolute `.git` directory.
- `GIT_WORK_TREE` names B's absolute root.

No other Git selection override is needed. B has a committed HEAD on `main`, a clean tracked working tree, no existing `nerd/recurse` branch, and no hook, lock, permission, ownership or checkout conflict that makes the stated Git operations fail. Use ordinary ignored runtime state so incidental logs do not complicate the scenario. These are witness conditions, not observed configuration or instructions to execute the example.

| Boundary | Source-derived result |
|---|---|
| User selects A for ordinary recurse | The CLI supplies A as `Workspace`. |
| Driver constructs the run | Its filesystem root, project document and lock belong to A. |
| No in-flight record exists under A | Settlement returns without another operation. |
| Preparation invokes `rev-parse --show-toplevel` | Inherited absolute selectors identify B; the returned top level is discarded. |
| Preparation checks tracked status and current branch | It observes clean B on `main`. |
| B's local recurse branch does not exist | Preparation invokes `switch --quiet --create nerd/recurse` in the same Git context. |
| Git succeeds | B has a new local recurse branch and its HEAD selects it. A's repository has not been selected by this helper. |

The decisive mismatch is **A-scoped admission versus B-targeted host mutation**. It is already present before the shared driver's first graph/gate pass or `cfg.Execute` call. A later graph error, policy stop, cancellation or failed repair cannot make that prior branch mutation an A-targeted operation. The report does not claim that all boot activity is inference-free; it establishes that no selected repair execution or passing repair verdict is required for this boundary.

An optional extension uses an already-existing B `nerd/recurse` branch with different committed content. If B is clean and the switch is otherwise permitted, the existing-branch path can also update B's index and tracked files to that branch. This is normal checkout behavior applied to the wrong repository, not proof of arbitrary overwrite or irreversible data loss. The primary finding needs only branch creation.

### Controls and limits

- With the selectors absent and ordinary repository configuration, the usual working-directory discovery is a positive control.
- With B already on the desired branch, preparation returns without this branch mutation. It still has not proved that its selected repository corresponds to A.
- Dirty B, invalid selectors, an unavailable Git executable, inaccessible paths or a refused switch can stop the operation. A refusal for the wrong repository is still a routing problem, but is not the successful-mutation witness.
- A dirty A and clean B would demonstrate that the check describes the wrong tree; it is not needed and is excluded from the primary clean fixture.
- `--plan`/`--dry-run` does not enter this startup path. Do not describe a plan-only request as exhibiting the mutation.
- The run must actually reach `RunRecurseCycles`; missing runtime services and earlier boot failures remain real blockers.
- No remote push, permission bypass by the model, false kept commit, credential exposure, concurrent-owner collision or permanent data loss is established.
- Mission preparation can add independent preconditions before Git preparation. The ordinary witness avoids asserting that every mission reaches the same boundary.

## 4. Nonduplication boundary

The bounded screen inspected the current audit inventory, searched the available recurse audit corpus and current public code for repository-selection/environment terms, and read the nearest reports. This is not an exhaustive claim about historical branches, issues or private discussions.

| Existing report | What it already establishes | Distinct residual here |
|---|---|---|
| [Worktree ownership aliasing][prior-worktree] | The top-level result is discarded; distinct selected subdirectories can admit two live owners to one worktree. | One owner of A instead targets an unrelated B through ambient selectors, before an attempt. A worktree lock resolved using that same wrong environment could consistently lock B and still miss the authority mismatch. |
| [Index adoption identity][prior-index] | A verified working tree and committed index need a coherent adoption contract. Its witness explicitly excludes alternate repository selection. | The defect precedes candidate construction and affects startup branch selection; no foreign staged data is required. |
| [Literal path adoption][prior-literal] | Path bytes must not become Git pathspec programs. Its witness excludes alternate repository selection. | Ordinary names suffice. Literalizing every pathspec does not choose the right repository. |
| [Checkout project-policy continuity][prior-policy] | Branch selection within the intended checkout can leave a retained policy from its earlier branch. | Policy and Git operations refer to different repositories from the outset. Reloading A's document cannot bind B's branch operation to A. |
| [Workspace membership failure/census][prior-membership] | Membership read failures and freshness must remain explicit; the report credits sanitized Git selection as a protection. | The mutation helper does not use that protection, so the two consumers can disagree despite a correct membership census. |

The existing Git tests exercise ordinary preparation, dirty-checkout refusal, changed-path adoption/reversal, a subdirectory workspace and exclusion of foreign staged entries. They were read, not run. Those fixtures remain useful controls; the inspected cases do not create a second repository and install conflicting selection variables after fixture setup. This is a bounded statement about the read tests, not a claim that no related test exists anywhere. [Preparation fixture][tests-prepare] [Subdirectory/index fixtures][tests-scope]

## 5. Required contract and correction choices

Update the owning campaign/workspace specification before implementation. Keep source scope, Git worktree identity, administration/index identity and live ownership distinct but explicitly related.

1. **Admit the target before any Git effect, including recovery.** Derive the Git context from the selected filesystem workspace under an explicit environment policy. Verify that the actual worktree contains that workspace and identify the correct per-worktree administration state. A successful `rev-parse` exit is not the correspondence proof.
2. **Choose an explicit ambient-selection policy.** The ordinary workspace-selected API can either ignore inherited repository selectors in its own Git children, consistently with membership, or reject conflicting selectors with a clear target-mismatch diagnostic. If alternate administration is a supported feature, admit it as an explicit bound context rather than silently treating arbitrary inherited values as authority.
3. **Freeze that admitted context for every lifecycle operation.** Recovery history, clean checks, branch selection, path census, staging, commit, reversal and commit lookup must use the same repository/worktree/index contract. Do not rediscover a target independently under mutable ambient process state.
4. **Preserve subdirectory scope.** `show-toplevel == selected workspace` is too strict: the repository deliberately supports subdirectory workspaces. Prove containment and shared identity without widening tool paths or source policy to the whole repository.
5. **Compose with worktree ownership.** The separately proposed worktree lease should reserve the admitted worktree, not whichever foreign worktree inherited variables happen to select. Correct locking cannot turn the wrong destination into the right one.
6. **Keep environment changes child-local.** Do not globally unset process variables in a shared CLI/TUI as a repair. Other operations may have their own legitimate environment. Avoid stripping all `GIT_*` indiscriminately: user identity, behavior and supported deployment configuration need deliberate compatibility rules.
7. **Keep uncertainty and user work intact.** If target identity cannot be established, stop before mutation and retain pending recovery evidence. Do not switch a foreign repository back automatically as generic compensation: subsequent user activity can make that a second unowned effect.

A narrow correction can first contain the two-selector witness with a deliberate child environment and correspondence check. It should not be advertised as complete Git-environment hardening. Alternate index/object/common directories, environment-supplied Git configuration, separate Git directories and linked worktrees can also influence identity; specify supported cases and fail conservatively when they cannot be bound. The membership helper is evidence of an existing local convention, not proof its current drop list covers every Git configuration channel.

No additional model call, embedding search or deliberation round is intrinsically required. Host code should establish repository identity and expose typed admission/conflict observations; policy retains the executive decision where required by the architecture.

## 6. Costs and repair failure modes

- **Startup work:** child-environment filtering is proportional to environment entries; binding identity adds Git discovery/canonicalization and possibly filesystem checks. Measure process launches and cold startup latency rather than assuming the cost is negligible.
- **Compatibility:** a front-end may intentionally use a separate Git directory, alternate index or linked worktree. An unexplained blanket rejection can break legitimate use. A permissive fallback silently restores the defect.
- **Platform identity:** POSIX and Windows differ in path aliases and environment-key comparison. Qualify uppercase, case variants where meaningful, spaces/non-ASCII paths, symlinks and supported short-name aliases.
- **Scope:** equating source root with top level rejects supported subdirectory workflows; promoting all source operations to the discovered top level broadens their scope.
- **Timing:** checking only after `prepare` is too late. Checking once with sanitized discovery but executing mutations with inherited selectors is also insufficient. Recovery currently precedes preparation and needs the same bound context.
- **Coordination:** changing process-global environment creates cross-request races; locking the wrong selected repository makes the mismatch orderly rather than correct.
- **Diagnostics:** explain the conflicting target without dumping the whole environment or exposing unrelated secrets. Preserve enough identity to inspect an unresolved run without claiming its old intents belong to a newly selected repository.

No prevalence, speedup, token saving, reliability percentage or production incident is claimed.

## 7. Fair evaluation plan, entirely unrun

Start with isolated temporary A/B repositories and real Git, fixed valid driver inputs, and a deterministic executor that records entry. Create the fixtures under a controlled environment, then install the conflicting variables only for the process under examination. Otherwise the fixture's own Git setup can be redirected before the audit boundary and invalidate the comparison. The current `gitFixture` also leaves its environment inherited, making that separation particularly important. [Fixture setup][tests-fixture]

Compare the pinned baseline with the selected correction using identical repository trees, branch topology, permissions, runtime inputs and environment. Establish each repository's independent refs/HEAD/index/file identities before and after with an independent observer that does not inherit the conflicting selectors. An observer using the same misbound helper could incorrectly confirm the wrong repository twice.

| Case | Required observation |
|---|---|
| Clean A/B, selectors name B, B target branch absent | B has no new branch or HEAD change; the run either consistently uses admitted A or refuses before any Git effect. |
| B target branch exists with different content | B's HEAD/index/files remain untouched by the A-scoped request. |
| No selectors; ordinary valid A | Normal preparation and a genuine successful repair still progress. |
| Selectors consistently name admitted A | Declared compatibility behavior, with no accidental duplicate/refused owner. |
| Dirty A versus clean B, and clean A versus dirty B | Admission describes the intended workspace, not whichever repository is easier to admit. |
| A is not a repository; B is valid | B cannot make A's missing repository requirement silently pass. |
| Supported A subdirectory | Containing worktree is correctly bound while narrower source scope remains intact. |
| Real linked worktree, gitfile/separate Git directory | Correct per-worktree identity, or a precise unsupported outcome; no fallback to a foreign target. |
| Unknown/invalid selectors, repository discovery or canonicalization failure | Explicit pre-mutation refusal, no swallowed correspondence error. |
| Pending ordinary recovery or unsettled mission intent | Same target-binding contract applies before history/reversal; preserve existing mission reconciliation restrictions. |
| `--plan`/`--dry-run` | Remains outside mutating preparation. |
| CLI and chat ordinary routes | Same guarantee at the shared production boundary after each route's real startup preconditions. |
| Windows/POSIX, aliases, environment-key spelling | Honest supported behavior; no case-sensitive sanitizer assumption transplanted blindly. |
| Environment changes between lookup and effect in an embedding process | Bound child context remains stable, or operation stops before retargeting. |

Measure foreign-ref/HEAD/file mutations, wrong-tree cleanliness decisions, target disagreements between membership and lifecycle operations, false refusals, valid subdirectory regressions, and unresolved recovery cases separately. Record Git process count, environment/identity processing, latency and any retries. Keep normal successful-work controls: a repair that merely refuses every run is not success.

Then qualify normal CLI/chat entry paths with their real Cortex, CodeDOM/world-model context, campaign acceptance and ratchet. A deterministic startup test establishes the host target boundary, not end-to-end model quality or all Git semantics. Any later model study should hold model/version, task population, budgets and permissions comparable and account for new refusal/retry work.

**Closure criterion:** an A-scoped recurse invocation cannot mutate or recover against an unrelated B solely because of inherited Git selection state. Every relevant Git operation uses one admitted target relationship, while legitimate subdirectory scope and ordinary progress remain supported.

## Pinned references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/agents.md#L5-L35
[cli-contract]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/cmd/nerd/cmd_campaign_recurse.go#L51-L77
[cli-root]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/cmd/nerd/cmd_campaign_recurse.go#L120-L126
[cli-run]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/cmd/nerd/cmd_campaign_recurse.go#L151-L260
[chat-run]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/cmd/nerd/chat/campaign_recurse.go#L157-L228
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_cycle.go#L356-L450
[canonical]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/tools/workspace_guard.go#L58-L89
[lock]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_control.go#L16-L54
[settle]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_cycle.go#L1783-L1838
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_cycle.go#L489-L551
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_workspace.go#L60-L100
[git-contract]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git.go#L16-L25
[git-run]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git.go#L35-L76
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git.go#L50-L76
[git-rest]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git.go#L79-L146
[git-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git.go#L183-L244
[membership-env]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/workspace/git.go#L13-L60
[tests-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git_test.go#L13-L36
[tests-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git_test.go#L60-L78
[tests-scope]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/internal/campaign/recurse_git_test.go#L154-L201
[prior-worktree]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/Docs/audits/recurse-worktree-ownership-aliasing.md
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/Docs/audits/recurse-index-adoption-identity.md
[prior-literal]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/Docs/audits/recurse-git-literal-path-adoption.md
[prior-policy]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/Docs/audits/recurse-checkout-project-policy-continuity.md
[prior-membership]: https://github.com/theRebelliousNerd/codenerd/blob/90353b5c3effbaa45e7647834fe442527e6acb2f/Docs/audits/workspace-membership-failure-and-incremental-census.md
