# Testing alignment: system

## Required concurrent fixture contract (2026-10-31)

The Chrome rollback fixture explicitly injects `MockSystemKernel`
(`internal/system/factory_research_browser_test.go:142`,
`TestBootRollback_EndsResearchHeadlessChrome`). Browser event producers and
fact readers legitimately run concurrently. The mock must serialize mutations
of its fact/assertion slices and return detached fact/argument containers;
caller-owned ingestion or query slices must not mutate its stored evidence.
Deterministic ownership and concurrent no-loss tests must run without Chrome,
followed by the actual Chrome rollback and normal production-kernel close
fixtures under race. Preserve the mock's intentionally limited semantics:
exact predicate matching, nil QueryAll, existing assertion recording and no-op
retractions. This is a test-helper repair, not a change to production kernel,
browser behavior, permissions or lifecycle acceptance standards.

The local helper implementation serializes writes through `MockSystemKernel.Assert`
and `AssertBatch` and clones argument containers for `Query`
(`internal/system/mocks_test.go:40`, `internal/system/mocks_test.go:48`,
`internal/system/mocks_test.go:24`). `cloneMockFact` detaches the argument slice
while retaining the existing semantics of values inside it
(`internal/system/mocks_test.go:59`). `TestMockSystemKernelOwnsIngestedFacts`,
`TestMockSystemKernelQueryOwnsItsSnapshot` and
`TestMockSystemKernelConcurrentFactProducersAndReaders` provide the deterministic
ownership/no-loss regressions (`internal/system/mock_kernel_concurrency_test.go:12`,
`internal/system/mock_kernel_concurrency_test.go:39`,
`internal/system/mock_kernel_concurrency_test.go:60`). Their fail-before receipts
and local race checks are retained; aggregate verification is recorded separately.
The mock is a test-only type, not the kernel used by ordinary boot:
`initKernel` uses `NewDomainCortex` unless a caller explicitly overrides it
(`internal/system/factory.go:1439`). This evidence does not establish a general
absence of production browser races.

> Last verified: 2026-07-13. Tests are selected by risk boundary, not file count.

## Verification ladder

```powershell
# Fast focused safety and lifecycle gates
go test ./internal/system -run '^(TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard|TestKernelAdapter_|TestStartMaintenanceSchedule_|TestCortexClose_|TestCortexKey|TestGetOrBootCortex|TestBootCortexWithConfigLateFailure)' -count=1

# Concurrency-sensitive prompt and maintenance boundaries
go test -race ./internal/system -run '^(TestKernelAdapter_CompilationScopesIsolateConcurrentPrompts|TestStartMaintenanceSchedule_NoImmediateRunAndFastCancel|TestCortexClose_StopsMaintenanceBeforeLocalDB|TestCortexKey.*|TestGetOrBootCortex.*|TestBootCortexWithConfigLateFailureRollsBackAcquiredResources|TestCortexCloseIsIdempotent)$' -count=1

# Complete package
go test ./internal/system/... -count=1

# Explicit full boot
go test ./internal/system -run '^TestBootCortexEndToEnd$' -count=1
```

## Current risk map

| Risk | Current tests | Verdict |
|---|---|---|
| Complete boot object graph | `boot_test.go#TestBootCortexEndToEnd`, `factory_boot_test.go` overrides/no-LLM | **VERIFIED CURRENT** for local package boot |
| Authorization shard ownership and exact matching | `cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` | **VERIFIED CURRENT** |
| Prompt selector cross-talk/live-kernel mutation | four `TestKernelAdapter_*` cases in `prompt_kernel_scope_test.go` | **VERIFIED CURRENT** for concurrent, error, cancel, and retry-cache cases |
| Maintenance immediate run and shutdown ordering | three `maintenance_schedule_test.go` cases | **VERIFIED CURRENT** |
| Spawn routing | `factory_test.go#TestCortex_SpawnTask_Routing`, context and image cases | **VERIFIED CURRENT** |
| MCP and session kernel adapters | `factory_adapters_test.go`, `session_kernel_adapter_test.go` | **VERIFIED CURRENT** for covered methods |
| User-agent discovery/registry and HolographicCodeScope edges | `agent_registry_coverage_test.go` | **VERIFIED CURRENT** |
| CodeDOM through VirtualStore/Mangle | `dom_demo_test.go`, `dom_mangle_test.go`, `tool_compilation_test.go` | **VERIFIED CURRENT** for fixtures |
| Close step timeout/error/panic boundary | `cortex_close_test.go` plus minimal Cortex tests | **PARTIAL**; no real stuck subsystem or all-resource ownership test |
| Cache behavior | `factory_cache_test.go` | **VERIFIED CURRENT** for normalized reuse, disabled-set split, and failed-boot retry; Close eviction and Reset semantics remain open |
| Disabled-shard cache identity | cache behavior tests plus helper normalization/redaction tests | **VERIFIED CURRENT** |
| Boot failure rollback | `factory_rollback_test.go#TestBootCortexWithConfigLateFailureRollsBackAcquiredResources` | **VERIFIED CURRENT** for late LocalDB/Learning/JIT/embedding cleanup; queue/MCP/browser fault injection and cleanup-error joining are residuals |
| Canonical shard manifest | `registration_manifest_test.go` plus exact policy routing test | **VERIFIED CURRENT** for unique ownership, canonical boot consumption, and complete permission envelope |
| Session file adapter policy | only direct file-I/O behavior | **GAP**; no permission, containment, Dreamer, or no-double-effect proof |

## Required residual regressions

### P1: complete cache identity and eviction

The normalized disabled-shard slice is covered. Add engine/provider-mode
identity, explicit Close eviction, and Reset semantics without weakening the
current secret-redaction and normalization tests.

### P1: lifecycle registry breadth

Extend the existing forced late failure with fakes for spawn queue, MCP, browser,
and every optional closer. Assert exact reverse acquisition order, exactly-once
cleanup, joined cleanup errors, caller-owned override preservation, Windows
rename/reopen of SQLite, queue exit, and successful immediate retry.

### P1: adapter policy

Exercise session ReadFile/WriteFile against an exact permitted envelope. Negative
cases must cover missing kernel, target/payload mismatch, workspace escape,
Dreamer block, validation failure, and prevention of double execution.

### P1: teardown registry

Construct a Cortex containing every optional resource behind fakes that record
close order. Prove complete reverse order, idempotence, bounded error aggregation,
and no secrets in the resulting receipt.

## Receipt from this corpus pass

```text
go test -count=1 ./internal/system/...
ok codenerd/internal/system 79.191s

go test -race -count=1 -timeout=180s -run \
  'Test(CortexKey|GetOrBootCortex|BootCortexWithConfigLateFailure|CortexCloseIsIdempotent)' \
  ./internal/system
ok codenerd/internal/system 9.091s
```

The strict corpus validator's fixed verification profile also invokes the system
package. The receipt does not claim full `go test ./...`, network MCP/browser
connectivity, fuzzing, a long-running campaign, or whole-package `-race`.

## Handoff rule

After a system factory or adapter change, run the narrow regression first, the
focused race gate if concurrency/lifecycle changed, then the full system package.
For authorization changes also run the relevant core VirtualStore and session
safety suites. A build-only pass never overrides a behavioral failure.
