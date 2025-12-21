# internal/shards/tester - TesterShard Test Execution Engine

This package implements the TesterShard for test execution, generation, coverage analysis, and TDD repair loops. It supports multiple testing frameworks with automatic detection.

**Related Packages:**
- [internal/core](../../core/CLAUDE.md) - Kernel for fact assertion and VirtualStore for execution
- [internal/articulation](../../articulation/CLAUDE.md) - Piggyback Protocol for LLM response handling
- [internal/store](../../store/CLAUDE.md) - LearningStore for autopoiesis pattern persistence

## Architecture

The TesterShard follows the Cortex 1.5.0 §7.0 Sharding specification:
- Multi-framework support (Go, Jest, pytest, Cargo, JUnit, xUnit, RSpec, PHPUnit)
- Automatic framework detection from file extensions
- Coverage analysis with configurable goals
- TDD repair loops with retry limits
- Mock staleness detection and regeneration
- Autopoiesis pattern learning from test outcomes

## File Index

| File | Description |
|------|-------------|
| `tester.go` | Core TesterShard struct with Execute() orchestrating test actions and lifecycle management. Exports TesterShard, TesterConfig, TestResult, FailedTest, GeneratedTest, TesterTask types and dependency injection methods (SetLLMClient, SetKernel, SetVirtualStore). |
| `generation.go` | Test generation via LLM with framework-specific prompts and Piggyback Protocol handling. Exports generateTests() building system/user prompts, calling LLM with retry, and processing responses through articulation layer. |
| `execution.go` | Test execution for multiple frameworks via VirtualStore or direct shell. Exports runTests() building framework-specific commands, executing via kernel pipeline, and parsing results with timing metrics. |
| `detection.go` | Framework auto-detection and command building for 8+ testing frameworks. Exports detectFramework() using file extensions, buildTestCommand() and buildCoverageCommand() generating framework-appropriate CLI commands. |
| `parsing.go` | Task string parsing extracting action, target, and options from natural language. Exports parseTask() handling file:, function:, package: parameters and action aliases (test/run/gen/cov/tdd). |
| `pytest_parser.go` | State machine parser for comprehensive pytest verbose output diagnostics. Exports PytestParserState enum, PytestFailure struct, and ParsePytestOutput() extracting tracebacks, assertions, and where-clause introspection. |
| `output.go` | Output parsing for multi-framework failure detection with regex patterns. Exports containsFailure() for quick checks and parseFailedTests() with framework-specific parsing (Go, Jest, pytest, Cargo). |
| `helpers.go` | Utility functions for content hashing and language detection. Exports hashContent() for deduplication and detectLanguage() mapping file extensions to Mangle atoms. |
| `autopoiesis.go` | Self-improvement pattern tracking per §8.3 Autopoiesis specification. Exports trackFailurePattern(), trackSuccessPattern(), and loadLearnedPatterns() persisting recurring patterns to LearningStore. |
| `facts.go` | Mangle fact generation from test results for kernel propagation. Exports assertInitialFacts() for task start and generateFacts() creating test_state, test_output, coverage_metric, test_failure predicates. |
| `route.go` | Kernel action routing with blocking wait for OODA pipeline integration. Exports assertNextActionAndWait() for pending_action envelope submission and waitForRoutingResult() polling for routing_result. |
| `mocks.go` | Mock staleness detection and LLM-powered regeneration. Exports MockInfo struct, detectStaleMocks() analyzing interface file timestamps, extractMockImports() parsing mock patterns, and regenerateMocks() via LLM. |
| `parsing_test.go` | Unit tests for task parsing with defaults and parameter extraction. Tests parseTask() handling bare actions, file:, pkg: syntax, and action aliases. |

## Key Types

### TesterShard
```go
type TesterShard struct {
    id              string
    kernel          *core.RealKernel
    llmClient       types.LLMClient
    virtualStore    *core.VirtualStore
    learningStore   core.LearningStore
    testerConfig    TesterConfig
    failurePatterns map[string]int
    successPatterns map[string]int
}
```

### TestResult
```go
type TestResult struct {
    Passed      bool
    Output      string
    Coverage    float64
    FailedTests []FailedTest
    PassedTests []string
    Duration    time.Duration
    Diagnostics []core.Diagnostic
    Retries     int
    Framework   string
    TestType    string // "unit", "integration", "e2e", "unknown"
}
```

### TesterTask
```go
type TesterTask struct {
    Action   string // "run_tests", "generate_tests", "coverage", "tdd"
    Target   string // File path, package, or function
    File     string // Specific file
    Function string // Specific function
    Package  string // Package path
    Options  map[string]string
}
```

## Supported Frameworks

| Framework | Extension | Test Command | Coverage Command |
|-----------|-----------|--------------|------------------|
| gotest | .go | `go test` | `go test -cover` |
| jest | .ts/.js | `npx jest` | `npx jest --coverage` |
| pytest | .py | `pytest` | `pytest --cov` |
| cargo | .rs | `cargo test` | `cargo llvm-cov` |
| junit | .java | `mvn test` | `mvn test jacoco:report` |
| xunit | .cs | `dotnet test` | `dotnet test --collect:"XPlat Code Coverage"` |
| rspec | .rb | `rspec` | N/A |
| phpunit | .php | `vendor/bin/phpunit` | N/A |

## Execution Pipeline

```
Task String
    |
    v
parseTask() → Action, Target, Options
    |
    v
assertInitialFacts() → tester_task, coverage_goal
    |
    v
Action Dispatch:
    +--[run_tests]-------> runTests() → execute, parse output
    +--[generate_tests]--> generateTests() → LLM prompt, Piggyback
    +--[coverage]--------> runCoverage() → parse percentage
    +--[tdd]-------------> tddLoop() → retry with fixes
    +--[detect_stale_mocks] → detectStaleMocks()
    +--[regenerate_mocks]-> regenerateMocks()
    |
    v
generateFacts() → test_state, test_failure, coverage_metric
    |
    v
trackFailurePattern() / trackSuccessPattern() → Autopoiesis
```

## Dependencies

- `internal/core` - Kernel, VirtualStore, Diagnostic types
- `internal/articulation` - ProcessLLMResponse for Piggyback Protocol
- `internal/types` - LLMClient interface
- `internal/logging` - Structured logging with category

## Testing

```bash
go test ./internal/shards/tester/...
```
