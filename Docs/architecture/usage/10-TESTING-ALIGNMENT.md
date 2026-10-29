# usage — Testing Alignment

> Last verified: **2026-07-13**

## Commands

```powershell
go test ./internal/usage/...
go test ./internal/usage/... -count=1
go test ./cmd/nerd/ui/ -run UsagePage -count=1
```

## Test file map

| File | Focus |
|------|-------|
| `usage_types_test.go` | `TokenCounts.Add` table (zero, accumulate, zero-add, negatives, large) |
| `usage_tracker_test.go` | Track aggregates+persist, context helpers, NewTracker success/fail, Load matrix, NewContext embed |
| `usage_tracker_context_test.go` | Non-string context values → unknown, no panic |
| `usage_comprehensive_test.go` | Multi provider/model/op/session, no shard context, save/reload, Stats copy, FromContext nil, WithShardContext keys, JSON round-trip, corrupt NewTracker, copyTokenCountsMap, addToMap |

## Coverage strengths

| Area | Quality |
|------|---------|
| Aggregate correctness | **Strong** |
| Dimension segregation | **Strong** |
| Persistence round-trip | **Strong** |
| Load error paths | **Strong** (missing, read error, bad JSON, partial maps) |
| Context helpers | **Strong** |
| Panic regression (typed assert) | **Strong** |
| Stats defensive copy | **Strong** |
| MkdirAll failure | **Covered** |

## Gaps

| Gap | Why it matters | Suggested test |
|-----|----------------|----------------|
| Concurrent Track from N goroutines | Mutex correctness under load | `t.Parallel` storm + final totals |
| Debounce / AfterFunc behavior | Dirty race, save delay | Controlled clock or dirty flag inspection with short sleep |
| Explicit Save after dirty Tracks without waiting 5s | Already partial via tests setting `dirty=true` to disable autosave | Keep pattern documented |
| Integration: ZAI Track with NewContext | Cross-package wiring | perception test or higher-level boot test |
| Dual tracker same file | Chat + Cortex interleaving | Integration scenario |
| Cost / Events | Unimplemented | Only add when feature lands |
| Fuzz JSON Load | Corrupt corpus diversity | `testing/F` on Unmarshal path |

## Test techniques used in-package

- `t.TempDir()` workspaces (no pollution of real `.nerd`)  
- Pre-set `tracker.dirty = true` to **disable** autosave during unit tests (important pattern)  
- Direct `tracker.filePath` override for Load edge cases  
- Parallel only for pure helpers / context helpers that do not share tracker  

## Alignment with architectural principles

| Principle | Test evidence |
|-----------|---------------|
| Soft-fail corrupt | `TestTracker_NewTracker_WhenCorruptFileExists_ShouldStillCreateTracker` |
| Degrade attribution | context_test non-string |
| Stats isolation | `TestTracker_Stats_ShouldReturnCopy` |
| Unknown defaults | `TestTracker_Track_WhenNoShardContext_ShouldUseUnknown` |

## External tests touching usage

`cmd/nerd/ui/pages_test.go` — `TestUsagePageModelContent` builds a tracker and exercises page content with nil and non-nil trackers.
