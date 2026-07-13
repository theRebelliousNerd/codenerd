# 09 — Safety and invariants

## Constitutional boundary

**VERIFIED CURRENT.** Config cannot grant an action. It has no Mangle `Decl`,
producer, or consumer for permission facts. Default-deny authorization is owned
by `internal/core/defaults/schemas_safety.mg#permitted/3`,
`internal/core/defaults/policy/constitution.mg#permitted/3`, and
`internal/core/virtual_store_routing.go#VirtualStore.RouteAction`.

Execution allowlists, scheduler ceilings, CLI sandbox settings and fact limits
are defense-in-depth inputs. They must be enforced in addition to, never instead
of, exact `(Action, Target, Payload)` permission.

## Proven invariants

| Invariant | Evidence |
|---|---|
| Explicit provider cannot borrow another provider key. | `internal/config/user_config.go#UserConfig.GetActiveProvider`; `internal/config/config_comprehensive_test.go#TestGetActiveProvider_WhenProviderSetButKeyMissing_ShouldReturnEmptyKey` |
| Engine-specific positive concurrency can only lower the core API ceiling. | `internal/config/user_config.go#UserConfig.GetEffectiveMaxConcurrentAPICalls`; `internal/config/config_test.go#TestUserConfig_GetEffectiveMaxConcurrentAPICalls` |
| Explicit false survives JSON for JIT/reflection enablement. | `internal/config/jit.go#JITConfig.UnmarshalJSON`; `internal/config/reflection.go#ReflectionConfig.UnmarshalJSON` |
| `go.mod` prevents nested `.nerd` workspace capture. | `internal/config/user_config.go#FindWorkspaceRoot`; `internal/config/config_test.go#TestFindWorkspaceRoot_BypassesStrayNestedNerd` |
| Disabled MCP servers are not emitted. | `internal/config/integrations.go#IntegrationsConfig.ToMCPServerConfigs`; `internal/config/config_defaults_test.go#TestToMCPServerConfigs` |
| Hostile Codex CLI config cannot enable workspace writes, shell, or arbitrary overrides. | `internal/perception/codex_cli_client.go#CodexCLIClient.buildCLIArgs`; `internal/perception/codex_cli_client_test.go#TestCodexCLIClient_buildCLIArgs_FiltersEffectOverrides` |
| Shared Cortex rejects escaped execution directories and bad/non-positive timeouts. | `internal/system/factory_execution.go#executionLayerConfigs`; `internal/system/factory_execution_test.go#TestExecutionLayerConfigsRejectInvalidPolicy` |

## Broken or unproven invariants

1. **PARTIAL — secret containment.** Provider, Context7 and embedding keys remain
   plaintext JSON fields. Config and raw-trace files now request `0600`, and raw
   trace defaults false, but no redactor runs over opt-in full prompts/responses
   and platform ACL behavior is unproven. Evidence:
   `internal/config/persistence.go#writePrivateFileAtomically` and
   `internal/logging/llm_io_logger.go#LogLLMRequest`.
2. **PARTIAL — configuration integrity.** Same-directory synced rename, wizard
   merge, failure preservation, round-trip and Unix mode have focused tests, but
   no version conflict, backup/rollback, or Windows ACL proof completes the transaction.
3. **PARTIAL — input validity.** Unknown/trailing JSON is rejected. Invalid
   URLs/protocols, negative values, impossible reserve percentages, logging
   sampling outside `[0,1]`, and most engine/provider combinations still have no
   load gate.
4. **PARTIAL — execution containment.** Shared Cortex projects and validates all
   `ExecutionConfig` fields. Campaign start/resume copy binaries/env/directory but
   omit timeout/shared containment and soften load errors; dormant legacy boot
   still uses defaults.
5. **PARTIAL — process isolation.** missing JSON preserves process-global feature
   state; global timeouts have unsynchronized writes; logging is one-shot.

## Hostile-input and failure stance

**PROPOSED UPLIFT.** A present invalid file must fail before any provider, store,
kernel, prompt compiler, MCP client or executor is constructed. Diagnostics name
field paths and rule IDs but redact values classified as secrets. Path and URL
validation must occur before consumer conversion; resource values need both lower
and upper bounds.

**REJECTED.** Environment fallback is acceptable for an absent file but not for
a present malformed or explicitly contradictory one. The distinction preserves
first-run usability without silently defeating workspace intent.

## Trace and retention policy

**PARTIAL.** Raw logging now requires explicit `TraceLLMIO=true` in addition to
debug/category activation, its file requests `0600`, and defaults-off behavior is
tested by `internal/config/config_security_test.go#TestSensitiveTracingDefaultsOff`.
No bounded retention, response limit, content redaction, or focused secret
regression is proven.
Opt-in must remain prominent with destination/retention warnings and secret
tests.
