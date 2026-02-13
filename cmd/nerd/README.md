# cmd/nerd/

The CLI entrypoint for codeNERD. Built with Cobra for commands and Bubble Tea for the interactive TUI.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Portable Usage

The compiled `nerd.exe` binary is fully portable:

1. Drop it in any project root directory
2. Run `nerd init` to create the `.nerd/` workspace
3. Run `nerd` to launch the interactive TUI

Each project maintains its own `.nerd/` directory with project-specific knowledge and preferences.

## Structure

```text
cmd/nerd/
├── main.go              # Cobra root command and subcommands
├── ui/                  # UI components (campaign page, diff view, etc.)
└── chat/                # Interactive TUI (Bubble Tea)
    ├── model.go         # Main TUI model and state
    ├── session.go       # Chat session management (quiescent boot)
    ├── commands.go      # Slash command handlers
    ├── agent_wizard.go  # Agent creation wizard
    ├── glass_box.go     # Tool execution visibility
    └── northstar_*.go   # North Star goal tracking
```

## Commands

### Root Command

```bash
nerd                    # Launch interactive TUI
nerd --verbose          # Enable debug logging
nerd --timeout 5m       # Set operation timeout
```

### Subcommands

| Command | Description |
|---------|-------------|
| `nerd init` | Initialize workspace, create `.nerd/` |
| `nerd run "<instruction>"` | Single-shot OODA loop execution |
| `nerd scan` | Refresh codebase index |
| `nerd query <predicate>` | Query derived facts from kernel |
| `nerd why [predicate]` | Explain derivation chain |
| `nerd status` | Show system status |
| `nerd spawn <type> <task>` | Invoke SubAgent (legacy: maps to intents) |
| `nerd define-agent` | Define a new specialist agent |
| `nerd browser <action>` | Browser automation |
| `nerd campaign <action>` | Multi-phase goal orchestration |
| `nerd check-mangle <files>` | Validate Mangle syntax |
| `nerd embedding <action>` | Manage embeddings (set/stats/reembed) |
| `nerd test-context` | Run context system validation |

## Interactive TUI

The chat interface is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), providing:

- **Markdown rendering** via Glamour
- **Syntax highlighting** for code blocks
- **Spinner animations** during operations
- **Slash commands** for kernel interaction

### Chat Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/query <pred>` | Query Mangle kernel |
| `/facts` | Show current fact count |
| `/shadow` | Enter shadow mode |
| `/whatif <action>` | Project effects |
| `/campaign <start\|assault\|status\|pause\|resume\|list> [...]` | Manage multi-phase campaigns |
| `/sessions` | List and select previous sessions |
| `/load-session <id>` | Load a specific session |
| `/new-session` | Start a fresh session |
| `/jit` | Inspect last JIT-compiled prompt |
| `/transparency [on\|off]` | Toggle operation visibility |
| `/approve` | Approve pending changes |
| `/agents` | Show active SubAgents |
| `/config` | Configuration menu |
| `/clear` | Clear chat history |
| `/quit` | Exit TUI |

#### Adversarial Assault Campaigns

In chat mode you can launch a long-horizon soak/stress + Nemesis sweep:
- Slash command: `/campaign assault subsystem internal/core --race --vet --batch 25 --cycles 3`
- Natural language: `run an assault campaign on internal/core`

Artifacts persist under `.nerd/campaigns/<campaign>/assault/` (targets, batches, logs, results, triage).

### Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Shift+Enter` | New line |
| `Ctrl+C` | Cancel/Quit |
| `Ctrl+L` | Clear screen |
| `↑/↓` | Navigate history |
| `Tab` | Autocomplete |

## Architecture

```text
┌────────────────────────────────────────────────────────────┐
│                    Cobra Root Command                      │
│                         (main.go)                          │
└────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
    ┌─────────┐         ┌─────────┐         ┌─────────┐
    │  init   │         │   run   │         │  chat   │
    │ command │         │ command │         │  (TUI)  │
    └─────────┘         └─────────┘         └─────────┘
                              │                    │
                              ▼                    ▼
                    ┌──────────────────────────────────────┐
                    │         internal/session/            │
                    │    Executor, Spawner, SubAgents      │
                    └──────────────────────────────────────┘
                                      │
                              ┌───────┴───────┐
                              ▼               ▼
                    ┌─────────────┐   ┌─────────────┐
                    │ core/       │   │ prompt/     │
                    │ Kernel      │   │ JIT Compiler│
                    │ VirtualStore│   │ ConfigFactory│
                    └─────────────┘   └─────────────┘
```

## Adding New Commands

### 1. Add Cobra Subcommand

```go
var myCmd = &cobra.Command{
    Use:   "mycommand [args]",
    Short: "Description of my command",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
        return nil
    },
}

func init() {
    rootCmd.AddCommand(myCmd)
}
```

### 2. Add Chat Slash Command

In `chat/commands.go`:

```go
func (m *Model) handleSlashCommand(input string) tea.Cmd {
    switch {
    case strings.HasPrefix(input, "/mycommand"):
        return m.handleMyCommand(input)
    // ...
    }
}

func (m *Model) handleMyCommand(input string) tea.Cmd {
    // Implementation
    return nil
}
```

## Flags

### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose, -v` | bool | false | Enable debug logging |
| `--api-key` | string | env | Override API key |
| `--workspace` | string | cwd | Override workspace path |
| `--timeout` | duration | 2m | Operation timeout |

### Init Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | false | Force re-initialization |

### Spawn Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--shadow` | bool | false | Run in shadow mode |
| `--no-apply` | bool | false | Don't apply changes |

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- [Zap](https://github.com/uber-go/zap) - Structured logging

---

**Last Updated:** December 2024
**Architecture Version:** 2.0.0 (JIT-Driven)


> *[Archived & Reviewed by The Librarian on 2026-01-25]*