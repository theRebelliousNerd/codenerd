# 06 — Public API and Types: `internal/diff`

> Last verified against codebase: 2026-07-13  
> All symbols live in `internal/diff/diff.go` unless noted.

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
| `LineHeader` | 3 | **No** | reserved for UI |

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

### `Engine`

| Field | Visibility | Type | Notes |
|-------|------------|------|-------|
| `dmp` | unexported | `*diffmatchpatch.DiffMatchPatch` | Timeout set in `NewEngine` |
| `cache` | unexported | `sync.Map` | Keys `cacheKey`, values `*FileDiff` |

Callers must not assume struct layout stability for unexported fields.

## 4. Construction

### `func NewEngine() *Engine`

Creates a fresh engine:

- `diffmatchpatch.New()`
- `DiffTimeout = 5s`
- empty `sync.Map`

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

### `func (e *Engine) ComputeWordLevelDiff(oldLine, newLine string) []diffmatchpatch.Diff`

| Arg | Role |
|-----|------|
| `oldLine` / `newLine` | Single-line (or arbitrary string) pair |

**Returns:** sergi `[]Diff` after semantic cleanup.  
**Does not** use the content cache.  
**Coupling:** callers import `github.com/sergi/go-diff/diffmatchpatch` only if they inspect
`Diff.Type` / `Diff.Text` beyond opaque use — UI currently treats the slice as opaque-ish
with incomplete highlighting.

## 7. Cache control

### `func (e *Engine) ClearCache()`

Replaces `e.cache` with a new empty `sync.Map`. Does not clear other engines or
`DefaultEngine` unless called on that instance.

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

Note: bridge uses **DefaultEngine**, while the view’s word-level path uses a private
`diff.NewEngine()` — two caches.

## 9. Not exported (do not depend on from other packages)

- `hash`, `operation`, `cacheKey`
- `convertToHunks`, `diffsToOperations`, `groupIntoHunks`, `computeHunkCounts`
- `containsNullByte`, `clampContextLines`
- Constants `diffTimeout`, `defaultContextLines`, `maxContextLines` (unexported)

Tests in package `diff` may call unexported helpers; external packages must not.
