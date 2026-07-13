# 10 — Testing Alignment: JIT config

> Last verified against codebase: **2026-07-13**

## 1. Package tests

| File | Lines | Coverage focus |
|------|------:|----------------|
| `internal/jit/config/types_test.go` | 91 | `Validate` table |

### Cases in `TestAgentConfigValidation`

| Case | Expect |
|------|--------|
| Valid identity + tools + policies | OK |
| Missing identity | Error |
| Whitespace-only identity | Error |
| Empty policies slice | Error |
| Nil policies | Error |
| Missing alias (`base.mg`) | Error |
| Traversal policy | Error |
| Duplicate policy | Error |

**Note:** Test name still says `AgentConfig` historically; type under test is `EffectiveAgentRuntimeConfig`.

## 2. Adjacent unit tests (must stay green when schema changes)

| Area | Path | Why |
|------|------|-----|
| ConfigFactory Generate | `internal/prompt/config_factory_test.go` | Field population, merge, Validate pass-through |
| Default factory Validate | `TestDefaultConfigFactory_OutputPassesValidate` | Every default intent must Validate |
| Fallback Validate | `TestDefaultConfigFactory_FallbackPassesValidate` | Fallback identity still needs policies |
| Generation matrix | `internal/prompt/config_generation_test.go` | Persona tools/policies |
| Core policy inventory | `internal/core/policy_inventory_test.go` | Every stable set resolves to embedded boot members; aliases/traversal fail |
| Provider parity | `internal/prompt/config_policy_registry_test.go` | Both default config providers share canonical sets and every generated policy validates |
| Gaps | `internal/prompt/prompt_gaps_test.go` | nil slices, determinism, mutation safety |
| Spawner YAML | `internal/session/spawner_config_test.go` | Flat YAML shape |
| Spawner validation/bounds | `TestLoadSpecialistConfigRejectsInvalidRuntimeConfig`, `TestLoadSpecialistConfigPreservesBoundaryGates` | Blank identity, missing policies, traversal, size cap |
| Spawner generate | `spawner_improvements_test.go`, `spawner_gaps_test.go` | Fallback/empty paths |
| Executor tools | `executor_capability_test.go`, `executor_boundary_test.go` | Fail-closed nil/empty/unlisted modular and Ouroboros routes, Piggyback catalog, concurrent reads |

## 3. Integration / e2e

Build tag often required: `//go:build integration`

| Test file | Exercises |
|-----------|-----------|
| `tests/e2e/specialist_config_boundary_test.go` | YAML load / specialist boundary |
| `tests/e2e/session_clean_loop_integration_test.go` | Clean loop + tool budgets |
| `tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go` | Full stack |
| `tests/e2e/orchestrator_executor_integration_test.go` | Orchestrated multi-agent |
| `tests/e2e/orchestrator_executor_race_integration_test.go` | Concurrency |
| `tests/e2e/campaign_session_integration_test.go` | Campaign + config mocks |
| `tests/e2e/piggyback_executor_full_boundary_test.go` | Piggyback + allowlist |
| `tests/e2e/cross_boundary_integration_test.go` | Cross-package boundaries |
| Others importing `jit/config` | Async lifecycle, tool safety fallback, session isolation |

## 4. Commands

```powershell
# Core package
go test ./internal/jit/...

# Schema consumers
go test ./internal/prompt/ -count=1 -run "ConfigFactory|DefaultConfigFactory|ConfigAtom"
go test ./internal/session/ -count=1 -run "Config|Spawner|Specialist|Tool|Validate"

# Broad safety net when changing tags/fields
go test ./internal/prompt/ ./internal/session/ ./internal/jit/...

# Integration (when environment ready)
go test ./tests/e2e/ -tags=integration -count=1 -run "Specialist|Session|Config|Orchestrator"
```

## 5. Coverage status and residual gaps

| Gap | Severity | Suggested test |
|-----|----------|----------------|
| Specialist `Validate` after YAML unmarshal | **Verified** | `TestLoadSpecialistConfigRejectsInvalidRuntimeConfig` |
| `ToolLoop` honored by executor | High | Assert MaxIterations from cfg once wired |
| `RequirePolicyEnforcement` behavior | Medium | Flag true + empty policies refuses |
| Slice mutation across SubAgents | Medium | Already partially in prompt_gaps; extend to session |
| JSON tags round-trip | Low | Marshal/unmarshal parity with YAML |
| Empty/unlisted modular tool execution | **Verified** | `TestExecutorExecuteToolCallRequiresEffectiveCapability` |
| Ouroboros tool outside AllowedTools | **Verified** | `TestExecutorOuroborosRegistryDoesNotGrantCapability` asserts execute count remains zero |
| Default policy references resolve | **Verified** | `TestDefaultAgentPolicySetsResolveToEmbeddedPolicyInventory`, `TestDefaultConfigFactoryPoliciesResolveAgainstCoreInventory`, and provider parity |
| Policy set identity/version reaches session | Medium | Add receipt/integration test once global-versus-selective semantics are pinned |
| Specialist path/size hostile cases | **Verified** | `TestLoadSpecialistConfigPreservesBoundaryGates` |
| Rename test to EffectiveAgentRuntimeConfig | Low | Hygiene |

## 6. Alignment score

| Layer | Alignment |
|-------|-----------|
| Schema unit tests | **Good** for Validate edges |
| Factory tests | **Strong** |
| Executor field completeness | **Partial** (capability tools yes; ToolLoop/Safety/policy identity no) |
| e2e | **Present**, mock-heavy for factory |

Overall: schema and repaired specialist/capability boundaries are **test-backed**,
including canonical policy registry/provider parity and a focused race run;
incomplete field wiring and absent set identity/selective application mean green
tests can still miss product expectations for other YAML knobs.
