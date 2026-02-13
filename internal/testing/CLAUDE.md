# internal/testing - Test Infrastructure

This package provides test infrastructure for codeNERD, including the Context Test Harness for validating infinite context system behavior.

**Related Packages:**
- [internal/context](../context/CLAUDE.md) - Context compression and activation
- [internal/session](../session/CLAUDE.md) - Session management
- [internal/core](../core/CLAUDE.md) - Kernel integration

## Subdirectories

### context_harness/

Comprehensive testing framework for validating codeNERD's infinite context system.

| File | Description |
|------|-------------|
| `types.go` | Core type definitions including Scenario, Turn, Checkpoint, Metrics. Exports Scenario with ScenarioID (kebab-case) and Name (display). |
| `harness.go` | Main orchestrator for test execution. Exports Harness with RunScenario(), RunAll(), and scenario loading. |
| `simulator.go` | Session simulator with checkpoint validation. Exports Simulator for turn-by-turn execution with metrics. |
| `scenarios.go` | Pre-built test scenarios covering debugging, feature implementation, refactoring, and research. Exports AllScenarios(), DebuggingMarathonScenario(), etc. |
| `metrics.go` | Metrics collection for compression, retrieval, and performance. Exports MetricsCollector with timing and accuracy tracking. |
| `reporter.go` | Results reporting in console and JSON formats. Exports Reporter with Summary(), Details(), and JSON output. |
| `activation_tracer.go` | Traces spreading activation through fact graph. Exports ActivationTracer for observability. |
| `jit_tracer.go` | JIT prompt compilation tracing. Exports JITTracer for atom selection visibility. |
| `compression_viz.go` | Semantic compression visualization. Exports CompressionViz for before/after comparison. |
| `inspector.go` | Deep inspection tools for debugging. Exports Inspector for kernel and context state. |
| `integration.go` | Real codeNERD integration for live testing. Exports Integration with real kernel and session. |
| `file_logger.go` | File-based logging for test runs. Exports FileLogger for persistent test output. |

## Pre-Built Scenarios

| Scenario ID | Display Name | Turns | Tests |
|-------------|--------------|-------|-------|
| `debugging-marathon` | Debugging Marathon | 50 | Long-term context retention, solution tracking |
| `feature-implementation` | Feature Implementation | 75 | Multi-phase context paging (plan → implement → test) |
| `refactoring-campaign` | Refactoring Campaign | 100 | Cross-file tracking, long-term stability |
| `research-and-build` | Research + Build | 80 | Cross-phase knowledge retrieval |

## Key Types

### Scenario
```go
type Scenario struct {
    ScenarioID      string       // kebab-case identifier (e.g., "debugging-marathon")
    Name            string       // Human-readable name
    Description     string       // What the scenario tests
    Turns           []Turn       // Simulated conversation turns
    Checkpoints     []Checkpoint // Validation points
    ExpectedMetrics Metrics      // Expected performance
}
```

### Harness
```go
type Harness struct {
    scenarios map[string]*Scenario
    metrics   *MetricsCollector
    reporter  *Reporter
}

func NewHarness() *Harness
func (h *Harness) RunScenario(id string) (*ScenarioResult, error)
func (h *Harness) RunAll() ([]*ScenarioResult, error)
```

## Usage

### CLI
```bash
nerd test-context --scenario debugging-marathon
nerd test-context --all --format json > results.json
```

### Programmatic
```go
import "codenerd/internal/testing/context_harness"

harness := context_harness.NewHarness()
result, err := harness.RunScenario("debugging-marathon")
```

## Testing

```bash
go test ./internal/testing/...
```

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*