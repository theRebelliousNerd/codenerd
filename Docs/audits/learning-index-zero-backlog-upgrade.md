# Learning-index upgrades must not depend on embedding backlog

## Finding and decision

**An upgraded learning database can have every canonical embedding current while its old semantic index remains unusable indefinitely.** Current recall requires the new version columns in `learnings_vec`. A migration helper correctly rebuilds the old index from canonical vectors, but ordinary production reaches that helper only while synchronizing a nonempty batch of embedding updates. A healthy, quiet, zero-backlog shard never takes that path.

This is a **migration-reachability residual of the repaired learning-identity boundary**, not a reason to weaken that boundary. The old index must not be ranked under metadata it never stored. The narrow correction is to make schema readiness an independently owned obligation, reachable without inventing fresh learning work or paying to re-embed already-current content.

Source pin: main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), revalidated October 30, 2026 at 14:47 UTC. Static review covered guidance, active branches and research additions through [`research-main-20261030-1417`](https://github.com/theRebelliousNerd/codenerd/commit/6d97defb8901c0320d7561d4f15d4c13ee799bc9); an independent source challenge corroborated the legacy producer and zero-backlog path. No application, SQL experiment, tests, model inference or repository changes were performed.

## Credit the existing repair and its limits

The [JIT learning semantic-identity audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/jit-learning-semantic-identity-admission.md) owns query/row compatibility and the requirement that admission bind to the actual index bytes. Current code implements important protections: the query carries its producing model; ranking filters canonical model, dimension and document task; index model, task and handle hash must agree with the canonical row. These controls prevent stale or foreign index bytes from masquerading as current. Preserve them. [Typed query and aggregation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_search.go#L172-L237), [indexed eligibility](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_search.go#L332-L371).

The [embedding representation/migration audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/embedding-representation-and-migration.md) already owns restart-safe migration, representation compatibility and honest completion. This report does not claim those general principles as new. Its distinct witness needs no model or dimension change, provider failure, index-write failure, corruption or concurrent mutation: an ordinary pre-versioning database is reopened with the same healthy engine and a canonical backlog of zero.

The current implementation and test explicitly provide a legacy-index upgrade. The test creates the old two-column virtual table, writes canonical bytes, then **directly calls** `ensureLearningVecTable` before recall. It demonstrates the migration helper and subsequent safe reader, not that reopening or the ordinary worker reaches the helper when there is no embedding work. The test's deliberate stale-index bytes are not needed for this report's healthy legacy witness. [Existing regression](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/learning_vec_version_test.go#L7-L63).

## Source chain

### 1. The old table is a real persisted production shape

At earlier production pin `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, ordinary learning synchronization created:

```sql
CREATE VIRTUAL TABLE learnings_vec
USING vec0(embedding float[D], learning_id INTEGER)
```

It inserted the learning ID and vector into that table. Canonical `learnings` rows separately carried the handle/version/hash and embedding model/dimension/task. This is not an arbitrary unsupported schema manufactured solely for a counterexample. [Historical normal producer](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/reflection_worker.go#L581-L641).

The handle version remains **1** at both pins. A previously completed, same-engine corpus therefore need not acquire a descriptor-migration backlog just because the binary upgraded. [Historical version](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/reflection_utils.go#L10-L14), [current version](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_utils.go#L10-L14).

### 2. Normal reopening does not migrate this virtual table

`getDB` opens the shard database, initializes the canonical schema, runs migrations, and ensures learning indexes. The canonical schema uses `CREATE TABLE IF NOT EXISTS`. The migration list adds canonical learning columns; it does not describe the new `learnings_vec` columns. The additional learning-index helper ensures only the canonical `handle_hash` index. None of these steps calls the legacy virtual-table migration. [Database opening](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/learning.go#L115-L185), [canonical-column migrations](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/migrations.go#L84-L129), [learning index ensure](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/learning_reflection.go#L41-L48).

This is not a failure to discover databases on disk. `listShardTypes` combines open handles with `*_learnings.db` files and sorts the result. A reopened coder shard is available to both the worker and recall. [Shard discovery](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_search.go#L476-L524).

### 3. The worker's admission question is only about canonical embedding work

Normal factory boot attaches the configured healthy engine and reflection configuration to the LearningStore. The worker runs on a 45-second ticker, with no immediate cycle; explicit disablement stops it. Waiting for a tick is necessary for ordinary refresh, but is not sufficient for this upgrade. [Factory wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1728-L1771), [worker lifecycle](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_worker.go#L353-L438), [default configuration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/reflection.go#L48-L57).

For each shard, the cycle captures the engine's expected model, dimension and document task, counts canonical backlog, and requests candidates. Candidate SQL examines missing/outdated handles and missing/incompatible canonical embeddings. Neither the candidate predicate nor the backlog count inspects the virtual-table schema or index coverage. [Candidate predicate](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/learning_reflection.go#L50-L87), [backlog predicate](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/learning_reflection.go#L192-L226).

When the candidate list is empty, the cycle continues to the next shard. Only after producing nonempty updates and successfully committing them does it call `syncLearningVectorIndex`. Thus the affected shard can be revisited indefinitely without schema readiness being reconsidered. [Actual worker branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_worker.go#L441-L499).

### 4. The right helper exists behind that skipped branch

`syncLearningVectorIndex` calls `ensureLearningVecTable`. For an existing index, the helper reads its DDL and looks for the version columns. An unversioned table is rebuilt using canonical vectors of the selected dimension, stamped with their own model, task and handle hash; it does not call an embedding engine. [Sync entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_worker.go#L595-L611), [legacy detection and rebuild](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_worker.go#L652-L715).

The inspected production call chain does not invoke that ensure independently at store opening, on an empty worker pass, or before a semantic read. The helper is not absent or ineffective; its trigger is coupled to a different kind of work.

### 5. The new reader cannot use the unupgraded table

The reader first checks that a table named `learnings_vec` exists, then prepares a query referencing `v.embedding_model_id`, `v.embedding_task` and `v.handle_hash`. Those columns do not exist in the old table. The shard-level function logs the SQL failure and returns an error; it does not fall back to ranking the old unversioned bytes. That fail-closed behavior is correct. [Reader and error path](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_search.go#L332-L397).

The aggregate recall method skips non-cancellation shard errors. If no other shard contributes hits, it returns `nil, nil`; with healthy shards, it can return only their hits. Hence **a successful zero-result aggregate is possible**, not an assertion that every caller sees an error or performs a specific fallback. The per-shard log remains evidence of degradation. [Aggregation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_search.go#L213-L237).

## Controlled upgrade witness

Use a workspace with one affected learning shard:

1. Under the historical normal producer, save a valid short learning and complete reflection so its canonical handle, version, hash and embedding identity are current, and its old-format index contains the corresponding vector.
2. Close normally and upgrade to current main. Keep the same resolved engine name, actual dimension and task mapping; keep SQLite vector support available. No new or outdated learning candidates exist in this shard.
3. Reopen through ordinary factory-equivalent wiring with reflection enabled. Allow one or several worker cycles to finish. Candidate selection stays empty; the old index remains unchanged.
4. Issue a normal semantic learning query from that same engine. Its index query cannot prepare against the old schema. In the single-shard case the aggregate can return an empty result without an error, while logging the shard failure.
5. For a consumer-level omission, use a controlled query/vector pair that would retrieve the canonical learning after migration, but whose lexical query does not match it. On a fresh JIT collection, that learning cannot become a semantic retrieved candidate through this shard. Claim prompt inclusion after repair only when ordinary Mangle selection and whole-atom budget checks admit it.

The fifth step is a controlled evaluation condition, not a claim that natural-language synonym queries always have a particular cosine or that lexical retrieval universally fails.

## Controls, recovery and bounded impact

- **Lexical access remains.** JIT always runs a separate lexical learning search before merging semantic hits, with per-source limits and `(shard, ID)` deduplication. Chat reflection uses lexical learning retrieval when its ranked semantic learning list is empty. A lexical match can fully recover the needed fact. [JIT collection and hydration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L494-L569), [bounded merge](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L572-L594), [chat fallback](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/reflection.go#L91-L117).
- **Fresh and already-versioned indexes are not this witness.** Missing-table, unavailable-extension, canceled-query and foreign-model cases have separate behavior. Other healthy shards can still contribute semantic hits. Canonical learning facts and vectors remain stored.
- **New work can incidentally repair the affected shard.** Any later candidate batch that produces nonempty updates and commits successfully reaches index synchronization and its legacy upgrade. This can include a new learning; a candidate in another shard does not repair this one. Therefore the gap persists while the shard remains quiet/current, not unconditionally forever.
- **Explicit recovery exists.** A successful `ReembedAllLearningsForce` rebuilds the index before its force batches. The broader embedding maintenance command can use that route. It is more work than schema-only reconstruction because it requests fresh vectors for all candidates; success still depends on actual provider/storage outcomes. [Force path](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reflection_reembed.go#L82-L151).
- **No unsafe admission is proposed.** Do not remove the version checks, infer missing index metadata, or label old bytes with current stamps. This is availability/migration wiring, not a permission or fact-truth issue.
- **No blanket observability claim.** Errors are logged at the shard. However, the aggregate's nil error means the JIT collector's error-based degradation branch need not mark this particular loss. That is a qualification of the current result, not a separate claim that every diagnostic is silent.

All five active code/QA tips were inspected. `767a3291d6287b238417f2466592a9002810d05a` has current `reflection_worker.go` and `reflection_search.go` byte-identical. The other four inspected tips—`51cfd426`, `a53cfc30`, `80af1863`, `aa7988de`—predate the versioned reader in these files, so this exact upgrade regression must not be attributed to them. The recent 1417 documents concern campaign scope deduplication, correction ownership, manual alignment and working-context NUL recall, not this migration trigger.

## Repair contract and complete costs

Separate **canonical embedding freshness** from **index schema/readiness**. Check and own a legacy-index upgrade at an appropriate ordinary lifecycle boundary even when canonical embedding backlog is zero. A per-shard startup or bounded lazy-read migration is plausible; the choice must specify who owns it, how concurrent openers/readers coordinate, and what callers receive while readiness is incomplete.

Reuse canonical vectors and their recorded identity rather than invoke the model just to add index metadata. Do not blindly copy the existing destructive helper into every query: its current reconstruction drops/recreates the table, reads canonical rows into an updates slice, and repopulates it. A production integration needs an idempotent successful-generation marker, serialized migration or equivalent fencing, bounded cancellation/retry behavior and a defined crash/partial-rebuild outcome. An existing versioned table is not a license to skip independent coverage repair after an interrupted rebuild.

For N eligible canonical rows of dimension D, reconstruction reads and writes vector payload proportional to N×D plus metadata, and incurs SQL/index/journal work. The current helper can retain the whole reconstruction slice. Wiring it into startup moves work onto boot; wiring it into first recall moves work onto query latency. Chunking, staging or resumable publication can bound peak work but adds state and recovery complexity. No provider call is inherently needed for the same-space healthy fixture, but actual wall time, allocation, disk traffic and task benefit remain unmeasured.

Repeated attempted retrieval can pay query-embedding and SQL work without gaining this shard's semantic evidence. Do not turn that into a per-turn request count: compiler caches, query reuse, provider batching/retries and other useful consumers can change actual cost. A repaired semantic hit may add prompt tokens and influence later tool/model work; fewer tokens are not automatically better.

## Unrun evaluation

1. Construct the old schema through a legacy-compatible fixture with complete current canonical rows; close and reopen through ordinary store/factory wiring. Run the actual worker cycle with **zero candidates**, not a direct call to the migration helper. Assert schema readiness and successful same-space semantic recall.
2. Keep an explicit helper-level test, but pair it with the lifecycle test above. Verify no document embedding calls are required to reconstruct unchanged canonical vectors. Use a recording engine, not live inference.
3. Cover empty stores, fresh/versioned indexes, multiple shards, lexical-only hits, disabled reflection, same-space restarts, concurrent openers, cancellation, interrupted publication, and genuine storage/extension failures. Preserve wrong-space rejection and the stale-index-byte test. Treat model/dimension migration as a separately specified policy, not an assumed promise of this repair.
4. Traverse fresh JIT collection and ordinary chat reflection with controlled semantic-only and lexical-positive cases. Verify candidate/provenance preservation, truthful degraded/partial outcomes, and actual prompt inclusion only under normal policy and budgets.
5. Measure startup/first-recall delay, schema checks, canonical/vector bytes, SQL writes, allocation peak, migration retries, embedding calls and complete prompt cost. Pair normal-entry tasks at identical authority, models, context ceilings and source snapshots; evaluate recovered relevant evidence, verified outcomes, repair turns and end-to-end cost. Include failures and no-upgrade controls.

**Acceptance:** a healthy legacy learning index can reach the already-implemented safe format through normal lifecycle wiring without waiting for unrelated embedding work, while unversioned or stale bytes remain ineligible until that readiness is actually established.

