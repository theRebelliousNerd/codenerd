# Workspace scoped tests and recurse improvement admission

**Status: static source-derived audit and proposed contract.** Public CodeNERD revision `158be02e0641acd01b85d323405d256ccb6eb308`, inspected 2026-11-03. No implementation, repository write, build, test, gate, application path, model call, inference or benchmark was run. The witness is a conditional derivation, not an executed reproduction or an observed user incident.

## Decision in brief

Preserve the requirement that an improvement obtain a fresh test verdict. Repair the disagreement between improvement admission and the checks scheduled to supply that verdict. With the shipped policy, an ordinary improvement on a node with **no applicable node-scoped test gate**, whose available tests are all workspace-scoped, cannot satisfy the outer ratchet's tested requirement. It can still be admitted because a static metric is measurable, consume campaign work, improve that metric and then be reverted. [Admission][improve] [After-check selection][after] [Tested projection][tested] [Required verdict][ratchet]

This configuration is not limited to an unknown language or an absent test command. Detected JS package-script tests and Rust `cargo test` are workspace-scoped. A project-declared `commands.test` is also workspace-scoped and suppresses detected test gates of the same kind. Unless another applicable node test or a different admitted policy supplies the missing route, the ordinary improvement path omits those tests from its after-check set. [Declarations][declarations] [Rust detection][rust] [JS detection][js]

The guard is doing the right thing with the evidence it receives: it declines to keep an untested improvement. The actionable defect is admitting work for which the current schedule cannot produce a mandatory observation despite having a runnable workspace test. The repair is either to derive and execute an authorized fresh test obligation before deciding, or to defer the improvement before expensive work with an explicit reason. Removing the guard, relabeling a baseline as current evidence or forcing a workspace command into an unsupported node scope would not repair the contract.

## Scope and architectural contract

The finding concerns ordinary metric-based improvements, not every recurse attempt or every JavaScript/Rust repository. It assumes the shipped `recurse_ratchet_kind` facts and normal host projection, a non-cross-cutting node with a measurable angle, no applicable node-scoped Test gate, and no selected finding. Explicit additional node tests, different policies and finding-fix paths are material counterexamples.

Root guidance makes Mangle the executive and calls for production-path verification. Campaign guidance requires host-observed test evidence rather than model prose. Architecture guidance distinguishes intended capabilities from current reachability and prohibits narrowing a specification merely to excuse missing wiring. Accordingly, this audit proposes a verification-obligation contract in the existing loop, not another scheduler or an LLM exemption from testing. [Root guidance][root] [Campaign guidance][campaign-guidance] [Architecture contract][architecture]

The portable recurse proposal already asks for capability manifests and distinguishes executable gates from stronger behavioral evidence. Here the missing capability is unusually concrete: the system can discover and run the owner's tests, yet its ordinary improvement schedule does not make their fresh result available to the required consumer. A positive line-count or test-count change remains a proxy, not proof of useful behavior. [Portable boundary][portable]

## Production trace

### Test discovery can legitimately yield only workspace tests

`fromNerdMD` turns `commands.build`, `commands.test` and `commands.lint` into `ScopeAll` gates. `Detect` records their declared kinds and excludes detected gates of those kinds. Thus adding an authoritative `commands.test` to a Go or Python project can replace its otherwise detected node test with a workspace test. An extra `gates` entry alone does not suppress native tests: only command declarations populate that override set. This audit assumes no extra applicable node Test. [Declaration precedence and scope][declarations]

Native detection makes `rust:test` run `cargo test` with `ScopeAll`. `jsGates` reads the package's test script and creates `js:test` as the package-manager `run test` command, also with `ScopeAll`. These are source-supported configurations, not malformed declarations. `nodeGates` accepts only `ScopeNode` gates applicable to the node's languages, so none of those workspace tests enters `v.own`. [Rust][rust] [JS][js] [Node filtering][node-gates]

### Baseline execution is not the candidate verdict

The pass driver discovers gates and queries policy for ratchet kinds. When state is initially empty it calls `measureWorkspace`, which measures the workspace-scoped tests along with other applicable workspace gates and stores their results. At pass end it calls that measurement again **only if the kept count changed during the pass**. These routes are real, but neither supplies a fresh test result on an otherwise reverted improvement before that improvement's decision. It would be inaccurate to say workspace tests never run, or that they necessarily run at the end of every pass. [Pass and baseline][pass] [Conditional pass-end measurement][pass-end] [Workspace measurement][workspace]

The policy comment already describes workspace tests/audits as pass-level checks while listing only `/build` and `/lint` as workspace kinds remeasured for each attempt. The problem is their interaction with a separate mandatory improvement predicate, not discovery of that cadence itself. [Ratchet kinds][kinds]

### Admission asks about metrics but not a reachable test obligation

`improve` skips cross-cutting nodes, computes metrics, and asks `improveAngle`. The policy chooses an angle when its metric is measurable; it does not join that choice to an available test route. `metrics` can supply workspace test count and node source lines without any node test gate. In particular, `/simplify` requires source lines to decrease, so it can be selected in a JS/Rust node whose only tests are workspace-scoped. [Improvement entry][improve] [Metric production][metrics] [Angle selection][angles]

`improve` creates an `attemptSpec` containing an angle but no finding or target check. The normal CLI and chat paths both construct `RecurseAttemptCampaign`, run the campaign orchestrator and return owned writes to the shared `RunRecurseCycles` driver. For an improvement without a check, that adapter declares no specific acceptance witness. Inner task success or even a worker-run test is therefore not automatically an outer ratchet test receipt. [Improvement spec][improve] [CLI][cli] [Chat][chat] [Attempt adapter][attempt]

### After-check selection cannot supply Tested in this configuration

After a changed attempt, the driver runs all `v.own` gates. It additionally runs a workspace gate only when its kind belongs to the policy's ratchet-kind set or it is the selected finding's target. With no own Test, only build/lint ratchet kinds, and no finding on an improvement, no ordinary Test result is inserted into `after`. The workspace baseline may remain in merged state, but it is not a fresh `after` entry. [After-check selection][after]

The host sets `in.Tested` only while iterating `after`, when an entry has kind Test and is not unverified. This is a **completed test verdict**, not necessarily a passing one; existing incremental-repair semantics and regression guards remain separate. The policy bridge emits `recurse_ratchet_tested(Cycle)` only when that Boolean is true. No target fact is emitted for the improvement's empty target ID. [Tested projection][tested] [Fact bridge][facts]

Optional error-free snapshots are a different input. Their measurement sets `GatesBefore`/`GatesAfter`; their facts can supply comparison vetoes, but the inspected code does not turn them into `in.Tested` or the required predicate. Consequently, enabling `--gates` alone does not repair this ordinary improvement route. This is not a claim that those optional checks are useless or never execute tests. [Optional measurement][optional] [Separate fact projection][facts] [Snapshot vetoes][ratchet]

## Deterministic conditional witness

Take a recognized JS node with a fixed `package.json` declaring runnable build, lint and test scripts. Let the effective ordinary gates be `js:build`, `js:lint` and `js:test`, all workspace-scoped, with no extra applicable node test. Stipulate a clean, passing baseline; installed dependencies; ordinary permissions and budgets; and no pre-existing finding requiring repair. Use an eligible `/simplify` visit with source lines measurable.

The executor makes a permitted production-source edit that reduces the node's nonblank source count from 100 to 90. Test files, test count, declarations, runner configuration and scope remain unchanged. Stipulate that the inner campaign completes and returns the correct owned-write receipts, and that all independent build/lint, scope and optional comparison conditions succeed. Whether a concrete patch is behavior preserving remains an independent fixture obligation; no such patch was executed here.

| Boundary | Conditional observation or source-derived result |
|---|---|
| Initial workspace measurement | `js:test` completes successfully on the baseline. |
| Improvement admission | Lines are measurable, so `/simplify` can be selected. |
| Candidate metrics | Lines 100 to 90; test count K to K. |
| Fresh ordinary after-checks | Workspace build/lint run; `js:test` is not selected; no own Test exists. |
| Host input | `Changed=true`, positive simplify metric, empty target ID, `Tested=false`. |
| Policy | `recurse_improved` may hold, but `recurse_cycle_succeeds` does not: neither a resolved target nor improved-plus-tested is available. |
| Outcome | With no refusal, `/revert` is derived, not `/keep`. The host attempts owned-write compensation and records a reverted outcome if compensation succeeds. |

The key implication is deterministic under those inputs. Policy requires `recurse_cycle_succeeds` for keeping; improvement success requires both a metric move and `recurse_ratchet_tested`. The fallback derives revert when the cycle is not keepable and not refused. The host's revert branch captures a bounded attempted diff, calls `undoOwned`, increments `Reverted` in the ordinary case and appends the record. If compensation itself fails, the host returns an error instead; this audit does not promise successful rollback under every failure. [Policy][ratchet] [Compensation and recording][revert]

The same missing route applies to a comparable Rust-only configuration with detected `rust:test`, and to an owner `commands.test` override with no remaining applicable node Test. The JS witness is sufficient; it avoids test edits, unsupported coverage parsing, finding-census caps and malformed output.

## Existing safeguards and important counterexamples

**The tested guard is intentional and must stay.** `TestRecursePolicy_AnImprovementNeedsATestVerdict` supplies a positive test-count move, expects revert without `Tested`, then expects keep with it. It directly supports the policy interpretation. It supplies the Boolean itself and therefore does not establish discovery-to-after-check reachability for workspace-only tests. This test definition was read, not run. [Guard test][guard-test]

**Finding fixes are different.** When a workspace test is the selected finding's reporting gate, `isTarget` makes the driver remeasure it even though Test is absent from the general ratchet-kind set. A finding fix also has its own target-success branch. Do not infer that workspace test failures can never be repaired. [Target exception][after] [Success branches][ratchet]

**Node tests are a control, not a universal workaround.** An admitted applicable node Test can enter `after` and supply a verdict. A custom policy selecting workspace tests can also alter the result. Neither justifies silently rewriting an owner's workspace test command as node-scoped, adding meaningless placeholders, or claiming every JS/Rust setup is affected.

**Earlier green results are not fresh candidate evidence.** The initial baseline tested different source. Conditional pass-end measurement occurs after visit decisions and cannot rescue a change already reverted. A real inner task check might be reusable only through an explicit identity/freshness bridge; model prose, campaign completion or a generic “tests passed” flag is insufficient.

**This is conservative non-adoption, not unsafe adoption.** No false keep, permission bypass, damaged repository or false mission satisfaction is established. The loss is an unreachable improvement path and potentially wasted authorized work. A declining metric, forbidden write, failing required gate or unavailable tool can reject for other reasons; the witness deliberately holds those constant.

## Mission cost and reporting

Mission mode can add design cost before the same obstruction. The shared driver calls `Mission.prepare` before `Execute`. Preparation snapshots inputs, persists a pending receipt and invokes the deliberator; if its independent admissions permit proceeding, execution follows and only afterward does the ordinary ratchet discover the missing tested fact. This is a possible cost path, not a claim that every mission reaches execution or pays a fixed amount. [Preparation order][mission-order] [Deliberation and receipt][mission-prepare]

On successful settlement, `finish` records the outcome, increments mission cycles and increments the stall streak for a non-kept result. Policy separately bounds cycles, elapsed time, spend and no-progress cycles. These unkeepable improvements can therefore consume bounded mission opportunities, not just computation. Earlier admission may stop them; no infinite loop or fixed call count is claimed. [Mission settlement][mission-finish] [Mission bounds][mission-bounds]

The ordinary revert explanation assembles outcome, signature or worse gates, metric deltas and detail. In the clean witness there need be no worse gate and no executor error; it does not query a dedicated missing-test-obligation reason. The owner may therefore see positive metrics alongside a generic revert rather than the actionable configuration limitation. Add a specific derived reason, not an invented assertion that tests failed. [Reason construction][reason]

A repair that executes a full workspace test after each improvement can be materially expensive: process startup, compilation, test execution, I/O, fixture setup, service effects and artifact retention may scale with the whole repository rather than the edited node. It may duplicate a worker check. Conversely, early admission deferral can save doomed planning/deliberation/edit/compensation work but does not deliver the intended improvement capability. Track both costs and deferred useful work; no savings or frequency is measured here.

## Proposed repair contract

Before admitting an improvement for mutation, policy should be able to name a feasible, authorized verification obligation for that candidate. Host code supplies the effective gate inventory, scopes, command identities, availability and supported evidence routes. Mangle derives the required checks, readiness or a specific blocker. Preserve the existing division of labor and all independent guards.

Two bounded alternatives deserve evaluation:

1. **Fresh workspace verification when needed.** Derive an obligation to run the appropriate admitted workspace test on the candidate when no suitable node test can supply it. Execute through the existing host gate path and preserve argv, environment, allowed exits, source identity and result disposition. Policy must define the required set; choosing any cheap passing check is not proof it covers the changed node. Keep ordinary failure and unverified vetoes and the existing distinction between a verdict and a pass.
2. **Early explicit deferral.** When no authorized, affordable evidence route exists, skip or defer before deliberation and mutation with a reason such as “the configured test scope cannot currently certify this improvement.” Offer the owner a capability/contract decision rather than repeatedly spending on an unkeepable attempt. This is safe degradation, not completion of workspace-test portability.

A future receipt bridge can avoid duplicate processes only if it establishes the same admitted check, relevant source state, configuration and completed result, with no intervening invalidating write. Treat it as a separate evidence contract, not permission to set `Tested=true` from a baseline or successful campaign status. Reuse must preserve failures and unknowns as carefully as successes.

Update the owning specification and capability reporting before implementation. Do not weaken the tested rule, reclassify build/lint as tests, omit failing tests, invent zero-cost verification or hard-code a new host verdict. Retain existing scope, ownership, metric, optional snapshot, cancellation and adoption safeguards.

## Unrun validation and fair comparison

First use a real kernel and the shared loop with a scripted executor, actual project declarations and real fixture check commands. Instrument which commands ran and on which source state. The script should make a known source-only line reduction; it must not inject `Tested` or replace the ratchet. This isolates deterministic wiring without requiring inference, but it does not substitute for later normal CLI/chat validation.

| Proposed case | Required evidence |
|---|---|
| Native JS script test; native Rust test | Current missing after-test route captured; repaired mode obtains fresh evidence or defers explicitly. |
| Go/Python before and after adding `commands.test` | Declaration precedence and changed verification reachability are visible. |
| Workspace-only test with a positive simplify metric | No unconditional keep; current revert distinguished from proposed verified keep or early deferral. |
| Added legitimate node test | Existing successful route remains available without changing unrelated guards. |
| Workspace test as selected failing finding | Target exception and finding-fix semantics remain intact. |
| Optional snapshots on and off | No accidental equation of snapshot success with the ordinary tested fact. |
| Candidate test fails, cannot start, times out or is cancelled | Truthful result and existing rejection/unknown behavior preserved. |
| Worker receipt before versus after the final write | Stale evidence cannot be reused; admitted fresh reuse avoids only genuine duplication. |
| No test command or insufficient authority/budget | Specific early blocker; no model-generated verification substitute. |
| Mission mode and ordinary mode | Deliberation/executor calls, deferral reason, owned compensation and receipts measured separately. |

Compare unchanged behavior, early-deferral admission and policy-directed fresh workspace verification on matched repositories, source changes, gate inventories, permission scopes and full-cost budgets. Independently establish expected behavior and verification scope; a reduced line count is not the study's correctness oracle. Report supported retained improvements, false keeps, unjustified reverts, explicit deferrals, wasted executor/deliberation calls, subprocess counts, wall/CPU time, tokens/spend where applicable, bytes retained, cleanup failures and owner intervention. Include failed and abandoned attempts in denominators. A wider test schedule's added assurance and full cost must both be reported.

## Bounded overlap review and completion condition

The mission innovation document already states the exact broad cadence: own-node gates plus workspace build/lint and a selected target, with workspace tests at pass boundaries. This audit's incremental finding is the conjunction with `recurse_ratchet_tested` and ScopeAll-only discovery: an admitted ordinary improvement has no producer for its mandatory fresh test fact. [Existing cadence analysis][innovation]

The gate-subsumption repair concerns independent workspace findings reaching selection; the current witness has no finding to suppress. The coverage-identity audit concerns a metric dropped after multiple node tests ran; here no suitable ordinary after-test is scheduled. The finding-census audit concerns incomplete diagnostics after execution; no diagnostic overflow is needed here. The portable proposal provides the capability-manifest direction but not this specific unreachable tested obligation. [Subsumption][subsumption] [Coverage audit][coverage-audit] [Census audit][census-audit] [Portable proposal][portable]

Bounded searches for `recurse_ratchet_tested`, workspace-scoped improvement and `ScopeAll` improvement found the source, tests and these neighboring materials, not an earlier exact report. This is not an exhaustive claim about all branches or discussions.

Done means the existing loop either obtains the admitted fresh test evidence needed to judge an improvement or explains the missing route before spending on mutable work. The tested guard remains enforced. Normal JS, Rust and owner-declared workspace test configurations must be evaluated as configurations, not dismissed or advertised as universally supported based only on language detection.

## Pinned public sources

[root]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/agents.md#L3-L23
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/agents.md#L1-L8
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/architecture/AGENTS.md#L50-L111
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/portable-recurse-deliberation-crucible.md#L28-L59
[declarations]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/gates/detect.go#L15-L90
[rust]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/gates/detect.go#L123-L151
[js]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/gates/detect.go#L175-L213
[node-gates]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L899-L910
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L503-L520
[pass-end]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L552-L566
[workspace]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L653-L705
[kinds]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/core/defaults/policy/recurse.mg#L260-L264
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1087-L1107
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1454-L1475
[angles]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/core/defaults/policy/recurse.mg#L356-L398
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/cmd_campaign_recurse.go#L225-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/cmd/nerd/chat/campaign_recurse.go#L218-L267
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_attempt.go#L34-L113
[after]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1208-L1237
[tested]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1297-L1317
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_policy.go#L202-L242
[optional]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1274-L1281
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/core/defaults/policy/recurse.mg#L260-L329
[revert]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1383-L1432
[guard-test]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_policy_test.go#L316-L332
[mission-order]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L1150-L1188
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission.go#L209-L248
[mission-finish]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_mission.go#L319-L356
[mission-bounds]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/core/defaults/policy/mission.mg#L1-L24
[reason]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/internal/campaign/recurse_cycle.go#L286-L302
[innovation]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/innovations/mission-driven-forever-improvement.md#L40-L48
[subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/recurse-gate-subsumption.md#L158-L190
[coverage-audit]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/recurse-coverage-gate-identity.md#L5-L17
[census-audit]: https://github.com/theRebelliousNerd/codenerd/blob/158be02e0641acd01b85d323405d256ccb6eb308/Docs/audits/recurse-finding-census-completeness.md#L3-L24
