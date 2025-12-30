# Prompt Evolution Package

Implements System Prompt Learning (SPL) for codeNERD, enabling automatic evolution of prompt atoms based on execution feedback. This is Karpathy's "third paradigm" of LLM learning.

## Architecture

```
Execute → Evaluate (LLM-as-Judge) → Evolve (Meta-Prompt) → Integrate (JIT Compiler)
```

## Core Components

| File | Purpose |
|------|---------|
| `types.go` | Core type definitions (ExecutionRecord, JudgeVerdict, Strategy, etc.) |
| `judge.go` | LLM-as-Judge evaluation with explanations |
| `feedback_collector.go` | Execution outcome recording and storage |
| `strategy_store.go` | Problem-type-specific strategy database |
| `classifier.go` | Problem type classification |
| `atom_generator.go` | Automatic atom creation from failures |
| `evolver.go` | Main evolution orchestrator |

## Key Types

### ExecutionRecord
Captures everything about a shard task execution:
- Task request and context
- Agent actions taken
- Execution result (success/failure)
- Prompt manifest (which atoms were used)
- Verdict (filled after evaluation)

### JudgeVerdict
LLM-as-Judge evaluation result:
- PASS/FAIL verdict
- Explanation of WHY
- Error category (8 types)
- Improvement rule (learning signal)

### Strategy
Problem-solving strategy from SPL:
- Problem type (16 types)
- Shard type
- Content (the strategy text)
- Success/failure tracking
- Refinement history

## Error Categories

| Category | Description |
|----------|-------------|
| `LOGIC_ERROR` | Wrong approach or algorithm |
| `SYNTAX_ERROR` | Code syntax issues |
| `API_MISUSE` | Wrong API or library usage |
| `EDGE_CASE` | Missing edge case handling |
| `CONTEXT_MISS` | Missed relevant codebase context |
| `INSTRUCTION_MISS` | Didn't follow instructions |
| `HALLUCINATION` | Made up information |
| `CORRECT` | Task completed correctly |

## Problem Types (16)

Following the optillm SPL pattern:
- `debugging`, `feature_creation`, `refactoring`, `testing`
- `documentation`, `performance`, `security`, `api_integration`
- `data_migration`, `config_setup`, `error_handling`, `concurrency`
- `type_system`, `dependency_mgmt`, `code_review`, `research`

## Storage

```
.nerd/prompts/
├── evolution.db        # Execution records, verdicts
├── strategies.db       # Strategy database
└── evolved/
    ├── pending/        # Awaiting promotion
    ├── promoted/       # Promoted to corpus
    └── rejected/       # User rejected
```

## Integration Points

- **JIT Compiler**: Evolved atoms become a new source
- **Autopoiesis**: Part of the self-improvement system
- **Shards**: All shards can record executions
- **Kernel**: Evolution facts asserted for logic-driven triggers

## Usage

```go
// Create feedback collector
collector, _ := NewFeedbackCollector(nerdDir)

// Record execution
exec := &ExecutionRecord{
    TaskID: "task-123",
    ShardType: "/coder",
    TaskRequest: "Fix the bug in auth.go",
    ExecutionResult: ExecutionResult{Success: false, ...},
}
collector.Record(exec)

// Create judge and evaluate
judge := NewTaskJudge(llmClient, "gemini-3-pro")
verdict, _ := judge.Evaluate(ctx, exec)

// Update with verdict
collector.UpdateVerdict(exec.TaskID, verdict)
```

## Mangle Facts

```datalog
evolution_cycle(Timestamp, FailuresAnalyzed, AtomsGenerated, StrategiesUpdated).
evolved_atom(AtomID, ShardType, ProblemType, Confidence, Source).
strategy_used(StrategyID, TaskID, Success).
strategy_effectiveness(StrategyID, SuccessRate, Uses).
```

## Configuration

```go
type EvolverConfig struct {
    MinFailuresForEvolution int           // Default: 3
    EvolutionInterval       time.Duration // Default: 10 minutes
    MaxAtomsPerEvolution    int           // Default: 3
    ConfidenceThreshold     float64       // Default: 0.7
    AutoPromote             bool          // Default: true
    EnableStrategies        bool          // Default: true
}
```

---

**Remember: Push to GitHub regularly!**
