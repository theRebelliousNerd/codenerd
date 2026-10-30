---
doc-class: deep-dive
subsystem: knowledge-refresh
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Knowledge miss admission and retirement after retrieval recovery

**Decision:** distinguish a knowledge lookup's result from whether its rows survived prompt selection and fitting. Treat an admitted research gap as a current, query-scoped observation with an explicit retirement contract, rather than an everlasting consequence of one prompt carrying no rows.

**Evidence status:** read-only inspection of public CodeNERD main `486db57ce490302c65d7f484e7f6070ee0e70862` on 2026-10-30. The witness below is a conditional source prediction, not an observed production incident or executed regression. No code, tests, database experiment, model inference, provider request, or computer task was run. This document proposes changes; it implements none.

## Scope and prior ownership

The existing [knowledge refresh and prune audit](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/knowledge-refresh-prune.md) already identifies “No IncludedAtoms means corpus gap” as a separate lower-priority measurement concern. This document continues that triage; it does not claim to discover the base conflation anew. Its additional lifecycle question is what happens when a later compile successfully delivers existing knowledge for the same query before background research begins. Current usage recording cannot retire the old query miss, and refresh policy does not consult recovered delivery.

The refresh/prune overlap remains separately owned by that audit. [Generation-aware JIT optimization](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/jit-generation-aware-optimization.md) owns cache freshness and optional-source degradation. Its uncacheable-degraded-result guard does not settle the meaning or lifetime of a knowledge miss. Bounded searches and those neighboring reports support this ownership distinction, not an exhaustive novelty claim. Search results were indexed at an older revision; substantive implementation references were fetched independently at the pinned SHA.

## Production observation path

The compiler gathers knowledge before selection, dependency resolution, fitting, and final assembly. A successful public `Compile` calls `recordKnowledgeConsumption`, including cache hits and singleflight-shared results. The final result carries the fitted atoms and the source snapshot's list of consulted experts. That list names registered expert handles, not a structured per-expert successful lookup result. [Compile wrapper and pipeline](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_pipeline.go#L14-L29), [fitting and result publication](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_pipeline.go#L349-L367), [consulted expert list](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_generation.go#L98-L131)

Knowledge retrieval produces optional prompt atoms: `knowledgeAtomToPromptAtom` explicitly sets `IsMandatory=false`; expert provenance is attached separately. The fitter can reject an atom that does not fit, retry available variants, and ultimately omit it. This is legitimate budget behavior, not itself the defect. The prompt can succeed if its mandatory skeleton fits while optional knowledge does not. [Knowledge conversion](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_db.go#L462-L495), [budget rejection and second pass](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/budget.go#L643-L715), [fit and enforcement](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_render.go#L14-L63)

The recorder examines only final `IncludedAtoms`. For each consulted expert, at least one identified knowledge concept produces `RecordRetrieval`; no such concept produces `RecordMiss` using `cc.SemanticQuery`. It does not inspect retrieved candidates, selection exclusions, budget omissions, or degraded-source flags. A trimmed empty query is not recorded. [Recorder](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_knowledge_use.go#L19-L48), [miss storage](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/usage.go#L57-L77)

Lookup failure is another admission route, but not required for the budget witness. Expert lookup errors or timeout return no atoms and mark degradation. Collection is nonfatal, so a successful compile can still record a miss. Degradation prevents cache publication and allows later re-probing; it does not prevent consumption recording. This is a secondary conditional route, not a claim that every timeout produces a durable miss: the recording database write must itself succeed after the lookup problem. [Expert lookup](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_db.go#L433-L452), [nonfatal knowledge collection](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_pipeline.go#L261-L268), [degradation contract](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_generation.go#L178-L182)

## Concrete recovered delivery witness

Use a ready roster expert whose registered KB and refresh KB identify the same database, and a nonempty semantic query Q. Assume:

1. The KB already holds a matching row K, and the lookup returns it. Q is not the exact subject key of an existing successful acquisition.
2. In compile A, K is the expert's only delivered-eligible knowledge candidate, but its rendered content cannot fit the remaining budget. There is no smaller knowledge variant that fits and no other included concept from this expert. The mandatory skeleton fits, so compilation succeeds.
3. The recorder successfully writes `knowledge_misses(Q)`.
4. Before refresh runs, compile B uses the same Q with sufficient budget, successfully includes K, and successfully records its use. No new external acquisition for Q occurs.
5. At refresh, the agent is ready and measured, there is still no successful acquisition or recent attempted outcome keyed by Q, and Q is selected within the four-subject budget.

The observable source prediction is:

- Compile A records a query miss despite having retrieved an existing row.
- Compile B updates K's usage but leaves Q's miss intact.
- Refresh sees Q as a gap even though the original absence-from-prompt condition has recovered.

This does **not** establish that K semantically answers all of Q. Current code uses delivery of any concept from an expert as the positive consultation outcome. The narrow inconsistency is that the later positive outcome has no query-scoped representation capable of qualifying the earlier negative one. Research might independently be beneficial; this witness establishes unsupported persistence of the original gap evidence, not guaranteed useless research.

## Why the old miss survives

`RecordRetrieval` accepts concepts and a timestamp, not the query. It only upserts `knowledge_usage`, incrementing each distinct concept once per call. It neither updates nor removes `knowledge_misses`. `RecordMiss` stores a count and last-missed time per exact trimmed query. [Usage and miss writers](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/usage.go#L31-L77)

The next measurement selects every miss's query and count. It does not select `last_missed_at`, and has no recovered-query observation to compare with it. Thus old negative evidence is emitted again regardless of later delivery. Usage facts exist, but they describe concepts and counts, not whether Q's earlier observation remains current. [Usage measurement](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/measure.go#L85-L120)

The policy admits a gap when a miss exists, no source has an acquired outcome for that same query subject, and no source has recently attempted that query. It does not require a minimum miss count, check a miss age, or reject the gap on later successful retrieval/delivery. Successful acquisition under a broader library or module subject does not equal successful acquisition under arbitrary task prose Q. [Acquired and recent attempt definitions](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/knowledge_refresh.mg#L37-L45), [gap and priority](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/knowledge_refresh.mg#L97-L108)

## How the observation becomes research

The planner sorts refresh priorities by weight, then aggregate agent use, then deterministic agent and subject ordering. It keeps the budget's worth. A query absent from wanted registry subjects is normally skipped, but a derived gap is an explicit exception: it becomes `Subject{Agent: ..., Topic: Q, Gap: true}`. [Plan](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/knowledge_refresh.go#L267-L334)

The run calls `knowledge.Refresh` on selected subjects. Refresh asks all configured sources again. The standard acquirer contains grounded web search, Context7, pinned Go documentation, and repository importers. For a prose-only gap, Go documentation and repository sources classify absent module context as inapplicable; web and wired Context7 remain potential research paths. The web question explicitly asks the engineer's task prose. [Run](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/knowledge_refresh.go#L141-L167), [Refresh](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/acquire.go#L218-L230), [source wiring](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/wiring.go), [web gap question](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/sources.go#L80-L88), [Context7](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/sources.go#L124-L145), [Go documentation applicability](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/sources.go#L184-L190), [repository applicability](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/knowledge/sources.go#L271-L277)

The background worker is real production wiring: boot constructs it from the session's clients, research tools, workspace, and kernel, then starts it on a Cortex-owned lifetime. Scheduled and explicit refresh share `run`. [Boot](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/knowledge_refresh_boot.go#L11-L58), [explicit entry](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/knowledge_refresh.go#L191-L201)

## Existing guards and limits

- Stock schedule is 120 seconds after boot and hourly thereafter; a short-lived process may never refresh.
- Stock refresh budget is four subjects. A higher-priority version change or competing gap can displace Q.
- A successful acquisition for exact subject Q suppresses its gap. A recent attempt suppresses it while its whole-day age is below one.
- Any compile failure bypasses consumption recording. In particular, a mandatory skeleton that cannot fit is not this witness.
- No registered expert, no writable consumption database, or empty semantic query can prevent the miss from being recorded.
- Source availability, provider failure, cancellation, and storage failure affect actual research and materialization. No provider-spend amount or frequency is established here.
- Degraded results are uncacheable, but uncacheability is not miss retirement.
- A later compile delivering any concept updates usage, not an acquired outcome keyed by Q.
- The rule does not promise perpetual hourly research of an identical successful gap. Acquisition and retry guards must not be omitted when characterizing recurrence.

[Policy constants](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/knowledge_refresh.mg#L11-L33), [retry and gap guards](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/knowledge_refresh.mg#L87-L102), [compile success wrapper](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_pipeline.go#L23-L29)

## Proposed contract and policy ownership

Keep acquisition decisions in Mangle; make Go report sufficient typed evidence to support them.

First separate lookup and delivery. Per expert/query, distinguish not searched, successful empty lookup, successful nonempty lookup, lookup error/timeout, retrieved but policy-excluded, retrieved but budget-omitted, and delivered. A registered handle is not evidence that a successful search found nothing. Candidate hits, fitted delivery, and uncertainty should not collapse into one negative state.

Then define research eligibility and retirement explicitly. A lookup-level gap may be admitted only from a completed empty lookup under identified query and source conditions, unless policy deliberately chooses another reason. A delivery omission can justify context-allocation investigation, but must not silently become a corpus-empty assertion.

For later evidence, choose and document the intended meaning:

- If the gap means successful retrieval found no matching rows, a later successful nonempty retrieval for the same identity supersedes that narrow assertion.
- If the gap means the model still lacks adequate answer evidence, delivery of an arbitrary row cannot automatically close it. That stronger meaning requires an explicit sufficiency observation; it cannot be inferred merely from `IncludedAtoms` either negatively or positively.
- If historic misses are retained for diagnosis, separate history from the currently eligible state. Do not destroy history merely to stop obsolete work.

Identity must include the agent/workspace and query semantics actually searched, with an appropriate source generation or observation ordering. Existing recording uses `SemanticQuery` while lookup builds expanded and lexical queries, so “same query” needs an explicit contract rather than assuming task prose captures every routing term. A newer genuine empty result must be able to reopen eligibility; an older in-flight miss must not overwrite a newer recovery. A cache hit can record delivery of its generation-bound result without inventing a fresh database search.

Legacy miss rows lack sufficient provenance. A migration should label them historical/unknown or revalidate them under bounded policy before research, rather than treating them as either proven current gaps or safely resolved. Preserve exact literal query identity unless a separately justified normalization contract exists.

## Fair verification plan

All verification below is **proposed and unrun**.

Build the smallest deterministic compiler fixture with a real matching expert KB row and no external providers. Choose budgets so the first compile demonstrably succeeds, retrieval demonstrably returns K, and K is absent solely because it cannot fit; the second compile demonstrably includes the same K for Q. Assert intermediate lookup, selection, fit, recorder, measured facts, and derived refresh decision rather than checking only an eventual provider-call count.

Run a companion refresher fixture with a deterministic counting acquisition source. Ensure Q has no acquired/recent outcome and no competing higher-priority subjects. Verify that existing code selects the stale observation; a proposed correction should either avoid admitting the false corpus gap initially or retire its narrow negative assertion after valid recovery. Keep expected acquisition calls for genuine still-empty queries intact.

Controls should cover:

- Genuine empty lookup with no recovery; successful nonempty retrieval omitted by budget; successful nonempty retrieval omitted by policy; delivered knowledge.
- Lookup timeout/error followed by recovery; failed recording write; empty query; no registered expert.
- First compile failing because mandatory content cannot fit, which must not count as successful consumption.
- Exact query versus similar wording, different agent, different workspace, changed source generation, and changed expanded routing context.
- Cache hit and singleflight sharing without relabeling cached delivery as a new search.
- Later genuine miss reopening a recovered query, out-of-order completion, and concurrent recording against refresh measurement.
- Existing acquired Q and age boundaries around the one-day retry window; selected versus budget-displaced gaps; absent provider support and inapplicable module sources.
- Historical miss migration and crash/restart behavior.
- Successful retrieval that is relevant but insufficient: ensure the repair does not claim semantic answer completeness.

The existing [recorder test](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_knowledge_use_test.go) tests one successful delivered query and a different genuinely missing query. It does not exercise omission followed by same-query recovery. Its definitions were inspected, not executed.

## Costs and acceptance criteria

More precise observations require schema/state, ordering, and migration work. Per-query writes may increase SQLite contention and storage. Retaining histories adds maintenance costs; retaining only current state loses forensic detail. Adding source generation to every observation increases identity cardinality and can suppress reuse if too granular. Revalidating a gap before research adds lookup work and latency, even when it correctly avoids external acquisition.

Compare the same compiler inputs, budgets, source generations, provider availability, and refresh budget before and after the change. Measure admitted gaps by reason, resolved/reopened observations, research subjects selected, external requests, lookup work, database writes and lock time, refresh latency, and legitimate new knowledge acquired. Separately evaluate retrieval coverage and answer sufficiency; fewer provider calls alone is not success. The repair is acceptable if it removes stale or misclassified admission without suppressing genuine unresolved gaps or breaking budget discipline. No runtime savings, quality uplift, or production incidence rate has been measured.


## Primary reference

[SQLite UPSERT documentation](https://www.sqlite.org/lang_upsert.html) describes the conflict-update mechanism used for persistent observations. Whether a subsequent observation supersedes earlier evidence is an application-level identity and ordering contract.
