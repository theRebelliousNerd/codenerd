---
doc-class: research-proposal
subsystem: store-vector-migration
implementation-status: proposed-refinement
evidence-status: static-source-and-sql-semantics
last-verified: 2026-11-04
verified-against: 3c662caac3c59351983b7a7cb24ac42f853f595b
supersedes: []
---

# Resumed vector migration must retain NULL stamped rows

## Decision

Give force reembedding a NULL-safe pending-row predicate. The current resumed inventory is constructed by negating the semantic-read eligibility expression. For a legacy row with a non-NULL embedding and a SQL NULL model stamp, both eligibility and its SQL negation evaluate to NULL. Neither selects the row. A failed first force pass therefore leaves a row that subsequent same-target passes count as pending but never send back to the provider. [Inventory][inventory] [Eligibility and coverage][identity] [Resume state][resume]

This is a **retry-liveness failure**, not a false-success finding. Coverage still reports the row as unattributed; the run remains incomplete and the CLI exits nonzero. The operator is explicitly told to rerun the command, but the unchanged resumed selector cannot make progress on this row even after provider recovery. [Completion][finish] [CLI result][cli-result]

The bounded repair is to preserve every missing-attribution row in the authorized migration worklist while leaving semantic-read admission strict. Do not stamp the old vector with the current model name, loosen the read-side filter, delete legacy data, or require a new global migration architecture to fix this selector.

## Evidence and scope

Public `main` resolved to **3c662caac3c59351983b7a7cb24ac42f853f595b** on November 4, 2026. All repository links in this report are pinned there. Root guidance, scoped store guidance, relevant store architecture, current migration and recall implementation, authored tests, and the neighboring embedding audits were inspected. The recurse inventory and current recurse sources were screened first; the concrete Python lexical-state candidate overlapped existing import-scan research and was not republished. [Commit][pin] [Root guidance][root] [Store guidance][guidance] [Store specification][architecture]

No repository program, build, test, gate, benchmark, SQL experiment, model inference, training or provider request was run. The values and call outcomes below are source-and-semantics deductions, not captured execution output. Only documentation and source copies in scratch storage were prepared. The example uses synthetic content and no private project material.

| Status | Established boundary |
|---|---|
| Implemented | Nullable legacy model stamps; strict semantic eligibility; durable per-table migration state; resumed selection; conditional vector publication; coverage-based incomplete errors; CLI retry guidance. |
| Implemented protection | An unattributed old vector remains outside semantic ranking. Failed publication does not guess its producer identity. Coverage does not mistake this NULL-stamped row for a completed target row. |
| Partial | Authored resume coverage exercises foreign non-NULL stamps, but the selector does not cover the full pending domain. [Resume test][resume-test] |
| Proposed | A total pending predicate, partition invariants, NULL-specific restart acceptance, and a diagnostic distinction between pending work and failures attempted in the current pass. |
| Unverified | Runtime reproduction, incidence in actual databases, provider behavior, latency, resource cost and downstream task impact. |

The architecture corpus is useful for intent, but its older line counts and implementation descriptions are not substituted for current source. Root guidance explicitly warns about this distinction. The publication is a focused refinement of the already implemented migration work, not a claim that resumable migration or identity protection is absent.

## Production path

### Legacy NULL stamps are an intended state

The generic vector table originally has `content`, `embedding`, metadata and identity columns other than an embedding-model stamp. The additive migration declares `vectors.embedding_model` as `TEXT`, with no default or non-NULL constraint. Its comment explicitly identifies SQL NULL as the state of rows written before that column existed. The existing `TestLegacyDatabaseGainsTheStampColumn` creates a pre-stamp database, opens it through `NewLocalStore`, and asserts that the old vector survives with `embedding_model IS NULL`. This report reads that test as an authored contract, not a test result from this audit. [Schema][schema] [Migration][legacy-migration] [Legacy test][legacy-test]

NULL here is not the empty string, the JSON text `null`, an unknown provider version, or a corrupted vector. A valid stored vector with a missing SQL stamp is sufficient.

### Explicit force migration reaches the defective selector

The normal `nerd embedding reembed` command creates an engine from the workspace configuration and calls `ReembedAllDBsForce` over the workspace's `.nerd` roots. For each applicable database, `processLocalDB` opens a LocalStore, attaches the engine and directly calls `ReembedAllVectorsForce`. It records that table's outcome in the aggregate result. [CLI entry][cli-entry] [Workspace roots][roots] [Database dispatch][dispatch]

`ReembedAllVectorsForce` captures the engine, requires its nonempty identity, waits for pending index backfill, and opens or resumes the durable `embedding_migrations` record. A non-complete record with the same target is resumed under the same run ID. A resumed run passes its target to `listVectorsForReembed`; a fresh run passes an empty skip-space and therefore has no exclusion clause. [Force entry][force-entry] [Resume state][resume]

For a resumed run, the effective query is:

```sql
SELECT id, content, metadata
FROM vectors
WHERE TRIM(content) != ''
  AND NOT (embedding IS NOT NULL AND embedding_model = ?);
```

This is the composed source query, with `?` bound to the nonempty target identity. The expression inside `NOT` is also used for semantic-read eligibility. It is appropriate for admitting known compatible vectors, but negating it does not enumerate all rows that failed admission. [Inventory][inventory] [Eligibility][identity]

### Failure preservation and final coverage do different jobs

Batch embedding can fall back to per-item embedding. A per-item failure leaves the corresponding result empty; `applyVectorEmbeddings` counts it as skipped and leaves the existing row unchanged. The outer loop passes the skipped count to finalization. [Provider fallback][fallback] [Publication][publication]

`vectorCoverageLocked` explicitly counts a non-NULL embedding whose model is NULL or empty as unattributed. `Pending()` is total embeddable rows minus target-eligible rows. Finalization requires both zero failed items in this pass and zero pending coverage before marking the run complete. These are useful checks and remain load-bearing in this witness. The force method returns the incomplete error when that report is not complete. [Coverage][identity] [Completion][finish] [Returned outcome][settlement]

## One row and two invocations

This is an unrun deterministic fixture specification. It needs no concurrent writer, process race, invalid vector, changed engine identity, index failure or content change.

1. Start from a pre-stamp database containing one nonblank row, for example content `legacy alpha` and a finite nonzero four-dimensional embedding. Open it through the normal migration path. Its model stamp is SQL NULL, and there is no prior vector-migration record.
2. Use a deterministic double implementing only `EmbeddingEngine`, not either task-aware extension, whose `Name()` is a stable, nonempty `test:target` throughout both invocations and whose dimensions match the fixture. This makes the batch-and-fallback branch explicit. The first invocation's batch request fails, and its per-item fallback for this row also fails transiently. All storage operations succeed.
3. Call the normal force method. Because the run is fresh, the row is selected. The failed result is skipped, preserving the original vector and NULL stamp. Coverage is `Total=1`, `Eligible=0`, `Unattributed=1`, `Pending=1`. The pass records an incomplete run and returns `IncompleteReembedError`, with one failed item this pass.
4. Let the provider recover. Close and reopen the database, or keep the same handle. Use the same target identity, without an intervening semantic recall or another write that attributes the legacy row.
5. Invoke force migration again. The durable record is resumed. The NULL-stamped row does not satisfy the resumed inventory predicate, so no embedding request for that row occurs. Coverage remains pending, and the same run returns incomplete again, now with zero failed items in this pass.
6. Further identical invocations repeat step 5. Provider recovery cannot help a row never dispatched to the provider.

The key evaluation is:

```text
embedding IS NOT NULL                  = true
embedding_model = 'test:target'        = NULL
true AND NULL                         = NULL
NOT NULL                              = NULL
WHERE ... AND NULL                    does not retain the row
```

SQLite documents NULL propagation for ordinary comparisons and negation, the exceptions for `AND` and `OR`, and the boolean treatment of NULL. Its `IS` and `IS NOT` comparisons instead return a definite boolean when an operand is NULL. A WHERE condition that is false or NULL excludes the row. [SQLite expression semantics][sqlite-expr] [SQLite WHERE semantics][sqlite-where]

The selector is therefore not the set complement its use suggests:

| Embedding column | Model stamp | Current eligibility | Current negated eligibility | Selected on resume |
|---|---|---|---|---|
| SQL NULL | SQL NULL | false | true | Yes |
| SQL NULL | target | false | true | Yes |
| Non-NULL | SQL NULL | NULL | NULL | **No, despite pending coverage** |
| Non-NULL | empty string | false | true | Yes |
| Non-NULL | another nonempty model | false | true | Yes |
| Non-NULL | target | true | false | No, intended progress skip |

All rows in this table are nonblank; blank content is excluded by the existing inventory domain. The empty-string control matters: a test that uses `""` for every missing stamp would miss the SQL NULL case.

### User-visible consequence

The aggregate result remains incomplete because the table operation returned an incomplete error. The CLI displays the incomplete result and tells the operator to rerun `nerd embedding reembed` to resume. The second invocation's zero current-pass failures do not mean zero outstanding rows: the error separately carries `Total=1` and `Done=0`. No successful completion banner is established. [Manifest][manifest] [CLI result][cli-result]

The consequence is a persistent retry loop under an unchanged, supported command path. The row stays stored and keyword-recallable, but this explicit maintenance operation cannot restore its semantic eligibility. No measured user-task failure or irreversible loss is claimed.

## Existing recovery and limits

Automatic unattributed-vector healing already exists. Semantic recall invokes `maybeHealUnattributed`, whose worker selects rows with `embedding_model IS NULL OR embedding_model = ''` and computes fresh vectors before attributing them. A successful intervening recall can therefore change the row and break the retry loop. That is a separate recovery path, not proof the explicit force-resume path is complete. [Recall trigger][recall] [Healing selector][heal]

Engine attachment does not itself perform that semantic recall. It initializes the index and launches backfill, which reads only target-eligible canonical vectors. The NULL-stamped row is correctly excluded there. The CLI's inspected `processLocalDB` sequence attaches the engine and proceeds directly to force migration, with no generic semantic recall in between. Index reconciliation likewise selects eligible embedded rows; it does not manufacture a missing model identity. [Attachment][attach] [Backfill][backfill] [Database dispatch][dispatch] [Reconciliation][reconcile]

Qualifications that must survive a later implementation or summary:

- A completely successful fresh pass attributes the row and has no such retry problem.
- A missing embedding (`embedding IS NULL`) is selected on resume. The failure requires a present embedding and a NULL model stamp.
- A different nonempty model stamp and an empty-string stamp are selected on resume. This is not every partial migration.
- Changing the target creates a different run. The witness fixes the target identity and excludes model-name resolution changes.
- A semantic heal or another successful writer can restore attribution. Reopening alone preserves the durable pending state; it is not equivalent to running semantic recall.
- Indexed and non-indexed builds share the faulty inventory logic. This is not contingent on sqlite-vec availability, and fixing an index is not the central remedy.
- No second process is needed. The store's single-writer assumptions are respected.

## Required contract

For the existing target-coverage resumption mode, define the domain `D` as the nonblank rows the pass is responsible for and `E_T` as the rows admitted by the current target-space eligibility rule. The resumed worklist must contain `D minus E_T`, including rows for which the SQL eligibility expression is unknown.

At a stable snapshot, require:

1. **Coverage conservation:** each row in `D` is either target-eligible or selected as pending. No row disappears between those categories because a nullable attribute makes a predicate unknown.
2. **Disjointness:** a target-eligible row is not unnecessarily retried by this resumption mode.
3. **Progress after recovery:** if the finite pending corpus remains unchanged and the provider and storage succeed, repeated authorized continuation can bring every pending row into the target space.
4. **Attribution only after computation:** a NULL-stamped row receives `T` only alongside a successful fresh vector computed for the still-current content by the captured engine.
5. **Truthful settlement:** completeness remains based on actual target coverage. An empty selected worklist with positive pending coverage is an unresolved mismatch, not success.

These requirements do not specify a stronger vector-validation contract than currently implemented, guarantee success under permanent provider failure, or settle whether a same-space *forced refresh* needs run-specific per-row receipts. That last question is separate from this missing-attribution predicate and is not claimed solved here.

The host owns this deterministic selection and SQL publication. No model decision, new Mangle predicate, JIT prompt or discretionary retry agent is necessary. Existing kernel authority over user-visible work and the explicit authorization to reembed remain unchanged.

## Bounded implementation options

**Preferred direction:** use an explicit pending selector at the migration boundary rather than bare SQL negation of the read gate. One candidate expression, to validate rather than treat as a shipped patch, is:

```sql
embedding IS NULL
OR embedding_model IS NULL
OR embedding_model != ?
```

Combined with the existing nonblank domain and a nonempty target, this covers the pending states in the table. Parenthesize the disjunction when composing it with the domain filter. Keep the existing read-side predicate and producer-space checks intact.

An alternative is to make the equality inside the negated expression NULL-safe, for example using SQLite's `IS` relation with a bound non-NULL target. Another untested SQLite form is `(embedding IS NOT NULL AND embedding_model = ?) IS NOT TRUE`. Choose one explicit convention and exercise it against the supported database version, rather than scattering ad hoc NULL replacements. [SQLite expression semantics][sqlite-expr]

Normalizing old NULL stamps to an empty string would avoid this particular predicate result but would add a data rewrite solely to compensate for a selector. It is not required for the narrow correction and leaves the general nullable-state assumption fragile. Assigning the live target identity to old bytes is unacceptable: it would weaken the existing safety contract.

No new schema or whole-store shadow generation is required for this repair. Existing incomplete records should resume correctly after the selector changes, preserving their run IDs and completed target rows. Any optional diagnostic should distinguish `pending_rows`, `selected_rows`, `attempted_rows`, `provider_failures` and `published_rows`; the current pass's failure count cannot stand in for all outstanding work. Do not add a mandatory second full-table scan to every batch merely for a counter. A diagnostic count and the worklist must describe the same snapshot or disclose intervening writes.

## Relationship to prior work and literature

The existing embedding representation audit already identified partial migration and proposed durable progress, identity gates and coverage-based settlement. Its as-built update records those protections as implemented and identifies same-target resumption by target-space membership. This report does not reopen the repaired cross-model ranking or batch truncation issues. It isolates a residual defect in the implementation of that resumed membership test: SQL NULL is excluded from both its positive and negative query. The existing `TestForceReembed_ResumesAndCertifies` asserts successful resumption for rows initially stamped by model A; it was inspected, not run here, and does not instantiate a NULL model stamp. [Prior representation audit][prior] [Resume test][resume-test]

The legacy repair audit concerns the lifecycle of automatic recall-triggered healing. The current source now has per-space healing state, and this report does not reuse that document's historical one-shot-latch claim. The explicit force command's SQL worklist and durable run record are a different producer and continuation path. The stats audit concerns diagnostic activation and index work; the pooling audit concerns recursive vector arithmetic. Neither is this selector. [Legacy healing audit][legacy-audit] [Current healing state][heal-state] [Stats audit][stats-audit] [Pooling audit][pool-audit]

Franconi and Tessaris, *Relational Algebra and Calculus with SQL Null Values* (2022), formalize SQL equality and selection in the presence of NULL. Their treatment explains why selection keeps only true conditions and why two-valued set intuition cannot be applied naively to nullable SQL expressions. The application here is our own: the migration's intended pending set must be encoded explicitly, rather than assumed to equal rows selected by `NOT eligibility`. The paper is not evidence of this repository defect or a performance guarantee. [Primary paper, sections 1 and 2][null-paper]

The repository trace supplies the concrete causality; SQLite documentation supplies the deployed operator semantics. The research motivates the partition check without expanding this small repair into a redesign of missing-information representation.

## Full costs and failure modes

Let `N` be the embeddable corpus, `P` the truly pending rows, `K` the selected rows, `S` the selected content and metadata bytes, and `d` the vector dimension. The current selector materializes its selected inventory before processing batches of 32. Its inventory memory scales with `S` plus per-row overhead; a batch of `b ≤ 32` valid d-dimensional results carries `4 × b × d` float32 payload bytes, excluding JSON, metadata and provider buffers. Final coverage aggregates the canonical table. Actual query-plan, allocation and I/O costs must be measured, not inferred from these bounds. [Inventory][inventory] [Force loop][force-entry] [Coverage][identity]

In the one-row retry witness, `P=1` but `K=0`. Later force calls spend no embedding work on that missing row, which is failure to do the job, not an optimization. Repeated CLI invocations still have engine construction, database discovery/opening, schema readiness, engine attachment and possible index rebuild/backfill, work for other tables, inventory and coverage queries, migration-record updates, logging and operator time. Count those costs even when the generic-vector embedded count is zero. [CLI entry][cli-entry] [Database dispatch][dispatch] [Attachment][attach]

A corrected selector increases useful provider work by making omitted rows eligible for attempts. A failed batch may be followed by one per-item call for each selected row, in addition to provider-internal retries. Record all requests and reported input units, not just successful batch count. Restoring previously unserved content may increase semantic-recall coverage; it may also expose bad legacy source content to normal retrieval, so relevance and provenance remain evaluation concerns. No automatic correctness or dollar-saving claim follows.

| Failure or tradeoff | Required handling |
|---|---|
| Provider remains unavailable | Preserve the old bytes and missing stamp; report incomplete, with bounded behavior per authorized invocation. |
| Cancellation during a batch | Keep committed progress, report interruption and retain remaining rows for resumption; do not convert cancellation to completion. |
| Content changes while embedding | Retain the existing conditional publication guard; a computed vector must not attach to different content. |
| Target engine changes or resolves a different name | Preserve producer identity and the existing target transition behavior; measure separately from this stable-target witness. |
| SQL NULL silently changed to empty string in a fixture | Keep both cases. An empty-string-only test does not establish NULL correctness. |
| Selector includes completed rows | Detect duplicate provider work and verify intended progress skipping. |
| Index publication fails | Retain existing error and drift semantics; canonical eligibility is not by itself proof of indexed availability. |
| New observability adds unbounded scans or row content | Prefer aggregate, non-sensitive counters; measure scans, bytes and lock time, and avoid copying source text into logs. |

The narrow pending predicate should not materially change storage representation, but its execution plan and real backlog cost are unmeasured. A future bounded or paged inventory adds checkpoint and snapshot-consistency decisions; it is separate work and should not delay correcting the omitted row class.

## Unrun acceptance and evaluation plan

All cases below are proposed. None was executed for this audit.

1. **Selector truth table.** Populate nonblank rows covering every row in the table above, plus blank content. With a fixed nonempty target, compare the selected IDs against an independently specified expected pending set. Do not construct the oracle by negating the same SQL expression under test.
2. **Primary legacy retry.** Build a pre-stamp database through the existing test's fixture route. Fail both the first batch and its fallback for one row, then restore the provider. The second force pass must actually request that row, publish a fresh vector with the target identity, preserve the run ID and report complete only after pending coverage reaches zero.
3. **Reopen continuation.** Close after the first incomplete result, reopen, attach the same target, and rerun without semantic recall. Require the same recovery. This isolates durable resumption from recall-triggered healing.
4. **Mixed corpus.** Include a completed target row, a foreign-stamped row, an empty-string stamp, a NULL stamp and a missing embedding. Resume must skip completed target work and attempt all pending classes. Verify each row's provider call and content identity, not only the total count.
5. **Repeated failure and later recovery.** Refuse the NULL-stamped row across several explicit invocations, then allow it. It must remain pending, stored and absent from semantic eligibility until fresh computation succeeds. Bound retries by the existing invocation policy, not a new endless internal loop.
6. **Cancellation and publication errors.** Inject cancellation before dispatch, after one batch, and storage errors during publication. Distinguish attempts, durable success, pending coverage and returned error. A run with no selected work and positive pending coverage must not be treated as a successful retry.
7. **Alternative recovery control.** Insert an intervening successful semantic recall and demonstrate that the existing healer can attribute the row. This should break the original witness and confirms the report's boundary rather than contradicting it.
8. **Serving safety.** In indexed and non-indexed builds, retain the foreign/unattributed exclusions before successful computation. After completion, verify expected semantic retrieval and keyword preservation, including fresh reopen. No target stamp may be assigned solely from configuration.
9. **Normal CLI outcome.** Use the production command's test seams and real table dispatch with deterministic engines. First invocation must be incomplete; the recovered second invocation must perform the missing work and become complete. A mocked preconstructed aggregate manifest alone cannot cover the defective selector.
10. **Cost and task evaluation.** Compare pinned behavior, the narrow NULL-safe selector, and an explicit operator-recovery baseline on the same legacy corpus and failure schedule. Measure total invocations to recovery, all provider calls/input units, database and index I/O, peak memory, latency, CPU, logs, unresolved coverage and operator actions. For any claimed task benefit, use normal CodeDOM/world-model-enabled entry paths and measure correctness, relevant evidence recall, model turns/tokens and total cost per verified task.

Predeclare the pass criterion as correct worklist membership plus recovery after the provider and storage become healthy. Do not award the baseline a speed or cost win for doing zero work on still-pending rows. Conversely, do not claim the changed version is cheaper because it succeeds: measure the full cost of recovery and any additional evidence handling.

The source finding would be overturned by evidence that the supported force path converts every eligible legacy NULL stamp before selection, that the pinned query is not the executed inventory, or that the assumed SQL semantics differ in the supported engine. The inspected migration, dispatch and query contradict the first two possibilities; the operator semantics are documented. Only execution can establish reproduction and operational frequency.

## Adoption boundary

First add the pending-domain and continuation contract to the store specification, including the NULL case and CLI retry guarantee. Then implement the narrow selection change, add the deterministic and command-level acceptance coverage, and update the as-built evidence only after those checks run. Keep larger migration-generation, atomic-activation, representation-versioning and index-maintenance questions separate.

Completion means that a failed legacy row remains reachable by the next authorized continuation, is attributed only after successful fresh computation, and can be recovered through the command that tells the operator to retry. An honest incomplete result is necessary, but it does not alone provide a working recovery path.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/3c662caac3c59351983b7a7cb24ac42f853f595b
[root]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/agents.md
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/Docs/architecture/store/IMPLEMENTED_SPEC.md#L185-L218
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/local_core.go#L156-L170
[legacy-migration]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/migrations.go#L61-L65
[legacy-test]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_identity_test.go#L329-L355
[resume-test]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_identity_test.go#L417-L466
[inventory]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store_reembed.go#L130-L163
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_identity.go#L78-L158
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_migration.go#L86-L124
[force-entry]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store_reembed.go#L280-L354
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store_reembed.go#L368-L447
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store_reembed.go#L174-L277
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_migration.go#L126-L170
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store_reembed.go#L454-L465
[cli-entry]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/cmd/nerd/embedding_cmd.go#L165-L227
[cli-result]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/cmd/nerd/embedding_cmd.go#L228-L251
[roots]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/reembed_all.go#L85-L99
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/reembed_all.go#L190-L224
[manifest]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/reembed_all.go#L36-L78
[recall]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store.go#L464-L515
[heal]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_identity.go#L182-L233
[heal-state]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_identity.go#L358-L448
[attach]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store.go#L26-L88
[backfill]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store.go#L1049-L1066
[reconcile]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/internal/store/vector_store.go#L675-L737
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/Docs/audits/embedding-representation-and-migration.md#L39-L69
[legacy-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/Docs/audits/legacy-vector-repair-attempt-lifecycle.md
[stats-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/Docs/audits/embedding-stats-observation-boundary.md
[pool-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3c662caac3c59351983b7a7cb24ac42f853f595b/Docs/audits/embedding-recursive-pooling-weight-contract.md
[sqlite-expr]: https://www.sqlite.org/lang_expr.html
[sqlite-where]: https://www.sqlite.org/lang_select.html#whereclause
[null-paper]: https://arxiv.org/html/2202.10898v1
