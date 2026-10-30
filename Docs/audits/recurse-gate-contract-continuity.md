---
doc-class: deep-dive
subsystem: campaign
implementation-status: implemented
evidence-status: source-only
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
scope: recurse gate contract through campaign acceptance and session run_check
supersedes: []
---

# Recurse gate contracts must survive the acceptance handoff

The findings below describe the pinned baseline. The current implementation and
source-based rulings are recorded in **As built**; runtime verification is pending.

## Conclusion and boundary

A recurse fix starts with a measured workspace gate, but two parts of that gate's contract do not reach all of its consumers. The declared environment is absent from the attempt's campaign acceptance descriptor. The declared passing exit codes reach campaign acceptance, but disappear when that campaign hands its check to an editing task's `run_check`. Consequently the task can be required to satisfy a differently configured command, or be told that an exit its own gate explicitly accepts is a failure.

The recommendation is to preserve the admitted check contract through the existing gate → recurse attempt → campaign → task → execution → host evidence path. This is a consumer-conformance correction, not a new verifier or scheduler. The model still solves the problem; host adapters preserve execution facts; Mangle derives whether the declared obligation is met.

All source references are pinned to public main [`fef98230c20a5751e4c32a2fd2757832a9d39cdc`][baseline], rechecked on 2026-10-30 at 06:12 UTC. This review read source and test definitions only. No repository program, test, model inference, benchmark, or failure injection was run. These are conditional source deductions, not observed incidents, measured frequencies, or claims that every recurse attempt fails.

**The established effects are narrower than false adoption.** An accepted nonzero exit can be rejected by the task's local `/check` gate and displayed as a failed tool result. A declared environment can be missing from the task and campaign check executions. Extra continuation, remediation or eventual blockage is conditional on subsequent execution. The outer recurse ratchet independently remeasures gates after the executor returns; this report does not establish a false keep or a regression escaping that ratchet.

## Related primary guidance

[Bazel’s hermeticity guidance](https://bazel.build/basics/hermeticity) explains why build and test results depend on declared inputs and environment. It motivates comparing the actual command contracts across these consumers; it does not prove a CodeNERD failure or require adopting Bazel.

## What is already preserved, and what is lost

| Contract component | Reporting workspace gate | Campaign acceptance | Task `run_check` and session `/check` |
|---|---|---|---|
| Declared command, expanded node placeholders | Present | Copied from reporting result | Copied from acceptance |
| Workspace directory | Gate executes at workspace root | Explicit workspace root | Explicit workspace root |
| `nerd.md commands.env` | Overlaid by gate command builder | Not carried in `Acceptance` or execution command | Not carried in `CampaignCheck` or execution command |
| `OKExitCodes` | Used by `Gate.Passed` | Copied and correctly used | Not carried; policy and progress classification use zero-only success |
| Latest run after final write | Not a turn-sequence concern | Separate campaign round | Existing sequence qualification retained |
| Permissions and other verification | Independent obligations | Independent obligations | Independent obligations |

The environment and exit-code defects are related losses along the same handoff, but they are **separate mechanisms**. The exit-code witness below needs no environment mismatch. The environment witness uses zero-only success and needs no nonzero-pass declaration. Fixing one does not fix the other.

The [mission-driven forever-improvement proposal][mission] already calls for accountable, revision-aware links between obligations and evidence. This note identifies a smaller executable contract inconsistency in the existing loop; it does not claim the mission bridge as new. The [workspace-test-receipt-invocation-identity audit][receipt-audit] concerns substituting an earlier typed-test receipt for a planned workspace check. Here the command is actually executed again through campaign `run_check` or acceptance, but with lost configuration or a conflicting pass rule. The receipt-reuse repair does not supply these missing fields to this separate consumer. The [Astra queue][queue] and targeted searches of the available audit corpus revealed no exact report of this handoff; that is not a claim to have exhausted all historical discussion.

## Production route

Both CLI recurse and chat recurse construct an attempt campaign from the `RecurseAttempt`, run an orchestrator and return its kept writes. The ordinary finding path selects the reporting run, copies its output and argv, and copies the gate's `OKExitCodes`. `RecurseAttemptCampaign` places the copied command and codes into `Acceptance`. These are real production entry paths, not only manually constructed records. [CLI callback][cli] [Chat callback][chat] [Reporting run][reporting] [Attempt campaign][attempt]

An orchestrator task receives its write guard, recall query and campaign check. It then runs through the observed task executor and `RunUntilVerified`; a turn that is not done can continue with its missing evidence. The campaign-check helper copies only campaign ID, task ID and argv. The corresponding tools descriptor has those same fields and no environment or pass rule. [Task execution][spawn] [Check handoff][handoff] [Descriptor][descriptor]

`run_check` refuses model-supplied arguments, reads the host-supplied check, resolves the workspace root and invokes the bound tactile executor. It passes binary, arguments and directory, then records the raw exit. Session ingests each recorded acceptance run and asserts its exit on the turn's sequence clock. These protections are valuable: the missing contract must be supplied by the host, not added as a model-editable tool argument. [Tool execution][run-check] [Receipt ingestion][ingestion] [Host projection][projection]

### A. An accepted exit becomes a failed task check

The project-document schema explicitly supports `ok_exit_codes`. A supplied list must include zero, contain no negative values or duplicates, and cannot be empty. Thus `[0,3]` is a valid, explicit pass rule. Gate detection copies the list, and `Gate.Passed` tests membership. [Declared rule][exit-schema] [Validation][exit-validation] [Detection][detection] [Gate judgment][gate-pass]

Campaign acceptance preserves that rule correctly: `runAcceptanceRound` copies the codes and judges the result using `gates.Gate{OKExitCodes: okCodes}.Passed(exitCode)`. Its existing tests specifically preserve the nonzero-pass behavior. This stage is counterevidence against a claim that CodeNERD universally assumes zero. [Round judgment][round] [Acceptance controls][acceptance-tests]

The task consumer does assume zero. Mangle selects the latest check run after the last write; exit zero derives `turn_gate(Turn, /check, /passing)`, and any nonzero exit derives `/failing`. A turn whose catalog offered `run_check` and which retains net written files owes that gate. Independently, the live tool-result classifier marks every nonzero check receipt failed, so a policy-only correction would leave the presentation inconsistent. [Sequence and verdict][check-policy] [Owed obligation][owed] [Progress classification][progress]

#### Conditional exit-code witness

1. Use an admitted workspace node and a declared node-scoped lint or audit command with `ok_exit_codes: [0,3]`. Its documented success convention includes exit 3. The command and its pass declaration are unchanged throughout the attempt.
2. The original source genuinely fails this check with exit 1 and produces a finding selected by recurse. Normal planning, risk, permission and execution prerequisites succeed.
3. The editing task makes an authorized, correct net source change. Its catalog offers `run_check`. All unrelated obligations can be held constant and satisfied.
4. After its last write, the task invokes the actual `run_check` tool. The check genuinely exits 3. No later write or check supersedes this observation.

The gate's own rule accepts the observation, and campaign acceptance would accept the same exit. The session's `/check` instead derives failing, remains unmet and the progress event says FAIL. This is a deterministic mismatch of declared semantics under the stated conditions, not an assumption about a malicious or incompetent model.

Custom exit 3 is deliberate. Detected pytest exit 5 means no tests collected, and the workspace test planner separately distinguishes a passing process from evidence that tests ran. That distinction must remain intact. This witness concerns an explicitly declared lint/audit pass, not treating an empty test collection as behavioral verification.

The source does not prove that the whole recurse run must fail: a later turn might have no net writes, another legitimate run might exit zero, the worker might name a blocker, or independent verification might change the outcome. Those possibilities do not make the original rejected observation conform to its declared pass rule.

### B. The same command loses its declared environment

Both `commands.build/test/lint` and explicit `gates:` entries receive `commands.env` in `fromNerdMD`. `gates.Command` passes that map to `build.WorkspaceCommand`, which constructs the command and overlays the map last. The schema explains that these variables are command prerequisites invisible in the command string. [Environment declaration][env-schema] [Detection][detection] [Gate runner][gate-command] [Environment overlay][workspace-command]

The reporting-run → attempt adapter copies argv and exit codes, not the environment. Neither `RecurseAttempt` nor `Acceptance` carries it. `runAcceptanceRound` builds a tactile command containing only binary, arguments and workspace directory; the task's `run_check` does the same. [Attempt definition][attempt-type] [Reporting run][reporting] [Acceptance descriptor][acceptance-type] [Round execution][round] [Tool execution][run-check]

The standard executor's base environment is not a hidden repair of this omission. Boot configures it from `build.GetBuildEnv(nil, workingDir)`. That function uses essential variables, the supplied build configuration and header detection; it does not load `nerd.md commands.env`. The direct executor combines allowed ambient variables, its configured base environment and command-specific environment; the latter is absent here. This does not mean every variable differs: inherited/default values may coincidentally match. [Boot configuration][boot-env] [Build environment][build-env] [Direct executor][direct-env]

#### Conditional environment witness

Use an admitted declared node-scoped check whose `commands.env` includes a nonsensitive fixture value, for example `CHECK_PROFILE: strict`. Require that the fixture's parent environment and tactile base environment do not already supply the same value. The immutable checker first verifies this prerequisite, then checks the node's source. It exits 1 for missing/wrong profile or a genuine source defect and exits 0 for a valid source under the strict profile.

The original strict gate fails on a real source defect, and recurse selects it. An authorized task repairs that defect without altering the checker, environment declaration or permissions. The task's `run_check` executes the same argv and cwd without the declared override and fails its prerequisite; a fresh ordinary gate execution with the unchanged override would pass. If campaign acceptance is reached, its independently executed command has the same missing override.

This establishes differing invocations and a conditional spurious failure. It does not require source races, missing tools, a configuration edit, or receipt reuse. A checker unaffected by the variable, an empty environment map, or an executor already carrying an identical value is a negative control, not an affected case. Later outer gate measurement may still recognize a valid repair, so no false-revert or false-keep outcome is asserted unconditionally.

## Existing protections to preserve

The outer loop treats the executor's returned error as a recorded observation, not as final truth. Changed attempts remeasure the node's gates and the selected workspace ratchet/target gates; the policy then determines keep, revert or refuse. This is a meaningful mitigation. Do not describe it as rerunning every workspace test and audit after every attempt: the existing mission proposal already documents pass-boundary versus per-attempt scope. [Outer measurement and judgment][ratchet] [Mission scope][mission]

Within the turn, sequence qualification correctly rejects checks preceding the last write. Tool arguments are closed to model substitution. The campaign's independent acceptance round and round budget remain relevant. Observed task execution, write guards, gate/metric checks, cancellation and permission checks must not be removed to make this fixture pass. `run_check` and acceptance sharing the same tactile executor is useful parity **between those two consumers**, but it does not make either equivalent to the original gate whose environment was dropped.

No evidence here establishes a cross-workspace command, permission bypass, secretly modified checker, data loss, or a falsely kept change. The report also does not expand into configured Go-flag fidelity, execution-profile hermeticity, crash recovery, scope completeness or strategic requirement acceptance. Those deserve their own contracts rather than being silently folded into this finding.

## Proposed bounded correction

Update the affected campaign, tools, session and gate specifications before implementation. Preserve two independently testable parts of a host-owned check contract:

- **Invocation configuration:** the admitted declared environment overlay, scoped to its workspace and declaration identity, reaches both the task command and the campaign round. Reuse the governed command-construction semantics or an adapter with proved parity; do not bypass tactile authorization, auditing or timeout behavior by substituting a raw process launch.
- **Pass semantics:** the admitted passing-exit set reaches the task's evidence and policy, with zero-only behavior when omitted. Keep the actual exit code unchanged. Host facts should identify the permitted pass rule and observed outcome; Mangle should derive whether the corresponding turn obligation is met. The live progress consumer should use the same qualified result instead of separately assuming nonzero means failure.

Copy mutable slices/maps or bind immutable descriptors so later edits cannot silently alter an in-flight check. Define resume behavior explicitly: missing legacy configuration is unknown or requires re-resolution against a compatible declaration, not an invented equivalent check. Do not relabel an older observation with today's declaration. A restored environment or pass rule does not itself prove source applicability or semantic test coverage.

Environment values can contain secrets. Avoid putting raw values into task prose, kernel facts, audit logs, diagnostic diffs or unrestricted campaign snapshots. Use trusted runtime configuration or a governed reference with explicit version/re-resolution semantics; retain only safe public identity and reason metadata. An ordinary digest of low-entropy secrets is not a safe public substitute. The generic fixture value above is intentionally nonsecret.

Do not fix this by converting exit 3 to zero, allowing every nonzero exit, removing the task's `/check` obligation, accepting model claims of success, or granting a free-form command/environment argument. Those approaches erase the contract rather than preserve it.

## Existing tests and unrun evaluation

Read but not run: `TestRecurseCycle_PytestOKExitsTravelWithTheCheck` checks detection → attempt → campaign acceptance copying; `TestAcceptance_AGateOKExitIsAPass` checks campaign-round judgment; the spawn-check test carries argv into task context. These are useful positive controls but stop short of proving nonzero-pass semantics in session `/check`. The session check tests cover latest-after-write sequencing and catalog ownership; the tool tests cover workspace directory, refusal of model arguments, execution and raw output. None of the inspected fixtures carries this complete declared environment/pass rule through the full route. This is a statement about the inspected tests, not all possible repository coverage. [Recurse test][recurse-tests] [Acceptance tests][acceptance-tests] [Spawn test][spawn-test] [Session tests][session-tests] [Tool tests][tool-tests]

A separately authorized evaluation should begin with deterministic fixtures using the real production entry adapters, observed task path, tool dispatch and kernel policy, while holding model decisions fixed. No model is needed to prove the missing-field transformations. Test CLI and chat callbacks, not only record constructors. Use the normal world-model and CodeDOM context for subsequent complete-system evaluation; restricted-tool substitutes are supplementary.

| Fixture | Required distinction |
|---|---|
| Declared [0,3], genuine post-write exit 3 | Gate, campaign round, task policy and progress agree it satisfies that declared check |
| Declared [0,3], exit 1 | Fails everywhere; no widened pass rule |
| Omitted list; exit 3 | Remains a failure |
| Run before final write; later write | Earlier pass remains in history but cannot satisfy the latest obligation |
| Detected pytest no-tests exit | Process pass remains separate from test-coverage obligations |
| Declared nonsensitive environment only in nerd.md | Task and campaign execution receive the intended overlay without exposing it in prose/logs |
| Ambient value conflicts with declared value | Defined override precedence matches the gate |
| Empty overlay or already-identical values | Unchanged existing behavior; no fabricated mismatch |
| Cancellation, missing executable, infrastructure error | No invented successful check; actual execution state preserved |
| Concurrent campaigns with different pass rules/overlays | No cross-campaign or cross-turn contract bleed |
| Save/resume, declaration change, mutable input map/slice | Explicit identity/invalidation and safe copying |
| Outer ratchet observes a regression despite local check pass | Existing rejection remains; contract parity is not full acceptance |

Compare current main, pass-rule-only correction, environment-only correction and combined parity on identical source/checker/configuration snapshots. Do not change model, prompts, permissions, checker strength or success criteria between treatments and attribute the gain to this repair. Preserve real failures and no-change controls; a treatment that skips all checks must not look efficient.

Measure disagreement counts between consumers, false local rejection, genuine failure detection, unresolved obligations, continuation/remediation rounds, successful authorized outcomes and progress-label accuracy. Inspect exact command configuration through trusted fixture instrumentation, never public secret dumps. Report skipped/unavailable observations separately and require the existing outer ratchet controls to remain intact.

## Costs and acceptance

No speedup, token saving or implementation estimate has been measured. For K allowed exit codes and E declared environment entries, a straightforward descriptor copy and comparison adds O(K + E) metadata work, with environment normalization/sorting potentially O(E log E). Policy facts, joins, serialization/version handling, secret-safe configuration references and legacy-campaign migration are additional costs. Avoid replicating a complete ambient environment in every task.

Measure CPU and wall time for command resolution, process runs, policy evaluation, persistence and any retries; retained metadata/output storage; total model and tool tokens; and human intervention caused by blocked or conflicting configuration. Avoided spurious checks or repair turns are benefits to measure, not free offsets. Do not remove independent campaign or outer-loop verification merely to claim lower execution cost.

Acceptance is one production-route witness for each independently lost field, negative controls that preserve genuine failures and authority, consistent live evidence, and preserved outer-ratchet behavior. The desired result is modest and checkable: a task asked to run its finding's judge receives the same admitted configuration and pass semantics, without confusing that local pass with completion of the broader mission.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/fef98230c20a5751e4c32a2fd2757832a9d39cdc
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/innovations/mission-driven-forever-improvement.md
[receipt-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/workspace-test-receipt-invocation-identity.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/ASTRA_QUEUE.md
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/cmd_campaign_recurse.go#L199-L237
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/campaign_recurse.go#L157-L208
[reporting]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/recurse_cycle.go#L873-L880
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/recurse_attempt.go#L103-L116
[attempt-type]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/recurse_cycle.go#L43-L73
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/orchestrator_task_handlers.go#L30-L95
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/orchestrator_task_handlers.go#L98-L116
[descriptor]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/tools/campaign_check.go#L16-L46
[run-check]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/tools/shell/run_check.go#L77-L144
[ingestion]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/executor_tools.go#L1085-L1093
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/gate_facts.go#L691-L700
[exit-schema]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/projectdoc/nerdmd.go#L181-L194
[exit-validation]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/projectdoc/nerdmd.go#L588-L621
[detection]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/gates/detect.go#L54-L91
[gate-pass]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/gates/gates.go#L89-L112
[acceptance-type]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/orchestrator_acceptance.go#L31-L54
[round]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/orchestrator_acceptance.go#L149-L210
[check-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/defaults/policy/coder_safety.mg#L535-L551
[owed]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/defaults/policy/coder_safety.mg#L623-L633
[progress]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/executor_tools.go#L1114-L1127
[env-schema]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/projectdoc/nerdmd.go#L197-L207
[gate-command]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/gates/run.go#L83-L99
[workspace-command]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/build/workspace_command.go#L30-L60
[boot-env]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory_execution.go#L29-L41
[build-env]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/build/env.go#L131-L169
[direct-env]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/tactile/direct.go#L299-L320
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/recurse_cycle.go#L942-L1037
[recurse-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/recurse_cycle_e2e_test.go#L443-L501
[acceptance-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/orchestrator_acceptance_test.go#L208-L292
[spawn-test]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/campaign/spawn_task_check_test.go#L15-L44
[session-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/turn_check_test.go#L219-L352
[tool-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/tools/shell/run_check_test.go#L99-L212


## As built

Rulings against the current source:

| Finding or boundary | Ruling | Current implementation |
|---|---|---|
| Declared environment disappears between gate and attempt | Real; repaired | `internal/campaign/recurse_cycle.go` copies the reporting gate's environment into runtime-only `RecurseAttempt.CheckEnv`. |
| Campaign round and task omit the environment | Real; repaired | `recurse_attempt.go`, `acceptance_runtime.go`, `orchestrator_acceptance.go` and `orchestrator_task_handlers.go` carry a private workspace-bound contract into tactile execution. |
| Task receipts, `/check`, and live progress assume zero | Real; repaired | `internal/tools/{campaign_check,check_runs}.go`, `internal/tools/shell/run_check.go`, `internal/session/{gate_facts,executor_tools}.go` and `internal/core/defaults/policy/coder_safety.mg` preserve the raw exit and derive invocation-specific membership. |
| Campaign acceptance already preserves declared passing exits | Already correct before this change | `runAcceptanceRound` already used `gates.Gate.Passed`; `TestAcceptance_AGateOKExitIsAPass` covers that existing contract. |
| Either defect proves false outer adoption or bypassed risk preflight | Not established, as the audit correctly limits | `recurse_cycle.go` still remeasures and obeys the ratchet. `risk_gate_contract.go` and its hard/soft classification are unchanged. |

The private acceptance descriptor binds the gate ID, argv, pass set and workspace
alongside a copied environment. Context insertion/extraction and receipt storage
copy mutable inputs. `cloneCampaign` preserves a separate runtime contract for
in-process transactions. Environment values are excluded from attempt JSON,
mission deliberation requests, mission receipts, campaign snapshots and kernel
facts; tactile's existing audit redaction handles the execution environment.
The same shared `RecurseAttemptCampaign` adapter is called by CLI and chat,
including mission attempts. No prompt atom content changed.

Restored recurse checks, including legacy snapshots without the marker, stop at
`Orchestrator.Run`, task admission and acceptance settlement when the private
contract is unavailable. A new measured recurse invocation is required; today's
declaration is never attached to an old receipt. In-memory changes to gate ID,
argv or exits, and a different workspace, also refuse execution. Improvements
without a declared check retain their existing behavior.

Authored regression coverage:

- `internal/campaign/recurse_contract_continuity_test.go`: `TestRecurseContract_EnvironmentAndExitsReachTaskAndAcceptance` drives actual declarations, gate execution, recurse selection/callback, task handoff, tactile execution, campaign acceptance and the outer ratchet. Separate subcases isolate environment loss, nonzero exits and their combination. Restored/legacy and changed-contract tests cover early rejection.
- `internal/session/turn_check_contract_test.go`: real `run_check` dispatch, receipt ingestion, shipped policy and live progress; declared exit 3, genuine failure, omitted-rule defaults, execution error, final-write freshness, invocation/turn isolation and model-update protection.
- `internal/shards/turn_check_contract_test.go`: production manifest and complete corpus parity between Cortex and a single kernel, including membership, negation, later receipts and the independent test gate. New predicates are homed beside `turn_check_run` and `turn_gate` in the world shard (`internal/shards/registration.go`).
- `internal/tools/shell/run_check_test.go`: bound environment, raw exit, workspace refusal, no-result/error and cancellation controls. `campaign_check_test.go` and `check_runs_test.go` cover copying on both handoff boundaries.
- `TestMissionDeliberationStopsBeforeEffectsForValuesUnknownSpendAndDrift` in `internal/campaign/recurse_mission_policy_test.go` now checks that the actual deliberation frame and persisted mission state omit the runtime environment.

Fail-before evidence is **source-based only**: the pre-change attempt omitted the
environment, `CampaignCheck` and `AcceptanceRun` omitted allowed exits, the policy
rejected exit 3, and progress classified that receipt as FAIL. The positive
fixtures assert the opposite at those production boundaries. No failing-before
or passing-after run is claimed. Builds, tests and engine execution were
explicitly excluded from this authoring session. The scaffolded lab verdict,
owning-package tests, defaults inventories and shard-join checks remain for the
root verifier; no baseline exemptions were added. The as-built specification is
in `Docs/architecture/campaign/INTERNALS.md` and the affected core, session,
shards and tools corpora.
