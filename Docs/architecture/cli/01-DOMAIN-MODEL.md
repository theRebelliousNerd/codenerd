# cli — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `cmd/nerd/` (113 non-test .go, 55 tests, 2 .mg)**


## Source package

`cmd/nerd/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 2 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| `cmd/nerd/debug_program_ERROR.mg` | 16308 |
| `cmd/nerd/chat/debug_program_ERROR.mg` | 16073 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **CLI entrypoints, chat TUI, campaign and system commands**

## Data & control concepts

- Primary language surface: Go under `cmd/nerd/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
