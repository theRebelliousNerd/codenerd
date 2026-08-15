# init — Wiring and Integration

> Last verified: 2026-08-15

## CLI wiring (`cmd/nerd/cmd_init_scan.go`)

### `nerd init`

```
runInit
  timeout context + SIGINT cancel
  workspace from --workspace / CWD
  if --cleanup-backups → CleanupBackups; return
  if IsInitialized && !force → message; return
  DefaultInitConfig(cwd)
  ParseTypeUAgentFlags(--define-agent) → config.TypeUAgents (invalid ⇒ hard error)
  --no-interactive → config.Interactive = false
  load workspace UserConfig
  worker client → configured main client fallback → core.NewScheduledLLMCall
  pass provider/model labels for machine-readable enrichment metrics
  Context7 from CONTEXT7_API_KEY or config.json
  NewInitializer → Initialize-owned timeout → Success/failure check → Close
```

Flags owned by `cmd_init_scan.go`'s own `init()`: `define-agent` (repeatable), `no-interactive`.
Flags defined on the root/init command tree elsewhere in `cmd/nerd`: `force`, `cleanup-backups`, `api-key`, `timeout`, `workspace`.

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
| Project atoms | `buildProjectAtoms` → `knowledge.db` rows (phase 5b) |
| Corpus DB | `initializePromptDatabase` seeds/reconciles `.nerd/prompts/corpus.db`, then `ingestProjectAtomsIntoCorpus` adds the project atoms with a NULL `source_file` so reconciliation preserves them (phase 5c) |
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

## Agent curation wiring (phase 6)

```
determineRequiredAgents(profile)
  → mergeTypeUAgents        (config.TypeUAgents; name collision replaces the built-in)
  → curateAgents            (config.Interactive AND a real terminal, or an injected InteractiveIO)
       LoadAgentPreferences → auto_accept_recommended short-circuit
       ConvertToDetectedAgents → InteractiveAgentSelection → ConvertToRecommendedAgents
       SaveAgentPreferences(accepted, rejected)
  → result.RecommendedAgents → phase 7a knowledge bases
```

The terminal probe requires **both** stdin and stdout to be character devices.
stdin alone is not a gate: `go test`, cron and most CI runners attach
`/dev/null`, which is a character device, while redirecting stdout to a pipe.

## Tool needs wiring

`determineRequiredTools` runs in `generateFactsFile`, emitting
`missing_tool_for(/project_init, /capability)` — the same already-Declared
predicate autopoiesis and campaign assert on a capability gap. Init never calls
`ToolGenerator`; generation stays behind `ExecuteOuroborosLoop`.

## Wiring gaps (honest)

1. `nerd scan` reloads `profile.mg` rather than re-running init's detectors, so
   a changed dependency set does not refresh `project_framework` without a full
   `nerd init --force`.
2. Session persistence types still live in `init` rather than `internal/session`.
