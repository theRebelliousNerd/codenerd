# 10 — Testing alignment

## Current evidence

**VERIFIED CURRENT.** On 2026-07-13 both focused commands passed:

```text
go test -count=1 -timeout=240s ./internal/config/...       PASS
go test -race -count=1 -timeout=240s ./internal/config/... PASS
```

The receipt is `artifact:Docs/architecture/config/_progress.md`.

| Risk | Existing discriminator |
|---|---|
| JSON missing/malformed/round-trip | `internal/config/config_comprehensive_test.go#TestLoadUserConfig_WhenMalformedJSON_ShouldReturnError`; `#TestUserConfigSave_WhenRoundTrip_ShouldPreserve` |
| provider honesty | `internal/config/config_comprehensive_test.go#TestGetActiveProvider_WhenExplicitProvider_ShouldUseMatchingKey`; `#TestGetActiveProvider_WhenProviderSetButKeyMissing_ShouldReturnEmptyKey` |
| engine validation/defaults | `internal/config/config_comprehensive_test.go#TestSetEngine_WhenInvalid_ShouldReturnError`; `#TestGetCodexCLIConfig_WhenNil_ShouldReturnDefaults` |
| workspace capture | `internal/config/config_test.go#TestFindWorkspaceRoot_BypassesStrayNestedNerd`; `#TestFindWorkspaceRoot_FallsBackToGoMod` |
| scheduler | `internal/config/config_test.go#TestUserConfig_GetEffectiveAPISchedulerPolicy` |
| YAML env precedence | `internal/config/env_override_test.go#TestEnvOverrides_LLM` |
| MCP filtering | `internal/config/config_defaults_test.go#TestToMCPServerConfigs` |
| feature side effect | `internal/features/config_roundtrip_test.go#TestLoadUserConfig_InstallsFeaturesIntoRegistry` |
| strict decode / explicit routing | `internal/config/config_security_test.go#TestLoadUserConfigRejectsUnknownAndTrailingJSON`; `#TestHasExplicitLLMSelection` |
| atomic/private persistence | `internal/config/config_security_test.go#TestPrivateAtomicWritePreservesOriginalOnReplaceFailure`; `#TestUserConfigSaveIsPrivateAndRoundTrips` |
| trace defaults | `internal/config/config_security_test.go#TestSensitiveTracingDefaultsOff` |
| wizard merge | `cmd/nerd/chat/config_wizard_save_test.go#TestSaveConfigWizardPreservesUnownedSettings` |
| shared execution projection | `internal/system/factory_execution_test.go#TestExecutionLayerConfigsProjectUserPolicy`; `#TestExecutionLayerConfigsRejectInvalidPolicy` |
| fail-closed shared boot | `internal/system/factory_execution_test.go#TestInitCoreComponentsRejectsPresentInvalidConfig` |
| Codex subprocess isolation | `internal/perception/codex_cli_client_test.go#TestCodexCLIClient_buildCLIArgs_FiltersEffectOverrides` |

## Why green is not closure

**PARTIAL.** Focused tests now discriminate wizard field preservation,
pre-rename failure safety, Unix owner-only mode, unknown/trailing JSON,
malformed-present shared boot, shared execution projection/containment, Codex CLI
isolation, and trace-off defaults. Remaining gaps include full semantic
validation, concurrent writer/version behavior, Windows ACLs/backups, opt-in raw
trace redaction/retention, campaign/dormant parity, immutable reload, fuzzing,
and concurrent timeout Set/Get.

The race gate passes because tests do not concurrently exercise the mutable
global timeout or a shared mutable `UserConfig`; it is not evidence those paths
are race-safe.

## Required risk-selected gates

| Gate | Positive case | Negative/adversarial case |
|---|---|---|
| unit | every effective resolver accepts valid boundary values | negative/overflow values, reserve sums, bad enum/URL/duration/protocol rejected |
| persistence integration | merge preserves unrelated nested fields; reload equals commit | forced pre-rename failure, competing writers, malformed original, secret mode/redaction |
| boot integration | one snapshot reaches provider/scheduler/JIT/execution/MCP | present invalid file invokes zero downstream constructors and never falls back to env |
| permission integration | projected execution bounds and `permitted/3` both succeed | allowed binary without exact permission denies; permission with disallowed binary denies |
| race | concurrent snapshot readers and controlled reload are race-free | concurrent Set/Get/Save cannot expose partial state |
| fuzz | JSON decoder/migrator/validator never panics and diagnostics are bounded | unknown/deep/large/duplicate/Unicode/hostile path inputs fail deterministically |
| adversarial | trace opt-in produces bounded redacted event | seeded API key, OAuth token, prompt secret and env secret never persist |
| campaign/conformance | chat, shared Cortex and campaign receive equal projection IDs | a new constructor that omits a required projection fails parity CI |

## Fixed verification profile

The corpus validator owns the bounded package command. Structural validation
does not prove semantic wiring; product repairs need the focused regressions
above plus the affected consumer package tests. Live-provider tests are not
required for config parsing and must never consume real credentials for a
redaction gate.
