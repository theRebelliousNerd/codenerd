# 10-Point Wiring Checklist

Every new feature in codeNERD must connect to these integration points. Use this checklist to audit completeness.

---

## Quick Audit Form

Copy this checklist when auditing a new feature:

```markdown
## Feature Audit: [Feature Name]

### 1. Logging
- [ ] Uses appropriate category from 22 available
- [ ] Info-level for normal operations
- [ ] Debug-level for detailed traces
- [ ] Error-level with context for failures
- [ ] Timer helpers for performance-critical paths

### 2. Shard Registration (if applicable)
- [ ] Factory in registration.go
- [ ] Profile defined with correct ShardType
- [ ] All dependencies injected (Kernel, LLM, VirtualStore, [LearningStore])
- [ ] Interface fully implemented

### 3. Kernel/Schema (if new predicates)
- [ ] Decl added to schemas.mg
- [ ] Rules added to policy.gl (if derived)
- [ ] Fact generation code uses ToAtom()

### 4. Virtual Predicates (if external data)
- [ ] Decl in schemas.mg
- [ ] Resolution in VirtualStore.Get()
- [ ] Documentation in comments

### 5. CLI Commands (if user-facing)
- [ ] Handler in chat/commands.go
- [ ] Help text updated
- [ ] Verb in transducer VerbCorpus (if NL-triggered)

### 6. Transducer (if NL-triggered)
- [ ] Verb added to VerbCorpus
- [ ] Synonyms defined
- [ ] Patterns compiled
- [ ] ShardType mapping correct

### 6.5 JIT Prompt Compiler (if Type B/U shard)
- [ ] JIT Compiler initialized in session bootstrap
- [ ] SetJITCompiler() called on PromptAssembler
- [ ] ShardManager has promptLoader callback set
- [ ] ShardManager has jitRegistrar callback set
- [ ] ShardManager has jitUnregistrar callback set
- [ ] Type B/U shard has prompts.yaml in .nerd/agents/{name}/
- [ ] prompt_atoms table created in shard KB
- [ ] Corpus fragments loaded correctly

### 7. Actions (if new capabilities)
- [ ] ActionType constant defined
- [ ] Handler in VirtualStore.Execute()
- [ ] Permission check implemented

### 8. Tests
- [ ] Unit tests for core logic
- [ ] Integration test for wiring
- [ ] Live test that exercises full path

### 9. Config (if configurable)
- [ ] Config struct field added
- [ ] Default value set
- [ ] Documentation in config reference
```

---

## Point 1: Logging

**Location:** `internal/logging/logger.go`
**Reference:** [logging-system.md](../../codenerd-builder/references/logging-system.md)

### The 22 Categories

| Category | Constant | Use For |
|----------|----------|---------|
| `boot` | `CategoryBoot` | Startup, initialization |
| `session` | `CategorySession` | Session lifecycle |
| `kernel` | `CategoryKernel` | Mangle engine operations |
| `api` | `CategoryAPI` | LLM API calls |
| `perception` | `CategoryPerception` | NL -> Atoms |
| `articulation` | `CategoryArticulation` | Atoms -> NL |
| `routing` | `CategoryRouting` | Action dispatch |
| `tools` | `CategoryTools` | Tool execution |
| `virtual_store` | `CategoryVirtualStore` | FFI layer |
| `shards` | `CategoryShards` | Shard manager |
| `coder` | `CategoryCoder` | CoderShard |
| `tester` | `CategoryTester` | TesterShard |
| `reviewer` | `CategoryReviewer` | ReviewerShard |
| `researcher` | `CategoryResearcher` | ResearcherShard |
| `system_shards` | `CategorySystemShards` | System shard ops |
| `dream` | `CategoryDream` | Dream state |
| `autopoiesis` | `CategoryAutopoiesis` | Self-improvement |
| `campaign` | `CategoryCampaign` | Multi-phase goals |
| `context` | `CategoryContext` | Context compression |
| `world` | `CategoryWorld` | Filesystem projection |
| `embedding` | `CategoryEmbedding` | Vector operations |
| `store` | `CategoryStore` | Memory tier CRUD |

### Correct Usage

```go
// Category-specific convenience functions
logging.Kernel("Asserting fact: %s", fact.String())
logging.KernelDebug("Rule body: %v", ruleBody)

// Logger instance for more control
logger := logging.Get(logging.CategoryShards)
logger.Info("Spawning shard: id=%s type=%s", id, shardType)
logger.Error("Spawn failed: %v", err)

// Context logging for correlated events
ctx := logger.WithContext(map[string]interface{}{
    "shard_id":   shardID,
    "shard_type": shardType,
})
ctx.Info("Starting execution")
ctx.Error("Execution failed: %v", err)

// Performance timing
timer := logging.StartTimer(logging.CategoryAPI, "LLM request")
defer timer.StopWithThreshold(5 * time.Second)
```

### Common Failures

| Failure | Example | Fix |
|---------|---------|-----|
| Wrong category | Using `/shards` for kernel ops | Match category to component |
| Missing context | `"Error occurred"` | Include IDs, types, values |
| No debug logging | Only Info level | Add Debug for traces |
| Missing timer | No perf data for slow ops | Add StartTimer() |

---

## Point 2: Shard Registration

**Location:** `internal/shards/registration.go`
**Reference:** [shard-wiring-matrix.md](shard-wiring-matrix.md)

### Registration Steps

```go
// 1. Import your shard package
import "codenerd/internal/shards/myshard"

// 2. Add factory in RegisterAllShardFactories()
sm.RegisterShard("myshard", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := myshard.NewMyShard()
    shard.SetParentKernel(ctx.Kernel)
    shard.SetLLMClient(ctx.LLMClient)
    shard.SetVirtualStore(ctx.VirtualStore)
    // For Type B/U: shard.SetLearningStore(learningStore)
    return shard
})

// 3. Define profile in defineShardProfiles()
sm.DefineProfile("myshard", core.ShardConfig{
    Name:        "myshard",
    Type:        core.ShardTypeEphemeral,  // A, B, U, or S
    Permissions: []core.ShardPermission{...},
    Timeout:     10 * time.Minute,
    MemoryLimit: 12000,
    Model: core.ModelConfig{
        Capability: core.CapabilityHighReasoning,
    },
})
```

### Required Interface

```go
type ShardAgent interface {
    Execute(ctx context.Context, task string) (string, error)
    GetID() string
    GetState() ShardState
    GetConfig() ShardConfig
    Stop() error
    SetParentKernel(kernel Kernel)
    SetLLMClient(client perception.LLMClient)
    SetSessionContext(ctx *SessionContext)
}
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Factory not registered | "unknown shard type" | Add to `RegisterAllShardFactories()` |
| Profile not defined | Config is empty | Call `DefineProfile()` |
| Missing injection | Nil pointer in shard | Inject all dependencies |
| Wrong shard type | Unexpected lifecycle | Match Type A/B/U/S to needs |

---

## Point 3: Kernel/Schema

**Location:** `internal/core/defaults/schemas.mg`, `internal/mangle/policy.gl`
**Reference:** [mangle-programming skill](../../mangle-programming/SKILL.md)

### Declaration Pattern

```mangle
# In schemas.mg - EDB (Extensional Database)
# Every predicate MUST be declared before use

# Basic declaration
Decl my_predicate(Arg1, Arg2, Arg3).

# With types
Decl typed_pred(ID.Type<string>, Count.Type<int>, Active.Type<name>).
```

### Rule Pattern

```mangle
# In policy.gl - IDB (Intensional Database)
# Derived facts computed from rules

# Simple rule
derived_pred(X, Y) :- base_pred(X, Y), condition(X).

# With negation (all vars must be bound)
filtered(X) :- candidate(X), !excluded(X).

# Aggregation
count_by_type(Type, N) :-
    items(_, Type) |>
    do fn:group_by(Type),
    let N = fn:Count().
```

### Go Integration

```go
// Creating facts from Go
fact := core.Fact{
    Predicate: "my_predicate",
    Args: []interface{}{
        "string_value",           // string
        42,                       // int
        core.MangleAtom("/name"), // atom (constants start with /)
    },
}

// Converting to Mangle AST
atom, err := fact.ToAtom()

// Loading into kernel
kernel.LoadFacts([]core.Fact{fact})

// Querying
results, err := kernel.Query("my_predicate")
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Missing Decl | "undeclared predicate" | Add `Decl` to schemas.mg |
| Unbound variable | Safety violation | Bind all vars in positive literals |
| Wrong arg type | Type mismatch error | Check Decl types |
| Atom/string confusion | `/foo` vs `"foo"` | Use MangleAtom for constants |

---

## Point 4: Virtual Predicates

**Location:** `internal/core/virtual_store.go`
**Reference:** [codenerd-builder skill](../../codenerd-builder/SKILL.md)

### Declaration

```mangle
# In schemas.mg - Mark as virtual (computed at runtime)
Decl query_learned(Predicate, Args).
Decl recall_similar(Query, TopK, Results).
Decl query_knowledge_graph(EntityA, Relation, EntityB).
```

### Resolution

```go
// In virtual_store.go Get() method
func (vs *VirtualStore) Get(predicate string, args ...interface{}) ([]Fact, error) {
    switch predicate {
    case "my_virtual_pred":
        // Resolve from external system
        results, err := vs.externalAPI.Query(args[0].(string))
        if err != nil {
            return nil, err
        }
        // Convert to facts
        var facts []Fact
        for _, r := range results {
            facts = append(facts, Fact{
                Predicate: "my_virtual_pred",
                Args:      []interface{}{r.ID, r.Value},
            })
        }
        return facts, nil
    // ... other cases
    }
}
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Missing case | Predicate returns empty | Add case to Get() switch |
| Wrong arity | Type/count mismatch | Match args to Decl |
| No error handling | Silent failures | Return errors, log them |

---

## Point 5: CLI Commands

**Location:** `cmd/nerd/chat/commands.go`

### Adding a Command

```go
// In handleCommand() switch
case "/mycommand":
    // Parse arguments if any
    args := strings.TrimPrefix(input, "/mycommand ")

    // Execute command logic
    result, err := m.executeMyCommand(ctx, args)
    if err != nil {
        return m, fmt.Errorf("mycommand failed: %w", err)
    }

    // Update model state if needed
    m.output = result
    return m, nil
```

### Help Integration

```go
// In help output or /help handler
const helpText = `
/mycommand [args]  - Description of what it does
`
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Missing case | "unknown command" | Add case to switch |
| No error handling | Panic or silent failure | Return wrapped errors |
| Incomplete help | Users can't discover | Update help text |

---

## Point 6: Transducer

**Location:** `internal/perception/transducer.go`

### Adding a Verb

```go
// In VerbCorpus initialization
VerbEntry{
    Verb:      "/myverb",
    Category:  "/mutation",  // or /query, /instruction
    Synonyms:  []string{"myverb", "do-thing", "execute-thing"},
    Patterns:  []*regexp.Regexp{
        regexp.MustCompile(`(?i)^myverb\s+(.+)$`),
        regexp.MustCompile(`(?i)^do thing\s+(.+)$`),
    },
    Priority:  10,           // Higher wins on conflict
    ShardType: "myshard",    // Maps to shard registration
},
```

### Intent Flow

```
User Input -> Transducer.Parse() -> Intent{Category, Verb, Target, Constraint}
                                          |
                                          v
                             Kernel: user_intent(ID, Cat, Verb, Target, Constraint)
                                          |
                                          v
                             Rules derive: next_action
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| No verb match | Falls to generic handling | Add to VerbCorpus |
| Wrong shardType | Wrong shard executes | Match to registration name |
| Pattern conflict | Unexpected verb selected | Adjust priority |

---

## Point 6.5: JIT Prompt Compiler

**Location:** `internal/prompt/jit_compiler.go`, `internal/prompt/loader.go`
**Reference:** [prompt-architect skill](../../prompt-architect/SKILL.md)

### Bootstrap Sequence

The JIT Prompt Compiler requires careful initialization during the boot sequence:

```go
// Step 1: Create JIT Compiler with embedded corpus (after LLM Client)
corpus, err := prompt.LoadEmbeddedCorpus()
if err != nil {
    return nil, fmt.Errorf("failed to load prompt corpus: %w", err)
}

jitCompiler := prompt.NewJITPromptCompiler(corpus)

// Step 2: Create callbacks for ShardManager
promptLoader := prompt.CreatePromptLoader()
jitRegistrar := prompt.CreateJITDBRegistrar(jitCompiler)
jitUnregistrar := prompt.CreateJITDBUnregistrar(jitCompiler)

// Step 3: Wire ShardManager with callbacks
shardManager.SetPromptLoader(promptLoader)
shardManager.SetJITRegistrar(jitRegistrar)
shardManager.SetJITUnregistrar(jitUnregistrar)

// Step 4: Wire PromptAssembler with JIT Compiler
promptAssembler := articulation.NewPromptAssembler()
promptAssembler.SetJITCompiler(jitCompiler)

// Step 5: Optional - Enable JIT via environment variable
// Set USE_JIT_PROMPTS=true to enable JIT compilation
```

### Key Wiring Functions

| Function | Location | Purpose |
|----------|----------|---------|
| `NewJITPromptCompiler(corpus)` | `internal/prompt/jit_compiler.go` | Creates compiler with embedded corpus |
| `SetJITCompiler(compiler)` | `internal/articulation/prompt_assembler.go` | Attaches compiler to assembler |
| `CreatePromptLoader()` | `internal/prompt/loader.go` | Creates callback for loading YAML |
| `CreateJITDBRegistrar(compiler)` | `internal/prompt/loader.go` | Creates registration callback |
| `CreateJITDBUnregistrar(compiler)` | `internal/prompt/loader.go` | Creates cleanup callback |
| `SetPromptLoader(loader)` | `internal/core/shard_manager.go` | Sets loader callback |
| `SetJITRegistrar(registrar)` | `internal/core/shard_manager.go` | Sets registration callback |
| `SetJITUnregistrar(unregistrar)` | `internal/core/shard_manager.go` | Sets cleanup callback |
| `RegisterAgentDBWithJIT(compiler, name, dbPath)` | `internal/prompt/loader.go` | Opens DB & registers |
| `LoadAgentPrompts(dbPath, yamlPath)` | `internal/prompt/loader.go` | Loads YAML → SQLite |
| `ensurePromptAtomsTable(db)` | `internal/prompt/loader.go` | Creates prompt_atoms table |

### Shard Spawn Integration

For Type B/U shards, the JIT system integrates during spawn:

```go
// In ShardManager.SpawnAsyncWithContext()
func (sm *ShardManager) SpawnAsyncWithContext(ctx context.Context, shardType, task string) (string, error) {
    // ... other spawn logic ...

    // Check if this is a Type B/U shard
    profile := sm.GetProfile(shardType)
    if profile.Type == ShardTypePersistent || profile.Type == ShardTypeUser {
        // Check for prompts.yaml
        yamlPath := fmt.Sprintf(".nerd/agents/%s/prompts.yaml", shardType)
        if fileExists(yamlPath) {
            // Load prompts into shard KB
            if sm.promptLoader != nil {
                dbPath := fmt.Sprintf(".nerd/agents/%s/knowledge.db", shardType)
                err := sm.promptLoader(dbPath, yamlPath)
                if err != nil {
                    logging.Articulation("Failed to load prompts for %s: %v", shardType, err)
                }
            }

            // Register DB with JIT compiler
            if sm.jitRegistrar != nil {
                dbPath := fmt.Sprintf(".nerd/agents/%s/knowledge.db", shardType)
                err := sm.jitRegistrar(shardType, dbPath)
                if err != nil {
                    logging.Articulation("Failed to register JIT DB for %s: %v", shardType, err)
                }
            }
        }
    }

    // ... execute shard ...

    // On completion, unregister
    if sm.jitUnregistrar != nil {
        sm.jitUnregistrar(shardType)
    }

    return result, nil
}
```

### Prompts.yaml Structure

Type B/U shards should have a prompts.yaml file with this structure:

```yaml
# .nerd/agents/myagent/prompts.yaml
static_fragments:
  - name: "role_definition"
    content: "You are a specialized agent for..."
    priority: 100

  - name: "capabilities"
    content: |
      Your capabilities include:
      - Capability 1
      - Capability 2
    priority: 90

dynamic_atoms:
  - predicate: "domain_knowledge"
    description: "Specialist domain knowledge"
    query: "SELECT content FROM knowledge WHERE category='domain'"

  - predicate: "learned_patterns"
    description: "Patterns learned from previous executions"
    query: "SELECT pattern FROM learnings WHERE success=1"

compilation_rules:
  - when: "task_type = 'analysis'"
    include_fragments: ["role_definition", "capabilities"]
    include_atoms: ["domain_knowledge"]

  - when: "task_type = 'generation'"
    include_fragments: ["role_definition", "capabilities"]
    include_atoms: ["domain_knowledge", "learned_patterns"]
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| JIT Compiler not initialized | Prompts not compiled | Call NewJITPromptCompiler() in boot |
| SetJITCompiler() not called | Assembler doesn't use JIT | Wire to PromptAssembler |
| Callbacks not set | Shard DB not registered | Set all 3 callbacks on ShardManager |
| Missing prompts.yaml | Default prompts used | Create .nerd/agents/{name}/prompts.yaml |
| Table not created | Prompts not loaded | Ensure ensurePromptAtomsTable() runs |
| DB not unregistered | Memory leak | Call jitUnregistrar on shard completion |

---

## Point 7: Actions

**Location:** `internal/core/virtual_store.go`

### Adding an Action Type

```go
// 1. Define constant
const ActionMyAction = "my_action"

// 2. Add handler in Execute()
func (vs *VirtualStore) Execute(ctx context.Context, req ActionRequest) (ActionResult, error) {
    switch req.Type {
    case ActionMyAction:
        return vs.executeMyAction(ctx, req)
    // ...
    }
}

// 3. Implement handler
func (vs *VirtualStore) executeMyAction(ctx context.Context, req ActionRequest) (ActionResult, error) {
    // Validate permissions
    if !vs.hasPermission(req.SessionID, PermissionMyAction) {
        return ActionResult{}, ErrPermissionDenied
    }

    // Execute action
    result, err := vs.doThing(req.Target, req.Payload)
    if err != nil {
        return ActionResult{}, fmt.Errorf("my_action failed: %w", err)
    }

    // Return result with any facts to add
    return ActionResult{
        Output:     result,
        FactsToAdd: []Fact{...},  // Optional kernel facts
    }, nil
}
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Missing case | "unknown action type" | Add case to Execute() |
| No permission check | Security bypass | Check permissions first |
| Missing facts | State not updated | Return FactsToAdd |

---

## Point 8: Tests

**Location:** Various `*_test.go` files

### Test Hierarchy

```
Unit Tests          - Test isolated logic
Integration Tests   - Test component wiring
Live Tests          - Test actual execution with real deps
```

### Integration Test Pattern

```go
func TestMyFeatureIntegration(t *testing.T) {
    // 1. Setup full component graph
    cortex, err := coresys.BootCortex(ctx, testWorkspace, apiKey, false)
    require.NoError(t, err)
    defer cortex.Shutdown()

    // 2. Exercise the feature through its entry point
    result, err := cortex.ShardManager.SpawnWithContext(ctx, "myshard", "task", sessionCtx)

    // 3. Verify wiring worked
    require.NoError(t, err)
    require.NotEmpty(t, result)

    // 4. Verify side effects (logging, facts, etc.)
    facts, _ := cortex.Kernel.Query("shard_executed")
    assert.NotEmpty(t, facts)
}
```

### Live Test Pattern

```go
func TestMyFeatureLive(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping live test in short mode")
    }

    // Test against real external services
    // ...
}
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| No integration test | Wiring bugs in prod | Add integration test |
| Mocked too much | Real path untested | Add live test |
| Missing cleanup | Flaky tests | Use defer, t.Cleanup() |

---

## Point 9: Config

**Location:** `internal/config/config.go`

### Adding Config

```go
// 1. Add field to config struct
type MyFeatureConfig struct {
    Enabled    bool   `json:"enabled"`
    Timeout    int    `json:"timeout_ms"`
    MaxRetries int    `json:"max_retries"`
}

// 2. Add to main Config struct
type Config struct {
    // ...
    MyFeature MyFeatureConfig `json:"my_feature"`
}

// 3. Set default in defaults
func DefaultConfig() Config {
    return Config{
        // ...
        MyFeature: MyFeatureConfig{
            Enabled:    true,
            Timeout:    30000,
            MaxRetries: 3,
        },
    }
}
```

### Config File Location

```
.nerd/config.json
```

### Common Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| No default | Nil/zero config | Set in DefaultConfig() |
| Missing field | Old configs break | Make additive, not breaking |
| No validation | Invalid values | Validate in Load() |

---

## Verification Commands

### Run All Tests

```bash
go test ./...
```

### Build and Verify

```bash
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build ./cmd/nerd
```

### Check Logging Works

```bash
# Enable debug mode in .nerd/config.json
# Run nerd, check .nerd/logs/
```

### Verify Shard Registration

```bash
./nerd.exe
/spawn myshard "test task"
```

### Check Kernel Schema

```bash
# Run mangle query to verify predicate exists
# Or check for undeclared predicate errors in logs
```
