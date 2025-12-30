# Failure Patterns Catalog

Comprehensive catalog of integration failures, symptoms, and remediation.

---

## Quick Reference Table

| Symptom | Root Cause | Quick Fix |
|---------|-----------|-----------|
| "unknown shard type: X" | Factory not registered | Add `sm.RegisterShard("X", ...)` |
| "undeclared predicate: X" | Missing schema Decl | Add `Decl X(...).` to schemas.mg |
| Nil pointer in shard | Missing dependency injection | Add `shard.SetParentKernel()` etc. |
| "permission denied" | Profile lacks permission | Add to Permissions list |
| Shard does nothing | Execute() not implemented | Implement Execute() method |
| Facts not querying | Not loaded to kernel | Add `kernel.LoadFacts()` |
| LLM API errors | Invalid config/key | Update .nerd/config.json |
| No logging output | Category not enabled | Enable debug logging |
| Type B forgets knowledge | No LearningStore | Add `shard.SetLearningStore()` |
| System shard won't start | Not in auto-start list | Add to StartSystemShards() |

---

## Registration Failures

### Unknown Shard Type

**Symptom:**
```
Error: unknown shard type: myshard
```

**Root Cause:** Factory not registered in `registration.go`

**Fix:**
```go
// In internal/shards/registration.go
sm.RegisterShard("myshard", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := myshard.NewMyShard()
    shard.SetParentKernel(ctx.Kernel)
    shard.SetLLMClient(ctx.LLMClient)
    shard.SetVirtualStore(ctx.VirtualStore)
    return shard
})
```

### Profile Not Defined

**Symptom:**
```
Error: no profile found for shard: myshard
```
Or: Shard spawns with empty/default config

**Root Cause:** `DefineProfile()` not called

**Fix:**
```go
// In internal/shards/registration.go
sm.DefineProfile("myshard", core.ShardConfig{
    Name:        "myshard",
    Type:        core.ShardTypeEphemeral,
    Permissions: []core.ShardPermission{...},
    Timeout:     10 * time.Minute,
})
```

### Missing Dependency Injection

**Symptom:**
```
panic: runtime error: invalid memory address or nil pointer dereference
```
During shard Execute()

**Root Cause:** Factory doesn't inject required dependencies

**Fix:**
```go
// In factory function - ensure ALL required injections
shard.SetParentKernel(ctx.Kernel)      // Required for all
shard.SetLLMClient(ctx.LLMClient)      // Required for all
shard.SetVirtualStore(ctx.VirtualStore) // Required for most
shard.SetLearningStore(learningStore)   // Required for Type B/U
```

---

## Kernel/Schema Failures

### Undeclared Predicate

**Symptom:**
```
Error: undeclared predicate: my_predicate
```
At boot or first query

**Root Cause:** Missing `Decl` in schemas.mg

**Fix:**
```mangle
# In internal/core/defaults/schemas.mg
Decl my_predicate(Arg1, Arg2, Arg3).
```

### Arity Mismatch

**Symptom:**
```
Error: arity mismatch for my_predicate: expected 3, got 2
```

**Root Cause:** Decl and usage have different argument counts

**Fix:**
1. Check Decl: `Decl my_predicate(A, B, C).` - expects 3 args
2. Check usage in policy.gl or Go code
3. Align both to same arity

### Safety Violation

**Symptom:**
```
Error: unsafe variable X in rule
```

**Root Cause:** Variable used in negation or head without binding

**Fix:**
```mangle
# BAD - X not bound before negation
blocked(X) :- !allowed(X).

# GOOD - X bound in positive literal first
blocked(X) :- candidate(X), !allowed(X).
```

### Atom vs String Confusion

**Symptom:** Rules don't match, queries return empty

**Root Cause:** Using strings where atoms expected or vice versa

**Fix:**
```go
// Constants should be atoms (start with /)
fact := core.Fact{
    Predicate: "my_fact",
    Args: []interface{}{
        "string_value",              // OK for text
        core.MangleAtom("/constant"), // Use MangleAtom for constants
    },
}
```

---

## Action Layer Failures

### Unknown Action Type

**Symptom:**
```
Error: unknown action type: my_action
```

**Root Cause:** Action type not handled in VirtualStore.Execute()

**Fix:**
```go
// 1. Define constant
const ActionMyAction = "my_action"

// 2. Add case in Execute()
func (vs *VirtualStore) Execute(ctx context.Context, req ActionRequest) (ActionResult, error) {
    switch req.Type {
    case ActionMyAction:
        return vs.executeMyAction(ctx, req)
    // ...
    }
}
```

### Permission Denied

**Symptom:**
```
Error: permission denied for action: write_file
```

**Root Cause:** Shard profile doesn't include required permission

**Fix:**
```go
sm.DefineProfile("myshard", core.ShardConfig{
    Permissions: []core.ShardPermission{
        core.PermissionRead,
        core.PermissionWrite,  // Add missing permission
    },
})
```

### Virtual Predicate Returns Empty

**Symptom:** Query returns no results for virtual predicate

**Root Cause:** No handler case in VirtualStore.Get()

**Fix:**
```go
func (vs *VirtualStore) Get(predicate string, args ...interface{}) ([]Fact, error) {
    switch predicate {
    case "my_virtual_pred":
        return vs.resolveMyVirtualPred(args)
    // ... add missing case
    }
    return nil, nil // Falls through to empty
}
```

---

## Output Path Failures

### Silent Failure (No Logging)

**Symptom:** Code runs but no evidence of execution

**Root Cause:** Missing logging statements

**Fix:**
```go
import "codenerd/internal/logging"

func (s *MyShard) Execute(ctx context.Context, task string) (string, error) {
    logging.Shards("MyShard executing: %s", task)

    // ... logic ...

    if err != nil {
        logging.ShardsError("MyShard failed: %v", err)
        return "", err
    }

    logging.Shards("MyShard completed successfully")
    return result, nil
}
```

### Facts Not Visible After Execution

**Symptom:** Shard returns but kernel has no facts

**Root Cause:** `LoadFacts()` not called

**Fix:**
```go
func (s *MyShard) Execute(ctx context.Context, task string) (string, error) {
    // Generate facts from execution
    facts := []core.Fact{
        {Predicate: "shard_executed", Args: []interface{}{s.GetID(), task}},
    }

    // CRITICAL: Load into kernel
    s.kernel.LoadFacts(facts)

    return result, nil
}
```

### Piggyback Not Parsed

**Symptom:** User sees raw output including control data

**Root Cause:** Piggyback protocol not implemented or parsed

**Fix:**
```go
// In shard Execute()
return fmt.Sprintf(`%s

---CONTROL---
{"facts": [{"predicate": "task_complete", "args": ["%s"]}]}
`, userMessage, taskID), nil

// In caller
userMsg, controlFacts := ParsePiggyback(result)
```

---

## Lifecycle Failures

### System Shard Never Starts

**Symptom:** Expected Type S shard not in active list

**Root Cause:** Not in StartSystemShards() list

**Fix:**
```go
// In internal/core/shard_manager.go
func (sm *ShardManager) StartSystemShards(ctx context.Context) error {
    systemShards := []string{
        "perception_firewall",
        "my_system_shard",  // Add here
    }
    // ...
}
```

### Type B Forgets Knowledge

**Symptom:** Persistent shard doesn't recall learned information

**Root Cause:** LearningStore not injected

**Fix:**
```go
// In factory
shard.SetLearningStore(learningStore)
shard.SetKnowledgePath(".nerd/shards/myshard/knowledge.db")
```

### Goroutine Leak

**Symptom:** Memory grows, test hangs, race detector fires

**Root Cause:** No cleanup in Stop()

**Fix:**
```go
func (s *MyShard) Stop() error {
    // Cancel context
    s.cancel()

    // Close channels
    close(s.eventChan)

    // Wait for goroutines
    s.wg.Wait()

    return nil
}
```

---

## External System Failures

### LLM API Errors

**Symptom:**
```
Error: API request failed: 401 Unauthorized
```
Or: 400, 429, 500 errors

**Root Cause:** Invalid config, wrong model ID, rate limit, or API issues

**Fix:**
1. Check `.nerd/config.json` for valid API key
2. Verify model ID exists (e.g., `gemini-3-pro-preview`)
3. Check rate limits, add backoff
4. Monitor API status

### SQLite Errors

**Symptom:**
```
Error: no such table: learned_facts
```

**Root Cause:** Database not created during hydration

**Fix:**
```go
// In hydration/init logic
func initDatabase(path string) error {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return err
    }

    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS learned_facts (
            id INTEGER PRIMARY KEY,
            predicate TEXT,
            content TEXT,
            embedding BLOB,
            timestamp INTEGER
        )
    `)
    return err
}
```

### CGO Build Errors

**Symptom:**
```
cgo: C compiler "gcc" not found
```
Or: sqlite-vec functions not available

**Root Cause:** CGO not configured for sqlite-vec

**Fix:**
```bash
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build ./cmd/nerd
```

---

## Test Failures

### Tests Pass But Feature Doesn't Work

**Symptom:** All green in CI, broken in production

**Root Cause:** Mocking hides real integration issues

**Fix:** Add live integration tests:
```go
func TestMyFeatureLive(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping live test")
    }
    // Test with real dependencies
}
```

### Race Condition

**Symptom:**
```
WARNING: DATA RACE
```

**Root Cause:** Unsafe concurrent access

**Fix:**
```go
// Add mutex
type MyShard struct {
    mu sync.Mutex
    data map[string]string
}

func (s *MyShard) Update(key, value string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.data[key] = value
}
```

---

## Debugging Checklist

When something doesn't work:

1. **Check logs** - Enable debug, check `.nerd/logs/`
2. **Check registration** - Factory and profile exist?
3. **Check kernel** - Decl exists? Facts loading?
4. **Check actions** - Handler case exists? Permissions granted?
5. **Check output** - Logging present? Facts returned?
6. **Run audit** - `python audit_wiring.py --component X --verbose`

Most failures are wiring gaps, not logic bugs. Fix the wiring first.
