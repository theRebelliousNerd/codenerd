---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: ab47cb643ef0de22d0cc43698f93fd1465203c98
supersedes: []
---

# Admit useful probes while design objections remain open

## Finding and decision

The conductor's empirical route is available only to candidates whose critical objections are all data objections. A candidate with both a concrete, independently executable data probe and a substantive critical design objection cannot use that route. If no other candidate qualifies as ready, empirical or a viable values fork, policy returns `needs_work`, and the conductor advances toward another model-led refinement round, subject to existing stop checks, without executing the named probe. The probe remains in the saved challenge but has no conductor dispatch. [Eligibility][eligibility] [Dispatch][dispatch] [Probe selection][probes]

**Classification: a proposed bounded-progress contract refinement, supported by an exact source boundary and an unrun conditional witness.** The current session specification explicitly describes the only-data restriction. This is not a claim that implementation violates that specification, that a live run has failed, or that the participants cannot investigate. Both participants already have their own guarded read-tool loop and receive the preceding challenge; either can resolve the uncertainty there. The narrower gap is that a valid, explicit request for useful evidence is not sufficient for the executive to schedule it when another kind of objection remains open. [Current specification][spec] [Role reads][role-reads] [Previous round][collector]

Strengthen the existing proposer/interrogator loop rather than adding another agent. Separate **readiness to adopt a design** from **eligibility to gather evidence useful to repairing it**. A conservative first treatment is a policy-owned mixed-objection investigation fallback only where the current result would otherwise be `needs_work`. Keep every unresolved design or values objection blocking adoption. Do not relax permissions, bypass denied reads, increase budgets, or mistake a successful read for a successful implementation.

The practical benefit to investigate is fewer avoidable unresolved designs and fewer rounds spent refining without the requested fact. That benefit is unmeasured. More probing can also waste time and money on designs that should simply be rejected.

### Source and execution boundary

The initial public main observation was 2a981f03eb95380a8066f16ad4319b240b3d922a. The final check at 14:20 UTC on November 3, 2026 resolved to **ab47cb643ef0de22d0cc43698f93fd1465203c98**, committed at 14:18:56 UTC. The complete comparison contains nine changed files in campaign/session execution and effect handling; none of the guidance, architecture, deliberation implementation, mission preparation, authored tests or neighboring audits cited here changed. The changed patches were inspected for impact on this pre-execution boundary. Repository links below use the final immutable pin. Initial source snapshots were checked against the public Git tree's blob identities, with the unchanged-file comparison establishing their applicability to the final pin. [Source commit][pin] [Comparison][comparison]

This audit read public repository guidance, current architecture, relevant implementation, authored tests, neighboring audits and their queue entries. It uses no private-project source or design details. No repository program, test, build, model inference, benchmark, gate or failure injection was run. No implementation, repository document, credential, permission, configuration or setting was changed. The witness, proposed contract, design alternatives and evaluation are unimplemented and unrun.

## 1. Why this matters to a Socratic requirements crucible

codeNERD already has the requested structure: a proposer develops materially different readings or designs, a requirements interrogator challenges them, and Mangle owns selection and stopping. The proposer must retain explicit requirements and use workspace evidence; the challenger distinguishes critical data, design and values items and supplies exact tools and arguments for data items. The existing portable-crucible proposal already pairs factual uncertainty with bounded probes and design contradictions with revision or rejection. This audit makes one interaction between those categories explicit. [Root contract][guidance] [Proposer][proposer] [Interrogator][interrogator] [Existing crucible direction][crucible]

Real design work often contains several unresolved kinds at once:

- A cache proposal lacks required invalidation semantics.
- A permitted source or documentation read can establish which update notifications and revision identifiers already exist.
- The read cannot itself add the missing semantics, but it can determine which refinement is feasible.

It would be incorrect to mark the design objection resolved merely to obtain the read. It would also be incorrect to relabel a substantive design defect as a data question. The question is whether the executive can investigate one admitted uncertainty while honestly preserving the other.

The recommended requirement is conditional, not “execute every proposed read”: when refinement is otherwise blocked, a useful, independently executable, authorized probe should be schedulable within the existing envelope, or receive a specific deferral/refusal disposition. The system must still be allowed to reject a doomed design, conserve resources, or stop when evidence cannot be obtained.

This is a progress and cost-assurance issue. The inspected behavior remains conservative about adoption: mixed critical objections do not make a design ready. No safety bypass, unauthorized code change or incorrect verification result is established.

## 2. Implemented behavior and the missing distinction

| Boundary | Implemented at the pin | Consequence |
|---|---|---|
| Item contract | An assessment can contain several items, each with a key, severity, kind, detail and optional tool call. | Critical data and critical design items can coexist structurally. |
| Validation | Kinds and severities are closed; every critical data item requires a non-nil probe. There is no prohibition on mixed kinds. | The primary witness is valid participant output, not a malformed-response fallback. |
| Fact projection | Candidate identity/rank, item key/severity/kind, success and citations enter Mangle. | The executive sees the mixed classification; probe arguments remain in the saved Go round. |
| Empirical eligibility | A candidate needs a critical item and absence of any critical non-data item. | One critical design or values item excludes its conductor probe route. |
| Values eligibility | A candidate needs success and only critical values items; a fork requires two viable alternatives. | Mixed data/design candidates do not become preference questions. |
| Driver | Only the `empirical` verdict invokes `runProbes`; `needs_work` has no probe action. | The next refinement can start without the explicit new observation. |
| Participant tools | Each role can independently request permitted reads under its own evidence/context controls. | A real model can rescue the case; conductor non-dispatch is not universal inability to read. |
| Persistence | The whole round, including challenge tool calls, is saved. | The probe request is retained; it is not an execution receipt or proof of consumption. |

Sources: [Types][item-types] and [validation][validation], [fact projection][projection], [policy][eligibility], [driver][dispatch], [role loop][role-reads], [snapshot writer][persistence].

The current session specification is candid: an empirical candidate has only data blockers, while a design blocker requests refinement. A change should update that contract deliberately, with its observable acceptance conditions, rather than rewriting “implemented” status to imply the stronger behavior already exists. [Specification][spec] [Architecture guidance][architecture]

## 3. Exact source chain

### 3.1 A mixed assessment is valid

`validateRound` checks candidate count, nonempty unique candidate identifiers, nonblank briefs/success tests, exactly one assessment per candidate, unique item keys within each assessment, allowed severity and kind, and a non-nil probe for critical data. It does not require that all critical items on an assessment share a kind. A critical data item with a concrete tool call plus a critical design item therefore passes these particular structural checks. Actual tool availability and authorization remain separate execution concerns. [Validation][validation]

`assertRound` projects every item into `deliberation_item(thread, round, candidate, key, severity, kind)`. It does not discard either side of the mixture. The candidate's proposal order supplies the rank used for selection. [Projection][projection]

### 3.2 Candidate-level classification prevents investigation

The policy derives `deliberation_nondata` from any critical item whose kind is not data. Empirical eligibility requires the absence of that fact. A critical design item therefore excludes an otherwise concrete data probe from candidate-level empirical eligibility. [Kind and eligibility rules][eligibility]

The same mixed candidate cannot be ready because it has critical items. It cannot be a values-only candidate because its critical data/design items derive non-values status. If the other candidates are likewise ineligible, the remaining rule derives `needs_work`. This is a normal policy result, not a missing-decision error. [Decision precedence][decisions]

These decision rules have no separate item-level investigation eligibility relation. That is the missing distinction: a candidate can be unready while one of its unresolved questions is ready to investigate.

### 3.3 The conductor does not dispatch a probe for that result

The service obtains one decision, records a proof, appends and saves the round, then switches on the verdict. Ready and values outcomes finish; empirical invokes `runProbes`; needs-work advances without an intervening conductor probe. [Decision and dispatch][dispatch]

`runProbes`, when called, considers only the selected candidate and only its critical data items. It checks the call allowance, executes the supplied call, saves output or error, and projects a receipt digest. Another candidate's empirical selection does not incidentally execute the mixed candidate's read. [Probe runner][probes]

This finding occurs before any probe result exists. Changing result attribution, proof rendering or observation hashing alone cannot make this missing dispatch occur.

### 3.4 Refinement is a real alternative, but not an executive guarantee

The next round receives the previous round in its frame. The proposer and challenger each run through the ordinary deliberation role agent, whose offered tools and read execution do not require the candidate-level empirical verdict. A participant can inspect the missing fact, amend the design, discard the candidate, or propose a purely factual subproblem. [Collector][collector] [Registered role][shard] [Participant reads][role-reads]

That alternative limits the claim. The only-data design may intentionally avoid reading for candidates likely to be rejected. A competent participant can also resolve factual prerequisites while refining. The proposed change must demonstrate added value over that existing behavior, not against an artificially tool-disabled baseline.

However, relying on a later participant to notice and reissue a precise probe differs from the executive scheduling the probe already present in the structured assessment. The latter can make the refinement contract more dependable when the model truthfully retains several objections.

### 3.5 Recurse reaches this shared boundary

Mission preparation passes the full mission, selected attempt, history reference, preceding receipt and North Star into a design-only deliberation thread. It calls the same `Service.Run`. Its action mapping treats proceed outcomes as ready, ask-once as a values pause, and other outcomes as stopped before the subsequent mission admission check. This proposal therefore concerns an existing recurse design path, not a hypothetical standalone agent. [Mission input][mission-input] [Mission handoff][mission-handoff]

The mission source/gate/write-scope/permission checks remain separate. A newly admitted investigation must not authorize an attempt, change its node, alter its check, or convert an unresolved design into a passing witness. [Mission requirements][mission-spec]

The direct `/deliberate` and `/innovate` command also constructs a design-only thread and calls the same service. The core interaction is not specific to a programming language. [Chat entry][chat]

## 4. Minimal unrun witness

Use a fresh design-only thread, a stable workspace, normal proof support, successful persistence, available context and call allowances, no consultations, and two valid first-round candidates. Use a permitted local file read rather than an MCP operation so the witness does not depend on the separate effect-ceiling audit.

### Workspace and request

The synthetic workspace contains an ordinary short `api-notes.md` file describing the existing update lifecycle: committed updates expose a record identifier and monotonically increasing revision to an event sink. The task is to design a lookup cache while preserving the requirement that a read after a committed update cannot return the old value.

The file is an evaluation fixture, not a claim about a particular existing codeNERD file.

### Candidate A

A proposes a process-local cache keyed only by record ID. Its nonblank success test describes fresh reads following committed updates. Its assessment has success false and two critical items:

1. **Design, key `invalidation-semantics`:** the proposal does not specify how writes invalidate or advance cached entries; it therefore does not yet satisfy the required visibility behavior.
2. **Data, key `update-hook-shape`:** establish the existing update hook's payload and ordering. Supply a concrete offered `read_file` call for `api-notes.md`.

The first is a substantive design omission, not a rephrasing of missing information. Reading the file will not itself design or validate invalidation. The second is independently executable and can inform whether to refine around revision keys, invalidation callbacks or another mechanism.

### Candidate B

B proposes a time-to-live-only cache. Its successful-read condition is explicit, but its assessment has success false and a critical design item noting that a nonzero TTL permits stale reads after a committed update. It has no data probe. Recommend A. Supply ordinary supporting citations and valid unique IDs/keys; do not depend on the separate citation-admission issue.

### Predicted first-round result

Under the inspected source:

1. Structural validation accepts both assessments.
2. A derives critical and non-data status, so it is not empirical; neither candidate is ready or values-eligible.
3. Policy derives `needs_work`.
4. The round is saved.
5. The conductor probe callback receives **zero calls** for `update-hook-shape`.
6. The next proposer sees the preceding challenge and may independently read, refine or reject.

These are source-predicted outcomes. No fixture was executed.

### Conditional repeated schedule

For the liveness stress case, scripted participants return the same candidates and stable critical items without role-internal reads or consultations. With a fresh fingerprint and the source defaults, round one sets the fingerprint and leaves the unchanged count at zero; rounds two and three advance it to one and two. Before round four, the no-progress rule stops the thread. The schedule has three completed rounds and six participant invocations, with zero conductor probe executions. [Fingerprint][fingerprint] [Loop and stop][loop] [Default bounds][bounds] [Stop policy][stops]

That count is a static control-flow calculation for this scripted schedule, not measured latency, provider cost or a claim about normal model behavior. A participant invocation can contain multiple provider exchanges; the simple scripted response contains none of the actual paid-model work. Changing IDs, observations, output, budgets or participant behavior can change the schedule.

The desired treatment must obtain a new fact **without deleting `invalidation-semantics`**. A later proposed invalidation design still needs a new assessment. Even if that design becomes ready, implementation and verification remain future work.

### Essential counterexamples

- If a participant performs the read itself and refines successfully, the baseline may be just as good or better.
- If A's design is unrepairable or the probe is irrelevant, rejecting A is preferable to probing it.
- If the supposed design item merely means “I do not know the hook,” it should be classified as data; that is not this witness.
- If a candidate already qualifies as ready, preserve its existing priority rather than investigating every alternative.
- If another current empirical or values route exists, the conservative fallback proposed below leaves that existing decision intact.
- If the read is denied, unavailable or would exceed the remaining envelope, preserve the unresolved state. It must not acquire a success receipt.

## 5. Proposed contract: investigate an objection without retiring the rest

Everything in this section is a proposed design, not an existing API, schema or implementation.

### 5.1 Minimal semantics

Add a checkable requirement, provisionally **GAP-DELIBERATION-MIXED-INVESTIGATION**:

> When the ordinary decision would be needs-work, an explicitly identified critical-data probe on a mixed-objection candidate may be selected for bounded investigation if it is independently executable, useful to a current refinement decision, authorized, and leaves an admitted opportunity to consume its result. Investigation does not resolve any design or values objection by itself.

The obligation is to derive a disposition for a useful probe, not to promise all probes execute. An operation may be selected, deferred to a named prerequisite, refused by a guard, skipped because its candidate is rejected, or held because the envelope is exhausted. These labels are proposed distinctions; current snapshots do not encode this complete state machine.

Keep ready selection, existing all-data empirical selection and already-viable values forks ahead of the fallback for the first treatment. This limits behavioral change and avoids delaying an available preference decision with speculative investigation of another design. Broader information-value scheduling is a separate proposal requiring its own evidence.

### 5.2 Responsibilities

**Model and JIT contract:** identify the exact question, a concrete offered read, what outcome would distinguish refinements, and why the read can happen before the other objection is resolved. Preserve true critical classifications. The model's explanation is a proposal of usefulness, not authority or a proof of semantic independence.

**Go boundary:** validate identities, arguments, references and bounded data shapes; expose host-observed availability, permission outcomes and costs; execute only the selected guarded call; retain the result with its owner and subject. Do not hide an executive “best candidate” decision in a Go heuristic.

**Mangle:** derive whether the fallback is available, which candidate/item is selected, whether the active budget and consumption opportunity permit it, and what remains blocked afterward. The policy must produce a unique, inspectable outcome. Existing ranking can be retained for a narrow treatment; do not treat prose estimates as calibrated utilities.

**Caller:** preserve original request, mission scope, constraints and disposition. A probe result is evidence for the next review, never execution approval.

The existing driver already filters critical data items when running probes. That reusable mechanism is not evidence that merely changing one negated predicate suffices. A new verdict or subphase must be wired through the driver's exhaustive switch, proof support, persistence, bounds and production callers. [Existing runner][probes] [Existing switch][dispatch]

### 5.3 Small first treatment versus a dependency graph

A small treatment can reuse current candidate/item/call identities and admit the fallback for a single selected mixed candidate, then re-enter the ordinary refinement loop. It adds no new agent and need not add a new model call merely to classify the candidate. Its main trade-off is that the present structure does not express formal dependencies or irreparable rejection, so an apparently useful probe can still be wasted.

If controlled cases show that this trade-off matters, add explicit prerequisite and disposition records. Candidate revision, item identity and source scope must distinguish “not yet meaningful,” “not authorized,” “obsolete,” and “not worth further investigation.” Unknown semantic dependencies should not be declared proven independent by the host.

Do not build a general planning graph just because a graph is possible. It adds schema, validation, cycle handling, migration, replay and context costs. Compare that treatment against the smaller fallback before adopting it.

### 5.4 Preserve evidence and lifecycle constraints

- Keep candidate and item identity attached to the selected call and its arguments. Reuse or extend the invocation-identity work; a tool name and output string alone are insufficient for all read types. [Existing identity audit][identity-audit]
- Retain the substantive non-data objection until a later explicit review accounts for it. This proposal does not solve the separate critical-item retirement gap. [Existing retirement audit][retirement-audit]
- Leave a bounded consumer phase. Dispatching a new probe on the last usable round and stopping before inspection would replace this gap with the existing consumption-horizon gap. If no consumer can be admitted, record the read as unexecuted/deferred rather than silently borrowing another round. [Existing horizon audit][horizon-audit]
- An error, truncation or denied read is not a complete observation. A retry needs ordinary policy and budget admission; repeated failure is not progress.
- Persist selected work and observed outcomes using the existing thread ownership. On restart, distinguish a retained request from a completed read; do not assume exact-once execution or silently replay remote operations.
- Keep a conservative unresolved outcome where source freshness, evidence identity or applicability is unknown. Do not represent an old probe as current truth after a relevant change.
- The label “data probe” must not widen the read-only effect ceiling. The known MCP effect-ceiling question remains an independent safety dependency for those tools; the narrow local-read treatment must not claim to solve it. [Existing effect audit][effect-audit]

### 5.5 Portability boundary

This is a language-independent deliberation protocol. It can apply to Go, Rust, Python, JS/TS, a mixed repository or a documentation-only design if an admitted tool can supply the relevant observation.

It does not establish universal tool or recurse support. Repository layout, OS path rules, encodings, generated artifacts, permissions, unsupported tools, remote service semantics and source freshness can all change whether a probe is meaningful or available. Preserve the typed capability limits and design-only fallback recommended in the broader portable-crucible proposal. Do not add a shell escape to compensate for a missing adapter. [Portable capability direction][portable]

## 6. Costs and failure modes

### Full cost accounting

The small fallback may reduce model-only repetition, but it also admits reads the baseline would avoid. No speedup or token saving is claimed.

Account for the complete request:

- **Provider work:** proposer and challenger inputs, outputs, reasoning usage when available, cached/uncached tokens, JIT and tool-definition context, consultations, retries, errors and any later assessment. A fixed tool allowance is not a fixed number of provider requests or dollars.
- **Tool work:** filesystem or index I/O, search extent, remote latency and fees where applicable, parsing/serialization, guard checks, and discarded or duplicate observations. Small returned output does not prove cheap retrieval.
- **Executive work:** new facts, joins, selection queries, proof recording and cleanup. A bounded extra candidate/item scan is small relative to some reads, but actual fixpoint costs depend on the program and store; measure them.
- **Context:** incoming evidence plus metadata competes with the next round's frame and closing notes. The complete encoded payload, including escaping, must remain admissible. More evidence can increase refusal or truncation rather than improve the decision.
- **Durability:** the current writer serializes the whole thread, syncs a temporary file and renames it. Extra evidence appears again in later full snapshots. Count cumulative bytes serialized/written, sync latency and recovery overhead, not just final artifact size. [Snapshot writer][persistence]
- **Opportunity cost:** investigating A may delay useful refinement, another candidate or a timely preference question. Preserve existing decision priorities in the first treatment and measure this delay.
- **Engineering and maintenance:** specification changes, policy/driver integration, new records if needed, migrations, explanation rendering, platform controls, regression coverage and independent semantic review. A dependency-graph treatment costs more than the minimal fallback.
- **Outcome cost:** include valid designs unnecessarily blocked, bad designs incorrectly admitted, unnecessary questions and unresolved sessions. Optimizing raw probe count can reward doing no useful work.

For each treatment, report total resources per attempted task and per independently accepted design. Report failures and censored runs in the denominator. Keep observed provider usage separate from estimated prices and unknown spend. Existing mission limits and uncertainty stops remain authoritative.

### Failure modes to make explicit

| Failure mode | Required outcome |
|---|---|
| Probe is useful but another substantive objection remains | Read may be admitted; adoption remains blocked. |
| Candidate should be rejected regardless of the answer | Reject or explicitly defer; do not spend merely because a probe exists. |
| Probe needs a design choice that has not been made | Retain that prerequisite; do not fabricate concrete arguments. |
| Participant already obtained equivalent applicable evidence | Reuse only with established identity/freshness; avoid a duplicate read. |
| Candidate or source changes before consumption | Revalidate applicability; retain obsolete evidence as history, not current truth. |
| Read is denied or unavailable | Preserve the reason and unresolved item; do not switch tools to evade a guard. |
| Evidence exceeds context | Preserve truthful coverage/truncation and a permitted narrowing route, or stop unresolved. |
| No consumer fits the remaining envelope | Do not dispatch an otherwise useless final read or increase the envelope silently. |
| Several candidates/items contend | Unique policy selection and explicit deferred work; no accidental iteration-order priority. |
| Probe returns success but design still lacks required behavior | No automatic item retirement or ready outcome. |
| Saved request survives an interrupted read | Distinguish unknown execution outcome from an unstarted operation before retry. |
| Only user preference remains | Existing bounded values handling, not an invented factual answer or mission auto-selection. |

These are proposed obligations, not claims that the present system already handles every case.

## 7. Fair, unrun evaluation

### 7.1 Deterministic policy and conductor controls

Use the real policy and conductor with controlled participant responses and a counting local-read fixture. Keep proof recording, normal persistence and real argument/permission validation in the end-to-end fixture. A pure policy test is useful but not a substitute for the production route.

The initial differential should assert the exact first-round boundary: valid mixed assessment, `needs_work`, and zero current conductor dispatches. A treatment should admit the named read only under its declared fallback contract and show that the next admitted review receives the corresponding observation.

| Case | Control purpose |
|---|---|
| Critical data + critical design, useful independent read | Primary changed case; retain the design blocker after reading. |
| Critical data only | Preserve ordinary empirical behavior. |
| Critical data + important design | Distinguish the critical-only exclusion from a blanket mixed-kind rule. |
| Critical design only | Do not invent a probe or label refinement empirical. |
| Critical data + values | Secondary mixed case; no premature question while factual viability is unresolved. |
| Another ready candidate | Preserve current ready priority. |
| Another ordinary empirical candidate | Preserve first-treatment precedence and do not attribute its result to the mixed candidate. |
| An existing viable values fork | Preserve that current route in the conservative fallback treatment. |
| Candidate rejected, probe irrelevant, or query dependent on unresolved choice | Measure and prevent unjustified extra work. |
| Role-internal read resolves the same uncertainty | Baseline rescue must remain available and be counted. |
| Denied/malformed/unavailable tool | Preserve refusal and unresolved evidence; no authority expansion. |
| Repeated identical request/result | No fabricated progress or unbounded duplicate probing. |
| Terminal-round, full-context or cancellation boundary | No unconsumable dispatch, borrowed budget or false completion. |
| Restart between selection, dispatch, persistence and review | Truthful ownership and outcomes without assumed exact-once behavior. |

The visible authored tests cover structural validation, choices, fingerprint behavior, persistence, role budgets/context and a Cortex boot-ready case. Those are useful existing controls, but the named tests do not themselves demonstrate mixed-objection conductor admission or its quality benefit. They were read, not run; no repository-wide coverage claim is made. [Validation tests][tests] [Agent tests][agent-tests] [Boot test][boot-test]

### 7.2 Production-entry validation

After an authorized implementation exists, exercise both the normal design-only chat command and bounded recurse mission preparation. Confirm that the selected observation reaches a subsequent proposer/challenger frame through the actual service wiring. Verify that no attempt executes while the design objection remains unresolved and that mission source, gate, budget and permission admission are unchanged.

Use controlled providers first. Live models and actual tool adapters are a later authorized evaluation, not part of this audit. Compare identical source snapshots, participant prompts, tool catalogues, model/provider identities, budgets and original requirements wherever possible.

### 7.3 Semantic quality and cost study

Use a prespecified, repository-disjoint set of requirements/design tasks across several languages, mixed repositories, documentation-only fixtures and at least one unsupported ecosystem with admitted ordinary reads. Include both useful independent probes and traps where a read cannot help. Do not choose only tasks designed to favor the fallback.

Compare at least:

1. Current only-data conductor policy with normal participant reads.
2. Current policy plus a prompt-only reminder to perform useful reads during refinement.
3. The conservative policy-owned mixed-objection fallback, retaining the same overall resource envelope.
4. A dependency-aware treatment only if the simpler comparison identifies a need.

For the prompt-only comparison, report that its prompt differs; do not attribute all improvement to policy. For any graph-based treatment, count its added classification/review work. Run matched seeds where supported and enough repetitions to expose model variance. Independently review the final designs against explicit requirements; do not let the same interrogator be the sole outcome judge.

Primary outcomes should include independently acceptable designs, incorrect readiness, avoidable unresolved designs, correct rejection, preserved critical objections and successful evidence consumption. Secondary outcomes include total latency, provider usage/cost, source/tool work, round counts, repeated reads, user questions and context failures. Report uncertainty and disagreements. A greater fraction of probes executed is not by itself success.

A rejection criterion for the proposal is appropriate: if ordinary participant reads already resolve these tasks reliably and the fallback mainly adds redundant or irrelevant work without better outcomes at equal cost, keep the simpler current policy. If the fallback improves only a narrow task family, constrain its applicability rather than claiming universal benefit.

## 8. Primary literature and limits of inference

**ReAct** studies interleaving reasoning with environment actions so observations can update plans. It supports testing an explicit observation/refinement connection. It does not establish this particular codeNERD scheduling rule, candidate taxonomy, effect boundary or cost benefit. [Yao et al., ReAct](https://arxiv.org/abs/2210.03629)

**Self-Refine** reports benefits from iterative feedback and refinement on its studied tasks. It gives a serious baseline reason to retain ordinary model-led refinement instead of assuming every objection needs another tool call. The experiment here must compare against that existing route rather than replace it on architectural preference alone. [Madaan et al., Self-Refine](https://arxiv.org/abs/2303.17651)

**Large Language Models Cannot Self-Correct Reasoning Yet** reports limitations of intrinsic reasoning correction without external feedback in its evaluated settings. This motivates measuring whether useful external facts reduce unsuccessful repetition; it is not proof that codeNERD's models cannot refine or that probing always improves them. [Huang et al.](https://arxiv.org/abs/2310.01798)

**Principles of Mixed-Initiative User Interfaces** discusses decisions under uncertainty, the costs and benefits of intervention, and user control. Applied here, factual investigation and owner preference should remain distinct, and neither “ask more” nor “read more” should be treated as free. This is a design inference, not a result about this repository. [Horvitz, CHI 1999](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/11/chi99horvitz.pdf)

## 9. Relationship to existing audits

The reviewed neighboring corpus already supplies most of the broader crucible design. This report should remain a focused addition:

| Existing work | Boundary already covered | Distinct contribution here |
|---|---|---|
| Portable recurse deliberation crucible | Evidence-producing design funnel, factual probes, design revision and capability limits. | The current candidate-wide only-data restriction and a mixed-objection admission contract. |
| Probe-consumption horizon | A dispatched probe can lack a later consumer. | The explicit probe is not dispatched in the first place, even with review capacity available. |
| Progress-observation identity | Different executed reads can collapse in progress accounting. | There is no executed result to hash. |
| Critical-item retirement | Earlier objections can disappear from a later readiness assessment. | Preserve those objections while allowing investigation; do not resolve the gap by deleting them. |
| Read-invocation identity / citation admission | Evidence may lose its subject or lack grounded admission. | These remain prerequisites for trustworthy results, not explanations of the pre-dispatch exclusion. |
| Values option, evidence freshness and answer recovery | Offered choice authority, applicability and durable continuation. | The primary case is before any values question, with a real design objection. |
| MCP effect ceiling / proof ownership | Effects and recording authority need exact ownership. | The primary witness uses an admitted local read and working proof support; no widening or repair is claimed. |
| Selected-review handoff | Important review findings can be omitted at delivery. | This concerns critical open work before a ready handoff exists. |

Sources: [Crucible][crucible], [horizon][horizon-audit], [progress identity][progress-audit], [retirement][retirement-audit], [read identity][identity-audit], [citation][citation-audit], [values admission][values-audit], [freshness][freshness-audit], [answer recovery][answer-audit], [effect ceiling][effect-audit], [proof ownership][proof-audit], [review handoff][handoff-audit]. The values-admission audit already uses mixed blockers as a non-ready control; this proposal retains that result and changes only the opportunity to investigate before readiness.

## 10. Decision and exit criteria

**Decision recommended for review:** accept the narrower requirement that an unready design may still have useful evidence work, and evaluate a policy-owned fallback before building a richer planner. Do not yet claim an implementation defect against the only-data specification or promise that this change is beneficial on all repositories.

A future implementation can close the proposed gap when:

1. The specification distinguishes design adoption from mixed-objection investigation and records the chosen scheduling priority.
2. A source-pinned real-policy/conductor witness covers the primary case and its counterexamples.
3. The original design objection remains blocking after the read until a subsequent explicit review resolves, rejects or supersedes it.
4. The selected result is associated with its candidate, item, call and applicable source, and reaches an admitted consumer.
5. Existing ready/empirical/values priorities, budgets, evidence boundaries and mission admission remain intact under the declared treatment.
6. Denied, useless, obsolete and out-of-envelope probes have truthful dispositions.
7. Production-entry checks and a fair quality/cost comparison support the intended scope, with limitations reported.

Until then, the supported result is a static architectural finding and a bounded proposal. The system already has the Socratic participants; the next question is whether the executive should give a useful factual subtask a route through a still-unresolved design.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/ab47cb643ef0de22d0cc43698f93fd1465203c98
[comparison]: https://github.com/theRebelliousNerd/codenerd/compare/2a981f03eb95380a8066f16ad4319b240b3d922a...ab47cb643ef0de22d0cc43698f93fd1465203c98
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/agents.md#L3-L66
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/architecture/AGENTS.md#L48-L106
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/architecture/session/IMPLEMENTED_SPEC.md#L818-L847
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/prompt/atoms/system/deliberation_proposer.yaml#L14-L28
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L48
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/types.go#L144-L181
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/policy.go#L10-L49
[eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/core/defaults/policy/deliberation.mg#L14-L36
[decisions]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/core/defaults/policy/deliberation.mg#L43-L58
[stops]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/core/defaults/policy/deliberation.mg#L38-L41
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/service.go#L120-L164
[loop]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/service.go#L96-L164
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/outcome.go#L138-L181
[fingerprint]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/outcome.go#L116-L135
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/round.go#L10-L71
[role-reads]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/read_tools.go#L57-L210
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/shards/deliberation.go#L49-L64
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/persistence.go#L25-L64
[bounds]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/config/routing.go#L18-L31
[mission-input]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/campaign/recurse_mission.go#L174-L248
[mission-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/campaign/recurse_mission.go#L248-L319
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L102-L125
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/cmd/nerd/chat/deliberation.go#L96-L119
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/deliberation_test.go#L34-L176
[agent-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/agent_test.go#L67-L182
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/system/deliberation_boot_test.go#L17-L49
[crucible]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/portable-recurse-deliberation-crucible.md#L87-L103
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/portable-recurse-deliberation-crucible.md#L35-L65
[horizon-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-probe-consumption-horizon.md
[progress-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-progress-observation-identity.md
[retirement-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-critical-item-retirement.md
[identity-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-read-invocation-identity.md
[citation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-citation-admission.md
[values-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-values-option-admission.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-values-evidence-freshness.md
[answer-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-answer-evaluation-recovery.md
[effect-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-mcp-effect-ceiling.md
[proof-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-proof-recording-ownership.md
[handoff-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/Docs/audits/deliberation-selected-review-handoff.md

[item-types]: https://github.com/theRebelliousNerd/codenerd/blob/ab47cb643ef0de22d0cc43698f93fd1465203c98/internal/deliberation/types.go#L17-L69
