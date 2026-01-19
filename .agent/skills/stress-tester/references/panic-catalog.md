# Panic Catalog

Known panic vectors and crash scenarios in codeNERD, organized by subsystem.

## Critical Panic Vectors

### P1: Kernel Boot Failure (CRITICAL)

**Location:** `internal/core/kernel.go:99-100`

**Trigger:** Corrupted or invalid .mg files in `.nerd/mangle/`

**Code:**
```go
if err := k.initialize(); err != nil {
    panic("CRITICAL: Kernel failed to boot: " + err.Error())
}
```

**Stress Test:**
```bash
# Create invalid mangle file
echo "invalid mangle syntax" > .nerd/mangle/test.mg
nerd run "hello"  # Should panic
```

**Recovery:** Delete corrupted .mg files, reinitialize with `nerd init --force`

---

### P2: Mangle Derivation Explosion

**Location:** `internal/mangle/engine.go`

**Trigger:** Cyclic rules or rules that derive exponentially

**Example Cyclic Rule:**
```mangle
Decl reachable(X, Y).
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), reachable(Y, Z).  # Exponential!
```

**Symptom:** Memory exhaustion, eventual OOM kill

**Note:** BUG #17 comment suggests gas limits may not be fully implemented for learned rules

**Stress Test:**
```bash
# Load cyclic rules from assets/cyclic_rules.mg
nerd check-mangle assets/cyclic_rules.mg
```

---

### P3: Nil Pointer in Shard Injection

**Location:** Various shard constructors

**Trigger:** Missing dependency injection (SetLLMClient, SetKernel, etc.)

**Symptom:** `panic: runtime error: invalid memory address or nil pointer dereference`

**Common Scenarios:**
- Shard created but `SetLLMClient()` not called
- VirtualStore not injected
- Kernel reference nil

**Stress Test:**
```bash
# Rapid shard spawning may race against injection
for i in {1..50}; do nerd spawn coder "task $i" & done
```

---

### P4: Map Concurrent Access

**Location:** ShardManager registry, VirtualStore handlers

**Trigger:** Concurrent read/write to maps without proper locking

**Symptom:** `fatal error: concurrent map read and map write`

**Stress Test:**
```bash
# Concurrent operations on shared state
nerd spawn coder "task 1" &
nerd spawn tester "task 2" &
nerd spawn reviewer "task 3" &
nerd agents  # Query while spawning
```

---

### P5: Channel Deadlock

**Location:** SpawnQueue, Campaign orchestration

**Trigger:** Goroutine blocks on channel with no reader/writer

**Symptom:** `fatal error: all goroutines are asleep - deadlock!`

**Risk Areas:**
- SpawnQueue worker goroutines
- Campaign progress channels
- Shard result channels

---

### P6: Stack Overflow in Recursion

**Location:** World scanner, AST analysis, Dataflow

**Trigger:** Deeply nested structures

**Symptom:** `runtime: goroutine stack exceeds 1000000000-byte limit`

**Risk Scenarios:**
- Directory depth > 1000
- AST depth > 1000 (deeply nested code)
- Cyclic dependencies in dataflow graph

**Stress Test:**
```bash
# Create deeply nested directory structure
python -c "import os; [os.makedirs(f'deep/{'a/'*i}') for i in range(500)]"
nerd scan
```

---

### P7: Slice Index Out of Bounds

**Location:** Various array/slice operations

**Trigger:** Empty slices accessed without bounds check

**Common Locations:**
- `parts[0]` without checking `len(parts) > 0`
- History access `history[len-1]` on empty history
- Fact args access

**Stress Test:**
```bash
# Edge cases with empty inputs
nerd perception ""
nerd run ""
```

---

### P8: Context Cancellation Cascade

**Location:** All context-aware operations

**Trigger:** Parent context cancelled while child operations running

**Symptom:** `context canceled` errors cascading, potential panic in cleanup

**Risk Areas:**
- Shard execution mid-operation
- Campaign phase execution
- LLM API calls

---

### P9: JSON Unmarshal Panic

**Location:** Piggyback parsing, config loading

**Trigger:** Unexpected JSON structure

**Symptom:** `panic: json: cannot unmarshal X into Go value of type Y`

**Risk Areas:**
- LLM returns invalid JSON in Piggyback
- Corrupted config.json
- Malformed tool output

**Stress Test:**
```bash
# Force malformed Piggyback by overloading LLM
nerd spawn coder "generate the longest possible response"
```

---

### P10: Division by Zero

**Location:** Complexity metrics, coverage calculations

**Trigger:** Zero divisor in metric calculations

**Code Example (ReviewerShard):**
```go
complexity := totalLines / numFunctions  // Panic if numFunctions == 0
```

**Stress Test:**
```bash
# Review empty files
echo "" > empty.go
nerd review empty.go
```

---

## Medium Risk Panic Vectors

### P11: File Handle Exhaustion

**Trigger:** Too many open files (fd limit)

**Symptom:** `too many open files` then potential panic in cleanup

**Risk Areas:**
- Browser automation (Chrome processes)
- Tool generation (compile cycles)
- Large codebase scanning

---

### P12: Goroutine Leak → OOM

**Trigger:** Goroutines spawned but never terminated

**Symptom:** Memory grows until OOM

**Risk Areas:**
- SpawnQueue workers not shut down
- Campaign goroutines on abort
- Tool execution timeouts

---

### P13: Database Lock Timeout

**Trigger:** Concurrent SQLite access without WAL mode

**Symptom:** `database is locked` then potential panic

**Risk Areas:**
- LearningStore writes
- Knowledge graph updates
- Trace persistence

---

### P14: Thunderdome TODO Incomplete

**Location:** `internal/autopoiesis/thunderdome.go:290`

**Note:** Comment `// TODO: Call the tool's actual entry point` suggests attack execution may be incomplete

**Implication:** Tools may pass "testing" without actual stress

---

## Panic Recovery Patterns

### Pattern 1: Defer + Recover

```go
func safeExecute() (result string, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic recovered: %v", r)
        }
    }()
    // ... risky operation
}
```

**Used In:** Shard execution, tool execution

### Pattern 2: Context Timeout

```go
ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
defer cancel()
```

**Used In:** All long-running operations

### Pattern 3: Channel Select with Default

```go
select {
case result := <-ch:
    // handle
case <-ctx.Done():
    return ctx.Err()
default:
    // non-blocking fallback
}
```

**Used In:** Queue operations

---

## Panic Detection in Logs

Search patterns for log-analyzer:

```mangle
# Detect panic entries
Decl panic_entry(Time, Category, Message).
panic_entry(T, C, M) :- log_entry(T, C, /error, M, _, _),
    fn:contains(M, "panic").

# Detect nil pointer
Decl nil_pointer(Time, Category, Message).
nil_pointer(T, C, M) :- log_entry(T, C, /error, M, _, _),
    fn:contains(M, "nil pointer").

# Detect OOM
Decl oom_event(Time, Category, Message).
oom_event(T, C, M) :- log_entry(T, C, /error, M, _, _),
    fn:contains(M, "out of memory").
```

---

## Panic Prevention Checklist

Before running stress tests, verify:

- [ ] All .mg files in `.nerd/mangle/` are valid
- [ ] Config.json is properly formatted
- [ ] Sufficient disk space (>1GB free)
- [ ] Sufficient memory (>4GB free)
- [ ] No orphan processes from previous runs
- [ ] File descriptor limit is reasonable (`ulimit -n`)
- [ ] Database files are not locked
