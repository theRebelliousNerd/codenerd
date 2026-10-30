---
implementation-status: partial
verification-method: static-source-inspection
verified-against: d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2
date: 2026-10-30
---

# A consultation must not retire a boot-owned specialist database

## Finding and scope

One supervised-campaign consultation can remove a boot-owned expert database from the shared JIT compiler. The narrow production witness requires a user-defined agent whose name overlaps a default campaign advisor (`coder`, `tester`, `reviewer`, or `researcher`), plus an admitted supervised campaign resume that reaches its enabled advisory risk gate. This is not a claim that ordinary custom-agent execution takes the shard-manager path: the normal Cortex front door deliberately routes those tasks directly to the task executor.

The manager treats its temporary execution as owner of a database registration keyed by persona name. Boot owns a registration under that same name. Spawn replaces and closes the boot handle; completion removes and closes the replacement. A later ordinary task for that persona therefore has no registered custom database, although its profile and on-disk data still exist. Source predicts missing custom prompt/knowledge retrieval, not deletion of the database file, loss of built-in atoms, a permission bypass, or a measured model-quality regression.

All source references below pin main `d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2`, resolved on October 30 at approximately 04:08 UTC. Research branch `research-main-20261030-0211` was reported merged and deleted; its audits are in the inspected main inventory. This pass performed source inspection only: no tests, inference, runtime execution, or repository changes. The local research document is not an implementation.

## The production chain

1. Boot's agent synchronizer scans agent directories and records successfully synchronized `prompts.yaml` files under their directory names. It does not exclude the four advisor names. Boot registers each discovered DB with the compiler, then defines its shard profile as `TypeUser`. Thus a valid `.nerd/agents/coder/prompts.yaml` is sufficient for the relevant naming condition; the witness does not require malformed configuration. [Discovery](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/prompt/sync/synchronizer.go#L71-L103), [boot registration](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L1942-L1970).
2. Boot also binds the manager's `.nerd` directory and compiler registration/unregistration callbacks. These are real production hooks, not test-only seams. [Wiring](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L2148-L2156).
3. The system campaign runner admits a resumable campaign and calls `orch.Run`. Its orchestrator constructs a consultation engine backed by `campaignRunnerShardManagerConsultationSpawner`; that adapter calls `shardMgr.Spawn` directly. The advisory board receives this engine and defaults to the four names above. [Resume and run](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/shards/system/campaign_runner.go#L261-L306), [engine construction](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/shards/system/campaign_runner.go#L347-L375), [adapter](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/shards/system/campaign_runner.go#L447-L457), [default advisors](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/campaign/shard_advisory_board.go#L47-L61).
4. This is not merely a provider constructed but never called: `Run` invokes risk preflight. Preflight can skip disabled or below-threshold work; when the campaign is gated and the advisory gate is enabled, it calls `runAdvisoryRiskGate`, which calls the board. The witness must select that branch. [Run](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/campaign/orchestrator_execution.go#L43-L48), [skip and gate conditions](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/campaign/risk_scoring.go#L299-L409), [board call](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/campaign/risk_scoring.go#L815-L855).
5. For `TypeUser` or persistent profiles, manager spawn registers the DB under `typeName` and records `activeJITDBs[instanceID] = typeName`. Its completion cleanup calls the unregistrar with that name, not an ownership token or the handle it acquired. [Spawn](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/core/shards/manager_spawn.go#L320-L325), [completion](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/core/shards/manager_spawn.go#L506-L517).
6. The registrar opens a new SQL handle on every registration. `RegisterShardDB` closes a distinct previous handle under the normalized name and installs the new handle. `UnregisterShardDB` deletes and closes whatever handle is currently stored under that name. There is no reference count or registration-generation comparison at this boundary. [Callbacks](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/prompt/loader.go#L691-L752), [compiler ownership](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/prompt/compiler_db.go#L213-L255).
7. The next compilation captures its expert handle from this shared name map. With no handle, it does not load that selected expert's custom atoms. This is a registration-lifetime error before relevance selection or token budgeting. [Source capture](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/prompt/compiler_generation.go#L100-L117), [source collection](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/prompt/compiler_sources.go#L86-L100).

## Existing protections and limits

Normal Cortex user-agent execution bypasses manager registration entirely, so it does not establish this defect by itself. Default generalist profiles are ephemeral and do not enter the registration block; a stock campaign without the named user profile is a negative control. The supervised campaign must pass admission and dependency checks. Advisory failure and missing votes remain fail closed; the finding does not undo those repairs. [Normal routing](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L544-L574), [generalist lifecycle](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/core/shards/config.go#L10-L24).

Compiler locks serialize map changes; source-epoch bumps and prompt-cache invalidation correctly follow registration changes. Those protections do not retain the boot registration. The problem is ownership, not a missing cache clear. Existing compiled prompt text or other context sources can cushion impact; no claim is made that every request loses all domain knowledge or necessarily fails. Boot or explicit re-registration can restore the DB. The disk database is not removed.

Overlapping same-name manager executions expose the broader API hazard: a first completion can remove the later execution's registration. That is a source-level extension, not required for the single-consultation witness and not evidence of a default concurrent production workload.

## Proposed contract

Make registration ownership explicit. A boot-owned persona database should remain registered for the compiler lifetime or until its owning registry deliberately replaces/removes it. A temporary shard must borrow that registration without deleting it. If temporary registration remains necessary, return a scoped lease or generation-bound token and release only that acquisition; preserve other owners and readers, and define handle retirement after all users finish.

An alternative narrow repair is to route the campaign runner's consultations through the same task-executor consultation adapter as the other doors. That removes this production trigger, but does not by itself repair the manager API's shared-name lifetime contract. Compare the options rather than silently substituting one for the other. Do not repair the issue by suppressing all `Close` calls, forbidding previously valid names without a migration policy, or removing cache invalidation.

## Verification and costs to establish

Use a deterministic fixture with a valid custom `coder` atom carrying a unique marker, boot-owned registry state, an admitted resumed campaign whose enabled advisory gate is reached, and a controlled consultation executor. Capture registration identity before consultation, after completion, and in a subsequent actual compiled coder request. The custom source must remain available; sibling registrations must remain unchanged. Hold the acceptance gates and effective tools constant.

Controls: stock ephemeral coder; an unrelated custom expert; normal direct Cortex execution; advisory gate disabled; campaign below threshold; consultation failure/cancellation; overlapping same-name executions; explicit owner replacement while an old execution completes; compiler shutdown. No source removal should masquerade as successful optional empty retrieval. Existing single-shard cancellation cleanup tests exercise one registration and are not evidence for shared-owner preservation.

Measure open SQL pools, retained bytes, compile/re-registration latency, cache invalidation counts, custom atom coverage and actual request tokens. A lease may retain a pool longer; eliminating redundant registration may avoid opens and cache churn. Neither savings nor downstream task-quality gains are established here. Use matched models, budgets, task corpus and gates for any later live comparison.

The inspected consultation-coordination, task-terminal-ownership, specialist-roster-workspace-identity and expert-roster-domain-retirement audits concern different contracts. This report claims bounded nonduplication from that inventory and source review, not exhaustive historical novelty.

## Current-source rulings

Rechecked against worktree base `fef98230c20a5751e4c32a2fd2757832a9d39cdc`
before authoring the repair. No builds, tests, inference or live comparison were
run. Line numbers in the original audit are historical; the functions below are
the current source anchors.

| Claim | Ruling and current evidence |
|---|---|
| Discovery permits a custom agent named `coder`, `tester`, `reviewer` or `researcher`; boot registers it and defines a user profile. | **Real.** `internal/prompt/sync/synchronizer.go` `SyncAll` accepts those directory names; `internal/system/factory.go` `initIntelligenceLayer` registers discovered DBs and defines `ShardTypeUser` profiles. Naming remains valid. |
| Boot connects manager execution to the shared compiler registration map. | **Real.** `internal/system/factory.go` `initShardManagement` installed both registration and name-based unregistration callbacks. The repair replaces that pair with one scoped acquisition callback. |
| An admitted supervised campaign can consult these names through the manager. | **Real.** `internal/shards/system/campaign_runner.go` `startCampaign` calls `ResumeCampaign` and `Run`; `orchestratorConfig` supplies the manager-backed consultation engine to the advisory board. `internal/campaign/shard_advisory_board.go` still defaults to the four advisor names. |
| The advisory provider is reachable, conditional on risk gating. | **Real.** `internal/campaign/orchestrator_execution.go` `Run` invokes `runRiskPreflight`; `internal/campaign/risk_scoring.go` skips disabled/below-threshold work and calls `runAdvisoryRiskGate` when gated and enabled. The repair preserves these branches and fail-closed votes. |
| Spawn registers by persona and completion unregisters whatever occupies that name. | **Real; repaired in authored code.** `internal/core/shards/manager_spawn.go` previously recorded the persona string in `activeJITDBs`. It now records the acquisition's release function and invokes only that release on completion. |
| Registration replaces and closes a prior handle; unregistration has no ownership check. | **Real; repaired in authored code.** `internal/prompt/loader.go` previously opened a pool on every callback; `internal/prompt/compiler_db.go` replaced and closed by normalized name. Scoped acquisitions now reuse the current generation. Explicit owner replacement/removal retains acquired old handles until their leases and readers drain. |
| A later compile has no custom source after that cleanup. | **Real consequence of the former lifecycle.** `internal/prompt/compiler_generation.go` snapshots the selected DB; `internal/prompt/compiler_sources.go` reads custom atoms only when that source exists. Source collection remains intact; scoped cleanup preserves the owner source. |
| Overlapping same-name executions can retire each other's source. | **Real; repaired in authored code.** Temporary registrations share a pool and release their own references. An old release cannot remove a replacement registration. |
| Normal Cortex execution bypasses the manager; ephemeral defaults avoid this registration block; locks and cache invalidation do not establish ownership. | **Already correct protections, not earlier fixes of this defect.** See `internal/system/factory.go` `SpawnTaskWithTarget`, `internal/core/shards/config.go` `DefaultGeneralistConfig`, and compiler registration locks/source epochs. No routing change or cache-invalidation removal is needed. |
| Database deletion, permission bypass or measured quality/resource savings follow from this defect. | **Not claimed by the audit and not established here.** The repair retains files and acceptance gates; operational and quality measurements remain pending. |

None of the audit's defect claims was already fixed or contradicted by current
source. Routing only the campaign consultation through a different executor
would remove one trigger while leaving the manager's shared-name ownership
error; the repair instead fixes that boundary for every caller.

## As built

Commits: none; the authored changes are uncommitted in this worktree.

`internal/types/interfaces.go`, `internal/core/shards/manager.go`,
`internal/core/shards/manager_spawn.go`, `internal/prompt/loader.go` and
`internal/system/factory.go` now carry one scoped registration acquisition and
its idempotent release end to end. The former unregistrar API is removed.
`internal/prompt/compiler_shard_lifecycle.go` owns generation, owner, execution
lease and reader references. `compiler_db.go` and `compiler.go` integrate explicit
registry replacement, removal and shutdown. `compiler_generation.go`,
`compiler_pipeline.go`, `compiler_experts.go`, `compiler_knowledge_use.go` and
`vector_searcher.go` pin shard readers through their real use, including
singleflight work that outlives a canceled caller. As-built contracts are in
`Docs/architecture/shards/IMPLEMENTED_SPEC.md` and
`Docs/architecture/prompt/IMPLEMENTED_SPEC.md`.

Authored regressions:

- `internal/system/shard_db_ownership_test.go`:
  `TestBootOwnedShardDBSurvivesManagerConsultation` uses discovery and real boot
  callback wiring, checks registration identity during/after success and failure,
  preserves a sibling, and checks a custom marker in the next actual coder
  compilation. `TestManagerTemporaryDBLivesUntilOverlappingExecutionsFinish`
  checks shared pools and final closure after success, failure and cancellation.
- `internal/shards/system/campaign_runner_db_ownership_test.go`:
  `TestCampaignRunnerAdmittedRiskConsultationKeepsOwnerDB` journals a campaign,
  resumes it through admission and calls `Run` with the runner's production
  consultation wiring. Its enabled advisory gate rejects the protected change;
  the owner DB remains registered and no campaign task executes.
- `internal/prompt/compiler_shard_lifecycle_test.go`: owner borrowing without
  source-epoch churn; temporary overlapping leases; owner replacement and sibling
  preservation; explicit owner removal; speaker/pushed-expert readers after
  replacement and caller cancellation; shutdown and late acquisitions.
- `internal/core/shards/manager_spawn_context_test.go`: acquisition failure
  stops before execution; the existing canceled-waiter cleanup now exercises
  scoped release rather than name-based unregistration. Existing source-snapshot
  test helpers release their reader references.

Fail-before evidence is **source-derived, not executed**: the former boot callback
opens and installs a distinct pool before the controlled advisor executes, so
the boot regression's identity check fails; completion then deletes the persona
entry. The former direct replacement also closes the handle still captured by
the blocked compilation, violating the reader regression's successful Ping
assertion. No red/green test result is claimed.

Status remains **partial** pending execution of the owning packages and the
authored boot/resume regressions, observed fail-before/red-green evidence, and
the proposed resource/latency/token measurements. No prompt atom, Mangle rule,
predicate manifest or starvation baseline changed. The private-terms scanner
was not found within this worktree, so that repository-required scan also
remains pending.
