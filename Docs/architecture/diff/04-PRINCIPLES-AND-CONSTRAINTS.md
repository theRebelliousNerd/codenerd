---
doc-class: governance
subsystem: internal/diff
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# 04 Principles and Constraints — internal/diff

Governance for `internal/diff`: numbered constraints any change to this package
must respect, each with the code or ruling it comes from. Shipped behavior
claims cite repo-relative path plus symbol and line, verified this turn
against `internal/diff/diff.go` (557 lines), `internal/diff/cache.go`
(265 lines), `internal/session/turn_diff.go`, `cmd/nerd/ui/diffview.go`,
and `agents.md`.

Vision trace: this package serves `agents.md:43-45` — "Tools exist for exactly
three things: condense the search space, reduce the turns to complete the task,
and offload cognition to deterministic code (a change with a blast radius is
carried out by the tool, not by the LLM hand-editing)." The engine offloads
diff computation to deterministic Go code and its two production consumers
show a repair round and a reviewer its own edits, reducing re-diagnosis turns.

## 1. Diff computation is offloaded to deterministic `sergi/go-diff`, not hand-rolled or LLM-edited

Constraint: line-diff semantics come from the third-party diffmatchpatch
engine plus a small line-op/hunk layer. Do not reimplement LCS in-package and
do not let the model hand-edit diff output.

- `internal/diff/diff.go:9` — `import diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"`, the only third-party diff import in the package.
- `internal/diff/diff.go:243-307` — `func (e *Engine) ComputeDiff` owns the pipeline: flags, binary short-circuit, cache lookup, compute, cache store.
- `internal/diff/diff.go:293-296` — inside `ComputeDiff`: `DiffLinesToChars` / `DiffMain(a, b, false)` / `DiffCleanupSemantic` / `DiffCharsToLines` via `e.dmp`.
- `internal/diff/diff.go:343-400` — `func (e *Engine) diffsToOperations` converts character diffs to line operations; `internal/diff/diff.go:403-486` — `func (e *Engine) groupIntoHunks` groups operations into hunks; `internal/diff/diff.go:317-332` — `func (e *Engine) convertToHunks` connects the two with clamped context.
- Witness consumers: `internal/session/turn_diff.go:62` — `fd := diff.ComputeDiff(path, path, before, after)` shows a repair round its own edits; `cmd/nerd/ui/diffview.go:907` — `var uiDiffEngine = diff.NewEngine()` and `cmd/nerd/ui/diffview.go:910-912` — `func CreateDiffFromStrings` calls `uiDiffEngine.ComputeDiff`.

## 2. Bounded cost: timeout plus clamped context

Constraint: a single diff is time-bounded and its context width is clamped, so
pathological inputs (massive minified single-line files, fuzzer extremes,
`MaxInt` context) cannot drive unbounded work or degenerate hunk grouping.

- `internal/diff/diff.go:14` — `const diffTimeout = 5 * time.Second` bounds pathological inputs.
- `internal/diff/diff.go:17` — `const defaultContextLines = 3` fallback width; `internal/diff/diff.go:20` — `const maxContextLines = 1000` upper bound.
- `internal/diff/diff.go:30-38` — `func clampContextLines(n int)` bounds to `[0, maxContextLines]`.
- `internal/diff/diff.go:194-199` — `func (o Options) contextLines()` maps zero to default, otherwise clamps; `internal/diff/diff.go:202-211` — `func (o Options) timeout()` maps zero to default and negative to no-timeout.
- `internal/diff/diff.go:220-230` — `func NewEngineWith(opts Options)` sets `dmp.DiffTimeout = opts.timeout()`.
- `internal/diff/diff.go:214-216` — `func NewEngine()` equals `NewEngineWith(Options{})` zero-value defaults.

## 3. Binary short-circuits to `IsBinary`, never hunks

Constraint: NUL byte on either side means binary. Return `IsBinary=true` with
an empty hunk list instead of sending blobs through diffmatchpatch, which
would yield garbage hunks and ruinous memory/time on large blobs.

- `internal/diff/diff.go:24-26` — `func containsNullByte(s string)` NUL sentinel.
- `internal/diff/diff.go:261-265` — inside `ComputeDiff`: NUL on either side sets `IsBinary=true`, calls `markBinary()`, returns with no hunks.
- `internal/diff/diff.go:98-105` — `struct FileDiff` flags `IsNew` / `IsDelete` / `IsBinary`; `internal/diff/diff.go:250-255` — empty-side flagging for `IsNew` / `IsDelete`.
- `internal/diff/cache.go:205-209` — `func (c *diffCache) markBinary` counter; `internal/diff/cache.go:28-43` — `struct Stats` with `Binary` at `cache.go:32`.

## 4. Cache is bounded LRU with deep-copy isolation; trust is opt-in

Constraint: the cache is a bounded LRU behind one mutex, evicting on entries
or bytes, never sharing mutable state with callers. Keys are trusted by
default; byte-verified proof is opt-in for apply-not-display paths because it
roughly doubles resident memory.

- `internal/diff/cache.go:15-24` — cache bounds comment; `internal/diff/cache.go:18` — `defaultMaxCacheEntries = 512`; `internal/diff/cache.go:23` — `defaultMaxCacheBytes = 32 << 20` (32 MiB).
- `internal/diff/diff.go:121-129` — `struct cacheKey` (two hashes plus both content lengths plus context width); `internal/diff/diff.go:132-136` — `struct contentFingerprint`; `internal/diff/diff.go:141-157` — `func fingerprint(s string)` dual FNV-1a in one pass.
- `internal/diff/cache.go:96-121` — `func (c *diffCache) get` returns `Clone()`; `internal/diff/cache.go:109-116` — verified entries byte-compare and drop on mismatch; `internal/diff/cache.go:110` — mismatch counts `collisions`.
- `internal/diff/cache.go:126-172` — `func (c *diffCache) put` stores `Clone()` at `cache.go:130`; `internal/diff/cache.go:132-136` — retained verify content charged to byte budget; `internal/diff/cache.go:159-161` — single diff larger than budget is not cached rather than thrashing the LRU.
- `internal/diff/cache.go:175-187` — `func (c *diffCache) evictLocked` drops LRU until both bounds hold; `internal/diff/cache.go:191-197` — `func (c *diffCache) clear` preserves cumulative counters; `internal/diff/cache.go:199-203` — `func (c *diffCache) markCompute`; `internal/diff/cache.go:211-224` — `func (c *diffCache) stats`; `internal/diff/cache.go:229-246` — `func (fd *FileDiff) Clone` deep copy; `internal/diff/cache.go:251-264` — `func (fd *FileDiff) approxSize` accounting.
- `internal/diff/diff.go:183-190` — `Options.VerifyCacheContent` off-by-default, doubles memory, for apply rather than display.
- `internal/diff/diff.go:233-235` — `func (e *Engine) Stats`; `internal/diff/diff.go:518-520` — `func (e *Engine) ClearCache` safe concurrent, preserves counters; `internal/diff/diff.go:238` — `var DefaultEngine` singleton.

## 5. Engine never emits `LineHeader`; framing lives in `Hunk`, rendering in caller

Constraint: hunk framing is data (`Hunk` fields), never a `Line`. The engine
emitting `LineHeader` is a bug. Renderers compose their own `@@` headers and
synthesize their own header rows with the UI-owned enum member.

- `internal/diff/diff.go:43-57` — `LineContext` at `:44`, `LineAdded` at `:45`, `LineRemoved` at `:46`, `LineHeader` at `:56`, with comment at `internal/diff/diff.go:48-55` stating the engine never produces one and a renderer composes its own header.
- `internal/diff/diff.go:89-95` — `struct Hunk` (`OldStart` / `OldCount` / `NewStart` / `NewCount`); `internal/diff/diff.go:82-86` — `struct Line`; `internal/diff/diff.go:60-66` — `SpanType` classification for word spans.
- `internal/session/turn_diff.go:69` — `fmt.Fprintf @@ -%d,%d +%d,%d` composes the header in the caller; `internal/session/turn_diff.go:85-94` — `func diffMarker` maps `LineAdded` to `+` and `LineRemoved` to `-`.

## 6. Word-level diff is uncached per-line spans in its own type, never a third-party type

Constraint: word diffs are per visible line-pair, cheap relative to a file
diff, and returned as codeNERD spans. Do not cache them and do not leak
`diffmatchpatch.Diff` across the public API, which would force every consumer
to import `sergi/go-diff`.

- `internal/diff/diff.go:76-79` — `struct WordSpan{Type SpanType; Text string}` with replacing-raw-return comment at `internal/diff/diff.go:70-75`.
- `internal/diff/diff.go:530-551` — `func (e *Engine) ComputeWordLevelDiff` returns old-then-new spans (`SpanEqual` plus `SpanDelete` / `SpanInsert`); `internal/diff/diff.go:554-556` — package wrapper `func ComputeWordLevelDiff`.
- `cmd/nerd/ui/diffview.go:970` — `d.diffEngine.ComputeWordLevelDiff(line.Content, nextLine.Content)` per-pair consumer for word highlights.

## 7. One shared engine per use-site; stats describe that engine

Constraint: each runtime use-site owns exactly one engine so cache lifetime
and `Stats` describe the work that site actually did. Do not split a site
across two engines with different lifetimes, and do not read another engine's
counters as your own.

- `internal/session/turn_diff.go:62` — agent loop uses package `diff.ComputeDiff`, which is `DefaultEngine` per `internal/diff/diff.go:310-312` — `func ComputeDiff` delegates to `DefaultEngine`.
- `cmd/nerd/ui/diffview.go:907` — `var uiDiffEngine = diff.NewEngine()` single UI engine; `cmd/nerd/ui/diffview.go:910-912` — `func CreateDiffFromStrings` computes on it; `cmd/nerd/ui/diffview.go:914-916` — `func DiffEngineStats() diff.Stats` reports its cumulative counters.
- `cmd/nerd/ui/diffview.go:897-907` — rationale comment: two caches diverged, one mutex-guarded engine removes the surprise.
- `internal/session/turn_diff.go:24-51` — `func turnDiffSection` returns `""` when nothing written; `internal/session/turn_diff.go:55-61` — `func renderFileDiff` identical-content short-circuit; `internal/session/turn_diff.go:63-64` — nil or zero-hunk diff renders `""`; `internal/session/turn_diff.go:78-83` — `func newFileNote` marks only `IsNew`.
