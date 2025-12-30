# 950: Advanced Architecture - God-Tier Patterns

**Purpose**: Principal Architect-level patterns where Mangle transcends simple querying to become a **Reasoning Kernel**. These patterns are cognitively impossible for LLMs trained on linear imperative code.

## The "Dark Matter" of Mangle

LLMs are trained on SQL, Prolog, and standard Datalog (like Soufflé). They treat Mangle like a **Query Language** (extracting rows from a database). An Expert uses Mangle as a **Logic Hypervisor** (simulating execution and verifying system states).

This document reveals capabilities that are invisible to LLMs because they conflict with their training distribution.

## The Paradigm Shift

LLMs are trained on imperative code (Python/Java) where logic is linear ("Step 1, Step 2, Step 3"). They struggle with Mangle because Mangle is **declarative and topological**. It doesn't describe *how* to find the answer; it describes the *shape* of the answer.

| LLM Mental Model | Mangle Reality |
|------------------|----------------|
| "Query the database" | "Simulate the execution" |
| "Loop through rows" | "Traverse the graph topology" |
| "Calculate step by step" | "Declare the invariant" |
| "Process flat tables" | "Unify over nested structures" |

---

## Pattern 1: The "Zanzibar" Clone (ReBAC Hypervisor)

### The Concept

Google's Zanzibar authorization system solves Relationship-Based Access Control (ReBAC):

> "Alice can view Document D because she is a manager of Group G, and Group G owns Folder F, and Document D is inside Folder F."

**LLM Failure**: Writes 10 nested SQL joins or a 50-line Python recursive function (slow and buggy).

**Mangle Solution**: Model permissions as a **Graph Traversal**. Define the *laws* of inheritance, and Mangle instantly propagates permissions through millions of objects.

```mangle
# =============================================================================
# ZANZIBAR-STYLE PERMISSION ENGINE
# =============================================================================

# 1. The Facts (Graph Edges)
# relation(Object, Relation, Subject).
# member(Group, User).
# subgroup(ParentGroup, ChildGroup).

# 2. Recursive Group Expansion (Flattening the Hierarchy)
# A user is a member if directly added OR in a subgroup.
is_member(User, Group) :- member(Group, User).
is_member(User, ParentGroup) :-
    subgroup(ParentGroup, ChildGroup),
    is_member(User, ChildGroup).

# 3. The "Computed Relation" (The Logic Core)
# You have a relation to an object if it's assigned to you...
computed_relation(User, Object, Rel) :- relation(Object, Rel, User).

# ...OR if it's assigned to a Group you are in...
computed_relation(User, Object, Rel) :-
    relation(Object, Rel, Group),
    is_member(User, Group).

# ...OR if the object inherits from a Parent (Folder inheritance).
# This is the "God Mode" recursive step handling infinite folder depth.
computed_relation(User, Doc, Rel) :-
    relation(Doc, /parent, Folder),
    computed_relation(User, Folder, Rel).

# 4. The Policy Layer (The Rules)
# Owners are implicitly Editors; Editors are implicitly Viewers.
can_access(User, Obj, /view) :- computed_relation(User, Obj, /viewer).
can_access(User, Obj, /view) :- computed_relation(User, Obj, /editor).
can_access(User, Obj, /edit) :- computed_relation(User, Obj, /editor).
can_access(User, Obj, /edit) :- computed_relation(User, Obj, /owner).
can_access(User, Obj, /admin) :- computed_relation(User, Obj, /owner).
```

**Why This Is "Out of This World"**: You implemented a Google-scale permission engine in ~25 lines. It handles infinite nesting depth automatically. The engine computes the transitive closure of all permission paths.

---

## Pattern 2: The "Sybil Hunter" (Graph Topology Analysis)

### The Concept

Fraud detection involves finding "islands" of users who only transact with each other (money laundering rings) or share device fingerprints.

**LLM Failure**: Writes a query to find "Users who spent > $1000." It fails to see the *network structure*.

**Mangle Solution**: Build **Connected Components** (clusters) and score the *density* of those clusters.

```mangle
# =============================================================================
# FRAUD RING DETECTOR
# =============================================================================

# 1. Define the "Link"
# Two users are linked if they shared a device or traded money.
linked(A, B) :- device_login(A, Device), device_login(B, Device), A != B.
linked(A, B) :- transaction(A, B, _).
linked(A, B) :- transaction(B, A, _).  # Symmetric

# 2. Recursive Clustering (Transitive Closure)
# If A touches B, and B touches C, they are in the same cluster.
same_cluster(User, Peer) :- linked(User, Peer).
same_cluster(User, Peer) :- same_cluster(User, Mid), linked(Mid, Peer).

# 3. Leader Election (Canonicalization)
# To identify the cluster, pick the mathematically smallest UserID
# in the group to act as the "Syndicate ID".
syndicate_id(User, LeaderID) :-
    same_cluster(User, Peer) |>
    do fn:group_by(User),
    let LeaderID = fn:min(Peer).

# 4. Cluster Size Analysis
cluster_size(LeaderID, MemberCount) :-
    syndicate_id(_, LeaderID) |>
    do fn:group_by(LeaderID),
    let MemberCount = fn:count().

# 5. The "Fraud Ring" Trigger
# Logic: "High connectivity (edges) in a small group."
suspicious_ring(LeaderID, Size) :-
    cluster_size(LeaderID, Size),
    Size > 5,
    Size < 50.  # Too large = legitimate community
```

**Why This Is "Out of This World"**: This turns Mangle into a graph database. You are mathematically detecting "collusion" without machine learning—just pure logic topology.

---

## Pattern 3: The "Neuro-Symbolic" Mediator (Hybrid AI Logic)

### The Concept

Use an LLM to judge content (e.g., "Is this post toxic?"), but enforce hard business rules (e.g., "Never block VIP users").

**LLM Failure**: Probabilistic models sometimes hallucinate or ignore instructions. Coding agents struggle to combine "fuzzy" vector scores with "hard" boolean logic.

**Mangle Solution**: Mangle acts as the **Logic Hypervisor**. It calls the LLM (via a custom Go function) for "fuzzy" parts but overrides with deterministic rules for "safety" parts.

```mangle
# =============================================================================
# NEURO-SYMBOLIC CONTENT MODERATION
# =============================================================================

# 1. Hard Whitelist (The "Override")
allow_post(PostID) :- author(PostID, User), is_vip(User).
allow_post(PostID) :- author(PostID, User), is_verified_journalist(User).

# 2. Hard Blacklist (The "Safety Net")
deny_post(PostID) :- contains_banned_keyword(PostID).
deny_post(PostID) :- author(PostID, User), is_banned_user(User).

# 3. The "AI" Judgment (The Fuzzy Logic)
# 'fn:predict_toxicity' is a custom Go function bound to Mangle
# that calls an external model or API.
toxic_score(PostID, Score) :-
    post_content(PostID, Text),
    Score = fn:predict_toxicity(Text).

# 4. The Final Decision (Neuro-Symbolic Fusion)
# Block if AI says it's bad (>0.9) AND it's not whitelisted.
moderation_decision(PostID, /block) :-
    toxic_score(PostID, Score),
    Score > 0.9,
    !allow_post(PostID).  # Logic overrides AI

moderation_decision(PostID, /block) :- deny_post(PostID).

moderation_decision(PostID, /allow) :-
    post(PostID),
    !moderation_decision(PostID, /block).

# 5. Audit Trail
flagged_for_review(PostID, Score) :-
    toxic_score(PostID, Score),
    Score > 0.7,
    Score <= 0.9,
    !allow_post(PostID).
```

**Why This Is "Out of This World"**: Creates a safe sandbox for AI. The LLM provides the *signal*, but Mangle provides the *decision*, ensuring business policies (like VIP status) are mathematically guaranteed to be respected.

### FFI Deep Dive: Custom Go Function Binding

Mangle is not a closed database—it's a **Go library**. You can bind custom Go functions to Mangle predicates, allowing logic to reach into the real world during query execution.

```mangle
# 'fn:verify_ed25519' is NOT in the database.
# It is a Go function you injected into the engine.

allow_deployment(App, Bin) :-
    policy(App, /allowed_key, PubKey),
    binary_signature(Bin, Sig),

    # THE BRIDGE: Pass data out to Go, get a boolean back.
    # The logic engine halts if the Go function returns false.
    :check(fn:verify_ed25519(PubKey, Sig)).
```

This turns Mangle from a static database into a **Policy Hypervisor** that can gate real-time traffic.

---

## Pattern 4: The "Meta-Circular" Linter (Recursive AST Analysis)

### The Concept

Standard Datalog operates on flat tuples. Mangle operates on **Trees (Maps/Lists)**. This allows you to traverse deeply nested, irregular JSON-like structures (like Abstract Syntax Trees) natively in the logic.

**LLM Failure**: Tries to flatten the code into a table (`block_id, statement_id`) and loses the structure. Cannot visualize iterating through a list inside a map inside a recursive rule.

**Mangle Solution**: Use **Structural Unification** to walk the AST tree and detect patterns like goroutine leaks.

### The "Forgotten Sender" (Goroutine Leak Detector)

```mangle
# =============================================================================
# AST-BASED GOROUTINE LEAK DETECTOR
# =============================================================================

# 1. Base Case: Find the 'go' keyword (spawning a goroutine)
spawn_site(Node) :-
    :match_field(Node, /type, /GoStmt),
    :match_field(Node, /call, CallExpr).

# 2. Recursive Descent: Does this block have a return or channel close?
# We traverse the nested statements inside the goroutine's body.
has_exit_path(Block) :-
    :match_field(Block, /statements, Stmts),
    :list:member(Stmt, Stmts),  # Iterate the list
    is_return_or_close(Stmt).

is_return_or_close(Stmt) :- :match_field(Stmt, /type, /ReturnStmt).
is_return_or_close(Stmt) :- :match_field(Stmt, /type, /CloseStmt).
is_return_or_close(Stmt) :-
    :match_field(Stmt, /type, /SendStmt),
    :match_field(Stmt, /chan, Chan),
    channel_has_receiver(Chan).

# 3. The Leak Detector
# A leak exists if a spawn occurs, but the body has NO guaranteed exit path.
goroutine_leak(Node) :-
    spawn_site(Node),
    :match_field(Node, /body, Body),
    !has_exit_path(Body).

# 4. Recursive Block Analysis
# Handle nested blocks (if/for/switch bodies)
has_exit_path(Block) :-
    :match_field(Block, /statements, Stmts),
    :list:member(Stmt, Stmts),
    :match_field(Stmt, /body, InnerBlock),
    has_exit_path(InnerBlock).
```

**Why This Is "Out of This World"**: LLMs hallucinate flat joins. They cannot visualize the **Structural Unification** of iterating through a list (`Stmts`) inside a map (`Block`) inside a recursive rule. Mangle treats the AST as a connected graph and walks it mathematically.

### General AST Pattern: Nested Structure Traversal

```mangle
# Generic pattern for finding any node type in a nested AST
contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /type, TargetType),
    Node = Root.

contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /children, Children),
    :list:member(Child, Children),
    contains_node_type(Child, TargetType, Node).

contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /body, Body),
    contains_node_type(Body, TargetType, Node).

# Usage: Find all function calls in a file
function_call_in_file(FileAST, CallNode) :-
    file_ast(File, FileAST),
    contains_node_type(FileAST, /CallExpr, CallNode).
```

---

## Pattern 5: The "Poisoned River" (Recursive Taint Analysis)

### The Concept

Detect SQL injection vulnerabilities by tracking data flow from user input to SQL execution.

**LLM Failure**: Looks for *local* errors. Cannot see that a variable defined 10 files ago is "tainted" and flows into a sensitive function.

**Mangle Solution**: Ingest the application's **Call Graph** and **Data Flow** as facts. Calculate the **Transitive Closure** of data flow to mathematically prove unsafe paths.

```mangle
# =============================================================================
# TAINT ANALYSIS ENGINE
# =============================================================================

# 1. Define Sources (User Input) and Sinks (Dangerous Operations)
source(Var) :- var_annotation(Var, /user_input).
source(Var) :- function_return(Var, /http_request_body).
source(Var) :- function_return(Var, /query_param).

sink(Var, /sql_injection) :- var_context(Var, /sql_query).
sink(Var, /xss) :- var_context(Var, /html_output).
sink(Var, /command_injection) :- var_context(Var, /shell_exec).

# 2. Define Sanitization (The Filter)
# If data passes through a sanitizer, the output is clean.
sanitized(Out) :- function_call(/escape_sql, _, Out).
sanitized(Out) :- function_call(/html_encode, _, Out).
sanitized(Out) :- function_call(/validate_input, _, Out).

# 3. Recursive Flow (The River)
# Data flows from A to B if there is an assignment or function call.
flows_to(A, B) :- assignment(A, B).
flows_to(A, B) :- function_call(_, A, B).
flows_to(A, C) :- flows_to(A, B), flows_to(B, C).

# 4. The "Dirty" Path Detector
# A variable is tainted if it comes from a source AND
# has NOT been sanitized along the path.
tainted(Var) :- source(Var).
tainted(B) :-
    flows_to(A, B),
    tainted(A),
    !sanitized(B).

# 5. The Alert
vulnerability(Var, Type, /critical) :-
    sink(Var, Type),
    tainted(Var).

# 6. Path Reconstruction (for debugging)
taint_path(Src, Dst, [Src, Dst]) :-
    source(Src),
    flows_to(Src, Dst),
    sink(Dst, _).

taint_path(Src, Dst, [Src | Rest]) :-
    source(Src),
    flows_to(Src, Mid),
    taint_path(Mid, Dst, Rest).
```

**Why This Is "Out of This World"**: An LLM cannot maintain the state of a variable across 50 function calls. Mangle solves this instantly by treating the code as a connected graph.

---

## Pattern 6: The "Deadlock Hunter" (Cycle Detection)

### The Concept

Detect deadlocks where Thread A waits for Lock 1 (held by B), while Thread B waits for Lock 2 (held by A).

**LLM Failure**: Processes code linearly and cannot visualize structural loops.

**Mangle Solution**: Ingest lock acquisition order and define a rule that triggers if a "Wait-For" path points back to the starter.

```mangle
# =============================================================================
# DEADLOCK DETECTOR
# =============================================================================

# 1. Identify direct dependencies
# Thread T waits for Lock L, which is currently held by Thread S.
waits_for(ThreadA, ThreadB) :-
    attempts_lock(ThreadA, Lock),
    holds_lock(ThreadB, Lock),
    ThreadA != ThreadB.

# 2. Recursive Pathfinding
# If A waits for B, and B waits for C, then A effectively waits for C.
wait_chain(A, B) :- waits_for(A, B).
wait_chain(A, C) :- wait_chain(A, B), waits_for(B, C).

# 3. The "Ouroboros" Rule (Deadlock)
# If a thread ends up in a chain waiting for ITSELF, the system freezes.
deadlock_detected(Thread) :- wait_chain(Thread, Thread).

# 4. Find all threads involved in deadlock
in_deadlock(Thread) :- deadlock_detected(Thread).
in_deadlock(Thread) :-
    wait_chain(Thread, Other),
    deadlock_detected(Other).

# 5. Minimal deadlock cycle
deadlock_cycle(T1, T2) :-
    waits_for(T1, T2),
    wait_chain(T2, T1).
```

**Why This Is "Out of This World"**: To an LLM, `wait_chain(Thread, Thread)` looks like a hallucination or typo. To Mangle, it's a valid geometric property of the graph that mathematically proves a deadlock.

---

## Pattern 7: The "Bisimulation" (Regression Verification)

### The Concept

Verify that a code change doesn't accidentally remove security checks or expose new permissions.

**LLM Failure**: Diffs the *text* of Version 1 vs Version 2 (noisy). Misses logical equivalence.

**Mangle Solution**: Load the "Security Invariants" of V1 and V2 and ask: *"Does there exist a permission in V2 that was blocked in V1?"*

```mangle
# =============================================================================
# SECURITY REGRESSION DETECTOR
# =============================================================================

# 1. Establish the "Effective Reality" for both versions
# (The engine handles recursive expansion of groups/roles)

# Version 1 effective permissions
v1_allowed(User, Resource, Action) :-
    v1_policy(User, Role),
    v1_role_perm(Role, Resource, Action).

v1_allowed(User, Resource, Action) :-
    v1_policy(User, Role),
    v1_role_inherits(Role, ParentRole),
    v1_role_perm(ParentRole, Resource, Action).

# Version 2 effective permissions
v2_allowed(User, Resource, Action) :-
    v2_policy(User, Role),
    v2_role_perm(Role, Resource, Action).

v2_allowed(User, Resource, Action) :-
    v2_policy(User, Role),
    v2_role_inherits(Role, ParentRole),
    v2_role_perm(ParentRole, Resource, Action).

# 2. The "Privilege Escalation" Diff
# Show me any access that exists in V2 but NOT in V1.
security_regression(User, Resource, Action) :-
    v2_allowed(User, Resource, Action),
    !v1_allowed(User, Resource, Action).

# 3. The "Privilege Reduction" Diff (may be intentional)
permission_removed(User, Resource, Action) :-
    v1_allowed(User, Resource, Action),
    !v2_allowed(User, Resource, Action).

# 4. Critical Regressions (sensitive resources)
critical_regression(User, Resource, Action) :-
    security_regression(User, Resource, Action),
    sensitive_resource(Resource).
```

**Why This Is "Out of This World"**: This ignores whitespace, variable renaming, and refactoring. It only cares about the **Logical Truth**. If the AI "cleaned up" code but broke logic, Mangle catches it immediately.

---

## The "Event Horizon" (Where Mangle Stops)

To use Mangle effectively, you must know what it **cannot** do. These are the hard limits where "God Mode" crashes.

### 1. Infinite Generation (The Halting Problem)

You cannot write rules that generate unbounded new values:

```mangle
# ❌ WILL CRASH - Infinite generation
next_day(D) :- current_day(Old), D = fn:plus(Old, 1).
current_day(D) :- next_day(D).

# ❌ WILL CRASH - Unbounded counter
count_up(N) :- count_up(M), N = fn:plus(M, 1).
```

Mangle must reach a "fixpoint" (a state where no new facts are generated). If your rules generate new unique values forever, the engine runs until OOM.

**Workaround**: Always bind data to an existing finite set:
```mangle
# ✅ SAFE - Bounded by existing dates
valid_day(D) :- calendar_date(D).
next_valid_day(D, Next) :-
    valid_day(D),
    valid_day(Next),
    Next = fn:plus(D, 1).
```

### 2. No Efficient "Shortest Path" (Dijkstra)

Standard Datalog cannot efficiently calculate weighted shortest paths in graphs with cycles:

```mangle
# ❌ PROBLEMATIC - May loop infinitely summing weights
path_cost(X, Y, Cost) :- edge(X, Y, Cost).
path_cost(X, Z, TotalCost) :-
    edge(X, Y, Cost1),
    path_cost(Y, Z, Cost2),
    TotalCost = fn:plus(Cost1, Cost2).
# This generates ALL paths, not just shortest
```

**Workaround**:
- Use Mangle for *connectivity* (reachability)
- Use a custom Go function or external solver for *optimization* (min-cost)

```mangle
# ✅ SAFE - Find connectivity, then optimize externally
reachable(X, Y) :- edge(X, Y, _).
reachable(X, Z) :- edge(X, Y, _), reachable(Y, Z).

# Collect all paths, let Go code find minimum
all_paths(X, Y, Cost) :- path_cost(X, Y, Cost).
```

### 3. Non-Monotonicity (Time Travel)

Mangle is timeless. You cannot change a fact:

```mangle
# ❌ IMPOSSIBLE - Facts are immutable
# "user is active, then becomes inactive"
status(/alice, /active).   # Stratum 0
status(/alice, /inactive). # ??? Can't retract!
```

**Workaround**: Include a "Time" or "Version" variable:
```mangle
# ✅ SAFE - Time-versioned facts
status(/alice, /active, 100).
status(/alice, /inactive, 200).

current_status(User, Status) :-
    status(User, Status, Time),
    !newer_status(User, Time).

newer_status(User, Time) :-
    status(User, _, NewerTime),
    NewerTime > Time.
```

### 4. Monotonic Aggregation Only

You cannot aggregate inside a recursive cycle easily:

```mangle
# ❌ PROBLEMATIC - Sum changes, recursion re-triggers
total(X, Sum) :-
    item(X, Val),
    total(X, OldSum),
    Sum = fn:plus(OldSum, Val).
```

**Workaround**: Stratify. Calculate graph connections first, then aggregate:
```mangle
# ✅ SAFE - Two-stage approach
# Stage 1: Find all reachable items
reachable_item(Root, Item) :- direct_item(Root, Item).
reachable_item(Root, Item) :-
    link(Root, Child),
    reachable_item(Child, Item).

# Stage 2: Aggregate (separate stratum)
total_value(Root, Sum) :-
    reachable_item(Root, Item),
    item_value(Item, Val) |>
    do fn:group_by(Root),
    let Sum = fn:sum(Val).
```

### 5. Side Effects

Mangle is pure logic. It cannot perform I/O:

```mangle
# ❌ IMPOSSIBLE - No side effects
send_alert(User) :- violation(User).  # Can't actually send!
delete_file(Path) :- expired(Path).   # Can't actually delete!
```

**Workaround**: Mangle outputs a **Plan**. Your Go application reads facts and performs actions:
```mangle
# ✅ SAFE - Declare intent, don't execute
should_alert(User, Reason) :-
    violation(User, Reason).

should_delete(Path) :-
    file(Path),
    expired(Path).
```

```go
// Go code reads the plan and executes
alerts := kernel.Query("should_alert(User, Reason)")
for _, alert := range alerts {
    sendEmail(alert.User, alert.Reason)
}
```

---

## Summary: The Three Modes of Mangle

| Mode | Description | Use Case |
|------|-------------|----------|
| **Query Engine** | Find rows matching criteria | "Which users are admins?" |
| **Graph Engine** | Traverse topological structures | "What can Alice access?" |
| **Reasoning Kernel** | Simulate execution, verify invariants | "Prove no SQL injection path exists" |

LLMs default to Mode 1. Experts operate in Mode 3.

---

## Quick Reference: Pattern Selection

| Problem | Pattern | Key Technique |
|---------|---------|---------------|
| Permission inheritance | Zanzibar Clone | Recursive computed_relation |
| Fraud ring detection | Sybil Hunter | Connected components + leader election |
| AI + Business Rules | Neuro-Symbolic | FFI to ML model + hard overrides |
| Code analysis / linting | Meta-Circular Linter | Structural unification over AST trees |
| Security vuln detection | Poisoned River | Taint analysis via transitive closure |
| Concurrency bugs | Deadlock Hunter | Cycle detection in wait-for graph |
| Breaking change detection | Bisimulation | Policy diff across versions |

---

**Next**: See [GO_API_REFERENCE](GO_API_REFERENCE.md) for binding custom Go functions to Mangle predicates.
