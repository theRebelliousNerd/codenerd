# campaign — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/campaign/` (complete internal coverage)
> **Implementation: `internal/campaign/` — 44 non-test .go, 29 tests, 1 .mg**


## 1. Purpose

Multi-phase goal orchestration, decomposition, context paging

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/campaign/` | Primary implementation |
| `Docs/architecture/campaign/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (44 src / 29 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/campaign/assault_tasks.go` | 1157 | source |
| `internal/campaign/prompts.go` | 1072 | source |
| `internal/campaign/decomposer.go` | 1059 | source |
| `internal/campaign/edge_case_detector.go` | 1057 | source |
| `internal/campaign/risk_scoring.go` | 1051 | source |
| `internal/campaign/replan.go` | 997 | source |
| `internal/campaign/types.go` | 916 | source |
| `internal/campaign/orchestrator_task_handlers.go` | 718 | source |
| `internal/campaign/shard_advisory_board.go` | 671 | source |
| `internal/campaign/tool_pregenerator.go` | 656 | source |
| `internal/campaign/intelligence_gatherer.go` | 652 | source |
| `internal/campaign/decomposer_requirements.go` | 591 | source |
| `internal/campaign/intelligence_gathering_methods.go` | 586 | source |
| `internal/campaign/decomposer_planning.go` | 575 | source |
| `internal/campaign/orchestrator_tasks.go` | 542 | source |
| `internal/campaign/context_pager.go` | 506 | source |
| `internal/campaign/checkpoint.go` | 477 | source |
| `internal/campaign/orchestrator_failure.go` | 434 | source |
| `internal/campaign/orchestrator_init.go` | 381 | source |
| `internal/campaign/decomposer_documents.go` | 363 | source |

### Types (sampled)

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

### Functions (sampled)

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
