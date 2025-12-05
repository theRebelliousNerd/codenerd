# internal/autopoiesis - Self-Modification Capabilities

This package implements autopoiesis (self-creation) - the ability for codeNERD to modify itself by detecting needs and generating new capabilities.

## Architecture

Autopoiesis provides five core capabilities:
1. **Complexity Analysis** - Detect when campaigns are needed
2. **Tool Generation** - Create new tools when capabilities are missing
3. **Persistence Analysis** - Identify when persistent agents are needed
4. **Ouroboros Loop** - Full tool self-generation cycle with safety verification
5. **Feedback & Learning** - Evaluate tool quality and improve over time

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `autopoiesis.go` | ~600 | Main orchestrator |
| `complexity.go` | ~300 | Task complexity analysis |
| `toolgen.go` | ~600 | LLM-based tool generation |
| `persistence.go` | ~400 | Persistent agent detection |
| `ouroboros.go` | ~550 | Full Ouroboros Loop with safety/compile/execute |
| `feedback.go` | ~700 | Quality evaluation, pattern detection, learning |

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

## Key Types

### Orchestrator
```go
type Orchestrator struct {
    complexity  *ComplexityAnalyzer
    toolGen     *ToolGenerator
    persistence *PersistenceAnalyzer
    ouroboros   *OuroborosLoop
    evaluator   *QualityEvaluator   // NEW
    patterns    *PatternDetector    // NEW
    refiner     *ToolRefiner        // NEW
    learnings   *LearningStore      // NEW
}

// Generation
func (o *Orchestrator) ExecuteOuroborosLoop(ctx, need) *LoopResult
func (o *Orchestrator) ExecuteGeneratedTool(ctx, name, input) (string, error)

// Feedback (NEW)
func (o *Orchestrator) RecordExecution(ctx, feedback)
func (o *Orchestrator) EvaluateToolQuality(ctx, feedback) *QualityAssessment
func (o *Orchestrator) GetToolPatterns(toolName) []*DetectedPattern
func (o *Orchestrator) ShouldRefineTool(toolName) (bool, []ImprovementSuggestion)
func (o *Orchestrator) RefineTool(ctx, toolName, code) (*RefinementResult, error)
func (o *Orchestrator) ExecuteAndEvaluate(ctx, name, input) (string, *QualityAssessment, error)
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
│   └── .learnings/
│       └── tool_learnings.json # Persisted learnings
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

## Testing

```bash
go test ./internal/autopoiesis/...
```
