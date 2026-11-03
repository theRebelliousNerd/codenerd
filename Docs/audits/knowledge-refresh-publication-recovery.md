---
doc-class: deep-dive
subsystem: knowledge-refresh
implementation-status: proposed
last-verified: 2026-11-02
verified-against: 38a2e5f08ef89bdc914e81755df98523566ed10f
verification-method: static-source-inspection
---

# Knowledge refresh needs an atomic publication and recovery contract

**Decision:** publish replacement knowledge and its acquisition ledger as one storage transition. A successful fetch is not a successfully installed replacement; a retained `acquired` record must not suppress repair after that record's materialization has been removed.

**Evidence boundary:** this is a conditional source-level finding in CodeNERD main `38a2e5f08ef89bdc914e81755df98523566ed10f`, not a reproduced incident or measured reliability/cost result. Inspection covered current guidance, implementation, relevant test definitions and neighboring audits. No application execution, tests, database experiment, model inference, provider request or repository modification was performed. Proposed evaluations below are unrun.

## 1. Residual contract, not the fixed refresh/prune defect

The earlier `knowledge-refresh-prune` audit concerned a successful replacement subsequently deleted by an old, concept-level prune authorization. It explicitly assumed storage and outcome recording succeeded and excluded store failures from that witness. The queue records that issue fixed in `32ad8379d`. Current code captures atom/vector high-water marks before measurement and applies outer pruning only through that measured generation. The regression test specifically combines refresh and old-row prune eligibility and checks that the replacement survives. That repair must remain intact. [Prior scope][prior]; [queue][queue]; [generation capture][measure-generation]; [generation-bounded deletion][generation-delete]; [regression definition][prune-test].

This report concerns an earlier boundary, **inside `knowledge.Refresh` itself**: old materialization is deleted and committed before replacement INSERTs and acquisition-ledger updates. A subsequent storage failure is correctly returned, but the committed deletion is not undone and the old successful ledger can remain. Ordinary `Fill` then trusts that ledger without checking the held materialization. No prune overlap or concurrency is required.

The neighboring index-repair/cache audit concerns semantic-index readiness omitted from compiled-prompt validity, not replacement/ledger atomicity. This report makes no new cache-staleness or model-answer claim. The bounded overlap review does not establish exhaustive novelty. [Index-repair audit][index-audit].

Store guidance requires successful writes to remain recallable after reopening, with explicit ownership and provenance. Init guidance deliberately treats optional research failures as warnings rather than fabricating knowledge. The proposed contract preserves those policies: retain the previous coherent generation on failed publication, expose the failed attempt, and let Mangle continue to choose refresh subjects and budgets. It does not move executive policy into a Go retry loop. [Store guidance][store-guidance]; [init guidance][init-guidance]; [root guidance][root-guidance].

## 2. Real entry points and existing error visibility

The explicit `nerd knowledge refresh` command boots the workspace Cortex, calls `RefreshKnowledge`, prints its report, and **returns its error**. This is a policy-selected, budgeted refresh, not an unconditional “force every source” command. The same refresher runs on the Cortex lifetime when enabled, using the session's configured search clients, Context7 tool, pinned documentation and repository import knowledge. Stock policy schedules an initial run after 120 seconds, then hourly, with a four-subject budget. [CLI][cli]; [boot wiring][boot]; [policy][policy].

The system run obtains ready roster agents and their KB paths, measures wanted subjects and held knowledge, derives a plan, opens each selected KB, and calls `knowledge.Refresh`. It collects storage errors and returns a non-nil aggregate error; scheduled execution logs failures. Report entries in `Refreshed` enumerate selected subjects even when the corresponding call failed, so that list alone is not a success receipt. **The first failing refresh in this report is not silent success.** [Run and error aggregation][run]; [scheduled handling][serve].

Ordinary acquisition also has a production caller: initializer agent creation/upgrading calls `createAgentKnowledgeBase`; when research is enabled and subjects remain after its coverage filtering, that method calls `knowledge.Fill`. It records returned errors in research notes, supports backfill, and reports final atom counts. The later suppression described here is conditional on the same subject reaching `Fill`; upgrades that filter it out do not exercise that call. Empty-KB notes and `check-kb` may reveal missing materialization; neither is asserted universally clean. The narrow issue is that a healthy `Fill` can skip this particular source/subject without a new failed outcome. [Agent creation][agent-create]; [initializer acquisition][init-fill]; [KB inspection][check-kb].

Current admission protections are substantial: one Cortex-owned slot serializes explicit and scheduled refreshes, waiting respects context cancellation, and mission suspension closes admission, cancels and joins an active run. Boot can suppress background refresh. These protections bound ownership and prevent unwanted overlap, but do not roll back an already committed deletion inside a run. They are not replaced by this proposal. [Admission and suspension][admission]; [boot wiring][boot].

## 3. The actual storage boundaries

`Fill` and `Refresh` share `fill`, differing in the `again` flag. `Fill` builds a skip map from prior `StatusAcquired` records; `Refresh` does not. Existing atoms are read to build content-deduplication maps, **not** to validate acquired-source coverage. Acquisition produces atoms and outcomes before replacement mutation begins. For each subject, the implementation has these boundaries: [Acquisition and skip][acquire]; [replacement sequence][fill].

| Stage | Current operation | Consequence if a later stage fails |
|---|---|---|
| Read/fetch | Read prior outcomes and atoms; call applicable sources | A source error or empty answer does not authorize its old-row replacement. |
| Remove old materialization | Collect prior concepts for sources whose new outcome is `acquired`; call `DeleteKnowledgeAtomsByConcept` | The helper commits deletion of matching atoms and canonical knowledge vectors before returning. |
| Install atoms | Loop through nonduplicate atoms; call `StoreKnowledgeAtomWithEmbedding` | Each canonical atom uses a separate `s.db.Exec` INSERT, not the deletion transaction. An error returns immediately. |
| Publish outcomes | Upsert each source/subject ledger with a separate `db.ExecContext` | Earlier materialization changes are not rolled back by a ledger error. |
| Return outcomes | Append this subject's outcomes only after its ledger writes finish | Early failure can return earlier subjects' work but omit this subject's successful-fetch outcome. |

The deletion helper owns `s.mu`, starts a transaction, deletes from `knowledge_atoms` and matching `vectors`, commits, then performs best-effort `vec_index` ghost cleanup when applicable. The subsequent atom operation reacquires the store lock and executes outside that transaction. A per-handle mutex is not a multi-operation commit boundary. [Deletion transaction][delete]; [canonical atom INSERT][atom-insert].

Embedding errors are a different case. `StoreKnowledgeAtomWithEmbedding` first stores the canonical atom; only an error from that step is returned by this path. A later vector/embedding failure increments an embedding-failure counter and logs a warning, while the method returns nil. Therefore **an embedding-provider failure is not the first-INSERT-failure witness**. Nor should context cancellation be used as an automatic substitute: the canonical INSERT uses `Exec`, while later ledger writes use `ExecContext`. [Embedding behavior][embedding].

SQLite supplies atomicity for the transactions the application actually groups. An individual statement outside an explicit transaction gets an implicit transaction; an error in a later transaction cannot undo a previously committed deletion. This is an application-level publication gap, not a claim that SQLite violates atomicity. New readers can observe committed intermediate states; a reader already in a snapshot may retain its prior view. [SQLite transaction semantics](https://www.sqlite.org/lang_transaction.html); [SQLite isolation](https://www.sqlite.org/isolation.html).

## 4. A precise conditional witness

Use a ready agent with one wanted Context7 topic and a valid prior acquired record for that exact source/subject. Its recorded concepts identify existing old atoms. For the system route, let that outcome be at least 91 whole days old, the subject selected within the budget, and Context7 wired. Keep aggregate usage below the prune threshold, or otherwise make the old concepts ineligible for outer pruning. Exclude concurrent writers, overlapping concept ownership and unrelated source failures from the minimal witness. Other production sources may still run; they do not need to be disabled to establish the affected pair's suppression. Context7 can legitimately return nonempty replacement paragraphs under the same topic/section concept keys. [Context7 producer][context7]; [age measurement][measure]; [stale/prune policy][policy].

1. Acquisition of the selected subject succeeds and yields replacement atoms for the affected source. Its temporary outcome is `acquired`.
2. `DeleteKnowledgeAtomsByConcept` successfully commits removal of the prior concepts and their canonical knowledge vectors.
3. The **first canonical atom INSERT attempted for this subject fails**. This is the explicit fault assumption; source inspection does not establish its runtime incidence. The function returns a `store <concept>` error before any ledger UPSERT for this subject.
4. The old `knowledge_acquisitions` row is unchanged: status, recorded count, concepts, version and attempted timestamp still describe the previous acquisition. The old atoms are gone, and no replacement atom from this subject has been installed in this minimal case.
5. Restore healthy storage and call ordinary `Fill` with the same source name and subject key. It adds that pair to the acquired skip map and does not call that source. The preliminary atom scan does not invalidate the ledger. In a one-source/one-subject fixture, this can return zero additions, no new outcomes and nil error. With the normal source set, other sources may run and succeed or fail independently.

Step 3 is visibly failed refresh; step 5 is the residual **false completion/suppression condition for one acquisition pair**, not proof that the CLI declared the entire workspace current. Closing and reopening the DB does not itself reconstruct the removed rows or alter the acquired ledger. A normal restart is consequently not a sufficient repair contract.

No model failure follows necessarily. Other sources, unrelated atoms, tools, retained in-memory context or later successful refreshes may compensate. The directly supported consequence is a mismatch between authoritative acquisition state and canonical materialization, followed by an ordinary acquisition path that does not attempt that pair.

## 5. Recovery and failure boundaries

`Refresh` sets `again=true`, bypassing ordinary acquired suppression. In the concrete stale-Context7 witness, the old attempted timestamp remains stale; another scheduled or explicit policy run can select the subject and restore it once acquisition and storage succeed. Selection still depends on the roster, wanted subject, budget, service lifetime and policy. This is **not** a permanent automatic-retry blackout. A version-change reason can likewise remain. However, the failed storage operation never records a new `failed` acquisition outcome, so the policy's failed-source retry rule is not the repair signal created by this failure. [Fill/Refresh distinction][fill]; [refresh reasons][policy].

Important controls and adjacent outcomes:

- **Source failure or empty response before mutation:** existing rows for that source are retained, and the unsuccessful outcome keeps previous concept provenance. Existing tests cover source-fetch failure retention. Preserve it.
- **Failure inside the deletion transaction's body:** the helper attempts rollback and returns an error; this is not the successful-delete witness. Begin and commit errors are also returned and require their own failure handling.
- **Failure on a later atom INSERT:** already committed replacement atoms can form a prefix while the ledger remains old. The first-insert case deliberately avoids relying on prefix interpretation.
- **Ledger UPSERT failure:** replacement atoms may already exist, while some or all ledger rows remain old. Multiple outcomes and subjects require explicit partial-success reporting rather than a whole-run fiction.
- **Embedding failure after canonical storage:** atom presence and semantic-index readiness differ; retain current observable degradation rather than relabeling every embedding failure as total acquisition failure.
- **Outer pruning after a failed refresh:** the run still processes its prune plan. Generation fencing protects new rows, not old rows that policy independently authorized for removal. The witness excludes that condition; atomic replacement alone does not promise indefinite fallback retention against deliberate pruning.

The inspected `TestRefreshReplacesWhatTheLastAcquisitionWrote` verifies successful replacement, ordinary acquired skipping and provider-failure retention. `TestFillRecordsProvenanceAndOutcomes` verifies retry of failed/unavailable sources. These definitions are useful existing contracts, but neither supplies the composed storage-failure/reopen/ordinary-Fill assertion here. They were read, not run. [Refresh tests][refresh-tests]; [Fill tests][fill-tests].

## 6. Proposed publication and recovery contract

**One chosen publication unit must contain the old-generation retirement, canonical replacement rows and the acquired ledger identifying those rows.** Define whether that unit is one source/subject or all successful sources for one subject. Per-source units allow unaffected sources to publish; per-subject units provide a stronger joint snapshot but roll back more useful work. Neither should imply whole-agent or whole-run atomicity unless implemented deliberately.

1. **Acquire and validate outside the write transaction.** Do not hold SQLite's writer reservation across web search, documentation extraction, model calls or embedding RPCs. Stage bounded validated atoms and, if required, computed embeddings; retain provenance and cancellation information.
2. **Revalidate before publication.** Compare the expected active generation/ownership with the current row state inside the transaction. The per-Cortex admission slot is not proof that every independent store handle or process is excluded. A generation conflict needs bounded retry or a visible superseded result, not blind replacement of newer knowledge.
3. **Publish through a store-owned transactional API.** Delete the authorized old rows, insert canonical replacements and update the matching acquisition-generation record using the same `sql.Tx`; commit once, otherwise roll back. Merely wrapping today's helpers in an outer transaction is insufficient: the deletion helper opens its own transaction and INSERT helpers call `sql.DB`. Go explicitly warns that nontransaction DB methods execute outside a `Tx`. [Go transaction guidance](https://go.dev/doc/database/execute-transactions).
4. **Separate active materialization from the latest attempt.** A failed attempt should be visible without erasing the identity of the still-active old generation. If the database is too unhealthy to record failure, error delivery must remain explicit; the rollback-preserved active state must not depend on a second compensating write succeeding. Do not “fix” the problem by marking acquired before writing or by simply deleting the old ledger first.
5. **State index semantics honestly.** Include canonical vectors in the publication unit when feasible, but define index/readiness repair separately where a virtual index or external operation cannot participate. A coherent lexical generation plus a truthful degraded semantic state is different from a promise of fully indexed knowledge. Preserve embedding-space checks, source invalidation and existing generation-scoped pruning.
6. **Reconcile legacy inconsistencies narrowly.** An acquired generation should carry enough ownership/identity information to detect incomplete materialization and authorize repair. Avoid treating any nonempty concept or an old raw atom count as proof of complete current coverage: deduplication, legitimate pruning and source changes complicate that inference. Keep retry policy, backoff and budgets explicit; do not reacquire every successful source on every init.

Committed database atomicity does not refund earlier provider/model cost, make external side effects transactional, or by itself prove power-loss durability. Durability depends on SQLite configuration, filesystem and storage behavior; that is a separate contract requiring separate evidence. [SQLite atomic-commit assumptions](https://www.sqlite.org/atomiccommit.html).

## 7. Costs and fair evaluation

There is no measured saving or reliability rate here. A grouped publication may reduce commit overhead, but a larger transaction occupies SQLite's single writer for longer and can increase contention, rollback work, temporary journal/WAL space and recovery pressure. WAL supports reader/writer overlap, not concurrent writers; long-lived readers can delay checkpoint progress. Do not claim a net latency win from fewer commits alone. [SQLite WAL concurrency](https://www.sqlite.org/wal.html#concurrency).

Staging adds bounded memory or temporary-file I/O; precomputed embeddings may be wasted on cancellation or generation conflict. A separate active-generation/attempt ledger requires migration, indexes, diagnostics and retention decisions. Failure retries consume external search/embedding capacity, tokens, money and wall time even if no row commits. Retaining old knowledge needs truthful freshness labeling and a policy for stale fallback. Stronger publication changes cache/source-generation behavior and may add invalidation churn; none of that is free. Cancellation and commit handling also need a defined “committed, rolled back, or needs reconciliation” result rather than a generic success counter.

All evaluations proposed below are **unrun**. Compare baseline with the same-source publication repair and, if desired, per-subject publication under matched source output, atom sizes, database pragmas, index mode, embedding configuration, retry policy and external-call budget. Use deterministic sources, not live model variability, for the contract test.

| Evaluation | Required observation |
|---|---|
| First replacement INSERT denied after old deletion succeeds | Capture returned error, reopened canonical atoms/vectors and the full acquisition row; remove fault, call `Fill`, count source calls, then call `Refresh` and observe recovery. A temporary fixture-only trigger using `RAISE(ABORT, ...)` can isolate this boundary; it is not an allegation about production triggers. [SQLite trigger semantics](https://www.sqlite.org/lang_createtrigger.html#the_raise_function). |
| Failure at later INSERT, ledger UPSERT and commit | Identify which publication units survive; verify no ledger certifies an unpublished generation and no counter treats rolled-back rows as committed. |
| Reader between retirement and installation | Inspect fresh reads and existing snapshots separately; repaired readers should see a coherent old or new publication, not an inter-commit absence. |
| Real explicit/scheduled caller | Verify CLI non-nil error and scheduled warning on failure; distinguish selected/refreshed labels, committed additions, per-source outcomes and degraded index state. |
| Healthy and existing-protection controls | Unchanged/changed successful answers; source error/empty/unavailable; nil engine and embedding error; duplicate atoms; old-generation prune overlap; admission suspension, cancellation and reopen. |
| Recovery and contention | Same versus different source/subject; remaining source success; stale/version policy selection; budget displacement; another writer changes generation; slow source outside the publication lock. |

Record acquisition/embedding calls, bytes staged, SQL statements/commits, writer wait and hold times, busy errors, retries, CPU, memory, disk/WAL growth, checkpoint work, p50/p95 latency and recovery time. Include failed attempts and rolled-back work in denominators. If later evaluating downstream model quality, hold task, retrieval/prompt limits and provider configuration constant, report verification success and total model/tool cost, and separate that experiment from the storage invariant.

**Acceptance:** failed publication leaves a coherent prior active generation or an explicitly incomplete state that is eligible for repair; successful publication couples the new canonical materialization to its acquisition identity; reopening and ordinary `Fill` cannot silently accept this deletion-induced mismatch. Existing source-failure retention, budgeted policy, mission ownership and generation-scoped pruning continue to hold.

[prior]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/Docs/audits/knowledge-refresh-prune.md#L30-L65
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/Docs/audits/ASTRA_QUEUE.md#L46-L56
[measure-generation]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/system/knowledge_refresh.go#L260-L273
[generation-delete]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/store/local_knowledge_delete.go#L40-L95
[prune-test]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/system/knowledge_refresh_prune_test.go#L16-L94
[index-audit]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/Docs/audits/knowledge-index-repair-cache-freshness.md#L3-L9
[store-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/store/agents.md
[init-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/init/agents.md
[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/agents.md
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/cmd/nerd/cmd_knowledge.go#L49-L91
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/system/knowledge_refresh_boot.go#L20-L71
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/core/defaults/policy/knowledge_refresh.mg#L11-L123
[run]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/system/knowledge_refresh.go#L70-L208
[serve]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/system/knowledge_refresh.go#L401-L424
[agent-create]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/init/agents.go#L530-L565
[init-fill]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/init/agents_knowledge.go#L35-L172
[check-kb]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/cmd/nerd/cmd_check_kb.go#L60-L155
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/system/knowledge_refresh_scope.go#L30-L144
[acquire]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/knowledge/acquire.go#L159-L190
[fill]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/knowledge/acquire.go#L193-L322
[delete]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/store/local_knowledge_delete.go#L20-L151
[atom-insert]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/store/local_knowledge.go#L48-L72
[embedding]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/store/local_knowledge.go#L384-L424
[context7]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/knowledge/sources.go#L115-L145
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/knowledge/measure.go#L26-L82
[refresh-tests]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/knowledge/refresh_test.go#L35-L68
[fill-tests]: https://github.com/theRebelliousNerd/codenerd/blob/38a2e5f08ef89bdc914e81755df98523566ed10f/internal/knowledge/knowledge_test.go#L42-L83
