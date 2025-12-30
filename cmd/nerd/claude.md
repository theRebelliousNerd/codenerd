# cmd/nerd - codeNERD CLI Entry Point

This directory contains the main CLI application for codeNERD, an AI-powered coding assistant with neuro-symbolic architecture.

## Architecture Overview

The CLI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI and follows the Elm architecture (Model-Update-View).

## File Index

| File | Description |
|------|-------------|
| `main.go` | Entry point that wires Cobra CLI with all subcommands and starts interactive chat mode. Exports `rootCmd` with flags for workspace, verbose, timeout, and system shard configuration. |
| `campaign_jit_provider.go` | Adapts `articulation.PromptAssembler` to `campaign.PromptProvider` interface to avoid circular deps. Enables JIT prompts for campaign roles without importing campaign into articulation. |
| `cmd_auth.go` | Manages CLI engine authentication for Claude Code and Codex CLIs. Provides `auth claude`, `auth codex`, and `auth status` subcommands that configure `.nerd/config.json`. |
| `cmd_campaign.go` | Campaign orchestration CLI with start/status/pause/resume/abort subcommands. Orchestrates long-running multi-phase goals from the command line without TUI. |
| `cmd_query.go` | Queries Mangle fact store and provides system status via `query`, `status`, and `why` commands. Implements "Glass Box" interface showing derivation traces for logical conclusions. |
| `dom_cmd.go` | Parent command for Code DOM utilities including demo, inspect, get, and edit subcommands. Provides semantic code editing capabilities through AST-aware operations. |
| `dom_apply_cmd.go` | Applies multi-element Code DOM plans from JSON files within a file's 1-hop scope. Supports dry-run mode, gofmt integration, and test execution after apply. |
| `dom_replace_cmd.go` | Search/replace across workspace or 1-hop scope with regex support. Includes safety caps on file count and dry-run mode for preview. |
| `dom_utils.go` | Helper utilities for Code DOM operations including chunked gofmt execution. Handles Windows command line length limits by batching file arguments. |
| `embedding_cmd.go` | Embedding engine operations mirroring TUI `/embedding` commands. Provides `set`, `stats`, and `reembed` subcommands for vector store configuration. |
| `stats.go` | Computes line count statistics for files or directories. Supports common source file extensions and filters test files. |
| `system_results.go` | Polls kernel for system shard results with configurable timeout. Returns routing and execution results for display in CLI output. |
| `main_test.go` | Unit tests for CLI command helpers and query functions. Tests argument joining, fact queries, and why command behavior. |

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

---

**Remember: Push to GitHub regularly!**
