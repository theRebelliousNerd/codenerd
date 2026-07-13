# system — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/system/` (complete internal coverage)
> **Implementation: `internal/system/` — 5 non-test .go, 11 tests, 1 .mg**


## 1. Purpose

System factory / boot wiring helpers

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/system/` | Primary implementation |
| `Docs/architecture/system/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (5 src / 11 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/system/factory.go` | 1136 | source |
| `internal/system/factory_adapters.go` | 433 | source |
| `internal/system/agent_registry.go` | 284 | source |
| `internal/system/holographic_code_scope.go` | 172 | source |
| `internal/system/cortex_close.go` | 62 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `AgentOnDisk` | `internal/system/agent_registry.go:19` |
| `SystemKernel` | `internal/system/factory.go:45` |
| `BootConfig` | `internal/system/factory.go:53` |
| `Cortex` | `internal/system/factory.go:209` |
| `LocalStoreTraceAdapter` | `internal/system/factory_adapters.go:25` |
| `KernelAdapter` | `internal/system/factory_adapters.go:46` |
| `HolographicCodeScope` | `internal/system/holographic_code_scope.go:23` |

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
