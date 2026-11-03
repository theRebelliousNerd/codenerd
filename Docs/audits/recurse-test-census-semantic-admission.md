---
doc-class: deep-dive
subsystem: campaign-recurse-evidence
implementation-status: proposed-refinement
last-verified: 2026-11-03
verified-against: a0edfea6e82cf3f5e99c0663c39aef5c9725105a
verification-method: static-source-inspection
---

# Recurse counts commented test text as measured improvement

## Decision and evidence boundary

Give recurse's test-growth metric a syntax-aware, configuration-bound test inventory before using it as affirmative improvement evidence. The current Go metric counts raw text matching a function-name regex in any admitted `*_test.go` file. A function-shaped line inside a block comment therefore raises `/tests`, although it declares no function and introduces no runnable test. Existing real tests can still provide the separate completed-test fact. Under the outer ratchet's remaining conditions, that combination can derive `/keep` for a test-growth angle. [Metric producer][metric] [Ratchet][ratchet] [Angles][angles]

The exact finding is **incorrect test-growth evidence at the deterministic outer ratchet**, not proof that every accepted change is useful or that test counts could ever establish behavioral quality alone. A syntax-aware count remains a proxy. Stronger claims need evidence that the added tests are collected, run and cover the intended behavior.

This is a conditional source trace, not an executed reproduction or observed production incident. No repository program, build, test, gate, model call, inference run or benchmark was executed. No implementation was changed. Root, campaign and relevant architecture guidance, current source and neighboring audits were read. Initial main was `714f27f88fbbe40779ef553591366614f8e07399`; the later comparison to the pin above changed only `Docs/audits/ASTRA_QUEUE.md`, leaving this source unchanged. [Root guidance][guidance] [Campaign guidance][campaign-guidance] [Revision comparison][compare]

## Intended obligation and existing controls

The mandatory improvement atom requires stabilize to add tests that pin behavior, harden to exercise uncovered error paths or edge cases, and extend to add a capability with a proving test. A comment-only change does **not** fulfill those instructions. A compliant worker, review or independent acceptance requirement should reject it. The defect is that the outer metric does not mechanically establish even the narrower fact its comment promises: that test functions were added. [Improvement instructions][prompt]

The witness below preserves the actual tests and their behavior. It needs no deleted test, changed assertion, skipped existing test, altered configuration, missing executable, suppressed failure, stale receipt, truncated output, wrong package selection or forged result. The ordinary gate executes the correct node and its existing real tests. The added text is only misclassified by the separate source census.

All other controls remain authoritative: ordinary permissions, write leases, project-forbidden paths, real build/lint/test verdicts, coverage guards, removal guards, cancellation, compensation and any independent review. Conditional acceptance requires those stages to permit the candidate to reach the ratchet. This report does not claim they always will.

## Producer to consumer trace

### Raw text becomes a test count

`goTestFunc` matches lines beginning with `func `, followed by a `Test` or `Fuzz` name and an opening parenthesis. `testsIn` first checks only the Go filename suffix, then increments for every regex match except the name `TestMain`. It does not tokenize or parse the file, exclude comments or strings, or bind matches to function declarations. [Regex and Go branch][metric]

`CountTests` walks admitted source files across the workspace and sums those values. The walker refreshes membership and reads the raw bytes before calling the counter. Membership proves that a path belongs to the workspace, not that matching bytes declare a test. [Census][census] [Source reads][reads]

Go's language specification makes a general comment lexical whitespace. Its test loader parses source and selects actual top-level function declarations, rather than scanning text in comments. Consequently the same bytes can increase codeNERD's metric without adding anything to Go's test inventory. [Go comments](https://go.dev/ref/spec#Comments) [Go test loader](https://go.dev/src/cmd/go/internal/load/test.go#L732)

### Fresh execution does not repair the census

`recurseRun.metrics` stores the workspace-wide `CountTests` value under `MetricTests`. The loop measures it before the attempt and again after changed files have triggered fresh own-node checks and applicable workspace ratchet checks. [Metric bridge][metrics-bridge] [Before measurement][before] [After measurement][after]

A fresh completed Test result with no known no-tests disposition sets `in.Tested`. This is useful evidence that a test command ran; it does not compare the source-count delta with collected or executed test identities. In this witness an existing real test executes normally, so retaining the recently added no-tests guard is both correct and insufficient to distinguish the candidate. [Tested projection][tested]

Mangle assigns `/tests` growth to `/stabilize`, `/harden` and `/extend`. A positive metric delta derives improvement; improvement plus the tested fact derives cycle success. Changed content with success, no worse gate, no forbidden write and no guarded metric regression can derive keep. The keep branch commits the changed paths and increments both Kept and Improved. [Angles][angles] [Ratchet][ratchet] [Adoption][adoption]

The policy is faithfully consuming the host's assertion. The repair belongs in evidence acquisition and its explicit contract, rather than weakening or replacing Mangle's authority.

## Conditional witness

Use an admitted ordinary **non-mission** recurse run in a healthy Go module:

- Node `pkg` contains production code and at least one unchanged, genuinely executed passing test.
- Normal Go build, node test and vet gates are available. No extra owner check rejects this particular comment addition.
- Select a pass whose angle uses `/tests` growth.
- A new file `pkg/census_example_test.go` is within the admitted node, included by workspace membership, and not owner-forbidden.
- Existing test files, production behavior, build settings and commands remain unchanged.

The candidate adds only this file:

```go
package pkg

/*
Illustrative example only.
func TestNeverRuns(t *testing.T) {}
*/
```

The function-shaped line must begin at column one, matching the inspected regex. The name and `testing.T` text are inside a comment; no testing import or real function is declared. This is proposed fixture content, not code executed in this audit.

Static consequence:

1. The filename passes `isTestFile`, and the raw function-shaped line adds one to `CountTests`.
2. No actual test is added. Normal Go parsing and test discovery ignore the comment.
3. Existing genuine tests still run. Their normal result can supply `Tested`; this is not an empty-test process.
4. Test-source files are excluded from `SourceLines`, and the candidate changes no production statements. A witness with unchanged measured coverage and no other regressions is therefore consistent with the inspected code.
5. The outer inputs can be `Changed=true`, tests increasing by one, `Tested=true`, no gate regression and no forbidden path. Those inputs permit keep on the relevant angle.

The exact unconditional source finding is step 1: comment text is counted as a test. Steps 3–5 are a conditional outer-loop composition. No claim is made that an actual model would choose this patch, that a reviewer would miss its lack of value, or that the complete campaign would accept it.

A comment example in an existing test file supplies the same raw-count error, but it is deliberately not the primary write witness because mission mode protects existing verification inputs.

## Why adjacent protections do not establish test growth

**Removal guard.** The session guard compares actual test declarations before and after writes and reports missing identities. Its parser walks AST function declarations, so commented text is not a surviving or newly declared test there. The primary witness removes no test at all. The guard can correctly report no removal while the unrelated regex census incorrectly reports growth. Preserve that guard rather than interpreting this finding as a removal bypass. [Removal contract][removal] [AST inventory][ast-inventory]

**Mission protection.** Mission preparation captures existing verification inputs. Existing Go test files are among the protected input set; the write guard marks pinned paths protected, and the outer mission path also treats changed pinned inputs as forbidden. An edit to an existing protected test file should therefore be refused. The primary witness is ordinary recurse and makes no claim about defeating those protections. A new file's admission in a particular mission, and mission-level evidence, require separate qualification. [Mission capture][mission-capture] [Verification inputs][verification-inputs] [Mission write guard][mission-write] [Outer mission guard][mission-outer]

**Campaign acceptance.** The attempt adapter declares a command witness when the attempt supplies a check. Its source explicitly distinguishes improvements, which have no single acceptance command and rely on later metrics. This does not remove the worker's obligations or any independent task review; it explains why outer metric correctness remains necessary. [Attempt contract][attempt]

**Known empty collection.** The repaired no-tests join prevents known empty test invocations from supplying affirmative evidence. Here a real test runs in the correct package, so the problem is not collection status for the whole command. It is the absence of a correspondence between the alleged newly added test and the actual test inventory. [Existing no-tests audit][prior-empty]

## Prior work and novelty boundary

The portable-crucible proposal already warns that static counts are proxies and calls for negative controls involving more counted tests without relevant assertions. Credit that broader proposal. This report supplies a narrower source-derived conformance failure: counted material is not a function at all, using a valid Go comment and without changing the runner or weakening an existing test. [Portable proposal][portable]

Other neighboring findings address different joins:

- Workspace-test scheduling concerned a missing fresh test producer and now records its repair. This witness uses a real fresh own-node test. [Workspace-test audit][prior-workspace]
- Cargo default-member applicability concerns a command that does not select the changed package. The witness here selects the correct Go package. [Cargo applicability audit][prior-cargo]
- Test-evidence capability concerns preserving package-qualified identities in execution and baseline attribution. This finding arises earlier in a separate source metric, before any test event for the alleged addition could exist. [Test identity audit][prior-identity]

The bounded GitHub audit search returned no matches for `CountTests`, `testsIn`, `test-shaped` or `block comment`. Searches for test-count terms returned the neighboring reports above. Search results were indexed at `cd06a6336b3a311b6723c8c11a572526d8c9b8f7`; important source and audit documents were read directly at newer main. This is a documented search boundary, not proof of novelty across every historical branch or private discussion.

The authored metric fixture covers several language conventions and explicitly includes a testdata Go fixture in the count. It has no comment/string exclusion assertion. It was read, not run. [Authored metric fixture][fixture]

## Proposed contract and unrun evaluation

A bounded first correction can use Go syntax to identify real test declarations and report parse failures as incomplete evidence. Do not replace this regex with a larger regex intended to parse comments, strings, declarations and build constraints.

Separate three observations:

1. **Declared test inventory:** actual definitions with package, file and name identity.
2. **Applicable inventory:** which definitions belong to the admitted toolchain, build configuration and invocation.
3. **Executed evidence:** which applicable tests ran, skipped or failed, and what behavior their assertions establish.

Syntax-aware declaration counting addresses the exact comment witness, but does not establish activation, execution or behavioral value. Preserve those boundaries in names, receipts and policy inputs. Use host-produced facts and let Mangle decide what an angle must demonstrate. Where an ecosystem adapter cannot establish a claimed inventory, retain unknown/incomplete capability instead of promoting raw matches to authoritative growth.

Proposed controls, all unrun:

| Case | Required observation |
|---|---|
| Primary block-comment addition with unchanged real tests | No test-growth credit and no keep justified solely by that alleged growth. |
| Equivalent text in a raw string | No declared test added. |
| Real new Go test covering a new behavior | Declared identity retained; normal applicable execution and acceptance remain possible. |
| Newly declared but build-excluded test | Declaration and active execution are distinguished. |
| Existing test unchanged, skipped or not selected | Existing process activity cannot be mistaken for execution of the new identity. |
| Existing test removed or weakened | Preserve removal, oracle and independent review safeguards. |
| Malformed source or unreadable file | No complete-inventory claim from partial bytes. |
| Mission edit to a pinned test | Existing protection continues to refuse the write. |
| Python, JS/TS and Rust adapters | Independently qualify comments, strings, disabled tests and runner-specific discovery; do not infer portability from the Go fix. |
| CLI and chat ordinary recurse | Exercise the shared production loop, not just a standalone counter or hand-built ratchet facts. |

Measure false growth credit, missed real tests, inventory completeness, correct rejection/retention through the loop, and the cost of parsing and identity tracking. Hash or reuse only under a matching source and build context. Added parsing, discovery or event retention has real CPU, I/O and storage costs; runtime discovery can execute project code and needs its own authorization.

Completion means the host cannot turn commented test-looking text into affirmative test-growth evidence, while useful real test additions still traverse normal checks. It does not mean a larger count alone proves a behavior improvement.

## Pinned references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/agents.md
[compare]: https://github.com/theRebelliousNerd/codenerd/compare/714f27f88fbbe40779ef553591366614f8e07399...a0edfea6e82cf3f5e99c0663c39aef5c9725105a
[metric]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/gates/metrics.go#L17-L85
[census]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/gates/metrics.go#L88-L95
[reads]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/gates/metrics.go#L121-L168
[metrics-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_cycle.go#L1575-L1604
[before]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_cycle.go#L1230-L1242
[after]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_cycle.go#L1324-L1355
[tested]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_cycle.go#L1421-L1439
[angles]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/core/defaults/policy/recurse.mg#L356-L398
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/core/defaults/policy/recurse.mg#L299-L329
[adoption]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_cycle.go#L1481-L1507
[prompt]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/prompt/atoms/campaign/recurse_attempt.yaml#L41-L58
[removal]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/session/test_removal_guard.go#L16-L49
[ast-inventory]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/session/test_removal_guard.go#L359-L396
[mission-capture]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_mission.go#L212-L237
[verification-inputs]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/evidence/change.go#L379-L423
[mission-write]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_mission_scope.go#L13-L74
[mission-outer]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_cycle.go#L1357-L1396
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/campaign/recurse_attempt.go#L42-L55
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-empty]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/Docs/audits/recurse-no-tests-evidence-continuity.md
[prior-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/Docs/audits/recurse-workspace-test-witness.md
[prior-cargo]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/Docs/audits/recurse-cargo-default-member-evidence.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/Docs/audits/test-evidence-capability.md
[fixture]: https://github.com/theRebelliousNerd/codenerd/blob/a0edfea6e82cf3f5e99c0663c39aef5c9725105a/internal/gates/metrics_test.go#L5-L32

