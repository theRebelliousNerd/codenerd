# codeNERD Mangle Schema Reference

Complete reference for all Mangle schemas and logic rules in codeNERD.

## 1. Perception Layer Schemas

### 1.1 user_intent

The seed for all subsequent logic. Captures user desire without execution.

```mangle
Decl user_intent(
    ID.Type<n>,
    Category.Type<n>,      # /query, /mutation, /instruction
    Verb.Type<n>,          # /explain, /refactor, /debug, /generate, /scaffold
    Target.Type<string>,   # "auth system", "login button"
    Constraint.Type<string> # "no external libs", "max_complexity < 10"
).
```

**Category Types**:
- `/query`: Read-only information requests
- `/mutation`: State-altering requests
- `/instruction`: New rules or preferences

**Verb Ontology** (controlled vocabulary):
- `/explain`, `/refactor`, `/debug`, `/generate`, `/generate_test`
- `/scaffold`, `/define_agent`, `/fix`, `/explore`, `/review`

**Polymorphic Dispatch**:
```mangle
# fix applied to File -> TDD loop
strategy(/tdd_repair_loop) :-
    user_intent(_, /mutation, /fix, Target, _),
    file_topology(Target, _, _, _, _).

# fix applied to Schema -> migration
strategy(/migration_generation) :-
    user_intent(_, /mutation, /fix, Target, _),
    database_schema(Target, _).
```

### 1.2 focus_resolution

Maps fuzzy references to concrete filesystem locations.

```mangle
Decl focus_resolution(
    RawReference.Type<string>,  # "the auth thing"
    ResolvedPath.Type<string>,  # "/src/auth/handler.go"
    SymbolName.Type<string>,    # "AuthHandler"
    Confidence.Type<float>      # 0.0 - 1.0
).
```

**Clarification Logic**:
```mangle
clarification_needed(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 0.85.

# Blocks execution if clarification needed
next_action(/interrogative_mode) :-
    clarification_needed(_).
```

### 1.3 ambiguity_flag

Represents holes in the knowledge graph.

```mangle
Decl ambiguity_flag(
    MissingParam.Type<string>,   # "TargetFunction"
    ContextClue.Type<string>,    # Part of prompt hinting at missing data
    Hypothesis.Type<string>      # LLM's best guess
).
```

**Short-Circuit Logic**:
```mangle
# Existence of ambiguity blocks execution
next_action(/interrogative_mode) :-
    ambiguity_flag(_, _, _).
```

## 2. World Model (EDB) Schemas

### 2.1 file_topology

The Fact-Based Filesystem representation.

```mangle
Decl file_topology(
    Path.Type<string>,           # Unique identifier
    Hash.Type<string>,           # SHA-256 for change detection
    Language.Type<n>,            # /go, /python, /ts, /rust
    LastModified.Type<int>,      # Unix timestamp
    IsTestFile.Type<bool>,       # Critical for TDD
    Size.Type<int>               # Bytes
).
```

**Windowing for Large Files**:
```mangle
Decl file_chunk(
    Path.Type<string>,
    StartLine.Type<int>,
    EndLine.Type<int>,
    Content.Type<string>
).
```

### 2.2 symbol_graph

AST projection for semantic reasoning.

```mangle
Decl symbol_graph(
    SymbolID.Type<string>,       # "func:main:AuthHandler"
    Type.Type<n>,                # /function, /class, /interface, /variable
    Visibility.Type<n>,          # /public, /private, /protected
    DefinedAt.Type<string>,      # Path + Line Number
    Signature.Type<string>       # "(User) -> Result<bool>"
).
```

### 2.3 dependency_link

The Call Graph (A calls B).

```mangle
Decl dependency_link(
    CallerID.Type<string>,
    CalleeID.Type<string>,
    ImportPath.Type<string>
).
```

**Transitive Impact Analysis**:
```mangle
# Direct impact
impacted(X) :- dependency_link(X, Y, _), modified(Y).

# Recursive closure
impacted(X) :- dependency_link(X, Z, _), impacted(Z).

# Mark for testing
needs_testing(X) :- impacted(X).
```

### 2.4 diagnostic

Linter-Logic Bridge - compiler errors as logical constraints.

```mangle
Decl diagnostic(
    Severity.Type<n>,            # /panic, /error, /warning, /info
    FilePath.Type<string>,
    Line.Type<int>,
    ErrorCode.Type<string>,      # E0308 (Rust), TS2322 (TypeScript)
    Message.Type<string>
).
```

**The Commit Barrier**:
```mangle
# Blocks git commit if any errors exist
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).

# The system is physically incapable of committing broken code
git_commit_allowed() :-
    not block_commit(_).
```

## 3. Executive Policy (IDB) Schemas

### 3.1 strategy_selection

Dynamic dispatch of logical workflows.

```mangle
Decl active_strategy(Strategy.Type<n>).

# Only one strategy active per shard
strategy_conflict() :-
    active_strategy(A),
    active_strategy(B),
    A != B.
```

**Strategy Selection Rules**:
```mangle
# TDD for bug fixes in test files
active_strategy(/tdd_repair_loop) :-
    user_intent(_, /mutation, /fix, _, _),
    focus_resolution(_, Path, _, _),
    file_topology(Path, _, _, _, true).

# Breadth-first for exploration
active_strategy(/breadth_first_survey) :-
    user_intent(_, /query, /explore, _, _),
    file_count(N), N > 100.

# Project init for scaffolding
active_strategy(/project_init) :-
    user_intent(_, /mutation, /scaffold, _, _),
    file_count(0).
```

### 3.2 test_state

TDD loop state tracking.

```mangle
Decl test_state(State.Type<n>).
# States: /passing, /failing, /compiling, /unknown, /log_read, /cause_found, /patch_applied

Decl retry_count(N.Type<int>).
```

### 3.3 repair_action (TDD Repair Loop)

```mangle
# Read error log when failing
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

# Analyze after reading logs
next_action(/analyze_root_cause) :-
    test_state(/log_read).

# Generate patch after analysis
next_action(/generate_patch) :-
    test_state(/cause_found).

# Run tests after patching
next_action(/run_tests) :-
    test_state(/patch_applied).

# Surrender after max retries
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.
```

### 3.4 impact_radius (Refactoring Guard)

```mangle
Decl test_coverage(Symbol.Type<string>, Covered.Type<bool>).

# Unsafe if impacted symbol has no tests
unsafe_to_refactor(Target) :-
    impacted(Dependent),
    depends_on(Dependent, Target),
    test_coverage(Dependent, false).

# Block write_file if unsafe
block_action(/write_file, Target) :-
    unsafe_to_refactor(Target).
```

## 4. Virtual Predicate Schemas

### 4.1 shell_exec_request

Safe execution sandbox for shell commands.

```mangle
Decl shell_exec_request(
    Binary.Type<string>,         # "go", "npm", "cargo"
    Arguments.Type<string>,      # JSON array of strings
    WorkingDirectory.Type<string>,
    TimeoutSeconds.Type<int>,
    EnvironmentVars.Type<string> # Allowed vars only
).

Decl shell_exec_result(
    RequestID.Type<n>,
    ExitCode.Type<int>,
    Stdout.Type<string>,
    Stderr.Type<string>
).
```

**Allowlist Constraint**:
```mangle
Decl binary_allowlist(Binary.Type<string>).
binary_allowlist("go").
binary_allowlist("npm").
binary_allowlist("cargo").
binary_allowlist("git").

# Block if not in allowlist
block_action(/shell_exec, Req) :-
    shell_exec_request(Binary, _, _, _, _),
    not binary_allowlist(Binary).
```

### 4.2 structural_search

Semantic grep using AST patterns.

```mangle
Decl structural_search(
    Pattern.Type<string>,        # "try { ... } catch { ... }"
    Language.Type<n>,            # For parser selection
    Scope.Type<n>                # /file, /directory, /repo
).

Decl structural_match(
    Pattern.Type<string>,
    Path.Type<string>,
    Line.Type<int>
).
```

## 5. Constitution (Safety Layer)

### 5.1 permission_gate

Master gatekeeper - default deny.

```mangle
Decl permitted(Action.Type<n>).
Decl safe_action(Action.Type<n>).
Decl dangerous_action(Action.Type<n>).
Decl admin_override(User.Type<string>).
Decl signed_approval(Action.Type<n>).

# Safe actions are permitted
permitted(Action) :- safe_action(Action).

# Dangerous actions require override + approval
permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

# The Go runtime checks this before execution
# if !Mangle.Query("permitted(?)", action) { return AccessDenied }
```

### 5.2 network_policy

Data exfiltration prevention.

```mangle
Decl network_allowlist(Domain.Type<string>).
network_allowlist("github.com").
network_allowlist("pypi.org").
network_allowlist("crates.io").

security_violation(/exfiltration, URL) :-
    next_action(/network_request, URL),
    extract_domain(URL, Domain),
    not network_allowlist(Domain).
```

### 5.3 dangerous_action Classification

```mangle
dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "rm").

dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "sudo").

dangerous_action(Action) :-
    action_type(Action, /write_file),
    file_path(Action, Path),
    fn:string_contains(Path, ".env").
```

## 6. Sharding Schemas

### 6.1 shard_lifecycle

```mangle
Decl shard_lifecycle(
    ShardID.Type<n>,
    ShardType.Type<n>,           # /generalist, /specialist
    MountStrategy.Type<n>,       # /ram, /persistent_sqlite
    KnowledgeBase.Type<string>,  # Path to .db (null for generalists)
    Permissions.Type<string>     # Subset of parent permissions
).

# Specialists require knowledge base
invalid_shard(ShardID) :-
    shard_lifecycle(ShardID, /specialist, _, null, _).
```

### 6.2 delegate_task

```mangle
Decl delegate_task(
    ShardType.Type<n>,
    TaskDescription.Type<string>,
    Result.Type<string>
).

# Delegation triggers shard spawn
spawn_shard(ShardType) :-
    delegate_task(ShardType, _, _).
```

### 6.3 shard_profile (Persistent Specialists)

```mangle
Decl shard_profile(
    AgentName.Type<n>,           # /agent_rust, /agent_k8s
    Description.Type<string>,    # Mission statement
    ResearchKeywords.Type<string>, # Topics to master
    AllowedTools.Type<string>    # JSON array of tool names
).

# Research needed before instantiation
needs_research(Agent) :-
    shard_profile(Agent, _, Topics, _),
    not knowledge_ingested(Agent).
```

## 7. Memory & Context Schemas

### 7.1 activation (Spreading Activation)

```mangle
Decl activation(Fact.Type<n>, Score.Type<int>).
Decl new_fact(Fact.Type<n>).
Decl active_goal(Goal.Type<n>).

# Base activation from recency
activation(Fact, 100) :- new_fact(Fact).

# Spreading from goals to tools
activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# Recursive spread through dependencies
activation(FileB, Score2) :-
    activation(FileA, Score),
    Score > 40,
    dependency_link(FileA, FileB, _),
    Score2 = Score * 0.5.

# Context pruning
context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.
```

### 7.2 memory_operation

```mangle
Decl memory_operation(
    Operation.Type<n>,           # /promote_to_long_term, /forget, /archive
    Fact.Type<string>
).

# Promotion criteria
to_promote(Fact) :-
    mentioned_count(Fact, N),
    N > 3.

# Archive criteria
to_archive(Fact) :-
    last_accessed(Fact, Time),
    current_time(Now),
    Now - Time > 3600,
    not activation(Fact, _).
```

## 8. Piggyback Protocol Schema

### 8.1 dual_payload

```mangle
Decl surface_response(Text.Type<string>).
Decl control_packet(
    MangleUpdates.Type<string>,      # JSON array of atoms
    MemoryOperations.Type<string>,   # JSON array of ops
    AbductiveHypothesis.Type<string> # nullable
).
```

### 8.2 report_template

```mangle
Decl report_template(
    TaskComplexity.Type<int>,    # Derived from action count
    SuccessState.Type<bool>,
    ArtifactsCreated.Type<string> # JSON array of paths
).

# Template selection
use_summary_template() :-
    report_template(Complexity, _, _),
    Complexity > 5.

use_detailed_template() :-
    report_template(Complexity, _, _),
    Complexity <= 5.
```

## 9. Abductive Reasoning Schemas

```mangle
Decl symptom(Entity.Type<n>, Symptom.Type<n>).
Decl known_cause(Symptom.Type<n>, Cause.Type<n>).
Decl abductive_hypothesis(Hypothesis.Type<string>).

# Generate hypothesis when cause unknown
missing_hypothesis(RootCause) :-
    symptom(_, Symptom),
    not known_cause(Symptom, _),
    infer_cause(Symptom, RootCause).

# Trigger clarification
clarification_needed(Symptom) :-
    missing_hypothesis(Symptom).
```

## 10. Browser Peripheral Schemas

For the baked-in Semantic Browser:

```mangle
Decl dom_node(ID.Type<n>, Tag.Type<n>, Parent.Type<n>).
Decl attr(ID.Type<n>, Key.Type<string>, Val.Type<string>).
Decl geometry(ID.Type<n>, X.Type<int>, Y.Type<int>, W.Type<int>, H.Type<int>).
Decl computed_style(ID.Type<n>, Prop.Type<string>, Val.Type<string>).
Decl interactable(ID.Type<n>, Type.Type<n>).
Decl visible_text(ID.Type<n>, Text.Type<string>).

# Spatial reasoning example: "checkbox to the left of 'Agree'"
target_checkbox(CheckID) :-
    dom_node(CheckID, /input, _),
    attr(CheckID, "type", "checkbox"),
    visible_text(TextID, "Agree"),
    geometry(CheckID, Cx, _, _, _),
    geometry(TextID, Tx, _, _, _),
    Cx < Tx.
```
