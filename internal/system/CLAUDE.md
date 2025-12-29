# internal/system - Cortex Bootstrap & Factory

This package provides the core initialization and factory logic for the Cortex. It acts as the "Motherboard" that wires all components together.

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - Kernel and ShardManager being wired
- [internal/perception](../perception/CLAUDE.md) - Transducer initialization
- [internal/shards/system](../shards/system/CLAUDE.md) - System shards being registered

## Architecture

The system package provides:
- **Cortex struct**: Fully initialized system instance with all components
- **BootCortex**: Single entry point ensuring consistent wiring
- **Agent Registry**: Discovery and synchronization of user-defined agents

## File Index

| File | Description |
|------|-------------|
| `factory.go` | Main `BootCortex()` function that initializes the entire system stack. Exports `Cortex` struct containing Kernel, LLMClient, ShardManager, VirtualStore, Transducer, Orchestrator, BrowserManager, Scanner, UsageTracker, LocalDB, JITCompiler. |
| `agent_registry.go` | User-defined agent discovery and registry synchronization. Exports `AgentOnDisk`, `DiscoverAgentsOnDisk()` scanning .nerd/agents/, and `SyncAgentRegistryFromDisk()` updating agents.json with prompts.yaml directories. |
| `cortex_close.go` | Resource cleanup for Cortex instances. Exports `Close()` method that stops ShardManager, closes JITCompiler, LocalDB, and perception layer - critical for Windows temp directory cleanup with SQLite handles. |
| `holographic_code_scope.go` | Wrapper for world.FileScope ensuring deep (Cartographer) facts are incrementally maintained in kernel. Exports `HolographicCodeScope` bridging VirtualStore policies to code_defines/code_calls facts for "Holographic Retrieval". |
| `dom_demo_test.go` | Demo tests for DOM/Mangle integration with browser facts. Tests page element projection to Mangle predicates. |
| `dom_mangle_test.go` | Unit tests for DOM fact generation and honeypot detection. Tests Mangle rule evaluation against page elements. |

## Key Types

### Cortex
```go
type Cortex struct {
    Kernel         core.Kernel
    LLMClient      perception.LLMClient
    ShardManager   *core.ShardManager
    VirtualStore   *core.VirtualStore
    Transducer     *perception.RealTransducer
    Orchestrator   *autopoiesis.Orchestrator
    BrowserManager *browser.SessionManager
    Scanner        *world.Scanner
    UsageTracker   *usage.Tracker
    LocalDB        *store.LocalStore
    Workspace      string
    JITCompiler    *prompt.JITPromptCompiler
}
```

## Boot Sequence

1. Initialize Logging System
2. Initialize Usage Tracker
3. Load User Config
4. Create LLM Client (provider selection)
5. Initialize Kernel with schemas/policy
6. Create VirtualStore with constitutional rules
7. Wire Transducer with taxonomy
8. Initialize ShardManager with factories
9. Register System Shards
10. Boot Autopoiesis Orchestrator
11. Initialize JIT Prompt Compiler

## Dependencies

- All major internal packages (core, perception, articulation, shards, etc.)
- `github.com/mattn/go-sqlite3` - SQLite for project corpus

## Testing

```bash
go test ./internal/system/...
```

---

**Remember: Push to GitHub regularly!**
