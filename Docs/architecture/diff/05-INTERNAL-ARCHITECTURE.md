# 05 — Internal Architecture: `internal/diff`

> Last verified against codebase: 2026-07-13  
> Source of truth: `internal/diff/diff.go`

## 1. Component diagram

```
                    ┌──────────────────────────────────────┐
                    │              Engine                  │
                    │  dmp: *diffmatchpatch.DiffMatchPatch │
                    │  cache: sync.Map[cacheKey]*FileDiff  │
                    └──────────────────┬───────────────────┘
                                       │
          NewEngine / DefaultEngine    │
                                       ▼
┌────────────┐   strings    ┌──────────────────┐   []Diff   ┌─────────────────┐
│  Callers   │ ───────────► │   ComputeDiff    │ ─────────► │ convertToHunks  │
└────────────┘              │  flags + binary  │            │  clamp context  │
                            │  cache probe     │            └────────┬────────┘
                            └────────┬─────────┘                     │
                                     │ store                         ▼
                                     │                      ┌─────────────────┐
                                     │                      │diffsToOperations│
                                     │                      └────────┬────────┘
                                     │                               ▼
                                     │                      ┌─────────────────┐
                                     │                      │ groupIntoHunks  │
                                     │                      │ computeHunkCounts│
                                     │                      └────────┬────────┘
                                     ▼                               ▼
                            ┌──────────────────┐            ┌─────────────────┐
                            │    *FileDiff     │◄───────────│  []Hunk / Line  │
                            └──────────────────┘            └─────────────────┘
```

Word-level path is separate and short:

```
ComputeWordLevelDiff(oldLine, newLine)
  → dmp.DiffMain → DiffCleanupSemantic → []diffmatchpatch.Diff
```

## 2. Data model relationships

```
FileDiff 1──* Hunk 1──* Line
   │
   ├── OldPath, NewPath : string
   ├── IsNew, IsDelete, IsBinary : bool
   └── Hunks

Hunk
   ├── OldStart, OldCount, NewStart, NewCount : int
   └── Lines []Line

Line
   ├── LineNum : int   (1-based display)
   ├── Content : string
   └── Type : LineType  (Context|Added|Removed|Header)
```

Internal intermediate:

```
operation { typ LineType; oldLine, newLine int; content string }
cacheKey  { oldHash, newHash uint64 }
```

## 3. Pipeline stages (file-level)

| Stage | Function | Input | Output |
|-------|----------|-------|--------|
| 1 Flag | `ComputeDiff` head | contents | `IsNew` / `IsDelete` |
| 2 Binary | `containsNullByte` | contents | early `FileDiff` |
| 3 Cache | `hash` + `sync.Map` | contents | hit → shallow path rewrite |
| 4 Reduce | sergi `DiffLinesToChars` | texts | char-coded strings + map |
| 5 Diff | `DiffMain` + semantic cleanup | codes | `[]Diff` |
| 6 Expand | `DiffCharsToLines` | `[]Diff` + map | line-granularity diffs |
| 7 Ops | `diffsToOperations` | `[]Diff` | `[]operation` |
| 8 Hunks | `groupIntoHunks` | ops + context | `[]Hunk` |
| 9 Count | `computeHunkCounts` | each hunk | OldCount/NewCount |
| 10 Cache | `Store` | `*FileDiff` | future hits |

## 4. Line numbering rules

- Internal counters in `diffsToOperations` are **0-based**.  
- Stored `Line.LineNum` and hunk starts are **1-based** (`oldLine+1` / `newLine+1`).  
- Added lines use new-side number; removed/context prefer old-side for `LineNum`.  
- When a hunk starts on a pure add/delete op, starts may clamp to `0` if the start
  operation carries `-1` on that side.

## 5. Hunk close policy

```
for each op i:
  if change and no current hunk:
      start hunk; prepend context [i-context, i)
  if in hunk:
      append op as Line
      if context and (i - lastChangeIdx) > contextLines:
          trim trailing context to contextLines
          compute counts; emit hunk; current = nil
if open hunk remains: emit
```

Identical files produce no change ops → zero hunks (after equal-only ops may still
yield no emitted hunks because no `isChange` starts a hunk).

## 6. State machines

### Engine lifetime

```
NewEngine → idle
  ├─ ComputeDiff* → (cache grow) → idle
  ├─ ComputeWordLevelDiff* → idle  (does not use cache)
  └─ ClearCache → idle (empty map)
```

No explicit closed/disposed state; GC reclaims when unreferenced.

### Cache entry lifecycle

```
miss → compute FileDiff → Store(pointer)
hit  → shallow copy struct → rewrite paths → return (shares Hunks backing array)
ClearCache → drop all entries (in-flight Stores may write to old or new map)
```

## 7. Constants as architecture

| Constant | Architectural role |
|----------|-------------------|
| `diffTimeout` | Hard latency budget for Myers |
| `defaultContextLines` | UX default aligned with git-ish hunks |
| `maxContextLines` | Fuzz / abuse ceiling |

## 8. Why line reduction first

Character-level Myers on whole files with many newlines produces awkward boundaries when
re-split into lines. `DiffLinesToChars` compresses each unique line to a char token so
`DiffMain` works on a short alphabet of “line IDs,” then expands back. This is the
correct default for **code review hunks**.
