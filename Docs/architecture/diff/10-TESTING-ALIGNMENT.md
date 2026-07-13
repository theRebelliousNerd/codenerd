# 10 — Testing Alignment: `internal/diff`

> Last verified against codebase: 2026-07-13

## 1. Test assets

| File | ≈Lines | Style |
|------|-------:|-------|
| `internal/diff/diff_test.go` | 484 | Table-ish sequential tests, boundary suite (2026-05-24), benchmarks |
| `internal/diff/diff_comprehensive_test.go` | 466 | `t.Parallel()` comprehensive suite |

No external fixtures directory; all content is inline strings / generated lines.

## 2. Coverage map

| Behavior | Tests (representative) | Strength |
|----------|------------------------|----------|
| Simple addition | `TestComputeDiff_SimpleAddition`, `…WhenSingleLineAdded…` | Strong |
| Simple deletion | `TestComputeDiff_SimpleDeletion`, `…WhenSingleLineRemoved…` | Strong |
| Modification (remove+add) | `…WhenSingleLineModified…` | Strong |
| New file | `TestComputeDiff_NewFile`, `…WhenOldEmpty…` | Strong |
| Delete file | `TestComputeDiff_DeletedFile`, `…WhenNewEmpty…` | Strong |
| Both empty | `TestComputeDiff_EmptyStrings`, `…WhenBothEmpty…` | Strong |
| Identical | `TestComputeDiff_NoChanges`, `…WhenIdentical…` | Strong |
| Multi-hunk distance | `TestComputeDiff_MultipleHunks` | Medium (asserts ≥1, not exact 2) |
| Context present | `TestComputeDiff_ContextLines` | Medium |
| Cache + path rewrite | `TestComputeDiff_Caching`, `…WhenCached…` | Strong |
| ClearCache | `TestClearCache_ShouldNotAffectResults` | Strong |
| Empty lines | `TestComputeDiff_EmptyLines` | Medium |
| Large file | `TestComputeDiff_LargeFile` (1k), `…WhenLargeFile…` (5k) | Strong |
| Hunk counts | `TestComputeDiff_HunkCounts`, `…WhenHunkCounts…` | Strong |
| Word-level | `TestComputeWordLevelDiff*`, comprehensive siblings | Strong |
| Convenience API | `TestComputeDiff_ConvenienceFunction…` | Strong |
| Binary NUL | `TestComputeDiff_BinaryContent`, `…WhenBinary…` | Strong |
| Huge/negative context | `TestComputeDiff_HugeContext` | Strong |
| Empty paths | `TestComputeDiff_EmptyPaths` | Strong |
| Concurrency | `TestComputeDiff_WhenConcurrent_ShouldNotRace` | Strong |
| Hash determinism | `TestHash_*` | Strong |
| Whitespace change | `…WhenOnlyWhitespaceChanges…` | Medium |
| Trailing newline | `…WhenTrailingNewlineDiffers…` | Weak (nil-check only) |
| Benchmarks | `BenchmarkComputeDiff_{Small,Large,WithCache}` | Present |

## 3. Explicit TEST_GAP TODOs (from source comments)

Documented in `diff_test.go` headers and footers (partial list):

- Hash collisions / empty-string collision edge  
- Shallow copy mutation of cached hunks  
- ClearCache vs concurrent Store race  
- Cache OOM under 100k unique pairs  
- Massive pure deletion allocation  
- Minified monolith hang (timeout exists; hard guarantee tests limited)  
- Invalid UTF-8 / unpaired surrogates  
- Extreme path strings (`\n`, huge length) for *serialization* (engine preserves; no serializer)  
- Context boundary exactly `2 * contextLines` merge behavior  

Some older TODOs partially closed by later tests (binary, both-empty) — keep corpus honest:
treat checklist as living, re-verify against source comments.

## 4. Downstream tests

`cmd/nerd/ui/word_diff_test.go` exercises package `ComputeDiff` and `Line` types through
the UI rendering path — integration-ish, not a substitute for package unit tests.

## 5. Commands

```powershell
# Unit
go test ./internal/diff/...

# Race
go test -race ./internal/diff/...

# Verbose single test
go test ./internal/diff/ -run TestComputeDiff_BinaryContent -v

# Benchmarks
go test ./internal/diff/ -bench=. -benchmem

# Consumer
go test ./cmd/nerd/ui/ -run 'Diff|Word'
```

## 6. Alignment score

| Dimension | Score | Note |
|-----------|------:|------|
| Happy-path completeness | 5/5 | |
| Boundary / binary | 4/5 | Strong NUL; non-NUL binary residual |
| Concurrency | 4/5 | Compute race covered; ClearCache concurrent gap |
| Cache correctness under mutation | 2/5 | Known shallow trap untested as fail-closed |
| Performance regression gates | 3/5 | Benchmarks exist; not CI-thresholded in-package |

**Overall testing alignment: ~4/5** for a utility of this size — dense tests, known residual edges.
