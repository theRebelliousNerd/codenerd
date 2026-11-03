# Bind campaign test-write completion to the task's evidence, not unrelated dirty tests

**Status:** source-supported conditional completion defect; regression/addendum to the checkpoint's F12 evidence-ownership invariant. Not a runtime reproduction or an implemented fix.

**Source freeze:** public `theRebelliousNerd/codenerd` main commit [`ccbe41302073b364ef2cb40ba49b9736738ae6e2`](https://github.com/theRebelliousNerd/codenerd/commit/ccbe41302073b364ef2cb40ba49b9736738ae6e2), inspected 2026-11-03. The introducing commit is one commit after [`0f60e7f901ead31f2dd7fba06f1696281939d65d`](https://github.com/theRebelliousNerd/codenerd/commit/0f60e7f901ead31f2dd7fba06f1696281939d65d).

**Method:** read-only public source, policy, tests, commit diff, and bounded audit-inventory review. No application execution, tests, model calls, policy evaluation, local Git operations, or repository modifications. All witnesses and checks below are proposed, not observed.

**Moving-head note:** main advanced during inspection to `74dca64c1f99aa1305038184587dbb33b96b61d7`. The [eight-file delta](https://github.com/theRebelliousNerd/codenerd/compare/ccbe41302073b364ef2cb40ba49b9736738ae6e2...74dca64c1f99aa1305038184587dbb33b96b61d7) changes working-loop verification counting and tool-call accounting, not the campaign checkpoint, campaign completion, task delegation, no-write verdict rules, or final-report-audit admission used here. This report's citations remain frozen to `ccbe413`; it does not claim execution was verified at either head.

## Finding

The new `test_write` micro-checkpoint fallback accepts any surviving dirty or untracked `*_test.go` file reported by the workspace's Git status. The file need not be in the task's declared write set, an artifact, an observed write of this attempt, or an earlier attempt belonging to the task. It can predate the campaign and belong to unrelated human work. The helper receives only the workspace, so it cannot establish those relationships. [Fallback and producer][checkpoint]

This changes a task-specific evidence boundary: when every declared and observed path is missing, a `test_write` task that wrote nothing can now pass the filesystem checkpoint because an unrelated test file is dirty. The normal task wrapper can then mark that task completed and record success, conditional on the worker returning an actual eligible `/done` result. [Transaction][transaction], [completion][completion]

The `/done` precondition does not make this checkpoint branch unreachable. Campaign `/test_write` delegates as session `/test`, while `/test` is deliberately not a write-oriented intent. A successful test-running turn with no mutation can satisfy the session's mechanical completion rules. In addition, the final-report audit skips a no-write, non-write-oriented turn. Those are sensible properties for ordinary test execution; the mismatch is treating them plus an unrelated dirty file as proof that a separate campaign test-writing obligation was fulfilled. [Task delegation][task-routing], [persona verb][tester-verb], [write-intent policy][write-policy], [turn verdict][verdict], [report-audit admission][report-audit]

This is not a claim that every model will omit the requested tests, that prose alone bypasses `/done`, or that a whole campaign necessarily finishes incorrectly. It is a source-supported loss of task ownership at a completion gate, with a reachable conditional upstream evidence shape. Phase checks, acceptance, or later work may still detect or repair the omission.

## Delta and duplication boundary

At parent revision `0f60e7f`, the checkpoint's final path was an error once no declared or attempt-written path survived. Its comment explicitly rejected another file elsewhere as evidence for a task that wrote nothing. `ccbe413` retains that comment and then adds the broader dirty-test fallback. The old same-basename issue is therefore a useful invariant and historical predecessor, not a claim that the original implementation remains unchanged. [Parent checkpoint][parent-checkpoint], [current checkpoint][checkpoint]

The bounded duplication screen read the current audit queue, the deletion and capability checkpoint audits, the checkpoint tests, and the C2/L4 journey; it also searched the public repository for the new helper and F12/checkpoint references. It did not exhaust every branch or historical discussion.

- **F12:** the existing checkpoint test says a task that wrote nothing must not pass because another `README.md` exists. Its fixture uses `/document`; it does not exercise `/test_write` or Git dirt. The new regression has a different admission condition, and does not require even the same basename. [F12 test][checkpoint-tests]
- **Campaign deletion microcheckpoint:** that audit concerns false rejection of an attempt-owned authorized removal when no path survives. This report concerns false acceptance based on a surviving file the attempt did not own. A removal witness would not repair this fallback. [Deletion audit][deletion-audit]
- **Campaign capability checkpoint continuity:** that audit concerns a resolved non-file capability being misread as a file obligation. This report retains a real file-writing obligation and demonstrates borrowing another task's file evidence. [Capability audit][capability-audit]
- **C2:** correcting a planner's guessed target using the attempt's own writes is already supported. The proposed remedy must retain that flexibility; exact declared-path matching alone is not the desired fix. [Owned-write control][checkpoint-tests], [C2/L4 history][journey]
- **TDD red-phase behavior:** accepting a genuinely written reproducer that fails against unfixed production code is a separate change in the same commit. The minimal witness here uses no write and a green ordinary test execution, so it does not rely on the new red-phase rule. Do not remove legitimate red-phase behavior to address this evidence-ownership issue.

## Conditional minimal witness

Use a small ordinary Git repository with the configured workspace at its Git root. Use simple ASCII paths to avoid porcelain quoting or nested-workspace interpretation questions.

1. Before campaign task T starts, `unrelated/cache_test.go` exists and is modified or untracked. Its contents are valid, and it belongs to unrelated work. It is not T's artifact, declared target, or previously retained output. A stable modification to an already tracked test is sufficient; no concurrent writer is needed.
2. The admitted plan contains T of `TaskTypeTestWrite`, no explicit shard override, and a nonempty exact write set `feature/new_behavior_test.go`. That path is absent and remains absent. There is no matching glob or other surviving declared target. A downstream task may depend on T.
3. Effect validation, workspace membership, context assembly, risk admission, kernel operations, permissions, and the write-set lease succeed. Missing output paths are not rejected merely for being absent: the target contract checks for a declared target, and the mutation snapshot records known absence. [Effect validation][effect], [snapshot][snapshot]
4. The normal observed executor handles the generated test-writing brief but performs an allowed, successful test run over existing tests without creating or editing files. It returns no dispatch error, no surviving self-reported-incomplete flag, and an actual `/done` outcome. Its `Changed` and `Writes` are empty. This worker behavior is a fixture precondition, not a prediction about a live model. The policy explanation below shows why no unconditional write-count guard rules this shape out.
5. Git status succeeds and reports ` M unrelated/cache_test.go` or `?? unrelated/cache_test.go`; that file remains on disk. No external process changes either relevant path during settlement.

The resulting source-derived sequence is:

- The attempt starts with an empty owned-write record. `spawnTask` records the observed return's empty writes.
- The test-writing handler returns successfully after the observed turn; it does not separately require a written test path.
- The exact declared target fails `os.Stat`; there are no attempt-written paths and no glob matches.
- The new fallback scans all workspace status records, recognizes the unrelated `_test.go` suffix, and returns success solely because the file exists.
- The transaction has no `test_write` analogue of `validateFileModifyOutcome` on the success path. It ends the attempt successfully.
- `runSingleTask` calls `completeTask`, which sets `TaskCompleted`, increments the campaign count, and asserts `task_result(..., /success, ...)`. The dependency rule no longer blocks another task on T's completed status, subject to all other scheduling conditions. [Attempt record][attempt-record], [handler][handler], [checkpoint][checkpoint], [transaction][transaction], [completion caller][completion], [completion state][completion-state], [dependency rule][dependency]

At the parent revision, the same checkpoint inputs return the missing-path error. Removing only the unrelated dirty test from the new-revision fixture should restore that error. These are proposed differential checks, not results from executed tests.

## Production trace and guardrails

### 1. The task remains a declared mutation

`TaskTypeTestWrite` is a mutating type, so it participates in snapshotting and write-set locking. `validateTaskEffect` requires a target for that kind. This finding supplies a target and does not use the empty-write-set shortcut. The target resolver obtains the non-glob write-set path, and the test-writing handler constructs `generate_tests file:...` for an absent non-directory target. [Mutation type][mutation-type], [effect contract][effect], [target resolution][target], [handler][handler]

### 2. A session `/done` is real, but does not itself prove test writing

The task persona table maps `/test_write` to `/tester`; `task_delegation` uses `persona_verb`, and tester maps to `/test`. The handler queries that derived delegation and passes it to `spawnTask`. [Task routing][task-routing], [tester verb][tester-verb], [delegation query][delegation-query], [handler][handler]

The delegation corpus explicitly excludes `/test` from the file-mutation terminal contract. `/test` requires tools through its `/run_tests` mapping, but not a write tool. The hollow rules reject a tool-requiring turn with no successful call and a write-oriented turn with no write; they do not make `/test` a write-oriented verb. The no-write verification arm, combined with no hollow/build failure and no self-reported incompleteness, permits `turn_done`. [Write policy][write-policy], [test mapping][test-mapping], [tool requirement][tool-requirement], [hollow rules][hollow], [verdict][verdict]

The host emits counts from `SuccessfulToolCalls`, `WriteToolEffects`, and measured test calls. A green test command gives the witness a real successful tool and test execution without a mutation. `checkHollowSuccess` reads the actual policy verdict, and the observed return carries the captured outcome. `RunUntilVerified` accepts `/done` only after checking the dispatch error. This trace does not treat the string "done" as authority. [Measured facts][measurements], [verdict reader][verdict-reader], [observed return][observed], [verdict loop][loop]

The final-report audit is not an unconditional backstop: it returns immediately when `WriteToolEffects == 0` and the verb is not write-oriented. The minimal witness falls in that branch even when final-report auditing is enabled. Other unfinished-work producers can still withhold `/done`; the fixture requires they do not. [Report-audit admission][report-audit]

### 3. Ownership exists upstream but is not consumed by the fallback

`spawnTask` installs the task's write guard and records `ret.Writes`. An open attempt stores those receipts by task ID. The write guard checks mission scope and obtains leases for actual write paths; another task's held path is refused. These controls govern mutation, not the read-only act of counting another file as evidence. The dirty-status helper neither calls the guard nor queries the attempt record. [Observed execution][spawn], [attempt ownership and write guard][attempt-record]

The checkpoint first honors owned writes, which is correct, but its fallback takes only `o.workspace`. Git dirty state describes divergence from the index/repository baseline, not authorship, task identity, the beginning of this attempt, or fulfillment of its request. The helper does not compare a before-state, look for an attempt receipt, read the file's contents, or establish that any test was added for T. [Checkpoint][checkpoint]

### 4. Limits and controls that remain

- **No permission bypass is established.** A denied test execution, failed dispatch, cancellation surfaced as a dispatch/attempt error, missing context, or non-`/done` worker does not become successful merely because the helper could find a dirty file. The success route still passes the normal observed execution boundary.
- **No concurrency is required.** A pre-existing unrelated modification is enough. Concurrent sibling work is an additional attribution case to test, not a necessary assumption.
- **Clean unrelated files do not trigger this new fallback.** A non-Git workspace or failed Git status returns no fallback witnesses. Deleted test paths fail the final stat. These distinguish the new condition from "any test file anywhere passes."
- **Not every test-writing task is affected.** If a declared or owned path survives, the earlier gate already returns. The new behavior matters when those task-specific candidates fail and unrelated dirty tests remain.
- **The fallback is logged.** It reports that dirty/untracked workspace test files satisfied the checkpoint, so this is not a claim of total invisibility. The returned value is only `nil`; the task completion path does not receive a typed ownership or adoption receipt from that log.
- **Partial-write preservation is separate.** The same commit extends preservation to `test_write`, but that route requires recorded writes and returns the attempt error. It is not needed for the zero-write success witness and should not be described as silently marking failed attempts completed. [Partial retention][retention]
- **Task completion is not campaign acceptance.** Later phase checks or acceptance may detect missing tests. The established consequence is premature task completion and loss of that prerequisite's blocking status, not proof that final acceptance passes or that a production incident occurred.

## Proposed repair direction

Keep the distinction between writing tests, running tests, and adopting an already existing test witness. Do not make all session `/test` operations require writes; normal test execution is legitimately read-only.

For a campaign test-writing obligation, measure a task-owned candidate using the existing observed-write receipts, or a separately admitted reusable witness tied to the same task/request and current file revision. A reusable witness must say which earlier attempt or explicitly adopted external artifact fulfilled this obligation and why it remains applicable. A Git dirty bit can help discover candidates, but must not itself be completion authority.

Have the kernel derive whether the typed obligation is satisfied from those measurements, consistent with the repository's executive/measurement split. Preserve permission checks, write-set leases, same-task retries, legitimate guessed-target corrections, no-op/reuse decisions with evidence, TDD red-phase reproductions, and subsequent acceptance gates. Do not restore the redundant whole-tree micro-checkpoint build: a build pass cannot establish who produced a test or which task it satisfies. [Repository architecture][guidance], [campaign contract][campaign-guidance], [C2/L4 history][journey]

This is a proposed contract, not a completed design or a claim that a protected-policy edit is necessary. Whether the narrowest remedy is removing the unowned fallback, adding host-side ownership admission, or carrying an explicit task-effect predicate should be decided with the production checks below.

## Proposed checks, all unrun

1. **Parent/new differential:** missing exact target, empty attempt writes, unrelated pre-existing modified tracked `_test.go`. Parent rejects; new revision currently accepts. A corrected version should reject unless a separate owned/adopted witness exists.
2. **Untracked variant:** the same with an unrelated untracked `_test.go` of a different basename. It must not discharge T.
3. **Controls:** clean unrelated test, non-test dirty file, deleted test, Git failure, and non-Git workspace must not satisfy the fallback. Test these independently of parsing edge cases.
4. **Production composition:** drive `executeTaskWithRollback` and `runSingleTask` with an observed executor returning a genuine admitted no-write `/test` outcome, not a mocked micro-checkpoint result. Assert T is not completed, no `/success` task result is emitted, and a dependent remains blocked when its requested test is absent. Separately exercise the real kernel through `assertTurnEvidence`/the verdict reader for the proposed no-write evidence shape; do not assume source-level derivation is a passed policy test.
5. **Owned alternate path:** an attempt writes a legitimate test at a corrected path outside the planner's mistaken exact target, with observed receipts and permission. Preserve the existing C2 acceptance path.
6. **Prior-attempt reuse:** a previously preserved test from the same task may be reused only with an explicit current witness; a different task's identical basename, dirty state, or recent timestamp must not substitute.
7. **Sibling isolation:** two tasks with disjoint write sets run with one producing a test and the other writing none. Only the owning task gets evidence from that write. Keep the unrelated file unchanged on the failing task's rollback.
8. **TDD and ordinary execution:** legitimate written red-phase reproducers, ordinary read-only `/test_run`, cancellation, denied tools, self-reported-incomplete returns, and failed compiler outcomes retain their respective contracts. Do not fix task ownership by weakening test verdicts or requiring gratuitous edits.

The existing `micro_checkpoint_test.go` was read in full. It checks missing/exact paths, F12's unrelated-document case, and an attempt-owned alternate path, but contains no `test_write` dirty-Git case. The introducing commit changes no campaign test file; its added test concerns session TDD gates. This is a bounded test-source observation, not proof that no other test anywhere could expose the issue. [Checkpoint tests][checkpoint-tests], [introducing change][commit]

## Evidence index

[commit]: https://github.com/theRebelliousNerd/codenerd/commit/ccbe41302073b364ef2cb40ba49b9736738ae6e2
[parent-checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/0f60e7f901ead31f2dd7fba06f1696281939d65d/internal/campaign/micro_checkpoint.go#L23-L56
[checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/micro_checkpoint.go#L26-L101
[transaction]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_task_transaction.go#L53-L119
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_task_transaction.go#L122-L173
[completion]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_tasks.go#L450-L483
[completion-state]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_tasks.go#L659-L703
[dependency]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/campaign_tasks.mg#L19-L24
[mutation-type]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/task_mutation_types.go#L6-L20
[effect]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/task_effect_contract.go#L8-L38
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_task_transaction.go#L316-L383
[target]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_task_handlers.go#L641-L678
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_task_handlers.go#L876-L930
[task-routing]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/campaign_rules.mg#L638-L691
[tester-verb]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/delegation.mg#L525-L530
[delegation-query]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_decisions.go#L60-L90
[write-policy]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/delegation.mg#L210-L222
[test-mapping]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/delegation.mg#L150-L155
[tool-requirement]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/delegation.mg#L257-L281
[hollow]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/coder_safety.mg#L708-L739
[verdict]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/coder_safety.mg#L770-L800
[measurements]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/session/executor.go#L2816-L2861
[verdict-reader]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/session/executor_tools.go#L2271-L2329
[report-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/session/admission_audit.go#L60-L83
[observed]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/session/observed_return.go#L47-L85
[loop]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/session/verdict_loop.go#L110-L157
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_task_handlers.go#L31-L119
[attempt-record]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/orchestrator_attempt_writes.go#L16-L155
[checkpoint-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/micro_checkpoint_test.go#L27-L242
[deletion-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/campaign-deletion-microcheckpoint.md
[capability-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/campaign-capability-checkpoint-continuity.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/journeys/07-external-audit-2026-09-19.md#L52-L80
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/agents.md
