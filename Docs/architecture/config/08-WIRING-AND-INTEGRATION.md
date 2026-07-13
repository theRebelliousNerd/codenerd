# 08 — Wiring and integration

> Constructor, boot, dispatch, mutation and bypass audit.

## Process boot order

```text
cmd/nerd main
  1 logging.Initialize(cwd) -> logging package reads .nerd/config.json once
  2 config.GlobalConfig() -> LoadUserConfig -> features.SetActive (if file parsed)
  3 command PersistentPreRunE -> config.Load(.nerd/config.yaml) for Cobra timeout
  4 chat/shared system boot -> LoadUserConfig again -> construct runtime layers
```

**VERIFIED CURRENT.** `internal/logging/logger.go#Initialize` is `sync.Once`
guarded and reads only its logging projection before `cmd/nerd/main.go#main`
calls `GlobalConfig`. Save/reload does not reinitialize logging.

**VERIFIED CURRENT.** The YAML early path in `cmd/nerd/main.go#rootCmd` controls
the Cobra execution timeout only; JSON remains the broad runtime aggregate.

## Shared Cortex projection

| Stage | Projection | Evidence |
|---|---|---|
| core components | one JSON load, effective JIT and API scheduler; present-invalid returns error | `internal/system/factory.go#initCoreComponents` |
| perception | same snapshot via `ProviderConfigFromUserConfig`; ambient fallback only when no explicit LLM selection | `internal/system/factory.go#initPerceptionLayer` |
| execution | contained working directory, parsed timeout, env and binary allowlists | `internal/system/factory_execution.go#executionLayerConfigs`; `internal/system/factory.go#initExecutionLayer` |
| intelligence | embedding, reflection, MCP integrations | `internal/system/factory.go#initIntelligenceLayer` |
| prompt | JIT budget/debug compilation and assembler inside intelligence setup | `internal/system/factory.go#initIntelligenceLayer` |
| shards | core limits, scheduler-backed clients, JIT config | `internal/system/factory.go#initShardManagement` |

**VERIFIED CURRENT.** `initCoreComponents` returns a `LoadUserConfig` error and
stores one `appCfg`; perception resolves from that pointer. An ambient XAI key
does not rescue a present-invalid file, and an explicit unusable LLM selection
does not route to a different ambient provider. Evidence:
`internal/system/factory_execution_test.go#TestInitCoreComponentsRejectsPresentInvalidConfig`
and `internal/config/config_security_test.go#TestHasExplicitLLMSelection`.

**VERIFIED CURRENT.** Shared Cortex projects every `ExecutionConfig` field into
the tactile/VirtualStore pair, rejects non-positive/bad durations, and rejects a
working directory outside the workspace. Evidence:
`internal/system/factory_execution_test.go#TestExecutionLayerConfigsProjectUserPolicy`
and `#TestExecutionLayerConfigsRejectInvalidPolicy`.

## Interactive compatibility boot

**VERIFIED CURRENT (dormant).**
`cmd/nerd/chat/session_boot.go#performSystemBootLegacy` has no Go caller. It
loads config, projects core limits and JIT, and constructs scheduler, kernel,
shards, prompt assembler, and VirtualStore if reactivated.

**PARTIAL.** It soft-ignores `GlobalConfig` errors, uses only
`DefaultAPISchedulerConfig` plus concurrency/slot timeout rather than the full
effective scheduler policy, and constructs default VirtualStore config. Its
behavior is not fully equivalent to shared Cortex boot.

## Other consumers

| Surface | Realized behavior |
|---|---|
| campaigns | **PARTIAL** — start/resume soften config errors, choose LLM from ambient state before config load, and copy binaries/env/directory without shared containment or configured timeout in `cmd/nerd/cmd_campaign.go#runCampaignStart` and `#runCampaignResume`. |
| MCP | **VERIFIED CURRENT** — enabled map entries convert and connect during `internal/system/factory.go#initIntelligenceLayer`; URL/protocol/timeout validation is absent in config. |
| embedding/init | **PARTIAL** — `internal/init/initializer.go#Initializer.ensureEmbeddingEngine` soft-falls back to global/default config after load error. |
| features | **PARTIAL** — parsed file installs process-global state; missing file returns before `SetActive`, intentionally preserving a prior workspace's registry state. |
| timeouts | **VERIFIED CURRENT** — consumers read process-global copies through `GetLLMTimeouts`; there is no teardown or per-workspace identity. |

## Mutation lifecycle

```text
wizard/auth/embedding command
  -> load UserConfig
  -> mutate
  -> UserConfig.Save (marshal, synced private temporary, rename)
  -> disk changes
  -> active Features/logging/scheduler/JIT remain unchanged until explicit reload/restart
```

**PARTIAL.** Auth, slash handlers and the wizard load then mutate. Save is
atomic/private at the file primitive; tests prove wizard preservation,
pre-rename old-byte preservation, round-trip and Unix `0600`. There is still no
semantic validation, writer-version conflict check, backup, active snapshot
update, or cross-platform permission proof.

## Lifecycle and teardown

Config owns no closer. Consumer resources close through Cortex/session teardown.
The process-global feature registry, logging `sync.Once`, LLM timeouts, and global
scheduler outlive an individual config load; no workspace-scoped teardown resets
them. Mid-process multi-workspace reuse must therefore be treated as a state
boundary, not a normal reload.
