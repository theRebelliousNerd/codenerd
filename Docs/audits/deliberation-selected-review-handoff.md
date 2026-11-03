---
doc-class: deep-dive
subsystem: deliberation-session
implementation-status: proposed-refinement
audit-status: source-grounded-conditional
audit-date: 2026-11-03
verified-against: 8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86
verification-method: read-only-source-inspection
---

# Carry the selected challenge through deliberation handoff

## Finding and decision

**A ready design can lose all of its important challenger findings at the point where the user or execution worker receives it.** The conductor correctly accepts important, nonblocking items and saves them with the selected round. Its final renderer then emits only the proposer's brief, assumptions, success test and proof identifier. The chat adoption path likewise copies only proposer fields into the resumed request. It does not carry the selected assessment, an important-item count, or a reference identifying the saved assessment. [Selected result][finish] [Final rendering][note] [Chat adoption][adopt]

This contradicts the session specification's stated intent that important items remain visible without blocking readiness. The important/critical distinction is useful and should remain: readiness should not imply that the challenger had no remaining concerns. The recommended correction is a bounded, candidate-bound review handoff, using the existing saved round. It does not require another agent, another model call, a new planner, or promotion of every important improvement into a blocking requirement. [Session contract][spec]

The concrete source finding is an omission in the final design surface and the ordinary chat handoff. It is **not** evidence of a defective downstream code change, an authorization bypass, a failed gate accepted as passing, or proof that a worker cannot rediscover the concern. The complete assessment survives in the saved thread. Recurse mission execution also carries a receipt containing the thread path, which is a real recovery route and limits the mission claim. [Persistence][save] [Mission reference][mission-types] [Mission worker input][mission-worker]

### Source pin and boundaries

The initial public main observation resolved to f2990adf9184fccbb233c803620ce1a2235b4ed8, committed at 09:38 UTC on November 3, 2026. The final check resolved to **8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86**, committed at 09:43:36 UTC. The complete comparison lists only Docs/audits/ASTRA_QUEUE.md as changed; the final queue was reread. All implementation and architecture files cited below are unchanged across that comparison. Links use the final pin. [Initial pin][initial-pin] [Final pin][pin] [Comparison][compare]

This audit inspected public repository guidance, architecture, implementation, relevant existing audits and authored tests. No repository code, test, build, model inference, benchmark, gate or failure injection was executed. No repository file, credential, configuration or setting was changed. Only scratch source copies, this Markdown deliverable and its source manifest were written. Every witness and evaluation below is proposed and unrun.

## 1. Why this boundary matters to Socratic design

The existing architecture already supplies a proposer and a requirements interrogator. The proposer develops concrete alternatives and states assumptions; the interrogator evaluates each against the request and separates critical blockers from important improvements. The model supplies creative and critical judgments, while Mangle decides eligibility, selection and stopping. This audit preserves that division rather than suggesting that another round of agreement establishes truth. [Root contract][guidance] [Proposer][proposer] [Interrogator][challenger]

A useful design result contains two related products:

1. The selected proposal: what to do, where, under which assumptions, and what success would mean.
2. The selected review: what the challenger still considers important about that exact proposal, with its original nonblocking status.

The second is not interchangeable with the first. The challenger runs after the proposer has finished. A newly identified important concern need not already appear in the candidate's assumptions or brief. When the candidate is ready, the service returns immediately rather than giving the proposer another turn to copy the concern. Copying only candidate fields therefore cannot reliably preserve the critique. [Round order][round] [Ready return][service]

The precise architectural requirement proposed here is **review visibility at a terminal handoff**, not compulsory implementation of advisory suggestions. A user may decide to defer an optional improvement. An execution worker may keep it outside the authorized change scope. Neither should have to infer that a hidden critique exists from a bare readiness label.

## 2. What is already implemented

| Boundary | Current source behavior | Consequence for this finding |
|---|---|---|
| Role contract | The challenger distinguishes critical blockers from important improvements. | A successful review may legitimately retain important items. |
| Typed result | Each item has a key, severity, kind, detail and optional probe, associated with a candidate assessment. | The information exists in a structured, candidate-bound form. |
| Validation | Every candidate has exactly one assessment; item keys are unique within it; severity and kind use closed vocabularies. | An ordinary important design item is valid output. |
| Readiness | A candidate needs a brief, citation, success and no critical item. | Important items do not prevent readiness, intentionally. |
| Persistence | The full round includes the proposal and challenge; the thread snapshot is saved before and during finish. | This is not universal loss or absence of a durable record. |
| Result rendering | The note reads the candidate and one proof identifier, not assessment items. | The user-facing design can omit all important concerns. |
| Chat adoption | The request keeps original intent and adds chosen brief, success test and assumptions. | The resumed task has no explicit selected review handoff. |
| Mission adoption | The mission stores the candidate, thread ID and thread path; the task input receives the mission receipt. | The saved critique can be recovered through that reference; direct visibility is still different from guaranteed consumption. |

Sources: [Role contract][challenger], [record types][types], [validation][validation], [readiness policy][policy], [service][service], [snapshot writer][save], [renderer][note], [chat][adopt], [mission receipt][mission-types], [mission input][mission-worker].

The readiness citation check tests the presence of a supplied citation fact, not its truth or relevance. That is an existing, separate grounding question. This report does not strengthen its claim by assuming that a recorded policy proof verifies the natural-language critique, or that every important item is a correct observation.

## 3. Exact source chain

### 3.1 A ready result can contain an important finding

The requirements-interrogator atom explicitly asks for separate critical and important items. Its definition of success concerns the proposed reading, not a claim that an implementation or an unrun test has completed. The Item and Assessment types preserve the distinction and associate the item with CandidateID. Validation accepts severity "important" and kind "design"; no probe is required for such an item. [Interrogator][challenger] [Types][types] [Validation][validation]

The ready eligibility rule joins the candidate, brief, citation and success facts, then requires absence of a critical item. The helper defining critical items matches only severity /critical. Consequently, a successful cited candidate with an important design concern can correctly be selected as ready. This is desired policy behavior, not the defect. [Eligibility][policy]

### 3.2 The completed review is retained, then the conductor stops

The collector calls the proposer, passes its candidates to the challenger and validates the resulting round. Service.Run asserts that round, obtains its policy decision, records the selected candidate and proof, appends the whole round to the thread, and saves it. For a ready verdict it immediately calls finish. [Collector][round] [Service][service]

Finish constructs an Outcome containing the original thread and selected candidate. It marks the thread ready, records the chosen ID, saves again, and emits out.Note(). Nothing here deletes the assessment. The renderer has access to the selected round and can join its assessment without asking another model to recreate it. [Finish][finish]

### 3.3 The final surface drops the assessment

Outcome.Note reads the candidate's Brief, Assumptions and SuccessTest. For an assuming disposition it adds the yolo preference disclosure. From the last round it extracts only the first recorded proof ID. It never reads Challenge.Assessments or Item.Detail and does not indicate that any important item remains. [Renderer][note]

The direct /deliberate and /innovate handler returns an assistant surface containing that note. The design-only values continuation does the same after successful resolution. This supplies a normal production route to the omission; no unusual worker integration, corrupted state or custom internal caller is necessary. [Command and continuation][commands]

The live ready-round message identifies the round, verdict and chosen candidate, and finish emits the same note. Those specific progress messages do not carry the omitted detail either. This does not establish that every diagnostic trace or tool UI hides it. [Service progress][service] [Finish progress][finish]

### 3.4 The ordinary chat execution request also drops it

adoptDeliberation keeps the original intent and replaces its target with the selected candidate's target. It appends the chosen brief, success test and joined assumptions to the original constraint, clears the prior response, and returns the original request plus that constraint. It saves the same note for later delivery. It does not add the selected assessment, item count, thread ID or thread path to this handoff. [Adoption][adopt]

The normal RouteDeliberate branch calls that adapter when the conductor returns ready, then continues ordinary routing. A resumed non-design-only values thread likewise calls the adapter before processInput. Thus changing the selected challenger's important-item detail while holding the original request, candidate and other inputs fixed does not change these explicit handoff fields. [Ordinary route][route] [Continuation][commands]

The original request may independently contain the same concern, or a worker may discover it through its own evidence gathering. Those are possible alternate sources, not preservation of the actual review output. The narrowly demonstrated omission is from this task construction and final design surface.

### 3.5 A proof identifier does not substitute for the missing detail

assertRound projects an item into thread, round, candidate, key, severity and kind facts; it does not project Item.Detail. A selected-ready proof also does not need an important item as a positive readiness prerequisite. The final note prints only a proof identifier, not a review body or a resolvable assessment reference. It cannot be assumed to disclose the omitted natural-language concern. [Fact projection][projection] [Policy][policy] [Note][note]

This does not invalidate the policy proof. The proof can correctly establish readiness under a rule that permits important items. The failure is treating that disposition's presentation as sufficient delivery of the review.

## 4. Small unrun witness

Use the normal first divergent round with two valid candidates, a stable workspace, successful persistence, normal proof support, no consultation, no probe and ample context.

- Candidate c1 has a nonblank brief and success test, an appropriate source citation and a successful assessment. Its only item is key "cache-observability", severity "important", kind "design".
- The item's detail is: "Core cache behavior can proceed. Capacity evictions are not distinguished from cold-start misses in the proposed diagnostics, which will make large-workspace pressure harder to investigate."
- Candidate c2 is a valid alternative with success false and a critical design blocker. Recommend c1.
- Keep the c1 brief and assumptions free of the diagnostic concern. This is natural because the challenger identifies it after the proposal.

The source-derived path is:

1. c1 is eligible and selected as ready; c2 does not compete as ready or empirical.
2. The complete c1 assessment, including its important item, is saved in the thread.
3. /deliberate returns the c1 brief, assumptions, success test and proof identifier, without the diagnostic concern or a remaining-item notice.
4. For a separately authorized ordinary chat request, adoptDeliberation constructs a worker request without that concern or a selected-review reference.

The case does not depend on misclassifying a mandatory requirement as optional. The item is deliberately a nonblocking assurance improvement. An item that truly blocks the user's explicit acceptance conditions should be classified and handled as critical through the existing path.

A differential witness can change only Item.Detail to an unrelated important concern while preserving candidate, key, kind, severity and the rest of the round. The saved review changes, but the explicit candidate-based rendering and chat task projection do not read that changed field. Opaque proof metadata may vary between independent runs; its mere variation is not delivery of the concern. No executed comparison is claimed.

### Controls and counterexamples

- **No remaining item:** the compact result should remain compact; do not fabricate a warning.
- **Critical design item:** the ordinary-ready rule should still refuse the candidate. This report does not show a critical blocker passing that rule.
- **Concern already repeated in assumptions:** the current renderer carries that duplicate. The primary witness places the new observation only in the proper challenger field.
- **Unselected candidate has an important item:** do not attach its critique to the winner. Stable candidate association is essential.
- **Saved-thread inspection:** loading the JSON recovers the critique. Persistence is working at the inspected boundary and must not be described as absent.
- **Mission task:** its receipt supplies ThreadPath. A worker that reads the saved round may recover the critique. This report does not establish that every mission worker misses it.
- **Permission hold:** preserving review content must not alter the original verb, restriction or existing propose/explicit work-admission behavior.
- **Values continuation:** test an actually offered selected option, not the separately audited unoffered-option case.
- **Truncation, unavailable files and source drift:** exclude these from the minimal witness, then evaluate them separately.

## 5. Recurse and portability: the measured boundary, not a universal claim

Mission preparation runs this same deliberator and stores the chosen candidate in MissionReceipt.Design. It also records the child thread's identity and path. RecurseAttemptCampaign carries that receipt into the campaign. The unified task-spawn path serializes the complete mission receipt into the execution input, including later refinement and remediation calls. This is meaningful counterevidence against claiming that mission workers have no way to inspect the selected challenge. [Mission preparation][mission-prepare] [Attempt bridge][attempt] [Worker bridge][mission-worker]

The remaining design question is whether a reference alone fulfills "important items remain visible." This pass does not establish a general host-enforced dereference or a guarantee that a model consumes that thread before working. It also does not establish the contrary for every downstream tool or context path. The actionable confirmed gap remains the direct design surface and ordinary chat projection; mission is a required parity check for the proposed shared handoff.

The issue is language-independent. It occurs before Go, Python, JavaScript/TypeScript or Rust verification adapters execute. Adding a language-specific gate cannot recreate an important observation that was omitted from a design reply. Conversely, carrying an item does not prove the foreign-language gate supports it. Keep the portable-crucible proposal's separation between proposed requirements, observed evidence, adapter capability and acceptance. [Portable direction][portable]

This is a small strengthening of the existing Socratic funnel: preserve the result of interrogation across its final boundary. It is not a proposal to rewrite recurse scheduling, introduce another orchestration framework or use advisory critique as independent permission.

## 6. Proposed contract, explicitly not implemented

Update the session and shard contracts before implementation. Use the selected persisted assessment as the source of truth for this handoff, rather than asking the proposer to repeat or summarize it.

A bounded **selected-review view** should establish:

| Field or relation | Required meaning |
|---|---|
| Origin | Thread, round and selected candidate identity; no merging with rejected alternatives. |
| Review | Original item key, severity, kind and detail, explicitly labeled as challenger assessment. |
| Disposition | Important items remain nonblocking; inclusion alone does not authorize new work. |
| Completeness | Total and included items, plus an explicit omitted/unavailable state if the full review is not inline. |
| Recovery | A valid, scoped reference to the retained selected review when needed; not merely an opaque digest. |
| Currency | The assessment belongs to the selected design version/round. A later edit must not silently make an old concern current or resolved. |
| Surface | The final user result identifies significant remaining considerations, or clearly says a bounded review is available. |
| Execution input | Normal chat and mission consumers receive the same selected-review identity and availability semantics. |

The existing Thread already owns the complete review. The minimal repair can derive a bounded view without duplicating every round or adding another durable log. If a versioned record is introduced, define how older threads lacking the new projection are handled: derive it from their retained assessment where possible; otherwise report review availability as unknown rather than inventing an empty result.

Retain Mangle's authority over readiness and stopping. A host renderer can transport the already-selected assessment without making a new readiness decision in Go. Do not turn "important" into "must implement now" or infer that a suggestion changes the user-authorized scope. If an item exposes a genuine blocking contradiction, re-enter the existing review/admission path rather than quietly upgrading its severity during rendering.

Bound both serialized content and item cardinality. A fixed top-N warning list without an omission count would reproduce the defect for later items. Prefer a short visible remainder notice plus a genuinely resolvable record over an unbounded critique transcript. Do not cut a qualification in a way that reverses its meaning, and do not promote arbitrary model-provided references into trusted paths.

This proposal intentionally leaves the wider question of requirement-to-executable-check mapping to the portable-crucible work. Preserving an important diagnostic concern is not a receipt that the diagnostics exist or that their tests ran.

## 7. Failure modes the correction must not introduce

1. **Wrong candidate or round:** carrying every assessment can contaminate the chosen design with a rejected alternative's objection. Join exactly.
2. **Accidental obligation expansion:** optional advice becomes unrequested implementation work. Preserve severity and original authority.
3. **Old review presented as current:** a changed candidate retains stale findings. Bind review identity to the chosen round/design and disclose later state.
4. **Silent clipping:** the first few concerns survive but later ones disappear. Carry counts, availability and recovery semantics.
5. **Broken reference disguised as coverage:** a thread path exists in a receipt but is unavailable to the consumer. Distinguish retained, accessible and consumed.
6. **Model claim becomes host truth:** preserving critique is not verification of it. Keep the assessor and evidence status explicit.
7. **Prompt contamination:** an item's detail contains instructions. Treat it as review data under existing restrictions, never authority.
8. **Repeat pressure:** every remediation turn re-injects a long already-resolved concern. Prefer the current selected review and explicit disposition over growing history.
9. **Terminal-state regression:** adding review rendering must not turn a ready design into repeated model refinement simply to obtain prose that already exists.
10. **Permission regression:** selection, visibility and an answer to a values question still do not approve mutation.

## 8. Proposed evaluation, all unrun

### A. Deterministic transport and selection

Use the existing scripted role seams, real conductor/policy path and production chat entry points. Capture the result and the actual worker request rather than testing only a proposed helper.

- Run the witness above through /deliberate and require the selected important concern to remain visible or explicitly recoverable.
- Exercise normal RouteDeliberate adoption and the saved-values continuation. Check both the displayed note and captured execution request.
- Verify c1/c2 and round identity, including multiple assessments with identical item keys but different details.
- Change only the selected Item.Detail. Require a corresponding review-data difference at the consumer boundary.
- Put the concern only on an unselected candidate. Require absence from the selected-review view.
- Save and reload the ready thread; confirm exact review reconstruction and legacy behavior.
- Include no items, several important items, all kinds, critical controls, malformed assessment coverage and unavailable proof/persistence.
- Exercise multibyte text, JSON escaping, large details and long lists against actual serialized bounds.
- Preserve the original intent and permission hold. A transport correction must not invoke an unauthorized worker.
- Exercise the mission receipt and unified spawnTask route. Test actual reference resolution when used, not just presence of ThreadPath in JSON.

Current authored tests check assumptions, proof display, original request/brief propagation, persistence and permission holds. The inspected fixtures do not assert that a selected important item reaches the final surface and captured worker request. They were read, not run; this is a scoped test-gap observation, not a claim that the whole repository lacks related coverage. [Note fixture][note-test] [Chat fixture][chat-test]

### B. Behavioral value under authorized later evaluation

A deterministic preservation test can establish delivery, not usefulness. For a later authorized live evaluation, compare:

- A: current selected-candidate-only handoff;
- B: bounded selected-review handoff using exactly the same saved proposals and assessments;
- C: an explicit bounded-reference variant, if proposed, with measured retrieval behavior.

Keep model/provider version, task scope, permissions, acceptance conditions and budgets fixed. Pair tasks so the source review is held constant when comparing transport. Separately test end-to-end naturally produced reviews to measure whether relevant items are actually emitted; do not mix review-generation changes into the transport effect.

Use held-out tasks with benign but material nonblocking concerns, irrelevant critique controls, already-addressed concerns, cross-language targets and resource-tight cases. Independent reviewers or predefined checks should judge whether the user-visible design preserves the concern accurately and whether any executed change stays within authority. Report per-family and aggregate results, not only successful examples.

Measure false omission, wrong-candidate leakage, spurious obligations, availability/consumption failures, unsupported assertions of resolution, added work, correctness and completion. Include stopped, failed and deferred cases in denominators. Compare both equal-budget quality and full cost to reach the same independently accepted endpoint. No improvement percentage is established here.

## 9. Full costs and tradeoffs

The minimal projection needs no additional model inference and no extra deliberation round: the assessment is already present. That does not make it free.

- **Host work:** locating the selected assessment and rendering its items costs work proportional to the inspected assessment/item data; encoding and any digest calculation cost time proportional to serialized bytes. Define bounds instead of assuming the existing candidate cap bounds item count or detail length.
- **Context:** inline critique adds input tokens to the worker and visible output to the user. Repeated task/remediation handoffs can repeat that cost. Token counts depend on the tokenizer and provider; bytes are not a portable token count.
- **Storage:** the current thread already retains these details. A new duplicated projection adds bytes and copy cost; a derived view avoids some duplication but adds lookup/read costs. Neither requires retaining an entire raw transcript.
- **References:** a compact reference adds read latency, possible tool/model turns, file-retention and access checks, and missing/stale-record failure handling. Measure whether it is actually resolved.
- **Downstream effort:** visible optional concerns can produce useful targeted work or distract the worker into unnecessary changes. The scope/authority contract and irrelevant-critique controls are therefore part of quality, not secondary considerations.
- **Operations:** schema compatibility, tests, renderer parity and maintenance across chat and mission paths have engineering costs. An additional full review call merely to restore omitted text would also add latency, spend and stochastic drift; it should not be the default remedy.

Report total model calls, input/output tokens, cache charges where visible, estimated and recorded spend, tool calls, read bytes, serialized/persisted bytes, host CPU/allocations, latency, user questions and unauthorized-scope attempts. Include review generation and execution/verification, not just the added handoff bytes. Deduplicate at the record/identity level only when meanings and destinations remain distinct.

## 10. Primary literature and limits of the analogy

[Self-Refine][self-refine] studies iterative generation, feedback and refinement using the same model. It motivates evaluating whether available critique reaches the stage that can use it. Its task results do not establish that codeNERD's challenger is correct, that a particular critique should block readiness, or that this handoff change improves outcomes.

[Reflexion][reflexion] uses feedback-derived reflective text in episodic memory for later trials and studies different feedback and incorporation methods. The relevant design inference is modest: feedback retention and its consumption are distinct mechanisms that should be measured. An archived thread or proof identifier alone is not evidence of consumption.

[Huang et al.][self-correction] examine intrinsic self-correction without external feedback and report limitations on the studied reasoning tasks. This is a reason to avoid claiming that more critique or repeated agreement guarantees correctness. It does not refute the narrower deterministic contract of accurately transporting an already-produced review.

These are primary research sources, not evidence that the current repository path has been exercised. This audit imports no benchmark score or reliability claim into codeNERD.

## 11. Nonduplication and exit criterion

The bounded duplicate screen read the current audit inventory and queue, all existing deliberation reports, the portable-crucible proposal and neighboring review/learning/handoff reports. Targeted public code searches included "important deliberation", "important items", "ThreadPath" and the intersection of deliberation, important and handoff. Search results were indexed at e2705a0e8f77607af1ae26b11f767949e842a914; decisive files were fetched directly at the audit pin. This is not an exhaustive claim about private discussion or every historical branch.

- **Consultation qualifier continuity** loses specialist caveats before the challenger receives them. Here no consultation is needed: the challenger has already produced and persisted a correct typed item. The loss is later, at final rendering and chat adoption. [Prior qualifier audit][qualifier-audit]
- **Read invocation identity** loses tool arguments in an evidence observation. This witness requires no tool read or ambiguous observation. [Read audit][read-audit]
- **Values option admission and evidence freshness** concern which answer can proceed and whether saved support remains current. This case can finish in the first uninterrupted round with ordinary readiness. [Option audit][option-audit] [Freshness audit][freshness-audit]
- **Terminal-budget reports** concern the ability to produce an answer within the byte bound. Here a valid complete answer already exists and context is ample. [Budget audit][budget-audit]
- **Portable recurse deliberation/crucible** proposes broader acceptance and evidence contracts. This report supplies a smaller current implementation gap in the existing terminal handoff. [Portable proposal][portable]
- **Review feedback identity, expert critique coverage and campaign learning fidelity** concern other producers, aggregation or durable learning. The selected deliberation assessment and its final consumers are distinct. [Review audit][review-audit] [Expert audit][expert-audit] [Learning audit][learning-audit]

**Exit criterion:** for every terminal selected design with important items, the user-facing result and the authorized consuming task receive a bounded, correctly associated review or an explicit availability/omission state with a working recovery route. No item from a rejected candidate is silently attached, no important item is silently promoted to mandatory work, and no readiness or permission rule is weakened. Establish that through the normal chat and mission entry paths before claiming completion.

The present source supports the omission finding and identifies the existing data needed to repair it. Runtime incidence, behavioral benefit, retrieval reliability and full cost remain unmeasured.

[initial-pin]: https://github.com/theRebelliousNerd/codenerd/commit/f2990adf9184fccbb233c803620ce1a2235b4ed8
[pin]: https://github.com/theRebelliousNerd/codenerd/commit/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86
[compare]: https://github.com/theRebelliousNerd/codenerd/compare/f2990adf9184fccbb233c803620ce1a2235b4ed8...8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/agents.md#L1-L27
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L895
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/prompt/atoms/system/deliberation_proposer.yaml#L14-L53
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L52
[types]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/types.go#L17-L69
[round]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/round.go#L10-L72
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/core/defaults/policy/deliberation.mg#L10-L58
[service]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/service.go#L121-L165
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/outcome.go#L16-L74
[note]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/outcome.go#L89-L115
[save]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/persistence.go#L23-L74
[adopt]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/cmd/nerd/chat/deliberation.go#L58-L68
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/cmd/nerd/chat/deliberation.go#L70-L119
[route]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/cmd/nerd/chat/process.go#L363-L382
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/policy.go#L10-L49
[mission-types]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/campaign/recurse_mission.go#L69-L97
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/campaign/recurse_mission.go#L243-L322
[mission-worker]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/campaign/orchestrator_task_handlers.go#L31-L63
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/campaign/recurse_attempt.go#L55-L81
[note-test]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/deliberation_test.go#L180-L192
[chat-test]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/cmd/nerd/chat/deliberation_test.go#L142-L219
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/portable-recurse-deliberation-crucible.md
[qualifier-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/deliberation-consultation-qualifier-continuity.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/deliberation-read-invocation-identity.md
[option-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/deliberation-values-option-admission.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/deliberation-values-evidence-freshness.md
[budget-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/deliberation-terminal-budget-continuation.md
[review-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/review-feedback-identity-and-current-verdicts.md
[expert-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/expert-critique-domain-coverage.md
[learning-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/Docs/audits/campaign-learning-receipt-fidelity.md
[self-refine]: https://arxiv.org/abs/2303.17651
[reflexion]: https://arxiv.org/abs/2303.11366
[self-correction]: https://arxiv.org/abs/2310.01798


[validation]: https://github.com/theRebelliousNerd/codenerd/blob/8a06c5c89e89e3817552a3b2ebb3aeb0c626ae86/internal/deliberation/types.go#L144-L181
