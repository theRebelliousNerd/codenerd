# Bind npm checkpoint reports to the invocation that produced them

**Status:** source-supported conditional verification-integration gap; proposed repair, not an executed incident.  
**Source:** public CodeNERD main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), inspected 2026-10-30.  
**Method:** static repository and primary-provider-source review, including independent challenge. No tests, application execution, model inference, training, benchmarks, local Git or repository writes were performed. The examples and evaluations below are unrun.

## Finding

An npm phase checkpoint can reject a successful current test invocation using **a previous invocation's failed JSON report**.

The runner requests `--json --outputFile=.nerd/npm-test.json` as a best-effort addition to `npm test`. After the process returns, it reads that fixed pathname if present. Any parsable report with a positive total replaces the counts parsed from the current process output. It neither establishes that this invocation wrote the report nor retires a previous report before running. With an old report containing one failure and a current successful npm script that does not rewrite the file, the checkpoint returns failure. [Request, execution and consumption][runner]

This is a **freshness/ownership defect in an optional evidence file**, not a claim that every npm runner ignores the added flags, every successful npm process proves adequate coverage, or an old passing report overrides a current nonzero exit. The nonzero-exit check remains authoritative. The strongest supported consequence is false checkpoint rejection and conditional unnecessary remediation/rechecking, not acceptance of a known failing process.

## Scope, prior work and freshness check

Root guidance requires stale evidence not to remain current truth. Campaign guidance requires real verification and preserves phase/checkpoint obligations. This proposal keeps those requirements and the existing configured executor; it does not replace a test gate with model prose. [Root guidance][root], [campaign guidance][guidance]

Earlier orchestrator QA already calls out checkpoint non-determinism, timeout/sandbox boundaries and failure amplification. Those are useful predecessors. This memo narrows the cause to a deterministic stale-file substitution after a successful current invocation, rather than reopening the entire checkpoint subsystem. Existing parser tests also preserve several real fixes: avoiding double-counting test names, not reading `ok` inside package names, retaining failure in mixed summaries and parsing Jest count fields. None of those parsing protections proves a report belongs to this invocation. [Prior QA][qa], [parser controls][tests]

The main audit inventory and research branches through `research-main-20261030-1517` were screened. The recent reproduction-outcome audit concerns the expected meaning of a diagnostic test result, not this reuse of a prior npm report. The checkpoint-profile audit concerns build configuration continuity, not report ownership. The latest research branch is documentation-only. The entire inspected `internal/campaign/checkpoint.go` matches main on all five active non-research branch heads; no alternate repair in that file was found. [Latest research branch][research]

## Real production route

1. **TUI campaign execution.** Normal campaign startup constructs the orchestrator with the live executor and workspace. The started message launches its background `Run`; this is not limited to an isolated parser test. [TUI construction][construction], [startup/run][chat], [Update][update]
2. **Checkpoint installation.** `NewOrchestrator` builds a `CheckpointRunner` using that executor/workspace and sets its command timeout from campaign policy. [Installation][installation]
3. **Phase admission.** When no task is active and the kernel says the phase's tasks are done, the task loop first considers quality remediation, then invokes `runPhaseCheckpoint`. An objective whose method is `/tests_pass` reaches `runTestsCheckpoint`. The witness requires reaching this stage; it does not assume every generated plan contains such an objective. [Phase consumer][phase], [objective dispatch][phase-record], [method dispatch][method]
4. **npm selection.** The framework detector recognizes a package.json-only workspace as the generic npm route, using the Jest framework atom as its fallback. Jest, Vitest and Mocha mappings all produce `npm test`. A higher-priority Go/Cargo marker can select a different route and is excluded from this witness. [Detection and mapping][framework]
5. **Fixed optional report.** A plain `npm test` command receives the reporter flags and runs through the configured tactile executor. The code reads current output and checks execution error/exit status, then optionally loads `.nerd/npm-test.json`. Its only override criterion is that the parsed passed-plus-failed count is positive. [Runner][runner], [count parser][json-parser]
6. **Effect on the campaign.** `runPhaseCheckpoint` stores the failed checkpoint details and asserts the corresponding fact. The task loop increments failure accounting and asks policy for replan, recheck or closure as unverified. A valid nonempty remediation scope can produce a new file-modification task briefed with those failed details. [Recording][phase-record], [policy-directed handling][phase], [remediation producer][remediation]

The supported consequence is therefore more than an inaccurate standalone count: this count participates in a live phase-admission result. Whether subsequent remediation runs, changes files, fixes the report path or reaches a later checkpoint is conditional on the remaining scheduler, scope, permission, task-verdict and acceptance gates.

## Why a successful npm script need not create this file

npm documents `npm test` as executing the package's `scripts.test`, and its example is `node test.js`. npm also forwards arguments after `--` to the selected script. It does not implement Jest's JSON reporter itself. A project-owned script can execute real assertions, return zero and accept/ignore reporter arguments without writing the requested pathname. That is a legitimate **conditional runner behavior**, not a claim about all Node, Jest, Vitest or Mocha invocations. [npm test contract][npm-test], [argument forwarding][npm-run]

Jest separately documents `--outputFile` with `--json`. In the inspected Jest v30.2.0 source, result processing serializes the completed results and writes the specified output file when those options are enabled; the write branch is not conditioned on all tests passing. This establishes an ordinary possible producer of a prior failed report. It does not establish which Jest/npm version is installed in any user's environment. [Jest option contract][jest-doc], [pinned report writer][jest-source], [completed-results handoff][jest-results]

## Conditional witness, with controls

Use a synthetic foreign Node workspace on a supported host, with healthy storage and executor configuration:

1. It contains `package.json`, has no higher-priority Go/Cargo marker, and reaches a live campaign objective using `/tests_pass`.
2. `.nerd/npm-test.json` already exists and remains readable, holding a valid prior report such as `{"numPassedTests":0,"numFailedTests":1}`. It may have been produced by a previous Jest-backed checkpoint. Its existence and old provenance are explicit witness preconditions, not a model prediction.
3. The current package's test script really executes passing assertions, terminates normally with exit zero, and does not replace that file. A supported `node test.js` script that intentionally does not consume reporter flags is a clean fixture. A test-runner migration from Jest is one possible history; the finding does not require the current campaign to perform the migration.
4. No current command error, timeout, cancellation, concurrent writer or unrelated report rewrite occurs. Current stdout carries a recognizable passing result, for example a plain `1 passed` summary. A positive current count avoids depending on the optimistic empty-output fallback.
5. All normal phase-completion prerequisites have been met, so the checkpoint is actually reached.

Source-derived outcome:

| Evidence | What it belongs to | What current code uses |
|---|---|---|
| Exit zero | Current invocation B | `hasExitError=false` |
| Passing stdout | Current invocation B | Initially parsed, then replaced |
| JSON failed count of one | Earlier invocation A | Overrides B's counts because total is positive |
| Checkpoint verdict | Supposed to describe B | Failure, with A's count plus B's raw output |

The result is a failed checkpoint whose count is not current evidence. The details still append current raw stdout; an attentive operator or later task can notice the contradiction. Thus the error is not wholly silent, and this audit does not claim that all diagnostics are discarded.

The eventual evaluation must compare the same invocation with the old report absent, present-and-stale, and freshly rewritten. Do not claim a reproduced failure until those conditions and the full phase consumer have been exercised.

## Mitigations and limits that remain important

- **Current process failures still fail.** A nonzero current exit is checked independently of JSON counts. An execution error returns failure before this report branch. A stale green report therefore does not establish a successful checkpoint over a nonzero current command in this trace. [Runner][runner]
- **Fresh cooperating reporter is a control.** If the current supported reporter replaces the file with its actual report before returning, the particular stale-file witness does not occur.
- **Not all runners accept these flags.** A script that rejects `--json` or `--outputFile` can fail for that separate compatibility reason. Do not present that as an instance of the zero-exit witness or assume every npm mapping is Jest.
- **Missing, unreadable, malformed and zero-total files do not override.** They leave the current stdout parser's result in place. The parser's deliberate optimistic fallback is a separate existing policy, explicitly represented in tests; freshness repair should not silently redefine it. [JSON branch][runner], [parser tests][tests]
- **No cross-campaign race is needed.** A sequential earlier report plus a later successful invocation is sufficient. Shared-path concurrent producers are useful future controls, but this audit does not depend on defeating the TUI's single-managed-campaign admission.
- **No fabricated overall success is claimed.** The failure leaves the phase open or, at its configured cap, unverified. Existing checkpoint policy prevents silently completing a repeatedly failing phase. Default checkpoint attempts are bounded; a repair or an authorized operator may also remove/replace the stale report before the next run. [Failure handling][phase], [defaults][defaults]
- **Remediation is conditional.** The producer requires a usable phase write scope and successful fact publication; it does not guarantee an actual mutation. Any admitted task still owes its normal execution checks. [Remediation][remediation]
- **A full successful test command is not automatically sufficient coverage.** The proposed change establishes report attribution, not that all required tests were selected, executed or meaningful. Zero tests, skipped tests and a script that merely prints “passed” require separate evidence controls.
- **No permission or sandbox bypass is asserted.** This is the existing checkpoint executor path, not a proposal to grant shell access or weaken its authority, timeout, process cleanup or task controls.

## Proposed fresh-evidence contract

### Invocation ownership

Give each checkpoint execution an opaque invocation identity bound to campaign, phase, objective/method and attempt. Bind the command, working directory, runner/reporter identity, effective relevant configuration and source snapshot where available. The process receipt and optional report must refer to that execution, rather than to the mere existence of a conventional filename.

For a reporter whose capability is established, allocate a **unique runner-owned report path** under the current run's artifact namespace and pass it to that invocation. Consume only that path after the process has settled. Store a content digest and provenance with the parsed result. Treat unexpected pre-existence, failed creation/publication or malformed report as explicit evidence conditions.

Do not blindly delete a fixed workspace file as though ownership were already proven. A transitional cleanup approach must establish ownership, handle cleanup failure explicitly and avoid racing another run. Modification time or unchanged size alone is not an ownership proof: coarse timestamps, replacement, equal-size results and concurrent producers can defeat those checks. A unique path plus an explicit lifecycle is easier to reason about than “newer than the clock.”

### Reporter compatibility and outcome precedence

Keep **runner capability** distinct from generic `npm test`. npm is a script dispatcher; it does not prove that a Jest reporter is installed. Request structured output only through a compatible adapter or a declared/tested reporter capability. Do not silently make optional JSON mandatory for every npm script.

Go should publish typed observations: process started/settled, exit outcome, selected parser, report state (`fresh`, `missing`, `stale`, `malformed`, `unsupported`), counts/selection coverage and source identity. Mangle should decide the checkpoint disposition. In particular:

- a current nonzero exit, timeout or refusal cannot be overridden by stale or fresh green counts;
- an unowned prior report cannot replace current stdout or become evidence for either pass or fail;
- contradictory **fresh** evidence needs an explicit conservative disposition, not silent replacement;
- unsupported optional reporting should use the declared current-output contract, or report inconclusive if the objective requires stronger evidence;
- clearing stale evidence is not permission to claim tests executed where the current receipt does not establish that.

The user-facing checkpoint receipt should say which evidence supplied its counts and why an available file was excluded. Preserve bounded original output, the original report and identity metadata for diagnosis without spreading sensitive test output indiscriminately.

### Persistence and recovery

Historical reports remain historical; do not rewrite their provenance or silently attach them to the newest attempt. Legacy fixed-path reports should not be current inputs to new runs. Retain them as diagnostics under a clear migration/retention policy if needed.

Define cleanup after cancellation, crash and disk-full conditions. A late report from a retired attempt must not settle its successor. If the process ran but its required report could not be safely attributed, preserve that uncertainty rather than retrying mutations against a fictitious test failure.

## Alternatives, costs and failure modes

| Approach | Benefit | Limitation / cost |
|---|---|---|
| Current fixed path; manual deletion when symptoms appear | No implementation cost. | Couples attempts; diagnosis and repeated host/model work may be paid by the user. Not a correctness contract. |
| Ignore all sidecar JSON | Removes stale-file substitution with little state. | Loses useful structured counts for real Jest runs; current stdout parsing can be less precise. Fair baseline, not automatically the preferred fix. |
| Delete/truncate fixed file before launch | Small sequential repair for owned files. | Needs ownership, deletion-failure handling and concurrency protection; can erase useful or foreign evidence. |
| Accept report only if mtime/size changed | Low metadata cost. | Does not reliably prove producing invocation; must not be the sole evidence boundary. |
| Unique per-invocation report plus capability-aware parser | Stronger attribution, preserves useful structured evidence and history. | Adds identifiers, artifact lifecycle, parser/receipt states, persistence and cleanup; needs adapter-specific compatibility and downstream policy integration. |

No additional model or embedding request is necessary for attribution. Extra local costs include path allocation, filesystem operations, report hashing/parsing, receipt serialization, kernel facts/derivations, retention and cleanup. Unique files can accumulate after crashes; bound their retention and separate audit retention from active evidence. Large reports need byte limits and bounded presentation without silently dropping the outcome-bearing fields.

Potential savings are avoided host reruns, erroneous corrective tasks, planning/worker tokens and operator diagnosis. None is measured. Report both gross added overhead and net avoided work, including cases where every report is already fresh and the proposed lifecycle only adds cost.

## Evaluation plan — not run

Compare current main, current-output-only, ownership-safe fixed-file retirement, and unique invocation reports under the same test script, command timeout, executor, workspace, phase plan and output. Exercise the real TUI → orchestrator → checkpoint → phase failure-policy path. Parser-only tests are supplemental.

| Case | Required observation |
|---|---|
| Old red report, current genuine passing script, no rewrite | Current code rejects using old count; repaired design excludes it and judges only current admissible evidence. |
| Same script with report absent | Preserve current legitimate outcome; isolate freshness from parsing and coverage. |
| Fresh red report and current nonzero exit | Still fail; never downgrade a true failure. |
| Old green report with current nonzero exit | Current safeguard preserved; no stale pass. |
| Fresh positive report/current passing run | Structured counts retained with invocation identity. |
| Unsupported/rejected flags | Classify capability/command failure distinctly; do not assume universal flag tolerance. |
| Malformed, unreadable, empty, zero-total, oversized or partially published report | Explicit bounded outcome; no fabricated fresh counts or unbounded read. |
| Equal-size/reused-timestamp report; late previous writer; concurrent report producer | Verify identity, not only filename, mtime or size. |
| Timeout, cancellation, crash, storage failure, failed cleanup | Preserve process outcome, retired ownership and recoverable uncertainty. |
| Repeated checkpoint failures and valid remediation scope | Account for failure counters, new tasks and cap behavior; don't require a particular model repair. |
| No `/tests_pass` objective, higher-priority Go/Cargo detection, genuine fresh reporter | Negative controls; unrelated verification routes unchanged. |
| Zero/skipped tests and fake pass prose | Freshness repair must not be mistaken for new execution/coverage evidence. |

Measure current-versus-consumed report identity, exit status, parsed counts, checkpoint/phase status, stale exclusions, false rejects and false accepts, executed checks, retry/remediation counts, model calls/tokens, host latency/CPU, report bytes, filesystem/kernel overhead, retained artifacts and human recovery time. Preserve raw receipts for failures in the evaluation itself.

**Acceptance criterion:** each checkpoint's test counts come from its own admissible execution evidence; old reports neither block nor approve new work, while real current failures and uncertainty retain their existing protective meaning.

[root]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/agents.md
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint.go#L159-L238
[json-parser]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint.go#L544-L553
[qa]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/journal_entry.md#L185-L197
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint_parsers_test.go#L5-L105
[research]: https://github.com/theRebelliousNerd/codenerd/commit/eb50ab1d51fc131a4d9af49f76f5aea62c855832
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L84-L139
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L29-L153
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L325-L353
[installation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_init.go#L63-L87
[phase]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L152-L227
[phase-record]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_utils.go#L48-L117
[method]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint.go#L117-L155
[framework]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/framework.go#L64-L184
[remediation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_checkpoint_remediation.go#L44-L95
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/campaign.go#L162-L180
[npm-test]: https://docs.npmjs.com/cli/v11/commands/npm-test/
[npm-run]: https://docs.npmjs.com/cli/v10/commands/npm-run-script/
[jest-doc]: https://jestjs.io/docs/cli#--outputfilefilename
[jest-source]: https://github.com/jestjs/jest/blob/855864e3f9751366455246790be2bf912d4d0dac/packages/jest-core/src/runJest.ts#L90-L129
[jest-results]: https://github.com/jestjs/jest/blob/855864e3f9751366455246790be2bf912d4d0dac/packages/jest-core/src/runJest.ts#L313-L334

