# Shard Agents: Recursive Cognition & Fractal Parallelism

**Version:** 2.0.0 (Dec 2024 - JIT Clean Loop Architecture)
**Specification Section:** §6.0, §7.0, §9.0
**Implementation:** `internal/session/spawner.go`, `internal/session/subagent.go`

---

> ## ⚠️ MAJOR ARCHITECTURE CHANGE (Dec 2024)
>
> **Domain shards have been DELETED.** The following directories no longer exist:
> - `internal/shards/coder/`
> - `internal/shards/tester/`
> - `internal/shards/reviewer/`
> - `internal/shards/researcher/`
> - `internal/shards/nemesis/`
> - `internal/shards/tool_generator/`
>
> **Replaced by JIT Clean Loop:**
> - `internal/session/executor.go` - The clean execution loop (~50 lines)
> - `internal/session/spawner.go` - JIT-driven SubAgent spawning
> - `internal/session/subagent.go` - Context-isolated SubAgent implementation
> - `internal/prompt/config_factory.go` - Intent → tools/policies mapping
> - `internal/prompt/atoms/identity/*.yaml` - Persona atoms (coder, tester, reviewer, researcher)
> - `internal/mangle/intent_routing.mg` - Mangle routing rules for persona selection
>
> **Philosophy:** The LLM doesn't need 5000+ lines of Go code telling it how to be a coder/tester/reviewer. It needs JIT-compiled prompts with persona atoms, tool access via VirtualStore, and safety via Constitutional Gate. Everything else was cruft that hamstrung the LLM.
>
> The content below is preserved for **historical reference only**.

---

## 1. The Hypervisor Pattern (LEGACY)

codeNERD implements **Fractal Cognition** through ShardAgents—miniature, hyper-focused agent kernels that spawn, execute specific sub-tasks in total isolation, and return distilled logical results. The main Kernel acts as a **Hypervisor**: it does not think about *how* to solve a sub-problem; it only thinks about *who* to assign it to.

### 1.1 Why Sharding?

| Problem | Solution |
|---------|----------|
| **Context Window Exhaustion** | Shards have isolated context; parent window stays clean |
| **Reasoning Dilution** | Specialized shards focus on one task with domain expertise |
| **Sequential Bottleneck** | Multiple shards run in parallel (up to `maxConcurrent`) |
| **Monolithic Failure** | Shard failure is isolated; parent can retry or delegate elsewhere |
| **One-Size-Fits-All Models** | Each shard type has purpose-driven model selection |

### 1.2 Architecture Overview

```text
                           ┌─────────────────────────┐
                           │      Main Kernel        │
                           │  (The Hypervisor)       │
                           └───────────┬─────────────┘
                                       │
                   ┌───────────────────┼───────────────────┐
                   │                   │                   │
           ┌───────▼───────┐   ┌───────▼───────┐   ┌───────▼───────┐
           │ ShardManager  │   │ ShardManager  │   │ ShardManager  │
           │               │   │               │   │               │
           │  Type 1       │   │  Type 2-4     │   │  Campaigns    │
           │  (System)     │   │  (Task)       │   │  (Campaign)   │
           └───────┬───────┘   └───────┬───────┘   └───────┬───────┘
                   │                   │                   │
    ┌──────────────┼──────────────┐   │    ┌──────────────┼──────────────┐
    │              │              │   │    │              │              │
┌───▼───┐    ┌─────▼────┐    ┌───▼───▼───┐   ┌───▼───┐   ┌───▼───┐
│Percept│    │WorldModel│    │ Executive │   │Coder  │   │Tester │
│Shard  │    │Shard     │    │ Shard     │   │Shard  │   │Shard  │
└───────┘    └──────────┘    └───────────┘   └───────┘   └───────┘
```

---

## 2. The Four Shard Types

### 2.1 Type 1: System Level (Permanent)

**The Operating System of the Agent**

System shards run continuously in the background, monitoring the environment and maintaining system homeostasis. They are the "always-on" infrastructure that supports all other operations.

```go
// internal/core/shard_manager.go:16-19
const (
    // Type 1: System Level (Permanent)
    // Lifecycle: Always On
    // Memory: Persistent, High-Performance
    ShardTypeSystem ShardType = "system"
)
```

| Property | Value |
|----------|-------|
| **Lifecycle** | Always On (24h timeout) |
| **Memory** | Persistent, High-Performance |
| **Model** | Balanced (Gemini Flash, Claude Sonnet) |
| **Concurrency** | Parallel background loops |
| **Context** | Shared with parent kernel |

**Built-in System Shards:**

| Shard Name | Role | Permissions |
|------------|------|-------------|
| `perception_firewall` | Transduce NL → Logic atoms | `read_file`, `ask_user` |
| `world_model_ingestor` | Maintain file/symbol/diagnostic facts | `read_file`, `exec_cmd`, `code_graph` |
| `executive_policy` | Derive next_action from strategy | `read_file`, `code_graph`, `ask_user` |
| `constitution_gate` | Enforce safety, block violations | `ask_user` |
| `tactile_router` | Plan tool calls with allowlists | `exec_cmd`, `network`, `browser` |
| `session_planner` | Maintain agenda, checkpoints | `ask_user`, `read_file` |

**System Shard Execution Loop:**

```go
// internal/core/system_shard.go:38-68
func (s *SystemShard) Execute(ctx context.Context, task string) (string, error) {
    s.SetState(ShardStateRunning)
    defer s.SetState(ShardStateCompleted)

    // Prime with LLM call to seed role-specific intent
    if llm := s.llm(); llm != nil {
        userPrompt := fmt.Sprintf("System Startup. Task: %s. Status: Online.", task)
        _, _ = llm.CompleteWithSystem(ctx, s.systemPrompt, userPrompt)
    }

    // System Shard Main Loop (doesn't exit after one task)
    ticker := time.NewTicker(10 * time.Second) // Heartbeat
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return "System Shard shutdown", ctx.Err()
        case <-s.stopCh:
            return "System Shard stopped", nil
        case tick := <-ticker.C:
            // Propagate heartbeat fact to parent kernel
            _ = s.kernel.Assert(Fact{
                Predicate: "system_heartbeat",
                Args:      []interface{}{s.id, tick.Unix()},
            })
        }
    }
}
```

---

### 2.2 Type 2: Ephemeral (LLM Created, Non-Persistent)

**The "Interns"**

Ephemeral shards are lightweight, fast-spawning agents for quick tasks. They have no persistent memory and die immediately after completing their task.

```go
// internal/core/shard_manager.go:21-24
const (
    // Type 2: Ephemeral (LLM Created, Non-Persistent)
    // Lifecycle: Spawn -> Execute Task -> Die
    // Memory: RAM Only
    ShardTypeEphemeral ShardType = "ephemeral"
)
```

| Property | Value |
|----------|-------|
| **Lifecycle** | Spawn → Execute → Die |
| **Memory** | RAM Only (1000 facts max) |
| **Model** | High Speed (Gemini Flash, Claude Haiku) |
| **Timeout** | 5 minutes |
| **Context** | Completely isolated (zero pollution) |

**Use Cases:**
- Quick refactoring of a single function
- Writing a unit test for a specific method
- Fixing a typo or small bug
- Running a single linter check
- Generating boilerplate code

**Default Configuration:**

```go
// internal/core/shard_manager.go:91-106
func DefaultGeneralistConfig(name string) ShardConfig {
    return ShardConfig{
        Name: name,
        Type: ShardTypeEphemeral,
        Permissions: []ShardPermission{
            PermissionReadFile,
            PermissionWriteFile,
            PermissionExecCmd,
        },
        Timeout:     5 * time.Minute,
        MemoryLimit: 1000,
        Model: ModelConfig{
            Capability: CapabilityHighSpeed, // Default to speed
        },
    }
}
```

---

### 2.3 Type 3: Persistent (LLM Created)

**The Background Workers**

Persistent shards are created by the LLM for long-running background tasks. They maintain task-specific SQLite storage and can survive session boundaries.

```go
// internal/core/shard_manager.go:26-29
const (
    // Type 3: Persistent (LLM Created)
    // Lifecycle: Long-running background tasks
    // Memory: Persistent SQLite (Task-Specific)
    ShardTypePersistent ShardType = "persistent"
)
```

| Property | Value |
|----------|-------|
| **Lifecycle** | Long-running, survives sessions |
| **Memory** | Persistent SQLite (task-specific) |
| **Model** | Balanced (Gemini Flash-8B, Claude Sonnet) |
| **Timeout** | Configurable (up to hours) |
| **Context** | Isolated with checkpoint/resume |

**Use Cases:**
- Long-running test suites
- Background code analysis
- Continuous integration tasks
- Incremental compilation watching
- Large codebase migrations

**State Machine:**

```text
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│  IDLE   │────▶│SPAWNING │────▶│ RUNNING │────▶│COMPLETED│
└─────────┘     └─────────┘     └────┬────┘     └─────────┘
                                     │
                                     ▼
                               ┌─────────┐
                               │SLEEPING │ (Can resume)
                               └─────────┘
```

---

### 2.4 Type 4: User Configured (Persistent Expert)

**The "Experts"**

User-configured shards are defined via CLI and pre-populated with deep domain knowledge. They are the "PhD holders" of the agent ecosystem.

```go
// internal/core/shard_manager.go:31-34
const (
    // Type 4: User Configured (Persistent)
    // Lifecycle: Explicitly defined by User CLI
    // Memory: Deep Domain Knowledge
    ShardTypeUser ShardType = "user"
)
```

| Property | Value |
|----------|-------|
| **Lifecycle** | Defined → Hydrated → Sleeping → Waking → Sleeping |
| **Memory** | Deep Domain Knowledge (SQLite Knowledge Shards) |
| **Model** | High Reasoning (Gemini Pro, Claude Opus) |
| **Timeout** | 30 minutes |
| **Context** | Pre-populated with research results |

**Use Cases:**
- RustExpert: Deep knowledge of Rust compiler internals
- SecurityAuditor: OWASP, CVE databases, vulnerability patterns
- K8sArchitect: Kubernetes best practices, deployment patterns
- ReactOptimizer: React performance patterns, hooks internals

**Default Configuration:**

```go
// internal/core/shard_manager.go:108-127
func DefaultSpecialistConfig(name, knowledgePath string) ShardConfig {
    return ShardConfig{
        Name: name,
        Type: ShardTypeUser,
        Permissions: []ShardPermission{
            PermissionReadFile,
            PermissionWriteFile,
            PermissionExecCmd,
            PermissionCodeGraph,
            PermissionResearch,
        },
        KnowledgePath: knowledgePath,
        Timeout:       30 * time.Minute,
        MemoryLimit:   10000,
        Model: ModelConfig{
            Capability: CapabilityHighReasoning, // Experts need high reasoning
        },
    }
}
```

**CLI Definition:**

```bash
# Define a new specialist
nerd define-agent --name "RustExpert" --topic "Tokio Async Runtime"

# This triggers:
# 1. Creates shard_profile atom in EDB
# 2. Triggers perform_deep_research virtual predicate
# 3. Research results written to .nerd/shards/RustExpert_knowledge.db
# 4. Knowledge shard mounted on future spawns
```

---

## 3. Mangle Policy Rules

### 3.1 Shard Type Classification

```mangle
# SECTION 18: SHARD TYPE CLASSIFICATION (§6.1 Taxonomy)

# Type 1: System Level - Always on, high reliability
shard_type(/system, /permanent, /high_reliability).

# Type 2: Ephemeral - Fast spawning, RAM only
shard_type(/ephemeral, /spawn_die, /speed_optimized).

# Type 3: Persistent LLM-Created - Background tasks, SQLite
shard_type(/persistent, /long_running, /adaptive).

# Type 4: User Configured - Deep domain knowledge
shard_type(/user, /explicit, /user_defined).
```

### 3.2 Model Capability Mapping

```mangle
# Model capability mapping for shards
shard_model_config(/system, /high_reasoning).
shard_model_config(/ephemeral, /high_speed).
shard_model_config(/persistent, /balanced).
shard_model_config(/user, /high_reasoning).
```

### 3.3 Delegation Rules

```mangle
# SECTION 8: SHARD DELEGATION

# Delegate code tasks to coder shard
delegate_to_shard(/coder) :-
    user_intent(_, /mutation, Verb, _, _),
    code_mutation_verb(Verb).

code_mutation_verb(/refactor).
code_mutation_verb(/fix).
code_mutation_verb(/implement).
code_mutation_verb(/generate).

# Delegate test tasks to tester shard
delegate_to_shard(/tester) :-
    user_intent(_, _, /test, _, _).

delegate_to_shard(/tester) :-
    test_state(/failing),
    retry_count(N), N < 3.

# Delegate review tasks to reviewer shard
delegate_to_shard(/reviewer) :-
    user_intent(_, /query, /review, _, _).

# Delegate research to researcher shard
delegate_to_shard(/researcher) :-
    user_intent(_, /query, /research, _, _).

delegate_to_shard(/researcher) :-
    missing_hypothesis(_),
    requires_domain_knowledge().
```

---

## 4. Built-in Shard Implementations

### 4.1 CoderShard

**Purpose:** Code writing, modification, and refactoring
**Type:** Typically Type 2 (Ephemeral) or Type 4 (User-configured specialist)
**Implementation:** [internal/shards/coder.go](internal/shards/coder.go)

```go
type CoderShard struct {
    mu sync.RWMutex

    // Identity
    id     string
    config core.ShardConfig
    state  core.ShardState

    // Coder-specific
    coderConfig CoderConfig

    // Components - each shard has its own kernel
    kernel       *core.RealKernel
    llmClient    perception.LLMClient
    virtualStore *core.VirtualStore

    // State tracking
    startTime   time.Time
    editHistory []CodeEdit
    diagnostics []core.Diagnostic

    // Learnings for autopoiesis
    rejectionCount  map[string]int
    acceptanceCount map[string]int
}
```

**Key Features:**
- Language-agnostic (auto-detects from file extension)
- Impact analysis before writes
- Autopoiesis tracking (rejection/acceptance counts)
- TDD integration (build checks after edits)

---

### 4.2 TesterShard

**Purpose:** Running tests, analyzing failures, generating test code
**Type:** Type 2 (Ephemeral) for quick runs, Type 3 (Persistent) for long suites
**Implementation:** [internal/shards/tester.go](internal/shards/tester.go)

**Key Features:**
- Multi-framework support (Go test, Jest, pytest, cargo test)
- Failure analysis and root cause detection
- Coverage tracking
- Retry logic with exponential backoff

---

### 4.3 ReviewerShard

**Purpose:** Code review, quality analysis, architectural feedback
**Type:** Type 2 (Ephemeral) for quick reviews, Type 4 (User) for deep expertise
**Implementation:** [internal/shards/reviewer.go](internal/shards/reviewer.go)

**Key Features:**
- Multi-dimension scoring (readability, performance, security, maintainability)
- Pattern detection (anti-patterns, code smells)
- Diff-based review (only analyze changed code)
- Architectural consistency checks

---

### 4.4 ResearcherShard

**Purpose:** Deep web research to build knowledge shards
**Type:** Type 3 (Persistent) or Type 4 (User-configured)
**Implementation:** [internal/shards/researcher/](internal/shards/researcher/)

```go
type ResearcherShard struct {
    mu sync.RWMutex

    // Identity
    id     string
    config core.ShardConfig
    state  core.ShardState

    // Research-specific
    researchConfig ResearchConfig
    httpClient     *http.Client

    // Components
    kernel    *core.RealKernel
    scanner   *world.Scanner
    llmClient perception.LLMClient
    localDB   *store.LocalStore

    // Tracking
    startTime   time.Time
    stopCh      chan struct{}
    visitedURLs map[string]bool
}
```

**Key Features:**
- Multi-page web scraping with depth control
- Domain allowlist/blocklist
- Knowledge atom extraction
- SQLite persistence for knowledge shards
- Concurrent fetching with rate limiting

**Research Workflow:**

```text
1. QUERY ANALYSIS
   └── Extract keywords from research query

2. WEB SEARCH
   └── Query search engines for relevant pages

3. PAGE TRAVERSAL
   └── BFS crawl with MaxDepth and MaxPages limits

4. CONTENT EXTRACTION
   └── Parse HTML, extract text, identify code blocks

5. KNOWLEDGE SYNTHESIS
   └── LLM summarizes into KnowledgeAtom structs

6. PERSISTENCE
   └── Write atoms to .nerd/shards/{agent}_knowledge.db

7. VERIFICATION (Viva Voce)
   └── Generate questions, verify shard can answer
```

---

## 5. ShardManager API

### 5.1 Core Methods

```go
// Spawn synchronously spawns a shard and waits for result
func (sm *ShardManager) Spawn(ctx context.Context, shardType string, task string) (string, error)

// SpawnAsync spawns a shard in background, returns ID immediately
func (sm *ShardManager) SpawnAsync(ctx context.Context, shardType string, task string) (string, error)

// GetResult retrieves result of completed shard
func (sm *ShardManager) GetResult(shardID string) (*ShardResult, bool)

// DefineProfile defines a specialist shard profile
func (sm *ShardManager) DefineProfile(name string, config ShardConfig)

// StartSystemShards starts all Type 1 system shards
func (sm *ShardManager) StartSystemShards(ctx context.Context) error

// StopAll stops all active shards
func (sm *ShardManager) StopAll()
```

### 5.2 Shard Result

```go
type ShardResult struct {
    ShardID   string
    Task      string
    Output    string
    Error     error
    Duration  time.Duration
    Facts     []Fact       // Facts to propagate back to parent
    Timestamp time.Time
}
```

### 5.3 Usage Example

```go
// Create shard manager
sm := core.NewShardManager()
sm.SetLLMClient(llmClient)
sm.SetLearningStore(learningStore)
sm.SetParentKernel(kernel)

// Start system shards (Type 1)
sm.StartSystemShards(ctx)

// Spawn ephemeral coder (Type 2)
result, err := sm.Spawn(ctx, "coder", "refactor file:auth.go spec:extract_validation")

// Define specialist (Type 4)
sm.DefineProfile("RustExpert", core.DefaultSpecialistConfig("RustExpert", ".nerd/shards/rust_knowledge.db"))

// Spawn specialist
result, err := sm.Spawn(ctx, "RustExpert", "review file:async_handler.rs")

// Spawn async and get result later
shardID, _ := sm.SpawnAsync(ctx, "tester", "run tests in internal/core")
// ... do other work ...
result, ok := sm.GetResult(shardID)
```

---

## 6. Shard Permissions

### 6.1 Permission Types

```go
const (
    PermissionReadFile  ShardPermission = "read_file"
    PermissionWriteFile ShardPermission = "write_file"
    PermissionExecCmd   ShardPermission = "exec_cmd"
    PermissionNetwork   ShardPermission = "network"
    PermissionBrowser   ShardPermission = "browser"
    PermissionCodeGraph ShardPermission = "code_graph"
    PermissionResearch  ShardPermission = "research"
    PermissionAskUser   ShardPermission = "ask_user"
)
```

### 6.2 Permission Inheritance

Shards inherit a **subset** of parent permissions—they can never escalate:

```text
Parent Kernel Permissions: [read_file, write_file, exec_cmd, network, browser]
                                       │
                                       ▼
         ┌─────────────────────────────┴─────────────────────────────┐
         │                                                           │
    CoderShard                                               ResearcherShard
    [read_file, write_file, exec_cmd]                        [read_file, network, browser]
         │                                                           │
         ▼                                                           ▼
    CANNOT escalate to network                               CANNOT escalate to write_file
```

### 6.3 Constitutional Enforcement

```mangle
# Shard cannot escalate its own permissions
block_action(ShardID, Action, "permission_escalation") :-
    shard_action(ShardID, Action),
    action_requires_permission(Action, Perm),
    !shard_has_permission(ShardID, Perm).

shard_has_permission(ShardID, Perm) :-
    active_shard(ShardID, _, _),
    shard_permission(ShardID, Perm).
```

---

## 7. Shard Lifecycle States

```go
const (
    ShardStateIdle      ShardState = "idle"       // Created, not started
    ShardStateSpawning  ShardState = "spawning"   // Being initialized
    ShardStateRunning   ShardState = "running"    // Actively executing
    ShardStateCompleted ShardState = "completed"  // Finished successfully
    ShardStateFailed    ShardState = "failed"     // Finished with error
    ShardStateSleeping  ShardState = "sleeping"   // Persistent, waiting (Type 3/4)
    ShardStateHydrating ShardState = "hydrating"  // Loading knowledge shard (Type 4)
)
```

**State Transitions:**

```text
              Type 2 (Ephemeral)                    Type 3/4 (Persistent)
              ─────────────────                     ─────────────────────

                   ┌─────┐                               ┌─────┐
                   │IDLE │                               │IDLE │
                   └──┬──┘                               └──┬──┘
                      │                                    │
                      ▼                                    ▼
                 ┌─────────┐                          ┌─────────┐
                 │SPAWNING │                          │HYDRATING│ (Load knowledge)
                 └────┬────┘                          └────┬────┘
                      │                                    │
                      ▼                                    ▼
                 ┌─────────┐                          ┌─────────┐
                 │ RUNNING │                          │ RUNNING │
                 └────┬────┘                          └────┬────┘
                      │                                    │
           ┌──────────┼──────────┐              ┌──────────┼──────────┐
           ▼          ▼          ▼              ▼          ▼          ▼
      ┌─────────┐┌────────┐           ┌─────────┐┌────────┐┌─────────┐
      │COMPLETED││ FAILED │           │COMPLETED││ FAILED ││SLEEPING │
      └─────────┘└────────┘           └─────────┘└────────┘└────┬────┘
                                                                │
           (Dies immediately)                                   │ (Wake)
                                                                ▼
                                                           ┌─────────┐
                                                           │ RUNNING │
                                                           └─────────┘
```

---

## 8. Model Selection Strategy

### 8.1 Purpose-Driven Selection

```go
type ModelCapability string

const (
    CapabilityHighReasoning ModelCapability = "high_reasoning" // Opus/Pro
    CapabilityHighSpeed     ModelCapability = "high_speed"     // Haiku/Flash
    CapabilityBalanced      ModelCapability = "balanced"       // Sonnet/Flash-8B
)
```

### 8.2 Type-to-Model Mapping

| Shard Type | Default Capability | Rationale |
|------------|-------------------|-----------|
| Type 1 (System) | Balanced | Continuous operation, reliability |
| Type 2 (Ephemeral) | High Speed | Quick tasks, minimize latency |
| Type 3 (Persistent) | Balanced | Long-running, cost-effective |
| Type 4 (User) | High Reasoning | Deep expertise requires best models |

### 8.3 Provider Examples

| Capability | Anthropic | Google | OpenAI |
|------------|-----------|--------|--------|
| High Reasoning | Claude Opus | Gemini 1.5 Pro | GPT-4o |
| Balanced | Claude Sonnet | Gemini 1.5 Flash-8B | GPT-4o-mini |
| High Speed | Claude Haiku | Gemini 1.5 Flash | GPT-3.5-turbo |

---

## 9. Autopoiesis Integration

### 9.1 Learning Hydration

When a shard spawns, it's hydrated with learnings from previous sessions:

```go
// internal/core/shard_manager.go:744-792
func (sm *ShardManager) hydrateWithLearnings(shard ShardAgent, shardType string) {
    // Load learnings for this shard type
    learnings, err := store.Load(shardType)

    // Assert each learning as a fact in the kernel
    for _, learning := range learnings {
        _ = kernel.Assert(Fact{
            Predicate: "learned_" + learning.FactPredicate,
            Args:      learning.FactArgs,
        })

        // High-confidence learnings become strong preferences
        if learning.Confidence >= 0.7 {
            _ = kernel.Assert(Fact{
                Predicate: "strong_preference",
                Args:      []interface{}{learning.FactPredicate, learning.FactArgs},
            })
        }
    }
}
```

### 9.2 Learning Extraction

After shard completion, learnings are extracted and persisted:

```go
// internal/core/shard_manager.go:705-742
func (sm *ShardManager) processLearnings(shardType string, result *ShardResult) {
    for _, fact := range result.Facts {
        if fact.Predicate == "promote_to_long_term" && len(fact.Args) >= 2 {
            factPredicate := fact.Args[0].(string)
            factArgs := fact.Args[1:]

            // Persist the learning
            store.Save(shardType, factPredicate, factArgs, campaignID)
        }
    }
}
```

---

## 10. Fact Propagation

### 10.1 Child → Parent

Shards can propagate facts back to the parent kernel:

```go
// internal/core/shard_manager.go:896-916
func (sm *ShardManager) PropagateFactsToParent(result *ShardResult) error {
    for _, fact := range result.Facts {
        if err := kernel.Assert(fact); err != nil {
            return err
        }
    }

    // Record delegation result
    return kernel.Assert(Fact{
        Predicate: "delegation_result",
        Args:      []interface{}{result.ShardID, result.Output, result.Error == nil},
    })
}
```

### 10.2 Parent → Child

Parent kernel can inject facts into shard context during spawn (via task description or explicit injection).

---

## 11. Concurrency Control

### 11.1 Semaphore-Based Limiting

```go
type ShardManager struct {
    // ...
    maxConcurrent int
    semaphore     chan struct{}
}

// Acquire before spawning
select {
case sm.semaphore <- struct{}{}:
    defer func() { <-sm.semaphore }()
case <-ctx.Done():
    return "", ctx.Err()
}
```

### 11.2 Default Limits

| Configuration | Default Value |
|--------------|---------------|
| `maxConcurrent` | 10 |
| System shard timeout | 24 hours |
| Ephemeral shard timeout | 5 minutes |
| Persistent shard timeout | Configurable |
| User shard timeout | 30 minutes |

---

## 12. Summary

| Type | Lifecycle | Memory | Model | Use Case |
|------|-----------|--------|-------|----------|
| **Type 1: System** | Always On | Persistent | Balanced | Infrastructure, monitoring |
| **Type 2: Ephemeral** | Spawn → Die | RAM Only | High Speed | Quick tasks, zero pollution |
| **Type 3: Persistent** | Long-running | SQLite | Balanced | Background tasks, checkpoints |
| **Type 4: User** | Defined → Hydrate → Sleep | Knowledge Shard | High Reasoning | Deep expertise, specialists |

The shard architecture enables codeNERD to:
1. **Scale reasoning** through parallel execution
2. **Preserve context** by isolating task-specific state
3. **Match models to tasks** via purpose-driven selection
4. **Learn continuously** through autopoiesis integration
5. **Maintain safety** through permission inheritance and constitutional enforcement
