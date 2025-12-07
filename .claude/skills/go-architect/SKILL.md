---
name: go-architect
description: Write production-ready, idiomatic Go code following Uber Go Style Guide patterns. This skill prevents common AI coding agent failures including goroutine leaks, race conditions, improper error handling, context mismanagement, and memory leaks. Includes Mangle integration patterns for feeding codeNERD logic systems. Use when writing, reviewing, or refactoring any Go code.
license: Apache-2.0
version: 1.0.0
go_version: 1.21+
last_updated: 2025-12-06
---

# Go Architect: Production-Ready Idiomatic Go

This skill ensures Claude writes safe, idiomatic, production-ready Go code. It addresses the documented failure modes where AI coding agents generate code that compiles but harbors latent defects in concurrency, memory management, error handling, and security.

## CRITICAL: Before Writing Go Code

**Read this section first.** AI agents consistently fail at Go due to training bias toward Python/JavaScript/Java. Go's explicit error handling, CSP concurrency model, and strict type system require different mental models.

### The Competence-Confidence Gap

AI agents generate syntactically correct Go that often:
- Compiles successfully but deadlocks at runtime
- Leaks goroutines that accumulate until OOM
- Ignores errors that cause silent data corruption
- Breaks context cancellation chains
- Creates race conditions on shared state

**Rule**: Every piece of generated Go code must pass the validation checklists in this skill.

## Critical Failure Modes (AI Agent Anti-Patterns)

### 1. Goroutine Leaks - The "Forgotten Sender"

**CRITICAL SEVERITY** - Leads to memory exhaustion and OOM crashes.

```go
// WRONG - AI Pattern: Fire and forget with unbuffered channel
func fetchWithTimeout(ctx context.Context) (string, error) {
    ch := make(chan string)  // Unbuffered!

    go func() {
        result := slowOperation()  // Takes 30 seconds
        ch <- result  // BLOCKED FOREVER if timeout fires first
    }()

    select {
    case result := <-ch:
        return result, nil
    case <-time.After(5 * time.Second):
        return "", errors.New("timeout")
        // Worker goroutine is now LEAKED - blocked on send forever
    }
}
```

```go
// CORRECT - Buffered channel allows goroutine to complete
func fetchWithTimeout(ctx context.Context) (string, error) {
    ch := make(chan string, 1)  // Buffered! Sender never blocks

    go func() {
        result := slowOperation()
        ch <- result  // Completes even if no receiver
    }()

    select {
    case result := <-ch:
        return result, nil
    case <-ctx.Done():
        return "", ctx.Err()
    }
}
```

**Validation**: For every `go func()`, verify there is a guaranteed exit path.

### 2. sync.WaitGroup Misplacement

**HIGH SEVERITY** - Race condition causes premature completion.

```go
// WRONG - AI Pattern: Add inside goroutine (RACE CONDITION)
func processItems(items []string) {
    var wg sync.WaitGroup

    for _, item := range items {
        go func(item string) {
            wg.Add(1)  // WRONG! May execute after Wait() returns
            defer wg.Done()
            process(item)
        }(item)
    }

    wg.Wait()  // May return immediately if goroutines haven't started
}
```

```go
// CORRECT - Add before spawning goroutine
func processItems(items []string) {
    var wg sync.WaitGroup

    for _, item := range items {
        wg.Add(1)  // Add BEFORE the go statement
        go func(item string) {
            defer wg.Done()
            process(item)
        }(item)
    }

    wg.Wait()  // Guaranteed to wait for all goroutines
}
```

**Rule**: `wg.Add(1)` MUST be called in the parent scope, NEVER inside the goroutine.

### 3. Map Race Conditions

**CRITICAL SEVERITY** - Data corruption, panics, undefined behavior.

```go
// WRONG - AI Pattern: Concurrent map access
var cache = make(map[string]string)

func handleRequest(key string) string {
    if val, ok := cache[key]; ok {  // DATA RACE
        return val
    }
    val := computeValue(key)
    cache[key] = val  // DATA RACE - concurrent write
    return val
}
```

```go
// CORRECT - Mutex-protected map
type SafeCache struct {
    mu    sync.RWMutex
    cache map[string]string
}

func (c *SafeCache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.cache[key]
    return val, ok
}

func (c *SafeCache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[key] = value
}
```

**Alternative**: Use `sync.Map` for simple key-value caching.

**Validation**: Every shared map MUST have mutex protection or use sync.Map.

### 4. Context Severance

**MEDIUM SEVERITY** - Cancellation fails, resources wasted.

```go
// WRONG - AI Pattern: Context severance breaks cancellation chain
func processRequest(ctx context.Context) error {
    // Creating new context breaks cancellation propagation!
    subCtx := context.Background()  // WRONG - parent cancellation ignored

    return heavyComputation(subCtx)  // Continues even if parent cancelled
}
```

```go
// CORRECT - Propagate parent context
func processRequest(ctx context.Context) error {
    // Derive from parent to maintain cancellation chain
    subCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    return heavyComputation(subCtx)  // Cancelled when parent cancelled
}
```

**Rule**: NEVER create `context.Background()` inside handlers. Always derive from parent.

### 5. Error Suppression

**HIGH SEVERITY** - Silent failures, data loss, security vulnerabilities.

```go
// WRONG - AI Pattern: Ignoring errors (37.6% increase in vulnerabilities)
func saveData(data []byte) {
    file, _ := os.Create("data.txt")  // Error ignored
    file.Write(data)                   // Error ignored
    file.Close()                       // Error ignored
}
```

```go
// CORRECT - Handle every error
func saveData(data []byte) error {
    file, err := os.Create("data.txt")
    if err != nil {
        return fmt.Errorf("create file: %w", err)
    }
    defer file.Close()

    if _, err := file.Write(data); err != nil {
        return fmt.Errorf("write data: %w", err)
    }

    return nil
}
```

**Rule**: NEVER use `_` to ignore errors except in tests or explicitly documented cases.

### 6. Panic Abuse

**HIGH SEVERITY** - Crashes entire service.

```go
// WRONG - AI Pattern: Using panic for control flow
func ParseConfig(data []byte) Config {
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        panic(err)  // WRONG - crashes entire server
    }
    return cfg
}
```

```go
// CORRECT - Return errors
func ParseConfig(data []byte) (Config, error) {
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return Config{}, fmt.Errorf("parse config: %w", err)
    }
    return cfg, nil
}
```

**Rule**: `panic` is ONLY for unrecoverable programmer errors. Libraries MUST return errors.

### 7. Slice Memory Leaks

**MEDIUM SEVERITY** - Unexpected memory retention.

```go
// WRONG - AI Pattern: Sub-slice keeps entire backing array
func extractHeader(data []byte) []byte {
    // data is 100MB, but we only need first 10 bytes
    return data[:10]  // WRONG - entire 100MB retained!
}
```

```go
// CORRECT - Copy to new slice
func extractHeader(data []byte) []byte {
    header := make([]byte, 10)
    copy(header, data[:10])
    return header  // Only 10 bytes retained
}
```

**Rule**: When returning a sub-slice of large data, COPY to a new slice.

### 8. Nil Channel Deadlock

**CRITICAL SEVERITY** - Silent hang, no error output.

```go
// WRONG - AI Pattern: Uninitialized channel
func process() {
    var ch chan int  // nil channel!

    go func() {
        ch <- 42  // BLOCKS FOREVER on nil channel
    }()

    <-ch  // BLOCKS FOREVER on nil channel
}
```

```go
// CORRECT - Always initialize channels
func process() {
    ch := make(chan int)  // Properly initialized

    go func() {
        ch <- 42
    }()

    fmt.Println(<-ch)
}
```

**Rule**: ALWAYS use `make(chan T)` or `make(chan T, size)`. NEVER leave channels nil.

## Idiomatic Go Patterns

### Interface Design: Accept Interfaces, Return Structs

```go
// CORRECT - Accept interface
func ProcessReader(r io.Reader) error {
    // Can accept *os.File, *bytes.Buffer, *strings.Reader, etc.
    data, err := io.ReadAll(r)
    if err != nil {
        return err
    }
    return process(data)
}

// CORRECT - Return concrete struct
func NewServer(addr string) *Server {
    return &Server{addr: addr}
}
```

**Rule**: Functions accept interfaces (for flexibility), return structs (for clarity).

### Interface Verification at Compile Time

```go
// Verify MyType implements Handler at compile time
var _ Handler = (*MyType)(nil)

// Catches implementation errors during compilation, not runtime
```

### Functional Options Pattern

```go
type ServerOption func(*Server)

func WithTimeout(d time.Duration) ServerOption {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithLogger(l *log.Logger) ServerOption {
    return func(s *Server) {
        s.logger = l
    }
}

func NewServer(addr string, opts ...ServerOption) *Server {
    s := &Server{
        addr:    addr,
        timeout: 30 * time.Second,  // Default
        logger:  log.Default(),     // Default
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(":8080", WithTimeout(time.Minute), WithLogger(myLogger))
```

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 1, 2, 3},
        {"negative", -1, -1, -2},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Add(tt.a, tt.b); got != tt.expected {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

### Worker Pool with Graceful Shutdown

```go
func WorkerPool(ctx context.Context, jobs <-chan Job, workers int) {
    var wg sync.WaitGroup

    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case job, ok := <-jobs:
                    if !ok {
                        return  // Channel closed, exit
                    }
                    process(job)
                case <-ctx.Done():
                    return  // Context cancelled, exit
                }
            }
        }()
    }

    wg.Wait()
}
```

## Error Handling Patterns

### Error Wrapping with Context

```go
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config JSON: %w", err)
    }

    return &cfg, nil
}

// Caller can unwrap
if errors.Is(err, os.ErrNotExist) {
    // Handle file not found
}
```

### Sentinel Errors

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
)

func GetUser(id string) (*User, error) {
    user, ok := users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}
```

### Custom Error Types

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// Type assertion
var validErr *ValidationError
if errors.As(err, &validErr) {
    // Handle validation error
}
```

## Performance Guidelines

### Pre-allocate Slices and Maps

```go
// WRONG - Repeated allocations
func collect(items []string) []Result {
    var results []Result
    for _, item := range items {
        results = append(results, process(item))  // Reallocates
    }
    return results
}

// CORRECT - Pre-allocate
func collect(items []string) []Result {
    results := make([]Result, 0, len(items))  // Pre-allocate capacity
    for _, item := range items {
        results = append(results, process(item))
    }
    return results
}
```

### Use strconv Over fmt

```go
// SLOW
s := fmt.Sprintf("%d", n)

// FAST
s := strconv.Itoa(n)
```

### Avoid Repeated String-to-Byte Conversion

```go
// WRONG - Converts on every call
func processMany(s string, n int) {
    for i := 0; i < n; i++ {
        process([]byte(s))  // Allocates every iteration
    }
}

// CORRECT - Convert once
func processMany(s string, n int) {
    b := []byte(s)  // Convert once
    for i := 0; i < n; i++ {
        process(b)
    }
}
```

## Generics Best Practices

### Use Appropriate Constraints

```go
// Too restrictive
func Sum[T int | int32 | int64](vals []T) T { ... }

// Better - more flexible
func Sum[T constraints.Integer | constraints.Float](vals []T) T {
    var sum T
    for _, v := range vals {
        sum += v
    }
    return sum
}
```

### Use ~ for Underlying Types

```go
type MyInt int

// Won't work with MyInt
func Double[T int](v T) T { return v * 2 }

// Works with MyInt and int
func Double[T ~int](v T) T { return v * 2 }
```

### Apply Generics Judiciously

```go
// DON'T overuse generics
// If code works with specific types, don't genericize it

// DO use generics for
// - Data structures (trees, queues, caches)
// - Algorithm implementations (sort, search)
// - Utility functions (map, filter, reduce)
```

## Module Management

### go.mod Best Practices

```bash
# Initialize module
go mod init github.com/org/repo

# Add dependencies (clean up unused)
go mod tidy

# Verify checksums
go mod verify

# Use replace for local development only
# Remove before committing
```

### Commit Both Files

Always commit `go.mod` AND `go.sum` to version control.

## Security Guidelines

### Avoid Package Hallucinations

AI agents may suggest non-existent packages. Always verify imports exist:

```bash
go get github.com/example/package  # Verify package exists
```

### Use crypto/rand for Security

```go
// WRONG - Predictable
import "math/rand"
token := rand.Int63()

// CORRECT - Cryptographically secure
import "crypto/rand"
b := make([]byte, 32)
rand.Read(b)
```

### Parameterized SQL Queries

```go
// WRONG - SQL injection
query := "SELECT * FROM users WHERE name = '" + name + "'"

// CORRECT - Parameterized
query := "SELECT * FROM users WHERE name = ?"
rows, err := db.Query(query, name)
```

## Mangle Integration: Feeding the Logic Kernel

codeNERD uses Mangle (Datalog) as its logic kernel. This section covers how to properly bridge Go code with Mangle.

### Import Structure

```go
import (
    "github.com/google/mangle/ast"
    "github.com/google/mangle/parse"
    "github.com/google/mangle/analysis"
    "github.com/google/mangle/engine"
    "github.com/google/mangle/factstore"
)
```

### Creating Facts from Go Structs

```go
type FileInfo struct {
    Path     string
    Language string
    Size     int64
}

// Convert Go struct to Mangle fact
func (f *FileInfo) ToAtom() (*ast.Atom, error) {
    return &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "file_topology"},
        Args: []ast.BaseTerm{
            ast.String(f.Path),
            ast.Atom(f.Language),  // Use Atom for enums/constants
            ast.Number(f.Size),
        },
    }, nil
}
```

### Proper Atom vs String Usage

```go
// WRONG - AI hallucinates string-based API
store.Add("file_info", "/path/to/file", "go")  // Does not exist!

// CORRECT - Use proper engine.Value types
fact, _ := factstore.MakeFact("file_info", []engine.Value{
    engine.String("/path/to/file"),  // String for paths
    engine.Atom("go"),               // Atom for language enum
})
store.Add(fact)
```

### Engine Initialization Pattern

```go
func initMangle() (*engine.Engine, error) {
    // 1. Parse rules
    units, err := parse.Unit(strings.NewReader(rules))
    if err != nil {
        return nil, fmt.Errorf("parse rules: %w", err)
    }

    // 2. Analyze for safety and stratification
    programInfo, err := analysis.Analyze(units, nil)
    if err != nil {
        return nil, fmt.Errorf("analyze rules: %w", err)  // Catches safety violations early
    }

    // 3. Create fact store
    store := factstore.NewSimpleInMemoryStore()

    // 4. Create engine
    eng, err := engine.New(programInfo, store)
    if err != nil {
        return nil, fmt.Errorf("create engine: %w", err)
    }

    return eng, nil
}
```

### Querying Mangle from Go

```go
func queryNextAction(ctx context.Context, eng *engine.Engine) (string, error) {
    // Build query atom
    query := &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "next_action"},
        Args: []ast.BaseTerm{
            ast.Variable("Action"),  // Unbound variable for query
        },
    }

    // Execute query
    results, err := eng.Query(ctx, query)
    if err != nil {
        return "", fmt.Errorf("query next_action: %w", err)
    }

    if len(results) == 0 {
        return "", nil  // No action derived
    }

    // Extract result
    actionVal := results[0].Args[0]
    switch v := actionVal.(type) {
    case ast.Atom:
        return string(v), nil
    default:
        return "", fmt.Errorf("unexpected action type: %T", actionVal)
    }
}
```

### External Predicates (Virtual Predicates)

For predicates that query external systems:

```go
// Implement ExternalPredicateCallback for file system queries
func fileExistsCallback(query engine.Query, cb func(engine.Fact)) error {
    // Check binding pattern - is the path bound or free?
    if query.Args[0].IsBound() {
        path := query.Args[0].AsString()
        if _, err := os.Stat(path); err == nil {
            cb(engine.Fact{Args: query.Args})  // File exists
        }
        return nil
    }

    // Path is free - enumerate all files (expensive!)
    return errors.New("file_exists requires bound path argument")
}
```

### codeNERD Integration Pattern

```go
// VirtualStore handles external system queries
type VirtualStore struct {
    predicates map[string]ExternalPredicateHandler
}

func (vs *VirtualStore) Register(predicate string, handler ExternalPredicateHandler) {
    vs.predicates[predicate] = handler
}

// Register file system, AST, and network predicates
vs := NewVirtualStore()
vs.Register("file_exists", fileExistsHandler)
vs.Register("symbol_defined", astQueryHandler)
vs.Register("http_status", networkQueryHandler)
```

### Atom Naming Conventions for Mangle

```go
// Use Go constants that mirror Mangle atoms
const (
    ActionRead   = "/read"
    ActionWrite  = "/write"
    ActionDelete = "/delete"

    StatusPending  = "/pending"
    StatusComplete = "/complete"
    StatusFailed   = "/failed"
)

// Convert to Mangle atom
func statusToAtom(status string) ast.Atom {
    return ast.Atom(status)  // Already in /atom format
}
```

## Validation Checklist

Before submitting any Go code, verify:

### Concurrency
- [ ] Every `go func()` has guaranteed termination
- [ ] Channels are buffered appropriately (no forgotten senders)
- [ ] `wg.Add(1)` is called BEFORE `go func()`
- [ ] Maps accessed from multiple goroutines have mutex protection
- [ ] Context is propagated (no `context.Background()` in handlers)

### Error Handling
- [ ] No ignored errors (no `_, _` except in documented cases)
- [ ] Errors are wrapped with context (`fmt.Errorf("...: %w", err)`)
- [ ] No `panic` for recoverable errors
- [ ] Sentinel errors defined for common cases

### Memory
- [ ] Large slice sub-slices are copied
- [ ] Channels are initialized with `make()`
- [ ] Resources are closed (using `defer`)

### Security
- [ ] Using `crypto/rand` for security-sensitive randomness
- [ ] SQL queries are parameterized
- [ ] Dependencies are verified to exist

### Mangle Integration
- [ ] Using `engine.Atom()` for constants, `engine.String()` for text
- [ ] Running `analysis.Analyze()` before engine creation
- [ ] Proper error handling for parse/analysis failures
- [ ] Context propagation in queries

## Reference Library

For deeper coverage, see the numbered references in the `references/` directory:

- [100-AI_FAILURE_MODES](references/100-AI_FAILURE_MODES.md) - Complete AI failure taxonomy
- [200-CONCURRENCY_PATTERNS](references/200-CONCURRENCY_PATTERNS.md) - Advanced concurrency
- [300-ERROR_HANDLING](references/300-ERROR_HANDLING.md) - Error patterns and anti-patterns
- [400-MANGLE_INTEGRATION](references/400-MANGLE_INTEGRATION.md) - Complete Mangle/Go guide
- [500-UBER_STYLE_GUIDE](references/500-UBER_STYLE_GUIDE.md) - Uber Go Style Guide summary

## Resources

- [Uber Go Style Guide](https://github.com/uber-go/guide)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Go Memory Model](https://go.dev/ref/mem)
- [Google Mangle](https://github.com/google/mangle)

---

**Next step**: For Mangle-specific patterns, see the [mangle-programming](.claude/skills/mangle-programming/SKILL.md) skill.
