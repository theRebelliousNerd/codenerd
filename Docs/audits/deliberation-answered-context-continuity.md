---
doc-class: research-proposal
subsystem: deliberation-values-continuation
implementation-status: proposed
last-verified: 2026-11-03
verified-against: 51020eb9acea19fcca802a2a5a92a1f336d305cf
verification-method: static-source-inspection
---

# Keep answered values context beyond the previous round

## Finding and decision

**A free-text values answer survives into later deliberation frames, but its originating question does not have the same lifetime.** The conductor saves the question and displayed options on the thread. On resumption it copies the answer into every new frame, while supplying only the immediately preceding completed round. The first resumed round therefore has the original question through its previous challenge. After another nonterminal round completes, that asking round leaves the explicit frame. The answer remains, without a dedicated question, offered-option record or reference linking it back to the interaction that gave it meaning. [Thread and frame][types] [Frame construction][collect] [Continuation][run]

This is a source-confirmed **answered-context delivery gap**, conditional on refinement continuing beyond its first resumed round and the meaning not being preserved by other supplied content. It is not deletion of the answer, proof of a wrong live design, a permission bypass or a claim that every model input lacks the information. All completed rounds, the question, options and answer remain in the saved thread. A model may preserve the preference in a candidate, obtain it through another context source or finish before the relevant boundary.

The proposed refinement is small: carry a bounded, host-owned record of the already-answered values interaction independently of the rolling previous-round window. Reuse the existing continuation, JIT roles and Mangle decisions. Do not resend the entire history, ask the user again by default, treat the answer as approval, or rely on a model to restate an earlier human constraint perfectly at every round.

## Source and verification boundary

Public main resolved to **51020eb9acea19fcca802a2a5a92a1f336d305cf**, committed at 16:49:42 UTC on November 3, 2026, and remained at that revision when rechecked at approximately 17:17 UTC. All repository links below use that immutable pin. [Commit][pin]

This review inspected public root and relevant scoped guidance, current session/shard/mission architecture, the deliberation package, production chat and mission adapters, authored tests, the current audit inventory and related reports. Cached public files were checked against the pinned Git blob identities before use; newly retrieved files came from the same revision.

No codeNERD program, build, test, model inference, benchmark, gate or failure injection ran. No implementation, credential, permission, configuration or setting was changed. Fixtures, contracts and evaluation outcomes below are proposed and unrun.

The session/shard specifications still describe runtime verification as pending for their original authoring pass. A later public live-exercise record reports a successful fifth deliberation run, converging in two rounds in yolo mode without asking the person. Credit that positive evidence for its stated exercise. It does not exercise a human answer followed by multiple refinement rounds, and this review did not rerun it. [Session specification][spec] [Shard specification][shard-spec] [Live exercise][live]

## 1. Why this matters to Socratic requirements work

The repository's north star separates creative interpretation from executive orchestration and requires bounded, recoverable working context. The proposer must preserve explicit constraints and treats the person's answer as an additional constraint on the original request. The challenger must assess the complete request and avoid reopening answered matters. These contracts make a human answer part of the continuing requirements problem, not an ephemeral note attached only to the next exchange. [Root guidance][guidance] [Proposer][proposer] [Challenger][challenger]

A short answer can be complete in its conversational setting. “Yes, for existing callers” means something concrete when it answers “Should existing callers retain the current default?” Its bytes alone do not state what was affirmed. Requiring the user to restate every question in their answer would shift bookkeeping back to the person after the assistant deliberately asked a contextual question.

Bounded context is still the right goal. The gap is that the two halves of one accepted interaction are projected with different lifetimes: the answer is persistent frame data; its question is incidental previous-round data. A durable source record and an explicit current delivery contract are separate properties.

The session specification accurately states that only the last completed round is replayed and that free text may trigger refinement. It does not promise the proposed dedicated answer-context carrier. This audit identifies the missing guarantee at the composition of those two existing behaviors rather than claiming that the literal one-round replay description is false. [Continuation design][spec] [Architecture guidance][architecture]

## 2. Implemented behavior to preserve

| Boundary | Implemented behavior | What it does not establish |
|---|---|---|
| Values publication | A concrete question and at least two eligible displayed alternatives are saved with the original request, intent and ownership. | Their appearance in every later participant frame. |
| Exact choice | A recognized candidate ID or rendered option replays the saved asking round without another role call. | The free-text refinement case studied here. |
| Free-text answer | The chat assigns the answer to the saved thread; the service refines under the original request and intent. | A persistent explicit relationship between that answer and its original question in the role input. |
| First resumed round | Previous points to the asking round, including its question and proposed candidates. | Retention once an intervening refinement round becomes Previous. |
| Persistence | The complete thread, including all rounds, is saved atomically. | Automatic delivery or consumption of every retained record. |
| Role instructions | Preserve constraints; do not reopen answered matters; never turn the answer into unrelated approval. | Host-checked retention of the interaction that specifies those constraints. |
| Executive checks | Validation, policy selection, proof recording, stopping and normal work admission remain in place. | Proof that all natural-language requirements were supplied to the role or interpreted correctly. |

Sources: [Publication and choice][finish], [chat continuation][chat], [service][run], [collector][collect], [persistence][save], [atoms][proposer], [policy projection][projection].

The precise issue is the explicit frame's source-context continuity. The record has not been erased, and preserving it does not independently prove that a candidate satisfies it.

## 3. Exact producer-to-consumer chain

### 3.1 The conductor knows what it asked and showed

For an ask-once disposition, finish trims and validates the challenge's question, queries values eligibility, places the recommended candidate first, appends the other eligible candidates, and requires at least two options. It then saves:

- the question in Thread.Question;
- the actual rendered options in Thread.Options;
- Asked=true and Status=awaiting_values.

The entire asking round also remains in Thread.Rounds. The exact displayed option sequence is a host-produced result, not necessarily the raw proposal order. [Question publication][finish]

The chat builds its clarification from Outcome.Question and Outcome.Options. On submission, the values path resumes the saved deliberation ID; the adapter loads that thread, checks awaiting_values, assigns Thread.Answer and calls the service. It does not replace Thread.Request, Thread.Intent or Thread.Brief with a new answer-bearing interaction record. [Question adapter][chat] [Submit routing][submit]

### 3.2 Free text deliberately takes a different route from exact choice

The service recognizes an exact candidate ID or rendered option while the thread is awaiting values. That route reasserts the saved round, adds an answer fact and finishes against that round. It avoids this frame-horizon issue and should remain efficient. [Fast path][run] [Choice matching][finish]

A contextual answer with an extra qualification, such as “Yes, for existing callers,” need not equal either accepted exact string. It enters the normal convergent loop. The service saves running, then constructs the next round. There is no automatic update of the original request or brief that materializes the answered question as a self-contained constraint. [Service][run] [Collector][collect]

This is an intended input category, not malformed output or an unauthorized new task. The specification explicitly permits free-text refinement.

### 3.3 The first resumed frame has real context

collectRound starts with the original brief, request, intent and the current answer. When rounds exist it sets Previous to the last completed round. The proposer sees that frame; after the proposal is decoded, the challenger sees the same previous round and answer plus current candidates and consultation evidence. [Frame construction and dispatch][collect]

Immediately after the question, Previous contains the asking round. Its Challenge.Question supplies the original question, and its Proposal.Candidates supplies candidate descriptions. Do not describe that first resumed frame as containing only an isolated answer.

Even at this point, the published Thread.Options list is not a dedicated frame field. The previous proposal and recommendation contain useful reconstruction material, but are not the exact emitted list: publication filters eligible alternatives, moves the recommendation first and decorates its label. A free-text response that refers to displayed order therefore deserves the actual shown context rather than an implicit expectation that each role reimplements the renderer.

### 3.4 One successful intervening round changes the replay source

Suppose the first resumed round returns needs_work because a design detail still requires refinement. The service appends that completed round, saves it, updates progress and loops. With ordinary remaining allowance, the next collectRound now points Previous at this new round. The original asking round is no longer in that field. [Loop and append][run]

Frame has an Answer field but no Question, Options, answered-question generation, asking-round reference or dedicated interaction record. Round itself has no nested previous round. Its Challenge.Question is an optional field for that round's challenge, not a host-owned copy of the earlier question. Consequently an ordinary non-values refinement round may have an empty question without being structurally invalid. [Types][types] [Validation][validation]

The original question can still survive in current or previous candidate prose, assumptions, item details, the original request/brief or other context. Those are real mitigating routes, not guarantees supplied by this projection. The host does not require those fields to preserve the exact interaction.

### 3.5 Other mechanisms do not create the missing explicit relation

The collector's additional ReceiptFacts come from deliberation_probe. They do not serialize the saved human question or displayed options. During a free-text continuation, the service's exact-choice deliberation_answer fact is not the branch being used. The existing deliberation_questioned fact records that a question was asked, not what it was or how its answer was grounded. [Collector][collect] [Service][run] [Schemas][schemas]

The production chat adapter serializes Frame as the shard task. The role agent initializes a payload with that frame and fresh role-local observations, then uses the task for JIT compilation. There is no automatic all-round replay in these functions. This does **not** prove every possible JIT injection, conversational brief or authorized retrieval source is identical or unable to recover the question; the finding is confined to the designated typed frame boundary. [Chat serialization][chat] [Role payload][agent]

Finally, the full thread persists. Loading its JSON can recover the question, answer and asking round. The missing property is an explicit bounded delivery/resolution contract for the current consumer, not permanent irrecoverability. [Persistence][save]

## 4. Minimal source-derived fixture, not executed

Use the normal non-yolo, design-only path with one thread, a stable workspace, successful persistence and proof recording, small strings and sufficient remaining context/round allowance. No consultation, empirical probe, source drift, cancellation, concurrent run or malformed JSON is necessary.

1. Round 1 supplies two valid values-eligible alternatives and asks: “Should existing callers retain the current default?” Let normal finish construct and save the actual displayed options.
2. Supply the free-text answer “Yes, for existing callers.” Choose ordinary candidate IDs and briefs so this matches neither an ID nor a full rendered option.
3. Capture proposer and challenger frames in round 2. The answer is present and Previous.Number is 1. The original question remains in Previous.Challenge.Question. This is the positive first-resume control.
4. Script a structurally valid round-2 proposal and one assessment per candidate, with every candidate retaining a critical design blocker and no ready, empirical or values-eligible alternative. Let policy derive needs_work. Leave the optional current Challenge.Question empty and do not repeat the original question or full option strings in other round-2 text.
5. Capture the round-3 proposer and challenger frames. The answer is still present and Previous.Number is now 2. Inspect the serialized frame for the original question, exact option list and any dedicated answer-context binding.
6. Separately inspect the saved thread. The originals remain in Thread.Question, Thread.Options and Round 1. This distinguishes retention from delivery.
7. End the future fixture through its controlled cancellation/settlement seam after capture, or supply an ordinary scripted final response solely to settle it. Do not count that scripted verdict as evidence of model understanding.

**Source-predicted observation:** the original question can be absent from the round-3 frame while its contextual answer remains. The exact displayed option list has no dedicated projection in either resumed frame.

The fixture deliberately does not make the roles semantically infallible. Its round-2 omission is structurally admissible and isolates whether the host preserves the human interaction independently of model repetition. A second-stage model evaluation is needed to determine whether actual models omit, reconstruct, misinterpret or correctly retain the preference, and at what frequency.

Round 3 is reachable under the documented default four-round allowance. The values round returns before the normal progress-fingerprint update; one subsequent needs_work round does not inherently force the default two-repeat stop. An evaluator should verify actual counters and policy in the future fixture rather than disabling stopping rules. [Service order][run] [Stop rules][policy] [Defaults][defaults]

### Useful controls and distinctions

- **Exact offered option:** use the existing no-new-role-call replay control; no horizon gap should be invented there.
- **Ready after round 2:** the original question was still available for that decision.
- **Self-contained free text:** “Existing callers must retain the current default” needs less originating context than “Yes, for existing callers.”
- **Incidental preservation:** explicitly repeat the complete constraint in round-2 assumptions; later roles may then have sufficient meaning today.
- **Display-order reference:** answer “The second option, but keep old callers unchanged”; vary proposal order and recommendation while checking the exact published list. Do not equate proposal index with display index.
- **Multiple subjects in the original request:** use two independently described defaults to test whether a short answer remains bound to the question actually asked. Avoid presuming which default the user probably meant.
- **Genuine ambiguity in the original exchange:** a carrier should preserve that ambiguity, not turn it into an authoritative interpreted requirement.
- **Saved-thread recovery:** demonstrate successful authorized recovery as a mitigating control and count its cost.
- **No answer:** do not fabricate a question-answer record.
- **Permission hold:** carrying a preference must not release propose/explicit work admission or change the original restriction.

No fixture results are claimed here.

## 5. Novelty and coordination with existing audits

| Related work | Its boundary | Distinct boundary in this report |
|---|---|---|
| Answer-evaluation recovery | Failed/running attempt status hides an already-supplied answer from normal discovery and resumption. | All calls and saves may succeed; the active consumer loses explicit context as its replay window advances. |
| Values-option admission | Exact answers may be matched against a broader candidate set than the displayed alternatives. | Deliberately nonmatching free text enters refinement; no unoffered candidate selection is needed. |
| Values-evidence freshness | A human answer can re-evaluate historical workspace premises without freshness admission. | Stable workspace; the missing relation is conversational meaning, not source freshness. |
| Critical-item retirement | An earlier objection can disappear without an explicit resolution before readiness. | Inspect role input before any later readiness claim; this is a human-answer context carrier, not an objection lifecycle. |
| Consultation-subject binding | The current challenger lacks the question/subject of specialist advice. | A human answer initially has its asking round, then loses that explicit source after an intervening round. The proposed carrier is the saved user interaction, not a specialist request/result. |
| Conversation brief/history continuity | Accepted conversational items and compressed history need source, scope and delivery guarantees. | The small separate deliberation Frame copies Answer indefinitely while replaying only one Round; no compression failure or brief rewrite is necessary. |
| Probe consumption horizon/frame capacity | Evidence may be collected with no remaining review, or make a later frame too large. | The later review runs with ample capacity; the question's absence is caused by field selection, not exhaustion. |

Sources: [Recovery][recovery-audit], [options][options-audit], [freshness][freshness-audit], [retirement][retirement-audit], [consultation binding][subject-audit], [brief][brief-audit], [history][history-audit], [probe horizon][horizon-audit], [capacity][capacity-audit].

This is a related continuity-contract refinement, not a new class of orchestration. The recovery and option-admission proposals already call for question-bound receipts. Reuse one such interaction record and extend its consumer projection, rather than inventing competing receipt schemas. The narrow new exit is retention across the second and later resumed frames. The inventory/neighbor review is bounded to the inspected public revision, not a claim about unpublished work. [Audit inventory][inventory]

## 6. Proposed contract, explicitly not implemented

### Required invariant

Whenever a participant receives an answer from a previous human values interaction, it also receives, or can resolve through an admitted bounded path, the original question and the options actually shown that the answer refers to. The relation must remain available after the asking round leaves Previous. Missing context must be explicit rather than silently represented as a fully grounded answer.

### Ownership and data

Use the existing thread as the durable owner. A proposed answered-interaction view would include:

- an opaque interaction identity scoped to thread/session/owner and its asking round or generation;
- the exact admitted question;
- the actual displayed option order and stable candidate identities/content binding;
- the exact user answer and its input kind, without replacing it with model paraphrase;
- an explicit availability state for any referenced content.

The exact schema is a spec-first decision in the existing session/shard contracts. The listed fields are a proposal, not existing exported APIs or predicates. The current single-question design needs only one active record; it does not require an unbounded conversation archive.

The host knows the published question and options. It should snapshot that relation from publication, then attach the received answer to it at resumption. It should not infer the question from the latest Challenge.Question, lexical similarity, current candidate ordering or a newly generated summary.

### Delivery and bounded context

Project the answered interaction independently of Previous for both proposer and challenger. Keep the current original request, intent and bounded previous round. Do not recursively nest complete history.

For a small interaction, inline the exact question and shown options beside the answer. For a larger interaction, an immutable, owner-scoped handle can be appropriate only if the role has an actually available resolution path. A digest or filename that the role cannot resolve is an identity marker, not delivered meaning. The frame must clearly say which fields are inline, omitted or unavailable.

Charge this view against the existing serialized context bound and terminal-note reserve. Do not hide growth in a system prompt, raise all budgets as the sole remedy, or truncate a negation/condition without disclosure. If essential context does not fit, use an explicitly admitted narrower view or stop with a named context blocker; do not ask a second preference question merely because bookkeeping dropped the first.

JIT atoms should state how to use the view: the answer constrains its original question's subject, is not approval of unrelated work, and does not by itself settle empirical or design objections. Preserve the existing read-only tool and constitutional boundaries.

### Executive and semantic responsibility

Go can retain exact bytes, bind identities, validate source availability and measure delivered bytes. Mangle should own whether required context is available for the current deliberation transition and whether to continue, ask, stop or proceed. The model still interprets natural language and proposes the design. Do not claim that an interaction ID proves correct interpretation or requirements satisfaction.

Likewise, a policy proof can establish the disposition derived from admitted facts; it does not prove that a pronoun was resolved correctly or that an implementation passes its tests. Keep those claims separate.

### Migration, failure and supersession

- Legacy threads with one intact asking round and saved Question/Options can potentially derive the carrier deterministically after identity/consistency checks. Do not silently choose among conflicting records.
- If the exact published options cannot be recovered, disclose partial context. Do not substitute today's candidates and call them the historical choices.
- A failed resolution must remain unknown, with the existing recoverable owner retained; it must not become an empty successful answer context.
- Preserve Unicode, punctuation, negations and qualifications exactly. Human-readable summaries may supplement, not overwrite, the source.
- If a supported future user correction supersedes an answer, retain the relationship and expose the effective version. This proposal does not add a new question, new authorization route or automatic retry policy.
- Keep mission-owned continuations isolated from ordinary chat discovery. Owner identity must constrain any reference resolver.

## 7. Recurse and portability scope

The same service is wired into recurse mission preparation, and mission receipts retain the deliberation thread path. Mission source/vision checks, write scope, protected paths, gate contracts and promotion remain separate safeguards. A question can stop a mission and be retained for its owner; ordinary conversational discovery deliberately excludes such records. [Mission constructor][mission-constructor] [Mission preparation][mission] [Owner filtering][save]

The concrete reachable witness here is the saved chat/design-only free-text continuation. This review does not claim a fully wired generic mission-answer resumption path, nor that a mission executed a wrong change. If such an owner-addressed continuation is added or extended, it should use the same answered-interaction delivery contract rather than bypassing owner isolation.

The gap precedes language-specific execution. It can affect a preference about Go compatibility, Python import behavior, Rust feature defaults or JavaScript package interfaces, but those domains do not repair the conversation record. Conversely, preserving the question does not establish that any foreign-language verifier supports the resulting acceptance condition. Evaluate these independently, as the portable-crucible proposal recommends. [Portable proposal][portable]

## 8. Full cost and failure accounting

The baseline already pays for proposer/challenger role invocations, role-local model/tool cycles, optional specialist work, JIT compilation/retrieval, kernel assertion/query/proof work and repeated whole-thread snapshots. The source's role budget counts tool executions; a role invocation is not automatically one model request, and a consultation is not necessarily one request either. Count actual calls and tokens. [Agent loop][agent] [Conductor][run] [Snapshot writer][save]

Let B be the serialized size of the proposed question/option binding beyond the already-carried answer; let K be the number of later provider requests that carry it. Inline delivery adds approximately K times B logical input bytes before provider-specific tokenization and caching. The question can already appear in Previous during the first resumed round, so deduplicating that body is possible only with an explicit representation that preserves the relation.

Host serialization and optional hashing scale linearly with the binding's bytes. The view's additional live memory is bounded by its admitted size. Persistent metadata is small when the existing Question/Options/Answer fields remain authoritative; copying the full interaction into every saved round would instead multiply storage and full-snapshot write volume. The current writer rewrites the whole serialized thread on each save, so even a modest new field is paid on repeated writes. These are analytic costs, not measurements.

A reference-based treatment reduces repeated inline text but adds lookup latency, I/O, availability handling and possibly role-local tool/model turns. Moving work behind a handle is not free. If the resolver is unavailable, the intended savings can become a visible stop rather than a successful design.

Measure at least:
- all billed/visible input, output and reasoning tokens, cache reads/writes and monetary cost where available;
- role, specialist and tool call counts, retries, JIT/kernel/proof latency, total latency and tail latency;
- serialized frame bytes, exact-source coverage and over-bound/refusal rates;
- snapshot bytes/writes, resolver reads, retention overhead and recovery latency;
- user questions repeated, user effort required to restate context, wrong-subject interpretations, unjustified assumptions and unresolved outcomes.

Potential benefit is fewer reconstructed/repeated questions and fewer preference-violating designs. Potential cost is reduced room for current evidence and higher input/lookup cost. Neither direction has been measured. A treatment that improves contextual-answer accuracy by increasing the total budget or replaying all history should not be presented as an equal-cost win.

## 9. Fair evaluation plan, all unrun

### Deterministic carrier acceptance

Extend the existing production-boundary fixture style rather than relying only on a separate mock planner. The current chat tests already script model boundaries while using routing, JIT, shards, policy, reads and persistence. Its values test answers an exact rendered option and checks that no extra role call occurs; that useful test does not exercise this multiround free-text horizon. [Existing fixture][chat-tests] [Exact-choice test][chat-choice-tests]

Required cases:

| Case | Required observation |
|---|---|
| First resumed frame | Existing previous-round question remains available; new carrier does not distort it. |
| Second and later resumed frames | Same original interaction remains identifiable after Previous changes. |
| Changed recommendation/display order | Binding retains the originally shown option sequence. |
| Extra qualification on an option | Full answer survives as free text; do not strip the qualification to force the fast path. |
| Exact recognized option | Preserve existing efficient replay and normal permission holds. |
| Missing/corrupt/inconsistent legacy context | Named unavailable/ambiguous outcome, not guessed historical context. |
| Tight context and escaped/Unicode text | Bound and terminal reserve hold; no silent source-byte loss. |
| Owner/session mismatch | No foreign interaction is supplied or resolved. |
| Failed evaluation or resolution | Keep recovery separate from context availability; do not reset budgets. |
| No values question | No unnecessary binding or additional model call. |

Capture the actual serialized frame, not only the final answer. Verify source fidelity and authority preservation separately from semantic quality. A passing carrier test proves delivery, not that a model used the information.

### Matched semantic comparison

Compare:
1. current one-previous-round behavior;
2. the same behavior with only stronger prompt reminders;
3. the proposed bounded answered-interaction carrier;
4. a full-history replay reference, labeled as potentially higher-cost rather than treated as a free baseline.

Use identical repository snapshots, initial requests, user-answer scripts, configured providers/model profiles, tools, JIT/world-model/CodeDOM setup, permissions and total resource ceilings. Include contextual and self-contained answers, one and several refinement rounds, multiple subjects, counterbalanced option orders, and legitimate unresolved designs. Include concise cases where the current implementation succeeds.

A useful first semantic test supplies matched captured frames with only the answer-context treatment varied, to isolate understanding. Then use normal /deliberate, /innovate and ordinary uncertainty-resolution paths to measure workflow effects. Do not generalize a restricted scripted readout to all production tasks.

Use held-out answer phrasings and repositories. Have evaluators inspect the original question, actual displayed options, exact answer and final candidate, without knowing the treatment. Score preservation of the answered requirement, attachment to the correct subject, unsolicited reopening, unauthorized-scope expansion and honest unresolved results. Report task-level paired outcomes and uncertainty, including failures/timeouts, not only successful completions or pooled per-round accuracy.

The decision criterion should be set before evaluation: the binding is required for carrier correctness; any claimed quality or efficiency benefit additionally needs improved or non-inferior relevant outcomes under matched costs, with no authority/context-bound regression. Do not borrow published reasoning benchmarks as a result for this implementation.

## 10. Primary literature and limits of transfer

- **Gotel and Finkelstein, An Analysis of the Requirements Traceability Problem (1994).** Their distinction between requirements' pre-specification origins and later specification traceability motivates retaining the interaction from which a constraint arose. It does not prescribe this schema or establish a codeNERD failure rate. [Original paper][traceability]
- **Clark and Brennan, Grounding in Communication (1991).** Their account treats communication as coordinated establishment of sufficient common understanding for a purpose, with costs for misunderstanding and repair. The engineering inference here is to preserve the question-answer relation instead of treating a contextual answer as standalone text. This is not a controlled study of LLM context projection. [Author-hosted chapter][grounding]
- **Yao et al., Tree of Thoughts (2023).** Its exploration and evaluation of multiple intermediate paths are relevant background for a divergent/convergent design process. Its reported tasks are different from interactive requirements clarification. It neither validates a human-answer retention contract nor justifies extra deliberation without accounting for the full cost. [Paper][tot]

These sources motivate the design and evaluation questions. The concrete finding comes from the pinned producer/consumer code, not analogy to literature.

## Exit criterion

The original values interaction remains explicitly available to both roles after the asking round leaves Previous, within the same context, stopping, ownership and permission constraints. Exact-choice replay remains efficient, unavailable context is visible, and production-path evaluations separate delivered context from semantic success and implementation verification.

A correct refinement should let the assistant remember what its own question meant without asking the person to perform that bookkeeping again.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/51020eb9acea19fcca802a2a5a92a1f336d305cf
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/agents.md#L3-L62
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/architecture/AGENTS.md#L1-L110
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L900
[shard-spec]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/architecture/shards/IMPLEMENTED_SPEC.md#L483-L519
[live]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/.claude/skills/codenerd-dogfood/references/component-ledger.md#L6333-L6342
[types]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/types.go#L48-L116
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/round.go#L10-L72
[run]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/service.go#L77-L165
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/outcome.go#L33-L86
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/cmd/nerd/chat/deliberation.go#L19-L137
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/cmd/nerd/chat/model_handlers.go#L462-L503
[save]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/persistence.go#L23-L173
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/types.go#L144-L181
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/policy.go#L10-L71
[schemas]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/core/defaults/schemas_deliberation.mg
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/core/defaults/policy/deliberation.mg
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/deliberation/read_tools.go#L85-L210
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/config/routing.go#L18-L32
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/prompt/atoms/system/deliberation_proposer.yaml#L13-L48
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/prompt/atoms/system/requirements_interrogator.yaml#L13-L51
[mission-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/internal/campaign/recurse_mission.go#L212-L312
[chat-tests]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/cmd/nerd/chat/deliberation_test.go#L28-L86
[chat-choice-tests]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/cmd/nerd/chat/deliberation_test.go#L179-L219
[recovery-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-answer-evaluation-recovery.md
[options-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-values-option-admission.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-values-evidence-freshness.md
[retirement-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-critical-item-retirement.md
[subject-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-consultation-subject-binding.md
[brief-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/conversation-brief-semantic-fidelity-and-progressive-disclosure.md
[history-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/conversation-history-coverage-handoff.md
[horizon-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-probe-consumption-horizon.md
[capacity-audit]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/deliberation-probe-frame-capacity.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits/portable-recurse-deliberation-crucible.md
[inventory]: https://github.com/theRebelliousNerd/codenerd/tree/51020eb9acea19fcca802a2a5a92a1f336d305cf/Docs/audits
[traceability]: https://discovery.ucl.ac.uk/id/eprint/749/1/2.2_rtprob.pdf
[grounding]: https://web.stanford.edu/~clark/1990s/Clark%2C%20H.H.%20_%20Brennan%2C%20S.E.%20_Grounding%20in%20communication_%201991.pdf
[tot]: https://arxiv.org/abs/2305.10601v2

