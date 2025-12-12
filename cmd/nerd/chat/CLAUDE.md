# chat - Interactive TUI Chat Interface

The chat package provides the interactive terminal user interface (TUI) for codeNERD. Built on the Bubbletea framework, it implements the user-facing conversation layer that translates natural language into Mangle kernel operations and delegates tasks to ShardAgents.

**Related Packages:**
- [internal/perception](../../../internal/perception/CLAUDE.md) - NL parsing, intent extraction, LLM provider system
- [internal/core](../../../internal/core/CLAUDE.md) - Mangle kernel, ShadowMode, ShardManager
- [internal/campaign](../../../internal/campaign/CLAUDE.md) - Campaign orchestration

## Architecture Overview

The chat package follows the Elm Architecture (Model-Update-View) pattern enforced by Bubbletea:

1. **Model** - Holds all application state (conversation history, kernel state, UI components)
2. **Update** - Processes messages/events and returns new state + commands
3. **View** - Renders the current state to the terminal

The package acts as the "Articulation Transducer" in the codeNERD architecture, bridging the gap between human-readable conversation and the logic-first Mangle kernel.

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `model.go` | ~691 | Core Model type, Config, Init, Update loop, message types |
| `commands.go` | ~936 | `/command` routing and handlers (/review, /agent, /campaign, etc.) |
| `session.go` | ~444 | Session initialization, kernel hydration, persistence |
| `process.go` | ~267 | Input processing, intent parsing, LLM delegation |
| `view.go` | ~177 | TUI rendering (header, history, footer, progress bars) |
| `campaign.go` | ~355 | Campaign orchestration UI and progress visualization |
| `helpers.go` | ~490 | Utility functions, file operations, articulation helpers |
| `delegation.go` | ~228 | Shard spawning, task formatting, response formatting |
| `shadow.go` | ~298 | Shadow Mode simulation, What-If analysis, derivation traces |
| `review_aggregator.go` | ~400 | Multi-shard review orchestration with adversarial integration |

## Key Types

### Model
The central state container for the chat TUI.

```go
type Model struct {
    // UI Components
    textinput       textinput.Model
    viewport        viewport.Model
    spinner         spinner.Model
    styles          ui.Styles

    // State
    history         []Message
    isLoading       bool
    width, height   int

    // Backend
    kernel          *core.RealKernel
    client          *perception.Client
    shadowMode      *core.ShadowMode
    shardMgr        *core.ShardManager
    workspace       string

    // Campaign
    activeCampaign  *campaign.Campaign
    campaignOrch    *campaign.Orchestrator
}
```

### Config
Configuration passed from main.go to initialize chat.

```go
type Config struct {
    DisableSystemShards []string  // Shards to disable on startup
}
```

### Message
Represents a single message in the conversation history.

```go
type Message struct {
    Role      string    // "user" or "assistant"
    Content   string    // Message text (supports markdown)
    Time      time.Time // Timestamp
    Artifacts []string  // Attached file paths
}
```

## Command Reference

| Command | Description | Handler |
|---------|-------------|---------|
| `/help` | Show available commands | Inline help text |
| `/clear` | Clear conversation history | Reset history slice |
| `/agent <name> <task>` | Spawn specific shard agent | `spawnShard()` |
| `/review [target]` | Multi-shard code review with adversarial analysis | `spawnMultiShardReview()` |
| `/security [target]` | Security analysis | `formatShardTask()` |
| `/campaign start <goal>` | Begin multi-phase campaign | `startCampaign()` |
| `/campaign status` | Show campaign progress | `renderCampaignStatus()` |
| `/shadow <action>` | Run shadow mode simulation | `runShadowSimulation()` |
| `/whatif <change>` | Counterfactual analysis | `runWhatIfQuery()` |
| `/config wizard` | Full configuration wizard | Interactive setup for all settings |
| `/config show` | Show current configuration | Display current config |
| `/config set-theme` | Set UI theme (light/dark) | Direct config modification |
| `/init` | Initialize codebase analysis | `runInit()` |
| `/scan [--deep]` | Re-scan workspace (fast; add --deep for Cartographer) | `runScan(deep)` |

## Multi-Shard Review System

The `/review` command triggers a sophisticated multi-shard review pipeline:

```
/review [target]
    |
    v
resolveReviewTarget() → Files to review
    |
    v
MatchSpecialistsForReview() → Select domain experts
    |
    v
Parallel Shard Execution:
├── ReviewerShard (always)
├── Matched Specialists (domain-specific)
└── NemesisShard (adversarial)
    |
    v
AggregatedReview:
├── FindingsByShard
├── DeduplicatedList
└── HolisticInsights
```

### Adversarial Integration

The NemesisShard runs in parallel with regular review:
- Analyzes patch for attack surface
- Generates targeted chaos tools via Ouroboros
- Runs attack vectors in Thunderdome sandbox
- Reports vulnerabilities to aggregated review

## Integration Points

### Kernel Integration
The chat Model holds a reference to `*core.RealKernel` for:
- Asserting user intent facts
- Querying derived actions
- Running derivation to fixpoint

### Perception Transducer & LLM Providers
Uses `perception` package for:
- Parse natural language into structured intents via `perception.RealTransducer`
- Extract entities and resolve focus
- Generate surface responses
- **LLM Provider System**: Auto-detected via `perception.NewClientFromEnv()` supporting:
  - Z.AI, Anthropic, OpenAI, Gemini, xAI, OpenRouter
  - Configuration in `.nerd/config.json` or environment variables
  - See [internal/perception/CLAUDE.md](../../../internal/perception/CLAUDE.md) for details

### Shadow Mode
Integrates with `core.ShadowMode` for:
- What-If counterfactual queries
- Safe simulation of proposed actions
- Safety violation detection

### Campaign Orchestrator
Connects to `campaign.Orchestrator` for:
- Multi-phase task decomposition
- Progress tracking and visualization
- Phase execution coordination

## Message Flow

```
User Input
    |
    v
handleCommand() or processInput()
    |
    +--> /command? --> handleCommand() --> direct action
    |
    +--> natural language --> processInput()
              |
              v
         perception.Parse()
              |
              v
         kernel.Assert(user_intent)
              |
              v
         kernel.Derive() [next_action]
              |
              v
         delegateToShard() or articulateResponse()
              |
              v
         Message{Role: "assistant", ...}
              |
              v
         viewport.SetContent(renderHistory())
```

## Dependencies

- `codenerd/cmd/nerd/config` - Configuration loading/saving
- `codenerd/cmd/nerd/ui` - UI components (Styles, LogicPane, SplitPane)
- `codenerd/internal/core` - Mangle kernel, ShadowMode, ShardManager
- `codenerd/internal/perception` - NL parsing, intent extraction
- `codenerd/internal/campaign` - Campaign orchestration
- `codenerd/internal/init` - Codebase initialization
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - UI components

## Usage

```go
import "codenerd/cmd/nerd/chat"

func main() {
    cfg := chat.Config{
        DisableSystemShards: []string{},
    }
    if err := chat.RunInteractiveChat(cfg); err != nil {
        log.Fatal(err)
    }
}
```

## Testing

```bash
go test ./cmd/nerd/chat/...
```

Note: The chat package currently has no test files. Testing is done through integration tests in `cmd/nerd/`.
