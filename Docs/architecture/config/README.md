# config — `internal/config`

`internal/config` is the single home of user-facing configuration: the
`UserConfig` root struct, its per-subsystem sections, and every
default → resolve → validate step between `.nerd/config.json` on disk and a
value another package can use. Load path: `LoadUserConfig`
(`internal/config/user_config.go:525-593`), save path: `Save`
(`internal/config/user_config.go:647-658`). 21 non-test `.go` files, 14
`*_test.go` files, and one maintenance note (`internal/config/agents.md`).

## Layout — file by file

| File | Holds |
|---|---|
| `user_config.go` | `UserConfig` (`user_config.go:27-285`); load/save; provider and engine selection (`GetActiveProvider` :940-1008, `GetEngine` :1011-1016, `SetEngine` :1033-1045, `HasExplicitLLMSelection` :1021-1030); key read/write (`APIKeyForProvider` :860-888, `SetAPIKeyForProvider` :894-928); secondary-slot resolution (`resolveSecondarySlot` :837-852); scheduler ceilings (`GetEffectiveMaxConcurrentAPICalls` :1137-1161, `GetEffectiveAPISchedulerPolicy` :1176-1226); the browser section (`BrowserAutomationConfig` :1314-1337, `DefaultBrowserAutomationConfig` :1340-1354, `GetBrowserConfig` :1359-1401); JIT accessors (`GetJITConfig` :1572-1597, `GetEffectiveJITConfig` :1600-1620); `DefaultUserConfig` (:1487-1525) |
| `llm.go` | `LLMConfig` (:16-22); `ClaudeCLIConfig` (:31-44) with `DefaultClaudeCLIConfig` (:229-234); `CodexCLIConfig` (:62-97) with `DefaultCodexCLIConfig` (:247-259), `DefaultCodexExecSkillName` (:56), `DefaultCodexMaxConcurrentCalls` (:58-59); `XAIOAuthConfig` (:134-159) with `DefaultXAIOAuthMaxConcurrentCalls` (:128) — there is no `DefaultXAIOAuthConfig`, defaults are inline in `GetXAIOAuthConfig` (`user_config.go:1108-1133`); `GeminiProviderConfig` (:190-203) with `DefaultGeminiProviderConfig` (:216-224); `CLIModelLabel` (:238-243); `GetClaudeCLICommand` (:273-275) |
| `memory.go` | `EmbeddingConfig` (:20-45); `ContextWindowConfig` (:55-87); `DefaultContextWindowConfig` (:115-130); `DefaultEmbeddingConfig` (:133-142); `DefaultOllamaEndpoint` (:17) |
| `reflection.go` | `ReflectionConfig` custom `UnmarshalJSON` (:28-49); `DefaultReflectionConfig` (:52-60); resolved onto `UserConfig` by `GetReflectionConfig` (`user_config.go:384-406`) |
| `shard.go` | `ShardProfile` (:5-39); sampling semantics (:15-19); the deleted per-shard ceilings (:23-31); `applyShardDefaults` (:42-50); `DefaultShardProfile` (:53-60); `DefaultShardProfiles` (:63-90) |
| `world.go` | `WorldConfig` (:6-15); `DefaultWorldConfig` (:18-41); worker-count and byte-budget constructors (`FastWorkerCount` :44-47, `DeepWorkerCount` :51-54, `MaxFastASTBytes` :58-61) |
| `ux.go` | `UIConfig` (:4-11); `DefaultUIConfig` (:13-19); `ViewMode` constants (:20-23) |
| `jit.go` | `JITConfig` (:6-54); `DefaultJITConfig` (:63-74) |
| `limits.go` | `CoreLimits` (:9-27); `ValidateCoreLimits` (:85-99); `DefaultCoreLimits` (:102-110); `APISchedulerPolicy` (:45-63); `EffectiveAPISchedulerPolicy` (:67-74) |
| `llm_timeouts.go` | `LLMTimeouts` (:19-58); `DefaultLLMTimeouts` (:62-75); `FastLLMTimeouts` (:80-93); `AggressiveLLMTimeouts` (:97-110); process-wide singleton `GetLLMTimeouts` (:116-118) / `SetLLMTimeouts` (:122-124) |
| `llm_timeouts_config.go` | `LLMTimeoutsConfig` (:24-61); `Resolve` (:98-135) with `baseProfile` (:64-76) and `applyDuration` (:81-94) |
| `integrations.go` | `IntegrationsConfig` (:7-11); `MCPServerIntegration` (:13-17); `DefaultTimeout` (:35-44); `ToMCPServerConfigs` (:47-79); `GetServer` (:89-97); `IsServerEnabled` (:100-103); `DefaultIntegrationsConfig` (:106-112) |
| `execution.go` | `ExecutionConfig` (:4-16); `DefaultExecutionConfig` (:19-33) |
| `build.go` | `BuildConfig` (:10-21); `DefaultBuildConfig` (:24-30); nil-receiver-tolerant `GetBuildConfig` (`user_config.go:427-444`) |
| `logging.go` | `LoggingConfig` (:6-17); `ToLoggingConfig` (:22-35); `IsCategoryEnabled` (:40-52); `DefaultLoggingConfig` (:55-73) |
| `tool_generation.go` | `ToolGenerationConfig` (:6-28); `AllowToolExec` (:27); `DefaultToolGenerationConfig` (:50-55) |
| `mangle.go` | `MangleConfig` (:4-10); `DefaultDerivedFactLimit` (:13). No registration function lives in this package |
| `persistence.go` | `decodeStrictJSON` (:17-33); `writePrivateFileAtomically` (:36-38); `WithReplace` (:40-49) |
| `removed_keys.go` | The four removed-key tables (:23-61); `rejectRemovedKeys` (:69-98); `issueForKey` (:101-112); `formatKnownKeys` (:116-126) |
| `agents.md` | 31-line maintenance note for this directory. Its tool-budget bullet (:14-17) is stale — see WIRING-AND-NOT-BUILT.md §4 |

## Start here

- How a file becomes values: INTERNALS.md §1 (`LoadUserConfig`,
  `internal/config/user_config.go:525-593`).
- How "written 0" differs from "omitted": INTERNALS.md §2
  (`coreLimitsKeysPresent` :600-612, `validateExplicitCoreLimits` :621-644).
- What is wired, what is dormant, and what old keys are refused:
  WIRING-AND-NOT-BUILT.md.
- Provider precedence and engine switching: INTERNALS.md §6
  (`GetActiveProvider` :940-1008, `SetEngine` :1033-1045).

## What this package does not do

- No per-shard context, token, or fact ceilings exist (`shard.go:23-31`
  records their deletion).
- No wall-clock run bounds exist; the keys that used to set them are refused
  (`removed_keys.go:28-61`).
- No `DefaultXAIOAuthConfig` exists (`llm.go`, cf. `DefaultClaudeCLIConfig`
  :229-234 and `DefaultCodexCLIConfig` :247-259).

---
*Verified 2026-09-20 against `b63e848`. INTERNALS.md answers how it works;
WIRING-AND-NOT-BUILT.md answers what is wired and what is not.*
