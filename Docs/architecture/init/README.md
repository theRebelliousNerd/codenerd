# init — `internal/init/`: the `nerd init` cold-start pipeline

The Go package `internal/init/` turns an ordinary repository into a codeNERD
workspace: it scans the project, writes the `.nerd/` directory (profile,
facts, knowledge bases, tool catalog, prompt data), and registers the
specialist agents it detected. Everything runs through one entry point,
`Initializer.Initialize` (`internal/init/initializer.go:451`), which executes
16 ordered phases and records the outcome in an `InitResult`
(`internal/init/initializer.go:215`).

## What `nerd init` creates under `.nerd/`

| Artifact | Writer |
|---|---|
| `agents.json` agent registry | `saveAgentRegistry` (`internal/init/agents_registration.go:73`; read back by `loadExistingAgentRegistry`, `internal/init/agents.go:637`) |
| `shards/{name}_knowledge.db` per-agent SQLite KBs, keyed on the lowercased agent name | `createType3Agents` (`internal/init/agents.go:677`) |
| Shared knowledge pool (path from `GetSharedKnowledgePath`, `internal/init/shared_kb.go:186`) | `CreateSharedKnowledgePool` (`internal/init/shared_kb.go:105`) |
| Codebase and campaign KBs | `createCodebaseKnowledgeBase` / `createCampaignKnowledgeBase` (`internal/init/profile.go:807`, `internal/init/profile.go:885`) |
| Core-shard (coder/reviewer/tester) KBs | `createCoreShardKnowledgeBases` (`internal/init/agents_registration.go:89`) |
| A `prompts.yaml` template per persistent agent | `generateAgentPromptsYAML` (`internal/init/agents.go:70`) |
| `mangle/extensions.mg`, `mangle/policy_overrides.mg` seed templates | `createMangleTemplates` (`internal/init/initializer.go:1245`) |
| `tools/available_tools.json` static tool catalog | `SaveToolsToFile` (`internal/init/tools.go:432`), called from phase 10 |
| `preferences.json` agent-selection record | `persistAgentSelection` (`internal/init/agents_curation.go:115`) |

Seed files are created without clobbering user content: `writeFileIfAbsent`
(`internal/init/initializer.go:1287`) uses `O_EXCL` so concurrent init runs
cannot race on check-then-write.

## Pipeline: `Initialize` (`internal/init/initializer.go:451`)

| # | Phase function | Line | Does |
|---|---|---|---|
| 0 | `runPhase0Migrations` | 713 | Schema/state migrations |
| 1 | `runPhase1DirectorySetup` | 737 | Creates the `.nerd/` layout |
| 2 | `runPhase2Scanning` | 790 | Project scan (`internal/init/scanner.go`, lockfile parsers in `internal/init/scanner_dependencies.go`) |
| 3 | `runPhase3Analysis` | 813 | Analysis step |
| 4 | `runPhase4Profile` | 820 | Builds the `ProjectProfile` (`internal/init/initializer.go:130`) |
| 5 | `runPhase5Facts` | 842 | Writes project facts; also appends tool-need facts (`internal/init/profile.go:131`) |
| 5b | `runPhase5bPromptAtoms` | 858 | Prompt atoms |
| 5c | `runPhase5cPromptDB` | 868 | Prompt database (`initializePromptDatabase`, `internal/init/profile.go:1007`) |
| 6 | `runPhase6AnalyzeAgents` | 881 | Recommend → merge Type-U → curate interactively → record (`internal/init/initializer.go:885`) |
| 7a | `runPhase7aCreateAgentKBs` | 903 | Builds per-agent KBs |
| 7b | `runPhase7bCreateCodebaseKB` | 936 | Shared pool, codebase KB, strategic knowledge |
| 7c | `runPhase7cCreateCoreShardKBs` | 968 | Core-shard KBs |
| 7d | `runPhase7dCreateCampaignKB` | 983 | Campaign KB |
| 7e | `runPhase7eGenerateTools` | 997 | Records tool needs; generates nothing (see WIRING-AND-NOT-BUILT.md) |
| 8 | `runPhase8Preferences` | 1016 | User preferences |
| 9 | `runPhase9Session` | 1035 | Session state |
| 10 | `runPhase10Tools` | 1047 | Static tool catalog → `tools/available_tools.json` |
| 11 | `runPhase11Registry` | 1075 | Agent registry |
| 12 | `runPhase12PromptSync` | 1087 | Prompt sync |
| — | `finalizeInitialization` | 1109 | Validates agent DBs, funnels the outcome into failures/warnings via `recordValidationOutcome` (`internal/init/initializer.go:1156`) |

Progress across phases is reported with ETA estimates by `ETATracker`
(`internal/init/eta_tracker.go:9`, durations in `DefaultPhaseDurations`,
`internal/init/eta_tracker.go:21`).

## Public API used outside this package

- `NewInitializer` (`internal/init/initializer.go:299`) with
  `DefaultInitConfig` (`internal/init/initializer.go:116`); `Close`
  (`internal/init/initializer.go:373`) releases the embedding engine, which is
  created lazily by `ensureEmbeddingEngine` (`internal/init/initializer.go:392`).
- `LoadProjectProfile` / `IsInitialized` (`internal/init/profile.go:735`,
  `internal/init/profile.go:796`) for reading back init state.
- Tool-catalog readers: `GetLanguageTools`, `GetFrameworkTools`,
  `GetDependencyTools`, `GenerateToolsForProject`, `LoadToolsFromFile`
  (`internal/init/tools.go:48`, `:291`, `:346`, `:387`, `:415`).
- `ValidateAgentDB` (`internal/init/validation.go:107`) checks a single agent DB.
- `InteractiveAgentSelection` (`internal/init/interactive.go:83`) is the prompt
  loop behind interactive curation.

## Production callers

- `cmd/nerd/cmd_init_scan.go:214` and `cmd/nerd/chat/helpers_scan.go:65,244`
  construct the `Initializer` for the CLI and chat `/init` paths.
- `internal/system/factory.go:1441` loads the persisted catalog via
  `LoadToolsFromFile`.

How the pipeline decides, what it stores, and — most importantly — what it
claims to do but does not, are in INTERNALS.md and WIRING-AND-NOT-BUILT.md.

---
Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f`
(17 non-test `.go` files, 20 `_test.go` files in `internal/init/`).
