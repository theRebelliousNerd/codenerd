# internal/prompt/sync - Agent YAML Synchronizer

This package handles synchronization of agent definitions from YAML files into shard-specific SQLite databases.

**Related Packages:**
- [internal/prompt](../CLAUDE.md) - AtomLoader for prompt atom processing
- [internal/system](../../system/CLAUDE.md) - BootCortex consuming discovered agents

## Architecture

User-defined agents are stored in `.nerd/agents/{agentName}/prompts.yaml`. This synchronizer:
1. Scans `.nerd/agents/` for agent subdirectories
2. Parses prompts.yaml files
3. Syncs atoms to shard-specific SQLite databases
4. Reports discovered agents for JIT registration

## File Index

| File | Description |
|------|-------------|
| `synchronizer.go` | Agent synchronizer syncing YAML to shard-specific SQLite databases. Exports `AgentSynchronizer`, `DiscoveredAgent` (ID/DBPath), `NewAgentSynchronizer()` with AtomLoader, `SyncAll()` scanning .nerd/agents/ subdirectories, and `GetDiscoveredAgents()` for JIT/ShardManager registration. |

## Key Types

### AgentSynchronizer
```go
type AgentSynchronizer struct {
    baseDir          string // .nerd/agents
    shardsDir        string // .nerd/shards
    atomLoader       *prompt.AtomLoader
    discoveredAgents []DiscoveredAgent
}
```

### DiscoveredAgent
```go
type DiscoveredAgent struct {
    ID     string // Agent name (e.g., "bubbleteaexpert")
    DBPath string // Path to knowledge DB
}
```

## Directory Structure

```
.nerd/
├── agents/
│   └── bubbleteaexpert/
│       └── prompts.yaml
└── shards/
    └── bubbleteaexpert_knowledge.db
```

## Sync Flow

1. `SyncAll()` scans `.nerd/agents/`
2. For each subdirectory with `prompts.yaml`:
   - Parse YAML atoms
   - Create/update `{agentName}_knowledge.db`
   - Track in discoveredAgents
3. `GetDiscoveredAgents()` returns list for registration

## Dependencies

- `internal/prompt` - AtomLoader
- `internal/logging` - Structured logging
- `github.com/mattn/go-sqlite3` - SQLite

## Testing

```bash
go test ./internal/prompt/sync/...
```

---

**Remember: Push to GitHub regularly!**
