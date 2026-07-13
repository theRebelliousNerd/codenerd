# cli — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `cmd/nerd/` (113 non-test .go, 55 tests, 2 .mg)**


## 1. Purpose

CLI entrypoints, chat TUI, campaign and system commands

## 2. Source paths

| Path | Role |
|------|------|
| `cmd/nerd/` | Primary implementation |
| `Docs/architecture/cli/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **70%** |
| Exported types (sampled) | Implemented | **70%** |
| Tests | Implemented | **70%** |
| Mangle local sources | Implemented | **70%** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 70% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

| Path | Lines |
|------|------:|
| `cmd/nerd/chat/process.go` | 1195 | source |
| `cmd/nerd/chat/session_boot.go` | 1101 | source |
| `cmd/nerd/cmd_campaign.go` | 1101 | source |
| `cmd/nerd/chat/review_aggregator.go` | 1070 | source |
| `cmd/nerd/chat/model_update.go` | 1048 | source |
| `cmd/nerd/ui/splitpane.go` | 1009 | source |
| `cmd/nerd/chat/multistep_corpus.go` | 992 | source |
| `cmd/nerd/chat/model_session_context.go` | 945 | source |
| `cmd/nerd/chat/commands_handlers.go` | 904 | source |
| `cmd/nerd/chat/northstar_wizard.go` | 868 | source |
| `cmd/nerd/chat/model_handlers.go` | 810 | source |
| `cmd/nerd/chat/multistep_decomposer.go` | 801 | source |

### Sampled types

| Type | Location |
|------|----------|
| `CampaignJITProvider` | `cmd/nerd/campaign_jit_provider.go:14` |
| `AgentWizardState` | `cmd/nerd/chat/agent_wizard.go:22` |
| `CommandCategory` | `cmd/nerd/chat/command_categories.go:8` |
| `CommandInfo` | `cmd/nerd/chat/command_categories.go:28` |
| `ConfigWizardStep` | `cmd/nerd/chat/config_wizard.go:24` |
| `ConfigWizardState` | `cmd/nerd/chat/config_wizard.go:49` |
| `ShardProfileConfig` | `cmd/nerd/chat/config_wizard.go:99` |
| `ProjectTypeInfo` | `cmd/nerd/chat/delegation.go:648` |
| `TaskStep` | `cmd/nerd/chat/delegation_multistep.go:21` |
| `RouteKind` | `cmd/nerd/chat/delegation_routing.go:20` |
| `RouteDecision` | `cmd/nerd/chat/delegation_routing.go:55` |
| `HelpRenderer` | `cmd/nerd/chat/help_renderer.go:12` |
| `ArticulationOutput` | `cmd/nerd/chat/helpers_articulation.go:92` |
| `ConversationContext` | `cmd/nerd/chat/helpers_articulation.go:118` |
| `Config` | `cmd/nerd/chat/model_types.go:52` |
| `ViewMode` | `cmd/nerd/chat/model_types.go:58` |
| `InputMode` | `cmd/nerd/chat/model_types.go:74` |
| `BootStage` | `cmd/nerd/chat/model_types.go:137` |
| `ContinuationMode` | `cmd/nerd/chat/model_types.go:146` |
| `Subtask` | `cmd/nerd/chat/model_types.go:172` |
| `ClarificationState` | `cmd/nerd/chat/model_types.go:193` |
| `Model` | `cmd/nerd/chat/model_types.go:213` |
| `ShardResult` | `cmd/nerd/chat/model_types.go:457` |
| `KnowledgeResult` | `cmd/nerd/chat/model_types.go:470` |
| `Message` | `cmd/nerd/chat/model_types.go:480` |

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Owner |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
