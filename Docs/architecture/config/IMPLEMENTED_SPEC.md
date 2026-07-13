# config — implemented specification

> Authority for realized behavior. Verified 2026-07-13 against
> `internal/config`, primary constructors, mutators, tests, and trace sinks.

## 1. Scope and authority

`internal/config` defines codeNERD's workspace configuration types, local file
load/save helpers, effective-value resolution, legacy env overlays, and two
process bridges: feature activation and LLM timeout presets. It does not own
provider implementations, Mangle policy, tool execution, or runtime teardown.

| Aggregate | File | Realized authority |
|---|---|---|
| `UserConfig` | `.nerd/config.json` | broad runtime input for provider/engine, limits, JIT, memory, integrations, world, UX and features |
| `Config` | `.nerd/config.yaml` or explicit path | legacy defaults/env aggregate; live Cobra use is execution timeout |
| logging projection | `.nerd/config.json` | independently decoded by `internal/logging/logger.go#loadConfig` before general config load |

**PARTIAL.** JSON is the broad primary aggregate, not a literal single source of
truth: YAML timeout, direct logging decode, environment provider detection,
process globals and consumer defaults remain parallel authorities.

## 2. JSON aggregate and persistence contract

`internal/config/user_config.go#UserConfig` contains:

- engine/provider/model and provider-specific API keys;
- Claude CLI, Codex CLI, xAI OAuth, Gemini, Ollama, worker and image blocks;
- context, embedding, reflection, shard profiles, limits and API scheduler;
- world, integrations, generated-tool target, build and execution settings;
- logging, JIT, learning, onboarding, transparency, guidance and feature flags.

### Load

`internal/config/user_config.go#LoadUserConfig` performs one local read and
`internal/config/persistence.go#decodeStrictJSON`, which rejects unknown fields
and a second/trailing JSON value.

| Input | Result |
|---|---|
| absent path | `&UserConfig{}`, nil; returns before feature activation |
| read error | nil, wrapped error |
| malformed JSON | nil, wrapped parse error |
| syntactically valid JSON | mutable aggregate, `features.SetActive(cfg.Features)`, boot summary |
| unknown JSON member | parse error |
| trailing JSON value | parse error |

Evidence:
`internal/config/config_comprehensive_test.go#TestLoadUserConfig_WhenMissingFile_ShouldReturnEmptyConfig`,
`#TestLoadUserConfig_WhenMalformedJSON_ShouldReturnError`, and
`internal/features/config_roundtrip_test.go#TestLoadUserConfig_InstallsFeaturesIntoRegistry`.
Unknown/trailing rejection is covered by
`internal/config/config_security_test.go#TestLoadUserConfigRejectsUnknownAndTrailingJSON`.

There is no schema version, size/depth bound, full `ValidateUserConfig`,
normalization receipt, or immutable snapshot.

### Save

`internal/config/user_config.go#UserConfig.Save` marshals every exported
non-omitted field and calls
`internal/config/persistence.go#writePrivateFileAtomically`. That helper creates
the parent with `0700`, writes/chmods/syncs/closes a same-directory temporary,
renames it, and requests `0600` on the result. Provider, Context7 and embedding
keys are ordinary plaintext fields and therefore persist when present.

**VERIFIED CURRENT.** Round-trip behavior is covered by
`internal/config/config_comprehensive_test.go#TestUserConfigSave_WhenRoundTrip_ShouldPreserve`.

**PARTIAL.** There is no semantic validation, lock/expected-version check,
backup, Windows ACL proof, active-state update or rollback snapshot. Focused
tests prove pre-rename failure preserves old bytes, Unix `0600`, reload, and
representative wizard field preservation; concurrent-writer and post-rename
outcome contracts remain absent.

## 3. Legacy YAML contract

`internal/config/config.go#DefaultConfig` creates a fully populated value tree.
`internal/config/config.go#Load` unmarshals YAML into it, then
`#Config.applyEnvOverrides` applies provider keys, MCP URLs, database path and
embedding settings in fixed order. Missing YAML is not an error.

`Config.Validate` checks provider membership and API-key presence (except
Ollama). `Config.ValidateCoreLimits` checks only lower bounds for total memory,
shards, facts and derived facts. Neither validator is called by `Load`.

`Config.Save` now shares the atomic/private writer with JSON. JSON and YAML
defaults still differ:

| Effective concept | JSON | YAML |
|---|---:|---:|
| max concurrent shards | 12 | 4 |
| execution timeout | 30s | 10m |
| context input tokens | 200000 | 128000 |

**VERIFIED CURRENT.** `cmd/nerd/main.go#rootCmd` reads `.nerd/config.yaml` only to
set the Cobra timeout when the flag was not explicit.

## 4. Effective-value contracts

### Provider and engine

`UserConfig.GetActiveProvider` behavior is:

```text
explicit provider -> that provider plus only its matching key
ollama            -> ("ollama", "ollama") sentinel
no provider       -> first key: anthropic, openai, gemini, xai,
                     zai, openrouter, legacy api_key-as-zai
no key            -> ("", "")
```

An explicit provider with no matching key returns `(provider, "")`. This is
proven by
`internal/config/config_comprehensive_test.go#TestGetActiveProvider_WhenProviderSetButKeyMissing_ShouldReturnEmptyKey`.

`UserConfig.SetEngine` accepts `api`, `claude-cli`, `codex-cli`, and `xai-oauth`.
Direct JSON decode does not call it, so invalid engines are rejected later by
the perception client factory, not at config load.

Codex defaults are model `gpt-5.4`, sandbox `read-only`, shell disabled, output
schema enabled, and concurrency 2. **VERIFIED CURRENT:** the backend ignores
configured sandbox/shell relaxation, forces read-only plus shell-disabled, and
allowlists only reasoning/verbosity/personality overrides. Hostile config is
covered by
`internal/perception/codex_cli_client_test.go#TestCodexCLIClient_buildCLIArgs_FiltersEffectOverrides`.

### Scheduler and core limits

`UserConfig.GetCoreLimits` replaces zero fields with 12288 MiB memory, 12 shards,
5 API calls, 120 minutes, 250000 facts and 100000 derived facts. Negative values
are not replaced or rejected.

`UserConfig.GetEffectiveMaxConcurrentAPICalls` starts from core API concurrency
and narrows it for positive Codex/xAI OAuth/Claude caps.
`GetEffectiveAPISchedulerPolicy` adds subscription spacing/adaptive defaults and
optional pointer overrides; its default slot timeout comes from process-global
`GetLLMTimeouts`.

Focused evidence:
`internal/config/config_test.go#TestUserConfig_GetEffectiveMaxConcurrentAPICalls`
and `#TestUserConfig_GetEffectiveAPISchedulerPolicy`.

### Context and JIT

`UserConfig.GetContextWindowConfig` applies field-by-field defaults, including
200000 input tokens and reserve percentages 5/30/15/50. It does not validate
percentage range/sum. `ContextWindowConfig.TotalContextWindow` adds output,
thinking and tool buffers; `EffectiveInputBudget` returns MaxTokens.

`UserConfig.GetEffectiveJITConfig` applies defaults, caps token budget to the
context input ceiling, and repairs reserved tokens when reserve is not smaller
than budget. `DefaultJITConfig` sets enabled/fallback true, budget 200000,
reserved 8000, semantic top-k 20, and `TraceLLMIO=false`.

The effective JIT projection reaches prompt compilation/assembly and system
shards through `internal/system/factory.go#initIntelligenceLayer` and
`#initShardManagement`.

### Execution

`UserConfig.GetExecution` defaults a binary list, timeout `30s`, working
directory `.`, and env names.

**VERIFIED CURRENT for shared Cortex.**
`internal/system/factory_execution.go#executionLayerConfigs` projects binaries,
env, working directory and timeout into tactile/VirtualStore construction,
contains directory resolution to the workspace, and rejects invalid/non-positive
durations. `internal/system/factory_execution_test.go` covers positive and
hostile cases.

**PARTIAL cross-surface.** Campaigns copy binaries/env/directory without the
shared timeout/containment helper. Dormant legacy interactive construction uses
defaults. No parity test covers every effect path.

### Integrations and other domains

- `IntegrationsConfig.ToMCPServerConfigs` emits enabled map entries with protocol,
  BaseURL and timeout strings; validation is deferred.
- `GetEmbeddingConfig` defaults local Ollama and canonicalizes bare
  `embeddinggemma` to `embeddinggemma:300m`.
- `DefaultWorldConfig` scales scan worker counts to CPU within fixed ranges.
- reflection and JIT use custom JSON boolean-presence tracking.
- many getters shallow-copy structs while maps/slices remain aliased; UX getters
  may return stored pointers directly.

## 5. Runtime wiring

### Shared system factory

| Stage | Config projection |
|---|---|
| `internal/system/factory.go#initCoreComponents` | one JSON load, effective JIT, scheduler; present-invalid returns error |
| `#initPerceptionLayer` | same snapshot provider/engine, worker/image; ambient fallback only with no explicit LLM selection |
| `#initExecutionLayer` | validated/contained binaries, env, directory and timeout via `executionLayerConfigs` |
| `#initIntelligenceLayer` | embedding/reflection/MCP/world plus JIT compiler and assembler budgets |
| `#initShardManagement` | core limits and JIT config |

The first stage propagates JSON load errors and stores one `appCfg`. Perception
resolves it through `ProviderConfigFromUserConfig`; `HasExplicitLLMSelection`
prevents an unusable explicit choice from routing to another ambient provider.
`internal/system/factory_execution_test.go#TestInitCoreComponentsRejectsPresentInvalidConfig`
proves a present-invalid file is terminal before perception even with an ambient
XAI key.

### Interactive compatibility and campaigns

`cmd/nerd/chat/session_boot.go#performSystemBootLegacy` has no Go caller; if
reactivated it projects core limits/JIT but uses default execution and an
incomplete scheduler projection. `cmd/nerd/cmd_campaign.go#runCampaignStart` and
`#runCampaignResume` soften load errors, select their LLM from ambient state
before config load, and copy binaries/env/directory plus limits/JIT/scheduler.
They omit configured execution timeout/shared containment and are not parity-checked.

### Mutation and reload

Auth/slash/embedding surfaces commonly load-mutate-save. The config wizard now
loads and merges its owned fields before Save. Save does not call `features.SetActive` or reload
logging/scheduler/JIT/clients. Logging initialization is `sync.Once`. There is no
cross-consumer reload transaction or config-owned teardown.

## 6. Nine-lane applicability matrix

| Lane | Current answer and boundary |
|---|---|
| Mangle | **N-A direct.** No `.mg`, `Decl`, rule, negation, recursion or fact producer exists in config. `MangleConfig` stores paths/budgets. `permitted/3` is declared by `internal/core/defaults/schemas_safety.mg#permitted/3` and derived by `internal/core/defaults/policy/constitution.mg#permitted/3`. |
| Permission and safety | **PARTIAL.** Config supplies resource/capability bounds but cannot authorize. Shared execution containment and Codex CLI isolation are tested; campaign/dormant parity and secret persistence/trace boundaries remain incomplete. Final route is `internal/core/virtual_store_routing.go#VirtualStore.RouteAction`. |
| Fact flow | **VERIFIED CURRENT.** Config precedes perception and kernel construction; it does not create `user_intent` or `next_action`. Limits enter `internal/core/limits.go#NewLimitsEnforcer`; articulation consumes timeout globals elsewhere. |
| JIT and agents | **PARTIAL.** Config owns no atom IDs or agent registry. It selects JIT budget/reserve/top-k/debug/trace consumed by prompt assembler and system shards; reserve clamp exists, but full input validity and safe tracing do not. |
| Wiring | **PARTIAL.** Provider, limits, scheduler, JIT, embedding, MCP, world and features have live constructors. Shared execution/error propagation is tested; campaign/dormant execution, secondary error handling and reload parity still diverge. |
| State and concurrency | **PARTIAL.** mutable `UserConfig`, aliased collections, process-global features/timeouts/logging/scheduler, missing-file preservation, and unsynchronized timeout Set/Get lack one workspace snapshot lifetime. |
| Recovery | **PARTIAL.** missing file degrades; shared boot rejects present-invalid while secondary paths may soften; pre-rename failure preserves old bytes, but no backup/version rollback exists; reload is restart-oriented and optional integrations degrade without one config receipt. |
| Observability | **PARTIAL.** boot/provider/feature logs exist. Raw trace now defaults false and requests `0600`, but lacks content redaction/retention, bounded response and projection provenance. |
| Testing | **PARTIAL.** unit/package-race plus strict decode, pre-rename failure, Unix mode, wizard merge, shared execution/fail-closed boot, hostile Codex and trace-default regressions pass. Semantic validation, cross-surface parity, fuzz, hostile secret, writer conflict, backup and Windows ACL gates are absent. |

## 7. Verified defect register

The evidence packet is
`artifact:.corpus-build/findings/config-audit-defects.md`. Headline defects:

1. Wizard preservation is tested; version conflict and mutator-wide conformance remain.
2. Shared execution projection is tested; campaign/dormant parity remains.
3. Atomic/private persistence has failure/Unix-mode evidence; conflict, backup, Windows ACL and plaintext-secret boundaries remain.
4. Opt-in raw LLM trace has no content redaction/bounded retention.
5. Strict syntax/shared fail-closed boot are tested; semantic invalidity and secondary error softening remain.
6. Process-global feature/timeout/logging/scheduler state lacks one workspace snapshot/reload lifecycle.

These are residual **PARTIAL** or **VERIFIED CURRENT** code paths, not claims
that an exploit or data loss was observed in a live user workspace. CFG-004's
Codex subprocess isolation repair is closed by hostile regression; the remaining
items stay open to their stated closure gates.

## 8. Verification and freshness

The focused package and race gates passed at the inspected commit. Exact command,
commit and dirty-tree fingerprints are in
`artifact:Docs/architecture/config/_progress.md`. Test green establishes current
covered behavior; it closes the specifically exercised repair slices, not the
remaining semantic, lifecycle, cross-surface, secret or platform risk gates.

Use [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) for the export catalog,
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) for constructors,
[09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) for trust boundaries,
and [TODO.md](TODO.md) for the sole uplift authority.
