# usage — Wiring and Integration

> Last verified: **2026-07-13**

## Registration model

There is **no** central plugin registry, Mangle Decl set, or VirtualStore route for usage. Integration is **manual context plumbing**.

## Boot wiring (Cortex)

**File:** `internal/system/factory.go`

1. `initCoreComponents` calls `usage.NewTracker(bctx.workspace)`.  
2. On error: stderr warning; tracker may be nil.  
3. Tracker stored on `bootContext.tracker`.  
4. Assembled into `Cortex.UsageTracker` when Cortex is returned (~line 1086 region).

```
BootCortex / GetOrBootCortex
  → initCoreComponents
      → usage.NewTracker(workspace)
  → Cortex{ UsageTracker: tracker, ... }
```

## CLI verb wiring

Each long-running / LLM-using command that boots Cortex attaches the tracker:

| File | Pattern |
|------|---------|
| `cmd/nerd/cmd_advanced.go` | `if cortex.UsageTracker != nil { ctx = usage.NewContext(ctx, …) }` |
| `cmd/nerd/cmd_interactive.go` | same |
| `cmd/nerd/cmd_instruction.go` | same |
| `cmd/nerd/cmd_direct_actions.go` | same (+ tracer note) |

If a new Cobra verb boots Cortex and calls the LLM **without** this attach, metering silently skips.

## Interactive chat wiring

| Step | File | Action |
|------|------|--------|
| Construct | `cmd/nerd/chat/session.go` | `usage.NewTracker(workspace)` into model |
| Store field | `cmd/nerd/chat/model_types.go` | `usageTracker *usage.Tracker` |
| Per-turn / stream | `cmd/nerd/chat/process.go` | `usage.NewContext` on process and stream contexts |
| Campaign phases | `cmd/nerd/chat/campaign.go`, `campaign_assault.go` | `NewContext` when tracker non-nil |

## Shard attribution wiring

**File:** `internal/core/shards/manager_spawn.go`

Before goroutine execution:

```go
ctx = usage.WithShardContext(ctx, config.Name, string(config.Type), sessionID)
```

`sessionID` falls back to `"current-session"` if manager session id empty.

Downstream LLM calls inside the shard inherit attribution **only if** they use this ctx (or a child).

## Perception producer wiring

**Only known live Track:**

`internal/perception/client_zai.go` after successful parse of completion:

```go
if tracker := usage.FromContext(ctx); tracker != nil {
    tracker.Track(ctx, c.model, "zai",
        zaiResp.Usage.PromptTokens,
        zaiResp.Usage.CompletionTokens,
        "chat")
}
```

Provider hard-coded `"zai"`; operation hard-coded `"chat"`.

## UI consumer wiring

**File:** `cmd/nerd/ui/usage_page.go`

- `NewUsagePageModel(tracker, styles)`  
- `UpdateContent` → `tracker.Stats()`  
- Tables: Provider, Model, Shard Type, Operation  
- Missing: Session table, Cost column, Events  

Nil tracker message: `"Usage tracking not available."`

## Fact-flow relationship (non-interference)

```
user input
  → perception (intent / LLM)
       │
       ├─ (metering side channel) Track if context has tracker
       │
  → user_intent facts
  → kernel next_action
  → VirtualStore / shards
  → articulation
```

Usage never injects `user_intent`, never queries the kernel, never routes actions.

## Wiring checklist for new features

| Change | Must also |
|--------|-----------|
| New LLM client | Call `FromContext` + `Track` on success with real token counts |
| New CLI command using LLM | `NewContext` after Cortex boot |
| New shard spawn path | Preserve or re-apply `WithShardContext` |
| New embedding path | Track with operation `"embedding"` if tokens known |
| Move workspace root | Ensure tracker recreated for new path (no global) |

## Dormant / partial hooks (do not delete casually)

| Hook | Status |
|------|--------|
| `UsageEvent` / Events slice | Schema only |
| `Cost` field | Schema only |
| `autoSaveTimer` | Field only |
| Chat vs Cortex dual NewTracker | Both live; coordination incomplete |
