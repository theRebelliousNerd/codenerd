// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains help text content for command documentation.
package chat

// =============================================================================
// HELP TEXT CONSTANTS
// =============================================================================

// helpCommandText contains the full help message for /help command.
const helpCommandText = `## Available Commands

| Command | Description |
|---------|-------------|
| /help | Show this help message |
| /clear | Clear chat history |
| /new-session | Start a fresh session (preserves old) |
| /sessions | List saved sessions |
| /continue | Resume from paused multi-step task |
| /status | Show system status (includes tools) |
| /init | Initialize codeNERD in the workspace |
| /init --force | Reinitialize (preserves learned preferences) |
| /scan | Refresh codebase index without full reinit |
| /config wizard | Full interactive configuration dialogue |
| /config show | Show current configuration |
| /config set-key <key> | Set API key |
| /config set-theme <theme> | Set theme (light/dark) |
| /read <path> | Read file contents |
| /mkdir <path> | Create a directory |
| /write <path> <content> | Write content to file |
| /search <pattern> | Search for pattern in files |
| /patch | Enter patch ingestion mode |
| /edit <path> | Edit a file |

| /append <path> | Append to a file |
| /pick | Open file picker to read a file |
| /define-agent | Define a new specialist agent |
| /agents | List defined agents |
| /spawn <type> <task> | Spawn a shard agent |
| /ingest <agent> <path> | Ingest docs into an agent KB |
| /legislate <constraint> | Synthesize & ratify a safety rule |
| /review [path] [--andEnhance] | Code review (--andEnhance for creative suggestions) |
| /security [path] | Security analysis |
| /analyze [path] | Complexity analysis |
| /clarify <goal> | Socratic requirements interrogation |
| /test [target] | Generate/run tests |
| /fix <issue> | Fix an issue |
| /refactor <target> | Refactor code |
| /northstar | Define project vision & specification |
| /vision | Alias for /northstar |
| /spec | Alias for /northstar |
| /query <predicate> | Query Mangle facts |
| /why <fact> | Explain why a fact was derived |
| /logic | Show logic pane content |
| /shadow | Run shadow mode simulation |
| /whatif <change> | Counterfactual query |
| /glassbox | Toggle Glass Box debug mode (inline system visibility) |
| /glassbox status | Show Glass Box status |
| /glassbox verbose | Toggle verbose details |
| /approve | Approve pending changes |
| /reject-finding <file>:<line> <reason> | Mark finding as false positive |
| /accept-finding <file>:<line> | Confirm finding is valid |
| /review-accuracy | Show review accuracy report |
| /campaign start <goal> | Start multi-phase campaign |
| /campaign status | Show campaign status |
| /campaign pause | Pause current campaign |
| /campaign resume | Resume paused campaign |
| /campaign list | List all campaigns |
| /launchcampaign <goal> | Clarify and auto-start a hands-free campaign |

### Tool Management (Autopoiesis)

| Command | Description |
|---------|-------------|
| /tool list | List all generated tools |
| /tool run <name> <input> | Execute a generated tool |
| /tool info <name> | Show details about a tool |
| /tool generate <description> | Generate a new tool via Ouroboros Loop |

Note: Tools are generated automatically when capabilities are missing,
or you can create them on-demand with /tool generate.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Ctrl+X | Stop current activity (visible during loading) |
| Shift+Tab | Cycle continuation mode (Auto → Confirm → Breakpoint) |
| Alt+L | Toggle logic pane |
| Alt+D | Toggle Glass Box debug mode |
| Alt+M | Toggle mouse capture (for text selection) |
| Ctrl+L | Toggle logic pane |
| Ctrl+G | Cycle pane modes |
| Ctrl+R | Toggle pane focus |
| Ctrl+P | Toggle campaign panel |
| Ctrl+C | Exit |

### Continuation Modes

| Mode | Behavior |
|------|----------|
| [A] Auto | Runs all steps automatically. Ctrl+X to stop. |
| [B] Confirm | Pauses after each step. Enter to continue. |
| [C] Breakpoint | Auto for reads, pauses before mutations. |
`
