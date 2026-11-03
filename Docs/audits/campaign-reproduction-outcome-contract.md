# Give campaign reproduction tasks a diagnostic outcome, not a green-suite prerequisite

**Status:** source-supported conditional integration gap; proposal, not an implemented fix or reproduced incident.  
**Source freeze:** public CodeNERD main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), inspected 2026-10-30.  
**Method:** static source and documentation inspection, including independent challenge. No application execution, tests, model inference, training, benchmarks, local Git or repository modifications were performed. Examples and evaluations below are proposed, not observed.

## Finding and decision

The campaign policy can require a reproduction run **before retrying a failed code-writing task**. The host inserts a diagnostic `/test_run` and makes the repair depend on it. That diagnostic then uses the ordinary green-suite handler: a real, normally completed test process reporting a failure is an unsuccessful task attempt, even when it has reproduced the failure the diagnostic was supposed to establish.

Consequently, **if the workspace remains red when the diagnostic runs**, its observation does not settle the diagnostic prerequisite. The repair remains ineligible while the diagnostic retries, and may require a later replan or operator intervention. The current behavior is fail-closed: it does **not** falsely claim the suite passed. The gap is a conflation of “the diagnostic obtained its evidence” with “the system under diagnosis is repaired.” [Producer and dependency][insert], [host interpretation][test-run], [dependency policy][dep-policy]

This is not an inevitable deadlock. Failed mutation rollback can restore a green workspace; the diagnostic can then pass. Attempt limits, backoff, replanning, independent repairs, cancellation and explicit task dispositions can change the outcome. The requested extension is a typed, policy-decided **diagnostic settlement contract**, while preserving ordinary test, repair and phase acceptance gates.

## Prior work and inspection boundary

The root guidance makes Mangle the executive and Go the measurement/FFI layer. Campaign guidance separately requires host execution and a successful, unchanged-workspace witness for ordinary `/test_run` verification. Those are safeguards to retain, not exceptions to remove. [Root guidance][root-guidance], [campaign guidance][campaign-guidance]

This is a continuation of known diagnostic quality concerns, not a claim that reproduction handling was never reviewed:

- July 9 QA already raises **repro-task cascade** and calls for protection against a failing diagnostic spawning another diagnostic. The current policy and tests provide that protection. [Earlier QA][qa-cascade], [current no-cascade test][tests]
- July 20 QA explicitly recommends tracking whether repro diagnostics gather useful context. The contribution here is the concrete producer → host runner → hard dependency trace that prevents a red observation from settling that prerequisite. [Earlier quality concern][qa-quality]
- The current failure tests deliberately verify insertion, the parent's dependency and reuse of the pending diagnostic. They do not execute that diagnostic through the real test handler. They are valuable controls, not an end-to-end red-reproduction result. [Current tests][tests]
- The external-audit journey's N02 correction made the kernel the sole scheduler. This proposal does not restore an in-memory scheduling fallback or ignore an unsatisfied dependency. [Scheduler safeguard][scheduler]

The existing audit inventory and research branches through `research-main-20261030-1450` were screened. The latter is docs-only and includes intake-pause ownership, not this diagnostic outcome seam. On the five active non-research branch heads inspected, the reproduction producer and decision policy match main, and the relevant `executeTask`, `executeTestRunTask` and blocking-dependency bodies are unchanged, even where their surrounding files differ. This review does not assert that every unrelated branch change is equivalent. [Latest research branch][research-1450]

## Production route

| Stage | Source-grounded behavior |
|---|---|
| Normal TUI entry | `/campaign start` plans and installs an orchestrator; `campaignStartedMsg` launches its background run. No custom scheduler or test-only entry is required. [Command][command], [start/run][chat-run], [Update][chat-update] |
| Actual failed task | The task executor's observed return is run through `RunUntilVerified`. The campaign carries the non-done outcome and missing-evidence atoms on its error; merely containing the word “test” in arbitrary prose is not the trigger. [Observed task handoff][spawn], [typed signals][signals] |
| Policy admission | A declared Go write extension establishes `task_writes_code`. Red signals include `/tests_not_green`, `/test_run_not_green` and `/tests_red`. After the configured count, with the latest failure red and the task not exhausted, the policy chooses `/repro_first`. [Write class][write-class], [decision policy][decision] |
| Host insertion | The failed task returns to pending/backoff. The inserted diagnostic is critical-priority `TaskTypeTestRun`, has `InferredFrom` and the repro inference reason, and becomes a hard `DependsOn` predecessor of the repair. It has no artifact target or inherited package selector. [Failure transition][failure-transition], [insertion][insert] |
| Real test execution | All `/test_run` tasks route to `executeTestRunTask` before explicit-shard routing. Its no-artifact default is `./...`; it constructs a host Go test command with workspace tags and `-count=1`, asserts the pending action and routes it through VirtualStore. [Dispatch][dispatch], [runner][test-run] |
| Observation interpretation | VirtualStore requires an error-free, successful execution with exit code zero. A nonzero failing suite returns `Success:false`; the campaign turns that into a `/tests_red` error. The post-run snapshot and positive `TestWitness` are only reached after success. [VirtualStore result][vs-test], [campaign result][test-run] |
| Scheduling consequence | `runSingleTask` handles the error as another failed attempt instead of calling `completeTask`. A pending, in-progress or failed diagnostic blocks its dependent; only completed/skipped satisfies the existing task-dependency rule. [Settlement][settlement], [dependency][dep-policy] |

The host does not interpret the diagnostic's natural-language brief, its previous failure line or its inference reason as a different expected result. In the inspected code, repro identity is used for recognition/reuse and to prevent recursive repro insertion, not to admit an observed failing test as a settled diagnostic. [Recognition][insert], [no-cascade policy][decision]

Default configuration uses **two** red-suite failures to ask for reproduction, **four** failed attempts as the task cap, replan-at-cap enabled, and a **15-minute** test-run timeout. These are configured bounds, not measured costs or a promise that every diagnostic consumes the timeout. [Defaults][defaults]

## Conditional witness: a red observation keeps the repair waiting

Use a synthetic valid campaign with an active phase and a repair task whose declared target is `internal/math/number.go`. Require:

1. The accepted campaign and kernel facts are consistent; the task has remaining attempts. Two actual observed task attempts have ended with a qualifying typed red-suite signal. No `/blocked` obstacle signal has already exhausted the task.
2. The policy derives `/repro_first`; insertion and dependency publication succeed. The case does not depend on a missing kernel row, ignored assertion error or malformed plan.
3. After all failed-attempt cleanup, the admitted diagnostic command still encounters an ordinary failing test. The source can be a stable pre-existing failure, or a separate red package within the default `./...` scope. If claiming that the *intended* failure was reproduced, the fixture must establish that failure's identity independently; the current diagnostic does not check such a match.
4. Boot, constitutional and permission checks admit the host action; its workspace, dependencies and tooling are available. The process finishes normally with a failing test, not cancellation, timeout, missing credentials or a failed process launch. No concurrent writer changes the workspace during the witness.
5. No other task, external editor or replan repairs/skips/retypes the diagnostic before the scheduling observation.

The resulting source-derived sequence is:

- The producer creates diagnostic D and records repair R → D.
- D really executes and observes red.
- D returns an error at the `!result.Success` branch; no positive test witness is created.
- `runSingleTask` records D's failed attempt. Below the cap it can retry after backoff.
- R is withheld by `has_blocking_task_dep`, even after R's own retry time has elapsed, because D has not completed or been skipped.

This establishes a **mis-settled diagnostic prerequisite and possible repeated verification cost**, not a prediction that a particular model prompt produces two bad attempts. The fixture's first two failures are explicit preconditions that must themselves be shown in an eventual production-path evaluation.

An unrelated red package is an important precision control: it demonstrates why blindly accepting *any* nonzero exit as successful reproduction would be unsafe. The new contract must identify what was run and which failure was observed, rather than converting an arbitrary build failure into evidence for the repair.

## Safeguards, competing paths and limits

- **Rollback matters.** Mutating attempts use the current snapshot/owned-write undo path. If the attempted change introduced the only failure and undo restores green, D can pass and R can retry. Do not assume a failed mutation's broken bytes necessarily survive into reproduction. [Attempt wrapper][rollback]
- **This is not a bypass finding.** Host tests still pass through VirtualStore's boot, constitution and default-deny permission gates. A denied action is not the ordinary red-test witness. The proposal must not relax those checks. [Routing guards][routing]
- **No infinite repro cascade is claimed.** `task_is_repro` prevents D from owing its own reproduction; a pending/in-progress existing diagnostic is reused. [Policy][decision], [producer][insert]
- **Attempts are bounded, but replanning can change the plan.** At the cap the task fails or receives a one-time replan according to policy. The replanner can skip or modify tasks; a skipped D satisfies the existing dependency rule. A valid replan can therefore restore progress. Its choice and eventual repair quality are not predicted here. [Cap and replan][decision], [host replan][failure-transition], [plan changes][replan]
- **Backoff is not deadlock.** Current clock publication and retry scheduling retain waits as waits. The relevant observation is R's withheld eligibility while D remains unsettled, not a claim of a busy loop. [Scheduler][scheduler], [retry/dependency policy][dep-policy]
- **Failure evidence is not invisible.** The diagnostic's error, attempts, typed signals and failure event remain available. The defect does not require a misleading success message or the loss of all logs. [Failure recording][failure-transition]
- **Completion stays separate.** Ordinary repair turns must still satisfy their own verdict and mutation checks; phase checkpoints and any campaign acceptance command remain additional obligations. Diagnostic settlement must not mint a passing `TestExecutionWitness`, `test_state(/passing)`, a successful regression check or a repaired-campaign claim from a red result.
- **The test runner itself may have effects.** This witness explicitly requires a stable workspace. A proposed red-result path needs the same post-run identity check currently performed only after a green result; “diagnostic” must not excuse changed inputs or unexplained side effects.
- **Recovery is not universally automatic.** Resume preserves attempt-cap limitations; changing a state to pending is not proof that a stable red diagnostic will now pass. [Resume limits][resume]

## Proposed contract

### 1. Keep purpose, execution and disposition distinct

Introduce a host-owned diagnostic obligation tied to **campaign, repair task, failed attempt and diagnostic attempt**. Record:

- purpose: reproduction/baseline observation versus regression verification;
- original command or typed package/test selection, tags, relevant environment/tool identity and originating failure evidence;
- before/after workspace identity and whether a real test process and selected tests executed;
- observed test failures, compile/setup failures, cancellation and timeout as separate outcomes;
- durable bounded summary plus a retrievable full receipt; expected failure matching and its uncertainty.

Go measures those facts; Mangle decides whether the diagnostic is **settled**, **inconclusive**, **stale**, **blocked**, or needs a narrower/repeated observation. An inference reason or free-form description alone should not authorize relaxing verification.

A matching, genuinely executed red result can settle a reproduction obligation **without passing the suite**. A green result can record “not reproduced on this snapshot,” with the next repair/replan move derived explicitly. An unrelated failure, no-test selection, compile-only failure, timeout, refusal or changed snapshot must not masquerade as the expected reproduction.

### 2. Make the dependency carry the right obligation

Prefer a typed diagnostic dependency or a policy-derived diagnostic settlement relation, rather than forcing a diagnostic through ordinary “tests passed” completion. If implementation instead retains `TaskCompleted` for the *diagnostic work*, every receipt/UI/learning consumer must distinguish that from successful software verification. Do not silently reuse existing `task_result(..., /success, ...)` or the positive test witness as if those two meanings were already equivalent.

Release only the originating repair's diagnostic hold. Preserve its remaining hard dependencies, backoff, permission/risk checks, test debt, task verdict, checkpoint and acceptance obligations. Hand the repair the observation and snapshot identity; do not merely remove the dependency and discard why it was run. Fence late results by task/attempt identity and define behavior when replanning retires the parent.

### 3. Preserve useful existing boundaries

Ordinary `/test_run` remains green-only. Do not turn this into a blanket “failed tests count as success” change. The separate change-evidence subsystem already distinguishes a **failing reproducer baseline** from a **passing regression baseline**, validates the selected test file/package and checks an unchanged snapshot. It is an in-repository design precedent, not evidence that campaign diagnostics already use it or that its whole API can be transplanted without cost. [Existing purpose-sensitive baseline][evidence-baseline]

Legacy saved diagnostics need explicit migration behavior: re-observe when a trustworthy current receipt is absent, or stop with a named inconclusive obligation. Do not infer expected failure identity from an old prose description and grant a green witness. Keep existing original attempt records immutable.

## Alternatives and full cost

| Alternative | Benefit | Cost / limitation |
|---|---|---|
| Keep current behavior and rely on cap/replan | No new state or policy; retains conservative verification. | Stable red diagnostics can spend repeated host runs and a planning call before repair resumes; recovery depends on replan/operator decisions. |
| Disable reproduction for the affected campaign or set its threshold beyond the attempt cap | Avoids this inserted prerequisite. | Removes the intended diagnostic pressure rather than supplying it; not a generally equivalent repair. |
| Accept any red diagnostic as completed | Small apparent change. | Unsafe conflation: infrastructure failures, unrelated packages and zero meaningful test execution can discharge the wrong obligation; downstream “success” and learning meanings also change. Reject this shortcut. |
| Narrow the diagnostic command only | Can reduce unrelated failures and full-suite work. | Does not resolve expected-red versus green semantics. Selection must come from observed evidence, not guessed text. |
| Typed, attempt-owned diagnostic receipt and derived settlement | Preserves truth, useful red evidence and independent verification gates. | Schema/policy, persistence, scheduler dependency, UI and downstream receipt/learning consumers require coordinated changes; identity matching and migration add complexity. |

No additional model request is inherently needed to classify a structured runner receipt. An optional model-based narrowing/explanation path would add tokens, latency, another failure mode and admission requirements, and must be priced separately.

Host costs include test compilation/execution, workspace hashing before **and after** unsuccessful runs, parsing/storing named events, receipt retention/hydration, kernel fact cardinality/fixpoint work, persistence and restart migration. Large-suite observations may require bounded summaries and deduplicated blobs. Preserve original output and avoid storing secrets indiscriminately. Measure human diagnosis/recovery time, not just test-process duration. No net token, CPU or latency saving is established by this source review.

## Fair evaluation plan — not run

Compare (A) current main, (B) scope-only narrowing and (C) typed diagnostic settlement under the same fixtures, toolchain/environment, policy limits, admission and deterministic executor outcomes. Exercise the actual TUI → orchestrator → observed attempt → producer → VirtualStore → kernel scheduler path; isolated rule/handler tests are supplemental.

| Case | Required observation |
|---|---|
| Intended failure remains red after two qualifying attempts | D records genuine matching red; current R remains blocked; proposed R can become eligible only when all other obligations permit. No green-suite witness is minted. |
| Rollback restores green | Current behavior can already progress; proposed path preserves this control and accurately records “not reproduced.” |
| Unrelated failing package under `./...` | Must not be accepted as matching reproduction merely because the command exited nonzero. Compare broad versus evidence-derived scope. |
| Regression `/test_run` with identical red output | Remains unsuccessful; no diagnostic exemption leaks into normal verification. |
| Timeout, cancellation, denial, launch failure, compile failure, no tests and skipped tests | Distinct dispositions; none automatically becomes expected reproduction. |
| Workspace changes during a red run | Receipt becomes stale/inconclusive; no settlement from mixed inputs. |
| Code versus Markdown/no qualifying extension; below threshold; exhausted parent | Existing admission and no-unnecessary-repro behavior preserved. |
| Repeated parent failures and failing diagnostic | One active diagnostic reused; no recursive cascade; attempt and backoff behavior remain accountable. |
| Cap with replan enabled/disabled; valid skip/retype; failed replan | Demonstrate current rescue/failure paths rather than claiming inevitable deadlock. |
| Pause/stop/restart, old result after replan, two diagnostic attempts | Owner/attempt freshness, durable receipt and cancellation preserved; no stale discharge. |
| Repaired task later fails a real verification/checkpoint | Diagnostic settlement does not hide that failure or complete the campaign. |

Record selected and executed tests, intended/actual failure identities, parent eligibility facts, diagnostic versus regression dispositions, attempt/replan counts, host invocations and duration, snapshot bytes/time, output/receipt bytes, kernel work, memory, model tokens/calls and operator interventions. Report false settlements and false holds separately. Include cases where the current conservative behavior is preferable and costs where the richer contract is slower.

**Acceptance criterion:** a valid red diagnostic observation can inform and settle the correctly owned reproduction obligation without asserting the software is correct, while unrelated/incomplete/stale observations cannot release it and all final verification obligations retain their meaning.

[insert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure.go#L193-L278
[test-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L735-L799
[dep-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L18-L106
[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/agents.md
[qa-cascade]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/2026-07-09_23-20-EST_orchestrator_boundary_analysis.md#L153-L157
[qa-quality]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/2026-07-20_orchestrator_failure_analysis.md#L399-L404
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure_test.go#L83-L182
[scheduler]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L68-L115
[research-1450]: https://github.com/theRebelliousNerd/codenerd/commit/ac3013965dd2ce27d66f04ab833638ac8f4db788
[command]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L660-L684
[chat-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L29-L153
[chat-update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L325-L353
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L80-L113
[signals]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/failure_signals.go#L56-L106
[write-class]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_decisions.mg#L30-L43
[decision]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_decisions.mg#L154-L249
[failure-transition]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure.go#L26-L160
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L209-L231
[vs-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store_actions.go#L438-L487
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L450-L477
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/campaign.go#L162-L180
[rollback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L52-L107
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store_routing.go#L35-L136
[replan]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L1038-L1073
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_resume.go#L110-L125
[evidence-baseline]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/evidence/change.go#L290-L323

