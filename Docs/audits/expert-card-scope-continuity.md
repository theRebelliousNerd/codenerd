---
doc-class: deep-dive
subsystem: expert-card-dealing
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Expert card scope continuity

## Conclusion

The first tranche makes a useful distinction: consultations are typed outcomes, leads can guide every planned step, advisers are restricted to their selected files, and omitted or failed cards are named instead of disappearing without explanation. This is a stronger starting point than attaching every consultation to every fresh prompt.

One narrow continuity gap remains in the inspected source: an already-consulted adviser can be selected for a later planned file but supply neither its existing consultation nor a new one. Consultation deduplication is agent-wide; adviser-card applicability remains tied to the initial file set.

Scope: static source inspection at public main commit `486db57ce490302c65d7f484e7f6070ee0e70862`. This is a source-derived scenario, not an observed failing run or measured quality regression. No repository edits, tests, model inference, or CodeNERD execution were performed.

## Source trace

1. Initial selection uses the intent target, then consultation is attached before execution: [executor.go 1483–1487](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor.go#L1483-L1487), [1564–1568](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor.go#L1564-L1568).
2. Each card copies the consultation's original `need.Files`; only those files are published as `turn_expert_card_file`: [expert_cards.go 39–43](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_cards.go#L39-L43), [65–80](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_cards.go#L65-L80).
3. A later step selects experts for its own target, sets JIT expert inputs, and records critique obligations. After a successful fresh compile it deals existing cards, then invokes consultation: [work_steps.go 819–843](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/work_steps.go#L819-L843).
4. Mangle deals an answered lead to every step, but an answered adviser only when the step file exactly joins the recorded card-file relation: [shard_experts.mg 225–236](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/shard_experts.mg#L225-L236).
5. The consultation helper filters solely on `consultedExperts[need.Agent]`. If that agent was already consulted, it does not extend the stored scope or buy another answer: [expert_spawn.go 68–76](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_spawn.go#L68-L76), [105–114](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_spawn.go#L105-L114).

## Concrete scenario and prerequisites

Assume a registered custom BubbleTeaArchitect with the Bubble Tea domain and no executor classification. Two distinct scanned files, A.go and B.go, both import Bubble Tea. Import evidence has confidence 90; a matching specialist without executor classification advises rather than leads. Those rules are in [shard_experts.mg](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/shard_experts.mg) and [shards.mg](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/shards.mg).

A /fix initially targets A.go and receives a successful adviser answer with card scope {A.go}. Its plan subsequently includes B.go, outside that initial selected scope. When B.go's prompt compiles successfully, the expert is selected for B.go but the A.go-only card is withheld. The subsequent consultation call skips the same agent because it has already been consulted in this result. B.go receives a notice identifying the withheld card, not its answer or a new B.go consultation.

The initial target could instead be a package directory: the gap requires B.go to fall outside the first selection's actual covered files. It is not enough that B.go is merely later in the plan.

## What this finding does not establish

- It does not apply to every expert. Built-in BubbleTeaExpert is executor-classified and normally produces a lead card for strong import evidence; that card reaches every step.
- It does not mean B.go loses all expert knowledge. The per-step selection still fills `stepCtx.Experts`; JIT knowledge remains a separate channel.
- A prior step's prose might incidentally repeat useful advice, but this is not systematic consultation continuity. Each tool pass seeds history from prior turns, its user input, and its own initial response, rather than importing the preceding step's system prompt: [executor_tools.go 180–194](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor_tools.go#L180-L194).
- It does not remove the later critique channel. [recordExpertCritics](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_critique_round.go#L39-L58) unions later selected files into the review's domain files. For critique-eligible verbs such as /fix, the reviewer can still examine B.go after edits.
- It is not silent: the step names the consultation withheld as not applying to its file.
- It is not a cross-request cache issue: a new process request creates a new [ExecutionResult](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor.go#L1296).
- It assumes a successful fresh step compile. A failed compile returns the original turn prompt, preserving its original consultation prose.
- It does not prove that more consultation improves outcomes. A missing pre-edit consultation may be immaterial, covered by JIT knowledge, caught in critique, or harmful only for particular tasks.

## Existing coverage

The [card test](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_cards_test.go) checks a handcrafted lead and adviser against the initially recorded files. The [continuity test](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/step_consultation_continuity_test.go) checks reuse for the same target and consultation on a fresh result, using the lead-classified BubbleTeaExpert fixture. These are useful regressions, but neither exercises the same adviser selected for a new file within one result.

## Design choices

The invariant to clarify is whether a new same-domain file should inherit earlier advice, trigger a scoped supplement, or explicitly proceed without consultation.

**Scope-aware reuse.** Extend the set of files eligible to receive an existing answer when a later selection matches its agent. This is cheap, but matching the same domain does not prove the answer covers the new file. Do not relabel A.go-specific advice as B.go-specific analysis; preserve original provenance and distinguish reusable domain guidance from file-specific claims.

**Supplement on uncovered scope.** Deduplicate by covered consultation scope rather than agent identity alone. Request a focused supplement for newly selected files and merge it into the turn's card set. This preserves specificity but increases latency and tokens, and requires explicit rules for duplicate facts, provenance, unsuccessful answers, scope changes, and recursion protection. An agent-keyed dealt map is also too coarse if multiple cards for one agent acquire different scopes.

**Plan-wide consultation.** Once the plan exists, select experts over the union of planned files and consult with that scope. This can reduce serial supplements, but it moves consultation relative to planning and may pay for steps beyond the execution bound or files that later cease to matter. Newly discovered files still need a policy.

**Deliberate bounded degradation.** Keep one consultation per agent but expose a typed uncovered-scope condition, with policy deciding whether JIT plus later critique is sufficient. This makes the tradeoff explicit rather than implying the current selection was satisfied by a card the step cannot see.

None requires making advisers unconditional leads or weakening file-sensitive dealing.

## Fair validation and cost accounting

No evaluation was run. If implementation is authorized later, start with deterministic assertions using a successful fresh JIT compile and an adviser registered through the real selection rules. Cover same agent/new file, initial scope containing both files, two different advisers, a lead control, first consultation on a step rather than the turn, failed or empty consultation, canonical path aliases, and a new request. Assert both positive delivery and negative leakage: unrelated file-specific advice must stay out.

Then compare the current design with a chosen alternative on matched end-to-end tasks, using the same base commit, model settings, prompt and tool budgets, seeds where supported, and evaluator criteria. Include cases where early advice is necessary, redundant with JIT, or misleading when reused outside its original scope.

Count the entire lifecycle: initial and supplemental consultations, prompt expansion, executing-model calls, critique, repairs, verification, retries, failure rates, wall-clock latency, and final verified task success. Report cost per completed verified task alongside raw token and latency totals. Moving work from critique to pre-edit advice can be a genuine win or merely an accounting shift; a lower consultation count alone does not decide it. Keep prompt-delivery correctness separate from downstream task-quality claims.

