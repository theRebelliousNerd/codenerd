---
doc-class: deep-dive
subsystem: deliberation-session
implementation-status: proposed
audit-status: source-grounded-conditional
audit-date: 2026-11-03
verified-against: 1540beaad60f3a01ab5ffdc162800225c0b74c90
verification-method: read-only-source-inspection
---

# A values answer must select an offered, admitted design

## Decision and scope

Bind a deliberation values answer to the alternatives actually admitted for that question. Do not let membership in the larger proposal list substitute for membership in the question's eligible option set.

**Finding:** the conductor filters the displayed options through `deliberation_eligible(..., /values_fork, ...)`, but its exact-answer parser searches every proposed candidate. The answer-to-`proceed` rule likewise requires only a proposed candidate, saved success and no critical non-values blocker. It does not require that the answered candidate was eligible for the values question or offered to the person. A structurally valid third candidate with success, no blockers and no citation is neither ordinary-ready nor a displayed values option, yet a custom answer exactly matching its ID or rendered option label can enter the saved-round fast path and derive `proceed`. The host then labels that candidate `ready`. [Question and parser][outcome] [Answer replay][service] [Policy][policy]

This is an **admission-domain mismatch**, not a claim that a values answer bypasses codeNERD's permission checks. The original verb and restrictions are preserved, design-only threads do not launch development work, and propose-mode mutations still use ordinary work admission. Those controls should remain. No bad implementation, escaped permission check, retained recurse improvement, live-model failure rate or measured savings is claimed. The conditional witness below is derived from source, not executed. [Adoption and design-only return][chat] [Existing permission control][chat-test]

The defect matters to the existing Socratic workflow because the question is supposed to resolve a preference among already viable designs. An answer may supply a preference; it cannot supply the missing design admission of an alternative the conductor excluded. A recorded kernel proof is only as strong as the premises of the rule it proves.

### Pin and inspection boundary

The initial public `main` read resolved to `f4d0ca1d7fba2a208febda4f11f879af23b15e06`. Rechecks during this audit observed `5df765e11a6e5f99064e49fec0dbd2862fe2bbb1` and then **`1540beaad60f3a01ab5ffdc162800225c0b74c90`** at approximately 09:19 UTC on 2026-11-03. Both successive GitHub comparisons showed exactly one changed path, `Docs/audits/ASTRA_QUEUE.md`, which was reread at each pin. All implementation and architecture sources discussed here are unchanged across those comparisons. Links below use the final pin. [Pin][pin] [Comparison][comparison]

This pass read public repository guidance, the session specification, existing relevant audits, JIT role atoms, deliberation source, chat integration, persistence and authored tests. It did not run tests, codeNERD, model inference, benchmarks or experiments; modify repository files; use credentials; or change settings. The only deliverable is this scratch Markdown audit.

## 1. Architectural contract being strengthened

The root guidance puts creative synthesis in the model and executive planning, orchestration and safety in the Mangle kernel. It requires typed operations and distinguishes evidence from models' descriptions. The session specification already describes a read-only proposer/challenger conductor, typed round facts, policy-owned decisions, values alternatives, a single options question outside yolo, durable continuation and normal work admission after clarification. This audit strengthens those existing boundaries; it does not propose a competing planner or more agents. [Root guidance][guidance] [Session specification][spec]

The proposer is instructed to preserve the whole request, provide stable candidate IDs and source evidence, and keep viable alternatives when only a preference remains. The requirements interrogator assesses success against the full request and evidence. It distinguishes critical data/design issues from a genuine values fork. These are useful semantic instructions, but the host must still enforce the small structural claim that an exact option selection names a member of the offered set. [Proposer][proposer] [Interrogator][interrogator]

Four sets must remain distinct:

1. **Proposed:** alternatives emitted by the proposer and structurally accepted by the conductor.
2. **Admitted for the current decision:** alternatives satisfying the policy's relevant eligibility predicate.
3. **Offered:** alternatives actually published as choices for this saved question.
4. **Answered:** the specific offered alternative selected, or a separate free-text constraint requiring interpretation.

The current implementation measures (2) when constructing (3), then uses (1) when recognizing and admitting (4). The missing relation is not semantic similarity or another model judgment. It is the exact membership relation between a saved question and its allowed answer branches.

## 2. Implemented behavior, without inflating it

| Boundary | Implemented at this pin | Limit relevant to this finding |
|---|---|---|
| Candidate input | Unique nonempty ID, nonblank brief and success test, candidate cap, exactly one assessment, closed severity/kind vocabulary. | Citation presence and values-option membership are not validation requirements. |
| Ordinary-ready eligibility | Candidate, brief, citation, saved success, and absence of critical items. | A successful candidate with no critical item but no citation is not ordinary-ready. |
| Values eligibility | Candidate, brief, saved success, at least one critical item and no critical non-values item. | It is a separate relation, not equivalent to all successful proposed candidates. |
| Values question | Queries eligible values candidates, recommends one and includes only other members of that set. Requires at least two displayed alternatives. | `Thread.Options` persists rendered strings, not a typed offered-ID record consumed by the fast path. |
| Custom reply | Chat accepts a selected option or arbitrary typed input and sends it to `resumeDeliberation`. | The route is reachable through the normal interactive handler, not only a private helper. |
| Exact recognition | `choice` accepts an ID or rendered option from any entry in `Round.Proposal.Candidates`. | It does not consult `Thread.Options` or values eligibility. |
| Replay admission | Reasserts the saved round, adds `deliberation_answer`, queries disposition and persists the result. | The disposition rule does not join the answer to the offered or values-eligible set. |
| Downstream scope | Original intent is preserved; design-only returns a brief; ordinary effects retain work admission. | Prevents treating this design-readiness defect as a general authorization bypass. |

Sources: [Validation and record types][types], [eligibility and disposition rules][policy], [question publication and recognition][outcome], [custom-input handler][handler], [fast-path replay][service], [chat continuation][chat].

The ordinary-ready citation check is itself only an existence check on a model-provided string. This audit does not claim it verifies the truth, relevance, coverage or freshness of that citation. Nevertheless, the code distinguishes a candidate with no citation from one that satisfies its ordinary-ready rule. The values-answer route can erase even that existing distinction.

## 3. Exact source chain

### 3.1 Valid proposal does not mean values option

`validateRound` checks the shape and assessment coverage of every proposal. A candidate with ID `c3`, a nonempty brief and success test, an empty evidence list and `success=true` with no items passes those checks. There is no requirement that every proposed candidate belong to any eligibility class. That is not inherently wrong: a proposal set can legitimately contain alternatives that the decision stage does not admit. [Validation][types]

The policy makes the distinction observable:

- Ordinary-ready requires a citation and no critical item.
- Values-fork eligibility requires at least one critical item, with none of those items classified as data or design.
- A round can become a values fork when two distinct values-eligible candidates exist, the recommended candidate is one of them, and there is no ready or empirical alternative. [Eligibility][policy]

Thus a no-citation/no-blocker `c3` can coexist with a legitimate `c1`/`c2` values fork without being part of it.

### 3.2 Publication correctly filters the options

In `finish`, `ask_once` queries `deliberation_eligible` and builds a map of candidates for the current thread, current round and `/values_fork` kind. Only the recommended candidate and other members of that map appear in `out.Options`. The thread stores the question and these options and becomes `awaiting_values`. [Question publication][outcome]

This is already the correct source of the question's selection domain. The proposal need not contain only displayed alternatives.

### 3.3 The ordinary chat route accepts custom input

`handleClarificationResponse` uses the selected option when the textarea is empty and uses the typed text otherwise. It then sends that response, the saved deliberation ID and the request identity to `resumeDeliberation`. The resume method loads the thread, requires `awaiting_values`, sets `Thread.Answer` and calls the shared service. [Typed-input route][handler] [Resume][chat]

Consequently, entering `c3` is a normal custom-answer input. The narrow finding does not depend on editing persisted JSON, impersonating the owner, forging a proof or accessing an internal test hook.

### 3.4 Exact recognition expands the domain

`choice(r, answer)` strips the recommendation suffix/outer whitespace and compares the answer with every proposed candidate's ID and `ID + ": " + Brief`. It receives only the round, not the saved options. It returns `c3` on the proposed witness. [Parser][outcome]

When the thread is `awaiting_values`, the service uses any such match as a concrete option answer. It reasserts the complete saved round and asserts `deliberation_answer(T, R, "c3")`, then immediately calls `finish`. There is no new proposer, challenger or probe call on that branch. [Replay][service]

### 3.5 The policy does not restore the publication boundary

The answer disposition at lines 57–58 requires a values-fork decision for the round, an answer naming a proposed candidate, saved success for that candidate, and no critical non-values item. It does not require values eligibility, an offered-option fact, or ordinary-ready eligibility for the answered candidate. [Answer disposition][policy]

For `c3`, absence of all critical items satisfies the negative `nonvalue` condition. It does not establish the positive critical-values condition that was necessary to offer it. This distinction between “has only values blockers” and “has no non-values blockers” is decisive.

With the thread already questioned and an answer present, neither `ask_once` nor `stop_values` supplies a competing disposition. In a non-yolo mode, `proceed_assuming` does not apply. The source predicates therefore support one `proceed` disposition for `c3`. Subject to the existing kernel/proof/persistence operations succeeding, `finish` sets `Status="ready"` and changes the chosen candidate to `c3`. [Disposition and status][policy] [Host finish][outcome]

This is a source-level derivation of a conditional path, not a runtime receipt.

## 4. Minimal unrun witness and controls

Use the normal configured candidate cap of four, a non-yolo mode, a valid session, no concurrent source changes, normal proof support and successful persistence. Supply this **proposed fixture**, through the existing scripted-role seams:

| Candidate | Nonempty brief and success test | Evidence | Assessment | Initial eligibility |
|---|---|---|---|---|
| `c1` | Existing API variant | A legitimate source citation | Success; critical values item `api-preference` | Values-fork |
| `c2` | New entry-point variant | A legitimate source citation | Success; critical values item `api-preference` | Values-fork |
| `c3` | Another proposed design | Empty | Success; no items | Neither ordinary-ready nor values-fork |

Recommend `c1` and provide one concrete preference question. No empirical candidate exists. The first result should display only `c1` and `c2`. Persist/reload without changing source or the saved record. Type `c3` as the answer.

**Current source implication:** `choice` selects `c3`; replay asserts its answer; the answer disposition admits it; `finish` reports it ready without a new assessment.  
**Required behavior under the proposed contract:** `c3` is not admitted as an exact answer to this question. Preserve the input as a new preference/constraint for bounded refinement, or return a specific unmatched-option disposition, without falsely reporting the excluded candidate as ready.

This witness uses one omission in structurally accepted model output, not fabricated success for a known failing implementation. The challenger may regard `c3`'s design as satisfactory; the conductor still has not admitted it through either the displayed values branch or ordinary-ready branch.

Negative controls matter:

- Answering the actual `c1` or `c2` option must continue to work.
- Giving `c3` a critical design blocker, or mixed critical data/values blockers, prevents the current answer rule from admitting it. A data-only `c3` instead creates an empirical candidate and changes the initial decision before this pause; keep that as a separate routing control. This is not a claim that arbitrary blocked candidates can proceed.
- Giving `c3` `success=false` likewise prevents the current proceed rule.
- Giving `c3` a citation and no critical item makes it ordinary-ready at the initial decision, so the round no longer reaches this values-pause witness. Do not use that case as a failed reproduction.
- Making `c3` values-eligible means it should be offered; that is a different control.
- An unrelated free-text answer that matches no proposed ID already falls into refinement. The defect is the accidental promotion of an unoffered candidate ID into the exact-option branch.
- Yolo normally does not pause for this question. Its recommendation route is a regression control, not the primary witness.
- Use design-only and propose-mode cases to demonstrate the readiness bug while verifying that no unauthorized effect follows.

The omission is plausible but its frequency is unknown. The current tests' fixtures already permit candidates without evidence, but they do not establish that a live proposer emits this particular three-candidate pattern.

## 5. Existing coverage and non-duplication

The unit test named `TestChoice_MatchesIDOrOfferedOption` constructs a round and checks ID/rendered-label matching. It does not provide a saved offered set, and cannot distinguish offered from merely proposed candidates. The round-validation tests omit a missing-evidence case, consistently with the current structural contract. These tests were read, not run. [Unit fixtures][unit-test]

The normal chat fixture has two candidates; its second candidate lacks a citation. In values mode both receive values blockers, so both are displayed. `TestDeliberationValuesForkAsksOnceAndResumesOriginal` resumes the first displayed choice, checks zero new model calls and checks ordinary propose-mode work admission. Keep those controls; add an excluded third alternative rather than weakening the existing assertions. [Chat fixture][chat-fixture] [Existing restart and permission control][chat-test]

This document is distinct from:

- **Deliberation values evidence freshness:** correct offered choice, stale supporting evidence after a pause. Here the workspace and evidence may remain unchanged; the answered candidate was outside the offered domain from the start. The existing freshness audit explicitly uses a genuine originally offered option. [Freshness audit][freshness]
- **Deliberation read invocation identity:** loss of tool arguments in observation replay. Here no new read is needed to expose the mismatch, and the citations of `c1`/`c2` can be accurate. [Read identity audit][read-audit]
- **Consultation qualifier continuity:** advice qualifiers lost at an adapter boundary. The witness uses no consultations. [Consultation audit][consult-audit]
- **Terminal budgets:** evidence closure, bounded answer production and continuation. This witness uses one completed round and the pre-existing zero-role-call option branch. [Budget audit][budget-audit]
- **Recurse values continuation ownership:** mission-owned questions entering another continuation surface. The witness uses a correctly owned conversational thread. [Ownership audit][owner-audit]
- **Portable recurse deliberation crucible:** broader requirement/evidence/adapter and oracle contracts. This supplies one concrete missing selection transition in its existing Socratic funnel; it does not restate that overall proposal. [Crucible audit][crucible]
- **Request-bound human approval:** authority to perform a reviewed effect. This audit concerns design-option membership, not approval or a change to permission policy. [Approval audit][approval-audit]

The checked public audit inventory has no dedicated values-option admission audit. That is a bounded deduplication observation, not a claim that every historical branch or private discussion was searched.

## 6. Proposed contract

Everything in this section is **proposed**, not implemented or validated by this audit.

### 6.1 Bind answers to the published question

Create an explicit host-owned offered-choice record for the saved question: thread/session/owner identity, round or question revision, candidate IDs, candidate-content binding, display order, recommended candidate and the policy/evidence admission identity relevant to publication. It should be produced from the same policy result used to render the question.

A compact representation is sufficient. Do not duplicate the entire history in each option or add a second conversation engine. Candidate IDs alone must be interpreted inside their saved question/round, not globally across later proposals.

The semantic rule is:

> Exact selection is possible only for a candidate that belonged to this question's offered set and still satisfies the selected-choice admission contract.

This can be checked cheaply. Typed ID/record membership is preferable to parsing a display string. Keep label matching as a compatibility convenience only after resolving it within the offered record, with ambiguous matches handled explicitly.

### 6.2 Make Mangle enforce the same relation

The host records the factual question/option identity; Mangle derives whether the selection can proceed. The answer rule should require both the question-bound offered relation and the appropriate values eligibility/admission premise, not merely absence of a non-values blocker.

A parser-only check helps the chat path but leaves the shared service's fact/protocol contract weak. A policy-only values-eligibility check fixes the principal witness but does not by itself prove that a candidate was actually published under this question. The durable record, recognition and disposition should use one domain.

Do not make an LLM responsible for checking exact membership. It can interpret a genuinely new preference or propose a new design, but cannot assert the host's offered-choice relation.

### 6.3 Separate custom input from selection

Custom text remains useful. If the person names an unoffered candidate or asks for another design, retain the exact answer as an added constraint under the original request and restrictions. It must not become an admitted exact selection merely because it happens to equal a proposal ID.

Decide explicitly between bounded refinement and a named unmatched-option result. Preserve the one-question/no-progress policy, original ownership and remaining budget. A deterministic mismatch does not require a paid model call just to identify it. If interpretation or a new design is necessary, charge that refinement normally and do not silently reset the budget.

No new question, wider scope, testing authority or execution authority follows from an unmatched selection. Also preserve explicit cancellation/withdrawal handling in the surrounding product; this audit does not specify that separate protocol.

### 6.4 Define evidence preconditions consistently

The immediate defect can be closed with offered-set/eligibility continuity. Separately, the specification should say why ordinary-ready requires a citation while values-fork eligibility does not.

Do not “fix” the witness by adding arbitrary citation-shaped strings or requiring irrelevant repository reads for genuinely preference-only questions. If a class of dependency-free choices legitimately needs no external evidence, give it a named admitted state and criteria. Missing citations alone are not a proof that no evidence dependency exists. This is a design clarification adjacent to the narrow defect, not a claim that all citation-free values choices are invalid.

An offered-choice record also does not solve the existing evidence-freshness audit. Offer identity, factual validity, preference resolution, execution authority and implementation acceptance remain separate judgments. Compose with existing freshness and owner-admission work rather than treating an option digest as a truth certificate.

### 6.5 Recover old records without inventing an offer

Existing threads have saved display strings and full rounds. A migration can recognize an old offered set only where the stored options map uniquely to the saved candidates and match the saved question/round contract. Do not reconstruct “offered” by including every proposal, or by applying a new policy and claiming its result was historically shown.

Where reconstruction is ambiguous or absent, retain the answer/request and use an explicit unresolved or refinement path. Do not discard the user's preference or make unknown legacy data silently current. Rejecting malformed records must not become a misleading success, and a failed write must not report a new persisted offer.

## 7. Costs and failure modes

### Full costs, not just role count

Let C be the number of candidates, I the number of assessment items, S the serialized saved thread size and H the bytes of candidate content included in an offer binding.

- **Question construction:** membership extraction is O(C + I) in host data handling, plus the existing kernel query cost; a content binding costs O(H) hashing/serialization. Added durable metadata is O(C), plus bounded identities/digests. These are analytic estimates, not timings.
- **Exact answer:** matching an offered ID can be O(C), or expected O(1) with a small map. Rechecking the relevant policy is additional kernel work. Reuse the current no-new-role-call fast path when all admission conditions hold.
- **Persistence:** the current save marshals and writes the complete thread, syncs a temporary file and renames it. A small new record does not make that O(1); write/serialization cost remains proportional to S. Do not claim free durability. [Persistence][persist]
- **Refinement:** a genuinely new preference may require the existing proposer/challenger pair, each with JIT compilation, context construction and bounded evidence loops, plus optional consultations and empirical probes. Count retries, failed reads, final-answer requests, proof generation, saving and downstream processing. Two top-level role invocations are not two total provider requests. [Collection][round] [Role loop][read-tools]
- **Defaults:** four rounds, two repeated fingerprints, six evidence tool calls per role, four candidates and 64 KiB role-context configuration are present in routing defaults. Consultation/probe budgets and context constraints also apply. They are not a total provider-price or wall-clock guarantee, and this audit does not recommend raising them. [Configuration][config]
- **Opportunity cost:** an invalid option previously accepted in zero new model calls may now stop or require refinement. That added cost purchases correct admission, but should be measured rather than hidden as a regression exception. Conversely, deterministic mismatch handling can avoid a needless model call.

Account for full input/output tokens, every consulted provider, cache hits, unsuccessful/abandoned continuations, tool I/O, proof and persistence time, operator time and resulting design quality. No cost, latency or quality improvement is claimed before evaluation.

### Principal failure modes

| Risk introduced or left unresolved | Required treatment |
|---|---|
| Offered set rebuilt from all proposals | Preserve the original filtered domain; fixture must catch this exact regression. |
| Correct candidate ID from another round/question | Bind to question/round and content, not ID alone. |
| Display-label change, localization or recommendation suffix | Keep semantic selection separate from presentation; make legacy mapping explicit. |
| Empty/ambiguous legacy offered set | Unknown/refinement outcome, never a fabricated offer. |
| Current eligibility differs from historical offer | Retain the preference but withhold unsupported readiness; compose with freshness policy. |
| UI-only fix leaves direct service reuse permissive | Enforce the shared contract and policy boundary as well. |
| Kernel failure or unavailable proof | Visible non-ready outcome; no fallback to the broader proposal set. |
| Persistence fails after staging the new record | Report the failure and avoid claiming durable admission. |
| Unoffered ID always triggers costly model work | Prefer a deterministic mismatch outcome when no interpretation is needed. |
| New guard rejects legitimate offered alternate | Preserve controls for nonrecommended options and exact restart replay. |
| New guard resets `Asked`, owner or budget | Treat this as a regression; do not restart an interview or transfer authority. |
| Stable option digest mistaken for evidence validity | Keep evidence currentness separate. |
| Additional metadata inflates prompts | Keep host selection records out of model frames unless needed; bound retained metadata. |

## 8. Primary research basis and limits

Gotel and Finkelstein's requirements-traceability study separates tracing the origins and development of a requirement from tracing its later realization. The relevant engineering lesson is to retain the connection between the person's preference question, the viable alternatives shown and the selected design, instead of preserving only the final ID. This is an application of their traceability framing, not empirical evidence that this particular patch improves codeNERD. [An analysis of the requirements traceability problem, 1994](https://discovery.ucl.ac.uk/id/eprint/749/)

Deniélou and Yoshida formalize protocol behavior using communicating automata and session types. Their work motivates treating the offer/reply exchange as a protocol whose legal transitions agree at both ends. This audit does not claim a session-type proof for codeNERD, recommend introducing a new type system, or borrow their liveness guarantees for this implementation. A small explicit offered-choice state relation is the proposed local application. [Multiparty Compatibility in Communicating Automata, 2013](https://arxiv.org/abs/1304.1902)

W3C PROV-DM models entities, activities, derivation and collections/membership. These provide a useful vocabulary for distinguishing a candidate, a published offer containing that candidate, and a later selection derived from the offer. The standard does not supply codeNERD's eligibility policy or establish that a candidate is correct. [PROV-DM, 2013](https://www.w3.org/TR/prov-dm/)

These sources support the design concepts only. The repository source chain supports the finding. No literature result establishes the frequency, exploitability, performance or correctness of codeNERD's current path.

## 9. Fair evaluation plan: entirely unrun

### Deterministic admission tests first

Use the existing scripted proposer/challenger seams with the production kernel. Exercise the normal chat custom-answer handler, persistence/reload, service and proof path. Do not require live inference to test a missing set-membership comparison.

| Fixture | Required observation |
|---|---|
| Three-candidate primary witness | `c3` is absent from published options and cannot become ready by exact-answer replay. |
| Exact recommended displayed option | Successful unchanged replay; no unnecessary role invocation. |
| Exact nonrecommended displayed option | Same preservation of the person's preference. |
| Offered ID versus complete label | Equivalent selection within the same saved domain. |
| Unoffered ID versus unoffered rendered label | Neither silently becomes an exact admitted answer. |
| No matching proposed ID | Custom-answer behavior remains bounded and preserves the original request. |
| Unoffered candidate with critical design or mixed data/values blockers | Non-ready; no regression to permissive fallback. |
| Unoffered candidate with `success=false` | Non-ready. |
| Third ordinary-ready candidate | Initial ready path, not an invalid values-pause fixture. |
| Three genuinely values-eligible candidates | All intended offered alternatives can be selected. |
| Restart with identical valid offer | Same selection domain and no lost preference. |
| Legacy strings with unique/ambiguous mapping | Explicit migration result; no invented historical offer. |
| Changed labels/order with stable semantic identity | No selection based on accidental list position. |
| Reused candidate ID in a later round | Prior question cannot select the newer meaning silently. |
| Missing proof, kernel error or failed save | No false ready or durable-success report. |
| Design-only/propose/yolo modes | Design-only does not launch work; propose still holds effects; yolo recommendation behavior stays explicit. |
| Mission-owned thread | Existing owner isolation is preserved; no foreground question theft. |
| Exhausted budget during necessary refinement | Honest stop with retained answer, not budget reset or fabricated success. |

Observe: proposed IDs, derived eligible IDs, persisted offered IDs/labels, selected ID, question/round identity, resulting facts/proof, status/note, role/probe counts, ordinary work-admission outcome and worker launch count. The acceptance invariant is that every successful exact selection can be traced to the same offered-domain member and current admission; do not score only the final surface text.

### Bounded comparative evaluation afterward

After deterministic correctness is established, compare:

- **A:** current saved-round selection behavior.
- **B:** offered-set plus policy-membership continuity, retaining the zero-role-call valid-answer path.
- **C, optional:** B plus a separately specified legacy migration/refinement strategy.

Use the same pinned request corpus, model/provider, source snapshots, autonomy settings, permitted effects and total budget. Include genuine new preferences, malformed/partial proposals, unoffered IDs, offered alternates, restarts and cost-negative cases. Do not evaluate B only on inputs designed to fail A. Keep factual-evidence freshness changes as a separate factorial condition so the old freshness defect does not masquerade as this fix's gain.

For portability, run the protocol cases across Go, Rust, Python, JS/TS, mixed and minimal non-code workspaces, with Unicode labels and Windows/POSIX-style targets. The selection relation is language-independent; those cases assess integration and presentation without assuming any particular compiler, shell or test adapter. This does not establish universal recurse portability.

Report false-ready selections, wrongful rejection of valid options, preserved preferences, unresolved/cancelled fraction, added user burden, total tokens/spend, paid calls, local I/O and end-to-end latency. Report both all-attempt and successful-attempt denominators. Blind the reviewer of selected-design adequacy to the implementation arm where feasible. Cluster uncertainty by task/repository family and report the small deterministic suite as a suite, not statistical evidence of live-model prevalence. A null or cost-negative result is legitimate.

## 10. Acceptance and adoption

Start with the owning deliberation/session specification. Add a named requirement that a values answer resolves only a member of the offered, admitted question domain. Define custom-answer and legacy dispositions, then align durable records, host recognition and Mangle disposition with that requirement.

The smallest useful exit condition is:

> With an unchanged, correctly owned saved round containing two displayed values alternatives and a third unoffered successful proposal, entering the third proposal's ID or label cannot produce current-ready by the exact-option shortcut. Both genuinely offered alternatives continue to resume through the existing no-new-role-call path and ordinary authority checks.

Closing this gap establishes selection-domain continuity. It does not establish truthful citations, complete requirement coverage, fresh evidence, correct implementations, successful tests or portable build/test adapters. Those remain their own contracts.

## Pinned references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/1540beaad60f3a01ab5ffdc162800225c0b74c90
[comparison]: https://github.com/theRebelliousNerd/codenerd/compare/f4d0ca1d7fba2a208febda4f11f879af23b15e06...1540beaad60f3a01ab5ffdc162800225c0b74c90
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/agents.md#L1-L24
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L898
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/prompt/atoms/system/deliberation_proposer.yaml#L14-L29
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L28
[types]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/types.go#L71-L181
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/core/defaults/policy/deliberation.mg#L14-L74
[outcome]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/outcome.go#L16-L114
[service]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/service.go#L74-L95
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/cmd/nerd/chat/model_handlers.go#L441-L499
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/cmd/nerd/chat/deliberation.go#L58-L119
[persist]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/persistence.go#L23-L74
[round]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/round.go#L10-L79
[read-tools]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/read_tools.go#L90-L217
[config]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/config/routing.go#L18-L31
[unit-test]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/internal/deliberation/deliberation_test.go#L33-L97
[chat-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/cmd/nerd/chat/deliberation_test.go#L54-L88
[chat-test]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/cmd/nerd/chat/deliberation_test.go#L179-L219
[freshness]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/deliberation-values-evidence-freshness.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/deliberation-read-invocation-identity.md
[consult-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/deliberation-consultation-qualifier-continuity.md
[budget-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/deliberation-terminal-budget-continuation.md
[owner-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/recurse-values-continuation-ownership.md
[crucible]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/portable-recurse-deliberation-crucible.md
[approval-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1540beaad60f3a01ab5ffdc162800225c0b74c90/Docs/audits/request-bound-human-approval.md

