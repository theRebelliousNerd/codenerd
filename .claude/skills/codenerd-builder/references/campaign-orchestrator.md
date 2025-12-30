# Campaign Orchestrator: Multi-Phase Goal Execution

**Architectural Standards for Sustained Context Management in Long-Running Agent Tasks**

Version: 1.5.0

## 1. The Problem: Context Window Collapse

Traditional AI agents fail catastrophically on complex, multi-day projects. The fundamental issue is **Context Window Collapse**:

### 1.1 The Goldfish Problem

LLMs are stateless. After each response, the "memory" is merely the chat history stuffed back into the prompt. For a 50-task project spanning multiple days:

- **Day 1**: Agent remembers everything perfectly
- **Day 2**: Context window 70% full, starts forgetting early decisions
- **Day 3**: Critical architectural decisions lost to truncation
- **Day 5**: Agent contradicts its own earlier work

### 1.2 The Coherence Cascade

Without sustained context management:

1. Agent forgets design decision from Phase 1
2. Implements incompatible approach in Phase 5
3. Integration fails
4. Agent "fixes" by undoing Phase 1 work
5. This breaks Phases 2-4
6. **Cascade failure**

### 1.3 The Solution: Campaign Orchestrator

The Campaign Orchestrator solves this by treating long-running tasks as **first-class citizens** with:

- **Phase-aware context paging**: Only load what's needed for current phase
- **Semantic compression**: Completed work → distilled summaries
- **Mangle-driven sequencing**: Logic determines task order, not the LLM
- **Checkpoint verification**: Each phase is validated before proceeding
- **Adaptive replanning**: Automatic recovery from cascading failures

## 2. Campaign Architecture

### 2.1 The Campaign Hierarchy

```
Campaign
├── Type: /greenfield | /feature | /audit | /migration | /remediation | /adversarial_assault
├── Goal: High-level objective
├── SourceMaterial: [spec.md, requirements.txt, ...]
├── ContextBudget: 100000 tokens
│
├── Phase 0: "Core Foundation"
│   ├── Status: /completed
│   ├── ContextProfile: profile_0
│   ├── CompressedSummary: "Created pkg/core with Kernel interface..."
│   ├── Objectives: [{type: /create, verify: /builds}]
│   └── Tasks:
│       ├── Task 0.0: "Create kernel.go" [/completed]
│       ├── Task 0.1: "Create factstore.go" [/completed]
│       └── Task 0.2: "Write unit tests" [/completed]
│
├── Phase 1: "Perception Layer" [/in_progress]
│   ├── ContextProfile: profile_1 (focus: internal/perception/*)
│   ├── Dependencies: [Phase 0]
│   └── Tasks:
│       ├── Task 1.0: "Create transducer.go" [/completed]
│       ├── Task 1.1: "Implement LLM client" [/in_progress]  ← Current
│       └── Task 1.2: "Add structured output" [/pending]
│
├── Phase 2: "Articulation Layer" [/pending]
│   ├── Dependencies: [Phase 0, Phase 1]
│   └── Tasks: [...]
│
└── Learnings: [{pattern: "prefer_table_tests", applied: true}]
```

### 2.2 Campaign Types

| Type | Purpose | Typical Duration | Context Strategy |
|------|---------|------------------|------------------|
| `/greenfield` | Build from scratch | Days to weeks | Heavy prefetch, broad focus |
| `/feature` | Add major feature | Hours to days | Narrow focus, dependency tracking |
| `/audit` | Stability/security review | Hours | Read-heavy, minimal writes |
| `/migration` | Technology migration | Days | Pattern matching, batch changes |
| `/remediation` | Fix issues across codebase | Hours to days | Issue-driven, incremental |
| `/adversarial_assault` | Soak/stress + adversarial sweep | Days to weeks | Deterministic batching, resumable artifacts |

Adversarial assault campaigns persist additional run artifacts under `.nerd/campaigns/<campaign>/assault/` (targets, batches, logs, results, triage) to support long-horizon triage and remediation.

## 3. The Orchestrator Loop

The orchestrator implements a modified OODA loop optimized for multi-phase execution:

```
┌─────────────────────────────────────────────────────────────────┐
│                    CAMPAIGN EXECUTION LOOP                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. QUERY MANGLE                                                  │
│    - current_phase(PhaseID)                                      │
│    - next_campaign_task(TaskID)                                  │
│    - campaign_blocked(CampaignID, Reason)                        │
│    - replan_needed(CampaignID, Reason)                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. PAGE CONTEXT                                                  │
│    - ActivatePhase(phase) → boost activation for focus patterns │
│    - PrefetchNextTasks(limit=3) → warm cache for upcoming work  │
│    - PruneIrrelevant(profile) → suppress unneeded schemas       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. EXECUTE TASK                                                  │
│    Based on task.Type:                                           │
│    - /file_create, /file_modify → delegate to CoderShard        │
│    - /test_write, /test_run → delegate to TesterShard           │
│    - /research → delegate to ResearcherShard                     │
│    - /verify → run build/lint checks                             │
│    - /refactor, /integrate → delegate to CoderShard             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. HANDLE RESULT                                                 │
│    Success:                                                      │
│    - completeTask(task, result)                                  │
│    - applyLearnings(task, result) → autopoiesis                  │
│    - emitProgress()                                              │
│                                                                  │
│    Failure:                                                      │
│    - handleTaskFailure(phase, task, err)                         │
│    - recordAttempt(task, err)                                    │
│    - if attempts >= 3: markTaskFailed()                          │
│    - if replan_needed(): triggerReplan()                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. PHASE TRANSITION                                              │
│    When all tasks complete:                                      │
│    - runPhaseCheckpoint(phase)                                   │
│    - CompressPhase(phase) → distill to summary                   │
│    - completePhase(phase)                                        │
│    - startNextPhase() if eligible                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. CAMPAIGN COMPLETE?                                            │
│    - All phases /completed or /skipped → SUCCESS                 │
│    - campaign_blocked derived → FAILURE                          │
│    - Context cancelled → PAUSED                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 4. Context Paging System

The Context Pager solves the "Goldfish Problem" by treating context as a scarce resource.

### 4.1 Budget Allocation

```go
totalBudget:     100000  // 100k tokens total
coreReserve:       5000  // 5% - Campaign identity, rules, current state
phaseReserve:     30000  // 30% - Current phase files, dependencies
historyReserve:   15000  // 15% - Compressed summaries of past phases
workingReserve:   40000  // 40% - Active task execution context
prefetchReserve:  10000  // 10% - Hints for upcoming 3 tasks
```

### 4.2 Phase Activation

When a new phase starts:

```go
func (cp *ContextPager) ActivatePhase(ctx context.Context, phase *Phase) error {
    // 1. Get context profile for this phase
    profile := cp.getContextProfile(phase.ContextProfile)

    // 2. Boost activation for phase-specific files
    for _, pattern := range profile.FocusPatterns {
        cp.boostPattern(pattern, 120)  // High boost
    }

    // 3. Load task artifacts into context
    for _, task := range phase.Tasks {
        for _, artifact := range task.Artifacts {
            cp.kernel.Assert(Fact{
                Predicate: "phase_context_atom",
                Args:      []interface{}{phase.ID, artifact.Path, 100},
            })
        }
    }

    // 4. Suppress irrelevant schemas (negative activation)
    irrelevantSchemas := []string{"dom_node", "geometry", "vector_recall"}
    for _, schema := range irrelevantSchemas {
        if !contains(profile.RequiredSchemas, schema) {
            cp.suppressSchema(schema)  // -100 activation
        }
    }
}
```

### 4.3 Phase Compression

When a phase completes, we compress its context:

```go
func (cp *ContextPager) CompressPhase(ctx context.Context, phase *Phase) error {
    // 1. Gather all facts from this phase
    phaseFacts := cp.kernel.Query("phase_context_atom")

    // 2. Build accomplishments list
    var accomplishments []string
    for _, task := range phase.Tasks {
        if task.Status == TaskCompleted {
            accomplishments = append(accomplishments, task.Description)
            for _, artifact := range task.Artifacts {
                accomplishments = append(accomplishments, "→ Created: " + artifact.Path)
            }
        }
    }

    // 3. LLM generates concise summary (max 100 words)
    summary := cp.llmClient.Summarize(accomplishments)

    // 4. Store compression fact
    cp.kernel.Assert(Fact{
        Predicate: "context_compression",
        Args:      []interface{}{phase.ID, summary, len(phaseFacts), time.Now().Unix()},
    })

    // 5. Reduce activation of phase-specific facts
    for _, fact := range phaseFacts {
        cp.kernel.Assert(Fact{
            Predicate: "activation",
            Args:      []interface{}{fact, -100},  // Heavy suppression
        })
    }
}
```

**Compression Example:**

Before (500 tokens):
```
Phase 1 completed. Task 1.0 created internal/core/kernel.go with RealKernel struct
implementing Kernel interface with LoadFacts, Query, Assert, Retract methods. Uses
google/mangle/factstore for storage. Task 1.1 created internal/core/virtual_store.go
with VirtualStore struct implementing FFI routing for mcp_tool_result, file_content,
shell_exec_result predicates. Task 1.2 wrote 15 unit tests covering 87% of kernel...
```

After (50 tokens):
```
Phase 1: Created Mangle kernel with EDB/IDB separation, virtual store for FFI routing,
87% test coverage.
```

### 4.4 Context Profile Schema

```mangle
Decl context_profile(
    ID.Type<string>,
    RequiredSchemas.Type<string>,  # Comma-separated: "file_topology,symbol_graph"
    RequiredTools.Type<string>,    # Comma-separated: "fs_read,fs_write,exec_cmd"
    FocusPatterns.Type<string>     # Comma-separated: "internal/core/*,pkg/**/*.go"
).
```

## 5. Mangle Policy: Campaign Orchestration Rules

The campaign logic lives in [internal/mangle/policy.gl](internal/mangle/policy.gl) Section 19.

### 5.1 Phase Eligibility

A phase can only start when all hard dependencies are complete:

```mangle
# Helper: check if a phase has incomplete hard dependencies
has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(DepPhaseID, _, _, _, Status, _),
    Status != /completed.

# A phase is eligible when all hard dependencies are complete
phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).
```

### 5.2 Task Selection

Tasks are selected by priority with dependency awareness:

```mangle
# Priority ordering
priority_higher(/critical, /high).
priority_higher(/critical, /normal).
priority_higher(/critical, /low).
priority_higher(/high, /normal).
priority_higher(/high, /low).
priority_higher(/normal, /low).

# Helper: check if task has blocking dependencies
has_blocking_task_dep(TaskID) :-
    task_dependency(TaskID, BlockerID),
    campaign_task(BlockerID, _, _, Status, _),
    Status != /completed,
    Status != /skipped.

# Next task: highest priority pending task without blockers
next_campaign_task(TaskID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    !has_blocking_task_dep(TaskID).
```

### 5.3 Shard Delegation

Tasks are automatically delegated to specialized shards:

```mangle
# Auto-spawn researcher shard for research tasks
delegate_task(/researcher, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /research).

# Auto-spawn coder shard for file creation/modification
delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_create).

delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_modify).

# Auto-spawn tester shard for test tasks
delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_write).
```

### 5.4 Checkpoint Verification

Phases must pass checkpoints before completion:

```mangle
# Helper: check if phase has pending checkpoint
has_pending_checkpoint(PhaseID) :-
    phase_objective(PhaseID, _, _, VerifyMethod),
    VerifyMethod != /none,
    !has_passed_checkpoint(PhaseID, VerifyMethod).

has_passed_checkpoint(PhaseID, CheckType) :-
    phase_checkpoint(PhaseID, CheckType, /true, _, _).

# Trigger checkpoint when all tasks complete but checkpoint pending
next_action(/run_phase_checkpoint) :-
    current_phase(PhaseID),
    all_phase_tasks_complete(PhaseID),
    has_pending_checkpoint(PhaseID).

# Block phase completion if checkpoint failed
phase_blocked(PhaseID, "checkpoint_failed") :-
    phase_checkpoint(PhaseID, _, /false, _, _).
```

### 5.5 Replanning Triggers

Automatic replanning on cascading failures:

```mangle
# Trigger replan on 3+ task failures
replan_needed(CampaignID, "task_failure_cascade") :-
    current_campaign(CampaignID),
    failed_campaign_task(CampaignID, TaskID1),
    failed_campaign_task(CampaignID, TaskID2),
    failed_campaign_task(CampaignID, TaskID3),
    TaskID1 != TaskID2,
    TaskID2 != TaskID3,
    TaskID1 != TaskID3.

# Trigger replan if user provides new instruction
replan_needed(CampaignID, "user_instruction") :-
    current_campaign(CampaignID),
    user_intent(_, /instruction, _, _, _).

# Pause and replan action
next_action(/pause_and_replan) :-
    replan_needed(_, _).
```

### 5.6 Campaign Completion

```mangle
# Helper: check if any phase is not complete
has_incomplete_phase(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, Status, _),
    Status != /completed,
    Status != /skipped.

# Campaign complete when all phases complete
campaign_complete(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID).

next_action(/campaign_complete) :-
    campaign_complete(_).
```

## 6. The Decomposer: Plan Creation

The Decomposer creates campaign plans through LLM + Mangle collaboration.

### 6.1 Decomposition Workflow

```
User Goal + Source Documents
         │
         ▼
┌─────────────────────────────┐
│ 1. INGEST SOURCE DOCUMENTS  │
│    - Read spec.md, reqs.txt │
│    - Infer document types   │
│    - Build content map      │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 2. EXTRACT REQUIREMENTS     │
│    - LLM parses documents   │
│    - Outputs structured JSON│
│    - Priority classification│
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 3. PROPOSE PLAN (LLM)       │
│    - Generate phases        │
│    - Generate tasks         │
│    - Estimate complexity    │
│    - Define dependencies    │
│    - Output: RawPlan JSON   │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 4. BUILD CAMPAIGN           │
│    - Convert RawPlan →      │
│      Campaign struct        │
│    - Generate phase IDs     │
│    - Create context profiles│
│    - Map dependencies       │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 5. VALIDATE (MANGLE)        │
│    - Load campaign facts    │
│    - Query validation rules │
│    - Detect:                │
│      - Circular deps        │
│      - Unreachable tasks    │
│      - Missing dependencies │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 6. REFINE IF ISSUES         │
│    - Feed issues to LLM     │
│    - Generate corrected plan│
│    - Re-validate            │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 7. LINK REQUIREMENTS        │
│    - Match requirements to  │
│      tasks by keyword       │
│    - Record coverage facts  │
└─────────────────────────────┘
         │
         ▼
     Campaign Ready
```

### 6.2 RawPlan Schema (LLM Output)

```json
{
  "title": "Implement codeNERD Kernel",
  "confidence": 0.85,
  "phases": [
    {
      "name": "Core Foundation",
      "order": 0,
      "description": "Create the Mangle kernel and fact store",
      "objective_type": "/create",
      "verification_method": "/tests_pass",
      "complexity": "/high",
      "depends_on": [],
      "focus_patterns": ["internal/core/*", "pkg/**/*.go"],
      "required_tools": ["fs_read", "fs_write", "exec_cmd"],
      "tasks": [
        {
          "description": "Create kernel.go with RealKernel struct",
          "type": "/file_create",
          "priority": "/critical",
          "depends_on": [],
          "artifacts": ["internal/core/kernel.go"]
        }
      ]
    }
  ]
}
```

### 6.3 Validation Rules

```mangle
# Detect circular phase dependencies
plan_validation_issue(CampaignID, /circular_dependency, Msg) :-
    phase_dependency(PhaseA, PhaseB, _),
    phase_dependency(PhaseB, PhaseA, _),
    let Msg = fn:concat("Circular dependency between ", PhaseA, " and ", PhaseB).

# Detect unreachable tasks
plan_validation_issue(CampaignID, /unreachable_task, Msg) :-
    task_dependency(TaskID, DepTaskID),
    !campaign_task(DepTaskID, _, _, _, _),
    let Msg = fn:concat("Task ", TaskID, " depends on non-existent ", DepTaskID).
```

## 7. Checkpoint System

Each phase must pass verification before proceeding.

### 7.1 Verification Methods

| Method | Implementation | When to Use |
|--------|---------------|-------------|
| `/tests_pass` | Run `go test ./...` | Code creation phases |
| `/builds` | Run `go build ./...` | Any code modification |
| `/manual_review` | Pause for user approval | Critical decisions |
| `/shard_validation` | Spawn reviewer shard | Architectural phases |
| `/none` | Skip verification | Documentation, research |

### 7.2 Checkpoint Runner

```go
func (cr *CheckpointRunner) Run(ctx context.Context, phase *Phase, method VerificationMethod) (bool, string, error) {
    switch method {
    case VerifyTestsPass:
        output, err := cr.executor.Execute(ctx, ShellCommand{
            Binary:    "go",
            Arguments: []string{"test", "./..."},
            Timeout:   300,
        })
        return err == nil, output, nil

    case VerifyBuilds:
        output, err := cr.executor.Execute(ctx, ShellCommand{
            Binary:    "go",
            Arguments: []string{"build", "./..."},
            Timeout:   300,
        })
        return err == nil, output, nil

    case VerifyShardValidate:
        result, err := cr.shardMgr.Spawn(ctx, "reviewer",
            fmt.Sprintf("Review phase: %s", phase.Name))
        return result.Approved, result.Feedback, err

    case VerifyManualReview:
        // Pause and wait for user
        return cr.waitForUserApproval(ctx, phase)
    }
}
```

## 8. Replanning System

When too many tasks fail, the system automatically replans.

### 8.1 Replan Triggers

1. **Task Failure Cascade**: 3+ tasks fail in current phase
2. **User Instruction**: New requirements provided mid-campaign
3. **Explicit Trigger**: `replan_trigger(CampaignID, Reason, Timestamp)` fact

### 8.2 Replanner Workflow

```go
func (r *Replanner) Replan(ctx context.Context, campaign *Campaign) error {
    // 1. Gather failure context
    failures := r.gatherFailures(campaign)

    // 2. Query current state
    currentPhase := r.getCurrentPhase()
    remainingTasks := r.getRemainingTasks()

    // 3. Ask LLM to propose adjustments
    prompt := fmt.Sprintf(`The campaign has encountered failures:

Failures:
%s

Current Phase: %s
Remaining Tasks: %d

Propose adjustments:
1. Which tasks should be modified?
2. Which tasks should be skipped?
3. What new tasks are needed?
4. Should phase order change?

Output JSON with adjustments.`, failures, currentPhase, remainingTasks)

    adjustments := r.llmClient.Complete(ctx, prompt)

    // 4. Apply adjustments
    r.applyAdjustments(campaign, adjustments)

    // 5. Reload facts into kernel
    r.kernel.Retract("campaign_task")
    r.kernel.LoadFacts(campaign.ToFacts())

    return nil
}
```

## 9. Autopoiesis: Learning During Execution

The campaign learns from execution patterns.

### 9.1 Learning Types

| Type | Trigger | Storage |
|------|---------|---------|
| `/success_pattern` | Task completes with specific approach | Promote to long-term |
| `/failure_pattern` | Task fails repeatedly | Avoid in future |
| `/preference` | User accepts/rejects output 3x | Style preference |
| `/optimization` | Faster approach discovered | Performance hint |

### 9.2 Learning Detection in Mangle

```mangle
# Learn from phase completion
promote_to_long_term(/pattern, PhaseType, /success) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    phase_objective(PhaseID, PhaseType, _, _),
    phase_checkpoint(PhaseID, _, /true, _, _).

# Learn from task failures
campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, Now) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, _, _, /failed, TaskType),
    task_error(TaskID, _, ErrorMsg),
    current_time(Now).
```

### 9.3 Applying Learnings

```go
func (o *Orchestrator) applyLearnings(ctx context.Context, task *Task, result any) {
    // Query for learnings to apply
    facts, _ := o.kernel.Query("promote_to_long_term")

    for _, fact := range facts {
        learning := Learning{
            Type:      "/success_pattern",
            Pattern:   fmt.Sprintf("%v", fact.Args[0]),
            Fact:      task.Description,
            AppliedAt: time.Now(),
        }
        o.campaign.Learnings = append(o.campaign.Learnings, learning)
    }
}
```

## 10. Implementation Files

| File | Purpose |
|------|---------|
| [internal/campaign/orchestrator.go](internal/campaign/orchestrator.go) | Main execution loop |
| [internal/campaign/decomposer.go](internal/campaign/decomposer.go) | Plan creation |
| [internal/campaign/context_pager.go](internal/campaign/context_pager.go) | Context management |
| [internal/campaign/checkpoint.go](internal/campaign/checkpoint.go) | Phase verification |
| [internal/campaign/replan.go](internal/campaign/replan.go) | Adaptive replanning |
| [internal/campaign/types.go](internal/campaign/types.go) | Type definitions |
| [internal/mangle/policy.gl](internal/mangle/policy.gl) §19 | Campaign orchestration rules |

## 11. Usage Example

```go
// 1. Create decomposer
decomposer := campaign.NewDecomposer(kernel, llmClient, workspace)

// 2. Decompose user goal into campaign
result, err := decomposer.Decompose(ctx, DecomposeRequest{
    Goal:         "Implement the codeNERD framework",
    SourcePaths:  []string{"CLAUDE.md", "Docs/research/*.md"},
    CampaignType: CampaignTypeGreenfield,
    UserHints:    []string{"Start with kernel, then perception layer"},
    ContextBudget: 100000,
})

// 3. Create orchestrator
orchestrator := campaign.NewOrchestrator(OrchestratorConfig{
    Workspace:    workspace,
    Kernel:       kernel,
    LLMClient:    llmClient,
    ShardManager: shardMgr,
    ProgressChan: progressChan,
    EventChan:    eventChan,
})

// 4. Set campaign
orchestrator.SetCampaign(result.Campaign)

// 5. Run campaign
err = orchestrator.Run(ctx)

// 6. Monitor progress
go func() {
    for progress := range progressChan {
        fmt.Printf("Phase %d/%d: %s (%.0f%% complete)\n",
            progress.CurrentPhaseIdx+1,
            progress.TotalPhases,
            progress.CurrentPhase,
            progress.OverallProgress*100,
        )
    }
}()
```

## 12. Conclusion

The Campaign Orchestrator transforms codeNERD from a "turn-by-turn chat agent" into a **sustained execution engine** capable of multi-day, multi-phase projects.

Key innovations:

1. **Phase-Aware Context Paging**: Never lose critical decisions to truncation
2. **Semantic Compression**: >100:1 reduction without information loss
3. **Mangle-Driven Sequencing**: Logic determines order, not LLM whims
4. **Checkpoint Verification**: Catch errors before they cascade
5. **Adaptive Replanning**: Automatic recovery from failures
6. **Autopoiesis**: Learn and improve during execution

This is the difference between an AI that helps for 5 minutes and one that can build a complete system over a week.
