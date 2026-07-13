# core — Dependency Map

> Last verified: **2026-07-13**  
> Package: `codenerd/internal/core` (+ `codenerd/internal/core/shards`)

## 1. Upstream (core imports)

### First-party

| Import | Why |
|--------|-----|
| `internal/types` | Fact, Kernel, LLMClient, ShardAgent, limits interfaces |
| `internal/logging` | Categorized logs, timers, audit |
| `internal/mangle` | SchemaValidator, DifferentialEngine |
| `internal/mangle/feedback` | Rule normalization for HotLoad |
| `internal/tactile` | Safe command execution, audit logger |
| `internal/store` | LocalStore, LearningStore for VS |
| `internal/tools` | Modular tool registry |
| `internal/transparency` | GlassBoxEventBus, ToolEventBus |
| `internal/features` | Diff eval, per-shard facts flags |
| `internal/config` | Image shard type detection (shards pkg) |

### Third-party

| Import | Why |
|--------|-----|
| `codeberg.org/TauCeti/mangle-go/{ast,analysis,engine,factstore,provenance}` | Logic engine |

### Intentionally not imported

- `internal/session` — session implements core interfaces instead  
- `internal/articulation` / `internal/perception` — reverse deps  
- `cmd/nerd` — reverse dep  

## 2. Downstream (who imports core)

### High-signal consumers

| Area | Usage |
|------|--------|
| `internal/system` | Cortex boot, adapters, holographic scope |
| `internal/session` | Executor tools, kernel assert/query (via types often) |
| `internal/autopoiesis` | Hot rules, ouroboros, checkers |
| `internal/world` | May use Fact types / graph hooks |
| `cmd/nerd`, `cmd/nerd/chat` | Boot kernel + VS; CLI query/dream/shadow |
| `cmd/tools/corpus_builder` | Corpus generation against defaults |
| `tests/e2e/*` | Kernel, VS, Dreamer, Shadow, session integration |

Grep evidence pattern: `"codenerd/internal/core"` across repo (broad; includes tests).

### shards subpackage consumers

| Consumer | Usage |
|----------|--------|
| `VirtualStore` | default `NewShardManager` + SetVirtualStore self-wire |
| system/chat boot | factory registration, spawn hooks |
| e2e campaign/session | spawn plumbing |

Domain agent *types* register from `internal/shards` into this manager.

## 3. Dependency direction diagram

```
                    types  ◄─── core (aliases)
                      ▲
                      │
 perception ──assert──┤
                      │
              ┌───────┴────────┐
              │      core      │
              │ kernel · VS    │
              └───────┬────────┘
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
     session       system         autopoiesis
        │             │
        └──────► cmd/nerd / chat TUI
```

## 4. Embed dependency (build-time)

Go embed of `defaults/**/*.mg` means **binary content depends on Mangle corpus**. Changing policy/schema without rebuilding embeds yields stale constitution in released binaries.

## 5. Runtime file dependencies

| Path | When |
|------|------|
| `.nerd/mangle/extensions.mg` | User schema extensions |
| `.nerd/mangle/policy_overrides.mg` | User policy |
| `.nerd/mangle/learned.mg` | User learned rules |
| Workspace CWD / `workspaceRoot` | Path resolution |
| `debug_program_ERROR.mg` | Write on analyze failure |

## 6. Feature-flag coupling

| Flag / feature | Core impact |
|----------------|-------------|
| `features.IsDiffEvalEnabled` / `CODENERD_DIFF_EVAL` | `kernel_eval` path |
| `features.IsPerShardFactsEnabled` | Cortex `ShardFactRouter` |

## 7. Cyclic-risk notes

Historical cycle: `core → autopoiesis → articulation → core`. Mitigated by:

- types package for shared structs  
- interfaces in core for session/MCP  
- careful autopoiesis imports of core as leaf usage  

When adding imports, re-check `go list -f '{{.ImportPath}} {{.Imports}}'`.

## 8. Test-only dependencies

Core tests use mocks for Kernel/LLM/Executor; policy package has golden edb/golden pairs without full Go boot in some cases.

## 9. What core must not grow into

- Prompt atom authoring system (belongs in `internal/prompt`)  
- Full chat TUI (cmd/nerd)  
- Vector index engine (store/embedding)  
- Domain-specific campaign planner UI  

Core may hold **Decls and routing rules** that those systems assert/query.
