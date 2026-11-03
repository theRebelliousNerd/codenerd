---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 0a7c74cf19c3d01036b8d6462273164e8c670808
supersedes: []
---

# Require an explicit disposition for earlier critical design objections

## Finding and decision

The deliberation conductor validates each round independently and derives readiness from that round's success assertion, citation and absence of critical items. Its typed contract does not require a later assessment to explain what happened to an earlier critical item. A structurally valid later assessment can therefore omit or downgrade that item and become ready without an explicit resolution, supersession or retraction record. The earlier assessment remains in durable history, but its earlier critical classification does not participate in the new round's readiness decision. [Validation][validate] [Projection][project] [Readiness policy][policy]

**Classification: a source-grounded assurance-contract gap, with a conditional failure witness.** The current code follows the documented current-round assessment model. This report proposes strengthening that model for Socratic requirements work. It does not establish that an earlier objection remained substantively correct in any actual run, that the production models have emitted the witness below, or that a downstream implementation passed its verification incorrectly. [Current session contract][spec]

The practical requirement is small: once a critical objection has been admitted for a candidate, a later round should account for it explicitly before treating that candidate as ready. The model may explain why the objection is resolved, mistaken, superseded by a different design, or irrelevant to a rejected alternative. The host can validate the identity and completeness of that accounting; Mangle can derive its effect on readiness. Neither should pretend that a structured explanation proves a natural-language claim.

This is a focused continuation of the existing portable-crucible proposal, which already recommends linking revisions to objections and recording resolution evidence. The new contribution is the exact current-round readiness boundary, a minimal unrun witness, the distinction between missing accounting and false semantics, and an evaluation that penalizes both unjustified readiness and unnecessary obstruction. It is not a proposal for another agent framework. [Existing crucible proposal][portable-objections]

### Source and execution boundary

The public main branch resolved to **0a7c74cf19c3d01036b8d6462273164e8c670808**, committed November 3, 2026 at 10:32:47 UTC. It was checked again unchanged at 10:46 UTC before freezing this report. All repository links below use that immutable pin. [Source commit][pin]

Root and scoped guidance, architecture, implementation, related audits and authored tests were read. No repository program, build, test, model inference, benchmark, gate or failure injection was run. No implementation, credential, permission, configuration or setting was changed. All witnesses, APIs, lifecycle states and evaluations proposed below are unimplemented and unrun.

## 1 The design requirement

The requested direction is a Socratic ideation and requirements crucible for recurse. The existing implementation already has the essential participants: a read-only proposer develops alternatives; the requirements interrogator challenges them; the kernel owns eligibility, ranking, stopping and disposition. The right next step is to strengthen the connection between criticism and refinement, not to add more votes for the same idea. [Root architecture][guidance] [Proposer][proposer] [Interrogator][challenger]

A useful exchange can change its mind. A critic may identify a real defect, misunderstand a constraint, learn that an alternative removes the problem, or find that a previously minor issue is critical. The result needs a recoverable account of those changes. Otherwise an empty new item list is ambiguous between:

- the earlier objection was resolved;
- the objection was shown to be mistaken;
- the candidate was redesigned so the objection no longer applies;
- the critic forgot or accidentally downgraded it;
- the critic chose not to mention it under a context or response budget.

These cases have different implications for readiness even though the current item list can be identical.

The root guidance assigns completion obligations to the executive and requires bounded, recoverable context. The architecture guidance requires explicit separation between target behavior and current implementation. This report accordingly proposes a new lifecycle contract rather than rewriting the current-state description to claim it already exists. [North-star obligations][guidance] [Architecture documentation contract][architecture]

### Preserve the current safeguards

The proposed refinement must retain:

1. A divergent first round and convergent subsequent rounds.
2. A complete assessment for each current candidate.
3. Distinct data, design and values categories.
4. Read-only evidence gathering through the existing guarded registry.
5. Policy-owned readiness, ranking, stop limits and permission boundaries.
6. Honest stopped or failed outcomes.
7. The original request, intent and restrictions.
8. The ability to correct an erroneous objection without requiring a new tool call merely for appearances.

A ready design still means a selected interpretation or design is ready for its next authorized stage. It does not mean code has changed, a test has run, a mission has been satisfied, or an effect has been authorized. The current interrogator prompt and mission receipt comments already make these distinctions. [Interrogator semantics][challenger] [Mission receipt][mission-receipt]

## 2 Current implementation and the exact missing transition

### 2.1 The previous objection is available to the participants

The Frame contains the original request, intent, optional brief, answer and previous Round. For later rounds, collectRound points Previous at the immediately preceding completed round. Both proposer and challenger receive that frame; the challenger also receives the current candidates and consultation evidence. [Frame][types-frame] [Collector][round]

The proposer atom instructs the model to refine using the previous challenge and measured probes while keeping identities stable. The interrogator atom requires unresolved items to remain explicit and tells it not to fabricate success to fit a budget. Therefore the simple omission witness below violates the intended model behavior. The host currently treats it as structurally valid, which is the narrower concern. [Proposer][proposer] [Interrogator][challenger]

This is not universal forgetting. The thread keeps all Rounds; Save serializes the complete thread to an atomic snapshot. During one Run invocation the asserted facts are retained until exact cleanup on exit. Nothing in the finding requires deleting earlier records. [Thread][types-thread] [Persistence][save] [Fact lifecycle][fact-lifecycle]

### 2.2 The typed assessment cannot express a retirement decision

An Assessment contains CandidateID, Success and Items. Each Item has a key, severity, kind, detail and optional probe. Neither has a field for disposition of an earlier item, the earlier item version, supporting observation IDs, a successor objection or an explicit retraction reason. [Types][types-items]

The validator receives only the new Round and a candidate cap. It checks candidate uniqueness, nonempty briefs and success tests, exactly one assessment per candidate, unique item keys within each assessment, closed vocabularies and probes for critical data items. Its local key map is new for each assessment. It has no previous ledger against which to require coverage. [Validation][validate]

Consequently, the error text about stable unique keys guarantees local uniqueness, not cross-round stability or continued accounting. This report does not claim a validator could mechanically prove every natural-language objection still applies. A proposed history-aware validator could detect missing explicit accounting for an earlier item.

### 2.3 Policy projects and selects only the current round

assertRound emits current candidate, citation, success and item facts. The item projection carries thread, round, candidate, key, severity and kind. It does not emit a transition connecting the old item to a resolved or superseded state. [Projection][project] [Schemas][schemas]

The readiness rule requires current candidate and brief facts, at least one current citation fact, current success and no critical item with the same thread, round and candidate. An earlier critical item with round 1 therefore cannot block a candidate assessed in round 2. The rules have no separate active-objection relation joining those rounds. [Readiness policy][policy]

Service.Run chooses exactly one decision for the current thread and round, records a proof, saves the round, and calls finish immediately for ready or values-fork outcomes. finish marks a proceed outcome ready. The later progress-fingerprint update is not a historical-objection validator and is not reached on this ready return. [Round dispatch][service-dispatch] [Finish][finish]

A proof envelope demonstrates the recorded policy derivation from its admitted premises. It cannot establish that an omitted prior objection was resolved, because that reconciliation is not among the premises. The current proof function appropriately retains generation, digest and truncation information; those safeguards should remain. [Proof recording][prove]

### 2.4 The route reaches both design chat and recurse mission preparation

The chat constructor supplies the shared Service, registered shard dispatch and read executor. The /deliberate and /innovate path creates a design-only thread and returns the result. It does not launch a development worker from that command. [Chat wiring][chat] [Design command][command]

The recurse mission constructor supplies the same Service. Mission preparation passes the mission, attempt, history and north-star context into a design-only thread, then maps a proceed outcome to its ready design disposition. The receipt stores the selected candidate and the thread reference. Later source, vision, write-admission and verification safeguards remain separate. This report does not claim those checks are bypassed or that every insufficient design would reach a successful mission outcome. [Mission constructor][mission-constructor] [Mission preparation][mission-prepare]

The missing accounting is language-neutral and precedes language-specific build or test adapters. Its existence can be inspected without running Go, Python, JavaScript, TypeScript or Rust verification. Demonstrating a corrected transition across those contexts would still not establish the completeness of their separate verification adapters.

## 3 Minimal source-derived witness

### Preconditions

Use a stable, small workspace and one fresh design-only thread. Keep the original request, candidate IDs, candidate content and source evidence unchanged. Use successful persistence and proof recording, sufficient context capacity and normal default round limits. No values question, consultation, source drift, concurrency, special language feature or read-tool ambiguity is needed.

A proposed fixture request is: design an in-memory event processor that rejects duplicate event IDs for the lifetime of the process, including simultaneous arrivals. The workspace evidence identifies the current event-handler interface. Two candidates have nonempty briefs, targets, success tests and real fixture citations:

- c1 uses a check-then-insert map without a serialized reservation step.
- c2 proposes a queue but has not specified who owns duplicate admission.

For this deliberately constructed fixture, the evaluator defines both as incomplete designs. This semantic ground truth belongs to the fixture oracle, not to a claim that the host can infer it from prose.

### Round 1

The challenger supplies a critical design item for each candidate:

| Candidate | Key | Severity | Kind | Stated issue |
|---|---|---|---|---|
| c1 | concurrent-admission | critical | design | Two arrivals can both pass the check before either records the ID. |
| c2 | admission-owner | critical | design | The queue proposal does not identify the operation that reserves an ID. |

Both success flags are false, and the recommendation names c1. There is no ready candidate, no data-only candidate and no values fork. The policy therefore has the needs-work route. The service persists the round and begins a convergent round within the existing limit. [Policy][policy] [Service][service-dispatch]

### Round 2

The proposer returns the same candidates and evidence. The challenger again recommends c1 and keeps c2 blocked but returns for c1:

- CandidateID c1;
- Success true;
- an empty Items list.

A companion case keeps concurrent-admission but changes its severity to important with no stated reason. The current type and validator admit both forms. In either case c1 now has success, a citation and no current critical item. With c2 still blocked, the current-round rule permits c1's readiness and proceed disposition. Earlier c1 facts remain associated with round 1. [Validation][validate] [Projection][project] [Readiness][policy] [Finish][finish]

**This is an unexecuted source prediction, conditional on the erroneous second assessment and the stated infrastructure preconditions.** It isolates the host's inability to distinguish unaccounted omission from justified retirement. It is not evidence that a live model will make the error, a runtime proof of the entire pipeline, or a claim that any change in an assessment must be forbidden.

### A failed-probe variation

A second proposed fixture begins with a critical data item and an allowed probe that returns an operational error. runProbes preserves the error in the saved round and emits a digest receipt; it does not itself fail the whole thread merely because that individual probe failed. The next assessment can still omit the item and assert success without a disposition tying it to alternative evidence. [Probe handling][probes]

A failed observation does not prove the requirement is impossible, and a later model may legitimately find another basis to settle it. The proposed contract should require explicit accounting for the failed route and the new basis, not automatically demand repetition of the identical probe. This variation is also unrun and should not be used to imply that all probe failures cause false readiness.

### Evidence already present in tests

The authored empirical chat fixture removes its earlier data item after inspecting a previous probe output containing its expected package text. That test exercises a legitimate transition and should preserve its successful outcome after adapting the scripted response to the proposed disposition schema. Its conditional reasoning lives inside the scripted model response; the production conductor still receives only the resulting new assessment. [Scripted fixture][fixture] [Authored empirical test][empirical-test]

The validation tests cover current-round malformed structures. The stop tests keep critical design items present and verify configured stop outcomes. In the files inspected, they do not exercise unchanged candidate plus earlier critical item plus unaccounted omission/downgrade. This is a statement about those read files, not a repository-wide proof of missing tests, and none was run here. [Validation tests][validation-tests] [Stop tests][stop-tests]

## 4 Relationship to existing audits

The current audit queue and relevant nearby reports were inspected before selecting this boundary. The proposed mechanism has prior art inside the repository.

| Existing report | Existing contribution | Additional boundary here |
|---|---|---|
| Portable recurse deliberation crucible | Connect revisions to objection keys and identify what evidence resolved an objection. | Trace the absence of an explicit retirement transition into current-round readiness, with a two-round witness. |
| Deliberation progress observation identity | Distinguish newly inspected scopes in the stop fingerprint; propose introduced, refined, discharged and reopened obligation changes. | Account for earlier critical items before readiness, rather than decide whether another round should run. |
| Deliberation selected review handoff | Preserve legitimately nonblocking important concerns in the terminal result. | Reconcile previously blocking concerns before the result becomes ready. |
| Deliberation values evidence freshness | Revalidate saved evidence when resuming a preference answer. | No pause, answer or source change is needed. |
| Deliberation values option admission | Bind an exact answer to the offered admissible domain. | No user option recognition is involved. |
| Planned work conservation | Preserve an edit plan's omitted remainder under a cap. | No plan cap or omitted candidate list is needed; the missing edge is a later assessment's retirement of an earlier objection. |
| Continuation obligation generation | Prevent earlier execution results discharging newly generated test debt by description. | No tester result, effect or execution-generation alias is needed; earlier design debt has no explicit lifecycle transition. |

Sources: [Crucible][portable-objections], [progress contract][progress-contract], [selected review][review-audit], [freshness][freshness-audit], [option admission][option-audit], [planned work][work-audit], [continuation][continuation-audit], [audit queue][queue].

The report should be tracked as a narrow refinement of those designs. Fixing only the progress fingerprint or final warning renderer would not establish its proposed readiness invariant. Conversely, this contract would not repair their omitted invocation identities or handoff fields.

## 5 Proposed contract and ownership

Everything in this section is prospective. Names describe responsibilities, not existing APIs.

### 5.1 Preserve a candidate-bound objection identity

Use a thread-owned identity for each admitted critical objection, associated with:

- the candidate identity and design revision to which it applies;
- the originating round, kind, severity, detail and source assessment;
- the request or requirement it concerns when known;
- any observation or proposed discriminant attached to it;
- its current disposition and the transition that established that disposition.

Do not make a user-visible item key the sole identity across unrelated candidates or threads. Keep both a stable display key and an unambiguous host identity. Candidate renaming, prose rewording or an absent item in the new array must not create a fresh unburdened candidate automatically.

A host-computed content digest can detect that a candidate's serialized content changed. It cannot determine whether that change resolves a design objection. The model supplies a resolution claim and rationale; the host records the exact old and new content, and policy controls whether the supplied transition is admissible.

### 5.2 Require coverage of the prior active set

For each current candidate's inherited active critical items, the new assessment must explicitly choose a disposition. A possible closed vocabulary is:

| Proposed disposition | Required meaning |
|---|---|
| remains open | The concern still applies. It continues to block this candidate. |
| resolved by refinement | The claim identifies the old objection and the particular design revision addressing it. |
| resolved by evidence | The claim references a relevant observed result and describes what it settles. |
| retracted as mistaken | The challenger explains the prior error and the basis for correction. |
| superseded | A named successor objection or revised requirement carries the remaining concern. |
| candidate retired | That candidate is no longer being considered; its debt is historical, not silently attached to unrelated alternatives. |

A severity downgrade from critical to important also needs a transition explaining why the original requirement does not remain blocking. It must not be an alias for leaving the original mandatory behavior unsatisfied. Any important item remaining after an admitted downgrade still belongs in the review handoff described by the existing audit.

The host can check exhaustive identity coverage, referential integrity, missing reasons, stale candidate versions, unavailable receipts, conflicts and lifecycle consistency. These checks are useful without claiming that Go or Mangle understands the full prose. Semantic validity remains an explicit assessment claim unless supported by an independently executable oracle.

### 5.3 Separate observation from resolution

An observation can support, contradict or leave an objection unsettled. Merely having a receipt, a digest, a citation string or a successful tool return is not a resolution certificate.

For data-backed resolution, bind the claim to the exact executed subject, relevant result and available scope/completeness status. Respect the existing invocation-identity and freshness audits. A timeout, denial, truncated read or skipped probe is not affirmative support for the missing fact. It may support a different conclusion, such as that a proposed approach cannot be established within current constraints, but that conclusion must be stated.

For a design-backed resolution, permit a sufficient revision or correction argument without requiring fabricated new external evidence. The change in the proposal can itself be the relevant object of review. Retain the before/after relation and rationale. Do not equate another agreeing model with an independent observation.

### 5.4 Keep executive decisions in Mangle

Go should parse, preserve identities, measure versions, validate references and publish typed transition facts. The existing JIT role should propose transitions. Mangle should derive whether a previously critical item remains active, whether reconciliation is complete, and whether the current candidate qualifies for ready or values-fork disposition.

Do not add an ad hoc Go rule that silently deletes all old items whenever the candidate brief changes. Do not permit the model to assert that an item is cleared directly into executive authority. The proof should expose the transition premises and distinguish semantic claims from host observations.

The simplest deterministic guard is coverage: no earlier applicable critical identity can disappear without a disposition. Stronger checks can be introduced only where their evidence contracts exist. A reason string alone improves accountability but remains vulnerable to incorrect reasoning; quantify that limitation rather than describing the system as formally verifying requirements.

### 5.5 Bound context without bounding away obligations

Retain a complete durable obligation set and project a bounded active view. A projection needs counts, explicit coverage state and a resolvable reference for omitted detail. A new candidate assessment cannot become ready by reviewing only the visible prefix.

Use the existing thread snapshots where practical rather than creating a parallel conversation database. Do not replay every historical round into every model call. Inject active objections plus the minimum transition evidence; let permitted targeted reads recover older detail. Preserve original request constraints and the existing role-input byte checks.

If the active set cannot fit the contract's limits, return a typed incomplete or blocked result, or derive a bounded reconciliation continuation under the existing authorization and resource envelope. An arbitrary truncation followed by readiness would recreate the gap. A continuation does not grant a new round budget, more spend or permission to edit.

### 5.6 Lifecycle and compatibility

Candidate retirement and semantic revision are necessary, not escape hatches to prohibit wholesale. If c1 is rejected and a materially different c2 is selected, c1-only objections should not prevent c2 readiness. A request-wide requirement, such as retaining an API or supporting cancellation, may still apply to every candidate. Model-proposed applicability should be explicit and reviewable.

During resume, preserve thread owner, candidate revision and objection identity. A free-text answer may change requirements, but should not retire unrelated objections. Preserve values handling and apply the separately specified offered-domain admission requirement. Mission-owned threads remain with their mission owner.

Legacy snapshots do not contain retirement records. Do not retroactively invent evidence that an old item was resolved. Distinguish historical ready outcomes from a new request to resume work. If active legacy work requires stronger admission, reconstruct the relevant prior set and obtain explicit reconciliation within the allowed envelope; otherwise return an honest incompatible or incomplete state. Migration policy needs a specification decision before implementation.

## 6 Failure modes the refinement must handle

1. **The critic was wrong initially.** Requiring old objections to remain forever rewards overcautious or mistaken criticism. Provide a correction route and evaluate false obstruction.
2. **The design changed legitimately.** Old candidate-specific assumptions may no longer apply. Bind each claim to its actual revision and require an explanation rather than forcing identical keys onto unrelated designs.
3. **Renaming becomes an escape.** A new ID or rewritten brief must not silently clear unchanged request-wide debt. Record lineage and applicability explicitly.
4. **A generic explanation launders unsupported success.** A nonempty rationale is not a verified resolution. Preserve its model-claim status and check independent evidence where available.
5. **The ledger becomes an unbounded checklist.** Bound records and projections, record overflow honestly, and avoid letting a model manufacture infinite trivial blockers.
6. **Downgrades evade a hard requirement.** Record why blocking status changed and preserve any still-mandatory requirement.
7. **Old evidence is reused on new content.** Resolution references must identify their observation and design/source applicability. A matching item name alone is insufficient.
8. **A rejected option contaminates the winner.** Keep candidate-specific concerns separate from cross-candidate requirements.
9. **Stopped reconciliation is labeled failure of the user's idea.** Distinguish budget exhaustion, unavailable evidence, inconsistent output and a substantive rejected design.
10. **The stronger contract merely moves mistakes into a new field.** Measure unsupported retirement claims and independent design quality, not just JSON-schema compliance.

These are proposed design hazards, not additional reproduced defects.

## 7 Full cost accounting

The proposal does not establish a token or latency saving. It may prevent a misleading early exit while adding work.

### Current baseline costs

At the inspected source, a completed round invokes proposer and challenger role paths, with possible internal read/tool interactions and optional consultations. The defaults are four rounds, four candidates, six evidence-tool calls per role's configured allowance, and 65,536 bytes for the serialized role payload. System prompts and tool schemas sit outside that payload check. The Agent currently counts executed tools and may still request a final answer after tools close; the current-state architecture table's older combined-call wording should not be used as a precise model-call bound. Measure actual calls. [Defaults][defaults] [Collector][round] [Agent budget implementation][agent]

Each saved round includes proposals, assessments, probes and a proof envelope. Save rewrites the whole serialized thread. It is invoked at multiple lifecycle points, including after conductor probe results. Storage work therefore depends on the number and timing of saves and cumulative snapshot size, not merely the final file size. [Save][save] [Service saves][service-dispatch] [Probe saves][probes]

### Incremental proposed costs

Let R be completed rounds, K the admitted candidates in a round, I the total distinct retained objections, E the retained evidence/transition records, and S_j the serialized thread bytes at save j.

- **Host validation:** identity maps can check one new assessment and its inherited active set in work proportional to those records and references. Exact costs depend on lifecycle representation; semantic comparison must not be disguised as a constant-time hash check.
- **Kernel work:** additional active-item and transition facts, joins, invalidation and proof nodes. Measure closure, retraction and explanation overhead on the production corpus.
- **Persistence:** additional identity, reason and reference bytes increase S_j. Total serialization/write work is approximately proportional to the sum of S_j across saves, excluding filesystem overhead. If retained state grows roughly linearly and full snapshots are written a fixed number of times each round, the rewritten bytes can grow quadratically in R. Existing defaults bound R, but configurable limits still matter.
- **Model input:** every role request that repeats the active view pays its byte/token cost again. Do not count only unique ledger bytes. Include JIT system prompts, tool schemas, original request, history, repair notes and evidence escaping.
- **Model output:** explicit dispositions, rationales and evidence references add output tokens. A same-turn structured transition may avoid another critic call; a rejected transition may require a repair or another round.
- **External work:** any follow-up read, specialist consultation, retry, blocked tool or observation refresh counts. More objections must not silently multiply external calls beyond existing limits.
- **Engineering:** spec changes, schema evolution, producer/consumer wiring, policy, durable compatibility, explainability, tests, corpus regeneration and operational debugging.
- **Human attention:** clearer provenance may reduce review effort, while persistent uncertain objections may increase it. Measure both clarification count and review time.
- **Failure recovery:** count malformed responses, cancelled runs, unavailable evidence, resumed legacy threads and failed migrations, including their storage and model cost.

Use a common accepted endpoint when comparing approaches. A baseline that exits early with an unsupported design is not cheaper completion of the same task. Conversely, a ledger that blocks a valid solution indefinitely is not higher quality merely because it never says ready.

## 8 Evaluation plan with no claimed results

### Stage A deterministic contract witnesses

Use scripted role responses and stable fixtures to isolate host/policy behavior before testing live model quality. Exercise the production chat and mission constructors where possible, not only a stand-alone imitation of the policy.

| Case | Required result under the proposed contract |
|---|---|
| Unchanged c1 omits a prior critical item | Reconciliation incomplete; no ready disposition solely from omission. |
| Same key silently downgraded to important | Require an explicit admitted downgrade reason. |
| Genuine design revision addresses the concern | Permit readiness after a correctly linked resolution assessment. |
| Prior objection demonstrably mistaken | Permit an explicit retraction; do not preserve a false blocker forever. |
| c1 retired, unaffected c2 selected | Retain history without importing c1-only blockers into c2. |
| Candidate renamed with unchanged mandatory flaw | Renaming alone does not erase inherited or request-wide debt. |
| Failed probe followed by no alternative evidence | Remain unknown/open or explicitly reject the approach; no fabricated resolution. |
| Failed probe followed by valid alternative evidence | Allow a grounded transition without forcing the failed read again. |
| Duplicate or conflicting transitions | Reject ambiguity rather than last-writer selection. |
| Missing, stale or wrong-candidate evidence reference | Refuse that transition's claimed support. |
| Active-set overflow | Preserve incomplete coverage; never turn a reviewed prefix into whole-set readiness. |
| Legacy or resumed thread | Apply the specified compatibility policy without invented retirement history. |
| Values response | Resolve only its actual question/domain; retain unrelated critical obligations. |
| Already-ready clean first round | Preserve the baseline fast path without manufacturing objections. |

For each, record exact input/output, admitted facts, proof provenance, thread snapshot and disposition. Tests should assert both safety and liveness properties. These are future experiments, not receipts.

### Stage B adversarial and metamorphic checks

Permute candidate and item order; rewrite presentation-only prose; rename display keys with explicit lineage; vary unrelated files; change only evidence subjects; reuse the same display key in different candidates; inject an extra irrelevant objection. The expected behavior should follow semantic ownership and admission, not incidental order or labels.

Include negative controls where no new tool observation is necessary: a reasoning correction, a narrower design scope accepted by the original request, and a corrected mistaken interpretation. A policy that requires a fresh read in every case would fail these controls.

### Stage C fair model comparison

Compare at least:

1. Current conductor and prompts.
2. Prompt-only instruction to account for old objections.
3. Typed dispositions plus host completeness validation and Mangle lifecycle.
4. The same typed approach with additional independent verification only where a real oracle exists.

Hold model/version, reasoning settings, JIT corpus, workspace, tool access, permission mode, resource limits and task set constant. Pair cases, randomize condition order, separate tuning from evaluation and report seeds/repetition counts where available. Preserve negative results.

Use independently reviewed requirements and adversarial fixtures, not the same challenger as the sole success judge. Assess unresolved critical requirements in the selected design, unjustified retirements, valid designs wrongly blocked, trace coverage, recovery quality and end-to-end task completion. Report uncertainty and disagreements rather than reducing all outcomes to a single pass rate.

Report all attempted tasks, including stopped, failed, cancelled, budget-exhausted and unsupported cases. Measure both equal-budget quality and full cost to reach the same independently accepted endpoint. Include total input/output/cached/reasoning tokens where reported, all model and tool calls, consultation costs, retries, latency distributions, storage/proof overhead and human decisions. Unknown usage remains unknown.

### Stage D portability

Run the same logical objection lifecycle over Go, Python, JavaScript/TypeScript and Rust examples, including a mixed repository. Use source-backed but language-appropriate requirement subjects, such as concurrent admission, resource cleanup or compatibility. The lifecycle should not depend on Go package strings or one test runner's output.

Report adapter capability separately. A design lifecycle pass is not proof of installed tooling, complete test discovery, command authority, reproducible dependency resolution or a valid acceptance oracle in those languages.

## 9 Primary literature and what it supports

Gotel and Finkelstein's 1994 requirements-traceability study distinguishes tracing requirements before their specification from tracing their later realization. Applied here, the useful connection is from an objection's origin through refinement and disposition, not just from the final design to a test name. This motivates preserving reasoning history; it does not prove that the proposed schema improves codeNERD. [An analysis of the requirements traceability problem](https://discovery.ucl.ac.uk/id/eprint/749/)

Doyle's 1977 truth-maintenance paper describes recording justifications and revisiting dependent beliefs as those justifications change. The relevant lesson is that a changed conclusion should have an inspectable basis and dependencies. This audit does not propose replacing Mangle with that system or treating model-generated justifications as true. The application to objection retirement is a design inference. [Truth Maintenance Systems for Problem Solving](https://www.ijcai.org/Proceedings/77-1/Papers/035.pdf)

Huang and colleagues' ICLR 2024 study examines intrinsic self-correction without external feedback and reports limitations in its reasoning settings. It supports evaluating actual correction rather than assuming another round is better. codeNERD has tools and external evidence, so that paper does not directly predict its performance or establish the frequency of this omission. Its relevance is an evaluation caution, not a universal claim about current models. [Large Language Models Cannot Self-Correct Reasoning Yet](https://proceedings.iclr.cc/paper_files/paper/2024/hash/8b4add8b0aa8749d80a34ca5d941c355-Abstract-Conference.html)

## 10 Recommended next decision and exit criteria

Adopt a specification-level requirement that readiness accounts for earlier applicable critical objections through explicit dispositions. Resolve candidate lineage, request-wide applicability, legacy behavior and evidence-admission scope before implementing it. Start with the existing Service, JIT output contracts, saved rounds and Mangle policy rather than a parallel planner.

**Checkable exit:** through normal design-chat and recurse-mission entry paths, a candidate cannot obtain readiness solely by omitting or silently downgrading an earlier applicable critical objection. A legitimately resolved, superseded, mistaken or candidate-specific retired objection has an explicit, bounded and recoverable transition and does not cause unnecessary obstruction. Permission and verification boundaries remain unchanged. Independent evaluation establishes the quality/cost trade-off before any efficiency or correctness gain is claimed.

This audit establishes the missing accounting representation and its current-round consumer. It leaves semantic incidence, runtime behavior, production performance and the benefit of the proposed refinement unmeasured.

## Source links

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/0a7c74cf19c3d01036b8d6462273164e8c670808
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/agents.md#L5-L68
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/architecture/AGENTS.md#L35-L77
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L900
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/prompt/atoms/system/deliberation_proposer.yaml#L14-L48
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L52
[types-items]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/types.go#L17-L68
[types-thread]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/types.go#L71-L92
[types-frame]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/types.go#L103-L116
[validate]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/types.go#L144-L181
[round]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/round.go#L10-L72
[project]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/policy.go#L10-L39
[schemas]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/core/defaults/schemas_deliberation.mg#L1-L36
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/core/defaults/policy/deliberation.mg#L14-L58
[fact-lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/service.go#L53-L73
[service-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/service.go#L100-L165
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/outcome.go#L16-L37
[prove]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/policy.go#L42-L72
[save]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/persistence.go#L24-L74
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/outcome.go#L138-L181
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/cmd/nerd/chat/deliberation.go#L16-L50
[command]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/cmd/nerd/chat/deliberation.go#L96-L119
[mission-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/campaign/recurse_mission.go#L68-L97
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/campaign/recurse_mission.go#L174-L319
[fixture]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/cmd/nerd/chat/deliberation_test.go#L41-L86
[empirical-test]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/cmd/nerd/chat/deliberation_test.go#L230-L243
[validation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/deliberation_test.go#L50-L81
[stop-tests]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/cmd/nerd/chat/deliberation_test.go#L245-L268
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/config/routing.go#L18-L31
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/internal/deliberation/read_tools.go#L84-L207
[portable-objections]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/portable-recurse-deliberation-crucible.md#L87-L103
[progress-contract]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/deliberation-progress-observation-identity.md#L151-L179
[review-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/deliberation-selected-review-handoff.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/deliberation-values-evidence-freshness.md
[option-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/deliberation-values-option-admission.md
[work-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/planned-work-conservation.md
[continuation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/continuation-obligation-generation.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/0a7c74cf19c3d01036b8d6462273164e8c670808/Docs/audits/ASTRA_QUEUE.md

