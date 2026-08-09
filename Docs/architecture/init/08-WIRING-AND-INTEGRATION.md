# init — Wiring and Integration

> Last verified: 2026-08-09

## CLI wiring (`cmd/nerd/cmd_init_scan.go`)

### `nerd init`

```
runInit
  timeout context + SIGINT cancel
  workspace from --workspace / CWD
  if --cleanup-backups → CleanupBackups; return
  if IsInitialized && !force → message; return
  DefaultInitConfig(cwd)
  load workspace UserConfig
  worker client → configured main client fallback → core.NewScheduledLLMCall
  pass provider/model labels for machine-readable enrichment metrics
  Context7 from CONTEXT7_API_KEY or config.json
  NewInitializer → Initialize-owned timeout → Success/failure check → Close
```

Flags relevant (defined on root/init command tree elsewhere in `cmd/nerd`): `force`, `cleanup-backups`, `api-key`, `timeout`, `workspace`.

### `nerd scan`

```
runScan
  require IsInitialized
  world.NewScanner().ScanWorkspace
  PersistFastSnapshotToDB(knowledge.db)
  kernel.LoadFacts + optional LoadFactsFromFile(profile.mg)
```

Note: **scan does not re-run** agent KB creation or `buildProjectProfile` detectors from init.

## Chat wiring

| Site | Behavior |
|------|----------|
| `session_persistence.go` | Save/load `SessionState` + `ChatMessage` history under `.nerd/` |
| `commands_tools.go` | Report `IsInitialized` |
| `commands_handlers_analysis.go` | Guard re-init style operations with force flag |
| `session_boot_helpers.go` | Boot-time helpers referencing init types/registry patterns |
| `helpers_scan.go` | Scan-related workspace helpers |

Chat boot is **not** `Initializer.Initialize`. Boot assumes artifacts already exist or creates minimal state as needed (session save can mkdir `.nerd` without full init).

## Shard / kernel integration during init

```
NewInitializer
  resolve workspace → core.NewRealKernelWithWorkspace
  coreshards.NewShardManager (or injected)
  optional SetLLMClient + GroundingHelper

Initialize:
  shardMgr.StartSystemShards
  kernel.LoadFacts(scan facts)
  registerAgentsWithShardManager → DefineProfile for each CreatedAgent
  northstar.NewStore (schema only, then Close)
```

Agents registered as **persistent researcher-based profiles** with read_file + code_graph permissions by default (see `agents_registration.go`).

## Prompt / JIT integration

| Step | Wire |
|------|------|
| Init-time compile | `assembleJITPrompt` / `withJITPrompt` → embedded corpus atoms with `InitPhases` |
| Project atoms | `populateProjectAtoms` → store PromptAtom rows |
| Corpus DB | `initializePromptDatabase` under `.nerd/prompts/` |
| YAML agents | `generateAgentPromptsYAML` |
| Sync | `prompt.ReloadAllPrompts(ctx, nerdDir, embedEngine)` |

## Research tools integration

```
tools.NewRegistry()
research.RegisterAll(registry)
registry.Execute(..., "context7_fetch", {topic})
parseResearchResult → knowledge atoms
```

Used for Type-3 agent KBs and core shard KBs when LLM/research not skipped.

## Fact-flow placement

Init is **pre-OODA**:

```
[init] workspace substrate
          │
          ▼
[chat boot] Cortex assembly loads profile / world / agents
          │
          ▼
user_intent → kernel → next_action → VirtualStore → articulation
```

Without init, many commands print “run nerd init first”; chat may still run with degraded world model.

## Wiring gaps (honest)

1. `InitConfig.Interactive` default true, but `runInit` does not call `InteractiveAgentSelection`.
2. Type U parse APIs not wired into `runInit` flag set (no merge into recommended agents in CLI path).
3. `generateProjectTools` intentionally not calling VirtualStore/Ouroboros yet.
4. ProgressChan unused by default CLI path (no channel attached in `runInit`).
