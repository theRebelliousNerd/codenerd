# usage — Internal Architecture

> Last verified: **2026-07-13**

## Component map

```
internal/usage
├── types layer          (usage_types.go)
│     UsageData
│     UsageEvent          [schema only today]
│     AggregatedStats
│     TokenCounts + Add
│
└── tracker layer        (usage_tracker.go)
      Tracker
        mu, data, filePath, dirty, autoSaveTimer(unused)
      NewTracker / Load / Save / saveLocked
      Track / Stats
      NewContext / FromContext / WithShardContext
      helpers: addToMap, copyTokenCountsMap
```

## Key types (behavioral)

### `Tracker`

In-memory owner of `UsageData` plus persistence path. Thread-safe via `sync.Mutex`.

| Field | Purpose |
|-------|---------|
| `mu` | Serialize Track/Load/Save/Stats |
| `data` | Live aggregates (+ optional events) |
| `filePath` | Absolute-ish path to `usage.json` |
| `dirty` | Debounce gate for AfterFunc |
| `autoSaveTimer` | Reserved; unused |

### `UsageData`

Root JSON document:

```json
{
  "version": "1.0",
  "events": [ /* optional, currently empty/absent */ ],
  "aggregate": {
    "total_project": { "input": 0, "output": 0, "total": 0 },
    "by_provider": { "...": { "input": 0, "output": 0, "total": 0 } },
    "by_model": {},
    "by_shard_type": {},
    "by_operation": {},
    "by_session": {}
  }
}
```

### `TokenCounts`

Counters are `int64` for sums; `Track` accepts `int` per call and promotes. `Cost float64` is reserved (`cost_est_usd`).

## Data flow

### Write path

```mermaid
sequenceDiagram
  participant C as Caller (ZAI client)
  participant T as Tracker
  participant D as usage.json

  C->>T: Track(ctx, model, provider, in, out, op)
  T->>T: lock mu
  T->>T: extract shard/session from ctx
  T->>T: TotalProject.Add + addToMap(*)
  alt not dirty
    T->>T: dirty=true; AfterFunc(5s)
  end
  T->>T: unlock
  Note over T,D: ~5s later
  T->>T: Save → lock → MarshalIndent → WriteFile
  T->>T: dirty=false
```

### Read path

```
UI / tests
  → Stats()
  → copy maps
  → render tables / assert
```

### Boot path

```
workspace path
  → MkdirAll(.nerd)
  → ReadFile(usage.json) if exists
  → Unmarshal into data
  → re-init nil maps
```

## State machine (dirty / save)

```
          Track
            │
            ▼
     ┌──────────────┐
     │ clean        │◄──────────────┐
     │ dirty=false  │               │
     └──────┬───────┘               │
            │ first Track           │ AfterFunc: Save + dirty=false
            ▼                       │
     ┌──────────────┐               │
     │ dirty=true   │── more Track ─┤  (no new timer)
     │ timer armed  │               │
     └──────┬───────┘               │
            │ 5s fire               │
            └───────────────────────┘
```

**Hazard:** Track after Save snapshot but before dirty clear → possible lost dirty (see failure modes).

## Context value design

Two key styles coexist:

1. **Typed private key** for `*Tracker` (safe, package-private).  
2. **String keys** for attribution metadata (public contract with `core/shards`).

```
ctx
 ├─ contextKey{} → *Tracker
 ├─ "shard_name" → string
 ├─ "shard_type" → string
 └─ "session_id" → string
```

## Concurrency model

| Operation | Locking |
|-----------|---------|
| Track | Full critical section under `mu` |
| Save / Load | Full under `mu` |
| AfterFunc | Calls `Save()` (locks), then locks again to clear dirty |
| Stats | Lock; copy maps; unlock |

No channels, no worker pool. Suitable for moderate Track rates (LLM QPS), not for millions of micro-events without batching changes.

## Dependency direction (internal)

```
usage  →  stdlib only
           context, encoding/json, fmt, maps, os, path/filepath, sync, time
```

Zero imports of other `codenerd/internal/*` packages — keeps the sensor leaf-like and testable in isolation.
