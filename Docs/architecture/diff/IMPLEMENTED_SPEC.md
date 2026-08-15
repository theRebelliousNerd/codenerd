# codeNERD `internal/diff` — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-08-15  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/diff/diff.go`, `internal/diff/cache.go`  
> Scale: **2** non-test Go files; **5** test files; **0** Mangle  
> External engine: `github.com/sergi/go-diff` v1.4.0 (`diffmatchpatch`)

## 1. Overview

`internal/diff` is a **focused text-diff engine adapter**. It replaces a historical hand-rolled
LCS implementation with the sergi `diffmatchpatch` library and exposes a stable, UI-friendly
data model:

| Type | Role |
|------|------|
| `FileDiff` | One file’s change set (paths, flags, hunks) |
| `Hunk` | Grouped change region with unified-diff-style starts/counts |
| `Line` | Single context / added / removed line |
| `Engine` | Configurable worker with timeout, cache, word-level API |

### Key characteristics

| Property | Value |
|----------|-------|
| Package size | Tiny — single production file |
| Algorithm owner | sergi/go-diff (Myers-family + semantic cleanup) |
| Line reduction | `DiffLinesToChars` → `DiffMain` → `DiffCharsToLines` |
| Default context | 3 lines (`defaultContextLines`) |
| Pathological timeout | 5s (`diffTimeout` on `dmp.DiffTimeout`) |
| Binary gate | NUL byte (`0x00`) → `IsBinary`, empty hunks |
| Cache | Bounded LRU keyed by two hashes + length per side; deep-copied `*FileDiff` |
| Singleton | `DefaultEngine` + package-level `ComputeDiff` |
| Concurrency | Mutex-guarded LRU; concurrent `ComputeDiff` and `ClearCache` tested under `-race` |
| Mangle / kernel | **None** — pure Go library |
| Primary consumer | `cmd/nerd/ui/diffview.go` (DiffApprovalView) |

### Why this package exists

The TUI needs structured hunks and line types to:

1. Render `@@ -a,b +c,d @@` headers and colored +/- lines.  
2. Drive word-level highlighting on adjacent remove/add pairs.  
3. Support interactive approve/reject of proposed mutations without re-parsing git patches.

Centralizing that conversion in `internal/diff` keeps the UI free of `diffmatchpatch` details
and keeps timeouts / binary short-circuits / caching in one place.

### High-level control flow

```
ComputeDiff(oldPath, newPath, oldContent, newContent)
   │
   ├─ empty old? → IsNew
   ├─ empty new? → IsDelete
   ├─ NUL in either? → IsBinary, return (no hunks)
   ├─ cache hit (FNV-1a pair)? → shallow-clone paths, return
   │
   ├─ DiffLinesToChars(old, new)
   ├─ DiffMain(a, b, false)
   ├─ DiffCleanupSemantic
   ├─ DiffCharsToLines
   │
   ├─ convertToHunks (defaultContextLines=3, clamped)
   │     ├─ diffsToOperations  (line ops + 0-based counters)
   │     └─ groupIntoHunks     (context windows, close on gap)
   │
   └─ cache.Store(key, fileDiff) → return
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Engine` + `NewEngine` | **Implemented** | Timeout configured at construction |
| `DefaultEngine` singleton | **Implemented** | Process-wide shared cache |
| Package `ComputeDiff` | **Implemented** | Delegates to `DefaultEngine` |
| Line-level file diffs | **Implemented** | Line reduction path |
| Hunk grouping + context | **Implemented** | Default 3; clamp [0, 1000] |
| New / delete flags | **Implemented** | Empty-string content |
| Binary short-circuit | **Implemented** | NUL detection |
| Word-level diffs | **Implemented** | Returns `[]WordSpan` — no third-party type in the public API |
| Cache + `ClearCache` | **Implemented** | Path rewrite on hit |
| Concurrent safety | **Implemented** | Mutex-guarded LRU; `get` returns a deep `Clone` |
| Cache size bound / eviction | **Implemented** | LRU on entry count **and** approximate bytes |
| Content collision defense | **Implemented** | Two independent hashes + both lengths in the key; opt-in exact content verify via `Options.VerifyCacheContent`, rejected hits counted in `Stats.Collisions` |
| Unified-diff emit/parse | **Out of scope** | Consumers format themselves |
| Side-by-side model | **Out of scope** | UI concern |
| Kernel / `permitted` | **N/A** | Not an effectful surface |
| Logging / metrics | **Implemented (metrics)** | `Engine.Stats()` — hits, misses, computes, binary, evicted, collisions, entries, bytes. No logging by design |

**Overall:** living production utility — **not** pre-implementation. Cache hygiene
(deep copy, bounds, collision defense) and the public-API type leak are closed;
the package's public surface no longer mentions `diffmatchpatch`.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/diff/
  diff.go                      # Public model, Options, Engine, hunk conversion, word spans
  cache.go                     # Bounded LRU, Stats, FileDiff.Clone/approxSize
  diff_test.go                 # Core + boundary tests + benchmarks
  diff_comprehensive_test.go   # Parallel/comprehensive suite
  cache_test.go                # Deep-copy, eviction, ClearCache race, Stats
  word_span_test.go            # WordSpan contract, LineHeader invariant, collision verify
  benchmark_test.go            # Realistic-source + verified-hit benchmarks and CI smoke
```

No subpackages, no `.mg`, no config YAML, no README inside the package.

### 3.2 Production file map (`diff.go`)

| Region (approx lines) | Symbols | Role |
|-----------------------|---------|------|
| 1–39 | package, consts, `containsNullByte`, `clampContextLines` | Bounds & binary probe |
| 41–75 | `LineType`, `Line`, `Hunk`, `FileDiff` | Public data model |
| 77–102 | `Engine`, `cacheKey`, `NewEngine`, `DefaultEngine` | Engine construction |
| 104–165 | `ComputeDiff` (method + package) | Main entry |
| 167–185 | `convertToHunks` | Diff → hunk adapter |
| 187–253 | `operation`, `diffsToOperations` | Line ops |
| 255–351 | `groupIntoHunks`, `computeHunkCounts` | Context grouping |
| 353–366 | `hash` | FNV-1a 64-bit |
| 368–378 | `ClearCache`, `ComputeWordLevelDiff` | Cache control + intra-line |

### 3.3 Test inventory

| File | ≈Lines | Focus |
|------|-------:|-------|
| `diff_test.go` | 484 | Simple add/delete, multi-hunk, cache, large file, word-level, benchmarks, binary, empty strings, huge context, empty paths |
| `diff_comprehensive_test.go` | 466 | Parallel engine creation, identical content, paths, concurrency race, hash, whitespace, convenience function |

Both files carry explicit `// TODO: TEST_GAP:` comments enumerating residual risks (hash collision,
shallow cache mutation, OOM under mass unique diffs, minified monolith timeout, etc.).

---

## 4. Public data model

### 4.1 `LineType`

```
LineContext = 0   // unchanged
LineAdded   = 1
LineRemoved = 2
LineHeader  = 3   // UI-owned: the engine never emits it, by decision
```

**`LineHeader` decision (2026-08-15).** Kept, not deprecated, and not deleted.
Hunk framing lives in the `Hunk` counters, so a renderer composes its own
`@@ -a,b +c,d @@` row rather than receiving one as a `Line` — but the row it
composes still needs a `LineType`, and `cmd/nerd/ui` uses `DiffLineHeader` for
exactly that. So `LineHeader` is a **UI-owned member of the enum**: legal to
construct, never produced by `ComputeDiff`. The invariant is enforced by
`TestComputeDiff_WhenAnyInput_ShouldNeverEmitLineHeader`, so it cannot rot back
into ambiguity.

### 4.2 `Line`

| Field | Meaning |
|-------|---------|
| `LineNum` | 1-based display line (old side for remove/context; new side for add) |
| `Content` | Line text without trailing newline |
| `Type` | `LineType` |

### 4.3 `Hunk`

| Field | Meaning |
|-------|---------|
| `OldStart` / `NewStart` | 1-based start positions (0 edge for pure add/delete starts) |
| `OldCount` / `NewCount` | Lines counting toward old/new (context counts on both) |
| `Lines` | Ordered `[]Line` |

Counts are derived by `computeHunkCounts`: context increments both; remove → old only;
add → new only.

### 4.4 `FileDiff`

| Field | Meaning |
|-------|---------|
| `OldPath` / `NewPath` | Caller-supplied labels (not filesystem I/O) |
| `Hunks` | Change groups (empty if identical, binary, or empty/empty) |
| `IsNew` | `oldContent == ""` |
| `IsDelete` | `newContent == ""` |
| `IsBinary` | NUL in either side; hunks forced empty |

**Note:** both-empty sets **both** `IsNew` and `IsDelete` true and yields zero hunks
(covered by tests).

---

## 5. Deep dive: `ComputeDiff`

### 5.1 Inputs and outputs

```
func (e *Engine) ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff
func ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff  // DefaultEngine
```

- Paths are **opaque labels**. Empty paths are allowed and preserved (including on cache rewrite).  
- Contents are full file texts as Go strings (UTF-8 assumed; invalid UTF-8 is not specially handled).  
- Return is always non-nil for normal calls (engine never returns `nil` from success path).

### 5.2 Short-circuits

1. **Empty content flags** — independent of paths.  
2. **Binary** — `strings.IndexByte(..., 0x00) >= 0` on either side → `IsBinary=true`, empty `Hunks`, **no** cache store, **no** Myers run.  
3. **Cache hit** — FNV-1a(`old`) × FNV-1a(`new`) → shallow copy of cached `FileDiff` with paths overwritten.

### 5.3 Line-level reduction

Using sergi’s character-reduction for lines avoids newline-boundary artifacts when converting
ops back to lines:

```
a, b, lineArray := dmp.DiffLinesToChars(old, new)
diffs := dmp.DiffMain(a, b, false)
diffs = dmp.DiffCleanupSemantic(diffs)
diffs = dmp.DiffCharsToLines(diffs, lineArray)
```

`DiffTimeout = 5s` limits pathological single-line / huge inputs (library-enforced, not
re-checked after).

### 5.4 Operations and hunk grouping

`diffsToOperations`:

- Splits each diff chunk on `\n`, drops trailing empty split artifact.  
- Tracks 0-based `oldLine` / `newLine` counters.  
- Inserts use `oldLine = -1`; deletes use `newLine = -1`.

`groupIntoHunks(ops, contextLines)`:

- Starts a hunk on first non-context op; prepends up to `contextLines` leading context.  
- Closes when trailing context exceeds `contextLines` after last change.  
- Final open hunk is closed with `computeHunkCounts`.  
- `contextLines` is always `clampContextLines` → `[0, 1000]`.

Public `ComputeDiff` always uses `defaultContextLines = 3` (no public override parameter).

### 5.5 Caching semantics

| Aspect | Behavior |
|--------|----------|
| Key | `cacheKey{oldHash, newHash}` FNV-1a 64-bit |
| Value | `*FileDiff` as computed (paths of first insert) |
| Hit | Shallow struct copy; `OldPath`/`NewPath` replaced; **Hunks slice shared** |
| Miss | Compute, `cache.Store`, return original pointer |
| Binary | Not cached |
| Clear | `ClearCache` empties the LRU in place (safe against concurrent `ComputeDiff`) and preserves cumulative `Stats` |

**Shallow-copy trap:** mutating `result.Hunks` or nested `Line` values after a cache hit
mutates the shared cached structure for future callers. Documented as TEST_GAP; not yet
defended with deep clone or immutable slices.

**Collision risk:** different content pairs that hash to the same `(oldHash, newHash)` can
return wrong hunks. FNV-1a 64-bit makes this rare for practical code sizes; no verification
step exists.

### 5.6 Word-level API

```
func (e *Engine) ComputeWordLevelDiff(oldLine, newLine string) []WordSpan
func ComputeWordLevelDiff(oldLine, newLine string) []WordSpan  // uiDiffEngine-independent; DefaultEngine
```

- Runs `DiffMain` + `DiffCleanupSemantic` **without** line reduction.  
- Returns sergi’s own `Diff` slice (not `[]Line`).  
- Used by `DiffApprovalView.renderWordDiffPair`, which now paints the spans: the runs unique to each side are highlighted, unchanged runs stay in the base style
  ranges (styles prepared, full char-range painting still partial).

---

## 6. Integration map

### 6.1 Direct importers (production)

| Consumer | Path | Usage |
|----------|------|-------|
| Diff approval TUI | `cmd/nerd/ui/diffview.go` | Type aliases; `NewEngine()` for word diffs; `CreateDiffFromStrings` → `diff.ComputeDiff`; `IsBinary` rendering; hunk headers |

Evidence:

- Type aliases: `DiffLine = diff.Line`, `FileDiff = diff.FileDiff`, etc.  
- `diffEngine: diff.NewEngine()` in `NewDiffApprovalView`.  
- `CreateDiffFromStrings` → `diff.ComputeDiff(...)`.  
- `ComputeWordLevelDiff` in `renderWordDiffPair`.

### 6.2 Test importers

| Consumer | Path |
|----------|------|
| Word-diff UI tests | `cmd/nerd/ui/word_diff_test.go` |

### 6.3 Non-importers (important negative evidence)

As of 2026-07-13, **no** other `internal/*` package imports `codenerd/internal/diff`.
Kernel, VirtualStore, shards, tools, perception, articulation, campaign, store — **none**.

This is intentional: diff is a **presentation / review** helper, not an executive decision
component. Constitutional safety for *applying* mutations lives elsewhere (policy /
Dreamer / VirtualStore write paths); this package only *describes* text deltas.

### 6.4 Fact-flow diagram (placement)

```
┌──────────────┐   user_intent    ┌────────────┐  next_action  ┌─────────────┐
│  perception  │ ───────────────► │   kernel   │ ────────────► │ VirtualStore│
└──────────────┘                  └────────────┘               └──────┬──────┘
                                                                      │ propose edit
                                                                      ▼
                                                              old/new content
                                                                      │
                                                              ┌───────▼───────┐
                                                              │ internal/diff │
                                                              └───────┬───────┘
                                                                      │ FileDiff
                                                              ┌───────▼───────┐
                                                              │ ui DiffApproval│
                                                              └───────┬───────┘
                                                                      │ y/n
                                                                      ▼
                                                              apply or discard
```

---

## 7. Constants and tunables (source of truth)

| Name | Value | Purpose |
|------|-------|---------|
| `diffTimeout` | `5 * time.Second` | Bound pathological Myers cost (`dmp.DiffTimeout`) |
| `defaultContextLines` | `3` | Public path hunk context |
| `maxContextLines` | `1000` | Clamp for `convertToHunks` / fuzz safety |

There is **no** env var, config key, or Mangle atom controlling these. Changing behavior means
editing `diff.go` (or extending the API).

---

## 8. Concurrency model

```
Engine
  ├── dmp  *DiffMatchPatch   // not mutex-guarded; DiffMain is per-call stateful on dmp?
  └── cache *diffCache       // mutex-guarded LRU
```

- Concurrent `ComputeDiff` on one `Engine` is exercised by
  `TestComputeDiff_WhenConcurrent_ShouldNotRace` (20 goroutines).  
- `ClearCache` empties in place under the cache mutex — no reassignment, no race
  on the abandoned map (benign) or mix maps depending on timing; not formally proven free
  of subtle races under `-race` for clear-during-compute (listed as TEST_GAP).  
- `DefaultEngine` is global; package-level `ComputeDiff` callers share one cache. `cmd/nerd/ui` no longer uses it: the UI has a single package engine (`uiDiffEngine`) shared by `CreateDiffFromStrings` and every `DiffApprovalView`, so file diffs and word diffs hit one cache instead of two.

---

## 9. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md), [12-FAILURE-MODES.md](12-FAILURE-MODES.md),
and [TODO.md](TODO.md) for:

- Unbounded cache growth  
- Shallow cache clone / shared hunks  
- Hash-only keys  
- `LineHeader` deliberately UI-owned and test-enforced  
- No public `contextLines` parameter on `ComputeDiff`  
- Word-diff returns codeNERD `WordSpan`; sergi types no longer escape the package  
- No structured logging on timeout / binary short-circuit  

Non-gaps: “needs Mangle Decl”, “needs VirtualStore route”, “pre-implementation 0%” —
**false** for this package.

---

## 10. Related corpora

| Corpus | Relation |
|--------|----------|
| `Docs/architecture/cli/` | Hosts DiffApprovalView consumer docs (`07-UI-PAGES-AND-OUTPUT.md`) |
| `Docs/architecture/core/` | Policy/execute path that *applies* edits after review |
| `Docs/architecture/tools/` | Tooling may produce content; does not own this package |
| Vectryx | **None** — no product coupling |

---

## 11. Verification commands

```powershell
go test ./internal/diff/...
go test -race ./internal/diff/...
go test ./internal/diff/ -bench=BenchmarkComputeDiff -benchmem
go test ./cmd/nerd/ui/ -run 'Diff|Word|Approval'
```

---

## 12. One-line summary

**`internal/diff` is a timeout-bounded, cache-aware, line-hunk adapter over sergi go-diff
that feeds the codeNERD interactive mutation-approval UI — pure library, no kernel surface.**
