# Tagged test obligations need a satisfying build configuration and an execution witness

## Decision and evidence boundary

At the inspected revision, the written tagged-test helper can run successfully without including the test file it is meant to verify. It collects positive tag names from a Boolean build constraint and enables all of them. That is not necessarily a satisfying assignment. A successful process exit then passes its check even when the requested named test never ran.

This is a source-derived, conditional witness at CodeNERD main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, rechecked unchanged on October 29, 2026 at 13:38 UTC. No tests, application execution, model inference, benchmarks, or local Git were run for this audit. It establishes a missing tagged-test proof, not an observed incident, an authorization bypass, or unconditional final `/done`. Other gates and acceptance verification can still block completion.

Preserve the existing build-exclusion, test-retention, pinning, baseline, package-identity, and memoization safeguards. The proposed refinement is to require both a supported configuration that includes the written test and a package-qualified execution receipt for it.

## The source path is live

The TUI mutation path requires a derived verification obligation and a configured verifier, then invokes `VerifyWithRetry`. Shared boot takes `cortex.TaskExecutor` and attaches it to that verifier; when delegation selects the inline route, the task executor clones the session executor and calls `ProcessWithIntent`. This connects the inner test gate to a production entry path, not merely a direct helper test. The outer TUI verifier can still reject the task independently: no accepted final chat verdict is proved here. [Mutation delegation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L534-L571), [shared executor](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L100-L106), [verifier wiring](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L168-L170), [inline execution](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L404-L441).

The `/test` round invokes `verifyAndRepairTests`, which calls `gateTests`. On a fresh own-test measurement, `measureOwnTests` splits runnable and wholly tag-gated packages, measures the default suite with or without coverage, attributes baseline failures, and calls `withWrittenTagGatedTests`. The witness below uses a runnable package and a passing default suite, so neither the compile-only branch nor baseline exemption explains the gap. [Round dispatch](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_tools.go#L458-L503), [test gate entry](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/build_verify.go#L338-L363), [fresh measurement](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/build_verify.go#L546-L587).

The helper proceeds only when the incoming verdict is `VerifyPassed`. For each written test file with discovered tags and top-level test names, it invokes `verifyTests` with `-count=1`, `-tags`, and an anchored `-run` expression. It returns a nonpassing targeted result, but after targeted passes it returns the original default verification, not an aggregate containing the targeted receipts. [Written tagged-test helper](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/tag_gated_packages.go#L205-L251).

The underlying tag collector parses the constraint, visits both sides of both `AndExpr` and `OrExpr`, and records tag names occurring outside negation. It does not evaluate the complete expression under the selected configuration. [Parsing and collection](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/tag_gated_packages.go#L94-L145).

## A body edit with an unchanged constraint is enough

Consider an authorized, successfully tracked edit to the body of an existing `pkg/choice_test.go`. Its valid header remains:

```text
//go:build (feature_a && !feature_b) || (!feature_a && feature_b)
```

The existing unique top-level `TestChoice(t *testing.T)` keeps its name. The body remains syntactically valid but contains an assertion that would fail if that test were executed. Assume:

- The edit uses a recognized tracked write path; tests are enabled and the `/test` round is actually owed and admitted.
- The toolchain and kernel are healthy, with no cancellation or concurrent edits during measurement.
- The package has ordinary Go source and passing default tests, with no custom `TestMain` behavior.
- Neither feature tag is effective in the default configuration, including project settings and `GOFLAGS`.
- No platform or filename constraint additionally excludes the target.
- No other file changes inclusion under these feature tags in a way that breaks compilation.
- Only this test body changes; its preimage is known. No test name is removed, no production file changes, and no relevant importer or other independent blocker intervenes.
- The own-test memo is fresh or misses. The argument does not depend on stale reuse.

The expression is exclusive-or:

| feature_a | feature_b | Target file included |
|---|---|---|
| false | false | No |
| true | false | Yes |
| false | true | Yes |
| true | true | No |

The default suite passes while excluding `choice_test.go`. Ordinary source keeps the package runnable rather than wholly tag-gated. The helper collects `feature_a` and `feature_b` and constructs the equivalent invocation:

```text
go test -json -count=1 -tags feature_a,feature_b -run ^(TestChoice)$ ./pkg
```

This is an argv description, not a command executed for the audit. With both tags true, both alternatives in the original constraint are false. The target is still excluded. Since no other selected test has that unique name, this otherwise healthy package has no matching test to run. [Package split](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/tag_gated_packages.go#L28-L53), [selection and invocation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/tag_gated_packages.go#L216-L251).

This no-match outcome need not fail the ordinary Go testing process. CodeNERD declares Go 1.27.3 in `go.mod`. At the corresponding immutable Go source revision, `runTests` starts with success, and `M.Run` warns about no tests; the special no-tests failure applies to testing the standard `testing` package itself. With no other failure, the normal path sets exit code zero and prints a pass. This is source corroboration, not verification of an installed runtime. [Declared Go version](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/go.mod#L1-L3), [Go no-tests handling and successful exit](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/testing/testing.go#L2601-L2645), [initial success state](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/testing/testing.go#L2710-L2712).

The verification runner maps a completed command with a nil error to `VerifyPassed`. `verifyTests` preserves the parsed result but creates its passing verification from that command outcome without requiring an event for every requested test. The tagged helper checks that verdict and returns the original default pass. The Go process success is legitimate for its selected test set; the unsupported step is treating it as proof of the helper's intended named-test obligation. Consequently the helper ran, but that obligation was not proved. [Runner outcome](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/verify_outcome.go#L134-L143), [outcome conversion](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/test_verify.go#L270-L327), [helper return](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/tag_gated_packages.go#L243-L251).

Within the inner gate, `syncTestGateFacts` publishes the returned `TestCheck` verdict and calls `assertTurnTestFacts`, whose source is that returned `TestCheck.Result`; successful targeted receipts discarded by the helper are not recovered there. Policy derives a passing `/test` gate from a passing measurement when no importer blocker exists. This supports the narrow inner-gate evidence claim, not acceptance by the outer verifier. [Gate synchronization](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/gate_facts.go#L47-L86), [parsed-result publication](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/turn_test_facts.go#L38-L44), [passing rule](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/coder_safety.mg#L436-L438).

## Existing defenses and limits of the claim

The inspected tests already exercise useful simple-tag behavior: the written-test helper test checks an integration-tagged failure and preserves plain or already-red results; package tests cover `integration && !race` and `(integration || smoke) && go1.21`. Enabling both positive names satisfies that latter disjunction; it does not test the exclusive-or case. No nonmonotone mutually exclusive fixture appears in these two inspected files. These tests were read, not run. [Helper tests](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/written_tag_gated_test_runs_test.go#L17-L86), [package and tag tests](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/tag_gated_packages_test.go#L10-L84).

The unchanged header matters. `RejectAddedBuildExclusion` rejects newly added constraints that move a file from included to excluded; it returns when no constraint was added. The witness does not disable or contradict this guard. Keeping the test name avoids relying on a test-deletion path. [Build-exclusion guard](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion.go#L16-L65).

The test-only edit is deliberate: `pinUnits` skips test paths when choosing changed production units. A production-code witness would need to account separately for pinning and its tagged-test safeguards; it should not be substituted without analysis. [Pinning unit selection](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/pin_gate.go#L186-L218).

Closure remeasurement calls `gateTests` again. Even a genuine fresh run repeats the same unsatisfying tag selection; simply bypassing the memo does not fix this witness. Conversely, a failing build, an actually selected failing test, a different effective environment, or an independent acceptance obligation can stop the turn. [Closure measurement](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/change_evidence.go#L133-L152).

This is distinct from the prior baseline-exemption audit: there the raw default failure prevents the helper from running before a later exemption. Here the default suite passes and the helper actually runs. It is also distinct from the memo-identity audit: this witness survives a fresh cache miss. Package-qualified identity remains necessary, but correctly identifying a nonexistent execution does not make it evidence. The inspected audit screen found no exact prior satisfying-assignment witness; that is a bounded review result, not an exhaustive novelty claim. [Baseline composition audit](https://github.com/theRebelliousNerd/codenerd/blob/23246584179fc6f489fc4bf9b665eaaef8c3374a/Docs/audits/baseline-exemption-tagged-test-obligations.md), [memoization identity audit](https://github.com/theRebelliousNerd/codenerd/blob/68ebef86024f41d9e7fe487202e566f0d5b43b1b/Docs/audits/test-gate-memoization-identity.md), [test-evidence identity audit](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/test-evidence-capability.md).

The active code-lane revisions were also checked: their `tag_gated_packages.go` content matches main. This is not a claim that all other files or runtime behavior are identical. [Lane a53cfc3](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/session/tag_gated_packages.go), [lane 80af186](https://github.com/theRebelliousNerd/codenerd/blob/80af1863d3a3aa46a722ac8b0c8a0985582d4995/internal/session/tag_gated_packages.go), [lane aa7988d](https://github.com/theRebelliousNerd/codenerd/blob/aa7988dec4c62116656ea722ff73b687a8509908/internal/session/tag_gated_packages.go).

## Proposed proof contract

Keep configuration validity, file inclusion, and observed execution separate.

First, prefer a project-declared, approved test configuration. Evaluate the entire parsed Boolean constraint against the actual effective configuration, not a union of positive names. Include toolchain, GOOS, GOARCH and architecture features, cgo, compiler, release tags, custom tags, applicable `GOFLAGS`, and filename constraints. Go already provides `constraint.Parse` and `Expr.Eval`; expression evaluation alone does not account for every file-selection rule. Never invent platform tags to simulate another executable environment. [Constraint API](https://pkg.go.dev/go/build/constraint), [Go build constraints and filename rules](https://pkg.go.dev/cmd/go#hdr-Build_constraints).

If project policy permits choosing custom tags, a bounded deterministic search may select a satisfying supported configuration. Otherwise report the obligation unresolved. Do not silently enumerate an unlimited configuration matrix, launch services, acquire credentials, or expand permissions to make a test runnable.

Second, retain a host-observed receipt keyed by package, full test name, source revision, and effective configuration, bound to the expected source file and its inclusion evidence. Require the intended named run and successful completion under that configuration. An alternative, mutually exclusive file in the same package can define the same test name, so a name event alone must not substitute for proving the intended definition was selected. A skip, no matching test, empty stream, canceled run, or malformed/incomplete evidence must not discharge this obligation. A named pass proves execution to that extent, not assertion quality or acceptance of the user's requested behavior.

Third, preserve the raw default outcome and targeted receipts as separate observations rather than replacing one with the other or dropping successful targeted evidence. Compact typed configuration/test receipts plus recallable full output are sufficient; this is not a requirement to duplicate every raw output in active context. Go should parse, validate, and report observations; Mangle should derive the obligation and whether its evidence discharges it. Keep baseline exemption, importer attribution, coverage, and completion decisions distinct. Compatible test obligations may share an invocation only if their individual configuration and execution identities remain recoverable.

## Fair evaluation before implementation claims

Compare three designs under matched environments, authorized services, timeouts, and resource budgets:

| Design | What the exclusive-or witness should establish |
|---|---|
| Current source behavior | Target remains excluded even though the targeted process can pass |
| Required named-event check only | Detects the missing execution but does not itself execute the target |
| Approved satisfying configuration plus named-event receipt | Includes and executes the target, or explicitly leaves the obligation unresolved |

Exercise all four truth-table rows; simple conjunctions; negative literals; disjunctions with mutually exclusive alternatives; an ordinary existing integration tag; and unsupported platform or filename constraints. Add controls for `TestMain` exit or skip, no matching tests, empty or malformed JSON, cancellation, duplicated names in different packages, same-name definitions in mutually exclusive files of one package, a fresh memo miss, and closure remeasurement. Keep exclusion headers pre-existing so the build-exclusion guard is not bypassed or mistaken for the issue being tested.

Capture actual argv and effective configuration, selected files, package-qualified named run/pass events, and the resulting Mangle obligation state. Process success alone is not the endpoint. Include a target whose assertion fails when genuinely selected, plus passing controls. Preserve regressions for existing baseline, package identity, and cache behavior.

Measure compile/list/startup overhead and intended test execution separately, alongside wall time, CPU, memory, I/O, database/fact volume, and diagnostic tokens. Configuration evaluation and receipt checking inherently require no model calls; downstream repair may add them. Grouping compatible invocations could save launches, but discovering a previously omitted test can increase total cost. No speedup, defect rate, resource saving, or end-to-end completion improvement is measured by this source audit.

