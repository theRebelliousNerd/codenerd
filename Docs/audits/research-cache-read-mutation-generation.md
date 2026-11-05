---
doc-class: deep-dive
subsystem: tools/research
implementation-status: planned
last-verified: 2026-11-05
verified-against: a28ef2677172b7daa4df4d26d86c67118acc1771
verification-method: static-source-inspection
supersedes: []
---

# Research-cache expiry and promotion must preserve newer writes

## Finding and proposed decision

**An older research-cache reader can delete or replace a newer successful write for the same key.** The map has a mutex, but Get releases its read lock before acting on the entry it observed. Expiry cleanup deletes the current key without checking that it still identifies the expired entry. Disk fallback similarly publishes a previously read value without checking for a newer Set. The result can be a fresh value disappearing from both memory and disk, or subsequent readers receiving an older value after Set has completed. [Get][get] [Set][set]

The proposed contract is narrow: within one stable cache owner, a read's delayed cleanup or promotion must not invalidate a later accepted write. Preserve TTL, bounded memory, best-effort persistence, key containment and policy admission. This is storage-ordering work, not a proposal to move executive decisions from Mangle into Go.

**Evidence boundary:** static inspection of public main a28ef2677172b7daa4df4d26d86c67118acc1771. The three primary implementation files were re-fetched from default main immediately before preparing publication and matched the pinned versions. No application, tests, model calls, builds, database experiments or runtime reproduction were performed. All schedules and evaluations below are proposed, unrun source witnesses. This document changes no implementation or accepted architecture status.

## Distinction from existing work

The [workspace-destination audit][workspace] already owns process-global research-cache rebinding, including a rejected second boot redirecting the first owner's future writes. This finding holds with exactly one workspace, one cache object and an immutable disk directory. It requires neither another boot nor a different workspace.

The [context-feedback publication audit][feedback] already establishes the general principle that an old computation needs publication fencing. This is a separate implementation and a stronger destructive manifestation: research-cache expiry can remove a newly stored record, rather than merely hide current database feedback behind an old cached score. The [broker cache-epoch audit][broker] concerns interpretation of provider cache renewal receipts, not local cache mutation.

The current audit inventory and targeted repository searches for ResearchCache, research cache expiry, cache resurrection, and research-cache generation were screened. Existing research-cache architecture, tests and the workspace audit were inspected. No exact same-owner expiry/delete or disk-promotion overwrite report was found. This is bounded nonduplication, not a claim to have screened every historical branch or discovered the general concurrency principle.

## Implemented boundary and reachability

The process singleton is created with 1,000 entries and a 30-minute TTL. Get and Set handlers both obtain it. Set creates a fresh CacheEntry and installs it while holding the cache mutex; it also performs the best-effort disk write before releasing that mutex. [Singleton and defaults][singleton] [Handlers][handlers] [Set][set]

RegisterAll installs cache get/set/clear/stats. Registry.ExecuteTool checks arguments, capability envelope and write guard, but releases its registry read lock before invoking tool.Execute. It does not serialize whole cache invocations. Shard execution can run in goroutines. These establish a callable shared boundary and a concurrency-capable runtime, not proof that a particular production task issued the necessary overlapping calls. The witness's explicit precondition is two otherwise admitted calls overlapping on the same cache key. [Registration][register] [Registry dispatch][registry] [Shard execution][spawn]

The constitution lists get/set/stats as safe actions but deliberately excludes clear because it discards shared work. Safe classification is not a universal permission grant: preserve the other admission checks. Accordingly, the primary witnesses use only Get and Set. Clear appears below only as a direct cache API regression case; the proposed repair must not make the denied clear action callable. [Policy][policy]

Root guidance requires stale evidence not to survive a source change as current truth. A cache that silently republishes the older of two explicitly stored research values violates the intended direction of that requirement. It does not establish that an old value actually reached a model, caused a bad answer, or bypassed a safety gate. [Root guidance][root]

## Witness A: expiry of V0 deletes completed V1

Use one cache, key K, an expired V0, and adequate capacity. Persistence can be disabled for the minimal witness.

1. Reader G enters Get(K), reads the pointer to V0 under RLock and releases that lock at line 67.
2. G observes that V0 is expired and pauses before acquiring the write lock at line 73.
3. Writer S calls Set(K, V1, source). S creates a new, unexpired entry, replaces the map slot and returns successfully.
4. G resumes. It acquires the write lock and deletes K unconditionally at line 74. It never compares the current pointer or generation with V0.
5. A new Get(K), invoked after both calls complete, misses instead of returning V1.

With persistence enabled and successful file operations, S has also written V1 before returning. G then calls disk.remove(K), which resolves the current hash-named path and removes it without inspecting the record. Thus a fresh cache instance using that directory also misses. [Get][get] [Set][set] [Disk removal][disk-remove]

This is not merely an overlapping read returning a permissible old result. Its side effect persists after the newer writer has completed. Nor is it a Go map data race: individual map accesses are locked. A race-detector-only acceptance test would not establish correct ordering.

A related schedule pauses G after deleting V0 from memory but before disk.remove. S then publishes V1 to both tiers. G removes only the fresh disk copy. The live process may continue returning V1 from memory, masking the loss until eviction or restart. Holding the map lock only around deletion is therefore insufficient to protect persistence.

## Witness B: disk promotion overwrites completed V1

Use one fixed directory containing an unexpired V0 under K, while K is absent from memory. This can arise from an ordinary memory eviction because evictOldest removes only the map entry. Choose enough free capacity during the actual race so unrelated eviction does not drive the result. [Eviction][eviction]

1. G observes a memory miss, calls disk.read(K), and obtains V0.
2. Pause G after the disk read but before it acquires the cache mutex at line 86.
3. S completes Set(K, V1, source), including a successful disk write.
4. G resumes and assigns fromDisk to c.entries[K] at line 90 without checking the current slot.
5. A new, nonoverlapping Get(K) returns V0 from memory, although disk holds V1 and Set(V1) completed before that new read began.

Disk.read validates schema, exact key and expiry. Those checks remain useful; they do not prove that its snapshot is still the latest when promoted. The overwrite retains V0's original timestamps. It does not renew V0 indefinitely. A later Set, eviction/reload or expiry may end the stale serving interval. [Disk read][disk-read] [Get][get]

The two witnesses share missing read-to-mutation ownership, but need separate oracles: preventing old promotion does not automatically stop expiry from deleting a renewed entry.

## Controls and limits

- Serialized Get/Set calls avoid these schedules. A system that externally serializes every access to this cache would exclude the stated precondition; no such whole-operation serialization exists in the inspected registry/cache boundary.
- A fresh memory hit can return the entry it observed during an overlapping Set without itself being the defect. The requirement concerns late mutation and what subsequent nonoverlapping callers receive.
- The minimal deletion witness does not depend on disk failure, hash collisions, malformed keys, workspace rebinding, pointer mutation by callers, capacity exhaustion or clock discontinuity.
- Disk persistence is deliberately best-effort. The durable-loss witness assumes V1's write actually succeeded; it does not reinterpret the Set handler's success string as a general fsync/durability guarantee. [Disk write][disk-write]
- The diskStore mutex protects directory selection, not a cross-process record transaction. A same-object fix must not be presented as proof of independent-process coherence. [Directory selection][disk-dir]
- No measured frequency, incident, confidentiality disclosure, model-quality effect or savings estimate is claimed. Consequences established here are cache-value loss and stale cache serving under the specified schedules; extra refetch latency/cost and downstream wrong answers are conditional.

## Proposed implementation contract

Choose and document an ordering point for each cache operation.

1. Expiry removal must remain conditional on the exact entry/generation observed as expired. Recheck after obtaining mutation ownership. An old expiry observation is not authority to remove the current key.
2. Disk promotion must confirm that no Set, Clear or owner reset superseded the read. If a current entry exists, prefer it under its own TTL contract or retry the lookup. A key mutation generation plus owner-wide invalidation epoch is one possible design, not a shipped API.
3. Memory deletion and disk deletion must share an ordering protocol. A pointer check followed by unlocking and unconditional file removal still permits Witness A's disk-only variant.
4. Clear and hydration need the same protocol. A disk read begun before Clear must not republish after it. This requirement concerns storage semantics and does not relax clear's policy exclusion.
5. Keep the stable workspace-owner requirement from the existing audit. Generation fencing against the wrong mutable owner is not workspace isolation.
6. Preserve miss/error behavior, schema/key validation, TTL, corruption fallback, memory limits and best-effort disk errors. Do not solve the race by disabling persistence or silently refusing all cache writes.

For one in-process cache, serializing the relevant disk read/delete/promotion under an operation lock is the simplest correctness candidate. It lengthens critical sections and can block unrelated keys on slow filesystem operations. Per-key coordination plus a clear/owner epoch offers more concurrency but introduces lifecycle and reclamation complexity. Neither approach alone coordinates separate processes using the same directory. If that guarantee is desired, specify a separate cross-process protocol and atomic file ownership rather than relying on the directory mutex.

These are mechanical storage consistency choices suitable for Go. Mangle remains responsible for whether a research operation is admitted and whether its evidence belongs in active context.

## Evaluation plan, all unrun

Use harmless sentinel values and deterministic barriers at the read/mutation seams rather than hoping a sleep triggers the race. Keep the underlying handlers and storage code in the test path. A future test seam must be narrowly scoped; do not bypass admission to manufacture reachability.

| Fixture | Required oracle |
| --- | --- |
| Expired memory V0, pause before cleanup, complete Set(V1), resume | V1 remains recallable after both operations. |
| Same sequence, persistence enabled and writes successful | Fresh cache instance restores V1. |
| Pause expiry after memory deletion, Set(V1), resume disk removal | V1's disk file survives. |
| Disk V0 read, complete Set(V1), resume promotion | Later Get returns V1, never promoted V0. |
| Disk V0 read overlaps direct API Clear | Post-clear reads cannot recover V0 without a later write. |
| Ordinary sequential expiry and replacement | Existing TTL/miss/overwrite behavior retained. |
| Capacity eviction followed by disk fallback | Correct bounded memory and valid persistent reuse retained. |
| Corrupt schema/key, expired disk record, disabled disk, failed disk write | Existing safe miss and best-effort semantics retained. |
| Same key versus independent keys | Correct ordering without unnecessary global serialization where claimed. |
| Actual registered get/set dispatch with allowed and refused envelopes | The storage fix preserves policy/write-guard behavior. |
| Separate processes sharing one directory | Report supported contract explicitly; do not infer it from same-process success. |

Current tests cover sequential overwrite, expiration, restart, hydration, clear, traversal-shaped keys and corrupt-file handling. Parallel test declarations generally use separate cache instances; they do not create the decisive same-instance interleavings. Preserve these controls and add deterministic overlap tests. [Overwrite/expiry tests][coverage-tests] [Disk tests][disk-tests] [Basic cache tests][basic-tests]

Measure lock wait, filesystem operations, hit/miss results, resident entries and p95 latency under cold/warm reads and mixed-key writes. Do not claim token savings merely from fewer misses. If a later agent evaluation is authorized, compare equal tasks and evidence freshness alongside provider calls, latency and cost.

**Checkable exit:** an old Get cannot delete or promote over a later accepted same-key write; later nonoverlapping reads and successful restart restoration preserve that write, while existing admission and cache fallback contracts still hold.

[get]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache.go#L61-L92
[set]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache.go#L95-L115
[singleton]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache.go#L25-L58
[handlers]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache.go#L178-L245
[eviction]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache.go#L133-L147
[disk-read]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache_disk.go#L133-L164
[disk-remove]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache_disk.go#L163-L167
[disk-dir]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache_disk.go#L58-L100
[disk-write]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache_disk.go#L103-L130
[register]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/register.go#L7-L49
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/registry.go#L345-L423
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/core/shards/manager_spawn.go#L411-L469
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/core/defaults/policy/constitution.mg#L263-L272
[root]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/agents.md#L21-L28
[workspace]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/Docs/audits/research-cache-workspace-destination.md
[feedback]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/Docs/audits/context-feedback-cache-publication.md
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/Docs/audits/broker-cache-epoch-renewal-contract.md
[coverage-tests]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/research_coverage_test.go#L535-L563
[disk-tests]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/cache_disk_test.go#L11-L197
[basic-tests]: https://github.com/theRebelliousNerd/codenerd/blob/a28ef2677172b7daa4df4d26d86c67118acc1771/internal/tools/research/research_test.go#L214-L303
