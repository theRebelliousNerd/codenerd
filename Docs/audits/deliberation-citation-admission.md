---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: ccbe41302073b364ef2cb40ba49b9736738ae6e2
supersedes: []
---

# Admit evidence references before they support a ready design

## Finding and scope

The ordinary deliberation readiness rule requires a citation, but its input is the presence of any string in a model-authored evidence list. The candidate validator never examines that list; the fact adapter emits every entry; the rule tests only that a citation fact exists. Consequently, an empty string, whitespace or an invented locator can satisfy the citation premise when the challenger reports success and no critical objection. An empty list cannot satisfy that same premise. [Candidate and validation][types] [Projection][projection] [Readiness][policy]

**Confirmed static finding:** the evidence-presence check distinguishes an empty evidence list from a nonempty one, not an admitted evidence reference. **Conditional consequence:** an otherwise admissible first-round candidate with no usable citation can receive the ordinary ready/proceed disposition. This is not a demonstrated bad design, observed model hallucination, permission bypass or falsely passing implementation test. The candidate may be semantically good despite its deficient evidence record. The issue is what the host's readiness evidence can establish.

Strengthen this existing funnel rather than add another agent framework: reject blank evidence entries, distinguish a model-proposed locator from a host-resolved source reference, and let Mangle require admitted evidence where the design's declared grounding contract requires it. Preserve creative reasoning and explicitly unsupported hypotheses. Do not equate source existence with proof that a design is correct.

This report uses public CodeNERD source at [ccbe41302073b364ef2cb40ba49b9736738ae6e2][pin], resolved as main on November 3, 2026. Guidance, architecture, implementation, related audits and authored tests were read. No repository program, build, test, gate, model inference, benchmark or failure injection was executed. No code, settings, credentials or repository files were changed. All witnesses, refinements and evaluations below are unrun proposals.

Before freezing, main advanced to `74dca64c1f99aa1305038184587dbb33b96b61d7`. The [complete two-commit comparison](https://github.com/theRebelliousNerd/codenerd/compare/ccbe41302073b364ef2cb40ba49b9736738ae6e2...74dca64c1f99aa1305038184587dbb33b96b61d7) changes eight other files, leaving the implementation, guidance and audit files cited here unchanged. The report retains its original immutable pin rather than imply that any newer runtime was exercised.

## 1. The existing Socratic contract

The repository assigns creative synthesis to the model and executive decisions to Mangle. It requires revision-aware, recoverable evidence and JIT-first behavior. Architecture guidance requires intended contracts to remain separate from current implementation and evidence. This finding does not propose replacing a challenger with a deterministic semantic oracle. It concerns the smaller boundary between the challenger's judgment and the source identity that judgment cites. [Root guidance][guidance] [Architecture rules][architecture-guidance]

The proposer is instructed to preserve the complete request, identify assumptions, inspect workspace evidence and supply citations. The challenger is instructed to assess against the request and evidence, keep unresolved blockers explicit, and never fabricate evidence to fit a budget. Those are useful model instructions. They do not create a host admission check for the evidence strings returned by the model. [Proposer][proposer] [Challenger][challenger]

The current session specification correctly distinguishes readiness of a proposed reading from verification of an implementation. It documents typed round facts, policy selection, bounded evidence collection and recorded proof envelopes. It also labels the relevant paths source-authored rather than claiming completed runtime validation. Keep those distinctions. This report does not convert a design-ready status into an implementation-complete guarantee. [Session contract][session-spec]

Already implemented safeguards include:

- A divergent first round contains at least two candidates, within the configured cap.
- Every candidate has one assessment, a nonblank brief and success-test text, and a unique identifier.
- Critical data, design and values objections have distinct handling.
- Mangle selects a candidate and determines continuation or stopping.
- The service requires a recorded proof before returning a decision.
- The selected design and complete round remain in a durable thread.
- Design-only chat returns a brief without dispatching a development worker.
- Recurse mission preparation retains separate source/vision, scope, permission and verification controls.

These remain necessary even after evidence admission is strengthened. [Round collection][round] [Validation][types] [Decision loop][service] [Proof][proof] [Persistence][persistence] [Chat command][chat-command] [Mission preparation][mission]

## 2. Exact producer-to-consumer chain

### 2.1 Evidence is unstructured model output

`Candidate.Evidence` is `[]string`. It has no source-kind field, observation identifier, source version, range, admission state or declared relationship to a particular claim. `collectRound` decodes the proposer's result, passes the candidates to the challenger, decodes the challenge and calls `validateRound`. [Candidate shape][candidate-shape] [Collector][round]

`validateRound` checks candidate count, identity, brief, success-test text, assessment coverage and objection vocabulary. It neither requires evidence nor rejects blank entries or checks that a locator refers to available material. Allowing evidence-free candidates during exploration can be reasonable: an incomplete proposal must be expressible. The missing boundary is promoting a list entry into readiness evidence without a separate admission step. [Validation][types]

### 2.2 Every entry becomes the same kind of fact

`assertRound` loops through every candidate's `Evidence` and emits `deliberation_citation(thread, round, candidate, citation)`. There is no trimming, resolution or receipt lookup in that loop. The adapter explicitly uses `types.MangleString`, whose `ToAtom` case constructs a string constant directly; it does not reject an empty value. The schema requires a string, not a nonblank or resolved reference. [Projection][projection] [String conversion][string-conversion] [Citation schema][schema]

This is not an injection finding. The explicit typed-string conversion is a safeguard to preserve. The issue is that a correctly typed empty or invented string is nevertheless admitted as a citation fact.

### 2.3 Ordinary readiness uses existential presence

The ready rule joins candidate, brief, citation and success facts and requires no critical item. Its citation argument is anonymous: no value, observation, source, coverage or resolution predicate is checked. A successful assessment plus `Evidence: []string{""}` therefore supplies the same citation-existence premise as a usable source reference. [Readiness rule][policy]

The empirical and values rules have different premises. In particular, the values rule does not require a citation. That is already an identified policy-design question in the values-option audit; it is not being presented here as a second new defect. This report's minimal witness concerns only the ordinary ready path. [Values-option report][values-audit]

### 2.4 Proof recording preserves the derivation, not missing source checks

`Service.Run` asserts the round, obtains exactly one decision, records its proof and finishes immediately for a ready verdict. `finish` derives the disposition and sets the thread ready on proceed. The proof recorder checks that proof support exists in the current epoch and retains digest, generation and truncation information. None of that resolves a citation string or proves the cited material supports the candidate. [Decision loop][service] [Finish][finish] [Proof recorder][proof]

A valid policy proof can therefore faithfully explain a decision whose admitted citation premise is only an empty string. This is a limitation of the premise, not a reason to remove proof recording or distrust every Mangle derivation.

### 2.5 Both idea chat and recurse use this conductor

The chat constructor dispatches the registered roles through the shared service. `/deliberate` and `/innovate` create design-only threads and return the outcome note. A ready note begins with the selected brief and includes success-test text and, when available, a proof identifier. That surface does not independently validate the citation. [Chat construction][chat] [Design command][chat-command] [Outcome note][note]

The recurse constructor wires the same service. Mission preparation maps proceed to its ready-design disposition and retains the selected candidate. Its subsequent source and vision comparisons do not establish whether a citation ever identified observed material: an unchanged workspace can contain an invented locator just as easily as a changed one. Those comparisons and later execution gates remain separate protections. No claim is made that this evidence defect necessarily causes a mutation, survives the ratchet or yields an incorrect mission-success receipt. [Mission constructor][mission-constructor] [Mission preparation][mission]

## 3. Minimal unrun witness and controls

Use a fresh design-only thread, ordinary configuration, successful persistence and recorded proof support. Keep all data below the context bound. Use a single round with no consultation, conductor probe, source change, human pause or prior objection. Script the model boundary to isolate admission, rather than depend on a live model making this mistake.

The request asks for a design that preserves an existing public API while adding bounded caching. Candidate `c1` proposes a bounded internal cache; `c2` proposes an explicit caller-provided cache. Both have distinct IDs, nonblank briefs, targets and success tests. The challenger marks `c1` successful with no items and gives `c2` a critical design item about its unresolved compatibility contract. It recommends `c1`.

Vary only `c1`'s evidence:

| Variant | Evidence list | Static prediction for round 1 |
|---|---|---|
| A | `[]` | No citation fact for `c1`; it is not ordinary-ready. With the stated other assessment, the round needs work. |
| B | `[""]` | One citation fact exists; `c1` meets the ordinary ready premises. |
| C | `["   "]` | Same presence result as B. |
| D | `["missing/never-created.go:999"]` | Same presence result; the conductor contains no locator resolution step. |
| E | A real, applicable, admitted source reference | A strengthened contract should preserve this successful design path. |

These are source-derived conditional predictions, not runtime results. The test harness must verify the real kernel and production role path rather than assuming all infrastructures succeed. The claims about B-D are limited to admission behavior; the fixture's scripted challenger is itself asserting success. No source inspection can establish how often production models produce these replies.

Important controls:

- A critical item on `c1` must still prevent ordinary readiness even with valid evidence.
- A false success flag must not become true merely because a citation resolves.
- Genuine request-only conceptual work must not be forced to cite a nonexistent implementation file.
- A citation to a real but irrelevant file should pass source-identity validation only, not be labeled semantically supporting by the host.
- An actual read can exist in another trace or in the role's observations. The finding is not that no tool ran anywhere; it is that the ready premise is not joined to such a record.
- A good challenger can catch the problem and issue a blocker. That is valuable counterevidence to universal failure, but it does not change the host's admission contract.

### Existing authored coverage

The validation fixture intentionally supplies candidates without evidence and treats the round as structurally valid. Its malformed-round cases cover identity, success-test text, assessment coverage and objection shape, not blank or unresolvable citation admission. This supports preserving a distinction between structurally valid exploration and ready evidence. [Validation tests][validation-tests]

The chat fixture creates an actual `request.go` and uses `request.go:1` for its first candidate; the second has no citation. Its empirical variant checks a returned file-read result. The production-boot policy fixture supplies the phrase `the original request` as its citation. These are useful controls and show why a proposed fix must allow request evidence rather than require every reference to be `path:line`. In the files inspected, no blank/nonexistent-reference readiness witness is present. This is not a claim about every test in repository history; none was run here. [Chat fixture][chat-fixture] [Empirical test][empirical-test] [Boot fixture][boot-test]

## 4. Nonduplication

Earlier values-option and selected-review audits identify ordinary-ready citation presence as an existence-only check. This report isolates the empty-list versus blank-entry differential, traces its production admission path, and specifies bounded repairs. The pinned audit inventory, current queue, all named deliberation reports and targeted repository searches for citation facts and evidence were screened. No exact predecessor for this first-round citation-admission witness was identified in that bounded review. The broader design has clear prior art in this repository, and no exhaustive novelty claim is intended. [Audit queue][queue]

| Existing work | Relationship and separate boundary |
|---|---|
| Portable recurse deliberation and crucible | Already requires real source material and a reviewed evidence contract. This report supplies the exact current adapter/rule mismatch and empty-versus-empty-entry witness. It is a narrow refinement, not another full crucible proposal. |
| Deliberation read invocation identity | Preserves arguments with observations inside a role. Correctly attributed tool results do not by themselves prevent a different, model-invented citation from satisfying readiness. A receipt mechanism can be shared. |
| Deliberation critical-item retirement | Requires explicit accounting before earlier blockers disappear. The present witness is round one and has no earlier blocker to retire. |
| Deliberation values evidence freshness | Revalidates real historical evidence after a pause. Here no pause or source drift is needed, and the cited material need never have existed. |
| Deliberation selected-review handoff | Preserves important findings after selection. This issue occurs before readiness and needs no warning to be dropped. |
| Verification evidence and judge contracts | Addresses downstream implementation judging and typed execution evidence. This concerns upstream read-only design selection, not execution completion. |
| Research observation identity and admission | Addresses acquisition/result semantics in external research. An accurately acquired observation can coexist with this missing candidate-to-source admission join. |

Sources: [Crucible][portable], [read identity][read-audit], [objection retirement][retirement-audit], [values freshness][freshness-audit], [review handoff][handoff-audit], [verification judging][verification-audit], [research observations][research-audit].

## 5. Proposed bounded contract

Everything below is prospective. Start with a specification decision about what a ready design's grounding claim means. Do not quietly relabel existing model assertions as verified facts.

### 5.1 Separate three distinct checks

1. **Syntax:** the reference is nonblank, bounded and well-formed for its declared kind.
2. **Identity and availability:** a host-issued receipt identifies material admitted to this request, including scope/version and any partial or unavailable status.
3. **Support:** the material actually supports a particular assumption, claim or requirement.

The host can enforce the first two under explicit source adapters. The challenger, approved external criteria and later behavioral verification contribute to the third. Resolving a file, matching a digest or counting citations cannot establish entailment or design correctness. Record these states separately so an identity-only check does not become a stronger claim.

### 5.2 Minimum repair and stronger follow-on

The minimum correction is to refuse blank/whitespace evidence entries as readiness support while retaining evidence-free exploratory candidates. That closes B/C only. It must not be reported as closing invented references or unsupported claims.

The stronger design uses typed evidence references to a bounded, host-owned observation index. A record should identify source kind, request/thread owner, producer observation, source identity/version, accessible span or selector, content digest where applicable, and coverage/availability state. Candidate references name those records, with a concise stated purpose. The host supplies admission facts; Mangle derives eligibility from the required admitted-reference relation. Model prose does not manufacture that relation.

Useful source kinds include original request/brief, workspace file or CodeDOM element, conductor probe, role read, and external/consultation material. Their contracts differ. A request establishes what the user asked, not whether a workspace implementation exists. A failed or truncated search establishes only its recorded outcome/coverage, not universal absence. External evidence cannot inherit a local Git digest as its identity. These distinctions build on existing evidence audits rather than replace them. [Read identity][read-audit] [Research observations][research-audit]

Do not automatically open arbitrary paths or URLs copied out of a citation string. Resolution uses existing guarded reads and source containment. Unknown reference kinds, permission-denied sources and missing receipts yield explicit unsupported/unavailable states. No new read, network, shell, edit or consent authority follows from evidence admission.

### 5.3 Keep the protocol portable

Do not use a single `path:line` regular expression as the universal grammar. Windows drive syntax, colon-containing filenames, Unicode, generated files, CodeDOM identifiers, non-Git workspaces, user-supplied requirements and remote documentation need source-specific representations. A host-issued reference can keep literal display text separate from its identity.

A fresh repository with no implementation can still receive a useful conceptual design grounded in the original request and explicitly stated assumptions. If that design depends on an unverified assertion about existing code, label that assertion unknown and keep the associated data obligation. Unsupported language/build adapters must not prevent safe design work, and support for references must not imply support for execution or verification in that ecosystem.

### 5.4 Lifecycle, bounds and failure ownership

- Preserve the original evidence claim and its admission result in the saved round. Do not rewrite a bad reference to whichever file looks plausible.
- Keep candidate-to-reference association through proposer, challenger, policy, saved thread and selected handoff.
- Share existing observation receipts where possible. The current role function returns text, not a typed observation bundle; adding a host-owned receipt channel is real implementation work, not merely adding a validator. [Agent return][agent] [Collector][round]
- Bound reference count, encoded bytes, source expansion and resolution work. Charge extra reads to the existing budget and preserve protocol/control room. If metadata cannot fit, return explicit incomplete grounding or a recoverable reference; do not silently count a retained prefix as complete support.
- Persist enough source metadata for declared recovery; a digest alone cannot reopen omitted content. Expired or missing references remain unknown.
- Preserve historical threads without inventing admission receipts. A migration may retain their design text as legacy/unverified evidence; it must not silently upgrade their grounding status.
- Stop and failure handling must preserve the pending evidence obligation. A formatting repair, if adopted, is separately bounded and policy-owned; it does not authorize another execution attempt or a new user interview.

## 6. Failure modes to test

| Failure mode | Required behavior under the proposed contract |
|---|---|
| Empty, whitespace, null-decoded or malformed reference | Not admitted as evidence support. Preserve an explicit diagnosis. |
| Nonexistent or out-of-range source locator | Unknown/refused; do not fabricate material or broaden filesystem access. |
| Real but unrelated source | Identity may resolve; semantic support remains a separate assessment. |
| Source read failed or was cut | Retain the actual error/coverage qualifier with the reference. |
| Reference belongs to another request/candidate | Reject or explicitly re-admit under the current owner; never treat ID equality as authority. |
| Source changes between acquisition and use | Apply its validity contract or mark stale; an old receipt is not current proof. |
| No existing code or unsupported ecosystem | Permit explicitly conceptual, request-grounded design without false workspace claims. |
| Resolver cancellation or budget exhaustion | Return a truthful incomplete outcome, not an empty successful evidence set. |
| Excessive references or repeated locators | Bound and deduplicate resolution while preserving which claims remain unsupported. |
| Legacy save, missing receipt or expired remote handle | Preserve useful history but do not reconstruct invented provenance. |

## 7. Full cost accounting

The existing design already incurs proposer and challenger calls per collected round, any role reads, conductor probes, specialist consultations and their nested calls, JIT compilation, kernel assertion/query/proof work, and complete-thread persistence. A successful one-round witness does not establish the average production cost. Failed, stopped and repeated attempts belong in the denominator. [Collector][round] [Agent][agent] [Probe execution][probes] [Service][service] [Persistence][persistence]

The minimum blank check adds work linear in evidence text length, plus an explicit failure/refinement path. Its direct CPU cost is likely small, but the total cost can rise if malformed outputs require another round. That direction is an expectation, not a measurement.

Host-resolved references add parsing, lookup, source reads where not already available, hashing, validation and serialization. Reusing receipts can avoid redundant reads, but requires durable ownership, invalidation and retrieval work. Extra metadata consumes context and output budget and can cause earlier evidence closure. Storing whole source bodies repeatedly raises disk and replay cost; references trade those bytes for lookup latency and expiry failure modes. Additional Mangle facts and joins have kernel cost. Compatibility migration and source-specific resolvers add maintenance and testing cost.

Measure all provider input, output, reasoning and cache-accounted tokens available from the provider; all retries/consultations; read count and bytes; source-resolution latency; peak host memory; receipt and snapshot storage; kernel/proof time; end-to-end latency; and human review/recovery work. Report unresolved references and false rejections alongside prevented unsupported readiness. No token savings, hallucination reduction, quality uplift or net monetary benefit has been measured here.

## 8. Fair evaluation plan, entirely unrun

### A. Deterministic contract checks

Use the real `Service`, normal role adapter, registered tools and production kernel boot with a scripted model boundary. Establish the A-D witness and E control, exact facts/dispositions, persisted evidence, emitted note and absence of a design-only development dispatch. Run schema/conversion checks for empty and whitespace strings independently. A monolithic policy fixture alone is not enough to demonstrate production predicate routing.

For a minimum blank-only repair, expect B/C to stop supplying evidence while D remains explicitly outside the fixed scope. For typed admission, require D to be unresolved, a valid reference to preserve success, and a request-only design to remain possible. Test malicious-looking strings as inert data and confirm the resolver cannot expand authority.

### B. Metamorphic and portability checks

Hold candidate meaning and challenger assessment constant while changing only reference identity: valid to missing, valid to wrong owner, known source to unobserved source, complete to truncated, current to stale. These should change the grounding disposition where appropriate without changing permissions. Reordering reference lists or map fields must not create new evidence. Repeat references must not multiply support.

Include Go, Python, JS/TS, Rust and an unknown-language/non-Git workspace; Windows and POSIX path cases; Unicode and unusual filenames; request-only designs; CodeDOM elements; and synthetic external observations with distinct identities. This validates the common design-evidence protocol, not language adapters, toolchain completeness or universal codebase support.

### C. Controlled quality/cost comparison

Compare (1) current code, (2) blank-only admission and (3) bounded typed references under the same pinned models/providers, JIT corpus, tasks, permissions and total budgets. Predefine tasks, source truth and required support before sampling. Keep all failures, stops and refusals. Stratify by task/source kind and include genuinely unsupported requests rather than filtering them out.

Have reviewers blinded to treatment score unsupported ready decisions, citation-source accuracy, support relevance/completeness, useful completed designs and unnecessary refusals. Report paired outcomes with uncertainty and the full costs in Section 7. A stricter system can appear more reliable merely by refusing everything; valid-design completion and recovery cost must be reported with false readiness. A grounded reference can still support a wrong design, so independent design assessment remains necessary.

Live model sampling and any execution crucible require separate authorization. None is part of this source-only audit.

## 9. Primary literature and limits

- W3C [PROV-DM](https://www.w3.org/TR/prov-dm/) separates entities, activities and agents and provides derivation/usage relations. The relevant design inference is that an evidence label and the material/observation it identifies are different records. This does not require adopting RDF or the complete PROV model.
- Gao et al., [Enabling Large Language Models to Generate Text with Citations](https://aclanthology.org/2023.emnlp-main.398/), EMNLP 2023, evaluates answer correctness and citation quality separately. It supports evaluating reference/support quality rather than counting citation-shaped strings. Its results concern its own QA benchmarks, not CodeNERD, and give no measured failure rate or expected uplift for this system.
- NASA's [requirements checklist](https://www.nasa.gov/reference/appendix-c-how-to-write-a-good-requirement/) and [requirements verification matrix](https://www.nasa.gov/reference/appendix-d-requirements-verification-matrix/) motivate explicit, traceable requirements and named verification approaches. A source reference is useful traceability, not a substitute for verification. This proposal does not claim NASA process compliance.

## 10. Decision and exit criteria

Treat this as a narrow strengthening of the existing portable-crucible proposal. First specify and close the blank-citation presence defect. Then decide whether ready means a reviewed conceptual proposal, a source-grounded design, or a named combination, and implement typed admission to match that promise. Keep those scopes visible in status and evaluation rather than bundling them into a single “grounded” Boolean.

The minimum exit is that blank/whitespace strings never supply the citation premise while valid existing designs and exploratory candidates still work. The stronger exit is that every reference used as evidence support has an admitted, recoverable source identity with explicit limits, and missing/unsupported material cannot be promoted merely by writing its name. Neither exit establishes semantic design correctness, empirical requirement satisfaction or improved model quality without the separate evaluations above.

## Pinned public references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/ccbe41302073b364ef2cb40ba49b9736738ae6e2
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/agents.md#L5-L110
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/architecture/AGENTS.md#L1-L113
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/prompt/atoms/system/deliberation_proposer.yaml#L13-L52
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/prompt/atoms/system/requirements_interrogator.yaml#L13-L51
[session-spec]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L898
[candidate-shape]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/types.go#L17-L38
[types]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/types.go#L144-L181
[round]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/round.go#L10-L72
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/policy.go#L10-L39
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/schemas_deliberation.mg#L29-L35
[string-conversion]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/types/types.go#L218-L253
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/core/defaults/policy/deliberation.mg#L18-L58
[service]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/service.go#L100-L165
[proof]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/policy.go#L42-L72
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/outcome.go#L16-L74
[note]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/outcome.go#L89-L115
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/persistence.go#L24-L79
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/cmd/nerd/chat/deliberation.go#L18-L50
[chat-command]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/cmd/nerd/chat/deliberation.go#L96-L119
[mission-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/campaign/recurse_mission.go#L244-L323
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/read_tools.go#L85-L210
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/outcome.go#L138-L181
[validation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/deliberation/deliberation_test.go#L36-L76
[chat-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/cmd/nerd/chat/deliberation_test.go#L42-L99
[empirical-test]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/cmd/nerd/chat/deliberation_test.go#L230-L243
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/internal/system/deliberation_boot_test.go#L11-L49
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/ASTRA_QUEUE.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/portable-recurse-deliberation-crucible.md
[values-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/deliberation-values-option-admission.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/deliberation-read-invocation-identity.md
[retirement-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/deliberation-critical-item-retirement.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/deliberation-values-evidence-freshness.md
[handoff-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/deliberation-selected-review-handoff.md
[verification-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/verification-evidence-and-judge-contracts.md
[research-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ccbe41302073b364ef2cb40ba49b9736738ae6e2/Docs/audits/research-observation-identity-and-admission.md
