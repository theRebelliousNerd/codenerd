---
doc-class: deep-dive
subsystem: gates-errorfree-go-test-census
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: 097d67a6c0e4b79313ae689d8e0b541a7a65688a
verification-method: static-source-inspection
---

# Error-free G3 loses unrelated package failures beside a compile diagnostic

## Finding and decision

The error-free Go test reader applies a whole-invocation compile-error suppression rule to individual package failures. Once any positioned compile diagnostic is parsed, `readGoTest` emits no fallback finding for **any** failed package without a failed named test. An unchanged compile error in package A can therefore conceal a newly failing `TestMain` or initialization in independent package B from G3's finding census. B's failure output remains in the saved text, but its identity and count do not reach the comparison policy. [Reader][reader] [Measurement][measurement] [Fact projection][facts]

This is a **partial failure-census defect**, not a false process pass. Both G3 measurements can remain `fail`. The consequential error is that an actual new package failure can appear as an unchanged, fully enumerated `fail 1 -> fail 1` comparison. With only G3 selected, the standalone comparison can report the pair keepable and no new findings. The optional recurse error-free ratchet consumes the same deficient snapshot facts. Other ordinary recurse gates can independently veto the change; this report does not claim an inevitable adopted regression. [Comparison policy][policy] [Standalone commands][cli] [Recurse bridge][recurse-bridge]

Preserve the existing incremental-work policy. Make deduplication local to the failure whose evidence establishes it: a compile diagnostic for A is not evidence that B's terminal failure has been represented. Host code should conserve package verdicts and their known relationships; Mangle should continue deciding nonregression and adoption.

## Evidence boundary and architectural fit

Public main resolved to **097d67a6c0e4b79313ae689d8e0b541a7a65688a**, committed November 4, 2026 at 07:51:04 UTC. All codeNERD references below use that pin. This review read root/public-repository and campaign guidance, campaign/build architecture, implementation, relevant authored tests, the audit inventory/queue and neighboring reports. Architecture documents were treated as intent and navigation, not substitutes for source.

No repository program, compiler, build, test, gate, model inference, benchmark or failure injection ran. No implementation, repository file, credential or setting was changed. The fixture and outcomes below are conditional source derivations, not a reproduced incident. Proposed evaluations are unrun.

The root guidance assigns executive decisions to Mangle and demands production-path validation. The campaign specification describes error-free snapshots as the data underlying the recurse nonregression check. The right repair is therefore a faithful observation boundary, not a model judgment about whether a panic matters or a parallel Go adoption heuristic. [Root guidance][guidance] [Architecture standard][architecture] [Campaign contract][campaign]

## 1. Reachable observation-to-decision chain

### G3 is a real production reader

`ErrorFreeMeasurers` defines G3 as `go test -count=1 -json` over the selected test packages, defaulting to `./...`, with `readGoTest` as its decoder. The command path applies the workspace Go invocation configuration, runs in the workspace root, and captures separate stdout/stderr. This is distinct from the ordinary declared-gate `gates.Run` / `FindingsAll` route. [G3 selection][selection] [Process and decoding][measurement] [Ordinary reader][ordinary]

The standalone `nerd gates measure` command accepts `--only test`, measures and saves the snapshot. `nerd gates diff` loads two snapshots, asks `gates.Compare`, prints the result and returns an error only if the pair is not keepable. The after/before package selection and root can be identical in this witness, so domain admission does not reject it. [Commands][cli] [Saved snapshots and domains][facts] [Comparison][compare]

The recurse CLI's `--gates` option constructs a `MeasureErrorFree` callback and passes it as `RecurseCycleConfig.MeasureGates`. The cycle reads a full baseline, measures after a changed attempt, projects both snapshots and queries the ratchet. G3 continues using its test package selection; only the designated lint gates are narrowed by `ScopedPackages`. [Recurse configuration][recurse-cli] [Refresh][refresh] [After snapshot][after] [Fact bridge][recurse-bridge]

The inspected chat recurse constructor does not itself install this optional `MeasureGates` callback. Do not generalize this optional G3 integration claim to every chat recurse run. Its ordinary gates remain a separate observation path. [Chat constructor][chat]

### Package ownership is discarded at the decisive branch

The reader:

1. Decodes `Action`, `Package`, `Test` and `Output`.
2. Records a finding for each named-test `fail`, and marks `failedTests[Package]`.
3. Records each package-only `fail` in `pkgFailed`.
4. Combines all build output and stderr into one `CompileFindings` call.
5. For each failed package without a named-test failure, emits the package fallback **only if the global compile-findings slice is empty**.

The test-local `failedTests[pkg]` exclusion is package-qualified. The compile exclusion is not: it checks `len(compile) == 0`, without linking a diagnostic or build-failure event to the particular package. [Exact branch][reader]

A supported positioned line such as `a/broken_test.go:3:9: undefined: Missing` suffices to make that slice nonempty. `CompileFindings` preserves its file, line, column, check and message. The failure of unrelated B does not need to emit a positioned diagnostic and is not input to this build-output parser. [Compile decoder][compile]

The source's own reader comment promises a package finding for a package failing without a failed test, explicitly naming initialization panic and `TestMain` exit. The issue is a specific violation of that contract when a different package also fails to compile.

### Positive count prevents the existing uncertainty guard

Once the reader returns at least one finding, `commandGate.Measure` chooses `MeasureFail` and stores the residual output. It sets `Unenumerated` only in the no-findings, nonzero-exit branch. Thus A's retained compile finding prevents the opaque-failure flag from being set, even though B's package failure is absent. `MeasureErrorFree` assigns `Count = len(Findings)`. [Measurement classification][measurement] [Count assignment][count]

`Snapshot.Facts` exports status, count, domain, optional unenumerated flag and finding keys. It does not ask the kernel to parse `Output` or restore suppressed terminal events. Retaining B's text in JSON is useful forensic evidence but does not repair the policy input. [Projection][facts]

For same-domain `fail -> fail`, the current comparison declares regression when count grows, or when after becomes opaque while before was not. It deliberately permits equal-count failure replacement because repairing a compiler problem may reword another. Here the old finding is unchanged and an additional independent failure is lost **before** that intentional policy operates. [Policy][policy]

## 2. Conditional witness with independent packages

Qualify the fixture using the repository's supported Go toolchain before calling it a reproduction. The module directive names Go 1.27.3. Official Go documentation describes one test binary per listed package and distinguishes terminal package events from named-test events. It also defines `TestMain` as the package test entrypoint, able to perform setup/teardown and control its exit. The event protocol carries package identity precisely because parallel package/test streams can interlace. [Module][gomod] [Go test packages][go-test] [TestMain][test-main] [Event protocol][test2json]

### Preconditions

Use a small synthetic module with independent packages `example/a` and `example/b`:

- A has one unchanged, recognized positioned compile diagnostic in its test source, on both sides.
- B has passing named tests and an unchanged `TestMain` that checks a real production invariant during setup or teardown. Before, B exits zero. After a permitted production-source edit in B, the invariant fails and B exits nonzero, while no named test produces a `fail` event.
- B does not import the broken A test compilation unit. The invocation actually attempts B's test binary despite A's independent test-build failure.
- Both measurements select the same package patterns, root, toolchain and relevant configuration. No permission change, unsupported executable, missing package, cancellation, incomplete JSON, output truncation or concurrent workspace edit is involved.
- Each side's complete output contains the same single recognized compile diagnostic for A. After additionally contains B's package-only terminal `fail`. No extra located compile diagnostic is attributed to B.

A startup panic before any named test is an alternate package-only failure, but the primary fixture can use a deliberate, deterministic nonzero `TestMain` result to avoid relying on panic stack formatting. The production invariant changes; the test harness does not. No model or generated-test manipulation is necessary.

### Expected observations and projection

| Boundary | Before | After |
|---|---|---|
| A test-build outcome | Same positioned compile error | Same positioned compile error |
| B test-program outcome | Pass | Package-only fail, no named-test fail |
| Total process outcome | Nonzero because of A | Nonzero because of A and B |
| `CompileFindings` | One A finding | The same one A finding |
| `pkgFailed` | Includes any A package fail event | Also includes B |
| B fallback branch | Not needed | Suppressed because `len(compile) > 0` |
| G3 status/count | `fail`, 1 | `fail`, 1 |
| `Unenumerated` | False | False |
| Finding identities | A compile identity | Same A identity only |
| B's output text | No failure | Retained in residual output when provided |

This table is a source-derived hypothetical, not actual output from a run. The exact A build-event shape can vary by toolchain; the essential premises are one parsed A compile finding and a terminal B package failure. On the current supported protocol, build identity is available for a more precise decoder. The reader's local event type does not retain `FailedBuild`. [Reader][reader] [Protocol][test2json]

For a comparison with only G3 selected:

1. The domain strings match.
2. Neither side is unavailable or unverified.
3. The count does not grow.
4. Neither side is opaque under the facts supplied.
5. No finding key is new or gone.

The policy therefore has the inputs for `same` and `cycle_keepable`. The CLI can render G3 as `fail 1 -> fail 1`, with zero new/gone findings, and print its normal keepable verdict. A correct census containing A's unchanged compile finding and B's new package failure would give growth and activate the existing nonregression rule. [Rules][policy] [CLI rendering][render]

### Bound the recurse consequence

The same measurement defect is present when recurse enables G3. Its error-free snapshot veto cannot see this added B failure. That does **not** alone establish a kept commit: ordinary workspace or node test gates, scope protections, other error-free gates, target resolution, improvement metrics and mission evidence may independently reject the attempt. The ordinary Go findings reader is separate and must be evaluated independently rather than assumed to share the same suppression rule. [Ordinary gate path][ordinary] [Remeasurement][after] [Keep conditions][recurse-policy]

The standalone false nonregression classification is already a substantive failure of the exposed comparison contract. It is not necessary to invent an all-guards-bypassed adoption story.

## 3. Counterexamples and limits

- With no compile diagnostic anywhere, the current reader emits B's package fallback. The existing authored fixture checks a failed named test in A plus an initialization-panic-like package event in B, but supplies no compile diagnostic; it does not exercise this interaction. [Reader fixture][fixture]
- If B emits a named-test failure, the reader appends that test finding before the global compile check. That is not the primary witness.
- If the old compile failure is fixed while B newly fails, there may be a legitimate equal-count replacement question. The witness deliberately leaves A unchanged, isolating genuine growth hidden as equality.
- If B never executes because it depends on A's failed compilation, a runtime-failure claim for B is invalid. Keep dependency-caused build failures separate from independent executed package failures.
- G3 never becomes `pass` in this witness. The error is a false claim of no worsening and no new finding, not a green process result.
- A failed package with no named tests is not necessarily a source defect. Environmental/test-harness failures still need to be represented and classified; their presence cannot be erased by another package's compiler output.
- This report does not claim every Go version produces identical build events or that all terminal events with missing information are safe to count as independently attributable defects.

## 4. Novelty boundary

The bounded screen reviewed the current audit inventory and queue, read the related reports below, and searched the repository for `readGoTest`, `pkgFailed`, `len(compile)`, package failure/compile combinations and related `TestMain` audit references. The exact reader and suppression branch appeared in implementation/tests, not in a prior audit found by this screen. This is not a claim about all historical branches or private reports. [Queue][queue]

| Existing work | Its boundary | Difference here |
|---|---|---|
| Test verdict diagnostic coverage | Ordinary test responsibility facts can omit a silent failed named test while another diagnostic enables baseline exemption. | This is the separate error-free G3 reader, package-only terminal failure, unrelated compile suppression and snapshot nonregression. |
| Recurse finding census completeness | Work-list caps can hide later findings from ordinary target/count decisions. | Complete small output and uncapped G3 findings are sufficient here; the failure is suppressed before any cap. |
| Recurse diagnostic subject identity | Normalized diagnostic identity can merge distinct named subjects. | B receives no finding at all; no identifier collision or normalization is needed. |
| Scoped gate overlay conservation | Empty/partial overlays must not manufacture complete evidence. | Both snapshots can be fresh whole-domain measurements; no overlay is required. |
| Gate comparison domain admission | Different root/package selections must not compare as one domain. | Both observations have the same correctly admitted domain. |
| Mission error-free witness receipts | Mission evidence needs the appropriate invocation/snapshot-bound receipt. | Correctly bound receipts would still carry the deficient census produced here. |

[Prior verdict audit][prior-verdict] [Prior census audit][prior-census] [Prior subject audit][prior-subject] [Prior overlay audit][prior-overlay] [Prior domain audit][prior-domain] [Prior receipt audit][prior-receipt]

These reports establish useful general evidence-conservation principles. This report's contribution is the concrete **global compile-findings condition suppressing unrelated terminal package failures** and its current consumer consequence.

## 5. Proposed observation contract

Specify the following invariant in the owning campaign/gate documentation before implementation:

> Every terminal failed package in a completed supported test invocation is represented by its own residual package failure, or by explicitly associated failing-test/build evidence that accounts for that package. Evidence for another independent package is not an accounting witness.

A bounded design should:

1. **Retain verdict identity before diagnostic enrichment.** Preserve package-only terminal failures, named-test outcomes and available build-cause identity separately. Add diagnostic locations as evidence, not as a requirement for existence.
2. **Make suppression attributable.** Suppress a redundant package fallback only with an explicit relationship to already represented evidence. A matching file directory alone is insufficient for imported build failures, external-test variants or generated test packages.
3. **Keep unresolved causes visible.** When the decoder cannot establish the relationship, preserve a residual package finding or an explicit incomplete-census disposition with package identity. Do not use a nonempty aggregate findings list as proof of completeness.
4. **Define the unit being counted.** Preserve intentional deduplication between a package's aggregate fail and its own failed named tests. Distinguish independently executed package failure from dependent-package fallout from one failed build. Avoid silently changing the policy to count one defect many times.
5. **Use supported protocol evidence.** Current Go documentation provides `FailedBuild` for a package failure caused by a build failure. Qualification must cover the supported Go version's build events and fallback behavior for older saved data; do not infer a semantic guarantee solely from an optional field's presence.
6. **Carry completeness through snapshots and facts.** If the parser exposes unresolved failures, the measurement, saved snapshot, fact projection and comparison must preserve them. Existing `Unenumerated` handling is a useful mechanism, but a global boolean alone does not restore missing package attribution.
7. **Keep policy ownership intact.** The host conserves measurements and associations. Mangle decides comparable domains, regressions and the recurse verdict. No LLM call is needed to decide whether a package-only fail event exists.
8. **Keep old receipts honest.** A decoder fix cannot reconstruct discarded events from existing typed finding lists. Where bounded saved output permits re-decoding, make that generation explicit; otherwise remeasure or mark the old census unsupported for the strengthened guarantee. Do not silently rewrite historical evidence as newly verified.

The smallest containment can conservatively retain the package failure whenever compile attribution is uncertain. Its possible duplicate-count and false-regression costs must be documented. The durable solution should define correlation rather than simply delete a condition and declare the entire census complete.

## 6. Costs, failure modes and fair unrun evaluation

Tracking package verdicts and build-cause associations should require linear work in observed events plus bounded per-package maps. This is a design estimate, not a measured benchmark. Parsing already retains per-package output; a correction does not inherently require another process, model request or complete tool rerun. Additional retained verdict/association records, schema compatibility and Go-version qualification are real costs.

Main repair risks:

- Double-counting a compiler error once per dependent package and producing avoidable reversions.
- Suppressing a runtime package failure merely because its source directory also has some diagnostic.
- Mistaking a `TestMain` exit after passing tests for ordinary success.
- Erasing ambiguity when build-event correlation is missing or partial.
- Making unavailable, cancelled or malformed invocations look fully measured.
- Correcting only standalone comparison while recurse or persisted snapshots continue projecting old incomplete rows.
- Claiming full raw-log completeness as a side effect. Buffering and output budgets are separate contracts.

All evaluation below is **proposed and unrun**:

| Layer | Required fixture or observation |
|---|---|
| Decoder unit | Before/after complete streams with unchanged compile A and newly failing package-only B. Preserve B despite A; exercise both event orders and interleaving. |
| Named-test control | A compile failure plus B's named-test failure remains represented, without redundant aggregate counting under the chosen contract. |
| No-compile control | Existing package-only fallback behavior remains; explicitly cover `TestMain` and initialization failure. |
| Correlation control | One failed dependency build with several dependent packages is distinguished from several independently executed failing package binaries. |
| Native Go qualification | A temporary module with independent A/B, fixed test harness and a production invariant change. Verify B actually executes, complete protocol output and the precise supported toolchain/build events. |
| Classification | Nonzero status with positive findings and a residual package failure cannot advertise a complete unchanged census. |
| Snapshot round trip | Save/load preserves residual failure identities and completeness; old snapshots are not silently upgraded. |
| Real kernel comparison | Same root/package domain, old compile finding unchanged, new residual B failure: G3 is worse under the existing growth rule. Baseline-equal failures stay same. |
| Standalone CLI | Normal measure/diff path reports B as new and returns the non-keepable outcome, with independently checked displayed counts. |
| Recurse integration | Normal CLI, world/CodeDOM and policy enabled, optional G3 configured. Show that the snapshot veto now sees the growth. Also show which independent ordinary gates already veto it; do not misattribute their protection to G3. |
| Failure handling | Missing executable, cancellation, malformed JSON, unavailable build correlation and changing domain retain truthful unverified/unadmitted behavior. |
| Cost and usefulness | Event count, parse time, memory/retained bytes, false duplicate findings, false nonregressions, false reverts and unnecessary reruns. Compare identical captured complete inputs and the same independent expected failure census. |

Read the existing fixtures rather than using their names as proof of coverage. `TestReadGoTest` covers two packages but not a compile-plus-independent-package failure. Error-free admission tests construct snapshots and exercise comparison guards; they do not establish that this producer emitted every necessary row. Neither was run here. [Reader fixture][fixture] [Admission fixtures][admission-tests]

**Closure criterion:** a fresh supported G3 invocation cannot lose an independently failed package solely because another package emitted a compile diagnostic; its saved and projected census supports the existing no-worsening decision without duplicate dependency fallout. The result must hold through actual measurement and comparison, not only a constructed policy fact test.

## Pinned references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/architecture/AGENTS.md
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/architecture/campaign/INTERNALS.md#L138-L149
[reader]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree_readers.go#L20-L87
[compile]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/lintreport/lintreport.go#L36-L60
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree.go#L109-L151
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree.go#L249-L302
[count]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree.go#L179-L193
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree_snapshot.go#L100-L134
[compare]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree_snapshot.go#L199-L254
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/core/defaults/policy/gates.mg#L79-L154
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/cmd/nerd/cmd_gates.go#L37-L115
[render]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/cmd/nerd/cmd_gates.go#L149-L170
[recurse-cli]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/cmd/nerd/cmd_campaign_recurse.go#L348-L371
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/campaign/recurse_error_free.go#L27-L49
[after]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/campaign/recurse_cycle.go#L1331-L1444
[recurse-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/campaign/recurse_policy.go#L257-L280
[recurse-policy]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/core/defaults/policy/recurse.mg#L270-L329
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/cmd/nerd/chat/campaign_recurse.go#L218-L244
[ordinary]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/findings.go#L112-L184
[fixture]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree_test.go#L178-L199
[admission-tests]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/internal/gates/errorfree_admission_test.go
[gomod]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/go.mod#L1-L3
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/ASTRA_QUEUE.md
[prior-verdict]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/test-verdict-diagnostic-coverage.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/recurse-finding-census-completeness.md
[prior-subject]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/recurse-diagnostic-subject-identity.md
[prior-overlay]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/scoped-gate-overlay-evidence-conservation.md
[prior-domain]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/gate-comparison-domain-admission.md
[prior-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/097d67a6c0e4b79313ae689d8e0b541a7a65688a/Docs/audits/mission-errorfree-witness-receipts.md
[go-test]: https://pkg.go.dev/cmd/go@go1.27.3#hdr-Test_packages
[test-main]: https://pkg.go.dev/testing@go1.27.3#hdr-Main
[test2json]: https://pkg.go.dev/cmd/test2json#hdr-Output_Format

