---
doc-class: deep-dive
subsystem: perception-chat-prompt
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
supersedes: []
---

# Conversation brief semantic fidelity and progressive disclosure

## Decision

Keep the conversation-first brief, but stop making its correctness depend on repeatedly rewriting one unowned summary and then treating a clipped string block as the complete work contract. Preserve accepted items with source references; give work its own brief projection; disclose complete essential items before that work proceeds; and make omitted detail recoverable through scoped references.

This is a specific refinement of the new brief, not another conversation memory service. Mangle should decide applicability, retirement, required context and readiness. Go should retain source bytes, validate identities, apply accepted updates, assemble bounded requests and report what was actually disclosed. Models continue to interpret language and propose meaning; typed bookkeeping cannot prove arbitrary natural-language equivalence.

**Evidence status:** static source inspection and a proposed design/evaluation program only. No tests, builds, live journeys, inference, profiling, failure injection, implementation or repository writes were performed for this note. The source deductions below establish available paths and missing contracts; they do not establish how often a model loses a requirement or takes up the wrong task.

## 1. Source freeze and adjacent work

- Main: [356f65e0e6bb639adaa90ab36212daa714114871][main].
- Feature: [c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c][feature], committed 09:41:55 UTC. The compare reports 27 commits ahead and zero behind main.
- Research documents: [fdb4a3e18505a29b8550a516467e0c3560fe5dcc][research], committed 09:31:52 UTC.
- All three branch heads were re-resolved unchanged at approximately 09:50 UTC.
- A branch-specific pull-request collection returned no entries during inspection. This is not a claim about CI or merge readiness.
- Initial inspection used ea3145e266ce52cb2fdec7e9794865858c14f1f7. Its two-commit delta to the final feature pin was inspected. The final pin adds useful read-only routing/task fixes, anchors verb-pair decomposition to perceived intent, clears the typed `/go`, and revises self-diagnosis. Its brief replacement change is the input-box reset, not semantic admission or scoped disclosure.
- `internal/perception/brief.go`, `policy/brief.mg` and the conversation-first architecture document are feature additions and are absent at the main pin. The injection renderer exists on main; its application to this conversation brief is branch-specific.

Read guidance: root [agents.md][guidance], [perception guidance][perception-guidance], [prompt guidance][prompt-guidance], chat guidance and the [architecture-document standard][doc-standard]. These require grounded current-state claims, spec-first work, JIT-first prompts, recoverable context and policy-owned executive decisions. The [conversation-first specification][architecture] is the intended behavior; implementation sources below establish the current boundary.

This note deliberately does not reopen:

| Existing study | Boundary left there |
|---|---|
| [Side-turn ordering and cancellation ownership](side-turn-ordering-and-cancellation-ownership.md) | Receipt order, execution identity, atomic publication, base-version conflicts and cancellation settlement. A perfectly serialized update can still lose meaning; that is the gap here. |
| [Perception restriction continuity](perception-restriction-continuity.md) | Typed read-only/hypothetical/confirmation enforcement and remaining handoff holes. Brief text is not a new permission envelope. |
| [TUI evidence turn ownership](tui-evidence-turn-ownership.md) | Source acquisition, result correlation and truthful articulation. This note concerns the source and applicability of remembered conversational claims, not whether a tool result belongs to a turn. |
| [JIT dependency-closed budgeting](jit-dependency-closed-budgeting.md) | Cross-atom dependency closure after fitting. Here the content can already be incomplete *inside one atom* before fitting. |
| [Retrieval freshness and context value](retrieval-freshness-coverage-and-context-value.md) | Repository search coverage, caches and source observation freshness. A conversation requirement is not a retrieved code observation and needs different retirement semantics. |

The earlier [prompt-budget audit, §2.3][prior-budget] and [knowledge-delivery journey, §1][prior-window] already describe merged mandatory-atom growth, the all-or-nothing budget cliff and the visible caps introduced to bound it. Those are prior findings, not discoveries here. The new contribution is the conversation brief's semantic-fidelity/admission mismatch and its lexicographic crowd-out witness. The older documents are historical context, not substitutes for the current code citations below.

## 2. What already works

The feature has a real, typed five-field brief, not merely a prose instruction to remember. [`Brief`][brief-type] projects goal, constraints, decisions, open questions and readiness into kernel facts. [`recordBrief` and `currentBrief`][brief-store] publish and reconstruct it; a nil update preserves the kept brief. The prompt atom tells perception to preserve the current goal while discussing it, record constraints/decisions and remove answered questions. [Prompt contract][brief-prompt]

Perception sees the running brief alongside the current request and a separately policy-bounded history window. The history builder retains whole turns rather than slicing them and exposes omission notices; it deliberately retains the newest turn even when that turn alone exceeds the configured byte budget, with a notice. The brief itself is rendered whole in that prompt section. This distinction matters: reducing history is already an explicit policy, but fidelity of the surviving brief is not thereby guaranteed. [Prompt construction and history contract][perception-prompt]

The kernel derives readiness and work admission. An unresolved question prevents `brief_ready`; Jev can mark an existing question answered, which excludes it from the blocking set. Propose/explicit modes hold mutation work; read-only requests remain independently admitted. [Brief rules][brief-policy], [Jev answer rule][jev-answer], [work admission][work-policy]

JIT snapshots and cache identity already track dynamic kernel state. Do not “fix” semantic drift by bypassing that freshness machinery or restoring a stale prompt cache. A current snapshot can faithfully cache the wrong semantic projection; freshness and fidelity are separate properties. [Dynamic snapshot collection][snapshot]

Finally, the source already restricts the *running-background-job notification* to the chat agent. The preceding commit explicitly reports that a researcher took up another job after seeing its task. Credit that fix: the finding below concerns the separate, still-wildcard brief rules, not the repaired notification. [Chat-only running-job rule][background-scope]

## 3. Gap B1: an accepted brief has no item-level evidence or explicit retirement

### Source-derived finding

`Brief` carries strings and a Boolean, without item identity, originating message, speaker, scope, support, status or supersession. Every non-nil returned brief retracts the five prior predicates and asserts the new snapshot. The replacement routine does not compare which accepted requirements disappeared or ask what newer evidence justified a changed decision. [Type and fact projection][brief-type], [replacement][brief-store]

This does not mean whole-snapshot formats are inherently wrong. The missing contract is the admission of their *semantic difference*. Even with one request at a time and perfect atomic replacement, an omitted item and an intentional retraction are indistinguishable to this representation. Likewise, the label “Decision” cannot distinguish a user-approved choice from a model suggestion copied into a later summary.

A proposed sequential witness, not an observed incident:

1. The user requests a parser change with “preserve leading zeros” and “empty input returns an empty result.”
2. Several clarifying turns establish the output shape.
3. A well-formed new brief preserves the goal and recent output decision but omits the leading-zero requirement.
4. `recordBrief` replaces the previous snapshot. Nothing in this brief path records the omitted requirement as still active, superseded or unresolved.

Conversely, the user can explicitly revoke an old choice, but a returned brief that accidentally retains it keeps injecting that stale choice. Both failure directions need evaluation: forgetting current obligations and retaining superseded ones.

The point is not to classify every omission as unauthorized, or to freeze all historical text forever. Topic changes and genuine corrections must work. The system needs evidence-linked disposition, rather than inferring that absence from a regenerated list proves deliberate retirement.

### Minimal semantic contract

Preserve the original user message independently of its model-authored interpretation. An accepted item needs at least:

- Stable item ID and brief/work scope.
- Kind: goal, requirement, decision, open question, answer, assumption or observation.
- Exact accepted text plus references to supporting message/source spans.
- Origin/authority: user-stated, jointly accepted, model-proposed, inferred or externally observed.
- Status: active, proposed, answered, superseded, withdrawn, stale or disputed.
- A change reason and supporting source for replacement/retirement.

These are proposed fields, not existing identifiers. Source support is a traceability claim, not proof of entailment. Quotes can be genuine yet misinterpreted; semantic quality still needs evaluation.

Have the model propose add/amend/answer/withdraw operations against known item IDs, or compute an equivalent typed delta from its snapshot. Keep unaffected accepted items by construction. A missing item is not a withdrawal. A model hypothesis stays a hypothesis unless the existing policy admits it as an agreed decision. Never let tool/document text silently acquire the authority of a user instruction.

Use explicit source revisions only where they have meaning. A user requirement does not expire because a source file changes. A claim that “the API currently accepts X” is an observation and should depend on the source/evidence revision. A user's correction supersedes a prior preference through conversation evidence. Treating all three as generic timeless “decisions” loses these distinctions.

## 4. Gap B2: conversation continuity is not task applicability

### Source-derived finding

The prompt says small talk and questions about code leave the work brief unchanged. The policy then projects every brief goal, constraint, decision and question through `injectable_context("*", Line)`. The JIT collector explicitly admits the wildcard for every shard. [Prompt preservation rule][brief-prompt], [wildcard projection][brief-policy], [collector][collector]

Task isolation does not remove this path. `CloneForTask` clears per-run conversation/session state but shares the kernel and JIT compiler. The spawner also passes its kernel and compiler to a subagent. These are useful lifecycle boundaries, not per-task brief selection. [Inline clone][clone], [subagent construction][spawn]

A sequential witness needs no stale asynchronous completion:

1. The active work brief concerns adding API rate limiting.
2. The user asks an independent question about a CSV parser.
3. Perception correctly follows the prompt and preserves the rate-limiting brief while classifying the CSV question.
4. If that question delegates, the CSV worker's compile is eligible to include the rate-limiting goal/decisions as generic live brief context.

This establishes wrong-scope *eligibility and delivery through the normal collector*, subject to its bounds, not that the worker necessarily follows it. Other injected/task instructions may keep the answer correct. That behavioral rate is an evaluation question. The source provides no brief/task applicability key that distinguishes helpful continuity from an unrelated instruction.

### Required distinction

Keep three views:

1. **Conversation view:** the chat agent may see active work, queued topics and recent transitions so it can converse coherently.
2. **Work view:** each work item binds an applicable accepted brief revision and its essential requirements.
3. **Step/specialist view:** the kernel projects the subset needed by this task/step, with shared constraints inherited only when their scope applies.

A child is not entitled to every conversational goal just because it shares a workspace. Conversely, suppressing all parent context would lose valuable constraints. Projection must carry provenance and relevance, not simply remove `"*"` and hope callers reconstruct the brief.

An unrelated read-only question should not retire the active work. A new task should not inherit old task-specific choices automatically. Resuming an earlier task should recover its accepted requirements without resurrecting superseded decisions. These are semantic scope decisions. Their concurrent publication mechanics belong to the side-turn ownership study.

Do not bind an entire long-running job forever to an obsolete brief. An accepted amendment needs a defined application point: before the next affected planning/action boundary, rederive the work projection and identify changed obligations. If a prior action is now inconsistent, surface that fact rather than pretending a new prompt retroactively changed the effect.

## 5. Gap B3: the ready brief can become incomplete inside its mandatory prompt atom

### Source-derived finding

The collector flattens matching injection rows into a canonical string set. `canonicalStringSet` sorts lexicographically and deduplicates. The renderer then:

1. Takes the first configured number of rows.
2. Applies head-clamping to each row.
3. Applies a second clamp to the aggregate block.
4. Wraps that result in one mandatory `kernel/context/...` atom.

[`collectKernelInjectedAtomsFromConfig`][collector], [canonical ordering][canonical], [renderer][renderer]

The current defaults are 60 rows, a 1,024-byte row-content threshold and a 16,384-byte aggregate-content threshold, plus truncation-marker overhead; configuration can override them. Despite the configuration names ending in `Chars`, the clamping helpers count bytes, preserve UTF-8 boundaries and append markers outside the content threshold. These are rendering controls, not exact final-payload ceilings or per-brief semantic guarantees. The `Brief` representation itself has no corresponding item-count or length bounds. [JIT defaults][jit-defaults], [clamping semantics][elide], [brief type][brief-type]

Credit the existing truncation notices. The problem is not invisible byte truncation: the model can be told that text is incomplete. The missing guarantee is that required brief meaning survived, or that omitted meaning has a specific retrievable identity before work continues.

Two deterministic source-derived witnesses:

- A long constraint ends with a crucial exception or negation after the row head limit. The renderer retains its prefix and a truncation marker, not the full condition.
- Sixty distinct `Brief constraint: ...` rows sort ahead of `Brief goal: ...`. At the default row cap, the goal is omitted even with no competing non-brief rows. Other lexical predecessors can displace brief content with fewer brief items. The aggregate cap can remove additional material independently.

Neither witness requires the model to lose a requirement during summarization. Even a perfect brief can be rendered incompletely. “Mandatory” applies to the resulting atom, not to every constituent requirement.

The readiness predicate reads the full kernel facts, not a receipt of the final worker request. It therefore cannot certify that the model received a faithful work brief. This is an input-quality gap, not a claim that readiness bypasses constitutional permission. [Readiness][brief-policy], [collection/rendering][collector]

### Progressive disclosure contract

Do not solve this by removing bounds or arbitrarily increasing them. Split the projection into:

- **Essential work capsule:** task identity, current goal, applicable hard requirements, unresolved blockers, and the accepted item/revision IDs. Required meaning must be whole, or the task must be narrowed/paused with an explicit reason.
- **Decision synopsis:** concise, attributable choices sufficient for this step, with handles to exact supporting messages and full rationale.
- **Optional history/detail:** scoped, paginated retrieval whose handle binds work/item identity and revision, rather than an orphaned truncation marker.

Some requirements are longer than a convenient capsule. A paraphrase is not automatically lossless. Preserve the source and use a reviewed compact representation only where equivalence is sufficiently established for the task; otherwise hydrate the complete requirement or refuse to proceed under an incomplete specification. Ordinary harmless conversation need not be blocked merely because optional history is long.

The compiler should prepare a brief-disclosure receipt recording requested item IDs, selected representations, deferred items/handles and unresolved omissions. Complete disclosure must be certified against the final provider-bound payload after every later fit and transformation, not merely against compiler output. Policy can then distinguish “ready to plan,” “ready to execute this step,” and “missing essential brief evidence.” Do not rely on the model to announce that an unseen requirement is missing.

Extend existing context archive/recall machinery where compatible. First verify owner, lifetime, lookup authorization and revision behavior: a tool-observation archive is not automatically a durable conversation-source store. A stale or inaccessible handle must report that state, not return an unrelated current item.

## 6. Readiness should express resolved meaning, not only the absence of a question string

The existing Jev rule may classify a brief question as answered while the original `brief_open_question` fact remains. Readiness excludes it from the blocking set, but wildcard injection still emits it as “Brief open question.” [Answer rule][jev-answer], [brief rules][brief-policy]

That is a source-visible distinction, not an assertion that Jev answers are wrong. A consumer can receive an apparently open question after admission considered it resolved. The proposed item model should bind an answer and its source to the question, keep the original question for history, and render its effective status consistently. If the answer is not yet available to the intended worker, classify that as missing disclosure rather than silently equating “someone judged it answered” with “this worker has the answer.”

Readiness still belongs in Mangle and remains separate from user approval, tool capability and permission. No brief projection should grant an effect, erase a typed restriction or substitute for exact-request approval.

## 7. Literature: useful principles, not imported performance claims

- [LongMemEval, Wu et al., ICLR 2025](https://arxiv.org/abs/2410.10813v2) separates extraction, cross-session reasoning, temporal reasoning, updates and abstention; it also distinguishes indexing, retrieval and reading. Adopt that separation in the evaluation: good recall does not prove correct interpretation of a superseding instruction. Its chat QA benchmark is not a coding-agent safety proof.
- [LoCoMo, Maharana et al., ACL 2024](https://aclanthology.org/2024.acl-long.747/) evaluates long conversational memory using grounded temporal/event structure and human-verified dialogue. Use its lesson to construct dialogue histories with explicit causal changes, not only isolated fact-recall questions. The proposed coding conversations need their own task/effect oracles.
- [Lost in the Middle, Liu et al., TACL 2024](https://aclanthology.org/2024.tacl-1.9/) finds position-sensitive performance in long-context retrieval tasks. This motivates varying requirement placement and distractors; it does not show that a shorter brief eliminates positional failures or that the same effect size applies to the configured models.
- [MemGPT, Packer et al., 2024 revision](https://arxiv.org/abs/2310.08560v2) demonstrates hierarchical context management and movement between memory tiers. Borrow the separation between a bounded active view and recoverable detail, not its model-led executive control. CodeNERD's policy retains the admission/disclosure decision.

None of these papers establishes a token saving, task-success uplift or failure rate for this repository.

## 8. Cost model and alternatives

Let `B_t` be the rendered brief size on conversational turn t, `D_t` its model-generated replacement, and `W_j` the brief context delivered to worker request j. The brief-related input/output exposure is approximately:

`sum_t B_t + sum_t D_t + sum_j W_j`.

This excludes ordinary task/history/tool tokens and is not an invoice formula: provider caching, output rates and reasoning usage differ. If each of n turns adds d retained tokens and rewrites the whole brief, the cumulative brief input/output volume can grow on the order of `d*n^2`. A constant-sized summary avoids that growth only by choosing what not to retain, which is exactly the fidelity question.

Stable items and deltas can reduce repeated *output* of unchanged content. They do not eliminate the need to read enough context to interpret a new message. Scoped projection can reduce unrelated worker input, but paging may add retrieval/model turns and latency. Immutable source retention costs storage; item/source indexes cost CPU and memory; applicability rules and exact disclosure receipts cost policy evaluation. Semantic checking may be expensive and must not become an unconditional second model call without measured value.

Compare four designs under the same task and correctness obligations:

1. Current whole brief plus bounded history.
2. Larger/full-history context as a quality reference where the input window permits.
3. Stable items with explicit deltas but no task-conditioned disclosure.
4. Stable items plus scoped capsules and recoverable detail.

Measure cold and warm paths. Do not count cache hits as avoided input without measuring provider accounting. Preserve current kernel/source generation checks; cache a projection only under the work, accepted brief revision, policy and relevant source identities that determined it.

The target is **less total cost per correctly completed task**, not fewer tokens in one request. Include recovery turns, incorrect-task work, forgotten-requirement repairs, refusals and unnecessary clarification. A system that cheaply completes the wrong brief is not an optimization.

## 9. Evaluation program — proposed, not executed

### Layer A: deterministic contracts

Use controlled perception outputs to isolate transport/projection from language understanding, but exercise the real chat entry path, kernel, JIT collector and final provider-request capture.

| Case | Required result |
|---|---|
| Unchanged accepted requirement absent from a proposed delta | Remains active unless a supported retirement is admitted. |
| Explicit correction versus unrelated small talk | Correction supersedes exactly its target; small talk changes no work obligations. |
| Active task A, independent question B, resume A | B receives only applicable context; A recovers its current accepted requirements. |
| Long final-clause exception | Full essential semantics reach the actual request or execution reports incomplete brief evidence. |
| More than 60 brief rows; competing injections; small configured caps | No essential item silently disappears inside a “mandatory” atom; omission receipts identify every deferred item. |
| Goal/constraint lexical renaming with equal meaning | Selection is policy-driven, not accidentally changed by alphabetical order. |
| Answered question retained for history | Worker gets its answer and effective status, not a contradictory open-question label. |
| Source-backed observation changes; user preference unchanged | Observation becomes stale; the unrelated user requirement does not expire. |
| Failed/stale/unauthorized recall handle | Explicit unavailable/deferred disposition; no invented detail and no scope widening. |

Existing [work-admission tests][admission-tests] cover kernel round-trip, nil preservation, whole replacement and queryable injection rows. Existing [conversation journeys][journey-tests] script perception and record task execution. Those are valuable positive controls, but scripted intended briefs do not measure whether a real model preserves meaning; querying `injectable_context` is not proof of final request delivery.

### Layer B: semantic fidelity

Build held-out multi-turn coding conversations with human-adjudicated item histories: exact requirements, nonrequirements, proposals, accepted decisions, corrections, answers and applicability. Include negation, exceptions, “only,” quantities/units, literal filenames, ambiguous pronouns, conditional instructions, abandoned proposals and several task switches. Place critical information at early, middle and late positions; vary delay and distractor density. Include missing-answer cases where abstention is correct.

Score separately:

- Requirement retention recall and unsupported-requirement precision.
- Supersession accuracy and stale-choice resurrection.
- Correct task/step applicability.
- Answer/source attribution and unsupported decision promotion.
- Essential meaning actually delivered at the final request boundary.
- Downstream task success, constraint satisfaction and incorrect extra work.
- Total provider tokens/cost, tool calls, latency, archive reads and clarification burden.

Do not grade only string overlap or only the brief writer's self-assessment. Use exact assertions for literals and effects, blinded human rubrics for semantic equivalence, and disagreement adjudication. Report sample sizes, model/provider/config pins, repeated-run variation and uncertainty. Separate classifier failure from publication, retrieval, clipping, interpretation and execution failure.

### Layer C: normal production journeys

After deterministic and semantic checks, run the existing production chat → intent → held/background/foreground work routes with CodeDOM/world context enabled. Capture the actual requests of both inline clones and isolated subagents. Confirm that a mid-work amendment reaches the next relevant worker boundary rather than merely appearing in the parent kernel. Keep the side-turn ordering suite as a dependency, not a substitute for these fidelity checks.

No success threshold is claimed achieved here. Proposed release criteria are zero essential-disclosure violations in the deterministic suite, no regression in existing admission/restriction cases, and a predeclared noninferiority margin on end-task correctness before accepting cost improvements. Hard-constraint failures must remain individually visible, not hidden in an average quality score.

## 10. Smallest coherent implementation queue

These are candidate spec gaps, not claims that identifiers already exist in the repository:

| Candidate gap | Spec owner | Checkable exit |
|---|---|---|
| Brief semantic changes | Perception/chat | Accepted items retain source identity; omission is distinguishable from supported withdrawal; sequential correction fixtures pass. |
| Work-specific projection | Chat/session/prompt | Independent tasks receive no unrelated brief instructions; applicable shared requirements survive in actual requests. |
| Essential disclosure | Prompt/context | No clipped essential item is represented as completely delivered; required missing detail blocks only the affected work and remains recoverable. |
| Answer and readiness fidelity | Perception/core/chat | Effective answered state, source and worker-visible answer agree before readiness relies on resolution. |
| Evidence-backed efficiency | Usage/evaluation | Paired normal-path runs report quality and full lifecycle cost with reproducible source/config pins. |

Start with item preservation and a final-request disclosure witness. Then add scope-aware projection, then optimize paging and compact forms. A small in-memory implementation can establish these invariants before any durable migration; it should not claim restart persistence until the source records and accepted item history are actually persisted and restored.

The feature's immediate value is conversation that shapes work before action. The remaining goal is to make the same accepted meaning survive every transition: **user message → interpreted item → admitted brief → scoped task → complete model-visible obligation**, with a source-backed explanation whenever meaning is changed, deferred or retired.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/fdb4a3e18505a29b8550a516467e0c3560fe5dcc
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md
[perception-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/agents.md
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/agents.md
[doc-standard]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/09-architecture-doc-standard.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/conversation-first-agent.md
[brief-type]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/brief.go#L9-L58
[brief-store]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold.go#L100-L140
[brief-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/atoms/perception/understanding.yaml#L123-L139
[perception-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/transducer_llm.go#L328-L379
[brief-policy]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/brief.mg#L1-L43
[jev-answer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/jev_evidence.mg#L71-L76
[work-policy]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/routing_arbitration.mg#L250-L263
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/compiler_kernel.go#L164-L218
[background-scope]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/routing_arbitration.mg#L320-L326
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/compiler_kernel.go#L221-L274
[clone]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L585-L641
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/spawner.go#L409-L430
[canonical]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/context.go#L759-L772
[renderer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/compiler_kernel.go#L142-L161
[jit-defaults]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/config/jit.go#L122-L124
[elide]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/types/elide.go#L78-L151
[admission-tests]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_admission_test.go#L99-L138
[journey-tests]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/journey_conversation_test.go#L17-L78
[prior-budget]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/2026-09-09-system-hardening/02-PROMPT-BUDGET.md#23-kernel-injected-atoms-promptcompilergo
[prior-window]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/03-knowledge-delivery-and-window.md#kernel-injected-context-atoms-the-spreading-activation-seam


## As-built status

Verified 2026-10-28 on `main` after `c28a1212` (the feature branch is merged). One narrow finding was built; the item-level redesign was not.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| B1: `Brief` is five string fields with no item identity, source, status or supersession; each update replaces the whole snapshot | Real, not built | `internal/perception/brief.go`; `writeBrief` (`cmd/nerd/chat/work_hold.go`) now swaps it in one transaction, which fixes torn publication (side-turn audit) but not semantic loss |
| B2: brief rows are injected with the `"*"` wildcard into every shard and task clone | Real, not built | `internal/core/defaults/policy/brief.mg` `injectable_context("*", ...)` |
| B3: rows are sorted lexicographically, capped at 60 and clamped; 60 `Brief constraint:` rows displace `Brief goal:` | Real, not built | `canonicalStringSet` (`internal/prompt/context.go:769`); row cap and clamps in the injected-context renderer (`internal/articulation/prompt_assembler.go:492`) |
| Section 6: an answered question (Jev) stops holding work but is still injected as "Brief open question" | Real, fixed | `brief.mg:44-50`: the open-question row excludes `brief_question_answered`; an answered one renders as answered |
| Running-background-job notification is chat-agent only | Confirmed | `background_job` injection rule (credited by the audit) |

### What was built

- `internal/core/defaults/policy/brief.mg`: the injected brief agrees with readiness on answered questions. Spec: `Docs/architecture/conversation-first-agent.md`, brief section.

### Tests

`cmd/nerd/chat/brief_answered_question_test.go`: with two open questions and a Jev judgment answering one, the answered one is not injected as open and is rendered as answered; the other stays open. `TestJevJourney_AnsweredOpenQuestionUnblocksYolo` and the work-admission tests pass.

### Left open, with reasons

- B3 ordering and cap: the fix belongs in the injected-context renderer (priority-aware selection instead of lexicographic order and a row cap that can drop the goal). That renderer (`internal/articulation/prompt_assembler.go`, `internal/prompt`) is under concurrent change by the articulation lane; renaming row prefixes to game the sort would be a workaround, not a fix.
- B2 applicability: removing the wildcard would drop valid parent constraints from workers; binding brief items to a work scope needs B1's item identity first.
- B1 item identity, source references, status and delta admission, the essential work capsule and disclosure receipts: a redesign of the brief contract (perception prompt atom, `Brief` type, policy and renderer), not built in this pass.
- No predicate was added (only rules), so no predicate-corpus rebuild is needed for this change.
