# internal/regression - Battery Test Harness

This package provides a lightweight, optional regression battery harness for YAML-defined task suites.

**Related Packages:**
- [internal/shards/nemesis](../shards/nemesis/CLAUDE.md) - Gauntlet consuming batteries

## Architecture

Batteries are YAML-defined task suites that can be run:
- As part of Nemesis gauntlets
- Manually to continuously evaluate agent behavior
- For regression testing of generated tools

The harness uses fail-fast semantics to keep gauntlet latency bounded.

## File Index

| File | Description |
|------|-------------|
| `battery.go` | YAML-defined regression task suite runner with fail-fast execution. Exports `Battery` (Version/Tasks), `Task` (ID/Type/Command/TimeoutSec), `Result` (Success/Output/Error/DurationMs), `LoadBattery()` parsing YAML, and `RunBattery()` executing tasks sequentially with timeout. |

## Key Types

### Battery
```go
type Battery struct {
    Version int    `yaml:"version"`
    Tasks   []Task `yaml:"tasks"`
}
```

### Task
```go
type Task struct {
    ID         string `yaml:"id"`
    Type       string `yaml:"type"`    // "shell"
    Command    string `yaml:"command"`
    TimeoutSec int    `yaml:"timeout_sec,omitempty"`
}
```

### Result
```go
type Result struct {
    TaskID     string
    Success    bool
    Output     string
    Error      string
    DurationMs int64
}
```

## Battery YAML Format

```yaml
version: 1
tasks:
  - id: build_check
    type: shell
    command: go build ./...
    timeout_sec: 60
  - id: vet_check
    type: shell
    command: go vet ./...
    timeout_sec: 30
```

## Execution Behavior

- Tasks run sequentially in order
- First failure stops execution (fail-fast)
- Default timeout: 5 minutes per task
- Working directory configurable

## Dependencies

- `gopkg.in/yaml.v3` - YAML parsing
- Standard library only

## Testing

```bash
go test ./internal/regression/...
```
