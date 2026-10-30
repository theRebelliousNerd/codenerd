---
doc-class: deep-dive
subsystem: retrieval-context
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Qualify semantic corpus observation before retiring persisted vectors

## Finding and decision

**Tier 4 can turn an incomplete filesystem observation into a complete-looking semantic corpus and destructive cache retirement.** Two producers discard uncertainty: `corpus` suppresses `WalkDir` errors, and `vectorsFor` skips head-read errors as though the file had no text to represent. A nil-error `Warm` then passes the reduced digest set to `FileVectorStore.Retain`, whose contract explicitly requires a complete current-digest census.

With a healthy persisted vector for a still-existing source file, a transient read failure can therefore delete that vector as stale. After access recovers, a fresh owner must submit the unchanged head for embedding again unless another source restores the row. The query path can also report full coverage of a denominator already reduced by the failed observation.

This is a narrow adoption residual of existing coverage and retention contracts, not a claim that caching, source freshness or partial-result handling are absent. **Retaining a historical vector does not authorize ranking an unreadable or unverified file.** The fix must separate safe retention from current serving eligibility.

Evidence is static source inspection of public main [388205138ba6d47c1cff0c5e97c896dfa642a391](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), revalidated during this pass. No application, tests, model inference, benchmark, failure injection or repository modification was performed. The witnesses below are conditional source traces, not reported incidents or measured costs.

## Existing contract and normal production route

The current retrieval specification describes the corpus as admitted source files with nonblank, nonbinary heads; persisted vectors are keyed by the exact represented head digest and engine space. It says retirement follows a complete pass. Source makes the requirement explicit: `Retain` may delete a digest only when the supplied keep set represents the whole current corpus. This condition is stronger than “no embedding call returned an error.” [Implemented specification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/retrieval/IMPLEMENTED_SPEC.md#L388-L398), [retention precondition and implementation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store.go#L149-L183).

This runs through normal initialization. After a configured engine is successfully attached, `initSemanticSearcher` opens the workspace's file-vector store, builds the shared searcher and starts background `Warm`. The task executor and its spawner receive that searcher. Warm-up is not conditional on a model choosing a retrieval tool; it runs until completion or shutdown cancellation. With no engine it does not run; with no persistent store there is no durable pruning consequence. [Engine setup](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1728-L1761), [warm-up initialization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_semantic.go#L12-L60), [task wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2697-L2710).

## Two independent owners of incomplete observation

### A. The path census can suppress a directory-read failure

Inside `corpus`, the walk callback immediately returns nil for any `walkErr`. The outer function consequently receives no error for a swallowed filesystem failure and returns the paths it happened to collect, provided no other error or cancellation occurred. A directory whose entries could not be read can therefore contribute none of its source paths without marking the census incomplete. [Corpus walk](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L285-L319).

Go's walk callback contract gives the callback control over whether an encountered error is propagated or suppressed. Returning nil does not certify that the missing subtree was inspected. This is a normal callback outcome, not a Go-runtime error-propagation bug. [Go WalkDir callback contract](https://pkg.go.dev/io/fs#WalkDirFunc).

**Do not confuse this with intentional exclusion.** Workspace membership returns `fs.SkipDir` for an excluded directory, and the corpus callback propagates it. That correctly prunes an out-of-domain subtree. Membership initialization/admission errors and context cancellation also have existing error paths. The witness requires an in-domain subtree and a later filesystem enumeration failure, not an excluded directory or a failed membership setup. [Membership gate](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/workspace/membership.go#L455-L492).

Both `Warm` and `SimilarFiles` call `forget(paths)` after this nominally successful census. Its memory deletion is based only on absence from those paths. Thus enumeration failure can retire in-memory vectors before durable retirement is considered. [Memory retirement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L269-L282), [warm caller](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L149-L169), [query caller](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L210-L220).

### B. A listed file's head-read failure is treated like no representable text

Even with a complete path census, `vectorsFor` skips a file when `readHead` returns an error. That is the same branch used for whitespace/empty text. The failed path contributes neither a head nor a digest, and no read error is retained for the eventual return. `cv.heads` is then set to the count of successful nonblank heads. [Head acquisition and reduced inventory](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L321-L361).

The central witness uses an actual open/read failure with no valid head, such as a temporary access failure after path admission. It does not depend on file deletion, changed source bytes or an embedding outage. Workspace membership is not a promise that the process can successfully read those bytes at the later acquisition boundary. [Head reader](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L486-L505), [file membership](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/workspace/membership.go#L291-L326).

Unlike census failure, this head-read failure need not delete a same-process memory entry: the path was still in `paths`, so `forget` retains it. It is nevertheless excluded from the current returned vector map because its current digest could not be established. That current-serving behavior is appropriate; the unqualified coverage and later durable retirement are not.

## How uncertainty reaches retirement and the brief

### Durable retirement

`Warm` copies `cv.heads` and `len(cv.vecs)` into its report. If `vectorsFor` returns no error, it calls `Retain(ctx, space, cv.digests)`. The store enumerates that space's rows and deletes every digest absent from the supplied map. A missing digest has no retained cause: genuine removal, intentional exclusion and failed observation become indistinguishable to this consumer. The production warm-up wrapper logs success when the call returns nil, including its reduced coverage and “stale rows pruned” count. [Warm decision](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L143-L185), [store deletion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store.go#L160-L183), [success/error logging](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_semantic.go#L49-L60).

`SimilarFiles` does **not** call durable `Retain`; do not attribute persistent deletion to every query. Its shared observation code creates the separate coverage consequence.

### Coverage qualification

The query initially sets Eligible from listed paths, but later replaces it with `cv.heads`. With a successful query embedding and matching vector dimensions, one readable file and one omitted unreadable file can yield `Eligible=1, Covered=1, Incomplete=""`. The failed head is absent from both numerator and denominator. [Query coverage construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L210-L253).

If at least one surviving semantic result is usable and not already selected, `semanticExpansion` sees neither an error, an Incomplete reason nor a coverage deficit, and keeps complete status. That status becomes issue-scoped outcome/coverage facts. Existing policy derives caveats for partial/unavailable outcomes, not a semantic IO deficit the producer never reported. Other tiers can still independently report problems. [Tier 4 disposition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L578-L603), [outcome facts](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/outcome.go#L128-L161), [caveat policy](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/retrieval.mg#L38-L52).

This establishes lost qualification, not a false user-task completion verdict. The brief describes starting points, and world/CodeDOM, explicit file reads, other retrieval tiers or a later retry may expose the file or its access failure.

## Minimal conditional witness

Use two admitted, non-test source files A and B with stable, distinct nonblank/nonbinary heads. A healthy previous owner has persisted vectors for both heads in space S. No other readable file has B's digest.

1. Start a **fresh searcher/process** using the healthy same-space store. Its memory cache is empty. Membership/path enumeration succeeds.
2. During warm-up, A's head is readable but opening/reading B's head fails. B still exists; its source has not changed.
3. A's vector loads from the store. There is no provider failure, concurrent publication, cancellation or store failure.
4. `vectorsFor` returns one head and one vector with nil error. Warm reports 1 eligible, 1 covered, 1 stored, 0 embedded.
5. Retain succeeds with only A's digest and deletes B's row. Warm returns nil with 1 pruned.
6. Restore access to unchanged B. A subsequent query or warm-up by an owner without B's memory vector obtains B's original digest, misses the store and submits it to the embedding engine.

The predictable additional work is an otherwise avoidable embedding input for B's head at the engine boundary, conditional on the later acquisition completing. Provider-side caching may change actual compute/billing. No extra model turn or task failure is implied.

The census-failure alternative substitutes step 2 with an admitted B-containing subtree whose directory listing fails. The callback suppresses the failure, B is absent from the path list, and the same incomplete keep set can reach Retain. An existing searcher can additionally lose B's memory entry through `forget`. These are **alternative producer failures**, not a requirement that both occur together.

## Controls that bound the finding

- A readable file sharing B's digest keeps the shared row. An already absent row cannot be newly lost. Retain touches only its supplied space, not every embedding model.
- A same-process head-read failure may leave B's old memory entry available for reuse **after** a successful current-head read. That can mask immediate re-embedding despite durable-row loss; the fresh-owner condition is deliberate.
- A reported membership, context, embedding or store error can stop the successful-retirement witness. The problem is specifically swallowed filesystem uncertainty before those guards.
- Actual deletion, deliberate membership exclusion and successfully observed empty/blank/binary heads are legitimate exclusion/retirement controls. Their treatment should not be weakened.
- No engine means no semantic path; no store means no durable deletion. Query-only calls can have coverage/memory effects without durable pruning.
- The bytes remain in the workspace. Healthy later observation and embedding recover retrieval; the finding is not permanent source/data loss, a permission bypass, or proof of a wrong patch.
- Existing tests verify current-content reuse, disappearance, concurrent same-path misses, space partitioning and dimension mismatch. The whole-corpus fixture deliberately excludes a readable whitespace-only file. These are inspected test definitions, not results from this pass. [Persistence/space controls](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store_test.go#L22-L118), [memory and dimension controls](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/outcome_test.go#L140-L233).

## Narrow repair and failure modes

Carry observation completeness independently from the set of successful paths/heads. Census failures and head failures need separate causes, with bounded counts/details. Do not derive “complete” from only successful work.

A conservative first repair can refuse memory retirement based on an incomplete census and skip whole-space durable Retain unless both census and head observation are complete. Preserve successful vectors and truthful partial results where the API supports them. Failing the whole lookup is simpler but may discard useful positive evidence and increase fallback work; choose that trade-off explicitly.

Do not rank an unreadable file using its old vector merely because retirement was deferred. Current ranking still requires an admitted current path, successfully observed head digest and compatible representation. Retained historical cache bytes are only candidates for future validated reuse.

**Blank/EOF handling needs care.** The present head reader returns `io.EOF` for a genuinely zero-length file. Blindly making every head-reader error a corpus failure would turn ordinary empty source files into permanent deficits. Distinguish graceful end-of-input and the intended empty/whitespace/binary exclusion from unexpected IO failures. Also specify partial-byte read behavior instead of silently treating an interrupted prefix as a complete represented head. [Current reader](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L486-L505), [Go EOF and Reader contracts](https://pkg.go.dev/io#Reader).

Fine-grained retirement is more complex because durable rows identify content/space, not the file paths supporting them. A path-to-digest support census could allow healthy scopes to retire independently, but adds ownership, update and migration obligations. Do not infer that one unreadable path supported no old digest. A global skip is simpler and conservative, but sustained failures postpone cleanup and can grow retained storage; expose that state and choose a retention policy without presenting uncertainty as confirmed absence.

Reuse existing outcome/fact/caveat channels. Genuine filesystem acquisition failures must not be relabeled `embedding_pending`, which policy intentionally suppresses while a healthy warm-up progresses. Preserve shutdown joining, no IO-wide mutex, chunk progress, source/space identity and normal policy/tool authority.

## Cost and fair unrun evaluation

The narrow benefit is conservation of reusable representations and honest observation status. Filesystem walk/head-read work still occurs. Error tracking adds bookkeeping and diagnostics; retaining rows during uncertainty saves delete/reinsert work but increases temporary storage. Optional retries add IO/latency and need bounded budgets. A fail-whole-pass implementation can increase definition scans or later model/tool work. A richer scope-support table costs metadata, SQL work and lifecycle complexity.

Measure head reads/hashes, eligible/observed/unreadable counts, reused/embedded vectors, SQL rows deleted/written, request attempts and provider usage, elapsed time, peak memory, store growth, fallback work and end-to-end verified-task cost. Do not equate one extra submitted head with one billable provider request or a specific dollar amount.

Proposed acceptance, not executed:

1. Seed A/B vectors, create a fresh owner, and inject a deterministic B open/read error after successful admission. Assert partial/unavailable observation, no unsupported pruning, no current B ranking, and retained A evidence.
2. Independently inject a directory enumeration failure after membership preparation. Verify incomplete census does not justify memory or durable disappearance; restore enumeration and validate reuse.
3. Restore unchanged B and compare same-owner and fresh-owner paths. Require reuse of a retained compatible digest only after a successful current read. Include a readable same-digest sibling and a different-space row.
4. Preserve true deletion, ignore exclusion, no-manifest/non-Git membership, zero-length EOF, whitespace, binary and normal short-head controls. Retention after a fully successful census must still work.
5. Cover provider failures, dimension mismatch, SQL failures, cancellation, shutdown and concurrent warm/query calls. Do not conflate acquisition incompleteness with background pending.
6. Trace the nonempty-survivor case through real TaskRetriever, outcome facts and brief caveats with world/CodeDOM enabled. Also exercise no usable semantic matches and other-tier recovery; no claim that every request must fail.
7. Pair healthy and intermittent-failure workloads at equal task/model/tool budgets. Compare correctness and coverage qualification alongside total latency, provider work and storage—not merely reduced pruning or a shorter prompt. Tests should inject IO errors reliably rather than assume permission-mode changes fail under privileged execution.

## Prior work and branch scope

The [retrieval freshness audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-freshness-coverage-and-context-value.md#L87-L121) already owns general semantic lifetime and honest degraded outcomes; its [as-built status](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-freshness-coverage-and-context-value.md#L189-L199) credits real repairs. The [structural coverage audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/structure-coverage-and-refactor-preconditions.md#L40-L52) already states that IO failure is not disappearance. The present contribution is the specific remaining semantic producer-to-Retain chain, its denominator and its recoverable embedding cost—not a new general principle.

The [scanner graph retirement study](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/scanner-graph-source-retirement.md) concerns old graph support surviving a successful refresh; here an unsuccessful observation incorrectly retires reusable vectors. The [semantic head coalescing study](https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/semantic-head-work-coalescing.md) concerns duplicate inputs during a complete cold pass, not retirement after a failed census.

Guidance, previous inventory and new research additions through [1227](https://github.com/theRebelliousNerd/codenerd/tree/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits) were reviewed. Main and active branch tips were unchanged from the preceding pass; the three inspected lanes do not change these retrieval files. The detached legacy Jules branch has no common ancestor for comparison. This is a bounded novelty check, not an exhaustive claim about every historical discussion.

**Acceptance condition:** only complete authoritative observations justify cache retirement, incomplete acquisition remains visible, and retained cache data never substitutes for current-source validation.

