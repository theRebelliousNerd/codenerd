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

## As built (2026-10-28)

- **Identity is package+test end to end.**
  - `failing_test`, `turn_failing_test`, `turn_test_failed_before`, `turn_importer_failing_test` and `turn_importer_failed_before` carry the package. The /test and importer rules in `coder_safety.mg` join on the pair.
  - `session.TestKey` (`failedTestKeys`) is what baseline attribution, `failuresAllPredate` and `partitionPreExisting` compare.
  - Pre-existing failures are reported as `pkg.TestName`.
  - Witness tests: `TestPartitionPreExisting_SameNameInAnotherPackageIsNew` and `TestTestGate_SameNameInAnotherPackageIsCharged`. The second runs on the kernel: an old `example/a.TestLoad` failure no longer lets a new `example/b.TestLoad` failure pass the gate.
- **The wrapper.** `syncTestGateFacts` now calls `assertTurnTestFacts` instead of an inline copy of its loop, so the function has its production caller and nothing is asserted twice.
- **Context relevance from test failures.** `context_compilation.mg` read an unscoped `failing_test` that no producer asserts. It now reads the turn's own failure sites (`turn_test_failure_at`, `turn_test_build_failure`) and the source file each failing test file covers (`test_file_for`).
- **Deferred:** the stronger causal check, where an identical package+test that failed before and fails now with a different defect still needs evidence beyond identity.
