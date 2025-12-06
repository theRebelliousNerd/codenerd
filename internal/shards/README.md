# internal/shards/

ShardAgents are ephemeral sub-kernels for parallel task execution. Each shard owns its Mangle kernel instance, allowing isolated reasoning and safe concurrent operation.

## Shard Types

### Type A: Ephemeral Generalists

> Spawn → Execute → Die. RAM only.

These shards handle one-off tasks and are garbage collected after completion.

| Shard | File | Purpose |
|-------|------|---------|
| **CoderShard** | `coder.go` | Code generation, refactoring, bug fixes |
| **TesterShard** | `tester.go` | Test creation, TDD loops, coverage |
| **ReviewerShard** | `reviewer.go` | Code review, security audit |

### Type B: Persistent Specialists

> Pre-populated with domain knowledge. SQLite-backed.

These shards maintain knowledge across sessions.

| Shard | Directory | Purpose |
|-------|-----------|---------|
| **ResearcherShard** | `researcher/` | Deep research, knowledge ingestion |

### Type S: System Shards

> Built-in capabilities, always available.

| Shard | Directory | Purpose |
|-------|-----------|---------|
| **FileShard** | `system/` | File read/write/search |
| **ShellShard** | `system/` | Command execution |
| **GitShard** | `system/` | Version control |

### Type O: Ouroboros

> Self-generating tools.

| Shard | File | Purpose |
|-------|------|---------|
| **ToolGeneratorShard** | `tool_generator.go` | Creates new tools at runtime |

## Structure

```
shards/
├── registration.go      # Shard registry and factory
├── coder.go            # CoderShard implementation
├── tester.go           # TesterShard implementation
├── reviewer.go         # ReviewerShard implementation
├── tool_generator.go   # Ouroboros Loop
├── researcher/         # Deep research subsystem
│   ├── researcher.go   # ResearcherShard
│   ├── extract.go      # Knowledge extraction
│   └── CLAUDE.md       # Subsystem docs
└── system/             # Built-in shards
    ├── base.go         # Common system shard interface
    ├── file.go         # FileShard
    ├── shell.go        # ShellShard
    └── git.go          # GitShard
```

## Shard Interface

Every shard implements:

```go
type Shard interface {
    ID() string
    Config() ShardConfig
    State() ShardState
    Execute(ctx context.Context, task string) (string, error)
    Shutdown() error
}

type ShardConfig struct {
    Type           ShardType    // /generalist, /specialist
    MountStrategy  MountType    // /ram, /sqlite
    KnowledgeBase  string       // Path to knowledge DB
    Permissions    []Permission // Allowed operations
}

type ShardState struct {
    Status    Status    // /idle, /running, /failed
    StartedAt time.Time
    TaskCount int
    LastError string
}
```

## Lifecycle

```
┌─────────────────────────────────────────────────────────┐
│                    SHARD MANAGER                        │
│                 (core/shard_manager.go)                 │
└────────────────────────┬────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    ┌─────────┐     ┌─────────┐     ┌─────────┐
    │ Spawn() │     │Execute()│     │Shutdown │
    │         │────▶│         │────▶│   ()    │
    └─────────┘     └─────────┘     └─────────┘
         │               │               │
         ▼               ▼               ▼
    ┌─────────┐     ┌─────────┐     ┌─────────┐
    │ Create  │     │  Run    │     │ Collect │
    │ Kernel  │     │  Task   │     │ Facts   │
    └─────────┘     └─────────┘     └─────────┘
```

## CoderShard

Handles all code modification tasks.

### Capabilities

- **Generate** - Create new code from spec
- **Refactor** - Restructure existing code
- **Fix** - Debug and patch bugs
- **Explain** - Document code behavior

### Task Format

```json
{
  "action": "fix",
  "target": "internal/auth/handler.go",
  "context": "The login function fails for users with special characters",
  "constraints": ["maintain backwards compatibility", "add tests"]
}
```

### Autopoiesis Integration

CoderShard tracks acceptance/rejection patterns:

```go
// On user rejection
c.trackRejection(action, reason)

// After 3 rejections of same pattern
// → promotes to long-term preference
```

## TesterShard

Handles test creation and the TDD repair loop.

### Capabilities

- **Generate** - Create tests for existing code
- **TDD Loop** - Write → Run → Analyze → Fix
- **Coverage** - Identify untested paths
- **Mutation** - Test quality via mutation testing

### TDD Repair Loop

```mangle
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.
```

## ReviewerShard

Handles code review with security focus.

### Capabilities

- **Review** - General code review
- **Security** - OWASP Top 10 scanning
- **Style** - Code style enforcement
- **Architecture** - Design pattern analysis

### Output Format

```json
{
  "severity": "warning",
  "file": "auth/handler.go",
  "line": 42,
  "code": "SEC001",
  "message": "SQL injection vulnerability",
  "suggestion": "Use parameterized queries"
}
```

## ResearcherShard

Deep research with persistent knowledge.

### Capabilities

- **Web Research** - Fetch and parse documentation
- **llms.txt** - Context7-style knowledge ingestion
- **Knowledge Atoms** - Extract and store facts
- **Specialist Hydration** - Pre-populate new specialists

See [researcher/CLAUDE.md](researcher/CLAUDE.md) for detailed docs.

## ToolGeneratorShard

The Ouroboros Loop - self-generating capabilities.

### Trigger

```mangle
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

next_action(/generate_tool) :-
    missing_tool_for(_, _).
```

### Output

Generates new tool definitions that are hot-loaded into the runtime.

## Creating a New Shard

### 1. Define the Shard

```go
type MyShard struct {
    id           string
    config       core.ShardConfig
    state        core.ShardState
    kernel       *core.RealKernel
    llmClient    perception.LLMClient
    virtualStore *core.VirtualStore
}

func NewMyShard(id string, deps Dependencies) *MyShard {
    return &MyShard{
        id: id,
        config: core.ShardConfig{
            Type:          core.ShardTypeGeneralist,
            MountStrategy: core.MountRAM,
        },
        state: core.ShardState{Status: core.StatusIdle},
    }
}
```

### 2. Implement the Interface

```go
func (s *MyShard) Execute(ctx context.Context, task string) (string, error) {
    s.state.Status = core.StatusRunning
    defer func() { s.state.Status = core.StatusIdle }()

    // 1. Parse task
    // 2. Assert facts to kernel
    // 3. Query for next_action
    // 4. Execute action
    // 5. Return result + facts

    return result, nil
}
```

### 3. Register the Shard

In `registration.go`:

```go
func init() {
    RegisterShard("myshard", NewMyShard)
}
```

### 4. Add Mangle Rules

Create `internal/mangle/myshard.gl`:

```mangle
# MyShard-specific rules
next_action(/my_action) :-
    shard_type(/myshard),
    task_requires(/my_capability).
```

## Permissions

Shards operate under constitutional safety:

```mangle
permitted(Action) :-
    shard_permission(ShardID, Action),
    !blocked_by_policy(Action).
```

### Permission Types

| Permission | Description |
|------------|-------------|
| `/read_file` | Read filesystem |
| `/write_file` | Write filesystem |
| `/exec_cmd` | Execute commands |
| `/network` | Network access |
| `/git_write` | Git modifications |
