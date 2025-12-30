# internal/usage - Token Usage Tracking

This package provides token usage recording and persistence for LLM API calls across codeNERD sessions.

**Related Packages:**
- [internal/perception](../perception/CLAUDE.md) - LLMClient tracking usage
- [internal/system](../system/CLAUDE.md) - UsageTracker initialization
- [internal/config](../config/CLAUDE.md) - Usage limits configuration

## Architecture

The usage package provides:
- **Tracker**: Records and persists token usage to .nerd/usage.json
- **Aggregated Stats**: Breakdowns by provider, model, shard type, operation
- **Cost Estimation**: Optional USD cost estimates
- **Context Injection**: Tracker accessible via context.Context

## File Index

| File | Description |
|------|-------------|
| `usage_tracker.go` | Core `Tracker` managing token usage recording and persistence. Exports `Tracker` with Load/Save, `RecordUsage()` for individual events, `GetStats()` returning aggregated stats, and context helpers for tracker injection/extraction. |
| `usage_types.go` | Type definitions for usage data and aggregation. Exports `UsageData` (root structure), `UsageEvent` (single LLM transaction with model/provider/tokens/shard), `AggregatedStats` (by provider/model/shard/operation), and `TokenCounts` with cost estimate. |
| `usage_tracker_test.go` | Unit tests for tracker persistence and aggregation. Tests Load/Save cycle and multi-dimensional aggregation. |

## Key Types

### UsageEvent
```go
type UsageEvent struct {
    Timestamp     time.Time
    Model         string
    Provider      string
    InputTokens   int
    OutputTokens  int
    ShardType     string // ephemeral, specialist, system, user
    ShardName     string
    SessionID     string
    OperationType string // chat, embedding, tool_gen
}
```

### AggregatedStats
```go
type AggregatedStats struct {
    TotalProject TokenCounts
    ByProvider   map[string]TokenCounts
    ByModel      map[string]TokenCounts
    ByShardType  map[string]TokenCounts
    ByOperation  map[string]TokenCounts
    BySession    map[string]TokenCounts
}
```

### TokenCounts
```go
type TokenCounts struct {
    Input  int64
    Output int64
    Total  int64
    Cost   float64 // Optional USD estimate
}
```

## Persistence

Usage data is stored in `.nerd/usage.json`:
```json
{
  "version": "1.0",
  "aggregate": {
    "total_project": { "input": 50000, "output": 20000, "total": 70000 },
    "by_provider": { "anthropic": { ... }, "google": { ... } }
  }
}
```

## Dependencies

- Standard library only

## Testing

```bash
go test ./internal/usage/...
```

---

**Remember: Push to GitHub regularly!**
