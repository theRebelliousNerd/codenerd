---
doc-class: shipped
subsystem: diff
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# Diff — Implemented Spec (shipped)

This file is the authoritative record of what `internal/diff` does today.
On any disagreement with another document in `Docs/architecture/diff/`
(`README.md`, `INTERNALS.md`, `WIRING-AND-NOT-BUILT.md`, `01-VISION.md`,
`04-PRINCIPLES-AND-CONSTRAINTS.md`), **this file wins**.

Every claim below cites a repo-relative path plus a symbol and line read
2026-09-21. Claims without such a citation are not shipped claims.

Vision trace: this package serves `agents.md:43-46` — tools offload cognition
to deterministic code and reduce the turns to complete the task. The seams are
`turnDiffSection`/`renderFileDiff` showing a repair round its own edits
(`internal/session/turn_diff.go:24-52`, call at `:62`) and the TUI diff view
(`cmd/nerd/ui/diffview.go:907`, `911`, `915-917`, `970`).

## 1. Package shape

The package is two production files: `internal/diff/diff.go` (557 lines) and
`internal/diff/cache.go` (265 lines). The only third-party import is
`sergi/go-diff/diffmatchpatch` at `internal/diff/diff.go:9`
(`import diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"`).
Diff computation is offloaded to that library, not hand-rolled: the call chain
`DiffLinesToChars` / `DiffMain(a,b,false)` / `DiffCleanupSemantic` /
`DiffCharsToLines` runs inside `Engine.ComputeDiff` at
`internal/diff/diff.go:293-296`.

## 2. Bounded cost: timeout and clamped context

- `const diffTimeout` at `internal/diff/diff.go:14` is `5 * time.Second`.
- `const defaultContextLines` at `internal/diff/diff.go:17` is `3`.
- `const maxContextLines` at `internal/diff/diff.go:20` is `1000`.
- `func clampContextLines` at `internal/diff/diff.go:30-38` bounds its input
  to `[0,1000]`.
- `method Options.contextLines` at `internal/diff/diff.go:194-199` returns
  `defaultContextLines` on zero, else the clamped value.
- `method Options.timeout` at `internal/diff/diff.go:202-211` returns
  `diffTimeout` on zero and genuinely-zero (no timeout) on negative.
- `func NewEngineWith` at `internal/diff/diff.go:220-230` applies the resolved
  timeout at `:224` (`dmp.DiffTimeout = opts.timeout()`).
- `func NewEngine` at `internal/diff/diff.go:214-216` is
  `NewEngineWith(Options{})`; the zero `Options` at
  `internal/diff/diff.go:162-191` selects the defaults (`struct Options`,
  fields `ContextLines:166`, `DisableCache:170`, `MaxCacheEntries:173`,
  `MaxCacheBytes:177`, `Timeout:181`, `VerifyCacheContent:190`).

## 3. File diff pipeline (`Engine.ComputeDiff`)

`method Engine.ComputeDiff` at `internal/diff/diff.go:243-307` executes in
this order; `func ComputeDiff` at `internal/diff/diff.go:310-312` is the
`DefaultEngine` wrapper (`var DefaultEngine` at `internal/diff/diff.go:238`):

1. Empty-side flags: `oldContent == ""` sets `IsNew` at `:250-252`;
   `newContent == ""` sets `IsDelete` at `:253-255`. Flags live on
   `struct FileDiff` at `internal/diff/diff.go:98-105`
   (`IsNew`, `IsDelete`, `IsBinary`).
2. Binary short-circuit at `:261-265`: `func containsNullByte` at
   `internal/diff/diff.go:24-26` (NUL sentinel) on either side sets
   `IsBinary=true`, calls `markBinary`, and returns an empty hunk list —
   never hunks. Counter `method diffCache.markBinary` at
   `internal/diff/cache.go:205-209`; `Stats.Binary` at
   `internal/diff/cache.go:32`.
3. Cache lookup: context width resolved at `:267`; fingerprints at `:272-273`
   via `func fingerprint` at `internal/diff/diff.go:141-157` (dual FNV-1a,
   one pass); key assembled at `:274-278` as `struct cacheKey` at
   `internal/diff/diff.go:121-129` (two hashes plus both lengths plus
   `contextLines`); `get` at `:280-288` returns a deep copy retargeted to the
   caller's paths at `:284-285`.
4. Compute on miss: `markCompute` then the `sergi/go-diff` chain at `:292-296`
   (`DiffLinesToChars`, `DiffMain`, `DiffCleanupSemantic`,
   `DiffCharsToLines`).
5. Hunk grouping: `fileDiff.Hunks = e.convertToHunks(diffs, contextLines)` at
   `:299` (`method Engine.convertToHunks` at `internal/diff/diff.go:317-332`).
6. Cache store at `:302-304`: `put` with `e.opts.VerifyCacheContent`
   (`method diffCache.put` at `internal/diff/cache.go:126-172`); the caller
   keeps sole ownership of the returned value.

`method Engine.ClearCache` at `internal/diff/diff.go:518-520` drops entries
without reassigning the cache field (concurrency-safe) and preserves
cumulative counters (comment at `:515-517`).
`method Engine.Stats` at `internal/diff/diff.go:233-235` returns
`e.cache.stats()`.

## 4. Cache: bounded LRU with deep-copy isolation

- Bounds: `const defaultMaxCacheEntries` at `internal/diff/cache.go:18` is
  `512`; `const defaultMaxCacheBytes` at `internal/diff/cache.go:23` is
  `32 << 20` (32 MiB). Whichever bound trips first evicts.
- `struct Stats` at `internal/diff/cache.go:28-43` (`Hits:29`, `Misses:30`,
  `Computes:31`, `Binary:32`, `Evicted:33`, `Entries:34`, `Bytes:35`,
  `Collisions:42`). `Collisions` counts verification rejections only.
- `method diffCache.get` at `internal/diff/cache.go:96-121` returns
  `entry.diff.Clone()` at `:120`; a verified entry whose stored content
  differs is dropped and counted as miss plus collision at `:109-116`.
- `method diffCache.put` at `internal/diff/cache.go:126-172` stores
  `fd.Clone()` at `:130`, charges retained verify content to the byte budget
  at `:132-136`, and skips a single oversize diff rather than thrashing the
  LRU at `:159-161`.
- `method diffCache.evictLocked` at `internal/diff/cache.go:175-187` drops
  least-recently-used entries until both bounds hold.
- `method diffCache.clear` at `internal/diff/cache.go:191-197` preserves
  counters; `method diffCache.markCompute` at
  `internal/diff/cache.go:199-203` advances the compute counter.
- `method FileDiff.Clone` at `internal/diff/cache.go:229-246` deep-copies
  hunks and lines; `method FileDiff.approxSize` at
  `internal/diff/cache.go:251-264` charges line content plus per-line
  (`:253`) and per-hunk (`:254`) overhead.
- Trust is opt-in: `Options.VerifyCacheContent` at
  `internal/diff/diff.go:183-190` is off by default and roughly doubles cache
  memory; the widened two-hash key (`internal/diff/diff.go:114-120` comment,
  `struct cacheKey` at `:121-129`, `struct contentFingerprint` at `:132-136`)
  is the default collision defense.

## 5. Line model: the engine never emits `LineHeader`

- `type LineType` at `internal/diff/diff.go:41`; `LineContext` at `:44`,
  `LineAdded` at `:45`, `LineRemoved` at `:46`; `LineHeader` at `:48-56`
  with the comment that the engine emitting one would be a bug — framing
  lives in `struct Hunk` at `internal/diff/diff.go:89-95`
  (`OldStart/OldCount/NewStart/NewCount`) and `struct Line` at
  `internal/diff/diff.go:82-86`.
- Callers compose their own `@@` framing: `internal/session/turn_diff.go:69`
  (`fmt.Fprintf @@ -%d,%d +%d,%d`) and map line kinds via
  `func diffMarker` at `internal/session/turn_diff.go:85-94`
  (Added→`+`, Removed→`-`, default→space), used at `:71`.
- Turn-diff guards: `func turnDiffSection` at
  `internal/session/turn_diff.go:24-52` returns `""` when nothing was written;
  `func renderFileDiff` at `internal/session/turn_diff.go:55-76`
  short-circuits identical content at `:59-61` and nil/zero-hunk results at
  `:63-64`; `func newFileNote` at `internal/session/turn_diff.go:78-83`
  marks only `IsNew`.

## 6. Word-level diff is uncached spans in an own type

- `struct WordSpan` at `internal/diff/diff.go:76-79`
  (`Type SpanType; Text string`); `type SpanType` at
  `internal/diff/diff.go:60` with `SpanEqual` at `:63`, `SpanDelete` at `:64`,
  `SpanInsert` at `:65`. The comment at `:68-75` states this replaces the raw
  `diffmatchpatch.Diff` return so consumers never import `sergi/go-diff`.
- `method Engine.ComputeWordLevelDiff` at `internal/diff/diff.go:530-551`
  runs `DiffMain` plus `DiffCleanupSemantic` at `:531-532` and is explicitly
  uncached (comment at `:527-529`); `func ComputeWordLevelDiff` at
  `internal/diff/diff.go:554-556` is the `DefaultEngine` wrapper.

## 7. One shared engine per use-site

- Agent loop: `internal/session/turn_diff.go:62`
  (`fd := diff.ComputeDiff(path, path, before, after)`) runs on
  `DefaultEngine` (`internal/diff/diff.go:310-312`).
- TUI: `var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907`
  (`diff.NewEngine()`, rationale at `:897-906` — one engine removes the
  two-cache surprise and is safe via the internal mutex);
  `func CreateDiffFromStrings` at `cmd/nerd/ui/diffview.go:910-912` diffs on
  that engine (`uiDiffEngine.ComputeDiff` at `:911`);
  `func DiffEngineStats` at `cmd/nerd/ui/diffview.go:915-917` reports that
  engine's counters (`uiDiffEngine.Stats()` at `:916`).
- The side-by-side word path calls `d.diffEngine.ComputeWordLevelDiff` at
  `cmd/nerd/ui/diffview.go:970`.

## 8. What this file does not claim

The following are deliberately absent: exact test names and line numbers in
`internal/diff/*_test.go` and `cmd/nerd/ui/word_highlight_test.go`;
absence claims about who never calls `NewEngineWith`, `ClearCache`, or
`DiffEngineStats` in production; and any truncation or size budget for
prompt-bound rendering. Those belong in `03-GAP-ANALYSIS.md` once witnessed,
not here.
