# 06 — Public API and Types: campaign

> Last verified: **2026-07-13**  
> Package: `codenerd/internal/campaign`  
> Focus: exported surface that callers (CLI, chat, tests) actually use.

## Construction entry points

| Symbol | File | Notes |
|--------|------|-------|
| `NewOrchestrator(OrchestratorConfig) (*Orchestrator, error)` | `orchestrator_init.go` | Validates required deps |
| `NewDecomposer(kernel, llm, workspace) *Decomposer` | `decomposer.go` | Also constructed inside orchestrator |
| `NewContextPager(kernel, llm, budget) *ContextPager` | `context_pager.go` | |
| `NewCheckpointRunner(executor, te, workspace) *CheckpointRunner` | `checkpoint.go` | |
| `NewReplanner(kernel, llm) *Replanner` | `replan.go` | |
| `NewAdversarialAssaultCampaign(workspace, AssaultConfig) *Campaign` | `assault_campaign.go` | Deterministic plan |
| `DefaultAssaultConfig() AssaultConfig` | `assault_types.go` | Defaults |
| `NewIntelligenceGatherer(...)` | `intelligence_gatherer.go` | Multi-arg DI |
| `NewShardAdvisoryBoard(ConsultationProvider)` | `shard_advisory_board.go` | |
| `NewEdgeCaseDetector(kernel, scanner)` | `edge_case_detector.go` | |
| `NewToolPregenerator(...)` | `tool_pregenerator.go` | |
| `NewDocumentIngestor(dbPath, embedCfg)` | `document_ingestor.go` | |
| `NewStaticPromptProvider()` | `campaign_prompts.go` | Default prompts |
| `NewLocalSpecialistKnowledgeProvider(workspace)` | `specialist_knowledge.go` | |

## Orchestrator methods (primary)

| Method | File | Purpose |
|--------|------|---------|
| `Run(ctx) error` | `orchestrator_execution.go` | Main loop |
| `LoadCampaign(id) error` | `orchestrator_lifecycle.go` | Disk → memory + kernel |
| `SetCampaign(*Campaign) error` | lifecycle | Memory + facts + save |
| `Pause` / `Resume` / `Stop` | `orchestrator_control.go` | Control plane |
| `GetProgress() Progress` | control | UI DTO |
| `SetPromptProvider` | init | JIT |
| `SetTaskExecutor` | init | JIT execution loop |
| `SetSpecialistKnowledgeProvider` | init | Domain knowledge |
| `SetNorthstarObserver` | init | Vision gate |
| `SetIntelligenceGatherer` / `SetAdvisoryBoard` / `SetEdgeCaseDetector` / `SetToolPregenerator` | init | Intelligence suite |

## Decomposer

| Symbol | Purpose |
|--------|---------|
| `Decompose(ctx, DecomposeRequest) (*DecomposeResult, error)` | Full plan pipeline |
| `SetPromptProvider` / `SetShardLister` / intel setters | Wiring |
| `GetLastIntelligence() *IntelligenceReport` | Last Step-0 report |
| Grounding/thinking helpers | Gemini advanced features |

### Request / result

```text
DecomposeRequest {
  Goal, SourcePaths []string, CampaignType, UserHints []string,
  MaxPhases, ContextBudget int
}

DecomposeResult {
  Campaign *Campaign
  ValidationOK bool
  Issues []PlanValidationIssue
  SourceDocs []SourceDocument
  Requirements []Requirement
}
```

## Domain types (exported)

### Enums / atoms

| Type | Representative values |
|------|----------------------|
| `CampaignType` | `/greenfield`, `/feature`, `/audit`, `/migration`, `/remediation`, `/adversarial_assault`, `/custom` |
| `CampaignStatus` | `/planning` … `/failed` |
| `PhaseStatus` / `TaskStatus` | pending/in_progress/completed/failed/skipped (+ task blocked) |
| `TaskType` | file/test/research/shard/tool/verify/document/refactor/integrate/campaign_ref/assault_* |
| `TaskPriority` | critical/high/normal/low |
| `ObjectiveType` / `VerificationMethod` / `DependencyType` | create/test/… ; tests_pass/builds/… ; hard/soft/artifact |
| `CampaignRefFailurePolicy` | propagate/absorb/transform |
| `AssaultScope` / `AssaultStageKind` | repo/module/subsystem/package ; go_test/vet/nemesis/command |
| `RiskGateMode` / `RiskGateToggle` / `RiskGateName` / `RiskGateOutcome` | auto/force_*; auto/enabled/disabled; northstar/edge/advisory |
| `CampaignRole` | prompt role atoms (see `campaign_prompts.go`) |
| `ReplanReason` | task_failed, new_requirement, user_feedback, dependency_change, blocked |
| `FileAction` | edge detector create/extend/modularize family |

### Structs operators care about

| Type | Role |
|------|------|
| `Campaign` | Durable plan root |
| `Phase`, `Task` | Graph nodes |
| `PhaseObjective`, `PhaseDependency`, `Checkpoint` | Verification graph |
| `TaskArtifact`, `TaskAttempt` | Provenance |
| `ContextProfile` | Per-phase schema/tool/pattern needs |
| `Learning` | Autopoiesis notes on campaign |
| `Progress` | UI progress |
| `OrchestratorEvent` | Event stream |
| `OrchestratorConfig` | Full DI surface |
| `AssaultConfig`, `AssaultStage` | Assault knobs |
| `CampaignRiskDecision`, `RiskGateEvaluation`, `RiskInputSnapshot` | Risk outputs |
| `IntelligenceReport` (+ nested FileInfo, SafetyWarning, …) | Pre-plan intel |
| `AdvisoryRequest` / `AdvisoryResponse` / `AdvisorySynthesis` | Board I/O |
| `PregenerationResult`, `ToolGap`, `GeneratedTool` | Tool pregen |
| `CampaignRefResult` | Nested campaign envelope |
| `PlanValidationIssue`, `ReplanTrigger`, `ReplanResult` | Plan quality |

### Interfaces

| Interface | Purpose |
|-----------|---------|
| `PromptProvider` | `GetPrompt(ctx, role, campaignID)` |
| `ShardLister` | `ListAvailableShards()` for planning |
| `ConsultationProvider` | Advisory LLM/shard consult |
| `SpecialistKnowledgeProvider` | Knowledge atoms for tasks |

## Fact emission API

| Method | Emits |
|--------|-------|
| `(*Campaign).ToFacts() []core.Fact` | Full plan |
| `(*Phase).ToFacts()` | Phase subgraph |
| `(*Task).ToFacts()` | Task subgraph |
| `(*ContextProfile).ToFacts()` | Profile facts (if present) |

Callers that mutate campaigns outside orchestrator should re-`LoadFacts` consistently.

## Errors

From `errors.go` + lock manager:

- `ErrDecompositionFailed`, `ErrTaskTimeout`, `ErrCampaignTimeout`
- `ErrCheckpointFailed`, `ErrReplanExhausted`
- `ErrNilDependency`, `ErrInvalidConfig`, `ErrNilCampaign`, `ErrNilKernel`
- `ErrEmptyRequirement`, `ErrEmptyGoal`
- `ErrWriteSetLockTimeout`

## Helpers commonly re-exported by behavior

| Symbol | Notes |
|--------|-------|
| `AssaultConfig.Normalize()` | Clamp/defaults |
| `GetShardTypeForRole` / `GetCampaignPhaseForRole` | JIT provider mapping (used by CLI adapter) |

## What is intentionally unexported

- `writeSetLockManager` / lease internals  
- `campaignJournalEvent`  
- `riskGateResolved`  
- Many assault file DTOs and private discover helpers  

Tests may reach package-private symbols within package tests only.

## CLI adapter type (outside package)

`cmd/nerd/campaign_jit_provider.go` defines `CampaignJITProvider` implementing `campaign.PromptProvider` without creating an import cycle into articulation from campaign.
