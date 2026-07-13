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

A Cortex instance should be uniquely identified by every boot-shaping input:

- workspace root  
- provider  
- API key  
- model  
- engine and provider mode
- normalized disabled-system-shard policy

Switching any dimension boots (or reuses) the correct instance. Stale singletons
bound to the wrong context are forbidden. **VERIFIED CURRENT** for workspace,
provider, API key, model, and the normalized disabled-shard set. **PARTIAL:** the
separately configured engine/provider mode is not yet represented.

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
4. Cancel and join background maintenance before closing SQLite
5. Evict this instance from the process cache

**VERIFIED CURRENT:** steps 1–5 are implemented for shards, maintenance, MCP,
browser, closable embeddings, JIT, LocalDB, LearningStore, perception, and cache
eviction. Named boot-stage errors reuse the same aggregate cleanup, and Close is
idempotent for the tested slice. Vision: replace the manual list with a typed
acquisition registry that proves exact reverse order, caller-owned override
semantics, and bounded cleanup receipts.

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
| Different disabled-system-shard sets never alias in cache | Guaranteed and regression-tested today |
| Every engine/provider-mode identity is distinct | Required; not yet implemented |
| Late boot failure releases verified owned acquisitions | Guaranteed for the regression-tested aggregate cleanup path |
| Redacted boot and close receipt | Proposed bounded operator surface |
