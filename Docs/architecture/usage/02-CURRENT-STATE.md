# usage — Current State

> Last verified: **2026-07-13**  
> Inventory is 1:1 with `internal/usage/` on disk.

## Package metrics

| Metric | Value |
|--------|------:|
| Non-test `.go` files | 2 |
| Test `.go` files | 4 |
| Mangle (`.mg`) | 0 |
| Package docs in source tree | 0 (`README`/`agents.md` absent) |
| Approx. non-test LOC | ~258 |
| Approx. test LOC | ~~900+ |

## File inventory

### Production

| Path | ~Lines | Role |
|------|-------:|------|
| `internal/usage/usage_tracker.go` | 211 | `Tracker`, load/save, Track, Stats, context helpers |
| `internal/usage/usage_types.go` | 47 | `UsageData`, `UsageEvent`, `AggregatedStats`, `TokenCounts`, `Add` |

### Tests

| Path | Role |
|------|------|
| `internal/usage/usage_tracker_test.go` | NewTracker, Track aggregates, Load cases, context embed |
| `internal/usage/usage_tracker_context_test.go` | Non-string context values must not panic |
| `internal/usage/usage_types_test.go` | `TokenCounts.Add` table tests |
| `internal/usage/usage_comprehensive_test.go` | Multi-dimension segregation, save/reload, Stats copy, JSON round-trip, corrupt file, helpers |

## Hotspots (behavioral)

### 1. Construction & path layout

`NewTracker(workspacePath)`:

1. Ensures `workspacePath/.nerd` exists (`MkdirAll` 0755).  
2. Sets `filePath` to `workspacePath/.nerd/usage.json`.  
3. Initializes empty aggregate maps + `Version: "1.0"`.  
4. Calls `Load()`; on error (corrupt/missing maps issues) **continues with empty in-memory data** without returning error.

### 2. Track path

`Track(ctx, model, provider, input, output, operation)` under mutex:

- Reads context string keys `"shard_type"`, `"shard_name"`, `"session_id"` with comma-ok assertions → default `"unknown"`.  
- Updates `TotalProject` and maps: `ByProvider`, `ByModel`, `ByShardType`, `ByOperation`, `BySession`.  
- **Does not** append to `UsageData.Events`.  
- **Does not** set `TokenCounts.Cost`.  
- Debounced auto-save: if `!dirty`, set dirty and `time.AfterFunc(5s, Save + clear dirty)`.

### 3. Stats isolation

`Stats()` returns a struct copy with **deep-copied maps** via `maps.Copy` so callers cannot mutate tracker state through returned maps.

### 4. Context API

| Helper | Key style | Notes |
|--------|-----------|-------|
| `NewContext` / `FromContext` | Private typed `contextKey{}` | Correct typed key for tracker pointer |
| `WithShardContext` | String keys `"shard_name"`, `"shard_type"`, `"session_id"` | Shared with `Track` readers; collision risk with other packages using same string keys |

### 5. Dead / unused fields (in-package)

| Symbol | Status |
|--------|--------|
| `Tracker.autoSaveTimer` | Declared; **never assigned** (AfterFunc not stored) |
| `UsageData.Events` | JSON field present; Track never appends |
| `TokenCounts.Cost` | JSON omitempty; never written |
| Comment “ByShardName” | Acknowledged but not implemented |

## Runtime artifacts

| Artifact | Location | Format |
|----------|----------|--------|
| Usage store | `<workspace>/.nerd/usage.json` | Pretty-printed JSON (`MarshalIndent`) |
| File mode | 0644 on write | |

## What “done” looks like today (honest)

Implemented and usable:

- Project-level aggregate token accounting  
- Multi-dimension maps  
- Workspace file persistence + reload  
- Context ambient pattern  
- Shard attribution hook  
- ZAI completion metering  
- TUI read path  

Not implemented despite types/comments:

- Event log  
- Cost estimates  
- Per-shard-name aggregates  
- Universal multi-provider metering  
- Atomic replace / fsync durability  
- Dirty re-arm after in-flight Track-during-save  

## Diagram — current control flow

```
system.initCoreComponents / chat.session
        │
        ▼
  usage.NewTracker(workspace)
        │
        ▼
  Cortex.UsageTracker  OR  chat.Model.usageTracker
        │
        ▼
  usage.NewContext(ctx, tracker)     [CLI, process, campaign…]
        │
        ├─► shards: WithShardContext(name, type, sessionID)
        │
        ▼
  perception (ZAI): FromContext → Track(..., "chat")
        │
        ▼
  .nerd/usage.json  ◄── Stats() ── ui.UsagePageModel
```
