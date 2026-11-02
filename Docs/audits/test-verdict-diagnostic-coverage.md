# Test verdicts are not diagnostic records: a mixed-failure baseline exemption gap

## Status and scope

**Supported by static source tracing; not reproduced by execution.** At revision `7224e77959cfd0be52ad46010d9c903d3fa7bfb7`, a newly failing Go test that produces no recognized located diagnostic or panic can be absent from the failure-identity relation used by the turn's `/test` gate. If another test emits a recognized diagnostic and also failed in the baseline, that other row can satisfy the gate's existence guard while every represented failure is baseline-exempt. The gate can consequently derive `/passing` despite a new failed terminal test verdict.

This is a **partial-coverage problem**, not “all failures without logs pass.” A failing measurement with zero `failing_test` rows is explicitly rejected. The gap needs a represented, baseline-associated failure to coexist with an unrepresented new failure. Package-qualified identity is already present at this revision; this finding does not depend on the earlier package/name collision.

The result is a false acceptance of the derived test responsibility gate, and a missed opportunity to enter automatic repair. It is not evidence of an observed incident, an unconditional successful turn, a permission bypass, or deletion of the raw failure. Independent build, coverage, vet, test-retention, pinning and acceptance obligations can still block completion. No repository code was changed and no tests, reproductions or benchmarks were run for this review.

## 1. The representation contract

A terminal verdict and a human-oriented diagnostic are different observations:

- A verdict establishes that a particular package-qualified test failed.
- A diagnostic can explain the failure and locate a useful repair site.
- Failure identity must not depend on whether the test emitted an explanation.

Go's official API documents `T.Fail` as marking the test failed while continuing execution; `T.Error` combines logging with failure. A test can therefore legitimately fail without a located log. The test2json format likewise separates terminal `fail` actions from `output` actions and supplies package/test identity independently. This is ordinary tool output, not malformed input. [Go testing API](https://pkg.go.dev/testing#T.Fail), [test2json protocol](https://pkg.go.dev/cmd/test2json).

For a completed measurement, let:

```text
S = {(package, test) | terminal test verdict is fail}
D = {(package, test) | a failure identity is emitted for attribution}
B = {(package, test) | a compatible baseline verdict is fail}
```

The minimum coverage invariant is `S ⊆ D`. Diagnostic-bearing identities may include additional package-level failures, but no known failed test should disappear because its diagnostic is absent. The new-failure decision should account for `S \ B`, not merely a diagnostic-selected subset `D \ B`.

The current fail-closed guard checks whether `D` is empty. That is not a proof that `D` covers `S`. A nonempty proper subset is precisely the problematic case.

## 2. What the parser preserves, and what the fact producer drops

The JSON parser calls `addVerdict` for terminal test events and stores a test's status independently of its output. Materialization retains both fields, then performs a separate diagnostic-extraction pass. A test needs no located log to obtain `StatusFail`. [Parser terminal verdicts][parser], [result materialization][materialize].

The diagnostic extractor visits failed tests, but `testFailures` creates records from recognized file/line-shaped output or from panic extraction. It does not create a fallback record merely because a test's status is failed. A plain `t.Fail()` test with only runner markers therefore contributes a failed test status but no diagnostic `Failure`. Package-level panic fallback does not supply an ordinary status-only test record. [Diagnostic extraction][diagnostics].

The fact producer exposes the divergence:

1. `Facts()` emits `test_case(Package, Test, /fail, Elapsed)` from test statuses.
2. It emits `failing_test(Package, Test, Message)` from `failingTests()`.
3. `failingTests()` begins with `firstFailures(r.Failures)`, adds package-level build-failure records, and never enumerates all failed test statuses.

Thus both failed `test_case` rows survive, while only the diagnostically represented failure becomes a `failing_test` row. The actual implementation is package-qualified; some nearby comments still describe the former schema and must not be taken as current behavior. [Fact emission][facts], [failure identity projection][identities].

This is not simply an inability to locate a source file. Panic extraction can emit a failure even without a workspace frame. The relevant condition is **no recognized diagnostic or panic record**, not merely “no usable location.”

## 3. Static witness

Use one ordinary, untagged Go package, a complete measurement and a conclusive baseline overlay. Both tests already exist, so neither new-test discovery nor build-tag selection is necessary:

| Qualified test | Baseline terminal verdict | Head terminal verdict | Head diagnostic extraction |
|---|---|---|---|
| `example/p::TestExisting` | fail | fail | Located log from `t.Error("known failure")` |
| `example/p::TestSilent` | pass | fail | No recognized diagnostic; calls `t.Fail()` |

For example, an edit can change an existing condition inside `TestSilent` so its already-present `t.Fail()` executes. Alternatively an implementation change can cause an existing silent assertion to fail. These are hypothetical witness constructions, not an executed fixture. Use stable tests, known preimages, sufficient budget, a real kernel, and an unchanged workspace during the measurement. A new head panic, unknown baseline, unrelated new logged failure or independent gate failure is outside this minimal witness.

The structured head result contains:

```text
test_case("example/p", "TestExisting", /fail, ...)
test_case("example/p", "TestSilent",   /fail, ...)
failing_test("example/p", "TestExisting", "known failure")
# No failing_test row for TestSilent.
```

Baseline attribution independently uses `failedTestKeys`, which enumerates every failed test status, not diagnostic records. It reruns the failing top-level selections against the overlay and records the baseline failed keys. With the table above it correctly records only `example/p::TestExisting`. It does not convert the raw failed outcome to a pass. [Complete status-key extraction][keys], [baseline attribution][baseline].

After turn scoping, the relevant policy inputs are:

```text
turn_test_measured(T, /failing)
turn_test_case(T, "example/p", "TestExisting", /fail, ...)
turn_test_case(T, "example/p", "TestSilent",   /fail, ...)
turn_failing_test(T, "example/p", "TestExisting", "known failure")
turn_test_failed_before(T, "example/p", "TestExisting")
```

Turn scoping renames and prefixes the emitted facts; it does not reconstruct missing identities from failed status rows. Synchronization preserves the raw failing measurement and asserts the baseline keys. [Turn-scoped facts][turnfacts], [gate synchronization][sync].

The policy derives an own failure only from a `turn_failing_test` key not present in the baseline. It never consults `turn_test_case` for this attribution. The old row establishes `turn_has_failing_test`; it produces no own-new-failure because its key is in the baseline. The missing new row cannot produce an own failure. With no importer blocker, the rule at lines 423–427 derives `turn_gate(T, /test, /passing)`. The no-row failure rule at lines 411–413 does not fire because one row exists. [Exact attribution and gate rules][policy].

In set terms:

```text
S = {Existing, Silent}
D = {Existing}
B = {Existing}

S \ B = {Silent}   # A real new failed verdict.
D \ B = {}         # What the gate sees.
D ≠ {}              # Its empty-projection safeguard is satisfied.
```

All identities above are package-qualified; abbreviations in this last display are only for readability.

## 4. Reachability through repair, importers and closure

### Ordinary post-edit test round

The host invokes `go test -json`, parses its output, and retains a conclusive command failure as `VerifyFailed`. The own-package gate then attributes the baseline. This is the normal host verification route, not a hand-injected fact scenario. [Host test execution][runner], [own-test measurement][measure].

`verifyAndRepairTests` stores the result and asks `testGateRed` whether policy charges the failure to the turn. With a kernel, this helper synchronizes the facts and asks the derived `/test` gate; it does not use the raw exit as its sole authority. If the gate is not red, the repair function returns before its repair loop. With no kernel, the helper falls back to the raw failed outcome and remains red. [Repair early return][repair], [kernel/fallback decision][gatehelpers].

### Why importers do not automatically catch this witness

`failuresAllPredate` examines the complete status-derived head keys. It sees `TestSilent` absent from the baseline and returns false. Therefore `gateTests` takes the conclusive-own-failure branch, clears the current importer measurement and returns without running importers. The scheduling layer correctly recognizes that not all failures predate the edit; the policy layer later uses a smaller set and disagrees. No importer blocker is required to be present in this witness. [All-predate check][predate], [importer scheduling][importers].

This is a particularly useful consistency boundary: two components compare the same conceptual failure set through different projections. The review does not claim that importer execution itself has been independently reproduced with this case.

### Closure is not necessarily a rescue

Closure's `closingTestGate` queries the same derived gate. For `/fix`, the additional final verification path first calls `gateTests` again. If the own raw verdict is failed and `failuresAllPredate` is false, `finalFixTests` returns that own result before launching the full-suite command. This witness takes that branch. The returned result retains baseline attribution, becomes `FinalTestCheck`, and is then projected through the same gate synchronization logic. Merely having a final-suite function does not establish a full-suite recheck for this branch. [Final fix verification][finalfix], [closure routing][closure], [final-result preference][sync].

This still does not prove every surrounding completion obligation accepts. It establishes that the inspected final-test path does not inherently restore the missing failure identity.

## 5. Counterexamples and limits

| Variation | Expected result from inspected logic |
|---|---|
| Only `TestSilent` fails, with no represented failure anywhere | The zero-row safeguard rejects the failing measurement. |
| New test uses `t.Error` or another recognized located log | It acquires a failure identity; absent a matching baseline key, the gate is red. |
| New failure emits a recognized panic | Panic extraction can supply its identity; the simple silent witness no longer applies. |
| Baseline cannot run conclusively or preimages are unknown | No matching exemption is established for the old diagnostic; that represented failure remains chargeable. |
| Both test keys already fail in a valid baseline | Exemption may be intended; this is not a newly failing-key witness. |
| Executor has no kernel | Raw failure fallback remains red. |
| Another new diagnostic-bearing own failure is present | It supplies a chargeable key and makes the gate red. |
| A retained importer blocker is present in another path | It can prevent derived passing; the traced fresh mixed-own-failure path clears importer evidence. |
| Other build/coverage/acceptance obligations fail | Overall turn can still be rejected even if this gate is misclassified. |

The parser still knows the new test failed. Raw command outcome, failed test-case facts and status-based summary counts remain evidence available to other consumers. This report therefore does not claim that all output is green or that all failure data has been erased.

Baseline equality of a test key is itself only association, not proof of identical causal failure. That broader limitation is separate from the narrower defect here: a newly failed key is available and is omitted from the authoritative attribution relation.

## 6. Duplicate review and existing coverage

A focused comparison found this to be a distinct residual of the evidence boundary, not the already-audited package-key collision:

- [`test-evidence-capability.md`][prior] describes package/name identity loss and explicitly assumes named diagnostic rows in its witness. Its note correctly observes that a status-only fixture with no failure rows can fail closed. The mixed case here has a nonempty but incomplete projection, so that safeguard does not suffice. The earlier audit's desired status-based failure-set contract is directly relevant, but its concrete collision mechanism is not being rediscovered.
- [Baseline exemption/tagged obligations][tagbaseline] concerns skipped or unmeasured tagged-test obligations after baseline acceptance. Both tests here already ran and have failed terminal verdicts.
- [Tagged constraint witness][tagconstraint] concerns selecting a satisfying build configuration. The witness here uses ordinary untagged tests.
- Targeted searches of audit/QA material for `firstFailures`, `t.Fail`, baseline and diagnostic terms did not identify a prior analysis of this partial status-to-identity coverage case. This is a focused duplicate check, not a claim to have exhaustively read repository history.

Existing tests exercise useful adjacent behavior but do not establish this invariant:

- The mixed baseline regression uses a logged new `t.Fatalf` failure. [Mixed baseline test][baselinetest].
- Policy fixtures cover an old represented failure, two represented old/new failures, and a failing measurement with zero represented rows. The missing case is an old represented row plus a new failed `turn_test_case` without a `turn_failing_test`. [Policy fixtures and assertions][policytests].
- Fact tests cover logged mixed pass/fail output and build failure handling; those fixtures do not prove that each status-failed test receives a failure identity. [Fact tests][facttests].

No test results are asserted here; these are observations from reading test source.

## 7. Proposed contract and bounded remedies

A minimal repair should make **failed identity total over failed terminal verdicts** while keeping diagnostic richness optional.

One producer-side option is to enumerate failed tests by qualified key and emit exactly one attribution identity for every failed key. Retain the first rich diagnostic where available; otherwise use an explicit “failed without diagnostic” message or a separate identity fact. Do not invent a source location or manufacture `test_failure_at` data. Preserve package-level build and panic evidence conservatively, and deduplicate by package plus full test name.

A policy-side alternative is to derive test failure identities directly from the authoritative failed `turn_test_case` rows, then union appropriate package-level failure evidence. This requires the importer channel to preserve the same completeness: its current fact synchronization selects failure-identity rows rather than automatically importing all test-case rows. Updating only the own gate would leave representation contracts inconsistent. [Importer fact adapter][importerfacts].

Whichever approach is chosen:

1. Keep raw run verdict distinct from attribution/exemption.
2. Preserve full package and subtest identity; top-level names are selectors, not equality keys.
3. Require a conclusive compatible baseline before exemption.
4. Treat unknown identity/measurement completeness conservatively.
5. Keep the explicit projection predicates used for negation; this issue is not a reason to replace them with unsafe wildcard negation.
6. Ensure one observation cannot simultaneously establish contradictory passing and failing gate results.

Saltzer and Schroeder's fail-safe-default and complete-mediation principles provide a useful design analogy: a nonempty sample of checked objects is not evidence that every relevant object was checked. Here the objects are failed test identities, not access requests. This analogy motivates a completeness invariant; it does not establish a security exploit classification. [Original primary paper, §I.A.3](https://web.mit.edu/Saltzer/www/publications/protection/Basic.html).

## 8. Costs, risks and fair evaluations — all unrun

**Expected implementation cost:** enumerating test statuses adds work linear in the number of parsed tests and storage linear in distinct failed identities, with hash-based deduplication if used. It need not launch additional Go processes or increase model calls. Existing status facts already carry the identities. A policy-only remedy changes relation joins and fact propagation instead; its engine cost should be measured, not assumed negligible.

**Behavioral cost:** more correct red gates can cause extra repair attempts or expose previously hidden failures. That is expected, but diagnostics-free failures offer weaker repair localization. A generic failure identity should not be mistaken for a precise source attribution. Parent tests that fail because a subtest failed need deliberate identity handling; both terminal failures are observations, and baseline comparisons must remain consistent rather than silently discarding parents.

**Evaluation plan, not execution:**

| Layer | Proposed evaluation | Acceptance criterion |
|---|---|---|
| Parser/facts | Complete JSON fixtures for `Fail`, `FailNow`, logged `Error`, panic, package build failure and mixed log/no-log cases | Every terminal failed test key appears in the identity relation; absent locations remain absent. |
| Exact regression | Baseline old logged failure plus head old logged and new silent failure | Raw outcome remains failed and derived test gate is failing, never passing. |
| Fair positive control | Both status-failed keys also fail in a conclusive baseline | Intended baseline exemption is preserved. |
| Fair negative controls | Only silent failure; absent baseline; new logged failure; no-kernel executor | Existing fail-closed behavior remains intact. |
| Identity | Equal test names in different packages; distinct full subtests; parent/child failures | No cross-package or top-level collision is introduced. |
| Kernel parity | Apply the same fact matrix to real and Cortex kernel adapters used by existing policy tests | Both derive identical, noncontradictory gate verdicts. |
| Integration | Trace host measurement → overlay baseline → importer scheduling → repair decision → closure, including `/fix` early final return | No boundary drops a status-failed key; raw outcome and responsibility remain distinct. |
| Metamorphic | Keep verdicts fixed while removing or replacing diagnostic text | Attribution identity and baseline membership do not change merely because explanatory output changes. |
| Robustness | Truncated/canceled output, unavailable baseline, malformed diagnostics and repeated events | Unknown evidence cannot become affirmative exemption; existing completion semantics remain explicit. |
| Cost | Compare equal-size streams across many passing tests and many failed tests, with/without rich logs | Report time, allocations and fact counts separately; do not use faster but incomplete projection as the correctness baseline. |

The smallest decisive regression spans the producer and policy rather than asserting only a hand-built policy row: otherwise a test can prove the consumer is correct while continuing to omit the producer's missing identity.

## Conclusion

At the pinned revision, baseline attribution reads the complete status-failure set, while the policy gate reads a diagnostic-selected failure set. The mixed witness makes that inconsistency observable in the static control flow: a genuinely new failed test survives as a failed verdict but is absent from attribution, and an old logged failure prevents the empty-set safeguard from activating. The repair target is evidence **coverage**, not more logging and not another package-key redesign.

[parser]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/testfacts/parse.go#L94-L172
[materialize]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/testfacts/materialize.go#L25-L73
[diagnostics]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/testfacts/failures.go#L43-L123
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/testfacts/facts.go#L49-L120
[identities]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/testfacts/facts.go#L145-L195
[keys]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/testfacts_helpers.go#L52-L96
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/test_baseline.go#L20-L95
[predate]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/test_baseline.go#L97-L120
[turnfacts]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/turn_test_facts.go#L38-L70
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/gate_facts.go#L48-L93
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/core/defaults/policy/coder_safety.mg#L335-L427
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/test_verify.go#L256-L318
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/build_verify.go#L557-L598
[repair]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/build_verify.go#L341-L398
[gatehelpers]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/gate_facts.go#L453-L483
[importers]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/build_verify.go#L516-L552
[finalfix]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/gate_memo.go#L202-L256
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/change_evidence.go#L74-L144
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/Docs/audits/test-evidence-capability.md#L61-L96
[tagbaseline]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/Docs/audits/baseline-exemption-tagged-test-obligations.md
[tagconstraint]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/Docs/audits/tagged-test-constraint-witness.md
[baselinetest]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/test_baseline_test.go#L184-L224
[policytests]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/shards/turn_gate_policy_test.go#L31-L126
[facttests]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/testfacts/facts_summary_test.go#L1-L109
[importerfacts]: https://github.com/theRebelliousNerd/codenerd/blob/7224e77959cfd0be52ad46010d9c903d3fa7bfb7/internal/session/importer_gate_facts.go#L90-L134

