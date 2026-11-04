---
doc-class: deep-dive
subsystem: campaign-recurse-evidence
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: e6e04f6bc0266b792942948eb2a668753c7abe11
verification-method: static-source-inspection
---

# Recurse source metrics treat failed reads as completed censuses

## Decision and evidence boundary

Preserve source-acquisition failures before recurse uses test counts or source-line counts as improvement evidence. The shared source walker currently skips an admitted file when `os.ReadFile` fails and returns no error for that omission. It likewise suppresses non-root traversal errors. The counters can therefore return a partial integer with a nil error, and the outer ratchet treats it as a measured total. Recovery between two reads can look like added tests; a new read failure can look like removed source lines. [Walker][walker] [Counters][counters] [Metric bridge][bridge]

This is a concrete acquisition-contract defect, with conditional adoption consequences. It is not an observed bad commit, an executed reproduction, or a claim about how often filesystem errors occur. No repository program, build, test, gate, model call, inference run or benchmark was executed, and no implementation or repository file was changed. The report was prepared from current public source and audit documents; only scratch research material and this document were written.

Root guidance assigns measured facts to the host and executive decisions to Mangle. The campaign guidance requires real host witnesses. The current architecture and forever-loop journey were read as intent and context, with source deciding current behavior. The journey explicitly distinguishes an unavailable metric from a measured metric. This report does not advocate inventing values for unavailable metrics or expanding the authority to read excluded paths. [Root guidance][guidance] [Campaign guidance][campaign-guidance] [Campaign architecture][architecture] [Journey][journey]

## Exact producer-to-consumer failure

### A failed observation becomes a number

`CountTests` initializes a total, calls `walkSources` over the workspace recursively, and sums `testsIn` only for delivered file bytes. `SourceLines` uses the same walker for each node directory and counts nonblank lines in files classified as non-test source. [Counters][counters]

The walker distinguishes some failures correctly:

- Failure obtaining workspace membership or refreshing its snapshot is returned.
- A traversal error at the requested start directory is returned.
- A directory-admission error is returned, including the normal skip-directory sentinel where appropriate.

But it handles other failed observations as ordinary omission:

- A non-root traversal error returns `SkipDir` for a directory, or nil otherwise.
- A leaf admission error shares the same nil-return branch as a legitimately excluded file.
- A source file's `ReadFile` error returns nil without invoking the counting callback or recording a deficit. [Walker][walker]

The direct file-read case is sufficient for this finding. It does not depend on whether the present membership implementation can produce a particular leaf error, or on the separate membership failure issue. A failed read of one existing admitted source file contributes zero and loses its cause. If the rest of the walk completes, the counter's error is nil.

Go's documented API distinguishes a successful complete `ReadFile` from a path error. Its walk callback contract lets the caller stop and return an encountered error; skipping an unreadable subtree is not an automatic assertion that the subtree was empty. The loss here is in codeNERD's callback, not a requirement imposed by the standard library. [Go ReadFile](https://pkg.go.dev/os#ReadFile) [Go WalkDirFunc](https://pkg.go.dev/io/fs#WalkDirFunc)

### The integer becomes ratchet evidence

Both the CLI and chat construct `RunRecurseCycles` with the normal attempt campaign executor. The shared `improve` path first calls `metrics` to establish which angle is measurable. The attempt then calls it again for the before state, executes the candidate, reruns relevant checks after a nonempty change, and calls it for the after state. [CLI][cli] [Chat][chat] [Selection][selection] [Before][before] [After][after]

`metrics` admits a `CountTests` or `SourceLines` result whenever the returned error is nil. A swallowed read error meets that condition. The returned map contains only integer values, with no read-completeness field, failed path or acquisition reason. [Metric bridge][bridge]

The policy bridge pairs the ordinary metric names present before and after and emits `recurse_metric(Cycle, Metric, Before, After)`. Its special missing-after handling for per-gate coverage does not validate source-census completeness. Mangle selects test growth for stabilize/harden/extend and a source-line decrease for simplify. A moved metric plus a completed nonempty test invocation can establish improvement success; changed content with success and no independent veto can derive keep. The keep branch enters Git adoption and increments the improved count. [Fact projection][projection] [Angle and guards][angles] [Tested projection][tested] [Ratchet][ratchet] [Adoption][adoption]

The rules are consistent with the facts supplied. They cannot recover the discarded acquisition failure.

## Conditional witnesses

These are proposed fault-schedule fixtures, not recorded runtime output. Use an ordinary non-mission recurse run so no claim depends on defeating mission protection or mission acceptance. Keep source membership, command declarations, toolchain and configuration constant. Use real, valid tests with stable identities, no comment-shaped fake tests, and complete normal gate output.

### A. Recovery is credited as test growth

Let the actual unchanged workspace contain `N` counted tests, with `q > 0` in one admitted existing test file.

1. The initial measurability call succeeds, and a test-growth angle is selected.
2. During the attempt's before census only, the selected existing file's read fails. Other reads and membership refresh succeed. The counter returns `N-q, nil`.
3. The candidate makes an otherwise permitted change but adds no tests. All tests retain their actual identities and bodies.
4. The subsequent checks genuinely execute the applicable existing tests and satisfy all independent obligations relevant to this witness.
5. The file is readable during the after census. The counter returns `N, nil`.
6. The ratchet receives a positive test-count delta although the true inventory did not grow.

The unconditional source conclusion is the false nil-error baseline total under the stated read failure. A keep additionally requires changed content, a completed nonempty test invocation, no other gate or guarded metric regression, and all normal execution, review, permission, ownership and adoption conditions to permit the candidate to reach this boundary. Those conditions are not asserted as observed.

The mandatory improvement prompt asks for actual behavior-pinning tests. A worker or independent review that enforces that obligation should reject the no-new-test candidate. That is a real countervailing protection, not evidence that a false host measurement is harmless. [Improvement instructions][prompt]

### B. A read failure is credited as simplification

For an admitted simplify attempt, let the node's true source total remain `L`, and an existing production source file contribute `q > 0` nonblank lines. Before reads succeed. The candidate changes permitted bytes without reducing that total. Checks complete correctly. During the later source-line census only, reading the existing production file fails.

The after line total becomes `L-q` with nil error. Keep test-count acquisition healthy, coverage unchanged, and the remaining safeguards satisfied. The deterministic metric boundary can then report simplification without any actual source-line reduction.

The failure must be scoped to the later census read, rather than a persistent inability to compile the source. If compilation, an evidence snapshot, Git adoption or another independent check also fails, that stage may prevent adoption. This report does not assert that a permanently unreadable source file survives those controls.

### C. Useful work is falsely rejected

Start with a correct baseline count `N`. A legitimate candidate adds `m > 0` genuine tests and passes the normal checks. If the after census omits an existing file containing `q > m` tests because that one read fails, it reports `N+m-q < N`.

The test-count guard can then reject the candidate as if tests had been removed, even though none was. This symmetric outcome matters independently of false keeps: observation faults can waste completed work, model calls, checks and recovery effort. No frequency or monetary estimate has been measured.

## Why this is not the neighboring finding

The current public audit corpus was searched at the same main revision. Exact `CountTests` and `SourceLines` searches each returned only the test-census semantic audit; `walkSources` returned no audit. The broader metric/unreadable search returned neighboring reports, not this failure trace. This is a bounded novelty check, not proof about every private discussion or historical branch.

- **Test-census semantic admission** already establishes that raw comment text can count as a test and calls for unreadable-file controls in its future matrix. Credit that expectation. Its main mechanism is misclassifying bytes that were successfully read. Here every counted test is real; the defect is a known failed read converted to a successful partial total. Syntax-aware parsing alone does not repair it. [Semantic census audit][prior-semantic]
- **Finding-census completeness** concerns diagnostic caps/truncation and target-absence conclusions. This case has complete, correct gate results and fails in the separate filesystem metric acquisition. The current finding and truncation protections should remain intact. [Finding census audit][prior-findings]
- **Coverage gate identity** concerns pairing independent runner measurements. This finding needs no coverage decrease or coverage parser ambiguity. [Coverage audit][prior-coverage]
- **Workspace-membership failure** concerns an authority operation being converted to a negative membership result before a scanner sees it. The primary witness here assumes membership admission succeeds and supplies a genuine file-read error afterward. Fixing membership alone would leave this nil-return branch. [Membership audit][prior-membership]
- **Portable crucible** already proposes explicit evidence availability, costs and truthful measurement. The present report supplies a specific source-backed correction within that broader design, not a new orchestration system. [Portable proposal][portable]

The authored metric fixtures exercise healthy counts and direct/recursive source-line scope; they do not establish handling of a failed file read or partial directory enumeration. They were read, not run. [Metric fixtures][fixtures]

## Required contract and bounded correction

1. **Complete is distinct from partial.** A numeric source metric may be used as a comparable total only when the required enumeration, membership decisions and content reads succeeded. Keep the acquisition cause and affected scope when they did not.
2. **Excluded is distinct from unreadable.** Preserve normal ignore rules, node-depth limits and excluded-file skips. An admitted source that could not be read is not an empty source or an authorized exclusion. Do not retry by bypassing membership or permissions.
3. **Before failure does not start an unsupported experiment.** If the selected angle lacks a complete baseline, retain the existing unavailable/skipped semantics with a useful reason, or hold according to explicit policy. Do not let partial counts select or justify a mutating attempt.
4. **After failure does not prove movement.** A comparison with an incomplete required census cannot claim growth, reduction or nonregression. Let Mangle derive the appropriate unverified/hold/revert disposition using the ordinary owned-write and recovery rules. Never silently reinterpret an unknown count as zero or as the last good current value.
5. **The display agrees with the admission.** Journal and user-facing reasons must identify unavailable acquisition rather than render a bare apparent improvement delta. Persist bounded references and causes if needed, not an unlimited list of every pathname.
6. **Recovery is remeasurement, not improvement.** A later complete observation can restore availability. It must not be compared with an earlier partial subtotal as if both describe the same complete inventory.

For the narrow defect, propagating read/traversal errors is a small first step. The existing caller omits errored metrics; that prevents this particular partial integer from masquerading as a total, but does not by itself provide visible causes or define admission when an independently required guard metric is missing. Specify that caller behavior before claiming a full completeness contract. A richer result is justified only to support useful partial display or scoped recovery, not as ceremony.

Do not bundle a complete parser rewrite, new model judge, new durable scanner, or universal filesystem snapshot guarantee into this correction. Syntax, active test selection and behavioral test value remain separate obligations. A completed walk also does not prove no concurrent mutation occurred; source-revision consistency needs its own evidence.

## Fair evaluation plan, all unrun

Use deterministic failure injection at the actual acquisition boundary, with a recorded operation/path and one occurrence. Avoid relying only on chmod, which can behave differently under privileged runners and across operating systems. A helper returning a chosen integer or a hand-built ratchet fact does not test failure transport.

Compare the current and proposed path on identical source bytes, membership rules, node scopes, commands, model-independent attempt output and fault schedule:

| Case | Required observation |
|---|---|
| Baseline test-file read fails, then recovers | No test-growth credit from restored visibility; acquisition cause retained. |
| After production-file read fails | No line-reduction credit from omitted bytes. |
| Real tests added; after old-test read fails | Explicit incomplete/unverified result, not a fabricated test-removal verdict. |
| Non-root directory enumeration fails | No nil-error complete total over a silently skipped subtree. |
| Start directory or membership refresh fails | Preserve existing error propagation and make the consumer's unavailable disposition visible. |
| Legitimately ignored file or nonrecursive child | Normal exclusion remains normal; no permission expansion or unnecessary reads. |
| Empty admitted file and truly empty source scope | Legitimate zero is retained when the observation completed. |
| Healthy real test growth and real simplification | Existing normal acceptance remains possible. |
| Failure recurs or disappears | Bounded retry/hold behavior; restored observation is not counted as source improvement. |
| Cancellation, owned-write undo or adoption uncertainty | Preserve existing lifecycle and compensation contracts; no blind replay. |

Qualify the shared loop through both CLI and chat after leaf checks. For false acceptance, assert the full production ratchet/adoption decision with a controlled eligible candidate, rather than only the raw count. For false rejection, demonstrate the candidate really added tests independently of the counter. Include ordinary and mission paths separately; do not infer mission satisfaction from a kept ordinary improvement.

Measure false improvement credit, false rejection, unknown/partial frequency, cause visibility, successful recovery, file reads, directory walks, membership refresh calls, elapsed time, peak memory, retries, gate reruns and compensation/adoption outcomes. Charge all failed and abandoned work to the denominator.

Propagating one error is cheap relative to repeatedly rereading a workspace, but richer path-level receipts, recovery and caches have CPU, storage and maintenance costs. The current path may measure source repeatedly for measurability, baseline and candidate, and `SourceLines` refreshes membership for each requested node directory. Do not add an unconditional extra full census or presume incremental reuse is free; any cache needs the same membership/content identity and error disposition. No token savings, reliability rate, speedup or net benefit was measured.

Completion means that recognized source-acquisition failure can no longer masquerade as the measured metric movement used to justify a keep or metric-regression rejection, while healthy improvements and authorized exclusions retain their existing meaning.

## Pinned source references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/architecture/campaign/README.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/journeys/10-forever-loop.md#L77-L103
[walker]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/gates/metrics.go#L121-L168
[counters]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/gates/metrics.go#L88-L118
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_cycle.go#L1575-L1604
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/cmd/nerd/cmd_campaign_recurse.go#L225-L269
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/cmd/nerd/chat/campaign_recurse.go#L218-L248
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_cycle.go#L1183-L1208
[before]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_cycle.go#L1230-L1257
[after]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_cycle.go#L1324-L1355
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_policy.go#L226-L248
[angles]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/core/defaults/policy/recurse.mg#L356-L398
[tested]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_cycle.go#L1421-L1439
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/core/defaults/policy/recurse.mg#L299-L329
[adoption]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/campaign/recurse_cycle.go#L1481-L1507
[prompt]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/prompt/atoms/campaign/recurse_attempt.yaml#L41-L58
[prior-semantic]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/audits/recurse-test-census-semantic-admission.md
[prior-findings]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/audits/recurse-finding-census-completeness.md
[prior-coverage]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/audits/recurse-coverage-gate-identity.md
[prior-membership]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/audits/workspace-membership-failure-and-incremental-census.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/Docs/audits/portable-recurse-deliberation-crucible.md
[fixtures]: https://github.com/theRebelliousNerd/codenerd/blob/e6e04f6bc0266b792942948eb2a668753c7abe11/internal/gates/metrics_test.go#L5-L52

