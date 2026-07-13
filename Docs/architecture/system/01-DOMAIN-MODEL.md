# system — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/system/` (complete internal coverage)
> **Implementation: `internal/system/` — 5 non-test .go, 11 tests, 1 .mg**


## Package

`internal/system/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `AgentOnDisk` | `internal/system/agent_registry.go:19` |
| `SystemKernel` | `internal/system/factory.go:45` |
| `BootConfig` | `internal/system/factory.go:53` |
| `Cortex` | `internal/system/factory.go:209` |
| `LocalStoreTraceAdapter` | `internal/system/factory_adapters.go:25` |
| `KernelAdapter` | `internal/system/factory_adapters.go:46` |
| `HolographicCodeScope` | `internal/system/holographic_code_scope.go:23` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `DiscoverAgentsOnDisk` | `internal/system/agent_registry.go:25` |
| `SyncAgentRegistryFromDisk` | `internal/system/agent_registry.go:84` |
| `SyncAgentRegistryFromDiscovered` | `internal/system/agent_registry.go:95` |
| `Close` | `internal/system/cortex_close.go:13` |
| `GetOrBootCortex` | `internal/system/factory.go:128` |
| `ResetGlobalCortex` | `internal/system/factory.go:176` |
| `ResetCortexForWorkspace` | `internal/system/factory.go:186` |
| `Complete` | `internal/system/factory.go:241` |
| `CompleteWithSystem` | `internal/system/factory.go:245` |
| `CompleteWithTools` | `internal/system/factory.go:249` |
| `SpawnTask` | `internal/system/factory.go:256` |
| `SpawnTaskWithContext` | `internal/system/factory.go:279` |
| `Execute` | `internal/system/factory.go:310` |
| `StartMaintenanceSchedule` | `internal/system/factory.go:321` |
| `BootCortex` | `internal/system/factory.go:373` |
| `BootCortexWithConfig` | `internal/system/factory.go:1016` |
| `IngestHybridPrompts` | `internal/system/factory.go:1085` |
| `StoreReasoningTrace` | `internal/system/factory_adapters.go:33` |
| `LoadReasoningTrace` | `internal/system/factory_adapters.go:39` |
| `NewKernelAdapter` | `internal/system/factory_adapters.go:54` |
| `Query` | `internal/system/factory_adapters.go:58` |
| `AssertBatch` | `internal/system/factory_adapters.go:74` |
| `Complete` | `internal/system/factory_adapters.go:147` |
| `CompleteWithSystem` | `internal/system/factory_adapters.go:151` |
| `CompleteWithTools` | `internal/system/factory_adapters.go:156` |
| `Assert` | `internal/system/factory_adapters.go:177` |
| `Query` | `internal/system/factory_adapters.go:221` |
| `Retract` | `internal/system/factory_adapters.go:265` |
| `LoadFacts` | `internal/system/factory_adapters.go:291` |
| `Query` | `internal/system/factory_adapters.go:295` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/system/debug_program_ERROR.mg` | 16308 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **System factory / boot wiring helpers**
