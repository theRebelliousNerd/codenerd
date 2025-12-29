# internal/testing/context_harness - Context System Test Harness

This package validates codeNERD's infinite context system through realistic session simulations. It tests compression, spreading activation, and retrieval accuracy across multi-turn coding sessions.

**Related Packages:**
- [internal/context](../../context/CLAUDE.md) - Compression and activation engines
- [internal/core](../../core/CLAUDE.md) - Kernel and fact store
- [internal/store](../../store/CLAUDE.md) - Knowledge persistence

## Architecture

```
CLI (cmd_test_context.go)
    │
    ▼
Harness ──────────────────────────────────────────────┐
    │                                                 │
    ▼                                                 │
SessionSimulator ─────────────► RealContextEngine     │
    │ (per-turn execution)          │                 │
    │                               ├── Compressor    │
    ▼                               ├── Kernel        │
Scenario ───► Turns[] ───► Checkpoints[]              │
                                                      │
Observability Layer ◄─────────────────────────────────┘
    ├── PromptInspector      (prompts.log)
    ├── JITTracer            (jit-compilation.log)
    ├── ActivationTracer     (spreading-activation.log)
    ├── CompressionVisualizer (compression.log)
    └── FileLogger           (manages all log files)
```

## File Index

| File | Description |
|------|-------------|
| `types.go` | Core types: Scenario, Turn, TurnMetadata, Checkpoint, Metrics, TestResult, CheckpointResult, SimulatorConfig. Scenarios have ScenarioID (kebab-case) and Name (display). |
| `harness.go` | Main orchestrator. Exports Harness with RunScenario(), RunAll(), ListScenarios(). NewHarnessWithObservability() wires in tracers. |
| `simulator.go` | Turn-by-turn execution with metrics collection. Exports SessionSimulator with SetObservability(), SetContextEngine(). Calls tracers on compression/retrieval events. |
| `scenarios.go` | Pre-built test scenarios. Exports AllScenarios(), GetScenario(), and 8 scenario constructors. |
| `engine_interface.go` | ContextEngine interface defining dual-mode API. Exports ContextEngine, ActivationBreakdown, CompressionStats, ActivationValidation types. |
| `mock_engine.go` | Fast mock engine for CI. Exports MockContextEngine with simplified scoring (no real LLM calls). |
| `real_engine.go` | Real integration engine. Exports RealIntegrationEngine wiring actual ActivationEngine, Compressor, and Kernel. |
| `test_kernel_factory.go` | Isolated kernel creation. Exports TestKernelFactory for fresh test kernels with minimal schemas. |
| `fact_seeder.go` | Fact seeding utilities. Exports FactSeeder for campaign, issue, symbol graph, and dependency link facts. |
| `scenarios_integration.go` | Integration scenarios (6 new). Exports CampaignPhaseTransition, SWEBenchIssueResolution, TokenBudgetOverflow, DependencySpreading, VerbSpecificBoosting, EphemeralFiltering scenarios. |
| `metrics.go` | Metrics collection. Exports MetricsCollector tracking compression ratios, retrieval precision/recall, latencies. |
| `reporter.go` | Results output. Exports Reporter with Report(), ReportSummary() supporting console and JSON formats. |
| `file_logger.go` | Log file management. Exports FileLogger creating session directories with 6 log files + MANIFEST.txt. |
| `jit_tracer.go` | JIT compilation tracing. Exports JITTracer, CompilationSnapshot, CompiledAtom. TraceCompilation() writes detailed atom selection logs. |
| `activation_tracer.go` | Spreading activation tracing. Exports ActivationTracer, ActivationSnapshot, FactActivation. TraceActivation() writes fact scoring details. |
| `compression_viz.go` | Compression visualization. Exports CompressionVisualizer, CompressionEvent. VisualizeCompression() shows before/after with ratios. |
| `piggyback_tracer.go` | Piggyback protocol tracing. Exports PiggybackTracer, PiggybackEvent. TracePiggyback() shows surface/control packet split and state changes. |
| `inspector.go` | Deep inspection tools. Exports PromptInspector for LLM prompt logging with token counts. |

## Engine Modes

| Mode | Flag | Description |
|------|------|-------------|
| Mock | `--mode=mock` (default) | Fast mock implementations for CI. No LLM calls. |
| Real | `--mode=real` | Real ActivationEngine, Compressor, Kernel. Uses LLM. |

## Available Scenarios

### Mock Scenarios (fast, for CI)

| ID | Name | Turns | Tests |
|----|------|-------|-------|
| `debugging-marathon` | Debugging Marathon | 50 | Long-term context retention, error tracking |
| `feature-implementation` | Feature Implementation | 75 | Multi-phase context paging |
| `refactoring-campaign` | Refactoring Campaign | 100 | Cross-file tracking, long-term stability |
| `research-and-build` | Research and Build | 80 | Cross-phase knowledge retrieval |
| `tdd-loop` | TDD Loop | 40 | Test-fix cycle compression |
| `campaign-execution` | Campaign Execution | 60 | Context paging across phases |
| `shard-collaboration` | Shard Collaboration | 50 | Piggyback protocol, cross-shard context |
| `mangle-policy-debug` | Mangle Policy Debug | 45 | Logic-specific context retrieval |

### Integration Scenarios (requires --mode=real)

| ID | Name | Turns | Tests |
|----|------|-------|-------|
| `campaign-phase-transition` | Campaign Phase Transition | 60 | Phase reset, context paging, campaign-aware activation |
| `swebench-issue-resolution` | SWE-Bench Issue Resolution | 50 | Issue context, tiered file boosting (+50 for Tier 1) |
| `token-budget-overflow` | Token Budget Overflow | 100 | Compression triggering at 60% utilization |
| `dependency-spreading` | Dependency Spreading | 40 | Symbol graph spreading, 50% depth decay |
| `verb-specific-boosting` | Verb-Specific Boosting | 30 | 8 intent verb boost validation |
| `ephemeral-filtering` | Ephemeral Filtering | 20 | Boot guard, fact category filtering |

## Key Types

### Scenario
```go
type Scenario struct {
    ScenarioID      string       // kebab-case (e.g., "debugging-marathon")
    Name            string       // Human-readable
    Description     string
    Turns           []Turn       // Simulated conversation
    Checkpoints     []Checkpoint // Validation points
    ExpectedMetrics Metrics      // Performance thresholds
}
```

### Turn
```go
type Turn struct {
    TurnID   int
    Speaker  string // "user" or "assistant"
    Message  string
    Intent   string // "debug", "implement", "test", etc.
    Metadata TurnMetadata
}

type TurnMetadata struct {
    FilesReferenced         []string
    SymbolsReferenced       []string
    ErrorMessages           []string
    Topics                  []string
    IsQuestionReferringBack bool
    ReferencesBackToTurn    *int
}
```

### Checkpoint
```go
type Checkpoint struct {
    AfterTurn    int
    Query        string   // Simulated retrieval query
    MustRetrieve []string // Required fact IDs
    ShouldAvoid  []string // Noise fact IDs
    MinRecall    float64
    MinPrecision float64
    Description  string
}
```

## Output Directory Structure

```
.nerd/context-tests/
└── session-YYYYMMDD-HHMMSS/
    ├── MANIFEST.txt           # Session metadata and file descriptions
    ├── summary.log            # Overall session statistics
    ├── prompts.log            # Full prompts sent to LLM
    ├── jit-compilation.log    # JIT atom selection details
    ├── spreading-activation.log # Fact scoring and retrieval
    ├── compression.log        # Before/after compression
    └── piggyback-protocol.log # Control packet parsing
```

## Usage

### CLI
```bash
# List available scenarios
nerd test-context

# Run specific scenario
nerd test-context --scenario debugging-marathon

# Run all scenarios
nerd test-context --all

# Verbose output with all details
nerd test-context --scenario tdd-loop -v

# JSON output for CI
nerd test-context --all --format json > results.json

# Disable specific tracers
nerd test-context --scenario debugging-marathon --trace-jit=false
```

### Programmatic
```go
import "codenerd/internal/testing/context_harness"

// Create harness with observability
harness := context_harness.NewHarnessWithObservability(
    kernel,
    config,
    os.Stdout,
    "console",
    promptInspector,
    jitTracer,
    activationTracer,
    compressionViz,
    contextEngine,
)

// Run scenario
result, err := harness.RunScenario(ctx, "debugging-marathon")
```

## How Tracing Works

1. **FileLogger** creates session directory and 6 log files
2. **Tracers** are created with writers from FileLogger
3. **Simulator** calls tracers during turn execution:
   - Compression events → `compressionViz.VisualizeCompression()`
   - Retrieval events → `activationTracer.TraceActivation()`
   - JIT compilation → `jitTracer.TraceCompilation()`
4. **FileLogger.Close()** writes footers and MANIFEST

## Testing

```bash
go test ./internal/testing/context_harness/...
```

## Extending

### Adding New Scenarios

1. Create constructor in `scenarios.go`:
```go
func MyNewScenario() *Scenario {
    return &Scenario{
        ScenarioID:  "my-new-scenario",
        Name:        "My New Scenario",
        Turns:       []Turn{...},
        Checkpoints: []Checkpoint{...},
        ExpectedMetrics: Metrics{...},
    }
}
```

2. Register in `AllScenarios()` and `GetScenario()`.

### Adding New Tracers

1. Create tracer struct with `io.Writer`
2. Add to `Harness` and `SessionSimulator`
3. Wire in `NewHarnessWithObservability()`
4. Call tracer methods from `executeTurn()`

---

**Remember: Push to GitHub regularly!**
