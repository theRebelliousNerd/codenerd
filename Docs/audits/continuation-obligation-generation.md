---
doc-class: deep-dive
subsystem: cli
implementation-status: proposed-refinement
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Continuation test obligations need a source generation

## Decision

Preserve bounded, nonrepeating continuation, but distinguish an already-attempted obligation from a **new test obligation raised after another step changes the code**.

The current continuation rules discharge every test obligation sharing a description once any tester result exists for that description. A later coder step can therefore raise fresh test debt that an earlier tester result immediately suppresses. The outstanding work remains represented by a `pending_test` fact, but no executable subtask derives from it.

This is narrower than claiming that failed or unverified steps should retry indefinitely. The existing one-round policy is intentional. The proposed change is to make its identity precise enough to distinguish repeated work from newly created work.

**Evidence limit:** source inspection only. No codeNERD execution, tests, inference or implementation were performed. The sequence below is a source-derived validation witness, not a reproduced incident. Preparing or publishing this documentation does not change that evidence status.

## Source scope and reachable entry

The discharge rules are identical on [main](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/codedom_continuation.mg#L16-L52) and [feature](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/codedom_continuation.mg#L16-L52). The observed-return adapter, critic/test producer and continuation vocabulary tests are also byte-identical. Feature removes the root-description truncation used on main; that does not repair reuse of an unchanged short description across different writes.

The verified TUI entry is **approved dream-plan execution**:

- [`handleSubmit`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_handlers.go#L211-L219) recognizes an execution trigger when a pending plan exists.
- [`handleDreamPlanExecution`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_handlers.go#L695-L778) approves and starts the plan, converts its subtasks, and directly calls `executeSubtask`. [Main has the same producer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L673-L757).
- [`executeSubtask`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_continuation.go#L61-L152) obtains an observed return, injects its result and obligations, then queries the continuation rules.

Do not generalize this to every ordinary delegated mutation. The [normal mutation-delegation path](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L520-L591) requires verification and returns before the later `checkContinuation` calls. Feature's background query lane is another distinct execution path.

## How the identity loses the new obligation

[`injectShardResultFacts`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_continuation.go#L168-L245) creates separate execution and obligation IDs, but the discharge join ignores those IDs. It records:

- `shard_result(executionID, status, shard, description, summary)`;
- `pending_test(obligationID, "Write tests for: " + root)`;
- `pending_fix(obligationID, "Fix the findings reported in: " + root)`.

`obligationRoot` strips both prefixes. The policy then considers a test description discharged whenever any `/tester` result has that description. It checks neither observation generation, changed paths, snapshot, temporal order nor result status.

### Conditional production witness

Assume an approved plan starts a coder task with short description **R**, automatic/yolo continuation is enabled, and no interruption or clarification intervenes. In the following witness, tester-first selection among initially available obligations is an explicit condition, not guaranteed ordering.

1. The coder returns with an untested production-Go write and a structured critic finding. The host raises test obligation T1, described as `Write tests for: R`, and a fix obligation described as `Fix the findings reported in: R`.
2. The tester is selected first and executes T1. Its result is retained under that exact description.
3. The coder then executes the fix, changing an existing production file in a different package that remains testless, and returns normally with that path in `Untested`. The host raises T2, again described as `Write tests for: R`.
4. The earlier tester result still satisfies `test_obligation_discharged("Write tests for: R")`. T2 has a new ID and new causal evidence, but no tester subtask derives from it.

A same-package new helper is not a sufficient witness: the current detector treats any existing `_test.go` in the package as sufficient to suppress `Untested`. Nor does a tester writing only tests guarantee a critic finding; the critic excludes test files. The above assumes an initial critic finding and tester-first selection, without assuming that a tester may repair arbitrary source. [Untested detection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/test_verify.go#L189-L244), [critic input exclusion](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/critic.go#L318-L376), [work mode](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_mode.go#L43-L52).

**Established consequence:** newly measured test debt can be suppressed within this continuation chain. This does not establish that the earlier test failed, that the later code is defective, or that the final turn receives `/done`.

## Existing safeguards and intentional behavior

Several facts materially limit the finding.

**The one-round fix policy is deliberate.** [`TestFindingsOweAFixToTheCoderOnce`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/shard_result_vocabulary_test.go#L95-L149) explicitly expects a second finding to raise another fact without dispatching another fix round. Calling that behavior an accidental missing test would be incorrect.

**Failure is not automatically success.** [`executeSubtask`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_continuation.go#L87-L152) stops on a returned error; its terminal summary derives from the last return. `/failed` and `/unverified` are intentionally terminal status vocabulary. A failed/unverified matching result alone is therefore insufficient to establish a new defect.

**The coder still runs real gates.** Test debt is [measured by the test driver](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/build_verify.go#L334-L373), and [turn verification](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/coder_safety.mg#L659-L702) normally withholds the write-verification arm while untested or uncovered debt remains. The chain may stop honestly marked unverified. Lost follow-up is not proof of a false completion verdict.

**Ordinary new turns clear these facts.** [`clearStaleKernelFacts`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_helpers.go#L13-L20) retracts `shard_result`, `pending_test` and `pending_fix`. This report does not claim routine cross-user-turn reuse.

## Prior work and the narrower contribution

The [Wave 1 review](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/impl/REVIEW-wave1.md#L165-L183) records F8 as fixed by [097d59ac](https://github.com/theRebelliousNerd/codenerd/commit/097d59acda74f1caa6c1fa8139c5685a2b1ede42). That repair removed status-vocabulary mismatches and repeated tester dispatch until a ten-step ceiling. It intentionally introduced description-based discharge. Its anti-repeat purpose should survive.

The current test suite covers initial debt, successful tester discharge, one fix round, normalized descriptions and one retry for a hollow step. It does not cover **successful tester result → later coder write → newly raised test debt**.

The inspected current feature/research audit inventories and journey/implementation records contain related contracts, but not that specific generation join:

- [Background continuation ownership](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/background-outcome-and-continuation-ownership.md): successor ownership after asynchronous completion.
- [Planned-work conservation](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/planned-work-conservation.md): an edit planner's discarded overflow.
- [Test evidence capability](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/test-evidence-capability.md): package-qualified failure attribution.

This is a refinement of an existing continuation design, not discovery of obligation ledgers or the original F8 defect.

## Proposed contract and costs

Use an immutable chain/root identity, obligation kind, causal observation and relevant source generation. A returned attempt should settle the obligation it actually received; its description remains presentation text, not sufficient identity.

Separate:

- an attempt being terminal;
- its obligation being satisfied;
- policy deciding not to retry;
- a later observation creating new debt.

The same failed attempt may remain stopped without automatic retry. Newly created debt should receive an explicit pending, held, superseded or stopped disposition—not inherit satisfaction from an older source state. Missing generation evidence must not silently mean “same generation.”

This is an application of established incremental-computation reasoning. *Build Systems à la Carte*, §4.2.2, records dependency values or hashes and checks them before reusing prior work. The relevant inference here is that matching a task label is insufficient when its evidence-bearing inputs changed; adopting a build system is unnecessary. [Primary paper](https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf).

Prefer reusing existing observed snapshots and write observations over another workspace scanner. A conservative chain generation is simpler but may retest unrelated changes; dependency-scoped generations reduce excess work but require reliable scope accounting. Record metadata rather than duplicating raw findings into large facts.

Measure additional gate/model calls, source-measurement cost, retained fact size and time to resolve new debt. No performance benefit or cost has been measured. Preserve approval, cancellation and repeated-failure controls; a fresh obligation is not authorization for unlimited execution.

## Validation before implementation acceptance

Use the real continuation corpus and production TUI command path with deterministic executor returns.

| Case | Required outcome |
|---|---|
| Initial untested write | One tester obligation derives. |
| Successful tester, unchanged source | No repeated tester dispatch. |
| Initial finding and test debt; tester first → coder changes a testless package | New debt cannot be discharged by the earlier tester result. |
| Coder changes code but supplies adequate current tests | No unnecessary new test obligation. |
| Failed or unverified attempt | Honest terminal disposition; no mandatory infinite retry. |
| Repeated unchanged finding | Existing bounded-fix behavior remains explicit. |
| Missing snapshot or observation identity | No accidental equivalence with old evidence. |
| Stop or required approval | New debt remains retained without unauthorized dispatch. |
| Normal verified delegation and background query | No claim that these bypass paths exercised continuation. |

Existing tests should be retained and extended, not rewritten to hide the intentional one-round behavior. All proposed tests remain **unrun**.
