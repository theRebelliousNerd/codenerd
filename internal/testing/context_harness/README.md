# codeNERD Context Test Harness

A comprehensive testing framework for validating codeNERD's infinite context system (compression, retrieval, and context paging) with realistic coding session simulations.

## Overview

The Context Test Harness stress-tests codeNERD's core context management capabilities:

- **Compression**: Semantic compression of conversation history (100:1 target ratio)
- **Retrieval**: Spreading activation accuracy (precision/recall/F1)
- **Paging**: Multi-phase campaign context management
- **Degradation**: Quality stability over 100+ turn sessions

## Architecture

```
internal/testing/context_harness/
├── types.go           # Core data structures (Scenario, Turn, Checkpoint, Metrics)
├── simulator.go       # Session simulator (executes scenarios, validates checkpoints)
├── scenarios.go       # Pre-built test scenarios
├── metrics.go         # Metrics collection (compression, retrieval, performance)
├── reporter.go        # Results reporting (console, JSON)
├── harness.go         # Main orchestrator
└── integration.go     # Integration with real codeNERD compression/retrieval
```

## Pre-Built Scenarios

### 1. Debugging Marathon (50 turns)
Tests long-term context retention and solution tracking.

**Pattern**: User reports error → Multiple hypothesis/test cycles → Final solution

**Checkpoints**:
- Turn 45: Recall original error after 45 turns
- Turn 49: List all failed solution attempts

**Expected Metrics**:
- Compression: 5:1
- Recall: 85%
- Precision: 80%

### 2. Feature Implementation (75 turns)
Tests multi-phase context paging (plan → implement → test).

**Pattern**: Planning (15 turns) → Implementation (40 turns) → Testing (20 turns)

**Checkpoints**:
- Turn 60: Retrieve planning details from earlier phase
- Turn 74: Track test failures across testing phase

**Expected Metrics**:
- Compression: 6:1
- Recall: 87%
- Precision: 83%

### 3. Refactoring Campaign (100 turns)
Tests long-term stability and cross-file tracking.

**Pattern**: Multi-file refactoring with cross-references

**Checkpoints**:
- Turn 50: Track all file modifications
- Turn 95: Recall original refactoring rationale

**Expected Metrics**:
- Compression: 8:1 (higher for long sessions)
- Recall: 85%
- Precision: 82%

### 4. Research + Build (80 turns)
Tests cross-phase knowledge retrieval.

**Pattern**: Research phase (40 turns) → Implementation phase (40 turns)

**Checkpoints**:
- Turn 50: Retrieve research findings from earlier phase

**Expected Metrics**:
- Compression: 7:1
- Recall: 88%
- Precision: 84%

## Usage

### Run a Single Scenario

```bash
nerd test-context --scenario debugging-marathon
```

### Run All Scenarios

```bash
nerd test-context --all
```

### JSON Output

```bash
nerd test-context --scenario debugging-marathon --format json > results.json
```

### Configuration Options

```bash
nerd test-context \
  --scenario refactoring-campaign \
  --token-budget 10000 \           # Token budget for context retrieval
  --max-turns 150 \                # Override scenario turn count
  --paging=true                    # Enable context paging
```

### List Available Scenarios

```bash
nerd test-context
```

## Metrics

### Compression Metrics
- **Compression Ratio**: `original_tokens / compressed_tokens`
  - Target: >5:1 for short sessions, >8:1 for long sessions

### Retrieval Metrics
- **Precision**: `relevant_retrieved / total_retrieved`
  - How much noise was avoided?

- **Recall**: `relevant_retrieved / total_relevant`
  - How much signal was captured?

- **F1 Score**: `2 * (precision * recall) / (precision + recall)`
  - Harmonic mean of precision and recall

### Performance Metrics
- **Compression Latency**: Time to compress a turn
- **Retrieval Latency**: Time to retrieve context
- **Peak Memory**: Maximum memory usage during session

### Degradation Metrics
- **Quality Degradation**: Response quality over session length
  - 0.0 = no degradation, 1.0 = complete failure

## Checkpoints

Each scenario has **validation checkpoints** that test retrieval accuracy at specific turns:

```go
Checkpoint{
    AfterTurn:    45,
    Query:        "What was the original error?",
    MustRetrieve: []string{"turn_0_error", "turn_0_stack_trace"},
    ShouldAvoid:  []string{"turn_30_unrelated"},
    MinRecall:    0.9,
    MinPrecision: 0.8,
}
```

- **MustRetrieve**: Facts that MUST be in retrieval results (high relevance)
- **ShouldAvoid**: Facts that should NOT be retrieved (noise)
- **MinRecall/MinPrecision**: Minimum acceptable thresholds

## Sample Report

```
═══════════════════════════════════════════════════════════════
  CONTEXT TEST HARNESS REPORT: Debugging Marathon
═══════════════════════════════════════════════════════════════

✓ STATUS: PASSED

METRICS:
───────────────────────────────────────────────────────────────
  Compression Ratio:        5.43x
  Avg Retrieval Precision:  82.35%
  Avg Retrieval Recall:     88.12%
  Avg F1 Score:             85.14%
  Token Budget Violations:  0
  Avg Compression Latency:  45ms
  Avg Retrieval Latency:    128ms
  Peak Memory:              256.32 MB

EXPECTED vs ACTUAL:
───────────────────────────────────────────────────────────────
  Compression Ratio:     5.00x (expected) | 5.43x (actual) ✓
  Retrieval Recall:      85.00% (expected) | 88.12% (actual) ✓
  Retrieval Precision:   80.00% (expected) | 82.35% (actual) ✓
  Token Violations:      0 (max) | 0 (actual) ✓

CHECKPOINT RESULTS:
───────────────────────────────────────────────────────────────
✓ Checkpoint 1 (Turn 45): Should recall original error after 45 turns
    Precision: 85.00% | Recall: 90.00% | F1: 87.41%

✓ Checkpoint 2 (Turn 49): Should track all failed solution attempts
    Precision: 80.00% | Recall: 87.50% | F1: 83.58%

═══════════════════════════════════════════════════════════════
```

## Creating Custom Scenarios

```go
func CustomScenario() *Scenario {
    return &Scenario{
        Name:        "My Custom Scenario",
        Description: "Custom test scenario description",
        Turns: []Turn{
            {
                TurnID:  0,
                Speaker: "user",
                Message: "Start of scenario",
                Intent:  "plan",
                Metadata: TurnMetadata{
                    FilesReferenced: []string{"file1.go"},
                    Topics:          []string{"planning"},
                },
            },
            // ... more turns
        },
        Checkpoints: []Checkpoint{
            {
                AfterTurn:    10,
                Query:        "What was the plan?",
                MustRetrieve: []string{"turn_0_plan"},
                MinRecall:    0.9,
                MinPrecision: 0.8,
            },
        },
        ExpectedMetrics: Metrics{
            CompressionRatio:   5.0,
            AvgRetrievalRecall: 0.85,
            AvgRetrievalPrec:   0.80,
        },
    }
}
```

## Integration with codeNERD

The harness integrates with codeNERD's production systems:

- **Compressor** (`internal/context/compressor.go`): Semantic compression
- **ActivationEngine** (`internal/context/activation.go`): Spreading activation
- **Kernel** (`internal/core/kernel.go`): Fact storage and querying

## Continuous Integration

Add to CI pipeline to catch context system regressions:

```yaml
- name: Test Context System
  run: |
    go build -o nerd ./cmd/nerd
    ./nerd test-context --all --format json > context-test-results.json

- name: Upload Results
  uses: actions/upload-artifact@v3
  with:
    name: context-test-results
    path: context-test-results.json
```

## Future Enhancements

- [ ] Load real conversation logs (from `.nerd/logs/`)
- [ ] Add adversarial scenarios (context bombing, rapid topic shifts)
- [ ] Benchmark against GPT-4/Claude native context windows
- [ ] Add visual reports (charts, graphs)
- [ ] Continuous monitoring (track metrics over time)

## References

- **CLAUDE.md §8.2**: Infinite Context via Semantic Compression
- **CLAUDE.md §8.1**: Logic-Directed Context (Spreading Activation)
- `internal/context/compressor.go`: Compression implementation
- `internal/context/activation.go`: Spreading activation implementation
- `internal/campaign/context_pager.go`: Context paging implementation
