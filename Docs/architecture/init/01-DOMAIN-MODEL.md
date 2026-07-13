# init — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/init/` (complete internal coverage)
> **Implementation: `internal/init/` — 16 non-test .go, 7 tests, 1 .mg**


## Package

`internal/init/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/init/debug_program_ERROR.mg` | 16308 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Workspace/project initialization and scanning**
