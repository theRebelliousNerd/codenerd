---
doc-class: research-proposal
subsystem: chat-experts-session
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# A knowledge-only expert need must not become a chat consultation

## Finding and decision

The kernel deliberately distinguishes **an expert whose stored knowledge is needed** from **an expert whose own consultation or critique is required**. Current slash-command orchestration loses that distinction. `specialistsFromSelection` converts every `Selection.Needs` entry into a `SpecialistMatch`, preserving `Execute` but not `Advise`, `Spawn`, or `Critique`. When all selected experts are knowledge-only, the list is still nonempty and none executes directly; `/fix` and `/refactor` therefore enter the advisory-with-critique topology and invoke those experts anyway. [Typed selection][need], [projection][projection], [chat branch][branch].

A concrete supported case is a valid Go target with a registered, usable Go expert and only language evidence. Policy gives that evidence confidence 40; the threshold for advice is strictly greater than 40, and execution requires greater than 80. Consequently the kernel derives a knowledge need but no advice, spawn, or critique role. The chat consumer nevertheless asks the expert for advice, and asks it for critique if generic execution returns without error. These are additional task dispatches, not a measured fixed number of model calls: each task can make several calls, fail before inference, or be canceled. [Language policy][language], [role policy][roles], [critique rule][critique-rule], [advisory execution][mode].

**Decision:** preserve the full role-bearing selection through chat delegation. Keep all knowledge needs available to JIT, but dispatch only the consultations and critiques the kernel actually derived. Topology orders eligible work; it must not silently promote knowledge-only needs into executable phases. Prefer the existing shared executor consultation machinery where it can own the episode without duplicate chat-side calls.

This is not a tool-permission escalation, guaranteed bad advice, or proof that every slash command makes unnecessary calls. It is a source-supported policy-to-host projection mismatch, with conditional latency, cost, and context-quality consequences. More advice can sometimes help; fewer calls are not automatically better task quality.

## Scope and prior work

Source pin: public main `388205138ba6d47c1cff0c5e97c896dfa642a391`, checked 2026-10-30 around 15:38 UTC and revalidated unchanged at 15:44 UTC. Review covered root, chat, perception, articulation and session guidance, current main audits, active branches, and research additions through `research-main-20261030-1517` (`eb50ab1d51fc131a4d9af49f76f5aea62c855832`). An independent source challenge corroborated the role projection and bounded dispatch count. This audit ran no application, tests, inference, benchmarks, or failure injection and changed no repository files. Witnesses and evaluations are proposed, not observed incidents.

The root contract makes Mangle the executive and Go the measurement/transport layer. The expert policy itself explicitly says language evidence pushes knowledge without spawning another expert. Session guidance preserves separate capability and exact permission checks. This proposal does not alter any of those controls. [Root guidance][root], [policy's intended split][language], [session guidance][session-guide].

Related work already owns important parts of this design:

- **Slash-command specialist target continuity** identified the path-plus-constraint parsing handoff. Current handlers pass a structured `slashTarget`; the resolver uses its target, and generic fallback preserves target, constraint, and request. This finding assumes that repair works: the file resolves and the correct expert is selected. The old proposal also requires policy to remain authoritative; the new contribution is a concrete negative-role witness at a later consumer. [Prior target audit][target-audit], [fixed handlers][handlers], [fallback][fallback].
- **Planned-step expert consultation continuity** already cautions against turning language-only expert names into mandatory model calls while preserving newly derived step roles. That principle is not new. The present route is chat's pre-execution adapter and its phase loops, not the planned-step prompt replacement. [Step audit][step-audit].
- **Expert lending dependency contract** uses language-only evidence to establish why JIT knowledge can matter without consultation. Here the defect is the inverse: a knowledge-only need gets promoted to a call by a separate host path. Do not fix this by dropping its stored knowledge from compilation. [Lending audit][lending-audit].
- **Advisory repair continuation** concerns a coding model's read-first response after an actionable finding. This finding precedes that question: whether this expert should have been dispatched at all. It does not replace that audit's continuation contract. [Repair audit][repair-audit].

The cartographer branch `767a3291d6287b238417f2466592a9002810d05a` has the same `expert_selection.go` projection as main. The other active tips were revalidated at `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908`; they do not contain that source path. This is not an assertion that older branches have identical expert routing.

## Producer-to-consumer trace

### 1. The kernel supplies enough information

`experts.Need` carries `Agent`, technologies, files, confidence, `Execute`, `Advise`, `Spawn`, and `Critique`. `Select` queries the corresponding relations and attaches their answers to the same need. No new classifier or inferred score is necessary to distinguish knowledge-only admission. [Need contract][need], [selector reads][select-reads].

For a `.go` target, `extension_language(".go", /go)` and `task_technology(..., /language)` provide the language evidence. A matching registered agent domain yields `expert_needed`; language confidence is 40. The execution rule needs an executor classification and confidence above 80. The advice rule needs confidence above 40 and absence of execution. `expert_spawn` derives only from those role decisions; `expert_critique` in turn requires a spawn and advisory-with-critique topology. With only the language evidence, all those call-bearing roles remain absent. [Extension table][extensions], [technology and need rules][technology], [confidence][language], [roles][roles], [spawn and critique][spawn-rules].

This distinction is implemented and tested as text in the repository. `TestSelectReadsTheKernelsDecisionAndRetracts` explicitly checks that the language-only Mangle need has confidence 40, neither execution nor advice, and an empty spawn field. The test is relevant evidence of the intended contract, not an executed result from this audit. [Selector test][select-test].

### 2. Slash dispatch retains the need but loses the negative role

The `/fix` handler parses target and constraint, formats the task, and calls `spawnShardWithSpecialists` with the structured record. The helper resolves files, asks `selectShardExperts`, and converts the result with `specialistsFromSelection`. The conversion iterates **all** needs and stores `ShouldExecute: need.Execute`; it carries no advice, spawn, or critique decision. [Handler][handlers], [selection adapter][projection], [dispatch][branch].

The next branch treats only `len(specialists)==0` as the plain-execution case. A knowledge-only need makes that test false. Since its `ShouldExecute` is false, the expert-led early return is also skipped. The helper then asks the kernel for the verb's topology. Both `/fix` and `/refactor` derive `/advisory_with_critique`, so it passes the unfiltered specialist list to `executeAdvisoryWithCritiqueMode`. The topology is genuinely policy-derived; the defect is using it as a substitute for each specialist's separate eligibility. [Dispatch][branch], [topology table][topology].

### 3. The phase loops dispatch every projected specialist

Phase one passes the entire list to `gatherSpecialistAdvice`. That helper loops over every entry and calls `m.spawnTask(ctx, s.AgentName, adviceTask)` without consulting its score or a derived role. Advice errors are displayed; successful advice is appended to the generic coding task. [Advice loop][advice], [execution mode][mode].

The generic task then runs normally. If it returns an error, phase three is explicitly skipped. If it returns nil error, `gatherSpecialistCritique` receives the same list and invokes every entry again. It does not check `Need.Critique`, which was discarded before this phase. Critique is displayed in the final report; this audit does not claim those opinions automatically become mandatory repairs or override the generic executor's verdict. [Critique dispatch][mode], [critique helper][critique].

`spawnTask` is a real execution doorway, not a display-only action. For ordinary agent profiles it constructs a task request and calls the task executor. The JIT executor normalizes the intent, applies its ordinary execution route, and uses the string API's verification contract. These downstream controls can refuse or fail work; they do not reconstruct the parent selection's omitted no-spawn decision. [Task bridge][bridge], [executor contract][executor].

### 4. The shared executor already handles the negative case correctly

`withExpertConsultations` skips every need whose `Spawn` is empty and returns the existing prompt when none remain. For admitted needs it uses the routing ladder, which can use already delivered knowledge, consult, escalate, or retain an unresolved disposition. The session compilation context separately receives `sel.Agents()` for knowledge lending. This is an existing separation between knowledge admission and consultation dispatch, not a suggested simplification that throws knowledge away. [Shared consultation consumer][session-consumer], [compilation context][context].

The current chat policy test recognizes the distinction in a comment, but its assertions only check that the language-only entry remains in the converted list and has `ShouldExecute=false`. Its mixed fixture includes a strong Bubble Tea executor; it does not drive a language-only selection through the advisory phase loops. Preserving that fixture is useful, but it does not establish that no advice was bought. [Chat test][chat-test].

## Bounded witness and controls

Use an ordinary existing in-workspace Go file such as `mathutil.go`, a healthy registered Go expert with its `/go` domain, and normal world/CodeDOM context. Ensure the file has no matching import evidence and no other selected high-confidence executor. Submit `/fix mathutil.go handle the empty input without changing the exported API` through the actual idle TUI command entry.

The expected source-derived boundary sequence is:

| Boundary | State supported by source |
|---|---|
| Parsed command | Target is `mathutil.go`; constraint remains in the task. |
| Expert selection | Go expert needed at 40; `Execute=false`, `Advise=false`, `Spawn=""`, `Critique=false`. |
| Chat conversion | One specialist remains, with `ShouldExecute=false`. |
| Phase one | One Go-expert advice task is dispatched, subject to downstream admission/cancellation. |
| Main execution | Generic task receives its original task plus any returned advice. |
| Phase three | If main execution returns nil error, one Go-expert critique task is dispatched. |

This predicts one additional advice dispatch and one conditional critique dispatch for that expert relative to the kernel's no-spawn/no-critique decision. It does not predict exactly two provider requests or a specific monetary cost.

Important controls and exclusions:

- No registered matching expert, a failed selection, or no resolved target takes a different route. No automatic expert availability is assumed.
- A strong executor in the selected set takes the earlier expert-led branch. The primary witness excludes it; a mixed test must not be misreported as the language-only path.
- Import evidence, custom policy thresholds, or an explicit request for a live expert can legitimately change the derived roles. Observe the actual policy answers rather than treating every Go file as knowledge-only.
- Generic execution errors skip critique; missing task executors, provider failures, cancellation, and input-window admission can prevent inference. Do not charge an unstarted model call as executed work.
- Advice may improve the outcome. It remains subject to the real tool catalog, exact permission checks, workspace confinement, task verification, and request lifecycle. The finding does not establish unauthorized writes or weakened safety gates.
- Main-task wording survives in the formatted execution task. Any separate issue about the advisory task's omission of the parent's specific goal is outside this report's primary claim.

## Repair contract

1. **Retain one full selection.** Carry the kernel's role-bearing need or a typed immutable projection containing all relevant decisions and parent identity. Do not encode “not executor” as “therefore advisor.” Keep knowledge needs distinct from active consultation and critique sets.
2. **Reuse the existing owner.** Prefer passing the full structured task to the shared executor so its JIT-first consultation routing can own the calls once. If chat retains its phase presentation, its execution adapter must consume the same policy roles and preserve outcome identity; avoid running both chat and executor consultations for the same obligation.
3. **Preserve knowledge-only delivery.** When no live expert role is derived, use normal generic execution with the original `Target`, `Constraint`, and `Request`. The existing `spawnSimpleShard` already supplies those fields; the executor can reselect knowledge for compilation. Do not filter all knowledge-only needs out of the system merely to make a dispatch counter fall.
4. **Gate critique independently.** Only a derived critique obligation enters that phase, with the existing execution-error skip and source/scope relevance controls. No-spawn is not a fake successful consultation; it is an intentional role disposition.
5. **Keep decisions in policy.** Do not add another Go confidence threshold of 40 or 80. The host should transport and obey derived role facts so custom policy and future changes remain consistent.
6. **Make observation inexpensive and precise.** Record the selection identity, knowledge need, role disposition, actual dispatch identity and outcome. Do not expose source paths or user content to external telemetry by default. A status claiming “with specialist matching” need not imply a live model call occurred.

## Costs and fair evaluation

The current route can add serialized advisory latency before generic execution and critique latency afterward, although each phase runs multiple experts concurrently. It can also enlarge the main task with extra advice and add expert task tool calls, token use, logs, and failure handling. Actual cost depends on agent configuration and continuation behavior. Removing these calls may reduce irrelevant advice or may remove a useful accidental intervention; source inspection alone cannot decide the quality tradeoff.

A role-preserving adapter adds small metadata and bookkeeping costs. Consolidation with the shared executor has migration and presentation costs and can reveal duplicate-consultation assumptions. Rechecking target identity and preserving JIT knowledge may still incur normal selection and retrieval work. Do not remove verification, useful expert lending, or failure disclosure to achieve a lower call count.

Proposed evaluations, not run:

- **Kernel and adapter:** assert the complete language-only role tuple, then pass it through the actual chat adapter and phase decision. A helper test checking only `ShouldExecute` is insufficient.
- **Normal TUI entry:** use a real file, registered expert, ordinary scanner/CodeDOM context and the actual `/fix` handler. Capture task dispatches at the executor boundary. For the primary fixture, expect generic execution without live advice/critique, while retaining the expert in the knowledge-selection path.
- **Positive role controls:** use framework-import evidence yielding a lead, an advisor yielding advice, and a real critique obligation. Verify those calls still occur under the proper owner and do not duplicate between chat and executor. Include an expert whose name is unknown to the static classification but whose policy-derived advisory role is valid.
- **Topology and failure controls:** test `/refactor`, an advisory-only verb where reachable, a parallel verb where reachable, empty/no-kernel selection, failed advice, main execution error, canceled work and unavailable providers. Assert topology never substitutes for participant eligibility.
- **Knowledge preservation:** inspect actual compiled inclusion under sufficient budget, not just selected names. Also test policy/budget rejection as a valid outcome. A zero-call path that drops relevant stored knowledge is not an acceptable repair.
- **Whole-task comparison:** hold workspace revision, model configuration, expert material, policy and budgets fixed. Compare verified completion, constraint compliance, useful knowledge delivery, dispatch/provider-call counts, tokens, latency, failure recovery and human rework. Include cases where the extra old-path advice was useful, and report quality uncertainty rather than declaring every avoided call waste.

Acceptance means a language-only need remains available as knowledge without being promoted to a live role, while a policy-derived consultation still executes exactly through its intended owner. It does not mean minimizing expert use irrespective of task quality.

[root]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md
[session-guide]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/agents.md
[need]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/experts/select.go#L48-L84
[select-reads]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/experts/select.go#L209-L265
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/expert_selection.go#L13-L53
[handlers]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_evolution.go#L11-L68
[branch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L865-L928
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L931-L941
[topology]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L715-L731
[extensions]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/coder_language.mg#L30-L48
[technology]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shard_experts.mg#L29-L109
[language]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shard_experts.mg#L111-L141
[roles]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shards.mg#L174-L197
[spawn-rules]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shard_experts.mg#L143-L223
[critique-rule]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shard_experts.mg#L212-L223
[mode]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_modes.go#L137-L230
[advice]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_modes.go#L331-L370
[critique]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_modes.go#L372-L421
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L35-L80
[executor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L315-L449
[session-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/expert_spawn.go#L52-L114
[context]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/expert_context.go#L13-L48
[select-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/experts/select_test.go#L32-L76
[chat-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/specialist_policy_test.go#L30-L63
[target-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/slash-command-specialist-target-continuity.md
[step-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/planned-step-expert-consultation-continuity.md
[lending-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/expert-lending-dependency-contract.md
[repair-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/advisory-repair-continuation.md
