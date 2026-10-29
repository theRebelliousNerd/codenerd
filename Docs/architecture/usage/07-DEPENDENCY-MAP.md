# usage — Dependency Map

> Last verified: **2026-07-13**  
> Evidence: package imports + `rg "codenerd/internal/usage"`.

## Upstream (what usage imports)

| Import | Kind | Use |
|--------|------|-----|
| `context` | stdlib | Tracker embedding + Track metadata |
| `encoding/json` | stdlib | Persist/load |
| `fmt` | stdlib | Error wrap on MkdirAll |
| `maps` | stdlib | `maps.Copy` for Stats |
| `os` | stdlib | Read/Write/Mkdir |
| `path/filepath` | stdlib | `.nerd/usage.json` paths |
| `sync` | stdlib | Mutex |
| `time` | stdlib | AfterFunc debounce; `UsageEvent.Timestamp` type |

**No** `codenerd/internal/*` imports. Package is a **leaf** of the internal graph.

```
[other packages] ──import──► usage ──► stdlib only
```

## Downstream (who imports usage)

### Production Go (non-test)

| Importer | Path | How used |
|----------|------|----------|
| System boot / Cortex | `internal/system/factory.go` | `NewTracker`, field `UsageTracker`, bootContext.tracker |
| Perception ZAI | `internal/perception/client_zai.go` | `FromContext` + `Track` |
| Shard manager | `internal/core/shards/manager_spawn.go` | `WithShardContext` on spawn |
| Advanced CLI | `cmd/nerd/cmd_advanced.go` | `NewContext` from Cortex |
| Interactive CLI | `cmd/nerd/cmd_interactive.go` | `NewContext` |
| Instruction CLI | `cmd/nerd/cmd_instruction.go` | `NewContext` |
| Direct actions | `cmd/nerd/cmd_direct_actions.go` | `NewContext` |
| Chat process | `cmd/nerd/chat/process.go` | `NewContext` on process/stream contexts |
| Chat session | `cmd/nerd/chat/session.go` | **Own** `NewTracker(workspace)` for chat model |
| Chat model types | `cmd/nerd/chat/model_types.go` | `usageTracker *usage.Tracker` field |
| Campaign | `cmd/nerd/chat/campaign.go` | `NewContext` |
| Campaign assault | `cmd/nerd/chat/campaign_assault.go` | `NewContext` |
| UI usage page | `cmd/nerd/ui/usage_page.go` | `Tracker`, `Stats`, `TokenCounts` |

### Tests

| Importer | Path |
|----------|------|
| UI pages test | `cmd/nerd/ui/pages_test.go` — constructs tracker for usage page |

## Dual construction note

There are **two live construction paths**:

1. **Cortex path:** `internal/system/factory.go` → `Cortex.UsageTracker` → CLI verbs attach with `NewContext`.  
2. **Chat session path:** `cmd/nerd/chat/session.go` → `Model.usageTracker` → process/campaign attach.

Both write the **same file path** if they share workspace (`.nerd/usage.json`). Concurrent processes or dual in-memory trackers without coordination can interleave Load/Save (see failure modes). Prefer one logical owner per process; chat should ideally reuse Cortex’s tracker when both exist in one process.

## Packages that do **not** depend on usage (relevant)

| Package | Note |
|---------|------|
| `internal/core` kernel | No import; no facts |
| `internal/prompt` | No metering |
| `internal/store` | Not used as backing store today |
| `internal/logging` | Usage does not call logging (gap) |
| Non-ZAI perception clients | Do not call Track (gap) |

## Verify reverse deps

```powershell
rg "codenerd/internal/usage" -g "*.go" --glob "!*_test.go"
```

## Dependency risk summary

| Risk | Level | Detail |
|------|-------|--------|
| Import cycles | **None** | Leaf package |
| Fan-out of API | **Low** | Small surface |
| Fan-in of Track producers | **High concentration** | Single ZAI call site — brittle for complete metrics |
| Dual tracker ownership | **Medium** | Chat + Cortex both construct |
