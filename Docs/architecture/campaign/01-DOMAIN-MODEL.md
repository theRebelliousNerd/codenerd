# campaign — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/campaign/` (complete internal coverage)
> **Implementation: `internal/campaign/` — 44 non-test .go, 29 tests, 1 .mg**


## Package

`internal/campaign/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `AssaultScope` | `internal/campaign/assault_types.go:4` |
| `AssaultStageKind` | `internal/campaign/assault_types.go:14` |
| `AssaultStage` | `internal/campaign/assault_types.go:25` |
| `AssaultConfig` | `internal/campaign/assault_types.go:37` |
| `CampaignRole` | `internal/campaign/campaign_prompts.go:19` |
| `PromptProvider` | `internal/campaign/campaign_prompts.go:34` |
| `StaticPromptProvider` | `internal/campaign/campaign_prompts.go:41` |
| `CheckpointRunner` | `internal/campaign/checkpoint.go:17` |
| `ContextPager` | `internal/campaign/context_pager.go:18` |
| `ShardLister` | `internal/campaign/decomposer.go:18` |
| `Decomposer` | `internal/campaign/decomposer.go:24` |
| `DecomposeRequest` | `internal/campaign/decomposer.go:210` |
| `DecomposeResult` | `internal/campaign/decomposer.go:220` |
| `DocClassification` | `internal/campaign/decomposer.go:229` |
| `RawPlan` | `internal/campaign/decomposer_requirements.go:479` |
| `RawPhase` | `internal/campaign/decomposer_requirements.go:486` |
| `RawTask` | `internal/campaign/decomposer_requirements.go:501` |
| `DocumentIngestor` | `internal/campaign/document_ingestor.go:15` |
| `FileAction` | `internal/campaign/edge_case_detector.go:31` |
| `FileDecision` | `internal/campaign/edge_case_detector.go:64` |
| `SplitSuggestion` | `internal/campaign/edge_case_detector.go:91` |
| `EdgeCaseDetector` | `internal/campaign/edge_case_detector.go:98` |
| `EdgeCaseConfig` | `internal/campaign/edge_case_detector.go:107` |
| `EdgeCaseAnalysis` | `internal/campaign/edge_case_detector.go:734` |
| `ConsultationProvider` | `internal/campaign/intelligence_gatherer.go:32` |
| `BatchConsultRequest` | `internal/campaign/intelligence_gatherer.go:37` |
| `ConsultationResponse` | `internal/campaign/intelligence_gatherer.go:45` |
| `IntelligenceGatherer` | `internal/campaign/intelligence_gatherer.go:65` |
| `IntelligenceConfig` | `internal/campaign/intelligence_gatherer.go:91` |
| `IntelligenceReport` | `internal/campaign/intelligence_gatherer.go:146` |
| `FileInfo` | `internal/campaign/intelligence_gatherer.go:222` |
| `SymbolInfo` | `internal/campaign/intelligence_gatherer.go:232` |
| `ChurnHotspot` | `internal/campaign/intelligence_gatherer.go:241` |
| `CommitInfo` | `internal/campaign/intelligence_gatherer.go:250` |
| `LearningPattern` | `internal/campaign/intelligence_gatherer.go:259` |
| `PreferenceSignal` | `internal/campaign/intelligence_gatherer.go:268` |
| `EntityCluster` | `internal/campaign/intelligence_gatherer.go:275` |
| `SafetyWarning` | `internal/campaign/intelligence_gatherer.go:282` |
| `MCPToolInfo` | `internal/campaign/intelligence_gatherer.go:292` |
| `CampaignArtifact` | `internal/campaign/intelligence_gatherer.go:302` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewAdversarialAssaultCampaign` | `internal/campaign/assault_campaign.go:19` |
| `DefaultAssaultConfig` | `internal/campaign/assault_types.go:61` |
| `Normalize` | `internal/campaign/assault_types.go:78` |
| `NewStaticPromptProvider` | `internal/campaign/campaign_prompts.go:44` |
| `GetPrompt` | `internal/campaign/campaign_prompts.go:49` |
| `GetCampaignPhaseForRole` | `internal/campaign/campaign_prompts.go:86` |
| `GetShardTypeForRole` | `internal/campaign/campaign_prompts.go:109` |
| `NewCheckpointRunner` | `internal/campaign/checkpoint.go:24` |
| `Run` | `internal/campaign/checkpoint.go:45` |
| `RunAll` | `internal/campaign/checkpoint.go:441` |
| `RunQuick` | `internal/campaign/checkpoint.go:466` |
| `NewContextPager` | `internal/campaign/context_pager.go:37` |
| `SetBudget` | `internal/campaign/context_pager.go:61` |
| `GetUsage` | `internal/campaign/context_pager.go:77` |
| `ResetPhaseContext` | `internal/campaign/context_pager.go:87` |
| `ActivatePhase` | `internal/campaign/context_pager.go:101` |
| `CompressPhase` | `internal/campaign/context_pager.go:207` |
| `PrefetchNextTasks` | `internal/campaign/context_pager.go:314` |
| `PruneIrrelevant` | `internal/campaign/context_pager.go:350` |
| `NewDecomposer` | `internal/campaign/decomposer.go:46` |
| `SetPromptProvider` | `internal/campaign/decomposer.go:77` |
| `SetShardLister` | `internal/campaign/decomposer.go:89` |
| `SetIntelligenceGatherer` | `internal/campaign/decomposer.go:98` |
| `SetAdvisoryBoard` | `internal/campaign/decomposer.go:107` |
| `SetEdgeCaseDetector` | `internal/campaign/decomposer.go:116` |
| `SetToolPregenerator` | `internal/campaign/decomposer.go:125` |
| `GetLastIntelligence` | `internal/campaign/decomposer.go:133` |
| `IsGroundingAvailable` | `internal/campaign/decomposer.go:142` |
| `IsThinkingAvailable` | `internal/campaign/decomposer.go:147` |
| `EnableURLContext` | `internal/campaign/decomposer.go:153` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/campaign/debug_program_ERROR.mg` | 16308 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Multi-phase goal orchestration, decomposition, context paging**
