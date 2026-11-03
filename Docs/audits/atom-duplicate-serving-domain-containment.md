---
doc-class: deep-dive
subsystem: prompt-admission
implementation-status: proposed-refinement
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
scope: provider and model containment in atom duplicate admission
---

# Atom duplicate admission must preserve the candidate serving domain

## Decision and scope

An existing atom should justify refusing a candidate as a duplicate only when it covers the candidate's instructions **and** its supported serving domain contains the candidate's. Current duplicate admission omits provider and model selectors from its deterministic containment check, although the serving path enforces both. An atom valid only for provider P1 can therefore suppress an otherwise eligible candidate for P2 without being available to replace it on P2.

This is a learning admission discrepancy, not a cross-provider prompt leak. The existing pin enforcement works and makes the lost coverage observable: the retained P1 atom does not serve P2. In the evolution path, a successfully completed generation also consumes its failure cohort even when duplicate refusal leaves no candidate. The recommendation is to repair the containment proof, not to weaken pins or make every refusal retry indefinitely.

The review is pinned to main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), rechecked unchanged at approximately 10:12 UTC on October 30, 2026. An independent source challenge corroborated the provider/model omission and consumer path. No repository program, test, model, or benchmark was executed, and no repository file was changed. This document proposes a refinement; its witness is conditional on valid generation and judgment results, not a claim that a live model produced them.

## The existing contract already separates content from scope

[`Admission.decide`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/admission.go#L129-L147) refuses a candidate as `/duplicate` when a compared atom receives a coverage probability at or above the configured yes threshold and `Reaches(existing, candidate)` returns true. The function is explicitly intended to establish that the existing atom is selected wherever the candidate would be. This is stronger than overlap, shared role, or textual similarity.

The split is deliberate. [`coversQuestion` and `textOf`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/atomicity.go#L92-L150) give Jev the instruction text and leave selector reachability to deterministic code. Scope is not hidden in a model-readable side channel: the compared state contains text alone. A correct positive coverage judgment can therefore coexist with disjoint serving scopes. Asking Jev to infer which provider should receive the instruction would undo this separation rather than repair it.

[`Reaches`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/admission.go#L156-L193) compares role, language, framework, intent, mode, campaign and build phases, initialization and north-star phases, Ouroboros stage, and world state. It does not compare `Providers` or `Models`. When every listed dimension passes, differing nonempty provider/model lists cannot prevent its true result. [`Nearest`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/admission.go#L195-L227) selects a bounded set by lexical similarity and skips the candidate's own ID, but does not remove atoms with incompatible serving pins.

By contrast, [`PromptAtom.MatchesContext`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L511-L537) enforces the provider selector and then requires a matching exact or family model token. Missing identity fails a constrained dimension. [`EvolvedAtomManager.GetMatching`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L278-L305) performs that context check before admitting a promoted atom or considering a quarantined candidate's exact-version trial authorization. Thus the proposed replacement atom can be correctly excluded at the consumer while still being accepted as sufficient replacement evidence at duplicate admission.

## The production evolution path reaches this check with valid pins

This does not require malformed authored YAML or manually attaching an arbitrary provider selector to a fabricated candidate.

1. [Factory initialization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L112-L168) creates the workspace evolver, configures Jev when available, installs atom admission over shipped and evolved atoms, and registers the evolved-atom manager with the compiler. Automatic cycles remain [opt-in](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L374-L395); registration is not evidence that automatic evolution is enabled.
2. [`generateForGroup`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L175-L235) validates pin provenance, selects a qualifying bounded cohort, obtains screening permission, and acquires a generation claim before purchasing generation. The finding assumes those gates legitimately pass and budget remains.
3. [`parseGeneratedAtoms` and `convertToPromptAtom`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/atom_generator.go#L168-L265) retain source provider/model provenance and stamp the pin before returning a generated candidate. [`requirePin` and `applyPin`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/pin_policy.go#L36-L63) require the role and provider, require the model for narrower scopes, overwrite the controlled selectors, and normalize them. The generating model cannot silently broaden those selectors.
4. [`admitGenerated`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/admission.go#L51-L92) gathers all in-memory evolved atoms without filtering by the candidate's provider or model, adds the shipped corpus, and submits each candidate to the shared check. A duplicate refusal increments the refused count and omits the candidate from the returned set.

An older P1 atom can be present legitimately after a provider change. [Loading](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L219-L275) reads promoted and pending records. [Stored-pin enforcement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/pin_migration.go#L33-L74) derives each atom's selectors from that record's own saved provider, model, and source role. It does not repin every atom to the currently configured provider. That is correct migration behavior; deleting or broadening the P1 atom is not the remedy.

## A valid conditional witness

Use two distinct, canonical serving providers P1 and P2, and an otherwise compatible role such as `/coder`. Keep role, language, and every other selector dimension involved in `Reaches` equal or appropriately broader in the existing atom. This isolates the pin omission from the separate session verb/persona attribution issue.

Let A be an already promoted, properly persisted P1 atom with valid provenance. Let B be the sole candidate generated from a qualifying P2 cohort, with a different ID and a valid P2 pin stamped by the normal producer. Both contain the same or semantically equivalent project-specific instruction, within the configured size bound. A is among the bounded lexical neighbors submitted to Jev; no other globally applicable atom independently covers the instruction. A legitimate atomicity result reports that A covers B at or above the configured threshold, while known-by-default, concern-count, and padding readings remain below their refusal thresholds.

The source then predicts:

| Stage | Result under these conditions |
| --- | --- |
| Content reading | A covers B's instruction; scope is intentionally not part of that reading |
| Deterministic reachability | `Reaches(A, B)` returns true because provider/model selectors are absent from its comparison |
| Candidate admission | B is refused as `/duplicate` and is not retained |
| P2 serving context | A fails the provider match; B would satisfy that pin if retained, but is absent |
| Successful claim completion | The cohort is consumed even though the retained candidate list is empty |

This witness does not need two processes, lease expiry, a race, unknown provenance, alias collision, or failed persistence. A same-provider witness also exists when A and B have incompatible model scopes. That variant must choose contexts genuinely outside A's supported model domain; different spellings alone are insufficient because [model canonicalization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/pinning.go#L86-L139) intentionally recognizes some aliases, and [`ModelPinTokens`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/pinning.go#L268-L280) emits exact and family tokens. The cross-provider construction is sufficient without relying on family-boundary subtleties.

Promoted A avoids a separate question about whether an unapproved trial candidate is a legitimate replacement. Admitting B would still not guarantee its promotion, trial authorization, selection within prompt budget, final prompt inclusion, or an improved task result. The proven discrepancy is the incorrect duplicate refusal and resulting loss of the candidate, not a guaranteed missing sentence in every future prompt.

## Why consumption matters without making consumption itself the defect

After admission, [`generateForGroup`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L238-L265) applies atom caps and invokes publication even when no candidate remains. [`publishGeneration`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L273-L319) first completes the owned claim and records evidence consumption, then stores any remaining candidates. With an empty list there is no candidate write to fail and trigger the publication-failure recovery path.

[`EvidenceLedger.Complete`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L378-L403) preserves the owner fence and consumes task/verdict references transactionally. The [next ordinary failure query](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/feedback_collector.go#L527-L571) excludes those exact task/verdict versions. Consequently, a false refusal is not automatically corrected by the next serial cycle. New failures, revised verdict versions, or an explicitly designed recovery operation are different cases.

Consuming evidence after a completed but unproductive purchase prevents paying for the same nothing on every cycle; the source states that intention. Preserve it. Do not solve this finding by unconditionally releasing every refused cohort, disabling duplicate checks, resetting all consumed evidence, or letting a new screening identity manufacture fresh purchase authority. The [0948 consumed-evidence admission audit](https://github.com/theRebelliousNerd/codenerd/blob/4d0c0c64ddc5da4b87587c1bec4368ca7c70edd5/Docs/audits/prompt-evolution-consumed-evidence-admission.md) concerns stale cross-process snapshots and transactional purchase ownership. This audit concerns a deterministic false duplicate within an otherwise properly owned purchase; its consumption consequence does not duplicate that concurrency finding.

## Existing protections and prior work to preserve

The size bound executes before any atomicity request. [`Admission.Check`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/admission.go#L104-L126) admits within-bound candidates as `/unjudged` when Jev is absent or a judgment fails. The evolution wrapper also retains candidates on a check error and logs the condition. Thus this finding does not apply simply because Jev is disabled, unavailable, malformed, or unable to fit the question. It requires a usable positive coverage reading. Preserve the existing distinction between no reading and a negative admission decision; do not turn uncertainty about containment into a new refusal.

The prior [exact-model pin identity audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/exact-model-pin-tag-identity.md) concerns collapsing distinct serving-model tags in normalization and grouping. Current [`NormalizeModelToken`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/pinning.go#L86-L139) preserves nondefault colon tags, including numeric tags, while retaining documented routing and Bedrock equivalences. Credit those repairs. This residual persists even when P1/P2 and both model tokens are perfectly distinct, because duplicate admission does not consult either dimension at all.

Likewise, keep strict missing-pin handling, source-role pinning, stored-provenance migration, trial approval and version checks, generation/screening budgets, cohort bounds, cancellation, claim ownership, and final Mangle admission. The retained atom must not be broadened merely to make a duplicate explanation true. Text equivalence is not evidence that a rule learned under one model should apply to all others.

## A serving domain containment contract

Define the deterministic claim before changing code:

> For the declared selector semantics, every supported non-nil compilation context eligible for B must also be eligible for A before A can justify refusing B as a duplicate.

The immediate repair must include provider and model scope. Empty selectors retain their existing unconstrained meaning. A constrained A cannot cover an unconstrained B merely because they overlap on one context. For provider lists, canonical set containment is the relevant direction: A's allowed values must contain B's. Disjoint sets must fail; equality and genuine broad-to-narrow coverage may pass. Use the same normalization contract as pin production and serving, rather than introducing a second alias policy.

Model containment requires particular care. The consumer accepts any token among a context's exact and family tokens. Comparing raw model strings or testing only B's generating context is not a proof that all of B's contexts are covered. A simple normalized-list containment check can be a conservative first repair, but it may miss safe family-to-exact coverage and admit extra redundant atoms. If richer containment is added, derive it from the existing exact/family match semantics, preserve its direction, and prove it with cases in both directions. Do not broadly strip release markers from exact pins to manufacture equivalence. When containment cannot be proven, decline the duplicate reason; other existing admission reasons may still apply.

Keep the deterministic relation authoritative even if candidates are prefiltered before lexical ranking. Prefiltering obviously incompatible providers/models can avoid wasting bounded comparison slots and question text, but it is an optimization, not the correctness boundary. It changes which neighbors Jev sees, so retain the comparison count and whole-question token bound, and evaluate any resulting judgment changes separately. Do not increase the number of model calls to search indefinitely for a covering atom.

The contract should distinguish selector eligibility from final resource- and policy-dependent selection. This witness proves a missing provider/model check using promoted A; it does not establish that repairing those two fields proves full substitutability across dependencies, tool availability, category competition, trial lifecycle, or all future selectors. Name and document the relation's scope, inventory the actual serving dimensions, and require new selector dimensions to extend its conformance cases. Preserve Mangle's final policy role rather than making this helper a new authorization engine.

## Rollout and historical recovery

A containment correction need not change atom pins, content, IDs, or trial versions. Avoid unnecessary rewrites of correctly pinned persisted atoms. Update the package contract and production-path tests before enabling the corrected decision, and make decision diagnostics identify the compared atom and containment disposition without requiring private prompt text in logs.

Past refused candidates may no longer be available as complete replayable objects. Existing refusal logs identify the duplicate reason, while completed claims retain accepted atom IDs and consumed evidence; that does not establish a durable archive of every rejected candidate. Therefore do not promise automatic reconstruction or declare every zero-atom purchase erroneous. A zero result can be intentional for many independent reasons.

If historical repair is desired, make it a separate, bounded, operator-visible operation: identify affected purchases where evidence permits, retain original claim and spend records, record the correction policy version and replay reason, preserve exact provider/model/role ownership, and charge any new generation against explicit budgets. An incremented generator-version string alone is not a demonstrated recovery path: the current consumed-evidence primary key is [task plus verdict version](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L89-L94), and the ordinary read excludes that pair before a new claim is constructed. Design any replay eligibility explicitly rather than relying on comments about versioned reanalysis or deleting the ledger wholesale.

Mixed-version workers can continue making the old false decision against the same workspace. Coordinate deployment or expose an admission-policy version in diagnostics if rollout overlaps. This is not a reason to weaken the existing publication fence or introduce cross-process cache changes into this patch.

## Proposed evaluation without live inference

The following checks are proposed, not run. Use controlled atomicity answers and the real pin, admission, persistence, ledger, and serving helpers. Existing [truth-table and narrower-language tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomjudge/admission_test.go#L42-L94) already demonstrate the intended split between content coverage and scope; extend that style rather than relying on stochastic model agreement.

| Case | Required result |
| --- | --- |
| Same content and compatible role, disjoint providers | No duplicate refusal based on A; A still fails B's serving context |
| Same provider, genuinely incompatible exact models | No duplicate refusal based on A; no normalization or grouping regression |
| Same provider/model and all other selectors contained | Existing positive coverage still produces a duplicate refusal |
| Broad existing provider scope and narrow candidate | Valid containment remains possible; reversing the direction fails |
| Existing constrained provider/model and unconstrained candidate | Overlap alone is insufficient; the candidate's extra domain is retained |
| Exact and family model cases in both directions | Results agree with the documented serving-domain relation, not string similarity |
| Known aliases, nondefault numeric tags, unknown identity | Existing normalization distinctions and fail-closed serving remain intact |
| Jev absent, failed, incomplete, or question too large | Existing within-bound unjudged behavior remains; oversize still refuses before asking |
| Candidate ID equals existing ID or neighbor limit excludes A | Existing self-exclusion and bounded-neighbor behavior remain; no invented duplicate coverage |
| Real generated B from a qualified P2 cohort, promoted P1 A loaded from disk | B receives its normal pin and reaches admission; corrected duplicate logic keeps it if no other refusal applies |
| Corrected B retained versus legitimate same-scope duplicate | Both preserve owned completion and one purchase per consumed cohort; only the retained case stores B |
| Promoted and approved-trial consumers | No cross-provider leakage, trial bypass, permission expansion, or forced prompt inclusion |

For an end-to-end source-contract fixture, control the generator response and atomicity answers, then use the normal factory/evolution route and compiler registration rather than only calling `Reaches` directly. Keep the selected cohort genuinely eligible and the role selector aligned with the actual consumer so upstream attribution failures do not mask the result. Include the existing published-atom owner fence, budget exhaustion, cancellation, and failed-publication behavior as nonregression checks. A later authorized runtime evaluation should use the repository's required production build profile and real-kernel admission; this review performed neither.

Count incorrect duplicate refusals and duplicate admissions against an explicit scope oracle. Report them separately from candidate retention, promotion, actual exposure, and task success. A larger stored candidate count alone is not evidence of better learning.

## Costs and acceptance criteria

Adding two bounded selector comparisons is small local work, but a general containment abstraction adds maintenance and semantic-test obligations. Canonical sets may add allocations; prefiltering can add a pass over stored atoms but reduce question text. Measure both rather than claiming a latency or token saving.

Correct scope separation may retain more text-equivalent atoms across providers/models. That can increase disk use, candidate indexing, review and trial demand, and per-context selection work. Conservative model containment may retain some unnecessary copies. These are real costs of preserving the configured learning boundary, not evidence that pins should be widened. Keep per-cycle candidate and generation limits unchanged; additional successful retention may consume atom budget sooner and defer later groups.

Evaluate quality only on matched tasks, providers, models, tools, and budgets, with comparable trial exposure. Compare total completion quality, errors, latency, model spend, review burden, and prompt bytes; do not attribute improvements to containment merely because more atoms survive. The acceptance criterion for the immediate change is narrower: an atom unavailable throughout part of B's supported provider/model domain must not be accepted as sufficient coverage to discard B, while valid same-domain deduplication, unjudged admission, pins, lifecycle controls, and evidence accounting remain intact.
