# init — Public API and Types

> Last verified: 2026-07-13  
> Package import path: `codenerd/internal/init`

## Construction / lifecycle

| Symbol | File | Role |
|--------|------|------|
| `DefaultInitConfig(workspace)` | `initializer.go` | Defaults: Interactive true, Timeout 30m |
| `NewInitializer(InitConfig)` | `initializer.go` | Kernel, scanner, shard mgr, optional grounding, Context7 auto-detect |
| `(*Initializer).Close()` | `initializer.go` | Close localDB |
| `(*Initializer).Initialize(ctx)` | `initializer.go` | Full cold-start pipeline |

## Core config / result types

| Type | File | Fields of note |
|------|------|----------------|
| `InitConfig` | `initializer.go` | Workspace, LLMClient, ShardManager, Interactive, Timeout, SkipResearch, SkipAgentCreate, PreferenceHints, ProgressChan, Context7APIKey |
| `InitProgress` | `initializer.go` | Phase, Message, Percent, AgentUpdate, ETA fields |
| `AgentCreationUpdate` | `initializer.go` | Per-agent status/quality |
| `InitResult` | `initializer.go` | Success, Profile, Preferences, FilesCreated, FactsGenerated, agents, grounding |
| `ProjectProfile` | `initializer.go` | Identity + language + deps + paths + stats |
| `DependencyInfo` | `initializer.go` | Name, Version, MajorVersion, Type |
| `UserPreferences` | `initializer.go` | Style + safety + communication prefs |
| `RecommendedAgent` | `initializer.go` | Name, Type, Topics, Permissions, Tools… |
| `CreatedAgent` | `initializer.go` | KnowledgePath, KBSize, Status, Quality* |
| `SessionState` | `initializer.go` | SessionID, turns, suspension, goals |
| `ChatMessage` / `SessionHistory` | `initializer.go` | Conversation persistence |

## Profile / session package functions

| Symbol | File | Role |
|--------|------|------|
| `LoadProjectProfile(workspace)` | `profile.go` | Read `.nerd/profile.json` |
| `LoadPreferences(workspace)` | `profile.go` | Read preferences |
| `LoadSessionState` / `SaveSessionState` | `profile.go` | Session pointer file |
| `SaveSessionHistory` / `LoadSessionHistory` | `profile.go` | Per-session JSON |
| `ListSessionHistories` / `GetLatestSession` | `profile.go` | Session discovery |
| `IsInitialized(workspace)` | `profile.go` | True if `profile.json` exists |

## Agents / interactive / Type U

| Symbol | File | Role |
|--------|------|------|
| `AgentRegistry` | `agents.go` | Versioned agents list |
| `KnowledgeBaseStats` | `agents.go` | Upgrade counters + quality |
| `DetectedAgent` | `interactive.go` | Selection UI model |
| `AgentSelectionPreferences` | `interactive.go` | Accepted/rejected prefs |
| `InteractiveConfig` / `DefaultInteractiveConfig` | `interactive.go` | Stdio defaults |
| `InteractiveAgentSelection` | `interactive.go` | y/n/c selection loop |
| `ConvertToDetectedAgents` / `ConvertToRecommendedAgents` | `interactive.go` | Bidirectional conversion |
| `GetContext7AgentSuggestions` | `interactive.go` | Dep → suggestion list |
| `LoadAgentPreferences` / `SaveAgentPreferences` | `interactive.go` | Persist selection prefs |
| `TypeUAgentDefinition` | `typeu_agents.go` | User-defined agent |
| `ParseTypeUAgentFlag` / `ParseTypeUAgentFlags` | `typeu_agents.go` | `Name:role:topics` |
| `ValidateTypeUAgentDefinition` | `typeu_agents.go` | Alphanumeric name, role ≤100, 1–10 topics |
| `TypeUAgentError` | `typeu_agents.go` | Structured validation error |

## Tools catalog

| Symbol | File | Role |
|--------|------|------|
| `ToolDefinition` | `tools.go` | Static or MCP tool template |
| `(*ToolDefinition).IsMCPTool` | `tools.go` | Type == "mcp" |
| `GetLanguageTools` / framework / dep helpers | `tools.go` | Catalog builders |
| `GenerateToolsForProject` | `tools.go` | Deduped union |
| `GetToolsForAgentType` | `tools.go` | Affinity for agents |
| `SaveToolsToFile` / `LoadToolsFromFile` | `tools.go` | `.nerd/tools/available_tools.json` |

## Shared knowledge

| Symbol | File | Role |
|--------|------|------|
| `SharedKnowledgeTopics` | `shared_kb.go` | Topic list (JIT deferred research) |
| `BaseSharedAtoms` | `shared_kb.go` | Hardcoded foundation atoms |
| `CreateSharedKnowledgePool` | `shared_kb.go` | Writes `shards/core_concepts.db` |
| `GetSharedKnowledgePath` / `SharedKnowledgePoolExists` | `shared_kb.go` | Path helpers |
| `InheritSharedKnowledge` | `shared_kb.go` | Copy into agent DB |

## Validation / backups

| Symbol | File | Role |
|--------|------|------|
| `ValidationResult` / `ValidationSummary` | `validation.go` | Per-DB + overall |
| `RequiredTables` | `validation.go` | knowledge_atoms, cold_storage, vectors, knowledge_graph |
| `MinAtomCount` / `CurrentSchemaVersion` | `validation.go` | 5 / 4 |
| `ValidateAgentDB` / `ValidateAllAgentDBs` | `validation.go` | Post-init checks |
| `FindBackupFiles` / `CleanupBackups` | `validation.go` | Migration hygiene |

## ETA

| Symbol | File | Role |
|--------|------|------|
| `ETATracker` | `eta_tracker.go` | Phase duration learning |
| `DefaultPhaseDurations` | `eta_tracker.go` | 22-phase baselines |
| `NewETATracker` | `eta_tracker.go` | Constructor |

## Strategic knowledge (exported types)

| Symbol | File | Role |
|--------|------|------|
| `StrategicKnowledge` | `strategic_knowledge.go` | Vision/architecture JSON model |
| `ComponentInfo` / `PatternInfo` | `strategic_knowledge.go` | Nested model pieces |
| `DocumentInfo` | `strategic_documents.go` | Doc metadata |
| `DocProcessingStatus` constants | `strategic_documents.go` | Campaign statuses |
| `DocIngestionState` / `DocProcessingEntry` | `strategic_documents.go` | Resume state |
| `(*Initializer).PersistStrategicKnowledge` | `strategic_knowledge.go` | Public persist helper |
| `(*Initializer).GatherProjectDocumentation` | `strategic_knowledge.go` | Doc gather |
| `(*Initializer).ProcessDocumentsWithTracking` | `strategic_documents.go` | Doc pipeline |
| `(*Initializer).SynthesizeFromStoredAtoms` | `strategic_documents.go` | Synthesis |

## Unexported but central

Most of `Initialize` internals (`determineRequiredAgents`, detectors, `createType3Agents`, JIT helpers) are **unexported methods** on `Initializer`. External packages should use `NewInitializer` + `Initialize` or the session/profile helpers above rather than re-implementing detection.
