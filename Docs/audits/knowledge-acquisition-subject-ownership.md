---
doc-class: deep-dive
subsystem: knowledge-acquisition
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Knowledge refresh must preserve distinct subject ownership

**Decision:** retain the exact acquisition owner through production, persistence and replacement. A human-readable subject slug is not an ownership identity. Refreshing one subject must not delete another subject's rows merely because both producers generated the same concept label.

**Finding:** the current web and Context7 producers erase punctuation when naming concepts, while the acquisition ledger distinguishes the original subjects. In one agent KB, `C++` and `C#` therefore have different acquisition records but can both own `web/c/section_0`. Successful refresh deletes by that concept alone. Refreshing the second subject can remove the first subject's newly acquired section, leaving both ledger entries fresh and acquired. This sequence needs no failed source, concurrent writer, pruning, malicious output or runtime crash.

**Evidence status:** static inspection of public main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked as main on 2026-10-30 at approximately 08:12 UTC. This is a conditional source-derived defect, not an observed production incident. No application execution, tests, database experiment, model inference, provider request or benchmark was performed. This Markdown proposes a repair and evaluation; it implements neither.

## Contract and prior ownership

The existing acquisition contract is already source-and-subject scoped: a source that reacquires a subject replaces the rows its previous acquisition wrote; a failed source leaves its prior rows. The ledger has primary key `(source, subject)`, and `Outcome.Concepts` is documented as the rows that acquisition wrote. The defect is narrower than a request for a new feature: deletion discards the owner distinction the acquisition API and ledger already preserve. [Acquisition contract and schema](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L91-L102), [refresh semantics](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L193-L225)

Repository guidance assigns relevance, retention and executive policy to Mangle, requires stale evidence not to remain current truth, and asks for validation through normal production paths. Store guidance requires durable writes to remain recallable and provenance to stay explicit. A repair should preserve those contracts rather than replace refresh policy with a model's discretion or make all knowledge mandatory in prompts. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md), [store guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/agents.md)

The nearest prior audit is [knowledge refresh and prune](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-refresh-prune.md). It owns an old pruning authorization deleting a newly installed generation under the same concept. Its generation fence is now present in main and must be credited. This report instead concerns **two distinct subjects sharing one concept label during acquisition replacement**, before the outer prune phase. Neither removing pruning nor retaining its generation fence repairs the witness.

The [knowledge miss audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-miss-retirement.md) concerns research-gap admission and retirement. The [literal consultation identity audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-consultation-literal-identity.md) concerns ask identity at consultation admission. The [expert-deck proposal](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/innovations/plan-bound-expert-decks.md) concerns plan-bound steering and mandates. None is a replacement-owner contract for colliding persisted subjects.

Deduplication included the relevant main audits and innovations, targeted searches for knowledge slug/collision/replacement, and the added-file inventories of public branches `research-main-20261030-0715` at `493e65a2892d7c5262de95999b8c2e256f6a3c4f` and `research-main-20261030-0747` at `136e02f0a38897a40931747b1e56a6f0c14feb19`. No exact owner was found in that bounded review; this is not an exhaustive novelty claim.

## Production wiring preserves subjects until concept construction

Init obtains subjects through `KnowledgeSubjects` and calls `knowledge.Fill` with the real configured acquirer. Topic admission trims whitespace and deduplicates case-insensitively, but does not erase punctuation: `C++` and `C#` remain two subjects. `Subject.Key()` returns the module when present and otherwise the topic. The witness uses ordinary topics, not module aliases or a hypothetical custom producer. [Topic admission](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources.go#L15-L35), [init caller](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/agents_knowledge.go#L130-L149), [subject key](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L26-L58)

The factory registers knowledge refresh as a boot step. Boot constructs the acquirer from configured grounded-search clients, Context7, pinned documentation and world-model importers. The refresher reads ready agents and their actual KB paths/topics, reconstructs wanted subjects with the same admission function, measures each KB, and passes the budget-selected subject list to `knowledge.Refresh`. Explicit `Cortex.RefreshKnowledge` uses that same run method. [Factory registration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2919-L2922), [boot wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh_boot.go#L11-L58), [production refresh](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L116-L169), [entry and roster](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L200-L227)

The loss of identity occurs in the actual producers:

- `slug` lowercases the input, replaces every run outside ASCII letters/digits with a hyphen, and trims surrounding hyphens. Both example subjects become `c`.
- Web names each qualifying paragraph `web/<slug(Subject.Key())>/section_<index>`. It requires cited sources, and its content includes the original question, citations and retrieval date.
- Context7 analogously names each paragraph `context7/<slug(topic-or-module)>/section_<index>`.

A source can return different, legitimate text for distinct subjects without affecting this deterministic label collision. [Slug and grounded web producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L20-L88), [Context7 producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L113-L145)

The production web wrapper matters: `WebChain.Name()` is `web`, while each atom retains concrete provider provenance such as `web/openai`. The acquisition outcome therefore matches the policy's `web` age rule. It would be incorrect to dismiss this path by comparing the provider-specific atom source with that policy constant. `NewAcquirer` actually installs the chain, not just an isolated `WebSource`. [WebChain and acquirer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L20-L102)

## A clean serial witness

Assume one ready agent has both topics, with one shared KB, and that each initial and refreshed web answer has one qualifying paragraph and citations. Storage succeeds. Both subjects remain wanted, and both are selected within the refresh budget. Other sources may be unavailable or inapplicable; the witness only claims loss of the colliding web rows.

1. **Initial acquisition preserves both bodies.** There is no prior successful acquisition to replace. `Fill` deduplicates by concept **and content**, not concept alone; the distinct question footer makes the two web contents different even if their main prose happens to match. Both rows can therefore coexist with concept `web/c/section_0`. The store uses a plain insert, and the deletion API explicitly documents that the knowledge table has no uniqueness constraint. [Deduplication and insertion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L235-L259), [row insert](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L43-L71), [nonunique concept contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge_delete.go#L10-L20)

2. **The ledger preserves two owners, but records the same concept.** Acquisition constructs an outcome from the source's logical name and the exact subject key, then collects each returned concept. Thus `(web,C++)` and `(web,C#)` each list `web/c/section_0`. [Outcome construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L160-L204)

3. **Ordinary staleness admits both subjects.** At 31 whole days after their successful web acquisitions, age exceeds the stock 30-day maximum. A four-subject budget can select both if more urgent candidates do not displace them. The held rows can still be only 31 days old, below the separate 60-day pruning threshold. No pruning precondition is needed. [Age and budget constants](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L8-L33), [stale derivation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L78-L82), [wanted and budget selection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L329-L357)

4. **The first refresh deletes both old bodies.** `fill` looks up the first subject's previous concept list using its precise owner key, then passes those bare concepts to `DeleteKnowledgeAtomsByConcept`. That function deletes *every* row whose concept matches, plus matching knowledge vectors, without checking acquisition subject or source. The first subject's replacement is then inserted and its ledger timestamp updated. [Replacement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L267-L303), [delete predicate](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge_delete.go#L20-L40)

5. **The second refresh deletes the first replacement.** Its prior ledger also names `web/c/section_0`. The same broad deletion now removes the just-inserted first subject's row and associated vectors. The second subject installs its own replacement. Both successful outcomes are upserted with a current attempted time and the shared concept label. The survivor depends on subject processing order, not on knowledge relevance. [Loop and ledger update](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L267-L320)

The minimal destructive variant refreshes only one of the two subjects: it deletes the other subject's old material too. Selecting both merely makes the discrepancy especially clear because both acquisitions report fresh success while only one corresponding body remains.

## Consequence and limits

The established consequence is loss of another acquisition's materialized section and any matching knowledge vectors, despite successful acquisition bookkeeping. The next measurement reads the ledger directly into `knowledge_outcome`; it does not verify that the subject's own body is still represented. `knowledge_acquired` follows acquired outcomes, and unlearned admission is suppressed by that fact. A freshly successful outcome is also not stale. Without another trigger, this deletion does not itself schedule immediate repair. [Ledger measurement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/measure.go#L35-L61), [acquired and unlearned rules](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L35-L82)

This does **not** establish a measured wrong answer, loss of all expertise, unavoidable model failure or a prevalence estimate. Another section, source, agent, live tool lookup or later gap-triggered acquisition may compensate. A later refresh can restore one body while evicting its sibling again; whether that occurs in deployment has not been measured. Distinct agent KBs are separate ownership domains, so this is not a claim that ordinary separate-agent files overwrite one another.

Context7 has the same naming problem, but the clean stock-policy witness above deliberately uses web: it admits ordinary stale refresh at 31 days without crossing the prune-age boundary. Direct Context7 replacement needs neither a 91-day scheduled fixture nor an invented pruning failure to exhibit the same owner loss.

## Existing safeguards and why they do not settle ownership

**The previous generation fence is real.** Main captures KB generations during measurement and calls `DeleteKnowledgeAtomsByConceptThrough` for the later prune phase. That method restricts atom and vector deletion to IDs at or below the measured generation. A replacement written after measurement survives that old prune authorization. Preserve this fix. [Measured generations and current prune call](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L95-L129), [fenced prune](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L174-L189), [generation-aware deletion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge_delete.go#L42-L94)

The replacement call at `acquire.go:289` still uses the unfenced concept-wide API. Applying a global generation cutoff there would not be a complete ownership repair: it might protect a sibling's newly inserted row, but it would still permit deletion of the sibling's preexisting row under the shared concept. Generation answers **which revision** may be retired; subject ownership answers **whose rows** may be retired. Both matter.

Other valid controls remain:

- Runs of one refresher are serialized. The witness is already serial. [Run mutex](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L42-L74)
- Failed, empty, unavailable and inapplicable outcomes do not authorize replacement. The witness requires acquired outcomes. [Replacement guard](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L272-L287)
- An acquired source is skipped by ordinary `Fill`; an intentional `Refresh` asks it again. The damaging lifecycle is replacement, not every repeated init.
- Different slugs, different KBs and no previous concept receipt do not satisfy this exact witness.
- Cited answers and per-row provider provenance are useful, but deletion does not inspect them.
- The store transaction makes atom/vector deletion coherent for its chosen concept set. It cannot make an overbroad set belong to the requesting subject.

Existing `TestRefreshReplacesWhatTheLastAcquisitionWrote` protects legitimate same-owner replacement and failed-refresh retention. Its fake producer names concepts using raw `s.Key()`, and its fixture has one subject. It therefore bypasses the real lossy slug transformation rather than exercising this collision. This is a statement about that inspected fixture, not a claim that all knowledge tests are absent. [Existing test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/refresh_test.go#L14-L68)

## Repair direction with coherent scoped replacement

First update the owning knowledge/store/system specification with a checkable invariant:

> A successful replacement retires only the prior materialization owned by that exact logical source and admitted subject in that KB. Its committed receipt names the installed generation. Another owner's rows are unchanged, even when display concepts or content are equal.

Keep policy in Mangle: age, wanted subjects, priority, budget and any materialization-repair admission remain derived decisions. Go should preserve identities, perform source I/O, validate results and enforce the store's mutation preconditions. This does not require a separate agent framework, a new prompt path or more routine model calls.

A coherent design should address all of the following:

1. **Separate identity from display.** Preserve the exact admitted subject key with an unambiguous encoding; retain a slug only for readability. An owner is at least the KB namespace, logical acquisition source and subject key. If module-versus-topic distinctions are added beyond today's `Subject.Key()` contract, version that identity change explicitly. A digest can provide a bounded key, but retain the original tuple and validate collisions; a punctuation-preserving label or readable slug alone is not a proof of uniqueness.

2. **Distinguish logical source from provider provenance.** A web acquisition is owned by `web`, with the successful provider recorded separately. Provider fallback must still replace the same subject's prior web generation rather than accumulating one unretirable corpus per provider. Conversely, source alone is too broad: two web subjects must not retire each other.

3. **Persist exact materialization ownership.** Bind atoms and vectors to owner and generation, preferably through stable row/acquisition references, not only a shared concept string. Fresh concepts can include a collision-resistant owner identity, but destructive operations should still validate ownership. If the design shares identical content across owners, store explicit ownership associations and remove the shared physical row only after no live owner requires it. The simpler alternative is separate owned materializations; do not introduce implicit sharing through content deduplication.

4. **Commit one replacement coherently.** Acquire and validate external results outside a long write transaction. Stage any required embedding work outside it too. Then, in one bounded persistence operation, check the expected prior owner generation, install the accepted replacement, retire only that owner's previous rows and update its materialization receipt. A stale concurrent attempt must not replace a newer generation merely because its provider call finished later. Failed acquisition retains the previous committed materialization.

5. **Represent degradation truthfully.** Current storage can retain an atom when embedding fails; a repair should not convert lexical-only knowledge into “not acquired” by accident. If that behavior is retained, record present rows separately from vector readiness and allow bounded vector repair without repurchasing the web answer. Preserve the producing embedding model identity required by store guidance. [Current embedding fallback](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L384-L412)

6. **Keep attempt and materialization evidence distinct.** The most recent source attempt may fail while a prior generation remains useful; a successful historical attempt may later have its rows intentionally pruned. A receipt should establish which owned generation is present, partial, retired or unknown without falsifying source-attempt history. Any repair trigger must respect the existing budget/backoff and legitimate pruning rather than immediately refilling every intentionally evicted row.

7. **Preserve generation-aware pruning.** Scoped replacement complements the landed prune fence; it does not replace it. Retire only the owned rows named by the relevant decision and generation, preserving unrelated writers and the newer generations the existing fence protects.

A smaller forward-only concept-identity patch is worth comparing, but only if its replacement path cannot still use legacy broad concept lists. Changing new labels while deleting old labels by concept recreates the destructive migration on the first refresh.

## Migration is part of correctness

Existing KBs do not have an unambiguous reverse mapping from concept to owner. The ledger can identify that two acquisitions claim the same concept, but that does not identify which existing row belongs to which claimant. Atom source records the concrete producer, not the acquisition subject; vectors carry concept metadata rather than an owner link. Identical content can also have been deduplicated. Do not infer ownership by reparsing generated prose or arbitrarily assigning every colliding row to the first subject. [Current stored row fields](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L32-L64), [vector metadata](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L400-L407)

A safe migration should inventory receipts, distinguish provable mappings from ambiguous ones, retain ambiguous legacy material with explicit unknown ownership, and reacquire affected subjects under versioned scoped identities within authorized budgets. Retirement of the ambiguous legacy set needs a deliberate migration rule after replacement coverage is established; the ordinary single-subject refresh must not guess. A failed or canceled migration must leave a recoverable manifest and must not label unmaterialized subjects freshly acquired.

Renaming concepts also affects content hashes, vectors, retrieval usage and any persisted references. Usage is measured by concept today, so splitting one historical concept's count across newly distinct owners cannot be done honestly without further evidence. Preserve ambiguous historical accounting as such rather than duplicating credit or silently choosing one owner. [Hash input and dedup state](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L239-L245), [usage measurement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/measure.go#L85-L101)

Define mixed-version behavior and rollback before rollout. An old binary that still deletes by concept must not be allowed to mutate a new ownership schema as though it were an ordinary old KB. Retaining legacy rows temporarily increases storage and retrieval ambiguity; quarantining them reduces that ambiguity but can reduce recall. Neither tradeoff is free, so expose the disposition and measure it.

## Fair evaluation and complete costs

Everything in this section is proposed and **unrun**. Establish the deterministic persistence contract before any model-quality experiment.

Use the pinned implementation as baseline, including its generation-aware prune fix. Compare a minimal namespaced-identity repair with coherent owner/generation-scoped replacement and migration; do not compare against an obsolete unfenced-prune baseline. Keep source payloads, timing, topics, source availability, policy budget and storage configuration matched. Count failed and canceled attempts, not only successful survivors.

The primary regression fixture should use real `KnowledgeSubjects`, `NewAcquirer`, `WebChain`, `WebSource` naming, the real store and real refresh policy with a deterministic grounded-search stub. The stub supplies distinct cited paragraphs; it must not construct already-correct concept names itself. Initialize both topics, advance the measurement clock to 31 whole days, ensure both remain selected, and exercise the actual Cortex refresh entry. Assert each owner's exact content after initial fill, after refresh, after reopen and after the next measurement. Check vectors when the embedding engine is enabled, not just row counts or ledger statuses. Also validate that scheduled boot installs the same path.

Negative controls and extensions should include:

- Two noncolliding topics, one topic, separate KBs, reversed input order, only one subject selected, and budget displacement.
- Punctuation variants, non-ASCII inputs, empty resulting slugs, and matching section indexes across distinct subjects.
- Same-owner refresh with unchanged text, changed text, removed sections and added sections; stale rows must actually retire.
- Failed, empty, unavailable and canceled acquisition; prior materialization must remain owned and available.
- Web-provider fallback; logical ownership stays stable while concrete provenance changes.
- Context7 using its actual producer, plus multiple sources for one subject.
- Existing same-concept prune-generation witness and an independently eligible old-row prune; neither prior correctness nor valid pruning may regress.
- Legacy collision receipts, ambiguous/shared rows, missing vectors, repeated migration, reopen during migration, and supported rollback behavior.
- Storage/ledger failure between stages and two same-owner attempts completing out of order; assess the proposed transactional and generation contract without assuming the original witness needs these failures.
- Semantic and lexical recall of each surviving subject under normal selectors and budgets; do not force all knowledge into the prompt to manufacture success.

Primary deterministic outcomes are cross-owner deletion count, exact retained content per owner, stale-row retirement, receipt/materialization agreement, recoverability and recall after reopen. Passing means the collision witness fails on the pinned baseline and passes after repair while negative controls preserve legitimate replacement and pruning. A row-count-only check is insufficient: two copies of one subject could satisfy the count while the other remains absent.

Account for schema/index growth; ownership and generation joins; content hashing; transaction count and lock duration; row/vector insertions and deletions; ANN maintenance; memory and scan costs; migration manifests; retained ambiguous material; provider calls and retries; embeddings and re-embedding; cancellation recovery; mixed-version compatibility; and human review. New bounded namespaced identities may have small direct CPU cost, but migration and duplicate owned materialization can dominate disk and provider spend. Scoped replacement may prevent destructive reacquisition churn, but no saving is established until measured.

If a later authorized evaluation asks whether preserved knowledge improves coding, use paired task families that require each colliding subject, fixed model/provider and tool budgets, and independent task acceptance. Report quality, missing-context failures, full token/tool/test/provider cost and latency, including unsuccessful attempts. Retrieval availability is not itself proof of better answers; shorter prompts or fewer calls are not improvements if necessary knowledge was lost. Predeclare acceptable correctness and cost thresholds, retain negative results and allow the conclusion that the robust design costs more without a demonstrated downstream quality gain.

## Acceptance boundary

The repair is ready only when a refresh can identify exactly whose prior generation it replaced, show that the replacement remains present after the whole run and reopen, and show that another subject's materialization was untouched despite colliding display labels. The existing generation fence must continue to protect newly acquired rows from old pruning decisions. Successful source output and a fresh acquired ledger entry alone are not that evidence.

