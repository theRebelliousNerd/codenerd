---
doc-class: deep-dive
subsystem: session-observation-core
implementation-status: proposed-refinement
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
supersedes: []
---

# Tool effect recovery needs a closure observation

## Decision

Preserve the complete historical tool-effect receipts, but distinguish the recovery a call owed when it returned from the recovery still owed when its turn closes. Export the latter from the authoritative Mangle closure rather than presenting every historical recovery as current parent uncertainty.

The shipped kernel already retires a rejected effect after a later measured write replaces it. The remaining gap is at the session-to-parent projection: an old receipt's cached `/repair` survives that retirement and is rendered as “tool effect not settled” and “owed recovery /repair.” A parent can therefore receive a truthful historical rejection under a false current-obligation label.

**Evidence status:** static source and test-definition inspection only. No application execution, tests, model calls, benchmarks, implementation changes or repository writes were performed for this investigation. The witness below is not a reproduced incident. The demonstrated consequence is inaccurate current uncertainty, not an automatic failed verdict, retry, duplicate write or permission bypass.

## Scope and prior work

Main resolved to `486db57ce490302c65d7f484e7f6070ee0e70862` during inspection on October 30, 2026. All source links below use that immutable revision. Root [guidance](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/agents.md), [core guidance](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/agents.md) and [session runtime contract](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/README.md) assign executive decisions to Mangle and preserve exact tool authorization.

This is a refinement of [tool-effect receipts and validation completeness](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/tool-effect-receipts-and-validation-completeness.md), not a rediscovery of its original dropped-output or uncounted-write findings. Its as-built sections record real progress: rejected writes remain in the effect set, earlier test witnesses are invalidated, validation outcomes are typed, and receipts now cross the observed-return boundary. Those repairs must remain. Its remaining durable-replay and concurrent-attribution limits are also not the subject here.

[Delegated-turn failure settlement](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/delegated-turn-failure-settlement.md) covers retaining partial execution evidence when a turn errors. Here the worker can recover and return normally. [Working-observation supersession](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/working-observation-supersession-contract.md) covers read-content coverage and acquired revisions, not effect-recovery projection. [Continuation obligation generation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/continuation-obligation-generation.md) concerns description-based reuse of tester results, a different obligation boundary.

The research branch `research-main-20261030-0211` was compared with main; its three added audits concern expert-card scope, knowledge-miss retirement and schema rollback, not this boundary. This is a bounded nonduplication check against the inspected current inventory and adjacent reports, not a claim about every historical discussion.

## Production path

This is not merely an unused codec. BootCortex installs the task delegator adapter on VirtualStore; its observed method forwards to the session executor. The JIT inline path builds an observed return from the completed execution result. VirtualStore's delegate handler obtains that return and encodes it for the parent.

- [Boot installation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/factory.go#L2739-L2753)
- [Observed adapter](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/factory.go#L664-L679)
- [JIT return construction](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/task_executor.go#L456-L463)
- [Delegate result acquisition](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/virtual_store_actions.go#L904-L925)
- [Parent output and codec](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/virtual_store_actions.go#L942-L993)

The delegate handler's error branch returns before this successful-result encoding; the minimal witness therefore uses a turn that recovers and returns without a Go error. Error diagnostics independently use `PartialState`, which also includes unsettled effects, but that is not required to establish the normal-return issue. [Partial projection](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/observation/partial.go#L30-L43)

## Where current meaning is lost

The shared tool accounting path calls `settleToolEffects` before returning any post-action error, preserving the existing effect-accounting repair. [executor_tools.go 1039–1108](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor_tools.go#L1039-L1108)

At each effectful call, settlement asserts the receipt, optionally performs the policy's revalidation move, obtains `tool_effect_recovery`, stores that string in `receipt.Recovery`, and appends the receipt. It does not revise earlier receipts when later calls change the same resource. [tool_effects.go 270–291](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/tool_effects.go#L270-L291)

The policy is more precise than that cached string. `turn_effect_current` identifies the last effect on each path, excluding explicitly superseded effects. Every recovery rule joins that current-effect relation. Thus a later verified repair removes the older call's current recovery without rewriting its historical validation. [Current effects](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/tool_effects.mg#L39-L49), [recovery rules](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/tool_effects.mg#L99-L115)

At closure, `assertSupersededEffects` can also identify replacements made by writers not represented in the receipt sequence. It asserts supersession facts but does not update the result's receipt slice. This useful host observation must not be replaced by a parent heuristic that merely picks the highest receipt sequence. [Closure observation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/tool_effects.go#L365-L393)

The mismatch then crosses the handoff:

1. `observedReturn` includes every `ToolReceipts` entry.
2. `projectReceipts` copies each old `Recovery` unchanged.
3. `EffectReceipt.Unsettled` treats any nonempty recovery as still owed; it does not consult subsequent receipts, a closure observation or applicability state.
4. `structuralUncertainty` adds those strings to the parent projection, which renders them under `unsettled`.

[Receipt projection](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/observed_return.go#L164-L190), [unsettled interpretation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/observation/effects.go#L45-L82), [uncertainty assembly](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/observation/subagent.go#L791-L797), [rendering](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/observation/subagent.go#L1147-L1155)

Historical evidence is not the defect. The defect is assigning it current obligation semantics without the current executive result.

## Source derived witness and existing safeguards

The existing real-kernel fixture already constructs the important sequence: a valid write, a malformed JSON write rejected by validation, then a verified repair of that same JSON path. It checks that the original call derives `/repair`, that the repair removes `turn_has_rejected_effect` and `/effect_rejected`, and that all three receipts remain. It does not project the repaired result to a parent. [TestToolEffects_PolicyDerivesRepairAndWithholdsTheTurn](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/tool_effects_test.go#L145-L190)

Following that fixture's sequence through the current projection leaves the rejected receipt's `Recovery` as `/repair`. The final projection therefore lists a repair still owed even though the same kernel has retired that obligation. No external writer, race, malformed provider packet, cross-turn reuse or failed final acceptance is necessary.

A separate fixture verifies that an externally replaced receipted change stops withholding the turn. That fixture likewise stops before parent projection. [Superseded-change fixture](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/tool_effects_test.go#L276-L297)

Important limits:

- The kernel's current-effect retirement works. This report does not claim that a repaired receipt still blocks the worker's `turn_verified`.
- The exported turn outcome, acceptance and successful checks can independently tell the parent that the task succeeded. A model may reconcile the contradiction correctly.
- No deterministic parent dispatch rule was established that retries solely because this uncertainty string exists.
- Receipt history, sequence numbers and content identities remain useful. Removing rejected receipts would destroy evidence rather than repair its meaning.
- A later write is not automatically a successful repair. Unknown, rejected and partially replaced current resources must retain their actual obligations.
- The existing capability, exact-permission, validator and task-acceptance gates remain authoritative. A settled historical recovery grants no authority for another action.

The practical risk is unnecessary investigation or repair, contradictory reporting, and wasted continuation work. Their frequency and cost have not been measured.

## Proposed closure contract

The following is design notation, not a shipped API.

Keep immutable call-time receipt observations: call identity, tool, per-resource before/after identities, original validation, handler error and recovery observed at that moment. Add a separate closure record keyed to the turn and receipt/resource identities, with a clear observation boundary.

The closure should carry policy-derived current applicability and recovery. Suggested distinctions are current with recovery, current without recovery, superseded, and unknown closure state. An empty recovery string alone is not enough to distinguish successfully settled state from a failed query or missing closure.

Capture the closure while the owning turn's facts still exist. Read the policy's conclusions in a coherent turn-local operation; export the data before cleanup. Go transports and records the result, while Mangle retains the decision about which effects are current and what remains owed. Do not reconstruct this executive decision independently inside `projectReceipts` or `EffectReceipt.Unsettled`.

For multi-resource calls, preserve resource-level applicability even if validation remains call-wide. If a rejected call changed A and B and a later verified call repairs only A, B still owes recovery. The parent must not list A as a current rejected resource, and must not clear the whole call merely because one resource was replaced. A call-level recovery can be accompanied by the exact current resource subset derived from the existing per-path relations.

Unknown closure, query failure and incomplete publication must be explicit; none may silently mean “no recovery.” Existing behavior for unreadable source state should remain conservative. A closure observation is not a permanent freshness guarantee against subsequent external writes. Later reuse needs its existing revision checks or an explicit new observation; this proposal does not introduce a lock, ownership CAS or exactly-once replay store.

Render history and current debt separately. A parent may receive “earlier validation rejected revision X; superseded by later observed effect” without receiving “owed recovery /repair” for X. Preserve the historical receipt for audit and hydration. Only current closure obligations belong in the actionable `unsettled` section.

## Alternatives and costs

Compare three variants:

- **A Current path:** immutable receipt plus cached call-time recovery rendered as current uncertainty.
- **B Presentation-only mitigation:** label the recovery as historical and stop claiming it is currently owed unless closure evidence exists. This removes the false assertion but weakens actionable parent information.
- **C Closure projection:** preserve history and export authoritative current applicability/recovery. This is the recommended contract if the added boundary can be implemented cleanly.

Avoid an additional model call. The relevant executive relations already exist. A closure projection adds kernel query/materialization work, metadata, serialization and test obligations; it can also require a schema/version decision for retained returns. Prefer one turn-scoped extraction over one full query per receipt. For R receipts and P resource effects, basic projection storage is proportional to R plus P; actual kernel evaluation cost depends on the existing corpus and cannot be inferred from that storage bound.

Measure closure CPU and allocations, kernel evaluation/query time, serialized bytes, retention cost, parent input tokens, extra tool/model rounds, repair attempts, task completion rate and total latency. A smaller or more confident parent response is not a benefit if unresolved effects disappear. Include failed, canceled and unknown-closure cases in the denominator.

No speedup, token savings or reliability improvement is claimed from static inspection.

## Proposed verification

No tests below were run.

| Fixture | Required observation |
|---|---|
| Rejected write then verified repair of same path | Original rejection remains in history; final kernel and parent agree that its repair is no longer owed. |
| Rejected write with no repair | Current rejection and repair obligation survive projection. |
| Rejected A and B then repair A only | B remains owed; A is not named as current rejected debt. |
| Repair itself rejected or unreadable | Current rejected or unknown effect remains visible. |
| External replacement recognized by closure | Exported applicability agrees with authoritative supersession; parent does not infer retirement solely from receipt order. |
| Closure query or publication fails | Explicit unknown/unavailable closure, not successful settlement. |
| Canceled or failed worker with partial writes | Preserve partial effects and current uncertainty without replaying the worker to rebuild reporting. |
| Short and retained large return | Same obligation semantics in verbatim output, projected output, serialization and hydration. |
| Same path later changes after closure | Closure stays honestly historical; existing freshness checks are not bypassed. |

Extend the existing repair fixture through `observedReturn` and `ProjectReturn`, then drive the installed observed delegator through the normal VirtualStore action and inspect the actual parent-facing result. Compare kernel current-effect/recovery relations, exported closure state and rendered parent uncertainty separately. A codec-only test does not prove production installation, and a kernel-only test does not prove faithful transport.

For system-level value, use matched authorized tasks containing both recoverable validator rejection and legitimate unresolved failures, with fixed model/provider configuration and identical acceptance requirements. Measure unnecessary repairs and verified outcomes alongside full cost. Include multi-resource failures and no-repair controls so a broad suppression cannot appear to improve the benchmark.

## Exit

A fully repaired rejected effect remains available as historical evidence but cannot reappear as current parent recovery debt. A partly repaired or unknown effect remains explicitly unresolved for the correct resources. The parent receives the same current obligation meaning the owning kernel derived, without a new inference engine, new authority or replay of completed effects.

