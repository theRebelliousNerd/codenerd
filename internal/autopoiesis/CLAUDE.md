# internal/autopoiesis - Self-Modification Capabilities

This package implements autopoiesis (self-creation) - the ability for codeNERD to modify itself by detecting needs and generating new capabilities.

**Subpackage:** See [prompt_evolution/CLAUDE.md](prompt_evolution/CLAUDE.md) for the System Prompt Learning (SPL) subsystem.

## Architecture

Autopoiesis provides ten core capabilities:

1. **Complexity Analysis** - Detect when campaigns are needed
2. **Tool Generation** - Create new tools when capabilities are missing
3. **Persistence Analysis** - Identify when persistent agents are needed
4. **Ouroboros Loop** - Full tool self-generation cycle with safety verification
5. **Feedback & Learning** - Evaluate tool quality and improve over time
6. **Reasoning Traces** - Capture LLM reasoning for optimization and debugging
7. **Tool Quality Profiles** - LLM-defined per-tool performance expectations
8. **Thunderdome** - Adversarial testing arena for attack vector validation
9. **Chaos Engineering** - Generate attack vectors to test tool robustness
10. **Prompt Evolution** - System Prompt Learning (SPL) for automatic prompt improvement

## File Index

| File | Description |
|------|-------------|
| `autopoiesis.go` | Package marker documenting orchestrator modularization across 10 files. Points to autopoiesis_types, orchestrator, kernel, delegation, agents, analysis, tools, feedback, profiles, helpers. |
| `autopoiesis_agents.go` | Agent creation and management for persistent Type 3 agents. Exports writeAgentSpec() and ListAgents() for agent storage in .nerd/agents. |
| `autopoiesis_analysis.go` | Main analysis pipeline combining complexity, persistence, and tool need detection. Implements Analyze() orchestrating complexity→persistence→tool detection flow. |
| `autopoiesis_delegation.go` | Kernel-mediated tool generation via Mangle delegate_task facts. Implements ProcessKernelDelegations() for campaign-triggered tool creation without direct coupling. |
| `autopoiesis_feedback.go` | Feedback and learning wrappers for tool execution evaluation. Exports RecordExecution(), EvaluateToolQuality(), GetToolPatterns(), ShouldRefineTool(), RefineTool(). |
| `autopoiesis_helpers.go` | Utility functions for tool generation gating and priority sorting. Implements shouldGenerateToolNeed() with confidence, cooldown, and evidence gates. |
| `autopoiesis_kernel.go` | Bridge to Mangle kernel for fact assertion and query. Implements SetKernel(), syncExistingToolsToKernel(), and assertToolRegistered() for neuro-symbolic integration. |
| `autopoiesis_orchestrator.go` | Main Orchestrator struct coordinating all autopoiesis capabilities. Exports NewOrchestrator() and DefaultConfig() with complexity, tool generation, learning, and trace subsystems. |
| `autopoiesis_profiles.go` | Quality profile wrappers for tool-specific performance expectations. Exports GetToolProfile(), SetToolProfile(), EvaluateWithProfile(), GenerateToolProfile(). |
| `autopoiesis_tools.go` | Tool generation wrappers exposing ToolGenerator and OuroborosLoop. Implements DetectToolNeed(), GenerateTool(), ExecuteOuroborosLoop() with kernel integration. |
| `autopoiesis_types.go` | Core type definitions including Config, AnalysisResult, AutopoiesisAction, CampaignPayload. Exports ActionType constants and AgentSpec, AgentMemory structures. |
| `checker.go` | SafetyChecker validating generated code against go_safety.mg policy. Exports Check() detecting forbidden imports, dangerous calls, and safety violations. |
| `checker_test.go` | Unit tests for SafetyChecker validation of forbidden imports and dangerous patterns. Verifies safety policy enforcement on generated Go code. |
| `complexity.go` | ComplexityAnalyzer determining when campaigns are needed for complex tasks. Exports Analyze() and AnalyzeWithLLM() with EstimatedFiles, SuggestedPhases output. |
| `complexity_test.go` | Unit tests for ComplexityAnalyzer heuristic and LLM-based analysis. Tests campaign threshold detection and phase suggestion accuracy. |
| `feedback.go` | Core feedback tracking with QualityEvaluator, ToolRefiner, and LearningStore. Implements tool execution evaluation, pattern-based refinement, and cross-session persistence. |
| `feedback_test.go` | Unit tests for feedback loop including quality evaluation and learning store. Verifies refinement trigger detection and learning persistence. |
| `ouroboros.go` | Transactional state machine for tool self-generation with six stages. Exports OuroborosLoop with Proposal→Audit→Simulation→Compile→Register→Execute cycle. |
| `ouroboros_test.go` | Unit tests for OuroborosLoop stage transitions and error handling. Tests compilation, registration, and execution workflows. |
| `panic_maker.go` | PanicMaker generating adversarial attack vectors for chaos testing. Exports Generate() creating nil_pointer, boundary, resource, concurrency, and format attacks. |
| `patterns.go` | PatternDetector for recurring issue identification across tool executions. Exports RecordExecution(), GetToolPatterns(), and DetectedPattern with confidence scoring. |
| `persistence.go` | PersistenceAnalyzer detecting when Type 3 persistent agents are needed. Implements Analyze() and AnalyzeWithLLM() for domain expertise and knowledge persistence needs. |
| `profiles.go` | Tool quality profile types with LLM-defined performance expectations. Exports ToolQualityProfile, ToolType constants, PerformanceExpectations, and ProfileStore. |
| `quality.go` | QualityAssessment with Completeness, Accuracy, Efficiency, and Relevance dimensions. Exports QualityEvaluator with Evaluate() and EvaluateWithLLM() methods. |
| `quality_test.go` | Unit tests for QualityEvaluator scoring and profile-aware evaluation. Verifies quality dimension calculations and issue detection. |
| `thunderdome.go` | Adversarial testing arena where tools fight attack vectors in sandboxes. Exports Thunderdome with Battle() executing parallel attacks and BattleResult survival status. |
| `tool_detection.go` | ToolNeed detection from user input patterns and LLM refinement. Implements DetectToolNeed() with missingCapabilityPatterns and toolTypePatterns matching. |
| `tool_generation.go` | GeneratedTool struct and tool code generation workflow. Implements GenerateTool() with code generation, test generation, schema creation, and validation. |
| `tool_templates.go` | Template-based tool generation for common patterns (validator, converter, parser). Exports ToolTemplate and toolTemplates map for boilerplate generation. |
| `tool_validation.go` | AST-based validation of generated tool code for syntax and safety. Exports validateCode() and validateCodeAST() checking imports and dangerous patterns. |
| `toolgen.go` | ToolGenerator for LLM-based tool code creation and management. Exports GenerateTool(), HasTool(), WriteTool(), and RegisterTool() for tool lifecycle. |
| `toolgen_test.go` | Unit tests for ToolGenerator including code generation and registration. Tests validation, template selection, and tool persistence. |
| `traces.go` | ReasoningTrace capturing LLM thought process during tool generation. Exports TraceCollector with chain-of-thought, key decisions, and generation audit analysis. |
| `yaegi_executor.go` | Yaegi interpreter for sandboxed Go code execution without compilation. Exports YaegiExecutor with ExecuteToolCode() avoiding dependency hell and build hangs. |

## The Ouroboros Loop

Named after the ancient symbol of a serpent eating its own tail, the Ouroboros Loop enables codeNERD to generate new tools at runtime.

### Loop Stages
```
Detection → Specification → Safety Check → Compile → Register → Execute
    ↑                                                              |
    └────────── Evaluate → Detect Patterns → Refine ───────────────┘
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

## Feedback & Learning System

The feedback system closes the autopoiesis loop by learning from tool executions.

### Learning Loop
```
Execute Tool → Evaluate Quality → Detect Patterns → Refine Tool
      ↑                                                  |
      └────────────────────────────────────────────────→┘
```

### Quality Dimensions

| Dimension | Description |
|-----------|-------------|
| **Completeness** | Did we get ALL available data? (pagination, limits) |
| **Accuracy** | Was the output correct and well-formed? |
| **Efficiency** | Resource usage and execution time |
| **Relevance** | Was output relevant to the user's intent? |

### Issue Types

| Issue | Description |
|-------|-------------|
| `incomplete` | Missing data (only fetched partial results) |
| `pagination` | Didn't handle pagination (only got first page) |
| `rate_limit` | Hit API rate limits |
| `slow` | Execution took too long |
| `partial_failure` | Partially worked but had errors |

### Improvement Suggestions

| Suggestion | When Applied |
|------------|--------------|
| `add_pagination` | Tool only fetched first page |
| `increase_limit` | Tool used default limit instead of max |
| `add_retry` | Tool failed on transient errors |
| `add_caching` | Same data fetched repeatedly |
| `parallelize` | Independent requests made sequentially |

## Tool Quality Profiles

Each tool has different performance expectations. A background indexer taking 5 minutes is fine; a calculator taking 5 minutes is broken. The LLM defines these expectations during tool generation.

### Tool Types

| Type | Description | Duration Expectation |
|------|-------------|---------------------|
| `quick_calculation` | Simple computation | < 1 second |
| `data_fetch` | API call, may paginate | 5-30 seconds |
| `background_task` | Long-running process | Minutes OK |
| `recursive_analysis` | Codebase traversal | Minutes OK |
| `realtime_query` | Fast, frequent calls | < 500ms |
| `one_time_setup` | Run once | Can be slow |
| `batch_processor` | Many items | Scales with input |
| `monitor` | Status checks | < 2 seconds |

### ToolQualityProfile
```go
type ToolQualityProfile struct {
    ToolName    string              `json:"tool_name"`
    ToolType    ToolType            `json:"tool_type"`
    Description string              `json:"description"`
    Performance PerformanceExpectations `json:"performance"`
    Output      OutputExpectations  `json:"output"`
    UsagePattern UsagePattern       `json:"usage_pattern"`
    Caching     CachingConfig       `json:"caching"`
    CustomDimensions []CustomDimension `json:"custom_dimensions"`
}

type PerformanceExpectations struct {
    ExpectedDurationMin time.Duration  // Faster = suspicious
    ExpectedDurationMax time.Duration  // Slower = problem
    AcceptableDuration  time.Duration  // Target
    TimeoutDuration     time.Duration  // Give up
    MaxRetries          int
    ScalesWithInputSize bool
}

type OutputExpectations struct {
    ExpectedMinSize     int       // Smaller = suspicious
    ExpectedMaxSize     int       // Larger = issue
    ExpectedTypicalSize int       // Normal size
    ExpectedFormat      string    // json, text, csv
    ExpectsPagination   bool
    RequiredFields      []string  // Must be in output
    MustContain         []string  // Must appear
    MustNotContain      []string  // Error indicators
}
```

### Profile-Aware Evaluation

```go
// Generate tool with profile
tool, profile, trace, _ := orchestrator.GenerateToolWithProfile(ctx, need, "user request")

// Execute with profile-aware evaluation
output, quality, _ := orchestrator.ExecuteAndEvaluateWithProfile(ctx, toolName, input)

// Quality assessment uses profile expectations:
// - Duration compared against profile.Performance
// - Output size compared against profile.Output
// - Custom dimensions extracted and scored
```

### Example: Calculator vs Background Indexer

**Calculator Tool** (quick_calculation):
- AcceptableDuration: 100ms
- ExpectedDurationMax: 1s
- 5-minute execution → Quality score: 0.1 (broken!)

**Indexer Tool** (background_task):
- AcceptableDuration: 2m
- ExpectedDurationMax: 10m
- 5-minute execution → Quality score: 0.8 (acceptable)

## Key Types

### Orchestrator
```go
type Orchestrator struct {
    complexity  *ComplexityAnalyzer
    toolGen     *ToolGenerator
    persistence *PersistenceAnalyzer
    ouroboros   *OuroborosLoop
    evaluator   *QualityEvaluator
    patterns    *PatternDetector
    refiner     *ToolRefiner
    learnings   *LearningStore
    profiles    *ProfileStore       // Tool quality profiles
    traces      *TraceCollector
    logInjector *LogInjector
}

// Generation
func (o *Orchestrator) ExecuteOuroborosLoop(ctx, need) *LoopResult
func (o *Orchestrator) ExecuteGeneratedTool(ctx, name, input) (string, error)

// Feedback & Evaluation
func (o *Orchestrator) RecordExecution(ctx, feedback)
func (o *Orchestrator) EvaluateToolQuality(ctx, feedback) *QualityAssessment
func (o *Orchestrator) GetToolPatterns(toolName) []*DetectedPattern
func (o *Orchestrator) ShouldRefineTool(toolName) (bool, []ImprovementSuggestion)
func (o *Orchestrator) RefineTool(ctx, toolName, code) (*RefinementResult, error)
func (o *Orchestrator) ExecuteAndEvaluate(ctx, name, input) (string, *QualityAssessment, error)

// Quality Profiles
func (o *Orchestrator) GetToolProfile(toolName) *ToolQualityProfile
func (o *Orchestrator) SetToolProfile(profile *ToolQualityProfile)
func (o *Orchestrator) GenerateToolProfile(ctx, name, desc, code) (*ToolQualityProfile, error)
func (o *Orchestrator) EvaluateWithProfile(ctx, feedback) *QualityAssessment
func (o *Orchestrator) ExecuteAndEvaluateWithProfile(ctx, name, input) (string, *QualityAssessment, error)
func (o *Orchestrator) GenerateToolWithProfile(ctx, need, req) (*GeneratedTool, *ToolQualityProfile, *ReasoningTrace, error)
```

### QualityAssessment
```go
type QualityAssessment struct {
    Score         float64         // 0.0 - 1.0
    Completeness  float64         // Did we get all data?
    Accuracy      float64         // Was output correct?
    Efficiency    float64         // Resource efficiency
    Relevance     float64         // Relevant to intent?
    Issues        []QualityIssue
    Suggestions   []ImprovementSuggestion
}
```

### DetectedPattern
```go
type DetectedPattern struct {
    ToolName    string
    IssueType   IssueType
    Occurrences int
    Confidence  float64       // 3+ occurrences = 0.7+
    Suggestions []ImprovementSuggestion
}
```

### ToolLearning
```go
type ToolLearning struct {
    ToolName        string
    TotalExecutions int
    SuccessRate     float64
    AverageQuality  float64
    KnownIssues     []IssueType
    AppliedFixes    []string
}
```

## Mangle Integration

### Tool Generation (Section 12B)
```datalog
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

next_action(/generate_tool) :-
    missing_tool_for(_, _).
```

### Tool Learning (Section 12C)
```datalog
# Quality tracking
tool_quality_poor(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality < 0.5.

# Trigger refinement
tool_needs_refinement(ToolName) :-
    tool_quality_poor(ToolName).

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /pagination),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

next_action(/refine_tool) :-
    tool_needs_refinement(_),
    !active_refinement(_).

# Learning pattern signals
learning_pattern_detected(ToolName, IssueType) :-
    tool_known_issue(ToolName, IssueType),
    issue_occurrence_count(ToolName, IssueType, Count),
    Count >= 3.

# Promote learnings to hints for future tools
tool_generation_hint(Capability, "add_pagination") :-
    learning_pattern_detected(_, /pagination).
```

## Example: Context7 API Tool Learning

Scenario: A tool fetches docs but only gets 10% of available data.

1. **First Execution**
   - Tool returns ~1KB of docs
   - QualityEvaluator detects: output size < expected minimum
   - Issue: `IssueIncomplete` with severity 0.7
   - Suggestion: `SuggestIncreaseLimit`

2. **Second Execution**
   - Same issue detected
   - PatternDetector: 2 occurrences, confidence 0.5

3. **Third Execution**
   - PatternDetector: 3 occurrences, confidence 0.7
   - Mangle derives: `tool_needs_refinement("context7_docs")`
   - ToolRefiner generates improved version with pagination

4. **After Refinement**
   - New tool fetches all 10 pages
   - Output size: ~10KB
   - Quality score: 0.9 (was 0.3)
   - LearningStore records: "add_pagination" fixed the issue

5. **Future Tool Generation**
   - When generating similar API tools
   - Mangle derives: `tool_generation_hint(_, "add_pagination")`
   - New tools include pagination by default

## Directory Structure

```
.nerd/
├── tools/
│   ├── context7_docs.go        # Generated source
│   ├── context7_docs_test.go   # Generated tests
│   ├── .compiled/
│   │   └── context7_docs       # Compiled binary
│   ├── .learnings/
│   │   └── tool_learnings.json # Persisted learnings
│   └── .profiles/
│       └── quality_profiles.json # Tool quality profiles
```

## Safety Features

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

## Reasoning Traces

The trace system captures the "why" behind tool generation for optimization and debugging.

### What's Captured

| Component | Description |
|-----------|-------------|
| **SystemPrompt** | System prompt sent to LLM |
| **UserPrompt** | Full user prompt |
| **RawResponse** | Complete LLM response |
| **ChainOfThought** | Extracted reasoning steps |
| **KeyDecisions** | Major choices and why |
| **Assumptions** | Assumptions the LLM made |
| **Alternatives** | Options considered but rejected |

### ReasoningTrace
```go
type ReasoningTrace struct {
    TraceID        string
    ToolName       string
    UserRequest    string
    DetectedNeed   *ToolNeed
    SystemPrompt   string
    UserPrompt     string
    RawResponse    string
    ChainOfThought []ThoughtStep
    KeyDecisions   []Decision
    Assumptions    []string
    QualityScore   float64  // Filled after execution feedback
}
```

### Generation Audit

Analyze patterns across ALL tool generations:

```go
audit, _ := orchestrator.AnalyzeGenerations(ctx)

// Summary statistics
audit.TotalGenerations    // 50
audit.SuccessRate         // 0.85
audit.AverageQuality      // 0.72

// Common patterns
audit.CommonDecisions     // [{Topic: "pagination", Choice: "cursor-based", SuccessRate: 0.9}]
audit.CommonIssues        // [{Issue: "incomplete", Occurrences: 12, CommonCauses: [...]}]

// Optimization opportunities
audit.Optimizations       // [{Area: "issue_prevention", Suggestion: "Add pagination by default"}]
```

## Mandatory Logging Injection

All generated tools MUST have verbose logging for learning. Logging is automatically injected before compilation.

### Required Log Points

| Log Type | Tag | When |
|----------|-----|------|
| Entry | `[TOOL_ENTRY]` | Function start |
| Exit | `[TOOL_EXIT]` | Function end (via defer) |
| Error | `[TOOL_ERROR]` | Every error |
| Timing | `[TOOL_TIMING]` | Execution duration |
| API Call | `[TOOL_API_CALL]` | External API requests |
| Iteration | `[TOOL_ITERATION]` | Loop execution counts |

### LoggingRequirements
```go
type LoggingRequirements struct {
    RequireEntryLog     bool  // Log on function entry
    RequireExitLog      bool  // Log on function exit
    RequireErrorLog     bool  // Log all errors
    RequireInputLog     bool  // Log input parameters
    RequireOutputLog    bool  // Log output/return values
    RequireTimingLog    bool  // Log execution duration
    RequireDecisionLog  bool  // Log key decisions
    RequireAPICallLog   bool  // Log external API calls
    RequireIterationLog bool  // Log loop iterations
}
```

### Example Injected Logging

```go
// Original code:
func fetchDocs(input string) (string, error) {
    resp, err := http.Get(url)
    if err != nil {
        return "", err
    }
    // ...
}

// After injection:
func fetchDocs(input string) (string, error) {
    log.Printf("[TOOL_ENTRY] fetchDocs: starting execution, input=%q", input)
    defer log.Printf("[TOOL_EXIT] fetchDocs: execution complete")
    _toolStartTime := time.Now()
    defer func() { log.Printf("[TOOL_TIMING] fetchDocs: duration=%v", time.Since(_toolStartTime)) }()

    log.Printf("[TOOL_API_CALL] fetchDocs: making request")
    resp, err := http.Get(url)
    if err != nil {
        log.Printf("[TOOL_ERROR] fetchDocs: %v", err)
        return "", err
    }
    // ...
}
```

### Using Traces for Optimization

```go
// Generate with full tracing
tool, trace, _ := orchestrator.GenerateToolWithTracing(ctx, need, "fetch context7 docs")

// Execute and evaluate
output, quality, _ := orchestrator.ExecuteAndEvaluate(ctx, tool.Name, input)

// Update trace with feedback
orchestrator.UpdateTraceWithFeedback(tool.Name, quality.Score, []string{"pagination"}, nil)

// Later: analyze all generations
audit, _ := orchestrator.AnalyzeGenerations(ctx)
// audit.Optimizations contains suggestions like:
// "Issue 'pagination' occurred 12 times. Add pagination handling by default."
```

## Directory Structure

```
.nerd/
├── tools/
│   ├── context7_docs.go        # Generated source (with logging)
│   ├── context7_docs_test.go   # Generated tests
│   ├── .compiled/
│   │   └── context7_docs       # Compiled binary
│   ├── .learnings/
│   │   └── tool_learnings.json # Persisted learnings
│   ├── .profiles/
│   │   └── quality_profiles.json # Tool quality profiles
│   └── .traces/
│       └── reasoning_traces.json # Reasoning traces
```

## Thunderdome - Adversarial Testing Arena

The Thunderdome is where tools fight for survival. Attack vectors are run against compiled tools in isolated sandboxes.

### Battle Flow

```
Tool Generated → Thunderdome.Battle() → Attack Vectors Run → BattleResult
                        |
                        v
              Parallel Attack Execution:
              ├── Memory exhaustion attacks
              ├── Panic injection attacks
              ├── Race condition triggers
              ├── Resource leak tests
              └── Malformed input fuzzing
                        |
                        v
              Tool Survives? → Register in runtime
              Tool Fails? → Back to Ouroboros for refinement
```

### Attack Vector Types

| Category | Attack | Description |
|----------|--------|-------------|
| `memory` | `memory_exhaustion` | Allocate unbounded memory |
| `panic` | `nil_deref`, `bounds` | Trigger nil pointer / out of bounds |
| `concurrency` | `race_condition` | Concurrent access without sync |
| `resource` | `file_leak`, `goroutine_leak` | Unclosed resources |
| `logic` | `malformed_input` | Invalid/malicious input data |

### ThunderdomeConfig

```go
type ThunderdomeConfig struct {
    Timeout         time.Duration // Max time per attack
    MaxMemoryMB     int           // Memory limit
    WorkDir         string        // Temp directory
    KeepArtifacts   bool          // Keep logs for debugging
    ParallelAttacks int           // Concurrent attack count
}
```

### BattleResult

```go
type BattleResult struct {
    ToolName     string
    Survived     bool           // Tool passed all attacks
    TotalAttacks int
    Failures     int
    Results      []AttackResult
    Duration     time.Duration
    FatalAttack  *AttackVector  // Attack that killed the tool
}
```

## Testing

```bash
go test ./internal/autopoiesis/...
```

---

**Remember: Push to GitHub regularly!**
