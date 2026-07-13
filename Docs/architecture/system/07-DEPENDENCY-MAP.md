# system — Dependency Map

> Last verified: **2026-07-13**

## 1. Upstream (what system imports)

From `factory.go` and related sources — grouped by role:

### Executive / runtime core

| Package | Use |
|---------|-----|
| `codenerd/internal/core` | CortexKernel, RealKernel, VirtualStore, API scheduler, Dream*, Transaction, limits, adapters |
| `codenerd/internal/core/shards` | ShardManager, SpawnQueue |
| `codenerd/internal/types` | Shard types, tool defs, session context, priorities |

### Perception / articulation / prompts

| Package | Use |
|---------|-----|
| `codenerd/internal/perception` | LLM clients, transducer, taxonomy, tracing |
| `codenerd/internal/articulation` | PromptAssembler |
| `codenerd/internal/prompt` | JIT compiler, AtomLoader, corpus |
| `codenerd/internal/prompt/sync` | AgentSynchronizer |

### Effectors / world / tools

| Package | Use |
|---------|-----|
| `codenerd/internal/session` | Executor, Spawner, JITExecutor, TaskExecutor |
| `codenerd/internal/tactile` | DirectExecutor, FileEditor |
| `codenerd/internal/world` | Scanner, FileScope, EnsureDeepFacts, Cartographer |
| `codenerd/internal/shards` | RegisterAllShardFactories |
| `codenerd/internal/shards/system` | tactile_router, campaign_runner constructors |
| `codenerd/internal/browser` | SessionManager |
| `codenerd/internal/mcp` | Integration bridge |
| `codenerd/internal/autopoiesis` | Orchestrator / Ouroboros tools |

### Memory / config / platform

| Package | Use |
|---------|-----|
| `codenerd/internal/store` | LocalStore, LearningStore, graph adapter |
| `codenerd/internal/embedding` | EmbeddingEngine |
| `codenerd/internal/config` | UserConfig, FindWorkspaceRoot, JIT, world, limits |
| `codenerd/internal/logging` | Category loggers |
| `codenerd/internal/usage` | Tracker |
| `codenerd/internal/sqlpragmas` | SQLite pragmas on corpus DBs |
| `codenerd/internal/mangle` | Engine for browser manager |

### External

| Package | Use |
|---------|-----|
| `github.com/mattn/go-sqlite3` | blank import for corpus driver |
| `codeberg.org/TauCeti/mangle-go/{ast,parse,analysis}` | Fact string parse in adapters |

**Dependency shape:** system is a **composition root**. It sits near the top of the import DAG and must not be imported by leaf domain packages (core, store, etc.) to avoid cycles. HolographicCodeScope exists specifically because `core` cannot import `world`.

## 2. Downstream (who imports system)

### Production

| Consumer | Import alias | Entry |
|----------|--------------|-------|
| `cmd/nerd/cmd_advanced.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_knowledge.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_interactive.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_instruction.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_direct_actions.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_test_context.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_systems.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_spawn.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_query.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_transparency.go` | coresys | GetOrBootCortex |
| `cmd/nerd/cmd_campaign.go` | coresys | (system import present) |
| `cmd/nerd/dom_cmd.go` | coresys | DOM commands |
| `cmd/nerd/chat/session_shared_boot.go` | nerdsystem | **BootCortexWithConfig** |
| `cmd/nerd/chat/session_boot.go` | nerdsystem | chat boot helpers |
| `cmd/nerd/chat/session_boot_helpers.go` | nerdsystem | helpers |
| `cmd/nerd/chat/ingest.go` | nerdsystem | ingest path |

### Internal tests only

Self-tests under `internal/system/*_test.go`. No other `internal/*` package imports `system` (composition root pattern).

## 3. Evidence command

```powershell
rg "codenerd/internal/system" -g "*.go" --glob "!*_test.go"
rg "GetOrBootCortex|BootCortexWithConfig|BootCortex\(" -g "*.go"
```

## 4. Layer diagram

```
cmd/nerd ──────────────┐
cmd/nerd/chat ─────────┤
                       ▼
              internal/system   ◄── composition root
                       │
       ┌───────────────┼────────────────┐
       ▼               ▼                ▼
   core/session    perception/prompt   store/world/...
```

## 5. Cyclic-pressure notes

| Risk | Mitigation in system |
|------|----------------------|
| core → world for deep facts | HolographicCodeScope in system implements CodeScope |
| chat → system → chat | system never imports cmd/nerd |
| prompt ↔ core fact types | KernelAdapter converts Fact shapes |
| mcp string facts ↔ kernel | mcpKernelAdapter parse/load/retract |
