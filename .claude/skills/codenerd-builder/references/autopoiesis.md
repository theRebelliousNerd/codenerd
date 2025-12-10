# Autopoiesis: Self-Creation and Runtime Learning

**Version:** 2.0.0
**Specification Section:** §8.3, §12, §19.9
**Implementation:** `internal/autopoiesis/`, `internal/mangle/differential.go`, `internal/core/dreamer.go`

---

## 1. Theoretical Foundation

### 1.1 What is Autopoiesis?

The term "autopoiesis" (Greek: αὐτo- "self" + ποίησις "creation") was coined by biologists Humberto Maturana and Francisco Varela to describe systems capable of reproducing and maintaining themselves. In codeNERD, we extend this concept to mean:

> **A system that can structurally alter its own behavior by observing patterns in its interactions, without requiring external fine-tuning or retraining.**

This is fundamentally different from traditional machine learning:

| Traditional ML | codeNERD Autopoiesis |
|---------------|----------------------|
| Requires training data | Learns from live interactions |
| Updates model weights | Updates logic rules (IDB) |
| Offline process | Runtime, per-session |
| Probabilistic | Deterministic (logic-based) |
| Opaque | Fully auditable |

### 1.2 The Three Modalities of Self-Creation

codeNERD implements autopoiesis through three distinct mechanisms:

1. **Preference Learning** - Detecting repeated user rejections/acceptances and promoting patterns to long-term memory
2. **Tool Self-Generation** - The "Ouroboros Loop" where the agent writes, compiles, and binds new tools at runtime
3. **Campaign Learning** - Extracting success/failure patterns from multi-phase goal execution

---

## 2. The Learning Architecture

### 2.1 Information Flow

```text
                              ┌──────────────────┐
                              │   User Actions   │
                              │  (Accept/Reject) │
                              └────────┬─────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                        SHARD EXECUTION                          │
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │ CoderShard  │    │ TesterShard  │    │ ReviewerShard    │   │
│  │             │    │              │    │                  │   │
│  │ rejection   │    │ rejection    │    │ rejection        │   │
│  │ Count[k]++  │    │ Count[k]++   │    │ Count[k]++       │   │
│  │             │    │              │    │                  │   │
│  │ acceptance  │    │ acceptance   │    │ acceptance       │   │
│  │ Count[k]++  │    │ Count[k]++   │    │ Count[k]++       │   │
│  └──────┬──────┘    └──────┬───────┘    └────────┬─────────┘   │
│         │                  │                     │              │
└─────────┼──────────────────┼─────────────────────┼──────────────┘
          │                  │                     │
          ▼                  ▼                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                     MANGLE KERNEL                               │
│                                                                 │
│   rejection_count(Pattern, N) ← [from Go runtime]               │
│                                                                 │
│   preference_signal(Pattern) :-                                 │
│       rejection_count(Pattern, N), N >= 3.                      │
│                                                                 │
│   promote_to_long_term(FactType, FactValue) :-                  │
│       preference_signal(Pattern),                               │
│       derived_rule(Pattern, FactType, FactValue).               │
│                                                                 │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LEARNING STORE                               │
│                   (SQLite per shard)                            │
│                                                                 │
│   .nerd/shards/coder_learnings.db                               │
│   .nerd/shards/tester_learnings.db                              │
│   .nerd/shards/reviewer_learnings.db                            │
│                                                                 │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │  id | fact_predicate | fact_args | confidence | learned │   │
│   │-----+----------------+-----------+------------+---------│   │
│   │  1  | avoid_pattern  | ["unwrap"]| 0.95       | 2025-.. │   │
│   │  2  | style_pref     | ["concise"]| 0.80      | 2025-.. │   │
│   └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                   COGNITIVE REHYDRATION                         │
│             (On session start / shard spawn)                    │
│                                                                 │
│   Learnings → Mangle Facts → Influence next_action derivation   │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 The Trigger Threshold

The system uses a **3-strike rule** for preference detection:

```mangle
# SECTION 12: AUTOPOIESIS / LEARNING (§8.3)

# Detect repeated rejection pattern
preference_signal(Pattern) :-
    rejection_count(Pattern, N),
    N >= 3.
```

**Why 3?**
- 1 rejection = noise (user changed their mind)
- 2 rejections = coincidence (similar but unrelated)
- 3 rejections = pattern (statistically significant preference)

This threshold is configurable but defaults to 3 based on empirical observation of user interaction patterns.

---

## 3. Preference Learning (Shard-Level)

### 3.1 Rejection/Acceptance Tracking

Each shard maintains in-memory counters for patterns:

```go
// internal/shards/coder.go

type CoderShard struct {
    // ...

    // Learnings for autopoiesis
    rejectionCount  map[string]int  // key: "action:reason"
    acceptanceCount map[string]int  // key: "action"
}
```

**Tracking Events:**

```go
// Track rejection when edit is blocked or fails
func (c *CoderShard) trackRejection(action, reason string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    key := fmt.Sprintf("%s:%s", action, reason)
    c.rejectionCount[key]++
}

// Track acceptance when edit succeeds
func (c *CoderShard) trackAcceptance(action string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.acceptanceCount[action]++
}
```

**Integration Point:**

```go
// In Execute() method
if err := c.applyEdits(ctx, result.Edits); err != nil {
    // Track rejection for autopoiesis
    c.trackRejection(coderTask.Action, err.Error())
    return "", fmt.Errorf("failed to apply edits: %w", err)
}

// Track success for autopoiesis
c.trackAcceptance(coderTask.Action)
```

### 3.2 Pattern Types

The system recognizes several pattern categories:

| Pattern Type | Example | Derived Rule |
|-------------|---------|--------------|
| `avoid_pattern` | `"unwrap"` in Rust | Don't use `.unwrap()` |
| `style_preference` | `"concise"` | Keep responses brief |
| `tool_preference` | `"manual_review"` | Always ask before large edits |
| `language_hint` | `"go:error_handling"` | Use Go-style error returns |

### 3.3 Promotion Logic

The Mangle kernel derives when a pattern should be promoted:

```mangle
# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).
```

The Go runtime intercepts `promote_to_long_term` derivations and writes to the LearningStore.

---

## 4. The LearningStore (Persistence Layer)

### 4.1 Architecture

The `LearningStore` provides shard-specific SQLite persistence:

```go
// internal/store/learning.go

type Learning struct {
    ID             int64     `json:"id"`
    ShardType      string    `json:"shard_type"`      // "coder", "tester", "reviewer"
    FactPredicate  string    `json:"fact_predicate"`  // e.g., "style_preference"
    FactArgs       []any     `json:"fact_args"`       // Arguments to the predicate
    LearnedAt      time.Time `json:"learned_at"`
    SourceCampaign string    `json:"source_campaign"` // Campaign that taught this
    Confidence     float64   `json:"confidence"`      // 0.0-1.0, decays over time
}

type LearningStore struct {
    mu       sync.RWMutex
    basePath string              // Default: ".nerd/shards"
    dbs      map[string]*sql.DB  // One DB per shard type
}
```

### 4.2 Storage Layout

```text
.nerd/
└── shards/
    ├── coder_learnings.db
    ├── tester_learnings.db
    └── reviewer_learnings.db
```

### 4.3 Schema

```sql
CREATE TABLE IF NOT EXISTS learnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    shard_type TEXT NOT NULL,
    fact_predicate TEXT NOT NULL,
    fact_args TEXT NOT NULL,           -- JSON serialized
    learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    source_campaign TEXT DEFAULT '',
    confidence REAL DEFAULT 1.0,
    UNIQUE(fact_predicate, fact_args)  -- Prevent duplicates
);
CREATE INDEX IF NOT EXISTS idx_learnings_predicate ON learnings(fact_predicate);
CREATE INDEX IF NOT EXISTS idx_learnings_confidence ON learnings(confidence);
```

### 4.4 Key Operations

**Upsert with Reinforcement:**

```go
// Save persists a learning, reinforcing if it already exists
func (ls *LearningStore) Save(shardType string, factPredicate string, factArgs []any, sourceCampaign string) error {
    // Upsert - if exists, increase confidence (reinforce learning)
    _, err = db.Exec(`
        INSERT INTO learnings (shard_type, fact_predicate, fact_args, source_campaign, confidence)
        VALUES (?, ?, ?, ?, 1.0)
        ON CONFLICT(fact_predicate, fact_args) DO UPDATE SET
            confidence = MIN(1.0, confidence + 0.1),  -- Reinforce by 10%
            learned_at = CURRENT_TIMESTAMP,
            source_campaign = excluded.source_campaign
    `, shardType, factPredicate, string(argsJSON), sourceCampaign)
    return err
}
```

**Load with Confidence Threshold:**

```go
// Load retrieves learnings above minimum confidence
func (ls *LearningStore) Load(shardType string) ([]Learning, error) {
    rows, err := db.Query(`
        SELECT id, shard_type, fact_predicate, fact_args, learned_at, source_campaign, confidence
        FROM learnings
        WHERE confidence > 0.3  -- Ignore low-confidence learnings
        ORDER BY confidence DESC, learned_at DESC
    `)
    // ...
}
```

---

## 5. Confidence Decay (Forgetting)

### 5.1 The Forgetting Mechanism

Learnings that are not reinforced gradually fade:

```go
// DecayConfidence reduces confidence of old learnings over time.
// This implements "forgetting" - learnings not reinforced will fade.
func (ls *LearningStore) DecayConfidence(shardType string, decayFactor float64) error {
    // Decay learnings older than 7 days that haven't been reinforced
    _, err = db.Exec(`
        UPDATE learnings
        SET confidence = confidence * ?
        WHERE learned_at < datetime('now', '-7 days')
    `, decayFactor)

    // Clean up very low confidence learnings
    _, err = db.Exec(`DELETE FROM learnings WHERE confidence < 0.1`)
    return err
}
```

### 5.2 Decay Schedule

| Age | Decay Factor | Effect |
|-----|--------------|--------|
| 0-7 days | 1.0 | No decay |
| 7-14 days | 0.9 | 10% reduction |
| 14-21 days | 0.81 | 19% total reduction |
| 21-28 days | 0.73 | 27% total reduction |
| ... | ... | ... |
| >60 days | <0.1 | Deleted (forgotten) |

This mimics human memory: frequently-used knowledge is retained, while unused knowledge fades.

---

## 6. The Ouroboros Loop (Tool Self-Generation)

### 6.1 Concept

When the agent encounters a problem that cannot be solved with existing tools, it can write, compile, and bind new tools at runtime—consuming its own tail like the mythical Ouroboros.

### 6.2 Detection Logic

```mangle
# SECTION 12: AUTOPOIESIS / LEARNING (§8.3)

# Helper: derive when we HAVE a capability (for safe negation)
has_capability(Cap) :-
    tool_capabilities(_, Cap).

# Derive missing_tool_for if user intent requires a capability we don't have
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation if tool is missing
next_action(/generate_tool) :-
    missing_tool_for(_, _).
```

### 6.3 Refactored Architecture (v2.0)

The Ouroboros Loop has been completely rewritten as a **Mangle-Governed Transactional State Machine**.

**Key Components:**

| Component | File | Purpose |
|-----------|------|---------|
| OuroborosLoop | `internal/autopoiesis/ouroboros.go` | Transactional state machine |
| SafetyChecker | `internal/autopoiesis/checker.go` | AST + Mangle policy validation |
| ToolGenerator | `internal/autopoiesis/toolgen.go` | LLM-based code generation |
| ToolCompiler | `internal/autopoiesis/ouroboros.go` | Go compilation with wrapper |
| RuntimeRegistry | `internal/autopoiesis/ouroboros.go` | Tool registration and execution |
| state.mg | `internal/autopoiesis/state.mg` | Mangle state machine rules |
| go_safety.mg | `internal/autopoiesis/go_safety.mg` | Embedded safety policy |

### 6.4 The 4-Phase Protocol

```text
┌─────────────────────────────────────────────────────────────────┐
│                    OUROBOROS LOOP                                │
│                 Transactional State Machine                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     │
│  │   PROPOSAL   │ --> │    AUDIT     │ --> │  SIMULATION  │     │
│  │              │     │              │     │              │     │
│  │ Generate &   │     │ SafetyCheck  │     │ Differential │     │
│  │ Sanitize     │     │ + Retry Loop │     │ Engine       │     │
│  └──────────────┘     └──────────────┘     └──────────────┘     │
│         │                    │                    │              │
│         │ Feedback on        │ Max retries        │ Transition   │
│         │ violations         │ exceeded           │ invalid      │
│         └────────────────────┴────────────────────┘              │
│                              │                                   │
│                              v                                   │
│                    ┌──────────────┐                              │
│                    │    COMMIT    │                              │
│                    │              │                              │
│                    │ Compile +    │                              │
│                    │ Register +   │                              │
│                    │ Hot-Reload   │                              │
│                    └──────────────┘                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Phase Details:**

1. **PROPOSAL** - Generate tool code via LLM, sanitize with Mangle transpiler
   - If retry: regenerate with safety violation feedback

2. **AUDIT** - SafetyChecker validates code against `go_safety.mg`
   - Extracts AST facts: imports, calls, goroutines
   - Queries `?violation(V)` predicate
   - Retry loop with max 3 attempts

3. **SIMULATION** - DifferentialEngine validates state transition
   - Assert current and proposed states
   - Check `?valid_transition(Next)` - stability must not degrade
   - Check `?stagnation_detected()` - no repeated code hashes

4. **COMMIT** - Compile, register, and hot-reload
   - Write to `.nerd/tools/`
   - Compile with `go build`
   - Register in RuntimeRegistry
   - Update Mangle with `tool_registered` facts

### 6.5 SafetyChecker (AST + Mangle Policy)

```go
// internal/autopoiesis/checker.go

//go:embed go_safety.mg
var goSafetyPolicy string

type SafetyChecker struct {
    config      OuroborosConfig
    policy      string          // Embedded Mangle policy
    allowedPkgs []string        // Dynamic allowlist
}

type SafetyReport struct {
    Safe           bool
    Violations     []SafetyViolation
    ImportsChecked int
    CallsChecked   int
    Score          float64  // 0.0 = unsafe, 1.0 = safe
}
```

**Violation Types:**

| Type | Description | Severity |
|------|-------------|----------|
| `ViolationForbiddenImport` | Import not in allowlist | Blocking |
| `ViolationDangerousCall` | Call to dangerous function | Blocking |
| `ViolationUnsafePointer` | Use of unsafe package | Blocking |
| `ViolationReflection` | Use of reflect package | Critical |
| `ViolationCGO` | CGO usage detected | Blocking |
| `ViolationExec` | os/exec without permission | Blocking |
| `ViolationPanic` | Panic call in code | Blocking |
| `ViolationGoroutineLeak` | Goroutine without context | Blocking |

**AST Fact Extraction:**

```go
// Emitted facts for Mangle policy evaluation
ast_import(File, Package)           // Import statements
ast_call(Function, Callee)          // Function calls
ast_goroutine_spawn(Target, Line)   // go statements
ast_uses_context_cancellation(Line) // Context usage check
ast_assignment(Var, Value)          // Variable assignments
```

**Default Allowed Packages:**

```go
base := []string{
    "bytes", "bufio", "context", "encoding/base64", "encoding/hex",
    "encoding/json", "errors", "fmt", "io", "log", "math", "regexp",
    "sort", "strconv", "strings", "sync", "sync/atomic", "time",
    "unicode", "unicode/utf8", "net/url",
}
// Conditionally added based on OuroborosConfig:
// - AllowFileSystem: "os", "path/filepath", "io/ioutil", "path"
// - AllowNetworking: "net", "net/http"
// - AllowExec: "os/exec"
```

### 6.6 Mangle State Machine (state.mg)

The state machine is defined in `internal/autopoiesis/state.mg`:

**Core Declarations:**

```mangle
# State tracking
Decl state(StepID, Stability, Loc).
Decl proposed(StepID).
Decl history(StepID, Hash).

# Iteration and retry tracking
Decl iteration(StepID, IterNum).
Decl max_iterations(Limit).
Decl retry_attempt(StepID, AttemptNum, Reason).
Decl max_retries(Limit).

# Stability penalties
Decl base_stability(StepID, Score).
Decl effective_stability(StepID, Score).
Decl cumulative_penalty(StepID, Total).
```

**Stability Penalty Rules:**

```mangle
# Panic penalty: -0.2
has_panic_penalty(StepID) :-
    error_history(StepID, /panic, _).

# Retry penalty (>=2 attempts): -0.1
has_retry_penalty(StepID) :-
    retry_attempt(StepID, N, _),
    N >= 2.

# Both penalties: -0.3
cumulative_penalty(StepID, 0.3) :-
    has_panic_penalty(StepID),
    has_retry_penalty(StepID).

# Effective stability = base - penalties
effective_stability(StepID, Effective) :-
    base_stability(StepID, Base),
    cumulative_penalty(StepID, Penalty),
    Effective = fn:minus(Base, Penalty).
```

**Transition Validation:**

```mangle
# Valid only if stability does not degrade
valid_transition(Next) :-
    state(Curr, _, _),
    proposed(Next),
    effective_stability(Curr, CurrEff),
    effective_stability(Next, NextEff),
    NextEff >= CurrEff.
```

**Halting Oracle (Stagnation Detection):**

```mangle
# Same code hash with different step IDs = stagnation
stagnation_detected() :-
    history(StepA, Hash),
    history(StepB, Hash),
    StepA != StepB.
```

**Termination Conditions:**

```mangle
should_halt(StepID) :- max_iterations_exceeded(StepID).
should_halt(StepID) :- max_retries_exceeded(StepID).
should_halt(StepID) :- stagnation_detected().
should_halt(StepID) :- stability_degrading(StepID).

# Convergence: stability stable for 2+ iterations
converged(StepID) :-
    state_at_iteration(StepID, N1, S1),
    state_at_iteration(StepID, N2, S2),
    N2 = fn:plus(N1, 1),
    S1 = S2,
    N2 >= 2.
```

### 6.7 DifferentialEngine (Incremental Evaluation)

The DifferentialEngine (`internal/mangle/differential.go`) optimizes Mangle evaluation for the Ouroboros simulation phase:

```go
type DifferentialEngine struct {
    baseEngine   *Engine
    programInfo  *analysis.ProgramInfo
    strataStores []*KnowledgeGraph  // Store per stratum
    predStratum  map[ast.PredicateSym]int
    strataRules  [][]ast.Clause
}
```

**Features:**

1. **Stratum-Aware Caching** - EDB facts (stratum 0) cached separately from IDB derivations (stratum 1+)

2. **Delta Propagation** - Only re-evaluate strata affected by new facts

3. **Snapshot Isolation (COW)** - Create isolated copies for simulation:
   ```go
   snapshot := diffEngine.Snapshot()  // Copy-on-write
   ```

4. **ChainedFactStore** - Query across strata layers:
   ```go
   type ChainedFactStore struct {
       base    []factstore.FactStore  // Lower strata (read-only)
       overlay factstore.FactStore    // Current stratum (writable)
   }
   ```

5. **Virtual Predicate Lazy Loading** - On-demand fact loading:
   ```go
   diffEngine.RegisterVirtualPredicate("file_content", func(path string) (string, error) {
       return os.ReadFile(path)
   })
   ```

**Usage in Ouroboros:**

```go
// Create differential engine from base
diffEngine, _ := mangle.NewDifferentialEngine(o.engine)

// Assert current state
diffEngine.AddFactIncremental(mangle.Fact{
    Predicate: "state",
    Args:      []interface{}{stepID, 0.0, 0},
})

// Assert proposed state
diffEngine.AddFactIncremental(mangle.Fact{
    Predicate: "proposed",
    Args:      []interface{}{nextStepID},
})

// Query transition validity
result, _ := diffEngine.Query(ctx, fmt.Sprintf("valid_transition(%s)", nextStepID))
if len(result.Bindings) == 0 {
    // Transition rejected - stability would degrade
}
```

### 6.8 Tool Compilation and Registration

**ToolCompiler** handles Go compilation:

```go
type ToolCompiler struct {
    config OuroborosConfig
}

type CompileResult struct {
    Success     bool
    OutputPath  string
    Hash        string  // SHA-256 of binary
    CompileTime time.Duration
    Errors      []string
}
```

**Compilation Process:**

1. Create temp directory with `go.mod`
2. If code is `package main`, use directly
3. Otherwise, wrap with main.go that calls entry point
4. Run `go build -ldflags "-s -w" -o <output>`
5. Calculate SHA-256 hash of binary

**RuntimeRegistry** manages tool lifecycle:

```go
type RuntimeRegistry struct {
    tools map[string]*RuntimeTool
}

type RuntimeTool struct {
    Name         string
    Description  string
    BinaryPath   string
    Hash         string
    Schema       ToolSchema
    RegisteredAt time.Time
    ExecuteCount int64
}

// Execute runs tool via stdin/stdout JSON protocol
func (rt *RuntimeTool) Execute(ctx context.Context, input string) (string, error)
```

### 6.9 Configuration

```go
type OuroborosConfig struct {
    ToolsDir        string        // ".nerd/tools"
    CompiledDir     string        // ".nerd/tools/.compiled"
    MaxToolSize     int64         // 100KB max
    CompileTimeout  time.Duration // 30s
    ExecuteTimeout  time.Duration // 60s
    AllowNetworking bool          // false by default
    AllowFileSystem bool          // true (read-only)
    AllowExec       bool          // true
    WorkspaceRoot   string        // Project root
}

type ExecuteConfig struct {
    Retry     RetryConfig  // MaxRetries: 3, RetryDelay: 100ms
    HotReload bool         // true by default
    MaxIters  int          // 10 max iterations
}
```

### 6.10 Safety Constraints

The Constitution governs tool generation:

```mangle
# Tool generation requires explicit capability
permitted(/generate_tool) :-
    current_mode(/development),
    !network_restricted_shard().

# Block in production without override
block_action(/generate_tool, "production_mode") :-
    current_mode(/production),
    !admin_override(/tool_generation).
```

---

## 7. Campaign Learning (Multi-Phase)

### 7.1 Phase Success Patterns

During campaign execution, the system learns from successful phases:

```mangle
# SECTION 19.9: Autopoiesis During Campaign

# Track successful phase types for learning (Go runtime extracts from kernel)
phase_success_pattern(PhaseType) :-
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, /completed, Profile),
    phase_objective(PhaseID, PhaseType, Desc, Priority),
    phase_checkpoint(PhaseID, CheckpointID, /true, ValidatedAt, ValidatorShard).

# Learn from phase completion - promotes success pattern for phase type
promote_to_long_term(/phase_success, PhaseType) :-
    phase_success_pattern(PhaseType).
```

### 7.2 Failure Pattern Learning

The system also learns from failures to avoid repeating mistakes:

```mangle
# Learn from task failures for future avoidance
campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, Now) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, _, _, /failed, TaskType),
    task_error(TaskID, _, ErrorMsg),
    current_time(Now).
```

### 7.3 Learning Persistence

When the Go runtime observes these derived atoms:

```go
// In campaign orchestrator
learnings := kernel.Query("campaign_learning(_, /failure_pattern, ?, ?, _)")
for _, l := range learnings {
    taskType := l.Args[0].(string)
    errorMsg := l.Args[1].(string)

    // Persist to learning store
    learningStore.Save("campaign", "avoid_error", []any{taskType, errorMsg}, campaignID)
}
```

---

## 8. Cognitive Rehydration

### 8.1 Session Startup

When a new session begins or a shard spawns, learnings are "rehydrated" into working memory:

```go
// In shard initialization
func (c *CoderShard) rehydrateLearnings() error {
    learnings, err := c.learningStore.Load("coder")
    if err != nil {
        return err
    }

    // Convert learnings to Mangle facts
    for _, l := range learnings {
        fact := core.Fact{
            Predicate: l.FactPredicate,
            Args:      l.FactArgs,
        }
        c.kernel.Assert(fact)
    }

    return nil
}
```

### 8.2 Influence on Behavior

Rehydrated learnings affect future derivations:

```mangle
# If we learned to avoid unwrap(), block code that uses it
code_style_violation(File, Line, "avoid_pattern_unwrap") :-
    avoid_pattern("unwrap"),
    code_contains(File, Line, ".unwrap()").

# Warn before generating code that violates learned preferences
clarification_needed("style_violation") :-
    code_style_violation(_, _, _).
```

---

## 9. Implementation Checklist

### 9.1 For New Shards

When implementing a new shard type:

```go
type MyShard struct {
    // Required for autopoiesis
    rejectionCount  map[string]int
    acceptanceCount map[string]int
    learningStore   *store.LearningStore
}

// Must implement:
func (s *MyShard) trackRejection(action, reason string)
func (s *MyShard) trackAcceptance(action string)
func (s *MyShard) rehydrateLearnings() error
func (s *MyShard) persistLearnings() error
```

### 9.2 Mangle Policy Extensions

For new learning types:

```mangle
# 1. Define the pattern detection
my_pattern(PatternKey) :-
    my_rejection_count(PatternKey, N),
    N >= 3.

# 2. Define the promotion rule
promote_to_long_term(/my_learning, PatternValue) :-
    my_pattern(PatternKey),
    derived_my_rule(PatternKey, PatternValue).

# 3. Define how learnings influence behavior
next_action(/my_modified_action) :-
    my_learning(Constraint),
    user_intent(_, _, Action, _, _),
    action_violates(Action, Constraint).
```

---

## 10. Monitoring and Debugging

### 10.1 Learning Statistics

```go
// Get stats for a shard
stats, err := learningStore.GetStats("coder")
// Returns:
// {
//   "total_learnings": 42,
//   "avg_confidence": 0.73,
//   "by_predicate": {
//     "avoid_pattern": 15,
//     "style_preference": 20,
//     "tool_preference": 7
//   }
// }
```

### 10.2 Mangle Queries

```mangle
# What patterns have been detected?
?- preference_signal(X).

# What should be promoted to long-term memory?
?- promote_to_long_term(Type, Value).

# What tools are missing?
?- missing_tool_for(Intent, Cap).

# What campaign learnings exist?
?- campaign_learning(CampaignID, Type, TaskType, Error, Time).
```

### 10.3 CLI Commands

```bash
# View learnings for a shard
nerd learnings list --shard coder

# Export learnings
nerd learnings export --shard coder --format json

# Import learnings (from another project)
nerd learnings import --shard coder --file learnings.json

# Decay old learnings manually
nerd learnings decay --shard coder --factor 0.9

# Clear all learnings for a shard
nerd learnings clear --shard coder --confirm
```

---

## 11. Security Considerations

### 11.1 Learning Injection

Malicious users could attempt to inject harmful learnings:

**Mitigation:**
- Learnings are validated against the Mangle schema before persistence
- Predicate names must be in an allowlist
- Arguments are type-checked

### 11.2 Runaway Learning

A bug could cause infinite learning loops:

**Mitigation:**
- Rate limiting on `Save()` operations (max 10 learnings per minute)
- Maximum learnings per shard (default: 1000)
- Confidence ceiling of 1.0 prevents infinite reinforcement

### 11.3 Tool Generation Safety

Self-generated tools could be malicious:

**Mitigation:**
- Static analysis before compilation
- Sandboxed execution (WASM or restricted Go)
- Require explicit approval for dangerous operations
- All generated tools logged for audit

---

## 12. Summary

Autopoiesis in codeNERD provides three levels of self-modification:

1. **Micro-Level (Preference Learning):** Individual shards learn user preferences through rejection/acceptance tracking. These learnings persist across sessions and decay over time if not reinforced.

2. **Meso-Level (Campaign Learning):** Multi-phase campaigns track success and failure patterns, allowing the system to avoid repeating mistakes across similar projects.

3. **Macro-Level (Tool Generation):** When capabilities are missing, the Ouroboros Loop allows the agent to write and bind new tools at runtime, expanding its own capability surface.

All learning is:
- **Deterministic** - Based on Mangle logic, not probabilistic inference
- **Auditable** - Stored in queryable SQLite databases
- **Decaying** - Unused learnings fade over time
- **Safe** - Governed by Constitutional constraints

This creates an agent that genuinely improves through use—not through retraining, but through structured self-observation.
