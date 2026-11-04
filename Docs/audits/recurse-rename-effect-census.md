---
doc-class: deep-dive
subsystem: campaign-recurse-adoption
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 5ae093ee2982d19251545cf6b2c0dc252c476f62
verification-method: static-source-inspection
main-rechecked: 2026-11-04T19:18:27Z
supersedes: []
---

# Preserve rename source deletions in recurse adoption

## Finding and decision

**Recurse can verify a file move but commit only its destination.** Its changed-path collector invokes `git diff --name-only --relative -z HEAD -- .`. When Git represents an owned deletion and staged addition as a rename, name-only output contains the destination, not both affected paths. The collector loses the source deletion. The later path-limited commit correctly obeys the smaller supplied set, so the source can remain in the committed tree even though it was absent during verification. [Collector][collector] [Git name projection][git-name] [Scoped commit][commit]

The result can be an incomplete kept commit and a still-pending deletion in the checkout. For ordinary Go files in the same package, retaining both copies of a moved function can make the committed tree fail to compile while the checked working tree passed. This is a **conditional adoption-fidelity defect**, not an observed incident or a claim that every rename is lost. The witness requires destination staging before collection, enabled rename detection, a real keep judgment and successful Git operations. It needs no foreign file content, pathspec wildcard, verification side effect or filesystem edit after verification.

The bounded correction is to preserve every changed endpoint in the authoritative effect census independently of Git's rename presentation. Disabling rename detection for this machine-consumed name list is one narrow option; parsing typed rename records and retaining both endpoints is another. Preserve exact ownership checks, literal path selection, the scoped commit containment and the existing ratchet. Do not restore whole-index commits to recover the missing deletion.

**Evidence boundary:** public main was pinned to [5ae093ee2982d19251545cf6b2c0dc252c476f62][pin]. Root and scoped guidance, campaign architecture, the complete current audit inventory, relevant reports, implementation and authored tests were inspected. CodeNERD source copies were checked against the pin's Git blob identities. Git's upstream implementation was separately inspected at [c44beea485f0f2feaf460e2ac87fdd5608d63cf0][git-pin], the resolved `v2.51.0` commit. No repository program, Git behavioral reproduction, build, test, gate, model call, inference, training or benchmark was run. No implementation, credential, setting or permission was changed. All proposed fixture outcomes below are source-derived expectations, not execution results. Evaluations remain unrun.

## Architectural role and current status

Root guidance makes the model responsible for creative work and the deterministic executive responsible for planning, evidence and safety. The campaign mission contract preserves normal write guards, acceptance, regression checks and owned compensation. It also explicitly distinguishes measured progress from strategic mission satisfaction. The adoption adapter must carry the host-observed effect set faithfully; another proposer or reviewer turn cannot repair a deletion that the host excludes from the eventual commit. [Root guidance][root] [Campaign guidance][campaign-guidance] [Mission contract][mission-spec]

The architecture guidance requires current behavior, intended contracts and gaps to remain distinct. This report follows the live CLI/chat adapters and shared driver instead of treating historical campaign descriptions as proof of current behavior. [Architecture guidance][architecture] [Campaign corpus][campaign-readme]

| Boundary | Status at the pin | Relevant limit |
|---|---|---|
| Normal recurse CLI and chat execution | Wired in source | Both call the shared cycle driver and return campaign kept-write receipts. [CLI][cli] [Chat][chat] |
| Startup ownership | Implemented | One recurse lock and a clean tracked checkout at preparation; this is not exclusion of every independent Git client for the entire run. [Admission][admission] [Preparation][prepare] |
| Per-path mutation observations | Implemented | `FileWrite` retains known before/after file states, including existence and content. A deletion and creation can be represented separately. [Observation shape][writes] |
| Changed-path collection | Implemented, incomplete for detected renames | Name-only output drops the source endpoint; the untracked pass cannot recover a deleted source. [Collector][collector] |
| Mission checks | Implemented over the collected set | Returned receipts are indexed, but protection and attribution checks iterate `changed`. [Checks][checks] |
| Scoped Git adoption | Implemented | It stages and commits only the supplied paths, preserving unrelated staged entries. An omitted owned deletion is also excluded. [Commit][commit] [Containment fixture][index-test] |
| Outcome publication | Implemented | Successful helper return supplies the kept commit hash; later notification uses the same collected names. No committed-tree equivalence check appears in this path. [Keep][keep] [Settlement][settlement] |
| Rename-invariant effect census and its acceptance fixtures | Proposed | No repair or passing execution is claimed by this report. |

The existing scoped commit is a valuable repair. The new requirement is completeness of its input, not permission to adopt every staged change.

## Git contracts that make the boundary observable

Git's name-only format projects a changed file pair to a post-image pathname. Its upstream `diff.c` implementation selects `p->two->path` for that format. NUL termination preserves pathname bytes; it does not add the discarded source endpoint. A rename-aware name/status representation carries more information than this projection. [Git diff manual][git-diff] [Pinned name-only implementation][git-name]

Path arguments select partial-commit behavior. Git builds a temporary candidate from the previous tree and updates only matching paths, while preserving other staged changes in the real index. Thus an accurate path-limited commit cannot infer that an omitted source deletion belongs with a supplied destination. [Git commit manual][git-commit] [Path selection][git-select] [Candidate construction][git-partial]

Rename detection is a comparison transformation over deletion/addition candidates, not an independent mutation or an authorization decision. Git provides `--no-renames` to disable that transformation for a command even when configuration enables it. These contracts establish the source trace; they do not establish the installed Git version or configuration on any user's machine. [Git rename design][git-diffcore] [Rename option][git-diff]

The finding is therefore about the semantic completeness of a machine-consumed list. Correct delimiter handling and a successful command exit are insufficient if the selected output format has already removed a required endpoint.

## Source trace through the live adoption path

### The collector loses a tracked deletion

Before execution, the driver captures the initial nonignored untracked names and writes the in-flight record. After the executor returns and cancellation handling is complete, `changedByAttempt` invokes `recurseGit.changed`. The collector reads name-only differences against HEAD, adds new untracked names, deduplicates and sorts. Its wrapper removes only the recurse ledger prefix. [Attempt baseline][attempt-start] [Collection call][measurement] [Collector][collector] [Wrapper][wrapper]

For a detected rename from A to B, Git's name projection supplies B. A is absent from the working tree, so it cannot appear in the subsequent `ls-files --others` result. When B is staged it is not an untracked file either. Neither phase adds A, and no receipt reconciliation augments the resulting set before its use by adoption.

This is not the separate preexisting-untracked exclusion: A was tracked in HEAD, and B did not exist before the attempt. It is also not a parser splitting a two-path rename record incorrectly. The chosen format emits only one pathname for the pair.

### Verification observes the working tree while protection consumes the smaller set

The driver remeasures node and required workspace gates after collection. It uses `len(changed)` to establish that something changed and `r.forbidden(changed)` to check forbidden paths. Mission mode maps returned writes by path, then checks ownership, known pre/post states, current postimages, protected inputs and scope only for entries in `changed`. Optional error-free measurements derive their changed Go package list from this same set. [Measurement and checks][measurement] [Mission checks][checks] [Package projection][packages]

A complete receipt for deleting A does not automatically repair the census: there is no union of all retained receipt endpoints into `changed`. In the primary witness both A and B are authorized and inside the same node scope, so this observation is not used to claim a permission bypass. The discrepancy remains even when every actual filesystem effect was properly admitted and recorded.

The Mangle ratchet still requires real success, no regressions, and no refusal. A move that fails the configured checks cannot reach the keep branch merely because the source name is missing. Mission mode also obtains a workspace snapshot and records check evidence before Git adoption. Those controls establish facts about the checked workspace; they do not add a missing path to the commit call. [Ratchet][ratchet] [Mission snapshot][mission-snapshot]

### Partial adoption preserves the wrong prior-tree entry

On `/keep`, the driver passes `changed` directly to `recurseGit.commit`. The helper stages those names with NUL-framed pathspec input and invokes `git commit --quiet --file=- -- <paths>`. In the witness the supplied paths are B and the independently repaired file C, not A. Git's partial candidate starts from HEAD, where A exists, and updates B and C. Therefore the resulting commit contains both A and B. [Keep][keep] [Helper][commit] [Git candidate construction][git-partial] [HEAD base index][git-base]

After successful helper return, the driver records the hash, increments its kept count, publishes the journal/mission outcome, removes the in-flight record and passes the same name set to `OnKept`. There is no intervening assertion that the committed tree contains A's verified deletion. With the stated fully staged move, A's deletion remains in the real index and working tree, so the still-open checkout can look functionally repaired even while the recorded commit is incomplete. [Settlement][settlement] [Git partial-commit contract][git-commit]

## Concrete witness with no changes after verification

Use a clean Git repository on the already selected recurse branch, with a stable HEAD, ordinary index, readable regular files, successful membership admission and no relevant exclusions. Use plain ASCII filenames with no pathspec metacharacters, no symlinks, no case-only spelling changes, no sparse checkout and no hooks that alter or reject the commit. The following is a proposed fixture, not an executed reproduction.

A supported Go package contains:

```text
go.mod
pkg/answer.go
pkg/old.go
pkg/answer_test.go
```

The ordinary production sources are:

```go
// pkg/answer.go
package pkg

func Answer() int { return 0 }
```

```go
// pkg/old.go
package pkg

func helper() int { return 42 }
```

An existing, unchanged test requires `Answer()` to return 42. The initial test genuinely fails. The authorized attempt changes `Answer` to return `helper()` and relocates the unchanged helper from `pkg/old.go` to `pkg/new.go`. This is a proposed admitted repair with an accompanying in-scope source move, not a prediction that a model will choose the move or an assertion that the move is required to fix the return value. It needs no filename-sensitive test or weakened oracle.

Let A be `pkg/old.go`, B be `pkg/new.go`, and C be `pkg/answer.go`. All three paths are inside the task's existing `pkg` write scope, neither moved endpoint is a pinned test or protected oracle, and the executor returns accurate separate deletion, creation and modification receipts. The existing attempt constructor's write set derives from node paths; this witness requires no scope expansion. [Attempt scope][scope] [Receipt shape][writes]

After startup admission, but before outer collection, an independent owner Git client stages the completed A-to-B move. This actor changes the index only. No model shell or arbitrary Git tool is assumed. The source's existing helper fixture already treats a newly staged file during an attempt as a supported state. The staging step is a witness precondition, not an instruction for this audit to perform it. [Existing staged-file fixture][git-test]

Use rename detection that recognizes the single identical, nonempty A/B pair. Stipulate no competing identical deleted/added files, rename-disable option or configuration, cancellation, Git error or filesystem change after the repair. C may remain unstaged until the helper handles it. All configured inner acceptance and outer ratchet requirements must genuinely allow the repaired working tree.

| Stage | HEAD | Real index | Working tree | Host consequence |
|---|---|---|---|---|
| Initial admission | A exists, B absent, C returns 0 | Same as HEAD | Same as HEAD | Clean tracked checkout accepted; unchanged test fails. |
| Owned repair and move, then owner staging | Unchanged | A absent, B contains A's bytes; C may remain old | A absent, B contains helper, C calls helper | Full effect is deletion A, creation B, modification C. |
| Rename-aware name-only census | Unchanged | Unchanged | Unchanged | `changed` contains B and C, not A. |
| Normal checks and ratchet | Unchanged | Unchanged | Only B supplies helper; `Answer()` returns 42 | Genuine `/keep` is required. No post-check mutation occurs. |
| Scoped helper commits B and C | **A and B both define helper; C calls it** | A absent, B and repaired C present | Same repaired layout | B/C are committed, A's deletion remains pending. |
| Settlement succeeds | Same incomplete commit | Same pending deletion | Same checked layout | Kept receipt names a commit that did not retain the entire repair. |

The duplicate `helper` definition is a source-level consequence in a checkout containing both ordinary `.go` files of the same package; Go forbids duplicate declarations in that package scope. [Go declaration rules][go-scope] A fresh checkout of the resulting commit therefore has a concrete compilation problem absent from the verified working tree. That final compilation was not run here; it is the proposed fixture's independent publication oracle. A simpler byte/tree comparison already establishes incomplete adoption without requiring Go compilation.

The witness also does not require a malicious actor, a foreign staged version, or a race between the last check and commit. Owner staging happens before collection and can remain stable throughout all measurements.

### Controls that delimit the claim

- **Leave B untracked until outer collection.** Git sees A as a deletion and the extra untracked pass supplies B. This does not establish the lost-source witness.
- **Disable rename detection for the census.** The same staged move is represented as deletion plus addition, so both paths can enter the list. The proposed fix must not depend on the owner's ambient setting.
- **Use sufficiently different deleted/added content that no rename is detected.** Both names may survive. This is why the present edit/delete/create fixture does not cover this case.
- **Fail acceptance, a ratchet condition or Git commit.** Do not claim a successful kept commit. Preserve current failure and ambiguity behavior.
- **Start with tracked dirt already present.** Preparation refuses it. The relevant staging happens after that admission.
- **An external write or commit changes the candidate afterward.** That introduces a separate freshness/ownership case; it is unnecessary for this finding and must not be hidden inside the test.

## Failure and recovery boundaries

The primary failure is retaining the wrong tree, not losing the source bytes: A's previous contents remain in Git, B is recorded, and the pending deletion may still be visible. The owner can inspect and reconcile it. Automatic repair must not silently amend a published commit or sweep unrelated staged work merely to make the checkout clean.

The stale deletion can be encountered by later work or rejected as tracked dirt on a subsequent invocation. This report does not predict an exact later-loop outcome, eventual self-repair or infinite retry. Later behavior depends on the selected work, callbacks and other writers. The already-recorded local incomplete commit is sufficient to establish the contract failure.

Current cancellation and refuse/revert handling use `undoOwned`, which starts from the complete returned write observations and version checks. It is not the old broad Git-revert cleanup that blindly restores every workspace difference. Credit that repair: the missing name in `changed` does not by itself prove that owned cancellation restoration loses A. The old `revert` helper and its tests should not be mistaken for the live cleanup path. [Cancellation][cancellation] [Refusal branch][refusal] [Owned cleanup][undo]

Once adoption starts, the driver intentionally preserves ambiguous intent rather than undoing a possibly successful commit after an acknowledgement failure. Keep that behavior. A future tree-equivalence check that fails after a commit must distinguish a known mismatching commit from an unknown publication outcome; it must not rerun a destructive operation blindly. [Adoption ambiguity][keep] [Mission durability contract][mission-durability]

A move across two directories can additionally omit the source directory from `goPackagesOf(changed)` and from the later `OnKept` path list. That is a source-supported consequence of the common input set, but it is not needed for the same-package commit witness and does not establish which downstream cache is stale or which optional check would have failed. [Package projection][packages] [Notification][settlement]

## Proposed endpoint completeness contract

### Required invariants

1. **Preserve every net changed resource.** For a tracked deletion plus creation represented as a rename, both paths must reach effect reconciliation. The authoritative changed set must not vary merely because Git enables rename presentation for the same HEAD/worktree state.
2. **Keep observation separate from authority.** A path's appearance in a Git diff is not an ownership receipt or permission. Join each endpoint to its admitted scope, exact before/after state and owner; preserve foreign or uncertain changes under existing hold rules.
3. **Retain operation semantics where needed.** A rename changes both endpoints. A copy does not delete its source. Do not indiscriminately authorize or stage the source of every multi-path display record.
4. **Bind checks and adoption to one candidate.** The accepted candidate must include the verified deletion and creation, with the same parent, bytes and relevant metadata. A successful path-limited Git call establishes only that Git accepted its supplied selection.
5. **Publish an honest result.** A kept receipt must not imply full adoption if the retained tree differs from the candidate. Keep reconciliation information and report a hold/uncertain outcome rather than deriving success from the process exit alone.
6. **Preserve bounded recovery.** Unknown attribution, path ambiguity, source drift, index conflict or partial publication must not trigger whole-index adoption, unconditional restoration, scope expansion or blind retry.

### Narrow producer option

Add a command-local no-rename requirement to the machine-consumed changed-name query while keeping NUL framing and workspace-relative selection. A detected move is then represented by its deletion and addition, which is the needed effect census. User-facing diff and history views may still detect renames; they serve a different purpose. This is a proposed implementation direction, not a command executed in this review.

This option does not require learning a rename relationship or asking another model to infer one. It also does not close preexisting-untracked exclusions, verification effects after collection, literal-pathspec problems, concurrent edits, hooks or parent movement. Keep those separate obligations visible. A narrow change can close this witness without claiming snapshot-atomic publication.

### Typed-record option

Use a documented machine format such as NUL-terminated name/status or raw records and parse record arity and status explicitly. Preserve both endpoints for rename records, normalize ordinary add/delete/modify/type-change records to the corresponding affected paths, and state the distinct treatment of copies and unmerged entries. Reject truncated or unsupported records instead of returning a plausible partial list.

The parser's semantic test must establish endpoint completeness, not only successful NUL splitting. Keep endpoint provenance for diagnostics if useful, then derive the admitted union. An exact-content move and a similarity-detected move may receive different display confidence, but neither may erase the source deletion from the adoption set.

### Stronger candidate publication

The existing index-adoption report already proposes an exact candidate tree and admitted parent, with isolation from unrelated index entries. Reuse that design rather than presenting it as new here. Endpoint-complete acquisition is a prerequisite to that candidate, and explicit committed-tree comparison provides a separate safeguard against later adapter mistakes. Preserve the current foreign-index containment test and user staging; never fall back to pathless commit as a convenience. [Prior candidate-tree proposal][prior-index]

Host code should measure path endpoints, ownership evidence, candidate identity and Git outcomes. Mangle should retain authority over admission, refusal, hold and completion. No additional prompt or model-discretion exception is needed for the narrow fix.

## Unrun evaluation and full costs

### Existing test evidence

`TestRecurseGit_ChangedKeepCommitRevert` covers an edit, deletion, staged creation, ignored output, preexisting untracked data and trailers. Its deleted file contains `here\n`, while the staged new file contains `new\n`; it does not construct the identical-content rename pair. The kept branch subsequently edits/adds files without moving a tracked file. The subdirectory fixture and foreign-index fixture are valuable controls for workspace containment and scoped commit behavior. They were read, not run, and their existence is not a passing execution receipt for this finding. [Git fixture][git-test] [Subdirectory fixture][subdir-test] [Foreign-index fixture][index-test]

### Required acceptance fixtures

1. **Real Git census and adoption.** In an isolated fixture, create and stage an exact-content move after clean preparation. Call the actual collector and helper. Require both endpoints in the authoritative set; inspect the resulting commit tree directly and prove A absent/B correct. Also prove the fixture's Git comparison really classified the pair as a rename in the positive control.
2. **Metamorphic rename setting control.** Hold HEAD, index, worktree and receipts constant. Vary command-local rename detection and relevant supported configuration. The authoritative effect set and retained tree must be equivalent, even if a user-facing diff changes presentation.
3. **Normal production route.** Use a scripted worker through the ordinary recurse campaign executor, with a real kernel, write observations, host gates, mission variant, journal and commit helper. Model the owner staging actor at a synchronization boundary before collection. Require a genuine failing unchanged test before the repair and genuine success after it. Do not hardcode the keep verdict or grant the model Git shell access for the test.
4. **Independent committed-result check.** Verify the created tree or a fresh isolated checkout. The unchanged Go tests and compilation must pass there, not only in the original workspace that still carries the pending deletion.
5. **Ownership and index controls.** Include a staged foreign change, unknown/mismatching receipt, protected endpoint and out-of-scope endpoint. Preserve the foreign staged bytes, forbid unowned adoption, and keep refusal/compensation behavior. Both paths being observed must not expand authority.
6. **Record and pathname cases.** Exercise modification, pure addition/deletion, exact and edited rename, copy detection, multiple competing identical files, empty-file moves, Unicode, whitespace/newline names, leading dashes, literal metacharacters, nested workspaces and supported case behavior. Keep unsupported filesystem/Git states explicit.
7. **Failure and interruption.** Exercise diff failure, malformed typed output if that alternative is chosen, staging failure, commit rejection, cancellation and acknowledgement loss. Check actual HEAD/index/worktree state before retrying; keep durable ambiguous intent when necessary.

Report helper tests, integration tests, normal-entry-path validation and committed-tree checks separately. A scripted move establishes the host boundary; it does not establish that an unconstrained model reliably proposes appropriate renames in real projects.

### Cost model and fair comparison

Let C be the number of affected path records, P their total pathname bytes, R the number of detected renames and U the untracked-name bytes acquired. The current host already starts a diff process and an untracked-list process, captures their output, constructs a set and sorts it. Collection memory includes output buffers plus the set/list, roughly proportional to P+U, with sorting proportional to the number of unique names times its logarithm. Git's repository/index scanning and diff work are additional, not represented by that host-only estimate. [Collector][collector] [Runner buffers][runner]

An endpoint-complete name list adds up to R source names relative to a destination-only rename projection. Their byte lengths, deduplication and sorting must be counted. Command-local `--no-renames` may avoid rename matching work, but a speedup is not established here. Typed records add status/endpoint parsing and may retain rename-matching cost. Git documents potentially expensive rename/copy matching; real cost depends on candidates, configuration and repository shape. Do not compare a rename-disabled complete census against a rename-enabled incomplete one using runtime alone. [Git rename work limits][git-diff]

Stronger candidate-tree construction or post-commit comparison adds object/index work, subprocesses, temporary storage, failure cleanup and possibly isolated-checkout cost. If a repaired cross-directory census schedules source-package checks previously omitted, charge their full startup, compilation, execution, output and snapshot costs. More checking is not a regression merely because the old path silently skipped required work.

For task-level evaluation, count worker and deliberation calls, input/output/cached tokens, retries, gate time, Git time, compensation/reconciliation work, total wall time and human intervention per correctly retained change. Include cold and warm repositories, rename-heavy refactors and ordinary edits with equal permissions and checks. Report semantic success first: exact retained effect set, preserved owner index, correct durable status and committed-tree validity. No quantitative token, latency, throughput or monetary benefit is claimed.

## Relationship to prior audits

The current audit tree, queue and targeted searches for rename and name-only behavior were screened, with the relevant full reports read. This is a bounded nonduplication check, not a claim about every historical branch or private discussion.

- **Index adoption identity** concerns foreign staged content entering an older pathless commit. Current source has the scoped commit containment. Here that containment excludes a required owned deletion because the producer omitted its name. The broader exact-candidate contract is shared prior work. [Index audit][prior-index]
- **Untracked owned effect adoption** concerns an owned modification excluded because its filename existed in the initial untracked set. Here the deleted source was tracked, the destination is newly staged, and Git's rename projection removes the endpoint. Repairing the untracked-baseline rule alone does not repair this witness. [Untracked audit][prior-untracked]
- **Post-verification effects** concerns changes created after the census by outer verification commands. Here the whole move and staging precede the census, and checks are nonmutating. Moving the same lossy census later still omits A. [Verification-effects audit][prior-effects]
- **Literal Git path adoption** concerns selected names becoming patterns. These filenames are plain and selection can be perfectly literal; the missing source was never supplied. Preserve literal-selection hardening as an independent requirement. [Literal-path audit][prior-literal]
- **Compensation ownership** concerns undoing unrelated writes and establishes receipt-based compensation. The primary witness fully owns both endpoints and concerns successful keep publication; current live cleanup receives the complete receipts. [Compensation audit][prior-compensation]
- **Orientation history rename identity** concerns historical lineage aliases across commits. This report concerns a present effect census and partial commit; no historical alias chain is involved. [Orientation audit][prior-orientation]
- **Git system-fact porcelain transport** concerns removal of the first status column by whitespace trimming. This report preserves every byte returned but chooses a projection that omits a needed endpoint. [Porcelain audit][prior-porcelain]

**Closure criterion:** under the supported Git/workspace contract, staging an authorized move or changing rename presentation cannot cause recurse to omit its source deletion from effect reconciliation or claim full adoption of a commit that retains that source. The verified and retained candidate must agree without weakening scope, ownership, test or index-preservation guarantees.

## Source references

[root]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/agents.md#L3-L59
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/agents.md#L1-L15
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/architecture/AGENTS.md#L13-L68
[campaign-readme]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/architecture/campaign/README.md#L1-L38
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L112
[mission-durability]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L185-L214
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/cmd/nerd/cmd_campaign_recurse.go#L225-L264
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/cmd/nerd/chat/campaign_recurse.go#L218-L265
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L356-L430
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git.go#L50-L76
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git.go#L35-L47
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git.go#L79-L123
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git.go#L125-L147
[writes]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/observation/subagent.go#L192-L207
[checks]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1352-L1398
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1481-L1507
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1536-L1571
[attempt-start]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1239-L1252
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1324-L1356
[wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1660-L1675
[packages]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_error_free.go#L100-L124
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/core/defaults/policy/recurse.mg#L260-L329
[mission-snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1446-L1458
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_attempt.go#L125-L143
[git-test]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git_test.go#L83-L152
[subdir-test]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git_test.go#L154-L176
[index-test]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_git_test.go#L178-L201
[cancellation]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1285-L1321
[refusal]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1508-L1532
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/internal/campaign/recurse_cycle.go#L1886-L1920
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/recurse-index-adoption-identity.md
[prior-untracked]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/recurse-untracked-owned-effect-adoption.md
[prior-effects]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/recurse-post-verification-effects.md
[prior-literal]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/recurse-git-literal-path-adoption.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/recurse-compensation-write-ownership.md
[prior-orientation]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/orientation-history-rename-identity.md
[prior-porcelain]: https://github.com/theRebelliousNerd/codenerd/blob/5ae093ee2982d19251545cf6b2c0dc252c476f62/Docs/audits/git-sysfacts-porcelain.md
[pin]: https://github.com/theRebelliousNerd/codenerd/commit/5ae093ee2982d19251545cf6b2c0dc252c476f62
[git-pin]: https://github.com/git/git/commit/c44beea485f0f2feaf460e2ac87fdd5608d63cf0
[git-name]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/diff.c#L6314-L6320
[git-select]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/commit.c#L251-L289
[git-base]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/commit.c#L311-L340
[git-partial]: https://github.com/git/git/blob/c44beea485f0f2feaf460e2ac87fdd5608d63cf0/builtin/commit.c#L496-L553
[git-diff]: https://git-scm.com/docs/git-diff
[git-commit]: https://git-scm.com/docs/git-commit
[git-diffcore]: https://git-scm.com/docs/gitdiffcore#_diffcore_rename_for_detecting_renames_and_copies
[go-scope]: https://go.dev/ref/spec#Declarations_and_scope
