---
doc-class: deep-dive
subsystem: campaign-recurse-adoption
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-03
verified-against: 49f9610dfc91b9241ab7474e0326431f9b7a1831
verification-method: static-source-inspection
supersedes: []
---

# Recurse must account for owned changes to preexisting untracked files

## Finding and decision

**A recurse attempt can be checked together with an owned source change that its kept commit omits.** The changed-path collector deliberately excludes filenames that were already untracked before execution. That exclusion still applies when the executor actually changes one of those files and returns an accurate owned-write receipt. The keep path uses the collector's smaller set, not a reconciled set of retained effects, for Git adoption. [Collector][collector] [Owned return and collection][execution] [Keep][keep]

A concrete conditional witness uses ordinary Go source and an unchanged tracked test. An authorized attempt changes a tracked function to call a helper, and changes a preexisting untracked helper to produce the required value. The checked working tree can pass. The kept commit contains the new call but not the helper file, so the same committed tree lacks that definition. This requires no concurrent writer, foreign staged change, pathspec syntax, weakened test, missing receipt, or unsupported-language adapter.

**Recommended contract:** reconcile all net retained owned effects with the proposed adoption set before judging a keep. A preexisting untracked filename is a baseline classification, not evidence that its contents stayed unchanged. Where adopting its prior content is not authorized, preserve the owner's work and hold or compensate the attempt rather than silently treating the smaller commit as the checked result.

This is a source-supported conditional integration finding, not an executed reproduction or a claim about how often models make this change. It refines the existing candidate-tree proposal with an **omitted owned effect** counterexample. It does not propose staging every untracked file.

## Evidence boundary and distinctness

All CodeNERD references use public main [49f9610dfc91b9241ab7474e0326431f9b7a1831][pin], observed and rechecked unchanged on November 3, 2026, including a recheck at approximately 23:12 UTC. Reviewed sources include root and campaign guidance, the mission architecture, shared driver, Git adapter, receipts, membership, evidence snapshots, write guards, production CLI/chat adapters, authored tests, the audit inventory and related reports.

No application, build, test, gate, model inference, benchmark or failure injection ran. No repository code, credentials, settings or permissions changed. Only scratch research copies and this report were written. Fixture outcomes below are deductions from the source and documented tool semantics, explicitly conditional on the stated admissions and successful operations.

The [audit inventory][queue] and inspected neighboring reports did not identify this exact owned-preexisting-untracked witness. This is a bounded novelty screen:

- [Compensation ownership][prior-compensation] established that differences from HEAD do not prove authorship and led to observed-write compensation. This report credits that repair. Here the executor **does** supply the relevant ownership; the adoption census nevertheless omits it.
- [Index adoption][prior-index] traced extra foreign staged content entering an older pathless commit. Current code uses a path-limited commit and has a corresponding preservation fixture. The present witness has no foreign index entry and loses a required owned file rather than adding an unrelated one.
- [Literal path adoption][prior-literal] concerns filenames being interpreted as patterns and acquiring an untouched owner file. All names here are plain exact paths; literal-pathspec mode would not add a name that never reaches the helper.
- [Portable deliberation/crucible][portable] already calls for versioned behavioral evidence and candidate identity. This report contributes a specific host boundary and production-route acceptance criterion; it does not claim those general ideas as new.

The broader exact-candidate recommendation is intentionally shared with prior work. The new result is a smaller, deterministic counterexample that survives the landed path-limited commit correction and accurate owned-write capture.

## 1. The implemented representations do not have the same domain

### Startup allows an untracked source file

Git preparation checks tracked dirt using `status --porcelain --untracked-files=no`; it does not reject an otherwise admitted untracked source file. The existing preparation test explicitly requires this behavior. [Preparation][prepare] [Admission fixture][prepare-test]

Workspace membership independently loads both cached and other files through `ls-files -z -co --exclude-standard`. A regular, nonignored helper source file can therefore be a legitimate workspace member even though it is not part of HEAD. This is not an ignored output or Git-administration exception. [Membership][membership] [Git file classifications][git-ls-files]

Before each attempt, the driver records the untracked **name set** and persists it in the in-flight record. It does not snapshot each member's content in this particular set. [Attempt baseline][baseline]

### Execution can own a write to that existing file

Both CLI and chat construct a normal `RecurseAttemptCampaign`, run its orchestrator and return `KeptWrites()`. That accessor returns successful attempts' recorded writes; it does not filter them by Git tracking status. [CLI][cli] [Chat][chat] [Owned return][owned-return]

The campaign's write guard invokes mission admission, then the write-set lease. Mission admission checks canonical scope, configured protection and the verification-input map. It does not declare every preexisting untracked regular source file immutable. The automatic input map pins existing Go tests, module/work files, `nerd.md` and `testdata`, not every ordinary Go implementation file. A helper inside the selected package can be admitted if ordinary permissions and the other controls permit it. [Write guard][guard] [Mission scope][scope] [Pinned inputs][oracle]

This is not a permission bypass: the witness expressly requires both writes to be authorized and accurately observed. If the owner or project forbids changing the helper, that is a refusal control and this witness does not proceed.

### The adoption census discards that owned write by name

The Git collector unions:

1. tracked working-tree changes against HEAD under the workspace; and
2. currently untracked names **absent** from the pre-attempt untracked set.

An existing untracked file modified in place is absent from the first set and excluded from the second. The wrapper removes ledger paths but adds no owned-receipt reconciliation. Thus a valid receipt does not make the file part of `changed`. [Collector][collector] [Wrapper][wrapper]

Mission mode records the complete returned write list to a sidecar. Its later attribution loop, however, iterates over `changed`: every selected path must have a known matching postimage and satisfy scope/protection. It does not perform the reverse check that every retained source effect in `owned` has an explicit adoption disposition. The subset can pass all those checks while the extra owned effect remains unexamined by that loop. [Receipt persistence][receipt-storage] [Post-attempt checks][attribution]

### Gates inspect the working tree, while Git records the smaller set

Nonempty `changed` triggers remeasurement against the actual workspace. The detected Go test gate runs the package with `-count=1 -cover`; the ordinary runner uses the configured workspace. Go compiles the package's applicable source files rather than requiring them to be tracked by Git. [Remeasurement][remeasure] [Go gates][go-gates] [Runner][runner] [Go command contract][go-test]

Mission snapshots likewise include admitted regular untracked files. Hashing the actual helper bytes is correct for a **working-tree** snapshot; that does not place the helper into a Git candidate tree. [Snapshot][snapshot]

The stock ratchet can derive keep when there is a selected change, the target is resolved, and no measured regression/refusal applies. A real keep passes only `changed` into the helper, which stages and commits those supplied paths. Git's path-limited commit semantics do not spontaneously include an omitted untracked helper. [Ratchet][ratchet] [Keep][keep] [Commit helper][commit] [Git commit contract][git-commit]

Successful settlement records the commit, retains the remeasured gate state, clears the in-flight record and passes the same smaller set to `OnKept`. There is no keep-path compensation of the omitted owned helper. [Keep][keep] [Settlement][settlement]

## 2. Minimal conditional witness

This is a **hypothetical fixture specification, not a fixture created or run by this review**.

Use a plain Go module and package `pkg`, with no special build tags, generated files, symlinks, ignore exclusions, custom loaders, unusual path characters or hooks. The committed tree contains:

```go
// pkg/answer.go, tracked
package pkg

func Answer() int { return 0 }
```

```go
// pkg/answer_test.go, tracked and unchanged throughout
package pkg

import "testing"

func TestAnswer(t *testing.T) {
    if got := Answer(); got != 42 {
        t.Fatalf("Answer() = %d, want 42", got)
    }
}
```

Before recurse starts, the workspace additionally contains this **nonignored, untracked** regular file:

```go
// pkg/helper.go, not present in HEAD
package pkg

func helper() int { return 41 }
```

The baseline tracked tree is self-contained: `Answer` does not reference `helper`. Both the committed tree alone and the current workspace can compile the package; the target test fails because `Answer()` returns zero. The extra helper is initially unused.

Assume:

- Startup, graph discovery, kernel boot, tool availability and storage succeed.
- The selected attempt is the ordinary `TestAnswer` finding on package `pkg`, not an error-free collapsed group.
- Both implementation writes are within its admitted scope, not protected, and permitted by the normal tools.
- If mission mode is used, its source/vision, design, usage and time admissions succeed.
- The attempt returns accurate known before/after receipts for both files, with no external writer or intervening source change.
- Neither file is staged by the worker; Git tracking status remains as specified until outer adoption.
- Required acceptance and ratchet checks actually admit the candidate; no custom gate or hook rejects its untracked dependency.
- Git and final ledger writes succeed.

The candidate changes exactly these bodies:

- tracked `Answer`: `return helper()`;
- untracked `helper`: `return 42`.

The tracked test is not edited. No test-count increase or weakened assertion is needed. The current workspace has both definitions; executing `TestAnswer` in that workspace exercises the new call and returns the requested value. The fixture is also compatible with nonregressing source-line/test counts and increasing rather than declining statement coverage. Actual gate results remain for the proposed evaluation to establish.

The source-derived trace is:

| Boundary | Expected state under the stated conditions |
|---|---|
| Startup | The clean tracked checkout is admitted; helper's untracked status is allowed. |
| Attempt baseline | `pkg/helper.go` enters `UntrackedBefore`. |
| Successful execution | Both files have changed; both writes are returned as owned. |
| Git census | `pkg/answer.go` is selected. `pkg/helper.go` is excluded solely because its name was already untracked. |
| Checks | Package checks read both current files. The unchanged test can pass in this workspace. |
| Mission checks, if enabled | The selected tracked path has an accurate receipt and is in scope. The owned helper receipt is retained, but the post-check loop does not require it to join the selected set. |
| Keep helper | Only `pkg/answer.go` is supplied to add and commit. |
| Published local tree | `Answer` calls `helper`, but no `pkg/helper.go` exists in that tree. |
| Successful settlement | The commit is recorded as kept while the modified helper remains untracked in the working tree. |

The precise failure is **a retained local commit that does not contain the source required by its checked implementation**. Building that tree without the untracked overlay would lack the `helper` definition. This review did not materialize or compile that tree and makes no observed error-message claim.

The example intentionally avoids a weaker baseline argument. The original committed implementation did not need the untracked helper; the candidate creates that dependency. The issue is not merely that a preexisting untracked workspace was already nonreproducible.

### Why additional ordinary checks need not expose it

Repeated checks in the unchanged owner workspace continue seeing the helper. End-of-pass remeasurement can therefore remain green. A pre-commit hook checking that same workspace is not necessarily a candidate-tree check. A source hash that includes the helper can truthfully identify the tested workspace while still failing to bind the commit to those bytes.

In mission mode, the gate snapshots can match the recorded before/after workspace identities and the policy can classify the kept, resolved, changed-snapshot observation as a supported gate deficit. That support is valid only for the measured workspace including the helper. It must not be treated as evidence that the commit alone recreates the supported state. The mission specification already limits strategic claims; this audit does not relabel the result as general mission satisfaction. [Witness comparison][witness] [Mission finish][mission-finish] [Evidence rule][mission-rule] [Mission specification][mission-spec]

## 3. Controls and limits that must remain explicit

- **Untouched preexisting untracked files must remain excluded from automatic adoption.** Existing tests intentionally protect them. The repair is not `git add --all` on the whole workspace.
- A helper newly created during the attempt is normally included by the collector. The distinguishing condition is that its name existed in the untracked baseline.
- If helper is already tracked, its modification enters the tracked diff. If the worker stages it before collection, it can also enter the diff against HEAD; the primary witness explicitly excludes that action.
- A candidate that remains self-contained in the tracked file does not exhibit the commit dependency failure.
- A candidate-tree check, a policy that prohibits preexisting untracked-source edits, or a relevant custom hook may block this candidate. None is a mandatory completeness check in the inspected shared path.
- Existing cancellation/refusal uses owned-write compensation, not the older raw Git revert helper. This audit does not reopen the old broad-rollback claim.
- If the **only** owned change is to a preexisting untracked file, `changed` can be empty. That is a related liveness case: outer checks are skipped and the stock ratchet reverts a no-selected-change attempt. The primary finding instead supplies a tracked change so the defective candidate can reach keep.
- No remote push, disclosure, irreversible loss, malicious intent, guaranteed model behavior or frequency estimate is established.
- Plain paths and regular source make the representation mismatch independent of shell quoting or OS filename oddities. Windows and POSIX behavior should both be evaluated; this review ran neither.
- Generated files, ignored inputs, submodules and foreign staged content have additional authority and reproducibility contracts. They are not required to establish this finding.

## 4. Proposed host/policy contract

Update the owning campaign/mission specification before implementation. Preserve Mangle as the executive and the model as the creative center; another model vote cannot supply a missing candidate member. [Root guidance][guidance] [Mission specification][mission-spec]

Use separate observations rather than one overloaded list:

1. **Baseline classification:** tracked state and preexisting untracked identities, with enough content/version evidence to distinguish untouched owner work from an admitted modification.
2. **Net owned effects:** canonical retained before/after versions from the observed execution ledger, accounting for repeated writes and writes restored to baseline.
3. **Proposed publication:** exact paths, operations, modes and content identities authorized for the candidate tree, bound to the expected parent/ref.
4. **Verification environment:** the source/input identities actually consumed by required checks, including explicit overlays that are not in the candidate.

Before keep, every material retained owned effect needs a recorded disposition: included in the authorized candidate, restored and followed by applicable remeasurement, or held with no successful adoption. A previously untracked name cannot silently erase that obligation.

Adding an owned file to a commit includes its **whole resulting content**, including material predating the attempt. Permission to change one part is not necessarily permission to adopt all preexisting owner content. Do not infer publication authority from a write receipt. If the owner has not authorized that adoption, preserve the file and request the narrow decision or require a different self-contained candidate.

A bounded first correction can fail closed when a retained owned source effect lies outside the collector set, explain the path and baseline status, and retain recovery evidence. It should not force a new live model call just to classify a deterministic set mismatch. For an authorized complete candidate, build the candidate tree explicitly and verify it in a context that does not borrow undeclared untracked workspace inputs.

The existing private-index/worktree proposals are implementation options, not automatic solutions. A private index seeded with an incomplete path set still omits the helper. A worktree that copies all user untracked files can reproduce the same misleading check. Candidate construction, input closure, authority and check materialization must agree.

Keep uncertain-commit acknowledgement handling: once adoption may have occurred, reconcile the actual commit before retrying or compensating. Do not use a completeness repair to reset foreign staging, delete owner files, weaken protected-input checks or invent an atomic transaction against arbitrary editors.

## 5. Cost and failure handling

No savings, latency improvement, success-rate increase or implementation estimate was measured.

Receipt-to-candidate reconciliation can be host-only and needs no added inference. Its cost includes canonicalization, collapsing repeated writes, checking net versions, hashing affected bytes, metadata persistence and conflict handling. A map-based comparison is linear in the represented paths after normalization; content checks additionally scale with bytes read. That is a design cost model, not a measured runtime bound.

Candidate materialization adds Git/tree work, disk use, checkout or overlay handling, build caches and possibly repeated gates. Reverification after restoring an excluded effect is real work, not a free bookkeeping step. Conservative refusal may increase owner questions and lost repair opportunities; automatically including files risks unauthorized adoption. Compare both costs honestly.

Required failure dispositions include unknown/missing receipts, preexisting file changed by another writer, repeated writes ending at the original version, receipt persistence failure, candidate construction failure, gate timeout, cancellation, insufficient authority and ambiguous commit acknowledgement. Unknown completeness must not become keep merely because a smaller path list is available.

Retain enough evidence to distinguish the measured workspace, intended candidate and actual commit after a restart. Full prior content can be sensitive even in an untracked file; retain only authorized necessary artifacts with existing access controls rather than indiscriminately logging source into prose.

## 6. Fair, unrun behavioral evaluation

Authored tests were read, not run. The existing Git fixture protects an **untouched** `owner.txt` while modifying other files; it does not modify that preexisting untracked file through an owned receipt. The foreign-index fixture verifies the landed path-limited commit containment. Both remain required controls. [Existing ownership fixture][git-test] [Foreign-index fixture][index-test]

Start with deterministic execution and the actual shared driver, registered write tools, observed receipts, normal acceptance engine, real kernel and real Git in disposable workspaces. Use the ordinary CLI/chat construction with world-model and CodeDOM context enabled for integration coverage. A collector-only test proves the name subtraction, not the complete kept-but-unreproducible outcome.

Required cases, all proposed and unrun:

| Case | Required observation |
|---|---|
| Primary tracked caller plus modified preexisting untracked helper | Complete authorized adoption or explicit hold; never kept with an omitted required helper. Inspect actual committed tree, not only workspace status. |
| Untouched unrelated preexisting untracked file | Preserve it and exclude it from automatic adoption; normal self-contained repair still succeeds. |
| Same helper created after the baseline | Preserve normal new-file inclusion and ownership checks. |
| Helper already tracked | Both legitimate modifications remain part of the candidate. |
| Preexisting helper is the sole owned change | No false “no change” classification of the receipt; disposition is explicit and policy-controlled. |
| Helper write restored to its original bytes before return | No false incomplete-effect hold based only on an intermediate receipt. |
| Permission to edit but not adopt preexisting content | Hold or rework; do not silently add the file. |
| Foreign postimage change or missing before/after evidence | Preserve owner state and report conflict; no forceful compensation or automatic staging. |
| Protected test/config input changed | Existing refusal survives; this repair does not broaden oracle authority. |
| Cancellation or failure before/after candidate construction | Correct owned compensation or durable uncertainty; no orphan successful record. |
| Same workspace passes, reconstructed candidate lacks an input | Candidate checks detect the mismatch; workspace-only success is insufficient. |
| Windows/POSIX, nested workspace, plain paths and paths with spaces | Equivalent membership, receipt, candidate and result identities. |
| Restart around adoption acknowledgement | Actual tree/parent are reconciled before retry; no duplicate or misleading kept record. |

For the primary fixture, retain the exact pre/post bytes of both implementation files, tracked test bytes, untracked baseline, returned receipts, selected path set, gate invocations/results, candidate tree and final commit tree. Compare the current and proposed behavior under the same candidate and admissions. Separately inspect the original committed baseline to verify it did not depend on the helper; otherwise the fixture would confound a preexisting missing-input problem with the new candidate.

If later authorized, a real-model evaluation should use frozen, repository-disjoint tasks across supported ecosystems. Include source, fixture and configuration overlays; equal permissions, information and acceptance obligations; and a baseline that is allowed to complete legitimate self-contained repairs. Report supported committed outcomes, holds, false holds, extra owner decisions, recovery work, wall time, host IO/storage, gate time and all model tokens/spend. More kept commits or more refusals alone is not success.

**Exit criterion:** the normal production route cannot publish a candidate whose successful behavioral witness silently relies on an omitted retained owned source effect, while preserving ordinary owner-untracked-file protection and successful authorized repairs.

## Source references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/49f9610dfc91b9241ab7474e0326431f9b7a1831
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/agents.md#L1-L66
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/Docs/audits/ASTRA_QUEUE.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/Docs/audits/recurse-compensation-write-ownership.md
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/Docs/audits/recurse-index-adoption-identity.md
[prior-literal]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/Docs/audits/recurse-git-literal-path-adoption.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/Docs/audits/portable-recurse-deliberation-crucible.md
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_git.go#L47-L77
[prepare-test]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_git_test.go#L60-L78
[membership]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/workspace/git.go#L99-L120
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1239-L1250
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/cmd/nerd/cmd_campaign_recurse.go#L225-L269
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/cmd/nerd/chat/campaign_recurse.go#L218-L265
[owned-return]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/orchestrator_attempt_writes.go#L112-L128
[guard]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/orchestrator_attempt_writes.go#L137-L154
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_mission_scope.go#L14-L91
[oracle]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/evidence/change.go#L379-L423
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_git.go#L79-L123
[wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1660-L1674
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1288-L1339
[receipt-storage]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_mission_storage.go#L64-L77
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1352-L1390
[remeasure]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1324-L1349
[go-gates]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/gates/detect.go#L123-L132
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/gates/run.go#L45-L99
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/evidence/change.go#L168-L246
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/core/defaults/policy/recurse.mg#L266-L329
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1481-L1507
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_git.go#L125-L149
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1548-L1571
[witness]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_cycle.go#L1420-L1457
[mission-finish]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_mission.go#L326-L374
[mission-rule]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/core/defaults/policy/mission.mg#L24-L31
[git-test]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_git_test.go#L81-L151
[index-test]: https://github.com/theRebelliousNerd/codenerd/blob/49f9610dfc91b9241ab7474e0326431f9b7a1831/internal/campaign/recurse_git_test.go#L178-L200
[git-ls-files]: https://git-scm.com/docs/git-ls-files
[git-commit]: https://git-scm.com/docs/git-commit
[go-test]: https://pkg.go.dev/cmd/go#hdr-Test_packages

