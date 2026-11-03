---
doc-class: research-proposal
subsystem: deliberation-consultation
implementation-status: proposed
last-verified: 2026-11-03
verified-against: 9dba8b3255f0d11eaf076d817d4036f149b0750b
verification-method: static-source-inspection
---

# Bind deliberation consultations to the question and candidate

## Finding and decision

**The current challenger frame does not explicitly retain the question that produced specialist advice.** The proposer can request a specialist's opinion about a candidate. The conductor executes that request, then projects its result to `ProbeResult{Key: specialist, Tool: "consult", Output: advice}`. It puts those observations and the current candidates in the challenger frame, but not the current consultation requests. Different questions to the same specialist can therefore produce identical challenger evidence, even when their answers apply to different candidate designs. [Round collection][collect] [Data shapes][types]

There is a related upstream limit at the same subject boundary: the consultation callback receives the original request and the consultation's specialist/question fields, not the current candidate records. Both production adapters forward that original request as specialist context. A question that refers to a newly proposed candidate by its stable ID does not automatically carry the candidate's design. A self-contained question can avoid that limit; the typed contract does not require or establish that it does. [Service callback][service] [Chat adapter][chat] [Mission adapter][mission]

**Classification: a source-confirmed subject-information loss and a proposed subject-binding contract.** This is not evidence of a wrong live design, a specialist-cache defect, permission bypass or false implementation-verification result. The precise downstream finding is that the explicit challenger frame cannot distinguish the question in the witness below. The upstream finding concerns the explicit consultation task, not proof that every possible JIT or retrieval source is identical.

The narrow refinement is to preserve one bounded request/result relation: which question was asked, about which current candidate snapshot or explicitly request-wide subject, and which result answered it. Use the existing proposer, consultation manager, challenger, JIT compiler and Mangle decisions. A specialist's identity is not the subject of its advice, and repeating the specialist call is not a substitute for carrying the question already paid for.

## Source and verification boundary

Public main resolved to **9dba8b3255f0d11eaf076d817d4036f149b0750b**, committed at 16:26:08 UTC on November 3, 2026. It remained at that revision when rechecked at approximately 16:46 UTC. All repository citations below use that immutable revision. [Commit][pin]

This review read public root and relevant scoped guidance, current session/shard/mission architecture, the deliberation package and its production adapters, the shared consultation request/result path, relevant authored tests, the audit inventory and neighboring reports. No codeNERD execution, build, test, model inference, benchmark, gate or failure injection ran. No implementation, credential, permission, configuration or setting was changed. All example outcomes, APIs and evaluations proposed below are unrun.

The session specification's older runtime-pending statement is not a reason to erase later positive evidence: the repository's live-exercise ledger records a fifth deliberation run converging in two rounds after earlier defects were fixed. That is owner-reported evidence for that exercise, not a receipt for consultation-subject attribution, and it was not rerun here. [Session spec][session-spec] [Live exercise][live]

## 1. Why this matters to Socratic requirements work

Steve's repository guidance separates creative problem solving from executive control and requires relevant context to be bounded, revision-aware and recoverable. The proposer is specifically instructed to compare materially different candidates, give them stable identities and explicit success conditions, and optionally ask a concrete specialist question about a candidate. The challenger then assesses each candidate against the complete request and available evidence. [Root guidance][guidance] [Proposer atoms][proposer] [Challenger atoms][challenger]

That division only helps if the evidence's subject survives the handoff. “This strategy is safe” can be a useful answer to a well-specified question; it is insufficient to tell a different participant which of several strategies was reviewed. Likewise, asking about `c1` is useful only when the recipient can identify what `c1` denotes in this round. Candidate IDs are local handles, not globally meaningful descriptions.

This is especially relevant to foreign repositories. A specialist might advise on a Python package initializer, Rust feature arrangement, JavaScript workspace boundary or Go cancellation policy. Language expertise does not identify an unpublished in-memory design. The same relation is needed across languages; it should not depend on a Go package name or a specialist inferring which candidate the proposer probably meant.

The current specification describes consultation advice as evidence supplied to the challenger, not authority to change the workspace. It does not explicitly promise the subject-binding protocol proposed here. The report therefore preserves current behavior and defines the missing checkable guarantee rather than describing the entire feature as broken. [Shard spec][shard-spec] [Architecture guidance][arch-guidance]

## 2. Exact producer-to-consumer chain

### 2.1 The proposal owns the question, but no typed subject

`Proposal` contains both `Candidates` and `Consultations`. Each candidate has an ID, brief, target, success test, assumptions and evidence. Each consultation has only a specialist and question. There is no candidate ID, candidate revision, request-wide scope marker or host request ID on that consultation. [Data shapes][types]

`collectRound` decodes the proposal and checks candidate and consultation list bounds. It copies only `r.Proposal.Candidates` into `frame.Candidates`. Its consultation loop has the complete proposal available, but calls:

`s.Consult(ctx, consultation, t.Request)`

The supplied context is the original request, not the generated proposal or a selected candidate. [Collection and dispatch][collect]

This is a concrete absence, not a claim that every question lacks context. The proposer may quote the candidate's full design in `Question`. The problem is that neither the request type nor this boundary establishes which design was actually supplied.

### 2.2 Both production adapters preserve that limited input

The chat constructor and recurse-mission constructor both create a shared-manager request from the same fields: requester is the proposer role, specialist and question come from the consultation, and context is the supplied original request. They inherit and merge a read-only restriction. Neither adds current candidate content or a candidate snapshot reference. [Chat adapter][chat] [Mission adapter][mission]

The shared manager is not dropping a field it received. Its task builder renders `Question` and `Context`, and its content key covers requester, specialist, question, context and response contract with versioned, length-delimited hashing. It produces per-consumer request IDs and response receipts and coalesces only identical in-flight work. Completed advice is not cached. Those are working distinctions to preserve. [Prompt and content key][consult-prompt] [Manager][manager] [Receipt types][consult-types]

However, a content hash can only distinguish information supplied to it. Two current candidate snapshots omitted from all those fields cannot become distinct consultation content keys merely because they exist in the parent proposal.

### 2.3 The return handoff drops the question

On return, the conductor constructs a `ProbeResult` with specialist as `Key`, the literal tool name `consult`, advice as `Output`, and an error string when appropriate. It then assigns `frame.Evidence = r.Probes` and calls the challenger. [Collection and dispatch][collect]

`Frame` contains current candidates, evidence, original request/intent/brief, answer, receipt facts and the previous round. It has no field for the current consultation list. On the first round there is no previous round that might incidentally repeat the question. [Frame][frame]

The observation does not carry the manager's request ID or content key either. Returning those identifiers alone would still not tell the challenger the question unless an actual, authorized resolution route existed.

### 2.4 Durable history is a real limit on the finding

After a round is collected, validated and judged, the service appends the complete `Round` to the thread and saves it. That round retains `Proposal.Consultations`, the candidates and its probes. The next convergent frame includes that previous round. Thus this report does **not** claim permanent loss of the question from successfully completed-round history. [Round shape][types] [Save order][service] [Persistence][save]

But the current challenger runs before that save and before the question appears as a previous-round field. Its verdict may be ready in the first round, ending the design flow. A later recovery route or a possible extra round does not establish delivery to the decision currently being made. [Current challenger dispatch][collect] [Ready path][service]

For multiple consultations, current execution order can help a future reader associate the executed prefix of requests and results. That is not an explicit result identity, and it does not help the current challenger when the corresponding request list is absent. No concurrency, result reordering, duplicate specialist or partial failure is needed for the primary witness.

## 3. Two minimal source-derived witnesses

These are proposed request-capture fixtures, not tests that were run. Use normal valid candidates, a registered available specialist, stable workspace, generous remaining context, successful calls and ordinary short strings. Keep all permissions and existing budgets unchanged.

### Witness A: different questions, identical challenger frame

Keep the original request and both candidates identical in two otherwise isolated first-round runs:

- Candidate `c1`: an in-process cache with local invalidation.
- Candidate `c2`: a shared cache with cross-process invalidation.
- Both candidates have distinct IDs and nonblank briefs/success tests; any other fields are held constant.

The original user request asks the system to compare those approaches. In run A, the proposal asks the registered cache specialist:

> For c1, is the proposed invalidation approach sufficient under the single-process requirement?

In run B it asks the same specialist:

> For c2, is the proposed invalidation approach sufficient under the multi-process requirement?

These are illustrative fixture questions, not factual claims about a real cache. Make the deterministic specialist seam return the same successful advice in both cases, such as “The stated invalidation approach is sufficient for the stated requirement.” It may be correct for both questions. No caveat or reference difference is required.

The consultation tasks differ, as they should. Yet each conductor produces the same evidence record: same specialist key, `consult` tool, advice and absent error. The current candidate list, original request, intent, brief and phase also match, and first-round `Previous` is nil. Consequently, the two explicit challenger frames are identical. [Dispatch and frame construction][collect] [Frame fields][frame]

**The missing discriminant is which candidate/requirement the advice answers.** There is no requirement that the challenger approve either candidate. It might independently inspect both designs, treat the advice as unusably vague, or ask for more evidence. The proven static limitation is that this input cannot attribute the already-acquired advice to the different question. Model error, repeat-read rate and extra cost remain empirical questions.

This witness still holds if the existing qualifier-loss proposal is repaired while the two complete specialist replies have identical qualifications. It also needs no role-local tool read, so adding arguments to role-read receipts does not supply the missing consultation question.

### Witness B: changed candidate, unchanged explicit specialist task

Keep the original request, specialist and consultation question constant:

> Does c1 preserve cross-process invalidation?

In proposal A, `c1` describes local-only invalidation. In proposal B, `c1` describes a versioned shared invalidation channel. The candidate snapshots differ materially, but their local display ID remains `c1`. The second candidate can remain unchanged.

The current service and adapters supply the same original request and question in both runs. The manager's explicit consultation task and content key are therefore identical. [Callback dispatch][collect] [Adapters][chat] [Mission adapter][mission] [Prompt and key][consult-prompt]

This does not claim the entire specialist model request, every dynamic JIT result or execution outcome must be identical. It establishes that the designated consultation channel conveys no difference between the candidate snapshots. A specialist may recover useful workspace facts, but workspace inspection alone does not identify which unpublished design the parent just called `c1`.

A deliberately self-contained question quoting the candidate's actual design is an important positive control: its task and content key differ today. The proposed contract must preserve that success without depending on every proposer to duplicate all necessary candidate context correctly.

### Controls that prevent overclaiming

- If there are no consultations, this boundary is unused.
- A request-wide question can be valid without a candidate subject; it should be explicitly represented as such, not rejected.
- If the question embeds all needed candidate content, upstream ambiguity can be absent.
- If advice explicitly repeats its exact question and subject, downstream ambiguity can be absent.
- Current candidates may be sufficient for the challenger to reach the right conclusion independently.
- Saved completed rounds retain proposal questions; later frames may recover them.
- Read-only restrictions, constitutional permission, mission source checks and downstream verification are independent controls. Losing subject information does not itself defeat them.
- An unambiguous subject does not establish that the expert's answer is correct, current, complete or independently verified.

## 4. Novelty and neighboring work

This is a focused continuation of existing evidence-continuity work, not a new orchestration framework.

| Existing report | Existing finding | Distinct boundary here |
|---|---|---|
| Deliberation consultation qualifier continuity | Production adapters retain advice but discard caveats and references. | The complete answer can have identical qualifiers and still need its originating question and current subject. The older report explicitly says exact question association can compose with its correction. |
| Deliberation read invocation identity | Role-local tool observations omit the invocation arguments. | The primary witness makes no role-local tool call. The proposer-owned consultation request crosses a different service callback and is omitted from the current challenger frame. |
| Consultation coordination and safe reuse | Shared artifacts, request receipts, cancellation and computational equivalence. | The current shared manager correctly distinguishes the fields it receives. The parent fails to supply a current candidate distinction and later discards the question relation. Do not reopen resolved cache/coalescing defects. |
| Terminal knowledge synthesis grounding | Parent evidence and consultation coverage must survive terminal synthesis. | This is the separate proposer → specialist → requirements-challenger path, with current candidate context and a typed frame. |
| Portable recurse deliberation/crucible | A broader evidence-producing design funnel and language-independent evaluation. | This report supplies an exact currently wired consultation-subject gap and falsifiable request-payload witnesses inside that funnel. |

The relevant reports and current audit inventory were inspected; this is a bounded duplicate screen, not a claim about every historical discussion or unpublished branch. The repair should be coordinated with qualifier and read-receipt work so one evidence schema is not independently reinvented three times. [Qualifier audit][qualifier-audit] [Read audit][read-audit] [Coordination audit][coord-audit] [Synthesis audit][synthesis-audit] [Portable proposal][portable]

## 5. Proposed contract, not implemented behavior

### Required invariant

For each consultation observation supplied to a challenger, the receiver can identify the exact admitted question and its intended subject as supplied to the specialist, or receives an explicit unavailable/ambiguous disposition. A successful transport alone must not imply that a particular candidate was reviewed.

### Ownership and validation

1. **Proposal declares subject.** A consultation names an explicit request-wide subject, one or more current candidate IDs, or another permitted typed subject. The proposer still creates the question. Do not infer subject from specialist name, list order or lexical mention of `c1`.
2. **Host resolves the subject before dispatch.** Validate candidate membership and freeze the relevant current snapshot. Preserve at least the material design, target, success conditions and assumptions needed to interpret the question, either inline or through a genuinely resolvable bounded reference. Stable display ID alone is not a revision identity.
3. **Specialist receives the admitted question/subject relation.** Extend the existing callback contract rather than depending on extra prose in an unrelated global request. Keep original request constraints and read-only restrictions.
4. **Result remains attached to that request.** Retain an opaque host receipt identity, exact question or bounded resolvable question reference, subject scope and snapshot identity alongside the qualified result and disposition. The host records what it actually supplied, not what the specialist later claims it saw.
5. **Challenger receives the relation in the current round.** It must not have to wait for a future `Previous` field. The JIT role contract explains the data without treating advice or embedded question text as instructions.
6. **Mangle retains executive authority.** The host validates identity and transports evidence. The kernel decides eligibility, continuation and disposition. A subject-bound result is still a model opinion, not a new success fact or permission token.
7. **Persistence and replay preserve scope.** Completed rounds retain the same relation. Candidate replacement or a revised material design makes old advice historical until its applicability is explicitly accounted for. This report does not authorize automatic extra calls or reset existing budgets.

The exact data shape and predicate additions require spec-first design in the existing owning corpora. No schema, API or migration was implemented by this audit.

### Bounded representation choices

**Inline minimal subject** is the simplest first treatment: carry the original question and the relevant candidate snapshot in the consultation request; return the request identity and question/subject relation with the result. It adds bytes but avoids a new lookup dependency.

**Retained subject reference** can avoid repeated candidate bodies when several consultations review the same snapshot. It is valid only if the receiver has an authorized, usable retrieval route and the host records whether the needed content was actually available. A digest or file path alone is not delivered meaning.

**Self-contained-question linting** can catch blank or obviously missing information, but cannot deterministically prove that arbitrary natural language fully specifies a design. Requiring the model to mention the candidate more carefully is a weaker baseline to evaluate, not a substitute for host subject binding.

No option requires a new agent, global transcript replay, completed-answer cache, broader tool envelope or an additional model judge.

## 6. Failure and migration behavior

| Situation | Required proposed disposition |
|---|---|
| Unknown candidate ID | Reject or return an explicit unresolved-subject result before buying a consultation; do not substitute the first candidate. |
| Request-wide or cross-candidate question | Preserve its declared broader scope and supplied subjects; do not force an arbitrary single candidate. |
| Candidate keeps its ID but changes materially | Preserve the old reviewed snapshot and prevent automatic attribution to the revised design. |
| Same specialist answers two questions | Keep distinct request/result identities even if outputs are identical. |
| Specialist returns an error or cancellation | Preserve the question/subject relation with the failure; do not treat the absence of advice as favorable review. |
| Consultation is deferred by existing allowance | Retain its unattempted disposition without fabricating a response or automatically raising budgets. |
| Subject cannot fit the remaining context | Narrow explicitly, use an admitted reference, or disclose unavailable context. Do not retain positive advice while silently removing what it applies to. |
| Reference is missing, expired, inaccessible or from another owner | Mark unavailable; do not dereference across scope or guess from a similar candidate. |
| Untrusted question or candidate text contains instructions | Keep it as task data under the existing JIT/read-only/permission boundary. |
| Legacy completed round has requests/results but no explicit join | Preserve it as history. Any reconstruction must state its evidence and ambiguity; do not stamp a new reviewed-snapshot guarantee onto old records. |
| Save or challenger evaluation fails after a consultation | Report the actual stage and preserve current failure semantics; this proposal does not imply that all intermediate work was already durably saved. |

The existing option/answer recovery and source-freshness audits remain relevant but separate. Subject binding does not authorize reusing stale advice or silently resuming stopped work. [Values freshness audit][freshness-audit] [Answer recovery audit][recovery-audit]

## 7. Full cost accounting

The baseline already pays for proposer and challenger role invocations, optional specialist execution, role-local and conductor reads, JIT compilation, fact evaluation, explanation and repeated full-thread snapshot writes. The consultation list is capped by the candidate allowance and dispatch is further gated by the tool-call allowance. A specialist dispatch is not necessarily one model call: it enters the configured consultation execution path. Counting only the conductor callback would understate cost. [Collection][collect] [Production adapters][mission] [Agent loop][agent] [Save][save]

Let C be dispatched consultations, Q their total question bytes, U the total bytes of distinct subject snapshots, and D the bytes duplicated when snapshots are repeated inline.

- **Host work:** subject validation, freezing/copying, hashing and reference lookup. Inline construction is proportional to the admitted question/subject material, not just the number of IDs.
- **Model input:** the specialist gains subject bytes it previously lacked; the challenger gains question and subject association. An inline treatment can add approximately Q + U + D across requests, plus request/result metadata; actual serialized and provider input must be measured. The candidate bodies already in the challenger frame should not be copied again unnecessarily.
- **Context opportunity cost:** added provenance can displace useful evidence or cause earlier bounded-context closure. Measure actual serialized bytes and provider token counts. Bytes are not a universal token measure.
- **Storage and persistence:** request/result metadata and snapshots increase retained thread size; complete snapshot rewrites amplify that increase across saves. Referenced storage adds retention, lookup and unavailable-reference handling.
- **Kernel and proof costs:** any new admission facts add joins, invalidation/retraction and explanation work. Do not claim a constant-time or free policy check without measuring the chosen representation.
- **Specialist reuse:** including current subject identity prevents false equivalence for omitted designs but may reduce coalescing opportunities. The existing manager already includes exact question/context fields; preserve that scheme and extend its semantic inputs deliberately.
- **Latency and recovery:** added validation should not cause new model calls. If a subject is unavailable, explicit narrowing/recovery may require calls later, only within existing authorization and budgets. Count failed, cancelled and repeated work.
- **Human cost:** measure extra clarifications and review effort as well as machine spend. The correction must not turn missing repository data into a preference question for the user.

The benefit hypothesis is better attribution and less repeated investigation, not fewer bytes. Correctly carrying a question can increase token use and still improve the design decision. No token, latency or reliability improvement has been measured.

## 8. Fair, unrun evaluation plan

### A. Deterministic request-boundary checks

Use scripted proposer/specialist/challenger seams to capture actual requests, not a hand-constructed duplicate of the serializer.

1. Execute Witness A through `collectRound` and confirm the old challenger-frame collision. Require the proposed path to retain the question/subject distinction.
2. Execute Witness B through both actual adapter constructions and the shared manager's prompt builder. Require material subject changes to alter the supplied specialist task or resolved subject reference.
3. Preserve positive controls: self-contained questions, self-describing answers and genuinely request-wide questions remain usable.
4. Request two opinions from the same specialist in one round; return identical advice, different advice, an error, and a cancellation. Require one attributable disposition per admitted request.
5. Vary only question, candidate target, success test, assumption or material brief. Identify which changes the proposed subject contract considers relevant; do not hide semantic policy in an opaque hash.
6. Capture the first challenger input, then separately inspect saved/reloaded completed rounds. Persistence is not proof of delivery, and delivery is not proof of durability.
7. Cover long Unicode questions, escaped strings, multiple candidate subjects and near-bound inputs. Assert actual encoded bounds and explicit missing-context behavior.
8. Exercise stale/missing reference, foreign-owner reference and legacy-round cases. Never convert a guess at an old array pairing into a fresh assurance receipt.

The existing deliberation tests cover values, stops, probes, persistence and commands. The shared-manager tests cover content identity, receipts and concurrent behavior. Those are useful seams and controls, not executed evidence for the new relation. The inspected tests do not establish delivery of a current candidate snapshot to a consultation followed by its question/result join to the first challenger frame. [Deliberation tests][tests] [Chat tests][chat-tests] [Manager tests][manager-tests]

### B. Production route and portability

Run the normal design-only `/deliberate` and `/innovate` routes, ordinary uncertainty resolution and recurse-mission preparation with production JIT, world-model and CodeDOM context enabled. Keep mission source/gate restrictions and downstream execution admission intact. Restricted scripted fixtures isolate the boundary but cannot replace those routes. [Chat routes][chat] [Mission constructor][mission] [Mission spec][mission-spec]

Use held-out Go, Python, Rust, JavaScript/TypeScript and mixed-repository requirements questions. Include designs that exist only in the proposal, because a source-only suite can mask omitted candidate context by letting every specialist recover the answer from files. Include cases where consultation adds no useful information so the evaluation does not reward unnecessary specialists.

### C. Equal-budget comparisons

Compare:

- the current baseline;
- the baseline plus a prompt instruction requiring self-contained questions and answers;
- bounded inline question/subject/result binding;
- a reference-based treatment only if its retrieval and retention contract is actually implemented.

Use the same models, provider settings, task/snapshot set, candidate/round/tool limits, permissions, prompt corpus outside the changed contract and total end-to-end budget. Randomize task order; use blinded independent review and observable requirement checks. Do not score the same challenger that produced a verdict as the sole quality oracle.

Report subject-attribution accuracy, unsupported candidate-specific claims, correct and incorrect readiness, false obstruction, unresolved outcomes, unnecessary repeat reads/consultations and independently accepted designs. Include every attempted task, including failures, stops, context refusals, cancelled runs and unsupported cases.

Report full calls, tokens, provider-reported usage categories, JIT/tool/consultation latency, host memory/copying, serialized input bytes, persistence bytes and human decisions. Unknown usage stays unknown. Report both equal-budget quality and total cost to reach the same independently accepted endpoint. A shorter failed design session is not an efficiency win.

## 9. Primary literature and its limits

- **W3C PROV-DM** distinguishes entities, activities and responsible agents and links outputs to the activities and inputs that produced them. It supports the design distinction between “which specialist” and “which question/subject was reviewed.” It does not mandate this schema or establish model-answer truth. [PROV-DM](https://www.w3.org/TR/2013/REC-prov-dm-20130430/)
- **Madaan et al., Self-Refine** studies iterative generation, feedback and refinement with the same model. It motivates preserving the subject of feedback through a refinement loop. Its reported benchmark gains do not establish a benefit for this codeNERD transport change. [Paper](https://arxiv.org/abs/2303.17651)
- **Yao et al., Tree of Thoughts** studies search over alternative reasoning states with evaluation. It provides context for comparing candidates while retaining the state being evaluated. It is not evidence that extra experts, more branches or this proposed metadata are automatically better. [Paper](https://arxiv.org/abs/2305.10601)

These sources were checked directly. They inform proposed contracts and evaluation design, not the static source finding, which follows from the repository's own producer/consumer fields.

## 10. Recommended decision and checkable exit

Specify a bounded consultation subject/result relation in the existing deliberation and adapter contracts, coordinating it with the already proposed qualifier-preserving result. Preserve the manager's current content-key and per-consumer receipt semantics. Implement only after that spec decision; this audit supplies no implementation.

**Exit criterion:** through both production adapters, a candidate-specific consultation receives the admitted current subject, and the current challenger can determine which exact question and supplied subject produced each result. A broader request-wide consultation remains valid. Missing or unavailable subject information is explicit. Completed-round replay preserves that relation without silently promoting historical advice to current evidence. Permissions, existing budgets and independent implementation verification remain unchanged. Any quality or efficiency claim requires the matched evaluation above.

## Repository source links

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/9dba8b3255f0d11eaf076d817d4036f149b0750b
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/agents.md#L1-L69
[arch-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/architecture/AGENTS.md#L1-L116
[session-spec]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L895
[shard-spec]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/architecture/shards/IMPLEMENTED_SPEC.md#L486-L519
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[live]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/.claude/skills/codenerd-dogfood/references/component-ledger.md#L6333-L6342
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/prompt/atoms/system/deliberation_proposer.yaml#L12-L49
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/prompt/atoms/system/requirements_interrogator.yaml#L12-L52
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/round.go#L10-L72
[types]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/types.go#L17-L93
[frame]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/types.go#L104-L119
[service]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/service.go
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/cmd/nerd/chat/deliberation.go
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/cmd/nerd/cmd_campaign_mission.go#L40-L68
[consult-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/consultation/prompt.go#L13-L67
[manager]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/consultation/manager.go#L36-L111
[consult-types]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/consultation/types.go#L40-L132
[save]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/persistence.go#L26-L75
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/read_tools.go#L84-L208
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/deliberation/deliberation_test.go
[chat-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/cmd/nerd/chat/deliberation_test.go
[manager-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/internal/consultation/manager_test.go
[qualifier-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/deliberation-consultation-qualifier-continuity.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/deliberation-read-invocation-identity.md
[coord-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/consultation-coordination-and-safe-reuse.md
[synthesis-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/terminal-knowledge-synthesis-grounding.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/portable-recurse-deliberation-crucible.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/deliberation-values-evidence-freshness.md
[recovery-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9dba8b3255f0d11eaf076d817d4036f149b0750b/Docs/audits/deliberation-answer-evaluation-recovery.md

