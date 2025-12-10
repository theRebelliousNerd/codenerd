# System-Specific Audits

Detailed audit guides for each of the major codeNERD subsystems.

---

## Mangle System Audit

The kernel is the executive brain. Wiring failures here cause silent logic errors.

### Schema Audit

**What to check:**
- Every predicate used in policy has a `Decl`
- Arities match between Decl and usage
- Types specified for predicates requiring them

**How to verify:**
```bash
# Run audit script - will flag undeclared predicates
python audit_wiring.py --verbose

# Manual check: search for predicate usage
cd internal/mangle
grep -r "my_predicate" policy.gl
# Then verify schemas.mg has:
# Decl my_predicate(...).
```

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| Missing Decl | "undeclared predicate" at boot | Add `Decl predicate(...).` to schemas.mg |
| Arity mismatch | Type error or empty results | Match arg count in Decl and usage |
| Atom vs String | Type confusion | Use `/atom` for constants, `"string"` for text |

### Policy Audit

**What to check:**
- All derived predicates have rules in `policy.gl`
- Variables are safe (bound in positive literals before use in negation)
- Stratification correct (no negation cycles)
- Aggregations use correct syntax

**How to verify:**
```go
// Test that rules derive expected facts
func TestPolicyDerivation(t *testing.T) {
    kernel := setupKernel(t)

    // Load base facts
    kernel.LoadFacts([]Fact{
        {Predicate: "base_fact", Args: []interface{}{"value"}},
    })

    // Query derived predicate
    results, err := kernel.Query("derived_fact")
    require.NoError(t, err)
    assert.NotEmpty(t, results, "Rule should derive facts")
}
```

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| Safety violation | "unsafe variable X" | Bind X in positive literal first |
| Stratification error | "negation cycle" | Reorder rules, break cycle |
| Aggregation syntax | Parse error | Use `\|> do fn:group_by(...), let N = fn:Count()` |

### Fact Generation Audit

**What to check:**
- Go structs implement `ToAtom()` correctly
- Atoms use `/lowercase` for constants (not strings)
- `LoadFacts()` called after fact creation
- Timestamps use consistent format

**How to verify:**
```go
func TestFactGeneration(t *testing.T) {
    fact := MyFact{ID: "123", Type: "test"}

    // Convert to Mangle atom
    atom, err := fact.ToAtom()
    require.NoError(t, err)

    // Verify structure
    assert.Equal(t, "my_fact", atom.Predicate)
    assert.Equal(t, 2, len(atom.Args))

    // Verify constants are atoms, not strings
    typeArg := atom.Args[1]
    assert.True(t, strings.HasPrefix(typeArg.(string), "/"), "Should be atom /test")
}
```

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| String instead of atom | Type mismatch in rules | Use `core.MangleAtom("/value")` |
| Facts not loaded | Query returns empty | Call `kernel.LoadFacts()` |
| Wrong arity | Type error | Match `ToAtom()` args to Decl |

---

## Storage System Audit

codeNERD has 4 storage tiers. Each requires different wiring.

### RAM Tier (FactStore)

**What to check:**
- Kernel initialized with FactStore
- Facts loaded during session
- Cleared between sessions if needed

**How to verify:**
```go
func TestRAMTier(t *testing.T) {
    cortex := setupCortex(t)

    // Add fact
    cortex.Kernel.LoadFacts([]Fact{{Predicate: "test", Args: []interface{}{"value"}}})

    // Query immediately (should work)
    results, _ := cortex.Kernel.Query("test")
    assert.NotEmpty(t, results)

    // After session end, should be gone (if not persisted)
}
```

### Vector Tier (SQLite + Embeddings)

**What to check:**
- SQLite DB created at configured path
- `learned_facts` table exists with schema:
  - `id INTEGER PRIMARY KEY`
  - `predicate TEXT`
  - `content TEXT`
  - `embedding BLOB`
  - `timestamp INTEGER`
- Embedding generation working
- Similarity search functional

**How to verify:**
```bash
# Check DB exists
ls .nerd/vector/learned.db

# Check schema
sqlite3 .nerd/vector/learned.db "PRAGMA table_info(learned_facts);"

# Check embeddings populated
sqlite3 .nerd/vector/learned.db "SELECT COUNT(*) FROM learned_facts WHERE embedding IS NOT NULL;"
```

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| DB file missing | "no such table" | Create DB in hydration |
| No embeddings | Similarity search fails | Generate embeddings on insert |
| SQLite-vec not enabled | Vector functions fail | Build with CGO_CFLAGS |

### Graph Tier (Knowledge Graph)

**What to check:**
- `knowledge_graph` table exists:
  - `entity_a TEXT`
  - `relation TEXT`
  - `entity_b TEXT`
  - `confidence REAL`
- Relations populated from code analysis
- Graph queries working

**How to verify:**
```go
func TestKnowledgeGraph(t *testing.T) {
    kg := setupKnowledgeGraph(t)

    // Add relation
    kg.AddRelation("Function.foo", "calls", "Function.bar", 1.0)

    // Query
    results := kg.Query("Function.foo", "calls", "")
    assert.Contains(t, results, "Function.bar")
}
```

### Cold Storage Tier

**What to check:**
- `cold_storage` table exists:
  - `key TEXT PRIMARY KEY`
  - `value BLOB`
  - `category TEXT`
  - `created_at INTEGER`
- Learned patterns persisted
- Retrieval working

**How to verify:**
```bash
sqlite3 .nerd/cold/storage.db "SELECT key, category FROM cold_storage;"
```

---

## Compression System Audit

Token budget management prevents context overflow.

### Token Budgets

**What to check:**
- Budget configured in shard profiles
- Budget enforcement in context selection
- Overflow handling (paging or truncation)

**How to verify:**
```go
func TestTokenBudget(t *testing.T) {
    shard := setupShard(t)

    // Large context
    largeTask := strings.Repeat("word ", 10000)

    // Execute - should not overflow
    result, err := shard.Execute(ctx, largeTask)
    require.NoError(t, err)

    // Verify context was compressed
    // (check logs for compression activity)
}
```

**Configuration:**
```go
// In shard profile
core.ShardConfig{
    MemoryLimit: 12000,  // tokens
    Model: core.ModelConfig{
        MaxTokens: 4096,   // output limit
    },
}
```

### Spreading Activation

**What to check:**
- Relevance scoring implemented
- Activation spread from query atoms
- Top-K selection working

**How to verify:**
```bash
# Enable /context logging
# Look for "spreading activation" and "selected N atoms"
```

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| Context overflow | 400 error from LLM | Lower MemoryLimit or compress context |
| Irrelevant facts | Poor LLM responses | Improve relevance scoring |
| Missing activation | All facts selected | Implement spreading activation |

---

## Autopoiesis Audit

Self-improvement requires careful wiring to avoid instability.

### Ouroboros Loop

**What to check:**
- Tool generation enabled in config
- ToolGeneratorShard registered (Type B)
- Generated tools persisted to `.nerd/tools/`
- Tools loaded on next boot

**How to verify:**
```bash
# Trigger tool generation
./nerd.exe
> I need a tool to check go.mod consistency

# Check tool created
ls .nerd/tools/
# Should see go_mod_checker.go

# Check persisted
./nerd.exe
> /list-tools
# Should include go_mod_checker
```

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| Tools not generated | No `.nerd/tools/` files | Check ToolGeneratorShard wiring |
| Tools not loaded | Not in registry | Add loader in boot sequence |
| Unsafe tools | Security risk | Verify Constitution Gate approval |

### Safety Checking

**What to check:**
- Constitution Gate (Type S) running
- Generated code reviewed before execution
- Rejection patterns logged for learning

**How to verify:**
```go
func TestConstitutionGate(t *testing.T) {
    gate := setupConstitutionGate(t)

    // Unsafe action
    action := ActionRequest{Type: "exec", Payload: "rm -rf /"}

    permitted := gate.Check(action)
    assert.False(t, permitted, "Should reject unsafe action")
}
```

### Learning Persistence

**What to check:**
- Rejection patterns stored in cold storage
- Learned rules added to policy
- Dream State processing enabled

**How to verify:**
```bash
sqlite3 .nerd/cold/storage.db "SELECT * FROM cold_storage WHERE category = 'rejection_pattern';"
```

---

## TUI Integration Audit

Bubbletea TUI wiring connects UI to logic.

### Command Handlers

**What to check:**
- Commands registered in `handleCommand()` switch
- Help text includes new commands
- Error messages displayed to user

**How to verify:**
```go
// In cmd/nerd/chat/commands.go
case "/mycommand":
    result, err := m.executeMyCommand(ctx, input)
    if err != nil {
        m.output = fmt.Sprintf("Error: %v", err)
        return m, nil
    }
    m.output = result
    return m, nil
```

### View Updates

**What to check:**
- Model state updated in `Update()`
- View rendering in `View()`
- Progress indicators for long operations

**How to verify:**
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case MyCustomMsg:
        m.state = msg.NewState
        return m, nil
    }
    return m, nil
}
```

### Session State

**What to check:**
- Session context preserved across commands
- History maintained
- Current task tracked

**How to verify:**
```go
type SessionContext struct {
    WorkingDir string
    History    []string
    CurrentTask string
    Metadata   map[string]interface{}
}
```

---

## Campaign System Audit

Multi-phase orchestration for long-running goals.

### Orchestrator Setup

**What to check:**
- Campaign created with phases
- Context paging configured
- Phase transitions working

**How to verify:**
```go
func TestCampaignOrchestration(t *testing.T) {
    campaign := NewCampaign("Multi-phase task")
    campaign.AddPhase("Research", researchTask)
    campaign.AddPhase("Design", designTask)
    campaign.AddPhase("Implement", implementTask)

    // Execute
    err := campaign.Execute(ctx)
    require.NoError(t, err)

    // Verify all phases completed
    assert.Equal(t, 3, campaign.CompletedPhases())
}
```

### Phase Management

**What to check:**
- Phase dependencies respected
- Context carried forward between phases
- Failure recovery working

### Context Paging

**What to check:**
- Large campaigns split into pages
- Page boundaries preserve context
- Token budgets respected per page

**Common failures:**

| Failure | Symptom | Fix |
|---------|---------|-----|
| Phases out of order | Wrong execution sequence | Define dependencies |
| Context lost | Later phases missing info | Implement context carry-forward |
| Token overflow | Campaign fails mid-execution | Enable context paging |
