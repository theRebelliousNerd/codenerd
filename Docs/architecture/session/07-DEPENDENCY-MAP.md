# 07 — Dependency Map: session

> Last verified: 2026-07-13

## 1. Upstream imports (session → others)

From package sources:

| Import | Use |
|--------|-----|
| `codenerd/internal/articulation` | Piggyback process, control packets, constitutional override |
| `codenerd/internal/core` | ToolRegistry, MangleUpdatePolicy/Filter, types coupling |
| `codenerd/internal/config` (appconfig) | LLM timeouts for spawn |
| `codenerd/internal/jit/config` | `EffectiveAgentRuntimeConfig` |
| `codenerd/internal/logging` | CategorySession |
| `codenerd/internal/perception` | Intent, Transducer, ConversationTurn, SharedTaxonomy |
| `codenerd/internal/prompt` | CompilationContext/Result |
| `codenerd/internal/tools` | Global modular registry |
| `codenerd/internal/types` | Kernel, VirtualStore, LLMClient, SessionContext, ToolCall |
| `gopkg.in/yaml.v3` | Specialist config parse (spawner) |

**No** direct import of `cmd/`, `campaign`, or `system` (good layering).

## 2. Downstream consumers (others → session)

| Consumer | Evidence |
|----------|----------|
| `internal/system/factory.go` | NewExecutor, NewSpawner, NewJITExecutor, Cortex fields |
| `internal/system/factory_test.go` | Mock TaskExecutor |
| `internal/system/factory_adapters.go` | session adapters comments |
| `cmd/nerd/cmd_campaign.go` | Local session stack for orchestrator |
| `cmd/nerd/chat/session_boot.go` | import session |
| `cmd/nerd/chat/session_boot_helpers.go` | import session |
| `cmd/nerd/chat/session_adapters.go` | import session |
| `cmd/nerd/chat/model_types.go` | holds session types |
| `cmd/nerd/chat/delegation_routing.go` | `session.TaskRequest` |
| `cmd/nerd/chat/delegation.go` | TaskExecutor migration comments |
| `internal/campaign/orchestrator_*.go` | TaskExecutor dependency |
| `internal/verification/verifier.go` | import session |
| `tests/e2e/*` | multiple session integration tests |

## 3. Dependency direction diagram

```
perception, prompt, articulation, tools, core, types, config, logging, jit/config
        ▲
        │ imports
   internal/session
        ▲
        │ imports
 system │ campaign │ verification │ cmd/nerd/* │ tests/e2e
```

## 4. Interface seams (cycle breakers)

| Seam | Why |
|------|-----|
| Local `JITCompiler` / `ConfigFactory` | Avoid over-binding concrete prompt types beyond Compile/Generate |
| `SessionPersister` | Storage without importing store package |
| `Compressor` | Avoid importing internal/context |
| `InteractiveExecutiveGate` | Optional VS capabilities without forcing full RouteAction |
| system `session*Adapter` types | Bridge Cortex concretes to `types.*` expected by session |

## 5. Shared mutable dependencies

| Resource | Sharing | Risk control |
|----------|---------|--------------|
| Kernel | Shared across SubAgents | Task intent IDs; pending_action retract |
| VirtualStore | Shared | Executive gate; workspace FS reality |
| LLM client | Shared (worker vs main) | Priority context; capability hints |
| tools.Global() | Process-wide | Allow-list + safety |

## 6. Version / budget coupling

Token budget must be set from user context window configuration by higher layers when available. Defaults are generous (65536) to avoid the historical 8192 atom-drop bug.
