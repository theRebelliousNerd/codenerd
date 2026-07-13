# campaign — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/campaign/` (44 non-test .go, 29 tests, 1 .mg)**


## Source package

`internal/campaign/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 1 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| `internal/campaign/debug_program_ERROR.mg` | 16308 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Multi-phase goal orchestration and context paging**

## Data & control concepts

- Primary language surface: Go under `internal/campaign/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
