---
doc-class: deep-dive
subsystem: diff
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# diff internals

Verified 2026-09-20 against commit `231cfa7` (`main`). All line references are
to `internal/diff/diff.go` or `internal/diff/cache.go`.

## Stages of ComputeDiff (diff.go:243-307)

1. Flag `IsNew`/`IsDelete` on empty sides (diff.go:250-255).
2. Binary short-circuit: a NUL byte on either side returns `IsBinary=true`
   with no hunks (diff.go:261-265), so large blobs never reach
   diffmatchpatch.
3. Cache lookup under `cacheKey`: two independent content hashes plus both
   content lengths plus the resolved context width, so engines with different
   context settings never share entries (diff.go:267-288).
4. Miss: `DiffLinesToChars` → `DiffMain` → `DiffCleanupSemantic` →
   `DiffCharsToLines` (diff.go:293-296), then `diffsToOperations`
   (diff.go:343-400) splits text on `\n`, drops the trailing empty split, and
   numbers old/new lines (`-1` for the absent side).
5. `groupIntoHunks` (diff.go:403-486) opens a hunk at each change with up to
   `contextLines` of leading context, closes it after `contextLines` of
   trailing context, and counts `OldCount`/`NewCount` via `computeHunkCounts`
   (diff.go:489-498). Context defaults to 3 (`defaultContextLines`, diff.go:17)
   and is clamped to `[0, 1000]` (`clampContextLines`, diff.go:30-38).
6. The result is cached as a deep copy; the caller owns what it received
   (diff.go:301-304).

## Cache (cache.go)

- Bounded LRU behind one mutex: 512 entries / 32 MiB by default
  (cache.go:15-24). Eviction drops least-recently-used entries until both
  bounds hold (cache.go:175-187); a single diff larger than the whole budget
  is not cached at all rather than thrashing the LRU (cache.go:159-161).
- Deep copy in both directions: `get` returns a clone (cache.go:96-121) and
  `put` stores a clone (cache.go:126-172), so caller mutation can never
  corrupt a cached entry. `Clone` copies the hunks slice and every lines
  slice (cache.go:229-246).
- Keys widen to two hashes because one FNV-1a collision would silently serve
  the wrong diff (diff.go:114-120); `fingerprint` derives both in one pass
  (diff.go:141-157). With `VerifyCacheContent`, inputs are retained and
  byte-compared on hit; a mismatch drops the entry and counts
  `Stats.Collisions` (cache.go:107-116). Verification memory is charged to
  the byte budget (cache.go:132-136).
- `ClearCache` empties entries in place, safe against concurrent
  `ComputeDiff`, and preserves cumulative counters (diff.go:515-520,
  cache.go:191-197). `Timeout` defaults to 5s (`diffTimeout`, diff.go:14);
  negative disables the bound (diff.go:202-211).

## Deliberate non-goals in the code

- `LineHeader` is never emitted by the engine (diff.go:48-56); hunk framing
  lives in `Hunk` fields only.
- `ComputeWordLevelDiff` is uncached: per-line-pair, cheap relative to a file
  diff (diff.go:526-529). It returns `WordSpan` values, not third-party
  structs, so consumers never import `sergi/go-diff` (diff.go:70-75).

## How it is verified

- `internal/diff/cache_test.go:167`
  (`TestClearCache_ConcurrentWithComputeDiff_ShouldNotRace`) runs clear
  against concurrent compute under `-race`; `cache_test.go:203` pins that
  clear preserves counters; `cache_test.go:244-287` pins that the context
  width changes grouping and is part of the cache key.
- A benchmark smoke test asserts no-heap-growth behaviour
  (`internal/diff/benchmark_test.go`); word-level tests live in
  `internal/diff/word_span_test.go`.
