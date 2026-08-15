# 06 — Public API and Types: `internal/diff`

> Last verified against codebase: 2026-08-15  
> All symbols live in `internal/diff/diff.go` unless noted (`Stats` and the cache live in `internal/diff/cache.go`).

## 1. Import

```go
import "codenerd/internal/diff"
```

## 2. Enums

### `LineType`

| Constant | Value | Produced by engine? | Typical render |
|----------|------:|---------------------|----------------|
| `LineContext` | 0 | Yes | `  content` |
| `LineAdded` | 1 | Yes | `+ content` |
| `LineRemoved` | 2 | Yes | `- content` |
| `LineHeader` | 3 | **No, by decision** | synthesized by the renderer for its own `@@` row |

## 3. Structs

### `Line`

| Field | Type | Notes |
|-------|------|-------|
| `LineNum` | `int` | 1-based display number |
| `Content` | `string` | Without terminating newline |
| `Type` | `LineType` | |

### `Hunk`

| Field | Type | Notes |
|-------|------|-------|
| `OldStart` | `int` | 1-based (or 0 edge) |
| `OldCount` | `int` | context + removed |
| `NewStart` | `int` | 1-based (or 0 edge) |
| `NewCount` | `int` | context + added |
| `Lines` | `[]Line` | Ordered |

### `FileDiff`

| Field | Type | Notes |
|-------|------|-------|
| `OldPath` | `string` | Label only |
| `NewPath` | `string` | Label only |
| `Hunks` | `[]Hunk` | May be empty |
| `IsNew` | `bool` | `oldContent == ""` |
| `IsDelete` | `bool` | `newContent == ""` |
| `IsBinary` | `bool` | NUL in either content |

### `SpanType` / `WordSpan`

| Constant | Meaning |
|----------|---------|
| `SpanEqual` | run present on both sides |
| `SpanDelete` | run present only on the old side |
| `SpanInsert` | run present only on the new side |

| Field | Type | Notes |
|-------|------|-------|
| `Type` | `SpanType` | |
| `Text` | `string` | never empty |

Equal + delete spans concatenate to the old line exactly; equal + insert spans
concatenate to the new line exactly. Renderers rely on that to paint one side
without re-splitting text.

### `Options`

| Field | Zero value means | Notes |
|-------|------------------|-------|
| `ContextLines` | `defaultContextLines` (3) | negative for genuinely zero context |
| `DisableCache` | caching on | counters still advance |
| `MaxCacheEntries` | 512 | LRU bound |
| `MaxCacheBytes` | 32 MiB | approximate resident payload bound |
| `Timeout` | 5s | negative disables the bound |
| `VerifyCacheContent` | off | retains exact inputs and byte-compares on hit |

### `Stats`

`Hits`, `Misses`, `Computes`, `Binary`, `Evicted`, `Entries`, `Bytes`,
`Collisions`. Cumulative since engine construction; `ClearCache` does not reset
them. `Collisions` is nonzero only under `VerifyCacheContent` and means a key
match failed content verification.

### `Engine`

| Field | Visibility | Type | Notes |
|-------|------------|------|-------|
| `dmp` | unexported | `*diffmatchpatch.DiffMatchPatch` | Timeout set in `NewEngineWith` |
| `cache` | unexported | `*diffCache` | Mutex-guarded LRU of deep-copied `*FileDiff` |
| `opts` | unexported | `Options` | |

Callers must not assume struct layout stability for unexported fields.

## 4. Construction

### `func NewEngine() *Engine`

Creates a fresh engine:

- `diffmatchpatch.New()`
- `DiffTimeout = 5s`
- empty bounded LRU

### `func NewEngineWith(opts Options) *Engine`

Same, tuned by `opts`. `NewEngineWith(Options{})` is identical to `NewEngine()`.

### `var DefaultEngine = NewEngine()`

Package-level singleton. Used by package `ComputeDiff`.

## 5. File-level compute

### `func (e *Engine) ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff`

| Arg | Role |
|-----|------|
| `oldPath` / `newPath` | Labels copied into result (and rewritten on cache hit) |
| `oldContent` / `newContent` | Full text; empty string drives IsNew/IsDelete |

**Returns:** non-nil `*FileDiff` on normal operation.

**Side effects:** may read/write engine cache (except binary short-circuit path, which does
not store).

### `func ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff`

Delegates to `DefaultEngine.ComputeDiff`.

## 6. Word-level compute

### `func (e *Engine) ComputeWordLevelDiff(oldLine, newLine string) []WordSpan`

| Arg | Role |
|-----|------|
| `oldLine` / `newLine` | Single-line (or arbitrary string) pair |

**Returns:** `[]WordSpan` after semantic cleanup, in old-then-new reading order.  
**Does not** use the content cache.  
**Coupling:** none — `diffmatchpatch` no longer appears in this package's public
API, so a consumer never imports sergi to read a result.

### `func ComputeWordLevelDiff(oldLine, newLine string) []WordSpan`

Delegates to `DefaultEngine`.

## 7. Cache control

### `func (e *Engine) ClearCache()`

Empties this engine's LRU in place (safe against a concurrent `ComputeDiff`) and
preserves cumulative `Stats`. Does not clear other engines or `DefaultEngine`
unless called on that instance.

### `func (e *Engine) Stats() Stats`

Snapshot of this engine's counters.

## 8. Convenience usage examples

### Minimal

```go
fd := diff.ComputeDiff("a.go", "a.go", oldSrc, newSrc)
if fd.IsBinary {
    // show binary notice
}
for _, h := range fd.Hunks {
    // render @@ -OldStart,OldCount +NewStart,NewCount @@
}
```

### Isolated engine (preferred in long-lived UI)

```go
eng := diff.NewEngine()
fd := eng.ComputeDiff(path, path, before, after)
// ...
eng.ClearCache()
```

### UI alias pattern (existing)

```go
// cmd/nerd/ui/diffview.go
type FileDiff = diff.FileDiff
const DiffLineAdded = diff.LineAdded
```

### CreateDiffFromStrings bridge

```go
// cmd/nerd/ui/diffview.go
func CreateDiffFromStrings(oldPath, newPath, oldContent, newContent string) *FileDiff {
    return diff.ComputeDiff(oldPath, newPath, oldContent, newContent)
}
```

The bridge and every `DiffApprovalView` now share `ui.uiDiffEngine`, so file
diffs and word diffs land in one cache. `ui.DiffEngineStats()` reports it.

## 9. Not exported (do not depend on from other packages)

- `hash`, `fingerprint`, `contentFingerprint`, `operation`, `cacheKey`, `diffCache`
- `convertToHunks`, `diffsToOperations`, `groupIntoHunks`, `computeHunkCounts`
- `containsNullByte`, `clampContextLines`
- Constants `diffTimeout`, `defaultContextLines`, `maxContextLines` (unexported)

Tests in package `diff` may call unexported helpers; external packages must not.
