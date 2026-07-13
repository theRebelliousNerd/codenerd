# Gap analysis: system

> The rows below compare the reviewed live tree with [01-VISION.md](01-VISION.md).
> Feature decisions and acceptance contracts live only in [TODO.md](TODO.md).

## Evidence-ranked matrix

| Priority | Gap | Current evidence | Desired boundary | Verdict |
|---:|---|---|---|---|
| P1 | Session file adapter bypasses VirtualStore | `factory_adapters.go#sessionVirtualStoreAdapter.ReadFile` and `.WriteFile` call `os` | Typed contained file capabilities with exact permission and no double execution | **BUILD** |
| P1 | Engine/provider mode is absent from cache identity | `resolveProviderModelForKey` returns provider/model; config engine can vary independently | One canonical typed identity for every boot-shaping input | **BUILD** |
| P1 | Lifecycle cleanup is enumerated rather than registered | rollback reuses `cortexFromBootContext(...).Close`; Close owns MCP/browser/closable embedding and stores, but no typed acquisition order/ownership record exists | Typed acquisition registry with caller-owned override policy and cleanup receipt | **EVOLVE** |
| P2 | Cache eviction/reset edges lack decisive tests | reuse, disabled-set split, and failed-boot retry are covered; explicit Close eviction and Reset semantics are not | Close eviction plus evict-only/reset-and-close contract tests | **BUILD** |
| P2 | Chat uses direct BootCortexWithConfig | `cmd/nerd/chat/session_shared_boot.go#performSystemBootShared` | Explicit decision: shared cache identity or intentionally separate lifecycle | **BLOCKED_BY_SPEC** |
| P2 | Reset evicts without Close | `ResetGlobalCortex`, `ResetCortexForWorkspace` | Separate explicit evict-only and reset-and-close APIs | **EVOLVE** |
| P2 | Trace load adapter is a nil stub | `factory_adapters.go#LocalStoreTraceAdapter.LoadReasoningTrace` returns nil, nil | Implement or remove the advertised read capability | **BUILD** |
| P2 | No correlated boot receipt | category logs are stage-local | Redacted stage/resource/degradation/close artifact | **EVOLVE** |
| P3 | Crash artifact remains in source tree | `internal/system/debug_program_ERROR.mg` | Relocate future dumps under workspace `.nerd/debug/` and remove tracked accident safely | **EVOLVE** |

## Closed truth gaps

| Former gap | Current evidence | Status |
|---|---|---|
| Maintenance cancel discarded / DB-close race | `factory.go#Cortex.StartMaintenanceSchedule`, `cortex_close.go#Cortex.Close`, `maintenance_schedule_test.go` | **VERIFIED CURRENT** |
| Authorization predicates split across Cortex shards | `defaultKernelShardConfigs`, exact policy routing regression | **VERIFIED CURRENT** |
| Destructive route can continue without Dreamer | `VirtualStore.RouteAction` and `PreflightDestructiveToolCall` fail closed | **VERIFIED CURRENT** |
| Prompt selector facts mutate live Cortex | `KernelAdapter.NewCompilationScope` and prompt-scope regressions | **VERIFIED CURRENT** |
| Effect boundary loses executive correlation | VirtualStore parses and reuses the supplied action ID | **VERIFIED CURRENT** |
| Disabled-system-shard requests alias in cache | `normalizeDisableSystemShards`, `cortexKey`, and `TestGetOrBootCortexDisabledShardSetIsPartOfIdentity` | **VERIFIED CURRENT** |
| Failed late boot leaves acquired DBs/workers alive | `bootCortexWithSteps`, `rollbackBootContext`, and forced late-failure regression | **VERIFIED CURRENT** for enumerated owned resources |
| Predicate ownership has two drifting boot tables | production `defaultKernelShardConfigs` consumes `DefaultShardPredicateManifests`; uniqueness and exact-envelope tests pass | **VERIFIED CURRENT** |

## Dependency order

```text
exact executive envelope + canonical manifest (verified)
  +--> disabled-shard cache identity (verified)
  |      +--> complete engine identity
  |
  +--> transactional aggregate rollback (verified slice)
  |      +--> typed acquisition registry / receipt
  |
  +--> policy-preserving session adapter
```

The adapter repair can proceed independently after its typed capability and
double-execution contract are pinned. The resource receipt can now build on a
real rollback boundary, but it must not overstate the current enumerated Close
path as exact reverse-order ownership metadata.

## Non-gaps and non-goals

- Missing LLM credentials intentionally produce `missingLLMClient`; store and
  query operations can still boot.
- Unavailable embeddings, agent sync, hybrid ingest, and MCP connection are
  intentionally degradable today. The gap is receipt/ownership clarity, not
  necessarily hard failure.
- Global boot serialization is acceptable until measured multi-workspace
  contention proves otherwise.
- System should not absorb Mangle policy, prompt atoms, tool handlers, or UI.
