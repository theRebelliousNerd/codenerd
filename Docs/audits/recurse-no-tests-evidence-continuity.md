---
doc-class: deep-dive
subsystem: campaign-recurse
implementation-status: proposed-refinement
last-verified: 2026-11-03
verified-against: c9f7ac633cdea475e628bbfbbe892a8b725bb2ed
verification-method: static-source-inspection
continues: Docs/audits/test-evidence-capability.md
---

# Recurse drops the known distinction between a passing gate and no collected tests

## Finding and evidence boundary

Carry the detected test runner's `no_tests` disposition into recurse's ratchet and mission evidence. The shared gate type already distinguishes pytest exit 5 from an accepted invocation without a no-tests marker. The ordinary session test planner consumes that distinction. Recurse does not: a completed test process sets its `Tested` bit, and an empty-collection rerun removes all target findings and can set `TargetOK`. Stable source snapshots cannot establish that a missing target was exercised. [Gate contract][gate] [Session classifier][classify] [Outer ratchet][outer]

This is a concrete remaining consumer of the earlier test-evidence contract, not a new discovery that pytest can collect no tests. It is also not a proposal to call exit 5 a failed test or to undo declared successful-exit handling. A directory without tests need not be a defective directory. What it cannot provide is affirmative testing evidence, or a witness that an earlier failing test was repaired.

The review is pinned to public main `c9f7ac633cdea475e628bbfbbe892a8b725bb2ed`. It inspected current source, guidance, architecture and neighboring audits only. No repository code, tests, builds, gates, models or behavioral fixtures ran. The schedules below are conditional source derivations, not observed incidents. No implementation or performance result is claimed.

## Existing representation, lost at this consumer

`Gate.NoTestsExitCodes` expressly represents passing exits that measured no test and therefore do not cover the invocation's target. Detection assigns `{0,5}` to pytest's allowed exits and `{5}` to its no-test exits. `Result.Unverified` means an execution error, not empty collection. Consequently a normally completed detected pytest exit 5 has `Passed=true`, no execution error and `Gate.NoTests(5)=true`. [Detection][detect] [Gate contract][gate] [Runner][runner]

Among completed runs, a useful distinction is failed, accepted without a no-tests marker, and accepted with known empty collection. The session planner uses `NoTests` after the normal failure/error classifications and emits `/no_tests`, preserving this fact instead of conflating it with `/passed`. [Session classifier][classify]

Recurse stores the full gate result, so the information is available. It then projects only pass/fail/unverified in `gateRun.verdict`. `Findings` intentionally returns no findings for a passing result. At the outer post-change boundary:

1. Every test gate with no execution error sets `in.Tested=true`.
2. For the selected target key, no execution error plus absence of the old finding sets `in.TargetOK=true`.
3. Neither predicate consults `Gate.NoTests`.

The policy adapter turns those booleans into `recurse_ratchet_tested` and `/resolved`. Mangle then faithfully decides over the weakened observation. A target marked resolved satisfies the success arm; an improvement requires a moved metric plus the tested fact. Neither repairs the lost distinction. [Verdict projection][verdict] [Finding projection][findings] [Outer ratchet][outer] [Fact adapter][adapter] [Policy][ratchet]

## Primary conditional witness: the test still exists but is no longer collected

Use a normal Python workspace with detected pytest, a single-directory node `pkg`, one existing failing test in `pkg/test_widget.py`, an existing `pkg/conftest.py`, and a separate existing passing smoke-test directory. There is no collapsed component, no output truncation, no finding cap, no command mismatch, and no stale snapshot. The node is properly selected from its actual failing test.

Assume the collection-input path lies within the mechanically admitted write scope, and no owner-configured protected path or other independent host guard refuses it. This is not authorization to weaken testing: the mandatory recurse atom expressly forbids weakening, skipping or deleting a check, and its mission atom forbids changing acceptance standards to obtain a pass. The bad candidate below violates those instructions. Its surviving the model's work and any advisory review requires those participants to fail to reject it. The finding is the missing deterministic evidence defense if that happens, not a claim that the operation is permitted. [Mandatory attempt instructions][instructions]

The hypothetical host-observed change replaces an existing `collect_ignore=[]` assignment in `pkg/conftest.py` with `collect_ignore=["test_widget.py"]`, without changing line count. The failing test file, function, assertion and underlying defect remain untouched. A capable, compliant model should refuse this supposed repair; the host should also avoid treating its explicit empty-collection result as target evidence.

Pytest officially supports `collect_ignore` in `conftest.py` to exclude modules, and exit 5 denotes empty collection. With no other tests under this node, its same detected command now reports no collected tests. These external references support the runner semantics, not execution of this hypothetical CodeNERD fixture. [Pytest collection documentation](https://docs.pytest.org/en/stable/example/pythoncollection.html#customizing-test-collection) [Pytest exit codes](https://docs.pytest.org/en/stable/reference/exit-codes.html)

CodeNERD's static test counter still sees the same Python test functions in the same files. The source-line metric also need not change, because the assignment is replaced in place. Compileall can remain green. Thus the outer metric nonregression guard need not catch this loss of execution coverage. No test function is textually deleted; this is a collection exclusion, and it does not depend on lowering the static test count. [Metrics][metrics]

The source-derived target rerun is now passing and finding-free. Recurse supplies `/resolved`, despite the old failing test having no current execution observation. With the other ordinary ratchet conditions satisfied, it permits keep. [Finding projection][findings] [Outer ratchet][outer] [Policy][ratchet]

### Why this does not assume failed-task writes survive

Both production recurse doors execute the normal attempt campaign and return its `KeptWrites`. Failed task attempts undo their writes and are not added to that set. A claim that an inner `/no_tests` failure automatically leaves adoptable writes would be wrong. [CLI adapter][cli] [Chat adapter][chat] [Write settlement][writes]

There is instead a conditional mechanically eligible successful-task path, after the instruction-violating candidate escapes model/judge rejection. The governed target check and campaign acceptance copy the target's allowed exit codes, so exit 5 satisfies that command's existing pass rule. The ordinary session planner still correctly labels its own empty collection `/no_tests`; its policy explicitly permits a separately passing model-selected test process after the last write to meet that generic obligation. The existing smoke suite can provide that process. Other ordinary task obligations must also pass. These facts do not authorize skipping the target test. The independently passing smoke suite is real test evidence, but it is not evidence that the selected `pkg/test_widget.py` failure was repaired or even rerun. [Attempt contract][attempt] [Campaign acceptance][acceptance] [Session fallback rule][fallback]

Accordingly, the main claim is not that no tests run anywhere, that every affected task completes, that the model is authorized to suppress tests, or that all independent checks are bypassed. It is that the outer target-specific witness loses a known empty-collection disposition. Neither correct generic session testing nor correct copying of allowed exits restores that missing target relationship. The scenario remains conditional on a bad candidate and unsuccessful semantic rejection; this review does not measure their frequency.

## Mission evidence consequence and limits

Mission mode makes the same outer decision and additionally derives `missionWitness=/resolved` when `TargetOK` is true and the before/after measurements have the required source-snapshot identity. Mission policy can then emit `/supported_gate_deficit` for a kept, changed-source attempt. No collected-test or target-execution observation participates in that join. [Mission witness][mission-witness] [Mission policy][mission-policy]

Source stability is a real protection: the changed collection rule can be stable throughout the after run. It establishes exactly which source produced empty collection, not whether that source repaired the earlier failing assertion. The mission report still does not claim arbitrary North Star satisfaction, and this audit must not upgrade its narrow status into such a claim.

The mission version of the collection-input example requires that the file passes mission scope/protection checks and that model/judge review fails to reject the forbidden weakening. The current automatic verification-input collector includes existing Go tests, module/work files, `nerd.md` and `testdata`, not arbitrary Python collection inputs. Configured protected paths and ordinary host permissions remain independent controls, and the mandatory prompt prohibition still applies to files not enumerated by that collector. That uneven inventory is already documented by the portable-crucible proposal and is **not claimed as a new finding here**. The new, narrower point is that even the available positive `NoTests` signal is dropped at the target/evidence consumer. [Oracle collector][oracle] [Mission write guard][write-guard] [Mandatory attempt instructions][instructions] [Portable proposal][portable]

## Separate improvement case

A source node with tests held in another directory can legitimately produce pytest exit 5 before and after an improvement. A source-line reduction still gets `recurse_ratchet_tested` from that empty invocation, enabling the improvement success arm if the other conditions hold. The inner task may have obtained good independent test evidence, or may be prevented from completing without it. The outer receipt does not distinguish those cases. This is a provenance and evidence-admission gap, not proof that every such improvement is bad. [Outer ratchet][outer] [Improvement policy][ratchet]

## Nonduplication and existing controls

- **Test evidence capability, W3:** established the no-tests distinction and records its session-planner repair. This is a continuation into a different production consumer, not reopening that fixed session defect. [Prior W3][prior]
- **Recurse workspace test witness:** concerns no scheduled test-gate observation for a workspace-scoped check. Here a node-scoped check runs and returns a known empty collection. [Witness audit][witness]
- **Test verdict diagnostic coverage:** concerns Go terminal failures omitted from diagnostic-derived failure identities in baseline exemptions. Here pytest explicitly reports no collected tests and no parsing ambiguity is needed. [Diagnostic audit][diagnostic]
- **Recurse finding census completeness:** concerns a 50-finding cap used as a complete census. Here there is one prior finding and a true empty collection; no cap or lost output is involved. [Census audit][census]
- **Recurse gate contract continuity:** preserves environment and permitted exit codes. Keep that repair. Process pass and test coverage remain distinct obligations. [Contract audit][contract]
- **Portable recurse deliberation/crucible:** already proposes explicit no-tests semantics and broader oracle protection. This report supplies the exact current projection and production handoff needing that contract. [Portable proposal][portable]

The bounded screen found no exact existing report of this outer recurse target-to-empty-collection path. It does not establish novelty across all private discussion or repository history.

## Proposed contract and unrun evaluation

Keep ordinary process pass semantics. Carry a separate typed evidence disposition through individual invocation results, aggregate gate observations, attempt acceptance descriptors, ratchet facts and mission sidecars. A no-tests result must not by itself establish that an earlier test finding resolved or that an improvement was tested. Mangle should decide what alternate compatible witness satisfies the obligation; Go should preserve the measured disposition and attribution.

For a prior failing-test target, require current evidence that exercises the target or an explicitly admitted replacement contract. Unchanged source test counts and a generic passing process are insufficient target witnesses. If the check becomes nonapplicable because the project intentionally retires or moves a test, record that disposition and its authorization instead of relabeling it a repaired failure. Do not turn every testless source directory into a failure.

An implementation should be evaluated through actual detector, gate runner, attempt campaign, session tool policy, independent campaign acceptance, outer ratchet and mission receipt persistence. Constructor-only fixtures demonstrate projection but not the full inner flow. Proposed cases, all unrun:

- Prior failing test, then empty collection while its function and assertion remain: no target-resolved or supported-deficit claim from that run.
- Same case with a separate passing smoke process: preserve that real evidence without attributing it to the missing target.
- Actual target repaired and executed: keep the legitimate success.
- Existing testless source node: record no-tests without inventing a failed test; require a compatible independent improvement witness.
- Custom project-declared successful exit 5 without detector-defined no-tests semantics: preserve the owner's declared meaning.
- Failed task attempt: rollback and `KeptWrites` exclusion remain intact.
- Protected collection input, unauthorized write, cancellation, missing executable, real regression and no-change attempt: retain existing refusals.
- Collapsed nodes with mixed executing and no-tests children: retain per-invocation domain evidence, rather than reading one summarized exit as coverage of every child.
- Mission drift, stale baseline, or unknown provenance: no source-bound support by inference.

Measure selected-target execution agreement, false resolution, retained legitimate fixes, conservative holds, rerun count, model/tool tokens, process time, retained bytes and operator recovery effort. Adding a typed disposition is small metadata work; stronger target or replacement-witness admission can cost extra checks and must be measured. No token or reliability improvement is claimed without that evaluation.

[gate]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/gates/gates.go#L76-L112
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/gates/detect.go#L134-L141
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/gates/run.go#L16-L80
[classify]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/session/workspace_test_plan.go#L131-L156
[verdict]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/recurse_cycle.go#L205-L220
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/gates/findings.go#L98-L107
[outer]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/recurse_cycle.go#L1208-L1317
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/recurse_policy.go#L179-L232
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/core/defaults/policy/recurse.mg#L260-L329
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/gates/metrics.go#L38-L130
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/cmd/nerd/cmd_campaign_recurse.go#L225-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/cmd/nerd/chat/campaign_recurse.go#L227-L265
[writes]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/orchestrator_attempt_writes.go#L106-L182
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/recurse_attempt.go#L97-L115
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/orchestrator_acceptance.go#L156-L225
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/core/defaults/policy/coder_safety.mg#L620-L636
[mission-witness]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/recurse_cycle.go#L1321-L1339
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/core/defaults/policy/mission.mg#L26-L29
[oracle]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/evidence/change.go#L376-L423
[write-guard]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/campaign/recurse_mission_scope.go#L13-L79
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/Docs/audits/test-evidence-capability.md
[witness]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/Docs/audits/recurse-workspace-test-witness.md
[diagnostic]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/Docs/audits/test-verdict-diagnostic-coverage.md
[census]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/Docs/audits/recurse-finding-census-completeness.md
[contract]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/Docs/audits/recurse-gate-contract-continuity.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/Docs/audits/portable-recurse-deliberation-crucible.md
[instructions]: https://github.com/theRebelliousNerd/codenerd/blob/c9f7ac633cdea475e628bbfbbe892a8b725bb2ed/internal/prompt/atoms/campaign/recurse_attempt.yaml#L10-L39
