---
doc-class: deep-dive
subsystem: expert-card-dealing
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Expert card continuity into final repair

## Conclusion and evidence boundary

The first expert-card tranche fixes an important transport boundary: a consultation is retained as a typed outcome, Mangle deals answered lead cards to planned steps and adviser cards to their selected files, and each step names withheld or unsuccessful cards. Step-specific selection also buys consultations for newly relevant experts instead of merely lending their stored knowledge. Initial consultations still reach final verification through the original turn prompt.

A narrower residual concerns **consultations first obtained inside a planned step**. Their answers reach that step and remain in the result's card collection, but the final shared repair stage receives the original turn prompt rather than a stage-appropriate projection of the accumulated cards. Thus an answer can be present while the step edits a file and absent when a later build or test repair edits the same file.

This is static source inspection of public main `486db57ce490302c65d7f484e7f6070ee0e70862`. It is a conditional transport witness, not an executed failure, guaranteed bad repair, or measured regression. No tests, application execution, model inference, training or benchmarks were performed. This document proposes no weakening of tool, permission, write-set or verification gates.

## Prior work and distinction

The [planned-step consultation audit](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/planned-step-expert-consultation-continuity.md) identified loss of initial consultation prose during successful step recompilation. Its observation that final verification retains the original consultation remains correct for consultations already in the original prompt. The implementation now carries those answers into steps and can obtain new answers during step compilation; the latter answers are not retroactively added to the original string.

The research branch's [expert-card scope audit](https://github.com/theRebelliousNerd/codenerd/blob/3c8192aa3009058f5f78a4e74a42bd86691e29fa/Docs/audits/expert-card-scope-continuity.md) addresses an already-consulted adviser selected for a new file: agent-wide deduplication prevents a supplement while the old card's scope excludes the file. Here the expert is new, consultation succeeds, and the initial step receives it. The later consumer boundary is the problem, not adviser scope or consultation deduplication. A lead card suffices to demonstrate this residual.

The [plan-bound expert-decks proposal](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/innovations/plan-bound-expert-decks.md) already distinguishes generated advice, actual delivery and discharged obligations. This finding is one concrete consumer boundary for that contract, not a replacement deck framework.

## Producer-to-consumer trace

1. Each step selects its own experts, records critique obligations, compiles its context, receives already applicable cards, and obtains any new consultations. The returned string is the step prompt: [work_steps.go 819–843](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/work_steps.go#L819-L843).
2. The consultation helper marks agents consulted, records typed cards, and returns its augmented prompt string. It does not update the caller's original turn prompt: [expert_spawn.go 96–115](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_spawn.go#L96-L115). Card recording stores results and publishes card/file facts: [expert_cards.go 65–81](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_cards.go#L65-L81).
3. The normal step and its commit-regime retry consume `stepPrompt`: [work_steps.go 889–895](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/work_steps.go#L889-L895), [923](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/work_steps.go#L923).
4. After the steps, the shared gate builds history from prior-turn messages, the task and `last.Text`. It calls `verifyCompletedToolTurn` with the original `systemPrompt`, not `stepPrompt` or the accumulated card collection: [work_steps.go 967–985](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/work_steps.go#L967-L985).
5. The gate forwards that prompt to build, test and critic repair functions: [executor_tools.go 480–500](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor_tools.go#L480-L500). The build path passes it to `repairLoop`: [build_verify.go 321](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/build_verify.go#L321), then `repairRound`: [repair_loop.go 268](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/repair_loop.go#L268), then `workingAttempt`: [build_verify.go 787](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/build_verify.go#L787). The request uses that prompt in `completeWithWorkingContext`: [build_verify.go 823–836](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/build_verify.go#L823-L836).

The inspected card implementation has no separate repair-stage projection. Its current policy is step-file based: [shard_experts.mg 225–236](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/shard_experts.mg#L225-L236). Merely retaining the answer in `ExecutionResult` does not deliver it to a model request.

## Conditional witness and controls

Use a normally admitted multi-step write task. The initial target does not select expert E. A later planned file B supplies strong import evidence for E, so successful step compilation obtains E's consultation. Give E a unique answer-specific marker absent from the task, static KB, project instructions and file contents. Use a lead-classified expert to avoid conflating this case with adviser-file scope.

The step on B receives the answer. Grant a later build or test failure that admits a repair request touching B, and suppose the final step response does not repeat the marker. The final repair request's original system prompt does not contain the step-only answer, and the shown history handoff does not copy that step's system prompt. This isolates transport rather than assuming the advice must improve the repair.

Important controls and limitations:

- An expert consulted before planning is not this case: its prose already lives in the original turn prompt.
- No final repair means no affected editing consumer. A passing verification is not itself a missing-advice failure.
- The last response may repeat the advice; tool observations, static knowledge or the repairing model may independently recover the same reasoning. Such contingent repetition is not a card-delivery contract.
- Expert critique remains recorded and may introduce relevant findings later. This is not disappearance of all domain review, nor proof that every repair lacks domain knowledge.
- A failed step compile returns the original turn prompt before buying a new step consultation; it does not satisfy the witness's successful-production premise.
- The normal step and its retry retain their local advice. The residual begins after those passes, at the shared final stage.
- Freshness matters independently: copying every historical card could expose obsolete advice after intervening edits. Transport repair must not silently declare old claims current.

## Design alternatives

Define the consumer contract before changing rendering. A final build repair, a scoped test repair and a whole-task verification review need not receive the same hand.

**Stage-aware dealing.** Publish the repair's stage and grounded affected-file scope, then have policy derive applicable accumulated cards. Preserve producing scope and revision information. If the affected scope is unavailable, policy should choose explicit broader delivery, retrieval or an unavailable disposition rather than disguising an empty scope as complete filtering.

**Bounded repair manifest with retrieval.** Include card identities, scopes and outcomes in a compact manifest, with an existing bounded retrieval mechanism for full answers. This avoids blind whole-deck copying but needs an actual recoverable artifact path and an admission rule for critical material. A handle the repair cannot resolve is not preservation.

**Whole-turn projection as a diagnostic control.** Rendering all accumulated answers into the final prompt is simple and useful for testing transport. It is not automatically the production answer: it can leak irrelevant file-specific claims, repeat initial cards, consume context, and preserve stale assumptions. Do not mutate an in-flight prompt or weaken file-sensitive dealing to force universal delivery.

**Explicit non-delivery.** Policy may decide an advisory card is unnecessary for a particular repair and record why. That is different from the caller never considering step-produced cards. Failed, empty, unavailable and deliberately omitted outcomes should remain distinguishable.

None of these alternatives turns expert prose into authority. Advisory steering is not a mandate. Obligations require authorization from the user/specification/existing policy and checkable discharge; an expert cannot widen capabilities, waive tests or invent an unlimited chain of required work. Likewise, delivered advice is not verified behavior. Keep card generation, applicability, rendering and verification as separate receipts.

## Validation and full-cost comparison

No evaluation was run. Begin with deterministic transport fixtures exercising the actual successful step compile and final repair request boundary. Assert both positive delivery and negative leakage, not merely card storage or candidate selection.

Cover initial-only versus step-only experts; leads and advisers; multiple later experts; a repair on the consulted file versus an unrelated file; no repair; final-response repetition as a rescue control; build and test repairs; failed/empty consultation; failed compilation; intervening edits invalidating advice; repeated repair rounds; and tight request budgets. Verify that unchanged initial consultations are not duplicated and that permission/write/verification constraints stay unchanged.

Compare current behavior, whole-turn copying as a diagnostic baseline, and the chosen stage-aware projection on matched tasks and identical source/model/tool constraints. At the request level, measure correct card delivery, incorrect leakage, explicit omissions and freshness dispositions. At the task level, separately measure independently verified success, unnecessary edits, repair regressions and reviewer corrections. A marker arriving is transport success, not proof of better reasoning.

Charge initial and step consultations, any projection or reconsultation calls, JIT/fixpoint work, full input expansion, cached versus uncached tokens, repair/critique/verification work, retries, failures, latency and retained artifact memory. A repair that reads fewer tokens by silently losing necessary advice is not an efficiency win; copying irrelevant advice is not a quality win. Report complete verified-task cost rather than only the local repair prompt size.

Acceptance is a stage-appropriate, source-grounded disposition for accumulated consultation artifacts at each final repair request, while preserving initial-turn continuity, step-local dealing, freshness, bounded context and independent authorization and verification.
