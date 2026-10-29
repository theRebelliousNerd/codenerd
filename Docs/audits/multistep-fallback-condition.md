---
title: "Multi-step fallback conditions are lost before TUI dispatch"
status: "Source-confirmed wiring defect; evaluation proposed, not run"
reviewed-on: "2026-10-28"
repository: "theRebelliousNerd/codenerd"
main-compared-against: "356f65e0e6bb639adaa90ab36212daa714114871"
active-feature-compared-against: "c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c"
active-feature-branch: "feat/conversation-brief"
current-research-compared-against: "a636eed04b7650ea4037c9494d86e7337d4df6b3"
historical-research-compared-against: "c592ff223cd207fca4c29ae793d5eba57aa4781d"
scope: "Source-only research; no implementation, tests, runtime inference, or repository writes"
---

# Multi-step fallback conditions are lost before TUI dispatch

## Finding

The TUI recognizes a difference between “do Y if X succeeds” and “do Y if X fails,” but its executable step representation discards that difference. Both become an ordinary predecessor index. The multi-step executor then enables every dependent step when the predecessor returned a **nil Go error**, not when the requested condition is true.

Consequently, a failure-only fallback can be dispatched after verified success and skipped after a hard execution error. A success-only branch can be dispatched after an unverified or even a semantically **/failed** result if the return error is nil. The /failed case is not merely a hypothetical test-double return: the production verdict capture and observed-return adapter preserve this combination.

This is an execution-control defect, not simply inaccurate wording. The current code already distinguishes “ran” from “verified” in its final report, but the scheduler does not use that distinction.

**Recommended repair boundary:** preserve an explicit condition-bearing plan edge through decomposition, work admission, outcome settlement, and dispatch; let the Mangle executive derive which successor is eligible from the owned terminal outcome and existing authority. Do not substitute a prompt reminder or a second Go policy engine.

**Evidence strength:** the representation loss and dispatch gate are directly visible in both pinned source revisions. The witness and expected outcomes below are source-derived, not executed. There are no measured reliability, latency, or token improvements in this report.

## Revisions and limits

The branch heads were rechecked on 2026-10-28 at approximately 18:13 UTC and were unchanged:

| Surface | Pinned revision | Review result |
|---|---|---|
| main | [356f65e0e6bb639adaa90ab36212daa714114871](https://github.com/theRebelliousNerd/codenerd/tree/356f65e0e6bb639adaa90ab36212daa714114871) | Predicate loss and nil-error dependency gate present. |
| Active feature, feat/conversation-brief | [c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c](https://github.com/theRebelliousNerd/codenerd/tree/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c) | Same issue; feature was 27 commits ahead of main and zero behind at inspection. Work admission remains relevant. |
| Current research, research-main-20261028 | [a636eed04b7650ea4037c9494d86e7337d4df6b3](https://github.com/theRebelliousNerd/codenerd/tree/a636eed04b7650ea4037c9494d86e7337d4df6b3/Docs/audits) | Current audit additions checked for duplication. |
| Historical research, codenerd-research-docs | [c592ff223cd207fca4c29ae793d5eba57aa4781d](https://github.com/theRebelliousNerd/codenerd/tree/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits) | Historical audit corpus and adjacent topics checked. |

Source retrieval was read-only through the GitHub connector. No repository was cloned, modified, built, or tested. No application, model inference, live workflow, or user computer was used. This standalone report was saved outside the repository at the parent's explicit request.

This report concerns the TUI's small multi-step lane. It is not a finding about campaign DAG execution, the session executor's edit-step planner, or the post-shard CodeDOM continuation policy.

## Production trace

### 1. The corpus promises genuine branch semantics

The source defines distinct relations:

- sequential: predecessor must complete;
- conditional: successor runs only if predecessor succeeds;
- fallback: successor runs only if predecessor fails.

Those are not merely labels inferred by this review; they are the source's own contract. [StepRelation definitions](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_corpus.go#L30-L51)

The corpus contains explicit success, tests-pass, failure, and recovery patterns. In particular, fallback_if_fails records RelationFallback and selects fallback_chain. It recognizes clauses such as “if it fails,” “otherwise,” and “on failure.” The success patterns carry RelationConditional. [Success patterns](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_corpus.go#L358-L394), [failure patterns](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_corpus.go#L396-L432)

### 2. Decomposition erases the branch predicate

TaskStep contains Verb, Target, ShardType, Task, and DependsOn. It has no condition, relation, evidence requirement, or outcome discriminator. [Executable step type](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_multistep.go#L17-L24)

decomposeConditionalChain constructs two steps and marks the second with DependsOn: []int{0}. decomposeFallbackChain simply calls decomposeConditionalChain. A comment says their execution differs and is handled in executeMultiStepTask, but that distinction is not retained in the returned data. [Conditional and fallback decomposers](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_decomposer.go#L210-L258)

InferDependencies reinforces the loss: sequential, conditional, and fallback each produce the same preceding-step index. The comment on fallback again promises execution-time handling, without transporting anything that identifies the fallback. [Dependency inference](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_corpus.go#L707-L741)

### 3. The production route admits the plan without restoring that information

decideRoute calls the actual decomposition code, but publishes only multi_step_plan_step(index, shard) for each step. The kernel can see that a routable multi-step plan exists, not the condition joining its steps. [Route-plan publication](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L162-L186)

The policy's multi-step lane requires the relevant signal, at least two routed steps, a mutation intent, no direct-answer request, and no earlier lane. On the feature branch, work must also be admitted. Those are real restrictions and should not be ignored when reproducing the problem. They do not encode success/failure predicates for individual successors. [Plan readiness, lane selection, and work admission](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/routing_arbitration.mg#L222-L267)

The ordinary TUI processing path invokes executeMultiStepTask when RouteMultiStep is derived. This is not an unused helper or a speculative alternate entry point. [Production dispatch](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L443-L452)

### 4. The executor's only dependency gate is result-map membership

executeMultiStepTask creates stepResults as a map from step index to output text. A dependency is met if its index exists in that map.

After spawning a step:

1. A non-nil error prints failure and continues before adding the predecessor to the map.
2. A nil error stores the predecessor's output, regardless of ret.Outcome.
3. A separate continuationOutcomeFor check increments verifiedSteps only for completed outcomes. This affects reporting, not successor eligibility.

Thus the gate is “returned without a Go error,” not “succeeded,” “failed,” “tests passed,” or even simply “reached a terminal outcome.” [Dependency gate and nil-error insertion](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_dream.go#L692-L742)

The current report deliberately distinguishes ran and verified and prints the step verdict. That useful behavior does not repair the earlier dispatch decision. [Current verdict-aware reporting](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_dream.go#L744-L766)

The same defect is present in main. The corpus and decomposer were byte-identical between the compared revisions; nearby executor changes did not replace its dependency mechanism. [Main step type](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_multistep.go#L17-L24), [main executor](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_dream.go#L698-L774)

### 5. A semantic /failed return can genuinely have a nil Go error

This distinction changes the diagnosis and must be retained in implementation and evaluation.

captureTurnOutcome selects /failed for verdict.BuildFailed after the result.Error and hollowErr cases. That branch assigns the outcome but does not set result.Error. It separately selects /done or /unverified. [Verdict capture](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_memory.go#L180-L198)

The ordinary executor closure sets result.Error from hollowErr when needed; it does not turn every non-/done verdict into a Go error. At function exit it returns a non-nil error for a hollow-success error and otherwise returns result, nil. [Closing the turn](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1337-L1396)

observedReturn copies res.TurnOutcome into out.Outcome separately from the optional res.Error-derived out.Failure. [Observed-return projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/observed_return.go#L56-L66)

The inline JIT task executor returns a non-nil error only for ProcessWithIntent's error or result.Error; otherwise it returns observed, nil. It does not convert observed.Outcome == "/failed" into an error. [Task-executor return boundary](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L394-L409)

Accordingly, do **not** summarize the bug as “fallbacks never run on failure.” A semantically failed, nil-error predecessor is inserted in the map, so both failure-only and success-only successors are allowed by the same gate. The former may accidentally be correct for that particular outcome; the latter is not.

## Source-derived witness and counterexamples

A deliberately narrow witness is:

> fix parser.go, if it fails, test parser.go

Assume a normal admitted mutation intent, valid coder/tester verb routing, no earlier/direct-answer lane, and an available task executor. The compound-pattern signal recognizes the fix/test pair independently of fuzzy verb counting. The explicit failure-clause pattern decomposes two routed steps, and both the failure-only and success-only forms reduce to the same dependency. [Strong compound signal](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_multistep.go#L124-L146), [fallback clause](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_corpus.go#L396-L415)

The repeated explicit target is intentional. “test it” can be captured by a higher-priority implicit fix/verify or pronoun-oriented pattern; that would make a weak fixture for this exact issue. The corpus's implicit_fix_verify pattern has priority 88, above fallback_if_fails at 83. [Competing implicit pattern](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_corpus.go#L225-L245)

This witness has not been run. It specifies what a production-shaped evaluation should control, rather than claiming an observed interactive transcript.

| Predecessor return | Current dependency gate | Failure-only successor | Success-only successor |
|---|---|---|---|
| Verified /done; nil error | Enabled | Incorrectly enabled | Can be correct. |
| Settled /failed; nil error | Enabled | Can be correct for an authorized matching failure | Incorrectly enabled. |
| /unverified or other non-success; nil error | Enabled | Unsupported unless the requested predicate explicitly includes this state | Incorrectly treated as eligible for success-only work. |
| Non-nil execution error | Not enabled | Incorrectly skipped if this is the settled, authorized failure the condition names | Correctly withheld, but for the wrong general mechanism. |
| Cancellation or uncertain partial execution | Depends on error return, not condition | Must not be interpreted automatically as permission to recover, replay, or roll back | Must not count as success. |

Important controls and limits:

- A plain sequential nil-error path may look correct. A success-conditional path following /done may also look correct. Happy-path execution alone cannot distinguish the defective implementation.
- A nil-error /failed case is a required control: it refutes the overly broad claim that every failure suppresses fallback and shows why switching only on err is inadequate.
- A coder may run tests internally to satisfy its own contract. The issue concerns dispatch of the explicit second tester step, not whether any test command ran anywhere.
- Questions, unroutable/incomplete decompositions, held work, direct-answer requests, and earlier policy lanes need not enter this executor.
- Downstream authorization or tool gates can still block effects. The source proves wrong host-side successor eligibility, not that every dispatched child necessarily performs an unauthorized mutation.
- Original request text can remain available in context or reporting. A child might infer the condition and decline. That is not equivalent to preserving a deterministic execution predicate at the scheduling boundary.
- The source does not establish the frequency of affected production requests, observed damage, or the success rate of an eventual repair.

## Existing coverage and nonduplication

### Existing tests inspected

The inspected coverage includes:

- Compound mutation routing and route/kernel behavior in [routing_arbitration_roundtrip_test.go](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/routing_arbitration_roundtrip_test.go#L222-L236), including neighboring one-lane, absent-kernel, incomplete-plan, and cleanup checks.
- A multi-step end-to-end routing case in [task_routing_arbitration_e2e_test.go](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/task_routing_arbitration_e2e_test.go#L574-L592), which checks the route context rather than condition-to-child-dispatch correspondence.
- Feature-branch intent anchoring in [delegation_multistep_test.go](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_multistep_test.go#L10-L37).
- Leading-verb/whole-word parsing in [multistep_parse_verb_test.go](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/multistep_parse_verb_test.go).

No inspected test asserts that the selected fallback/success predicate survives through actual successor dispatch. This is a scoped coverage observation, not an exhaustive claim that the entire repository lacks any relevant test.

### Audit comparison

The review inspected main's relevant audits and implementation notes, 61 top-level historical audit documents within the inspected size range, and current research-only additions. Searches included the defining symbols decomposeFallbackChain, fallback_chain, InferDependencies, stepResults, DependsOn, and relevant conditional/fallback terminology; adjacent reports were read for semantic overlap rather than relying only on titles.

The nearest prior findings address different ownership boundaries:

| Existing audit | Existing scope | Why this finding is distinct |
|---|---|---|
| [planned-work-conservation](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/planned-work-conservation.md) | Session plan work conservation/overflow | This defect loses an activation predicate on the TUI's small multi-step edges; no plan truncation is required. |
| [delegated-turn-failure-settlement](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/delegated-turn-failure-settlement.md) | Settlement of partial delegated returns and current constraints | Even a fully settled, accurately typed return cannot repair an edge whose predicate was discarded before dispatch. |
| [background-outcome-and-continuation-ownership](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/background-outcome-and-continuation-ownership.md) | Background outcome and continuation ownership | This is an explicit user-requested successor condition in the foreground multi-step plan. |
| [perception-restriction-continuity](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/perception-restriction-continuity.md) | Restriction transport and enforcement | Here the parser already recognizes the branch relation; the executable type drops it. |
| [side-turn-ordering-and-cancellation-ownership](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/side-turn-ordering-and-cancellation-ownership.md) | Turn ordering and cancellation | This issue occurs with one ordinary turn and no concurrent cancellation. |

Current-branch additions checked included [brief-go-intent-handoff](https://github.com/theRebelliousNerd/codenerd/blob/a636eed04b7650ea4037c9494d86e7337d4df6b3/Docs/audits/brief-go-intent-handoff.md), [native-response-control-settlement](https://github.com/theRebelliousNerd/codenerd/blob/a636eed04b7650ea4037c9494d86e7337d4df6b3/Docs/audits/native-response-control-settlement.md), [perception-query-persona-continuity](https://github.com/theRebelliousNerd/codenerd/blob/a636eed04b7650ea4037c9494d86e7337d4df6b3/Docs/audits/perception-query-persona-continuity.md), [native-keyword-context-allocation-and-byte-budgets](https://github.com/theRebelliousNerd/codenerd/blob/a636eed04b7650ea4037c9494d86e7337d4df6b3/Docs/audits/native-keyword-context-allocation-and-byte-budgets.md), and [retrieval-delta-publication-fencing](https://github.com/theRebelliousNerd/codenerd/blob/a636eed04b7650ea4037c9494d86e7337d4df6b3/Docs/audits/retrieval-delta-publication-fencing.md). None records this condition-to-dispatch loss.

The existing [S1 verdict-from-evidence implementation note](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/impl/S1-verdict-from-evidence.md) and current executor already improve truthful status reporting. Preserve that progress. This proposal fixes the scheduling use of outcomes rather than reopening the report wording fix.

The novelty claim is bounded by the inspected source and audit corpus. It is not a claim about inaccessible discussions, every historical commit, or every document in the repository.

## Repair contract

### Preserve the user's control structure

Represent a condition-bearing edge, not only a predecessor index. The minimum durable identity should include the plan/revision, predecessor and successor step IDs, and an explicit activation condition. Preserve that structure through proposal/approval, route publication, execution, and reporting.

Useful distinctions include:

- after the predecessor has settled, where ordinary sequencing really means that;
- on verified success;
- on a specified settled failure;
- on a named evidence condition, such as this step's tests passing.

Do not treat these as interchangeable. “If tests pass” must not silently become “the child returned nil error” or “some unrelated build is green.” If the parser cannot retain the intended predicate or its scope, the plan must be held or rejected before effects rather than silently lowered to an unconditional dependency.

### Keep executive ownership in Mangle

Go should transport the plan, current admission/cancellation facts, and observed terminal result. Mangle should derive eligible successors from:

1. the approved current plan revision;
2. the predecessor's owned, settled terminal record;
3. the edge's activation predicate;
4. the current work admission and cancellation state;
5. existing action/tool authority.

The dispatch mechanism should consume that decision. A separate Go switch that re-decides semantic success/failure would create a second executive and perpetuate disagreement between evidence, policy, and dispatch.

The exact schema is an implementation decision; this report does not propose a patch. The invariant is that the condition must remain inspectable and enforceable end to end.

### Separate outcomes, errors, and dispositions

At minimum, preserve the difference between succeeded, failed, unverified/incomplete, cancelled, and unknown. A Go error is a transport/execution signal, not the complete semantic outcome vocabulary. Missing evidence is not evidence that an arbitrary opposite branch is true.

Report distinct successor dispositions such as:

- executed;
- condition not selected;
- blocked by authority or cancellation;
- failed;
- unresolved because predecessor settlement/evidence is insufficient.

Do not label all of these “dependencies not met,” and do not count an unselected branch as an execution failure.

### Failure is not blanket authority for recovery

A matching failure predicate is necessary but not sufficient for fallback execution. It does not grant permission to undo unrelated work, widen tool rights, replay completed effects, or interpret cancellation as an instruction to roll back.

Preserve partial-effect evidence and attempt ownership. Recovery must remain bounded by the original authorized task, current plan, applicable tool gates, and the actual failure classification. Late results from another attempt or an obsolete plan must not activate a successor.

### Conservative alternative

Until conditional plans have an enforceable representation, explicitly refuse or hold them before starting the first effectful step. Compare this fail-closed alternative with the full repair during evaluation.

Delegating the entire raw request to a child may avoid this particular decomposer, but it is not evidence that the host now enforces the branch. A prompt-only reminder is likewise not an equivalent repair.

## Design references

These are bounded comparisons, not a proposal to adopt another workflow engine.

- The W3C SCXML recommendation treats a transition's condition separately from its target and defines transition selection accordingly. This supports the modest design principle that retaining a successor alone does not retain the predicate that enables it. [SCXML transition selection](https://www.w3.org/TR/2015/REC-scxml-20150901/#SelectingTransitions)
- The Amazon States Language specification models error fallback through a matching error condition and an explicit successor; it does not define fallback as ordinary success-only continuation. This is a useful reference for keeping settled error classification and next-state selection separate. [States Language fallback states](https://states-language.net/#fallback-states)

Neither reference establishes codeNERD-specific correctness, performance, or a universal interpretation of ambiguous natural-language conditions.

## Proposed evaluation — not run

### Harness and oracle

Exercise the normal TUI route, decomposition, work admission, and multi-step executor with independently captured child dispatches. Keep CodeDOM/world/evidence features enabled for the relevant integration cases. Do not replace the entire route with hand-constructed steps and then claim end-to-end coverage.

Use deterministic owned predecessor outcomes to isolate dispatch correctness, followed by production-shaped cases that produce those outcomes through the real observed-return boundary. In particular, include the real verdict.BuildFailed → /failed → observed return with nil error path, not only a fabricated mock failure.

The oracle should inspect the approved condition, terminal evidence, child invocation identity/count, and actual effects/receipts. Final prose is not a sufficient oracle. Parser-only tests and tests of the final ran/verified counts are useful but insufficient.

### Acceptance matrix

| Case | Required observation |
|---|---|
| Failure-only edge after verified /done with nil error | Fallback child is not dispatched; disposition says condition not selected. |
| Failure-only edge after settled matching /failed with nil error | Fallback child is eligible exactly once, subject to unchanged authority. This is an explicit positive control. |
| Success-only edge after the same /failed with nil error | Success child is not dispatched. This is the paired negative control that exposes conflation of semantic outcome and Go error. |
| Failure-only edge after a matching settled hard execution error | Authorized fallback can run once after settlement; the error must not merely remove the dependency record. |
| Success-only edge after verified /done | Success child runs once; healthy behavior is preserved. |
| Success-only edge after nil-error /unverified or stalled/incomplete result | No success dispatch; unresolved or blocked disposition remains truthful. |
| Tests-pass edge with absent, stale, unrelated, or wrong-attempt test evidence | No dispatch on that evidence; only the named current predicate can satisfy the edge. |
| Cancellation before or after partial execution | No automatic recovery grant; cancellation and partial effects are retained. |
| Partial mutation followed by failure | Recorded effects are preserved; no replay of already completed operations solely to discover the branch. |
| Unsupported or conflicting conditional parse | Hold/refuse before the first effect, not silently execute an unconditional plan. Include competing higher-priority patterns. |
| Ordinary sequential and independent plans | Existing intended behavior is preserved under the newly explicit semantics. |
| Held work later approved | Approval applies to the same plan revision and condition; stale or changed plans require appropriate renewed admission. |
| Duplicate/late result from another attempt or obsolete plan | Cannot enable a second or wrong successor. |
| Downstream gate denial | Remains denied; branch eligibility never bypasses authorization or creates false completion. |

Do not count refusal, unresolved execution, timeout, or cancelled requests as successful repair merely because no wrong fallback ran. Score correctness and useful completion separately.

### Fair comparison

Compare three treatments:

1. the existing implementation;
2. explicit pre-effect refusal/hold for unsupported conditional plans;
3. typed condition preservation with Mangle-owned successor eligibility.

Match repository revision apart from the proposed change, model/provider, context, tool authority, feature flags, work mode, and test workload. Include success, semantic failure with nil error, hard failure, unverified, cancellation, and partial-effect cohorts. Report the full request denominator and the fraction rejected/held; do not restrict the comparison to plans that reached a convenient terminal state.

Primary outcomes:

- wrong-branch dispatch rate;
- missed authorized fallback rate;
- false success-branch admission;
- duplicate successor execution;
- truthful branch/terminal dispositions;
- verified final task outcomes.

Separately report model calls and input/output tokens, tool calls, retries, human recovery steps, kernel/host time, retained state bytes, and end-to-end latency. Include costs incurred before a refusal or failure. Report distributions and cohort sizes rather than a single blended success rate.

### Costs and trade-offs

The proposed fix adds schema and lifecycle work: typed plan edges, outcome ownership, policy rules, state cleanup, proposal/version continuity, and broader integration coverage. Retained state is at least proportional to plan steps/edges and owned attempt records. Query and publication overhead must be measured; no numeric overhead estimate is justified by source inspection alone.

Avoiding an incorrectly dispatched fallback may save a child run. Correctly running a fallback that the current code skips may increase model/tool cost while improving correctness. Holding ambiguous plans may reduce wrong effects while increasing user interaction and lowering autonomous completion. These must be reported rather than hidden as efficiency gains.

The main expected benefit is **correct execution control**, not a promised token reduction. No benchmark, live validation, or savings claim has been made.

## Exit criteria

The repair is complete only when:

1. success/failure/evidence predicates remain present from recognized request through approved plan and dispatched successor;
2. the executive, not nil-error map membership, determines successor eligibility;
3. semantic /failed with nil error, hard errors, unverified states, and cancellation remain distinguishable;
4. branch dispatch and final reporting agree without discarding partial effects or bypassing authority;
5. the production-shaped acceptance matrix passes with independent dispatch/effect evidence;
6. costs and completion trade-offs are reported over the same full workload.

Until then, the accurate claim is that the current TUI can recognize conditional syntax but does not preserve its meaning at the multi-step scheduling boundary.

