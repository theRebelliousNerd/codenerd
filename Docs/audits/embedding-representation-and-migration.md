---
doc-class: research-proposal
subsystem: embedding-store-prompt
implementation-status: planned
last-verified: 2026-10-28
verified-against: 556f9fe092d0745248414ebdf2f6fe0db4d93dd5
verification-method: static-source-inspection
---

# Embedding representation contracts and restart-safe migration

**Status:** source-inspected research and proposed design, not an implementation or measured result.  
**Source pin:** main at `556f9fe092d0745248414ebdf2f6fe0db4d93dd5`, checked on 2026-10-28 at approximately 06:15 UTC. The substantive source inspection used `35ede84560a179897ed9627493d872fe96db126f`; the [comparison to the final pin](https://github.com/theRebelliousNerd/codenerd/compare/35ede84560a179897ed9627493d872fe96db126f...556f9fe092d0745248414ebdf2f6fe0db4d93dd5) changes orientation-related files, not the embedding, store, or prompt sources cited here.  
**Scope:** embedding construction → effective input representation → durable vectors → query eligibility → reembedding and repair.  
**Evidence limit:** no code or tests were executed, no provider was called, and no runtime incident or performance improvement is claimed. An independent source-level review confirmed the generic LocalStore compatibility gap and supplied the timing/scope qualifications incorporated below. That review is not behavioral validation or acceptance of every proposed design choice.

## 1. Decision

Finish embedding as a trustworthy semantic-transduction boundary: a vector must identify **which transformation produced it, which exact input it represents, and which queries may compare against it**. Changing configuration or completing some embedding calls must not make incompatible or incomplete representations eligible.

The actionable capability is not “reconnect the old initialization functions.” Context-aware initialization already reaches production consumers. The missing contract is the continuity between provider identity, representation fidelity, durable publication, and read eligibility.

Three source-supported findings justify this proposal:

1. Ordinary `LocalStore.vectors` rows have no authoritative model/task/representation identity. Equal-dimensional vectors from unrelated models remain eligible.
2. Force reembedding preserves failed rows’ old vectors. They are ordinarily absent from the cleared sqlite-vec index immediately after that force pass, but same-dimensional old vectors remain eligible for brute-force retrieval and can be restored by dimension-only reconciliation or later engine attachment.
3. Native Ollama batching omits the provider's truncation control. The documented default permits truncation, so successful batch processing need not enter the single-input split-and-pool path for an oversized item.

These findings are static deductions with designed counterexamples, not reproduced failures.

Keep the provider package a semantic leaf. Go performs bounded IO, numerical validation, identity checks, and storage operations. Mangle continues deciding context relevance, fallback disclosure, task obligations, and authorization. Similarity never grants permission. This extends existing mechanisms rather than adding another retrieval architecture.

## 2. Current wiring and already-retained capabilities

### 2.1 Do not restore contextless wrappers to make reachability green

The current [dead-code baseline](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/scripts/testdata/deadcode-baseline.txt) no longer lists embedding initialization functions.

The live system factory calls `embedding.NewEngineWithContext`, checks health, and attaches the resulting engine to LocalStore, LearningStore, and the prompt loader. The constructor inherits caller cancellation; its optional startup ensure cap does not become a whole-task deadline. [Factory](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/system/factory.go#L1684-L1727), [constructor](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/engine.go#L101-L168).

The existing [perception capability recovery research](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/Docs/audits/perception-capability-recovery.md) records the October 28 disposition: unused contextless constructors were deleted or moved to test seams; production retains context-aware construction. Workspace-owned classifiers and turn-scoped semantic receipts remain explicitly deferred. That proposal remains authoritative for classifier ownership; this document does not reopen its multi-workspace design decision.

The [embedding scoped guidance](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/agents.md) likewise identifies caller-aware construction and lifecycle ownership as requirements. Preserve those guarantees while refining representation identity. A model namespace repair must not regress bootstrap cancellation or substitute another model when the configured one fails.

### 2.2 Native batching and model checks already exist

Do not propose adding Ollama native batching: it is already implemented in bounded chunks with a per-item fallback. Embedding architecture prose claiming batching is sequential is stale relative to [current implementation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/ollama.go#L278-L392).

Do not describe all stored embeddings as unstamped. Prompt atoms already reject unstamped or differently stamped model names before comparison. Learnings and reasoning traces have separate model/dimension/task identity fields and separate migration/recall code. They require their own qualification, not extrapolation from the generic vectors table. [Prompt admission](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/prompt/vector_searcher.go#L68-L137), [learning fields](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/learning.go#L25-L38), [trace schema fields](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/local_core.go#L295-L304).

The repository already encountered equal-dimensional model mismatch in prompt atoms. Its [September 17 corrective commit](https://github.com/theRebelliousNerd/codenerd/commit/71748955cff2d464fcfefc0488bb8b20736b919e) documents the motivation for stamping and filtering. This is historical rationale, not a measurement repeated by this research.

## 3. Source-supported deficiencies

### G1 — Equal dimensions do not establish an embedding space

Ordinary vectors persist `id`, `content`, `embedding`, `metadata`, and creation time. The writer does not authoritatively stamp provider/model/task identity. Caller metadata may contain arbitrary fields, but the shared writer and reader do not enforce an embedding-identity contract through those fields. [Schema](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/local_core.go#L143-L155), [writer](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store.go#L125-L203).

`SetEmbeddingEngine` recreates the vector index using the new engine's dimensions. Backfill reads every non-null vector and admits it when parsing succeeds and its length matches. Brute-force recall compares stored vectors without a model-identity filter. [Attachment](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store.go#L26-L79), [backfill](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store.go#L943-L1022), [brute-force path](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store_bruteforce.go#L16-L95).

**Inference:** switching from model A to different model B with the same output dimension can leave A's vectors eligible for B's queries. No provider outage or malformed vector is needed. The dimension check correctly prevents some incompatible comparisons, but cannot identify equal-dimensional incompatible spaces.

This is a generic LocalStore finding. It does not establish that every semantic consumer is vulnerable or that a production session has already returned a wrong neighbor.

### G2 — Partial reembedding is not a completed migration

Force reembedding clears the index, embeds and commits batches, and leaves failed or invalid items at their previous embeddings. It can return a nil error despite per-item embedding or serialization failures after partial success; if no embedding succeeds and a fallback error was recorded, that error is returned. This does not mean every failure is swallowed: batch-publication SQL, index-update, and commit errors propagate, although the initial index-clear failure is logged and execution continues. [Force loop](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store_reembed.go#L235-L379), [batch publication](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store_reembed.go#L152-L229).

The timing distinction matters:

- After a successful index clear, a failed row normally remains in the canonical `vectors` table but is absent from `vec_index`.
- Brute-force recall can still rank that old vector when its dimensions match the query; dimensional mismatch is rejected by cosine comparison.
- Reconciliation selects missing index rows by valid vector JSON, matching dimension, and absent row ID—not embedding-space identity.
- A later engine attachment similarly backfills same-length persisted vectors.

[Reconciliation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store.go#L619-L715).

**Counterexample to verify later:** store A-space rows, switch to equal-dimensional B, successfully reembed one row while another fails, then run reconciliation or reopen and attach B. The retained A-space row can qualify for reinsertion. Do not misreport this as an assertion that failed rows necessarily remain in the index immediately after force reembedding.

The all-database orchestrator has useful completion counts and a skipped list, but no durable target-generation manifest or completeness certificate. Its final nil error is not proof that every relevant row migrated. [Orchestrator](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/reembed_all.go#L16-L133). The CLI prints counts and skipped/errored databases, so the problem is not the total absence of diagnostics; it is the lack of a machine-checkable migration completion contract. [CLI result handling](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/embedding_cmd.go#L185-L205).

### G3 — Native batch success may hide loss of input coverage

Single Ollama embeddings use `embedLong`: refused oversized inputs are split and length-weighted vectors are pooled and normalized. [Single path](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/ollama.go#L95-L105), [splitting/pooling](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/long_input.go#L52-L82).

For multiple items, native `/api/embed` success bypasses that path. Its request type contains model, inputs, and keep-alive, but no truncation field. [Dispatch](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/ollama.go#L351-L384), [wire types](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/ollama.go#L663-L671).

Ollama documents `truncate=true` as the default; `false` requests an error for oversized inputs. [Official API, accessed 2026-10-28](https://docs.ollama.com/api/embed).

**Inference:** on a server exhibiting that documented behavior, an oversized item can receive a valid, correctly sized vector for truncated text and never trigger fallback. Cardinality, dimensions, and finiteness cannot detect this. The server/version and concrete model still require qualification before claiming a reproduced loss.

This matters to prompt selection: `textForAtomEmbedding` intentionally uses full content when no description exists, relying on engine handling rather than representing only the opening. [Prompt representation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/prompt_reembed.go#L206-L215).

### G4 — Model name is useful but not a complete transformation identity

Ollama names its engine with the configured/resolved model name, and GenAI with its configured model string. These names do not themselves encode preprocessing, role, provider revision, or pooling algorithm. [Ollama identity](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/ollama.go#L395-L409), [GenAI identity](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/embedding/genai.go#L372-L382).

This is a proposed contract strengthening, not evidence that every unchanged-name configuration has drifted. Preserve the present prompt model guard while deciding how to extend it. Do not weaken admission to accommodate legacy unstamped data.

## 4. Required representation and eligibility contract

The following shapes and names are proposed, not shipped APIs.

### 4.1 Distinguish identities rather than overload a model string

An **embedding-space descriptor** identifies the provider, explicitly configured model, resolved immutable revision when available, output dimensionality, normalization, and compatible query/document role family.

A **representation descriptor** identifies effective input selection, preprocessing version, task instructions, chunking/pooling algorithm and version, and truncation policy. “Full content,” “description only,” and “selected source head” are different representations even when they use the same provider.

A **record provenance descriptor** identifies source identity/revision, exact effective-input digest, corpus/owner generation, actual produced dimension, creation attempt, and qualification status.

Do not include credentials or raw secret-bearing endpoint strings in identity fingerprints. Workspace ownership is distinct from mathematical compatibility: compatible spaces do not authorize cross-workspace sharing. Conversely, unnecessary per-document namespaces can destroy useful reuse and overshard indexes.

If a provider exposes only a mutable alias, record that uncertainty. An explicit operator namespace epoch can invalidate reuse, but it is not evidence of immutable provider weights. Do not invent a resolved revision, silently equate aliases, or substitute a supposedly compatible model.

### 4.2 Eligibility is stronger than byte preservation

A stored vector is eligible only when all of these hold:

1. Its space descriptor is compatible with the query's.
2. Its document role is valid for the requested query role.
3. Its dimension and finite values satisfy the declared representation.
4. Its representation descriptor and full/partial input coverage satisfy the requesting consumer's policy. Query and document descriptors need not be identical; their distinct transformations must form an explicitly accepted pairing.
5. Its source/effective-input revision is current for the requested evidence.
6. Its generation is admitted for serving.
7. Its owner/privacy scope permits this consumer.

Query and document task strings need not be identical. Gemini specifies paired retrieval roles, and its current embedding models differ in task-instruction and batching semantics. Provider/model capability qualification is necessary; a configured model string is not proof of equivalent behavior. [Official Gemini embedding guide, accessed 2026-10-28](https://ai.google.dev/gemini-api/docs/embeddings).

Unknown legacy vectors remain stored and lexically recallable but do not gain semantic eligibility merely by stamping today's configuration. Preserve data; distinguish preservation from qualification. An operator may explicitly schedule their reembedding after reviewing cost and provider scope.

Storage applies a deterministic compatibility gate. The kernel decides what eligible material matters to a turn and whether incomplete semantic coverage warrants lexical fallback, another retrieval step, or an explicit caveat. Neither side should silently take the other's responsibility.

### 4.3 Success must describe input coverage

A successful embedding means either:

- the full selected input was represented under the declared transformation; or
- an explicitly requested partial representation was produced, with coverage recorded.

Provider defaults must not silently choose partial representation. Single and batch APIs must share representation semantics. They need not produce bit-identical floats, but batching must not change covered text, role, or preprocessing.

Pooling is an intentional lossy transform, not proof of equivalence to a native whole-input embedding. Version and evaluate it. A chunk manifest or bounded digest of chunk boundaries can explain the representation without retaining redundant sensitive text.

The provider adapter must preserve one-to-one input/output correspondence where its public API promises it. A provider model whose response aggregates multiple inputs cannot be treated as an interchangeable batch engine merely because the request shape is accepted. Validate item IDs or indexes when the response exposes them. A positional-only response, such as the inspected Ollama response shape, cannot reveal a permutation of otherwise valid same-cardinality vectors. Positional ordering is then a qualified provider assumption, not something local cardinality checks prove. Where stronger correspondence is required and no item identity is available, use one-input requests or an independently justified protocol; do not claim that local validation can detect an unobservable permutation.

## 5. Proposed migration and publication lifecycle

### 5.1 Durable states

Use a durable per-store migration record with source/target descriptors, required row inventory, source digests, progress counts, failure categories, and checkpoint.

A proposed state machine is:

`planned → preparing → populating → validating → ready → active → retiring`

Explicit side states include `paused`, `failed`, and `canceled`. Cancellation does not erase successful durable progress and does not mean completion. A run identifier distinguishes resumed work from a different target.

1. **Plan:** discover intended runtime stores and their relevant tables. Preserve the current `.nerd`-only maintenance boundary; tracked build-input databases remain excluded.
2. **Prepare:** resolve the target descriptor and validate model/endpoint capabilities with bounded work.
3. **Populate:** write target vectors into a separate generation, or use explicitly tagged per-row target admission when partial availability is an accepted product choice.
4. **Validate:** establish target coverage, input-digest consistency, vector/index correspondence, and representative retrieval quality.
5. **Activate:** atomically publish the admitted generation for that database.
6. **Retire:** remove old generations only after readers drain and retention/rollback policy permits.

The existing [runtime-root restriction](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/reembed_all.go#L30-L43) is a requirement to retain, not an invitation to rescan build assets.

### 5.2 Publication must be conditional

Provider work happens outside long-held store locks. At publication, compare the source/effective-input digest, target descriptor, and generation/attempt identity against the current record. A vector generated for stale content must not overwrite the new representation.

An old in-flight engine request likewise cannot publish into a newly selected namespace. Bind producing identity to the result rather than rereading whichever engine happens to be attached when the network call returns.

Canonical row and same-database index publication should have an explicit transactional contract. Where an index update fails after canonical persistence, report repairable drift without making the row semantically eligible under the wrong identity. Repair/backfill must use the same compatibility gate as ordinary writes and reads.

### 5.3 Serving during migration is a policy decision, not an accident

An explicit configuration change must not silently keep querying with the previous provider. During migration, supported choices are:

- target-compatible partial semantic retrieval with coverage disclosure;
- lexical fallback while target coverage is incomplete;
- a policy-required wait before proceeding with work dependent on semantic coverage.

A staged activation that continues using the old engine is only valid when the operator explicitly requested staged migration and the serving contract identifies that old generation. It must not masquerade as the new configuration taking effect.

Readers acquire a generation-stable view. They should not mix A-space and B-space scores into one ranking, even if both generations are individually valid. Ranking separately and combining outputs would be a distinct retrieval design requiring explicit semantics and evaluation, not an implementation shortcut.

### 5.4 Multiple databases are not one transaction

Cross-database transactions are not required. Maintain a workspace manifest accurately reporting each store's generation and migration status instead.

“Complete” requires every required store and row to satisfy the chosen acceptance policy. Otherwise return partial/incomplete status with affected stores, row counts, resumable checkpoint, and reasons. Missing optional tables are different from table inspection failure; canceled work is different from empty work; skipped invalid source rows are different from successfully migrated rows.

Retain lexical/source data while a vector migration is incomplete. Rollback restores an explicitly compatible serving generation; it must not reintroduce stale source evidence as current truth.

## 6. Ownership, interfaces, and observability

Extend consumer-facing interfaces deliberately rather than create a universal store abstraction without a consumer. A proposed result envelope can carry a vector, producing descriptor ID, input digest, actual dimensions, coverage status, and bounded accounting. A batch envelope additionally preserves item order and per-item failure state.

The implementation sequence should inventory current consumers: generic vectors, prompt atoms, reasoning traces, learnings, perception corpora, and MCP descriptors have distinct schemas and lifecycles. The generic vectors gap is the first repair target; a shared descriptor contract can be adopted incrementally with adapters and explicit legacy behavior.

A compact retrieval/migration receipt should include:

- opaque operation and generation IDs;
- producing/query descriptor IDs;
- eligible, excluded, failed, and pending counts;
- exclusion/fallback reasons;
- source revision/input-digest checks;
- attempted versus actual provider path;
- full/partial representation status;
- migration activation and rollback state.

Do not log full prompts, raw secrets, or hidden reasoning. Digests and row references are not automatically privacy-safe if they permit membership inference; apply workspace access and retention policy.

The present LocalStore sqlite-vec query explicitly orders `vec_distance_cosine`; its internal “ANN” naming is not evidence of an approximate algorithm. Describe the actually inspected indexed SQL path and brute-force path, and require the same eligibility semantics in both. [Query](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/store/vector_store.go#L855-L940).

## 7. Costs and operational limits

Report a resource vector rather than adding unlike quantities:

- provider calls and provider-reported input units;
- local CPU/GPU time and model-residency cost;
- p50/p95 latency;
- bytes read/written, WAL growth, peak memory;
- old/target/index storage;
- retries, duplicate work, and stale-result rejection;
- downstream prompt tokens, model turns, and verified task outcomes;
- operator review and recovery time.

Raw float32 storage is `4 × N × D` bytes per vector copy. Shadow migration temporarily retains additional vector/index copies; the existing JSON representation and database indexes add overhead. This is arithmetic, not a measured repository footprint.

Avoid materializing the entire corpus when a checkpointed bounded traversal suffices. Bound provider concurrency, pending result bytes, SQL transaction size, and cancellation/join behavior. A failed batch followed by per-item fallback can multiply calls; account for it rather than attributing all work to a single successful batch.

Monetary estimates require current provider prices, currency, and disclosed amortization assumptions. A cheaper embedding batch is not cheaper overall if truncation loses the relevant tail and creates another model turn. Conversely, dual-generation safety has real disk/IO costs and should not be presented as free.

No model substitution, unbounded cloud reembedding, or automatic external spend follows from detecting mismatch. Capability qualification should not silently upload user content to a different provider. The configured provider and explicit user maintenance scope remain authoritative.

## 8. Failure modes and required behavior

| Failure | Required behavior |
|---|---|
| Equal-dimensional model switch | Exclude old-space vectors; retain source/lexical recovery. |
| Mutable alias changes behind stable name | Invalidate or explicitly mark unresolved identity; do not claim immutability. |
| Batch provider truncates oversized text | Refuse silent partial representation or use explicitly declared coverage. |
| Provider returns aggregate instead of per-item vectors | Fail correspondence contract; do not assign one vector to unrelated items. |
| Partial migration | Preserve checkpoint and target coverage; do not emit complete status. |
| Old-space failed row is later repaired | Repair applies the same identity gate and cannot resurrect it in target space. |
| Content changes during provider call | Reject stale publication and schedule current-input work if authorized. |
| Engine changes during provider call | Keep result bound to producing descriptor; never restamp as new engine. |
| Crash between canonical/index writes | Detect drift, repair only eligible rows, and report incomplete coverage. |
| Interrupted activation | Recover one durable active generation per database, not an ambiguous mixture. |
| Insufficient disk for shadow generation | Stop safely before losing serving/source data; offer bounded alternatives. |
| Unqualified legacy vectors | Preserve bytes, deny unsupported semantic comparison, explain fallback. |
| Misleading metrics | Distinguish attempted/used path and migrated/eligible/total rows. |
| Excess receipt or cache growth | Enforce retention and byte budgets without deleting required source evidence. |

## 9. Evaluation and acceptance design

The following are proposed evaluations. No tests were written or run in this research.

### 9.1 Deterministic contract cases

1. **Same dimension, different models:** deterministic A/B providers produce deliberately incompatible coordinate meanings. Indexed and brute-force recall must never rank A records using B queries.
2. **Legacy unknown identity:** source data survives close/reopen and lexical recall; semantic exclusion is explicit.
3. **Partial force, then maintenance:** one B update succeeds and another fails. Inspect immediate index absence separately from later reconcile/attach behavior; old-space rows must not reenter target results.
4. **Cancellation/crash at boundaries:** interrupt after a batch commit, before activation, and during retirement. Resume reports durable state truthfully and performs no duplicate publication.
5. **Concurrent source edit:** delay provider response, change source, release response. Stale publication is refused.
6. **Concurrent engine change:** delay A response, select B, release A. No B stamp is attached to A bytes.
7. **Overlong batch input:** a provider contract fixture models the documented default-truncation behavior. Full-input mode must explicitly prevent silent truncation or route through its declared transform.
8. **Single/batch parity:** use identical short and long texts, including evidence only in the tail. Covered input and descriptor are equivalent; floating-point tolerance is specified separately from retrieval quality.
9. **Malformed responses and observable correspondence:** empty, non-finite, wrong-dimension, wrong-cardinality, and unexpected aggregate responses cannot become qualified rows. Where responses expose item IDs/indexes, mismatches, duplicates, or missing identities are refused. For positional-only responses, valid same-cardinality permutation is not locally detectable: qualify the provider's ordering guarantee or use one-input requests when the required assurance exceeds that guarantee. Do not write an acceptance claim that assumes invisible reordering can be detected.
10. **Cross-store progress:** missing optional table, inaccessible required table, partial rows, and cancellation produce distinct manifest states.
11. **Build-tag/path parity:** supported indexed and non-indexed paths enforce identical compatibility and source-freshness rules.
12. **Owner and authorization boundaries:** mathematically compatible vectors from an unauthorized owner remain inaccessible; migration never touches excluded build assets.

### 9.2 Provider qualification

Use a recorded provider/model/version/config identity and controlled non-sensitive corpus. Verify default and explicit truncation behavior, task-role handling, batch correspondence, model revision reporting, dimensions, retry/cancellation, and long-input coverage. Do not infer universal provider support from one model or SDK success.

The native batching finding must be reproduced against the actual configured Ollama/server version before it is described as a user-visible incident. Official API documentation establishes a contract risk, not the installed server's behavior.

### 9.3 End-to-end value

Run normal production entry paths with world model and CodeDOM enabled, fixed tasks/models/policies, and the same repair allowance. Compare:

- current production behavior;
- eligibility-only protection;
- eligibility plus representation-fidelity correction;
- generation-safe migration and truthful coverage reporting.

Measure retrieval relevance/coverage, task correctness, false confidence under degraded retrieval, total model turns/tokens, latency, memory/disk, operator interventions, and cost per verified successful task. Include provider outage, cold boot, migration mid-session, and reopened stores. A retrieval microbenchmark alone cannot establish user-task benefit.

Use tail-dependent tasks to distinguish genuinely full-input representation from opening-only embeddings. Do not call a pooled vector “equivalent” merely because its shape is correct. Evaluate pooling and chunk-level retrieval as alternative representation choices if quality is inadequate.

## 10. Build queue and unresolved design decisions

These are proposed work items, not accepted feature cards or implementation status.

| Order | Proposed capability | Checkable exit |
|---|---|---|
| 1 | Generic-vector identity admission | Equal-dimensional mismatch and unknown legacy rows cannot rank in either read path or be resurrected by repair. |
| 2 | Explicit input-coverage contract | Single/batch representation semantics agree; native batching cannot silently truncate full-input requests. |
| 3 | Truthful resumable migration | Per-store progress survives interruption; partial conversion cannot report complete; conditional publication rejects stale source. |
| 4 | Generation-stable activation | Readers cannot mix spaces; activation/rollback/reopen recover one declared generation. |
| 5 | Cross-surface adoption | Prompt, trace, learning, perception, and MCP contracts are individually inventoried and migrated without weakening existing guards. |
| 6 | Production quality/cost qualification | Normal-entry task results satisfy predeclared correctness, coverage, and total-cost gates. |

Open decisions:

- Which provider model revisions can be resolved reliably, and what uncertainty policy applies when they cannot?
- Is strict whole-store target coverage required for activation, or may partial semantic availability be admitted with explicit policy and lexical fallback?
- What source/representation digests already exist and can be reused without redefining their meaning?
- Should long documents use pooled vectors, chunk-level retrieval, or both? This needs evidence, not the assumption that one pooled vector is always sufficient.
- What migration retention window balances rollback with disk pressure?
- How should existing independently stamped stores map to a shared descriptor without forcing unsupported all-or-nothing schema changes?
- Which receipts must reach Mangle, which stay operational, and how are they bounded?

Completion is not “all old initialization functions are called” or “reembed returned nil.” It is: **every served vector has an attributable compatible representation; incomplete migration is recoverable and honest; batch optimization preserves declared input coverage; and production task outcomes establish the benefit.**

