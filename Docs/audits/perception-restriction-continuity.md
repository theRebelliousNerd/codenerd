---
doc-class: deep-dive
subsystem: perception
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
supersedes: []
---

# Perception restriction continuity from understanding to action admission

codeNERD can recognize that a request is hypothetical, asks for confirmation, or prohibits changes. The remaining capability is to make those recognized limits survive the conversion to executable intent, delegation, tool selection, and final action admission. At the inspected revision, important restriction fields remain inside `Understanding` or the full routing object, while the normal execution path receives a smaller `Intent`. Ordinary constraint prose often survives; the typed obligation does not have an equivalent end-to-end contract.

**Recommendation:** specify a request-owned restriction envelope, preserve it through every execution handoff, and make Mangle derive its consequences at both catalog selection and final action admission. Reuse the existing request-bound approval system where consent is required. Do not replace that system, add a second permission engine in Go, or treat a model-produced signal as human approval.

**Source freeze:** `theRebelliousNerd/codenerd` main at [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), committed 2026-10-28 06:24:06 UTC. Inspection and a repeated main-head check occurred on 2026-10-28; the checked head remained unchanged. All implementation links below use this commit. This is source-based research and a proposed design, not an observed incident report. No code implementation, repository edits, test execution, live application probes, or performance measurements were performed. The proposed verification steps are not passed results.

## Scope and relation to existing work

The existing [perception capability recovery audit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/perception-capability-recovery.md) addresses semantic-memory ownership, turn-scoped semantic evidence, and decision receipts. Its as-built section reports that legacy wrappers were removed or moved to test seams while the broader ownership work remains deferred. This document does not rediscover those proposals or recommend reconnecting wrappers.

The [learned correction integrity and lifecycle draft](https://github.com/theRebelliousNerd/codenerd/blob/4f69c316ea742b0662c700091ee5a2f472f36598/Docs/audits/learned-correction-integrity-lifecycle.md), inspected on `codenerd-research-docs`, concerns a different qualifier boundary: SQL-backed learned examples drop a stored constraint during recall, the learned-write adapter replaces the operation context, and the consolidation worker outlives dependencies during shutdown. The subject here is the **current request after it has already been understood**, not a recalled example or background-learning job.

The [request-bound human approval audit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/request-bound-human-approval.md) now contains an as-built implementation. Its broker, immutable request, operation preview, exact grant, and admission/resume checks are existing capabilities to preserve. The present finding is not that codeNERD has no approval UI or no safety gate. It is that a current user's recognized confirmation requirement is not transported into those capabilities for ordinary otherwise-permitted actions.

This work also differs from response admission and conversational-history fidelity. A later history record losing a constraint can affect future turns, but repairing that record would not establish the restriction at the current action gate. That boundary deserves its own verification.

The repository's [architecture contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md) assigns executive decisions to Mangle, requires JIT-first model-facing behavior and spec-first changes, and requires validation through normal production paths with world-model and CodeDOM context. Those requirements shape this proposal.

## The live path and the information it retains

The interactive chat path prefers `PerceptionFirewallShard.Perceive`, with direct transducer parsing as its fallback. The firewall calls `ParseIntentWithGCD`, and the canonical understanding adapter obtains an `Understanding` from `LLMTransducer`, derives routing, and converts it to `Intent`. The normal executor independently calls the same transducer interface when it is not supplied a preset intent. Delegated tasks commonly supply a preset and deliberately skip classification of the synthetic task text.

Evidence: [chat ingress](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L166-L184), [firewall call](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/perception.go#L476-L506), [understanding and conversion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L418-L464), [executor observe](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1376-L1388), and [preset execution](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L384-L392).

| Recognized information | What crosses the canonical conversion | What the inspected source establishes |
|---|---|---|
| User constraint text | `UserConstraints` is joined into `Intent.Constraint` | Prose is preserved for several downstream consumers; not an enforcement proof |
| Question signal | `Intent.IsQuestion` | Interactive arbitration explicitly publishes it as an intent signal |
| Scope and suggested tools | `ScopeLevel`, `ScopeFile`, `ToolsNeeded` | Interactive arbitration uses them to require workspace evidence |
| Confirmation requirement | No corresponding `Intent` field or routing fact | The recognized boolean does not establish a request-bound consent obligation |
| Hypothetical signal | No corresponding `Intent` field | The kernel-backed helper computes a derived mode, but that is not the interactive dream-lane condition |
| Derived blocked tools | Not carried in `Intent` or the asserted routing facts | Tests can inspect the list, while the execution boundary has no equivalent input from it |

The [conversion itself](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L467-L525) is the positive evidence for this projection. The [interactive signal bridge](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_routing.go#L158-L187) shows that question, file-scope, suggested-tool, clarification-history, and surface-response signals are carried explicitly. There is no corresponding hypothetical or confirmation signal in that bridge.

## Recognized restrictions are not an executable obligation

`Understanding` defines `RequiresConfirmation` as the user's desire for approval before action. It also has `IsHypothetical`, and `Routing` describes `BlockedTools` as tools that cannot be used because of constraints. `deriveBlockedTools` queries the constraint table and adds four write-related tool names when `Understanding.IsReadOnly()` returns true. These are real computations, not absent features. [Understanding types](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding.go#L73-L150), [blocked-tool computation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L883-L900).

The computation ends in an object the normal `Intent` conversion does not carry. `assertRoutingFacts` publishes `current_understanding`, suggested and derived mode, primary shard, context priorities, and tool priorities. It does not publish user restriction atoms, blocked tools, or a confirmation obligation. Repository searches at the source pin found `BlockedTools` consumed by tests rather than execution code, `RequiresConfirmation` used by its type/helper, logging and tests, and `GetLastUnderstanding` used for debug/test inspection rather than as a production policy bridge. [Published facts](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L671-L725).

The routing schema already describes `no_changes`, `dry_run`, and `confirm_first`, including `constraint_type`, `constraint_forces_mode`, and `constraint_blocks_tool`. Searches found no routing or admission consumer of `constraint_type` or `constraint_forces_mode`; their registration is not such a consumer, and the tool-blocking table feeds the in-memory helper above. The presence of these tables therefore does not establish that the final gate sees a turn restriction. [Restriction tables](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/schema/intent_routing.mg#L403-L424).

There is a second, narrower representation issue to specify before using these tables as authority. `UserConstraints` is free text, while `IsReadOnly` recognizes exact strings such as `no_changes`, `read_only`, and `dry_run`, and table queries also expect compatible values. Both the embedded prompt and the JIT understanding atom ask for explicit constraints and give natural-language examples; they do not turn arbitrary prose into a complete formal policy language. Preserving a field and interpreting every possible value are different obligations. [Read-only helper](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding.go#L205-L221), [embedded constraint prompt](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L824-L865), [JIT constraint prompt](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms/perception/understanding.yaml#L95-L128).

### What this does not establish

It does not establish that every user instruction is lost or that every restricted request causes an unauthorized effect. The constraint string survives in `Intent`, delegation and prompt construction. The model may honor it, a persona's allowed tools may already exclude the effect, or another guard may refuse the operation. The missing claim is narrower and architectural: **a correctly recognized restriction does not by itself have a typed, policy-enforced route to the actual action boundary**.

## Hypothetical recognition does not select the interactive dream lane

The source provides a particularly clear example of the distinction between recognizing a limit and enforcing it. `deriveMode` returns `dream` when `Signals.IsHypothetical` is true. In contrast, `understandingToIntent` chooses its verb from action and domain, and its category from semantic type and action. `modify` maps to `/fix`; a hypothetical semantic type with a modifying action is classified as `/mutation`. The interactive dream rule requires a `/dream` **verb**, not `derived_mode(/dream)` or the hypothetical signal.

Evidence: [hypothetical mode](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L742-L771), [verb and category conversion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L575-L665), and [interactive dream rule](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/routing_arbitration.mg#L157-L161).

The repository's conclusion baseline marks `derived_mode` as unconsumed. Production-source searches at this pin found its producers, retraction and ownership declarations, but no executive consumer that converts it into the interactive dream lane. The actual lane rule above is the decisive evidence for that routing claim; the baseline is supporting static evidence, not a live execution result. [Conclusion baseline](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/testdata/unconsumed_conclusions.txt#L140).

A source-derived example is an understanding of “What would happen if we modified retryLoop?” containing `action_type=modify`, `semantic_type=hypothetical`, and `is_hypothetical=true`. With a routing kernel attached, the helper computes a dream mode and read-only blocked tools, but the compatibility projection can still be mutation `/fix`. That does not prove a particular subsequent handler executes. It proves that the recognized hypothetical state is insufficient to select the documented non-executing route through this contract.

### Routing publication and cleanup need a common lifecycle

Chat calls `clearStaleKernelFacts` after the perception call. That cleanup retracts `current_understanding`, `llm_suggested_mode`, `derived_mode`, primary shard, and context/tool priorities, despite its comment saying perception will reassert them. At this call site, perception has already run. When the transducer publishes to that same kernel, the ordering removes the newly published routing evidence. Consequently, wiring a new consumer to those transient facts alone would still not establish their survival through the normal chat sequence. [Actual call ordering](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L166-L242), [retraction block](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L39-L50).

Do not assume every entry path has published those facts. The preferred firewall's `ensureTransducer` constructs its own transducer and sets its prompt assembler, but does not call `SetKernel`. With no routing kernel, `deriveRouting` returns the model's suggested mode and shards directly and does not derive blocked tools or publish routing facts. That is another reason acceptance must exercise the real firewall, not only a test that manually calls `SetKernel` on a standalone transducer. [Firewall construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/perception.go#L234-L260), [nil-kernel routing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L597-L607).

This is an integration constraint for the proposed work, not a claim that every route or tool priority currently depends on the erased facts. The current interactive decision bridge separately publishes the smaller signals it consumes.

## Existing action gates remain important but do not supply the missing link

The executor checks the effective JIT allowlist before backend dispatch and performs constitutional admission for gated calls. It freezes generated-tool arguments, checks exact action/target/payload permission, and has later guards. These protections remain necessary. A restriction repair must narrow them, not bypass them. [Allowlist and safety checks](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2377-L2463), [exact permission matching](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2991-L3048).

However, the ordinary constitutional `safe_action` arm derives `permitted` from the pending operation and dangerous-content checks, without joining a current-user restriction. `write_file` and `fs_write` are explicitly safe actions in this policy. A condition described as “safe” in that rule is not evidence that the user requested its effect on this turn. [Safe-action rule and entries](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/constitution.mg#L5-L11), [file operations](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/constitution.mg#L97-L110).

The shipped human-approval rules are request-bound and substantially stronger than an action-type-wide approval flag. They apply to dangerous actions that the policy explicitly marks human-approvable, currently `delete_file` and `run_arbitrary_command`; a recoverable-delete exception also exists. The source does not connect `Understanding.Signals.RequiresConfirmation` to a consent obligation for an ordinary write that would otherwise be permitted. [Current approval rules](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/constitution.mg#L13-L75).

Thus “modify this file, but show me the change and get confirmation first” is a useful acceptance case: the action may correctly remain a modification, while the requested ordering and consent must constrain its execution. Changing its verb to a generic question or relying on a later model to reread the sentence is not an equivalent deterministic contract.

## Proposed restriction contract

The following is a design, not an existing API or runnable implementation. Begin by amending the perception, session, core policy, and approval capability specs with explicit exits. Preserve the root rule that models describe and the kernel decides.

### Represent a restriction without creating a grant

Use a versioned, immutable descriptor containing the request/turn identity, normalized recognized obligations, their scope, source reference or input-span reference, interpretation status, and a digest. Retain the original constraint text separately for explanation and unsupported detail. Do not require raw prompts or hidden reasoning in the receipt.

An initial delivery can support whole-turn read-only or hypothetical work and confirmation before defined effects. Target-specific prohibitions, conditional exceptions, and “do X before Y” dependencies need explicit semantics before being claimed as enforced. A single `is_negated` boolean cannot say which action is prohibited.

Distinguish a recognized obligation from no recognized obligation, a missing legacy field, and an unsupported or contradictory interpretation. Choose and document compatibility behavior for each producer rather than letting an absent field quietly mean unrestricted execution. Low-confidence interpretation should create a policy-governed clarification or restricted continuation, not fabricated consent.

Model output may propose a restriction that narrows behavior; it must not grant authority, release a host restriction, or assert that the user approved an operation. A source-span reference improves attribution but does not itself prove the interpretation is correct. Quoted text, examples, retrieved files, and an assistant's old response are not interchangeable with the current user's instruction. Those distinctions belong in the perception contract and its adversarial evaluation.

### Preserve the descriptor across execution boundaries

Carry the same descriptor or immutable reference through `Understanding`, `Intent`, `TaskRequest`, preset intent, multistep decomposition, and subagent execution. The current task request carries a constraint string, and the preset path deliberately avoids re-perceiving the task. That is a useful property: delegation should not require another model call to reconstruct the restrictions it was already given. [TaskRequest](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L16-L48), [preset construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L217-L235), [preset dispatch](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L384-L392).

A child may receive additional restrictions but may not silently remove its parent's. Specify how a later authorized user instruction supersedes a restriction, including effects already dispatched. Do not implement release by mutating a shared object or by reinterpreting the previous reply as consent.

Bind each pending action to its owning task/turn and restriction generation. Preset executor tasks already obtain unique task-scoped intent IDs because they share a session kernel. A new global `read_only` fact or a restriction tied only to `/current_intent` would regress that isolation. [Task-scoped intent contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1072-L1085), [intent publication](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1189-L1215).

### Derive consequences in Mangle at selection and admission

Use the same normalized restriction semantics for two distinct purposes:

1. **Catalog selection:** withhold tools whose relevant effects violate the restriction before compiling their prompt atoms and schemas.
2. **Final admission:** reject an actual operation that violates the restriction even if the model calls it anyway, an alias exposes it, or a backend has a broader static capability envelope.

The existing turn catalog already narrows tool envelopes based on language and missing prerequisites. It supplies a natural integration point, but is not a substitute for final admission. [Catalog narrowing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/turn_catalog.go#L20-L80), [policy distinction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/jit_tools.mg#L1-L19).

The final check needs trusted effect classification, not only the four names in `deriveBlockedTools`. Reuse and validate existing backend/effect metadata for file aliases, CodeDOM operations, generated tools, MCP calls, and command-like actions. An unknown effect cannot silently qualify as read-only. A “run tests” operation can create build artifacts or invoke project code; define whether a restriction concerns source modification, workspace mutation, external effects, or all of them. Do not advertise stronger isolation than the selected effect contract actually provides.

Every relevant permission route must respect restriction satisfaction. Adding a `blocked` fact does not cancel an independently derived positive `permitted` fact. Either incorporate the obligation conjunctively into all applicable admission arms or introduce a mandatory final derived admission relation over existing permission and request restrictions. Review the bridge, approval, recoverable-delete, and override paths as well as `safe_action`. The design choice must preserve one executive authority rather than create conflicting Go and Mangle decisions.

### Reuse request-bound consent without broadening authority

A user's “confirm first” is an additional obligation for an otherwise allowable effect. It must not make a hard-prohibited operation human-approvable. Reuse the existing broker, exact operation preview, grant reservation, post-wait rebuilding, and normal gate rerun. Include the relevant restriction generation or digest in the binding so an old grant cannot satisfy a changed request.

No attached approval host means the required effect remains unexecuted with an actionable result. A retry, a repeated input, `requires_confirmation=false`, or a model-produced statement saying “approved” must not discharge the obligation. Do not reuse the clarification loop guard as consent: declining to clarify a target and approving a frozen mutation are different events.

### Publish and retire restrictions in an owned order

Retire the old turn's state before publishing the new state, or atomically publish a scoped snapshot. The current chat cleanup ordering is a concrete integration case to repair. Assertion, retraction, and query failures that prevent proving a restriction must be visible; they cannot be ignored while mutation continues under the assumption that no restriction exists.

For concurrent work, retract only the owning task's facts. Define how already-started calls interact with cancellation or restriction revision; a new restriction cannot truthfully undo an effect that has already happened. Link the admission decision to existing effect observations so the result can distinguish prevented, approved and dispatched, changed, unchanged, and unknown effects.

### Keep the evidence small and useful

Record the restriction descriptor digest, interpretation status, owning task/turn, relevant effect classification, admission result/reason, approval request if any, and outcome link. This is a focused input to the broader planned perception decision receipt, not a requirement to build the entire receipt platform first. Bound retention and avoid storing full user prompts, sensitive payloads, or hidden reasoning by default.

## Verification through normal execution

The existing read-only perception E2E test is useful but narrower than its title may suggest. It checks the returned intent, preserved constraint markers, cached `Understanding.IsReadOnly`, cached blocked tools, and selected routing facts. It does not invoke a registered write handler through the executor and prove that the restriction prevents dispatch. [Current test](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/tests/e2e/perception_contract_e2e_test.go#L119-L252).

The acceptance suite should start with deterministic provider fixtures to isolate transport and policy, then exercise ordinary production boot, chat, direct execution, delegation, and approval flows. Retain normal world-model, CodeDOM, JIT, and registered-tool behavior. No such suite was run for this report.

| Scenario | Required observation |
|---|---|
| Modify with confirmation required | Ordinary write waits for approval of the frozen request; no dispatch before the decision |
| Same request without an approval host | Effect remains unexecuted; actionable unmet-consent outcome |
| Modify with no changes allowed | Forbidden write never reaches its handler; permissible reading still works |
| Hypothetical with modifying action and suggested coder | Policy selects non-executing behavior despite contradictory tool/shard suggestions |
| Free-text read-only paraphrase | Interpretation status and normalized obligation are checked; unsupported phrasing is not labeled enforced |
| Base and Gemini transducers | Same restriction contract reaches admission on both paths |
| Firewall chat and direct executor | Both preserve the descriptor; post-perception cleanup cannot erase it |
| Preset, multistep, and subagent execution | Each child carries the applicable parent restriction without re-perception or widening |
| Concurrent differently restricted tasks | Each action uses only its owner's restrictions and approval; no sibling borrowing |
| Restricted turn followed by authorized edit | No stale overblocking from the previous turn |
| Approval rejection, expiry, changed payload or changed restriction | No dispatch; old grant cannot authorize a new subject |
| Duplicate resume or duplicate approval event | At most the intended single execution claim succeeds |
| Assertion or policy-query failure | Restriction uncertainty is explicit and relevant effects fail closed |
| Aliases, CodeDOM, generated tools and MCP effects | Equivalent effects satisfy equivalent restrictions across backend boundaries |
| Unknown effect metadata | Explicit unsupported or denied outcome, not assumed read-only |
| Quoted prohibition or old assistant instruction | Does not become a falsely attributed current-user restriction |
| Unrestricted positive control | Real authorized work still completes through the normal tools and gates |
| Action already dispatched before cancellation | Effect is accounted honestly; prevention is not claimed retrospectively |

Use handler invocation counts, immutable requests, effect receipts, and file/world-state observations as appropriate. A list containing `write_file`, an absent catalog entry, or a reassuring response is not by itself evidence of prevented execution. Conversely, zero dispatch for every task is not a successful system: measure useful authorized completion and correct refusal separately.

For interpretation quality, build held-out paraphrases of read-only, hypothetical and confirmation requests, including mixed requests, negation scope, quoted instructions, near-neighbor mutation examples, and context changes. Keep evaluator labels independent from the training examples. Report the difference between extraction errors, transport loss, policy admission errors, and final task failure rather than pooling all of them into “intent accuracy.”

## Full cost accounting and comparative evaluation

No savings or performance improvement is claimed from this inspection. The purpose is reliable requested behavior; resource changes must be measured rather than inferred from a smaller prompt or a larger test count.

Compare four treatments with the same models, task snapshots, policy baseline, normal tool context, and repair budget:

1. Current production behavior.
2. Typed restriction transport only.
3. Transport plus catalog narrowing.
4. Transport, narrowing, final admission, and consent integration.

The transport-only treatment is deliberately incomplete. It isolates field preservation from actual enforcement. The catalog-only treatment similarly distinguishes behavioral steering from a final gate. Do not make the baseline artificially safer or cheaper by removing mutating tools, skipping normal context construction, or substituting already-correct preset fields that production would have lost.

Collect disjoint resource categories:

- common boot and cache hydration, semantic-query embeddings and retrieval, world-model and CodeDOM preparation, with cold/warm runs and any amortization stated explicitly;
- classification input/output/cache tokens, logical calls, HTTP attempts, failures and retries;
- any added restriction-extraction, clarification or repair calls and their tokens;
- restriction validation and hashing CPU, descriptor size, kernel fact count, rebuild/query cost, peak memory, and p50/p95 latency;
- tool catalog/schema tokens, selected prompt atoms, prefix-cache changes, and prompt-assembly work;
- per-action admission latency, effect-metadata lookup, policy-query failures, and rejected attempts;
- approval preview work, source rereads, journal I/O/storage, approval wait time, and reviewer minutes;
- actual tool and verification work, including local CPU/memory/I/O, test/build time, remote backend charges and model-generated repair work, counted once rather than omitted as unchanged baseline costs;
- verified task completion, correct refusal, forbidden effects, overblocking, extra turns, user corrections, and remediation;
- evidence retention, deletion, and operational diagnosis costs.

Report both full-run totals and the incremental cost of the restriction treatment. Common work may be held constant for an ablation, but it still belongs in end-to-end cost per correct outcome; avoid counting repair calls in both the model-call ledger and a second monetary repair subtotal.

Typed transport should not require an additional model call merely to copy an existing interpretation. That is a design expectation, not a measured result. Catalog narrowing may reduce repeated schemas and futile calls, but changed catalogs can also affect prompt caching. A stricter interpretation can prevent unwanted effects while increasing clarification, refusal, and human-review cost. Include both outcomes.

Use total cost per correctly completed or correctly refused request, with those denominators also reported separately. If expressing a scalar monetary total, convert all resource terms using disclosed prices, cache assumptions, hardware allocation and human-time valuation. Otherwise report the resource vector; tokens, milliseconds, stored bytes and reviewer minutes are not directly additive. Report uncertainty and sensitivity, especially where human waiting dominates elapsed latency.

A fair workload includes authorized edits, requests that must remain read-only, consent-gated mutations, safe information requests, and ambiguous cases. Compare equivalent intended work, not a treatment that appears fast because it refuses or silently drops more tasks. Evaluate source-equivalent deterministic fixtures first, then normal production runs with captured outcomes and no artificial restriction of the production tool surface.

## Delivery order and checkable exits

These are proposed local research identifiers, not already committed feature cards.

| Proposed gap | Priority | Checkable exit |
|---|---|---|
| GAP-PRC-01 Restriction semantics and ownership | P1 | Supported obligations, unsupported/absent states, scope, release and task inheritance are specified |
| GAP-PRC-02 Typed transport continuity | P1 | Both transducers and all normal preset/delegation paths preserve the same immutable restriction identity |
| GAP-PRC-03 Publication and retirement order | P1 | Normal chat cleanup and concurrent task lifecycle cannot erase or borrow current restrictions |
| GAP-PRC-04 Policy admission integration | P1 | Registered forbidden effects never dispatch; existing permission paths cannot bypass the restriction obligation |
| GAP-PRC-05 Consent composition | P1 | Otherwise-permitted consent-gated effects use exact host approval; hard prohibitions remain hard |
| GAP-PRC-06 Behavioral and cost evaluation | P2 | Extraction, preservation, enforcement, completion, overblocking and total resource use are reported separately |

The smallest coherent implementation should carry and enforce one specified restriction through a normal production path, including preset/delegation behavior and failure cases, before expanding the restriction vocabulary. Transport-only or prompt-only work can be a useful intermediate measurement, but must not be labeled completion of the capability.

The defining result is not that `NeedsConfirmation` gains a caller or that every field appears in a log. It is that a recognized current-user limit becomes an attributable obligation at the action boundary, and the system can demonstrate both the effects it prevented and the useful authorized work it still completed.

## Branch qualification — conversation brief and Jev (2026-10-28)

The mainline findings above remain frozen at [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871). A supplementary source inspection considered `feat/conversation-brief` at `58463864e3e78efdf72b0706a21f495ffead23c3` and then revalidated through the source freeze, [`9755db4f5851c28771dc1dd6d720856be1d5813a`](https://github.com/theRebelliousNerd/codenerd/commit/9755db4f5851c28771dc1dd6d720856be1d5813a) (committed 08:59:40 UTC; branch checked 2026-10-28 at 09:00 UTC). This is a branch-specific qualification, not a claim that the feature is merged into main or that its tests passed in this investigation.

**Credit the added safeguards.** The feature asks Jev alongside perception and records its judgments before routing. A thresholded positive `hypothetical` judgment on a mutation, or a negative `changes_files`/`names_target` judgment, derives `jev_disputes_mutation` and then `needs_clarification`. That is a real additional policy path for asking before acting, so the mainline example must not be read as proof that the feature branch necessarily proceeds to mutation. [Turn integration](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process.go#L177-L216), [thresholds and mutation dispute](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/jev_evidence.mg#L25-L48).

**Do not equate that path with restriction continuity.** It is a second model judgment, not transport of `Understanding.Signals.IsHypothetical`. Disabled, failed or canceled judgment collection leaves no Jev evidence. Reused held-intent execution skips starting a new Jev judgment; `recordTurnJudgments` still retracts the previous judgment predicates, then returns for the nil channel, so that execution does not inherit the prior turn-judgment evidence. The resulting clarification still obeys the existing lane precedence and repeat-input guard: `clarify_lane` excludes direct-answer, multi-step and early lanes, and `clarified_already`. These conditions limit the safeguard; they do not establish an observed bypass. The dream lane still requires the `/dream` verb. [Judgment collection](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/jev_turn.go#L31-L85), [clarification guards](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/routing_arbitration.mg#L303-L311), [dream lane](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/routing_arbitration.mg#L164-L166).

The feature also adds conversation-brief/work-mode admission: propose/explicit modes can hold work, a not-ready brief can hold yolo-mode work, and a go-ahead admits the held proposal. This is useful existing machinery to reconcile with any restriction envelope, not a reason to build another hold subsystem. However, those rules do not consume `RequiresConfirmation`; `understandingToIntent` now carries `Brief` but still lacks typed hypothetical, confirmation and blocked-tool fields, and `assertRoutingFacts` still publishes routing priorities/mode rather than those obligations. Work-mode admission is not an exact action grant or proof that every downstream tool honors a current-request restriction. [Work admission](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/routing_arbitration.mg#L233-L272), [Intent projection](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/perception/understanding_adapter.go#L474-L533), [routing publication](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/perception/transducer_llm.go#L681-L736).

**Mid-work instructions now have a separate path.** The side-turn change in `777e93c` reads a message through the direct transducer, updates the brief, and publishes `side_turn_intent`/`side_turn_judgment` rather than asserting a new `user_intent` in that bridge. Mangle selects stop, queue-for-later or conversation. Credit this as a path for revising the brief and requesting cancellation while foreground work runs, not proof that a new restriction reaches every action gate. Because parsing still uses the transducer, isolation of all routing facts remains conditional on its kernel attachment; the separate side-turn predicates alone do not prove it. [Side-turn bridge](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/side_turn.go#L157-L222), [side-turn policy](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/side_turns.mg#L10-L35).

**Remaining verification:** carry a correctly recognized restriction through the real foreground, held/resumed, delegated and background paths to catalog selection and final action admission, including Jev unavailable, repeated clarification, multi-step cases and restrictions added mid-work. Preserve the new branch safeguards while proving the typed obligation; neither their existence nor a passing isolated judgment test would discharge that contract. No runtime probes or tests were run for this appendix.

