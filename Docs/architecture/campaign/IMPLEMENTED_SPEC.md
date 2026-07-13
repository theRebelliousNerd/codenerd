# campaign — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/campaign/` (44 non-test .go, 29 tests, 1 .mg)**


## 1. Purpose

Multi-phase goal orchestration and context paging

## 2. Source paths

| Path | Role |
|------|------|
| `internal/campaign/` | Primary implementation |
| `Docs/architecture/campaign/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | Implemented | **85%** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 85% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
