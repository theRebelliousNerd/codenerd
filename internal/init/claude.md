# internal/init - Workspace Initialization

This package handles initialization of codeNERD workspaces, including codebase scanning, profile generation, and agent setup.

## Architecture

The initializer performs comprehensive workspace setup:
1. Scan codebase structure
2. Detect project type and framework
3. Generate Mangle profile facts
4. Create system agents (Type 1)
5. Initialize knowledge stores

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `initializer.go` | ~1700 | Main initialization logic |

## Key Types

### Initializer
```go
type Initializer struct {
    client    LLMClient
    workspace string
    kernel    *core.RealKernel
}

func (i *Initializer) Init(ctx context.Context, force bool) (*InitResult, error)
```

### InitResult
```go
type InitResult struct {
    WorkspacePath  string
    ProjectType    string
    Framework      string
    FilesScanned   int
    FactsGenerated int
    AgentsCreated  []CreatedAgent
    Warnings       []string
}
```

### CreatedAgent
```go
type CreatedAgent struct {
    Name   string
    Type   int  // ShardType
    KBSize int
    Status string
}
```

## Initialization Steps

1. **Validate Workspace**
   - Check directory exists
   - Verify not already initialized (unless --force)

2. **Scan Codebase**
   - Walk directory tree
   - Identify source files by extension
   - Extract file metadata (size, modified)

3. **Detect Project Type**
   - Check for go.mod → Go project
   - Check for package.json → Node.js
   - Check for Cargo.toml → Rust
   - Check for requirements.txt → Python

4. **Generate Profile**
   - Create `.nerd/profile.gl` with facts
   - Include file inventory
   - Include project metadata

5. **Create System Agents**
   - Initialize Type 1 system shards
   - Set up learning stores
   - Configure default profiles

6. **Persist State**
   - Write `.nerd/state.json`
   - Create session history directory

## Generated Facts

```datalog
# Project metadata
project_root("/path/to/workspace").
project_type(/go).
project_framework(/none).

# File inventory
file_exists("/src/main.go").
file_type("/src/main.go", /go).
file_size("/src/main.go", 1234).

# Directory structure
dir_exists("/src").
dir_contains("/src", "main.go").
```

## Directory Structure Created

```
.nerd/
├── profile.gl          # Generated Mangle facts
├── state.json          # Session state
├── sessions/           # Session histories
├── shards/             # Shard knowledge DBs
├── campaigns/          # Campaign checkpoints
└── tools/              # Generated tools
```

## Force Reinit

`/init --force` will:
- Preserve learned preferences from `.nerd/preferences.json`
- Regenerate profile.gl with fresh codebase scan
- Keep existing agent definitions

## Dependencies

- `internal/core` - Kernel for fact loading
- `internal/perception` - LLMClient for analysis
- `internal/world` - Scanner for codebase

## Testing

```bash
go test ./internal/init/...
```
