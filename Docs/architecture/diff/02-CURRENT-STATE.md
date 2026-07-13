# 02 — Current State: `internal/diff`

> Precise inventory as of 2026-07-13. Paths relative to repo root.  
> Totals: **1** non-test Go file ≈ **379** lines; **2** test files ≈ **949** lines; **0** `.mg`.

## 1. Package identity

| Property | Value |
|----------|-------|
| Import path | `codenerd/internal/diff` |
| Directory | `internal/diff/` |
| Production files | `diff.go` only |
| External dep | `github.com/sergi/go-diff` v1.4.0 (`go.mod`) |
| Subpackages | None |
| Config / YAML | None |
| Mangle | None |
| Agents.md in package | None |

Package comment (`diff.go`):

> Package diff provides robust diff computation using the sergi/go-diff library.  
> This replaces the manual LCS implementation with a battle-tested diff engine.

## 2. File inventory

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/diff/diff.go` | 379 | All production types, Engine, algorithms |
| `internal/diff/diff_test.go` | 484 | Functional tests, boundary suite, benchmarks |
| `internal/diff/diff_comprehensive_test.go` | 466 | Parallel comprehensive cases, concurrency, hash |

## 3. Exported surface (complete)

### Types

| Symbol | Kind | Location |
|--------|------|----------|
| `LineType` | `int` enum | `diff.go` |
| `LineContext`, `LineAdded`, `LineRemoved`, `LineHeader` | consts | `diff.go` |
| `Line` | struct | `diff.go` |
| `Hunk` | struct | `diff.go` |
| `FileDiff` | struct | `diff.go` |
| `Engine` | struct | `diff.go` |
| `DefaultEngine` | `*Engine` var | `diff.go` |

### Functions / methods

| Symbol | Signature (abbrev.) | Location |
|--------|---------------------|----------|
| `NewEngine` | `() *Engine` | `diff.go` |
| `(*Engine).ComputeDiff` | `(oldPath, newPath, old, new string) *FileDiff` | `diff.go` |
| `ComputeDiff` | package convenience → `DefaultEngine` | `diff.go` |
| `(*Engine).ClearCache` | `()` | `diff.go` |
| `(*Engine).ComputeWordLevelDiff` | `(oldLine, newLine string) []diffmatchpatch.Diff` | `diff.go` |

### Unexported (internal)

| Symbol | Role |
|--------|------|
| `cacheKey` | FNV pair key |
| `operation` | Intermediate line op |
| `convertToHunks`, `diffsToOperations`, `groupIntoHunks`, `computeHunkCounts` | Pipeline |
| `hash` | FNV-1a |
| `containsNullByte`, `clampContextLines` | Guards |
| `diffTimeout`, `defaultContextLines`, `maxContextLines` | Constants |

## 4. Behavioral hotspots

| Hotspot | Why it matters |
|---------|----------------|
| `ComputeDiff` binary branch | Prevents ruinous work on blobs |
| Cache Load/Store + shallow copy | Correctness under concurrent UI / multi-mutation review |
| `groupIntoHunks` context close logic | Controls hunk merge vs split; UI navigation depends on it |
| `diffsToOperations` newline splitting | Edge cases around empty lines / trailing newlines |
| `DiffTimeout = 5s` | Only hard cost bound |

## 5. Consumer inventory (reverse deps)

| Path | Kind | Notes |
|------|------|-------|
| `cmd/nerd/ui/diffview.go` | Production | Aliases + engine + `CreateDiffFromStrings` |
| `cmd/nerd/ui/word_diff_test.go` | Test | Direct `diff.ComputeDiff` / `diff.Line*` |

No other production importers found via `codenerd/internal/diff` grep (2026-07-13).

## 6. Test density snapshot

| Area | Coverage quality |
|------|------------------|
| Add / remove / modify | Strong |
| New / delete file flags | Strong |
| Identical content | Strong |
| Binary NUL | Strong |
| Cache path rewrite | Strong |
| Concurrent ComputeDiff | Present |
| Benchmarks small/large/cache | Present |
| Shallow cache mutation | **Gap** (TODO in tests) |
| Hash collision | **Gap** |
| Cache OOM under flood | **Gap** |
| Minified monolith timeout assert | **Gap** (timeout exists; hard assert limited) |
| ClearCache vs concurrent Store | **Gap** |

## 7. What is *not* present

- Multi-file patch objects  
- Git index / worktree integration  
- Streaming / incremental file readers  
- Structured logging  
- Metrics export  
- Mangle predicates / schemas  
- Config keys under `.nerd/`  
