# cmd/nerd - codeNERD CLI Entry Point

This directory contains the main CLI application for codeNERD, an AI-powered coding assistant with neuro-symbolic architecture.

## Architecture Overview

The CLI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI and follows the Elm architecture (Model-Update-View).

## File Structure

| File | Purpose | Lines |
|------|---------|-------|
| `main.go` | Entry point, CLI flags, command routing | ~1300 |
| `chat.go` | Core TUI model, Init, Update loop | ~600 |
| `chat_commands.go` | All `/command` handlers | ~1500 |
| `chat_process.go` | Input processing, intent handling | ~300 |
| `chat_view.go` | Rendering functions (View, header, footer) | ~300 |
| `chat_session.go` | Session persistence, state hydration | ~400 |
| `chat_campaign.go` | Campaign orchestration UI | ~350 |
| `chat_delegation.go` | Shard spawning and delegation | ~200 |

## Key Types

### chatModel
The central TUI model containing:
- **UI Components**: textinput, viewport, spinner, styles
- **State**: history, loading state, dimensions
- **Backend**: LLM client, kernel, shard manager, transducer
- **Campaign**: Active campaign, orchestrator, progress
- **Autopoiesis**: Self-modification orchestrator

### Message Types
- `responseMsg` - LLM response
- `errorMsg` - Error handling
- `campaignStartedMsg` - Campaign lifecycle
- `shardResultMsg` - Shard delegation results

## Command Flow

1. User input → `handleSubmit()`
2. Check for `/command` → `handleCommand()`
3. Otherwise → `processInput()`
4. Process intent via Transducer (Piggyback Protocol)
5. Check for shard delegation
6. Run autopoiesis analysis
7. Execute kernel actions
8. Articulate response
9. Compress context for infinite memory

## Key Commands

| Command | Handler Location | Description |
|---------|------------------|-------------|
| `/help` | chat_commands.go | Show help |
| `/init` | chat_commands.go | Initialize workspace |
| `/spawn` | chat_commands.go | Spawn ephemeral shard |
| `/campaign` | chat_campaign.go | Campaign orchestration |
| `/review` | chat_delegation.go | Code review (convenience) |
| `/shadow` | chat_commands.go | Shadow mode simulation |

## Dependencies

- `internal/perception` - Intent parsing (Transducer)
- `internal/core` - Kernel, ShardManager
- `internal/campaign` - Campaign orchestration
- `internal/autopoiesis` - Self-modification
- `internal/articulation` - Response generation
- `internal/store` - Persistence

## Testing

Run tests with:
```bash
go test ./cmd/nerd/...
```

## Adding New Commands

1. Add case in `handleCommand()` in `chat_commands.go`
2. If complex, create helper function
3. Update `/help` output
4. Add to VerbCorpus in `internal/perception/transducer.go` for natural language support
