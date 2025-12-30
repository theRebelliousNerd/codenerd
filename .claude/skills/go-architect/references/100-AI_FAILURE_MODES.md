# AI Coding Agent Failure Modes in Go

This reference documents the systematic failures of AI coding agents when generating Go code. These failures are predictable consequences of training bias toward Python, JavaScript, and Java—languages with fundamentally different concurrency models, error handling philosophies, and memory semantics.

## The Root Cause: Statistical Mimicry vs. CSP

Large Language Models are probabilistic pattern matchers trained on a corpus dominated by:
- **Python**: Dynamic typing, exceptions, GIL-protected threading
- **JavaScript**: Single-threaded event loop, async/await, promises
- **Java**: Thread-based concurrency, checked exceptions, OOP-heavy

Go's Communicating Sequential Processes (CSP) model, explicit error values, and pass-by-value semantics are underrepresented in training data. The AI "hallucinates" patterns from familiar languages, resulting in code that:

1. **Compiles successfully** (satisfies the type checker)
2. **Looks reasonable** (syntactically similar to working code)
3. **Fails at runtime** (deadlocks, races, leaks, panics)

## Failure Taxonomy

### Category 1: Concurrency Failures

#### 1.1 Goroutine Leaks - The Forgotten Sender

**Frequency**: HIGH | **Severity**: CRITICAL

The AI spawns goroutines that block indefinitely on channel operations.

```go
// AI-GENERATED PATTERN (DANGEROUS)
func queryWithTimeout(ctx context.Context, query string) (Result, error) {
    ch := make(chan Result)  // Unbuffered channel

    go func() {
        result := executeQuery(query)  // Takes 30+ seconds
        ch <- result  // BLOCKS FOREVER if timeout fires
    }()

    select {
    case r := <-ch:
        return r, nil
    case <-time.After(5 * time.Second):
        return Result{}, ErrTimeout
        // Goroutine is now LEAKED - 2KB+ memory consumed forever
    }
}
```

**Root Cause**: AI lacks temporal reasoning. It doesn't model what happens to the sender when the receiver abandons the channel.

**Fix**: Use buffered channel with capacity 1:

```go
ch := make(chan Result, 1)  // Buffered - sender can always complete
```

#### 1.2 WaitGroup Race - The Scheduler Gamble

**Frequency**: HIGH | **Severity**: HIGH

The AI places `wg.Add(1)` inside the goroutine, creating a race condition.

```go
// AI-GENERATED PATTERN (RACE CONDITION)
func processAll(items []Item) {
    var wg sync.WaitGroup

    for _, item := range items {
        go func(item Item) {
            wg.Add(1)  // WRONG - may execute after Wait() returns
            defer wg.Done()
            process(item)
        }(item)
    }

    wg.Wait()  // Returns immediately if goroutines haven't started
}
```

**Root Cause**: AI associates `Add` with the task being performed, not understanding it's a synchronization barrier that must be established before concurrency begins.

**Fix**: Call `Add` in parent scope before `go`:

```go
for _, item := range items {
    wg.Add(1)  // BEFORE go statement
    go func(item Item) {
        defer wg.Done()
        process(item)
    }(item)
}
```

#### 1.3 Map Race - The Implicit Thread Safety

**Frequency**: HIGH | **Severity**: CRITICAL

AI assumes maps are thread-safe (as in Python with GIL, or Java's ConcurrentHashMap).

```go
// AI-GENERATED PATTERN (DATA RACE)
var cache = make(map[string]Result)

func getCached(key string) Result {
    if r, ok := cache[key]; ok {  // READ - data race
        return r
    }
    r := compute(key)
    cache[key] = r  // WRITE - data race
    return r
}
```

**Root Cause**: Python's GIL and Java's synchronized collections create implicit thread safety expectations.

**Fix**: Use sync.RWMutex or sync.Map:

```go
var (
    cacheMu sync.RWMutex
    cache   = make(map[string]Result)
)

func getCached(key string) Result {
    cacheMu.RLock()
    if r, ok := cache[key]; ok {
        cacheMu.RUnlock()
        return r
    }
    cacheMu.RUnlock()

    r := compute(key)

    cacheMu.Lock()
    cache[key] = r
    cacheMu.Unlock()
    return r
}
```

#### 1.4 Nil Channel Deadlock

**Frequency**: MEDIUM | **Severity**: CRITICAL

AI forgets to initialize channels, leading to silent hangs.

```go
// AI-GENERATED PATTERN (DEADLOCK)
type Server struct {
    done chan struct{}  // Zero value is nil
}

func (s *Server) Shutdown() {
    close(s.done)  // PANIC: close of nil channel
}

func (s *Server) Wait() {
    <-s.done  // BLOCKS FOREVER on nil channel
}
```

**Root Cause**: In Python/JS, uninitialized variables are errors. In Go, nil channels are valid but block forever.

**Fix**: Always initialize in constructor:

```go
func NewServer() *Server {
    return &Server{
        done: make(chan struct{}),
    }
}
```

#### 1.5 Channel Close Panic

**Frequency**: MEDIUM | **Severity**: HIGH

AI closes channels from multiple goroutines or sends after close.

```go
// AI-GENERATED PATTERN (PANIC)
func pipeline(in <-chan int) <-chan int {
    out := make(chan int)

    go func() {
        defer close(out)  // Multiple goroutines may close
        for v := range in {
            out <- v * 2
        }
    }()

    go func() {
        defer close(out)  // PANIC: close of closed channel
        for v := range in {
            out <- v + 1
        }
    }()

    return out
}
```

**Rule**: Only ONE goroutine should close a channel (the owner/sender).

### Category 2: Error Handling Failures

#### 2.1 Error Suppression - The Silent Failure

**Frequency**: HIGH | **Severity**: HIGH

AI ignores errors to make code "cleaner."

```go
// AI-GENERATED PATTERN (DATA LOSS)
func saveUser(user User) {
    data, _ := json.Marshal(user)  // Error ignored
    os.WriteFile("user.json", data, 0644)  // Error ignored
}
```

**Root Cause**: AI optimizes for code brevity. Error handling is verbose, so it gets dropped.

**Research Finding**: 37.6% increase in vulnerabilities after 5 rounds of AI "improvements."

**Fix**: Handle every error explicitly:

```go
func saveUser(user User) error {
    data, err := json.Marshal(user)
    if err != nil {
        return fmt.Errorf("marshal user: %w", err)
    }
    if err := os.WriteFile("user.json", data, 0644); err != nil {
        return fmt.Errorf("write user file: %w", err)
    }
    return nil
}
```

#### 2.2 Panic as Exception

**Frequency**: HIGH | **Severity**: HIGH

AI treats panic like Python's raise or Java's throw.

```go
// AI-GENERATED PATTERN (CRASH)
func ParseConfig(path string) Config {
    data, err := os.ReadFile(path)
    if err != nil {
        panic(err)  // WRONG - crashes entire server
    }
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        panic(err)  // WRONG - crashes entire server
    }
    return cfg
}
```

**Root Cause**: Exception semantics from Python/Java/JS.

**Rule**: panic is for programmer errors only. Return errors for all recoverable failures.

#### 2.3 Error Type Confusion

**Frequency**: MEDIUM | **Severity**: MEDIUM

AI doesn't understand error wrapping and type assertions.

```go
// AI-GENERATED PATTERN (INCORRECT)
if err == os.ErrNotExist {  // WRONG - won't match wrapped errors
    // handle
}

// CORRECT
if errors.Is(err, os.ErrNotExist) {
    // handle wrapped errors too
}
```

### Category 3: Context Mismanagement

#### 3.1 Context Severance

**Frequency**: MEDIUM | **Severity**: MEDIUM

AI creates new contexts instead of propagating parents.

```go
// AI-GENERATED PATTERN (BROKEN CANCELLATION)
func handleRequest(ctx context.Context, req Request) {
    // Background work ignores cancellation
    subCtx := context.Background()  // WRONG

    go func() {
        heavyWork(subCtx)  // Continues even if client disconnects
    }()
}
```

**Fix**: Derive from parent:

```go
subCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

#### 3.2 The Fake Timeout

**Frequency**: MEDIUM | **Severity**: MEDIUM

AI wraps code in timeout but doesn't propagate cancellation.

```go
// AI-GENERATED PATTERN (INEFFECTIVE)
func compute(ctx context.Context) int {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    result := 0
    for i := 0; i < 1000000; i++ {
        result += expensiveOp(i)  // Never checks ctx.Done()!
    }
    return result
}
```

**Fix**: Check context in loops:

```go
for i := 0; i < 1000000; i++ {
    select {
    case <-ctx.Done():
        return result  // Honor cancellation
    default:
    }
    result += expensiveOp(i)
}
```

### Category 4: Memory Management Failures

#### 4.1 Slice Reference Leak

**Frequency**: MEDIUM | **Severity**: MEDIUM

AI doesn't understand slice backing array semantics.

```go
// AI-GENERATED PATTERN (MEMORY LEAK)
func getHeader(data []byte) []byte {
    // data is 100MB, we want first 64 bytes
    return data[:64]  // WRONG - keeps entire 100MB in memory
}
```

**Root Cause**: Python/JS slices are copies. Go slices are views into backing arrays.

**Fix**: Copy to new slice:

```go
func getHeader(data []byte) []byte {
    header := make([]byte, 64)
    copy(header, data[:64])
    return header
}
```

#### 4.2 Append Confusion

**Frequency**: MEDIUM | **Severity**: MEDIUM

AI doesn't understand slice header pass-by-value semantics.

```go
// AI-GENERATED PATTERN (DATA LOSS)
func appendItem(items []int, item int) {
    items = append(items, item)  // Modifies local copy only!
}

// Caller's slice is unchanged
```

**Fix**: Return the new slice or use pointer:

```go
func appendItem(items []int, item int) []int {
    return append(items, item)
}
```

### Category 5: Security Failures

#### 5.1 Package Hallucination (Slopsquatting)

**Frequency**: MEDIUM | **Severity**: CRITICAL

AI invents plausible package names that don't exist.

```go
// AI-GENERATED PATTERN (SUPPLY CHAIN RISK)
import "github.com/secure-go/crypto-utils"  // Does not exist!
```

**Risk**: Attackers can register hallucinated package names and inject malware.

**Fix**: Always verify packages exist before using.

#### 5.2 SQL Injection

**Frequency**: HIGH | **Severity**: CRITICAL

AI concatenates strings despite parameterized query support.

```go
// AI-GENERATED PATTERN (INJECTION)
query := "SELECT * FROM users WHERE name = '" + name + "'"
```

**Fix**: Always use parameterized queries:

```go
rows, err := db.Query("SELECT * FROM users WHERE name = ?", name)
```

#### 5.3 Insecure Randomness

**Frequency**: HIGH | **Severity**: HIGH

AI uses math/rand for security-sensitive operations.

```go
// AI-GENERATED PATTERN (PREDICTABLE)
import "math/rand"
token := rand.Int63()  // Predictable!
```

**Fix**: Use crypto/rand:

```go
import "crypto/rand"
b := make([]byte, 32)
crypto/rand.Read(b)
```

### Category 6: Idiomatic Failures

#### 6.1 Interface Pollution

**Frequency**: MEDIUM | **Severity**: LOW

AI creates large Java-style interfaces instead of small Go interfaces.

```go
// AI-GENERATED PATTERN (OVER-ABSTRACTION)
type UserService interface {
    GetUser(id string) (*User, error)
    CreateUser(u *User) error
    UpdateUser(u *User) error
    DeleteUser(id string) error
    ListUsers() ([]*User, error)
    SearchUsers(query string) ([]*User, error)
    // ... 20 more methods
}
```

**Fix**: Small, focused interfaces:

```go
type UserGetter interface {
    GetUser(id string) (*User, error)
}
```

#### 6.2 Return Interface, Accept Struct

**Frequency**: MEDIUM | **Severity**: LOW

AI reverses the Go proverb.

```go
// AI-GENERATED PATTERN (WRONG DIRECTION)
func NewService(cfg *Config) Service {  // Returns interface
    return &service{cfg: cfg}
}

func Process(s *ConcreteService) error {  // Accepts struct
    // ...
}
```

**Correct**: Accept interfaces, return structs.

## Prevention Strategies

### 1. Mandatory Concurrency Review

Any code with `go`, `chan`, `select`, or `sync` requires manual review for:
- Goroutine termination guarantees
- Channel buffer sizing
- Mutex coverage for shared state
- Context propagation

### 2. Static Analysis Pipeline

```bash
# Run race detector
go test -race ./...

# Run staticcheck
staticcheck ./...

# Run gosec for security issues
gosec ./...

# Run golangci-lint with comprehensive rules
golangci-lint run
```

### 3. Explicit Prompting

When asking AI for Go code, include:
- "Use explicit error handling, do not suppress errors"
- "Ensure all goroutines have guaranteed exit paths"
- "Use buffered channels where senders might be abandoned"
- "Propagate context for cancellation"
- "Use parameterized SQL queries"

### 4. Supply Chain Verification

```bash
# Verify all dependencies exist
go mod verify

# Check for known vulnerabilities
go list -json -m all | nancy sleuth
```

## Conclusion

AI coding agents are "Junior Go Developers" at best—they know syntax but lack experience with Go's sharp edges. Every piece of AI-generated Go code should be treated as potentially hazardous material requiring verification before production use.

The human developer's role shifts from writing syntax to being the **Architect of Liveness** (ensuring goroutines terminate) and **Guardian of Safety** (preventing races, panics, and leaks).
