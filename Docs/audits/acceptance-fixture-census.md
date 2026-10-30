---
doc-class: deep-dive
subsystem: acceptance-evidence
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Acceptance fixture protection must not inherit indexing exclusions

## Conclusion

The acceptance transaction can fail to protect a fixture that its actual regression test reads. Its protected-input census applies workspace membership exclusions before selecting test files, fixtures and module files. Its stability snapshot applies the same exclusions. An existing explicit `world.ignore_patterns` exclusion can therefore remove a test fixture from both safeguards without preventing the test process from reading it.

Keep world/indexing exclusions intact for their intended consumers. Give acceptance a separate, explicit verification-input contract, or refuse to certify an unsupported input domain. A genuine passing test against changed expectations is not evidence that the original caller-reviewed behavior was repaired.

This is a conditional source-derived finding, not an executed reproduction or an observed production incident. It does not claim that ordinary tracked testdata is excluded by default, that the model bypasses write permissions, or that every affected session necessarily completes. No tests, application execution, model inference, training, benchmark or repository mutation accompanied this review.

## Source and existing contract

Main was resolved to `486db57ce490302c65d7f484e7f6070ee0e70862` on October 30, 2026. Root, core, session and evidence guidance were inspected. The evidence package explicitly requires pinned test files, fixtures, module files, executable and environment, with changed verification inputs requiring a newly reviewed contract. The session acceptance specification makes the same promise and distinguishes acceptance from merely executing a turn. These are the intended protection contracts, not an assumption that arbitrary software can be proved correct by tests. [Evidence guidance][guidance] [Acceptance specification][spec]

The existing test-gate memoization audit concerns reuse of mechanical measurements. The workspace receipt audit concerns invocation equivalence. The judge/evidence and verification-applicability reports concern downstream adjudication and freshness. This finding instead concerns which caller-owned acceptance inputs are protected before and after an actual new execution. The inspected `research-main-20261030-0211` branch is five documentation-only commits ahead of this main, adding effect recovery, expert-card scope/repair, knowledge-miss retirement and schema rollback reports; none addresses this fixture census. The available audit inventory and targeted searches revealed no exact duplicate; this is not a claim to have exhausted every historical discussion.

## Production chain

The direct action path attaches a supplied acceptance contract with `evidence.WithContract`. Session execution reads it before model tool execution, calls `evidence.Begin`, and retains the transaction. At final evidence closure it calls `Verify`; a verified report sets `ChangeStage` to `behavior_verified`, while a nonverified report returns a verification error. [Contract attachment][attach] [Transaction creation][begin-caller] [Closure consumer][closure]

`Begin` validates and copies the contract, resolves and hashes the Go executable, saves the environment, snapshots the workspace and calls `verificationInputs`. It additionally resolves and hashes each declared `TestFile`, verifies that file defines the selected test in the selected package, and executes baseline obligations. A reproducer must supply an actual failure; regression obligations must pass. A second snapshot must equal the baseline snapshot. Direct protection of the named test file is a real safeguard and is not the missing edge here. [Baseline admission][begin]

`verificationInputs` obtains `workspace.For(root)` and refreshes it. While walking, it first skips directories rejected by `IncludesDir` and files rejected by `Includes`. Only afterward does it select `_test.go`, module/workspace files and paths under `testdata`. Thus a fixture excluded from membership never reaches the protection map. [Protected-input census][census]

`workspace.For` loads the workspace's `world.ignore_patterns`. Explicit patterns apply even to tracked paths; only implicit defaults yield to tracked files. `Includes` checks these exclusions before its tracked/member acceptance. Consequently, an explicit exclusion that existed before the transaction can hide the fixture irrespective of its tracked status. [Configuration source][membership-config] [File admission][membership-file] [Tracked-file distinction][membership-patterns]

`Snapshot` uses the same membership filtering, with purpose-specific exceptions for `.nerd/config.json` and agent definitions. It hashes included file names, modes and contents, and rejects included nonregular files. It does not hash the excluded fixture. Refreshing the same correct membership policy does not fix an inappropriate evidence domain. [Snapshot filtering][snapshot]

`Verify` rebuilds the protected census, checks its size, reads and compares every originally protected file, and takes a fresh snapshot. It requires an artifact change, executes the obligations, checks a second stable snapshot, then requires passing witnesses and unchanged executable/toolchain/environment identities. With no unknown condition it returns `verified`. These checks are substantive, but none adds the omitted fixture to the protection domain. [Verification decision][verify]

## Conditional fixture witness

Use an ordinary Go module at the authorized workspace root. Assume readable regular files, a working unchanged Go toolchain, adequate time, no other failing obligations, and no unrelated concurrent changes. Before `Begin`, `.nerd/config.json` contains an explicit exclusion:

```json
{"world":{"ignore_patterns":["testdata/expected.txt"]}}
```

The source `add.go` implements `Add(a,b int) int` as `a-b`. The unchanged caller-owned `add_test.go` contains a normal `TestAdd`: it reads `testdata/expected.txt`, parses the expected integer, and fails when `Add(2,3)` differs. The fixture initially contains `5`. The caller contract selects `TestAdd` in package `.` with `TestFile=add_test.go` and `Reproducer=true`.

1. `Begin` directly protects `add_test.go` and protects other included verification inputs. The excluded fixture is absent from the map and snapshot. `TestAdd` genuinely executes and fails because `-1` differs from `5`.
2. During the edit interval an otherwise authorized actor makes a real included source change that leaves the defect intact, such as a comment change in `add.go`. This matters: without an included artifact change, `Verify` would report `no artifact change`.
3. The same interval changes the excluded fixture from `5` to `-1`. The witness assumes the mutation is otherwise permitted; it does not claim any session tool can bypass membership, scope or permission guards. An external authorized editor is sufficient. No configuration, named test, module file, protected fixture or environment changes.
4. `Verify` observes unchanged protected count and hashes, a changed included snapshot relative to the baseline, and stable snapshots around its run. It executes the unchanged named test, which now genuinely passes against the changed expectation. The source-derived local outcome is `verified` despite the unreviewed expectation change.

The official Go documentation describes `testdata` as ancillary test data and states that test binaries run in their package source directories. `os.ReadFile` reads the named file; the acceptance runner launches the actual process with the workspace directory and retained environment, rather than a membership-filtered virtual filesystem. These establish why a relative fixture read can remain a real test dependency even when world indexing excludes it. The acceptance command includes `-count=1`; this witness is not a Go test-cache artifact. [Go test semantics](https://pkg.go.dev/cmd/go#hdr-Test_packages) [Go test flags](https://pkg.go.dev/cmd/go#hdr-Testing_flags) [ReadFile](https://pkg.go.dev/os#ReadFile) [Actual runner][runner]

An untracked fixture already excluded by Git ignore rules is another conditional class. It is not necessary for the primary witness, which uses the explicit configured pattern to avoid confusing default tracked-file handling with explicit exclusions. A fixture omitted only after baseline admission is also a different case: previously protected bytes still get checked. The primary witness requires exclusion from the start.

## What remains protected

- Changing the declared `TestFile` is detected even though its direct protection is separate from the census. The witness leaves it unchanged.
- Changing an included protected fixture, module file or test is detected. Ordinary included tracked `testdata` is a negative control, not a claimed failure case.
- No real included artifact change is rejected; the witness supplies one explicitly.
- A missing, skipped or failing selected test does not become a pass. This witness produces an actual pass against altered input.
- Executable replacement, effective environment differences, cancellation and included workspace movement retain their existing defenses.
- Additional mechanical, coverage, policy or caller obligations can independently reject or repair the turn. The proven boundary is the acceptance report's protected-input continuity, not an unconditional overall task success or authorization escape.

Existing evidence tests exercise genuine repair, later defects, weakened named tests, new tests, replaced Go executables, absent/skipped reproducers, and dirty/untracked/configuration snapshot changes. They do not instantiate an excluded fixture read by an unchanged acceptance test. The tests were read, not run. [Existing evidence tests][tests]

## Proposed bounded contract

Do not globally weaken workspace membership or open excluded paths to model retrieval. Separate visibility for indexing from input integrity for acceptance.

For the existing narrow Go contract, an initial conservative option is to enumerate and protect the supported test/fixture/module inputs using an acceptance-specific traversal, with explicit containment, nonregular-file, I/O and size/budget handling. Included `.nerd` runtime exclusions and Git administration should remain deliberate. If a required input cannot be inspected under the user's authorization, return a bounded unverified reason rather than silently omitting it. Exclusion is not authority to read private material.

An alternative is a caller-reviewed input manifest naming fixture dependencies alongside each obligation, resolved and pinned by the host. That supports unusual fixture locations without pretending all dependencies can be inferred from filenames. It must specify additions, removal, file identity, containment, symlinks, revision and unreadability behavior, and must not permit the model to weaken the manifest. For unsupported external, generated or dynamically selected dependencies, either supply an adequate governed identity or disclose unsupported assurance. A filename census alone does not make arbitrary Go tests hermetic.

Preserve actual-run evidence, executable/environment checks and before/after stability. Bind the protected-input manifest and its completeness status to the report so downstream consumers can distinguish a covered contract from a partial census. Proposed policy: missing input integrity is unverified, not a new model-voted judgment. No extra LLM call is necessary for the bounded fixture repair.

## Costs and fair evaluation

These are proposed measurements, not measured benefits. Broader input protection adds traversal, reads, hashing, memory for identities and report storage; large generated fixture trees can dominate the cost. Blind whole-disk hashing would add unacceptable scope and privacy risk. Explicit manifests reduce census work but add caller review, maintenance and omission risk. Failing closed on unknown inputs can reduce apparent completion rates and cause legitimate re-contracting work. Those costs must be counted, not hidden as success-only token savings.

Compare current behavior, an acceptance-specific supported-input census, and a caller-manifest variant using identical repositories, source states, toolchains, contracts and mutation schedules. Measure false acceptance of changed expectations, correct rejection reason, legitimate repair acceptance, bytes/files inspected, census/hash wall time and CPU, memory, subprocess count, end-to-end latency, report size and all retry/re-contracting work. For an optional separately authorized model-facing study, hold model/version and task budgets constant and include failed attempts, uncached/cached input, output and reported reasoning usage. A cheaper pass against altered expectations is not an efficiency gain.

Required proposed fixtures:

| Case | Required interpretation |
| --- | --- |
| Explicitly excluded fixture changes while named test remains unchanged | Changed acceptance input is rejected or explicitly unsupported, not verified |
| Included tracked fixture changes | Preserve existing rejection |
| Untracked Git-ignored fixture changes | Same integrity outcome once declared or supported |
| Unchanged fixture and a genuine source repair | Actual passing witness remains eligible |
| Only excluded fixture changes | Preserve no-artifact-change rejection |
| Declared test file changes | Preserve direct test-file protection |
| Required fixture disappears, becomes unreadable, or becomes a symlink | Bounded unverified/error; no silent omission |
| Input changes during a verification run | No stable-input certification |
| World exclusion changes after baseline | Validate both protected old bytes and new domain; no removal by policy rewrite |
| Runtime output unrelated to tests grows | Retain deliberate output exclusions without treating them as test inputs |
| Same cases through contract attachment and session closure | Report and consumer preserve the unverified disposition |

Use a deliberately broken source and a separately retained original expectation as the oracle. Do not derive expected success from the same changed fixture under test. A mutation that restores shared membership filtering should fail the regression. At least one boundary test must run `Begin` then `Verify` with a real Go test in a separately authorized evaluation environment; comparator-only tests cannot establish the process-to-report contract. Preserve negative controls before making any performance claim.

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/agents.md
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/architecture/session/CHANGE_EVIDENCE.md
[attach]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/cmd/nerd/cmd_direct_actions.go#L313-L319
[begin-caller]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor.go#L1296-L1303
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/change_evidence.go#L137-L143
[begin]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/change.go#L249-L323
[census]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/change.go#L379-L417
[membership-config]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/workspace/membership.go#L64-L72
[membership-file]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/workspace/membership.go#L291-L329
[membership-patterns]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/workspace/membership.go#L407-L430
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/change.go#L169-L246
[verify]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/change.go#L326-L373
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/change.go#L486-L577
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/evidence/change_test.go
