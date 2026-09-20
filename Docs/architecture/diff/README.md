# internal/diff

Line diffs for the agent loop and the TUI, computed with `sergi/go-diff`
(`diffmatchpatch`) and cached in a bounded in-process LRU.

Verified 2026-09-20 against commit `231cfa7` (`main`). The package is two
production files — `internal/diff/diff.go` (557 lines) and
`internal/diff/cache.go` (265 lines) — plus five test files
(`benchmark_test.go`, `cache_test.go`, `diff_comprehensive_test.go`,
`diff_test.go`, `word_span_test.go`).

## Pipeline in one paragraph

`ComputeDiff` (`internal/diff/diff.go:243-307`) short-circuits binary input
(`containsNullByte`, `internal/diff/diff.go:24-26`), looks the pair up in the
cache under a key of two content hashes plus both lengths plus the context
width (`fingerprint`, `internal/diff/diff.go:141-157`; `cacheKey`,
`internal/diff/diff.go:121-129`), and on a miss runs
`DiffLinesToChars`/`DiffMain`/`DiffCleanupSemantic`/`DiffCharsToLines`,
converts the result to line operations (`diffsToOperations`,
`internal/diff/diff.go:343-400`), groups them into hunks with context
(`groupIntoHunks`, `internal/diff/diff.go:403-486`), and caches a deep copy.
`ComputeWordLevelDiff` (`internal/diff/diff.go:530-551`) is a separate,
uncached per-line-pair span comparison returning `WordSpan` values.

## Public API

| Symbol | Where | Notes |
|---|---|---|
| `Engine` | `internal/diff/diff.go:108-112` | holds dmp instance, cache, options |
| `Options` | `internal/diff/diff.go:162-191` | `ContextLines`, `DisableCache`, `MaxCacheEntries`, `MaxCacheBytes`, `Timeout`, `VerifyCacheContent` |
| `NewEngine` / `NewEngineWith` | `internal/diff/diff.go:214-230` | zero `Options` == defaults; timeout default 5s (`diffTimeout`, `internal/diff/diff.go:14`) |
| `DefaultEngine` | `internal/diff/diff.go:238` | singleton behind the package-level functions |
| `(*Engine).ComputeDiff` / `ComputeDiff` | `internal/diff/diff.go:243-312` | file diff with caching |
| `(*Engine).ComputeWordLevelDiff` / `ComputeWordLevelDiff` | `internal/diff/diff.go:530-556` | uncached word spans |
| `(*Engine).ClearCache` / `(*Engine).Stats` | `internal/diff/diff.go:518-520`, `233-235` | clear keeps cumulative counters; `Stats` is `internal/diff/cache.go:28-43` |
| `FileDiff` / `Hunk` / `Line` | `internal/diff/diff.go:98-105`, `89-95`, `82-86` | flags `IsNew`, `IsDelete`, `IsBinary` on `FileDiff` |
| `LineContext` / `LineAdded` / `LineRemoved` / `LineHeader` | `internal/diff/diff.go:43-57` | the engine never emits `LineHeader`; it is UI-owned |
| `WordSpan` (`SpanEqual`/`SpanDelete`/`SpanInsert`) | `internal/diff/diff.go:62-79` | replaced the old raw `diffmatchpatch.Diff` return |

## Consumers

- `internal/session/turn_diff.go:62` — `renderFileDiff` calls package-level
  `diff.ComputeDiff` (i.e. `DefaultEngine`) to show a repair round its own edits.
- `cmd/nerd/ui/diffview.go:907-917` — the TUI keeps one private `uiDiffEngine`
  (`diff.NewEngine()`), exposed via `CreateDiffFromStrings` and
  `DiffEngineStats`; `cmd/nerd/ui/word_highlight_test.go:141-159` pins that the
  view and the helper share that single engine.

## Further reading

- `INTERNALS.md` — pipeline stages, cache design, and the invariants the tests pin.
- `WIRING-AND-NOT-BUILT.md` — what is reachable, what exists but nothing calls,
  and what the design assumes that the code does not do.
