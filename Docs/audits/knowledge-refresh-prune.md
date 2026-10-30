---
doc-class: deep-dive
subsystem: knowledge-refresh
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6
verification-method: static-source-inspection
---

# Knowledge refresh must not prune the replacement it just acquired

**Decision:** bind pruning to the measured row generation, or resolve refresh/prune overlap before mutations. An old concept's eligibility is not authority to delete a new row with that concept.

**Evidence status:** read-only inspection of public CodeNERD main `a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6` on 2026-10-30. This is a source-predicted, conditional composition defect, not an observed production incident. No code, tests, database experiment, model inference, provider request or computer task was executed. This document proposes changes; it does not implement them.

## Contract and ownership

Root guidance requires stale evidence not to survive source changes as current truth and assigns executive choices to Mangle. This report does not challenge pruning unused knowledge or refreshing stale acquisitions individually. It challenges their composition: a prune decision over an old row is applied by concept after a successful refresh has installed a new row. [Guidance](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/agents.md)

The relevant neighboring audit, [expert lending](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/audits/expert-lending-dependency-contract.md), owns persona-independent exported dependency graphs; it does not own acquisition replacement and pruning. Searches for `knowledge_refresh`, `knowledge_misses`, and refresh/prune combinations found implementation/tests and neighboring audits, but no dedicated refresh/prune overlap report. This is bounded deduplication, not an exhaustive novelty claim. Existing generic source-retirement and cache-generation contracts remain relevant principles; this report identifies a separate mutating lifecycle boundary.

## The production path

Boot constructs a refresher from the session's configured grounded search, Context7 tool, pinned Go documentation and world-model importers, and schedules it on the Cortex lifetime. Stock schedule is 120 seconds after boot, then hourly; explicit `RefreshKnowledge` uses the same `run`. A per-refresher mutex serializes scheduled and explicit runs. No concurrency is needed for the witness. [Boot](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/system/knowledge_refresh_boot.go#L11-L58), [schedule and entry](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/system/knowledge_refresh.go#L191-L201), [policy constants](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L11-L33)

Each run reads ready roster agents, wanted topics/modules, acquisition outcomes, held rows and retrieval usage; it asserts measurements and computes **both** refresh and prune lists. It then runs every selected refresh before executing the previously computed prune list. The prune call deletes by concept, not by the identity or age of the measured row. [Plan and ordering](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/system/knowledge_refresh.go#L141-L185), [prune extraction](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/system/knowledge_refresh.go#L336-L347)

The measurements use acquisition `attempted_at` for outcome age and atom `created_at` for row age. These are distinct clocks, each reduced to whole elapsed days. [Measurement](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/knowledge/measure.go#L25-L85)

## A concrete conditional witness

Use a ready agent whose registry still wants a particular Context7 topic. Assume:

1. Its Context7 acquisition is successful but at least 91 whole days old.
2. At least one existing concept, e.g. `context7/widget-calibration/section_0`, is more than 60 whole days old and has no retrieval entry.
3. Other concepts in that agent's usage table contribute a sum of at least 20 retrievals.
4. This subject is selected within the four-subject refresh budget; no more urgent subjects displace it.
5. Context7 is wired and successfully returns a qualifying section under the same concept key; storage and outcome recording succeed.

Condition 3 is deliberately stated as **summed per-concept retrieval counts**, not “20 distinct consultations.” The rule sums those counts, so one consultation carrying multiple concepts can contribute multiple units. The overlap defect does not depend on that separate metric interpretation. [Usage aggregation](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L48-L60)

The policy derives both `knowledge_refresh(..., /stale)` from Context7 age >90 and `knowledge_prune(..., /never_retrieved)` from row age >60, absent use and aggregate usage >=20. There is no mutual exclusion between these derivations. [Refresh](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L79-L82), [prune](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L110-L121)

Context7 deterministically names sections using topic slug plus section index, not acquisition generation. A refreshed section can therefore legitimately reuse the old concept despite new content. [Context7 producer](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/knowledge/sources.go#L115-L147)

`Refresh` calls `fill(..., true)`. On successful acquisition, `fill` deletes the concepts recorded for the previous source/subject, inserts newly acquired atoms, then upserts the acquisition record with successful status and current attempted time. Replacement is explicitly intended to include rows written again under the same concept. [Refresh contract](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/knowledge/acquire.go#L218-L230), [replacement and ledger](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/knowledge/acquire.go#L267-L323)

The outer run then calls `DeleteKnowledgeAtomsByConcept` with the **old** prune list. That store operation deletes every matching atom and its knowledge vectors in a transaction, without an age, row-ID, source-generation or last-use predicate. Thus the freshly inserted replacement is deleted. [Concept deletion](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/store/local_knowledge_delete.go#L9-L37)

## Observable consequence, not speculative model harm

Under those conditions the run can report atoms added and a successful acquisition while the just-acquired section is absent from the KB and its vectors. The acquisition ledger is not deleted or marked incomplete by pruning. The next measurement therefore still reports the subject acquired, and its fresh outcome is not stale. With no independent trigger, that original subject will not immediately be reacquired merely because the section vanished. [Acquired/unlearned policy](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L37-L45), [unlearned and stale](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L72-L82)

This does not establish guaranteed wrong model answers or loss of all expertise: another section, source, agent, later gap, dependency-version change or tool call may compensate. A new miss keyed to different task prose can trigger separate research. The precise failure is successful refresh followed by deletion of its replacement using obsolete eligibility.

## Guards and negative controls

- The mutex prevents overlapping runs of this refresher, but cannot repair sequencing inside one run.
- Refreshed concepts that were retrieved are not prune candidates.
- Young held rows, usage below threshold, and kinds outside the prunable list are excluded.
- A subject outside the budget is not refreshed; deleting its old eligible row is the intended pruning behavior, not this overlap witness.
- If a source changes the concept key, the old concept deletion need not delete its replacement. Context7's stable section keys make the case concrete.
- Failed/empty acquisition does not replace prior rows in `fill`; subsequent old-row pruning is a distinct retention-policy question.
- Read/store failures are reported and can change what is installed; the witness assumes successful operations.
- An unchanged successful answer still gets replacement processing, so this is not limited to changed text.

The existing system integration test covers successful acquisition, an isolated stale refresh, and subsequently an unrelated old `web/forgotten` row. It does not combine stale acquisition and prune eligibility for the same stable concept in one run. These test definitions were inspected, not executed. [Existing test](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/system/knowledge_refresh_test.go#L130-L181)

## Repair direction without moving policy into Go

Make the invariant explicit in schema/policy and the storage mutation contract: **a prune authorization names the evidence generation it evaluated; acquisition of a replacement invalidates that authorization.**

Options to evaluate:

- Derive exclusion for concepts owned by a selected refresh, using an explicit acquisition-to-concept relation and selected-plan facts. This requires distinguishing budget-selected refreshes from every theoretically refreshable subject.
- Prune measured rows before installing replacements, with generation-aware deletion to protect independently updated rows. Ordering alone addresses this single-run witness but not concurrent KB writers.
- Re-measure and rederive pruning after refresh; deletion should still compare row generation or equivalent transactional preconditions to avoid a new read/delete race.

Do not silently stop pruning all refreshed subjects forever, make every knowledge row mandatory in prompts, or erase valid source provenance. Avoid a long transaction around external research calls. Any acquisition/retirement ledger change should express retained versus present materialization clearly rather than pretend the last attempted status proves rows still exist.

## Proposed verification and costs

All verification below is **unrun**. Extend the real-boot fixture with a deterministic Context7 replacement using the actual stable naming contract, old acquisition and row clocks, enough usage on a separate concept, and a subject guaranteed inside budget. Assert both derivations before the run, then assert that the fresh replacement and vectors survive, its ledger matches retained materialization, and an immediate second run is coherent.

Controls: no-overlap refresh; intended old-row prune; same versus changed concept; same versus changed content; used concept; usage threshold boundaries; 60/61 and 90/91 whole-day boundaries; selected versus displaced subjects; multiple sections; failed/empty acquisition; refresh cancellation and storage failure; concurrent retrieval or source update between measurement and deletion. Verify direct explicit invocation and scheduled invocation share the corrected contract.

Re-measurement adds database scans and kernel derivations; generation-aware deletion adds schema/index and comparison work; provenance-aware exclusions add facts and plan state. Retaining fresh sections increases storage and may add retrieval candidates. Measure rows scanned, kernel evaluation latency, write transactions, vectors generated/deleted, provider calls and next-run behavior on matched fixtures. No cost saving, answer-quality gain or production incidence rate has been measured.

## Other triaged candidates

**“Retry admission lacks a window” is not supported as a blanket claim.** The explicit failed-attempt rule requires `Age >= knowledge_retry_after_days(1)`; unlearned and gap rules also reject recent attempts. New-source and version-change reasons are independent admissions. Any claim about repeated unavailable-source work needs a concrete mismatch between source availability and source outcome, not omission of the actual retry guard. [Rules](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/defaults/policy/knowledge_refresh.mg#L67-L102)

**“No IncludedAtoms means corpus gap” remains a separate, lower-priority measurement concern.** Consumption recording examines final fitted atoms and writes a miss when a consulted expert contributes none; successful compilation may omit retrieved candidates due to policy/budget or optional-source degradation. Therefore that observation alone does not distinguish genuinely empty retrieval from omitted delivery. The pipeline collects, selects, resolves and fits before setting `ExpertsConsulted`, and its success wrapper records consumption. Existing recorder test covers a true lexical miss rather than budget omission. This needs a targeted counterexample and contract decision, not presentation as already-measured unnecessary network spend. [Recorder](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/prompt/compiler_knowledge_use.go#L19-L48), [pipeline](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/prompt/compiler_pipeline.go#L262-L367), [test](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/prompt/compiler_knowledge_use_test.go)


## Primary reference

[SQLite transaction documentation](https://www.sqlite.org/lang_transaction.html) describes transactional boundaries. A transaction protecting one delete does not by itself validate an earlier decision derived from a different snapshot; the application must express the required preconditions.
