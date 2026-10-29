# init — `internal/init/`: the `nerd init` cold-start pipeline

`internal/init/` turns a repository into a codeNERD workspace. One entry
point, `Initializer.Initialize` (`internal/init/initializer.go:445`), runs
the phases below in order and records the outcome in an `InitResult`:
required failures make `Success` false, degraded enrichment is a warning.

## Entry points

| Front end | Path |
|---|---|
| `nerd init`, `nerd init --force` | `initCmd.RunE` → `runInit` (`cmd/nerd/cmd_init_scan.go:132`) → `runInitWithLLMConfigurer` → `configureInitLLM` (`:336`: worker else main client, planner slot from `.nerd/config.json`) → `NewInitializer` (`:209`) → `Initialize` (`:215`) |
| chat `/init`, `/init --force` | `commands.go:106` → `handleCmdInit` → `runInitialization` (`cmd/nerd/chat/helpers_scan.go`): session `m.client`, `Cortex.PlannerLLM()`, session embedding engine and shard manager → `NewInitializer` → `Initialize` → `Close` |
| chat `/refresh-docs [--force]` | `commands.go:111` → `runDocRefresh`: `OrientationDocuments` → `ProcessDocumentsWithTracking` → `SynthesizeFromStoredAtoms` → `PersistStrategicKnowledge` → `Close` |

Both front ends pass `Timeout: 0`: each model request reads
`llm_timeouts.per_call_timeout` when it is made. `--timeout` bounds only the
whole CLI command. Neither names a model or provider.

## LLM routing

Every model request crosses `withJITPrompt` (`internal/init/jit_integration.go:439`),
which resolves the slot's client (`route`, `internal/init/llm_routing.go:51`),
compiles the system prompt for that client (provider/model from the client
chain, budget capped by its input window), bounds the request and records the
outcome per serving client (`InitLLMMetrics.Clients`).

| Slot | Client | Phases |
|---|---|---|
| planner | `InitConfig.PlannerClient`, else `LLMClient` | document roles, north-star draft (phase 3), domain-expert proposal (phase 6), specialist prompts (7a), project tool selection (7e), strategic-knowledge synthesis (`/refresh-docs`) |
| main | `InitConfig.LLMClient` | grounded domain research (7a), per-chunk document extraction (`/refresh-docs`) |
| none | Context7 | stack-topic research (7a) |

## Phases and what reads their output

| # | Function (`internal/init/phases.go`) | Writes | Production reader |
|---|---|---|---|
| 0 | `runPhase0Migrations` :58 | shard DB schema upgrades | every KB open |
| 1 | `runPhase1DirectorySetup` :82 | `.nerd/` tree, `.gitignore`, `config.json` (absent only, provider-neutral), Mangle templates, regression seed, `knowledge.db`, northstar store | kernel boot (`internal/core/kernel_init.go:424`), config load, `nerd regression` |
| 2 | `runPhase2Scanning` :135 | scan file cache; counts for the profile | incremental scanner |
| 3 | `runPhase3Orientation` :161 → `runOrientation` (`phase_orient.go:130`) | `orientation/snapshot.json`, `orientation.mg`, `README.md`; north star | `kernel_init.go:486`, `orient.Refresh`, northstar guardian |
| 4 | `runPhase4Profile` :173 + `persistOrientationKnowledge` | `profile.json`; `knowledge.db` `strategic/*` | `IsInitialized`, chat context (`model_session_context.go`), `query_strategic` |
| 5 | `runPhase5Facts` :196 | `profile.mg` | chat boot (`session_boot_helpers.go:96`) |
| 5b/5c | `runPhase5bPromptAtoms` :212, `runPhase5cPromptDB` :222 | `prompts/corpus.db` | JIT compiler (`internal/system/factory.go:1808`) |
| 6 | `runPhase6AnalyzeAgents` :235 → `integrateEcosystem` (`phase_ecosystem.go:131`) | roster; `orientation/agents.md`, `domain_experts.json` | phase 7a; next init |
| 7a | `runPhase7aCreateAgentKBs` :257 → `createType3Agents` (`agents.go:426`) | `shards/<agent>_knowledge.db`, `agents/<agent>/prompts.yaml` | JIT `RegisterAgentDBWithJIT` (factory), expert knowledge retrieval |
| 7e | `runPhase7eProjectTools` :278 → `selectProjectTools` (`project_tools.go:82`) | `tools/project_tools.json` | phase 10; next init |
| 8 | `runPhase8Preferences` :306 | `preferences.json` (merged) | `internal/ux`, chat boot |
| 9 | `runPhase9Session` :329 | `session.json` (absent only) | session persistence |
| 10 | `runPhase10Tools` :347 | `tools/available_tools.json` | `WorkspaceStaticToolDefs` (`internal/system/factory.go:1665`) |
| 11 | `runPhase11Registry` :400 | `agents.json` (merged, grants limited to registered tools) | `LoadUserAgentDefinitions` (`factory.go:2221`), chat boot |
| 12 | `runPhase12PromptSync` :506 | prompt atoms into agent KBs | JIT compiler |
| — | `finalizeInitialization` :528 | validation, warnings, summary | — |

Progress events carry the running phase's number, the phase count and the
elapsed time; the chat shows them in the status bar. Human output goes only to
`InitConfig.Output` (stdout for the CLI, nothing for the chat).

Mechanisms behind the table — the roster, incremental re-init, knowledge
storage — are in INTERNALS.md; the Mangle side in 09-MANGLE-SURFACE.md.

---
Verified 2026-10-27 against the working tree (planner routing and init review pass).
