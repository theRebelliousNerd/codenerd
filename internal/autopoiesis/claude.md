# internal/autopoiesis - Self-Modification Capabilities

This package implements autopoiesis (self-creation) - the ability for codeNERD to modify itself by detecting needs and generating new capabilities.

## Architecture

Autopoiesis provides four core capabilities:
1. **Complexity Analysis** - Detect when campaigns are needed
2. **Tool Generation** - Create new tools when capabilities are missing
3. **Persistence Analysis** - Identify when persistent agents are needed
4. **Ouroboros Loop** - Full tool self-generation cycle with safety verification

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `autopoiesis.go` | ~470 | Main orchestrator |
| `complexity.go` | ~300 | Task complexity analysis |
| `toolgen.go` | ~600 | LLM-based tool generation |
| `persistence.go` | ~400 | Persistent agent detection |
| `ouroboros.go` | ~550 | Full Ouroboros Loop with safety/compile/execute |

## The Ouroboros Loop

Named after the ancient symbol of a serpent eating its own tail, the Ouroboros Loop enables codeNERD to generate new tools at runtime.

### Loop Stages
```
Detection → Specification → Safety Check → Compile → Register → Execute
    ↑                                                              |
    └──────────────────────────────────────────────────────────────┘
```

### Stage Details

| Stage | Description |
|-------|-------------|
| `StageDetection` | Detect missing capability via Mangle query |
| `StageSpecification` | Generate tool code via LLM |
| `StageSafetyCheck` | Verify code has no forbidden imports/calls |
| `StageCompilation` | Compile tool to standalone binary |
| `StageRegistration` | Register tool in runtime registry |
| `StageExecution` | Execute tool with JSON input/output |

## Key Types

### Orchestrator
```go
type Orchestrator struct {
    complexity  *ComplexityAnalyzer
    toolGen     *ToolGenerator
    persistence *PersistenceAnalyzer
    ouroboros   *OuroborosLoop  // The Ouroboros Loop
}

func (o *Orchestrator) QuickAnalyze(ctx, input, target) QuickResult
func (o *Orchestrator) Analyze(ctx, input, target) (*AnalysisResult, error)
func (o *Orchestrator) ExecuteOuroborosLoop(ctx, need) *LoopResult
func (o *Orchestrator) ExecuteGeneratedTool(ctx, name, input) (string, error)
```

### OuroborosLoop
```go
type OuroborosLoop struct {
    toolGen       *ToolGenerator
    safetyChecker *SafetyChecker
    compiler      *ToolCompiler
    registry      *RuntimeRegistry
}

func (o *OuroborosLoop) Execute(ctx, need) *LoopResult
func (o *OuroborosLoop) ExecuteTool(ctx, name, input) (string, error)
```

### SafetyChecker
```go
type SafetyChecker struct {
    forbiddenImports []string  // unsafe, syscall, os/exec, net, etc.
    forbiddenCalls   []*regexp.Regexp  // os.RemoveAll, etc.
}

func (sc *SafetyChecker) Check(code string) *SafetyReport
```

### SafetyReport
```go
type SafetyReport struct {
    Safe           bool
    Violations     []SafetyViolation
    Score          float64  // 0.0 = unsafe, 1.0 = safe
}
```

### Forbidden Imports (Default)
| Import | Reason |
|--------|--------|
| `unsafe` | Memory safety |
| `syscall` | System calls |
| `runtime/cgo` | CGO |
| `plugin` | Plugin loading |
| `os/exec` | Command execution (unless allowed) |
| `net`, `net/http` | Networking (unless allowed) |

### Forbidden Calls (Default)
- `os.RemoveAll` - Recursive deletion
- `os.Remove` - File deletion
- `os.Chmod` - Permission change
- `os.Chown` - Ownership change
- `unsafe.Pointer` - Unsafe pointers

## Mangle Integration

The Ouroboros Loop integrates with Mangle policy (Section 12B):

```datalog
# Detect missing capability
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation
next_action(/generate_tool) :-
    missing_tool_for(_, _).

# Tool lifecycle states
tool_lifecycle(ToolName, /detected) :-
    missing_tool_for(_, ToolName).

tool_lifecycle(ToolName, /ready) :-
    tool_ready(ToolName).

# Blocked capabilities (never auto-generate)
dangerous_capability(/exec_arbitrary).
dangerous_capability(/network_unconstrained).
dangerous_capability(/system_admin).
```

## Tool Compilation

Generated tools are compiled as standalone binaries:

1. Source written to `.nerd/tools/<name>.go`
2. Wrapped with JSON input/output main function
3. Compiled with `CGO_ENABLED=0` for safety
4. Binary stored in `.nerd/tools/.compiled/<name>`
5. Executed via subprocess with timeout

## Directory Structure

```
.nerd/
├── tools/
│   ├── json_validator.go       # Generated source
│   ├── json_validator_test.go  # Generated tests
│   └── .compiled/
│       └── json_validator      # Compiled binary
```

## ComplexityLevel
| Level | Description |
|-------|-------------|
| ComplexitySimple | Single action, single file |
| ComplexityModerate | Multiple files, one phase |
| ComplexityComplex | Multiple phases, dependencies |
| ComplexityEpic | Full feature, multiple components |

## Detection Patterns

### Complexity Indicators
- Epic: "implement full system", "build complete feature"
- Complex: "refactor entire", "multi-phase migration"
- Moderate: "update all files", "rename across"

### Persistence Indicators
- Learning: "remember my preferences", "always use"
- Monitoring: "watch for changes", "alert me when"
- Triggers: "on every commit", "whenever I push"

### Tool Need Indicators
- "can't you do X", "is there a way to"
- "I need a tool for", "how can I"

## Integration Points

- `processInput` in chat.go calls `QuickAnalyze`
- Warns user about complex tasks needing campaigns
- Recommends persistent agents when detected
- Triggers Ouroboros Loop on `/generate_tool` action
- Generated tools available via `ExecuteGeneratedTool()`

## Dependencies

- `internal/perception` - LLMClient for analysis

## Testing

```bash
go test ./internal/autopoiesis/...
```
