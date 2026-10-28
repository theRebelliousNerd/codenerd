---
doc-class: deep-dive
subsystem: cli
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
supersedes: []
---

# Preserve grounding through terminal knowledge synthesis

The normal chat turn now has a useful evidence-delivery repair, but the final answer after specialist consultation does not inherit it. Initial articulation has a JIT compilation path and explicit carriers for conversation context and typed workspace observations. JIT and brief delivery remain conditional on a successful compile and selection/injection limits; persona-only fallback is possible. If that response asks for specialist knowledge, the terminal synthesis request instead receives the original input string and successful specialist prose under the architect persona plus one hardcoded instruction. It does not receive the first request's evidence, coverage warnings, resolved conversation context or compiled brief.

**Recommendation:** make terminal knowledge synthesis a distinct, JIT-governed answer phase with an immutable parent answer context and a complete consultation outcome ledger. Preserve the existing one-follow-up-call design and exactly-once application of first-call controls. Do not repair this by re-entering perception/routing, blindly replaying the entire transcript, or treating specialist prose as a substitute for host observations.

This is a source-grounded refinement proposal, not an executed test report. No tests, implementation, provider calls or performance measurements were performed. The feature branch was inspected initially at ea3145e and requalified against [c2d39e5](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c); the intervening comparison changes no synthesis code. The synthesis function is text-identical to main at 356f65e. Main and feature therefore share the terminal handoff gap, while the typed residual evidence repair discussed below is specifically credited to the feature.

## Scope and existing work

The [root contract](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md) assigns executive context decisions to Mangle and requires JIT-first model-facing behavior. [Prompt guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/agents.md) also distinguishes saved/retrieved evidence from evidence actually consumed by a request. Those are the relevant criteria: a successful first request and a successful specialist are not proof of grounding in the request that writes the final answer.

This document does not refile these adjacent findings:

- [TUI evidence turn ownership](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/tui-evidence-turn-ownership.md) owns residual result correlation, bounded local reads, coverage and routing disposition. Its current as-built appendix credits the new evidence carrier and explicitly leaves late results and paged revision handling open. This proposal begins after that repaired first articulation call.
- [Consultation coordination and safe reuse](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/consultation-coordination-and-safe-reuse.md) owns the shared consultation manager, campaign/advisory receipts and partial outcomes. Main chat's model-authored `knowledge_requests` route uses its own direct `spawnTaskWithContext` loop in `process_knowledge.go`; the terminal projection below is not the campaign advisory-board path.
- [JIT generation-aware optimization](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/jit-generation-aware-optimization.md) owns source generations, reusable compilation snapshots and cache publication. A correct compiler cannot protect a request that never calls it.
- The [CLI control-before-knowledge contract](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/cli/03-GAP-ANALYSIS.md#required-contract-control-packet-application-with-knowledge-requests) is correctly implemented: memory/observation controls are applied before consultation and recorded without replaying their effects. Preserving an effect is different from carrying the evidence and obligations needed by the later answer.

## What the production path actually carries

| Boundary | Source evidence | Current behavior |
|---|---|---|
| Initial host evidence | [process.go 842–905](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L842-L905) | Residual and gather-evidence outcomes are attached to `ConversationContext.Evidence`. The same context includes recent history, last shard result, shard history and compressed context. |
| Initial prompt construction | [persona.go 487–541](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/persona.go#L487-L541), [helpers_articulation.go 237–270](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_articulation.go#L237-L270) | Main articulation attempts JIT, with a persona-only fallback on failure or unavailable budget; it separately renders available workspace evidence, explicit coverage and warnings in the request. |
| Brief delivery | [brief.mg 32–43](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/brief.mg#L32-L43), [compiler_kernel.go 225–276](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/compiler_kernel.go#L225-L276) | Brief goals, constraints, decisions and questions derive injectable context. The compiler can admit matching rows within its injection limits and successful compilation; not every brief row is unconditionally delivered. This is not an automatic property of every LLM client call. |
| Control settlement | [process.go 984–1023](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L984-L1023) | First-call controls are applied and recorded before branching to knowledge gathering. The call to gathering passes requests, raw input and rendered interim response—not `convCtx` or `systemPrompt`. |
| Consultation execution | [process_knowledge.go 63–99](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L63-L99) | Each specialist gets its query, purpose and separately constructed session context. Result captures specialist, query, purpose, output, timestamp and error. This is not a claim specialists receive no context. |
| UI handoff | [model_types.go 849–858](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_types.go#L849-L862), [model_update.go 881–930](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L881-L930) | The message carries results, original input, interim response and a flag. Update retains results in memory, invokes error-filtered knowledge persistence and adds the interim response to history, then calls synthesis with only input and results. |
| Terminal answer | [process_knowledge.go 146–194](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L146-L194) | Synthesis builds a new prompt from input and successful response strings. Its system prompt is `withArchitectPersona` plus hardcoded framing; it invokes `CompleteWithSystem` directly and returns plain `assistantMsg`. It neither calls JIT nor renders the prior evidence/context. |

The specialist loop may independently rediscover an important fact or repeat a constraint in its output. The original input may itself contain sufficient context. Those possibilities can make individual answers correct. They do not provide a host-enforced continuity guarantee, and cannot establish that the final response saw a particular source observation.

## Finding one the final answer loses its parent grounding

The first-call evidence is a local variable captured by the articulation goroutine. Neither the consultation message nor the synthesis function accepts it. The initial compiled prompt is likewise local. Adding the interim response to UI history does not restore either: terminal synthesis never reads that history, and an assistant paraphrase would not be the same evidence even if it did.

A useful conditional witness is a source question requiring both repository evidence and domain advice:

1. The host reads a bounded prefix of `app.log` containing a unique observation and marks it PARTIAL.
2. Initial articulation is asked to interpret that observation and requests a specialist's general advice.
3. The specialist returns an accurate generic explanation that does not repeat the source body or its coverage.
4. Terminal synthesis receives the original sentence and the generic explanation, but no host observation or PARTIAL state.

In this path, the final model cannot verify the unique observation from its actual request. It is also not told which part of the source was unavailable. The source supports that missing-input claim; it does **not** prove the model necessarily fabricates a whole-file conclusion.

A conversation-only witness is equally direct: a user asks “Which of those should we choose?” after the earlier conversation established two alternatives and a specific constraint. Initial articulation can resolve the reference from `RecentTurns` and JIT brief context; the final synthesis only sees that short sentence unless the consultation output happens to restate everything. The whole point of the first request's conversation-context renderer is absent at the terminal request.

The important invariant is not “every phase receives all prior tokens.” It is: **the final answer receives the evidence, resolved references, constraints and incompleteness needed for its claims, or the host records that they are unavailable and limits the answer accordingly.** Delivery receipts should bind to the terminal request, not only to the first articulation or to a stored kernel fact.

## Finding two partial consultation coverage disappears at synthesis

The gathering loop retains an error per result. The final prompt skips any result with a nonnil error or an empty response. It adds a failure notice only if no usable response remains; when one response succeeds and another fails, the request contains no account of the missing specialist or missing question. It then asks for a direct and complete answer. [Gathering and projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L115-L177).

The host knows more than it tells the final model:

- `KnowledgeResult.Purpose` is recorded, but synthesis renders only specialist, query and response. Why the question mattered is dropped. [Result fields](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_types.go#L523-L534).
- Errors remain in the UI handler's in-memory `pendingKnowledge` and `knowledgeHistory`, but are not passed as coverage evidence to the final prompt. Although Update invokes `persistKnowledgeResults`, that function skips results whose `Error` is nonnil; this path does not durably store those error outcomes. An empty response with a nil error is not excluded by that persistence filter, even though synthesis omits it. In-memory retention is neither durable failure accounting nor final-request disclosure. [Update handoff](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L892-L911), [persistence filter](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L240-L254).
- Requests beyond the first two are sliced away with a log message before execution. No result is created for the omitted requests, so the terminal prompt cannot distinguish “never requested” from “requested but not admitted.” This is about explicit disposition, not a proposal to remove resource limits. [Request cap](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_knowledge.go#L27-L54).

For example, one specialist can explain the migration syntax while a second was asked whether the change preserves the user's compatibility requirement. If the second call fails, the first explanation remains useful. The final model should be told that compatibility is unresolved; it should not have to infer that from absence.

This is not the older failure mode where a shared advisory caller drops successful rejections on an aggregate error. Here successes are kept, while missing coverage is erased in a separate main-chat projection. Reuse the shared typed outcome machinery where appropriate, but verify this actual consumer.

## Existing tests establish two local properties not the composed one

The evidence tests include a real `processInput` fixture and assert that the first answering request contains a head sentinel and PARTIAL marker. That is meaningful coverage of the new route. Its mock response is a plain answer; it does not issue knowledge requests. [Evidence fixture](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence_test.go#L228-L265).

The synthesis tests establish one LLM call, no pipeline re-entry, presence of original question and successful advice, and an all-empty failure marker. They do not carry a parent evidence/context object or assert mixed-success failure disclosure. [Synthesis tests](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/knowledge_synthesis_test.go#L9-L84). The persona-seam test covers persona placement using the framing string, not a complete JIT/evidence-bearing terminal request. [Persona test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/persona_test.go#L149-L159).

Those tests should remain. The missing test composes their two boundaries: first articulation requests knowledge, Update receives it, and the second provider request is inspected for the same required source/coverage and current answer obligations. None of these tests were run for this report.

## Proposed answer context contract

Use existing task/turn identity and evidence types rather than adding a competing planner. A small immutable continuation record should carry:

- Parent session and turn identity, original request, resolved referents, intent and answer scope.
- Applicable brief/constraint revision, including the distinction between historical context and current admissibility.
- Required evidence observations or authorized recoverable handles, their source identity, coverage and delivery status.
- User-facing blocker and verification obligations still relevant to the final answer.
- Planned consultation request identities, query, purpose and obligation served. These describe requested work, not outcomes that have not happened yet.
- The request-specific prompt manifest and its consumer role. A compiler-global “last result” is not the identity of this answer.
- Already-applied controls as a receipt, never executable instructions to replay.

The names are design notation, not existing APIs. Freeze the parent answer context and planned request identities before launching consultations. Record each request's outcome separately as it settles; only after settlement construct the immutable completed outcome ledger for synthesis, including explicit failed, cancelled, empty or not-admitted entries. Carry both the frozen parent context and this later ledger through `knowledgeGatheredMsg`. Do not claim that a completed ledger exists at launch, or mutate the parent snapshot to add outcomes. Reading `m.history`, `m.lastUserUtterance()` or current global brief later is not equivalent: another chat operation can change what “last” means. When fresh state is genuinely required, explicitly reconcile the owner-bound record against the new state rather than silently adopting another request's context.

### Compile for a terminal consumer

Introduce a synthesis-specific consumer role in the existing JIT context/need contract. Preserve relevant general identity, evidence and brief context, with the architect persona still at its required first position. Do not indiscriminately reuse the entire first prompt: it teaches `knowledge_requests` because the first response has a consumer for that field. Terminal plain-text synthesis intentionally has no such consumer. The existing [consumer_need distinction](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/jit_needs.mg#L83-L102) already provides the right design principle.

The need mapping is necessary but not sufficient. The current `protocol/piggyback/envelope`, `control_first` and `mangle_updates` atoms are mandatory and do not declare a consumer-need selector; simply using a new consumer name does not exclude their JSON/control instructions. The terminal output-mode contract must explicitly govern selection of these protocol atoms and transitively dependent atoms, preserving dependency closure while excluding instructions whose controls the terminal reader will not execute. Keep this selection in the existing compiler and Mangle policy, not an ad hoc string-removal pass. Assert the actual terminal atom inventory and rendered output contract, including absence of mandatory Piggyback-envelope instructions on the plain-text synthesis path. Preserve the protected persona wording and verify that terminal output-mode framing does not cause its generic control references to be interpreted as a request to replay effects. [Mandatory protocol atoms](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/atoms/protocol/piggyback.yaml#L4-L83).

The terminal role should not request another consultation, emit ignored controls or claim unavailable tools. It should explain source uncertainty and separate host observations from specialist advice. Mangle should derive required context and completion limits; Go should carry the owned snapshot, perform rendering and report measured omissions.

A compile failure or evidence-admission failure must produce a typed degraded/blocked answer outcome. A persona-only fallback must not silently count as successful delivery of required source obligations.

### Preserve all outcomes without replaying all content

Represent each consultation as succeeded, failed, cancelled, empty or not admitted, with identity and purpose. Render the successes and concise failure coverage relevant to the answer. Do not discard valid success because another call failed; do not turn empty output into success merely because its transport returned nil error.

Do not automatically retry every missing result. The kernel can distinguish an optional question from an obligation essential to the requested answer, and decide between a bounded retry, narrower supported answer or explicit blocker under the same authority. Shared consultation caching/coalescing is useful only when its identity and owner contract matches this parent context.

Keep first-call control effects exactly once. Synthesis should consume their dispositions as evidence where relevant, not rerun `applyArticulationControlPacket`, perception, routing or the initial turn. This preserves the existing reduction in duplicate execution and avoids a new source of recursive consultation.

## Budget and efficiency

One follow-up call is a useful call-count guarantee, not a complete context or cost bound. Current synthesis concatenates full successful responses. Adding a parent context without admission would make that request larger still.

Budget the complete terminal request: persona, selected synthesis atoms, parent evidence and coverage metadata, required resolved conversation context, successful consultation payloads, missing-coverage records and output reserve. Use whole observations or explicit recoverable projections; never cut a coverage marker, negation or source boundary merely to fit. If an evidence handle requires retrieval, the terminal phase must actually support that retrieval, not merely name an inaccessible handle.

Compare three matched treatments: current minimal synthesis; parent-context continuity with one terminal JIT compile; and continuity plus safe reuse of immutable parent material and shared consultation results. A whole-transcript replay is a diagnostic control, not the recommended design. Keep the same model, source snapshot, tool authority, output limit and question set.

Measure terminal-request evidence coverage, supported-answer accuracy, missed constraint rate, accurate partial-answer disclosure and false completeness alongside total calls, tokens, JIT CPU, retained bytes, provider latency and extra user clarification turns. Count failed consultations and abandoned answers. Extra compile/evidence tokens can be worthwhile if they remove a costly incorrect final answer; fewer calls alone do not establish better efficiency. No gain is claimed without those measurements.

## Proposed validation and implementation queue

| Fixture | Required terminal observation |
|---|---|
| Whole source sentinel, initial knowledge request, generic successful specialist | Final provider request contains the relevant source or usable evidence handle and whole-source coverage. |
| Partial prefix, answer-relevant fact present, unseen tail | PARTIAL coverage survives through Update and synthesis; unseen-source claims are not licensed. |
| Source unavailable, specialist returns only general advice | Final request retains the source failure and separates advice from repository-specific conclusions. |
| “Which of those” after a prior comparison and constraint | Resolved alternatives and applicable constraint reach the final request without replaying unrelated history. |
| Two requests, one success and one error | Useful success retained; missing request identity/purpose and unresolved obligation rendered. |
| Empty response with nil error | Explicit empty outcome, not counted as supporting evidence. |
| Three requests under two-request admission | Third request has an explicit not-admitted disposition; no unbounded execution is introduced. |
| Consultation completes after a newer conversation operation | Parent identity and owned context remain stable; no newest-history substitution. |
| Required terminal context cannot fit | Typed narrower/blocked outcome; no apparent successful grounded delivery after silent omission. |
| First call has permitted controls and knowledge request | Controls applied once before gathering, not replayed by synthesis; final answer can report actual dispositions. |
| Terminal JIT atom inventory | Appropriate identity/evidence/brief atoms remain; mandatory Piggyback-envelope/control atoms and incompatible dependents are excluded by policy, not merely absent from `DerivedNeeds`. |
| Successful one-call synthesis | No second perception/routing pass, recursive knowledge request or duplicate initial provider call. |
| Fresh parent context with no consultation | Existing direct articulation behavior and evidence tests remain intact. |

First update the CLI and prompt specs with the terminal grounding invariant and an owner. Then add the composed request-capture fixture before changing carriers. Implement the narrow immutable carrier and outcome projection, followed by the synthesis-specific JIT role and complete-request admission. Only after correctness is visible should compilation reuse or consultation coalescing be optimized.

The exit criterion is concrete: the provider request that writes the final answer can account for every answer-critical source and consultation obligation from the parent turn, including explicit absence. A first-call sentinel or a successful stored specialist response is not enough.


## As-built status

Verified and built 2026-10-28 on `main` after `c28a1212`. `synthesizeWithKnowledge` and the knowledge handoff were unchanged from the audited pins (the gathering loop had gained `withPartialState` on a failed consultation).

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| Finding one: synthesis receives only the input and successful prose, not the first request's evidence, coverage or conversation context | Real, fixed for evidence, recent conversation and compressed history | `knowledgeParentOf(convCtx)` freezes them when consultations start (`cmd/nerd/chat/process_knowledge.go:50`; call `process.go:1041`); `knowledgeGatheredMsg.Parent` carries them; `knowledgeSynthesisPrompt` renders them (`process_knowledge.go:234`) |
| Finding one: synthesis is not JIT-compiled and gets no brief | Real, left open | `synthesizeWithKnowledge` still uses `withArchitectPersona` plus fixed framing (`process_knowledge.go:189`) |
| Finding two: a failed or empty consultation is skipped silently when another succeeded | Real, fixed | Each is listed with specialist, query, purpose and outcome under a "returned no knowledge" heading |
| Finding two: `Purpose` is dropped | Real, fixed | Rendered with every consultation |
| Finding two: requests past the cap of two are sliced away with only a log | Real, fixed | Each gets a `KnowledgeResult` with `errKnowledgeNotRun` (`process_knowledge.go:84`) |
| Error outcomes are not persisted by `persistKnowledgeResults` | Confirmed, unchanged | Persistence stores knowledge, not failures; the failures now reach the answering request instead |
| Controls are applied once before gathering and not replayed | Confirmed, unchanged | `process.go` control settlement precedes the knowledge branch |
| Existing tests cover the two halves, not the composition | Confirmed; composed test added | `knowledge_grounding_test.go` |

### What was built

- Rulings: (1) the parent grounding is the answering request's own `ConversationContext` (recent turns, rendered evidence, compressed history), frozen at gathering, not the chat's later history. (2) Outcomes are data in the request body; the existing framing line is unchanged and no new instruction prose was added.
- `cmd/nerd/chat/process_knowledge.go`, `process.go` (one call), `model_types.go` (`Parent` field), `model_update.go` (one call); spec `Docs/architecture/cli/03-GAP-ANALYSIS.md` `GAP-CLI-KNOWLEDGE-GROUNDING`.

### Tests

`cmd/nerd/chat/knowledge_grounding_test.go`: a real `processInput` turn that reads `app.log` partly and requests a specialist returns `knowledgeGatheredMsg`, and its synthesis request holds the head sentinel, `PARTIAL` and "the rest was NOT read" together with the specialist's answer; mixed answered/failed/empty outcomes are all rendered with purpose; a third request is reported not run. The existing synthesis tests pass with the new signature.

### Left open

- A synthesis-specific JIT consumer role (brief and evidence atoms in, Piggyback envelope and control atoms out): the compiler's mandatory protocol atoms carry no consumer-need selector, so this is a compiler and policy change, not a chat change.
- No complete-request admission or budget for the terminal request; the parent's recent turns and evidence are rendered as the first request had them.
- Not-run and failed consultations are not retried or escalated by policy; they are disclosed.
