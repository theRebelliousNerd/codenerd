# init — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/init/` (complete internal coverage)
> **Implementation: `internal/init/` — 16 non-test .go, 7 tests, 1 .mg**


## 1. Purpose

Workspace/project initialization and scanning

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/init/` | Primary implementation |
| `Docs/architecture/init/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **70%** |
| Exported types (sampled) | Implemented | **70%** |
| Tests | Implemented | **70%** |
| Mangle local sources | Implemented | **70%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 70%** as living package (16 src / 7 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/init/initializer.go` | 1128 | source |
| `internal/init/scanner.go` | 1034 | source |
| `internal/init/profile.go` | 956 | source |
| `internal/init/strategic_knowledge.go` | 742 | source |
| `internal/init/agents.go` | 728 | source |
| `internal/init/strategic_documents.go` | 600 | source |
| `internal/init/tools.go` | 566 | source |
| `internal/init/agents_registration.go` | 521 | source |
| `internal/init/scanner_dependencies.go` | 504 | source |
| `internal/init/interactive.go` | 446 | source |
| `internal/init/validation.go` | 376 | source |
| `internal/init/agents_knowledge.go` | 369 | source |
| `internal/init/jit_integration.go` | 261 | source |
| `internal/init/shared_kb.go` | 195 | source |
| `internal/init/typeu_agents.go` | 178 | source |
| `internal/init/eta_tracker.go` | 158 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `AgentRegistry` | `internal/init/agents.go:206` |
| `KnowledgeBaseStats` | `internal/init/agents.go:213` |
| `ToolGenerationRequest` | `internal/init/agents_registration.go:231` |
| `ETATracker` | `internal/init/eta_tracker.go:9` |
| `InitProgress` | `internal/init/initializer.go:44` |
| `AgentCreationUpdate` | `internal/init/initializer.go:57` |
| `RecommendedAgent` | `internal/init/initializer.go:68` |
| `InitConfig` | `internal/init/initializer.go:81` |
| `ProjectProfile` | `internal/init/initializer.go:108` |
| `DependencyInfo` | `internal/init/initializer.go:141` |
| `UserPreferences` | `internal/init/initializer.go:149` |
| `InitResult` | `internal/init/initializer.go:169` |
| `CreatedAgent` | `internal/init/initializer.go:190` |
| `Initializer` | `internal/init/initializer.go:206` |
| `SessionState` | `internal/init/initializer.go:1094` |
| `ChatMessage` | `internal/init/initializer.go:1116` |
| `SessionHistory` | `internal/init/initializer.go:1123` |
| `DetectedAgent` | `internal/init/interactive.go:22` |
| `AgentSelectionPreferences` | `internal/init/interactive.go:42` |
| `InteractiveConfig` | `internal/init/interactive.go:50` |
| `AgentSuggestion` | `internal/init/interactive.go:331` |
| `BuildSystemInfo` | `internal/init/scanner.go:616` |
| `SharedKnowledgeAtom` | `internal/init/shared_kb.go:27` |
| `DocumentInfo` | `internal/init/strategic_documents.go:21` |
| `DocProcessingStatus` | `internal/init/strategic_documents.go:35` |
| `DocIngestionState` | `internal/init/strategic_documents.go:49` |
| `DocProcessingEntry` | `internal/init/strategic_documents.go:62` |
| `StrategicKnowledge` | `internal/init/strategic_knowledge.go:22` |
| `ComponentInfo` | `internal/init/strategic_knowledge.go:51` |
| `PatternInfo` | `internal/init/strategic_knowledge.go:60` |
| `ToolDefinition` | `internal/init/tools.go:13` |
| `TypeUAgentDefinition` | `internal/init/typeu_agents.go:12` |
| `TypeUAgentError` | `internal/init/typeu_agents.go:19` |
| `ValidationResult` | `internal/init/validation.go:20` |
| `ValidationSummary` | `internal/init/validation.go:35` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `DefaultPhaseDurations` | `internal/init/eta_tracker.go:21` |
| `NewETATracker` | `internal/init/eta_tracker.go:49` |
| `StartPhase` | `internal/init/eta_tracker.go:59` |
| `CompletePhase` | `internal/init/eta_tracker.go:67` |
| `GetETARemaining` | `internal/init/eta_tracker.go:76` |
| `GetElapsed` | `internal/init/eta_tracker.go:93` |
| `GetCurrentPhase` | `internal/init/eta_tracker.go:100` |
| `GetTotalPhases` | `internal/init/eta_tracker.go:107` |
| `DefaultInitConfig` | `internal/init/initializer.go:95` |
| `NewInitializer` | `internal/init/initializer.go:228` |
| `Close` | `internal/init/initializer.go:277` |
| `Initialize` | `internal/init/initializer.go:326` |
| `DefaultInteractiveConfig` | `internal/init/interactive.go:58` |
| `InteractiveAgentSelection` | `internal/init/interactive.go:67` |
| `ConvertToDetectedAgents` | `internal/init/interactive.go:259` |
| `ConvertToRecommendedAgents` | `internal/init/interactive.go:309` |
| `GetContext7AgentSuggestions` | `internal/init/interactive.go:342` |
| `LoadAgentPreferences` | `internal/init/interactive.go:392` |
| `SaveAgentPreferences` | `internal/init/interactive.go:420` |
| `BuildInitCompilationContext` | `internal/init/jit_integration.go:216` |
| `LoadProjectProfile` | `internal/init/profile.go:576` |
| `LoadPreferences` | `internal/init/profile.go:592` |
| `LoadSessionState` | `internal/init/profile.go:608` |
| `SaveSessionState` | `internal/init/profile.go:624` |
| `SaveSessionHistory` | `internal/init/profile.go:634` |
| `LoadSessionHistory` | `internal/init/profile.go:661` |
| `ListSessionHistories` | `internal/init/profile.go:676` |
| `GetLatestSession` | `internal/init/profile.go:696` |
| `IsInitialized` | `internal/init/profile.go:705` |
| `CreateSharedKnowledgePool` | `internal/init/shared_kb.go:105` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
