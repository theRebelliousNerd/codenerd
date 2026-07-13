# system — Testing Alignment

> Last verified: **2026-07-13**

## 1. How to run

```powershell
# Package tests
go test ./internal/system/...

# Skip full boot e2e
go test ./internal/system/... -short

# Full boot only
go test ./internal/system/ -run TestBootCortexEndToEnd -count=1

# Focused
go test ./internal/system/ -run TestBootCortexWithConfig -count=1
go test ./internal/system/ -run TestGetOrBoot -count=1   # may not exist yet
go test ./internal/system/ -run AgentRegistry -count=1
```

Note: full boot needs CGO/sqlite and can take tens of seconds (90s timeout in e2e).

## 2. Existing tests mapped to behaviors

| Test | File | Behavior covered |
|------|------|------------------|
| `TestBootCortexEndToEnd` | boot_test.go | Full assembly; non-nil kernel, VS, shards, session, JIT, assembler, transducer, LocalDB; Close |
| `TestBootCortexWithConfig_Overrides` | factory_boot_test.go | Kernel/LLM/UserConfig inject; system shards disabled |
| `TestBootCortexWithConfig_NoLLMConfigured` | factory_boot_test.go | Boot succeeds; Complete fails with missing client |
| `TestNormalizeShardTypeName` | factory_test.go | Slash/space trim |
| `TestCortex_SpawnTask_Routing` | factory_test.go | TaskExecutor path; intent not rewritten |
| `TestCortex_SpawnTaskWithContext_Routing` | factory_test.go | Context path |
| `TestSessionVirtualStoreAdapter` | factory_test.go | os Read/WriteFile fallback |
| `TestMCPKernelAdapter_*` | factory_adapters_test.go | Assert/Query/Retract + invalid fact |
| `TestSessionKernelAdapter` | session_kernel_adapter_test.go | Kernel forwarding |
| Agent discovery suite | agent_registry_coverage_test.go | Empty/missing/skip/sort/registry upsert |
| Holographic suite | agent_registry_coverage_test.go | workers default, Close, empty scope, ensureDeepFacts no-ops |
| `TestCortexClose_WhenNil` | agent_registry_coverage_test.go | nil Close |
| `TestVirtualStore_CompilationDelegation` | tool_compilation_test.go | VS compilation smoke |
| `TestCodeDOM_EndToEnd` | dom_demo_test.go | DOM demo integration |
| `TestCodeDOM_Mangle_EndToEnd` | dom_mangle_test.go | DOM + Mangle |

Mocks: `MockSystemKernel`, `MockLLMClient` (`mocks_test.go`); `MockTaskExecutor` (`factory_test.go`).

## 3. Coverage gaps

| Area | Gap severity | Notes |
|------|--------------|-------|
| `GetOrBootCortex` cache hit/miss | **High** | No dedicated unit test for double-check, multi-key, failure non-cache |
| `ResetCortexForWorkspace` / `ResetGlobalCortex` | Medium | No tests found |
| Maintenance schedule | Medium | No test that ticker/runMaintenance runs or Close race |
| `IngestHybridPrompts` | Medium | Exercised only inside full boot if hybrid prompts present |
| Image / worker LLM branch | Low | Needs config fixtures |
| MCP ConnectAll failure | Low | Soft path |
| SpawnTask **system** shard path | Medium | Only TaskExecutor path tested; not TypeSystem profile branch |
| sessionLLMAdapter ToolResults | Low | No adapter-level test |

## 4. Alignment with package principles

| Principle | Test support |
|-----------|--------------|
| Soft periphery / hard core | NoLLM boot test + full boot |
| DI overrides | BootConfig overrides test |
| Spawn routing | Unit tests with mock TaskExecutor |
| Agent registry best-effort | Extensive discovery tests |
| Cache identity | **Missing** — highest priority addition |

## 5. Recommended next tests (do not implement in this doc task)

1. `TestGetOrBootCortex_CacheHitSameKey`  
2. `TestGetOrBootCortex_DifferentModelDifferentInstance`  
3. `TestGetOrBootCortex_FailedBootNotCached` (force Boot fail via bad override if possible)  
4. `TestCortexClose_EvictsCache`  
5. `TestSpawnTask_SystemProfileRoutesToShardManager`  
6. `TestRunMaintenance_NilLocalDBNoPanic` after Close  

## 6. Relation to campaign / assault

Package tests are the unit gate. Long-horizon `/campaign assault` validates boot indirectly via CLI/TUI but is not a substitute for GetOrBootCortex unit coverage.
