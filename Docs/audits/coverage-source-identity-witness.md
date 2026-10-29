---
doc-class: deep-dive
subsystem: session-verification
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 4d984f9c9b719b013a6400f0c88deb48d2e5eb82
verification-method: static-source-inspection
supersedes: []
---

# Coverage source identity must survive the element-witness join

## Decision

Keep the single coverage-producing test invocation and the Mangle-owned witness verdict. Replace raw filename-suffix association with an explicit, unambiguous mapping from each profile source to its workspace file. Preserve that identity through changed-line filtering, element attribution and closure evidence.

The current implementation can assign an executed coverage block from `extra.go` to a changed, unexecuted function in module-root `a.go`. Both profile filtering and element filtering accept `strings.HasSuffix(profileFile, writtenPath)`. With a root-relative write path of `a.go`, `module/extra.go` passes that comparison. If the unrelated executed block overlaps the changed function's line span, the element becomes measured and not uncovered. Mangle can then discharge its execution witness under a passing test gate.

This is a source-derived **execution-evidence attribution defect**, not an observed successful shipment of incorrect code. Coverage repair can notice and repair the actual unexecuted block before closure. Independent gates can still reject the turn. Neither an inevitable false completion nor a measured failure frequency is established.

Only source and existing test definitions were inspected. No repository tests, application inference, compilation, benchmarks, failure injection or implementation changes were performed. All evaluation below is proposed.

## What already works

The system does more than infer execution from imports. `witness_candidate` is separate from the authoritative witness verdict: static dependency edges can identify candidate tests but do not discharge an obligation. Execution evidence comes from the turn's own coverage run and must join a passing test gate. This separation is valuable and should remain. [Witness policy][witness]

The host obtains pass/fail and coverage in a single invocation rather than paying for two test runs. It retains executed blocks long enough to determine whether a changed element ran at all. Empty bodies, absent profile entries and entirely unexecuted bodies have distinct interpretations. The finding is not that these capabilities are disconnected; it is that a file-identity shortcut can attach genuine evidence to the wrong element. [Coverage producer][profile] [Element interpretation][elements]

The target guarantee is intentionally weaker than complete branch coverage. A changed function whose one statement block ran is not `turn_element_uncovered`, even if another branch was missed. This report does not replace that policy with full branch coverage. It asks that the block which discharges the existing guarantee actually belong to that function's source file.

## Source freeze and distinct scope

The inspected main revision is [`4d984f9c9b719b013a6400f0c88deb48d2e5eb82`](https://github.com/theRebelliousNerd/codenerd/commit/4d984f9c9b719b013a6400f0c88deb48d2e5eb82). Root, session and core guidance were reviewed, together with the current audit-path inventory and relevant existing source/test files. Searches for `blocksForWritten` and coverage-suffix terminology did not identify an existing audit of this exact boundary; this is not a claim about all historical discussions.

The existing [test-removal subject-identity audit][retirement] addresses unrelated declarations whose bare names justify exempting removed tests. This proposal concerns profile-file ownership before execution facts reach policy. The existing [verification architecture description][architecture] describes suffix matching, but its warning-only coverage wording should not substitute for reading the current element-witness consumer.

## Actual producer-to-consumer route

### 1. The write set legitimately contains short root-relative names

`canonicalizeWrittenPath` cleans a relative target and returns its slash-normalized relative spelling. For an absolute target within the workspace it returns the relative path. A valid write to a module-root file can therefore be recorded as `a.go`; it is not required to carry a leading directory or module prefix. [Path normalization][paths]

`measureOwnTests` runs the applicable package tests and, when coverage is requested, passes `result.WrittenPaths` into `verifyTestsWithCoverage`. The latter reads the emitted profile through `parseCoverBlocks`. This is the ordinary test measurement path, not a dead convenience helper. [Own-test measurement][measurement] [Coverage producer][profile]

### 2. Ingestion admits a lexical suffix as source ownership

`parseCoverBlocks` normalizes written path spellings and retains a profile block when its normalized file ends with one of them. The retained record still has the profile's file name, line range, statement count and execution count. Both executed and unexecuted blocks are intentionally retained.

The [official Go coverage parser](https://go.dev/src/cmd/vendor/golang.org/x/tools/cover/profile.go) groups blocks by their full profile filename; the suffix association here is CodeNERD's own mapping. The missing condition is a source-identity match. A raw suffix does not require a path-component boundary. The string `module/extra.go` ends in `a.go` even though it is not the same file as `module/a.go`. [Profile filtering][profile]

### 3. The element join repeats the same association

`narrowToChangedLines` first calls `elementUncoveredRefs` on the whole block list and only afterwards reduces it to unexecuted changed-line debt. [Evidence ordering][narrow]

`elementUncoveredRefs` parses the current changed file, identifies its changed element references from its preimage, then calls `blocksForWritten`. That helper again selects blocks by raw suffix. `statementBlockInSpan` checks a positive statement count and overlapping line numbers; it cannot recover file identity after the wrong file has been admitted. `elementMeasured` accepts any such block. `elementUncovered` returns false as soon as an overlapping block has positive execution count. [Element association][elements]

The mistake is therefore not merely an extra warning in a report. It can change the positive execution classification of a changed element.

### 4. Closure turns the classification into authoritative facts

`recordBuildState` asserts `turn_element_measured` from `result.ElementMeasured` and `turn_element_uncovered` from `result.ElementUncovered`. [Closure publication][closure]

The policy derives `witness_executed` from measured and not uncovered. It derives `witness_met` when the element owes a witness, execution was established, and the test gate is passing with no red test gate. The static candidate-test edge is not involved in this witness. [Witness policy][witness]

The file-level fallback intentionally excludes unexecuted blocks lying within a changed element's span, because those are the element witness's responsibility. This prevents two verdict paths over the same evidence, but it means the incorrectly discharged element is not automatically rescued by file-level debt at closure. [File-level debt partition][partition]

## Conditional, unexecuted witness

Prepare an ordinary admitted Go module whose root is also the declared workspace. Use distinct function names so this does not depend on symbol-name collision:

| Source | State | Relevant line |
|---|---|---|
| `a.go` | Existing `Changed` function is edited; its known preimage is retained | Its statement block overlaps line 3 |
| `extra.go` | Unchanged `Other` function | Its executed statement block also overlaps line 3 |
| A valid root test file | Existing passing test calls `Other`, never `Changed` | Test execution supplies the positive count |

Assume both source files are included by the normal build configuration, the write succeeds, and the root-package coverage-producing test run passes. Its genuine profile contains a zero-count statement block for `module/a.go` and a positive-count statement block for `module/extra.go`, with ranges overlapping the changed function's span.

1. The write set contains `a.go`.
2. Profile ingestion keeps both blocks because both names end with `a.go`.
3. `blocksForWritten` again assigns both to `a.go`.
4. The changed function's span overlaps both. It is measured, and `elementUncovered` returns false upon seeing the unrelated positive block.
5. If this classification reaches closure with the green test gate, the changed function's witness is met despite its own block never executing.

The positive block need not occur first in the profile. The function returns false whenever it encounters an overlapping executed block. Multiple blocks or branches are unnecessary; a one-statement body makes the ownership error easiest to isolate.

This witness establishes the classification and policy consequence conditionally. It does **not** assume that a coding model chooses these files, ignores a repair prompt, or reaches final acceptance. Those are separate runtime questions.

## Repairs and counterevidence that limit the claim

The coverage repair stage sees the true zero-count block from `a.go`, because zero-count filtering retains it. If the suite passed and uncovered blocks remain, `verifyAndRepairCoverage` can ask for tests and remeasure. An actual repair that executes `Changed` eliminates the problem for that turn. The proposed audit must not imply this safeguard is absent. [Coverage repair][repair]

If a repair leaves the original function unexecuted, however, repeating the same file mapping is not an independent identity check. The element classification can remain false even though the profile itself is honest. At closure, the real zero-count block inside the changed function is intentionally assigned to element-level rather than file-level debt. [Partition][partition]

Other boundaries remain authoritative: permission and write guards, successful build, actual test outcomes, pinning, critic findings, acceptance requirements, test retention and any other outstanding obligations can still stop the turn. The finding does not grant tool authority, fabricate a passing test process, or demonstrate a whole-system completion bypass.

Important negative controls:

- A nonoverlapping positive block in `extra.go` does not discharge the function through this rule.
- An unexecuted colliding block alone does not create positive execution evidence.
- Ordinary `pkg/a.go` does not suffix-match `pkg/extra.go`; the root-file witness should not be generalized to every sibling path.
- A failed test gate does not satisfy `witness_met` even with incorrectly attributed execution.
- No usable profile or no matching statement block is a different evidence state, not this collision.
- An actual executed block from `a.go` can legitimately satisfy the existing coarse execution guarantee.

Existing test definitions cover profile parsing, unrelated written-file exclusion, Windows-style normalization, zero-statement blocks, measured versus unexecuted elements and real coverage fixtures. Those are useful controls, not evidence that a root filename suffix collision is safe. No inspected fixture covered this exact collision. [Profile test definitions][profile-tests] [Element evidence test definitions][element-tests]

## Proposed contract: a profile block owns one source identity

The conservative contract is:

> A block may support a changed element's execution witness only when the host has resolved the profile source to that element's exact workspace source under the measured build configuration, and the block overlaps that element's measured span.

### Option A: minimal separator-aware correction

At minimum, distinguish equality or a slash-delimited relative path suffix from an arbitrary character suffix. This rejects `module/extra.go` for `a.go`. Use one shared mapping helper in ingestion, changed-line filtering, element filtering and uncovered-path rendering rather than correcting only one occurrence.

This is a useful narrow repair, not a complete identity model. Nested modules, different import roots, multiple candidate files, generated paths and workspace/package boundaries can still make suffix-only association ambiguous. A boundary-aware suffix must not silently become a claim that package identity is solved.

### Option B: measured-package source map

Prefer a mapping built from the actual packages/configuration being measured: module/import identity, package directory, source path and relevant build configuration. Resolve each profile path against that map, preserve the canonical workspace-relative source identity on the admitted block, and carry it through all later joins. Cache this mapping only under inputs that determine its meaning.

Resolve to one source, or record unresolved/ambiguous. Do not select the first suffix candidate. An unmapped positive block must not discharge an execution obligation. Diagnostics can preserve the original profile spelling and mapping reason without treating uncertainty as a fabricated execution count.

The implementation should reuse package/build metadata already available in the execution path where possible. A new full package discovery process per block would be an unnecessarily expensive way to repair a cheap string match. Whether reusable metadata already has the exact configuration identity needed requires an implementation pass; this proposal does not assert that such a cache is wired today.

### Option C: source-bound evidence envelope

For a broader follow-on, associate the run identifier, source identity/revision, configuration identity and block range with execution facts before projecting them to element-level facts. This makes stale or mismatched measurements inspectable and supports provenance diagnostics. It must not be confused with per-test execution attribution: the aggregate profile still cannot say which individual test exercised the block.

Mangle should continue deciding obligations and their satisfaction. Go should supply exact measured identities and explicit uncertainty, not ask an expert model to guess whether two source paths mean the same file.

## Full costs and failure modes

The current matching loops compare blocks with written paths and repeat filtering per written file. A centralized source map can avoid some repeated comparisons, but no speedup has been measured. Construction and invalidation can cost more than the comparisons for small turns.

Costs to include:

- Package/module metadata acquisition, parsing and possible subprocess startup.
- Memory for mappings, provenance and collision diagnostics.
- Invalidation when module layout, generated files, build tags, platform, toolchain or package configuration changes.
- Extra uncertainty holds and additional repair/model/tool rounds caused by stricter admission.
- The unchanged coverage instrumentation and test-run cost; this repair does not make those free.
- Maintenance cost from duplicated path logic if consumers are not migrated together.

Failure modes include stale cached maps, mixing host paths with import paths, silently choosing among duplicate candidates, mapping an excluded file into the active configuration, and preserving a correct file mapping while joining it to stale line spans. The last issue is separate from this stable-source witness but belongs in the envelope's acceptance contract.

Avoid claiming improvement from fewer string comparisons alone. The relevant outcome is the cost and correctness of reaching a justified completion, including repairs, retries, test CPU, wall time and model tokens.

## Fair evaluation plan

Compare three implementations: current raw suffix matching, the minimal path-boundary correction, and explicit package/source mapping. Keep the existing witness policy and package test selection fixed so changes in attribution are not confounded with a stronger coverage requirement.

Use a deterministic integration fixture through the normal write/test/closure route with known preimages and captured genuine profiles. Include the root `a.go`/`extra.go` witness, a direct legitimate execution control, a nonoverlapping collision control, both-cold blocks, failed tests, missing profiles, Windows spellings, nested module layouts and ambiguous source mappings. Include changed methods, functions spanning several blocks and excluded files without assuming those must all use the same mapping policy.

Exercise both a coverage repair that actually executes the changed function and one that leaves the coverage gap unresolved. Record raw profile rows, resolved file identities, `ElementMeasured`, `ElementUncovered`, `turn_element_measured`, `turn_element_uncovered`, `witness_met`, remaining obligations and final outcome separately. The goal is to distinguish a repaired execution gap from a false discharge that merely happens to be caught by another gate.

Report false execution attribution, false holds, unresolved mapping count, coverage repair rounds, test process count, total CPU/wall time, memory, model tokens and tools, and final task correctness. Measure warm and cold mapping caches. A source-only witness is sufficient to motivate the change but not to claim production prevalence, a reliability percentage or a performance win.

[paths]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/executor_tools.go#L1438-L1455
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/build_verify.go#L571-L585
[profile]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/coverage_profile.go
[narrow]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/change_gates.go#L311-L340
[elements]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/turn_element_coverage.go
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/executor.go#L2876-L2891
[witness]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/core/defaults/policy/witness.mg
[partition]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/executor.go#L2897-L2942
[repair]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/change_gates.go#L373-L445
[retirement]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/Docs/audits/test-removal-subject-identity.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/Docs/architecture/session/09-VERIFICATION-GATES.md
[profile-tests]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/coverage_profile_test.go
[element-tests]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/turn_test_facts_test.go
