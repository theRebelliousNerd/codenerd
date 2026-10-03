# Gap analysis: system

## Accepted cold-bootstrap cancellation target (2026-10-02)

**VERIFIED CURRENT — bounded implementation/live exit:** default full-bootstrap HTTP cancellation, stage/cache/rollback controls and focused race pass in `artifact:.corpus-build/runs/all-features-20261002/round6-coldboot-corrected.receipt.json` and `round6-coldboot-race.receipt.json`. Fresh normal-entry cold20s probe exits21.6602s with typed deadline failure and drained output (`round6-firstboot-idle.receipt.json`), within the specified5s allowance. This supersedes the unimplemented observation below for these exact behaviors. Backend learned-store SQL, worst-case cleanup, complete affected suites and model-turn/world context remain separate obligations; GAP-CLI-01 is not globally closed.

**PARTIAL — GAP-SYSTEM-BOOT-CONTEXT:** normal-entry `chat --timeout 20s` on a cold fixture exited after 58.4495 seconds, without an external kill. `factory.go#initKernel` reaches contextless perception/classifier construction, and boot continues constructing JIT/session and starting maintenance after cancellation. The measured cold hydration consumed about 47.922 seconds; cleanup was subsecond in this probe, not the cause. Evidence: `artifact:.corpus-build/runs/all-features-20261002/round5-firstboot-idle.receipt.json` and `02-execution/handoff-5.json`.

**PROPOSED UPLIFT:** carry the caller context through classifier/embedding construction; check it before and after each boot stage; rollback every owned resource on cancellation even when a stage returns nil. Canceled boots must not publish a cached Cortex or start maintenance. Cache admission must be cancellation-aware without detached boot work; valid cached reuse and no-deadline boot remain supported. Preserve joined cleanup errors and typed cancellation identity. Exit requires deterministic default-path blocked embedding, staged cancellation/rollback and retry/cache controls, then a rebuilt sqlite_vec binary with a fresh normal-entry 20-second probe that exits within a stated five-second cleanup allowance and drains output. This is not model-turn/world-model acceptance, and GAP-CLI-01 stays open until its full contract is proved.

## Accepted generated-session bridge target (2026-10-02)

**PARTIAL — GAP-SYSTEM-GENERATED-FEEDBACK:** `internal/system/factory.go#initAutopoiesisAndBrowser` installs the feedback adapter at line 1770; `#initFinalExecutors` supplies the session its raw registry at line 2350. `internal/system/factory_tool_executor.go#orchestratorToolExecutor.ExecuteTool` at line 47 owns automatic tool learning, but generated session execution bypasses it. Root source inspection and artifact:.corpus-build/runs/all-features-20261002/02-execution/handoff-3.json establish this distinction, not passing behavior.

**PROPOSED UPLIFT:** bridge admitted generated calls through the existing executive/feedback route without replacement action IDs, permission widening, duplicate effects or fabricated receipts. Factory ownership carries configured request/lifetime contexts and restores tools consistently. Missing adapter/authorization visibly refuses. Real tool failures retain output and learning exactly once. Pin refinement/recording admission, cancellation and join before claiming shutdown completion. Root acceptance uses actual boot and Process with deterministic model inputs, effects and learning snapshots, plus disconnected-route, feedback-free, refusal, cancellation and recovery controls. Shared core bridge changes require their own accepted spec and path ownership first. Target remains unimplemented/unverified; companion session/testing gaps own consumer and acceptance obligations.

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
| P2 | Reset evicts without Close | Closed 2026-09-25: both evict-only resets were removed (no caller, not even a test). The one eviction left is `Cortex.Close` -> `evictCortexByKey`, which closes what it evicts. | — | **CLOSED** |
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

## Prompt snapshot adapter obligation

The production KernelAdapter and its owned compilation scopes must expose a
complete `QueryAll` fact snapshot in prompt types for dynamic cache identity.
Delegate to the adapter's kernel, propagate errors, and detach returned map,
row, and argument-slice ownership. A scope queries its cloned kernel, not the
live executive. This closes the factory-side dependency of
GAP-PROMPT-DYNAMIC-CACHE-IDENTITY; a compiler-only test cannot prove this wiring.
Verify nil/error behavior, real declared facts, returned-slice isolation, and an
existing scope retaining its original evidence after the live kernel changes.

The adapter is implemented in `internal/system/factory_adapter_snapshot.go:16`.
Real-kernel acceptance in `internal/system/factory_adapter_snapshot_test.go:101`
passes clone retention, detached ownership, private mutation, closed-scope, and
fresh-scope controls. A clean parent requires first-read materialization in
`internal/core/kernel_eval.go:428`; copying its clean flag into an empty clone
store loses evidence. Neither this adapter nor the component tests establish
all factory integrations or multi-workspace policy isolation.
