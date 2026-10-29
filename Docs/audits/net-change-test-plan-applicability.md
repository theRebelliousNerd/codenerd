---
doc-class: deep-dive
subsystem: session
implementation-status: proposed
last-verified: 2026-10-29
verified-against: f84ce0df57b21de3725b8fa02d13e88f74192714
verification-method: static-source-inspection
supersedes: []
---

# Net change test plan applicability

## Conclusion

Gate ownership now follows net changes, but non-Go test planning still follows historical written paths. In a mixed-language turn, a file restored exactly to its original state can therefore create a language-specific test obligation contributed only by historical activity, not a remaining edit.

The strongest conditional case combines a real Python change with a touched-and-restored Rust file before the first post-edit planning round. If Cargo is unavailable, the planner can retain an unavailable Rust test obligation even when all selected Python checks pass. That is an applicability mismatch, not a fabricated failure or evidence that the Rust check ran.

The net-change policy was introduced in `51b803b11419e0f65bfaf22149a4d049563a6dc2` and is present in inspected main revision `f84ce0df57b21de3725b8fa02d13e88f74192714`. The proposed extension makes policy and planning consume the same revision-bound change measurement while preserving historical write and outcome evidence. No tests, application-model inference, training, scanner, or implementation were run. [Introducing change][introduction] [Net-change ownership][ownership] [Planner input][planner-input]

## Two different definitions of applicable work

`writeReverted` compares known original and current file state, including existence and exact bytes. Unknown preimages or unavailable current state do not establish a reversion. `assertTurnWrites` refreshes reversion evidence for historical written paths while preserving the write history. The new policy deliberately distinguishes changes remaining in the workspace from paths merely touched; restored paths contribute no gate ownership. Historical writes still matter to the finish obligation. [Reversion measurement][reversion] [Fact refresh][fact-refresh] [Ownership and finish][turn-rounds]

The non-Go test path uses a different input. `workspaceTestRun` calls `writtenNonGo` with `result.WrittenPaths`. That helper collects languages and directories from every relevant raw path, excluding Go and documentation, without consulting preimages or reversion evidence. The planner selects test gates whose language is absent or belongs to that collected language set. It processes both executable gates and unavailable gates. [Raw-path collection][raw-paths] [Plan selection][plan-selection]

This leaves a bounded precision gap. Policy's write classes are coarse: Python and Rust both fall under `/other`, rather than receiving separate per-language ownership facts. The kernel can correctly derive a `/test_run` obligation from remaining changes while the host populates that round from a broader historical language set. This is conservative over-obligation, not kernel authorization of a forbidden action. Retaining unavailable evidence is correct once an obligation applies. The question here is whether the restored component should have entered the active plan at all.

## Conditional witness

Assume a healthy, ordinary, authorized TUI write turn with known preimages:

1. The workspace has Python and Cargo project markers. Python and pytest are available; Cargo is absent from PATH.
2. A Python source change remains in the final tree.
3. A Rust file is edited and then restored byte-for-byte before the **first** post-edit planning round. Its path remains in `result.WrittenPaths`, and no intervening critic or repair edit reintroduces a Rust change before planning.
4. No `nerd.md` command override or independent global requirement mandates Rust testing, and no caller acceptance override applies. The components are independent, and the temporary Rust edit causes no external side effects.
5. Every selected Python check positively passes, rather than reporting no tests collected. Keep the selected directory identical, or otherwise ensure every selected Python node passes, so the older language/directory-union issue is not the cause.

Detection can independently find Python and Rust gates. Python testing is node-scoped; Cargo testing is workspace-scoped and language-qualified as Rust. Missing programs are retained in the detector's unavailable set rather than silently dropped. [Detection and overrides][detection] [Unavailable gate retention][unavailable-detection]

The remaining Python change keeps `/test_run` owed. Raw-path collection nevertheless includes Rust, so the plan can contain an unavailable `rust:test@.` obligation. `workspaceTestRun` asserts planned obligations and records their outcomes. `runPlannedCheck` handles an unavailable planned check without pretending to execute it. [Round ownership][ownership] [Plan recording][plan-recording] [Unavailable outcome][planned-outcome]

Compare a turn with the same final tree but only the Python path in its write history. Under these assumptions, the language-qualified Rust gate is not selected. The difference is the restored path's historical presence, not a remaining Rust change or a new global testing requirement.

## Why a passing Python run does not settle this case

Current policy intentionally does not let an unrelated passing run clear an unavailable or failed obligation. Such an obligation needs qualifying later evidence for its own identity. By contrast, selected passing evidence can discharge certain uncovered or no-tests conditions. That distinction makes **unavailable Rust** the useful witness; an uncovered-only example would overstate the obstruction. [Outcome matching][outcome-policy]

An unmet owed obligation prevents the default verified-turn derivation, and policy can expose a missing-evidence label for the unavailable test. However, a caller acceptance override exists, and other gates remain independent. The source does not justify claiming inevitable overall failure, an endless repair loop, or a guaranteed refusal to finish. [Unmet obligations][unmet] [Default verification and override][verification-policy] [Missing-evidence labels][missing-labels]

Repair behavior is also bounded. The test round can log that a plan is not repairable and return without a Go error. Open-obligation logic excludes passed, unavailable, and indeterminate outcomes; when all selected Python checks passed, only unavailable Rust remains, and no separate model-run failure warrants repair, the planner need not request another model repair. The outcome is truthful missing evidence attached to an arguably inapplicable obligation, not a false process result. [Round handling][test-round] [Repairability][repairability] [Open obligations][open-obligations]

## Reachability and counterevidence

This is a serving session path, not only a planner unit-test seam. Chat's verified mutation route calls `VerifyWithRetry`, which runs attempts through `ExecuteObservedWithContext`. For policy-admitted inline work, the JIT executor calls `ProcessWithIntent`; its tool loop then reaches the shared post-edit driver. This trace does not predict which edits a model will choose. [Chat mutation route][chat-route] [Verified attempt dispatch][verified-attempt] [Inline execution][inline-execution] [Tool-loop entry][tool-loop-entry]

The write path guards execution before capturing preimages, preserves the first preimage, and appends historical written paths with deduplication. That makes both exact restoration evidence and retained write history plausible within the same admitted turn. [Write tracking][write-tracking]

The shared post-edit driver dispatches `/test_run` to `verifyAndRepairTestRun`. Before querying the next policy round, it refreshes write facts through `assertTurnWrites`. The mismatch therefore persists even when the new reversion facts are fresh: the subsequent host planner independently reads the historical path list. [Round dispatch][round-dispatch] [Write refresh before selection][round-selection]

The witness is intentionally before the first planning round. It does not depend on stale obligations retained from an earlier run. A turn whose changes are all reverted is also not this witness: no remaining non-Go change needs to trigger this test round, although historical writes still owe finishing work. A turn with only a remaining Go change is not interchangeable with the Python case.

Existing tests give useful counterevidence. Net-write tests cover reverted Mangle edits, re-editing, created-then-removed files, and missing preimages. Test-plan tests preserve unavailable obligations against unrelated passing evidence, cover the more permissive uncovered/language-free cases, and invalidate receipts after later writes. These controls were inspected, not executed; none establishes mixed net-change applicability for the new ownership policy. [Net-write controls][net-tests] [Unavailable controls][unavailable-tests] [Coverage controls][coverage-tests] [Later-write control][later-write-test]

A genuine remaining Rust change must still retain its unavailable requirement. Likewise, a global or dependency-driven Rust obligation may legitimately survive an unchanged Rust file. A workspace-scoped Rust check is still language-qualified; `ScopeAll` does not make it language-free. Filtering by extension alone is not a sufficient replacement contract.

## Relationship to earlier audits

The earlier test-evidence audit identified language/directory unions and addressed preservation of unavailable evidence. This finding does not reopen that repair: it depends on the current unavailable-preservation behavior working correctly, while identifying a newly divergent applicability input after net-change ownership was introduced. The proposed change should not let unrelated successful checks erase real missing evidence. [Earlier applicability concern][predecessor-union] [Earlier unavailable-evidence concern][predecessor-unavailable]

A source-only screen of 92 current main audit documents found no exact net-change/reverted-test-plan finding. The research lane at `83aa2027ee122c01f532bb0b0e33106d391af185` adds package-selector and generic-merge work, not this issue. The relevant planner and runner blobs were unchanged in three selected active lanes; this was not an exhaustive pull-request audit.

## Proposed revision-consistent applicability

Create one host-owned, revision-bound change snapshot for both policy facts and test-plan applicability. Preserve `result.WrittenPaths` as historical provenance; do not rewrite it to conceal earlier edits. Expose confirmed net changes, confirmed reversions, and uncertainty separately.

Build the language-specific plan from that shared measurement, while retaining explicit global checks and dependencies required by the task or project contract. Unknown preimages and failed current-state reads must remain conservative rather than being silently classified as unchanged. Re-editing a restored file must invalidate the snapshot and restore its applicable obligations.

Plan generations need explicit lifecycle semantics. If a later revision makes an earlier obligation inapplicable, record the reason and retire it from the active generation without deleting its historical failed or unavailable outcome. Do not silently forget failed checks merely because a path was restored. Conversely, do not carry an obsolete language requirement forward solely because its path once appeared in the write ledger.

The host should measure filesystem state and preserve provenance; policy should own applicability, acceptable evidence, and authorization. This aligns with the session's stated control split and revision-bound proof contract. It does not require a new model call or broad permission changes. [Session control boundary][session-guidance] [Revision-bound contract][revision-guidance]

## Cost and future evaluation

Existing reversion measurement already compares file state. Reusing a snapshot may avoid redundant reads; independently recomputing it could add I/O proportional to the inspected bytes. Track path count, byte count, snapshot lifetime, invalidations, and trace storage. Removing irrelevant checks may reduce process work or unavailability holds, but no performance or token savings have been measured.

Future evaluation should compare current behavior, an explicitly historical conservative policy, and the proposed net-change policy with all real global requirements preserved. Use paired turns with identical final trees but different write histories, passing them through the normal post-edit path with deterministic requests or a deterministic model double.

Include the Python-change/Rust-restore witness; Python-only history; genuine Rust net changes that retain unavailable obligations; all-reverted turns with no `/test_run` but a remaining `/finish`; Go-only remaining changes; restored-then-re-edited files; created-then-removed files; unknown preimages and read errors that retain conservative selection; global and language-free checks; dependency obligations; and explicit command overrides. Include plans created before a restoration to test active-generation retirement separately from the first-round witness. Confirm that later writes invalidate post-write receipts, that no-tests-collected and uncovered outcomes retain their distinct policy behavior, and that unrelated passes still cannot clear genuinely applicable unavailable evidence.

Capture change snapshots, selected obligation IDs, applicability reasons, outcomes, policy facts, repair rounds, process count, elapsed time, bytes read, and model tokens. Compare missing and excess obligations, not just final completion. Authorization denials must remain denials, genuine missing tools must remain truthfully unavailable, and acceptance overrides must be recorded separately from default verification. All evaluation remains future work.

[introduction]: https://github.com/theRebelliousNerd/codenerd/commit/51b803b11419e0f65bfaf22149a4d049563a6dc2
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/defaults/policy/coder_safety.mg#L466-L495
[planner-input]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_run.go#L43-L61
[reversion]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/preimage.go#L115-L132
[fact-refresh]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/executor.go#L2551-L2586
[turn-rounds]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/defaults/policy/turn_rounds.mg#L50-L78
[raw-paths]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_run.go#L155-L182
[plan-selection]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_plan.go#L73-L129
[detection]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/gates/detect.go#L19-L153
[unavailable-detection]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/gates/gates.go#L154-L195
[plan-recording]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_run.go#L66-L87
[planned-outcome]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_run.go#L112-L132
[outcome-policy]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/defaults/policy/coder_safety.mg#L620-L636
[unmet]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/defaults/policy/coder_safety.mg#L642-L649
[verification-policy]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/defaults/policy/coder_safety.mg#L745-L746
[missing-labels]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/defaults/policy/coder_safety.mg#L781-L787
[test-round]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/test_run_round.go#L36-L60
[repairability]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/test_run_round.go#L118-L130
[open-obligations]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_plan.go#L173-L176
[round-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/executor_tools.go#L520-L563
[round-selection]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/executor_tools.go#L599-L636
[net-tests]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/net_write_test.go#L27-L87
[unavailable-tests]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_plan_test.go#L133-L163
[coverage-tests]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_plan_test.go#L254-L286
[later-write-test]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/workspace_test_plan_test.go#L299-L313
[predecessor-union]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/Docs/audits/test-evidence-capability.md#L213-L217
[predecessor-unavailable]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/Docs/audits/test-evidence-capability.md#L190-L205
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/agents.md#L3-L26
[revision-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/agents.md#L96-L105
[write-tracking]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/executor_tools.go#L1028-L1141
[chat-route]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/cmd/nerd/chat/process.go#L555-L571
[verified-attempt]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/verification/verifier.go#L283-L299
[inline-execution]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/task_executor.go#L395-L441
[tool-loop-entry]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/session/executor.go#L1441-L1452

