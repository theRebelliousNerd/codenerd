# transparency — Wiring and Integration

> Last verified: 2026-07-13  
> Evidence from `session_boot.go`, `session_shared_boot.go`, VirtualStore, ShardManager, system base/router, chat glass_box + commands.

## 1. Boot sequence (interactive chat)

Simplified from `cmd/nerd/chat/session_boot.go`:

```
1. Load app config
2. transparencyCfg := appCfg.GetTransparencyConfig()
3. transparencyMgr := NewTransparencyManager(transparencyCfg)
4. ... kernel, VirtualStore, ShardManager ...
5. shardMgr.SetTransparencyManager(transparencyMgr)
6. glassBoxEventBus := NewGlassBoxEventBus(); Enable()
7. shardMgr.SetGlassBoxBus(glassBoxEventBus)
8. virtualStore.SetGlassBoxBus(glassBoxEventBus)
9. toolEventBus := NewToolEventBus()
10. virtualStore.SetToolEventBus(toolEventBus)
11. For each system shard: SetToolEventBus / SetGlassBox as applicable
12. CortexConfig carries:
      TransparencyMgr, GlassBoxEventBus, ToolEventBus → Model fields
13. TUI initGlassBox / initToolEventBus on boot complete message
```

`session_shared_boot.go` mirrors this for shared boot paths (including interface-based SetGlassBox/SetToolEventBus for shards).

## 2. Chat Model fields

From `model_types.go` (names):

- `transparencyMgr *transparency.TransparencyManager`  
- `glassBoxEventBus *transparency.GlassBoxEventBus`  
- tool event bus field + channel subscription state  
- glass box enabled flags, event ring, activity trail  

CortexConfig duplicates manager + both buses for handoff after async boot.

## 3. Slash commands

| Command | Handler area | Transparency role |
|---------|--------------|-------------------|
| `/transparency [on\|off]` | `commands_handlers_misc.go` | Toggle Manager + GetStatus |
| `/glassbox ...` | glass_box / command categories | Filter/status/verbose on bus |
| `/why <fact>` | evolution handlers + model_update | `NewExplainer().ExplainTrace` |
| `/explain` | `cmd_explain.go` | Provenance (kernel); complementary |
| (safety-related) | may use `ExplainSafetyAction` / FormatViolation | UX |

Command registry: `commands.go` routes; `command_categories.go` documents usage strings.

## 4. VirtualStore integration

```go
// virtual_store.go
SetGlassBoxBus(bus *transparency.GlassBoxEventBus)
SetToolEventBus(bus *transparency.ToolEventBus)

// virtual_store_routing.go — after action execution
emitToolAndRoutingEvents(req, result, dur)
  → ToolEvent{ToolName: verb, Result, Success, Duration}
  → GlassBoxEvent{Category: CategoryRouting, Summary: label, Duration}
```

Emits are **immediate** for routing Glass Box events so tool results stream live in debug mode.

## 5. ShardManager integration

```go
SetGlassBoxBus(bus)
SetTransparencyManager(tm any)  // stored only; type not used in manager methods reviewed

emitShardEvent(summary, details, source, dur)
  → EmitImmediate CategoryShard
```

`postSpawnHook` allows chat to inject GlassBox/ToolEventBus into newly spawned agents without shards importing chat.

**Gap:** no call to `transparencyMgr.StartShard` / `UpdateShardPhase` / `EndShard` in manager spawn path from current evidence.

## 6. System shard integration

`BaseSystemShard`:

- Fields: `GlassBox *GlassBoxEventBus`, `ToolEventBus *ToolEventBus`  
- Setters: `SetGlassBox`, `SetToolEventBus` (debug log on attach)

`router.go`: on tool completion/failure, if `ToolEventBus != nil`, emit truncated result (500 chars).

## 7. Explainer wiring

Not constructed at boot. Created when:

- `traceUpdateMsg` with `ShowInChat` (`model_update.go`)  
- Potentially other why/logic pane paths  

Requires a populated `*mangle.DerivationTrace` from kernel/heuristic tracer—not produced inside transparency.

## 8. Config wiring

```
UserConfig.Transparency *TransparencyConfig
UserConfig.GetTransparencyConfig() → default if nil
App/config yaml/json field Transparency
DefaultTransparencyConfig()
```

Chat reads config at boot; `/transparency` mutates **runtime Manager state**, which may not persist back to disk unless a separate config write path exists (treat as session toggle unless proven otherwise).

## 9. Registration checklist (for implementers)

When adding a new producer:

1. Import `codenerd/internal/transparency` only if you emit or format.  
2. Prefer injected bus pointer (nil-safe emit helpers).  
3. Choose Track B (Glass Box / Tool) vs Track A (Manager) consciously.  
4. Set Category + short Summary; put bulky text in Details.  
5. Use `EmitImmediate` for user-visible milestones; batched Emit for high-frequency noise (unless verbose).  
6. Never hold Manager lock across kernel evaluate.  
7. Add/adjust tests in producer package + transparency unit tests if types change.

## 10. Wiring gaps journal

| Item | Status | Evidence |
|------|--------|----------|
| Glass Box bus to VS + ShardMgr | **Wired** | session_boot |
| Tool bus to VS + system shards | **Wired** | session_boot, router |
| Manager to ShardMgr | **Attached only** | SetTransparencyManager any |
| ShardObserver feed | **Thin** | Manager façades exist; spawn uses Glass Box |
| SafetyReporter auto-feed | **Partial** | Manual Report* |
| Status flags Stream/JIT/Ops | **Display** | GetStatus only |
| Chat subscribe | **Wired** | glass_box.go |

## 11. Diagram — runtime instance graph

```
          ┌──────── session boot ────────┐
          │                              │
          ▼                              ▼
   TransparencyManager            GlassBoxEventBus ──subscribe──► Chat Model
          │                              ▲
          │                              │ emit
          │                     ┌────────┴────────┐
          │                     │                 │
          ▼                     │                 │
   (ShardObserver)        VirtualStore      ShardManager
   (SafetyReporter)             │                 │
                                │                 │
                                ▼                 ▼
                          ToolEventBus ◄── system.Router
                                │
                                └──subscribe──► Chat Model (always)
```
