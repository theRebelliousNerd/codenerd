# usage — Public API and Types

> Last verified: **2026-07-13**  
> Package path: `codenerd/internal/usage`

## Types

### `UsageData` — `usage_types.go`

```go
type UsageData struct {
    Version   string
    Events    []UsageEvent    // optional; not populated by Track today
    Aggregate AggregatedStats
}
```

JSON root persisted to disk.

### `UsageEvent` — `usage_types.go`

Single-transaction schema:

| Field | JSON | Intent |
|-------|------|--------|
| `Timestamp` | `timestamp` | When recorded |
| `Model` | `model` | Model id |
| `Provider` | `provider` | Provider id |
| `InputTokens` | `input_tokens` | Prompt tokens |
| `OutputTokens` | `output_tokens` | Completion tokens |
| `ShardType` | `shard_type` | ephemeral / specialist / system / user (comments) |
| `ShardName` | `shard_name` | Concrete shard name |
| `SessionID` | `session_id` | Session correlation |
| `OperationType` | `operation_type` | chat / embedding / tool_gen |

**Status:** defined for future event logs; not written by `Track`.

### `AggregatedStats` — `usage_types.go`

| Field | Meaning |
|-------|---------|
| `TotalProject` | Sum of all tracked tokens |
| `ByProvider` | map[string]TokenCounts |
| `ByModel` | map[string]TokenCounts |
| `ByShardType` | map[string]TokenCounts |
| `ByOperation` | map[string]TokenCounts |
| `BySession` | map[string]TokenCounts |

### `TokenCounts` — `usage_types.go`

| Field | Type | Notes |
|-------|------|-------|
| `Input` | int64 | Sum input |
| `Output` | int64 | Sum output |
| `Total` | int64 | Input+Output running sum |
| `Cost` | float64 | `cost_est_usd`, omitempty; unused |

#### Method

| Method | Signature | Behavior |
|--------|-----------|----------|
| `Add` | `(tc *TokenCounts) Add(input, output int)` | Adds to Input/Output/Total |

### `Tracker` — `usage_tracker.go`

Exported struct; fields are unexported (encapsulation). Construct only via `NewTracker`.

## Functions

| Symbol | File | Signature / role |
|--------|------|------------------|
| `NewTracker` | `usage_tracker.go` | `func NewTracker(workspacePath string) (*Tracker, error)` — create/load tracker; error only on MkdirAll failure |
| `(*Tracker) Load` | `usage_tracker.go` | Reload from `filePath`; missing file = nil error |
| `(*Tracker) Save` | `usage_tracker.go` | Persist current data |
| `(*Tracker) Track` | `usage_tracker.go` | `Track(ctx, model, provider string, input, output int, operation string)` |
| `(*Tracker) Stats` | `usage_tracker.go` | Snapshot `AggregatedStats` with copied maps |
| `NewContext` | `usage_tracker.go` | Embed `*Tracker` in context |
| `FromContext` | `usage_tracker.go` | Retrieve tracker or nil |
| `WithShardContext` | `usage_tracker.go` | `WithShardContext(ctx, name, typeName, sessionID string) context.Context` |

## Unexported helpers (documented for integrators)

| Symbol | Role |
|--------|------|
| `saveLocked` | Marshal + WriteFile under held lock |
| `addToMap` | Upsert TokenCounts in a map |
| `copyTokenCountsMap` | Defensive map copy |
| `contextKey` | Typed context key for tracker |

## Typical call recipes

### Boot owner

```go
tracker, err := usage.NewTracker(workspace)
// err: only directory creation failure
// tracker may still be non-nil with empty data if file corrupt
```

### Request path attach

```go
if tracker != nil {
    ctx = usage.NewContext(ctx, tracker)
}
```

### Shard attribution

```go
ctx = usage.WithShardContext(ctx, config.Name, string(config.Type), sessionID)
```

### After LLM success

```go
if t := usage.FromContext(ctx); t != nil {
    t.Track(ctx, model, provider, promptTokens, completionTokens, "chat")
}
```

### Operator UI

```go
stats := tracker.Stats()
// use stats.TotalProject, stats.ByProvider, ...
```

## Stability notes

| Surface | Stability |
|---------|-----------|
| Track / Stats / NewTracker / context helpers | **Stable** — multiple production callers |
| JSON field names on aggregates | **De facto stable** — on-disk contract |
| `Events` / `Cost` semantics | **Unstable / reserved** |
| String context keys | **De facto contract** with shards; changing breaks attribution |
