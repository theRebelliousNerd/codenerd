---
doc-class: deep-dive
subsystem: mcp-discovery-cache
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 66ea6a983e1aeb73d701d31187cae72f6223ef64
verification-method: static-source-inspection
supersedes: []
---

# Legacy MCP cache backfill can preserve stale tool schemas

## Finding and required outcome

**Rediscovery can give an old MCP tool descriptor the current server schema's hash without updating the descriptor itself.** For an analyzed cache row whose `SchemaHash` is empty, `processToolSchema` copies the newly observed hash onto the cached object, saves that object, emits its facts and returns it. The old description, input/output schemas, annotations and analysis remain. If the live schema changed while the stamp was absent, the stored hash no longer describes the stored bytes. [Cache branch][cache] [Persisted fields][save]

The current remote authority **correctly refuses** this inconsistent registration: it requires the host review to match both the stored hash and a recomputation from the descriptor. The consequence is stale discovery information and a persistently unavailable tool, not an established authorization bypass. Repeating discovery with the same live schema now takes the ordinary matching-hash cache branch, preserving the mismatch. A successful save also carries it across process restart. [Authority check][authority] [Cache branch][cache]

**Required contract:** a migration stamp must describe the cached value it accompanies. Missing provenance cannot be repaired by borrowing the fingerprint of a different, newly observed value. Refresh the descriptor and dependent analysis, or establish that the cached source fields really match the current observation before adopting their hash. Keep the final recomputation and host-review requirements intact.

This report is a static counterexample at [main revision `66ea6a983e1aeb73d701d31187cae72f6223ef64`][pin], rechecked unchanged at 16:20 UTC on November 4, 2026. Pinned local source copies were checked against the tree's Git blob hashes. No application, test, fixture, build, benchmark, remote MCP call, model inference or training was run. Production incidence, affected-user counts and repair costs remain unmeasured. Only documentation and read-only source research were performed.

## Architectural intent and implementation status

Root guidance requires revision-aware context and truthful host facts, while the architecture guidance distinguishes the intended capability from its implementation. MCP's own `SchemaHash` contract says that the fingerprint identifies the server schema from which analysis was derived. The package README explicitly describes legacy rows being backfilled without reanalysis. This report qualifies that existing optimization rather than inventing a new caching feature. [Root guidance][root] [Architecture guidance][guidance] [Type contract][type-contract] [README][readme]

The hash currently includes name, description, raw input-schema bytes, raw output-schema bytes and serialized annotations. That is the equality domain used in this report. Changing to semantic JSON equivalence, adding model/prompt versions, or designing a general cache framework is not necessary to close this particular gap. [Fingerprint definition][hash]

| Capability | Status at the pin | Evidence boundary |
|---|---|---|
| Persist descriptors and derived metadata across restarts | Implemented | The bridge opens the workspace MCP database; store reads and writes include schemas, annotations, analysis and hash. [Bridge][bridge] [Save][save] [Load][load] |
| Refresh an analyzed row with a known, different hash | Implemented | The nonmatching branch creates a new descriptor from the live schema and analyzes/classifies it. [Refresh][refresh] |
| Reuse an unchanged, properly stamped row | Implemented | Matching hashes take the cache branch; authored tests check analyzer-call conservation. [Cache][cache] [Tests][cache-tests] |
| Add a schema-hash column to an older database | Implemented | Initialization adds nullable `schema_hash TEXT`; the reader accepts a null/empty value. [Migration][migration] [Load][load] |
| Verify the cached descriptor before assigning a missing stamp | Missing on the inspected branch | Empty hash is an unconditional cache hit once `AnalyzedAt` is nonzero. [Cache][cache] |
| Refuse a descriptor whose stamp differs from its actual contents | Implemented at dispatch | Host review, stored hash and recomputed hash must agree. [Authority][authority] |
| Automatically repair that inconsistent descriptor during repeated same-schema discovery | Missing on the inspected path | The borrowed stamp equals the live hash, so the analysis/refresh path stays bypassed. [Cache][cache] |
| Runtime prevalence or measured efficiency improvement from the proposal | Unverified | All evaluations below are proposed and unrun. |

## Production path

### A supported legacy state enters ordinary discovery

The normal factory creates an MCP bridge from enabled server configuration, connects its VirtualStore adapters and model-facing control plane, and starts connection. The bridge opens `.nerd/mcp_tools.db`, creates the analyzer and manager, and installs the fact emitter. An auto-discover connection subsequently calls `DiscoverTools`. This is not limited to a private helper that production never reaches. [Factory entry][factory-entry] [Factory wiring][factory] [Bridge][bridge] [Auto discovery][auto]

The database migration appends `schema_hash TEXT` without a default or a `NOT NULL` constraint. SQLite's documented additive-column behavior does not rebuild old application values; the column definition supplies no old schema-analysis provenance. The Go loader represents this field as `sql.NullString` and leaves the Go hash empty when it is absent. Existing nonzero `analyzed_at` values are retained. The source and README therefore explicitly support the starting state, although this inspection did not find or inspect a real user's legacy database. [Migration][migration] [Load][load] [SQLite column addition][sqlite-alter] [SQLite defaults][sqlite-default]

`DiscoverTools` obtains server schemas, calls `beginRemoteToolRefresh`, processes each schema and places the returned objects into the current connection's tool cache. `processToolSchema` looks up the row by `serverID/name`. Neither name equality nor an old analysis timestamp establishes schema equality. [Discovery][discover] [Early revocation][revoke]

### The cache hit changes only the stamp

For an existing analyzed row, the hit condition is:

```go
existing.SchemaHash == "" || existing.SchemaHash == schemaHash
```

On the empty-hash arm, the only assignment to the cached descriptor before saving is:

```go
existing.SchemaHash = schemaHash
```

Here `schemaHash` was calculated from the **newly received** `MCPToolSchema`. `SaveTool` persists the fields of `existing`, not the newly received schema. The row update includes both its new hash and its old descriptor, classifications, embedding and `AnalyzedAt`. The helper then emits that object and returns before the live-schema construction, analyzer and classification block. [Cache branch][cache] [Save][save] [Skipped refresh][refresh]

This is not a partially completed background reanalysis. On this branch no reanalysis is started. The fact emitter publishes the supplied metadata, including its supplied nonempty hash; it does not recompute the descriptor hash as part of `EmitTool`. [Fact emission][emit] [Hash fact][hash-fact]

## Concrete causal witness

Use one configured server `srv` and one read-only tool `search_docs`. A complete, successful catalog contains the tool exactly once. Assume the database and fact writes succeed, the connection stays current, and ordinary authorization prerequisites are available. No concurrent refresh, malicious server, partial catalog, malformed JSON or hash collision is needed.

Let:

- `S0` be the stored descriptor, with name `search_docs`, a valid object input schema and description `Search archived documents`.
- `S1` have the same name and input/output schema bytes but description `Search current documents`.
- `H0 = ToolSchemaHash(S0)` and `H1 = ToolSchemaHash(S1)`, with `H0 != H1`.
- The legacy row contain `S0`, a nonzero `AnalyzedAt`, cached analysis for `S0`, and an empty schema-hash field.

The description-only change keeps arguments valid under both schemas and isolates the metadata/hash failure from argument validation.

| Stage | Source-derived state or result |
|---|---|
| First current discovery receives `S1` | `schemaHash` is `H1`; lookup returns the analyzed legacy row. |
| Empty-hash cache branch | The returned object still contains `S0`, now stamped `H1`; analyzer calls for this discovery are zero. |
| Successful store update and publication | Persistent row and current manager registration contain the old descriptor with the new stamp. Old analysis remains associated with it. |
| A host review is correctly pinned to live `H1` | Stored-hash comparison passes, but recomputation from the registered descriptor yields `H0`; authority returns a refusal before transport. |
| A review instead uses old `H0` | It fails the stored-hash comparison against `H1`. Changing only the review cannot make both checks true. |
| A second discovery receives unchanged `S1` | The ordinary `existing.SchemaHash == schemaHash` arm now passes. The stale descriptor is returned again without analysis. |
| Close and reopen the same database, then rediscover `S1` | The successful first save reloads the same old descriptor and `H1`; the same branch remains a hit. |

The final authority comparison is an explicit conjunction: the review hash must equal the stamp **and** `remoteSchemaHash(tool)`. The latter reconstructs `MCPToolSchema` from the tool's own fields. For this witness no review hash satisfies both equalities. This fail-closed behavior is the protection to preserve. [Recomputation][recompute] [Authority][authority]

Early refresh revocation does not repair the durable row. On later discovery, `beginRemoteToolRefresh` can reject the inconsistent in-memory registration because its recomputed hash differs. The following `processToolSchema` call then reloads the same durable row and returns it through the matching-stamp arm; final discovery publication installs it again. Revocation and cache repair are separate obligations. [Revocation][revoke] [Rediscovery][discover]

The mismatch is not permanent under every possible event. A later live schema whose hash differs again can enter normal refresh, and an explicit row repair can remove the bad state. The claim is narrower: repetition of the **same** successful discovery, including after restart, does not repair it. If the first save fails, the first in-memory result is still inconsistent but cross-restart persistence is not established; the primary witness assumes successful persistence.

## User-visible consequences and limits

The control plane prefers the persisted catalog, then validates calls against the resolved tool before manager dispatch. Its disclosure can therefore continue to show the old description, signature and analysis. If input schema rather than only description changes, valid live arguments may also be rejected against the stale schema before the protected dispatch boundary. That is an additional variant, not needed for the primary witness. [Catalog preference][catalog] [Call validation][call]

The CLI's offline review display compares the configured review hash to the stored hash. For a review pinned to `H1`, it can show a matching configured review beside the old descriptor. The CLI already disclaims live admission, so this is not a claim that it promises a permitted remote call. Its narrower diagnostic still cannot reveal the internal stamp/content inconsistency, which can send an operator toward unnecessary review edits rather than the cache migration. [CLI output][cli] [Review display][review-state]

Potential costs include failed task attempts, stale tool selection, repeated schema inspection and operator repair time. Their frequency is unknown. No unauthorized remote effect is demonstrated; no actual model retry loop was observed. A clean database, a legacy row whose complete source fields still match the live schema, or a normally stamped row that changes follows a different path and is not this failure.

## Prior work and literature boundary

The current audit inventory contains 376 Markdown files. The novelty screen examined that inventory, locally hash-verified relevant audit bodies and current GitHub searches for `processToolSchema`, `SchemaHash backfill`, `mcp schema migration` and `mcp_tools schema_hash`. The package README documents the optimization, but no exact report of borrowing a changed live hash for an old descriptor was located. This is a bounded repository check, not a claim of universal novelty.

- **MCP capability admission** owns review installation, exact permission and honest readiness. This finding relies on that implemented recomputation refusing the mismatch; it does not reopen the old missing-review wiring issue. [Admission audit][admission-audit]
- **MCP versioned wire fidelity** owns protocol decoding, catalog coverage and publication generations. The witness has one complete current catalog and no race. It fails later, while migrating a local cached descriptor. [Wire audit][wire-audit]
- **MCP retained scalar continuation** owns recoverable result disclosure after execution. The present failure occurs before any remote tool execution. [Retention audit][retention-audit]
- **Embedding representation and migration** addresses vector compatibility and consumer lifecycles. An embedding can remain stale here as one dependent artifact, but the witness works without any vector engine. [Embedding audit][embedding-audit]

Mokhov, Mitchell and Peyton Jones describe verifying traces as records of the dependency values or hashes used for a prior result. Reuse checks those recorded dependencies against current ones; changed dependencies require rebuilding and recording a new trace. The applicable lesson is the relationship between a cached result and its actual inputs. Assigning a new input fingerprint to an old result does not establish that relationship. This is a design analogy, not an application of their build-system correctness theorem to nondeterministic LLM analysis, and their performance results are not evidence of CodeNERD savings. [Build Systems à la Carte, section 5.2][paper]

## Proposed completion contract

### Preserve descriptor and derivation identity

The following changes are proposed, not implemented in this audit.

1. **Treat an absent stamp as unverified provenance.** Never establish a hit solely from a tool name and analysis timestamp. A safe initial policy is to refresh each unstamped analyzed row once from the current schema.
2. **Allow a verified migration fast path only when justified.** If all source fields needed by the current hash contract were retained faithfully, recompute a candidate hash from the cached descriptor and compare it with the live descriptor. Equality can support stamp backfill under that contract. Missing annotations, invalid raw schemas or unknown historical representation semantics require refresh, not assumed equality.
3. **Validate stamped cache integrity too.** A warm-hit predicate must bind the stamp to both the stored descriptor and the live observation. This is needed to repair already mis-stamped rows; changing only the empty-hash arm leaves previously affected rows stuck.
4. **Refresh dependent material together.** Construct the base descriptor from the new observation, then regenerate or explicitly invalidate its derived classifications, semantic metadata and embedding. Do not copy a new schema onto an object while silently keeping analysis from a different schema. Preserve usage history separately if that history remains meaningful under the chosen versioning policy.
5. **Keep incomplete analysis honest.** Current refresh can produce a classified descriptor even when model analysis is unavailable. Preserve that capability, but do not claim the previous expensive analysis was produced from the new input. A failed or canceled refresh must not create a fresh validity stamp for unrecomputed metadata. [Current analyzer paths][analyzer] [Current fallback classification][refresh]
6. **Publish a coherent version.** Persist and publish the descriptor, stamp and analysis disposition as one logical version, with explicit handling when persistence or fact publication fails. Reuse existing connection/catalog fencing rather than weaken it. A failed refresh may retain an old descriptor as historical information, but cannot silently reinstall it as a current match.

Hash equality is only this contract's evidence of input identity. It does not prove the model's analysis is correct, that annotations are trustworthy authority, or that the server still performs the advertised behavior. The host review and exact per-call policy remain independent requirements.

### Repair and diagnosis

Expose a distinct cache-integrity or legacy-refresh disposition instead of presenting the mismatch solely as missing host review. A targeted repair should reacquire and validate the affected descriptor, retaining enough old identity for diagnosis. It must not rewrite reviews, copy the new hash into an old review, disable recomputation, execute a tool to test its permissions, or delete the whole MCP database to hide the defect.

The migration must be repeatable. Once a coherent current row is durable, a later unchanged discovery should reuse it; an interrupted repair should remain distinguishable from that state. Diagnostics need tool ID, old/current descriptor digests, disposition and bounded failure details, not credential values or complete confidential schemas in routine logs.

## Costs and evaluation

### Cost model

Let `L` be legacy analyzed rows examined, `C` those requiring refresh, `B_old` and `B_live` their descriptor bytes, and `D` embedding dimensions. These are workload variables, not measured counts.

A verified fast path adds hashing and integrity comparison over approximately `B_old + B_live` bytes per inspected row, plus existing database reads. Conservative one-time refresh can incur analysis for all `L` rows; verified reuse aims to limit that to `C`, but may need conservative refresh where legacy provenance is incomplete. Hash checking is not a substitute for dependency completeness.

For each refreshed row include role-prompt compilation, analyzer input/output and any provider retries, optional embedding work, classification, serialization, database writes, vector-index maintenance and fact retraction/assertion. `SaveTool` performs a full row upsert and can also delete/reinsert its vector entry, so a stamp-only migration is not necessarily a tiny isolated write. The current per-tool discovery loop is sequential within a server; cross-server overlap means total stage durations cannot simply be summed as elapsed wall time. [Analyzer][analyzer] [Save and index maintenance][save-index] [Discovery loop][discover]

Keep separate ledgers for uncached/cached model input, output tokens, embedding tokens or units, network requests/bytes, host CPU, database I/O, peak memory, retained bytes, critical-path latency and operator minutes. Attribute retries once to their producing stage. Unknown remote billing is unknown, not zero. A proposed implementation may cost more on its first upgraded startup while restoring functionality; the present branch's zero analysis calls are not an efficiency win when the task remains blocked.

### Fair unrun evaluation plan

Use isolated on-disk databases and the ordinary manager/bridge route with a counted fake transport and deterministic analyzer first. Existing discovery tests already use an isolated database and a counting analyzer, but their change test begins with a fresh discovery that writes a valid hash. It does not establish the missing-stamp upgrade transition. Read their assertions as useful baseline coverage, not as executed evidence for this audit. [Existing tests][cache-tests]

| Case | Required observation |
|---|---|
| Legacy SQL `NULL` and explicit empty hash, nonzero analysis timestamp, changed description | Current descriptor replaces or explicitly supersedes old metadata; hash matches the actual descriptor; an otherwise admitted call reaches the counted transport exactly once. |
| Same starting row, changed input schema, output schema or annotations | Refresh observes every field covered by `ToolSchemaHash`; old signatures, classification and analysis cannot survive labelled current. |
| Legacy row whose complete inputs truly match | Conservative refresh or justified reuse is allowed; record which policy was used and its cost. No invented provenance. |
| Already mis-stamped row from the present branch | Repair does not depend on another server schema change. Repeated same-schema discovery converges to a coherent row. |
| Properly stamped unchanged row | Preserve warm reuse and the existing analyzer-call bound. |
| Ordinary known-hash schema change | Preserve existing reanalysis and stale-fact retirement behavior. |
| Correct current review, old review, absent review | Current coherent metadata can proceed only with all ordinary permission prerequisites; stale or absent review remains refused before transport. |
| Analyzer unavailable, malformed metadata, cancellation or persistence failure | Explicit degraded/unresolved disposition; no new stamp falsely certifies old analysis; no automatic remote execution. |
| Process restart before repair, after coherent persistence, and after an interrupted repair | Reopened state agrees with the recorded disposition; retries cannot turn an incomplete migration into a valid cache hit. |
| Repeated discovery after successful repair | Descriptor and facts remain current with no unnecessary repeated analysis. |

Measure the existing implementation, conservative refresh and verified-reuse proposal on identical schemas, legacy databases, review configuration and task questions. Include unchanged controls and no-cache controls. Record current-schema disclosure correctness, internal hash/content consistency, repair convergence, final authorized task success, refusals, analyzer/embedding calls and the complete resource ledgers above. Use real-kernel admission fixtures before treating a permissive fake kernel as evidence of safe execution.

Only after deterministic correctness is established should a separately authorized live evaluation measure latency, provider cost and retrieval quality. Predeclare success criteria, report cold upgrade separately from warm steady state, and use matched repeated trials if model behavior is involved. Do not call lower token use an improvement when it merely preserves an unusable cache row.

**Exit condition:** an analyzed legacy row cannot acquire the current schema hash while retaining a different source descriptor; previously inconsistent rows have a bounded repair path; unchanged verified rows retain reuse; and the exact authority checks continue to refuse stale or unreviewed requests. No implementation or acceptance evidence for that exit is claimed here.

## Source references

[pin]: https://github.com/theRebelliousNerd/codenerd/tree/66ea6a983e1aeb73d701d31187cae72f6223ef64
[root]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/agents.md#L1-L32
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/Docs/architecture/AGENTS.md#L1-L77
[type-contract]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/types.go#L125-L133
[readme]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/README.md#L152-L157
[hash]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/types.go#L357-L379
[factory-entry]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/system/factory.go#L1873-L1888
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/system/factory_mcp.go#L18-L55
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/integration.go#L89-L138
[auto]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/client.go#L381-L405
[migration]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/store.go#L121-L150
[load]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/store.go#L422-L515
[discover]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/client.go#L458-L561
[revoke]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/remote_authority.go#L621-L666
[cache]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/client.go#L565-L595
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/client.go#L597-L664
[save]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/store.go#L327-L386
[save-index]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/store.go#L327-L407
[emit]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/facts.go#L137-L155
[hash-fact]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/facts.go#L402-L428
[recompute]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/remote_authority.go#L123-L128
[authority]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/remote_authority.go#L288-L317
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/controlplane.go#L102-L117
[call]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/controlplane_invoke.go#L86-L124
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/cmd/nerd/cmd_systems.go#L110-L127
[review-state]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/cmd/nerd/cmd_systems.go#L661-L676
[analyzer]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/analyzer.go#L47-L130
[cache-tests]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/internal/mcp/facts_lifecycle_test.go#L12-L130
[admission-audit]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/Docs/audits/mcp-capability-admission.md
[wire-audit]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/Docs/audits/mcp-versioned-wire-fidelity.md
[retention-audit]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/Docs/audits/mcp-retained-scalar-continuation.md
[embedding-audit]: https://github.com/theRebelliousNerd/codenerd/blob/66ea6a983e1aeb73d701d31187cae72f6223ef64/Docs/audits/embedding-representation-and-migration.md
[sqlite-alter]: https://www.sqlite.org/lang_altertable.html#alter_table_add_column
[sqlite-default]: https://www.sqlite.org/lang_createtable.html#the_default_clause
[paper]: https://simon.peytonjones.org/assets/pdfs/build-systems-jfp.pdf#page=23
