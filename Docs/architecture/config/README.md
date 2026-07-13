# config — the workspace control plane

> Realized corpus for `internal/config`; verified 2026-07-13.

## In one minute

codeNERD users choose an LLM engine, resource ceilings, JIT budget, integrations,
and workspace behavior in `.nerd/config.json`. The useful outcome is simple: a
workspace starts with the backend and limits the operator intended, and a bad
configuration fails before an LLM call or tool effect.

**VERIFIED CURRENT.** `internal/config/user_config.go#LoadUserConfig` decodes the
JSON aggregate and its `Get*` resolvers supply defaults; provider, scheduler,
embedding, world, JIT, and feature consumers are live. The focused receipt is
`internal/config/config_comprehensive_test.go#TestUserConfigSave_WhenRoundTrip_ShouldPreserve`.

**PARTIAL.** The package now rejects unknown/trailing JSON and uses an atomic,
owner-only writer, but it has no full `UserConfig` semantic validator or immutable
snapshot. Shared Cortex fails closed, while secondary/campaign callers still
soften some errors; plaintext keys remain in the file, and writer version,
backup, Windows ACL and secret-redaction contracts remain open.

## Its place in codeNERD

Configuration declares capabilities and budgets; it is not the executive.
The LLM remains the creative center. Mangle and the Go executive derive and
enforce `permitted(Action, Target, Payload)` after config has selected providers,
limits, and adapters.

**VERIFIED CURRENT.** `internal/config/limits.go#CoreLimits` is projected into
`internal/core/limits.go#NewLimitsEnforcer`, and
`internal/config/user_config.go#GetEffectiveAPISchedulerPolicy` feeds the global
scheduler through `internal/system/factory.go#initCoreComponents`.

**PARTIAL.** Shared Cortex validates and projects every
`internal/config/execution.go#ExecutionConfig` field. Campaigns copy binaries,
environment and directory without shared timeout/containment, while dormant
legacy boot uses defaults. Config therefore cannot be treated as permission
proof or yet as a uniformly wired execution bound.

## A representative journey

```text
.nerd/config.json
  -> LoadUserConfig + features.SetActive
  -> system factory resolves provider, scheduler, limits, JIT and integrations
  -> perception turns input into user_intent
  -> kernel derives next_action and permitted/3
  -> VirtualStore performs or denies the effect
  -> articulation reports the result
```

**VERIFIED CURRENT.** A valid explicit provider is honored only with its matching
key by `internal/config/user_config.go#UserConfig.GetActiveProvider`, proven by
`internal/config/config_comprehensive_test.go#TestGetActiveProvider_WhenProviderSetButKeyMissing_ShouldReturnEmptyKey`.

**VERIFIED CURRENT failure journey.** Unknown/trailing or malformed JSON returns
an error from `LoadUserConfig`, and
`internal/system/factory.go#initCoreComponents` now stops shared Cortex boot
before perception; `internal/system/factory_execution_test.go#TestInitCoreComponentsRejectsPresentInvalidConfig`
proves an ambient key does not rescue the bad file. **PARTIAL:** some secondary
consumers still soften load errors, and semantic invalidity lacks one validator.

## What exists today

| Area | Claim | Evidence |
|---|---|---|
| Workspace | **VERIFIED CURRENT** — nearest `go.mod` is authoritative; deepest `.nerd` is fallback. | `internal/config/user_config.go#FindWorkspaceRoot`; `internal/config/config_test.go#TestFindWorkspaceRoot_BypassesStrayNestedNerd` |
| Provider | **VERIFIED CURRENT** — explicit provider does not borrow another provider's key. | `internal/config/user_config.go#UserConfig.GetActiveProvider` |
| Limits/JIT | **VERIFIED CURRENT** — effective scheduler and JIT budgets are resolved and wired. | `internal/system/factory.go#initCoreComponents`; `internal/system/factory.go#initIntelligenceLayer` |
| Persistence | **PARTIAL** — JSON/YAML use a synced same-directory temporary and `0600`, with failure/mode tests; writer conflict, backup, Windows ACL and secret-reference contracts remain. | `internal/config/persistence.go#writePrivateFileAtomically`; `internal/config/config_security_test.go#TestPrivateAtomicWritePreservesOriginalOnReplaceFailure` |
| Wizard | **PARTIAL** — it loads and preserves execution/logging/integration fields in a focused test; concurrent-version handling remains. | `cmd/nerd/chat/config_wizard_steps.go#Model.saveConfigWizard`; `cmd/nerd/chat/config_wizard_save_test.go#TestSaveConfigWizardPreservesUnownedSettings` |
| Execution | **PARTIAL** — shared Cortex projects/contains all fields; campaign copies binaries/env/directory without shared timeout/containment and dormant legacy boot bypasses them. | `internal/system/factory_execution.go#executionLayerConfigs`; `internal/system/factory_execution_test.go#TestExecutionLayerConfigsProjectUserPolicy` |
| Codex isolation | **VERIFIED CURRENT** — backend forces read-only, disables shell and allowlists overrides despite hostile config. | `internal/perception/codex_cli_client.go#CodexCLIClient.buildCLIArgs`; `internal/perception/codex_cli_client_test.go#TestCodexCLIClient_buildCLIArgs_FiltersEffectOverrides` |
| Mangle | **N-A direct** — config declares no predicates or rules; its values are constructor inputs to the executive. | `internal/config/mangle.go#MangleConfig`; `internal/core/defaults/schemas_safety.mg#permitted/3` |

The full realized contract and all nine applicability lanes live in
[IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md). Verified defects are also recorded at
`artifact:.corpus-build/findings/config-audit-defects.md`.

## North star

**PROPOSED UPLIFT.** One versioned, strictly decoded, validated and immutable
workspace snapshot should be the only boot input. It should carry field
provenance, expose redacted diagnostics, save atomically with owner-only
permissions, and be projected into every consumer before any network call or
effect adapter starts.

Non-goals: config will not derive `user_intent`, grant `permitted/3`, contain
prompt prose, hot-reload partial state automatically, or make secrets observable.

## Improvement frontier

The safest first repair is to finish `config-safe-persistence-v1`: merge,
pre-rename failure and Unix mode are tested, while conflict, backup,
cross-platform permission and secret contracts remain. Next is to finish
`config-strict-snapshot-v1`: unknown/trailing JSON and shared fail-closed boot are
tested, while semantic validation and secondary parity remain. The bounded future option is a
side-effect-free migration laboratory over redacted fixtures; it never boots a
client or executes a tool. All authoritative cards are in [TODO.md](TODO.md).

## Choose a reading route

- **90 seconds:** this page, then [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
- **10 minutes:** add [02-CURRENT-STATE.md](02-CURRENT-STATE.md),
  [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md), and
  [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md).
- **Deep implementation:** [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md),
  [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md),
  [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md), and [TODO.md](TODO.md).
