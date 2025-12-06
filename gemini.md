---
name: codenerd-builder
description: Build the codeNERD Logic-First Neuro-Symbolic coding agent framework. This skill should be used when implementing components of the codeNERD architecture including the Mangle kernel, Perception/Articulation Transducers, ShardAgents, Virtual Predicates, TDD loops, and the Piggyback Protocol. Use for tasks involving Google Mangle logic, Go runtime integration, or any neuro-symbolic agent development following the Creative-Executive Partnership pattern.
---

# codeNERD Builder

Build the codeNERD high-assurance Logic-First CLI coding agent.

## Core Philosophy

Current AI agents make a category error: they ask LLMs to handle everything—creativity AND planning, insight AND memory, problem-solving AND self-correction—when LLMs excel at the former but struggle with the latter. codeNERD separates these concerns through a **Creative-Executive Partnership**:

- **LLM as Creative Center**: The model is the source of problem-solving, solution synthesis, goal-crafting, and insight. It understands problems deeply, generates novel approaches, and crafts creative solutions.
- **Logic as Executive**: Planning, long-term memory, orchestration, skill retention, and safety derive from deterministic Mangle rules. The harness handles what LLMs cannot reliably perform.
- **Transduction Interface**: NL↔Logic atom conversion channels the LLM's creative outputs through formal structure, ensuring creativity flows safely into execution.

This architecture **liberates** the LLM to focus purely on what it does best, while the harness ensures those creative outputs are channeled safely and consistently. The result: creative power and deterministic safety coexist by design.

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

# Mangle Expert: PhD-Level Reference

You are an expert in **Google Mangle**, a declarative deductive database language extending Datalog with practical features for software analysis, security evaluation, and multi-source data integration.

## Core Philosophy

Mangle occupies a unique position in the logic programming landscape:

- **Bottom-up evaluation** (like Datalog) vs top-down (like Prolog)
- **Stratified negation** for safe non-monotonic reasoning
- **First-class aggregation** via transform pipelines
- **Typed structured data** (maps, structs, lists)

## Quick Reference: Essential Syntax

```mangle
# Facts (EDB - Extensional Database)
parent(/oedipus, /antigone).
vulnerable("log4j", "2.14.0", "CVE-2021-44228").

# Rules (IDB - Intensional Database) - "Head is true if Body is true"
sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.

# Query (REPL only)
?sibling(/antigone, X)

# Key syntax:
# /name     - Named constant (atom)
# UPPERCASE - Variables
# :-        - Rule implication ("if")
# ,         - Conjunction (AND)
# .         - Statement terminator (REQUIRED!)
```

---

## 1. Complete Data Types

### Named Constants (Atoms)

```mangle
/oedipus
/critical_severity
/crates.io/fnv
/home.cern/news/computing/30-years-free-and-open-web
```

### Numbers

```mangle
42, -17, 0            # 64-bit signed integers
3.14, -2.5, 1.0e6     # 64-bit IEEE 754 floats
```

### Strings

```mangle
"normal string"
"with \"quotes\""
"newline \n tab \t backslash \\"
`
Multi-line strings
use backticks
`
b"\x80\x81\x82\n"     # Byte strings
```

### Lists

```mangle
[]                    # Empty
[1, 2, 3]
[/a, /b, /c]
[[1, 2], [3, 4]]      # Nested
```

### Maps & Structs

```mangle
[/a: /foo, /b: /bar]                    # Map
{/name: "Alice", /age: 30}              # Struct
{/x: 1, /y: 2, /nested: {/z: 3}}        # Nested struct
```

### Pairs & Tuples

```mangle
fn:pair("key", "value")
fn:tuple(1, 2, "three", /four)
```

---

## 2. Type System

### Type Declarations

```mangle
Decl employee(ID.Type<int>, Name.Type<string>, Dept.Type<n>).
Decl config(Data.Type<{/host: string, /port: int}>).
Decl flexible(Value.Type<int | string>).        # Union type
Decl tags(ID.Type<int>, Tags.Type<[string]>).   # List type
```

### Type Expressions

```mangle
Type<int>              # Integer
Type<float>            # Float
Type<string>           # String
Type<n>                # Name (atom)
Type<[T]>              # List of T
Type<{/k: v}>          # Struct/Map
Type<T1 | T2>          # Union type
Type<Any>              # Any type
/any                   # Universal type
fn:Singleton(/foo)     # Singleton type
fn:Union(/name, /string)  # Union type expression
```

### Gradual Typing

Types are optional - untyped facts are valid. Type checking occurs at runtime when declarations exist.

---

## 3. Operators & Comparisons

### Rule Operators

```mangle
:-    # Rule implication (if) - preferred
<-    # Alternative implication syntax
,     # Conjunction (AND)
!     # Negation (requires stratification)
|>    # Transform pipeline
```

### Comparison Operators

```mangle
=     # Unification / equality
!=    # Inequality
<     # Less than (numeric)
<=    # Less or equal (numeric)
>     # Greater than (numeric)
>=    # Greater or equal (numeric)
```

---

## 4. Transform Pipelines & Aggregation

### General Form

```mangle
result(GroupVars, AggResults) :-
    body_atoms |>
    do fn:transform1() |>
    do fn:transform2() |>
    let AggVar1 = fn:aggregate1(),
    let AggVar2 = fn:aggregate2().
```

### Grouping

```mangle
# Group by single variable
count_per_category(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:Count().

# Group by multiple variables
stats(Region, Product, Count) :-
    sale(Region, Product, Amount) |>
    do fn:group_by(Region, Product),
    let Count = fn:Count().

# No grouping (global aggregate)
total_count(N) :-
    item(_, _) |>
    do fn:group_by(),
    let N = fn:Count().
```

### Aggregation Functions

```mangle
fn:Count()           # Count elements in group
fn:Sum(Variable)     # Sum numeric values
fn:Min(Variable)     # Minimum value
fn:Max(Variable)     # Maximum value
```

### Arithmetic Functions

```mangle
fn:plus(A, B)        # A + B
fn:minus(A, B)       # A - B
fn:multiply(A, B)    # A * B
fn:divide(A, B)      # A / B
fn:modulo(A, B)      # A % B
fn:negate(A)         # -A
fn:abs(A)            # |A|
```

### Complete Aggregation Example

```mangle
category_stats(Cat, Count, Total, Min, Max, Avg) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Count = fn:Count(),
    let Total = fn:Sum(Value),
    let Min = fn:Min(Value),
    let Max = fn:Max(Value) |>
    let Avg = fn:divide(Total, Count).
```

---

## 5. Structured Data Access

### Struct/Map Field Access

```mangle
# Using :match_field
record_name(ID, Name) :-
    person_record(ID, Info),
    :match_field(Info, /name, Name).

# Using :match_entry (equivalent)
record_name(ID, Name) :-
    person_record(ID, Info),
    :match_entry(Info, /name, Name).
```

### List Operations

```mangle
fn:list:get(List, Index)         # Get by index (0-based)
:match_cons(List, Head, Tail)    # Destructure [Head|Tail]
:match_nil(List)                 # Check if empty
:list:member(Elem, List)         # Membership check
fn:list_cons(Head, Tail)         # Construct [Head|Tail]
fn:list_append(List1, List2)     # Concatenate
fn:list_length(List)             # Length
```

### String Operations

```mangle
fn:string_concat(S1, S2)
fn:string_length(S)
fn:string_contains(S, Substring)
```

---

## 6. Recursion Patterns

### Transitive Closure (Reachability)

```mangle
# Base case: direct edge
reachable(X, Y) :- edge(X, Y).
# Recursive case: indirect path
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

### Path Construction

```mangle
# Simple paths with node list
path(X, Y, [X, Y]) :- edge(X, Y).
path(X, Z, [X|Rest]) :- edge(X, Y), path(Y, Z, Rest).
```

### Path with Cost Accumulation

```mangle
path_cost(X, Y, Cost) :- edge(X, Y, Cost).
path_cost(X, Z, TotalCost) :-
    edge(X, Y, Cost1),
    path_cost(Y, Z, Cost2) |>
    let TotalCost = fn:plus(Cost1, Cost2).
```

### Shortest Path

```mangle
shortest(X, Y, MinLen) :-
    path_len(X, Y, Len) |>
    do fn:group_by(X, Y),
    let MinLen = fn:Min(Len).
```

### Cycle Detection

```mangle
cycle_edge(X, Y) :- edge(X, Y), reachable(Y, X).
has_cycle(X) :- cycle_edge(X, _).
```

### Dependency Closure (Bill of Materials)

```mangle
depends(P, Lib) :- depends_direct(P, Lib).
depends(P, Lib) :- depends_direct(P, Q), depends(Q, Lib).

# With quantity multiplication
bom(Product, Part, TotalQty) :-
    assembly(Product, SubAssy, Qty1),
    bom(SubAssy, Part, Qty2) |>
    let TotalQty = fn:multiply(Qty1, Qty2).
```

### Mutual Recursion

```mangle
even(0).
even(N) :- N > 0, M = fn:minus(N, 1), odd(M).

odd(1).
odd(N) :- N > 1, M = fn:minus(N, 1), even(M).
```

---

## 7. Negation Patterns

### Set Difference

```mangle
safe(X) :- candidate(X), !excluded(X).
```

### Universal Quantification (All)

```mangle
# "All dependencies satisfied" = "no unsatisfied dependency"
all_deps_satisfied(Task) :-
    task(Task),
    !has_unsatisfied_dep(Task).

has_unsatisfied_dep(Task) :-
    depends_on(Task, Dep),
    !completed(Dep).
```

### Handling Empty Groups

```mangle
# Find projects WITH developers
project_with_developers(ProjectID) <-
    project_assignment(ProjectID, _, /software_development, _).

# Find projects WITHOUT developers
project_without_developers(ProjectID) <-
    project_name(ProjectID, _),
    !project_with_developers(ProjectID).
```

---

## 8. Safety Constraints

### Variable Safety

Every variable in rule head must appear in:

1. A positive body atom, OR
2. A unification `Var = constant`

```mangle
# SAFE
rule(X, Y) :- foo(X), bar(Y).
rule(X, Y) :- foo(X), Y = 42.

# UNSAFE - Y never bound
rule(X, Y) :- foo(X).
```

### Negation Safety

Variables in negated atom must be bound by positive atoms FIRST:

```mangle
# SAFE - X bound by candidate before negation
safe(X) :- candidate(X), !excluded(X).

# UNSAFE - X never bound
unsafe(X) :- !foo(X).
```

### Aggregation Safety

Grouping variables must appear in body atoms:

```mangle
# SAFE - Cat appears in body
count_per_cat(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:Count().

# UNSAFE - Cat never appears
bad(Cat, N) :-
    item(_, _) |>
    do fn:group_by(Cat),  # Can't group by unbound variable
    let N = fn:Count().
```

---

## 9. Mathematical Foundations

### Herbrand Semantics

- **Herbrand Universe**: Set of all ground terms constructible from constants
- **Herbrand Base**: Set of all ground atoms over Herbrand universe
- **Herbrand Interpretation**: Subset of Herbrand base (facts deemed true)
- **Minimal Model**: Smallest interpretation satisfying all rules

### Fixed-Point Semantics

**Immediate Consequence Operator**: T_P(I) = {head | head :- body in P, body true in I}

**Properties**:

- Monotonic: I ⊆ J → T_P(I) ⊆ T_P(J)
- Continuous (finite case)

**Least Fixpoint** (Tarski's Theorem):

```
lfp(T_P) = T_P^ω(∅) = ∪_{i=0}^∞ T_P^i(∅)
```

### Semi-Naive Evaluation

```
Δ₀ = EDB (base facts)
For each stratum S (in order):
    i = 0
    repeat:
        Δᵢ₊₁ = apply rules to Δᵢ (using all facts)
        Δᵢ₊₁ = Δᵢ₊₁ \ (all previously derived facts)
        i++
    until Δᵢ = ∅ (fixpoint reached)
```

**Key insight**: Only NEW facts trigger re-evaluation (efficiency).

### Stratification Theory

**Dependency Graph**:

- Nodes: Predicates
- Positive edges: p uses q in positive position
- Negative edges: p uses ¬q

**Valid Stratification**: Partition predicates into strata S₀, S₁, ..., Sₙ such that:

- Positive edges: within or forward strata (i → j where i ≤ j)
- Negative edges: strictly backward (i → j where i > j)

**Perfect Model Semantics**: Evaluate strata bottom-up, each to fixpoint.

### Complexity Analysis

**Data Complexity** (fixed program, variable data):

- Positive Datalog: P-complete
- Stratified Datalog (Mangle): P-complete

**Combined Complexity** (variable program and data):

- Positive Datalog: EXPTIME-complete
- Stratified Datalog: EXPTIME-complete

---

## 10. Comparison with Related Systems

| Feature | Mangle | Prolog | SQL | Datalog | Z3/SMT |
|---------|--------|--------|-----|---------|--------|
| **Evaluation** | Bottom-up | Top-down | Set-based | Bottom-up | Constraint |
| **Recursion** | Native | Native | CTE only | Native | Limited |
| **Aggregation** | Transforms | Bagof/setof | GROUP BY | Limited | No |
| **Negation** | Stratified | NAF | NOT EXISTS | Stratified | Full |
| **Optimization** | No | No | No | No | Yes |
| **Best for** | Graph analysis | AI/search | CRUD | Knowledge base | Constraints |

---

## 11. REPL Commands

```
<decl>.            Add type declaration
<clause>.          Add clause, evaluate
?<predicate>       Query predicate
?<goal>            Query with pattern
::load <path>      Load source file
::help             Show help
::pop              Reset to previous state
::show <pred>      Show predicate info
::show all         Show all predicates
Ctrl-D             Exit
```

---

## 12. Common Pitfalls

### Pitfall 1: Forgetting periods

```mangle
# WRONG
parent(/a, /b)

# CORRECT
parent(/a, /b).
```

### Pitfall 2: Unbound variables in negation

```mangle
# WRONG - X not bound first
bad(X) :- !foo(X).

# CORRECT - X bound by candidate first
good(X) :- candidate(X), !foo(X).
```

### Pitfall 3: Cartesian products

```mangle
# INEFFICIENT (10K x 10K = 100M intermediate)
slow(X, Y) :- table1(X), table2(Y), filter(X, Y).

# EFFICIENT (filter first)
fast(X, Y) :- filter(X, Y), table1(X), table2(Y).
```

### Pitfall 4: Direct struct field access

```mangle
# WRONG - pattern matching doesn't work this way
bad(Name) :- record({/name: Name}).

# CORRECT - use :match_field
good(Name) :- record(R), :match_field(R, /name, Name).
```

### Pitfall 5: Infinite recursion

```mangle
# DANGER - unbounded growth
count_up(N) :- count_up(M), N = fn:plus(M, 1).

# SAFE - bounded by existing data
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).
```

---

## 13. Production Patterns

### Vulnerability Scanner

```mangle
# Transitive dependency tracking
contains_jar(P, Name, Version) :-
    contains_jar_directly(P, Name, Version).
contains_jar(P, Name, Version) :-
    project_depends(P, Q),
    contains_jar(Q, Name, Version).

# Vulnerable version detection
projects_with_vulnerable_log4j(P) :-
    projects(P),
    contains_jar(P, "log4j", Version),
    Version != "2.17.1",
    Version != "2.12.4",
    Version != "2.3.2".

# Count affected projects
count_vulnerable(Num) :-
    projects_with_vulnerable_log4j(P) |>
    do fn:group_by(),
    let Num = fn:Count().
```

### Access Control Policy

```mangle
# Role hierarchy
has_role(User, Role) :- assigned_role(User, Role).
has_role(User, SuperRole) :-
    has_role(User, Role),
    role_inherits(SuperRole, Role).

# Permission derivation
permitted(User, Action, Resource) :-
    has_role(User, Role),
    role_permits(Role, Action, Resource).

# Deny overrides allow
denied(User, Action, Resource) :-
    explicit_deny(User, Action, Resource).

final_permitted(User, Action, Resource) :-
    permitted(User, Action, Resource),
    !denied(User, Action, Resource).
```

### Impact Analysis

```mangle
# Symbol dependencies
calls(Caller, Callee) :- direct_call(Caller, Callee).
calls(Caller, Callee) :- direct_call(Caller, Mid), calls(Mid, Callee).

# Modified file impact
impacted(File) :- modified(File).
impacted(File) :-
    impacted(ModFile),
    imports(File, ModFile).

# Test coverage requirement
needs_test(File) :-
    impacted(File),
    is_source_file(File),
    !is_test_file(File).
```

---

## 14. Installation & Resources

### Go Implementation (Recommended)

```bash
GOBIN=~/bin go install github.com/google/mangle/interpreter/mg@latest
~/bin/mg  # Start REPL
```

### Build from Source

```bash
git clone https://github.com/google/mangle
cd mangle
go get -t ./...
go build ./...
go test ./...
```

### Resources

- GitHub: <https://github.com/google/mangle>
- Documentation: <https://mangle.readthedocs.io>
- Go Packages: <https://pkg.go.dev/github.com/google/mangle>
- Demo Service: <https://github.com/burakemir/mangle-service>

---

## Grammar Reference (EBNF)

```ebnf
Program     ::= (Decl | Clause)*
Decl        ::= 'Decl' Atom '.'
Clause      ::= Atom (':-' Atom (',' Atom)*)? '.'
Atom        ::= PredicateSym '(' Term (',' Term)* ')'
             |  '!' Atom
             |  Term Op Term
Term        ::= Const | Var | List | Map | Transform
Const       ::= Name | Int | Float | String
Name        ::= '/' Identifier
Var         ::= UppercaseIdentifier
List        ::= '[' (Term (',' Term)*)? ']'
Map         ::= '{' (Name ':' Term (',' Name ':' Term)*)? '}'
Transform   ::= Term '|>' TransformOp
TransformOp ::= 'do' Function | 'let' Var '=' Function
Op          ::= '=' | '!=' | '<' | '<=' | '>' | '>='
```

---

**Remember**: In Mangle, logic determines reality. Write declarative rules that describe WHAT is true, not HOW to compute it. The engine handles evaluation order, optimization, and termination.
