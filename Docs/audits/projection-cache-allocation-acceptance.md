# Projection-cache fallback must precede count-driven allocation

## Decision and status

Preserve the parsed-orientation cache optimization, but validate its row count against the available serialized bytes and a supported resource policy before allocating the fact slice. Malformed cache data should produce a cache miss/error and source reparse, not an allocation governed by an unchecked count.

**Inspected branch pin:** `a53cfc308e8836c72370adaa46deba3f7b00180d`. Main remained `9dcb01b9736bf4763aa3ac391d29f133d73c6611` when checked on 2026-10-29 at approximately 11:59 UTC. The disk projection cache was introduced on this branch in [`8810a4c3`](https://github.com/theRebelliousNerd/codenerd/commit/8810a4c3e35da24351f55ac697649fcf9ef93054); a53 is the inspected state, not its introducing commit.

This is a **branch-only local-cache robustness acceptance issue**, not an observed outage or established remotely reachable vulnerability. No malformed fixture, tests, provider calls, inference, runtime reproduction, or repository changes were performed. The memo uses public source information.

## 1. Exact source boundary

`decodeParsedProjection` validates the magic and the SHA-256 key of the projection's current source bytes. It bounds each length-prefixed field against remaining bytes. But after successfully decoding the row-count uvarint it immediately executes `make([]Fact, 0, count)`, before checking whether even the first serialized row is present. There is no application count ceiling or relationship between count and remaining cache bytes. [Decoder](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/orientation_projection_cache.go#L61-L109).

A validly encoded but implausible count can therefore request disproportionate memory or exceed allocation limits before the normal truncated-row error is reached. The exact runtime result depends on count, architecture and memory availability: possible outcomes include a make-capacity panic, expensive allocation, or allocation failure. Do not assert one universal process outcome without runtime evidence.

The enclosing `cachedProjectionParse` reparses on returned decoder errors and documents unreadable/unwritable caches as costing only the parse. It does not convert allocation panics into returned errors; its fallback is reached only if decoding returns. Recovery at a higher layer, if present, would not establish this function's promised successful reparse. [Fallback](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/orientation_projection_cache.go#L112-L135).

The source digest is an identity key for the original text, not a checksum over the serialized payload. A matching header consequently does not prove that the subsequent cached count and rows are intact. This observation explains why payload corruption can survive the key check; it is not a claim that the cache was designed as an adversarial authentication boundary.

## 2. Practical reachability and trust

The file is local generated state at `<workspace>/.nerd/cache/orientation_projection.bin`, derived from `<workspace>/.nerd/orientation/orientation.mg`. The writer uses an atomic replacement with mode 0644, after creating the cache directory with mode 0755. Actual filesystem ownership and access policies still govern access; those mode constants are not evidence that an arbitrary user can modify the file. [Path and writer](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/orientation_projection_cache.go#L35-L55), [publication](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/orientation_projection_cache.go#L130-L135).

Kernel boot checks whether the saved orientation source exists, calls `loadOrientationProjection`, and appends its declarations and rows. Returned errors are logged. On an in-process cache miss, the loader reads the source and invokes the disk-cache decoder. A fresh process naturally has no in-memory projection entry. An existing matching in-process entry bypasses the disk decoder, so corrupting the disk cache need not affect an already warmed process immediately. [Boot caller](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/kernel_init.go#L489-L506), [memory cache and disk path](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/orientation_projection.go#L62-L109).

The specific count hazard requires readable local cache bytes, matching magic and source key, a readable declarations field, and a successfully decoded implausible row count. Corruption of the header or a stale source key produces an ordinary miss instead. There is no demonstrated remote/model path manufacturing this count. A malicious actor would need an appropriate local cache-write capability; the practical finding also covers accidental payload corruption that preserves the header. Neither its prevalence nor a production incident has been measured.

## 3. Existing defenses and exact limits

- Wrong magic, short header or wrong source key is a cache miss before row allocation.
- Invalid count varints return an error. The problematic class is a successfully decoded count whose claimed inventory is not feasible or acceptable.
- Declarations, predicate and encoded-argument byte lengths are bounded against remaining bytes. Trailing bytes are rejected.
- The nested `DecodeFactArgsBinary` consumes available bytes; string lengths and fixed-width fields are checked. It has no independent attacker-supplied argument-count preallocation. Do not conflate its bounded traversal with the outer count allocation. [Argument decoder](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/world_fact_codec.go#L98-L147).
- Cache writes use a unique temporary file, sync and replacement, reducing torn-write exposure. Atomicity does not validate old or externally modified payloads. [Atomic writer](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/atomicfile/atomicfile.go#L154-L222).
- Both projection source and cache use `os.ReadFile`, with no explicit file-size budget at these call sites. Fixing the row-count issue alone would not establish a whole-boot memory ceiling. The separate full-file read concern should not obscure the sharper issue: a small readable cache can claim disproportionate output capacity.
- Singleflight coalesces simultaneous loads by path; it does not validate count or make allocation failure a cache miss.

## 4. Prior-work relationship

The branch's performance audit records the binary world codec, persistent parsed projection cache and reported boot savings. Its as-built entry does not establish count validation or malformed-cache fallback safety. This memo is an acceptance refinement to that implementation, not a new proposal to add projection caching. [As-built entry](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/Docs/audits/mangle-kernel-performance.md#L383-L391).

The repository's atomic-file guidance addresses preserving complete files across replacement; the current cache already uses that writer. This finding is different: validating resource claims when reading an existing cache. Public code/doc searches also found generic historical cache-corruption and allocation discussions, but no exact prior memo for this newly introduced count-before-row-validation sequence. Search coverage is not an exhaustive proof of novelty; the default-branch search index did not include this branch's new decoder, which was inspected directly by commit.

## 5. Bounded remedy and costs

Proposed, not implemented:

1. Before count-driven allocation, establish that the count is feasible for the remaining encoded inventory, safe for integer/allocation arithmetic, and within an explicitly supported decoded-resource budget. A bound based only on representable integers is not a memory policy.
2. Limit initial capacity or use bounded staged growth; do not reserve the entire claimed count before validating rows. Preserve exact valid-cache semantics.
3. Reject malformed or over-budget cache data through the existing error/reparse path. Distinguish cache rejection from invalid source: reparsing an invalid or oversized source must still report the source's own failure honestly.
4. If a whole-file byte budget is adopted, enforce it before full materialization and define treatment of legitimately large projections. Do not silently truncate facts or invent an arbitrary limit without corpus-size evidence.

A constant-time count/remaining-byte check is cheap; a validating prepass adds another linear traversal. Staged growth trades lower speculative capacity for possible reallocations. Cache rejection costs source parsing and, when writable, replacement IO. A permanently unwritable or repeatedly corrupted cache can make that parse cost recur; logging should remain useful without flooding. Peak memory includes source bytes, cache bytes, decoded facts/strings and per-caller clones, not just the count slice. No new inference, embeddings or provider expenditure is needed.

The performance audit's existing measurements are historical claims, not reproduced results here. Compare any hardening against equal semantics and the same cache/source corpus, and report cold-start latency, CPU, allocation bytes, peak RSS, bytes read/written, fallback frequency and concurrent-caller behavior separately.

## 6. Fair evaluation matrix — proposed, not run

Compare current branch behavior, hardened decoding, and cache-disabled source parsing using the same valid source and resource envelope. Malformed cases should run only in an authorized isolated process with memory/time limits; no large-allocation demonstration is needed to justify static precondition checks.

| Case | Required outcome |
|---|---|
| Valid empty and representative large cache | Same declarations and facts as direct source parsing; no silent truncation. |
| Wrong magic, truncated header, stale source key | Ordinary miss and reparse. |
| Invalid/overflowing count encoding | Returned decode error, not speculative allocation. |
| Validly encoded count inconsistent with remaining rows | Bounded rejection before capacity reservation, followed by source reparse. |
| Truncated declarations, predicate or argument bytes; unknown argument tag; trailing bytes | Returned error and reparse with bounded work. |
| Count/file-size policy boundary | Supported size succeeds; excess is classified explicitly; source fallback has its own honest limits. |
| Corrupt cache with valid source | Successful source-derived result and cache replacement when writable. |
| Corrupt cache with invalid source | Source parse error is preserved; no false successful boot evidence. |
| Unreadable/unwritable cache | Source parse still works when source is valid; repeated-parse cost is measured. |
| Warm in-process hit, fresh process, concurrent cold callers | Document bypass/coalescing behavior; no claim that every load reads disk. |
| Concurrent atomic replacement | Reader obtains a valid prior/new artifact or safely rejects; platform-specific sharing failures do not masquerade as successful cache publication. |

**Exit condition:** malformed serialized counts cannot cause disproportionate speculative allocation before cache rejection; valid-cache equivalence and the intended boot improvement remain, while source-parse failures and resource limits are reported accurately.
