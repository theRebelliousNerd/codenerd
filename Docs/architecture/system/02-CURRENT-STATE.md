# system — Current State

> Last verified: **2026-07-13**  
> Source root: `internal/system/`

## 1. File inventory

### 1.1 Non-test source (5)

| Path | Lines (approx.) | Role |
|------|----------------:|------|
| [`factory.go`](../../../internal/system/factory.go) | ~1151 | Cache, `GetOrBootCortex`, `Cortex`, full boot pipeline, hybrid ingest, maintenance |
| [`factory_adapters.go`](../../../internal/system/factory_adapters.go) | ~433 | Trace, Kernel, MCP, session adapters; streaming stubs |
| [`agent_registry.go`](../../../internal/system/agent_registry.go) | ~284 | Discover `.nerd/agents/*`, sync `.nerd/agents.json` |
| [`holographic_code_scope.go`](../../../internal/system/holographic_code_scope.go) | ~172 | CodeScope + deep fact maintenance for VirtualStore |
| [`cortex_close.go`](../../../internal/system/cortex_close.go) | ~62 | `Cortex.Close` lifecycle |

### 1.2 Tests (11)

| Path | Focus |
|------|--------|
| `boot_test.go` | Full `BootCortex` e2e (skips `-short`) |
| `factory_boot_test.go` | DI overrides; no-LLM boot |
| `factory_test.go` | SpawnTask routing; normalize; VS adapter file I/O |
| `factory_adapters_test.go` | MCP kernel Assert/Query/Retract |
| `factory_helpers_test.go` | Helpers for factory tests |
| `session_kernel_adapter_test.go` | sessionKernelAdapter |
| `agent_registry_coverage_test.go` | Discovery + registry + holographic + Close(nil) |
| `mocks_test.go` | MockSystemKernel, MockLLMClient |
| `tool_compilation_test.go` | VirtualStore compilation delegation smoke |
| `dom_demo_test.go` | CodeDOM end-to-end |
| `dom_mangle_test.go` | CodeDOM + Mangle end-to-end |

### 1.3 Artifacts

| Path | Status |
|------|--------|
| `debug_program_ERROR.mg` | Kernel crash dump artifact (combined .mg sources). **Not** package logic. Should not be treated as intentional source. |

## 2. Package comment (source of truth)

```go
// Package system provides the core initialization and factory logic for the Cortex.
// It acts as the "Motherboard" that wires all components together.
```

## 3. Hotspots

### 3.1 `GetOrBootCortex` (factory.go)

Process-global `map[string]*Cortex` keyed by SHA-256 of identity tuple. Double-checked RLock/Lock. Calls `BootCortex` on miss; starts maintenance only on insert.

### 3.2 `BootCortexWithConfig` stage machine

Ordered `init*` functions (see [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md)). This is the densest control-flow hotspot in the package.

### 3.3 Multi-client LLM fan-out

During perception init:

- **main** — scheduled as `"main"` (TUI agent)  
- **shards** — optional worker from user config, scheduled as `"shards"`  
- **image_generator** — Gemini Nano Banana 2 family only, scheduled as `"image_generator"`

### 3.4 HolographicCodeScope

Breaks `core` ↔ `world` import cycle: VirtualStore gets a CodeScope that keeps deep facts (`world.EnsureDeepFacts` / Cartographer) in the kernel when files enter scope.

## 4. Exported symbols (summary)

See [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) for the full table. Headline exports:

- Types: `SystemKernel`, `BootConfig`, `Cortex`, `AgentOnDisk`, `KernelAdapter`, `LocalStoreTraceAdapter`, `HolographicCodeScope`
- Functions: `GetOrBootCortex`, `BootCortex`, `BootCortexWithConfig`, `ResetGlobalCortex`, `ResetCortexForWorkspace`, `IngestHybridPrompts`, agent discovery/sync, `NewKernelAdapter`, `NewHolographicCodeScope`
- Methods: `Cortex.SpawnTask*`, `StartMaintenanceSchedule`, `Close`

## 5. Runtime artifacts touched during boot

| Path under workspace | Purpose |
|----------------------|---------|
| `.nerd/config.json` | User config / provider / model / JIT / embedding / MCP |
| `.nerd/knowledge.db` | LocalStore (world facts, traces, maintenance) |
| `.nerd/shards/` | LearningStore + agent knowledge DBs |
| `.nerd/prompts/corpus.db` | Project prompt corpus for JIT |
| `.nerd/agents/*/` | User agent dirs (`prompts.yaml` required) |
| `.nerd/agents.json` | Agent registry mirror |
| `.nerd/mangle/scan.mg` | Cold world facts fallback if DB empty |
| `.nerd/browser/sessions.json` | Browser session store path |

## 6. Maturity assessment

| Area | Maturity |
|------|----------|
| Boot completeness | **High** — production path used by nearly all CLI cmds |
| Cache identity | **High** — multi-key, failure-safe |
| DI for tests | **High** — BootConfig overrides |
| Lifecycle completeness | **Medium** — Close strong; maintenance cancel unused |
| Adapter purity | **Medium** — VS file path falls back to os |
| Doc/corpus (this set) | **Rebuilt 2026-07-13** |
