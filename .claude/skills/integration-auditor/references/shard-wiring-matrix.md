# Shard Type Wiring Matrix

Complete wiring requirements for each of the 4 shard lifecycle types. Every shard implementation must satisfy ALL requirements for its type.

## Quick Reference Matrix

| Wiring Point | Type A (Ephemeral) | Type B (Persistent) | Type U (User) | Type S (System) |
|--------------|-------------------|---------------------|---------------|-----------------|
| **Registration** | `RegisterShard()` | `RegisterShard()` | `RegisterShard()` | `RegisterShard()` |
| **Profile** | `DefineProfile()` | `DefineProfile()` | `DefineProfile()` | System config |
| **Factory Injection** | Kernel, LLM, VirtualStore | Kernel, LLM, VirtualStore, LearningStore | Kernel, LLM, VirtualStore, LearningStore | Kernel, LLM, VirtualStore |
| **Memory** | RAM only | SQLite-backed | SQLite-backed | RAM only |
| **Lifecycle** | Spawn-Execute-Die | Long-lived | Long-lived | Auto-start |
| **Logging Category** | Specific or `/shards` | Specific or `/shards` | `/shards` | `/system_shards` |
| **Test Pattern** | Spawn-verify-cleanup | Spawn-persist-reload | Definition-spawn-verify | Auto-start-verify |
| **CLI Wiring** | `/command` trigger | `/init` + `/command` | `/define-agent` wizard | None (auto) |

---

## Type A: Ephemeral Shards

**Constant:** `ShardTypeEphemeral`
**Lifecycle:** Spawn -> Execute -> Die
**Memory:** RAM only (no persistence)

### Registration Requirements

```go
// 1. Factory in registration.go
sm.RegisterShard("myshard", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := myshard.NewMyShard()
    shard.SetParentKernel(ctx.Kernel)       // REQUIRED
    shard.SetLLMClient(ctx.LLMClient)       // REQUIRED
    shard.SetVirtualStore(ctx.VirtualStore) // REQUIRED
    return shard
})

// 2. Profile definition
sm.DefineProfile("myshard", core.ShardConfig{
    Name:        "myshard",
    Type:        core.ShardTypeEphemeral,  // <-- Type A
    Permissions: []core.ShardPermission{
        core.PermissionReadFile,
        core.PermissionWriteFile,
        // ... as needed
    },
    Timeout:     10 * time.Minute,
    MemoryLimit: 12000,
    Model: core.ModelConfig{
        Capability: core.CapabilityHighReasoning,
    },
})
```

### Wiring Checklist

- [ ] Factory registered in `internal/shards/registration.go`
- [ ] Profile defined with `ShardTypeEphemeral`
- [ ] Dependencies injected: Kernel, LLMClient, VirtualStore
- [ ] Implements: `Execute()`, `GetID()`, `GetState()`, `GetConfig()`, `Stop()`
- [ ] Logging uses appropriate category (e.g., `/coder`, `/tester`, `/reviewer`)
- [ ] CLI command added if user-facing (e.g., `/myshard`)
- [ ] Verb added to VerbCorpus in transducer if NL-triggered
- [ ] Test verifies spawn-execute-cleanup cycle

### Live Testing Pattern

```go
func TestMyShardEphemeral(t *testing.T) {
    // Setup
    sm := setupTestShardManager(t)

    // Spawn
    result, err := sm.SpawnWithContext(ctx, "myshard", "test task", sessionCtx)
    require.NoError(t, err)
    require.NotEmpty(t, result)

    // Verify cleanup - no active shards remaining
    active := sm.GetActiveShards()
    assert.Empty(t, active, "Ephemeral shard should auto-cleanup")
}
```

### Example Implementations

- **CoderShard** - `internal/shards/coder/coder.go`
- **TesterShard** - `internal/shards/tester/tester.go`
- **ReviewerShard** - `internal/shards/reviewer/reviewer.go`

---

## Type B: Persistent Specialists

**Constant:** `ShardTypePersistent`
**Lifecycle:** Created at `/init`, survives sessions
**Memory:** SQLite-backed knowledge base

### Registration Requirements

```go
// 1. Factory with LearningStore
sm.RegisterShard("specialist", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := specialist.NewSpecialistShard()
    shard.SetParentKernel(ctx.Kernel)
    shard.SetLLMClient(ctx.LLMClient)
    shard.SetVirtualStore(ctx.VirtualStore)
    shard.SetLearningStore(learningStore)    // REQUIRED for Type B
    shard.SetKnowledgePath(knowledgeDBPath)  // REQUIRED for Type B
    return shard
})

// 2. Profile with Persistent type
sm.DefineProfile("specialist", core.ShardConfig{
    Name: "specialist",
    Type: core.ShardTypePersistent,  // <-- Type B
    KnowledgePath: ".nerd/shards/specialist/knowledge.db",
    // ... other config
})
```

### Wiring Checklist

- [ ] Factory registered with LearningStore injection
- [ ] Profile defined with `ShardTypePersistent`
- [ ] Knowledge path configured in profile
- [ ] SQLite schema created for knowledge storage
- [ ] Hydration logic in `/init` or first spawn
- [ ] Dependencies injected: Kernel, LLMClient, VirtualStore, **LearningStore**
- [ ] Logging uses specific category or `/shards`
- [ ] Agent recommendation added to `internal/init/agents.go` if auto-created
- [ ] Test verifies knowledge persistence across spawns

### Live Testing Pattern

```go
func TestSpecialistPersistence(t *testing.T) {
    // Setup with real DB path
    sm := setupTestShardManager(t)
    dbPath := t.TempDir() + "/specialist.db"

    // First spawn - should hydrate
    result1, err := sm.SpawnWithContext(ctx, "specialist", "learn X", sessionCtx)
    require.NoError(t, err)

    // Verify knowledge persisted
    db, _ := sql.Open("sqlite3", dbPath)
    var count int
    db.QueryRow("SELECT COUNT(*) FROM learned_facts").Scan(&count)
    assert.Greater(t, count, 0, "Should have persisted knowledge")

    // Second spawn - should recall
    result2, err := sm.SpawnWithContext(ctx, "specialist", "recall X", sessionCtx)
    require.NoError(t, err)
    assert.Contains(t, result2, "X", "Should recall persisted knowledge")
}
```

### Example Implementations

- **ToolGeneratorShard** - `internal/shards/tool_generator.go`
- **GoExpert** - Recommended for Go projects
- **TSExpert** - Recommended for TypeScript projects

---

## Type U: User-Defined Specialists

**Constant:** `ShardTypeUser`
**Lifecycle:** Created via `/define-agent` wizard
**Memory:** SQLite-backed (user-populated)

### Registration Requirements

Type U shards use a **generic specialist factory** that's parameterized at runtime:

```go
// Generic factory registered once
sm.RegisterShard("user_specialist", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := specialist.NewUserSpecialistShard()
    shard.SetParentKernel(ctx.Kernel)
    shard.SetLLMClient(ctx.LLMClient)
    shard.SetVirtualStore(ctx.VirtualStore)
    shard.SetLearningStore(learningStore)
    shard.SetKnowledgePath(config.KnowledgePath)  // From user config
    shard.SetSystemPrompt(config.SystemPrompt)    // From wizard
    return shard
})

// Profile created dynamically from wizard input
// Stored in .nerd/agents.json
```

### Wiring Checklist

- [ ] Generic specialist factory exists in registration.go
- [ ] `/define-agent` wizard flow implemented
- [ ] Agent config persisted to `.nerd/agents.json`
- [ ] Knowledge DB created at custom path
- [ ] System prompt customization supported
- [ ] Dependencies injected including LearningStore
- [ ] Logging uses `/shards` category
- [ ] Test verifies wizard-to-spawn flow

### Live Testing Pattern

```go
func TestUserSpecialistCreation(t *testing.T) {
    // Simulate wizard output
    agentConfig := AgentConfig{
        Name:          "MyExpert",
        Type:          core.ShardTypeUser,
        SystemPrompt:  "You are an expert in...",
        KnowledgePath: t.TempDir() + "/myexpert.db",
    }

    // Register dynamic profile
    sm.DefineProfile(agentConfig.Name, agentConfig.ToShardConfig())

    // Spawn user-defined agent
    result, err := sm.SpawnWithContext(ctx, agentConfig.Name, "help with X", sessionCtx)
    require.NoError(t, err)
    require.NotEmpty(t, result)
}
```

### Example Implementations

- Created dynamically via `/define-agent` command
- Configuration stored in `.nerd/agents.json`

---

## Type S: System Shards

**Constant:** `ShardTypeSystem`
**Lifecycle:** Auto-start, long-running
**Memory:** RAM (session-scoped)

### Registration Requirements

```go
// 1. Factory in registration.go
sm.RegisterShard("my_system_shard", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := system.NewMySystemShard()
    shard.SetParentKernel(ctx.Kernel)
    shard.SetLLMClient(ctx.LLMClient)
    shard.SetVirtualStore(ctx.VirtualStore)
    return shard
})

// 2. System profile (different config pattern)
sm.DefineProfile("my_system_shard", core.ShardConfig{
    Name: "my_system_shard",
    Type: core.ShardTypeSystem,  // <-- Type S
    // System shards typically have elevated permissions
    Permissions: []core.ShardPermission{
        core.PermissionAll,  // Or specific elevated set
    },
    // No timeout - runs continuously
})

// 3. Add to system shard list for auto-start
// In shard_manager.go StartSystemShards()
```

### Wiring Checklist

- [ ] Factory registered in registration.go
- [ ] Profile defined with `ShardTypeSystem`
- [ ] Added to `StartSystemShards()` list
- [ ] Dependencies injected: Kernel, LLMClient, VirtualStore
- [ ] Implements continuous operation (not one-shot)
- [ ] Logging uses `/system_shards` category
- [ ] **No CLI command** (auto-started)
- [ ] Test verifies auto-start and continuous operation
- [ ] Graceful shutdown handling implemented

### Live Testing Pattern

```go
func TestSystemShardAutoStart(t *testing.T) {
    // Boot cortex (auto-starts system shards)
    cortex, err := coresys.BootCortex(ctx, workspace, apiKey, false)
    require.NoError(t, err)
    defer cortex.Shutdown()

    // Verify system shard is running
    active := cortex.ShardManager.GetActiveShards()
    found := false
    for _, s := range active {
        if s.Name == "my_system_shard" {
            found = true
            assert.Equal(t, core.ShardTypeSystem, s.Type)
            break
        }
    }
    assert.True(t, found, "System shard should auto-start")
}
```

### Example Implementations

- **perception_firewall** - NL -> atoms transduction
- **world_model_ingestor** - File topology maintenance
- **executive_policy** - next_action derivation
- **constitution_gate** - Safety enforcement
- **legislator** - Constraint synthesis
- **tactile_router** - Action -> tool routing
- **session_planner** - Agenda orchestration

---

## Cross-Type Integration Patterns

### Kernel Fact Injection

All shard types can inject facts into the kernel:

```go
// After shard execution, convert output to facts
facts := sm.ResultToFacts(shardID, shardType, task, result, err)
// Returns:
// - shard_executed(shardID, type, task, timestamp)
// - shard_success(shardID) or shard_failure(shardID, error)
// - shard_output(shardID, result)
// - Type-specific: review_summary, test_summary, etc.
```

### VirtualStore Access

All shard types access external systems through VirtualStore:

```go
// Inside shard.Execute()
result, err := s.virtualStore.Execute(ctx, core.ActionRequest{
    Type:   core.ActionReadFile,
    Target: "path/to/file.go",
})
```

### Inter-Shard Communication

Shards communicate through the kernel, not directly:

```go
// Shard A produces facts
kernel.LoadFacts([]core.Fact{{Predicate: "needs_review", Args: []interface{}{"file.go"}}})

// Shard B queries for work
facts, _ := kernel.Query("needs_review")
```

---

## Common Wiring Failures

### Type A Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Missing cleanup | Memory leak, lingering goroutines | Implement `Stop()` properly |
| No profile | Spawn fails with "unknown shard" | Call `DefineProfile()` |
| Wrong type | Unexpected persistence | Ensure `ShardTypeEphemeral` |

### Type B Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| No LearningStore | Knowledge not persisted | Inject `SetLearningStore()` |
| Missing schema | SQL errors on first use | Create tables in hydration |
| Path conflict | Knowledge overwritten | Unique path per specialist |

### Type U Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Wizard incomplete | Agent unusable | Complete all wizard steps |
| Config not saved | Agent forgotten | Persist to `agents.json` |
| Generic factory missing | Can't spawn | Register `user_specialist` factory |

### Type S Failures

| Failure | Symptom | Fix |
|---------|---------|-----|
| Not in auto-start | Shard never runs | Add to `StartSystemShards()` |
| One-shot execution | Exits immediately | Implement continuous loop |
| No graceful shutdown | Zombie processes | Handle context cancellation |
