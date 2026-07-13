# system — Vision

> Last verified: **2026-07-13**  
> Status: Target architecture for the motherboard package

## Product role

codeNERD needs one **deterministic assembly plane**: given a workspace and credentials, produce a fully wired runtime that any surface (CLI, TUI, workers) can trust.

That plane is `internal/system`.

```
                    ┌─────────────────────────────┐
                    │     internal/system          │
                    │  GetOrBootCortex / Boot*     │
                    └──────────────┬──────────────┘
                                   │ Cortex
          ┌────────────────────────┼────────────────────────┐
          ▼                        ▼                        ▼
     cmd/nerd Cobra          chat TUI                  future workers
```

## Target properties

### 1. One Cortex identity model

A Cortex instance is uniquely identified by:

- workspace root  
- provider  
- API key  
- model  

Switching any dimension boots (or reuses) the correct instance. Stale singletons bound to the wrong context are forbidden (realized as keyed cache; Bug #15).

### 2. One boot pipeline, many entry conveniences

| API | Audience |
|-----|----------|
| `BootCortexWithConfig` | Full DI surface (tests, TUI, advanced callers) |
| `BootCortex` | Thin wrapper for simple call sites |
| `GetOrBootCortex` | Production command handlers — cache + maintenance |

Vision: **all** long-lived process entry points eventually go through `GetOrBootCortex` (or a thin wrapper that still hits the same cache), including TUI, so resource ownership and config-change invalidation stay uniform.

### 3. Soft periphery, hard core

| Class | Boot behavior |
|-------|----------------|
| Kernel evaluate, JIT compiler, system shard start | **Hard fail** |
| Embedding health, MCP connect, taxonomy hydrate, agent sync, hybrid ingest, modular tools | **Soft warn**; continue |
| Missing LLM credentials | Boot **succeeds** with `missingLLMClient` so non-LLM commands still work |

### 4. Separation of construction vs execution

- **Construction** (this package): wire objects, register factories, load cold facts, start system shards.  
- **Execution** (`session`, VirtualStore handlers, shards): run OODA, tools, LLM turns.

The motherboard must not grow business-logic branches for individual tools or intents.

### 5. Explicit teardown

`Cortex.Close` must:

1. Stop system/domain shards and spawn queue  
2. Close SQLite-backed resources (LocalDB, LearningStore, JIT)  
3. Close perception layer globals  
4. Evict this instance from the process cache  

Vision: also cancel maintenance ticker and close MCP bridges (partial today).

### 6. Adapter layer stays thin

Adapters exist only to break import cycles and interface mismatches:

- core.Kernel ↔ prompt.KernelQuerier  
- core.Kernel ↔ mcp.KernelInterface  
- perception.LLMClient ↔ types.LLMClient / mcp.LLMClient  
- core.VirtualStore ↔ types.VirtualStore (session)  

Vision: session file I/O should route through VirtualStore policy path, not raw `os` fallbacks, once the types surface allows it.

## Non-goals

- Owning Mangle policy text or prompt atom content  
- Implementing tool handlers  
- UI, Cobra trees, or slash commands  
- Multi-process Cortex sharing (in-process cache only)

## Success metrics

| Metric | Target |
|--------|--------|
| CLI handlers using GetOrBootCortex | 100% of Cortex consumers in `cmd/nerd` (already near-complete) |
| TUI using same cache identity model | Preferred future state |
| Boot without API keys for store/query tools | Works (today) |
| Failed boot never poisons cache | Guaranteed (today) |
| Close leaves no open SQLite on Windows temp cleanup | Guaranteed for LocalDB/Learning/JIT (today) |
