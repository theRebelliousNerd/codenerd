# transparency — Dependency Map

> Last verified: 2026-07-13

## 1. Upstream (what transparency imports)

| Import | Used for | Files |
|--------|----------|-------|
| `codenerd/internal/config` | `TransparencyConfig` | `transparency.go` |
| `codenerd/internal/mangle` | `DerivationTrace`, `DerivationNode`, `Fact`, `SourceEDB` | `explainer.go` |
| stdlib `fmt`, `strings`, `sync`, `sync/atomic`, `time`, `reflect`, `sort` | plumbing | various |

**Does not import:** `internal/core`, `internal/logging`, `cmd/nerd`, Bubble Tea, network stacks.

## 2. Downstream (who imports transparency)

### 2.1 CLI / chat (primary UX)

| Package / file | Symbols used |
|----------------|--------------|
| `cmd/nerd/chat/session_boot.go` | Manager, GlassBox bus, Tool bus construction + inject |
| `cmd/nerd/chat/session_shared_boot.go` | Same |
| `cmd/nerd/chat/session.go` | Manager |
| `cmd/nerd/chat/model_types.go` | Manager, buses on Model / CortexConfig |
| `cmd/nerd/chat/model_update.go` | Explainer, bus init messages |
| `cmd/nerd/chat/model_lifecycle.go` | (lifecycle hooks involving transparency fields) |
| `cmd/nerd/chat/glass_box.go` | Subscribe/render Glass Box + tools |
| `cmd/nerd/chat/commands.go` | Routes `/transparency`, `/glassbox`, `/why` |
| `cmd/nerd/chat/commands_handlers_misc.go` | `/transparency` handler |
| `cmd/nerd/chat/view.go` | Display helpers |
| `cmd/nerd/chat/process.go` | Process path may touch activity/transparency |
| Multiple `*_test.go` under chat | Glass Box / tool tests |

### 2.2 Core executive

| Package / file | Symbols used |
|----------------|--------------|
| `internal/core/virtual_store.go` | Bus fields + setters |
| `internal/core/virtual_store_routing.go` | `ToolEvent`, `GlassBoxEvent` emit |
| `internal/core/shards/manager.go` | GlassBox bus, `emitShardEvent`, `SetTransparencyManager(any)` |
| `internal/core/shards/manager_spawn.go` | postSpawnHook (chat injects buses) |

### 2.3 System shards

| Package / file | Symbols used |
|----------------|--------------|
| `internal/shards/system/base.go` | `GlassBox`, `ToolEventBus` fields + setters |
| `internal/shards/system/router.go` | `ToolEvent` emit on tool completion |

### 2.4 Config (schema only; no import of transparency)

| Package | Role |
|---------|------|
| `internal/config/ux.go` | Defines `TransparencyConfig` |
| `internal/config/user_config.go` | Embeds + `GetTransparencyConfig` |
| `internal/config/config.go` | Top-level config field |

## 3. Dependency diagram

```
internal/config ──────────────┐
                              ▼
                    internal/transparency
                         ▲    ▲    ▲
                         │    │    │
              ┌──────────┘    │    └──────────┐
              │               │               │
      internal/core    internal/core/shards   internal/shards/system
              │               │               │
              └───────────────┼───────────────┘
                              ▼
                       cmd/nerd/chat
                              │
                              ▼
                         Bubble Tea TUI
```

Explainer special edge:

```
internal/mangle ◄── internal/transparency/explainer.go
```

## 4. Cycle prevention patterns

| Problem | Solution in codebase |
|---------|----------------------|
| shards → chat for UI | `postSpawnHook` on ShardManager; chat injects buses |
| core should not own TUI types | Event structs are plain Go in transparency |
| Manager type not in types pkg | Stored as `any` today (gap: should be interface in types if needed) |

## 5. Reverse-deps count (order of magnitude)

Grep for `codenerd/internal/transparency` shows **dozens** of references concentrated in:

1. `cmd/nerd/chat/*` (heaviest)  
2. `internal/core/*` + `internal/core/shards/*`  
3. `internal/shards/system/*`  
4. Docs / skills checklists  

This is a **Tier-2 shared foundation**: small package, wide fan-out. API changes to `GlassBoxEvent` / `ToolEvent` are high blast-radius.

## 6. Related but non-dependent systems

| System | Relationship |
|--------|--------------|
| `internal/logging` | Parallel observability; categories for files/logs, not Glass Box lines |
| Kernel provenance / `/explain` | Higher-fidelity proofs; Explainer formats heuristic traces |
| Campaign artifacts under `.nerd/campaigns` | May mirror logs; not written by this package |
| Prompt JIT compiler | Should emit `CategoryJIT` events; not required import |

## 7. Evidence commands

```powershell
rg "codenerd/internal/transparency" -g "*.go"
rg "NewGlassBoxEventBus|NewToolEventBus|NewTransparencyManager|NewExplainer" -g "*.go"
rg "SetGlassBoxBus|SetToolEventBus|SetTransparencyManager|SetGlassBox" -g "*.go"
```
