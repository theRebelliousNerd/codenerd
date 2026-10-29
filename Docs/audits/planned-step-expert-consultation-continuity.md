# Expert consultation continuity across planned steps

## Decision and evidence boundary

Keep the existing per-step JIT compilation, but give successful consultation results an explicit carrier across that compilation boundary. CodeNERD can currently pay for a relevant expert's pre-turn advice, then execute every planned edit step with a newly compiled prompt that does not carry that advice. The original enriched prompt remains available for the final verification stage, so this is a loss of pre-edit delivery, not proof that the answer is lost everywhere.

This report is pinned to public main [`99c5526e69391fafbce678b5fa467d589c4ad2a5`](https://github.com/theRebelliousNerd/codenerd/commit/99c5526e69391fafbce678b5fa467d589c4ad2a5), inspected October 29, 2026. It is based on source inspection and a conditional request-flow witness. No application execution, tests, model calls, inference, training or benchmarks were run. No quality loss, task failure rate or token saving is measured here.

Initial expert selection and consultation, per-step JIT selection, and post-edit expert critique are implemented. A structured consultation carrier and step-role reconciliation are proposals. Preserve the kernel's control over which experts are needed, and preserve the independent tool, safety and verification gates.

## Prior work and why this is a separate seam

The [consultation coordination audit](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/consultation-coordination-and-safe-reuse.md) concerns shared consultation identity, partial outcomes and reuse. [Campaign specialist retrieval](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/campaign-specialist-retrieval-contract.md) concerns delivery of stored knowledge. [Planned-work conservation](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/planned-work-conservation.md) concerns admission and reporting of planned edit sites. None should be replaced by a second implementation of its contract.

The [slash target continuity audit](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/slash-command-specialist-target-continuity.md) identifies a route where the initial selection never gets a target. Here, grant the opposite: the correct target arrives, a relevant expert is selected, and its consultation succeeds. The next transformation is what fails to preserve its result. Likewise, the [expert lending audit](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/expert-lending-dependency-contract.md) concerns dependencies within stored prompt atoms. A newly produced consultation answer is not the same object as those atoms.

The current main audit inventory, adjacent documents above, and session guidance were inspected. The existing guidance correctly requires step-specific knowledge and does not justify abandoning that improvement to keep one static prompt.

## Production path: the consultation succeeds before the boundary

The executor selects experts from the admitted intent target, supplies their names to initial JIT compilation, then assembles the compiled prompt with project instructions and target context. It calls `withExpertConsultations` and records the selected experts that owe critique before entering the tool loop. [Selection](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/executor.go#L1373-L1377), [consultation and loop handoff](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/executor.go#L1451-L1467).

`withExpertConsultations` runs the derived lead/advice requests, waits for their results, and appends a section to the system-prompt string. It preserves failed and empty outcomes as explanatory text, rather than silently treating them as successful advice. It returns the augmented string, not a collection of typed consultation artifacts that later compilation can select. The consultation task includes the intent, actual input and files in the expert's domain. [Consultation assembly](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/expert_spawn.go#L51-L113), [task construction](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/expert_spawn.go#L123-L145).

The normal `JITExecutor.SpawnConsultation` executes an isolated consultation subagent through `RunUntilVerified` and returns its output. The parent handoff shown above consumes that returned text. This report does not rely on a failed consultation, a fabricated response, or an async task identifier being mistaken for an answer. [Consultation producer](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/task_executor.go#L472-L485).

## The planned execution route replaces the enriched prompt

Before the first normal tool-loop generation, `runToolLoop` calls `planTurnSteps`. If the result has more than one step, it enters `runPlannedSteps`; otherwise it passes the original system prompt to the single-pass loop. [Branch](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/executor_tools.go#L61-L78).

Planning does not consume the enriched prompt either. It compiles the work-step planner role and calls the model with that role prompt and `workStepPlanUser(task, verb)`. Planning is subject to existing eligibility checks, including a write-oriented intent, working-loop support, a usable tool-results channel, write tools and sufficient edit sites in the brief. Therefore not every write task enters this route. [Eligibility and planner request](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/work_steps.go#L168-L220), [multi-step admission](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/work_steps.go#L239-L253).

For each admitted step, execution copies the compilation context, replaces `IntentTarget` with the step's file, and calls `stepSystemPrompt`. That helper recomputes language, needs and expert names, compiles a new JIT prompt, then returns that new prompt with project instructions and file context. It does not append the initial consultation section or accept an artifact containing the answers. The `turnPrompt` argument is used only when the compiler is absent or the new compile fails, is nil, or returns an empty prompt. [Step compilation](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/work_steps.go#L783-L819).

The normal and commit-regime retry passes both receive this step prompt. [Normal step](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/work_steps.go#L854-L871), [retry](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/work_steps.go#L891-L899). Initial generation uses the supplied prompt. [Request consumer](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/executor_tools.go#L92-L108).

The final shared verification call does receive the original `systemPrompt`, including its initial consultation section. Repair calls made through that stage can therefore have the advice available. This is an important recovery path, not proof that the model had the advice when it chose and made the original edits. [Final verification](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/work_steps.go#L943-L961).

## A precise conditional witness

Use a normally admitted `/fix` or `/refactor` task with a valid initial target and a registered expert selected by actual policy. The task identifies at least two edit sites and the planner returns two admitted steps. Grant successful initial JIT and consultation. Let the expert return an answer-specific marker or concrete recommendation that is absent from the task, static expert knowledge, project instructions and file contents.

Then let each step's JIT compile succeed with a nonempty prompt. Capture the actual provider requests, rather than only the initial compiler result:

1. The consultation produces the marker, and the initial `systemPrompt` contains it.
2. The planner request has its own role and task, not this consultation section.
3. Each planned edit pass uses a fresh step prompt without the answer carrier.
4. The final verification stage retains the original enriched prompt.

The marker isolates transport continuity; it is not evidence that arbitrary expert prose is useful or should be forced into every step. A substantive evaluation should separately choose advice that is relevant to one of the steps and assess its effect on verified work.

Controls include a one-step or no-plan task, where the original enriched prompt reaches the ordinary loop, and a failed or empty step compile, where the helper deliberately falls back to that original prompt. Those paths should continue working. Static expert atoms may independently convey the same advice, tools may discover the underlying fact, or the main model may reason correctly without it. The finding is not guaranteed incorrect code or total expert blindness.

## Related loss: step selection projects away new expert roles

At the step boundary, `neededExperts` calls the full selector and returns only `Selection.Agents()`. The selection type also carries `Spawn` and `Critique` decisions, but those are not returned by this projection. [Helper](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/expert_context.go#L19-L40), [selection contract](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/experts/select.go#L44-L99).

An expert needed only for a later step can thus supply stored JIT knowledge without its step-derived consultation or critique role being consumed by this path. The initial executor records critics from its initial selection, and the post-edit round reads those recorded rows. The selector's temporary task facts are retracted after selection, so they are not a durable replacement for an explicit turn critic record. [Selector lifetime](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/experts/select.go#L104-L133), [critic recording and consumption](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/session/expert_critique_round.go#L32-L46).

This second case requires a newly relevant expert that was not already included in the initial selection. It does not imply the initial expert's critique disappears. The policy intentionally distinguishes low-confidence language-only knowledge from stronger import-driven spawning; do not turn every step's expert name into a mandatory new model call. [Confidence and spawning policy](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/core/defaults/policy/shard_experts.mg#L74-L116).

## Proposed artifact and delivery contract

Represent consultation outcomes independently of the prompt string. Reuse existing task/turn identity and consultation machinery rather than constructing another scheduler. A bounded artifact should carry the producing request and agent, role, task and file scope, relevant source revision, response or error/empty disposition, and applicability limits. A per-request delivery receipt should identify which artifact or approved projection reached which planner, step or repair request.

W3C PROV-DM distinguishes an entity's generation from its use by an activity and models derivation and invalidation. That is useful grounding for separating “the expert answered” from “this editing request received the answer.” The proposed CodeNERD receipt is an application of that distinction, not a claim that provenance itself proves advice correct or that adopting a particular serialization fixes the route. [Primary specification](https://www.w3.org/TR/prov-dm/).

Proposed flow:

1. Run initial expert selection once and retain typed outcomes for the consultations it requests.
2. At planning, admit only advice relevant to choosing edit sites. A planner that needs none can record that disposition without replaying all answers.
3. At each step, retain the full current expert selection. Reconcile its roles with already satisfied consultation requests and recorded critics under the same parent turn.
4. Let the existing executive policy decide whether existing advice remains applicable, a new consultation is needed, or a missing opinion is merely disclosed. A changed file revision can invalidate advice even when the agent name is unchanged.
5. Compile the step's static knowledge and bounded applicable consultation projections together, preserving independent tool availability and authority. The receipt must reflect the actual rendered provider request, not a candidate that was later pruned.
6. Record newly owed critique once per relevant agent/scope under the parent turn, then preserve the normal post-edit ordering and repair gates.
7. Reuse the same admitted artifact for a commit-regime retry when still applicable. Do not reconsult merely because a prompt was recompiled.
8. Retire or archive turn artifacts after their consumers finish; bound retained bytes and provide explicit unavailable/truncated dispositions.

The smallest repair is a separate typed consultation-result collection supplied to step assembly. A more integrated option converts bounded artifact projections into dynamic JIT candidates with source and consumer identity. Either must avoid simply copying the whole initial prompt, which would reintroduce the wrong-language and irrelevant-context problems that step JIT was designed to solve.

## Costs and failure modes

The current path has already paid for initial consultation execution before deciding whether the task is planned. That cost includes its JIT compilation, model input/output and reasoning, tool work, latency, and possible retries. Restoring delivery may add input tokens to each relevant step; it does not make those tokens free or automatically reduce total spend. Conversely, re-running every expert on every step can multiply work while delivering no new evidence.

For S steps and A artifacts, naive applicability scanning costs O(SA), plus serialization proportional to admitted content. Indexing by target/domain can reduce lookup work but adds index storage and invalidation cost. Full prompt copies can consume memory and erode prefix-cache efficiency. Charge selection queries, fact cleanup, compilation, artifact validation, copying, storage, all consultations, planning, edit and repair calls, tool processes, verification and failed/cancelled attempts. Measure cold and warm operation separately.

Risks include stale advice after earlier edits, scope expansion beyond the expert's actual review, context overflow, conflicting expert recommendations, duplicate consultation or critique, artifacts attributed to the wrong resumed turn, and missing failure disclosures. A successful transport receipt is not a correctness verdict. Expert text must not gain authority to widen the tool envelope or waive verification. Any compression should preserve source scope and uncertainty, and disclose when the projection omits answer-critical content.

## Fair evaluation and acceptance

No evaluations below were run. At the socket level, use deterministic expert replies with unique markers and the real selector, plan branch, step compiler, loop request construction and final verification handoff. Compare current behavior, whole-prompt copying as a diagnostic control, a typed artifact carrier, and scope-aware JIT admission. Hold policy, task, static knowledge and compiler outputs fixed where needed to isolate delivery.

Cover: successful multi-step compiles; one-step/no-plan controls; failed, nil and empty compile fallback; relevant versus irrelevant advice; failed/empty consultations; two experts with different scopes; later steps introducing a new import-domain expert; an expert already consulted initially; commit-regime retries; source changes that invalidate earlier advice; cancellation; and a tight context window. Verify that new expert roles are reconciled without forcing language-only spawns or duplicating critique. Capture actual provider requests and executed consultation counts, not only `cc.Experts`.

At the complete-system level, compare matched multi-file tasks using the same model, tools, source revisions, expert material, verification obligations and total budget. Report verified task success, constraint compliance, needless edits, reviewer corrections, number and timeliness of useful advice deliveries, prompt/context failures, total tokens and cost, wall time, CPU and peak memory. Include a generic-only baseline to distinguish the cost and value of experts from the correctness of their transport. Count retries and abandoned work. Fewer calls caused by omitting advice are not an efficiency win; more advice tokens are not a quality win without evidence.

Acceptance requires that a successful, relevant consultation has a checkable disposition at each intended editing consumer; that planned-step compilation preserves current language and target knowledge; that step-only expert roles are either fulfilled or explicitly disposed under policy; and that ordinary single-pass, compile-fallback and final repair paths retain their existing behavior. The useful target is not “every prompt contains everything.” It is that a completed piece of reasoning reaches the request that needs it, at the right revision, with its costs and limitations visible.
