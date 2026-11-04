---
doc-class: deep-dive
subsystem: campaign-recurse-coverage-capture-admission
implementation-status: partial
evidence-status: source-only
last-verified: 2026-11-04
main-rechecked: 2026-11-04T21:47Z
verified-against: f9d26722c128542882ff7f5af4de5c1c2898caed
verification-method: static-source-inspection
supersedes: []
---

# Admit coverage completeness before judging recurse improvement

## Finding and decision

**A passing test gate can lose its lowest coverage report from the runner's retained output, yet supply an apparent coverage increase to the recurse ratchet.** The runner honestly marks dropped middle bytes, and the driver retains that fact. `metrics` nevertheless parses coverage from the head/tail excerpt without consulting completeness. The current protection for truncated findings applies only to failed results, so it does not reject this passing-check case. [Bounded runner][runner] [Measurement][measure] [Coverage projection][metrics] [Failure-only guard][verdicts]

One gate is sufficient. Suppose its three fixed package reports are 60%, 80%, 90% before and 70%, 40%, 90% afterward. If the middle package's reports fall outside both retained regions, the metric becomes 60% to 70%, although the complete report's minimum falls from 60% to 40%. With unchanged test count, a permitted changed candidate and all independent checks satisfied, `/harden` can derive `/keep`. The newly implemented per-gate comparison does not recover the missing report inside that gate. [Parser][coverage] [Per-gate facts][projection] [Policy][policy]

Require an admitted coverage observation, separate from the display excerpt and process exit. Until there is complete structured extraction, a truncated output cannot certify its full-report minimum or discharge required coverage nonregression. Preserve the genuine passing process verdict. Do not relabel a successful test execution as a failure merely because its optional presentation log is abbreviated.

This is a **current-main refinement of an acknowledged completeness concern**, not a claim to have invented that concern. The earlier coverage audit demonstrated a different, now-repaired cross-gate projection loss and explicitly called for missing/truncated observations to remain unknown. This report supplies the concrete passing-output witness, traces why the subsequent repairs leave it open, and defines a bounded extraction contract. [Earlier coverage audit][prior-coverage] [Current coverage addendum][coverage-addendum] [Current finding addendum][finding-addendum]

**Evidence boundary:** public main resolved to [f9d26722c128542882ff7f5af4de5c1c2898caed][pin]. Root and campaign guidance, campaign architecture, the recurse journey, the complete audit-path inventory, relevant prior reports, implementation and authored tests were inspected. Retained CodeNERD source bytes were checked against the pinned tree. No repository application, test, build, gate, fixture, benchmark, inference or training was run. No implementation was changed. The trace and evaluations below are conditional and unrun; the percentages are proposed witness inputs, not measured results.

## Architectural purpose and present support

Recurse's improvement loop is supposed to keep a changed candidate only when an appropriate measured metric improves and the required guards do not regress. Root guidance assigns creative choices to the model and executive decisions to Mangle. The correction belongs at their existing observation boundary: host code must preserve the meaning and completeness of what it measures, and policy must decide whether those observations justify retention. A model's explanation of why missing log lines were probably harmless cannot establish completeness. [Root guidance][root] [Campaign guidance][campaign-guidance] [Architecture][architecture] [Improvement contract][improve-policy]

Three facts must remain separate:

- **Process completion:** the program ran and its exit is accepted.
- **Capture completeness:** the relevant output or structured measurement stream was consumed without a loss that could change the metric.
- **Coverage comparability:** the before and after values describe the admitted same gate and coverage domain.

The witness has correct process completion and a fixed domain. It fails at capture completeness. Neither a successful exit nor a fresh source snapshot can restore bytes discarded before metric extraction.

| Capability | Status at the pin | Meaning |
|---|---|---|
| Bounded display output with explicit truncation | Implemented | `Output` contains head/tail bytes; `Truncated` records middle loss. [Runner][runner] |
| Aggregate truncation across node invocations | Implemented | `measure` retains each invocation and sets `gateRun.truncated`. [Measurement][measure] |
| Per-gate coverage comparison | Implemented | Each baseline coverage key is paired with its after value; a wholly absent after value becomes -1. [Metrics][metrics] [Projection][projection] |
| Conservative treatment of failed truncated findings | Implemented | A failed truncated after-result cannot appear harmless merely through a lower retained finding count. [Guard][verdicts] |
| Completeness admission for coverage from a passing excerpt | Missing on this path | `metrics` passes only output text to `Coverage`, then admits any parseable number. [Projection][metrics] |
| Full-stream structured coverage extraction independent of display retention | Proposed here | Preserve the decision statistic and its completeness before discarding presentation bytes. |
| Runtime incidence, benefit and overhead | Unverified | No workload or failure-injection study was performed. |

The existing improvements should remain. Reverting to first-gate selection or dropping the failure-only guard would reintroduce other defects. Removing the output cap could eliminate this clipping witness, but would abandon bounded retention and would not establish parser or domain completeness.

## Producer to decision trace

### The declared gate reaches the ordinary runner

A `nerd.md` gate with `kind: test` and `scope: node` becomes a `Gate` carrying its ID, command, language, environment and pass rule. `ForNode` substitutes `{pkg}` with the node's Go package spelling. The shared runner constructs the command through `build.WorkspaceCommand`; a Go invocation receives the configured build environment and flags. There is no forced conversion of every declared Go gate to JSON at this seam. The witness fixes the effective command/environment before and after. [Declarations][declarations] [Expansion][expansion] [Command construction][command] [Go invocation][go-invocation]

On an ordinary node visit the driver measures its own gates. After a nonempty candidate change it measures them again before calling `metrics` on the merged state. CLI and chat both use this same recurse driver around their campaign executors. This is a production consumer, not merely a parser that no caller uses. [Visit][visit] [After checks][remeasure] [CLI][cli] [Chat][chat]

### The runner consumes the output but retains only an excerpt

`gates.Run` sends stdout and stderr to the same `boundedBuffer`. Its default limit is 1 MiB. The buffer retains the first half and the final half, drops bytes between them, and inserts a marker in the returned string. After process completion the runner sets `Result.Truncated` from the dropped-byte count. `Passed` is separately derived from execution error and accepted exit status. Thus `Passed == true` and `Truncated == true` is an intentional, representable state. [Runner and pass][runner] [Buffer][buffer]

The distinction is sound. A bounded human-readable log need not make an otherwise successful command fail. The error occurs when the downstream metric silently treats that excerpt as the complete measurement population.

`measure` notices `Result.Truncated`, retains each invocation, and sets an aggregate truncation flag. For a node with several paths it concatenates the independently bounded outputs. The primary witness uses one node path and one invocation, so it does not depend on collapsed-node aggregation, first-invocation metadata or command-selection ambiguity. [Measurement][measure]

### Coverage is reduced after the loss

`Coverage` scans the supplied string for Go coverage phrases and pytest-cov total summaries. For all recognized percentages it returns the minimum in basis points. Its own comment says this makes a multi-package run as covered as its least-covered part. It receives no truncation state, required package inventory, parser completion status or source artifact from which to recover omitted percentages. [Coverage parser][coverage]

For each own-node test gate, `metrics` calls that function with `run.result.Output`. A successful parse creates `coverage:<gate ID>` and contributes to the aggregate `coverage` minimum. Neither `run.truncated` nor `run.result.Truncated` is read. [Metric producer][metrics]

The per-gate policy bridge correctly preserves the comparison across the same baseline gate key. It also substitutes -1 when that gate's after coverage is entirely absent. Here a high report survives, so the after key is present with a plausible but incomplete minimum. The missing-key safeguard never fires. [Current bridge][projection]

### Passing truncated results do not activate the finding safeguard

For a passing result, `FindingsAll` returns no failures. The driver marks an actually completed, non-no-tests test result as tested. It applies the truncation adjustment only when `!a.result.Passed`; the target-resolution truncation veto also requires failure. The witness is an improvement, not a fix target, and its gate passes on both sides. [Finding extraction][findings] [Tested and truncation projection][verdicts]

Policy sees a pass-to-pass gate with no finding increase, unchanged test count and a reported coverage rise. `/harden` can improve via coverage growth; coverage regression blocks improvements only when a decreasing coverage fact is supplied. Under the stated remaining prerequisites, the host receives `/keep` and enters normal adoption. [Policy][policy] [Metric rules][improve-policy] [Adoption][adoption]

A kernel cannot recover the low package report from a single, already-biased scalar. Adding another critic or requesting a model explanation would not restore that evidence.

## Concrete conditional witness

### Stable gate and package population

Use an admitted Go node `lib`, with one path `lib` and ordinary source files, plus the real subpackages `lib/b` and `lib/c`. Focus the sweep on `lib`; stipulate that `lib` has no workspace prerequisites that pull in those child nodes. The children may import `lib`, not the reverse. The existing focus filter follows prerequisites, not all descendants or callers, so this visit needs no child-node visit. Configure this node-scoped test gate before starting recurse: [Focus admission][focus] [Dependency closure][focus-filter]

```yaml
gates:
  - id: coverage-suite
    kind: test
    scope: node
    run: go test -v -p=2 -count=1 -cover {pkg} {pkg}/b {pkg}/c
```

At node `lib`, the three explicit operands are `./lib`, `./lib/b` and `./lib/c`. This is one gate invocation intentionally covering a fixed three-package suite. It is not the native default gate, not a shell command, and not an attempt that changes its verifier. This intentionally narrow declaration is for the focused `lib` witness; applying it unchanged to every child node would request different, potentially nonexistent operands and is not claimed to support a whole-tree sweep. The primary witness uses ordinary recurse without the optional error-free `MeasureGates` callback. Other installed gates can remain present; stipulate that they pass, have no coverage decline and introduce no independent veto. The example does not depend on suppressing them. [Declarations][declarations] [Expansion][expansion]

For a concrete producer model, inspect the Go source at [2aae1000ea1d4b28923b96db28acf4e3b99d6aff][go-pin]. Its test command retains passing verbose output, buffers output for this explicit multi-package, non-JSON, parallelism-greater-than-one case, and prints package results in order. That source also appends the package's real coverage summary. This pin grounds a feasible producer arrangement; it does **not** claim that an observed CodeNERD installation used this Go revision or that the fixture was executed. [Go verbosity][go-doc] [Go buffering][go-buffer] [Go output ordering][go-order] [Go success summary][go-summary]

Use fixed, pre-existing tests that genuinely execute and pass. Package A is `lib`, B is `lib/b`, C is `lib/c`. A emits a short verbose block. B's ordinary verbose logs exceed 512 KiB before B's coverage summary. C's ordinary verbose logs exceed 512 KiB after B's summary and before C's final summary. The log lines contain no coverage-like phrases. These same log volumes and ordering hold before and after; no test, gate, environment, toolchain, package selection or output limit changes during the attempt. No forged summary is needed.

Let L = 1,048,576 and H = L/2 = 524,288. Choose logs with a margin around the boundaries, so:

- every A coverage phrase is fully inside the first H bytes;
- every B coverage phrase is strictly after the first H bytes and before the final H bytes;
- every C coverage phrase is fully inside the final H bytes.

A verbose Go run can include both a test-binary coverage line and the command's `ok` summary. Put **both** copies of B's percentage in the dropped region. Merely dropping one copy would not establish the witness. The boundary inequalities, rather than a hoped-for timing race, define the proposed fixture.

### Same emitted reports and lost observations

Stipulate a real authorized source candidate that produces the following coverage reports while the tests continue to pass. The source edit may change how callers exercise branches; a coverage decrease need not make their assertions fail. This document does not supply or claim to have validated a concrete source patch yielding these percentages. That remains an explicit end-to-end fixture obligation.

| Observation | Before | After | Retention |
|---|---:|---:|---|
| A coverage | 60% | 70% | Head on both sides |
| B coverage | 80% | 40% | Dropped middle on both sides |
| C coverage | 90% | 90% | Tail on both sides |
| Minimum over complete emitted reports | 60% | 40% | Required by the existing full-output minimum interpretation |
| Minimum parsed from retained excerpt | 60% | 70% | What the current metric supplies |
| Test command exit | 0 | 0 | Genuine pass |
| Workspace test count | K | K | No count improvement or regression |
| Capture truncation | true | true | Accurately retained by the runner and driver |

Hold package coverage domains and the measurement convention fixed. The complete-output comparison is the audit's reference, not a new weighted-average policy or a claim that coverage alone proves behavioral quality. The failure exists even under CodeNERD's current, comparatively weak minimum rule.

### Causal result

1. The gate runs on both versions. All three package reports are emitted each time.
2. The buffer omits B's percentage from each retained string and accurately sets truncation.
3. `Coverage` sees A and C only, returning 6000 before and 7000 after.
4. `metrics` creates `coverage:nerd.md:coverage-suite` on both sides. The current bridge projects 6000 to 7000, not the complete result 6000 to 4000.
5. Genuine pass-to-pass results supply no failure-count regression and do supply the tested observation. The failed-result truncation safeguard is inapplicable.
6. On a harden pass, unchanged tests provide no alternate improvement. The apparent coverage increase supplies improvement, and no coverage-decrease fact is present.
7. Given a nonempty permitted source change, ordinary execution/ownership and adoption success, and no independent veto, the kernel can derive keep. The driver increments kept and improved and records the metric delta. [Metrics][metrics] [Bridge][projection] [Verdict facts][verdicts] [Policy][policy] [Keep][adoption]

This is a conditional false coverage-based keep relative to the existing complete-report minimum. It does not establish a deployed bad commit, an inevitable model-generated patch, disabled safety checks, or a behavioral regression escaping every later test.

### Controls that isolate the boundary

With all the same actual percentages but shorter innocuous logs, the complete retained reports produce 6000 to 4000 and the improvement coverage guard rejects the candidate. This comparison is between independently prepared output layouts, not an in-flight permission to edit tests. It shows that presentation volume can change the metric verdict while the underlying coverage facts remain fixed.

A second control keeps all actual percentages unchanged, but loses a previously retained low middle report only after verbosity grows. It can produce apparent growth without any coverage growth. This is the same mechanism and should be tested as a control, not published as another independent finding.

A small single-package run whose only coverage summary survives intact is not the primary witness. A complete structured measurement alongside a truncated presentation log can also be valid. The repair must distinguish these situations rather than declare every long-output command defective.

## Why the existing repairs and receipts do not close it

The per-gate identity repair prevents one independent gate's rise from hiding another baseline gate's fall. This witness keeps a single gate identity intact. Its minimum is computed from an incomplete subset **before** that comparison. Comparing every gate faithfully is necessary, but cannot validate a number already changed by output loss. [Coverage addendum][coverage-addendum]

The finding-census repair separates work selection from total finding identity, and conservatively handles failed truncated results. Here there are no failed results or omitted failure diagnostics. Passing tests can report lower coverage, and output limits can omit that observation without changing the process verdict. [Finding addendum][finding-addendum] [Current guard][verdicts]

The mission checks sidecar retains the runner's already-bounded output and result fields. Its schema does not preserve full raw output or a typed coverage extraction completion record. Saving that excerpt after the ratchet cannot reconstruct the missing B summary or repair the prior decision. The proposed correction should preserve usable structured evidence before presentation loss, not promise that a later copy of an excerpt is complete forensic evidence. The primary witness does not require mission mode. [Mission sidecar][sidecar] [Decision and recording order][record-order]

The source-identity coverage audit concerns the association between a profile block and a changed source element in session verification. This case needs no incorrect source suffix match. The reports can all have correct package ownership before the buffer removes one. [Profile identity audit][prior-source]

The newest mission-owned-write audit concerns durability during an execution ownership handoff. No crash, pending receipt, missing owned-write sidecar or cleanup is needed here. [Mission handoff audit][prior-mission]

The broad principle and the earlier coverage report's explicitly anticipated truncation case deserve credit. The distinct contribution is the current pass-to-pass producer trace, the numerical witness after both repairs, and an extraction/admission exit that can be checked without replacing the recurse architecture.

## Required observation contract

### Narrow safe admission

For the existing text-only parser, admit a coverage value only when the evidence supporting its declared aggregate is complete. The aggregate `gateRun` must inherit incompleteness from any relevant invocation. A parseable high value from one retained fragment does not establish the full minimum.

Apply this to the baseline as well as the after-result. Otherwise a truncated baseline can falsely make an angle measurable, or supply a misleading comparison even if the after-result is complete. Preserve a typed reason such as capture incomplete, parser incomplete, unsupported format or domain unresolved. An absent number alone is not a full account of why a required observation is missing.

If coverage is a required improvement guard, unknown coverage must not disappear merely because test count increased or lines decreased. The current missing-after behavior addresses the case where a baseline numeric key existed. A complete repair must also represent the baseline obligation when its capture was already incomplete, and let policy refuse an unsupported comparison. Keep the owner-visible outcome precise: the tests passed, but this attempt did not establish comparable coverage.

This is a proposed policy contract, not a claim that all such unknowns are already rejected. In configurations where coverage is deliberately informational, that narrower promise should be declared in advance instead of inferred from whichever reports survived.

### Extract before presentation loss

The preferred bounded design separates two consumers of the process stream:

1. A display collector keeps its existing bounded head/tail excerpt.
2. A measurement adapter consumes the stream before truncation and emits typed coverage observations with gate, invocation, package/domain and parser-version identity, together with completion or loss status.

For a supported textual minimum, the value can be reduced incrementally as complete records arrive. For a richer package-preserving contract, retain one admitted observation per package and an expected/observed inventory. Duplicate summaries from the test binary and command must be associated with the same producer, not mistaken for new domains. Where a structured profile is used, its generation, ownership, source identity, parse completion and cleanup need their own receipt. A file name alone is not a verified profile.

Completeness is not established merely by reaching the end of the retained excerpt, by seeing one final success line, or by finding a truncation marker in arbitrary program text. Require trustworthy capture/parser state. Bounded record framing must report an oversized or malformed record; silently skipping it recreates the same defect at a different limit. Cancellation, read failure and incomplete process I/O must leave observations qualified rather than promoting an interim minimum to complete.

Use the existing runner, measurement and Mangle boundary. Host code reports observed value, domain and completeness; Mangle derives measurability, comparability, improvement and nonregression. Do not grant the model authority to replace the verifier, rewrite the metric's domain, or waive incomplete evidence.

### Primary literature and technical limits

Dean and Ghemawat's **MapReduce** describes partial aggregation with a combiner when the reduction is associative and commutative. The relevant design idea is narrow: reduce the relevant measurements before discarding their raw transport, rather than treating an arbitrary excerpt as a sufficient statistic. A minimum can be combined exactly from complete partitions. That does not certify that the parser recognized every required package report, and it does not require a distributed MapReduce implementation here. [MapReduce, Section 4.3][mapreduce]

Go's documented test event stream separates package identity and output fragments, and its documentation recommends `go test -json` for multiple packages. That is a useful future adapter boundary, not proof that merely enabling JSON fixes this issue. A JSON stream truncated before extraction still loses records; arbitrary stdout phrases still require a trustworthy interpretation. Preserve framing, package identity and terminal completeness while maintaining the existing genuine process verdict. [Go test2json contract][test2json]

The source trace establishes the defect. These primary sources support the extraction design and producer semantics; neither paper nor tool documentation proves that any particular proposed repair is implemented or fast.

## Costs and failure modes

Let B be total emitted bytes, L the excerpt limit, P the number of relevant package/domain observations, and R the maximum admitted record size.

The present runner already receives B bytes even though it retains approximately L bytes of payload plus its marker. A streaming measurement consumer adds parsing work over B rather than only the retained excerpt. It therefore has additional CPU cost proportional to the stream, plus framing storage. The scalar minimum itself needs constant aggregate state; domain-preserving records need storage proportional to P, and safe framing needs a bounded R. These bounds do not include the Go toolchain's own per-package buffers, test memory or process I/O costs.

Preserving full raw output instead has disk cost proportional to B and creates retention, access, garbage-collection and interrupted-write obligations. Raising L postpones the counterexample and increases memory without establishing completeness. Re-running a quiet check can incur another entire test invocation and may observe a different source/environment; it is not a cost-free reconstruction of the original run. If remeasurement is supported, retain its distinct invocation and source identity.

| Failure or tradeoff | Required handling |
|---|---|
| Passing process with truncated text-only coverage | Keep the process pass; qualify coverage and do not certify the missing aggregate. |
| Truncated display with complete admitted structured coverage | Permit policy to use the structured metric, with its provenance retained. |
| Baseline already incomplete | Keep the required comparison unresolved; do not silently remove its guard obligation. |
| One failed/canceled invocation among several | Preserve existing verdict and failure safeguards; no positive aggregate from a partial set. |
| Oversized record or parser limit reached | Emit an explicit incomplete status and stop affirmative metric admission for the affected domain. |
| Repeated/contradictory package summaries | Use defined producer and terminal semantics; do not choose the most favorable number. |
| Unsupported format or coverage-like application output | Do not infer a trusted summary from an unqualified string match. |
| Full-output artifact cannot be written | Do not claim complete forensic retention; choose the supported conservative outcome. |
| Very large package inventory | Bound/admit inventory explicitly, rather than dropping a suffix while calling the aggregate complete. |
| Retry after transient capture failure | Count extra executions, retain distinct receipts and ensure the compared source remains applicable. |

Measure gate count and wall time, bytes emitted/retained/spooled, parser CPU, allocations and peak memory, record/domain limits, incomplete-observation rate, false keeps and false reverts, retry work, compensation/adoption cost and operator review. Any model study must additionally count all calls, tokens and provider spend, including rejected and abandoned attempts. No model call is intrinsically required for this correction, and no reliability, latency, token or cost improvement was measured in this audit.

## Fair evaluation plan entirely unrun

### Isolate extraction and admission

First compare the current path, a conservative text-completeness guard and a stream-based coverage adapter on the same byte streams, domains, process results and candidate metric inputs. Use the same output limit for each. The complete emitted stream is the reference for the existing minimum interpretation, not the retained head/tail excerpt.

Include:

- The exact three-package 60/80/90 to 70/40/90 witness, with both B summary copies fully dropped.
- Identical percentages with the low summary in the head, middle and tail; different innocuous log volumes must not change the admitted metric verdict.
- A genuinely improving complete result, a genuinely regressing complete result, unchanged coverage and unchanged tests.
- Before-only, after-only and both-side truncation; wholly absent and partially surviving reports.
- Passing, failing, no-tests, canceled and process-I/O-incomplete outcomes. Existing failure and no-tests guards must remain effective.
- Chunk boundaries through the coverage token, newline, UTF-8 sequence and JSON record; exact limit, limit plus one and oversized single records.
- Multiple invocations in a collapsed node where only a non-first invocation loses evidence.
- A display excerpt that truncates while independently extracted coverage remains complete, to measure unnecessary blocking.
- A baseline coverage obligation that is incomplete while tests increase or source lines decrease, ensuring it cannot silently vanish.
- A forged-looking marker in ordinary output without capture loss, and true loss without relying on prose-marker interpretation.

Do not count every conservative rejection as correct by definition. The stream-based adapter should retain ordinary valid long-output measurements when its structured evidence is complete, while the text-only guard may remain intentionally more conservative. Compare that availability cost explicitly.

### Exercise the real consumer chain

The authored bounded-buffer test checks head/tail retention with a small limit and an intact-output control. The coverage parser test checks minima across complete summaries. The current coverage audit fixture checks independent gates and a wholly missing after report using constructed gate results and a real kernel helper. These are useful controls, but they do not join passing output truncation to metrics and the final recurse decision. All were read, not run. [Buffer fixture][buffer-test] [Coverage fixture][coverage-test] [Per-gate fixture][identity-test]

A subsequent implementation evaluation should use temporary real packages with fixed verbose test logs, pin the Go toolchain and effective environment, and run the declared gate through `gates.Run`, `measure`, `metrics`, the current policy bridge and ratchet. Assert the emitted reference reports, byte-retention boundaries, typed capture status, parsed values, policy facts and final disposition. An adapter-only test or a manually injected `Truncated` boolean does not establish the full producer chain.

Then validate the candidate through normal CLI and chat recurse paths, with ordinary campaign permissions, write ownership, source scope, test/coverage guards and adoption unchanged. Use deterministic controlled executor output initially; live models are unnecessary for proving the observation defect. Any later model study should hold prompts, providers, budgets, gate definitions and tasks fixed and include unsuccessful attempts in its denominators.

**Checkable exit:** a passing gate's omitted coverage report cannot turn a complete-report decrease into admitted hardening, nor can incomplete baseline coverage silently remove a required nonregression obligation. An admitted value identifies its gate/domain and complete extraction evidence; the human excerpt may remain bounded. Complete supported long-output observations still work, and existing per-gate, failure-count, no-tests, permission, ownership and adoption controls remain intact.

## References

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/f9d26722c128542882ff7f5af4de5c1c2898caed
[root]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/agents.md#L1-L68
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/agents.md#L1-L15
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/Docs/architecture/campaign/INTERNALS.md#L149-L178
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/run.go#L15-L81
[buffer]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/run.go#L102-L134
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L759-L839
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L1575-L1604
[verdicts]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L1419-L1442
[coverage]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/metrics.go#L171-L204
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_policy.go#L226-L248
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/core/defaults/policy/recurse.mg#L265-L329
[improve-policy]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/core/defaults/policy/recurse.mg#L334-L398
[declarations]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/detect.go#L19-L90
[expansion]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/gates.go#L129-L147
[command]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/build/workspace_command.go#L30-L60
[go-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/build/invocation.go#L26-L42
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L1052-L1120
[remeasure]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L1324-L1350
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/cmd/nerd/cmd_campaign_recurse.go#L225-L246
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/cmd/nerd/chat/campaign_recurse.go#L218-L266
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/findings.go#L112-L120
[adoption]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L1481-L1513
[sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_mission_storage.go#L81-L160
[record-order]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L1442-L1463
[buffer-test]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/run_test.go#L100-L118
[coverage-test]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/gates/metrics_test.go#L48-L63
[identity-test]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_audits_test.go#L88-L190
[prior-coverage]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/Docs/audits/recurse-coverage-gate-identity.md#L104-L149
[coverage-addendum]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/Docs/audits/recurse-coverage-gate-identity.md#L1-L7
[finding-addendum]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/Docs/audits/recurse-finding-census-completeness.md#L1-L7
[prior-source]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/Docs/audits/coverage-source-identity-witness.md#L11-L29
[prior-mission]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/Docs/audits/recurse-mission-owned-write-publication.md#L14-L28
[mapreduce]: https://static.usenix.org/event/osdi04/tech/full_papers/dean/dean_html/index.html
[test2json]: https://pkg.go.dev/cmd/test2json
[go-pin]: https://github.com/golang/go/commit/2aae1000ea1d4b28923b96db28acf4e3b99d6aff
[go-doc]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/test.go#L107-L118
[go-buffer]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/test.go#L1467-L1510
[go-order]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/test.go#L1112-L1118
[go-summary]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/test.go#L1716-L1742

[focus]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_cycle.go#L515-L538
[focus-filter]: https://github.com/theRebelliousNerd/codenerd/blob/f9d26722c128542882ff7f5af4de5c1c2898caed/internal/campaign/recurse_dag.go#L36-L77
