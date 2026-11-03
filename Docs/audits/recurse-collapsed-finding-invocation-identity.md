---
doc-class: deep-dive
subsystem: campaign-recurse
implementation-status: proposed-refinement
last-verified: 2026-11-03
verified-against: add4ec711cf3e040107918469b08410a7917c572
verification-method: static-source-inspection
supersedes: []
---

# Collapsed recurse nodes lose the selected finding's invocation

## Decision and evidence boundary

Preserve a selected finding's producing invocation when a recurse node contains several source directories. The current measurement combines every directory's findings into one node result, while retaining only one directory's command. A later finding can consequently be handed a sibling directory's command as its own acceptance witness. Both the task's governed `run_check` and the campaign's independent acceptance round then execute that wrong command faithfully. [Measurement][measure] [Selection handoff][attempt] [Acceptance construction][adapter] [Task handoff][task-check]

This is a source-demonstrable **finding-to-invocation mismatch**, not a measured incident or a claim of unconditional unsafe adoption. The outer ratchet still reruns the node's checks and evaluates the selected finding. The immediate defect is that the inner repair obligation and the purported witness disagree; unnecessary remediation, unrelated work and resource consumption are conditional consequences. Mission records also preserve several invocations' concatenated output under one command, losing the evidence needed to distinguish their individual outcomes. [Outer measurement][remeasure] [Outer target][target] [Mission records][record]

The review is pinned to public main [`add4ec711cf3e040107918469b08410a7917c572`][baseline], resolved and inspected on November 3, 2026. No repository program, model, build, test, benchmark, gate or failure injection was run. The examples are conditional source derivations. No implementation, runtime reproduction, production frequency or token/performance improvement is claimed.

The affected case is one **node-scoped gate**, run separately for two or more directories of a collapsed import component, with findings from a directory other than the invocation whose command survived aggregation. It does not require two different gates, truncated output, a finding-cap overflow, lost environment variables, unusual passing exits, a stale source snapshot, a namespace collision, or optional error-free groups.

## Existing contract and ordinary reachability

Root guidance puts executive obligations in the kernel and asks for production-path verification. Campaign guidance distinguishes host-executed, snapshot-bound evidence from model prose. The attempt adapter explicitly describes its contract: a fix attempt carries the finding's own check, copied from the reporting run, into campaign acceptance. The architecture contract calls for preserving intended behavior and correcting the implementation rather than weakening the requirement to match current code. [Root guidance][guidance] [Campaign guidance][campaign-guidance] [Attempt contract][adapter] [Architecture guidance][architecture]

Multi-directory nodes are a supported production result. `DeriveWorkspaceDAG` reads the workspace's language imports, collapses strongly connected components, and appends cross-cutting stages. The collapse sorts each component's directories and retains them all in `SubsystemNode.Paths`. Python imports can form such a component without an invalid Go production-import cycle. The existing `TestDeriveWorkspaceDAG_AnImportCycleBecomesOneNode` fixture uses mutually importing `a` and `b` packages and expects `a+b`; it was read, not run. [DAG entry][dag] [Component construction][collapse] [Existing cycle fixture][cycle-test]

This is not limited to a contrived caller providing a multi-path node. Python detection supplies node-scoped `compileall` and pytest commands using `{node}`. Owner-declared `gates:` entries also support node scope. `Gate.ForNode` expands the directory argument, and `gates.Command` builds that exact invocation. Thus the same gate can legitimately execute as one command for `a` and another for `b`. [Detection][detect] [Declarations][declared] [Expansion][expand] [Runner][runner]

Both ordinary CLI recurse and mission recurse use the shared loop and `RecurseAttemptCampaign` adapter. The defect occurs before the mission's deliberation and before the task executor, so a more capable model or correct later command execution cannot restore the lost producer identity automatically. [CLI wiring][cli] [Attempt creation][attempt-object]

## Where the identity is lost

The producer initially has the necessary information. A `gates.Result` contains the expanded argv, the individual node/path, exit, pass state, output, execution error and duration. `Findings` receives that individual result before aggregation. Its stable identity is built from gate, target and, for diagnostics, normalized message. A finding does not retain an invocation handle or argv. [Result and runner][runner] [Finding shape][findings] [Identity construction][finding-id]

In `measure`, the driver iterates the collapsed node's `Paths` and runs the gate for each one. It changes each returned finding's `Node` to the collapsed node ID and appends all those findings to one slice. The original individual result's node is not retained alongside each finding.

The aggregation then:

1. Stores the first invocation's complete `Result`.
2. Appends later output strings, separated by a newline.
3. Changes retained argv and exit only when the aggregate had been passing and the new invocation is not passing.
4. Conjoins pass state and retains an execution error if any invocation has one.

With two normally completed failures, A followed by B, the aggregate therefore contains A's command and exit, both outputs and both findings. Later B cannot replace A's command because the aggregate is already nonpassing. More generally this is the first nonpassing invocation, not necessarily the first process that returned an ordinary failure; an unverified first invocation also makes the aggregate nonpassing. The report's main witness uses ordinary failures and does not depend on that extension. [Aggregation][measure]

This summary is useful for a component-level verdict, but it is not one process observation. The state shape contains only one `gates.Result` beside the union of findings. `sourceOf` checks whether the aggregate's findings include the selected ID and returns that entire aggregate. It cannot retrieve the individual invocation that produced B. `attempt` copies the aggregate's argv as B's `Check`, alongside the gate's environment and passing-exit configuration. [State shape][state] [Lookup][source] [Handoff][attempt]

The recently repaired environment and pass-rule copying therefore remain correct relative to the **wrong selected invocation**. This is not a request to undo those controls. Exact downstream copying cannot establish an association discarded upstream.

## Finding selection does not rescue the association

Directory execution order and finding selection order are different:

- Component paths are sorted lexicographically before measurement.
- Policy ranks candidate findings by kind and related facts, excludes attempted/stalled/refused findings, and does not prefer the invocation whose command happened to survive.
- Among equal best ranks, the Go adapter selects the lowest stable finding ID. Those IDs hash gate/target/signature rather than encode path order. [Component order][collapse] [Policy][selection] [ID construction][finding-id] [Selection adapter][next] [Sorted query results][selection-order]

A later directory's finding can therefore be selected first. No exact sample hash is needed for the stronger alternative: if A is attempted and remains open without stopping the run, the visit marks A attempted and can select still-unattempted B next. The aggregate still carries A's failing command. The normal visit loop continues after a settled attempt and repopulates the open candidates; it does not require the first directory to become green before another finding in the component may be selected. [Attempt marking][next] [Visit loop][visit]

The issue is not nondeterministic iteration. Deterministic directory order and deterministic finding order can deterministically disagree because they order different identities.

## Conditional production witness

Use a clean Git workspace with two ordinary Python source directories, `a` and `b`, whose readable imports form a component. Avoid reserved synthetic stage names and other unrelated graph defects. Let one admitted node-scoped check G operate only on the directory passed through `{node}`. It reports small, recognized positioned diagnostics and exits nonzero for a genuine defect in that directory.

A declared checker is sufficient to isolate the invocation contract. The automatically detected Python commands independently establish that per-directory node invocations are normal. This witness does not assume every pytest command is independent: a check that deliberately loads both directories is a separate control.

Stipulate the following ordinary observations:

- `G(a)` reports finding A and fails.
- `G(b)` reports distinct finding B and fails.
- B is the selected eligible finding, either by stable-ID order or after a settled unsuccessful attempt on A.
- The source, gate declaration, environment and passing-exit rule remain unchanged during measurement and handoff.

The derived attempt has **finding B but check G(a)**. That mismatch exists before any model executes or any repair succeeds. `RecurseAttemptCampaign` then asks to fix B so that G(a) passes and installs G(a) as acceptance. The task dispatcher copies that acceptance into its governed check context. `run_check` refuses model arguments and executes that context-owned argv; the independent acceptance round later copies the same argv into tactile execution. [Handoff][attempt] [Attempt task and acceptance][adapter] [Task context][task-check] [Governed execution][run-check] [Acceptance execution][acceptance]

Now consider an authorized change that fixes B while A remains broken. A correctly rerun G(b) would pass; the supplied G(a) still fails. The worker may receive continued check obligations or campaign remediation for a different defect than the selected work item. It may also fix A incidentally because the collapsed node admits both source directories. Neither possibility repairs the original finding-to-check claim.

The outer ratchet is a material mitigation. It remeasures all applicable node gates after changes and checks whether the selected B finding remains in the aggregate. It can recognize a real B repair even if the executor returned an error. Conversely, a worker that merely makes A green while leaving B unresolved does not automatically satisfy the outer target test. Accordingly, the report establishes a wrong inner witness and ambiguous per-invocation history, **not** that every affected attempt is reverted, every attempt stalls, or a wrong change is kept. [Remeasurement][remeasure] [Target test][target] [Executor error treatment][mission-support]

## Mission evidence consequence

Mission preparation copies the attempt's `Check` into its receipt. The check sidecar then emits one observation per aggregate state key, with one command, one exit, one status/error, and the concatenated output. It has no list of individual component invocations or finding-to-observation links. A B receipt can consequently name G(a), while its before/after sidecar contains text from both G(a) and G(b) without corresponding individual outcomes. [Receipt preparation][receipt] [Sidecar][record]

A stable workspace snapshot does not recover this identity. `measure` brackets the entire multi-invocation sweep with snapshots and can bind the aggregate to unchanged source. That establishes source stability over the sweep, not that every output line was produced by the one retained command. [Snapshot bracket][measure]

This is not the existing optional error-free witness omission. Here ordinary target keys exist, the ordinary aggregate can be source-bound, and outputs from both invocations can be retained. What is missing is the identity of each producing invocation and its association with the selected finding.

Nor does this alone make historical `/supported_gate_deficit` false. If complete ordinary remeasurement really shows B gone, that support can be justified by the aggregate evidence even while the archived command attribution is incomplete. Mission policy still distinguishes a kept, resolved, changed-source observation from arbitrary strategic requirement satisfaction. Preserve that distinction rather than escalating a receipt-fidelity defect into an unsupported mission-completion claim. [Support calculation][mission-support] [Mission policy][mission-policy]

## Safeguards and negative controls

The correction must preserve:

- Single-directory nodes and workspace-scoped gates, which have one invocation here.
- Correct first-failure selection when every relevant finding really belongs to that invocation.
- The all-components pass requirement and unverified-if-any-component-is-unverified rule.
- Existing gate environment and allowed-exit contracts, immutable/copying behavior, tactile authorization, tool argument closure, workspace binding and post-write freshness.
- Kernel candidate ranking, visit attempts, stalls and refusal behavior.
- Outer target remeasurement, independent regression checks, write attribution, mission protection, compensation and adoption controls.
- Historical source binding and the refusal to claim arbitrary requirement satisfaction from gate support.

Useful controls include a first component that passes followed by exactly one failing component, a single component path, a command that intentionally checks both components, all-green measurements, a missing executable, cancellation, nonzero declared passing exits, and a later invocation with a launch failure. These separate necessary aggregate behavior from inappropriate use of one summarized result as a selected finding's exact witness.

## Nonduplication screen

The source review read the current root/campaign/architecture guidance, audit queue and the neighboring reports listed below. Searches of these reports covered collapsed/multi-path nodes, first failures, `sourceOf`, sibling commands and invocation ownership. The GitHub code-search index did not return `sourceOf` references under `Docs/audits`, despite an inspected report mentioning it, so that index result is not treated as a comprehensive negative.

- **Recurse coverage gate identity** covers reducing several distinct test-gate metrics to the first parsable coverage value. It acknowledges collapsed-output aggregation as a control but does not trace a selected finding to a sibling's acceptance argv. This report uses one gate and no coverage percentages. [Coverage audit][prior-coverage]
- **Recurse finding census completeness** covers the 50-finding work cap entering counts and absence checks. It mentions per-invocation aggregation. The present witness uses two short recognized findings, both retained. [Census audit][prior-census]
- **Recurse gate contract continuity** repairs environment and allowed-exit information after the reporting check is selected. The current defect precedes that handoff and persists with those fields intact. [Contract audit][prior-contract]
- **Mission error-free witness receipts** covers discarded focused-group observations and absent ordinary-map keys. The present path is ordinary node-gate measurement with a valid aggregate key. [Mission audit][prior-mission]
- **Workspace test receipt invocation identity** concerns reusing a previous result for a different planned execution. Here the wrong command is chosen and executed afresh; no cached receipt substitution is needed. [Receipt audit][prior-receipt]
- **Recurse gate subsumption**, **workspace test witness**, **Go test dependency closure**, and **node namespace identity** address candidate suppression, missing scheduled tests, graph edges, and node-name collisions. This report assumes the component has already been validly derived and selected. [Subsumption][prior-subsumption] [Test scheduling][prior-tests] [Dependency closure][prior-dependencies] [Node namespace][prior-nodes]
- **Campaign acceptance case identity** addresses generated test obligations for actual modified artifacts, rather than the invocation selected for a recurse gate finding. [Case identity][prior-cases]

No exact prior report of this selected-finding-to-sibling-invocation path was found in that bounded screen. This is not a claim that all repository history, private discussions or every report was exhaustively searched.

## Proposed contract

Update the campaign/gates specifications and mission evidence shape before implementation.

Retain two different objects: the component-level aggregate used for broad gating, and immutable individual invocation observations used for attribution. Each observation needs its source gate declaration, canonical workspace, executed path/domain, exact argv and pass semantics, execution result and output identity. Preserve environment through the existing private runtime contract rather than exposing values in prompts, JSON receipts or kernel facts.

Associate findings with the observation or observations that actually produced them before overriding their scheduling node. An execution domain and a scheduling node are not the same identity. A diagnostic may name an imported dependency outside the command's own directory, so guessing the invocation from the diagnostic target path is not a sound replacement.

Define multiple-producer semantics explicitly. The same stable finding can appear in more than one invocation; retaining a many-to-many relation avoids arbitrarily assigning it to the first match. Do not casually change the stable finding ID to include argv, since that also changes retries, stall history and regressions. If policy chooses one compatible witness, make that choice explicit. If it requires all producers or an explicitly component-wide witness, represent that obligation as such.

At attempt construction, require the selected finding and its check descriptor to refer to the admitted observation. If provenance is missing or incompatible, remeasure or defer with an explicit reason; never substitute the aggregate's first failing command and call it the selected finding's own check.

Mangle should retain authority over eligibility, completion and acceptance of typed observations. Go should preserve producer identity and execute admitted commands. Neither reconstructing a witness from model prose nor giving the model an arbitrary command argument is an appropriate fix.

Mission sidecars should retain individual invocation records plus a separately labeled aggregate. Keep exact raw exits and errors; never replace them with a synthesized scalar that looks like a real process exit. Preserve bounded-output/completeness status and durable output references. A source-stable aggregate can point to the children that justify it. Legacy aggregate-only receipts remain historical and incomplete for exact invocation attribution; migration must not invent the missing records.

Do not “repair” the mismatch by requiring the model to fix every component defect in one selected-finding task, dropping the inner check, combining commands through an unrestricted shell, serializing all work by directory instead of policy, suppressing B, or weakening the outer ratchet.

## Unrun evaluation and costs

First add deterministic production-route fixtures that derive a real component from source imports, detect/read a node-scoped gate, collect actual per-directory results, let shipped policy select B, and inspect the attempted command. The minimal failing-before assertion is **finding B must not carry G(a)**. That can establish the handoff defect without claiming the full repair workflow completed.

Then, when execution is authorized, drive the real CLI/chat adapters, observed task path, governed `run_check`, campaign acceptance and outer ratchet with fixed scripted decisions. For mission mode, reload the saved receipt and check sidecars. Use the normal world-model/CodeDOM context for complete-system evaluation. Constructor-only or injected-Execute fixtures do not prove full inner-gate behavior.

Required cases:

| Case | Checkable outcome |
|---|---|
| A and B fail; B selected first | B's admitted witness comes from B's producing invocation. |
| A remains open after its visit attempt; B selected next | A's retained failure does not donate its command to B. |
| B repaired, A still failing | B's local witness reflects B; outer aggregate still preserves A's failure and ordinary ratchet rules. |
| First path passes, second fails; one path; workspace gate | Existing correct one-invocation association remains. |
| Same diagnostic produced by several paths | Explicit multiple-producer semantics; no accidental first-match ownership. |
| Diagnostic points to a dependency outside the invocation directory | Attribution follows the actual producer, not a target-path guess. |
| Later failure, unavailable tool or cancellation | Preserve each actual exit/error and aggregate unverified semantics. |
| Environment/pass-rule variations and caller mutation | Existing contract continuity, private environment handling and copying stay intact. |
| Mission before/after source drift | No support from stale evidence; individual records remain honest observations. |
| Mission restart or legacy receipt | Exact per-invocation data survives reload; old aggregate-only data is not upgraded by inference. |
| Forbidden/out-of-scope writes or regression | Independent refusal, compensation and nonregression remain. |

Compare the pinned baseline, attribution-only correction and full per-invocation persistence while keeping models, declarations, permission boundaries, sources and checks fixed. Measure command/selected-finding agreement, spurious local rejection, remediation turns, successful authorized repairs and retention of genuine failures. Keep outer adoption outcomes separate from local witness agreement.

If a component has P paths and F retained finding occurrences, preserving observations and attribution adds O(P + F) metadata beyond the already collected output. The current code already executes P gate processes; exact attribution need not add subprocesses. Remeasurement for ambiguous provenance, a deliberate all-producer witness, or stronger source bracketing can add work and must be measured. Output storage, snapshot hashing, serialization, kernel facts, retained memory, process runtime, model/tool tokens and human reconciliation are costs, not free byproducts.

The existing cycle, environment/exit-continuity and pytest fixtures are useful controls, but the inspected continuity fixture has a single `store` source directory and does not establish association after collapsed-node aggregation. No test absence is claimed across the whole repository. [Cycle fixture][cycle-test] [Continuity fixture][continuity-test] [Pytest fixture][pytest-test]

Acceptance is narrowly checkable: a selected finding receives an admitted witness from its actual producer, component-wide checks keep their independent obligations, and mission history can distinguish every contributing invocation. That restores the stated contract without treating a correctly copied wrong command as trustworthy evidence.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/add4ec711cf3e040107918469b08410a7917c572
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/architecture/AGENTS.md
[dag]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_workspace.go#L26-L101
[collapse]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_workspace.go#L148-L234
[cycle-test]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_workspace_test.go#L185-L203
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/gates/detect.go#L134-L142
[declared]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/gates/detect.go#L51-L89
[expand]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/gates/gates.go#L129-L146
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/gates/run.go#L16-L99
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/gates/findings.go#L17-L38
[finding-id]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/gates/findings.go#L99-L130
[state]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L205-L220
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L708-L777
[source]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L1539-L1563
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L1057-L1084
[attempt-object]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L1139-L1156
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/core/defaults/policy/recurse.mg#L133-L255
[next]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_policy.go#L55-L100
[selection-order]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_policy.go#L353-L373
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L958-L1001
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_attempt.go#L34-L173
[task-check]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/orchestrator_task_handlers.go#L67-L149
[run-check]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/tools/shell/run_check.go#L77-L144
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/orchestrator_acceptance.go#L156-L224
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/cmd/nerd/cmd_campaign_recurse.go#L223-L267
[remeasure]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L1208-L1230
[target]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L1297-L1317
[mission-support]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle.go#L1321-L1339
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_mission.go#L216-L222
[record]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_mission_storage.go#L80-L121
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/core/defaults/policy/mission.mg#L26-L29
[continuity-test]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_contract_continuity_test.go#L39-L86
[pytest-test]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/internal/campaign/recurse_cycle_e2e_test.go#L443-L501
[prior-coverage]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-coverage-gate-identity.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-finding-census-completeness.md
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-gate-contract-continuity.md
[prior-mission]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/mission-errorfree-witness-receipts.md
[prior-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/workspace-test-receipt-invocation-identity.md
[prior-subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-gate-subsumption.md
[prior-tests]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-workspace-test-witness.md
[prior-dependencies]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-go-test-dependency-closure.md
[prior-nodes]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/recurse-node-namespace-identity.md
[prior-cases]: https://github.com/theRebelliousNerd/codenerd/blob/add4ec711cf3e040107918469b08410a7917c572/Docs/audits/campaign-acceptance-case-identity.md

