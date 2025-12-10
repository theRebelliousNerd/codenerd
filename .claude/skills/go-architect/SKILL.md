---
name: go-architect
description: Write production-ready, idiomatic Go code following Uber Go Style Guide patterns. This skill prevents common AI coding agent failures including goroutine leaks, race conditions, improper error handling, context mismanagement, and memory leaks. Includes Mangle integration patterns and JIT compiler patterns (go:embed, callback wiring, multi-source compilation, SQLite table extension). Use when writing, reviewing, or refactoring any Go code.
license: Apache-2.0
version: 1.1.0
go_version: 1.21+
last_updated: 2025-12-09
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

## go:embed Patterns for Compile-Time Assets

Use `go:embed` to embed resources at compile time, enabling portable binaries with baked-in assets.

### Embedding Database Files

```go
import _ "embed"

// Embedding prompt corpus at compile time
//go:embed prompt_corpus.db
var embeddedCorpusDB []byte

// Embedding with placeholder for CI/CD
//go:embed prompt_corpus.db.placeholder
var embeddedCorpusPlaceholder []byte

// Loading embedded corpus to temp file for SQLite access
func LoadEmbeddedCorpus() (*sql.DB, error) {
    if len(embeddedCorpusDB) == 0 {
        return nil, fmt.Errorf("embedded corpus not available")
    }

    // Write to temp file for SQLite access
    tmpFile, err := os.CreateTemp("", "prompt_corpus_*.db")
    if err != nil {
        return nil, fmt.Errorf("create temp file: %w", err)
    }

    if _, err := tmpFile.Write(embeddedCorpusDB); err != nil {
        tmpFile.Close()
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("write corpus: %w", err)
    }

    if err := tmpFile.Close(); err != nil {
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("close temp file: %w", err)
    }

    // Open with SQLite
    db, err := sql.Open("sqlite3", tmpFile.Name())
    if err != nil {
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("open database: %w", err)
    }

    return db, nil
}
```

**Key Points:**

- Always check `len(embeddedCorpusDB) == 0` before using
- Clean up temp files on errors with `defer` or explicit cleanup
- Return meaningful errors with context wrapping

### Embedding Text Files

```go
//go:embed templates/*.txt
var templateFS embed.FS

func LoadTemplate(name string) (string, error) {
    data, err := templateFS.ReadFile("templates/" + name)
    if err != nil {
        return "", fmt.Errorf("load template %s: %w", name, err)
    }
    return string(data), nil
}
```

## Callback Wiring Pattern (Dependency Injection)

Enable loose coupling between components through function-typed callbacks.

### Callback Type Definitions

```go
// Callback types for dependency injection
type JITDBRegistrar func(shardName string, dbPath string) error
type JITDBUnregistrar func(shardName string) error
type PromptLoader func(shardName, yamlPath, dbPath string) error

// ShardManager with JIT callbacks
type ShardManager struct {
    promptLoader    PromptLoader
    jitRegistrar    JITDBRegistrar
    jitUnregistrar  JITDBUnregistrar
    mu              sync.RWMutex
}

// Setters for dependency injection (called during bootstrap)
func (sm *ShardManager) SetPromptLoader(loader PromptLoader) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.promptLoader = loader
}

func (sm *ShardManager) SetJITRegistrar(registrar JITDBRegistrar) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.jitRegistrar = registrar
}

func (sm *ShardManager) SetJITUnregistrar(unregistrar JITDBUnregistrar) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.jitUnregistrar = unregistrar
}
```

### Factory Functions for Creating Callbacks

```go
// Factory creates a callback that closes over the compiler instance
func CreateJITDBRegistrar(compiler *JITPromptCompiler) JITDBRegistrar {
    return func(shardName string, dbPath string) error {
        return RegisterAgentDBWithJIT(compiler, shardName, dbPath)
    }
}

func CreateJITDBUnregistrar(compiler *JITPromptCompiler) JITDBUnregistrar {
    return func(shardName string) {
        compiler.UnregisterShardDB(shardName)
        logging.Debug("Unregistered agent DB: %s", shardName)
    }
}

// Usage in bootstrap code
func Bootstrap(kernel *Kernel, shardMgr *ShardManager) error {
    // Create JIT compiler
    compiler, err := prompt.NewJITPromptCompiler(
        prompt.WithEmbeddedCorpus(embeddedCorpus),
        prompt.WithKernel(kernel),
    )
    if err != nil {
        return err
    }

    // Wire callbacks
    shardMgr.SetJITRegistrar(prompt.CreateJITDBRegistrar(compiler))
    shardMgr.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(compiler))

    return nil
}
```

**Key Points:**

- Callbacks enable wiring without import cycles
- Factory functions close over dependencies
- Use mutex protection when setting callbacks on shared state
- Always check for nil before invoking callbacks

### Safe Callback Invocation

```go
// Invoke callback with nil-safety
func (sm *ShardManager) RegisterAgentDB(shardName, dbPath string) error {
    sm.mu.RLock()
    registrar := sm.jitRegistrar
    sm.mu.RUnlock()

    if registrar == nil {
        logging.Debug("No JIT registrar configured, skipping DB registration")
        return nil // Not an error - JIT is optional
    }

    return registrar(shardName, dbPath)
}
```

## SQLite Table Extension Pattern

Extend existing databases with new tables at runtime without breaking existing functionality.

### Idempotent Table Creation

```go
// ensurePromptAtomsTable adds prompt_atoms table to existing knowledge DB
func ensurePromptAtomsTable(db *sql.DB) error {
    schema := `
        CREATE TABLE IF NOT EXISTS prompt_atoms (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            atom_id TEXT NOT NULL UNIQUE,
            content TEXT NOT NULL,
            token_count INTEGER NOT NULL,
            category TEXT NOT NULL,
            priority INTEGER DEFAULT 50,
            is_mandatory BOOLEAN DEFAULT FALSE,
            embedding BLOB,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );

        CREATE INDEX IF NOT EXISTS idx_atoms_category ON prompt_atoms(category);
        CREATE INDEX IF NOT EXISTS idx_atoms_priority ON prompt_atoms(priority DESC);
    `

    _, err := db.Exec(schema)
    if err != nil {
        return fmt.Errorf("create prompt_atoms table: %w", err)
    }

    return nil
}

// Call before first use to ensure schema exists
func LoadAgentPrompts(db *sql.DB, yamlPath string) error {
    // Ensure table exists (safe to call multiple times)
    if err := ensurePromptAtomsTable(db); err != nil {
        return err
    }

    // Load YAML data into table
    return loadYAMLIntoTable(db, yamlPath)
}
```

**Key Points:**

- Use `CREATE TABLE IF NOT EXISTS` for idempotency
- Use `CREATE INDEX IF NOT EXISTS` for indexes
- Call schema creation before any table operations
- Safe to call multiple times (no-op if exists)

### Migrating Existing Databases

```go
// AddColumn adds a column if it doesn't exist (SQLite limitation workaround)
func AddColumnIfNotExists(db *sql.DB, table, column, datatype string) error {
    // Check if column exists
    rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
    if err != nil {
        return fmt.Errorf("query table info: %w", err)
    }
    defer rows.Close()

    exists := false
    var cid int
    var name, dataType string
    var notNull, pk int
    var dfltValue *string

    for rows.Next() {
        if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
            return fmt.Errorf("scan column info: %w", err)
        }
        if name == column {
            exists = true
            break
        }
    }

    if exists {
        return nil // Column already exists
    }

    // Add column
    _, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, datatype))
    return err
}
```

## Multi-Source Compilation Pattern (JIT Prompt Compiler)

The JIT Prompt Compiler implements a **System 2 Architecture** for prompt engineering—moving from "Prompt String Concatenation" to a **JIT Linking Loader** with 50ms latency budget.

### Architectural Principles

| Principle | Implementation | Why It Matters |
|-----------|----------------|----------------|
| **Skeleton vs. Flesh** | Mandatory atoms (Mangle) + Optional atoms (Vector) | Prevents "Frankenstein Prompt" anti-pattern |
| **Mangle as Gatekeeper** | Use Datalog for inference, Go for arithmetic | Mangle isn't a calculator—don't use `fn:mult` for scoring |
| **Normalized Tags** | `atom_context_tags` link table | Zero JSON parsing overhead vs text columns |
| **Atom Polymorphism** | `content`, `content_concise`, `content_min` | Graceful degradation under token pressure |
| **Prompt Manifest** | Flight recorder for every compilation | Ouroboros can trace why atoms were included/excluded |

### JIT Compiler with Tiered Sources

```go
// JITPromptCompiler aggregates atoms from multiple sources
type JITPromptCompiler struct {
    // Tier 1: Immutable embedded corpus (highest priority)
    embeddedCorpus *EmbeddedCorpus

    // Tier 2: Project-level database
    projectDB *sql.DB

    // Tier 3: Per-shard databases (keyed by shard ID)
    shardDBs map[string]*sql.DB

    // Kernel for selection logic
    kernel *core.Kernel

    mu sync.RWMutex
}

// Compile aggregates from all registered sources
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var allCandidates []*PromptAtom

    // 1. Load from embedded corpus (always first, highest priority)
    if c.embeddedCorpus != nil {
        allCandidates = append(allCandidates, c.embeddedCorpus.All()...)
    }

    // 2. Load from project DB (if available)
    if c.projectDB != nil {
        projectAtoms, err := c.loadAtomsFromDB(ctx, c.projectDB)
        if err != nil {
            logging.Warn("Failed to load project atoms: %v", err)
            // Continue without project atoms - not fatal
        } else {
            allCandidates = append(allCandidates, projectAtoms...)
        }
    }

    // 3. Load from shard-specific DB (if registered for this shard)
    if cc.ShardID != "" {
        if shardDB, ok := c.shardDBs[cc.ShardID]; ok {
            shardAtoms, err := c.loadAtomsFromDB(ctx, shardDB)
            if err != nil {
                logging.Warn("Failed to load shard atoms: %v", err)
            } else {
                allCandidates = append(allCandidates, shardAtoms...)
            }
        }
    }

    // 4. Run selection logic (Mangle + vector search)
    selected := c.selector.SelectAtoms(ctx, allCandidates, cc)

    // 5. Assemble final prompt
    return c.assembler.Assemble(selected, cc.TokenBudget)
}
```

**Key Points:**

- **Skeleton first**: Embedded corpus provides mandatory atoms (Identity, Safety, Protocol)—compilation fails if these are missing
- **Flesh second**: Project/shard DBs provide contextual atoms—failures are warnings, not errors
- **Graceful degradation**: Missing optional sources result in less helpful but still functional prompts
- Use mutex protection when accessing shared DB maps
- Log warnings for failed sources, don't fail entire compilation

### Registering Dynamic Sources

```go
// RegisterShardDB registers a shard-specific database
func (c *JITPromptCompiler) RegisterShardDB(shardID string, db *sql.DB) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.shardDBs[shardID] = db
    logging.Info("Registered shard DB: %s", shardID)
}

// UnregisterShardDB removes and closes a shard database
func (c *JITPromptCompiler) UnregisterShardDB(shardID string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if db, ok := c.shardDBs[shardID]; ok {
        if err := db.Close(); err != nil {
            logging.Warn("Failed to close shard DB %s: %v", shardID, err)
        }
        delete(c.shardDBs, shardID)
        logging.Info("Unregistered shard DB: %s", shardID)
    }
}
```

## Mangle Integration: Feeding the Logic Kernel

codeNERD uses Mangle (Datalog) as its logic kernel. This section covers how to properly bridge Go code with Mangle.

### CRITICAL: Know What Mangle Can't Do

**The "Mangle as HashMap" Anti-Pattern**: A common AI failure mode is treating Mangle as a key-value store for exact-match lookups expecting fuzzy behavior.

```go
// WRONG MENTAL MODEL - expecting fuzzy matching from Mangle
// Storing 400+ intent_definition facts and expecting semantic search
facts := []Fact{
    {Predicate: "intent_definition", Args: []interface{}{"review my code", "/review", "codebase"}},
    {Predicate: "intent_definition", Args: []interface{}{"check my code", "/review", "codebase"}},
    // ... 400+ more exact-match patterns
}
// User says "audit my code" → NO MATCH (exact string matching only!)
```

**Mangle has NO string functions**:

```go
// THESE DO NOT EXIST - will silently fail or compile error:
// fn:string_contains, fn:substring, fn:match, fn:regex, fn:like
```

**What Mangle excels at** (use these):

```go
// Transitive closure - "what can reach what"
// reachable(X, Z) :- edge(X, Y), reachable(Y, Z).

// Deductive reasoning over facts
// permitted(Action) :- safe_action(Action), !blocked(Action).

// Aggregation after grouping
// count_by_category(Cat, N) :- item(Cat, _) |> do fn:group_by(Cat), let N = fn:Count().
```

**Neuro-Symbolic Solution**: When you need fuzzy/semantic matching:

1. Use **vector embeddings** for the fuzzy part (semantic similarity)
2. Feed results INTO Mangle as `semantic_match` facts
3. Let Mangle do deductive reasoning over the matches

```go
// CORRECT - neuro-symbolic architecture
// 1. Vector search returns semantic matches
matches := semanticClassifier.Search(ctx, userInput)

// 2. Inject as Mangle facts
for _, m := range matches {
    kernel.Assert(Fact{
        Predicate: "semantic_match",
        Args: []interface{}{userInput, m.Sentence, m.Verb, m.Similarity},
    })
}

// 3. Mangle derives final selection using deductive rules
// potential_score(Verb, 100.0) :- semantic_match(_, _, Verb, Sim), Sim >= 85.
```

See [mangle-programming skill Section 11](../mangle-programming/references/150-AI_FAILURE_MODES.md) for complete anti-pattern documentation.

### The "DSL Trap" and Split-Brain Loader

**Root Cause**: Developers treat `.mg` files as general design documents, mixing **Taxonomy** (Data), **Intents** (Configuration), and **Rules** (Logic). Mangle is a strict compiler, not a notebook.

**Solution**: Write a Go pre-processor that routes content to the correct system:

```go
// LoadHybridFile routes mixed content to correct backends
func LoadHybridFile(path string, vectorDB VectorStore, store factstore.FactStore) (string, error) {
    file, _ := os.Open(path)
    scanner := bufio.NewScanner(file)
    var mangleCode strings.Builder

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // Route INTENTs to Vector DB (fuzzy matching)
        if strings.HasPrefix(line, "INTENT:") {
            phrase, intentAtom := parseIntentLine(line)
            vectorDB.Add(phrase, intentAtom)
            continue // Don't send to Mangle!
        }

        // Route TAXONOMY to Mangle Store (graph structure)
        if strings.HasPrefix(line, "TAXONOMY:") {
            child, parent := parseTaxonomyLine(line)
            atom := ast.NewAtom("subclass_of", ast.Name(child), ast.Name(parent))
            store.Add(atom)
            continue
        }

        // Keep real logic for the compiler
        mangleCode.WriteString(line + "\n")
    }

    return mangleCode.String(), nil
}
```

**Key Principle**: One file for readability, but Go routes each line to its correct home (Vector DB vs Mangle).

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

### JIT Prompt Compiler Pattern

The PromptCompiler implements the neuro-symbolic pattern for dynamic prompt assembly. Vector DB finds relevant atoms, Mangle resolves dependencies and conflicts, Go assembles the final string.

```go
// PromptCompiler assembles context-appropriate prompts at runtime
type PromptCompiler struct {
    vectorStore   *store.PromptCorpusStore  // Baked-in atoms
    learnedStore  *store.LearnedPromptStore // User preferences
    kernel        *core.RealKernel          // Mangle linker
    promptTextMap map[string]string         // Atom ID -> text content
    mu            sync.RWMutex
}

// CompilePrompt generates a context-appropriate prompt
func (pc *PromptCompiler) CompilePrompt(ctx context.Context, taskContext string, phase string) (string, error) {
    // 1. DISCOVERY: Vector search for relevant atoms
    queryEmbed, err := pc.embedEngine.Embed(ctx, taskContext)
    if err != nil {
        return pc.fallbackPrompt(phase), nil // Graceful degradation
    }

    hits, err := pc.vectorStore.Search(queryEmbed, 10)
    if err != nil {
        return pc.fallbackPrompt(phase), nil
    }

    // 2. CONTEXT INJECTION: Assert facts for Mangle
    pc.mu.Lock()
    defer pc.mu.Unlock()

    // Clear previous compilation state
    pc.kernel.Retract("vector_hit")
    pc.kernel.Retract("current_phase")

    // Inject current phase
    pc.kernel.Assert(core.Fact{
        Predicate: "current_phase",
        Args:      []interface{}{core.MangleAtom(phase)},
    })

    // Inject vector search results
    for _, hit := range hits {
        pc.kernel.Assert(core.Fact{
            Predicate: "vector_hit",
            Args:      []interface{}{core.MangleAtom(hit.AtomID), hit.Similarity},
        })
    }

    // 3. LINKING: Mangle resolves dependencies and conflicts
    results, err := pc.kernel.Query(ctx, "ordered_result(?AtomID, ?Rank)")
    if err != nil {
        return "", fmt.Errorf("prompt compilation failed: %w", err)
    }

    // 4. ASSEMBLY: Build final prompt string
    type rankedAtom struct {
        ID   string
        Rank int64
    }
    var atoms []rankedAtom
    for _, r := range results {
        atoms = append(atoms, rankedAtom{
            ID:   r.Args[0].(string),
            Rank: r.Args[1].(int64),
        })
    }

    // Sort by rank (safety first, then role, tool, format)
    sort.Slice(atoms, func(i, j int) bool {
        return atoms[i].Rank < atoms[j].Rank
    })

    var builder strings.Builder
    for _, atom := range atoms {
        if text, ok := pc.promptTextMap[atom.ID]; ok {
            builder.WriteString(text)
            builder.WriteString("\n\n")
        }
    }

    return builder.String(), nil
}

// LoadHybridPromptFile parses PROMPT: lines and routes to correct backends
func (pc *PromptCompiler) LoadHybridPromptFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("open prompt file: %w", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var mangleCode strings.Builder

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // Skip comments and empty lines
        if line == "" || strings.HasPrefix(line, "#") {
            mangleCode.WriteString(line + "\n")
            continue
        }

        // Route PROMPT: lines to vector store + text map
        if strings.HasPrefix(line, "PROMPT:") {
            atomID, category, text, err := parsePromptLine(line)
            if err != nil {
                return fmt.Errorf("parse prompt line: %w", err)
            }

            // Store text for assembly
            pc.promptTextMap[atomID] = text

            // Generate embedding and add to vector store
            embed, err := pc.embedEngine.Embed(context.Background(), text)
            if err != nil {
                return fmt.Errorf("embed prompt atom: %w", err)
            }

            if err := pc.vectorStore.Add(atomID, category, embed); err != nil {
                return fmt.Errorf("store prompt atom: %w", err)
            }

            // Also assert category fact for Mangle rules
            pc.kernel.Assert(core.Fact{
                Predicate: "category",
                Args:      []interface{}{core.MangleAtom(atomID), core.MangleAtom(category)},
            })

            continue
        }

        // Keep real Mangle code for the compiler
        mangleCode.WriteString(line + "\n")
    }

    // Load Mangle rules (requires, conflicts, phase gating)
    if mangleCode.Len() > 0 {
        if err := pc.kernel.LoadRules(mangleCode.String()); err != nil {
            return fmt.Errorf("load prompt compiler rules: %w", err)
        }
    }

    return nil
}

// parsePromptLine extracts: PROMPT: /atom_id [category] -> "text"
func parsePromptLine(line string) (atomID, category, text string, err error) {
    // Strip "PROMPT: " prefix
    content := strings.TrimPrefix(line, "PROMPT:")
    content = strings.TrimSpace(content)

    // Extract atom ID (starts with /)
    parts := strings.SplitN(content, " ", 2)
    if len(parts) < 2 || !strings.HasPrefix(parts[0], "/") {
        return "", "", "", fmt.Errorf("invalid atom ID: %s", content)
    }
    atomID = parts[0]
    content = parts[1]

    // Extract category [xxx]
    if !strings.HasPrefix(content, "[") {
        return "", "", "", fmt.Errorf("missing category: %s", content)
    }
    catEnd := strings.Index(content, "]")
    if catEnd == -1 {
        return "", "", "", fmt.Errorf("unclosed category: %s", content)
    }
    category = "/" + content[1:catEnd] // Add / prefix for Mangle atom
    content = strings.TrimSpace(content[catEnd+1:])

    // Extract text after ->
    if !strings.HasPrefix(content, "->") {
        return "", "", "", fmt.Errorf("missing -> separator: %s", content)
    }
    content = strings.TrimSpace(strings.TrimPrefix(content, "->"))

    // Remove surrounding quotes
    if len(content) >= 2 && content[0] == '"' && content[len(content)-1] == '"' {
        text = content[1 : len(content)-1]
    } else {
        text = content
    }

    return atomID, category, text, nil
}
```

**Key Implementation Points:**

1. **Graceful Degradation**: If embedding fails, return a fallback static prompt
2. **Mutex Protection**: Mangle kernel state must be protected during compilation
3. **Retract Before Assert**: Clear previous compilation state to avoid stale facts
4. **Sort by Rank**: Mangle determines priority (safety→role→tool→format)
5. **Text Map Separation**: Prompt text stored in Go map, only metadata in Mangle

### FeedbackLoop Adapter Pattern

When integrating with the Mangle FeedbackLoop for LLM-generated rules, shards must provide an adapter that satisfies the `feedback.LLMClient` interface:

```go
// feedback.LLMClient interface (internal/mangle/feedback/loop.go)
type LLMClient interface {
    Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Adapter wrapping a shard's LLM client
type executiveLLMAdapter struct {
    shard *ExecutivePolicyShard
    ctx   context.Context
}

// Implement the interface with proper guard clause and cost guard integration
func (a *executiveLLMAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
    // Guard: check LLM client availability
    if a.shard.LLMClient == nil {
        return "", fmt.Errorf("no LLM client configured")
    }

    // Cost guard check for each retry attempt
    if a.shard.CostGuard != nil {
        can, reason := a.shard.CostGuard.CanCall()
        if !can {
            return "", fmt.Errorf("LLM call blocked: %s", reason)
        }
    }

    // Delegate to the actual LLM client
    return a.shard.GuardedLLMCall(ctx, systemPrompt, userPrompt)
}
```

**Usage in Shards:**

```go
// Create adapter and call FeedbackLoop
llmAdapter := &executiveLLMAdapter{shard: e, ctx: ctx}

result, err := e.feedbackLoop.GenerateAndValidate(
    ctx,
    llmAdapter,           // Adapter satisfies feedback.LLMClient
    e.Kernel,             // Kernel satisfies feedback.RuleValidator
    systemPrompt,
    userPrompt,
    "executive",          // Domain for valid examples
)

if err != nil {
    logging.SystemShards("Generation failed after %d attempts: %v", result.Attempts, err)
    return err
}

// Use the validated rule
if result.AutoFixed {
    // Sanitizer made corrections - may want lower confidence
    confidence = 0.75
}
validatedRule := result.Rule
```

**RuleValidator Interface:**

The Kernel implements this interface for sandbox validation:

```go
type RuleValidator interface {
    // HotLoadRule compiles rule in sandbox without committing
    HotLoadRule(rule string) error

    // GetDeclaredPredicates returns available predicates for feedback
    GetDeclaredPredicates() []string
}
```

**Key Pattern Points:**

1. **Adapter per shard** - Each shard creates its own adapter wrapping its LLM client
2. **Cost guard integration** - Check `CanCall()` on each retry, not just first attempt
3. **Context propagation** - Pass context through for cancellation
4. **Error wrapping** - Return meaningful errors with context
5. **Guard clauses** - Early return if LLM client unavailable

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
- [ ] Temp files from `go:embed` are cleaned up

### Security
- [ ] Using `crypto/rand` for security-sensitive randomness
- [ ] SQL queries are parameterized
- [ ] Dependencies are verified to exist

### Mangle Integration
- [ ] Using `engine.Atom()` for constants, `engine.String()` for text
- [ ] Running `analysis.Analyze()` before engine creation
- [ ] Proper error handling for parse/analysis failures
- [ ] Context propagation in queries
- [ ] FeedbackLoop adapter implements `feedback.LLMClient` with cost guard checks

### JIT Compiler / Multi-Source Patterns

- [ ] Embedded assets checked for zero-length before use
- [ ] Callbacks are nil-checked before invocation
- [ ] SQLite table creation uses `CREATE TABLE IF NOT EXISTS`
- [ ] Multi-source loading continues on error (graceful degradation)
- [ ] Mutex protection when accessing shared DB maps
- [ ] Database connections closed in `UnregisterShardDB`

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
