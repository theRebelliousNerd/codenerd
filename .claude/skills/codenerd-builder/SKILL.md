---
name: codenerd-builder
description: Build the codeNERD Logic-First Neuro-Symbolic coding agent framework. This skill should be used when implementing components of the codeNERD architecture including the Mangle kernel, Perception/Articulation Transducers, ShardAgents, Virtual Predicates, TDD loops, and the Piggyback Protocol. Use for tasks involving Google Mangle logic, Go runtime integration, or any neuro-symbolic agent development following the Inversion of Control pattern.
---

# codeNERD Builder

Build the codeNERD high-assurance Logic-First CLI coding agent.

## Core Philosophy

codeNERD inverts the traditional agent control hierarchy:

- **Logic is the Executive**: All state transitions, permissions, tool selections, and memory retrieval are decided by a deterministic Mangle Kernel
- **LLM is the Transducer**: The model acts only as a peripheral for Perception (NL -> Logic Atoms) and Articulation (Logic Atoms -> NL)
- **Correctness by Construction**: Actions are derived from formal policy, not generated stochastically

## Architecture Overview

```text
[ Terminal / User ]
       |
[ Perception Transducer (LLM) ] --> [ Mangle Atoms ]
       |
[ Cortex Kernel ]
       |
       +-> [ FactStore (RAM) ]: Working Memory
       +-> [ Mangle Engine ]: Logic CPU
       +-> [ Virtual Store (FFI) ]
             +-> Filesystem Shard
             +-> Vector DB Shard
             +-> MCP/A2A Adapters
             +-> [ Shard Manager ]
                   +-> CoderShard
                   +-> TesterShard
                   +-> ReviewerShard
                   +-> ResearcherShard
       |
[ Articulation Transducer (LLM) ] --> [ User Response ]
```

## Implementation Workflow

### 1. Perception Transducer Implementation

The Perception Transducer converts user input into Mangle atoms. Key schemas:

**user_intent** - The seed for all logic:
```mangle
Decl user_intent(
    ID.Type<n>,
    Category.Type<n>,      # /query, /mutation, /instruction
    Verb.Type<n>,          # /explain, /refactor, /debug, /generate
    Target.Type<string>,
    Constraint.Type<string>
).
```

**focus_resolution** - Ground fuzzy references to concrete paths:
```mangle
Decl focus_resolution(
    RawReference.Type<string>,
    ResolvedPath.Type<string>,
    SymbolName.Type<string>,
    Confidence.Type<float>
).

# Clarification threshold - blocks execution if uncertain
clarification_needed(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 0.85.
```

Implementation location: [internal/perception/transducer.go](internal/perception/transducer.go)

### 2. World Model (EDB) Implementation

The Extensional Database maintains the "Ground Truth" of the codebase:

**file_topology** - Fact-Based Filesystem:
```mangle
Decl file_topology(
    Path.Type<string>,
    Hash.Type<string>,       # SHA-256
    Language.Type<n>,        # /go, /python, /ts
    LastModified.Type<int>,
    IsTestFile.Type<bool>
).
```

**symbol_graph** - AST Projection:
```mangle
Decl symbol_graph(
    SymbolID.Type<string>,
    Type.Type<n>,            # /function, /class, /interface
    Visibility.Type<n>,
    DefinedAt.Type<string>,
    Signature.Type<string>
).

Decl dependency_link(
    CallerID.Type<string>,
    CalleeID.Type<string>,
    ImportPath.Type<string>
).

# Transitive Impact Analysis
impacted(X) :- dependency_link(X, Y, _), modified(Y).
impacted(X) :- dependency_link(X, Z, _), impacted(Z).
```

**diagnostic** - Linter-Logic Bridge:
```mangle
Decl diagnostic(
    Severity.Type<n>,      # /panic, /error, /warning
    FilePath.Type<string>,
    Line.Type<int>,
    ErrorCode.Type<string>,
    Message.Type<string>
).

# The Commit Barrier - blocks git commit if errors exist
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).
```

Implementation locations:
- [internal/world/fs.go](internal/world/fs.go) - Filesystem projection
- [internal/world/ast.go](internal/world/ast.go) - AST projection

### 3. Executive Policy (IDB) Implementation

The Intensional Database contains rules that derive next actions:

**TDD Repair Loop**:
```mangle
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

# Surrender after max retries
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.
```

**Constitutional Safety**:
```mangle
permitted(Action) :- safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "rm").
```

Implementation location: [internal/core/kernel.go](internal/core/kernel.go)

### 4. Virtual Predicates (FFI) Implementation

Virtual Predicates abstract external APIs into logic:

```go
// In virtual_store.go
func (s *VirtualStore) GetFacts(pred ast.PredicateSym) []ast.Atom {
    switch pred.Symbol {
    case "mcp_tool_result":
        return s.callMCPTool(pred)
    case "file_content":
        return s.readFileContent(pred)
    case "shell_exec_result":
        return s.executeShell(pred)
    default:
        return s.MemStore.GetFacts(pred)
    }
}
```

Implementation location: [internal/core/virtual_store.go](internal/core/virtual_store.go)

### 5. ShardAgent Implementation

ShardAgents are ephemeral sub-kernels for parallel task execution:

**Shard Types**:
- **Type A (Ephemeral Generalists)**: Spawn -> Execute -> Die. RAM only.
- **Type B (Persistent Specialists)**: Pre-populated with domain knowledge.

```mangle
Decl delegate_task(
    ShardType.Type<n>,
    TaskDescription.Type<string>,
    Result.Type<string>
).

Decl shard_lifecycle(
    ShardID.Type<n>,
    ShardType.Type<n>,       # /generalist, /specialist
    MountStrategy.Type<n>,   # /ram, /sqlite
    KnowledgeBase.Type<string>,
    Permissions.Type<string>
).
```

Implementation locations:
- [internal/core/shard_manager.go](internal/core/shard_manager.go)
- [internal/shards/coder.go](internal/shards/coder.go)
- [internal/shards/tester.go](internal/shards/tester.go)
- [internal/shards/reviewer.go](internal/shards/reviewer.go)
- [internal/shards/researcher.go](internal/shards/researcher.go)

### 6. Piggyback Protocol Implementation

The **Piggyback Protocol** is the Corpus Callosum of the Neuro-Symbolic architecture—the invisible bridge between what the agent says and what it truly believes.

**The Dual-Channel Architecture:**
- **Surface Stream**: Natural language for the user (visible)
- **Control Stream**: Logic atoms for the kernel (hidden)

```json
{
  "surface_response": "I fixed the authentication bug.",
  "control_packet": {
    "intent_classification": {
      "category": "mutation",
      "confidence": 0.95
    },
    "mangle_updates": [
      "task_status(/auth_fix, /complete)",
      "file_state(/auth.go, /modified)",
      "diagnostic(/error, \"auth.go\", 42, \"E001\", \"fixed\")"
    ],
    "memory_operations": [
      { "op": "promote_to_long_term", "key": "preference:code_style", "value": "concise" }
    ],
    "self_correction": null
  }
}
```

**Key Capabilities:**

1. **Semantic Compression** - Chat history compressed to atoms (>100:1 ratio)
2. **Constitutional Override** - Kernel can block/rewrite unsafe surface responses
3. **Grammar-Constrained Decoding** - Forces valid JSON at inference level
4. **Self-Correction** - Abductive hypotheses trigger automatic recovery

**The Hidden Injection:**
```text
CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object
containing surface_response and control_packet.
Your control_packet must reflect the true state of the world, even if the
surface_response is polite.
```

For complete specification, see [references/piggyback-protocol.md](references/piggyback-protocol.md)

Implementation location: [internal/articulation/emitter.go](internal/articulation/emitter.go)

### 7. Spreading Activation (Context Selection)

Replace vector RAG with logic-directed context:

```mangle
# Base Activation (Recency)
activation(Fact, 100) :- new_fact(Fact).

# Spreading Activation (Dependency)
activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# Recursive Spread
activation(FileB, 50) :-
    activation(FileA, Score),
    Score > 40,
    dependency_link(FileA, FileB, _).

# Context Pruning
context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.
```

## Key Implementation Patterns

### Pattern 1: The Hallucination Firewall

Every action must be logically permitted:

```go
func (k *Kernel) Execute(action Action) error {
    if !k.Mangle.Query("permitted(?)", action.Name) {
        return ErrAccessDenied
    }
    return k.VirtualStore.Dispatch(action)
}
```

### Pattern 2: Grammar-Constrained Decoding

Force valid Mangle syntax output:
- Use structured output schemas
- Implement repair loops for malformed atoms
- Validate against Mangle EBNF before committing

### Pattern 3: The OODA Loop

```
Observe -> Orient -> Decide -> Act
   |          |         |        |
   v          v         v        v
Transducer  Spreading  Mangle   Virtual
 (LLM)     Activation  Engine    Store
```

### Pattern 4: Autopoiesis (Self-Learning)

The system learns from user interactions without retraining:

**Rejection Tracking:**
```go
// In shard execution
if err := c.applyEdits(ctx, result.Edits); err != nil {
    c.trackRejection(coderTask.Action, err.Error())  // Track pattern
    return "", err
}
c.trackAcceptance(coderTask.Action)  // Track success
```

**Mangle Pattern Detection:**
```mangle
# 3 rejections = pattern signal
preference_signal(Pattern) :-
    rejection_count(Pattern, N), N >= 3.

# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).
```

**The Ouroboros Loop (Tool Self-Generation):**
```mangle
# Detect missing capability
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation
next_action(/generate_tool) :-
    missing_tool_for(_, _).
```

For complete specification, see [references/autopoiesis.md](references/autopoiesis.md)

## Mangle Logic Files

Existing logic files to extend:
- [internal/mangle/schemas.gl](internal/mangle/schemas.gl) - Core schema declarations
- [internal/mangle/policy.gl](internal/mangle/policy.gl) - Constitutional rules
- [internal/mangle/coder.gl](internal/mangle/coder.gl) - Coder shard logic
- [internal/mangle/tester.gl](internal/mangle/tester.gl) - Tester shard logic
- [internal/mangle/reviewer.gl](internal/mangle/reviewer.gl) - Reviewer shard logic

## 8. Campaign Orchestration (Multi-Phase Goals)

Campaigns handle long-running, multi-phase goal execution:

**Campaign Types**:
- `/greenfield` - Build from scratch
- `/feature` - Add major feature
- `/audit` - Stability/security audit
- `/migration` - Technology migration
- `/remediation` - Fix issues across codebase

**The Decomposer** - LLM + Mangle collaboration:
1. Ingest source documents (specs, requirements)
2. Extract requirements via LLM
3. Propose phases and tasks
4. Validate via Mangle (circular deps, unreachable tasks)
5. Refine if issues found
6. Link requirements to tasks

**Context Pager** - Phase-aware context management:
```go
// Budget allocation
totalBudget:     100000 // 100k tokens
coreReserve:     5000   // 5% - core facts
phaseReserve:    30000  // 30% - current phase
historyReserve:  15000  // 15% - compressed history
workingReserve:  40000  // 40% - working memory
prefetchReserve: 10000  // 10% - upcoming tasks
```

**Campaign Policy Rules** ([internal/mangle/policy.gl:479](internal/mangle/policy.gl#L479)):
```mangle
# Phase eligibility - all hard deps complete
phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).

# Next task - highest priority without blockers
next_campaign_task(TaskID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    !has_blocking_task_dep(TaskID).

# Replan on cascade failures
replan_needed(CampaignID, "task_failure_cascade") :-
    failed_campaign_task(CampaignID, TaskID1),
    failed_campaign_task(CampaignID, TaskID2),
    failed_campaign_task(CampaignID, TaskID3),
    TaskID1 != TaskID2, TaskID2 != TaskID3, TaskID1 != TaskID3.
```

Implementation locations:
- [internal/campaign/orchestrator.go](internal/campaign/orchestrator.go) - Main orchestration loop
- [internal/campaign/decomposer.go](internal/campaign/decomposer.go) - Plan decomposition
- [internal/campaign/context_pager.go](internal/campaign/context_pager.go) - Context management
- [internal/campaign/types.go](internal/campaign/types.go) - All campaign types with ToFacts()

## 9. Actual Kernel Implementation

The kernel is implemented in [internal/core/kernel.go](internal/core/kernel.go):

```go
type RealKernel struct {
    mu          sync.RWMutex
    facts       []Fact
    store       factstore.FactStore
    programInfo *analysis.ProgramInfo
    schemas     string  // From schemas.gl
    policy      string  // From policy.gl
}

// Key methods:
// - LoadFacts(facts []Fact) - Add to EDB and rebuild
// - Query(predicate string) - Query derived facts
// - Assert(fact Fact) - Add single fact dynamically
// - Retract(predicate string) - Remove all facts of predicate
```

**Fact struct with ToAtom()**:
```go
type Fact struct {
    Predicate string
    Args      []interface{}
}

func (f Fact) ToAtom() (ast.Atom, error) {
    // Converts Go types to Mangle AST terms
    // Handles: strings, name constants (/foo), ints, floats, bools
}
```

## 10. Shard Implementation Pattern

Each shard follows this pattern (see [internal/shards/coder.go](internal/shards/coder.go)):

```go
type CoderShard struct {
    id           string
    config       core.ShardConfig
    state        core.ShardState
    kernel       *core.RealKernel      // Own kernel instance
    llmClient    perception.LLMClient
    virtualStore *core.VirtualStore

    // Autopoiesis tracking
    rejectionCount  map[string]int
    acceptanceCount map[string]int
}

func (c *CoderShard) Execute(ctx context.Context, task string) (string, error) {
    // 1. Load shard-specific policy
    c.kernel.LoadPolicyFile("coder.gl")

    // 2. Parse task into structured form
    coderTask := c.parseTask(task)

    // 3. Assert task facts to kernel
    c.assertTaskFacts(coderTask)

    // 4. Check impact via Mangle query
    if blocked, reason := c.checkImpact(target); blocked {
        return "", fmt.Errorf("blocked: %s", reason)
    }

    // 5. Generate code via LLM
    result := c.generateCode(ctx, coderTask, fileContext)

    // 6. Apply edits via VirtualStore
    c.applyEdits(ctx, result.Edits)

    // 7. Generate facts for propagation back to parent
    result.Facts = c.generateFacts(result)
}
```

## 11. Policy File Structure

The policy file ([internal/mangle/policy.gl](internal/mangle/policy.gl)) has 20 sections:

1. **Spreading Activation** (§1) - Context selection
2. **Strategy Selection** (§2) - Dynamic workflow dispatch
3. **TDD Repair Loop** (§3) - Write→Test→Analyze→Fix
4. **Focus Resolution** (§4) - Clarification threshold
5. **Impact Analysis** (§5) - Refactoring guard
6. **Commit Barrier** (§6) - Block commit on errors
7. **Constitutional Safety** (§7) - Permission gates
8. **Shard Delegation** (§8) - Task routing
9. **Browser Physics** (§9) - DOM spatial reasoning
10. **Tool Capability Mapping** (§10) - Action mappings
11. **Abductive Reasoning** (§11) - Missing hypothesis detection
12. **Autopoiesis** (§12) - Learning patterns
13. **Git-Aware Safety** (§13) - Chesterton's Fence
14. **Shadow Mode** (§14) - Counterfactual reasoning
15. **Interactive Diff** (§15) - Mutation approval
16. **Session State** (§16) - Clarification loop
17. **Knowledge Atoms** (§17) - Domain expertise
18. **Shard Types** (§18) - Classification
19. **Campaign Orchestration** (§19) - Multi-phase execution
20. **Campaign Triggers** (§20) - Campaign start detection

## Resources

For detailed specifications, consult the reference documentation:

- [references/architecture.md](references/architecture.md) - Theoretical foundations and neuro-symbolic principles
- [references/mangle-schemas.md](references/mangle-schemas.md) - Complete Mangle schema reference
- [references/implementation-guide.md](references/implementation-guide.md) - Go implementation patterns and component details
- [references/piggyback-protocol.md](references/piggyback-protocol.md) - Dual-channel steganographic control protocol specification
- [references/campaign-orchestrator.md](references/campaign-orchestrator.md) - Multi-phase goal execution and context paging system
- [references/autopoiesis.md](references/autopoiesis.md) - Self-creation, runtime learning, and the Ouroboros Loop
- [references/shard-agents.md](references/shard-agents.md) - All four shard types, ShardManager API, and built-in implementations
