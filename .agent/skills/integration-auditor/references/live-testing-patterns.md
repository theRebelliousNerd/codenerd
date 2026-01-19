# Live Testing Patterns

Verify wiring works end-to-end with these proven test patterns.

---

## Pattern 1: Full Cortex Boot Test

Tests that the entire system boots correctly with all components initialized.

```go
func TestFullBoot(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping full boot in short mode")
    }

    workspace := t.TempDir()
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        t.Skip("No API key")
    }

    // Boot entire system
    cortex, err := coresys.BootCortex(context.Background(), workspace, apiKey, false)
    require.NoError(t, err)
    defer cortex.Shutdown()

    // Verify all system shards running
    active := cortex.ShardManager.GetActiveShards()
    expectedShards := []string{
        "perception_firewall",
        "world_model_ingestor",
        "executive_policy",
        "constitution_gate",
    }

    for _, name := range expectedShards {
        found := false
        for _, shard := range active {
            if shard.Name == name {
                found = true
                break
            }
        }
        assert.True(t, found, "Missing system shard: %s", name)
    }

    // Verify kernel operational
    results, err := cortex.Kernel.Query("system_boot")
    require.NoError(t, err)
    assert.NotEmpty(t, results)
}
```

**What this verifies:**
- Config loading works
- Kernel initializes
- All Type S system shards auto-start
- Kernel can query facts

---

## Pattern 2: Shard Spawn-Execute-Cleanup

Tests the complete lifecycle of an ephemeral shard.

```go
func TestShardLifecycle(t *testing.T) {
    cortex := setupCortex(t)
    defer cortex.Shutdown()

    // Spawn
    sessionCtx := &core.SessionContext{
        WorkingDir: t.TempDir(),
        History:    []string{},
    }

    result, err := cortex.ShardManager.SpawnWithContext(
        context.Background(),
        "coder",
        "create hello.go with Hello() function",
        sessionCtx,
    )

    // Verify execution
    require.NoError(t, err)
    require.NotEmpty(t, result)

    // Verify facts added
    facts, _ := cortex.Kernel.Query("shard_executed")
    assert.NotEmpty(t, facts)

    // Verify cleanup (Type A should auto-cleanup)
    time.Sleep(100 * time.Millisecond)
    active := cortex.ShardManager.GetActiveShards()
    for _, shard := range active {
        assert.NotEqual(t, "coder", shard.Name, "Should have cleaned up")
    }
}
```

**What this verifies:**
- Shard factory creates shard
- Dependencies injected correctly
- Execute() runs to completion
- Facts flow to kernel
- Cleanup happens after execution

---

## Pattern 3: Transducer NL-to-Action

Tests natural language parsing to action derivation.

```go
func TestNaturalLanguageParsing(t *testing.T) {
    cortex := setupCortex(t)
    defer cortex.Shutdown()

    // Natural language input
    input := "review the authentication code for security issues"

    // Parse through transducer
    intent, err := cortex.Transducer.Parse(input)
    require.NoError(t, err)

    // Verify intent extracted
    assert.Equal(t, "/review", intent.Verb)
    assert.Equal(t, "authentication code", intent.Target)
    assert.Equal(t, "security", intent.Constraint)

    // Verify fact created
    cortex.Kernel.LoadFacts(intent.ToFacts())
    facts, _ := cortex.Kernel.Query("user_intent")
    assert.NotEmpty(t, facts)

    // Verify next_action derived
    actions, _ := cortex.Kernel.Query("next_action")
    assert.NotEmpty(t, actions)
}
```

**What this verifies:**
- Transducer verb corpus matches patterns
- Intent extraction works
- Facts convert correctly
- Policy rules derive next_action

---

## Pattern 4: Virtual Predicate Resolution

Tests that virtual predicates resolve through VirtualStore.

```go
func TestVirtualPredicateResolution(t *testing.T) {
    cortex := setupCortex(t)
    defer cortex.Shutdown()

    // Populate some learned facts
    cortex.LearningStore.Learn(core.Fact{
        Predicate: "learned_pattern",
        Args:      []interface{}{"avoid_global_state"},
    })

    // Query virtual predicate (should call VirtualStore.Get)
    results, err := cortex.VirtualStore.Get("query_learned", "pattern")
    require.NoError(t, err)
    assert.NotEmpty(t, results)

    // Verify result structure
    assert.Equal(t, "learned_pattern", results[0].Predicate)
}
```

**What this verifies:**
- Virtual predicate declared in schemas
- VirtualStore.Get() handles the predicate
- External data source queried
- Results converted to facts

---

## Pattern 5: Piggyback Protocol

Tests dual-channel output from shard execution.

```go
func TestPiggybackOutput(t *testing.T) {
    cortex := setupCortex(t)
    defer cortex.Shutdown()

    sessionCtx := &core.SessionContext{
        WorkingDir: t.TempDir(),
    }

    // Execute shard
    result, err := cortex.ShardManager.SpawnWithContext(
        context.Background(),
        "coder",
        "create test.go",
        sessionCtx,
    )
    require.NoError(t, err)

    // Parse piggyback output
    userMsg, controlFacts := ParsePiggyback(result)

    // Verify user-facing message
    assert.Contains(t, userMsg, "Created test.go")

    // Verify control facts for kernel
    assert.NotEmpty(t, controlFacts)
    for _, fact := range controlFacts {
        cortex.Kernel.LoadFacts([]core.Fact{fact})
    }

    // Verify facts loaded
    facts, _ := cortex.Kernel.Query("file_created")
    assert.NotEmpty(t, facts)
}
```

**What this verifies:**
- Shard returns dual-channel output
- User message is human-readable
- Control facts parse correctly
- Facts flow to kernel for downstream rules

---

## Pattern 6: TDD Loop Integration

Tests the full test-driven repair cycle.

```go
func TestTDDLoop(t *testing.T) {
    cortex := setupCortex(t)
    defer cortex.Shutdown()

    sessionCtx := &core.SessionContext{
        WorkingDir: t.TempDir(),
    }

    // Start with failing test
    testFile := filepath.Join(sessionCtx.WorkingDir, "math_test.go")
    os.WriteFile(testFile, []byte(`
        package math
        import "testing"
        func TestAdd(t *testing.T) {
            if Add(2, 3) != 5 {
                t.Error("Add failed")
            }
        }
    `), 0644)

    // Execute TDD repair
    result, err := cortex.TDDLoop.Repair(context.Background(), testFile)
    require.NoError(t, err)

    // Verify implementation created
    implFile := filepath.Join(sessionCtx.WorkingDir, "math.go")
    assert.FileExists(t, implFile)

    // Verify test now passes
    testResult := cortex.TestRunner.Run(testFile)
    assert.True(t, testResult.Passed)
}
```

**What this verifies:**
- TesterShard detects failures
- CoderShard generates fixes
- Loop iterates until passing
- Implementation matches test requirements

---

## Pattern 7: Type B Persistence

Tests that persistent shards retain knowledge.

```go
func TestSpecialistPersistence(t *testing.T) {
    dbPath := filepath.Join(t.TempDir(), "specialist.db")

    // First cortex boot - learn something
    cortex1 := setupCortexWithDB(t, dbPath)
    result1, err := cortex1.ShardManager.SpawnWithContext(
        context.Background(),
        "specialist",
        "learn: always use context.Context as first parameter",
        sessionCtx,
    )
    require.NoError(t, err)
    cortex1.Shutdown()

    // Verify persisted to DB
    db, _ := sql.Open("sqlite3", dbPath)
    var count int
    db.QueryRow("SELECT COUNT(*) FROM learned_facts").Scan(&count)
    assert.Greater(t, count, 0, "Should have persisted facts")
    db.Close()

    // Second cortex boot - recall
    cortex2 := setupCortexWithDB(t, dbPath)
    result2, err := cortex2.ShardManager.SpawnWithContext(
        context.Background(),
        "specialist",
        "what do you know about function parameters?",
        sessionCtx,
    )
    require.NoError(t, err)
    assert.Contains(t, result2, "context.Context", "Should recall learned knowledge")
    cortex2.Shutdown()
}
```

**What this verifies:**
- LearningStore injection works
- Knowledge persists to SQLite
- Knowledge survives process restart
- Recall works in new session

---

## Pattern 8: Action Permission Check

Tests that VirtualStore enforces permissions.

```go
func TestActionPermissions(t *testing.T) {
    cortex := setupCortex(t)
    defer cortex.Shutdown()

    // Create shard with limited permissions
    config := core.ShardConfig{
        Name:        "limited",
        Type:        core.ShardTypeEphemeral,
        Permissions: []core.ShardPermission{core.PermissionRead}, // No write
    }

    // Try to execute write action
    req := core.ActionRequest{
        Type:      core.ActionWriteFile,
        SessionID: "test",
        Target:    "/tmp/test.txt",
        Payload:   "content",
    }

    result, err := cortex.VirtualStore.Execute(context.Background(), req)

    // Should be denied
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "permission denied")
}
```

**What this verifies:**
- Permission checks enforced
- Unauthorized actions rejected
- Error messages are clear

---

## Test Infrastructure Helpers

### Setup Helper

```go
func setupCortex(t *testing.T) *coresys.Cortex {
    t.Helper()

    workspace := t.TempDir()
    apiKey := os.Getenv("TEST_API_KEY")
    if apiKey == "" {
        apiKey = "test-key" // For mocked tests
    }

    cortex, err := coresys.BootCortex(
        context.Background(),
        workspace,
        apiKey,
        true, // test mode
    )
    require.NoError(t, err)

    t.Cleanup(func() {
        cortex.Shutdown()
    })

    return cortex
}
```

### Session Context Helper

```go
func testSessionContext(t *testing.T) *core.SessionContext {
    return &core.SessionContext{
        WorkingDir:  t.TempDir(),
        History:     []string{},
        CurrentTask: "test",
        Metadata:    map[string]interface{}{},
    }
}
```

### Kernel Query Helper

```go
func assertFactExists(t *testing.T, kernel core.Kernel, predicate string) {
    t.Helper()
    results, err := kernel.Query(predicate)
    require.NoError(t, err)
    assert.NotEmpty(t, results, "Expected fact %s to exist", predicate)
}
```

---

## Running Live Tests

```bash
# Run all tests
go test ./...

# Run live tests only (require API key)
TEST_API_KEY=your-key go test ./... -run Live

# Run with race detection
go test -race ./...

# Run specific pattern
go test ./internal/core -run TestShardLifecycle -v

# Skip live tests in CI
go test -short ./...
```
