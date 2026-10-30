---
doc-class: deep-dive
subsystem: cli-perception
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 4cb0b3650de09085351014d522beb440c7e97ae3
verification-method: static-source-inspection
supersedes: []
---

# Perception's early-answer lane needs an evidence admission contract

## Decision

Keep the cheap perception-answer path. Do not let a nonempty draft response stand in for evidence sufficiency. A classification that simultaneously says “conversation” and “this needs a workspace file or tool” is contradictory but currently admitted. The executive should resolve that contradiction before accepting the draft as the final answer.

The narrow source defect is a missing guard at a higher-precedence lane: `wants_direct_answer()` excludes workspace-evidence requests, but `perception_answer_lane()` does not. The existing evidence fallback cannot rescue a turn already accepted by that early lane. The proposed first increment uses the existing Mangle predicate and existing read/articulation path rather than adding another planner, model call, or tool capability system.

**Status:** static source-derived finding and proposed contract. No test, benchmark, codeNERD execution, live inference, or failure injection was performed. No incidence, latency improvement, token savings, or answer-quality result is claimed. A scripted classifier witness is specified below; it is not represented as a naturally observed classification.

## Source freeze and scope

Main resolved to `4cb0b3650de09085351014d522beb440c7e97ae3` during this pass. The inspected public source is linked at that immutable commit. Root `agents.md`, session and perception contributor guidance, and the public-repository rule informed the review. This document describes public codeNERD behavior only.

The relevant prior audit is [TUI evidence turn ownership][prior]. Its original no-candidate witness explicitly excludes other early/clarification lanes. The subsequent implementation added the `gather_evidence` lane, bounded reads, coverage-aware evidence and delivery into the answering prompt. Those are real improvements. This note does not relabel that implementation as absent; it identifies the early lane outside its witness boundary.

The current implementation is also distinct from [perception classification outcomes and recovery][classification-prior]. That audit handles malformed answers and failed model calls. Here the envelope is structurally admitted and its action vocabulary is known; two meaning-bearing fields disagree about whether answering requires observation.

## Implemented mechanism

The ordinary chat path invokes the perception firewall where available and otherwise the direct transducer. The returned intent is published, routing signals are asserted, and the kernel chooses the route. Failed classifications have their own stop before the selected lane acts. [Chat perception and routing][process]

Understanding conversion independently carries the action-derived verb, semantic-derived category, surface response, scope level, scope file and requested tools into `Intent`. It does not delete evidence fields when the action maps to a conversational verb. [Projection][adapter]

`decideRoute` emits `/scope_is_file` for a nonempty `ScopeFile` or file/line scope, `/suggests_tools` for a nonempty tool name, and `/has_surface_response` for a nonempty response. The candidate specialist comes from the verb corpus or an explicit perception shard hint. [Signal producer][signals] [Candidate resolution][candidate]

The policy derives `needs_workspace_evidence()` from either evidence signal. Both ordinary `wants_direct_answer()` rules include its negation. In contrast, either `perception_answer_lane()` rule needs only a surface response, an allowed verb or verb/target pair, and `/none` candidate. Neither tests the evidence predicate. The early lane then excludes delegation and evidence gathering. [Evidence and early rules][rules] [Evidence fallback][gather-rule]

When the selected route is `RoutePerceptionAnswer`, `processInput` returns the perception surface with an optional collected system summary. It does not proceed to its later information-gathering and evidence-bearing articulation request. This is not a claim that no evidence exists anywhere in the running system: the missing contract is admission and delivery sufficient for this answer. [Terminal early return][return]

## Source-derived witness

Use a normal admitted Understanding envelope with this combination of fields:

```json
{
  "understanding": {
    "primary_intent": "discuss",
    "semantic_type": "question",
    "action_type": "chat",
    "domain": "general",
    "confidence": 0.9,
    "scope": {
      "level": "file",
      "target": "README.md",
      "file": "README.md"
    },
    "suggested_approach": {
      "tools_needed": ["read_file"]
    }
  },
  "surface_response": "Here is my answer about that file."
}
```

This is an illustrative fixture, not a recommended classification. It deliberately carries a contradictory interpretation: conversational action plus explicit evidence need. Structural admission requires action and confidence, checks supported JSON field types and confidence range, and normalizes fields; it does not reject this cross-field combination. `chat` maps to `/converse`, whose taxonomy candidate is `/none`. The action/domain shard derivation has no affinity for `chat`/`general`; with no suggested shard it does not supply the candidate that would block this witness. The firewall normalizes and records the interpretation without erasing these evidence fields or rewriting its conversational surface. [Classification admission][admission] [Action mapping][adapter-full] [Taxonomy][taxonomy] [Shard derivation][derive-shards] [Affinity policy][affinity] [Firewall][firewall]

Assume no explicit shard hint, no held approval, no separate overriding self-diagnosis lane, and no special state that introduces another route. The derivation is:

1. The projection produces a `/query /converse` intent with a nonempty surface and file/tool signals.
2. `needs_workspace_evidence()` is true; ordinary direct answering is correctly vetoed.
3. `/converse` is a perception-answer verb, a response exists, and the candidate is `/none`; `perception_answer_lane()` is true anyway.
4. That lane is early, so the evidence fallback is excluded.
5. Chat returns the classifier's draft rather than collecting the required observation and obtaining an evidence-bearing answer.

The host does not have to manufacture an invalid verb to reach this shape. Nevertheless, this review does **not** establish how often a real model emits it. The question for a deterministic executive is whether it admits an unsupported answer when an accepted structured proposal itself says evidence is needed.

This is not proof that the answer must be factually wrong. Prior history or strategic context could already contain useful material, or the model could express uncertainty. The defect is that this early admission does not establish source sufficiency, currentness or coverage. Conversely, independent system-shard work is not proof that its result reached the already-generated response. The observation-to-answer handoff is the boundary to verify.

## Counterexamples and protections to preserve

- A non-`/none` candidate blocks the cited perception-answer rules. Legitimate specialist routing should remain available.
- Empty surface response blocks this fast lane.
- Ordinary `/analyze` evidence requests, and `/explain` requests outside its special early targets, can reach the implemented `gather_evidence` route. This finding does not invalidate those repaired journeys.
- A genuinely conversational message without evidence signals should still return perception's reply without an extra articulation call.
- Failed classifications are separately stopped. This proposal must not convert failures into actionable evidence requests.
- Evidence acquisition remains bounded and workspace-contained. The proposal grants no new write permission, general shell access, or broad tool envelope.
- A `/read` fixture with empty target and nonempty scope file is a poor primary witness: the adapter fills the target from the file, which can defeat the special empty-target early rule. The canonical `chat` to `/converse` route avoids that ambiguity.

Existing [evidence tests][tests] cover no candidate, low confidence, tools without a file, confident delegation, mutation exclusion, and a plain prose question. Their main fixtures use `/analyze` or `/explain` with a non-special target. Those tests support the implemented repair but do not establish the missing early-lane guard.

## Proposed policy contract

### Option A: conservative evidence veto

Require the absence of `needs_workspace_evidence()` for both perception-answer rules. This is the smallest conceptual change and belongs in Mangle, not a new Go routing override. The contradictory fixture then continues through existing evidence-oriented routing. A file is read under existing bounds; a tools-only request without a concrete file receives the existing explicit unavailable-evidence disposition rather than an invented claim of investigation.

The extra cost occurs only on turns previously terminated early despite evidence signals. This option can over-read if classification names a file incidentally or if current sufficient evidence is already present. That is a measurable trade-off, not a reason to pretend the signals do not exist.

### Option B: explicit evidence-satisfied fast path

Retain fast answering when the executive can establish that supplied evidence already satisfies the question. This requires a stronger contract than merely testing for a file fact: source identity, revision, coverage and delivery to the actual classification request must agree. A whole-file question cannot be discharged by a prefix, and an observation in the kernel cannot discharge an obligation if its body was not delivered to the answering model.

Do not add this machinery speculatively to repair the immediate omission. Start with the conservative veto unless current consumers already supply the necessary receipt. If implemented later, the sufficiency rule should explicitly name which evidence obligation is satisfied; arbitrary natural-language correctness remains unproven.

### Option C: resolve the contradiction by clarification

For ambiguous proposals, derive an explicit clarification or qualified answer rather than automatically reading. This may be appropriate when file scope looks incidental or the necessary source is unspecified. It adds user interaction and potentially another classification turn. It should be a declared policy alternative with an evaluation, not an accidental fallback to the original draft.

All options should preserve the architectural split: the model proposes meaning; the kernel admits the route and its obligations; Go performs bounded observation and carries receipts. There is no reason to delegate the contradiction test itself to a stronger model.

## Full costs and failure modes

The relevant cost is the cost to reach a useful grounded answer, not the number of calls in the first turn. A conservative veto adds a read, kernel/routing work, evidence tokens and possibly articulation. It may avoid a user correction, repeated investigation, or a confidently unsupported answer. Neither direction has been measured here.

Measure classification and articulation input/output tokens separately, provider retries, read bytes, evidence retention, total elapsed time, tool calls and follow-up turns. Include cache effects: an extra evidence-bearing request may reuse a stable prefix, but the new evidence and output still cost tokens. An evidence-satisfied fast path adds receipt creation, revision checks and invalidation costs; it may save observation only when reuse is valid.

Failure cases include spurious file signals causing unnecessary reads; partial files being treated as sufficient; stale cached bodies; tools-only signals with no resolvable source; unavailable files; route contradictions; and a guard inserted in only one of the two early rules. None justify silently treating a draft as evidence. Return an explicit bounded/partial/unavailable disposition when the obligation cannot be discharged.

## Fair evaluation and checkable exit

No evaluation below has been run. Use a scripted provider at the normal production entry and the real policy kernel, with source sentinels and recorded actual model requests. A helper-only route assertion is necessary but insufficient.

| Case | Required observation |
|---|---|
| `chat` plus file scope, tools and nonempty surface, no shard | Draft is not accepted as final without the evidence obligation's disposition |
| Same fixture, no file/tool signal | Cheap conversational answer remains available |
| Tools signal only, no resolvable file | Explicit unavailable/clarify disposition; no claimed read |
| Candidate specialist supplied | Existing authorized specialist route survives |
| Empty surface | Existing non-fast route behavior survives |
| File absent or outside workspace | Existing containment and unavailable result survive |
| Large file with sentinel beyond first region | Prefix is labelled partial; no claim of full read |
| File changed after prior observation | Old body cannot silently discharge current evidence need |
| Current evidence already supplied | Conservative option's extra work is counted; any reuse option proves actual delivery and coverage |
| Both early-rule forms | Guard parity is established rather than fixing only conversational verbs |
| Failed classification or held approval | Existing failure/approval precedence remains intact |

Compare unchanged baseline, conservative veto, and any explicit reuse design on identical scripted traces first, then on a held-out mix of real conversations if authorized separately. Report unnecessary escalations alongside missing-evidence answers. Do not call “one fewer call” a win if the turn leaves the user's question unanswered.

**Exit:** every admitted file/tool evidence requirement either reaches sufficient evidence delivered to the answering request, receives a truthful partial/unavailable disposition, or is explicitly superseded by a justified policy decision. A nonempty perception draft alone never discharges it. Ordinary evidence-free conversation preserves its fast path.

## Sources

[prior]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/Docs/audits/tui-evidence-turn-ownership.md
[classification-prior]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/Docs/audits/perception-classification-outcomes-and-recovery.md
[process]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/process.go#L190-L360
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/perception/understanding_adapter.go#L494-L545
[adapter-full]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/perception/understanding_adapter.go
[signals]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/delegation_routing.go#L193-L215
[candidate]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/delegation_routing.go#L314-L345
[rules]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/defaults/policy/routing_arbitration.mg#L112-L210
[gather-rule]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/defaults/policy/routing_arbitration.mg#L289-L310
[return]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/process.go#L356-L360
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/perception/classification_admission.go
[taxonomy]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/perception/taxonomy.go
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/turn_evidence_test.go
[derive-shards]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/perception/transducer_llm.go#L824-L879
[affinity]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/defaults/schema/intent_routing.mg#L268-L316
[firewall]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/shards/system/perception.go#L514-L651
