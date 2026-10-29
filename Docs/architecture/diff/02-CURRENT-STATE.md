---
doc-class: shipped
subsystem: diff
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# 02-CURRENT-STATE — internal/diff

Shipped status: a deterministic file/line diff engine built on `sergi/go-diff`,
fronted by a bounded LRU cache with deep-copy isolation, and wired into exactly
two production consumers — the agent repair loop and the TUI diff view.
Every claim below cites a repo-relative path with symbol and line, witnessed by
direct read on 2026-09-21 against a clean `main` working tree
(commit pin `231cfa7` is the last documented pin, per
`Docs/architecture/diff/README.md:6`).

## 1. `internal/diff/diff.go` (557 lines) — the engine

Deterministic computation is offloaded to a third-party library, not hand-rolled:
`Package diff provides robust diff computation using the sergi/go-diff library`
(`internal/diff/diff.go:1`), via
`import diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"`
(`internal/diff/diff.go:9`) — the only third-party import in the package.

Bounded cost is enforced by constants and clamps, not by convention:
`const diffTimeout = 5 * time.Second` (`internal/diff/diff.go:14`),
`const defaultContextLines = 3` (`internal/diff/diff.go:17`),
`const maxContextLines = 1000` (`internal/diff/diff.go:20`),
`func containsNullByte(s string) bool` NUL sentinel (`internal/diff/diff.go:24-26`),
`func clampContextLines(n int) int` bounding to `[0,1000]`
(`internal/diff/diff.go:30-38`).

The line model: `type LineType int` (`internal/diff/diff.go:41`);
`LineContext` / `LineAdded` / `LineRemoved` (`internal/diff/diff.go:44/45/46`);
`LineHeader` (`internal/diff/diff.go:48-56`) with the never-emit comment at
`internal/diff/diff.go:48-55` — hunk framing lives in the `Hunk` fields, so the
engine emitting a `LineHeader` would be a bug. Word spans use their own type:
`type SpanType int` (`internal/diff/diff.go:60`),
`SpanEqual` / `SpanDelete` / `SpanInsert` (`internal/diff/diff.go:63/64/65`),
`struct WordSpan` (`internal/diff/diff.go:76-79`).
Shapes: `struct Line` (`internal/diff/diff.go:82-86`),
`struct Hunk` with `OldStart`/`OldCount`/`NewStart`/`NewCount`
(`internal/diff/diff.go:89-95`),
`struct FileDiff` with `IsNew`/`IsDelete`/`IsBinary` flags
(`internal/diff/diff.go:98-105`),
`struct Engine` holding dmp instance, cache, and options
(`internal/diff/diff.go:108-112`).

Cache identity is a widened key, not a single hash: `struct cacheKey` carrying
two hashes plus both content lengths plus the context width
(`internal/diff/diff.go:121-129`), `struct contentFingerprint`
(`internal/diff/diff.go:132-136`), `func fingerprint(s string)` hashing twice in
one pass (`internal/diff/diff.go:141-157`).

Tuning surface: `struct Options` (`internal/diff/diff.go:162-191`) with
`ContextLines` (`internal/diff/diff.go:166`), `DisableCache`
(`internal/diff/diff.go:170`), `MaxCacheEntries` (`internal/diff/diff.go:173`),
`MaxCacheBytes` (`internal/diff/diff.go:177`), `Timeout`
(`internal/diff/diff.go:181`), and `VerifyCacheContent`
(`internal/diff/diff.go:190`), documented as off-by-default at
`internal/diff/diff.go:183-190`. Resolution: `method Options.contextLines`
(`internal/diff/diff.go:194-199`, zero means default, else clamped),
`method Options.timeout` (`internal/diff/diff.go:202-211`, zero means
`diffTimeout`, negative disables the bound). Construction:
`func NewEngine` (`internal/diff/diff.go:214-216`),
`func NewEngineWith` (`internal/diff/diff.go:220-230`, applying
`dmp.DiffTimeout = opts.timeout()` at `internal/diff/diff.go:224`),
`method Engine.Stats` (`internal/diff/diff.go:233-235`),
`var DefaultEngine` singleton (`internal/diff/diff.go:238`).

The entry point `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`)
runs, in order: empty-old-side sets `IsNew` (`internal/diff/diff.go:250-252`),
empty-new-side sets `IsDelete` (`internal/diff/diff.go:253-255`), NUL on either
side short-circuits to `IsBinary=true` with `markBinary()` and an empty hunk
list (`internal/diff/diff.go:261-265`), the cache key is built from both
fingerprints plus the resolved context width (`internal/diff/diff.go:272-278`),
a hit returns a deep copy with retargeted paths (`internal/diff/diff.go:280-288`),
and a miss runs `DiffLinesToChars` / `DiffMain` / `DiffCleanupSemantic` /
`DiffCharsToLines` (`internal/diff/diff.go:293-296`), converts to hunks
(`internal/diff/diff.go:299`), and caches a copy
(`internal/diff/diff.go:302-304`). The package-level `func ComputeDiff` is the
`DefaultEngine` convenience wrapper (`internal/diff/diff.go:310-312`).

Pipeline stages: `method Engine.convertToHunks`
(`internal/diff/diff.go:317-332`), `struct operation`
(`internal/diff/diff.go:335-340`), `method Engine.diffsToOperations`
(`internal/diff/diff.go:343-400`), `method Engine.groupIntoHunks`
(`internal/diff/diff.go:403-486`), `method Engine.computeHunkCounts`
(`internal/diff/diff.go:489-498`), retained FNV-1a `func hash`
(`internal/diff/diff.go:502-513`), concurrency-safe `method Engine.ClearCache`
that preserves cumulative counters (`internal/diff/diff.go:515-520`),
uncached `method Engine.ComputeWordLevelDiff`
(`internal/diff/diff.go:530-551`, no-cache rationale at
`internal/diff/diff.go:522-529`), and its package-level wrapper
(`internal/diff/diff.go:554-556`).

## 2. `internal/diff/cache.go` (265 lines) — bounded LRU with copy isolation

Bounds are explicit: `const defaultMaxCacheEntries` is `512`
(`internal/diff/cache.go:18`), `const defaultMaxCacheBytes` is `32 << 20`
(`internal/diff/cache.go:23`). Counters: `struct Stats`
(`internal/diff/cache.go:28-43`) with `Hits` (`internal/diff/cache.go:29`),
`Misses` (`internal/diff/cache.go:30`), `Computes`
(`internal/diff/cache.go:31`), `Binary` (`internal/diff/cache.go:32`),
`Evicted` (`internal/diff/cache.go:33`), `Entries` (`internal/diff/cache.go:34`),
`Bytes` (`internal/diff/cache.go:35`), `Collisions`
(`internal/diff/cache.go:42`).

Storage: `struct cacheEntry` (`internal/diff/cache.go:47-57`),
`struct diffCache` mutex-guarded LRU (`internal/diff/cache.go:61-75`),
`func newDiffCache` (`internal/diff/cache.go:77-90`).
`method diffCache.get` returns a `Clone()` and, under verification, drops
mismatched entries while counting collisions (`internal/diff/cache.go:96-121`).
`method diffCache.put` stores a `Clone()`, charges retained verify bytes, and
skips a single diff larger than the whole budget rather than thrashing the LRU
(`internal/diff/cache.go:126-172`). Eviction and lifecycle:
`method diffCache.evictLocked` (`internal/diff/cache.go:175-187`),
`method diffCache.clear` preserving counters (`internal/diff/cache.go:191-197`),
`method diffCache.markCompute` (`internal/diff/cache.go:199-203`),
`method diffCache.markBinary` (`internal/diff/cache.go:205-209`),
`method diffCache.stats` (`internal/diff/cache.go:211-224`).
Copy semantics: `method FileDiff.Clone` deep-copies hunks, lines, and values
(`internal/diff/cache.go:229-246`); `method FileDiff.approxSize` estimates
retained bytes for budget accounting (`internal/diff/cache.go:251-264`).

## 3. What runs today — the two wired consumers

Agent repair loop (`internal/session/turn_diff.go`): `func turnDiffSection`
(`internal/session/turn_diff.go:24-52`) returns `""` when nothing was written;
`func renderFileDiff` (`internal/session/turn_diff.go:55-76`) short-circuits
identical content (`internal/session/turn_diff.go:59-61`), diffs via package
`diff.ComputeDiff` — i.e. `DefaultEngine`
(`internal/session/turn_diff.go:62`), maps nil/zero-hunk results to `""`
(`internal/session/turn_diff.go:63-64`), composes the `@@ -%d,%d +%d,%d @@`
framing itself (`internal/session/turn_diff.go:69`), and renders markers via
`diffMarker(l.Type)` (`internal/session/turn_diff.go:71`).
`func newFileNote` marks only `IsNew` (`internal/session/turn_diff.go:78-83`);
`func diffMarker` maps Added to `+`, Removed to `-`, default to space
(`internal/session/turn_diff.go:85-94`).

TUI diff view (`cmd/nerd/ui/diffview.go`): one engine per package,
`var uiDiffEngine = diff.NewEngine()` (`cmd/nerd/ui/diffview.go:907`) with the
single-engine rationale (`cmd/nerd/ui/diffview.go:900-906`);
`func CreateDiffFromStrings` delegates to `uiDiffEngine.ComputeDiff`
(`cmd/nerd/ui/diffview.go:910-912`, call at `cmd/nerd/ui/diffview.go:911`);
`func DiffEngineStats` reports `uiDiffEngine.Stats()`
(`cmd/nerd/ui/diffview.go:915-917`, call at `cmd/nerd/ui/diffview.go:916`).
Each view holds the package engine: field `diffEngine *diff.Engine`
(`cmd/nerd/ui/diffview.go:146`), assigned as `diffEngine: uiDiffEngine` in the
constructor (`cmd/nerd/ui/diffview.go:181`). The side-by-side word path calls
`d.diffEngine.ComputeWordLevelDiff(line.Content, nextLine.Content)`
(`cmd/nerd/ui/diffview.go:970`). The single-engine invariant is pinned by
`func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView`
(`cmd/nerd/ui/word_highlight_test.go:141-162`), witnessed by the verification
pass on 2026-09-21.

## 4. What this file does NOT claim

Anything not opened above stays out of the shipped record: remaining test-file
bodies and their exact pins, callers of `Engine.ClearCache`, runtime behavior
of opt-in `VerifyCacheContent` / nonzero `Stats.Collisions`, and any truncation
or size budget on the consumer side. Those belong in `03-GAP-ANALYSIS.md`, not
here. On any disagreement between this file and prose elsewhere,
`IMPLEMENTED_SPEC.md` wins once it exists; until then, the cited lines above
are the authority.
