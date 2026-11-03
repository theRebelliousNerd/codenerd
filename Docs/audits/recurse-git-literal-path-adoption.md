---
doc-class: deep-dive
subsystem: campaign-recurse-git-adoption
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-03
verified-against: b3608de7580b6c33dd584f6e6fd6c24a33d42ff7
verification-method: static-source-inspection
supersedes: []
---

# Recurse must preserve literal filenames through Git adoption

## Finding and decision

The current recurse keep helper receives an exact list of changed filenames, but passes those names into Git's pathspec language without literal-pathspec mode. NUL-separated input to `git add` preserves record boundaries; it does not disable pathspec patterns. The later, correctly path-limited `git commit -- <paths>` interprets the same names as patterns again. A filename containing brackets can consequently select both itself and a different matching file. [Commit helper][commit] [Git parser][git-parser] [Git matching][git-match]

A particularly useful conditional witness needs **no concurrent writer, foreign staged change, malformed path, or model permission failure**. A preexisting, nonignored untracked owner file is allowed at startup and deliberately excluded from the changed-path census. If it matches the pattern formed by an owned changed filename, the add operation can stage it, and the path-limited commit can include it. The changed-path collector and the ratchet can be correct about their supplied set while the final Git operation selects more than that set. [Startup][prepare] [Census][changed] [Keep consumer][keep]

**Recommended contract:** names observed as files stay literal names at every exact-file Git boundary. Keep NUL-safe transport and path-limited publication, but make selection semantics explicit and deterministic. This is a narrow host-adapter correction, not a new planner, a reason to reject ordinary bracket-containing repositories, or a replacement for snapshot-bound adoption.

All CodeNERD source links use public main [`b3608de7580b6c33dd584f6e6fd6c24a33d42ff7`][main], observed at 22:37 UTC on November 3, 2026. Git implementation references use the official `v2.51.0` tag's commit [`c44beea485f0f2feaf460e2ac87fdd5608d63cf0`][git-pin]. That Git pin establishes the inspected parser and matching contract; it is not a claim about an installed toolchain. No application, repository command, test, build, gate, model inference, benchmark or failure injection was run. This is a conditional static-source finding and an unrun evaluation proposal.

## Why this matters to portable, Socratic recurse

The project's guidance gives the model creative work and the deterministic host/kernel the obligation to preserve executive contracts. Mission mode can deliberate over an admitted work item, evaluate objections, select a design, check source freshness and return exact observed writes. Those protections are valuable, but they cannot make a filename remain literal after a downstream adapter recasts it as a pattern. More deliberation or another model vote would add cost without correcting this boundary. [Root guidance][guidance] [Mission specification][mission-spec]

The behavioral success criterion is stronger than “a commit command returned zero.” The adopted change must correspond to the admitted effect set. Bracket syntax is also a meaningful portability case, not only a theoretical wildcard filename. The primary witness below uses brackets rather than `*` or `?`, avoiding those characters' Windows filename restriction. Upstream Git's own fixture constructs a bracket filename and separately takes special measures for its `*` filename on Windows. The test definitions were read, not executed here. [Upstream fixture][git-fixture]

The existing [portable deliberation/crucible proposal][portable] already owns the broader design and evidence contract. This note supplies one deterministic adoption obligation for that workflow: a sound design and a successful check must not silently acquire unrelated files while being recorded.

## Current production trace

Both CLI and chat invoke the common `RunRecurseCycles` driver. Their executor callbacks construct `RecurseAttemptCampaign`, run the campaign, and return its kept-write observations. The defect is in the shared host keep helper after the callback, not an imagined raw Git command issued by the model. [CLI adapter][cli] [Chat adapter][chat]

The relevant sequence is:

1. Git preparation rejects tracked dirt with `status --porcelain --untracked-files=no`. Preexisting untracked files are intentionally allowed. An authored fixture explicitly checks this behavior. [Preparation][prepare] [Admission fixture][admission-test]
2. Before an attempt executes, the driver obtains the set of nonignored untracked names and saves it with the in-flight record. [Attempt start][start]
3. After execution, `changedByAttempt` calls the Git collector. The collector combines tracked working-tree differences from HEAD with only those untracked names absent from the earlier set. The wrapper removes the loop's ledger paths. [Collector][changed] [Wrapper][changed-filter]
4. The driver measures gates and asks Mangle for the ratchet verdict. In mission mode it checks ownership, known before/after states, the current postimage, protected inputs and scope **for the paths in that collected set**. [Mission checks][mission-checks] [Ratchet policy][ratchet]
5. A real `ratchetKeep` passes that same set to `recurseGit.commit`. The helper supplies the names to `git add --all --pathspec-from-file=- --pathspec-file-nul`, then to `git commit --quiet --file=- -- <paths>`. Its runner sets no literal-pathspec argument or explicit environment policy. [Keep][keep] [Helper][commit] [Runner][runner]
6. On success, the driver records the returned hash and increments the kept count. The later `OnKept` callback still receives the original collected set, not a separately checked manifest of committed paths. [Publication][keep] [Notification handoff][on-kept]

This trace does not assume the ratchet admits every attempted edit. The primary witness requires a legitimate accepted edit and successful Git operations. Existing test, permission, mission and cancellation controls remain prerequisites.

## NUL framing is not literal selection

Git's manual distinguishes file-based pathspec input, its NUL-record option and the global literal-pathspec option. The wording about taking other characters literally in NUL input describes transport and quoting, so it should not be used alone to infer that pattern matching is disabled. [Git add manual](https://git-scm.com/docs/git-add#Documentation/git-add.txt---pathspec-from-fileltfilegt) [Git global options](https://git-scm.com/docs/git#Documentation/git.txt---literal-pathspecs)

The inspected implementation removes that ambiguity:

- `parse_pathspec_file` chooses a NUL or line reader and skips C-style unquoting for NUL records. It then passes every collected string into ordinary `parse_pathspec`. [File parser][git-parser]
- The add caller does not set `PATHSPEC_LITERAL_PATH`; the commit caller also uses ordinary parsing. [Add caller][git-add-parser] [Commit caller][git-commit-parser]
- Pathspec initialization treats a name as literal only under a literal flag/magic setting; otherwise it computes where wildcard syntax starts. [Initialization][git-init]
- The matcher accepts an exact spelling and can also accept a different name through wildcard matching. An exact match for one file does not disable matching for other files. [Matching][git-match]
- Git's own bracket-path fixture expects both the bracket-named file and its plain-name counterpart to match; its literal-mode control expects only the bracket file. Those are authored upstream assertions, not results obtained during this audit. [Bracket fixture][git-fixture]

The distinction is therefore **argument safety versus application-language semantics**. Direct `exec.CommandContext`, NUL records and `--` are good protections. None alone turns a pathspec into an exact filename. Adding shell quotes inside the transmitted string would introduce wrong filename bytes rather than repair this contract.

## Minimal conditional witness

Use a clean, admitted Git workspace with no in-flight recovery, no alternate index or repository selection, and the ordinary default pathspec environment. To avoid any branch-switch complication, start on the intended recurse branch. No ignored-file or sparse-checkout complication is required.

Within an otherwise supported workspace, let:

- A be the tracked, regular file `pkg/f[o][o].txt`, whose initial working-tree/index bytes equal HEAD.
- B be a nonignored, preexisting untracked regular file `pkg/foo.txt`, owned by the user and not part of the attempt.
- The executor make a legitimate, authorized change only to A.
- The actual configured checks and ratchet admit that change, with no cancellation, Git lock error, hook refusal or intervening edit.

The text suffix makes the filename relation easy to see; it is not a claim that a text-only workspace automatically has runnable checks. A production-path fixture must supply a real supported project and a declared check that exercises the chosen work. The naming relation is the same for bracket-containing source filenames.

| Boundary | A | B | Consequence from inspected source |
|---|---|---|---|
| Initial preparation | Clean tracked file | Already untracked | B does not violate the tracked-clean admission. |
| Attempt baseline | Original A | B's name enters the untracked baseline | The collector can distinguish B from a newly created attempt file. |
| After the accepted edit | Changed A | Unchanged B, still untracked | The collected changed set is exactly `[A]`. |
| Add supplied “A” | Exact match | Also matches pattern `f[o][o].txt` | Both can enter the index. |
| Commit supplied “A” | Selected | Now known to Git and also matches the pattern | The resulting local commit can contain A's edit and B's addition. |
| Kept record | Hash recorded | No separate B admission was made | Cycle attribution attaches to a commit wider than its supplied effect set. |

For the two Git operations' selection, this follows from their common parser and matcher. The prior add makes B known to Git before path-limited commit constructs its partial candidate. [Add file selection][git-add-select] [Index and HEAD selection][git-index-union] [Partial commit construction][git-partial]

The resulting violation is **extra local adoption of an untouched owner file**. It is not necessarily a failed test, corrupted bytes, exfiltration, remote push, permanent data loss or arbitrary command execution. B may already have been included in a workspace measurement as ordinary filesystem context; that does not authorize adding it to the cycle's commit. No malicious author or racing process is needed.

### Controls and limits

- With ordinary non-pattern A and an unrelated B, the current path-limited commit excludes B. Preserve the landed correction.
- If no counterpart exists, this exact extra-file witness is absent.
- If B is ignored and stays unselected by the add command, or the environment already enforces literal paths, this witness does not establish an extra commit.
- If the attempt is refused or reverted, it never reaches this keep path.
- If B was newly created after the attempt baseline, it may enter `changed`; mission ownership checks can then refuse it. The primary witness deliberately makes B older than that baseline.
- A hook that refuses the extra candidate may prevent the commit, but it is not a guarantee supplied by this helper.
- Mission source freshness and write receipts can all be correct for A. The expansion occurs afterward. This does not establish that every mission reaches execution or that the model bypassed its write guard.
- The mission spec's acknowledged arbitrary-editor race is not needed. All bytes can remain stable through validation and adoption. [Mission boundary][mission-boundary]

## Nonduplication and existing protections

The bounded screen read the current audit inventory and queue, campaign guidance and architecture, current recurse graph/gate/mission code, and related audit bodies. Repository search for `pathspec` returned the older index-adoption audit but no dedicated literal-pathspec adoption report; `literal-pathspecs` returned no repository result. Search is supporting discovery, not exhaustive historical proof.

The existing [recurse index-adoption audit][prior-index] concerns a **pathless** commit taking unrelated staged entries. Its general exact-candidate-tree recommendation remains relevant. Current source has added explicit path arguments to commit, and a dedicated fixture checks preservation of a plain-named foreign staged file. The queue describes that original witness as stale-closed in source. This report credits that repair and isolates a different remaining mechanism: supplied names themselves denote larger sets. The new primary witness starts with B untracked, with no foreign staged state and no concurrent writer. [Current helper][commit] [Foreign-index fixture][index-test] [Current queue][queue]

The [compensation ownership audit][prior-compensation] already establishes that “different from HEAD” does not prove authorship. Here the collector correctly excludes B; broadening occurs only in the consumer. Normal live refusal/cancellation uses `undoOwned`, not the older raw Git revert helper. Do not revive a claim that all current live reverts perform broad Git checkout. [Owned compensation][undo]

The broader repository already recognizes this distinction at a different boundary: the model-facing parsed Git-add step refuses globs and magic rather than attributing them to a plain-file list. That parser is not invoked by the recurse host helper and does not repair it. Host-held exact filenames should normally remain supported through literal encoding, rather than inherit a user-shell parser's rejection of valid names. [Separate shell parser][shell-parser]

Existing recurse fixtures cover clean admission, prior untracked-file preservation, ordinary keep/revert, subdirectory roots, and foreign staged entries. Their inspected names do not contain the matching bracket/plain pair. They are valuable positive controls, not evidence that exact naming survives pathspec interpretation. No fixture was run. [Git fixtures][tests]

## Proposed behavioral and implementation contract

1. **Exact names remain exact.** For a supplied nonempty set of observed file names, Git selection must not include another path solely because the name contains wildcard or magic syntax. Keep file identity separate from intentional query-pattern APIs.
2. **Apply the rule at both mutations.** Making only commit literal can still leave an unrelated file newly staged by add. Making only add literal can still leave commit vulnerable to matching an independently known counterpart. Do not consider the helper repaired until staging and commit both preserve the supplied set.
3. **Use supported Git literal semantics.** A deliberately configured global literal-pathspec invocation or correctly constructed literal pathspecs are candidate mechanisms. Preserve root-relative meaning, NUL-safe records and the existing `--` option separator. Do not use shell escaping as a substitute.
4. **Define ambient interaction.** The runner currently inherits the process environment. Account explicitly for Git's literal, glob, noglob and case-folding pathspec settings; conflicting global settings can fail rather than provide the desired semantics. Any normalization should be invocation-local, with unrelated settings and credential handling untouched. Git's parser already specifies these conflicts. [Global magic][git-global]
5. **Preserve independent work on failure.** If add succeeds and commit fails, do not clear the entire shared index. Preserve uncertain-adoption handling and distinguish staging-only, committed and unknown outcomes. This repair does not authorize destructive recovery.
6. **Check all exact-file consumers.** `attemptDiff` and the raw recovery `revert` helper also pass observed names as pathspecs. They should receive the same literal-selection review. That is a family-level follow-through, not an independently reproduced overwrite claim. Ordinary owned compensation must remain intact. [Diff and raw revert][other-consumers] [Recovery route][recovery]
7. **Retain stronger adoption obligations.** Literal selection does not bind content against later edits, protect against hook/filter changes, pin HEAD atomically, or replace the earlier exact-candidate-tree proposal. State the narrower guarantee accurately.
8. **Keep Mangle authoritative.** The ratchet still decides whether an attempt may be kept. Host code must implement the selected effect faithfully. No added model judge or extra deliberation round is required to decide whether an observed filename is a filename.

## Costs and failure modes

The minimal literal-selection repair adds no inherent model request, embedding, tool-generation step or additional Git subprocess. Its direct costs are invocation construction and regression coverage. Removing unintended pattern expansion may reduce unrelated index scanning, but no speedup is measured or promised.

Stronger pre-publication tree verification would add Git reads, hashing, candidate metadata and failure handling; do not bundle those costs invisibly into a comparison of literal-mode handling. A private index can help isolate publication but still needs correct literal selection when populated. It is not a substitute for the narrow repair.

Measure full accepted-work cost: Git process count and duration, files staged, candidate/committed paths and blobs, index bytes changed, cleanup/retry work, elapsed time and any model continuation indirectly caused by failures. A “fix” that rejects every bracket filename is not an acceptable portability win. Likewise, a policy that asks the owner to stage all untracked files merely transfers the burden and changes the ownership contract.

Explicit edge cases include literal colon-prefixed names on supporting platforms, brackets in directory segments, leading dashes, spaces/newlines, Unicode, file deletion, case handling, nested workspaces, ignored counterparts, sparse checkout, configured global pathspec flags, hook failures and interruption between staging and commit. Reject impossible or unauthorized inputs with an honest outcome; do not silently broaden the selected set.

## Fair evaluation plan, entirely unrun

Start with deterministic real-Git fixtures. Do not need a live model to establish this boundary.

| Fixture | Required observation |
|---|---|
| Primary bracket A, older untracked matching B | Collector returns only A; successful keep commits only A; B remains untracked with identical bytes. |
| Same pair, plain non-pattern A control | Existing ordinary behavior and owner-file preservation remain unchanged. |
| Matching B already tracked and independently staged, tested directly at helper boundary | Scoped helper must not acquire, unstage or overwrite B; retain the original foreign-index regression. |
| Bracket directory and ordinary leaf; nested selected workspace | Literal identity holds at every component and remains workspace-relative. |
| A added, modified or deleted | Correct add/delete semantics without selecting a counterpart; independently preserve B's index and bytes. |
| Spaces, newline, quotes, leading dash, Unicode; platform-valid colon or wildcard cases | Exact names round-trip. Unsupported filesystem names are explicit skips, not passes. |
| Ambient literal/glob/noglob/icase settings | Deterministic accepted semantics or explicit failure, never hidden expansion. |
| Add succeeds, commit hook rejects; cancellation or acknowledgement loss | No false keep; owner index remains independent; uncertain commit is reconciled before retry. |
| Ordinary live refusal/revert | Existing owned-write compensation behavior remains unchanged. |
| CLI and chat through the shared cycle driver | A legitimate stock-policy keep reaches the helper with expected paths and produces the exact local commit. |
| Mission mode | Preserved design/source/write/scope admission for A; no unrelated B enters adoption afterward. |

Capture the expected changed set, initial untracked set, actual index entries before/after, resulting commit's parent/tree/paths/blobs, original B bytes and tracking status, journal outcome and callback input. Do not rely only on Git exit status, a kept count, or a textual commit message. For setup and observation commands in the future fixture, use literal-safe operations too, so the oracle does not reproduce the bug under test.

Compare baseline and repaired code with the same repository, toolchain, hooks, environment, gate obligations and input bytes. Require both correctness and ordinary successful adoption. Then measure overhead separately. An authorized live-model study could later assess operational cost, but it is unnecessary to validate literal path selection and no such study was run here.

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/agents.md#L9-L28
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/Docs/audits/portable-recurse-deliberation-crucible.md
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git.go#L50-L76
[changed]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git.go#L79-L122
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git.go#L125-L146
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git.go#L35-L47
[start]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1239-L1250
[changed-filter]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1660-L1676
[mission-checks]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1352-L1391
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/core/defaults/policy/recurse.mg#L299-L329
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1481-L1507
[on-kept]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1548-L1571
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/cmd/nerd/cmd_campaign_recurse.go#L224-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/cmd/nerd/chat/campaign_recurse.go#L218-L265
[mission-boundary]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L198-L215
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/Docs/audits/recurse-index-adoption-identity.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/Docs/audits/recurse-compensation-write-ownership.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/Docs/audits/ASTRA_QUEUE.md#L354
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1889-L1922
[shell-parser]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/projectdoc/shell_git_step.go#L137-L160
[admission-test]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git_test.go#L60-L78
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git_test.go#L83-L175
[index-test]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git_test.go#L178-L201
[other-consumers]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_git.go#L149-L222
[recovery]: https://github.com/theRebelliousNerd/codenerd/blob/b3608de7580b6c33dd584f6e6fd6c24a33d42ff7/internal/campaign/recurse_cycle.go#L1798-L1837
[git-pin]: https://github.com/git/git/commit/c44beea485f0f2feaf460e2ac87fdd5608d63cf0
[git-parser]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/pathspec.c#L688-L721
[git-init]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/pathspec.c#L452-L533
[git-global]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/pathspec.c#L298-L323
[git-match]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/dir.c#L407-L426
[git-add-parser]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/add.c#L449-L467
[git-commit-parser]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/commit.c#L364-L384
[git-add-select]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/add.c#L493-L507
[git-partial]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/commit.c#L497-L559
[git-fixture]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/t/t6130-pathspec-noglob.sh#L7-L98


[git-index-union]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/commit.c#L251-L309
