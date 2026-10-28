---
doc-class: deep-dive
subsystem: articulation-session
implementation-status: planned
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Native tool precedence must not discard unrelated response controls

## Decision

Separate **which tool-call representation wins** from **whether a response's other control channels receive a disposition**. A native tool call may take precedence over an envelope tool request without making a co-carried memory operation, observation or parser rejection disappear. Settle each response's supported controls before the execution loop can replace that response; keep application outcomes separate from native tool execution and from the final answer.

The concrete residual is narrow: an intermediate native response can carry both a valid Piggyback envelope in `Text` and native `ToolCalls`. The promotion helper returns before parsing any text when native calls exist. The loop then replaces that response with the next one, while terminal control admission sees only the surviving response. A valid memory request in the earlier envelope therefore need not reach the memory applier or acquire any refusal/failure disposition.

**Evidence status:** static source and test-definition inspection only. No codeNERD execution, tests, model inference, failure injection, implementation or performance measurements were performed. Examples below are source-derived conditional fixtures, not reproduced incidents. This is an omission/reliability finding, not evidence of an authorization bypass or a claim that every native provider emits mixed replies.

## 1. Source freeze and non-overlap

On 2026-10-28 at approximately 17:08 UTC, the inspected heads were main [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), `feat/conversation-brief` [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c), historical `codenerd-research-docs` [`c592ff223cd207fca4c29ae793d5eba57aa4781d`](https://github.com/theRebelliousNerd/codenerd/commit/c592ff223cd207fca4c29ae793d5eba57aa4781d), and fresh `research-main-20261028` [`548bb1cf93ecbcd0fd3e9c31afd4e43ea2b60c54`](https://github.com/theRebelliousNerd/codenerd/commit/548bb1cf93ecbcd0fd3e9c31afd4e43ea2b60c54). All code citations below are immutable.

This is a focused extension of the historical [articulation admission and control-fidelity audit](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/articulation-admission-and-control-fidelity.md), not a replacement for it. That audit establishes the general requirement for one interpretation and explicit control dispositions. Its feature refresh already covers candidate eviction hiding ambiguity. The new contribution here is the **native-call short circuit followed by intermediate-response replacement**, with a normal-loop witness that requires neither malformed input nor a failed memory store.

Credit the feature's [as-built implementation](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/articulation-admission-and-control-fidelity.md#as-built-2026-10-28): duplicate-key admission, the `ControlsApplied` marker, scoped memory outcomes, parser-removal dispositions and the repaired single-shot Piggyback consumer are real. They do not cover a response that never enters admission. The explicitly deferred durable response identity, declared plain/envelope mode, provisional UI state and chat candidate-action dispositions are not new findings here.

The [delegated-turn failure-settlement audit](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/delegated-turn-failure-settlement.md) concerns measured work lost on hard loop errors. This witness follows a successful ordinary loop and final return. The [background-outcome audit](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/background-outcome-and-continuation-ownership.md) concerns a completed job's owner and successor; the missing operation here occurs inside one worker before job completion. The [conversation-history handoff audit](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/conversation-history-coverage-handoff.md) concerns context coverage. Keeping the raw intermediate reply in provider history is not the same as executing or explicitly rejecting its memory proposal.

## 2. The mixed response is representable; native precedence is intentional

Root guidance assigns executive truth to the logic layer. Scoped articulation guidance requires typed control validation and a disposition for applied controls; session guidance preserves native/Piggyback capability, permission, accounting and result contracts. None of this makes a parsed control an authority grant. [Root](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md#north-star), [articulation guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/articulation/agents.md), [session guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/agents.md).

The shared [envelope atom](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/atoms/protocol/piggyback.yaml#L1-L49) advertises Mangle and memory channels. The [consumer-need policy](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/jit_needs.mg#L83-L102) deliberately teaches envelope **tool requests** only to the text channel and **knowledge requests** only to chat articulation; it does not declare that native replies cannot contain memory controls. The [structured-output filter](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/output_mode.go#L23-L82) excludes competing protocols for particular structured roles, not every native worker. This is source-level protocol reachability, not a capture proving that every live prompt contains every optional atom.

The Anthropic adapter maps text and `tool_use` blocks into one response containing both flat `Text` and `ToolCalls`, while retaining ordered blocks. Mixed text/tool responses therefore do not require a fabricated response type. [Adapter](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/blocks_anthropic.go#L110-L150).

Native tool precedence itself is correct: the same model action must not execute twice merely because it appears in two representations. The existing [native-precedence test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/piggyback_promotion_test.go#L231-L256) constructs envelope text plus a native call and requires promotion to return false, preserve the entire text, and preserve the native call. It does not place a memory operation in the envelope or check whether non-tool controls later receive a disposition.

The gap is the **scope** of that precedence decision, not its existence.

## 3. Exact source chain

1. `generateResponse` obtains native responses directly from the client's native tool interface or `completeWithWorkingContext`. The latter prepares the request and forwards the provider result; it does not settle response controls. [Native generation](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1729-L1765), [working request wrapper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L922-L935).
2. `runToolLoopPass` calls `promotePiggybackToolRequests` after initial generation. The helper's first guard returns when `len(resp.ToolCalls) != 0`, before `ProcessLLMResponseAllowPlain` or `applyPiggybackControls`. [Initial boundary](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L103-L110), [short circuit](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/piggyback_promotion.go#L33-L71).
3. The first response is added to history with `AssistantMessageFrom`; this preserves transport content but applies no controls. The tool loop executes the native batch normally. [History](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L175-L189), [batch](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L276-L278), [message conversion](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/types/content_block.go#L247-L260).
4. On a successful follow-up, the loop calls the same promotion helper for the new response and assigns `currentResponse = nextResp`. A final response without tool calls returns through terminal verification. There is no admission of the replaced native response at this boundary. [Replacement and return](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L358-L397).
5. Only the returned response reaches the task tail's `admitResponseControls`. That method reads its argument, not all earlier assistant messages. [Task tail](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1324-L1331), [admission](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L2432-L2467).

Consequently the earlier memory operation never reaches the common [memory application call](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L2522-L2547). Its missing disposition is not a failed store report hidden in a log; the store was never asked by that control path.

Main has the same structural omission: [native short circuit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/piggyback_promotion.go#L27-L33), [intermediate replacement](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L352-L391), and [terminal-only processing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1302-L1308). The feature's improved admission API does not introduce the omission and should not be reverted to repair it.

## 4. Minimal source-derived witness and counterexamples

Use a scripted native client through the ordinary session tool loop, a workspace with an active working policy, a read-compatible intent, one offered and permitted read tool, and a recording memory kernel/store. The request authorizes remembering a harmless test preference while inspecting a file. No write-oriented edit plan or post-edit repair is needed.

The first response has one native read call and the following valid envelope as `Text`:

```json
{
  "control_packet": {
    "memory_operations": [
      {"op": "note", "key": "fixture_preference", "value": "tabs"}
    ],
    "tool_requests": []
  },
  "surface_response": "I will inspect the file and keep that preference."
}
```

The production tolerant parser accepts omitted optional fields; a schema-complete variant can fill the other fields without changing the witness. The next response is ordinary final text, with no tool calls and no repetition of the memory request. The native read completes normally, and the turn reaches its ordinary terminal path.

**Source prediction:** the native read is handled once by its normal gates; the first envelope's `note` is not applied; no failed/unsupported/rejected disposition is produced for it; the tail admits only the final text. Record `session_note` calls rather than looking merely for the preference sentence in history. Replacing `note` with `promote_to_long_term` and using a recording durable store establishes the corresponding omitted persistence request. Neither fixture needs a live model or real durable writes.

Important boundaries:

- The intermediate envelope remains in the local tool transcript. This is not a claim that every byte or semantic hint is erased. A later model can happen to repeat the operation; that is not deterministic settlement, and the witness explicitly does not repeat it.
- A final envelope reaching `admitResponseControls` is handled. Likewise, a native response used by forced finalization may have calls cleared and then reach terminal admission. Native calls do not make controls permanently impossible; **replacement before admission** is essential.
- Text-channel responses with valid envelope tool requests already go through promotion and apply controls once. The repaired single-shot Piggyback path also applies the whole packet. [Text channel](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/piggyback_channel.go#L76-L95), [single-shot path](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1851-L1872).
- A native response with empty text or plain prose proposes no such control. No application is missing in that case.
- An envelope tool request suppressed by native precedence is not the memory operation. The repair must not run the suppressed tool request or weaken native tool permission checks.
- This does not establish false task completion, unauthorized mutation, incidence in production, or a universal native-provider bug. Task verification, tool effects and control disposition are separate outcomes.

## 5. Proposed contract

The smallest coherent repair is a common **per-response admission boundary**, not a new tool executor or a full durable transaction system. These are proposed responsibilities, not existing APIs:

1. **Observe both channels before choosing tools.** Every complete ordinary execution response in the native/Piggyback loop reaches bounded envelope admission once before it can be replaced. Specialized planner, judge and other structured-output responses retain their own declared contracts; this does not route them through Piggyback. The result distinguishes ordinary prose, admitted controls, ambiguity and malformed attempted control. A successful generation is not an application receipt.
2. **Resolve only tool-call precedence.** Native calls remain the executable tool-call source when present. Envelope tool requests get an explicit precedence disposition where the protocol supports one; they are not appended to the native batch. Independent supported memory/observation channels still receive policy/application dispositions.
3. **Keep authority at the existing gates.** Parsed controls remain proposals. Preserve observation filtering, scoped memory handling, exact native tool permission and the capability envelope. If a control is unsupported on an intermediate native response by deliberate policy, report that disposition instead of dropping it silently. Do not convert response admission into a permission grant.
4. **Retain settlement when the response is superseded.** Attach the supported controls' host outcomes to the owning execution result or a bounded response-sidecar ledger before advancing to `nextResp`. A failure notice cannot depend on the intermediate surface becoming the final answer. Re-entering the same admitted response must not apply its controls again.
5. **Preserve provider transport independently.** Do not fix this by casually rewriting native text/tool blocks into a new order. `AssistantMessageFrom` deliberately preserves native blocks and signatures, while `Rewrite` rebuilds text/tool blocks. A sidecar can hold admitted surface/outcomes without destroying the raw provider transcript. If rewriting is chosen, test actual block ordering, signatures, server blocks and call/result pairing. [Existing block contracts](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/types/content_block.go#L247-L315).
6. **Keep cancellation and retry semantics explicit.** Before any newly reachable control effect, honor its current owner/authorization. If a later model call fails or is canceled, do not pretend an already-applied control never happened. Do not replay it merely to regenerate a final narrative. Durable cross-attempt identity remains the broader receipt workstream; an in-memory `ControlsApplied` mark is not crash-proof exactly-once delivery.

There are two defensible product policies for intermediate memory controls: apply admitted supported controls at the response boundary, matching current promoted-envelope behavior; or explicitly disallow/defer them under a documented request contract. The latter must preserve the deferred operation and its owner if it promises later application. Merely waiting for the model to restate it is neither policy.

Go should perform framing, measured transport selection, application and outcome recording. Mangle should retain admission/permission/continuation authority wherever those decisions are already executive responsibilities. No new lexical heuristics or duplicate persona/tool permission tables are needed.

## 6. Primary research principles and limits

Saltzer, Reed and Clark's [*End-to-End Arguments in System Design*](https://web.mit.edu/saltzer/www/publications/endtoend/endtoend.pdf) places application-specific correctness checks at the endpoints that know the application's requirements. The applicable design inference is that successful provider transport, valid tool/result pairing and a completed read do not establish delivery of a separate memory operation. The session/control owner must observe that result. This is an architectural analogy, not experimental evidence about codeNERD, and it does not justify replaying side effects after uncertainty.

Saltzer and Schroeder's [*The Protection of Information in Computer Systems*, basic principles](https://web.mit.edu/Saltzer/www/publications/protection/Basic.html) motivates fail-safe defaults and checking authority on each access. Applied here, repairing lost proposals must not turn co-carriage with an allowed native read into permission for another effect. Preserve the existing control-specific gates and explicit refusal paths. The paper supplies a design principle, not proof that the proposed implementation is secure or faster.

## 7. Costs and fair alternatives

No token saving or speedup has been measured. The minimal common boundary should not require another model call, but parsing intermediate native text adds CPU work and admission metadata. Avoid reparsing a response in generation, promotion and terminal settlement, and avoid copying full raw replies solely to attach small outcomes.

Some additional latency is the cost of correctly performing a memory operation that currently never runs: store writes, vector embedding or kernel updates are not free. Compare semantically matched alternatives, not a baseline that silently skips work against a candidate that does it. If intermediate controls remain unsupported, measure clarity/recovery cost rather than labeling fewer effects as higher efficiency.

Record total provider calls/tokens, control parse time, kernel/store calls, peak retained bytes, native tool executions, final-notice delivery, user recovery turns and time to the requested remembered state. Separate time to first streamed text, tool completion, control disposition and final task verdict. A bounded per-turn sidecar is much cheaper than a new durable event store, but does not provide crash recovery; adopting durable receipts adds storage, identity, privacy and replay-policy costs that should be justified separately.

Compare: present behavior; common admission with native precedence preserved; and an explicit unsupported-intermediate policy with accurate notices. If a simple common boundary reaches the correctness goal, do not require a distributed transaction framework as the first increment.

## 8. Proposed evaluation and checkable exit

All cases below are proposed and unrun. Start with the existing loop/promotion test setup and a recording memory kernel/store, then add a normal entry-path test with real policy and the production adapter. Helper-only tests cannot establish that a later replacement retained the outcome.

| Fixture | Required observations |
|---|---|
| Initial native read + envelope note; next reply plain | Native read executes once; note receives exactly one declared disposition; its applied or explicitly refused state survives replacement |
| The same mixed reply on a second or later native round | No initial-response-only fix; all generation boundaries use the same contract |
| Native call + different envelope tool request + note | Native precedence remains; no extra envelope tool execution; note disposition independent of that precedence |
| Successful and refusing durable store | Applied and failed persistence are distinct; failure reaches terminal result even when the final model text omits it |
| Malformed/ambiguous envelope next to a valid native call | No salvaged control effects; explicit admission result; native call treatment follows the declared independent-channel policy and exact ordinary gates |
| Plain native prose and tool-only response | No fabricated memory operations, no spurious envelope repair call, unchanged native tool accounting |
| Terminal envelope, promoted text-channel envelope, already-marked response | Existing controls still work once; extracted surface examples never regain authority |
| Signed thinking + text + native calls, server blocks | Provider replay order/signatures and tool-result IDs remain faithful; no flat/block divergence |
| Control applied, then later provider failure or cancellation | Actual control outcome retained; no retry of its effect just to repair the narrative; coordinate with failure settlement rather than masking that separate gap |
| Same response object admitted twice; two distinct responses carrying equal operations | First is not applied twice; distinct proposals are not deduplicated solely by payload equality; any deliberate cross-response dedupe policy is explicit |

Use controls with the same intent, tool catalog, memory authorization and request budget. Include rejected observations and unavailable stores so that "settled" cannot mean "always applied." Keep existing protected-predicate and no-control-from-malformed-text tests intact. For live trials later, use harmless explicit memory preferences and read-only investigation tasks, capture actual mixed-response frequency, and distinguish fixture correctness from incidence.

**Exit:** every supported non-tool control proposed by a complete response has an application, rejection, failure or owned-defer disposition before that response can be superseded; native tool precedence never silently deletes those other channels; normal-loop tests establish the outcome beyond the helper boundary; existing transport fidelity, exactly-once-per-response guards, permission checks and terminal verification remain intact. Update articulation/session specifications and the historical audit's as-built qualification only after implementation and the named checks actually run.
