# 02 — Current state

> Realized truth for `internal/config`, inspected 2026-07-13.

## Package shape

**VERIFIED CURRENT.** Seventeen non-test Go files define two aggregates and
their effective-value helpers; five package test files contain the focused
contract suite. There are no `.mg` sources in this package.

| Surface | Owner and behavior |
|---|---|
| JSON authority | `internal/config/user_config.go#UserConfig`, `#LoadUserConfig`, `#UserConfig.Save` |
| Legacy YAML | `internal/config/config.go#Config`, `#Load`, `#Config.Save`, `#Config.applyEnvOverrides` |
| Providers/engines | `internal/config/user_config.go#UserConfig.GetActiveProvider`, `#UserConfig.SetEngine`; CLI structs in `internal/config/llm.go` |
| Limits/scheduler | `internal/config/limits.go#CoreLimits`; `internal/config/user_config.go#UserConfig.GetEffectiveAPISchedulerPolicy` |
| Context/JIT | `internal/config/memory.go#ContextWindowConfig`; `internal/config/jit.go#JITConfig` |
| Execution/integrations | `internal/config/execution.go#ExecutionConfig`; `internal/config/integrations.go#IntegrationsConfig.ToMCPServerConfigs` |
| Process state | `internal/config/llm_timeouts.go#globalLLMTimeouts`; load-time `internal/features/features.go#SetActive` |

## Realized behavior

| Claim | Stable evidence |
|---|---|
| **VERIFIED CURRENT** — missing JSON yields an empty aggregate; malformed JSON errors. | `internal/config/user_config.go#LoadUserConfig`; `internal/config/config_comprehensive_test.go#TestLoadUserConfig_WhenMalformedJSON_ShouldReturnError` |
| **VERIFIED CURRENT** — explicit provider returns only its matching key; Ollama is keyless via a sentinel. | `internal/config/user_config.go#UserConfig.GetActiveProvider`; `internal/config/ollama_worker_config_test.go#TestGetActiveProvider_Ollama` |
| **VERIFIED CURRENT** — `go.mod` wins over nested `.nerd`; `.nerd` is fallback without a module. | `internal/config/user_config.go#FindWorkspaceRoot`; `internal/config/config_test.go#TestFindWorkspaceRoot_BypassesStrayNestedNerd` |
| **VERIFIED CURRENT** — effective scheduler concurrency is bounded by the smaller positive engine cap. | `internal/config/user_config.go#UserConfig.GetEffectiveMaxConcurrentAPICalls`; `internal/config/config_test.go#TestUserConfig_GetEffectiveMaxConcurrentAPICalls` |
| **VERIFIED CURRENT** — enabled MCP entries convert to runtime configs; disabled entries are skipped. | `internal/config/integrations.go#IntegrationsConfig.ToMCPServerConfigs`; `internal/config/config_defaults_test.go#TestToMCPServerConfigs` |
| **VERIFIED CURRENT** — Codex CLI ignores hostile sandbox/shell/override requests and emits read-only/shell-disabled arguments. | `internal/perception/codex_cli_client.go#CodexCLIClient.buildCLIArgs`; `internal/perception/codex_cli_client_test.go#TestCodexCLIClient_buildCLIArgs_FiltersEffectOverrides` |
| **VERIFIED CURRENT** — package and package-race gates pass. | `artifact:Docs/architecture/config/_progress.md` |

## Verified partial seams

| Severity | Claim and absent seam | Evidence |
|---|---|---|
| P0 | **PARTIAL** — atomic/private failure and Unix-mode tests exist, but plaintext values, writer conflicts, backups and Windows ACL semantics remain. | `internal/config/config_security_test.go#TestPrivateAtomicWritePreservesOriginalOnReplaceFailure`; `#TestUserConfigSaveIsPrivateAndRoundTrips` |
| P0 | **PARTIAL** — the wizard preserves representative unowned families in a focused test, but concurrent modification lacks an expected-version contract. | `cmd/nerd/chat/config_wizard_save_test.go#TestSaveConfigWizardPreservesUnownedSettings` |
| P0 | **PARTIAL** — shared Cortex projects binaries/env/directory/timeout with containment tests; campaigns copy binaries/env/directory without shared timeout/containment and dormant legacy boot bypasses the projection. | `internal/system/factory_execution.go#executionLayerConfigs`; `cmd/nerd/cmd_campaign.go#runCampaignStart`; `cmd/nerd/chat/session_boot.go#performSystemBootLegacy` |
| P0 | **PARTIAL** — strict syntax and fail-closed shared boot are tested, but semantic validation is incomplete and some secondary consumers soften load errors. | `internal/config/config_security_test.go#TestLoadUserConfigRejectsUnknownAndTrailingJSON`; `internal/system/factory_execution_test.go#TestInitCoreComponentsRejectsPresentInvalidConfig` |
| P1 | **PARTIAL** — full-prompt trace defaults false and its file requests `0600`, with default tests, but opt-in trace still has no content redaction, response bound or retention contract. | `internal/config/config_security_test.go#TestSensitiveTracingDefaultsOff`; `internal/logging/llm_io_logger.go#LogLLMRequest` |
| P1 | **PARTIAL** — present JSON installs feature state; missing JSON intentionally preserves the previous process-global state, so workspace reuse can inherit it. | `internal/config/user_config.go#LoadUserConfig`; `internal/features/config_roundtrip_test.go#TestLoadUserConfig_InstallsFeaturesIntoRegistry` |
| P1 | **PARTIAL** — timeout presets are centralized, but `globalLLMTimeouts` has no synchronization contract for concurrent Set/Get. | `internal/config/llm_timeouts.go#GetLLMTimeouts`; `internal/config/llm_timeouts.go#SetLLMTimeouts` |

## Dual-model drift

| Concept | JSON effective default | YAML default |
|---|---:|---:|
| concurrent shards | 12 | 4 |
| execution timeout | 30s | 10m |
| context input tokens | 200000 | 128000 |

**VERIFIED CURRENT.** The YAML path is not dead:
`cmd/nerd/main.go#rootCmd` loads `.nerd/config.yaml` to resolve the Cobra timeout.
It does not make YAML the runtime provider/JIT/execution authority.

## Current boundary

**VERIFIED CURRENT.** Config asserts no fact and declares no predicate. Its
Mangle-shaped type stores legacy schema/policy paths and numeric budgets only.
The live `permitted/3` declaration and derivation belong to
`internal/core/defaults/schemas_safety.mg#permitted/3` and
`internal/core/defaults/policy/constitution.mg#permitted/3`.
