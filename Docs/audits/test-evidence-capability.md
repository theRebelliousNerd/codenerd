---
doc-class: deep-dive
subsystem: verification
implementation-status: proposed-refinement
last-verified: 2026-10-28
verified-against: 8cd6e1f8804709996b46e43641a3ffe464d2088a
---

# Beyond the dormant wrapper: identity-preserving test intelligence

## Decision

`Executor.assertTurnTestFacts` is not evidence that CodeNERD forgot to send test results to its executive. Its substantive work has a production replacement: `syncTestGateFacts` directly calls `turnScopedTestFacts`, replaces prior measurements, and publishes baseline attribution. Restoring the wrapper would duplicate assertions, not restore a missing capability.

The worthwhile unfinished capability is narrower and more consequential: **preserve package-qualified test identity all the way from observed execution through baseline comparison, executive attribution, and repair guidance**. Rich facts already contain this identity; the active attribution path reduces them to test names. The next increment should make the existing test intelligence trustworthy across multi-package repositories before expanding its presentation.

Status: source-inspected analysis and design, not an executed reproduction. No code, tests, training, inference or benchmarks were run. Main was rechecked against `8cd6e1f8804709996b46e43641a3ffe464d2088a` immediately before drafting and was identical. No measured defect rate, speedup, or end-to-end false-completion claim is made.

## 1. What this code was supposed to be

The repository's [North Star and runtime contract](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/agents.md) makes the model creative and the kernel executive. Test output should therefore become typed observations from which completion and repair obligations are derived, rather than a model's narrative that “tests passed.” [Core runtime guidance](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/agents.md) also reserves completion for host-issued current acceptance evidence. Mechanical tests are one part of that contract, not a substitute for requested-behavior acceptance.

The [turn test-fact adapter](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/turn_test_facts.go) does three useful things:

1. Converts structured test results into turn-owned facts, avoiding unscoped state collisions between turns.
2. Retains package, test, positioned diagnostics and repeated-output information where the underlying schema supports it.
3. Archives full failing output by package/test and exposes recall handles instead of repeatedly flooding model context.

The dormant wrapper is only a convenience entry point into the first operation. The living capability is a test-observation pipeline and a bounded evidence memory. Treating the wrapper as the innovation would confuse an obsolete API with the purpose it served.

## 2. Current production wiring

| Stage | Inspected mechanism | Classification |
|---|---|---|
| Execute and interpret checks | `verifyTests` runs constrained `go test -json` and preserves explicit outcomes plus parsed `Result` | Implemented source path |
| Restore pre-edit comparison | `attributeTestFailures` uses available preimages and an overlay, then accepts only conclusive baseline runs | Implemented source path |
| Publish current test measurements | `syncTestGateFacts` retracts this turn's previous rows, asserts measured result and baseline names, then directly emits `turnScopedTestFacts` | Implemented replacement for wrapper |
| Decide /test disposition | Mangle joins measured head failure with absence/presence in the baseline; importer failure is a separate channel | Implemented source path |
| Record final evidence | `recordBuildState` invokes synchronization and explicitly avoids a second test-fact assertion | Implemented source path |
| Preserve package identity in baseline attribution | `failedTestNames` and baseline facts use names without package | Missing in inspected attribution contract |
| Demonstrate this path's runtime performance | Not measured here | Unverified |

Sources: [test verification](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/test_verify.go), [baseline comparison:18–149](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/test_baseline.go#L18-L149), [synchronization:47–90](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/gate_facts.go#L47-L90), [recording:2668–2744](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/executor.go#L2668-L2744), and [executive rules:367–441](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/defaults/policy/coder_safety.mg#L367-L441).

`testGateRed` and `testGatePassed` also synchronize before querying derived gate state in [gate_facts.go](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/gate_facts.go). A stale comment in `TestVerification.Result` still names the dormant wrapper; that is documentation drift, not proof of missing production wiring.

This complements, rather than duplicates, [verification evidence and judge contracts](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/audits/verification-evidence-and-judge-contracts.md). That document concerns the downstream advisory judge and typed handoff. This one concerns deterministic test identity *before* the executive's test verdict.

## 3. Where the information is lost

[Go's test2json contract](https://go.dev/cmd/test2json/?m=old) represents package and test separately; package distinguishes interleaved events from different packages. CodeNERD initially respects that structure:

- `test_case(Package, Test, Status, ElapsedMs)`;
- `test_failure_at(Package, Test, File, Line, Message, Count)`;
- `test_build_failure(Package, File, Line, Message)`.

But [`Facts()`](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/testfacts/facts.go#L9-L110) also emits `failing_test(Test, Message)` without package. A coarse “some test failed” predicate is adequate for selecting general failure context; it is not an injective identity for deciding whether a particular failure predates a change.

[`failedTestNames`:53–76](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/testfacts_helpers.go#L53-L76) loops over packages but inserts only `t.Name` into a string set. This preserves subtest suffixes, which is useful, while collapsing equal names from different packages. The baseline stores that set; `turn_test_failed_before(Turn, TestName)` and `turn_failing_test(Turn, TestName, Message)` then drive ownership. [Importer synchronization](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/importer_gate_facts.go) has the same name-only shape in its separate channel.

### Conditional witness, not an executed reproduction

Suppose a measured scope includes packages `example/a` and `example/b`, both containing `TestLoad`:

| Identity | Baseline | Head |
|---|---|---|
| `example/a::TestLoad` | fail | fail |
| `example/b::TestLoad` | pass | fail |

Assume the head runs emit named failure diagnostics for `TestLoad`, producing the corresponding `turn_failing_test` rows; this is not a fixture consisting only of failed status records. Without those rows, the existing no-named-failure rule can fail closed instead. The true new failure set contains `example/b::TestLoad`. Under name projection, both baseline and head failure sets are `{TestLoad}`. The current negated-baseline rule cannot distinguish the newly failing package. Provided no importer blocker or other new named failure intervenes, the inspected rules can classify this /test gate as passing despite the new package-qualified failure.

This is a static inference from the exact producer and rules, not proof of an observed incident. Other acceptance, coverage, build or completion obligations can still block the overall turn. Nor does a baseline failure with an identical package/test prove identical causal failure: a changed implementation might introduce a second defect in a test that was already broken. The first fix is identity preservation; causal certainty requires additional evidence, not a stronger adjective.

## 4. Proposed minimum capability contract

Keep legacy existence predicates where their consumers need only a failure flag. Introduce or evolve the authoritative attribution representation to preserve a typed key:

`TestKey = (PackageImportPath, FullSanitizedTestName)`

Avoid concatenation ambiguity: carry typed columns, or a canonical encoded identifier plus original fields. Do not use basename or top-level name as identity. Top-level projection remains appropriate for constructing a `-run` selector, not for deciding that a failure predates this turn.

For each measurement retain host provenance:

`Run = (Turn, Attempt, Phase, WorkspaceIdentity, SourceSnapshot, ScopeIdentity, CommandConfigIdentity)`

`Phase` distinguishes own/head, own/baseline, importer/head and importer/baseline. These are proposed contract fields, not a claim all are currently available. Unknown identity is explicit; do not fabricate it from model text. A baseline/head comparison is admissible only for compatible scope/configuration and the intentional source difference represented by the overlay. Concurrent unrelated edits invalidate that comparison rather than silently becoming part of “before.”

Define sets over qualified keys, not strings:

`F_head = {k | observed complete head outcome for k is fail}`

`F_base = {k | observed compatible baseline outcome for k is fail}`

`NewCandidates = F_head \\ F_base`

`BaselineAssociated = F_head ∩ F_base`

“Baseline-associated” is an observation, not a proof that the current change caused no additional harm. Unnamed build failures, missing package identity, incomplete streams, cancellation and unavailable baselines remain explicit unresolved/failed evidence under policy. Absence of a parsed failure must not mean a successful completed run.

### Safe migration sequence

1. Preserve package identity in parsed baseline records and qualify the existing head failure path.
2. Keep own/importer channels distinct; update both contracts together.
3. Derive old and new classifications in diagnostic shadow mode over the same immutable observations. Only one classifier retains executive authority during migration.
4. Review divergences, then switch the authoritative gate contract atomically with its declarations, producer, shard routing, retraction, model-assertion restrictions and readers.
5. Retire obsolete name-only attribution grants after compatibility review. Do not let an old passing derivation coexist as a second permissive route.

The Go boundary measures and validates; Mangle derives obligations. This proposal does not move executive decisions into a Go heuristic or allow the model to assert its own test evidence.

## 5. The next innovation after identity is correct

Use already structured observations to derive **specific evidence-linked repair obligations**, instead of merely saying “tests failed”:

- affected package and full test identity;
- head/baseline observation references and attribution confidence category;
- positioned diagnostics and relevant current source identity;
- bounded summary with a recoverable full-output handle;
- unresolved evidence obligation where diagnosis cannot be made;
- separate requirement/acceptance obligation when passing tests do not establish intended behavior.

The current archive already keys full failing output by package/test and content; build on it, do not create a second unbounded log store. JIT should receive a compact obligation plus selected applicable evidence, with explicit omissions. Additional output retrieval is policy-mediated. A failure message containing prompt-like text remains untrusted diagnostic data, never an instruction.

This is a proposal to complete the *meaning* of the pipeline, not evidence that all fine-grained repair readers are absent. A subsequent consumer inventory should trace each richer predicate to its current JIT/repair readers before adding any new route.

## 6. Cost, failure modes and evaluation

Identity restoration need not add a model call or another baseline test run: package information is already in parsed results. It does add larger keys, schema/routing maintenance, fact storage and join/index work. Run-scoped retention adds metadata and lifecycle complexity. Diagnostic shadowing adds duplicate derivation work temporarily. Any future evidence acquisition, rerun or model diagnosis must be charged separately.

Measure whole-task cost:

`C_total = C_existing_execution + C_baseline + C_parse + C_fact_update + C_fixpoint + C_archive + C_JIT + C_model + C_repair_and_reverification`.

Count failed/canceled attempts, input/output tokens, CPU, peak memory, storage, bytes retrieved and wall time. Report cache-read/write token subcounts under the provider's accounting contract; do not add cached tokens again when they are already included in total input tokens. Report qualified identity counts and concurrent attempts, not just test count. Avoid retaining every run in the active kernel: retain immutable receipts outside active state with policy-owned bounded current pointers, while making old evidence explicitly historical. Negated rules must operate on a complete committed measurement epoch, not a partially published stream.

Future authorized evaluation should include:

1. Two packages with identical test names, only one baseline failure: newly failing other package must remain actionable.
2. Equal subtest names under different packages and new child failures under an already failing parent.
3. Same package/test with changed failure diagnostics: report baseline association without unsupported causal exoneration.
4. Own tests passing while importer run fails or remains unfinished: preserve importer blocking.
5. Incomplete JSON, package build failure before any test, cancellation, unknown baseline preimage, missing identity and malformed output: never infer pass from missing rows.
6. Repair reruns and concurrent turns: no stale epoch, cross-turn borrowing or mixed old/new verdict.
7. Evidence selection with huge repeated logs: bounded prompt, available recall, no loss of failure identity.

At the socket level, replay identical immutable parsed observations into current versus qualified-identity contracts and compare expected dispositions against manually specified ground truth. At complete-system level, use normal production entry paths, same model/tools/budget, repositories with realistic pre-existing failures, and independent hidden acceptance checks. Measure escaped regressions, unnecessary repair, unresolved evidence, full tokens/cost and latency. Hold baseline rerun policy constant in the identity-only ablation so it cannot take credit for unrelated extra verification.

No savings should be claimed before those measurements. More accurate attribution might cost more while avoiding an unacceptable regression; reduced token use alone is not the objective.

## Handoff

Do not reconnect `assertTurnTestFacts` merely to make dead-code counts fall. Preserve its current replacement. Reconcile the session/testfacts/core specifications around package-qualified, current evidence; retain the dormant-wrapper finding as supersession. The checkable exit is not “function reachable.” It is: **a genuinely new package-qualified failure cannot disappear behind an unrelated baseline failure, and the executive can explain the distinction using current host evidence.**

## Research refinement — Preserve verification obligations, not merely the last passing process

**Status:** source-inspected proposal, 2026-10-28. No tests, executable fixtures, model inference, implementation, benchmarks or failure injection were performed for this refinement. The sequences below are conditional source witnesses, not reproduced incidents or measured frequencies. Publishing this research does not implement or verify the proposed runtime contract.

**Source freeze:** main `356f65e0e6bb639adaa90ab36212daa714114871`; feature `feat/conversation-brief` at `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`; research inventory `7ee5579ac456cf2a9f20d6338abbf633e74384a0`. The relevant `internal/gates/{gates,detect,run}.go`, `internal/session/{workspace_test_run,test_run_round,change_evidence}.go` and `internal/core/defaults/policy/coder_safety.mg` files have identical blob SHAs on main and feature. Citations below use these immutable source revisions.

### Decision and relationship to prior work

Keep the implemented package-qualified Go test facts, baseline attribution, host-only evidence predicates and post-write sequencing. Add an explicit coverage contract for the session's non-Go `/test_run` gate: a passing process must satisfy the applicable verification obligations, not erase them.

The original test-evidence audit concerns whether a newly failing package-qualified test is mistaken for a pre-existing failure. Its October 28 as-built reconciliation remains valid and should be preserved. This refinement concerns a different denominator: **which checks were required, which actually ran, and which claims the final report may make**.

The inspected research inventory contains adjacent work on [tool-effect validation completeness](tool-effect-receipts-and-validation-completeness.md), [judge applicability](verification-applicability-and-progressive-evidence.md), [documentation evidence](documentation-evidence-contract.md), and [planned-work conservation](planned-work-conservation.md). None of those inspected reports traces the `workspaceTestRun` adapter's loss of declared unavailable gates or its bypass by a pre-existing test receipt. Reuse their receipt and applicability concepts rather than introduce another evidence store.

The [repository North Star](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star) assigns completion to Mangle and requires tests that harden the actual system. [Session guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md) preserves explicit evidence, revision-bound completion and the distinction between observed outcomes and model claims. Existing workspace-gate machinery already supplies useful ingredients: stable gate IDs, scope, command, language, provenance, explicit unavailable gates and per-run results. The gap is not absence of this machinery; it is information lost when session verification consumes it.

### Existing protections to retain

- A tool name alone is not a test execution. `TestRun` is recorded where a process actually ran; dry-run or empty selections need not produce receipts. [Receipt definition](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/test_runs.go#L9-L18).
- Writes and test runs share a turn-owned sequence. Runs before the latest write cannot satisfy `/test_run`. [Policy](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/coder_safety.mg#L515-L568).
- The workspace detector distinguishes runnable and unavailable gates; a missing program does not become a successful `gates.Result`. [Detection partition](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/gates/gates.go#L134-L176), [runner](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/gates/run.go#L65-L110).
- When all selected workspace gates finish conclusively, `workspaceTestRun` retains an earlier failure despite a later successful gate. The defect below is not a universal last-success-wins bug inside that loop. [Aggregation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L69-L95).
- Verified caller acceptance is a separate completion mechanism. These findings do not demonstrate bypass of that contract, permission checks, or every independent completion obligation.

### W1 — A model-selected run can suppress the workspace's checks

`verifyAndRepairTestRun` calls `workspaceTestRun` only when `TestRunSinceLastWrite == nil`. It subsequently returns immediately if `/test_run` is green. A receipt therefore means both "a process ran" and "there is no need to consult the workspace's verification contract." [Caller](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/test_run_round.go#L42-L58).

The typed test tool permits test filtering and records the actual resulting process. Session ingestion retains its last receipt and publishes each exit to the sequence ledger. The Mangle relation carries only turn, sequence and exit—not gate, scope or covered obligations. [Typed filtering and recording](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/shell/verification.go#L193-L245), [session ingestion](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L1012-L1027), [gate derivation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/coder_safety.mg#L526-L568).

**Conditional witness:**

1. An authorized turn changes `svc/app.py`.
2. Its workspace declares a test command capable of detecting the introduced defect.
3. After the write, the model invokes an authorized supported test runner with a filter selecting an unrelated passing test.
4. The real process exits zero and produces a receipt.
5. The forcing round does not invoke `workspaceTestRun`; the recorded post-write zero exit satisfies `/test_run`.

No fabricated receipt, shell escape or permission failure is required. Other gates or a judge may still reject the task. The established finding is that this gate does not enforce the workspace-check contract its helper claims to provide.

An equivalent, adequately scoped run should be reusable. The needed distinction is **matching evidence versus merely present evidence**, not "always rerun every test."

### W2 — Unavailable declared gates disappear from aggregate coverage

`gates.split` retains unavailable checks in `Set.Unavailable`. `workspaceTestRun` examines only `set.Gates`, selecting applicable runnable tests. It never records the applicable unavailable entries. If at least one selected test passes, and all selected runnable tests pass, it publishes a zero-exit aggregate even when another declared applicable test could not run. [Partition](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/gates/gates.go#L143-L176), [consumer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L47-L95).

**Conditional witness:** declare two applicable test gates; one installed command succeeds, while the other command is absent from PATH. The detector correctly preserves the second as unavailable. The session adapter discards that distinction and can derive `/test_run /passing` from the first alone.

This does **not** mean a workspace with no runnable tests directly becomes green: `len(run)==0` returns false. The loss occurs particularly in the mixed available/unavailable case. A future contract must make required versus advisory status explicit, rather than assuming every discovered check is mandatory or allowing availability to decide that status implicitly.

There is a second form of loss after execution begins. If one selected gate fails, then a later gate returns `Unverified`, the adapter returns `false, ""` before storing the aggregate or publishing its sequence. Earlier output and the known failure remain local rather than reaching this receipt boundary. The caller then treats `byWorkspace=false` as the model-fallback path. [Early return](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L76-L95), [fallback/recheck](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/test_run_round.go#L59-L83).

An unavailable result does not itself mint green. A later passing model-selected process can do so because the required-check set and earlier failed aggregate were not retained. Use a missing workspace-relative executable as a proposed non-cancellation witness: such a command remains in runnable discovery until execution, and can fail to start after an earlier gate's failure. Avoid relying solely on cancellation, which may independently stop subsequent work.

#### Recheck variant: an ignored unavailable outcome can leave a passing tool receipt current

The initial workspace run can fail conclusively, leaving `byWorkspace=true`. A repair attempt may then produce a passing model-selected test receipt. The recheck invokes `workspaceTestRun` again but discards its returned boolean (`_, out := ...`). If that invocation becomes unavailable before publishing an aggregate, it leaves the newer tool receipt and sequence untouched. `testRunGatePassed` can consequently read that receipt's green result and return success for this repair gate. [Initial workspace ownership and repair recheck](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/test_run_round.go#L47-L83), [no-publication early return](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L80-L95).

This requires a change in availability or a later indeterminate/canceled run; it is not what happens when the workspace rerun conclusively fails. A transient launch failure is a cleaner proposed witness than cancellation because cancellation may terminate the wider task. The proven source boundary is `/test_run` and its repair result, not unconditional whole-task acceptance.

#### Multi-language limit

The adapter collects a union of written languages and a separate union of written directories. It selects gates whose language matches that union (or is language-free), then applies each node-scoped selected gate to the directory union. It does not retain a per-file or per-language required-coverage relation. [Selection and node expansion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L43-L77), [written-file projection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L109-L136).

Do not infer that every multi-language task is affected: a workspace-wide or language-free declared check can intentionally cover several languages. The narrower concern is that one available language's pass does not prove coverage of another language with no applicable runnable check. Language labels and directory proximity are not dependency proofs. Preserve explicit unknown coverage and project-declared cross-language scope rather than inventing a universal mapping from extensions to adequate tests.

### W3 — "No tests collected" is not positive test coverage

Detected Python gates deliberately accept pytest exits zero and five. Session aggregation normalizes either accepted outcome into the aggregate's zero exit. [Detector](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/gates/detect.go#L132-L140), [normalization](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run.go#L69-L100).

That choice is understandable for traversal: a source directory without colocated tests need not be classified as containing failed tests. It is insufficient as affirmative evidence that the changed source was tested.

For example, a project can put production files under `src/pkg/` and tests under a separate `tests/` directory. The detected node command targets the written directory. If that invocation collects nothing and exits five, its absence of failures is not evidence that the separate suite passed.

Pytest explicitly distinguishes exit five from successful collected tests, and directory arguments constrain collection. [Official exit semantics](https://docs.pytest.org/en/stable/reference/exit-codes.html), [official selection semantics](https://docs.pytest.org/en/stable/how-to/usage.html#specifying-which-tests-to-run).

Do not globally redefine exit five as a defect. Preserve a `no_tests` observation and require policy to distinguish a genuinely non-applicable check from an unresolved testing obligation. An explicit project-defined exit policy also needs its declared meaning preserved rather than silently relabeled as executed-test coverage.

### W4 — The host's completion sentence overstates even correct verdicts

For any write turn without an acceptance report, `appendEvidenceSummary` renders `verdictSentence`. Every `/done` result receives:

> Verified by evidence: the build and the tests were both measured green after the final edit.

[Renderer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/change_evidence.go#L324-L367).

But policy deliberately permits:

- documentation writes without build/test obligations;
- other-language writes with `/test_run`, not the Go build/test pair.

[Obligation rules](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/coder_safety.mg#L464-L481), [verification derivation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/coder_safety.mg#L666-L678).

The existing `aDocumentIsDoneOnExecution` test explicitly establishes the intended documentation completion path. It was inspected, not run. [Test definition](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/test_run_gate_test.go#L151-L160).

Thus the verdict may be correct while its host-authored explanation invents measurements. This does not depend on model hallucination. A stage such as `artifact_changed` can even appear beside the stronger build-and-tests sentence. The statement is conditional on reaching `/done` and having a write with no acceptance report; it is not emitted for every task.

Render the actual completed obligations, explicitly distinguishing "not required" from "passed," and retain the separate caller-acceptance report path.

### Proposed minimum contract

Freeze a verification plan before settling the turn:

- applicable gate ID, source/configuration identity and required/advisory disposition;
- expanded command, canonical working directory and scope;
- affected files or declared obligation coverage;
- execution outcome: passed, failed, unavailable, canceled, indeterminate, no-tests, not-applicable;
- source revision and evidence handle;
- any authorized replacement relationship between an existing tool receipt and a planned gate.

The host measures these fields. Mangle derives whether each required obligation is satisfied and what remains. A model's claim that an unrelated test is sufficient cannot create that relationship.

Preserve these invariants:

1. Discovery availability cannot shrink the required-check denominator.
2. An existing run suppresses a planned check only when it satisfies the same obligation under an explicit matching rule.
3. Partial execution publishes all known outcomes, including failures before unavailability.
4. Later success supersedes an earlier failure only for the same obligation and applicable revision.
5. No-tests remains distinct from positive execution coverage.
6. Human-facing claims are projections of the settled evidence, not a generic sentence attached to `/done`.

Keep unknowns honest. Do not install missing toolchains, broaden execution authority, waive checks or run arbitrary replacement commands merely to obtain green status. Matching a command is a useful first reference rule, not proof of semantic sufficiency: preserve the caller/project's declared contract and the existing acceptance mechanism.

### Proposed verification, not executed results

| Fixture | Required result |
|---|---|
| Unrelated passing filtered run precedes forcing | Required workspace checks still owed. |
| Equivalent applicable run precedes forcing | Reuse without redundant execution. |
| One declared gate passes; another is unavailable | Passing subset retained, missing obligation explicit; no complete aggregate. |
| Earlier gate fails; later executable cannot start | Failure and unavailable outcome both survive. |
| Later unrelated process passes after that partial run | Cannot clear the failed or missing obligation. |
| Initial workspace failure, passing repair-tool receipt, unavailable workspace recheck | Recheck cannot reuse the tool receipt as an unqualified workspace pass; prior failure and new unavailability remain distinct. |
| Python written directory collects no tests | No-tests classification, not positive coverage. |
| Tests live outside written directory | Valid project test scope remains reachable. |
| Two written languages, only one covered; explicit cross-language check control | Missing coverage remains visible in the first case; legitimate shared coverage is retained in the control. |
| All required checks pass | Existing successful behavior preserved. |
| Documentation-only completion | No invented build/test execution claim. |
| Non-Go test-only completion | Names actual test evidence without claiming a build. |
| Caller acceptance present | Existing acceptance-report semantics preserved. |

Exercise the composed route—typed execution receipts → post-edit scheduler → workspace discovery/runner → kernel closure → final rendering—not only direct helper tests. Existing [workspace tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/workspace_test_run_test.go) demonstrate direct helper success, failure and unavailable behavior; those inspected fixtures do not establish the pre-existing-receipt or mixed-availability contracts above. This is not a claim that every other test in the repository was searched or run.

### Cost and research discipline

Compare three alternatives at identical tasks, revisions and authority:

1. Current aggregate receipt.
2. Always execute the complete applicable workspace plan; simplest correctness reference.
3. Obligation-preserving plan with sound reuse of matching receipts.

Measure false completion, false rejection, unresolved obligations, actual regression detection, test CPU/wall time, discovery cost, retained receipt bytes, model repair calls, all token usage and human recovery. More formerly accepted tasks may correctly become unverified; that is not automatically a capability improvement unless recovery is also useful. Include all attempted and canceled checks, and do not subtract failed paid model requests from the cost denominator. Provider cache-read/write and reasoning subcounts must remain non-additive when already included in token totals.

Gate/node receipts can be bounded metadata with full output behind recoverable handles. Avoid adding every log to active model context. Matching and provenance have costs; require measured end-to-end savings before claiming that richer reuse beats the simpler reference. A conservative initial matcher can trade additional test execution for easier qualification; a more selective matcher needs explicit dependency and invalidation evidence.

Regression-test-selection research supports retaining dependency evidence when choosing fewer tests; it does not justify accepting any passing subset. Ekstazi's dynamic file-dependency approach is a relevant precedent, not a performance guarantee for CodeNERD's polyglot runtime. [Primary paper](https://mir.cs.illinois.edu/marinov/publications/GligoricETAL15PracticalRTS.pdf).

**Checkable exit:** every applicable required verification obligation has an explicit current disposition, unrelated success cannot erase missing or failed evidence, and the final report claims only measurements the host actually possesses.
